package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGenerateTechStep5ContentCreatesContentForPassedSubsections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTechStep5HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, final_decision, can_enter_content_generation, step5_status, step6_status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"tech-step5", "company-1", "Step5 Tech Bid", "PASS", 1, "idle", "idle")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_requirement_register (id, project_id, requirement_id, requirement_type, source_text, source_location, priority, must_be_explicit, expected_response_level, domain, response_tier, summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"reg-1", "tech-step5", "req_hse", "mandatory_clause", "按 HSE 有关规定达到安全文明施工标准。", "第四章 技术和商务要求", "high", 1, "subsection", "technical_bid", "must_standalone", "安全文明施工与 HSE 标准")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, requirement_ids_json, outline_version) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		"sub-1", "tech-step5", "一、安全文明施工与 HSE 标准专项落实措施", 1, "subsection", "not_started", `["req_hse"]`, 1)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "tech-step5"}}
	h.GenerateTechStep5Content(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var project struct {
		Step5Status sql.NullString `db:"step5_status"`
	}
	if err := db.Get(&project, `SELECT step5_status FROM tech_bid_projects WHERE id = ?`, "tech-step5"); err != nil {
		t.Fatal(err)
	}
	if project.Step5Status.String != "success" {
		t.Fatalf("step5_status = %q, want success", project.Step5Status.String)
	}
	var content string
	if err := db.Get(&content, `SELECT content_md FROM tech_bid_chapter_contents WHERE chapter_id = ?`, "sub-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "安全文明施工与 HSE 标准") || !strings.Contains(content, "实施措施") {
		t.Fatalf("content should include requirement summary and implementation body, got %s", content)
	}
	var status string
	if err := db.Get(&status, `SELECT generation_status FROM tech_bid_chapter_plans WHERE id = ?`, "sub-1"); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("generation_status = %q, want completed", status)
	}
}

func newTechStep5HandlerTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE tech_bid_projects (
			id TEXT PRIMARY KEY,
			company_id TEXT,
			project_name TEXT,
			final_decision TEXT,
			can_enter_content_generation INTEGER DEFAULT 0,
			step4_override_enabled INTEGER DEFAULT 0,
			override_enabled INTEGER DEFAULT 0,
			step5_status TEXT,
			step6_status TEXT,
			step6_payload_json TEXT,
			last_error_message TEXT,
			updated_at DATETIME,
			active_version_no INTEGER DEFAULT 1
		)`,
		`CREATE TABLE tech_bid_chapter_plans (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			parent_id TEXT,
			chapter_name TEXT,
			chapter_order INTEGER,
			node_level TEXT,
			generation_status TEXT,
			requirement_ids_json TEXT,
			outline_version INTEGER,
			updated_at DATETIME
		)`,
		`CREATE TABLE tech_bid_chapter_contents (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			chapter_id TEXT,
			version_no INTEGER,
			content_md TEXT,
			content_html TEXT,
			content_json TEXT,
			source_refs_json TEXT,
			generation_model TEXT,
			generation_prompt_json TEXT,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE tech_bid_requirement_register (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			requirement_id TEXT,
			requirement_type TEXT,
			source_text TEXT,
			source_location TEXT,
			priority TEXT,
			must_be_explicit INTEGER,
			expected_response_level TEXT,
			domain TEXT,
			response_tier TEXT,
			summary TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
