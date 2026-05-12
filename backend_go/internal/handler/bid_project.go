package handler

import (
	"backend_go/internal/agent"
	"backend_go/internal/model"
	"backend_go/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type BidProjectHandler struct {
	db              *sqlx.DB
	digitizeService *service.TenderDigitizationService
	taskService     *service.FileTaskService
	exporterService *service.Step6ExporterService
	docmindService  *service.DocMindParseService
}

func NewBidProjectHandler(db *sqlx.DB, digitizeService *service.TenderDigitizationService, taskService *service.FileTaskService, exporterService *service.Step6ExporterService, docmindService *service.DocMindParseService) *BidProjectHandler {
	return &BidProjectHandler{db: db, digitizeService: digitizeService, taskService: taskService, exporterService: exporterService, docmindService: docmindService}
}

func (h *BidProjectHandler) ListProjects(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	projects := []model.BidProject{}
	err := h.db.Select(&projects, "SELECT * FROM bid_projects WHERE company_id = ? ORDER BY updated_at DESC", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, projects)
}

func (h *BidProjectHandler) GetProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ? AND company_id = ?", id, companyID)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	h.syncProjectStepStatus(&project)

	steps := []model.BidProjectStep{}
	err = h.db.Select(&steps, "SELECT * FROM bid_project_steps WHERE project_id = ? ORDER BY step_order ASC", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to load steps: "+err.Error())
		return
	}

	files := []model.BidProjectFile{}
	err = h.db.Select(&files, `
		SELECT bpf.*, fa.parse_status, fa.id as asset_id 
		FROM bid_project_files bpf 
		LEFT JOIN file_asset fa ON bpf.file_asset_id = fa.id 
		WHERE bpf.project_id = ?`, id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to load files: "+err.Error())
		return
	}

	var latestVersion model.BidProjectVersion
	vErr := h.db.Get(&latestVersion, "SELECT * FROM bid_project_versions WHERE project_id = ? ORDER BY version_no DESC LIMIT 1", id)

	var latestRuleParseJSON string
	h.db.Get(&latestRuleParseJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", id, "extract_rules")

	var latestRuleParse interface{}
	if latestRuleParseJSON != "" {
		json.Unmarshal([]byte(latestRuleParseJSON), &latestRuleParse)
	}

	var latestCompanyAdaptationJSON string
	h.db.Get(&latestCompanyAdaptationJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? ORDER BY created_at DESC LIMIT 1", id, "company_adaptation")

	var latestCompanyAdaptation interface{}
	if latestCompanyAdaptationJSON != "" {
		json.Unmarshal([]byte(latestCompanyAdaptationJSON), &latestCompanyAdaptation)
	}

	var latestResourceCombinationJSON string
	h.db.Get(&latestResourceCombinationJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? ORDER BY created_at DESC LIMIT 1", id, "resource_combination")

	var latestResourceCombination interface{}
	if latestResourceCombinationJSON != "" {
		json.Unmarshal([]byte(latestResourceCombinationJSON), &latestResourceCombination)
	}

	var sharedTender model.SharedTender
	if project.SharedTenderID != nil {
		h.db.Get(&sharedTender, "SELECT * FROM shared_tenders WHERE id = ?", *project.SharedTenderID)
	}

	detail := model.BidProjectDetail{
		BidProject:                project,
		Steps:                     steps,
		Files:                     files,
		LatestRuleParse:           latestRuleParse,
		LatestCompanyAdaptation:   latestCompanyAdaptation,
		LatestResourceCombination: latestResourceCombination,
	}

	if vErr == nil {
		detail.LatestVersion = &latestVersion
	}
	if project.SharedTenderID != nil {
		detail.SharedTender = &sharedTender
	}

	c.JSON(200, detail)
}

func (h *BidProjectHandler) syncProjectStepStatus(project *model.BidProject) {
	if project.CurrentStep == nil || project.CurrentStepStatus == nil {
		return
	}

	currentStep := *project.CurrentStep
	currentStatus := *project.CurrentStepStatus

	if currentStatus != "running" {
		return
	}

	if currentStep == "tender_detail_extract" {
		var assetID string
		err := h.db.Get(&assetID, "SELECT file_asset_id FROM bid_project_files WHERE project_id = ? AND file_role = 'tender' ORDER BY created_at DESC LIMIT 1", project.ID)
		if err == nil && assetID != "" {
			var parseStatus string
			var errMsg *string
			h.db.Get(&parseStatus, "SELECT parse_status FROM file_asset WHERE id = ?", assetID)
			h.db.Get(&errMsg, "SELECT last_error_message FROM file_asset WHERE id = ?", assetID)

			if parseStatus == "success" || parseStatus == "approved" {
				h.db.Exec("UPDATE bid_projects SET current_step_status = 'success', updated_at = ? WHERE id = ?", time.Now(), project.ID)
				status := "success"
				project.CurrentStepStatus = &status
			} else if parseStatus == "failed" {
				msg := "数字化解析失败"
				if errMsg != nil && *errMsg != "" {
					msg += ": " + *errMsg
				}
				h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", msg, time.Now(), project.ID)
				status := "failed"
				project.CurrentStepStatus = &status
				project.LastErrorMessage = &msg
			}
		}
	}
}

func (h *BidProjectHandler) CreateProject(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		ProjectName string `json:"project_name" binding:"required"`
		TenderCode  string `json:"tender_code"`
		OwnerName   string `json:"owner_name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	query := `INSERT INTO bid_projects (id, company_id, project_name, tender_code, owner_name, project_status, current_step) 
              VALUES (?, ?, ?, ?, ?, 'created', 'tender_detail_extract')`
	_, err := h.db.Exec(query, id, companyID, input.ProjectName, input.TenderCode, input.OwnerName)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Init steps (simplified)
	steps := []string{"tender_detail_extract", "rule_parse", "company_adaptation", "resource_combination", "user_confirmation", "chapter_generation", "attachment_assembly", "risk_review", "output_finalize"}
	for i, name := range steps {
		h.db.Exec("INSERT INTO bid_project_steps (id, project_id, step_name, step_order, step_status) VALUES (?, ?, ?, ?, ?)", uuid.New().String(), id, name, i+1, "not_started")
	}

	c.JSON(201, gin.H{"id": id})
}

func (h *BidProjectHandler) StartWorkflow(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.Exec("UPDATE bid_projects SET project_status = 'running', current_step_status = 'running', updated_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "Workflow started (Go)"})
}

func (h *BidProjectHandler) AddProjectFile(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var input struct {
		FileAssetID string `json:"file_asset_id" binding:"required"`
		FileRole    string `json:"file_role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var asset struct {
		FileName   *string `db:"file_name"`
		StoredPath *string `db:"stored_path"`
		MimeType   *string `db:"mime_type"`
		FileSize   *int64  `db:"file_size"`
		Sha256     *string `db:"sha256"`
	}
	err := h.db.Get(&asset, "SELECT file_name, stored_path, mime_type, file_size, sha256 FROM file_asset WHERE id = ?", input.FileAssetID)
	if err != nil {
		Error(c, http.StatusNotFound, "File asset not found: "+err.Error())
		return
	}

	fid := uuid.New().String()
	isPrimary := 0
	if input.FileRole == "tender" {
		isPrimary = 1
	}

	query := `INSERT INTO bid_project_files (
		id, project_id, company_id, file_asset_id, file_role, file_name, stored_path, mime_type, file_size, sha256, is_primary_tender, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	_, err = h.db.Exec(query, fid, id, companyID, input.FileAssetID, input.FileRole, asset.FileName, asset.StoredPath, asset.MimeType, asset.FileSize, asset.Sha256, isPrimary, now)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to associate file: "+err.Error())
		return
	}

	c.JSON(201, gin.H{"id": fid, "file_id": fid})
}

func (h *BidProjectHandler) RunWorkflow(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Force bool `json:"force"`
	}
	c.ShouldBindJSON(&input)

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	currentStep := ""
	if project.CurrentStep != nil {
		currentStep = *project.CurrentStep
	}

	if currentStep == "tender_detail_extract" {
		// Get Tender File
		var file model.BidProjectFile
		err := h.db.Get(&file, "SELECT * FROM bid_project_files WHERE project_id = ? AND file_role = 'tender' ORDER BY created_at DESC LIMIT 1", id)
		if err != nil {
			Error(c, http.StatusBadRequest, "Please upload a tender document first")
			return
		}

		assetID := ""
		if file.FileAssetID != nil {
			assetID = *file.FileAssetID
		}

		if assetID == "" {
			Error(c, http.StatusInternalServerError, "File asset ID is missing")
			return
		}

		// Check parse status
		var parseStatus string
		h.db.Get(&parseStatus, "SELECT parse_status FROM file_asset WHERE id = ?", assetID)

		if parseStatus != "success" {
			// Trigger parsing task
			companyID, _ := c.Get("companyID")
			h.taskService.StartTask(assetID, companyID.(string), "auto", "bid-project")
			h.db.Exec("UPDATE bid_projects SET current_step_status = 'running', last_error_message = ?, updated_at = ? WHERE id = ?", "正在启动数字化解析...", time.Now(), id)
			c.JSON(200, gin.H{"success": true, "message": "Parsing task started"})
			return
		}

		h.db.Exec("UPDATE bid_projects SET current_step_status = 'success', updated_at = ? WHERE id = ?", time.Now(), id)
		c.JSON(200, gin.H{"success": true, "message": "Metadata extraction completed"})
		return
	}

	if currentStep == "rule_parse" {
		// Pre-check if document is parsed
		var file model.BidProjectFile
		err := h.db.Get(&file, "SELECT * FROM bid_project_files WHERE project_id = ? AND file_role = 'tender' ORDER BY created_at DESC LIMIT 1", id)
		if err != nil {
			Error(c, http.StatusBadRequest, "Please upload a tender document first")
			return
		}

		assetID := ""
		if file.FileAssetID != nil {
			assetID = *file.FileAssetID
		}

		var parseStatus string
		h.db.Get(&parseStatus, "SELECT parse_status FROM file_asset WHERE id = ?", assetID)

		if parseStatus != "success" && parseStatus != "approved" {
			Error(c, http.StatusBadRequest, "文件尚未通过数字化解析，请先在第一步执行‘解析’并等待解析结束。")
			return
		}

		// Concurrency check
		if project.CurrentStepStatus != nil && *project.CurrentStepStatus == "running" && !input.Force {
			c.JSON(200, gin.H{"success": true, "message": "Rule extraction is already running"})
			return
		}

		h.db.Exec("UPDATE bid_projects SET current_step_status = 'running', last_error_message = '正在解析招标规则...', updated_at = ? WHERE id = ?", time.Now(), id)
		go h.processCommerceRuleExtraction(id, project.CompanyID, input.Force)
		c.JSON(200, gin.H{"success": true, "message": "Rule extraction started in background"})
		return
	}

	if currentStep == "company_adaptation" {
		// Concurrency check
		if project.CurrentStepStatus != nil && *project.CurrentStepStatus == "running" && !input.Force {
			c.JSON(200, gin.H{"success": true, "message": "Adaptation is already running"})
			return
		}

		h.db.Exec("UPDATE bid_projects SET current_step_status = 'running', last_error_message = '正在获取公司家底并进行智能适配审核...', updated_at = ? WHERE id = ?", time.Now(), id)
		go h.processCompanyAdaptation(id, project.CompanyID, input.Force)
		c.JSON(200, gin.H{"success": true, "message": "Company adaptation started in background"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Workflow started (Mocked for this step)"})
}

func (h *BidProjectHandler) processCommerceRuleExtraction(projectID, companyID string, force bool) {
	// A2. panic recovery must update status to failed
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("规则解析 panic: %v", r)
			log.Printf("Recovered from panic in processCommerceRuleExtraction: %v", r)
			_, _ = h.db.Exec(
				"UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?",
				msg, time.Now(), projectID,
			)
		}
	}()

	// A3. 10-minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	now := time.Now()
	_, _ = h.db.Exec(`UPDATE bid_projects SET 
		current_step_status = 'running', 
		current_progress = 5,
		current_stage_message = '正在初始化提取任务...',
		last_error_message = NULL, 
		updated_at = ? 
		WHERE id = ?`, now, projectID)

	// Get Step ID
	var stepID string
	_ = h.db.Get(&stepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = ?", projectID, "rule_parse")

	// Get Tender File
	var file model.BidProjectFile
	err := h.db.Get(&file, "SELECT * FROM bid_project_files WHERE project_id = ? AND file_role = 'tender' ORDER BY created_at DESC LIMIT 1", projectID)
	if err != nil {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "Tender file not found", time.Now(), projectID)
		return
	}

	assetID := ""
	if file.FileAssetID != nil {
		assetID = *file.FileAssetID
	}

	if assetID == "" {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "File asset ID is missing", time.Now(), projectID)
		return
	}

	// Get File Content
	var fileContent struct {
		MarkdownText *string `db:"markdown_text"`
	}
	err = h.db.Get(&fileContent, "SELECT markdown_text FROM file_asset WHERE id = ?", assetID)
	if err != nil || fileContent.MarkdownText == nil || len(strings.TrimSpace(*fileContent.MarkdownText)) == 0 {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "未能获取到任何招投标文件原文，请确保您在第一步已成功上传并完成了全文数字化提取，或重新发起。 ("+fmt.Sprint(err)+")", time.Now(), projectID)
		return
	}

	// Call Digitization Service
	profile, err := h.digitizeService.DigitizeTenderFile(ctx, assetID, *fileContent.MarkdownText, "commerce", func(percent int) {
		// Progress update
		stageMsg := "正在通过 AI 分析标书内容..."
		if percent >= 90 {
			stageMsg = "正在生成结构化内容..."
		}
		h.db.Exec("UPDATE bid_projects SET current_progress = ?, current_stage_message = ?, updated_at = ? WHERE id = ?", percent, stageMsg, time.Now(), projectID)
	})

	// A1. Check err immediately
	if err != nil {
		errMsg := "规则解析失败: " + err.Error()
		if err == context.DeadlineExceeded {
			errMsg = "规则解析超时，请重试或检查模型服务状态"
		}
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", errMsg, time.Now(), projectID)
		return
	}
	if profile == nil {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "规则解析失败: digitize 返回空结果", time.Now(), projectID)
		return
	}

	// PHYSICAL DOCUMENT SEPARATION (Format Template Boundary)
	splitter := service.NewDocumentSplitterService(h.db)
	rulePath, templatePath, splitErr := splitter.SplitMarkdownContent(ctx, projectID, *fileContent.MarkdownText, service.DetectedBoundary{
		Detected:  profile.MergedProfile.FormatTemplateBoundary.Detected,
		StartPage: profile.MergedProfile.FormatTemplateBoundary.StartPage,
		EndPage:   profile.MergedProfile.FormatTemplateBoundary.EndPage,
	})

	if splitErr == nil {
		h.db.Exec(`UPDATE bid_projects 
			SET rule_markdown_path = ?, template_markdown_path = ?, template_start_page = ?, template_end_page = ?, updated_at = ? 
			WHERE id = ?`,
			rulePath, templatePath,
			profile.MergedProfile.FormatTemplateBoundary.StartPage,
			profile.MergedProfile.FormatTemplateBoundary.EndPage,
			time.Now(), projectID)
	} else {
		log.Printf("Document separation warning: %v", splitErr)
	}

	// Transform to commerce format
	h.db.Exec("UPDATE bid_projects SET current_progress = 95, current_stage_message = '正在保存解析结果...', updated_at = ? WHERE id = ?", time.Now(), projectID)
	rules := h.transformProfileToRules(profile)
	rulesJSON, _ := json.Marshal(rules)

	// Check if results are empty
	isEmpty := true
	if elig, ok := rules["eligibility"].([]map[string]interface{}); ok && len(elig) > 0 {
		isEmpty = false
	}
	if scoring, ok := rules["scoring"].([]map[string]interface{}); ok && len(scoring) > 0 {
		isEmpty = false
	}

	finishMsg := "招标规则解析完成"
	if isEmpty {
		finishMsg = "解析完成，但未发现有效规则（可能当前段落不包含明确指标）"
	}

	// Save Action
	actionID := uuid.New().String()
	_, err = h.db.Exec(`INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at, finished_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		actionID, projectID, stepID, "extract_rules", "success", string(rulesJSON), now, time.Now())

	if err != nil {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "Failed to save results: "+err.Error(), time.Now(), projectID)
		return
	}

	// Update Project Status
	_, _ = h.db.Exec("UPDATE bid_projects SET current_step_status = 'success', current_progress = 100, current_stage_message = ?, updated_at = ? WHERE id = ?", finishMsg, time.Now(), projectID)
}

