package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// FactOutlineMapping facts → 目录节点目标（CTO 第四步强映射）
type FactOutlineMapping struct {
	FactID        string   `json:"fact_id"`
	FactType      string   `json:"fact_type"`
	FactName      string   `json:"fact_name"`
	TargetLevel   string   `json:"target_level"` // chapter|unit|subsection
	TargetPath    []string `json:"target_path"`
	Required      bool     `json:"required"`
	Priority      string   `json:"priority"`
	MappingReason string   `json:"mapping_reason,omitempty"`
}

// OutlineCoverageResult 结构化覆盖率（与 LLM 审计互补）
type OutlineCoverageResult struct {
	FactTotal                         int      `json:"fact_total"`
	FactMapped                        int      `json:"fact_mapped"`
	CoverageRate                      float64  `json:"coverage_rate"`
	MissingFactIDs                    []string `json:"missing_fact_ids"`
	WeakFactIDs                       []string `json:"weak_fact_ids"`
	DuplicateNodeHints                []string `json:"duplicate_node_hints"`
	SubsectionRequirementCompleteRate float64  `json:"subsection_requirement_complete_rate"`
	Result                            string   `json:"result"` // PASS|REVISE|BLOCK
	Summary                           string   `json:"summary"`
}

// BuildFactToOutlineMappings 将 facts 绑定到骨架上的目标路径（AI）
func (s *TenderDigitizationService) BuildFactToOutlineMappings(ctx context.Context, projectID string, facts *FactExtractResult, sk *IndustrySkeleton) ([]FactOutlineMapping, error) {
	if facts == nil || sk == nil {
		return nil, fmt.Errorf("facts 或骨架为空")
	}
	log.Printf("[Digitize] Building fact→outline mappings for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_fact_outline_mapping")
	if promptBody == "" {
		promptBody = `你是技术标目录编排专家。根据《行业骨架》与《核验事实》，为每条事实生成目录落点映射。

### 核心映射准则：
1. **禁止泛标题**：严禁生成「主体内容」「技术要求响应」「相关措施」「其他说明」等笼统、无意义的标题。
2. **骨架池驱动**：target_path 的第二级（节标题）**必须建议**优先使用骨架中 LogicalChapter 定义的 UnitPool 中的项目。
3. **精准落点**：如果事实属于专项技术（如燃气、基坑、高大模板），小节标题（第三级）必须体现该专项名词。
4. **层级结构**：target_path 必须为 [章标题, 节标题, 小节标题]。

### 行业骨架（JSON）
{{skeleton}}

### 核验事实（JSON）
{{facts}}

### 输出要求
只输出 JSON 数组，不要 Markdown。每条对象字段：
- fact_id: 与事实中 id 一致
- fact_type: score_item|mandatory_spec|project_characteristic|special_topic
- fact_name: 事实名称
- target_level: subsection
- target_path: [章标题, 节标题, 小节标题]，须与骨架中章/节语义一致，严禁出现“主体内容”等废话。
- required: true
- priority: high|medium|low
- mapping_reason: 说明为何选择该落点`
	}
	if sysPrompt == "" {
		sysPrompt = "只输出合法 JSON 数组，不要解释。"
	}

	factsJSON, _ := json.Marshal(facts)
	skJSON, _ := json.Marshal(sk.LogicalChapters)

	cacheContext := fmt.Sprintf("### 行业骨架 chapters\n%s\n\n### 核验事实\n%s", string(skJSON), string(factsJSON))
	dynamicInstruction := "请为每条事实生成一条映射。target_path 必须与骨架章节逻辑一致。"

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.15)
	if err != nil {
		log.Printf("[Digitize] BuildFactToOutlineMappings LLM failed, using fallback: %v", err)
		return buildFactMappingsFallback(facts, sk), nil
	}

	clean := s.extractJSON(res)
	var raw interface{}
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		log.Printf("[Digitize] mapping JSON parse failed, fallback: %v", err)
		return buildFactMappingsFallback(facts, sk), nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		log.Printf("[Digitize] mapping root not array, fallback")
		return buildFactMappingsFallback(facts, sk), nil
	}

	out := make([]FactOutlineMapping, 0, len(arr))
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		fp := FactOutlineMapping{
			FactID:        strings.TrimSpace(pickStringFromMap(m, "fact_id", "id")),
			FactType:      strings.TrimSpace(pickStringFromMap(m, "fact_type", "type")),
			FactName:      strings.TrimSpace(pickStringFromMap(m, "fact_name", "name")),
			TargetLevel:   strings.TrimSpace(pickStringFromMap(m, "target_level", "level")),
			Required:      true,
			Priority:      normalizeFactPriority(pickStringFromMap(m, "priority")),
			MappingReason: pickStringFromMap(m, "mapping_reason", "reason"),
		}
		if v, ok := m["required"].(bool); ok {
			fp.Required = v
		}
		switch tp := m["target_path"].(type) {
		case []interface{}:
			for _, x := range tp {
				if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
					fp.TargetPath = append(fp.TargetPath, strings.TrimSpace(s))
				}
			}
		case string:
			if strings.TrimSpace(tp) != "" {
				fp.TargetPath = []string{strings.TrimSpace(tp)}
			}
		}
		if fp.FactID == "" || len(fp.TargetPath) == 0 {
			continue
		}
		if fp.TargetLevel == "" {
			fp.TargetLevel = "subsection"
		}
		out = append(out, fp)
	}
	if len(out) == 0 {
		return buildFactMappingsFallback(facts, sk), nil
	}
	return out, nil
}

