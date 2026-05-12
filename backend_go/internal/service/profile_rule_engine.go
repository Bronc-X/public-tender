package service

import (
	"fmt"
	"regexp"
	"strings"
)

// RuleHit represents a single match from the rule engine.
type RuleHit struct {
	RuleName    string   `json:"rule_name"`
	Category    string   `json:"category"` // duration/qualification/scoring/procurement/personnel
	Matches     []string `json:"matches"`
	FieldPath   string   `json:"field_path"`
	SourceLines []string `json:"source_lines"`
	Confidence  float64  `json:"confidence"`
}

// profileRule defines a single regex-based extraction rule.
type profileRule struct {
	Name       string
	Category   string
	Pattern    *regexp.Regexp
	FieldPaths []string // which profile field paths this rule maps to
	Confidence float64
}

// getProfileRules returns the first batch of regex rules for the rule engine.
func getProfileRules() []profileRule {
	return []profileRule{
		// ── Duration rules ──
		{
			Name:       "工期天数",
			Category:   "duration",
			Pattern:    regexp.MustCompile(`(\d+)\s*(个)?\s*(日历天|工作日|天|个月|月)`),
			FieldPaths: []string{"total_duration", "duration_requirements"},
			Confidence: 0.85,
		},
		{
			Name:       "工期节点日期",
			Category:   "duration",
			Pattern:    regexp.MustCompile(`(开工|竣工|完工|交付|开始|结束).*?(\d{4}[\-/年]\d{1,2}[\-/月]\d{1,2})`),
			FieldPaths: []string{"schedule_constraints"},
			Confidence: 0.80,
		},
		// ── Qualification rules ──
		{
			Name:       "资质等级",
			Category:   "qualification",
			Pattern:    regexp.MustCompile(`(一级|二级|三级|特级|甲级|乙级|丙级)\s*(及以上)?\s*(资质|资格|企业)`),
			FieldPaths: []string{"qualification_requirements"},
			Confidence: 0.85,
		},
		{
			Name:       "证书名称",
			Category:   "qualification",
			Pattern:    regexp.MustCompile(`(一级|二级)?\s*(注册)?\s*(建造师|安全员|造价师|监理工程师|安全工程师|结构工程师)`),
			FieldPaths: []string{"qualification_certificates", "personnel_requirements"},
			Confidence: 0.80,
		},
		// ── Scoring rules ──
		{
			Name:       "评分分值_前序",
			Category:   "scoring",
			Pattern:    regexp.MustCompile(`(技术|商务|价格|综合|施工组织|报价).{0,20}?(\d+)\s*分`),
			FieldPaths: []string{"method_and_score_weights"},
			Confidence: 0.80,
		},
		{
			Name:       "评分分值_后序",
			Category:   "scoring",
			Pattern:    regexp.MustCompile(`(\d+)\s*分.{0,10}?(技术|商务|价格|综合)`),
			FieldPaths: []string{"method_and_score_weights"},
			Confidence: 0.75,
		},
		{
			Name:       "评分权重",
			Category:   "scoring",
			Pattern:    regexp.MustCompile(`(权重|占比|比例).{0,10}?(\d+)\s*[%％]`),
			FieldPaths: []string{"method_and_score_weights"},
			Confidence: 0.80,
		},
		{
			Name:       "加减分项",
			Category:   "scoring",
			Pattern:    regexp.MustCompile(`(加分|减分|扣分|奖励|惩罚).{0,15}?(\d+)\s*分`),
			FieldPaths: []string{"bonus_items"},
			Confidence: 0.75,
		},
		// ── Procurement rules ──
		{
			Name:       "甲供识别",
			Category:   "procurement",
			Pattern:    regexp.MustCompile(`(甲供|甲方提供|招标人提供|招标人采购|业主提供|建设单位提供)`),
			FieldPaths: []string{"procurement_boundary", "owner_supplied_items"},
			Confidence: 0.85,
		},
		{
			Name:       "乙供识别",
			Category:   "procurement",
			Pattern:    regexp.MustCompile(`(乙供|乙方提供|中标人采购|中标人自行|承包人采购|施工单位自行)`),
			FieldPaths: []string{"procurement_boundary", "contractor_supplied_items"},
			Confidence: 0.85,
		},
		{
			Name:       "采购边界术语",
			Category:   "procurement",
			Pattern:    regexp.MustCompile(`(供货范围|责任界面|界面划分|材料划分|甲乙双方)`),
			FieldPaths: []string{"procurement_boundary"},
			Confidence: 0.70,
		},
		// ── Personnel rules ──
		{
			Name:       "岗位人数",
			Category:   "personnel",
			Pattern:    regexp.MustCompile(`(项目经理|技术负责人|安全员|质检员|施工员|资料员|材料员|试验员|测量员|安全总监).{0,15}?(\d+)\s*(人|名|位)`),
			FieldPaths: []string{"personnel_requirements"},
			Confidence: 0.85,
		},
		{
			Name:       "人数下限",
			Category:   "personnel",
			Pattern:    regexp.MustCompile(`(不少于|至少|最低|不得少于)\s*(\d+)\s*(人|名|位)`),
			FieldPaths: []string{"personnel_requirements"},
			Confidence: 0.75,
		},
	}
}

