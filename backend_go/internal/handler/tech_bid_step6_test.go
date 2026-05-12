package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestGenerateTechStep6PayloadRejectsProjectWhenStep4NotPassed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, final_decision, can_enter_content_generation, step6_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"tech-blocked", "company-1", "Blocked Tech Bid", "BLOCK", 0, "idle")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "tech-blocked"}}
	h.GenerateTechStep6Payload(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "STEP4_NOT_PASSED") {
		t.Fatalf("body should explain Step4 gate failure, got %s", w.Body.String())
	}
}

func TestGenerateTechStep6PayloadCreatesPayloadFromPassedChapterPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, final_decision, can_enter_content_generation, step6_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"tech-pass", "company-1", "Passed Tech Bid", "PASS", 1, "idle")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, requirement_ids_json, outline_version) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		"chapter-1", "tech-pass", "施工方案和技术措施", 1, "chapter", "completed", `["req_quality"]`, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"content-1", "tech-pass", "chapter-1", 1, "## 施工方案和技术措施\n落实质量验收标准。", "final")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "tech-pass"}}
	h.GenerateTechStep6Payload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var project struct {
		Step6Status      sql.NullString `db:"step6_status"`
		Step6PayloadJSON sql.NullString `db:"step6_payload_json"`
	}
	if err := db.Get(&project, `SELECT step6_status, step6_payload_json FROM tech_bid_projects WHERE id = ?`, "tech-pass"); err != nil {
		t.Fatal(err)
	}
	if project.Step6Status.String != "success" {
		t.Fatalf("step6_status = %q, want success", project.Step6Status.String)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(project.Step6PayloadJSON.String), &payload); err != nil {
		t.Fatalf("payload should be valid JSON: %v\n%s", err, project.Step6PayloadJSON.String)
	}
	if payload["project_id"] != "tech-pass" {
		t.Fatalf("project_id = %#v, want tech-pass", payload["project_id"])
	}
	slots, ok := payload["slots"].([]interface{})
	if !ok || len(slots) != 1 {
		t.Fatalf("slots = %#v, want one slot", payload["slots"])
	}
}

func TestGenerateTechStep6PayloadFallsBackToApprovedOutlineWhenContentMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, final_decision, can_enter_content_generation, step6_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"tech-outline-only", "company-1", "Outline Only Tech Bid", "PASS", 1, "idle")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_requirement_register (id, project_id, requirement_id, requirement_type, source_text, source_location, priority, must_be_explicit, expected_response_level, domain, response_tier, summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"reg-1", "tech-outline-only", "req_hse", "mandatory_clause", "按 HSE 有关规定达到安全文明施工标准。", "第四章 技术和商务要求", "high", 1, "subsection", "technical_bid", "must_standalone", "安全文明施工与 HSE 标准")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, requirement_ids_json, outline_version) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		"sub-1", "tech-outline-only", "一、安全文明施工与 HSE 标准专项落实措施", 1, "subsection", "not_started", `["req_hse"]`, 1)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "tech-outline-only"}}
	h.GenerateTechStep6Payload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var project struct {
		Step6Status      sql.NullString `db:"step6_status"`
		Step6PayloadJSON sql.NullString `db:"step6_payload_json"`
	}
	if err := db.Get(&project, `SELECT step6_status, step6_payload_json FROM tech_bid_projects WHERE id = ?`, "tech-outline-only"); err != nil {
		t.Fatal(err)
	}
	if project.Step6Status.String != "success" {
		t.Fatalf("step6_status = %q, want success", project.Step6Status.String)
	}
	if !strings.Contains(project.Step6PayloadJSON.String, "安全文明施工与 HSE 标准") {
		t.Fatalf("fallback payload should include requirement summary, got %s", project.Step6PayloadJSON.String)
	}
}