func buildFactMappingsFallback(facts *FactExtractResult, sk *IndustrySkeleton) []FactOutlineMapping {
	var chName, unitName string
	if len(sk.LogicalChapters) > 0 {
		chName = sk.LogicalChapters[0].Name
		if len(sk.LogicalChapters[0].UnitPool) > 0 {
			unitName = sk.LogicalChapters[0].UnitPool[0]
		} else {
			unitName = "第一节 核心技术要求响应"
		}
	} else {
		chName, unitName = "第一章 施工方案与核心技术措施", "第一节 重点难点分析与应对"
	}
	appendOne := func(id, typ, name, pri string) FactOutlineMapping {
		return FactOutlineMapping{
			FactID:      id,
			FactType:    typ,
			FactName:    name,
			TargetLevel: "subsection",
			TargetPath:  []string{chName, unitName, "（一）" + name},
			Required:    true,
			Priority:    normalizeFactPriority(pri),
		}
	}
	out := make([]FactOutlineMapping, 0)
	for _, it := range facts.ScoreItems {
		out = append(out, appendOne(it.ID, "score_item", it.Name, it.Priority))
	}
	for _, it := range facts.MandatorySpecs {
		out = append(out, appendOne(it.ID, "mandatory_spec", it.Name, it.Priority))
	}
	for _, it := range facts.ProjectCharacteristics {
		out = append(out, appendOne(it.ID, "project_characteristic", it.Name, it.Priority))
	}
	for _, it := range facts.SpecialTopics {
		out = append(out, appendOne(it.ID, "special_topic", it.Name, it.Priority))
	}
	return out
}

func collectFactIDSet(facts *FactExtractResult) map[string]struct{} {
	out := make(map[string]struct{})
	if facts == nil {
		return out
	}
	for _, it := range facts.ScoreItems {
		out[it.ID] = struct{}{}
	}
	for _, it := range facts.MandatorySpecs {
		out[it.ID] = struct{}{}
	}
	for _, it := range facts.ProjectCharacteristics {
		out[it.ID] = struct{}{}
	}
	for _, it := range facts.SpecialTopics {
		out[it.ID] = struct{}{}
	}
	return out
}

