package handler

import (
	"backend_go/internal/model"
	"backend_go/internal/service"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type SettingsHandler struct {
	db        *sqlx.DB
	generator *service.IndustrySkeletonGenerator
}

func NewSettingsHandler(db *sqlx.DB, generator *service.IndustrySkeletonGenerator) *SettingsHandler {
	return &SettingsHandler{db: db, generator: generator}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	var rows []struct {
		Key   string  `db:"key"`
		Value *string `db:"value"`
	}
	err := h.db.Select(&rows, "SELECT key, value FROM system_settings")
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	type SettingItem struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}

	result := make([]SettingItem, 0, len(rows))
	for _, row := range rows {
		val := ""
		if row.Value != nil {
			val = *row.Value
		}
		result = append(result, SettingItem{
			Key:         row.Key,
			Value:       val,
			Description: "", // Schema doesn't support descriptions yet
		})
	}

	c.JSON(200, result)
}

func (h *SettingsHandler) UpdateSettingsBatch(c *gin.Context) {
	var input struct {
		Settings []struct {
			Key   string `json:"key" binding:"required"`
			Value string `json:"value"`
		} `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	for _, s := range input.Settings {
		_, err := tx.Exec("INSERT OR REPLACE INTO system_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)", s.Key, s.Value)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}
func (h *SettingsHandler) GetOCRSettings(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var settings model.OCRSettings
	err := h.db.Get(&settings, "SELECT * FROM ocr_settings WHERE company_id = ?", companyID)
	if err != nil {
		// Return default settings if not found
		c.JSON(200, model.OCRSettings{
			Mode:           "auto",
			Status:         "not_configured",
			MaxConcurrency: 2,
			TimeoutSeconds: 60,
			RetryTimes:     3,
		})
		return
	}
	c.JSON(200, settings)
}

func (h *SettingsHandler) UpdateOCRSettings(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input model.OCRSettings
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec(`
		INSERT OR REPLACE INTO ocr_settings 
		(id, mode, service_url, service_port, api_key, token, default_strategy, max_concurrency, timeout_seconds, retry_times, model_version, status, company_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		"default_ocr", input.Mode, input.ServiceURL, input.ServicePort, input.APIKey, input.Token, input.DefaultStrategy, input.MaxConcurrency, input.TimeoutSeconds, input.RetryTimes, input.ModelVersion, "configured", companyID)

	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *SettingsHandler) TestOCRConnection(c *gin.Context) {
	var input struct {
		ServiceURL string `json:"service_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Service URL is required")
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(input.ServiceURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法连接到 OCR 服务: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "服务响应错误，状态码: " + http.StatusText(resp.StatusCode),
		})
		return
	}

	// Simple heuristic for version/type detection
	// Many PaddleOCR implementations return a placeholder or JSON
	serverHeader := resp.Header.Get("Server")
	if serverHeader == "" {
		serverHeader = "Generic OCR Engine"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "连接成功",
		"version": serverHeader,
	})
}

func (h *SettingsHandler) GetOCRProviderInfo(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var settings model.OCRSettings
	err := h.db.Get(&settings, "SELECT * FROM ocr_settings WHERE company_id = ?", companyID)

	endpoint := "http://127.0.0.1:18082"
	if err == nil && settings.ServiceURL != nil {
		host := *settings.ServiceURL
		port := "18082"
		if settings.ServicePort != nil && *settings.ServicePort != "" {
			port = *settings.ServicePort
		}
		endpoint = fmt.Sprintf("%s:%s", host, port)
	}

	// Check health: we consider any non-network error as 'alive' because root might be 404
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	healthy := err == nil // Any response means the port is open and server is up
	if resp != nil {
		resp.Body.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":   "PaddleOCR",
		"endpoint":   endpoint,
		"healthy":    healthy,
		"isDefault":  true,
		"pythonPath": "",
		"scriptPath": "",
	})
}

func (h *SettingsHandler) HealthCheckOCR(c *gin.Context) {
	// Re-use provider info logic but return format expected by handleCheckOcr
	companyID, _ := c.Get("companyID")
	var settings model.OCRSettings
	err := h.db.Get(&settings, "SELECT * FROM ocr_settings WHERE company_id = ?", companyID)

	endpoint := "http://127.0.0.1:18082"
	if err == nil && settings.ServiceURL != nil {
		host := *settings.ServiceURL
		port := "18082"
		if settings.ServicePort != nil && *settings.ServicePort != "" {
			port = *settings.ServicePort
		}
		endpoint = fmt.Sprintf("%s:%s", host, port)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint)
	ok := err == nil // Any server-side response (even 404) means the service is running
	if resp != nil {
		resp.Body.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      ok,
		"started": false, // We don't support auto-starting from Go yet
	})
}

func (h *SettingsHandler) TestAIConnection(c *gin.Context) {
	// 1. Fetch current settings from DB
	var rows []struct {
		Key   string  `db:"key"`
		Value *string `db:"value"`
	}
	err := h.db.Select(&rows, "SELECT key, value FROM system_settings WHERE key IN ('ai_ingest_endpoint', 'ai_ingest_model', 'ai_api_key')")
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch settings: "+err.Error())
		return
	}

	settings := make(map[string]string)
	for _, row := range rows {
		if row.Value != nil {
			settings[row.Key] = *row.Value
		}
	}

	endpoint := settings["ai_ingest_endpoint"]
	model := settings["ai_ingest_model"]
	apiKey := settings["ai_api_key"]

	if apiKey == "" {
		Error(c, http.StatusBadRequest, "API Key is not configured")
		return
	}

	// 2. Initialize AI Client
	// Use local import to avoid circular dependencies if any, but service is fine here
	aiClient := &service.AIClient{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
	}

	// 3. Perform a simple test call
	messages := []service.LLMMessage{
		{Role: "user", Content: "Connection test. Please reply with 'Successfully connected' and nothing else."},
	}

	resp, err := aiClient.CallLLM(messages, 0.7)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp,
	})
}

func (h *SettingsHandler) TestDoubaoConnection(c *gin.Context) {
	// 1. Fetch current settings from DB
	var rows []struct {
		Key   string  `db:"key"`
		Value *string `db:"value"`
	}
	err := h.db.Select(&rows, "SELECT key, value FROM system_settings WHERE key IN ('doubao_endpoint', 'doubao_model_id', 'doubao_api_key')")
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch settings: "+err.Error())
		return
	}

	settings := make(map[string]string)
	for _, row := range rows {
		if row.Value != nil {
			settings[row.Key] = *row.Value
		}
	}

	endpoint := settings["doubao_endpoint"]
	model := settings["doubao_model_id"]
	apiKey := settings["doubao_api_key"]

	if apiKey == "" {
		Error(c, http.StatusBadRequest, "Doubao API Key is not configured")
		return
	}

	// 2. Initialize AI Client (Doubao uses OpenAI compatible API)
	aiClient := &service.AIClient{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
	}

	// 3. Perform a simple test call
	messages := []service.LLMMessage{
		{Role: "user", Content: "Connection test. Please reply with 'Doubao connected' and nothing else."},
	}

	resp, err := aiClient.CallLLM(messages, 0.7)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp,
	})
}
func (h *SettingsHandler) ListIndustrySkeletons(c *gin.Context) {
	var skeletons []model.IndustrySkeletonDB
	err := h.db.Select(&skeletons, "SELECT * FROM tech_bid_industry_skeletons ORDER BY updated_at DESC")
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, skeletons)
}

func (h *SettingsHandler) UpdateIndustrySkeleton(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var input model.IndustrySkeletonDB
	if err := json.Unmarshal(body, &input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// 兜底：部分客户端/代理下 body 中 parent_id 可能丢失，可用查询参数或 Header 指定一级分类
	if input.ParentID == nil {
		if q := strings.TrimSpace(c.Query("categoryId")); q != "" {
			input.ParentID = &q
		} else if hdr := strings.TrimSpace(c.GetHeader("X-Skeleton-Parent-Id")); hdr != "" {
			input.ParentID = &hdr
		}
	}

	// Normalize parent_id: L1 items MUST have NULL, not "" or any falsy value.
	if input.ParentID != nil && strings.TrimSpace(*input.ParentID) == "" {
		input.ParentID = nil
	}

	if input.ID == "" {
		input.ID = uuid.New().String()
	}

	_, execErr := h.db.Exec(`
		INSERT OR REPLACE INTO tech_bid_industry_skeletons
		(id, industry_name, parent_id, logical_chapters_json, common_section_pool_json, industry_keywords_json, title_candidate_pool_json, matching_rules_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		input.ID, input.IndustryName, input.ParentID, input.LogicalChaptersJSON,
		input.CommonSectionPoolJSON, input.IndustryKeywordsJSON, input.TitleCandidatePoolJSON, input.MatchingRulesJSON)

	if execErr != nil {
		Error(c, http.StatusInternalServerError, execErr.Error())
		return
	}

	c.JSON(200, gin.H{"success": true, "id": input.ID})
}

func (h *SettingsHandler) DeleteIndustrySkeleton(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.Exec("DELETE FROM tech_bid_industry_skeletons WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func (h *SettingsHandler) GenerateIndustrySkeletonDraft(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		IndustryName string `json:"industryName"`
		ParentID     string `json:"parentId"`
	}
	// Try to bind JSON if body exists, but don't fail if it doesn't (backward compat)
	c.ShouldBindJSON(&req)

	industryName := req.IndustryName
	parentName := ""

	if id != "" && id != "undefined" {
		// Fetch current skeleton info
		var skeleton model.IndustrySkeletonDB
		err := h.db.Get(&skeleton, "SELECT * FROM tech_bid_industry_skeletons WHERE id = ?", id)
		if err == nil {
			industryName = skeleton.IndustryName
			if (req.ParentID == "" || req.ParentID == "undefined") && skeleton.ParentID != nil {
				req.ParentID = *skeleton.ParentID
			}
		}
	}

	// Fetch parent name if exists
	if req.ParentID != "" && req.ParentID != "undefined" {
		var parent model.IndustrySkeletonDB
		err := h.db.Get(&parent, "SELECT industry_name FROM tech_bid_industry_skeletons WHERE id = ?", req.ParentID)
		if err == nil {
			parentName = parent.IndustryName
		}
	}

	if industryName == "" {
		Error(c, http.StatusBadRequest, "Industry name or ID is required")
		return
	}

	// 1. Offload to background worker queue
	err := h.generator.EnqueueTask(id, industryName, parentName)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to enqueue generation task: "+err.Error())
		return
	}

	// 2. Return 202 Accepted immediately
	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "已加入 AI 生成队列",
		"id":      id,
		"status":  "queued",
	})
}