func TestExportTechFinalWordGeneratesMissingPayloadFromCompletedChapters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	commerceRoot := t.TempDir()
	techRoot := t.TempDir()
	t.Setenv("BID_EXPORT_DIR", commerceRoot)
	t.Setenv("TECH_BID_EXPORT_DIR", techRoot)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, final_decision, can_enter_content_generation, step6_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"tech-direct-export", "company-1", "Direct Export Tech Bid", "PASS", 1, "idle")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, requirement_ids_json, outline_version) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		"chapter-1", "tech-direct-export", "施工方案和技术措施", 1, "subsection", "completed", `[]`, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"content-1", "tech-direct-export", "chapter-1", 1, "## 施工方案和技术措施\n落实质量验收标准。", "final")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "tech-direct-export"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/tech-bid/projects/tech-direct-export/step6/export", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ExportTechFinalWord(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var project struct {
		Step6Status      sql.NullString `db:"step6_status"`
		Step6PayloadJSON sql.NullString `db:"step6_payload_json"`
	}
	if err := db.Get(&project, `SELECT step6_status, step6_payload_json FROM tech_bid_projects WHERE id = ?`, "tech-direct-export"); err != nil {
		t.Fatal(err)
	}
	if project.Step6Status.String != "success" || !strings.Contains(project.Step6PayloadJSON.String, "施工方案和技术措施") {
		t.Fatalf("export should persist generated payload, status=%q payload=%s", project.Step6Status.String, project.Step6PayloadJSON.String)
	}
	var exportPath string
	if err := db.Get(&exportPath, `SELECT file_path FROM tech_bid_outputs WHERE project_id = ? AND output_type = 'technical_word'`, "tech-direct-export"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(exportPath, techRoot) {
		t.Fatalf("technical export path = %q, want under TECH_BID_EXPORT_DIR %q", exportPath, techRoot)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("expected exported docx at %s: %v", exportPath, err)
	}
}

func TestRecordTechStep6OutputCreatesTechBidOutput(t *testing.T) {
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	if err := h.recordTechStep6Output("tech-pass", "/tmp/final-tech.docx"); err != nil {
		t.Fatal(err)
	}
	var row struct {
		ProjectID  string `db:"project_id"`
		OutputType string `db:"output_type"`
		FileName   string `db:"file_name"`
		FilePath   string `db:"file_path"`
		MimeType   string `db:"mime_type"`
		Status     string `db:"status"`
	}
	if err := db.Get(&row, `SELECT project_id, output_type, file_name, file_path, mime_type, status FROM tech_bid_outputs WHERE project_id = ?`, "tech-pass"); err != nil {
		t.Fatal(err)
	}
	if row.OutputType != "technical_word" || row.FileName != "final-tech.docx" || row.Status != "available" {
		t.Fatalf("unexpected output row: %+v", row)
	}
}

func TestRecordTechStep6OutputClearsStaleStep6Error(t *testing.T) {
	db := newTechStep6HandlerTestDB(t)
	h := NewTechBidProjectHandler(db, nil)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, company_id, project_name, current_step, current_step_status, final_decision, can_enter_content_generation, step6_status, last_error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"tech-pass", "company-1", "Passed Tech Bid", "output_finalize", "failed", "PASS", 1, "error", "no completed technical chapter content found")
	if err != nil {
		t.Fatal(err)
	}

	if err := h.recordTechStep6Output("tech-pass", "/tmp/final-tech.docx"); err != nil {
		t.Fatal(err)
	}

	var row struct {
		CurrentStepStatus string  `db:"current_step_status"`
		Step6Status       string  `db:"step6_status"`
		LastErrorMessage  *string `db:"last_error_message"`
	}
	if err := db.Get(&row, `SELECT current_step_status, step6_status, last_error_message FROM tech_bid_projects WHERE id = ?`, "tech-pass"); err != nil {
		t.Fatal(err)
	}
	if row.CurrentStepStatus != "success" {
		t.Fatalf("current_step_status = %q, want success", row.CurrentStepStatus)
	}
	if row.Step6Status != "success" {
		t.Fatalf("step6_status = %q, want success", row.Step6Status)
	}
	if row.LastErrorMessage != nil && *row.LastErrorMessage != "" {
		t.Fatalf("last_error_message = %q, want cleared", *row.LastErrorMessage)
	}
}

func newTechStep6HandlerTestDB(t *testing.T) *sqlx.DB {
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
			current_step TEXT,
			current_step_status TEXT,
			final_decision TEXT,
			can_enter_content_generation INTEGER DEFAULT 0,
			step4_override_enabled INTEGER DEFAULT 0,
			override_enabled INTEGER DEFAULT 0,
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
			outline_version INTEGER
		)`,
		`CREATE TABLE tech_bid_chapter_contents (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			chapter_id TEXT,
			version_no INTEGER,
			content_md TEXT,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE tech_bid_outputs (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			version_no INTEGER,
			output_type TEXT,
			file_name TEXT,
			file_path TEXT,
			mime_type TEXT,
			status TEXT,
			created_at DATETIME
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
