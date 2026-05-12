package service

import (
	"testing"
)

func TestDefaultFullResponseGateConfig(t *testing.T) {
	t.Parallel()
	c := DefaultFullResponseGateConfig()
	if c.BlockFullRateMax != 75 || c.PassFullRateMin != 90 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestMergeGateConfig_partial(t *testing.T) {
	t.Parallel()
	base := DefaultFullResponseGateConfig()
	ov := FullResponseGateConfig{
		BlockFullRateMax: 70,
		PassFullRateMin:  0,
	}
	out := mergeGateConfig(base, ov)
	if out.BlockFullRateMax != 70 {
		t.Fatalf("BlockFullRateMax: got %v", out.BlockFullRateMax)
	}
	if out.PassFullRateMin != 90 {
		t.Fatalf("PassFullRateMin should stay when ov is 0: got %v", out.PassFullRateMin)
	}
}

func TestLoadFullResponseGateConfig_emptyDB(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	c := LoadFullResponseGateConfig(db, "", "")
	if c.BlockFullRateMax != 75 {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadFullResponseGateConfig_jsonDefault(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_full_response_gate_config",
		`{"default":{"block_full_rate_max":60,"pass_full_rate_min":95}}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadFullResponseGateConfig(db, "", "")
	if c.BlockFullRateMax != 60 || c.PassFullRateMin != 95 {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadFullResponseGateConfig_byProfession(t *testing.T) {
	t.Parallel()
	db := newTestSQLiteDB(t)
	_, err := db.Exec(`INSERT INTO system_settings (key, value) VALUES (?, ?)`,
		"tech_bid_full_response_gate_config",
		`{"default":{"block_full_rate_max":75},"by_profession":{"水利":{"block_full_rate_max":72}}}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadFullResponseGateConfig(db, "水利水电施工", "")
	if c.BlockFullRateMax != 72 {
		t.Fatalf("expected profession overlay, got %+v", c)
	}
}

func TestApplyProfessionHints_water(t *testing.T) {
	t.Parallel()
	base := DefaultFullResponseGateConfig()
	c := applyProfessionHints(base, "水利枢纽", "")
	if c.BlockFullRateMax != 72 {
		t.Fatalf("expected 72 from builtin 水利 hint, got %v", c.BlockFullRateMax)
	}
}
