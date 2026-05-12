package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type HistoryProjectRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Origin      string  `json:"origin"`
	ProjectType *string `json:"project_type,omitempty"`
	TenderCode  *string `json:"tender_code,omitempty"`
	FileCount   int     `json:"file_count"`
	CreatedAt   *string `json:"created_at,omitempty"`
}

type ProjectFileRow struct {
	RowID            string  `json:"id"`
	FileAssetID      string  `json:"file_asset_id"`
	FileName         string  `json:"file_name"`
	FileRole         string  `json:"file_role,omitempty"`
	ParseReady       bool    `json:"parse_ready"`
	MarkdownReady    bool    `json:"markdown_ready"`
	ContentUpdatedAt *string `json:"content_updated_at,omitempty"`
	FileSize         *int64  `json:"file_size,omitempty"`
	Recommended      bool    `json:"recommended"`
}

type LocalFileInput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// CreateExtractTaskRequest carries user-selected markdown (already scoped client-side).
type CreateExtractTaskRequest struct {
	KnowledgeType        string `json:"knowledge_type" binding:"required"`
	SourceOrigin         string `json:"source_origin" binding:"required"`
	SourceProjectID      string `json:"source_project_id" binding:"required"`
	SourceProjectName    string `json:"source_project_name"`
	SourceFileID         string `json:"source_file_id" binding:"required"`
	ExtractScope         string `json:"extract_scope" binding:"required"`
	SelectedSectionsJSON string `json:"selected_sections_json"`
	MarkdownContent      string `json:"markdown_content" binding:"required"`
	PromptTemplateID     *int64 `json:"prompt_template_id"`
	PromptKey            string `json:"prompt_key"`
	PromptOverride       string `json:"prompt_override"`
}

type CommitExtractItem struct {
	ResultID    string `json:"result_id" binding:"required"`
	Include     bool   `json:"include"`
	ContentJSON string `json:"content_json"`
	Title       string `json:"title"`
}

type KnowledgeExtractService struct {
	db            *sqlx.DB
	ai            *AIClient
	promptService *PromptService
}

func NewKnowledgeExtractService(db *sqlx.DB, ai *AIClient, ps *PromptService) *KnowledgeExtractService {
	return &KnowledgeExtractService{db: db, ai: ai, promptService: ps}
}