func priorityOfFactID(facts *FactExtractResult, id string) string {
	if facts == nil {
		return "medium"
	}
	find := func(items []FactItem) string {
		for _, it := range items {
			if it.ID == id {
				return normalizeFactPriority(it.Priority)
			}
		}
		return ""
	}
	if p := find(facts.ScoreItems); p != "" {
		return p
	}
	if p := find(facts.MandatorySpecs); p != "" {
		return p
	}
	if p := find(facts.ProjectCharacteristics); p != "" {
		return p
	}
	if p := find(facts.SpecialTopics); p != "" {
		return p
	}
	return "medium"
}

// EnforceRequirementIDsFromMappings 按映射把 fact_id 写入小节 requirement_ids
func (s *TenderDigitizationService) EnforceRequirementIDsFromMappings(outline []map[string]interface{}, mappings []FactOutlineMapping) []map[string]interface{} {
	if len(mappings) == 0 {
		return outline
	}
	validIDs := map[string]struct{}{}
	for _, m := range mappings {
		if strings.TrimSpace(m.FactID) != "" {
			validIDs[m.FactID] = struct{}{}
		}
	}
	for i := range outline {
		ch := outline[i]
		chName := pickStringFromMap(ch, "name", "title")
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			uName := pickStringFromMap(um, "name", "title")
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				sName := pickStringFromMap(sm, "name", "title")
				ids := s.collectRequirementIDs(sm)
				idSet := map[string]struct{}{}
				for _, x := range ids {
					if _, ok := validIDs[x]; ok {
						idSet[x] = struct{}{}
					}
				}
				for _, m := range mappings {
					if !pathMatchesOutline(m.TargetPath, chName, uName, sName) {
						continue
					}
					if _, ok := idSet[m.FactID]; !ok {
						idSet[m.FactID] = struct{}{}
					}
				}
				if len(idSet) > 0 {
					merged := make([]string, 0, len(idSet))
					for x := range idSet {
						merged = append(merged, x)
					}
					sm["requirement_ids"] = merged
				}
			}
		}
	}
	return outline
}

// AlignFactMappingsToOutline keeps AI/fallback mappings usable when direct generation
// produced valid section titles that differ from the skeleton path labels.
func (s *TenderDigitizationService) AlignFactMappingsToOutline(outline []map[string]interface{}, facts *FactExtractResult, mappings []FactOutlineMapping) []FactOutlineMapping {
	if len(outline) == 0 || len(mappings) == 0 {
		return mappings
	}
	paths := collectOutlineSubsectionPaths(outline)
	if len(paths) == 0 {
		return mappings
	}
	factByID := map[string]FactItem{}
	for _, item := range orderedFacts(facts) {
		factByID[item.ID] = item
	}
	for i := range mappings {
		m := mappings[i]
		if mappingPathExists(m.TargetPath, paths) {
			continue
		}
		best := bestOutlinePathForMapping(m, factByID[m.FactID], paths)
		if len(best) == 0 {
			continue
		}
		mappings[i].TargetPath = best
		if mappings[i].MappingReason == "" {
			mappings[i].MappingReason = "自动按生成目录语义重定位"
		} else {
			mappings[i].MappingReason += "；自动按生成目录语义重定位"
		}
	}
	return mappings
}

