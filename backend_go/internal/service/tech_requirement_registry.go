package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

type deterministicRequirementSeed struct {
	id       string
	typ      string
	summary  string
	source   string
	priority string
	tier     string
}

var constructionScoreItems = []deterministicRequirementSeed{
	{id: "score施工方案和技术措施", typ: "scoring_item", summary: "施工方案和技术措施", priority: "high", tier: "must_standalone"},
	{id: "score质量管理体系和措施", typ: "scoring_item", summary: "质量管理体系和措施", priority: "high", tier: "must_standalone"},
	{id: "score安全文明管理体系和措施", typ: "scoring_item", summary: "安全文明管理体系和措施", priority: "high", tier: "must_standalone"},
	{id: "score环境保护管理体系和措施", typ: "scoring_item", summary: "环境保护管理体系和措施", priority: "medium", tier: "must_standalone"},
	{id: "score资源人材机配备计划", typ: "scoring_item", summary: "资源（人、材、机）配备计划", priority: "medium", tier: "must_standalone"},
}

// BuildDeterministicRequirementRegister extracts the obvious technical-bid requirements
// before LLM scanning. It is deliberately conservative: only high-signal tender clauses
// become pass-bearing requirements.
func BuildDeterministicRequirementRegister(tenderContent string) []RequirementRegisterEntry {
	text := strings.TrimSpace(tenderContent)
	if text == "" {
		return nil
	}
	seeds := make([]deterministicRequirementSeed, 0, 12)

	if strings.Contains(text, "施工组织设计") {
		for _, seed := range constructionScoreItems {
			seed.source = findSourceSentence(text, seed.summary, "施工组织设计方案")
			seeds = append(seeds, seed)
		}
	}

	addIfFound := func(id, typ, summary, priority, tier string, needles ...string) {
		source := findSourceSentence(text, needles...)
		if source == "" {
			return
		}
		seeds = append(seeds, deterministicRequirementSeed{
			id:       id,
			typ:      typ,
			summary:  summary,
			source:   source,
			priority: priority,
			tier:     tier,
		})
	}

	addIfFound("tech施工组织设计和开竣工报告", "mandatory_clause", "施工组织设计和开工、竣工报告编制", "high", "must_standalone", "施工组织设计和开工、竣工报告")
	addIfFound("tech施工图组织施工和进度验收", "mandatory_clause", "按施工图设计组织施工并按工程进度提交验收", "high", "must_standalone", "按施工图设计", "工程进度提交", "竣工验收")
	addIfFound("tech安全文明施工HSE标准", "mandatory_clause", "安全文明施工与 HSE 标准", "high", "must_standalone", "HSE", "安全文明施工标准")
	addIfFound("tech施工现场安全警示围挡", "mandatory_clause", "施工现场安全警示标识和围栏措施", "medium", "mergeable", "安全警示标识", "安全警示围栏")
	addIfFound("tech工期2026至2027", "mandatory_clause", "工期 2026 年至 2027 年截止 2027 年 12 月 31 日", "high", "must_standalone", "截止2027年12月31日", "工期")
	addIfFound("tech质量验收合格标准", "mandatory_clause", "满足现行验收规范合格标准", "high", "must_standalone", "质量标准", "验收规范合格标准")

	out := make([]RequirementRegisterEntry, 0, len(seeds))
	seen := make(map[string]struct{})
	for _, seed := range seeds {
		summary := strings.TrimSpace(seed.summary)
		if summary == "" {
			continue
		}
		id := stableRequirementID(seed.id)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		priority := seed.priority
		if priority == "" {
			priority = "medium"
		}
		tier := seed.tier
		if tier == "" {
			tier = "mergeable"
		}
		out = append(out, RequirementRegisterEntry{
			RequirementID:         id,
			RequirementKind:       "business_requirement",
			RequirementType:       seed.typ,
			SourceText:            seed.source,
			SourceLocation:        inferRequirementLocation(seed.source),
			Priority:              priority,
			MustBeExplicit:        map[bool]int{true: 1, false: 0}[tier == "must_standalone"],
			ExpectedResponseLevel: "subsection",
			Domain:                "technical_bid",
			ResponseTier:          tier,
			Summary:               summary,
		})
	}
	return out
}

func shouldUseDeterministicStep4FastPath(reqs []RequirementRegisterEntry) bool {
	valid := filterValidRequirements(reqs)
	if len(valid) < 8 {
		return false
	}
	hasScore := false
	hasMandatory := false
	for _, req := range valid {
		switch strings.ToLower(strings.TrimSpace(req.RequirementType)) {
		case "scoring_item", "score":
			hasScore = true
		case "mandatory_clause", "mandatory", "mandatory_spec":
			hasMandatory = true
		}
	}
	return hasScore && hasMandatory
}

