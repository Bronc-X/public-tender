package service

import (
	"strings"
	"testing"
)

func TestBuildDeterministicRequirementRegisterExtractsTechnicalScoringItems(t *testing.T) {
	t.Parallel()
	tender := `
第五章 评标办法
施工组织设计30%
施工组织设计方案，包括但不限于①施工方案和技术措施、②质量管理体系和措施、③安全文明管理体系和措施、④环境保护管理体系和措施、⑤资源（人、材、机）配备计划等。

第四章 技术和商务要求
（1）编制所承担工作的“施工组织设计和开工、竣工报告”。
（2）按施工图设计要求与审定的“施工组织设计”组织施工和按工程进度提交招标人验收。
（3）负责所承担的工程施工中的安全和文明施工，按西南油气田分公司HSE有关规定达到安全文明施工标准。
`

	got := BuildDeterministicRequirementRegister(tender)

	if len(got) < 6 {
		t.Fatalf("expected technical scoring and mandatory requirements, got %d: %#v", len(got), got)
	}
	assertHasRequirement(t, got, "施工方案和技术措施", "scoring_item")
	assertHasRequirement(t, got, "质量管理体系和措施", "scoring_item")
	assertHasRequirement(t, got, "安全文明管理体系和措施", "scoring_item")
	assertHasRequirement(t, got, "环境保护管理体系和措施", "scoring_item")
	assertHasRequirement(t, got, "资源（人、材、机）配备计划", "scoring_item")
	assertHasRequirement(t, got, "安全文明施工与 HSE 标准", "mandatory_clause")
}

func TestEnsureFactsForRequirementsAddsPassBearingFactsAndSkipsMetaRules(t *testing.T) {
	t.Parallel()
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{{ID: "ms_01", Name: "目录追溯要求", Content: "目录节点须可追溯到招标条款", Priority: "high"}},
	}
	register := []RequirementRegisterEntry{
		{
			RequirementID:   "meta_trace",
			RequirementType: "meta_rule",
			Priority:        "high",
			Summary:         "目录追溯要求",
			SourceText:      "目录节点须可追溯到招标条款",
		},
		{
			RequirementID:   "tech_safety_hse",
			RequirementType: "mandatory_clause",
			Priority:        "high",
			Summary:         "安全文明施工与 HSE 标准",
			SourceText:      "负责所承担的工程施工中的安全和文明施工，按西南油气田分公司HSE有关规定达到安全文明施工标准。",
		},
		{
			RequirementID:   "score_quality",
			RequirementType: "scoring_item",
			Priority:        "high",
			Summary:         "质量管理体系和措施",
			SourceText:      "施工组织设计方案包括质量管理体系和措施。",
		},
	}

	out := svc.EnsureFactsForRequirements(facts, register)

	if findFact(out.MandatorySpecs, "meta_trace") {
		t.Fatalf("meta rule should not be copied into pass-bearing facts: %+v", out.MandatorySpecs)
	}
	if !findFact(out.MandatorySpecs, "tech_safety_hse") {
		t.Fatalf("mandatory requirement should be added to MandatorySpecs: %+v", out.MandatorySpecs)
	}
	if !findFact(out.ScoreItems, "score_quality") {
		t.Fatalf("scoring requirement should be added to ScoreItems: %+v", out.ScoreItems)
	}
}