// EnsureProjectOverviewSection gives project-level facts and traceability rules a
// non-technical landing zone instead of forcing them into a concrete施工小节.
func (s *TenderDigitizationService) EnsureProjectOverviewSection(outline []map[string]interface{}, facts *FactExtractResult) []map[string]interface{} {
	if facts == nil || len(outline) == 0 {
		return outline
	}
	for _, ch := range outline {
		name := pickStringFromMap(ch, "name", "title")
		if strings.Contains(name, "项目理解") || strings.Contains(name, "总览") || strings.Contains(name, "响应索引") {
			return outline
		}
	}

	projectIDs := make([]interface{}, 0, len(facts.ProjectCharacteristics))
	for _, item := range facts.ProjectCharacteristics {
		if strings.TrimSpace(item.ID) != "" {
			projectIDs = append(projectIDs, item.ID)
		}
	}
	traceIDs := make([]interface{}, 0)
	appendTrace := func(items []FactItem) {
		for _, item := range items {
			text := item.Name + " " + item.Content
			if strings.Contains(text, "追溯") || strings.Contains(text, "引用") || strings.Contains(text, "来源") || strings.Contains(text, "冲突") || strings.Contains(text, "评分") {
				traceIDs = append(traceIDs, item.ID)
			}
		}
	}
	appendTrace(facts.ScoreItems)
	appendTrace(facts.MandatorySpecs)
	appendTrace(facts.SpecialTopics)
	if len(projectIDs) == 0 && len(traceIDs) == 0 {
		return outline
	}

	subs := []interface{}{}
	if len(projectIDs) > 0 {
		subs = append(subs, map[string]interface{}{
			"name":            "一、项目名称、招标人及招标代理机构响应",
			"response_goal":   "集中说明项目基础信息，避免全局信息散落到具体施工措施。",
			"requirement_ids": projectIDs,
		})
	}
	if len(traceIDs) > 0 {
		subs = append(subs, map[string]interface{}{
			"name":            "二、招标条款、评分点与事实引用响应索引",
			"response_goal":   "建立目录节点与招标条款、评分点、事实来源之间的追溯关系。",
			"requirement_ids": traceIDs,
		})
	}
	overview := map[string]interface{}{
		"name": "第一章 项目理解与招标要求响应总览",
		"units": []interface{}{
			map[string]interface{}{
				"name":        "第一节 项目基本情况与响应索引",
				"subsections": subs,
			},
		},
	}
	return append([]map[string]interface{}{overview}, outline...)
}

// NormalizeRequirementIDsFromFacts keeps only real fact IDs and rewrites common LLM aliases
// such as fact_1/fact_2 to the corresponding extracted fact ID.
func (s *TenderDigitizationService) NormalizeRequirementIDsFromFacts(outline []map[string]interface{}, facts *FactExtractResult) []map[string]interface{} {
	ordered := orderedFacts(facts)
	if len(ordered) == 0 {
		return outline
	}
	valid := map[string]struct{}{}
	for _, item := range ordered {
		if strings.TrimSpace(item.ID) != "" {
			valid[item.ID] = struct{}{}
		}
	}

	for i := range outline {
		units, _ := outline[i]["units"].([]interface{})
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
				ids := s.collectRequirementIDs(sm)
				normalized := make([]string, 0, len(ids))
				seen := map[string]struct{}{}
				for _, id := range ids {
					mapped := normalizeFactAlias(id, ordered, valid)
					if mapped == "" {
						continue
					}
					if _, ok := seen[mapped]; ok {
						continue
					}
					seen[mapped] = struct{}{}
					normalized = append(normalized, mapped)
				}
				sm["requirement_ids"] = normalized
			}
		}
	}
	return outline
}

func normalizeFactAlias(id string, ordered []FactItem, valid map[string]struct{}) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if _, ok := valid[id]; ok {
		return id
	}
	matches := regexp.MustCompile(`(?i)^(?:fact|requirement|req|id)[_-]?(\d+)$`).FindStringSubmatch(id)
	if len(matches) == 2 {
		n, err := strconv.Atoi(matches[1])
		if err == nil && n >= 1 && n <= len(ordered) {
			return ordered[n-1].ID
		}
	}
	return ""
}

func pathMatchesOutline(path []string, chapter, unit, subsection string) bool {
	if len(path) == 0 {
		return false
	}
	norm := func(a, b string) bool {
		a, b = strings.TrimSpace(a), strings.TrimSpace(b)
		if a == "" || b == "" {
			return false
		}
		return strings.Contains(strings.ToLower(a), strings.ToLower(b)) || strings.Contains(strings.ToLower(b), strings.ToLower(a))
	}
	if len(path) >= 3 {
		return norm(path[0], chapter) && norm(path[1], unit) && norm(path[2], subsection)
	}
	if len(path) == 2 {
		return norm(path[0], chapter) && norm(path[1], unit)
	}
	return norm(path[len(path)-1], subsection)
}

