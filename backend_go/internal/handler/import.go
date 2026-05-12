package handler

import (
	"backend_go/internal/service"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/xuri/excelize/v2"
)

type ImportHandler struct {
	db          *sqlx.DB
	taskService *service.FileTaskService
}

func NewImportHandler(db *sqlx.DB, taskService *service.FileTaskService) *ImportHandler {
	return &ImportHandler{
		db:          db,
		taskService: taskService,
	}
}

func (h *ImportHandler) CreateImportTask(c *gin.Context) {
	val, _ := c.Get("companyID")
	cid := val.(string)

	var req struct {
		TaskName          string `json:"task_name"`
		SourceProjectId   string `json:"source_project_id"`
		SourceFileId      string `json:"source_file_id"`
		TargetLibraryType string `json:"target_library_type"`
		OcrMode           string `json:"ocr_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	// Trigger automated task using the new service
	log.Printf("[Import] Starting task for file %s (mode: %s, library: %s)", req.SourceFileId, req.OcrMode, req.TargetLibraryType)
	taskID, err := h.taskService.StartTask(req.SourceFileId, cid, req.OcrMode, req.TargetLibraryType)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to start processing task")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": taskID, "status": "running"})
}

func (h *ImportHandler) GetTaskStatus(c *gin.Context) {
	id := c.Param("id")
	var task struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Progress int    `json:"progress"`
	}

	err := h.db.Get(&task, "SELECT id, status, progress FROM file_ocr_task WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Task not found")
		return
	}

	c.JSON(http.StatusOK, task)
}

// AnalyzeExcel reads headers from an uploaded excel file and provides mapping suggestions
func (h *ImportHandler) AnalyzeExcel(c *gin.Context) {
	var req struct {
		FileId string `json:"file_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request: file_id is required")
		return
	}

	// Find file path
	var storedPath string
	err := h.db.Get(&storedPath, "SELECT stored_path FROM file_asset WHERE id = ?", req.FileId)
	if err != nil {
		Error(c, http.StatusNotFound, "File asset not found")
		return
	}

	f, err := excelize.OpenFile(storedPath)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to open excel file: "+err.Error())
		return
	}
	defer f.Close()

	sheetNames := f.GetSheetList()
	if len(sheetNames) == 0 {
		Error(c, http.StatusBadRequest, "Excel file has no sheets")
		return
	}

	rows, err := f.GetRows(sheetNames[0])
	if err != nil || len(rows) == 0 {
		Error(c, http.StatusBadRequest, "Failed to read first sheet or sheet is empty")
		return
	}

	headers := rows[0]
	targetFields := []struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Required bool   `json:"required"`
	}{
		{Key: "project_name", Label: "项目名称", Required: true},
		{Key: "project_location", Label: "项目地点", Required: false},
		{Key: "owner_org", Label: "建设单位/业主", Required: false},
		{Key: "project_manager_name", Label: "项目经理", Required: false},
		{Key: "technical_leader_name", Label: "技术负责人", Required: false},
		{Key: "safety_leader_name", Label: "安全负责人", Required: false},
		{Key: "winning_date", Label: "中标时间", Required: false},
		{Key: "completion_date", Label: "完工/竣工时间", Required: false},
		{Key: "bid_amount_value", Label: "中标金额(万元)", Required: false},
		{Key: "amount_value", Label: "合同金额(万元)", Required: false},
		{Key: "scale_desc", Label: "规模/工程描述", Required: false},
	}

	type MappingSuggestion struct {
		TargetField  string  `json:"targetField"`
		TargetLabel  string  `json:"targetLabel"`
		SourceHeader *string `json:"sourceHeader"`
		Required     bool    `json:"required"`
	}

	suggestions := make([]MappingSuggestion, 0, len(targetFields))
	for _, target := range targetFields {
		var matchedHeader *string
		for _, h := range headers {
			trimmedH := strings.TrimSpace(h)
			if trimmedH == "" {
				continue
			}
			// Simple fuzzy match
			if strings.Contains(trimmedH, target.Label) || strings.Contains(target.Label, trimmedH) ||
				strings.Contains(strings.ToLower(trimmedH), strings.ReplaceAll(target.Key, "_", "")) {
				val := trimmedH
				matchedHeader = &val
				break
			}
		}
		suggestions = append(suggestions, MappingSuggestion{
			TargetField:  target.Key,
			TargetLabel:  target.Label,
			SourceHeader: matchedHeader,
			Required:     target.Required,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"fileId":           req.FileId,
		"sheets":           sheetNames,
		"headers":          headers,
		"suggestedMapping": suggestions,
	})
}

