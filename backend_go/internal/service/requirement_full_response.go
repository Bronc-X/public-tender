package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// RequirementRegisterEntry 招标要求总表单条（真相层，可与 facts ID 对齐）
type RequirementRegisterEntry struct {
	RequirementID         string `json:"requirement_id"`
	RequirementKind       string `json:"requirement_kind,omitempty"`
	RequirementType       string `json:"requirement_type"`
	SourceText            string `json:"source_text"`
	SourceLocation        string `json:"source_location"`
	Priority              string `json:"priority"`
	MustBeExplicit        int    `json:"must_be_explicit"`
	ExpectedResponseLevel string `json:"expected_response_level"`
	Domain                string `json:"domain"`
	ResponseTier          string `json:"response_tier"` // must_standalone | mergeable | background
	Summary               string `json:"summary"`
}

// FullRequirementResponseResult 完全响应率校验（超越「仅挂 requirement_ids」）
type FullRequirementResponseResult struct {
	RequirementTotal           int      `json:"requirement_total"`
	RequirementMapped          int      `json:"requirement_mapped"`
	RequirementFullyResponded  int      `json:"requirement_fully_responded"`
	RequirementWeaklyResponded int      `json:"requirement_weakly_responded"`
	RequirementOnlyTagged      int      `json:"requirement_only_tagged"`
	FullResponseRate           float64  `json:"full_response_rate"`
	WeakResponseRate           float64  `json:"weak_response_rate"`
	ResponseQualityScore       float64  `json:"response_quality_score"`
	MissingRequirementIDs      []string `json:"missing_requirement_ids"`
	WeakRequirementIDs         []string `json:"weak_requirement_ids"`
	OnlyTaggedRequirementIDs   []string `json:"only_tagged_requirement_ids"`
	ShellTitleHints            []string `json:"shell_title_hints"`
	HighPriorityMissingIDs     []string `json:"high_priority_missing_ids"`
	MandatoryMissingIDs        []string `json:"mandatory_missing_ids"`
	MandatoryInsufficientIDs   []string `json:"mandatory_insufficient_ids"`
	OnlyTaggedRatio            float64  `json:"only_tagged_ratio"`
	Result                     string   `json:"result"`
	Summary                    string   `json:"summary"`
	HardRuleWarnings           []string `json:"hard_rule_warnings"`
}

func isMandatoryRequirementType(typ string) bool {
	t := strings.ToLower(strings.TrimSpace(typ))
	if t == "" {
		return false
	}
	if t == "mandatory" || t == "mandatory_spec" {
		return true
	}
	return strings.Contains(t, "mandatory")
}

func isHighPriorityReg(reg RequirementRegisterEntry) bool {
	return normalizeFactPriority(reg.Priority) == "high"
}

var genericShellTitles = []string{
	"施工技术措施", "技术措施", "施工方案", "质量保证", "质量保证措施", "质量管理",
	"安全技术措施", "安全文明施工", "安全施工", "文明施工", "其他", "其他说明", "综合说明",
	"施工部署", "工程概况", "编制依据", "施工准备",
}

var enumPrefixRe = regexp.MustCompile(`^[（(][一二三四五六七八九十百千万\d]+[）)]`)

func stripEnumerationPrefix(title string) string {
	t := strings.TrimSpace(title)
	for i := 0; i < 3; i++ {
		t2 := enumPrefixRe.ReplaceAllString(t, "")
		t2 = strings.TrimSpace(t2)
		if t2 == t {
			break
		}
		t = t2
	}
	return strings.TrimSpace(t)
}

func isGenericShellTitle(title string) bool {
	t := stripEnumerationPrefix(title)
	if t == "" {
		return true
	}
	if utf8.RuneCountInString(t) <= 6 && !strings.ContainsAny(t, "坝垛堤防河道控导险工专项") {
		for _, g := range genericShellTitles {
			if t == g {
				return true
			}
		}
	}
	for _, g := range genericShellTitles {
		if t == g {
			return true
		}
	}
	return false
}