// BackfillRequirementIDs 为仍为空的小节分配剩余事实 ID，保证非空
func (s *TenderDigitizationService) BackfillRequirementIDs(outline []map[string]interface{}, facts *FactExtractResult) []map[string]interface{} {
	ordered := orderedFacts(facts)
	if len(ordered) == 0 {
		return outline
	}
	projectFactIDs := map[string]struct{}{}
	if facts != nil {
		for _, item := range facts.ProjectCharacteristics {
			if strings.TrimSpace(item.ID) != "" {
				projectFactIDs[item.ID] = struct{}{}
			}
		}
	}
	all := collectFactIDSet(facts)
	used := map[string]struct{}{}
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
				for _, id := range s.collectRequirementIDs(sm) {
					used[id] = struct{}{}
				}
			}
		}
	}

	remaining := make([]string, 0)
	for id := range all {
		if _, ok := used[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == 0 {
		remaining = make([]string, 0, len(ordered))
		for _, item := range ordered {
			if strings.TrimSpace(item.ID) != "" {
				remaining = append(remaining, item.ID)
			}
		}
	}

	ri := 0
	for i := range outline {
		ch := outline[i]
		chName := pickStringFromMap(ch, "name", "title")
		units, _ := ch["units"].([]interface{})
		if len(units) == 0 {
			candidates := filterFactsForOutlineContext(ordered, projectFactIDs, chName, "", chName)
			if best := bestFactIDForOutlineSubsection(candidates, chName, "", chName); best != "" {
				ch["units"] = []interface{}{
					map[string]interface{}{
						"name": "第一节 " + stripChapterNumber(chName) + "响应措施",
						"subsections": []interface{}{
							map[string]interface{}{
								"name":            "一、" + stripChapterNumber(chName) + "落实措施",
								"requirement_ids": []string{best},
							},
						},
					},
				}
				used[best] = struct{}{}
				continue
			}
		}
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			uName := pickStringFromMap(um, "name", "title")
			subs, _ := um["subsections"].([]interface{})
			if len(subs) == 0 {
				candidates := filterFactsForOutlineContext(ordered, projectFactIDs, chName, uName, uName)
				if best := bestFactIDForOutlineSubsection(candidates, chName, uName, uName); best != "" {
					um["subsections"] = []interface{}{
						map[string]interface{}{
							"name":            "一、" + stripChapterNumber(uName) + "落实措施",
							"requirement_ids": []string{best},
						},
					}
					used[best] = struct{}{}
					continue
				}
			}
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				ids := s.collectRequirementIDs(sm)
				if len(ids) > 0 {
					sm["requirement_ids"] = dedupeStringSlice(ids)
					continue
				}
				title := pickStringFromMap(sm, "name", "title")
				candidates := filterFactsForOutlineContext(ordered, projectFactIDs, chName, uName, title)
				if best := bestFactIDForOutlineSubsection(candidates, chName, uName, title); best != "" {
					sm["requirement_ids"] = []string{best}
				} else if ri < len(remaining) {
					sm["requirement_ids"] = []string{pickFallbackFactID(remaining, projectFactIDs, isOverviewContext(chName, uName, title), ri)}
				}
				ri++
			}
		}
	}
	return outline
}

func stripChapterNumber(title string) string {
	t := strings.TrimSpace(title)
	re := regexp.MustCompile(`^第[一二三四五六七八九十百千万\d]+[章节]\s*`)
	t = re.ReplaceAllString(t, "")
	if strings.TrimSpace(t) == "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(t)
}