func (h *BidProjectHandler) processCompanyAdaptation(projectID, companyID string, force bool) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("公司对标 panic: %v", r)
			log.Printf("Recovered from panic in processCompanyAdaptation: %v", r)
			_, _ = h.db.Exec(
				"UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?",
				msg, time.Now(), projectID,
			)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	now := time.Now()
	_, _ = h.db.Exec(`UPDATE bid_projects SET 
		current_step_status = 'running', 
		current_progress = 5,
		current_stage_message = '正在初始化对标分析任务...',
		last_error_message = NULL, 
		updated_at = ? 
		WHERE id = ?`, now, projectID)

	var stepID string
	_ = h.db.Get(&stepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = ?", projectID, "company_adaptation")

	resultsJSON, passedCount, err := h.digitizeService.ProcessCompanyAdaptation(ctx, projectID, companyID, func(percent int, msg string) {
		h.db.Exec("UPDATE bid_projects SET current_progress = ?, current_stage_message = ?, updated_at = ? WHERE id = ?", percent, msg, time.Now(), projectID)
	})

	if err != nil {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "对标失败: "+err.Error(), time.Now(), projectID)
		return
	}

	actionID := uuid.New().String()
	_, err = h.db.Exec(`INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at, finished_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		actionID, projectID, stepID, "company_adaptation", "success", resultsJSON, now, time.Now())

	if err != nil {
		h.db.Exec("UPDATE bid_projects SET current_step_status = 'failed', last_error_message = ?, updated_at = ? WHERE id = ?", "Failed to save results: "+err.Error(), time.Now(), projectID)
		return
	}

	finishMsg := fmt.Sprintf("指标适配完成，共有 %d 项满足要求", passedCount)
	_, _ = h.db.Exec("UPDATE bid_projects SET current_step_status = 'success', current_progress = 100, current_stage_message = ?, updated_at = ? WHERE id = ?", finishMsg, time.Now(), projectID)
}

func (h *BidProjectHandler) transformProfileToRules(profile *service.ProjectProfileDigitizationResult) map[string]interface{} {
	p := profile.MergedProfile

	eligibility := []map[string]interface{}{}

	// Map Qualifications (QualificationRequirements is []ProjectProfileListItem)
	for _, it := range p.BidderRequirements.QualificationRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "qualification",
			"category_group":    "资质要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Map Personnel
	for _, it := range p.BidderRequirements.PersonnelRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "person",
			"category_group":    "人员要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Map Performance (PerformanceRequirements is []ProjectProfileListItem, iterate like PersonnelRequirements)
	for _, it := range p.BidderRequirements.PerformanceRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "project_performance",
			"category_group":    "业绩要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Map Financial Requirements
	for _, it := range p.BidderRequirements.FinancialRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "financial",
			"category_group":    "财务要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Map Credit Requirements
	for _, it := range p.BidderRequirements.CreditRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "compliance_credit",
			"category_group":    "信誉要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Map Other Mandatory Constraints (Catch-All Bucket)
	for _, it := range p.BidderRequirements.OtherMandatoryRequirements {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "other_requirements",
			"category_group":    "其他要求",
			"source_type":       "eligibility",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	scoring := []map[string]interface{}{}
	for _, it := range p.EvaluationAndPerformanceRules.ScoringItems {
		// 轻量级方案：在装配商务标规则时，强制过滤并抛弃底层解析带出来的纯技术性关键评分项
		if strings.Contains(it.Value, "施工组织设计") || strings.Contains(it.Value, "施工方案") || strings.Contains(it.Value, "技术标") || strings.Contains(it.Value, "安全文明") || strings.Contains(it.Value, "项目经理答辩") {
			continue
		}
		scoring = append(scoring, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "scoring_item",
			"category_group":    "评分项",
			"source_type":       "scoring",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	// Also add disqualification rules to eligibility (as rejection_criteria)
	for _, it := range p.EvaluationAndPerformanceRules.DisqualificationRules {
		eligibility = append(eligibility, map[string]interface{}{
			"id":                uuid.New().String(),
			"category":          "rejection_criteria",
			"category_group":    "否决投标规定",
			"source_type":       "rejection_criteria",
			"requirement_text":  it.Value,
			"confidence":        it.Confidence,
			"requires_evidence": it.RequiresEvidence,
		})
	}

	return map[string]interface{}{
		"eligibility":      eligibility,
		"scoring":          scoring,
		"normalized_rules": append(eligibility, scoring...),
	}
}

func (h *BidProjectHandler) GetStep6Payload(c *gin.Context) {
	id := c.Param("id")

	var step6Status, step6PayloadJSON, templateMarkdownPath, ruleMarkdownPath, lastErrorMessage *string
	err := h.db.QueryRow("SELECT step6_status, step6_payload_json, template_markdown_path, rule_markdown_path, last_error_message FROM bid_projects WHERE id = ?", id).Scan(&step6Status, &step6PayloadJSON, &templateMarkdownPath, &ruleMarkdownPath, &lastErrorMessage)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch project step6 status")
		return
	}

	status := "idle"
	if step6Status != nil {
		status = *step6Status
	}

	var data map[string]interface{}
	if step6PayloadJSON != nil && len(*step6PayloadJSON) > 0 {
		importJson := json.Unmarshal([]byte(*step6PayloadJSON), &data)
		if importJson != nil {
			// ignore unmarshal err implicitly
		}
	}

	// Step 5 needs chapter source before a Word template is uploaded; prefer the
	// Step 6 template when present, otherwise fall back to the parsed rule markdown.
	if content, ok := readFirstProjectMarkdown(templateMarkdownPath, ruleMarkdownPath); ok {
		if data == nil {
			data = map[string]interface{}{
				"project_id": id,
				"slots":      []interface{}{},
			}
		}
		data["original_markdown"] = string(content)
	}

	// Fetch rules to get requirement texts
	var latestRuleParseJSON string
	h.db.Get(&latestRuleParseJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", id, "extract_rules")

	type RuleInfo struct {
		RequirementText string
		Category        string
	}
	ruleMap := make(map[string]RuleInfo)

	if latestRuleParseJSON != "" {
		var parseResult struct {
			Eligibility []struct {
				ID              string `json:"id"`
				RequirementText string `json:"requirement_text"`
				Category        string `json:"category"`
			} `json:"eligibility"`
			Scoring []struct {
				ID              string `json:"id"`
				RequirementText string `json:"requirement_text"`
				Category        string `json:"category"`
			} `json:"scoring"`
		}
		if err := json.Unmarshal([]byte(latestRuleParseJSON), &parseResult); err == nil {
			for _, r := range parseResult.Eligibility {
				ruleMap[r.ID] = RuleInfo{RequirementText: r.RequirementText, Category: r.Category}
			}
			for _, r := range parseResult.Scoring {
				ruleMap[r.ID] = RuleInfo{RequirementText: r.RequirementText, Category: r.Category}
			}
		}
	}

	// Fetch company adaptation results to get the canonical display order for Step 4 items
	var latestAdaptationJSON string
	_ = h.db.Get(&latestAdaptationJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? ORDER BY created_at DESC LIMIT 1", id, "company_adaptation")

	// Build ordered list of rule IDs from company_adaptation results (same source as Step 4 UI)
	var adaptationOrderedIDs []string
	if latestAdaptationJSON != "" {
		var adaptationData struct {
			Results []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Category string `json:"category"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(latestAdaptationJSON), &adaptationData); err == nil {
			// Step 4 groups by category (first-seen order), then items within each category
			// We replicate that exact grouping order here
			categoryOrder := []string{}
			categoryItems := map[string][]string{}
			for _, r := range adaptationData.Results {
				if r.Status == "ignored" {
					continue // Step 4 excludes ignored rules
				}
				if _, seen := categoryItems[r.Category]; !seen {
					categoryOrder = append(categoryOrder, r.Category)
				}
				categoryItems[r.Category] = append(categoryItems[r.Category], r.ID)
			}
			for _, cat := range categoryOrder {
				adaptationOrderedIDs = append(adaptationOrderedIDs, categoryItems[cat]...)
			}
		}
	}

	// Fetch confirmed resources mapped from Step 4 (resource_combination)
	var latestResourceCombinationJSON string
	_ = h.db.Get(&latestResourceCombinationJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", id, "resource_combination")

	if latestResourceCombinationJSON != "" {
		var bindingsData map[string]interface{}
		if err := json.Unmarshal([]byte(latestResourceCombinationJSON), &bindingsData); err == nil {
			if data == nil {
				data = make(map[string]interface{})
			}

			// Inject requirement text and category into bindings
			if bindingsMap, ok := bindingsData["bindings"].(map[string]interface{}); ok {
				for k, v := range bindingsMap {
					if bindObj, isObj := v.(map[string]interface{}); isObj {
						if ruleData, exists := ruleMap[k]; exists {
							bindObj["requirement_text"] = ruleData.RequirementText
							bindObj["category"] = ruleData.Category
						}
					}
				}
			}

			data["step4_bindings"] = bindingsData["bindings"]
			data["step4_bindings_order"] = adaptationOrderedIDs
		}
	}

	// Fetch step5 chapter bindings if they exist
	var latestStep5BindingsJSON string
	_ = h.db.Get(&latestStep5BindingsJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", id, "step5_chapter_bindings")
	if latestStep5BindingsJSON != "" {
		var step5Data map[string]interface{}
		if err := json.Unmarshal([]byte(latestStep5BindingsJSON), &step5Data); err == nil {
			if data == nil {
				data = make(map[string]interface{})
			}
			if b, ok := step5Data["bindings"]; ok {
				data["chapter_bindings"] = b
			}
		}
	}

	response := gin.H{
		"status": status,
		"data":   data,
	}
	if lastErrorMessage != nil && strings.TrimSpace(*lastErrorMessage) != "" {
		response["last_error_message"] = *lastErrorMessage
	}
	if latestExportPath := h.latestStep6OutputPath(id); latestExportPath != "" {
		response["latest_export_path"] = latestExportPath
		response["latest_download_url"] = fmt.Sprintf("/api/bid-projects/%s/step6/download?file=%s", id, filepath.Base(latestExportPath))
	}

	c.JSON(http.StatusOK, response)
}

func readFirstProjectMarkdown(paths ...*string) ([]byte, bool) {
	for _, p := range paths {
		if p == nil || strings.TrimSpace(*p) == "" {
			continue
		}
		content, err := os.ReadFile(*p)
		if err != nil {
			// The path in DB is relative to project root, but Go server CWD may be backend_go/.
			content, err = os.ReadFile(filepath.Join("..", *p))
		}
		if err == nil && len(content) > 0 {
			return content, true
		}
	}
	return nil, false
}

func (h *BidProjectHandler) latestStep6OutputPath(projectID string) string {
	var filePaths []string
	err := h.db.Select(&filePaths, `SELECT file_path FROM bid_project_outputs
		WHERE project_id = ? AND output_type = 'commerce_word' AND status = 'available'
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return ""
	}
	for _, filePath := range filePaths {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}
		if _, statErr := os.Stat(filePath); statErr == nil {
			return filePath
		}
	}
	return ""
}

func (h *BidProjectHandler) GenerateStep6Payload(c *gin.Context) {
	id := c.Param("id")

	// Update DB to generating
	_, err := h.db.Exec("UPDATE bid_projects SET step6_status = 'generating', last_error_message = NULL, updated_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update db status")
		return
	}

	go func(projectID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in GenerateStep6Payload: %v", r)
				h.db.Exec("UPDATE bid_projects SET step6_status = 'error', last_error_message = ? WHERE id = ?", fmt.Sprintf("panic: %v", r), projectID)
			}
		}()

		payload, buildErr := h.exporterService.BuildWebFormPayload(ctx, projectID)

		if buildErr != nil {
			log.Printf("Step 6 BuildWebFormPayload failed for project %s: %v", projectID, buildErr)
			h.db.Exec("UPDATE bid_projects SET step6_status = 'error', last_error_message = ? WHERE id = ?", buildErr.Error(), projectID)
			return
		}

		payloadBytes, _ := json.Marshal(payload)
		_, pErr := h.db.Exec("UPDATE bid_projects SET step6_status = 'success', step6_payload_json = ?, last_error_message = NULL, updated_at = ? WHERE id = ?", string(payloadBytes), time.Now(), projectID)
		if pErr != nil {
			log.Printf("Failed to persist step 6 payload for project %s: %v", projectID, pErr)
			h.db.Exec("UPDATE bid_projects SET step6_status = 'error', last_error_message = ? WHERE id = ?", pErr.Error(), projectID)
		}
	}(id)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *BidProjectHandler) ExportFinalWord(c *gin.Context) {
	id := c.Param("id")

	var input struct {
		PayloadJSON string `json:"payload_json" binding:"required"`
		// normally we'd dynamically fetch the template based on projectID,
		// hardcoded template string for the demo/implementation prototype.
		TemplatePath string `json:"template_path"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tmpl := input.TemplatePath
	if tmpl == "" {
		// Try to fetch real template path from DB
		var templateMdPath *string
		err := h.db.QueryRow("SELECT template_markdown_path FROM bid_projects WHERE id = ?", id).Scan(&templateMdPath)
		if err == nil && templateMdPath != nil && *templateMdPath != "" {
			mdPath := *templateMdPath
			if strings.HasSuffix(mdPath, "_template_structured.md") {
				docxPath := strings.TrimSuffix(mdPath, "_template_structured.md") + ".docx"
				if _, statErr := os.Stat(docxPath); statErr == nil {
					tmpl = docxPath
				} else if _, statErr := os.Stat(filepath.Join("..", docxPath)); statErr == nil {
					tmpl = filepath.Join("..", docxPath)
				}
			}
		}

		if tmpl == "" {
			log.Printf("Step 6 template docx not found for project %s; using generated default template", id)
		}
	}

	exportPath, err := h.exporterService.ExecuteSafeWordExport(c.Request.Context(), input.PayloadJSON, tmpl)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to execute safe export: "+err.Error())
		return
	}
	if err := h.recordStep6Output(id, exportPath); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to record export output: "+err.Error())
		return
	}

	downloadURL := fmt.Sprintf("/api/bid-projects/%s/step6/download?file=%s", id, filepath.Base(exportPath))
	c.JSON(http.StatusOK, gin.H{"success": true, "export_path": exportPath, "download_url": downloadURL})
}