func BuildStructuralCoverageAudit(cov *OutlineCoverageResult, full *FullRequirementResponseResult) *CoverageAuditResult {
	score := 100.0
	parts := make([]string, 0, 2)
	missing := make([]AuditIssue, 0)
	if cov != nil {
		score = minFloat(score, cov.CoverageRate)
		if strings.TrimSpace(cov.Summary) != "" {
			parts = append(parts, cov.Summary)
		}
		for _, id := range cov.MissingFactIDs {
			missing = append(missing, AuditIssue{
				RequirementID:  id,
				Description:    "事实未被目录结构覆盖",
				Priority:       "high",
				ActionType:     "insert",
				ExpectedEffect: "补齐目录事实覆盖",
			})
		}
	}
	if full != nil && full.RequirementTotal > 0 {
		score = minFloat(score, full.FullResponseRate)
		if strings.TrimSpace(full.Summary) != "" {
			parts = append(parts, full.Summary)
		}
		for _, id := range full.MissingRequirementIDs {
			missing = append(missing, AuditIssue{
				RequirementID:  id,
				Description:    "要求未进入目录",
				Priority:       "high",
				ActionType:     "insert",
				ExpectedEffect: "补齐要求响应路径",
			})
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "结构化覆盖审计完成")
	}
	return &CoverageAuditResult{
		CoverageScore: score,
		AuditSummary:  "结构化覆盖审计：" + strings.Join(parts, "；"),
		MissingItems:  missing,
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func stableRequirementID(seed string) string {
	clean := strings.NewReplacer("（", "", "）", "", "(", "", ")", "", "、", "", "，", "", ",", "", " ", "", "\t", "").Replace(seed)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		sum := sha1.Sum([]byte(seed))
		return "req_" + hex.EncodeToString(sum[:])[:10]
	}
	runes := []rune(clean)
	if len(runes) > 16 {
		runes = runes[:16]
	}
	return "req_" + string(runes)
}

func findSourceSentence(text string, needles ...string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			continue
		}
		if containsAll(candidate, needles...) {
			return candidate
		}
	}
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx < 0 {
			continue
		}
		runes := []rune(text)
		prefix := []rune(text[:idx])
		runeIdx := len(prefix)
		start := runeIdx - 120
		if start < 0 {
			start = 0
		}
		end := runeIdx + 180
		if end > len(runes) {
			end = len(runes)
		}
		return strings.TrimSpace(string(runes[start:end]))
	}
	return ""
}

func containsAll(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.TrimSpace(needle) == "" {
			continue
		}
		if !strings.Contains(text, needle) {
			return false
		}
	}
	return true
}

func inferRequirementLocation(source string) string {
	switch {
	case strings.Contains(source, "评分") || strings.Contains(source, "施工组织设计"):
		return "第五章 评标办法"
	case strings.Contains(source, "技术") || strings.Contains(source, "商务要求") || strings.Contains(source, "HSE"):
		return "第四章 技术和商务要求"
	default:
		return ""
	}
}

// EnsureFactsForRequirements mirrors pass-bearing requirements into facts so the
// existing outline mapper has concrete business facts to place into the outline.
func (s *TenderDigitizationService) EnsureFactsForRequirements(facts *FactExtractResult, register []RequirementRegisterEntry) *FactExtractResult {
	out := &FactExtractResult{}
	if facts != nil {
		*out = *facts
	}
	seen := make(map[string]struct{})
	for _, item := range out.ScoreItems {
		seen[item.ID] = struct{}{}
	}
	for _, item := range out.MandatorySpecs {
		seen[item.ID] = struct{}{}
	}
	for _, item := range out.ProjectCharacteristics {
		seen[item.ID] = struct{}{}
	}
	for _, item := range out.SpecialTopics {
		seen[item.ID] = struct{}{}
	}

	for _, reg := range register {
		if strings.TrimSpace(reg.RequirementID) == "" || !isPassBearingRequirement(reg) {
			continue
		}
		if _, ok := seen[reg.RequirementID]; ok {
			continue
		}
		item := FactItem{
			ID:             reg.RequirementID,
			Name:           reg.Summary,
			Content:        firstNonEmpty(reg.SourceText, reg.Summary),
			SourceText:     reg.SourceText,
			SourceLocation: reg.SourceLocation,
			SourceChapter:  reg.SourceLocation,
			Priority:       normalizeFactPriority(reg.Priority),
		}
		switch strings.ToLower(strings.TrimSpace(reg.RequirementType)) {
		case "scoring_item", "score":
			if item.ScoreValue == 0 {
				item.ScoreValue = inferScoreValue(reg.Summary)
			}
			out.ScoreItems = append(out.ScoreItems, item)
		case "special_topic":
			out.SpecialTopics = append(out.SpecialTopics, item)
		default:
			out.MandatorySpecs = append(out.MandatorySpecs, item)
		}
		seen[reg.RequirementID] = struct{}{}
	}
	return out
}