func bestFactIDForOutlineSubsection(facts []FactItem, chapter, unit, subsection string) string {
	bestID := ""
	bestScore := -1
	path := outlineSubsectionPath{Chapter: chapter, Unit: unit, Subsection: subsection}
	for _, fact := range facts {
		if strings.TrimSpace(fact.ID) == "" {
			continue
		}
		score := scoreMappingAgainstOutlinePath(FactOutlineMapping{FactID: fact.ID, FactName: fact.Name}, fact, path)
		if normalizeFactPriority(fact.Priority) == "high" {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			bestID = fact.ID
		}
	}
	if bestScore <= 0 {
		return ""
	}
	return bestID
}

func filterFactsForOutlineContext(facts []FactItem, projectFactIDs map[string]struct{}, chapter, unit, subsection string) []FactItem {
	if isOverviewContext(chapter, unit, subsection) {
		return facts
	}
	out := make([]FactItem, 0, len(facts))
	for _, fact := range facts {
		if _, isProject := projectFactIDs[fact.ID]; isProject {
			continue
		}
		out = append(out, fact)
	}
	if len(out) == 0 {
		return facts
	}
	return out
}

func pickFallbackFactID(ids []string, projectFactIDs map[string]struct{}, allowProject bool, offset int) string {
	if len(ids) == 0 {
		return ""
	}
	for i := 0; i < len(ids); i++ {
		id := ids[(offset+i)%len(ids)]
		_, isProject := projectFactIDs[id]
		if allowProject || !isProject {
			return id
		}
	}
	return ids[offset%len(ids)]
}

func isOverviewContext(chapter, unit, subsection string) bool {
	context := chapter + " " + unit + " " + subsection
	return strings.Contains(context, "项目理解") ||
		strings.Contains(context, "总览") ||
		strings.Contains(context, "响应索引") ||
		strings.Contains(context, "项目基本情况")
}

func dedupeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// ValidateOutlineSemanticCoverage 语义校验：requirement_ids 非空、ID 合法、覆盖率
func (s *TenderDigitizationService) ValidateOutlineSemanticCoverage(outline []map[string]interface{}, facts *FactExtractResult, mappings []FactOutlineMapping) *OutlineCoverageResult {
	valid := collectFactIDSet(facts)
	res := &OutlineCoverageResult{FactTotal: len(valid)}
	if res.FactTotal == 0 {
		res.CoverageRate = 100
		res.Result = "PASS"
		res.Summary = "无核验事实，跳过覆盖率统计"
		return res
	}

	mappingByFactID := map[string][]FactOutlineMapping{}
	for _, m := range mappings {
		if strings.TrimSpace(m.FactID) == "" {
			continue
		}
		mappingByFactID[m.FactID] = append(mappingByFactID[m.FactID], m)
	}
	factByID := map[string]FactItem{}
	for _, item := range orderedFacts(facts) {
		factByID[item.ID] = item
	}
	useSemanticMappings := len(mappingByFactID) > 0
	covered := map[string]struct{}{}
	weak := map[string]struct{}{}
	subTotal := 0
	subOK := 0
	dupTitles := map[string]int{}

	for _, ch := range outline {
		chName := pickStringFromMap(ch, "name", "title")
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			uName := pickStringFromMap(um, "name", "title")
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				subTotal++
				title := pickStringFromMap(sm, "name", "title")
				if title != "" {
					dupTitles[strings.ToLower(strings.TrimSpace(title))]++
				}
				ids := s.collectRequirementIDs(sm)
				if len(ids) == 0 {
					continue
				}
				subOK++
				for _, id := range ids {
					if _, ok := valid[id]; ok {
						if isOverviewContext(chName, uName, title) {
							covered[id] = struct{}{}
							continue
						}
						if !useSemanticMappings {
							covered[id] = struct{}{}
							continue
						}
						if factIDMatchesMappedPath(mappingByFactID[id], chName, uName, title) ||
							factMatchesOutlineContext(factByID[id], chName, uName, title) {
							covered[id] = struct{}{}
						} else {
							weak[id] = struct{}{}
						}
					}
				}
			}
		}
	}

	res.FactMapped = len(covered)
	if res.FactTotal > 0 {
		res.CoverageRate = float64(res.FactMapped) / float64(res.FactTotal) * 100
	}
	for id := range valid {
		if _, ok := covered[id]; !ok {
			res.MissingFactIDs = append(res.MissingFactIDs, id)
			if priorityOfFactID(facts, id) == "high" {
				res.WeakFactIDs = append(res.WeakFactIDs, id)
			}
		}
	}
	for id := range weak {
		if _, mapped := covered[id]; !mapped {
			appendUniqueString(&res.WeakFactIDs, id)
		}
	}
	for t, n := range dupTitles {
		if n > 1 {
			res.DuplicateNodeHints = append(res.DuplicateNodeHints, fmt.Sprintf("标题重复×%d: %s", n, t))
		}
	}
	if subTotal > 0 {
		res.SubsectionRequirementCompleteRate = float64(subOK) / float64(subTotal) * 100
	}

	// 判定
	switch {
	case res.CoverageRate < 60 || res.SubsectionRequirementCompleteRate < 50:
		res.Result = "BLOCK"
		res.Summary = fmt.Sprintf("覆盖率 %.1f%%，小节 requirement 完整率 %.1f%%，未语义映射事实 %d 条", res.CoverageRate, res.SubsectionRequirementCompleteRate, len(res.MissingFactIDs))
	case res.CoverageRate < 90 || len(res.MissingFactIDs) > 0 || res.SubsectionRequirementCompleteRate < 85:
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("覆盖率 %.1f%%，缺失事实 %v", res.CoverageRate, res.MissingFactIDs)
	case len(res.WeakFactIDs) > 0:
		res.Result = "REVISE"
		res.Summary = fmt.Sprintf("覆盖率 %.1f%%，但存在语义落点不一致事实 %v", res.CoverageRate, res.WeakFactIDs)
	default:
		res.Result = "PASS"
		res.Summary = fmt.Sprintf("覆盖率 %.1f%%，映射完整", res.CoverageRate)
	}
	return res
}