func titleSemanticMatch(title string, entry RequirementRegisterEntry) bool {
	t := strings.ToLower(stripEnumerationPrefix(title))
	key := strings.TrimSpace(entry.Summary)
	if key == "" {
		key = entry.RequirementID
	}
	key = strings.ToLower(key)
	if key != "" && strings.Contains(t, key) {
		return true
	}
	// 至少 4 字连续匹配（简化的关键词命中）
	runes := []rune(key)
	if len(runes) >= 4 {
		n := min(8, len(runes))
		sub := string(runes[:n])
		if strings.Contains(t, sub) {
			return true
		}
	}
	return false
}

func tierIsStandalone(tier string) bool {
	return strings.TrimSpace(strings.ToLower(tier)) == "must_standalone"
}

func synthesizeRegisterFromFacts(facts *FactExtractResult) []RequirementRegisterEntry {
	if facts == nil {
		return nil
	}
	var out []RequirementRegisterEntry
	add := func(id, typ, name, content, pri string, tier string) {
		out = append(out, RequirementRegisterEntry{
			RequirementID:         id,
			RequirementType:       typ,
			SourceText:            content,
			Priority:              normalizeFactPriority(pri),
			MustBeExplicit:        0,
			ExpectedResponseLevel: "subsection",
			ResponseTier:          tier,
			Summary:               name,
		})
	}
	for _, it := range facts.ScoreItems {
		tier := "mergeable"
		if normalizeFactPriority(it.Priority) == "high" {
			tier = "must_standalone"
		}
		add(it.ID, "score", it.Name, it.Content, it.Priority, tier)
	}
	for _, it := range facts.MandatorySpecs {
		add(it.ID, "mandatory", it.Name, it.Content, it.Priority, "must_standalone")
	}
	for _, it := range facts.ProjectCharacteristics {
		add(it.ID, "characteristic", it.Name, it.Content, it.Priority, "mergeable")
	}
	for _, it := range facts.SpecialTopics {
		add(it.ID, "special_topic", it.Name, it.Content, it.Priority, "must_standalone")
	}
	return out
}

// BuildRequirementRegister 从招标文件 + 事实库构建要求总表（全量章节扫描）；确保不遗漏任何硬性条款
func (s *TenderDigitizationService) BuildRequirementRegister(ctx context.Context, projectID string, facts *FactExtractResult, profileJSON, tenderContent string) ([]RequirementRegisterEntry, error) {
	log.Printf("[Digitize] Layer 3: Segmented requirement scanning starting for project: %s", projectID)

	deterministicReqs := BuildDeterministicRequirementRegister(tenderContent)
	if len(deterministicReqs) > 0 {
		log.Printf("[Digitize] Deterministic requirement prepass found %d requirements: %s", len(deterministicReqs), summarizeDeterministicRequirements(deterministicReqs))
	}

	promptBody, _ := s.promptService.GetPromptFull("tech_bid_requirement_register_segment")
	if strings.TrimSpace(promptBody) == "" {
		log.Printf("[Digitize] Requirement register prompt missing, using facts-based synthesis")
		return dedupeRequirements(append(deterministicReqs, synthesizeRegisterFromFacts(facts)...)), nil
	}

	// Step 1: Split to chapters for scanning
	chapters := s.SplitDocumentToChapters(tenderContent)
	log.Printf("[Digitize] Requirement scanner identified %d chapters", len(chapters))

	results := make(chan []RequirementRegisterEntry, len(chapters))
	var wg sync.WaitGroup
	errCh := make(chan error, len(chapters))

	// Parallel chapter scanning for requirements
	workerCount := 5
	taskCh := make(chan TenderChapter, len(chapters))
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range taskCh {
				res, err := s.scanRequirementsFromChapter(ctx, projectID, profileJSON, ch)
				if err != nil {
					errCh <- err
					continue
				}
				results <- res
			}
		}()
	}

	for _, ch := range chapters {
		taskCh <- ch
	}
	close(taskCh)
	wg.Wait()
	close(results)

	// Step 2: Merge and Deduplicate
	allReqs := make([]RequirementRegisterEntry, 0)
	for arr := range results {
		allReqs = append(allReqs, arr...)
	}

	if len(allReqs) == 0 {
		log.Printf("[Digitize] Warning: No requirements found in segmented scan, falling back to facts-based synthesis")
		return dedupeRequirements(append(deterministicReqs, synthesizeRegisterFromFacts(facts)...)), nil
	}

	// Final Deduplication by SourceText hash or ID
	allReqs = append(deterministicReqs, allReqs...)
	final := filterValidRequirements(dedupeRequirements(allReqs))
	if len(final) == 0 {
		log.Printf("[Digitize] Warning: Requirement scan produced only invalid entries, falling back to facts-based synthesis")
		return dedupeRequirements(append(deterministicReqs, synthesizeRegisterFromFacts(facts)...)), nil
	}
	log.Printf("[Digitize] Layer 3 completed. Final requirement registry size: %d", len(final))

	return final, nil
}

