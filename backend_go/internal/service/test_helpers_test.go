package service

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func newTestSQLiteDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE system_settings (key TEXT PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE tech_bid_projects (
		id TEXT PRIMARY KEY,
		company_id TEXT NOT NULL,
		project_name TEXT,
		outline_titles_json TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
