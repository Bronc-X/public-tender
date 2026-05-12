package service

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
)

// IndustryHardRule 行业目录硬规则（关键词命中类，可配置为 warn / revise / block）
type IndustryHardRule struct {
	ID                    string   `json:"id"`
	ProfessionSubstrings  []string `json:"profession_substrings"`
	ProjectTypeSubstrings []string `json:"project_type_substrings"`
	KeywordsAny           []string `json:"keywords_any"`
	Severity              string   `json:"severity"` // warn | revise | block
	Message               string   `json:"message"`
}

type industryHardRulesFile struct {
	Rules []IndustryHardRule `json:"rules"`
}

const defaultIndustryHardRulesJSON = `{"rules":[]}`

// LoadIndustryHardRules 从 system_settings 加载；失败或空则返回空规则（不阻断）
func LoadIndustryHardRules(db *sqlx.DB) []IndustryHardRule {
	if db == nil {
		return nil
	}
	var raw string
	if err := db.Get(&raw, `SELECT value FROM system_settings WHERE key = 'tech_bid_industry_hard_rules'`); err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var f industryHardRulesFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		log.Printf("[IndustryRules] invalid JSON tech_bid_industry_hard_rules: %v", err)
		return nil
	}
	return f.Rules
}

func matchProfessionRule(r IndustryHardRule, profession, projectType string) bool {
	p := strings.ToLower(profession + " " + projectType)
	if len(r.ProfessionSubstrings) == 0 && len(r.ProjectTypeSubstrings) == 0 {
		return true
	}
	for _, s := range r.ProfessionSubstrings {
		if strings.TrimSpace(s) != "" && strings.Contains(p, strings.ToLower(strings.TrimSpace(s))) {
			return true
		}
	}
	for _, s := range r.ProjectTypeSubstrings {
		if strings.TrimSpace(s) != "" && strings.Contains(p, strings.ToLower(strings.TrimSpace(s))) {
			return true
		}
	}
	return false
}

// applyIndustryHardRulesToResult 在完全响应硬门槛之后执行，可降级为 REVISE 或 BLOCK
func applyIndustryHardRulesToResult(db *sqlx.DB, res *FullRequirementResponseResult, outline []map[string]interface{}, profession, projectType string) {
	rules := LoadIndustryHardRules(db)
	if len(rules) == 0 {
		return
	}
	titles := CollectOutlineSubsectionTitles(outline)
	combined := strings.ToLower(strings.Join(titles, " "))
	var blocks []string
	var warns []string
	for _, r := range rules {
		if !matchProfessionRule(r, profession, projectType) {
			continue
		}
		if len(r.KeywordsAny) == 0 {
			continue
		}
		ok := false
		for _, kw := range r.KeywordsAny {
			k := strings.TrimSpace(strings.ToLower(kw))
			if k != "" && strings.Contains(combined, k) {
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		msg := strings.TrimSpace(r.Message)
		if msg == "" {
			msg = "行业规则「" + r.ID + "」：目录小节标题中未出现任一关键词 " + strings.Join(r.KeywordsAny, "、")
		}
		sev := strings.ToLower(strings.TrimSpace(r.Severity))
		if sev == "" {
			sev = "warn"
		}
		switch sev {
		case "block":
			blocks = append(blocks, msg)
		case "revise":
			warns = append(warns, msg)
			if res.Result == "PASS" {
				res.Result = "REVISE"
				res.Summary = "【行业硬规则】" + msg
			}
		default:
			warns = append(warns, msg)
		}
	}
	res.HardRuleWarnings = warns
	if len(blocks) > 0 {
		res.Result = "BLOCK"
		res.Summary = "【行业硬规则】" + strings.Join(blocks, "；")
	}
}