func (s *TenderDigitizationService) scanRequirementsFromChapter(ctx context.Context, projectID, profileJSON string, chapter TenderChapter) ([]RequirementRegisterEntry, error) {
	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_requirement_register_segment")
	if promptBody == "" {
		promptBody = `你是招标方案审计专家。请阅读以下章节内容，识别出所有【实质性要求】（评分项、废标项、硬性技术标准）。
输出 JSON 数组：
[{"requirement_id": "...", "requirement_type": "...", "summary": "...", "priority": "high|medium|low", "response_tier": "must_standalone|mergeable"}]`
	}
	if sysPrompt == "" {
		sysPrompt = "只输出 JSON 数组。"
	}

	cacheContext := fmt.Sprintf("### 项目画像\n%s\n\n### 当前章节\n%s\n\n### 章节内容\n%s",
		profileJSON, chapter.Title, chapter.Content)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock("请提取要求清单 JSON 数组。"),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.1)
	if err != nil {
		return nil, err
	}

	var arr []RequirementRegisterEntry
	clean := s.extractJSON(res)
	if err := json.Unmarshal([]byte(clean), &arr); err != nil {
		return nil, nil
	}

	// Attach chapter context
	for i := range arr {
		arr[i].SourceLocation = chapter.Title
	}

	return arr, nil
}

func dedupeRequirements(in []RequirementRegisterEntry) []RequirementRegisterEntry {
	seen := make(map[string]bool)
	var out []RequirementRegisterEntry
	for _, it := range in {
		key := it.Summary + "|" + it.RequirementID
		if !seen[key] {
			out = append(out, it)
			seen[key] = true
		}
	}
	return out
}

func filterValidRequirements(in []RequirementRegisterEntry) []RequirementRegisterEntry {
	out := make([]RequirementRegisterEntry, 0, len(in))
	for _, it := range in {
		if strings.TrimSpace(it.RequirementID) == "" {
			continue
		}
		if !isPassBearingRequirement(it) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func isPassBearingRequirement(reg RequirementRegisterEntry) bool {
	kind := strings.ToLower(strings.TrimSpace(reg.RequirementKind))
	typ := strings.ToLower(strings.TrimSpace(reg.RequirementType))
	value := kind
	if value == "" {
		value = typ
	}
	switch value {
	case "meta_rule", "project_metadata", "project_characteristic", "characteristic", "background", "traceability_rule":
		return false
	}
	if strings.Contains(typ, "meta") || strings.Contains(kind, "meta") {
		return false
	}
	return true
}

type subsectionHit struct {
	title string
	ids   []string
}

func collectSubsections(outline []map[string]interface{}, s *TenderDigitizationService) []subsectionHit {
	var hits []subsectionHit
	for _, ch := range outline {
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				title := pickStringFromMap(sm, "name", "title")
				ids := s.collectRequirementIDs(sm)
				hits = append(hits, subsectionHit{title: title, ids: ids})
			}
		}
	}
	return hits
}