// ExecuteImport processes the excel file and inserts records into the database based on mapping
func (h *ImportHandler) ExecuteImport(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var req struct {
		FileId     string `json:"fileId" binding:"required"`
		TargetType string `json:"targetType"` // default: performance
		Mapping    []struct {
			TargetField  string `json:"targetField"`
			SourceHeader string `json:"sourceHeader"`
		} `json:"mapping"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.TargetType == "" {
		req.TargetType = "performance"
	}

	// Get file path
	var storedPath string
	err := h.db.Get(&storedPath, "SELECT stored_path FROM file_asset WHERE id = ?", req.FileId)
	if err != nil {
		Error(c, http.StatusNotFound, "File asset not found")
		return
	}

	f, err := excelize.OpenFile(storedPath)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to open excel file")
		return
	}
	defer f.Close()

	sheetName := f.GetSheetList()[0]
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		Error(c, http.StatusBadRequest, "No data rows found in excel")
		return
	}

	headers := rows[0]
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.TrimSpace(h)] = i
	}

	// Prepare mapping index
	type fieldMap struct {
		TargetField string
		ColIdx      int
	}
	var activeMappings []fieldMap
	for _, m := range req.Mapping {
		if m.SourceHeader != "" {
			if idx, ok := headerMap[m.SourceHeader]; ok {
				activeMappings = append(activeMappings, fieldMap{m.TargetField, idx})
			}
		}
	}

	successCount := 0
	failCount := 0
	var errorMsgs []string

	batchID := uuid.New().String()

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		data := make(map[string]interface{})
		for _, m := range activeMappings {
			if m.ColIdx < len(row) {
				val := row[m.ColIdx]
				data[m.TargetField] = strings.TrimSpace(val)
			} else {
				data[m.TargetField] = ""
			}
		}

		// Validation
		projectName, _ := data["project_name"].(string)
		if projectName == "" {
			failCount++
			errorMsgs = append(errorMsgs, fmt.Sprintf("Row %d: Missing project name", i+1))
			continue
		}

		id := uuid.New().String()

		// Helper to handle excel dates
		formatExcelDate := func(raw interface{}) string {
			s := fmt.Sprintf("%v", raw)
			if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 { // Heuristic: likely an excel date serial
				t, err := excelize.ExcelDateToTime(f, false)
				if err == nil {
					return t.Format("2006-01-02")
				}
			}
			return s
		}

		if req.TargetType == "performance" {
			// --- Personnel Auto-Link Logic ---
			pmName := strings.TrimSpace(fmt.Sprintf("%v", data["project_manager_name"]))
			techName := strings.TrimSpace(fmt.Sprintf("%v", data["technical_leader_name"]))
			safetyName := strings.TrimSpace(fmt.Sprintf("%v", data["safety_leader_name"]))

			var pmID, techID, safetyID *string

			// Helper to find ID by name
			findID := func(name string) *string {
				if name == "" {
					return nil
				}
				var targetID string
				err := h.db.Get(&targetID, "SELECT id FROM person WHERE name = ? AND company_id = ? LIMIT 1", name, cid)
				if err == nil && targetID != "" {
					return &targetID
				}
				return nil
			}

			pmID = findID(pmName)
			techID = findID(techName)
			safetyID = findID(safetyName)
			// ---------------------------------

			query := `INSERT INTO project_performance (
				id, project_name, project_location, owner_org, 
				project_manager_name, pm_id,
				technical_leader_name, tech_leader_id,
				safety_leader_name, safety_leader_id, 
				winning_date, completion_date, amount_value, bid_amount_value, 
				scale_desc, company_id, source_batch_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

			// Parse amounts
			amount, _ := strconv.ParseFloat(fmt.Sprintf("%v", data["amount_value"]), 64)
			bidAmount, _ := strconv.ParseFloat(fmt.Sprintf("%v", data["bid_amount_value"]), 64)

			winningDate := formatExcelDate(data["winning_date"])
			completionDate := formatExcelDate(data["completion_date"])

			_, err = h.db.Exec(query,
				id, data["project_name"], data["project_location"], data["owner_org"],
				data["project_manager_name"], pmID,
				data["technical_leader_name"], techID,
				data["safety_leader_name"], safetyID,
				winningDate, completionDate, amount, bidAmount,
				data["scale_desc"], cid, batchID,
			)
			if err != nil {
				failCount++
				errorMsgs = append(errorMsgs, fmt.Sprintf("Row %d: DB error: %s", i+1, err.Error()))
				continue
			}
		}
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(rows) - 1,
		"success": successCount,
		"failed":  failCount,
		"errors":  errorMsgs,
		"batchId": batchID,
	})
}