func (h *BidProjectHandler) DownloadStep6Word(c *gin.Context) {
	id := c.Param("id")
	fileName := c.Query("file")
	if fileName == "" || strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
		Error(c, http.StatusBadRequest, "Invalid file name")
		return
	}

	filePath, err := resolveStep6DownloadPath(id, fileName)
	if err != nil {
		Error(c, http.StatusNotFound, "File not found or expired")
		return
	}

	c.FileAttachment(filePath, "商务标合卷（纯净套打版）.docx")
}

func (h *BidProjectHandler) recordStep6Output(projectID string, exportPath string) error {
	var versionNo interface{}
	var activeVersionNo int
	if err := h.db.Get(&activeVersionNo, "SELECT active_version_no FROM bid_projects WHERE id = ?", projectID); err == nil {
		versionNo = activeVersionNo
	}

	now := time.Now()
	_, err := h.db.Exec(`INSERT INTO bid_project_outputs
		(id, project_id, version_no, output_type, file_name, file_path, mime_type, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		projectID,
		versionNo,
		"commerce_word",
		filepath.Base(exportPath),
		exportPath,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"available",
		now,
	)
	if err != nil {
		return err
	}
	_, err = h.db.Exec(`UPDATE bid_projects
		SET current_step = 'output_finalize',
			current_step_status = 'success',
			step6_status = 'success',
			last_error_message = NULL,
			updated_at = ?
		WHERE id = ?`, now, projectID)
	return err
}

func resolveStep6DownloadPath(projectID string, fileName string) (string, error) {
	if fileName == "" || strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
		return "", fmt.Errorf("invalid file name")
	}

	projectPath := filepath.Join(step6ExportRoot(), sanitizeStep6PathPart(projectID), fileName)
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath, nil
	}

	legacyTmpPath := filepath.Join(os.TempDir(), fileName)
	if _, err := os.Stat(legacyTmpPath); err == nil {
		return legacyTmpPath, nil
	}

	return "", os.ErrNotExist
}

func step6ExportRoot() string {
	if root := os.Getenv("BID_EXPORT_DIR"); root != "" {
		return root
	}
	return filepath.Join("data", "exports", "bid_projects")
}

func sanitizeStep6PathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

var STEP_ORDER = []string{
	"tender_detail_extract",
	"rule_parse",
	"company_adaptation",
	"resource_combination",
	"user_confirmation",
	"chapter_generation",
	"attachment_assembly",
	"risk_review",
	"output_finalize",
}

func (h *BidProjectHandler) ConfirmStep(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		ConfirmType string `json:"confirm_type"`
	}
	c.ShouldBindJSON(&input)

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	currentStep := ""
	if project.CurrentStep != nil {
		currentStep = *project.CurrentStep
	}

	nextStep := ""
	for i, name := range STEP_ORDER {
		if name == currentStep && i < len(STEP_ORDER)-1 {
			nextStep = STEP_ORDER[i+1]
			break
		}
	}

	if nextStep != "" {
		_, err = h.db.Exec("UPDATE bid_projects SET current_step = ?, current_step_status = 'waiting', project_status = 'waiting', last_confirm_type = ?, updated_at = ? WHERE id = ?",
			nextStep, input.ConfirmType, time.Now(), id)
	} else {
		_, err = h.db.Exec("UPDATE bid_projects SET project_status = 'success', updated_at = ? WHERE id = ?", time.Now(), id)
	}

	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *BidProjectHandler) GoBackStep(c *gin.Context) {
	id := c.Param("id")

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	currentStep := ""
	if project.CurrentStep != nil {
		currentStep = *project.CurrentStep
	}

	prevStep := ""
	for i, name := range STEP_ORDER {
		if name == currentStep && i > 0 {
			prevStep = STEP_ORDER[i-1]
			break
		}
	}

	if prevStep != "" {
		_, err = h.db.Exec("UPDATE bid_projects SET current_step = ?, current_step_status = 'waiting', updated_at = ? WHERE id = ?",
			prevStep, time.Now(), id)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *BidProjectHandler) UpdateRules(c *gin.Context) {
	id := c.Param("id")
	var rules interface{}
	if err := c.ShouldBindJSON(&rules); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	rulesJSON, _ := json.Marshal(rules)

	var stepID string
	err := h.db.Get(&stepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = 'rule_parse'", id)
	if err != nil {
		// Log but don't strictly enforce if step_id isn't tightly coupled in this mock
		stepID = "unknown_step"
	}

	// Save to bid_project_actions as 'extract_rules'
	query := `INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = h.db.Exec(query, uuid.New().String(), id, stepID, "extract_rules", "completed", string(rulesJSON), time.Now())
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *BidProjectHandler) GetActions(c *gin.Context) {
	id := c.Param("id")
	stepName := c.Query("step_name")
	status := c.Query("status")

	queryStr := "SELECT * FROM bid_project_actions WHERE project_id = ?"
	params := []interface{}{id}

	if stepName != "" {
		// Resolve step_id from step_name
		var resolvedStepID string
		_ = h.db.Get(&resolvedStepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = ?", id, stepName)

		if resolvedStepID != "" {
			if stepName == "rule_parse" {
				queryStr += " AND (step_id = ? OR action_name = ? OR action_name = 'extract_rules')"
			} else {
				queryStr += " AND (step_id = ? OR action_name = ?)"
			}
			params = append(params, resolvedStepID, stepName)
		} else {
			if stepName == "rule_parse" {
				queryStr += " AND action_name IN (?, 'extract_rules')"
				params = append(params, stepName)
			} else {
				queryStr += " AND action_name = ?"
				params = append(params, stepName)
			}
		}
	}
	if status != "" {
		queryStr += " AND action_status = ?"
		params = append(params, status)
	}

	queryStr += " ORDER BY created_at DESC"

	rows, err := h.db.Queryx(queryStr, params...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	actions := []map[string]interface{}{}
	for rows.Next() {
		results := make(map[string]interface{})
		err = rows.MapScan(results)
		if err == nil {
			// Convert byte slices to strings for JSON
			for k, v := range results {
				if b, ok := v.([]byte); ok {
					results[k] = string(b)
				}
			}
			actions = append(actions, results)
		}
	}
	c.JSON(200, actions)
}

func (h *BidProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	var input struct {
		ProjectName string `json:"project_name"`
		OwnerName   string `json:"owner_name"`
		TenderCode  string `json:"tender_code"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec("UPDATE bid_projects SET project_name = ?, owner_name = ?, tender_code = ?, updated_at = ? WHERE id = ? AND company_id = ?",
		input.ProjectName, input.OwnerName, input.TenderCode, time.Now(), id, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *BidProjectHandler) DeleteProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	tx, err := h.db.Begin()
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	// 1. Delete associated data (cascade is safer)
	tx.Exec("DELETE FROM bid_project_steps WHERE project_id = ?", id)
	tx.Exec("DELETE FROM bid_project_files WHERE project_id = ?", id)
	tx.Exec("DELETE FROM bid_project_actions WHERE project_id = ?", id)
	tx.Exec("DELETE FROM bid_project_versions WHERE project_id = ?", id)

	// 2. Delete project
	_, err = tx.Exec("DELETE FROM bid_projects WHERE id = ? AND company_id = ?", id, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *BidProjectHandler) SaveResourceCombination(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Bindings map[string]interface{} `json:"bindings"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid input data")
		return
	}

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// 1. Get Step ID
	var stepID string
	_ = h.db.Get(&stepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = ?", id, "resource_combination")

	// 2. Save actions payload
	actionID := uuid.New().String()
	payloadBytes, _ := json.Marshal(input)
	now := time.Now()
	_, err = h.db.Exec(`INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at, finished_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		actionID, id, stepID, "resource_combination", "success", string(payloadBytes), now, now)

	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to save combination mapping")
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// SaveStep5Bindings handles saving the mapped bindings from Step 5 to the database
func (h *BidProjectHandler) SaveStep5Bindings(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Bindings map[string]interface{} `json:"bindings"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid input data")
		return
	}

	var project model.BidProject
	err := h.db.Get(&project, "SELECT * FROM bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// 1. Get Step ID (which might be generation or resource_combination, fallback to any active step)
	var stepID string
	_ = h.db.Get(&stepID, "SELECT id FROM bid_project_steps WHERE project_id = ? AND step_name = ?", id, "generation")

	// 2. Save actions payload
	actionID := uuid.New().String()
	payloadBytes, _ := json.Marshal(input)
	now := time.Now()
	_, err = h.db.Exec(`INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at, finished_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		actionID, id, stepID, "step5_chapter_bindings", "success", string(payloadBytes), now, now)

	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to save chapter bindings")
		return
	}

	c.JSON(200, gin.H{"success": true})
}
func (h *BidProjectHandler) UpdateCompanyAdaptation(c *gin.Context) {
	id := c.Param("id")
	var payload interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payloadJSON, _ := json.Marshal(payload)

	query := `INSERT INTO bid_project_actions (id, project_id, step_id, action_name, action_status, result_json, created_at)
              SELECT ?, ?, id, ?, ?, ?, ? FROM bid_project_steps WHERE project_id = ? AND step_name = ?`
	_, err := h.db.Exec(query, uuid.New().String(), id, "company_adaptation", "success", string(payloadJSON), time.Now(), id, "company_adaptation")
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}
func (h *BidProjectHandler) UploadStep6Template(c *gin.Context) {
	id := c.Param("id")

	file, err := c.FormFile("file")
	if err != nil {
		Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	uploadDir := "data/files/incoming/"
	os.MkdirAll(uploadDir, 0755)

	ext := filepath.Ext(file.Filename)
	fileID := uuid.New().String()
	saveName := fileID + ext
	savePath := filepath.Join(uploadDir, saveName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to save template file")
		return
	}
	absPath, _ := filepath.Abs(savePath)

	markdownContent := ""

	if strings.ToLower(ext) == ".docx" || strings.ToLower(ext) == ".pdf" {
		markdownContent, _, err = h.docmindService.ParseLocalFile(absPath, file.Filename)
		if err != nil {
			log.Printf("Failed to ParseLocalFile for %s step 6 template: %v", id, err)
			Error(c, http.StatusInternalServerError, "Failed to extract text from Word template: "+err.Error())
			return
		}
	} else {
		contentBytes, err := os.ReadFile(absPath)
		if err == nil {
			markdownContent = string(contentBytes)
		}
	}

	templateMdPath := filepath.Join(uploadDir, fileID+"_template_structured.md")
	os.WriteFile(templateMdPath, []byte(markdownContent), 0644)

	// Save to DB and reset step6 status
	_, err = h.db.Exec(`UPDATE bid_projects 
		SET template_markdown_path = ?, step6_status = 'idle', step6_payload_json = NULL, last_error_message = NULL, updated_at = ? 
		WHERE id = ?`,
		templateMdPath, time.Now(), id)

	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update project template path")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RegenerateSlot re-runs the AI extraction specifically for a single slot in step 6
func (h *BidProjectHandler) RegenerateSlot(c *gin.Context) {
	id := c.Param("id")
	slotID := c.Param("slot_id")

	var payloadJSON string
	err := h.db.QueryRow("SELECT step6_payload_json FROM bid_projects WHERE id = ?", id).Scan(&payloadJSON)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to load project Step 6 payload data")
		return
	}

	var payload agent.BidActionList
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to parse Step 6 payload")
		return
	}

	// Find the targeted slot
	var targetSlot *agent.BidActionSlot
	var targetIndex = -1
	for i, s := range payload.Slots {
		if s.SlotID == slotID {
			targetSlot = &payload.Slots[i]
			targetIndex = i
			break
		}
	}

	if targetSlot == nil {
		Error(c, http.StatusNotFound, "Slot not found in payload")
		return
	}

	fillPayload := &agent.FillPayload{
		ProjectID:    id,
		Chapter:      "",
		MarkdownText: "",
		Slots:        []agent.BidActionSlot{*targetSlot},
	}

	// Invoke the filler ONLY for this single slot
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resultList, err := agent.RunSlotFiller(ctx, fillPayload, h.db)
	regeneratedSlot, ok := resolveRegeneratedSlot(*targetSlot, resultList, err)
	if !ok {
		Error(c, http.StatusInternalServerError, "Failed to regenerate data for this slot")
		return
	}

	// Update the slot with newly generated value
	payload.Slots[targetIndex] = regeneratedSlot

	newPayloadBytes, _ := json.Marshal(payload)
	_, err = h.db.Exec("UPDATE bid_projects SET step6_payload_json = ? WHERE id = ?", string(newPayloadBytes), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to persist regenerated payload")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"new_value": regeneratedSlot.AISuggestedValue,
		"fallback":  err != nil || len(resultList.Slots) == 0,
	})
}

func resolveRegeneratedSlot(existing agent.BidActionSlot, result agent.BidActionList, runErr error) (agent.BidActionSlot, bool) {
	if runErr == nil && len(result.Slots) > 0 && strings.TrimSpace(result.Slots[0].AISuggestedValue) != "" {
		next := existing
		next.AISuggestedValue = result.Slots[0].AISuggestedValue
		next.Reason = result.Slots[0].Reason
		next.Status = result.Slots[0].Status
		if next.Status == "" {
			next.Status = agent.StatusApproved
		}
		return next, true
	}

	if strings.TrimSpace(existing.AISuggestedValue) == "" {
		return agent.BidActionSlot{}, false
	}

	fallback := existing
	fallback.Status = agent.StatusApproved
	if strings.TrimSpace(fallback.Reason) == "" {
		fallback.Reason = "AI 重新生成当前不可用，已沿用已确认的装配内容。"
	} else if !strings.Contains(fallback.Reason, "AI 重新生成当前不可用") {
		fallback.Reason = fallback.Reason + "\n\nAI 重新生成当前不可用，已沿用已确认的装配内容。"
	}
	return fallback, true
}
