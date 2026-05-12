package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestRunTechRiskReviewReturnsSuccessForPassedProjectWithoutRisks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newRiskReviewTestDB(t)
	h := NewRiskReviewHandler(db)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, final_decision, step5_status) VALUES (?, ?, ?)`, "p1", "PASS", "success")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, node_level, chapter_name, chapter_order, generation_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"sub1", "p1", "subsection", "一、安全文明施工", 1, "completed")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"content1", "p1", "sub1", 1, "### 一、响应目标\n### 三、实施措施\n### 四、质量与验收闭环", "final")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	h.RunTechRiskReview(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() == "null" {
		t.Fatalf("risk review response must not be null")
	}
}

func TestRunTechRiskReviewTreatsCompleteContentAsStep5Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newRiskReviewTestDB(t)
	h := NewRiskReviewHandler(db)
	_, err := db.Exec(`INSERT INTO tech_bid_projects (id, final_decision, step5_status) VALUES (?, ?, ?)`, "p1", "PASS", "idle")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, node_level, chapter_name, chapter_order, generation_status) VALUES (?, ?, ?, ?, ?, ?)`,
		"sub1", "p1", "subsection", "一、安全文明施工", 1, "completed")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"content1", "p1", "sub1", 1, "### 一、响应目标\n### 三、实施措施\n### 四、质量与验收闭环", "final")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	h.RunTechRiskReview(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "step5_status") {
		t.Fatalf("complete Step5 content should not produce step5_status risk, body=%s", w.Body.String())
	}
	var status string
	if err := db.Get(&status, `SELECT step5_status FROM tech_bid_projects WHERE id = ?`, "p1"); err != nil {
		t.Fatal(err)
	}
	if status != "success" {
		t.Fatalf("step5_status = %q, want success", status)
	}
}

func TestListTechRiskRecordsReturnsEmptyArrayWhenNoRisks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newRiskReviewTestDB(t)
	h := NewRiskReviewHandler(db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "p1"}}
	h.ListTechRiskRecordsByProject(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "[]" {
		t.Fatalf("body = %s, want []", w.Body.String())
	}
}

func newRiskReviewTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE tech_bid_projects (
			id TEXT PRIMARY KEY,
			final_decision TEXT,
			step5_status TEXT,
			step6_status TEXT,
			last_error_message TEXT,
			updated_at DATETIME
		)`,
		`CREATE TABLE tech_bid_chapter_plans (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			node_level TEXT,
			chapter_name TEXT,
			chapter_order INTEGER,
			generation_status TEXT
		)`,
		`CREATE TABLE tech_bid_chapter_contents (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			chapter_id TEXT,
			version_no INTEGER,
			content_md TEXT,
			status TEXT,
			updated_at DATETIME
		)`,
		`CREATE TABLE tech_bid_risk_records (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			chapter_id TEXT,
			risk_type TEXT,
			risk_level TEXT,
			risk_source TEXT,
			risk_detail TEXT,
			similarity_score REAL,
			check_result_json TEXT,
			status TEXT DEFAULT 'open',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