func (s *KnowledgeExtractService) ListTechBidHistoryProjects(companyID, q string) ([]HistoryProjectRow, error) {
	q = strings.TrimSpace(q)
	base := `
		SELECT p.id, p.project_name, p.project_type, p.tender_code, p.created_at,
			(SELECT COUNT(*) FROM tech_bid_tender_files t WHERE t.project_id = p.id) AS file_count
		FROM tech_bid_projects p
		WHERE p.company_id = ?
	`
	args := []interface{}{companyID}
	if q != "" {
		base += ` AND (p.project_name LIKE ? OR IFNULL(p.tender_code,'') LIKE ? OR IFNULL(p.project_type,'') LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	base += ` ORDER BY p.created_at DESC LIMIT 200`

	rows, err := s.db.Query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]HistoryProjectRow, 0)
	for rows.Next() {
		var id, name string
		var pt, tc sql.NullString
		var created sql.NullString
		var fc int
		if err := rows.Scan(&id, &name, &pt, &tc, &created, &fc); err != nil {
			continue
		}
		r := HistoryProjectRow{
			ID:        id,
			Name:      name,
			Origin:    "tech_bid",
			FileCount: fc,
		}
		if pt.Valid {
			r.ProjectType = &pt.String
		}
		if tc.Valid {
			r.TenderCode = &tc.String
		}
		if created.Valid {
			r.CreatedAt = &created.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *KnowledgeExtractService) ListTechBidProjectFiles(companyID, projectID string) ([]ProjectFileRow, error) {
	var n int
	err := s.db.Get(&n, `SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?`, projectID, companyID)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("project not found")
	}

	query := `
		SELECT tbf.id, COALESCE(fa.id, tbf.file_asset_id) AS asset_id,
			COALESCE(tbf.file_name, fa.file_name, '') AS fname,
			COALESCE(tbf.file_role, '') AS frole,
			fa.file_size,
			COALESCE(c.markdown_text, ''), COALESCE(c.plain_text, ''),
			c.created_at
		FROM tech_bid_tender_files tbf
		LEFT JOIN file_asset fa ON tbf.file_asset_id = fa.id AND IFNULL(fa.company_id,'') = ?
		LEFT JOIN file_content c ON c.id = (
			SELECT fc.id FROM file_content fc WHERE fc.file_asset_id = fa.id ORDER BY fc.created_at DESC LIMIT 1
		)
		WHERE tbf.project_id = ?
		ORDER BY
			CASE
				WHEN LOWER(COALESCE(tbf.file_name, fa.file_name, '')) LIKE '%施工组织%' THEN 0
				WHEN LOWER(IFNULL(tbf.file_role,'')) LIKE '%技术%' THEN 1
				ELSE 2
			END,
			tbf.created_at DESC
	`
	rows, err := s.db.Query(query, companyID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProjectFileRow, 0)
	for rows.Next() {
		var rowID, assetID, fname, frole string
		var fsize sql.NullInt64
		var md, plain string
		var cAt sql.NullString
		if err := rows.Scan(&rowID, &assetID, &fname, &frole, &fsize, &md, &plain, &cAt); err != nil {
			continue
		}
		markdownReady := strings.TrimSpace(md) != ""
		parseReady := markdownReady || strings.TrimSpace(plain) != ""
		rec := strings.Contains(strings.ToLower(fname+frole), "施工组织") ||
			strings.Contains(strings.ToLower(frole), "技术") ||
			strings.Contains(strings.ToLower(fname), "技术标")
		p := ProjectFileRow{
			RowID:         rowID,
			FileAssetID:   strings.TrimSpace(assetID),
			FileName:      fname,
			FileRole:      frole,
			ParseReady:    parseReady,
			MarkdownReady: markdownReady,
			Recommended:   rec,
		}
		if fsize.Valid {
			p.FileSize = &fsize.Int64
		}
		if cAt.Valid {
			s := cAt.String
			p.ContentUpdatedAt = &s
		}
		out = append(out, p)
	}
	return out, nil
}

// ResolveLocalFiles validates file_asset ids for a client-side project and returns the same shape as ListTechBidProjectFiles.
func (s *KnowledgeExtractService) ResolveLocalFiles(companyID string, files []LocalFileInput) ([]ProjectFileRow, error) {
	out := make([]ProjectFileRow, 0)
	uuidRe := regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	for _, f := range files {
		id := strings.TrimSpace(f.ID)
		if !uuidRe.MatchString(id) {
			continue
		}
		var md, plain string
		var fname string
		var fsize sql.NullInt64
		var cAt sql.NullString
		err := s.db.QueryRow(`
			SELECT COALESCE(f.file_name,''), COALESCE(f.file_size,0),
				COALESCE(c.markdown_text,''), COALESCE(c.plain_text,''), c.created_at
			FROM file_asset f
			LEFT JOIN file_content c ON c.file_asset_id = f.id
			WHERE f.id = ? AND f.company_id = ?
			ORDER BY c.created_at DESC LIMIT 1
		`, id, companyID).Scan(&fname, &fsize, &md, &plain, &cAt)
		if err != nil {
			continue
		}
		markdownReady := strings.TrimSpace(md) != ""
		parseReady := markdownReady || strings.TrimSpace(plain) != ""
		p := ProjectFileRow{
			RowID:         id,
			FileAssetID:   id,
			FileName:      coalesceStr(f.Name, fname),
			FileRole:      f.Role,
			ParseReady:    parseReady,
			MarkdownReady: markdownReady,
		}
		if fsize.Valid && fsize.Int64 > 0 {
			p.FileSize = &fsize.Int64
		}
		if cAt.Valid {
			s := cAt.String
			p.ContentUpdatedAt = &s
		}
		out = append(out, p)
	}
	return out, nil
}

func coalesceStr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func (s *KnowledgeExtractService) ListPromptTemplatesForType(knowledgeType string) ([]map[string]interface{}, error) {
	kt := strings.TrimSpace(knowledgeType)
	prefix := "knowledge_extract_" + kt + "_"
	var rows []struct {
		ID         int64  `db:"id"`
		PromptKey  string `db:"prompt_key"`
		PromptName string `db:"prompt_name"`
		Version    int    `db:"version"`
		Content    string `db:"content"`
	}
	err := s.db.Select(&rows, `
		SELECT id, prompt_key, prompt_name, version, content FROM prompt_template
		WHERE status = 1 AND prompt_key LIKE 'knowledge_extract_%'
		ORDER BY
			CASE WHEN prompt_key LIKE ? THEN 0 WHEN prompt_key = 'knowledge_extract_default' THEN 1 ELSE 2 END,
			prompt_key ASC
	`, prefix+"%")
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		if kt != "" && kt != "method" && !strings.HasPrefix(r.PromptKey, prefix) && r.PromptKey != "knowledge_extract_default" {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":          r.ID,
			"prompt_key":  r.PromptKey,
			"prompt_name": r.PromptName,
			"version":     r.Version,
			"content":     r.Content,
		})
	}
	return out, nil
}

// createExtractTaskRecord validates, resolves prompt, inserts task row as running, returns LLM inputs.
func (s *KnowledgeExtractService) createExtractTaskRecord(companyID string, req CreateExtractTaskRequest) (taskID string, userContent string, tmplSystem string, err error) {
	if req.SourceOrigin != "tech_bid" && req.SourceOrigin != "local_library" {
		return "", "", "", fmt.Errorf("invalid source_origin")
	}
	if req.ExtractScope != "full" && req.ExtractScope != "sections" {
		return "", "", "", fmt.Errorf("invalid extract_scope")
	}
	if strings.TrimSpace(req.MarkdownContent) == "" {
		return "", "", "", fmt.Errorf("markdown_content required")
	}

	taskID = uuid.New().String()
	var promptKey string
	var promptVer int
	var tmplContent string

	if req.PromptOverride != "" {
		tmplContent = req.PromptOverride
		promptKey = "override"
		promptVer = 0
	} else if req.PromptTemplateID != nil && *req.PromptTemplateID > 0 {
		var row struct {
			PromptKey     string `db:"prompt_key"`
			Version       int    `db:"version"`
			Content       string `db:"content"`
			SystemContent string `db:"system_content"`
		}
		err := s.db.Get(&row, `SELECT prompt_key, version, content, system_content FROM prompt_template WHERE id = ? AND status = 1`, *req.PromptTemplateID)
		if err != nil {
			return "", "", "", fmt.Errorf("prompt template not found")
		}
		promptKey = row.PromptKey
		promptVer = row.Version
		tmplContent = row.Content
		tmplSystem = row.SystemContent
	} else if req.PromptKey != "" {
		tmplContent = s.promptService.GetPrompt(req.PromptKey)
		tmplSystem, _ = s.promptService.GetPromptFull(req.PromptKey)
		promptKey = req.PromptKey
		promptVer = 1
	} else {
		key := "knowledge_extract_" + req.KnowledgeType + "_default"
		tmplContent = s.promptService.GetPrompt(key)
		tmplSystem, _ = s.promptService.GetPromptFull(key)
		if tmplContent == "" {
			tmplContent = s.promptService.GetPrompt("knowledge_extract_default")
			tmplSystem, _ = s.promptService.GetPromptFull("knowledge_extract_default")
			key = "knowledge_extract_default"
		}
		promptKey = key
		promptVer = 1
	}

	if tmplContent == "" {
		return "", "", "", fmt.Errorf("no prompt template available")
	}

	userContent = strings.ReplaceAll(tmplContent, "{{markdown_content}}", req.MarkdownContent)
	userContent = strings.ReplaceAll(userContent, "{{ markdown_content }}", req.MarkdownContent)

	var tmplID interface{}
	if req.PromptTemplateID != nil {
		tmplID = *req.PromptTemplateID
	}

	_, err = s.db.Exec(`
		INSERT INTO knowledge_extract_task (
			id, company_id, knowledge_type, source_origin, source_project_id, source_project_name,
			source_file_id, extract_scope, selected_sections_json, prompt_template_id, prompt_key, prompt_version,
			prompt_override, status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'running', ?)
	`, taskID, companyID, req.KnowledgeType, req.SourceOrigin, req.SourceProjectID, nullStr(req.SourceProjectName),
		req.SourceFileID, req.ExtractScope, nullStr(req.SelectedSectionsJSON), tmplID, promptKey, promptVer,
		nullStr(req.PromptOverride), time.Now())
	if err != nil {
		return "", "", "", err
	}
	return taskID, userContent, tmplSystem, nil
}

func (s *KnowledgeExtractService) executeExtractLLM(companyID, taskID string, userContent, tmplSystem string) error {
	msgs := []LLMMessage{}
	if strings.TrimSpace(tmplSystem) != "" {
		msgs = append(msgs, LLMMessage{Role: "system", Content: tmplSystem})
	}
	msgs = append(msgs, LLMMessage{Role: "user", Content: userContent})

	raw, err := s.ai.CallLLM(msgs, 0.2)
	if err != nil {
		_, _ = s.db.Exec(`UPDATE knowledge_extract_task SET status = 'failed', error_message = ?, updated_at = ? WHERE id = ? AND company_id = ?`, err.Error(), time.Now(), taskID, companyID)
		return err
	}

	items, jerr := parseKnowledgeExtractJSON(raw)
	if jerr != nil {
		_, _ = s.db.Exec(`UPDATE knowledge_extract_task SET status = 'failed', error_message = ?, updated_at = ? WHERE id = ? AND company_id = ?`, jerr.Error(), time.Now(), taskID, companyID)
		return jerr
	}

	var commitItems []CommitExtractItem
	for _, it := range items {
		rid := uuid.New().String()
		b, _ := json.Marshal(it)
		title := ""
		if v, ok := it["name"].(string); ok {
			title = v
		} else if v, ok := it["title"].(string); ok {
			title = v
		}
		sec := ""
		if v, ok := it["source_section"].(string); ok {
			sec = v
		}
		_, _ = s.db.Exec(`INSERT INTO knowledge_extract_result (id, task_id, title, content_json, source_section, selected_flag) VALUES (?,?,?,?,?,1)`,
			rid, taskID, title, string(b), sec)

		commitItems = append(commitItems, CommitExtractItem{
			ResultID:    rid,
			Include:     true,
			ContentJSON: string(b),
			Title:       title,
		})
	}

	_, _ = s.db.Exec(`UPDATE knowledge_extract_task SET status = 'completed', updated_at = ? WHERE id = ? AND company_id = ?`, time.Now(), taskID, companyID)

	// AUTO COMMIT: Directly trigger CommitTask logic
	if len(commitItems) > 0 {
		_, err = s.CommitTask(companyID, taskID, commitItems)
		if err != nil {
			log.Printf("[KnowledgeExtract] auto-commit failed for task %s: %v", taskID, err)
		}
	}

	return nil
}

// CreateAndRunTask runs extract synchronously (tests / legacy).
func (s *KnowledgeExtractService) CreateAndRunTask(companyID string, req CreateExtractTaskRequest) (string, error) {
	taskID, userContent, tmplSystem, err := s.createExtractTaskRecord(companyID, req)
	if err != nil {
		return "", err
	}
	if err := s.executeExtractLLM(companyID, taskID, userContent, tmplSystem); err != nil {
		return "", err
	}
	return taskID, nil
}

// StartExtractTaskAsync inserts the task and runs LLM in a background goroutine; returns task_id immediately.
func (s *KnowledgeExtractService) StartExtractTaskAsync(companyID string, req CreateExtractTaskRequest) (string, error) {
	taskID, userContent, tmplSystem, err := s.createExtractTaskRecord(companyID, req)
	if err != nil {
		return "", err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[KnowledgeExtract] async panic: %v\n%s", r, debug.Stack())
				msg := fmt.Sprintf("internal error: %v", r)
				_, _ = s.db.Exec(`UPDATE knowledge_extract_task SET status = 'failed', error_message = ?, updated_at = ? WHERE id = ? AND company_id = ?`, msg, time.Now(), taskID, companyID)
			}
		}()
		if err := s.executeExtractLLM(companyID, taskID, userContent, tmplSystem); err != nil {
			log.Printf("[KnowledgeExtract] async extract failed task=%s: %v", taskID, err)
		}
	}()
	return taskID, nil
}

func nullStr(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func parseKnowledgeExtractJSON(raw string) ([]map[string]interface{}, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "\ufeff")
	if idx := strings.Index(s, "```"); idx >= 0 {
		rest := s[idx+3:]
		rest = strings.TrimPrefix(rest, "json")
		rest = strings.TrimSpace(rest)
		if end := strings.Index(rest, "```"); end >= 0 {
			s = strings.TrimSpace(rest[:end])
		}
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		var one map[string]interface{}
		if err2 := json.Unmarshal([]byte(s), &one); err2 == nil {
			return []map[string]interface{}{one}, nil
		}
		return nil, fmt.Errorf("invalid JSON from model: %w", err)
	}
	return arr, nil
}