func factIDMatchesMappedPath(mappings []FactOutlineMapping, chapter, unit, subsection string) bool {
	if len(mappings) == 0 {
		return false
	}
	for _, m := range mappings {
		if pathMatchesOutline(m.TargetPath, chapter, unit, subsection) {
			return true
		}
	}
	return false
}

type outlineSubsectionPath struct {
	Chapter    string
	Unit       string
	Subsection string
}

func collectOutlineSubsectionPaths(outline []map[string]interface{}) []outlineSubsectionPath {
	out := make([]outlineSubsectionPath, 0)
	for _, ch := range outline {
		chName := pickStringFromMap(ch, "name", "title")
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			uName := pickStringFromMap(um, "name", "title")
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				sName := pickStringFromMap(sm, "name", "title")
				if strings.TrimSpace(sName) == "" {
					continue
				}
				out = append(out, outlineSubsectionPath{Chapter: chName, Unit: uName, Subsection: sName})
			}
		}
	}
	return out
}

func mappingPathExists(target []string, paths []outlineSubsectionPath) bool {
	for _, p := range paths {
		if pathMatchesOutline(target, p.Chapter, p.Unit, p.Subsection) {
			return true
		}
	}
	return false
}

func bestOutlinePathForMapping(mapping FactOutlineMapping, fact FactItem, paths []outlineSubsectionPath) []string {
	bestScore := -1
	bestIndex := -1
	for i, p := range paths {
		score := scoreMappingAgainstOutlinePath(mapping, fact, p)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return nil
	}
	p := paths[bestIndex]
	return []string{p.Chapter, p.Unit, p.Subsection}
}

