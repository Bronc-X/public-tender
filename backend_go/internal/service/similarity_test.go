package service

import (
	"testing"
)

func TestJaccardStringSet(t *testing.T) {
	t.Parallel()
	if j := JaccardStringSet([]string{"a", "b"}, []string{"a", "b"}); j != 1 {
		t.Fatalf("identical sets: expected 1, got %v", j)
	}
	if j := JaccardStringSet([]string{"a"}, []string{"b"}); j != 0 {
		t.Fatalf("disjoint: expected 0, got %v", j)
	}
	// {a,b} ∩ {b,c} = {b}, union = {a,b,c} → 1/3
	if j := JaccardStringSet([]string{"a", "b"}, []string{"b", "c"}); j < 0.32 || j > 0.35 {
		t.Fatalf("expected ~0.333, got %v", j)
	}
}

func TestNormalizeTitleForSimilarity_dedup(t *testing.T) {
	t.Parallel()
	a := normalizeTitleForSimilarity("（一） 施工 方案")
	b := normalizeTitleForSimilarity("(一)施工方案")
	if a != b {
		t.Fatalf("expected equal normalized keys, got %q vs %q", a, b)
	}
}

func TestLoadJaccardSimilarityWarnThreshold_defaultAndConfig(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	if g, w := LoadJaccardSimilarityWarnThreshold(db), DefaultJaccardSimilarityWarnThreshold; g != w {
		t.Fatalf("default: got %v want %v", g, w)
	}
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_outline_similarity_config", `{"jaccard_warn_threshold":0.7}`)
	if err != nil {
		t.Fatal(err)
	}
	if v := LoadJaccardSimilarityWarnThreshold(db); v != 0.7 {
		t.Fatalf("got %v", v)
	}
	_, _ = db.Exec(`INSERT OR REPLACE INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_outline_similarity_config", `{"jaccard_warn_threshold":0}`)
	if LoadJaccardSimilarityWarnThreshold(db) != DefaultJaccardSimilarityWarnThreshold {
		t.Fatal("invalid 0 should fall back to default")
	}
}

func TestBestHistoryJaccardHint(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, outline_titles_json) VALUES (?,?,?,?)`,
		"p1", "c1", "历史标书A", `["土石方开挖","混凝土浇筑","金结安装"]`)
	if err != nil {
		t.Fatal(err)
	}
	hint := BestHistoryJaccardHint(db, "c1", "p2", []string{"土石方开挖", "混凝土浇筑", "金结安装"})
	if hint == "" {
		t.Fatal("expected non-empty hint for identical title sets")
	}
}