func (s *KnowledgeExtractService) GetTaskResults(companyID, taskID string) ([]map[string]interface{}, error) {
	var cid string
	err := s.db.Get(&cid, `SELECT company_id FROM knowledge_extract_task WHERE id = ?`, taskID)
	if err != nil || cid != companyID {
		return nil, fmt.Errorf("task not found")
	}
	rows := []struct {
		ID               string `db:"id"`
		Title            sql.NullString
		ContentJSON      string `db:"content_json"`
		SourceSection    sql.NullString
		SelectedFlag     int `db:"selected_flag"`
		SavedKnowledgeID sql.NullString
	}{}
	err = s.db.Select(&rows, `SELECT id, title, content_json, source_section, selected_flag, saved_knowledge_id FROM knowledge_extract_result WHERE task_id = ? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		m := map[string]interface{}{
			"id":            r.ID,
			"content_json":  r.ContentJSON,
			"selected_flag": r.SelectedFlag == 1,
		}
		if r.Title.Valid {
			m["title"] = r.Title.String
		}
		if r.SourceSection.Valid {
			m["source_section"] = r.SourceSection.String
		}
		if r.SavedKnowledgeID.Valid {
			m["saved_knowledge_id"] = r.SavedKnowledgeID.String
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *KnowledgeExtractService) GetTaskMeta(companyID, taskID string) (map[string]interface{}, error) {
	var row struct {
		KnowledgeType     string         `db:"knowledge_type"`
		SourceOrigin      string         `db:"source_origin"`
		SourceProjectID   string         `db:"source_project_id"`
		SourceProjectName sql.NullString `db:"source_project_name"`
		SourceFileID      string         `db:"source_file_id"`
		Status            string         `db:"status"`
		ErrorMessage      sql.NullString `db:"error_message"`
	}
	err := s.db.Get(&row, `SELECT knowledge_type, source_origin, source_project_id, source_project_name, source_file_id, status, error_message FROM knowledge_extract_task WHERE id = ? AND company_id = ?`, taskID, companyID)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}
	m := map[string]interface{}{
		"knowledge_type":    row.KnowledgeType,
		"source_origin":     row.SourceOrigin,
		"source_project_id": row.SourceProjectID,
		"source_file_id":    row.SourceFileID,
		"status":            row.Status,
	}
	if row.SourceProjectName.Valid {
		m["source_project_name"] = row.SourceProjectName.String
	}
	if row.ErrorMessage.Valid && strings.TrimSpace(row.ErrorMessage.String) != "" {
		m["error_message"] = row.ErrorMessage.String
	}
	return m, nil
}

func (s *KnowledgeExtractService) CommitTask(companyID, taskID string, items []CommitExtractItem) (int, error) {
	meta, err := s.GetTaskMeta(companyID, taskID)
	if err != nil {
		return 0, err
	}
	if meta["status"] != "completed" {
		return 0, fmt.Errorf("task not completed")
	}
	knowledgeType := meta["knowledge_type"].(string)
	sourceOrigin := meta["source_origin"].(string)
	projID := meta["source_project_id"].(string)
	projName, _ := meta["source_project_name"].(string)
	fileID := meta["source_file_id"].(string)

	var fileName string
	_ = s.db.Get(&fileName, `SELECT file_name FROM file_asset WHERE id = ?`, fileID)

	n := 0
	for _, it := range items {
		if !it.Include {
			_, _ = s.db.Exec(`UPDATE knowledge_extract_result SET selected_flag = 0 WHERE id = ? AND task_id = ?`, it.ResultID, taskID)
			continue
		}
		content := it.ContentJSON
		if strings.TrimSpace(content) == "" {
			var existing string
			_ = s.db.Get(&existing, `SELECT content_json FROM knowledge_extract_result WHERE id = ? AND task_id = ?`, it.ResultID, taskID)
			content = existing
		}
		title := strings.TrimSpace(it.Title)
		if title == "" {
			var obj map[string]interface{}
			_ = json.Unmarshal([]byte(content), &obj)
			if v, ok := obj["name"].(string); ok && v != "" {
				title = v
			} else if v, ok := obj["title"].(string); ok {
				title = v
			}
		}
		if title == "" {
			title = "提炼条目"
		}

		srcRef := map[string]interface{}{
			"source_type":    "history_project_extract",
			"source_origin":  sourceOrigin,
			"source_project": projName,
			"source_file":    fileName,
			"task_id":        taskID,
		}
		refJSON, _ := json.Marshal(srcRef)
		sourceDesc := fmt.Sprintf("历史项目提炼 | %s | 项目:%s | 文件:%s", sourceOrigin, coalesceStr(projName, projID), coalesceStr(fileName, fileID))

		kid := uuid.New().String()
		now := time.Now()
		_, err := s.db.Exec(`
			INSERT INTO tech_bid_knowledge_items (
				id, company_id, item_type, item_name, item_content, tags_json, applicable_chapters, source_desc,
				source_project_id, source_file_id, source_reference, knowledge_status, manually_edited, extract_task_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, '[]', '', ?, ?, ?, ?, 'published', 0, ?, ?, ?)
		`, kid, companyID, knowledgeType, title, content, sourceDesc, projID, fileID, string(refJSON), taskID, now, now)
		if err != nil {
			log.Printf("[KnowledgeExtract] commit insert failed: %v", err)
			continue
		}
		_, _ = s.db.Exec(`UPDATE knowledge_extract_result SET selected_flag = 1, saved_knowledge_id = ? WHERE id = ? AND task_id = ?`, kid, it.ResultID, taskID)
		n++
	}
	_, _ = s.db.Exec(`UPDATE knowledge_extract_task SET updated_at = ? WHERE id = ?`, time.Now(), taskID)
	return n, nil
}

func (s *KnowledgeExtractService) CancelTask(companyID, taskID string) error {
	res, err := s.db.Exec(`UPDATE knowledge_extract_task SET status = 'cancelled', updated_at = ? WHERE id = ? AND company_id = ? AND status IN ('pending','running')`, time.Now(), taskID, companyID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("nothing to cancel")
	}
	return nil
}