// RunRuleEngine scans rawText with regex rules and returns all hits.
// It also cross-checks hits against existing profile field values to detect conflicts.
func RunRuleEngine(rawText string, profile *ProjectProfileResult) []RuleHit {
	if rawText == "" || profile == nil {
		return nil
	}

	rules := getProfileRules()
	lines := strings.Split(rawText, "\n")

	var allHits []RuleHit
	// Deduplicate matches per rule to avoid flooding
	type ruleKey struct {
		ruleName string
		match    string
	}
	seen := make(map[ruleKey]bool)

	for _, rule := range rules {
		var ruleMatches []string
		var ruleSourceLines []string

		for _, line := range lines {
			matches := rule.Pattern.FindAllString(line, -1)
			for _, m := range matches {
				m = strings.TrimSpace(m)
				key := ruleKey{rule.Name, m}
				if seen[key] {
					continue
				}
				seen[key] = true
				ruleMatches = append(ruleMatches, m)
				// Trim source line for readability
				sourceLine := strings.TrimSpace(line)
				if len(sourceLine) > 200 {
					sourceLine = sourceLine[:197] + "..."
				}
				ruleSourceLines = append(ruleSourceLines, sourceLine)
			}
		}

		if len(ruleMatches) == 0 {
			continue
		}

		// One hit per field path to allow targeted integration
		for _, fp := range rule.FieldPaths {
			allHits = append(allHits, RuleHit{
				RuleName:    rule.Name,
				Category:    rule.Category,
				Matches:     ruleMatches,
				FieldPath:   fp,
				SourceLines: ruleSourceLines,
				Confidence:  rule.Confidence,
			})
		}
	}

	return allHits
}

// ApplyRuleEngineHits integrates rule engine hits into the profile:
// - Appends rule-matched values as Candidates on the corresponding field
// - Adds conflict warnings to UncertainItems when rule values diverge from model values
func ApplyRuleEngineHits(hits []RuleHit, profile *ProjectProfileResult) {
	if len(hits) == 0 || profile == nil {
		return
	}

	// Group unique match strings by field path
	fieldMatches := make(map[string][]string)
	for _, hit := range hits {
		fieldMatches[hit.FieldPath] = appendUniqueStrings(fieldMatches[hit.FieldPath], hit.Matches...)
	}

	for fieldPath, matches := range fieldMatches {
		field := getProfileFieldByPath(profile, fieldPath)
		if field == nil {
			list := getProfileListByPath(profile, fieldPath)
			if list == nil {
				continue
			}
			items := make([]ProjectProfileListItem, 0, len(matches))
			for _, match := range matches {
				match = strings.TrimSpace(match)
				if match == "" {
					continue
				}
				defaultTrue := true
				items = append(items, ProjectProfileListItem{
					Name:             match,
					Value:            match,
					Confidence:       0.75,
					Missing:          false,
					RequiresEvidence: &defaultTrue,
				})
			}
			*list = mergeProjectProfileLists(*list, items)
			continue
		}

		candidateStr := strings.Join(matches, "；")
		if candidateStr == "" {
			continue
		}

		// Append to Candidates (avoid duplicates)
		field.Candidates = appendUniqueStrings(field.Candidates, candidateStr)

		// Check for conflicts: if model has a value and rule found something different
		if !field.Missing && !isEmptyProjectProfileValue(field.Value) {
			modelVal := strings.ToLower(strings.TrimSpace(field.Value))
			hasOverlap := false
			for _, m := range matches {
				if strings.Contains(modelVal, strings.ToLower(strings.TrimSpace(m))) {
					hasOverlap = true
					break
				}
			}
			if !hasOverlap {
				conflictNote := fmt.Sprintf("规则引擎命中「%s」，但模型值为「%s」，建议人工核实",
					truncateString(candidateStr, 80), truncateString(field.Value, 80))
				profile.UncertainItems = mergeProjectProfileLists(profile.UncertainItems, []ProjectProfileListItem{
					{
						Name:  "规则冲突:" + fieldPath,
						Value: candidateStr,
						Notes: conflictNote,
					},
				})
			}
		}
	}
}