// ValidateFullRequirementResponse 基于要求总表 + 目录小节标题做「完全响应」评估（门槛数值来自 LoadFullResponseGateConfig）
func (s *TenderDigitizationService) ValidateFullRequirementResponse(outline []map[string]interface{}, register []RequirementRegisterEntry, profession, projectType string) *FullRequirementResponseResult {
	res := &FullRequirementResponseResult{}

	// 过滤掉 RequirementID 为空、以及不能作为 Step4 PASS 依据的元规则/项目背景条目。
	var validRegister []RequirementRegisterEntry
	for _, e := range register {
		if strings.TrimSpace(e.RequirementID) != "" && isPassBearingRequirement(e) {
			validRegister = append(validRegister, e)
		}
	}

	if len(validRegister) == 0 {
		if len(register) > 0 {
			res.Result = "BLOCK"
			res.Summary = "要求总表缺少可作为放行依据的业务要求，仅包含元规则或项目背景，不能进入 Step6"
			applyIndustryHardRulesToResult(s.db, res, outline, profession, projectType)
			return res
		}
		res.FullResponseRate = 100
		res.Result = "PASS"
		res.Summary = "要求总表为空（或全部无效），跳过完全响应校验"
		applyIndustryHardRulesToResult(s.db, res, outline, profession, projectType)
		return res
	}
	cfg := LoadFullResponseGateConfig(s.db, profession, projectType)
	res.RequirementTotal = len(validRegister)
	hits := collectSubsections(outline, s)

	byID := map[string][]subsectionHit{}
	for _, h := range hits {
		for _, id := range h.ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			byID[id] = append(byID[id], h)
		}
	}

	for _, reg := range validRegister {
		rid := reg.RequirementID
		list := byID[rid]
		if len(list) == 0 {
			res.MissingRequirementIDs = append(res.MissingRequirementIDs, rid)
			continue
		}
		res.RequirementMapped++
		best := list[0]
		for _, x := range list[1:] {
			if utf8.RuneCountInString(x.title) > utf8.RuneCountInString(best.title) {
				best = x
			}
		}
		generic := isGenericShellTitle(best.title)
		semantic := titleSemanticMatch(best.title, reg)
		standalone := tierIsStandalone(reg.ResponseTier) || reg.MustBeExplicit == 1

		switch {
		case !generic && semantic:
			res.RequirementFullyResponded++
		case !generic:
			res.RequirementFullyResponded++
		case generic && standalone:
			res.RequirementOnlyTagged++
			res.OnlyTaggedRequirementIDs = append(res.OnlyTaggedRequirementIDs, rid)
			res.ShellTitleHints = append(res.ShellTitleHints, fmt.Sprintf("%s → 小节「%s」偏泛化", rid, best.title))
		case generic && !semantic:
			res.RequirementWeaklyResponded++
			res.WeakRequirementIDs = append(res.WeakRequirementIDs, rid)
			res.ShellTitleHints = append(res.ShellTitleHints, fmt.Sprintf("%s → 标题与要求语义匹配弱", rid))
		default:
			res.RequirementWeaklyResponded++
			res.WeakRequirementIDs = append(res.WeakRequirementIDs, rid)
		}
	}

	if res.RequirementTotal > 0 {
		res.FullResponseRate = float64(res.RequirementFullyResponded) / float64(res.RequirementTotal) * 100
		n := res.RequirementWeaklyResponded + res.RequirementOnlyTagged
		if res.RequirementMapped > 0 {
			res.WeakResponseRate = float64(n) / float64(res.RequirementMapped) * 100
			res.OnlyTaggedRatio = float64(res.RequirementOnlyTagged) / float64(res.RequirementMapped) * 100
		}
		// 质量分：充分响应 + 弱响应惩罚 + 空壳标题条数扣分（扣分参数来自 GateConfig）
		shellPen := float64(len(res.ShellTitleHints)) * cfg.ShellPenaltyPerHint
		if shellPen > cfg.ShellPenaltyMax {
			shellPen = cfg.ShellPenaltyMax
		}
		res.ResponseQualityScore = res.FullResponseRate*0.65 + (100-res.WeakResponseRate)*0.35 - shellPen
		if res.ResponseQualityScore > 100 {
			res.ResponseQualityScore = 100
		}
		if res.ResponseQualityScore < 0 {
			res.ResponseQualityScore = 0
		}
	}

	applyFullResponseHardGate(res, validRegister, &cfg)
	applyIndustryHardRulesToResult(s.db, res, outline, profession, projectType)
	return res
}

