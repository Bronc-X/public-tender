package service

import "strings"

// EnrichTenderForStep4 prepends lightweight domain hints (Phase 3) so downstream AI calls
// see project-type constraints without changing product architecture.
func EnrichTenderForStep4(markdown, profession, projectType string) string {
	pack := domainRulesPack(strings.TrimSpace(profession), strings.TrimSpace(projectType))
	if pack == "" {
		return markdown
	}
	return "<!-- step4_domain_rules -->\n" + pack + "\n\n" + markdown
}

func domainRulesPack(profession, projectType string) string {
	var b strings.Builder
	pt := strings.ToLower(projectType)
	pr := strings.ToLower(profession)

	switch {
	case strings.Contains(pt, "epc") || strings.Contains(pt, "总承包"):
		b.WriteString("【领域规则·EPC】强调设计施工一体化接口、里程碑与联合验收；目录须显式响应设计优化与采购界面。\n")
	case strings.Contains(pt, "市政") || strings.Contains(pr, "市政"):
		b.WriteString("【领域规则·市政】关注交通导改、地下管线保护与夜间施工合规；目录应覆盖交通组织专章。\n")
	case strings.Contains(pt, "房建") || strings.Contains(pr, "建筑"):
		b.WriteString("【领域规则·房建】强调样板引路、垂直运输与危大工程清单对齐；目录须映射评分办法中的施工方案权重。\n")
	default:
		b.WriteString("【领域规则·通用】目录节点须可追溯到招标条款/评分点；禁止仅挂标签而无实质响应路径。\n")
	}
	b.WriteString("编制约束：事实引用须带来源位置；冲突项须在目录层预留响应章节或说明。\n")
	return strings.TrimSpace(b.String())
}