func scoreMappingAgainstOutlinePath(mapping FactOutlineMapping, fact FactItem, path outlineSubsectionPath) int {
	context := path.Chapter + " " + path.Unit + " " + path.Subsection
	contextLower := strings.ToLower(context)
	score := 0
	if factMatchesOutlineContext(fact, path.Chapter, path.Unit, path.Subsection) {
		score += 20
	}
	for _, token := range outlineMatchKeywords(mapping.FactName + " " + fact.Name + " " + fact.Content) {
		if strings.Contains(contextLower, strings.ToLower(token)) {
			score += 4
		}
	}
	for _, token := range outlineMatchKeywords(strings.Join(mapping.TargetPath, " ")) {
		if strings.Contains(contextLower, strings.ToLower(token)) {
			score += 3
		}
	}

	switch mapping.FactType {
	case "project_characteristic":
		if strings.Contains(context, "项目理解") || strings.Contains(context, "总体分析") || strings.Contains(context, "履约能力") {
			score += 8
		}
	case "mandatory_spec", "score_item":
		if strings.Contains(context, "施工方案") || strings.Contains(context, "技术措施") || strings.Contains(context, "质量") || strings.Contains(context, "安全") {
			score += 6
		}
	case "special_topic":
		if strings.Contains(context, "施工方案") || strings.Contains(context, "专项") || strings.Contains(context, "技术") {
			score += 5
		}
	}

	if strings.Contains(path.Chapter, "施工方案") {
		score++
	}
	return score
}

func outlineMatchKeywords(s string) []string {
	candidates := []string{
		"项目", "招标", "招标人", "代理", "日期", "施工", "施工组织", "施工方案", "技术", "技术措施",
		"天然气", "燃气", "管道", "安装", "防腐", "PE", "调压", "质量", "验收", "安全",
		"HSE", "文明", "环境", "扬尘", "废弃物", "资源", "人员", "材料", "机械", "库房",
		"履约", "业绩", "商务", "合同", "结算", "付款", "工资", "交通", "噪音",
		"响应", "评分", "追溯", "引用", "来源", "冲突", "合规", "索引",
	}
	out := make([]string, 0)
	for _, c := range candidates {
		if strings.Contains(s, c) {
			out = append(out, c)
		}
	}
	return out
}

func factMatchesOutlineContext(fact FactItem, chapter, unit, subsection string) bool {
	name := strings.TrimSpace(fact.Name)
	if name == "" {
		return false
	}
	context := strings.ToLower(chapter + " " + unit + " " + subsection)
	if strings.Contains(strings.ToLower(context), strings.ToLower(name)) {
		return true
	}
	for _, token := range significantChineseTokens(name) {
		if strings.Contains(context, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func significantChineseTokens(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	replacer := strings.NewReplacer("与", " ", "和", " ", "及", " ", "、", " ", "/", " ", "，", " ", "；", " ", "：", " ", "(", " ", ")", " ", "（", " ", "）", " ")
	parts := strings.Fields(replacer.Replace(s))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len([]rune(p)) >= 2 {
			out = append(out, p)
		}
	}
	if len(out) == 0 && len([]rune(s)) >= 2 {
		out = append(out, s)
	}
	return out
}

func appendUniqueString(values *[]string, value string) {
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	*values = append(*values, value)
}

func orderedFacts(facts *FactExtractResult) []FactItem {
	if facts == nil {
		return nil
	}
	out := make([]FactItem, 0, len(facts.ScoreItems)+len(facts.MandatorySpecs)+len(facts.ProjectCharacteristics)+len(facts.SpecialTopics))
	out = append(out, facts.ScoreItems...)
	out = append(out, facts.MandatorySpecs...)
	out = append(out, facts.ProjectCharacteristics...)
	out = append(out, facts.SpecialTopics...)
	return out
}

// DecideNextFlowMerged LLM 审计与结构化覆盖率合并
func (s *TenderDigitizationService) DecideNextFlowMerged(audit *CoverageAuditResult, cov *OutlineCoverageResult) (string, string) {
	if cov != nil {
		if cov.Result == "BLOCK" {
			return "BLOCK", "结构化映射覆盖率: " + cov.Summary
		}
		if cov.Result == "REVISE" {
			return "REVISE", "结构化映射待补强: " + cov.Summary
		}
	}
	return s.DecideNextFlow(audit)
}
