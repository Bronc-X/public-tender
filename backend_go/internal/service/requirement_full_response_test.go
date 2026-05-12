package service

import (
	"testing"
)

func TestApplyFullResponseHardGate_pass(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullResponseGateConfig()
	res := &FullRequirementResponseResult{
		FullResponseRate:         95,
		WeakResponseRate:         0,
		OnlyTaggedRatio:          0,
		ResponseQualityScore:     90,
		MissingRequirementIDs:    nil,
		WeakRequirementIDs:       nil,
		OnlyTaggedRequirementIDs: nil,
		ShellTitleHints:          nil,
	}
	reg := []RequirementRegisterEntry{
		{RequirementID: "R1", RequirementType: "score", Priority: "medium", ResponseTier: "mergeable", Summary: "测试"},
	}
	applyFullResponseHardGate(res, reg, &cfg)
	if res.Result != "PASS" {
		t.Fatalf("expected PASS, got %s: %s", res.Result, res.Summary)
	}
}

func TestApplyFullResponseHardGate_block_highPriorityMissing(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullResponseGateConfig()
	res := &FullRequirementResponseResult{
		FullResponseRate:       100,
		MissingRequirementIDs:  []string{"H1"},
		HighPriorityMissingIDs: []string{},
	}
	reg := []RequirementRegisterEntry{
		{RequirementID: "H1", RequirementType: "score", Priority: "high", ResponseTier: "mergeable", Summary: "高优先级条"},
	}
	applyFullResponseHardGate(res, reg, &cfg)
	if res.Result != "BLOCK" {
		t.Fatalf("expected BLOCK, got %s", res.Result)
	}
}

func TestApplyFullResponseHardGate_revise_missing(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullResponseGateConfig()
	res := &FullRequirementResponseResult{
		FullResponseRate:      100,
		MissingRequirementIDs: []string{"R9"},
	}
	reg := []RequirementRegisterEntry{
		{RequirementID: "R9", RequirementType: "score", Priority: "medium", ResponseTier: "mergeable", Summary: "普通"},
	}
	applyFullResponseHardGate(res, reg, &cfg)
	if res.Result != "REVISE" {
		t.Fatalf("expected REVISE, got %s: %s", res.Result, res.Summary)
	}
}

func TestScoreRequirementResponseQuality(t *testing.T) {
	t.Parallel()
	cfg := DefaultFullResponseGateConfig()
	q := ScoreRequirementResponseQuality(90, 10, 2, &cfg)
	if q < 80 || q > 100 {
		t.Fatalf("unexpected quality score: %v", q)
	}
}

func TestDedupeStrings(t *testing.T) {
	t.Parallel()
	out := dedupeStrings([]string{"a", "a", " b ", ""})
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("got %#v", out)
	}
}

func TestValidateFullRequirementResponseBlocksWhenOnlyMetaRulesAreMapped(t *testing.T) {
	t.Parallel()
	svc := &TenderDigitizationService{}
	outline := []map[string]interface{}{
		{
			"name": "第一章 项目理解与招标要求响应总览",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 目录追溯机制",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、目录节点与招标条款追溯表",
							"requirement_ids": []string{"ms_01"},
						},
					},
				},
			},
		},
	}
	register := []RequirementRegisterEntry{
		{
			RequirementID:         "ms_01",
			RequirementType:       "meta_rule",
			Priority:              "high",
			ResponseTier:          "must_standalone",
			Summary:               "目录节点追溯要求",
			ExpectedResponseLevel: "subsection",
		},
	}

	res := svc.ValidateFullRequirementResponse(outline, register, "", "")

	if res.Result != "BLOCK" {
		t.Fatalf("meta-only register must not pass Step4, got %s: %+v", res.Result, res)
	}
	if res.RequirementTotal != 0 {
		t.Fatalf("meta rules should not count as pass-bearing business requirements, got total %d", res.RequirementTotal)
	}
}