func getProfileListByPath(profile *ProjectProfileResult, path string) *[]ProjectProfileListItem {
	if profile == nil {
		return nil
	}
	switch path {
	case "owner_supplied_items":
		return &profile.ConstructionCoreRequirements.OwnerSuppliedItems
	case "contractor_supplied_items":
		return &profile.ConstructionCoreRequirements.ContractorSuppliedItems
	case "performance_requirements":
		return &profile.BidderRequirements.PerformanceRequirements
	case "personnel_requirements":
		return &profile.BidderRequirements.PersonnelRequirements
	case "financial_requirements":
		return &profile.BidderRequirements.FinancialRequirements
	case "credit_requirements":
		return &profile.BidderRequirements.CreditRequirements
	case "bonus_items":
		return &profile.BidderRequirements.BonusItems
	case "qualification_requirements":
		return &profile.BidderRequirements.QualificationRequirements
	case "other_mandatory_requirements":
		return &profile.BidderRequirements.OtherMandatoryRequirements
	case "scoring_items":
		return &profile.EvaluationAndPerformanceRules.ScoringItems
	case "disqualification_rules":
		return &profile.EvaluationAndPerformanceRules.DisqualificationRules
	default:
		return nil
	}
}

// getProfileFieldByPath returns a pointer to the profile field for the given path.
func getProfileFieldByPath(profile *ProjectProfileResult, path string) *ProjectProfileField {
	switch path {
	case "project_name":
		return &profile.ProjectBaseInfo.ProjectName
	case "owner_unit":
		return &profile.ProjectBaseInfo.OwnerUnit
	case "location":
		return &profile.ProjectBaseInfo.Location
	case "category_and_scope":
		return &profile.ProjectBaseInfo.CategoryAndScope
	case "duration_requirements":
		return &profile.ProjectBaseInfo.DurationRequirements
	case "quality_standard":
		return &profile.ProjectBaseInfo.QualityStandard
	case "material_equipment_rules":
		return &profile.ConstructionCoreRequirements.MaterialEquipmentRules
	case "technical_specifications":
		return &profile.ConstructionCoreRequirements.TechnicalSpecifications
	case "site_management":
		return &profile.ConstructionCoreRequirements.SiteManagement
	case "acceptance_requirements":
		return &profile.ConstructionCoreRequirements.AcceptanceRequirements
	case "special_operations":
		return &profile.ConstructionCoreRequirements.SpecialOperations
	case "procurement_boundary":
		return &profile.ConstructionCoreRequirements.ProcurementBoundary
	case "schedule_constraints":
		return &profile.ConstructionCoreRequirements.ScheduleConstraints
	case "qualification_certificates":
		return &profile.BidderRequirements.QualificationCertificates
	case "method_and_score_weights":
		return &profile.EvaluationAndPerformanceRules.MethodAndScoreWeights
	case "technical_evaluation_dimensions":
		return &profile.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions
	case "payment_method":
		return &profile.EvaluationAndPerformanceRules.PaymentMethod
	case "settlement_rules":
		return &profile.EvaluationAndPerformanceRules.SettlementRules
	case "total_duration":
		return &profile.EvaluationAndPerformanceRules.TotalDuration
	default:
		return nil
	}
}

// appendUniqueStrings appends items to slice, skipping duplicates.
func appendUniqueStrings(existing []string, items ...string) []string {
	seen := make(map[string]bool, len(existing))
	for _, s := range existing {
		seen[s] = true
	}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			existing = append(existing, item)
		}
	}
	return existing
}

// truncateString truncates a string to maxLen characters with "..." suffix.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
