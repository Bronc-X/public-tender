package db

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func TestEnsureTechBidOutlineVerificationsSchemaCreatesSQLiteTable(t *testing.T) {
	database, err := sqlx.Connect("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	EnsureTechBidOutlineVerificationsSchema(database)

	var tableName string
	if err := database.Get(&tableName, `SELECT name FROM sqlite_master WHERE type='table' AND name='tech_bid_outline_verifications'`); err != nil {
		t.Fatalf("verification table was not created: %v", err)
	}

	_, err = database.Exec(`INSERT INTO tech_bid_outline_verifications
		(id, project_id, audit_id, final_decision, risk_level, summary, critical_issues_json, major_issues_json, suggested_actions_json, can_proceed, verification_method, verification_model)
		VALUES ('v1', 'p1', NULL, 'PASS', 'LOW', 'ok', '[]', '[]', '[]', 1, 'ai', 'test-model')`)
	if err != nil {
		t.Fatalf("verification table insert failed: %v", err)
	}
}
