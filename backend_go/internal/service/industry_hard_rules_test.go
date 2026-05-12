package service

import (
	"testing"
)

func TestMatchProfessionRule_global(t *testing.T) {
	t.Parallel()
	r := IndustryHardRule{ID: "x", KeywordsAny: []string{"k"}}
	if !matchProfessionRule(r, "", "") {
		t.Fatal("empty filters should match")
	}
}

func TestMatchProfessionRule_substring(t *testing.T) {
	t.Parallel()
	r := IndustryHardRule{
		ID:                   "w",
		ProfessionSubstrings: []string{"水利"},
		KeywordsAny:          []string{"x"},
	}
	if !matchProfessionRule(r, "某水利枢纽", "") {
		t.Fatal("should match 水利")
	}
	if matchProfessionRule(r, "房建", "") {
		t.Fatal("should not match")
	}
}

func TestApplyIndustryHardRulesToResult_block(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_industry_hard_rules",
		`{"rules":[{"id":"must_kw","keywords_any":["防汛"],"severity":"block","message":"须含防汛"}]}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res := &FullRequirementResponseResult{Result: "PASS", Summary: "ok"}
	outline := []map[string]interface{}{
		{
			"units": []interface{}{
				map[string]interface{}{
					"subsections": []interface{}{
						map[string]interface{}{"name": "施工组织"},
					},
				},
			},
		},
	}
	applyIndustryHardRulesToResult(db, res, outline, "", "")
	if res.Result != "BLOCK" {
		t.Fatalf("expected BLOCK, got %s / %v", res.Result, res.HardRuleWarnings)
	}
}

func TestApplyIndustryHardRulesToResult_warnOnly(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_industry_hard_rules",
		`{"rules":[{"id":"w","keywords_any":["BIM"],"severity":"warn","message":"建议写BIM"}]}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res := &FullRequirementResponseResult{Result: "PASS", Summary: "ok"}
	outline := []map[string]interface{}{
		{
			"units": []interface{}{
				map[string]interface{}{
					"subsections": []interface{}{
						map[string]interface{}{"name": "进度计划"},
					},
				},
			},
		},
	}
	applyIndustryHardRulesToResult(db, res, outline, "", "")
	if res.Result != "PASS" {
		t.Fatalf("warn should keep PASS, got %s", res.Result)
	}
	if len(res.HardRuleWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", res.HardRuleWarnings)
	}
}

func TestApplyIndustryHardRulesToResult_reviseFromPass(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_industry_hard_rules",
		`{"rules":[{"id":"r","keywords_any":["装配式"],"severity":"revise","message":"须装配式"}]}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	res := &FullRequirementResponseResult{Result: "PASS", Summary: "ok"}
	outline := []map[string]interface{}{
		{
			"units": []interface{}{
				map[string]interface{}{
					"subsections": []interface{}{
						map[string]interface{}{"name": "总则"},
					},
				},
			},
		},
	}
	applyIndustryHardRulesToResult(db, res, outline, "", "")
	if res.Result != "REVISE" {
		t.Fatalf("expected REVISE, got %s", res.Result)
	}
}