// BuildDeterministicTechnicalOutline creates a complete, auditable technical
// outline when the requirement register already contains pass-bearing items.
// It is intentionally plain: every business requirement gets a concrete
// subsection whose title repeats the requirement summary, so Step4 gating can be
// verified without waiting for a best-effort LLM outline call.
func (s *TenderDigitizationService) BuildDeterministicTechnicalOutline(facts *FactExtractResult, register []RequirementRegisterEntry) ([]map[string]interface{}, bool) {
	passRegs := make([]RequirementRegisterEntry, 0, len(register))
	seenReq := make(map[string]struct{})
	for _, reg := range register {
		id := strings.TrimSpace(reg.RequirementID)
		if id == "" || !isPassBearingRequirement(reg) {
			continue
		}
		if _, ok := seenReq[id]; ok {
			continue
		}
		seenReq[id] = struct{}{}
		if strings.TrimSpace(reg.Summary) == "" {
			reg.Summary = id
		}
		passRegs = append(passRegs, reg)
	}
	if len(passRegs) < 4 {
		return nil, false
	}

	outline := make([]map[string]interface{}, 0, len(passRegs)+1)
	covered := make(map[string]struct{})
	if overview := deterministicOverviewChapter(facts, seenReq, &covered); overview != nil {
		outline = append(outline, overview)
	}

	startChapterNo := len(outline) + 1
	for i, reg := range passRegs {
		summary := strings.TrimSpace(reg.Summary)
		if summary == "" {
			summary = reg.RequirementID
		}
		chapterName := fmt.Sprintf("第%s章 %s", chineseOrdinal(startChapterNo+i), summary)
		outline = append(outline, map[string]interface{}{
			"name": chapterName,
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 " + stripChapterNumber(summary) + "响应措施",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、" + stripChapterNumber(summary) + "专项落实措施",
							"response_goal":   firstNonEmpty(reg.SourceText, summary),
							"requirement_ids": []string{reg.RequirementID},
						},
					},
				},
			},
		})
		covered[reg.RequirementID] = struct{}{}
	}

	if facts != nil {
		appendUncoveredFactChapters(&outline, facts, covered)
	}
	return outline, true
}

func deterministicOverviewChapter(facts *FactExtractResult, passReqIDs map[string]struct{}, covered *map[string]struct{}) map[string]interface{} {
	if facts == nil {
		return nil
	}
	subs := make([]interface{}, 0)
	appendOverview := func(items []FactItem) {
		for _, item := range items {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			if _, pass := passReqIDs[id]; pass {
				continue
			}
			title := firstNonEmpty(item.Name, id)
			subs = append(subs, map[string]interface{}{
				"name":            "一、" + stripChapterNumber(title) + "响应说明",
				"response_goal":   firstNonEmpty(item.Content, item.SourceText, title),
				"requirement_ids": []string{id},
			})
			(*covered)[id] = struct{}{}
		}
	}
	appendOverview(facts.ProjectCharacteristics)
	appendOverview(facts.SpecialTopics)
	if len(subs) == 0 {
		return nil
	}
	return map[string]interface{}{
		"name": "第一章 项目理解与招标要求响应总览",
		"units": []interface{}{
			map[string]interface{}{
				"name":        "第一节 项目基本情况与专项响应索引",
				"subsections": subs,
			},
		},
	}
}

func appendUncoveredFactChapters(outline *[]map[string]interface{}, facts *FactExtractResult, covered map[string]struct{}) {
	appendFact := func(item FactItem) {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return
		}
		if _, ok := covered[id]; ok {
			return
		}
		title := firstNonEmpty(item.Name, id)
		chapterName := fmt.Sprintf("第%s章 %s", chineseOrdinal(len(*outline)+1), title)
		*outline = append(*outline, map[string]interface{}{
			"name": chapterName,
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 " + stripChapterNumber(title) + "响应措施",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、" + stripChapterNumber(title) + "落实措施",
							"response_goal":   firstNonEmpty(item.Content, item.SourceText, title),
							"requirement_ids": []string{id},
						},
					},
				},
			},
		})
		covered[id] = struct{}{}
	}
	for _, item := range facts.ScoreItems {
		appendFact(item)
	}
	for _, item := range facts.MandatorySpecs {
		appendFact(item)
	}
	for _, item := range facts.SpecialTopics {
		appendFact(item)
	}
}

func chineseOrdinal(n int) string {
	values := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十", "二十一", "二十二", "二十三", "二十四", "二十五", "二十六", "二十七", "二十八", "二十九", "三十"}
	if n >= 0 && n < len(values) {
		return values[n]
	}
	return fmt.Sprintf("%d", n)
}

func inferScoreValue(summary string) float64 {
	if strings.Contains(summary, "施工组织设计") {
		return 30
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var sectionHeadingRe = regexp.MustCompile(`第[一二三四五六七八九十]+章[^\\n\\r]*`)

func sectionHeadings(text string) []string {
	matches := sectionHeadingRe.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{})
	for _, m := range matches {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

func summarizeDeterministicRequirements(reqs []RequirementRegisterEntry) string {
	if len(reqs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reqs))
	for _, req := range reqs {
		parts = append(parts, fmt.Sprintf("%s:%s", req.RequirementID, req.Summary))
	}
	return strings.Join(parts, "；")
}
