package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend_go/internal/agent"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func TestRecordStep6OutputCreatesBidProjectOutput(t *testing.T) {
	db := newStep6HandlerTestDB(t)
	h := &BidProjectHandler{db: db}

	exportPath := filepath.Join(t.TempDir(), "export_project-1.docx")
	if err := os.WriteFile(exportPath, []byte("docx"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := h.recordStep6Output("project-1", exportPath); err != nil {
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
	if err := db.Get(&row, "SELECT project_id, output_type, file_name, file_path, mime_type, status FROM bid_project_outputs WHERE project_id = ?", "project-1"); err != nil {
		t.Fatal(err)
	}
	if row.OutputType != "commerce_word" {
		t.Fatalf("output_type = %q, want commerce_word", row.OutputType)
	}
	if row.FileName != "export_project-1.docx" {
		t.Fatalf("file_name = %q", row.FileName)
	}
	if row.FilePath != exportPath {
		t.Fatalf("file_path = %q, want %q", row.FilePath, exportPath)
	}
	if row.MimeType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("mime_type = %q", row.MimeType)
	}
	if row.Status != "available" {
		t.Fatalf("status = %q", row.Status)
	}
}

func TestResolveStep6DownloadPathPrefersProjectExportDir(t *testing.T) {
	exportRoot := t.TempDir()
	t.Setenv("BID_EXPORT_DIR", exportRoot)

	projectID := "project-1"
	fileName := "export_project-1.docx"
	expectedPath := filepath.Join(exportRoot, projectID, fileName)
	if err := os.MkdirAll(filepath.Dir(expectedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expectedPath, []byte("docx"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveStep6DownloadPath(projectID, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedPath {
		t.Fatalf("resolveStep6DownloadPath() = %q, want %q", got, expectedPath)
	}
}

func TestLatestStep6OutputPathPrefersNewestAvailableCommerceWord(t *testing.T) {
	db := newStep6HandlerTestDB(t)
	h := &BidProjectHandler{db: db}

	exportRoot := t.TempDir()
	oldPath := filepath.Join(exportRoot, "old.docx")
	newPath := filepath.Join(exportRoot, "new.docx")
	if err := os.WriteFile(newPath, []byte("docx"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := db.Exec(`INSERT INTO bid_project_outputs
		(id, project_id, version_no, output_type, file_name, file_path, mime_type, status, created_at)
		VALUES
		('old', 'project-1', 1, 'commerce_word', 'old.docx', ?, 'docx', 'available', '2026-04-28 01:00:00'),
		('new', 'project-1', 2, 'commerce_word', 'new.docx', ?, 'docx', 'available', '2026-04-28 02:00:00'),
		('draft', 'project-1', 3, 'commerce_word', 'draft.docx', '/tmp/draft.docx', 'docx', 'deleted', '2026-04-28 03:00:00'),
		('other', 'project-1', 1, 'other', 'other.docx', '/tmp/other.docx', 'docx', 'available', '2026-04-28 04:00:00')`, oldPath, newPath)
	if err != nil {
		t.Fatal(err)
	}

	got := h.latestStep6OutputPath("project-1")
	if got != newPath {
		t.Fatalf("latestStep6OutputPath() = %q, want %q", got, newPath)
	}
}

func TestLatestStep6OutputPathSkipsMissingCommerceWordFile(t *testing.T) {
	db := newStep6HandlerTestDB(t)
	h := &BidProjectHandler{db: db}

	exportRoot := t.TempDir()
	oldPath := filepath.Join(exportRoot, "old.docx")
	missingPath := filepath.Join(exportRoot, "missing.docx")
	if err := os.WriteFile(oldPath, []byte("docx"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := db.Exec(`INSERT INTO bid_project_outputs
		(id, project_id, version_no, output_type, file_name, file_path, mime_type, status, created_at)
		VALUES
		('old', 'project-1', 1, 'commerce_word', 'old.docx', ?, 'docx', 'available', '2026-04-28 01:00:00'),
		('new-missing', 'project-1', 2, 'commerce_word', 'missing.docx', ?, 'docx', 'available', '2026-04-28 02:00:00')`, oldPath, missingPath)
	if err != nil {
		t.Fatal(err)
	}

	got := h.latestStep6OutputPath("project-1")
	if got != oldPath {
		t.Fatalf("latestStep6OutputPath() = %q, want existing older file %q", got, oldPath)
	}
}

func TestGetStep6PayloadFallsBackToRuleMarkdownBeforeTemplateUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newStep6HandlerTestDB(t)
	h := &BidProjectHandler{db: db}

	rulePath := filepath.Join(t.TempDir(), "rule.md")
	ruleMarkdown := "# 商务标\n\n## 一、投标函\n请按招标文件要求填写投标函。"
	if err := os.WriteFile(rulePath, []byte(ruleMarkdown), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO bid_projects (id, active_version_no, current_step, current_step_status, step6_status, rule_markdown_path) VALUES (?, ?, ?, ?, ?, ?)`,
		"project-rule-md", 1, "user_confirmation", "waiting", "idle", rulePath)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "project-rule-md"}}
	h.GetStep6Payload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			OriginalMarkdown string `json:"original_markdown"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response should be valid JSON: %v\n%s", err, w.Body.String())
	}
	if !strings.Contains(body.Data.OriginalMarkdown, "一、投标函") {
		t.Fatalf("original_markdown = %q, want rule markdown content", body.Data.OriginalMarkdown)
	}
}

func TestRecordStep6OutputClearsStaleCommerceStep6Error(t *testing.T) {
	db := newStep6HandlerTestDB(t)
	h := &BidProjectHandler{db: db}
	_, err := db.Exec(`INSERT INTO bid_projects (id, current_step, current_step_status, step6_status, last_error_message, active_version_no) VALUES (?, ?, ?, ?, ?, ?)`,
		"project-1", "output_finalize", "failed", "error", "failed to fetch step5 bindings: SQL logic error: no such column: step5_bindings_json (1)", 1)
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(t.TempDir(), "export_project-1.docx")
	if err := os.WriteFile(exportPath, []byte("docx"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := h.recordStep6Output("project-1", exportPath); err != nil {
		t.Fatal(err)
	}

	var row struct {
		CurrentStepStatus string  `db:"current_step_status"`
		Step6Status       string  `db:"step6_status"`
		LastErrorMessage  *string `db:"last_error_message"`
	}
	if err := db.Get(&row, `SELECT current_step_status, step6_status, last_error_message FROM bid_projects WHERE id = ?`, "project-1"); err != nil {
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

func TestResolveRegeneratedSlotUsesFreshAIResult(t *testing.T) {
	existing := agent.BidActionSlot{
		SlotID:           "slot-1",
		AISuggestedValue: "旧值",
		Reason:           "旧原因",
		Status:           agent.StatusApproved,
	}
	result := agent.BidActionList{Slots: []agent.BidActionSlot{{
		AISuggestedValue: "新值",
		Reason:           "新原因",
		Status:           agent.StatusPending,
	}}}

	got, ok := resolveRegeneratedSlot(existing, result, nil)
	if !ok {
		t.Fatal("resolveRegeneratedSlot() rejected a valid AI result")
	}
	if got.SlotID != existing.SlotID {
		t.Fatalf("SlotID = %q, want %q", got.SlotID, existing.SlotID)
	}
	if got.AISuggestedValue != "新值" {
		t.Fatalf("AISuggestedValue = %q, want 新值", got.AISuggestedValue)
	}
	if got.Reason != "新原因" {
		t.Fatalf("Reason = %q, want 新原因", got.Reason)
	}
	if got.Status != agent.StatusPending {
		t.Fatalf("Status = %q, want %q", got.Status, agent.StatusPending)
	}
}

func TestResolveRegeneratedSlotFallsBackToExistingValue(t *testing.T) {
	existing := agent.BidActionSlot{
		SlotID:           "slot-1",
		AISuggestedValue: "已确认装配内容",
		Reason:           "来自 Step5 绑定",
		Status:           agent.StatusApproved,
	}

	got, ok := resolveRegeneratedSlot(existing, agent.BidActionList{}, errors.New("rate limited"))
	if !ok {
		t.Fatal("resolveRegeneratedSlot() rejected a usable fallback value")
	}
	if got.AISuggestedValue != existing.AISuggestedValue {
		t.Fatalf("AISuggestedValue = %q, want %q", got.AISuggestedValue, existing.AISuggestedValue)
	}
	if got.Status != agent.StatusApproved {
		t.Fatalf("Status = %q, want approved", got.Status)
	}
	if !strings.Contains(got.Reason, "AI 重新生成当前不可用") {
		t.Fatalf("Reason = %q, want fallback note", got.Reason)
	}
}

func TestResolveRegeneratedSlotRejectsEmptyFallback(t *testing.T) {
	if _, ok := resolveRegeneratedSlot(agent.BidActionSlot{}, agent.BidActionList{}, errors.New("rate limited")); ok {
		t.Fatal("resolveRegeneratedSlot() accepted an empty fallback")
	}
}

func newStep6HandlerTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE bid_project_outputs (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		version_no INTEGER,
		output_type TEXT,
		file_name TEXT,
		file_path TEXT,
		mime_type TEXT,
		status TEXT DEFAULT 'available',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE bid_projects (
		id TEXT PRIMARY KEY,
		active_version_no INTEGER DEFAULT 1,
		current_step TEXT,
		current_step_status TEXT,
		step6_status TEXT,
		step6_payload_json TEXT,
		last_error_message TEXT,
		rule_markdown_path TEXT,
		template_markdown_path TEXT,
		updated_at DATETIME
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