// ScoreRequirementResponseQuality 从已算好的弱响应率与完全响应率计算质量分（供测试或外部复用）
func ScoreRequirementResponseQuality(fullRate, weakResponseRate float64, shellHintCount int, cfg *FullResponseGateConfig) float64 {
	c := DefaultFullResponseGateConfig()
	if cfg != nil {
		c = *cfg
	}
	shellPen := float64(shellHintCount) * c.ShellPenaltyPerHint
	if shellPen > c.ShellPenaltyMax {
		shellPen = c.ShellPenaltyMax
	}
	q := fullRate*0.65 + (100-weakResponseRate)*0.35 - shellPen
	if q > 100 {
		return 100
	}
	if q < 0 {
		return 0
	}
	return q
}

func registerByID(register []RequirementRegisterEntry) map[string]RequirementRegisterEntry {
	m := make(map[string]RequirementRegisterEntry, len(register))
	for _, reg := range register {
		m[reg.RequirementID] = reg
	}
	return m
}

// applyFullResponseHardGate Step4 强制门槛：分层 Gate A/B/C，结果写入 res.Result / Summary
func applyFullResponseHardGate(res *FullRequirementResponseResult, register []RequirementRegisterEntry, cfg *FullResponseGateConfig) {
	if len(register) == 0 {
		return
	}
	if cfg == nil {
		d := DefaultFullResponseGateConfig()
		cfg = &d
	}
	byID := registerByID(register)

	for _, rid := range res.MissingRequirementIDs {
		if reg, ok := byID[rid]; ok {
			if isHighPriorityReg(reg) {
				res.HighPriorityMissingIDs = append(res.HighPriorityMissingIDs, rid)
			}
			if isMandatoryRequirementType(reg.RequirementType) {
				res.MandatoryMissingIDs = append(res.MandatoryMissingIDs, rid)
			}
		}
	}
	// 强制规范 + 仅挂标签（泛标题）→ 硬阻断；弱响应强制规范在 Gate B 中走 REVISE
	for _, rid := range res.OnlyTaggedRequirementIDs {
		if reg, ok := byID[rid]; ok && isMandatoryRequirementType(reg.RequirementType) {
			res.MandatoryInsufficientIDs = append(res.MandatoryInsufficientIDs, rid)
		}
	}
	res.MandatoryInsufficientIDs = dedupeStrings(res.MandatoryInsufficientIDs)

	// --- Gate A：硬性阻断 ---
	if len(res.HighPriorityMissingIDs) > 0 {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】高优先级要求未进入目录：%v；完全响应率 %.1f%%", res.HighPriorityMissingIDs, res.FullResponseRate)
		return
	}
	if len(res.MandatoryMissingIDs) > 0 {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】强制规范/废标条款未映射进目录：%v；完全响应率 %.1f%%", res.MandatoryMissingIDs, res.FullResponseRate)
		return
	}
	if len(res.MandatoryInsufficientIDs) > 0 {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】强制规范未充分展开（小节泛标题仅挂 ID）：%v", res.MandatoryInsufficientIDs)
		return
	}

	if res.FullResponseRate < cfg.BlockFullRateMax {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】完全响应率 %.1f%% 低于 %.0f%% 下限", res.FullResponseRate, cfg.BlockFullRateMax)
		return
	}
	if res.OnlyTaggedRatio > cfg.OnlyTaggedRatioBlock && res.RequirementMapped > 0 {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】仅挂标签占比 %.1f%% 过高（阈值 %.0f%%）", res.OnlyTaggedRatio, cfg.OnlyTaggedRatioBlock)
		return
	}
	if len(res.ShellTitleHints) > cfg.ShellHintsBlockCount {
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("【硬门槛】疑似空壳/泛标题小节过多（%d 条，阈值 %d）", len(res.ShellTitleHints), cfg.ShellHintsBlockCount)
		return
	}

	// --- Gate B：full_response_rate + 缺失 ---
	if len(res.MissingRequirementIDs) > 0 {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】尚有 %d 条要求未映射进目录：%v；完全响应率 %.1f%%", len(res.MissingRequirementIDs), res.MissingRequirementIDs, res.FullResponseRate)
		return
	}

	// mandatory 弱响应（非仅挂 ID）→ 至少修订
	mandWeak := make([]string, 0)
	for _, rid := range res.WeakRequirementIDs {
		if reg, ok := byID[rid]; ok && isMandatoryRequirementType(reg.RequirementType) {
			mandWeak = append(mandWeak, rid)
		}
	}
	if len(mandWeak) > 0 {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】强制规范语义匹配偏弱，需加强小节标题：%v", mandWeak)
		return
	}

	if res.RequirementOnlyTagged > 0 {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】存在 %d 条「仅挂 requirement_ids、标题偏泛化」的要求，需改为具体小节标题", res.RequirementOnlyTagged)
		return
	}
	if res.RequirementWeaklyResponded > 0 {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】存在 %d 条弱响应要求（标题与要求语义匹配不足）", res.RequirementWeaklyResponded)
		return
	}

	if res.FullResponseRate < cfg.PassFullRateMin {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】完全响应率 %.1f%% 未达 %.0f%% 放行线", res.FullResponseRate, cfg.PassFullRateMin)
		return
	}
	if res.ResponseQualityScore < cfg.MinQualityScore {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】响应质量分 %.1f 未达 %.0f 分（含空壳标题等扣分）", res.ResponseQualityScore, cfg.MinQualityScore)
		return
	}

	// --- Gate C：辅助 ---
	if len(res.ShellTitleHints) > cfg.ShellHintsReviseCount {
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("【门槛】仍存在 %d 条结构提示（> %d），建议继续消除泛标题", len(res.ShellTitleHints), cfg.ShellHintsReviseCount)
		return
	}

	res.Result = "PASS"
	res.Summary = fmt.Sprintf("【完全响应】已通过硬门槛：完全响应率 %.1f%%，质量分 %.1f", res.FullResponseRate, res.ResponseQualityScore)
}