func TestBuildDeterministicTechnicalOutlineFullyRespondsToRequirements(t *testing.T) {
	t.Parallel()
	svc := &TenderDigitizationService{}
	register := []RequirementRegisterEntry{
		{
			RequirementID:         "req_score施工方案和技术措施",
			RequirementKind:       "business_requirement",
			RequirementType:       "scoring_item",
			Priority:              "high",
			MustBeExplicit:        1,
			ResponseTier:          "must_standalone",
			Summary:               "施工方案和技术措施",
			ExpectedResponseLevel: "subsection",
		},
		{
			RequirementID:         "req_score质量管理体系和措施",
			RequirementKind:       "business_requirement",
			RequirementType:       "scoring_item",
			Priority:              "high",
			MustBeExplicit:        1,
			ResponseTier:          "must_standalone",
			Summary:               "质量管理体系和措施",
			ExpectedResponseLevel: "subsection",
		},
		{
			RequirementID:         "req_tech安全文明施工HSE标准",
			RequirementKind:       "business_requirement",
			RequirementType:       "mandatory_clause",
			Priority:              "high",
			MustBeExplicit:        1,
			ResponseTier:          "must_standalone",
			Summary:               "安全文明施工与 HSE 标准",
			ExpectedResponseLevel: "subsection",
		},
		{
			RequirementID:         "req_tech工期2026至2027",
			RequirementKind:       "business_requirement",
			RequirementType:       "mandatory_clause",
			Priority:              "high",
			MustBeExplicit:        1,
			ResponseTier:          "must_standalone",
			Summary:               "工期 2026 年至 2027 年截止 2027 年 12 月 31 日",
			ExpectedResponseLevel: "subsection",
		},
	}
	facts := svc.EnsureFactsForRequirements(&FactExtractResult{}, register)

	outline, ok := svc.BuildDeterministicTechnicalOutline(facts, register)
	if !ok {
		t.Fatalf("deterministic outline was not built")
	}

	full := svc.ValidateFullRequirementResponse(outline, register, "", "")
	if full.Result != "PASS" {
		t.Fatalf("full response = %s, want PASS: %+v", full.Result, full)
	}
	cov := svc.ValidateOutlineSemanticCoverage(outline, facts, nil)
	if cov.Result != "PASS" {
		t.Fatalf("coverage = %s, want PASS: %+v", cov.Result, cov)
	}
}

func TestShouldUseDeterministicStep4FastPathRequiresEnoughBusinessRequirements(t *testing.T) {
	t.Parallel()
	tender := `
第五章 评标办法
施工组织设计方案，包括但不限于①施工方案和技术措施、②质量管理体系和措施、③安全文明管理体系和措施、④环境保护管理体系和措施、⑤资源（人、材、机）配备计划等。
（1）编制所承担工作的“施工组织设计和开工、竣工报告”。
（2）按施工图设计要求与审定的“施工组织设计”组织施工和按工程进度提交招标人验收。
（3）负责所承担的工程施工中的安全和文明施工，按西南油气田分公司HSE有关规定达到安全文明施工标准。
（4）施工现场管理应采取相应措施（如安全警示标识、安全警示围栏等）。
1、工期：2026年-2027年（截止2027年12月31日）。
5 质量标准 满足国家相关法律、法规的规定及现行验收规范合格标准。
`
	reqs := BuildDeterministicRequirementRegister(tender)
	if !shouldUseDeterministicStep4FastPath(reqs) {
		t.Fatalf("expected deterministic fast path for complete tender requirements: %+v", reqs)
	}
	if shouldUseDeterministicStep4FastPath(reqs[:3]) {
		t.Fatalf("fast path should not run with too few business requirements")
	}
}

func TestBuildStructuralCoverageAuditUsesWorstGateScore(t *testing.T) {
	t.Parallel()
	cov := &OutlineCoverageResult{CoverageRate: 100, Result: "PASS", Summary: "coverage ok"}
	full := &FullRequirementResponseResult{RequirementTotal: 4, FullResponseRate: 91, Result: "PASS", Summary: "full ok"}

	audit := BuildStructuralCoverageAudit(cov, full)

	if audit.CoverageScore != 91 {
		t.Fatalf("CoverageScore = %.1f, want 91.0", audit.CoverageScore)
	}
	if !strings.Contains(audit.AuditSummary, "结构化") {
		t.Fatalf("AuditSummary should mention structural audit, got %q", audit.AuditSummary)
	}
}

func assertHasRequirement(t *testing.T, regs []RequirementRegisterEntry, summaryPart, typ string) {
	t.Helper()
	for _, reg := range regs {
		if reg.RequirementType == typ && strings.Contains(reg.Summary, summaryPart) {
			return
		}
	}
	t.Fatalf("missing %s requirement containing %q in %#v", typ, summaryPart, regs)
}

func findFact(items []FactItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