type RequirementCoverageEntry struct {
	ReqID          string   `json:"req_id"`
	MatchedFactIDs []string `json:"matched_fact_ids"`
	MatchStatus    string   `json:"match_status"` // full, weak, missing
	WeakReason     string   `json:"weak_reason"`
	MissingReason  string   `json:"missing_reason"`
	NeedsRescan    bool     `json:"needs_rescan"`
}

type CoverageAuditMatrix struct {
	Entries       []RequirementCoverageEntry `json:"entries"`
	CoverageScore float64                    `json:"coverage_score"`
	MissingCount  int                        `json:"missing_count"`
	TotalCount    int                        `json:"total_count"`
}

// BuildRequirementCoverageMatrix compares the independent requirement register with the fact registry.
func (s *TenderDigitizationService) BuildRequirementCoverageMatrix(facts *FactExtractResult, requirements []RequirementRegisterEntry) *CoverageAuditMatrix {
	factIDs := make(map[string]bool)
	allFacts := append(facts.ScoreItems, facts.MandatorySpecs...)
	allFacts = append(allFacts, facts.ProjectCharacteristics...)
	allFacts = append(allFacts, facts.SpecialTopics...)

	for _, f := range allFacts {
		factIDs[f.ID] = true
	}

	matrix := &CoverageAuditMatrix{
		Entries:    make([]RequirementCoverageEntry, 0, len(requirements)),
		TotalCount: len(requirements),
	}

	matchedCount := 0
	for _, req := range requirements {
		entry := RequirementCoverageEntry{
			ReqID: req.RequirementID,
		}
		if factIDs[req.RequirementID] {
			entry.MatchedFactIDs = []string{req.RequirementID}
			entry.MatchStatus = "full"
			matchedCount++
		} else {
			entry.MatchStatus = "missing"
			entry.NeedsRescan = true
			matrix.MissingCount++
		}
		matrix.Entries = append(matrix.Entries, entry)
	}

	if matrix.TotalCount > 0 {
		matrix.CoverageScore = float64(matchedCount) / float64(matrix.TotalCount) * 100
	} else {
		matrix.CoverageScore = 100
	}

	return matrix
}

// EvaluateRequirementCompleteness decides on PASS/BLOCK based on coverage matrix.
func (s *TenderDigitizationService) EvaluateRequirementCompleteness(matrix *CoverageAuditMatrix) (string, string) {
	if matrix.MissingCount > 0 {
		// If high priority missing, block.
		return "REVISE", fmt.Sprintf("发现 %d 条未命中的招标要求，需触发回扫补抽。", matrix.MissingCount)
	}
	if matrix.CoverageScore < 95 {
		return "REVISE", fmt.Sprintf("覆盖率 %.1f%% 低于 95%%，建议补强。", matrix.CoverageScore)
	}
	return "PASS", "标书要求已完全覆盖。"
}

// RescanMissingRequirements performs targeted extraction for requirements that were missed in the initial fact extraction.
func (s *TenderDigitizationService) RescanMissingRequirements(ctx context.Context, projectID string, matrix *CoverageAuditMatrix, requirements []RequirementRegisterEntry, tenderContent string) (*FactExtractResult, error) {
	var missingReqs []RequirementRegisterEntry
	for _, entry := range matrix.Entries {
		if entry.NeedsRescan {
			for _, r := range requirements {
				if r.RequirementID == entry.ReqID {
					missingReqs = append(missingReqs, r)
					break
				}
			}
		}
	}
	if len(missingReqs) == 0 {
		return &FactExtractResult{}, nil
	}

	chapters := s.SplitDocumentToChapters(tenderContent)
	rescanContext := buildRescanContext(missingReqs, chapters, tenderContent)
	log.Printf("[Digitize] Rescan: Attempting to extract facts for %d missing requirements across %d chapter slices (%d chars)", len(missingReqs), len(rescanContext.ChapterTitles), len(rescanContext.Content))

	reqsJSON, _ := json.Marshal(missingReqs)
	prompt := `以下是识别出但尚未提取具体事实的招标文件要求。请仅基于给定的相关章节片段，从原文中精准提取这些要求对应的【具体事实内容】及【原文证据】。
要求清单：
%s

相关章节：
%s

相关原文片段：
%s`

	// Relaxed timeout to 120s to avoid being too restrictive compared to global AI client timeouts (180s)
	rescanCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage("你是一个极度仔细的补漏专家。只返回 JSON 格式的 FactExtractResult。不得臆造未在片段中出现的事实。"),
		BuildDynamicUserBlock(fmt.Sprintf(prompt, string(reqsJSON), rescanContext.TitleSummary, rescanContext.Content)),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(rescanCtx, messages, 0.05)
	if err != nil {
		return nil, err
	}

	var result FactExtractResult
	cleanJSON := s.extractJSON(res)
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return &FactExtractResult{}, nil
	}
	return &result, nil
}

const (
	rescanChapterLimit     = 3
	rescanContextCharLimit = 24000
)

type rescanContextWindow struct {
	Content       string
	ChapterTitles []string
	TitleSummary  string
}

func buildRescanContext(missingReqs []RequirementRegisterEntry, chapters []TenderChapter, tenderContent string) rescanContextWindow {
	if len(chapters) == 0 {
		return rescanContextWindow{
			Content:      truncateForRescan(strings.TrimSpace(tenderContent), rescanContextCharLimit),
			TitleSummary: "未识别章节，已回退到全文截断片段",
		}
	}

	selected := collectRelevantRescanChapters(missingReqs, chapters)
	if len(selected) == 0 {
		selected = append(selected, chapters[0])
	}

	parts := make([]string, 0, len(selected))
	titles := make([]string, 0, len(selected))
	used := 0
	for _, ch := range selected {
		content := strings.TrimSpace(ch.Content)
		if content == "" {
			continue
		}
		remaining := rescanContextCharLimit - used
		if remaining <= 0 {
			break
		}
		content = truncateForRescan(content, remaining)
		parts = append(parts, fmt.Sprintf("## %s\n%s", ch.Title, content))
		titles = append(titles, ch.Title)
		used += len(content)
	}

	if len(parts) == 0 {
		return rescanContextWindow{
			Content:      truncateForRescan(strings.TrimSpace(tenderContent), rescanContextCharLimit),
			TitleSummary: "相关章节为空，已回退到全文截断片段",
		}
	}

	return rescanContextWindow{
		Content:       strings.Join(parts, "\n\n"),
		ChapterTitles: titles,
		TitleSummary:  strings.Join(titles, "；"),
	}
}

func collectRelevantRescanChapters(missingReqs []RequirementRegisterEntry, chapters []TenderChapter) []TenderChapter {
	selected := make([]TenderChapter, 0, rescanChapterLimit)
	seen := make(map[int]struct{})
	addChapter := func(ch TenderChapter) {
		if len(selected) >= rescanChapterLimit {
			return
		}
		if _, ok := seen[ch.Index]; ok {
			return
		}
		seen[ch.Index] = struct{}{}
		selected = append(selected, ch)
	}

	for _, req := range missingReqs {
		for _, ch := range matchChaptersForRequirement(req, chapters) {
			addChapter(ch)
		}
		if len(selected) >= rescanChapterLimit {
			break
		}
	}

	return selected
}

func matchChaptersForRequirement(req RequirementRegisterEntry, chapters []TenderChapter) []TenderChapter {
	if len(chapters) == 0 {
		return nil
	}

	location := normalizeRescanLookupText(req.SourceLocation)
	summary := normalizeRescanLookupText(req.Summary)
	source := normalizeRescanLookupText(req.SourceText)

	matched := make([]TenderChapter, 0, 2)
	for _, ch := range chapters {
		titleNorm := normalizeRescanLookupText(ch.Title)
		contentNorm := normalizeRescanLookupText(ch.Content)
		if location != "" && strings.Contains(titleNorm, location) {
			matched = append(matched, ch)
			continue
		}
		if summary != "" && (strings.Contains(titleNorm, summary) || strings.Contains(contentNorm, summary)) {
			matched = append(matched, ch)
			continue
		}
		if source != "" {
			snippet := source
			if len([]rune(snippet)) > 24 {
				snippet = string([]rune(snippet)[:24])
			}
			if strings.Contains(contentNorm, normalizeRescanLookupText(snippet)) {
				matched = append(matched, ch)
			}
		}
		if len(matched) >= 2 {
			break
		}
	}

	if len(matched) > 0 {
		return matched
	}

	for _, ch := range chapters {
		if strings.Contains(normalizeRescanLookupText(ch.Content), summary) {
			return []TenderChapter{ch}
		}
	}

	return []TenderChapter{chapters[0]}
}

func normalizeRescanLookupText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

func truncateForRescan(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// DecideNextFlowWithFullResponse Step4 总闸门：先完全响应硬门槛，再结构化覆盖率，最后 LLM 审计
func (s *TenderDigitizationService) DecideNextFlowWithFullResponse(audit *CoverageAuditResult, cov *OutlineCoverageResult, full *FullRequirementResponseResult) (string, string) {
	if full != nil && full.RequirementTotal > 0 {
		if full.Result == "BLOCK" {
			return "BLOCK", "[Step4 Gate] " + full.Summary
		}
		if full.Result == "REVISE" {
			return "REVISE", "[Step4 Gate] " + full.Summary
		}
		// full.Result == PASS → 继续与映射覆盖率、审计合并
	}
	return s.DecideNextFlowMerged(audit, cov)
}
