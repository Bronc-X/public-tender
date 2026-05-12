package handler

import (
	"backend_go/internal/agent"
	"backend_go/internal/db"
	"backend_go/internal/model"
	"backend_go/internal/service"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

type TechBidProjectHandler struct {
	db              *sqlx.DB
	digitizeService *service.TenderDigitizationService
}

func NewTechBidProjectHandler(db *sqlx.DB, digitizeService *service.TenderDigitizationService) *TechBidProjectHandler {
	return &TechBidProjectHandler{db: db, digitizeService: digitizeService}
}

// logStep4Approval writes step4_approval_logs when active_step4_run_id is set (Coordinator run).
func (h *TechBidProjectHandler) logStep4Approval(projectID, stage, action, operatorID, reason string) {
	var runID sql.NullInt64
	if err := h.db.Get(&runID, `SELECT active_step4_run_id FROM tech_bid_projects WHERE id = ?`, projectID); err != nil || !runID.Valid || runID.Int64 == 0 {
		return
	}
	store := service.NewStep4Store(h.db)
	_ = store.LogApproval(runID.Int64, projectID, stage, action, operatorID, reason)
}

func (h *TechBidProjectHandler) loadTenderContentForStep4(projectID string) (string, error) {
	var row struct {
		FileContentMarkdown sql.NullString `db:"fc_markdown_text"`
		FileContentPlain    sql.NullString `db:"fc_plain_text"`
		FileAssetMarkdown   sql.NullString `db:"fa_markdown_text"`
		FileAssetPlain      sql.NullString `db:"fa_plain_text"`
		TenderRawText       sql.NullString `db:"tf_raw_text"`
	}
	err := h.db.Get(&row, `
		SELECT
			fc.markdown_text AS fc_markdown_text,
			fc.plain_text AS fc_plain_text,
			fa.markdown_text AS fa_markdown_text,
			fa.plain_text AS fa_plain_text,
			tf.raw_text AS tf_raw_text
		FROM tech_bid_tender_files tf
		LEFT JOIN file_asset fa ON fa.id = tf.file_asset_id
		LEFT JOIN file_content fc ON fc.file_asset_id = tf.file_asset_id
		WHERE tf.project_id = ? AND tf.file_role = 'tender'
		ORDER BY fc.created_at DESC
		LIMIT 1`, projectID)
	if err != nil {
		return "", fmt.Errorf("招标文件缺失，请先上传并解析招标文件")
	}

	candidates := []sql.NullString{
		row.FileContentMarkdown,
		row.FileContentPlain,
		row.FileAssetMarkdown,
		row.FileAssetPlain,
		row.TenderRawText,
	}
	for _, candidate := range candidates {
		if candidate.Valid && strings.TrimSpace(candidate.String) != "" {
			return candidate.String, nil
		}
	}
	return "", fmt.Errorf("招标文件正文为空，请先完成文件解析后再生成技术标目录")
}

// updateProjectState is a unified helper to update project status and log the transition
func (h *TechBidProjectHandler) updateProjectState(projectID string, fromStep, toStep *string, fromStatus, toStatus string, transitionType string, operatorID *string, operatorName string, verificationMethod string, reason string, metadata interface{}) error {
	now := time.Now()

	// 1. Update the project table
	toStepVal := ""
	if toStep != nil {
		toStepVal = *toStep
	} else {
		// Preserve current step when toStep is nil (background process not changing step)
		var existing string
		_ = h.db.Get(&existing, "SELECT current_step FROM tech_bid_projects WHERE id = ?", projectID)
		if existing == "" {
			existing = "tender_parse"
		}
		toStepVal = existing
	}

	query := `UPDATE tech_bid_projects SET
		current_step = ?,
		current_step_status = ?,
		project_status = ?,
		verification_method = ?,
		last_error_message = ?,
		updated_at = ?
		WHERE id = ?`

	// Map status to project_status (granular mapping)
	projectStatus := "waiting"
	switch toStatus {
	case "running":
		projectStatus = "running"
	case "failed", "audit_failed", "schema_validation_failed", "verification_degraded":
		projectStatus = "failed"
	case "success":
		projectStatus = "success"
	}

	// Try to get progress from metadata if available
	progress := -1
	if metadata != nil {
		if m, ok := metadata.(map[string]interface{}); ok {
			if p, ok := m["progress"].(int); ok {
				progress = p
			} else if pf, ok := m["progress"].(float64); ok {
				progress = int(pf)
			}
		}
	}

	var err error
	if progress >= 0 {
		query = `UPDATE tech_bid_projects SET
			current_step = ?,
			current_step_status = ?,
			project_status = ?,
			verification_method = ?,
			last_error_message = ?,
			current_progress = ?,
			updated_at = ?
			WHERE id = ?`
		_, err = h.db.Exec(query, toStepVal, toStatus, projectStatus, verificationMethod, reason, progress, now, projectID)
	} else {
		_, err = h.db.Exec(query, toStepVal, toStatus, projectStatus, verificationMethod, reason, now, projectID)
	}
	if err != nil {
		log.Printf("[StateUpdate] Update table failed: %v", err)
		return fmt.Errorf("failed to update project status: %v", err)
	}

	// 2. Log the transition
	metadataJSON := "{}"
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		metadataJSON = string(b)
	}

	_, err = h.db.Exec(`INSERT INTO tech_bid_state_transitions 
		(project_id, from_step, to_step, from_status, to_status, trigger_type, verification_method, operator_id, operator_name, trigger_reason, metadata_json, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, fromStep, toStep, fromStatus, toStatus, transitionType, verificationMethod, operatorID, operatorName, reason, metadataJSON, now)

	if err != nil {
		log.Printf("[StateUpdate] Warning: Failed to log transition for %s: %v", projectID, err)
	}

	return nil
}

func (h *TechBidProjectHandler) ListProjects(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	projects := []model.TechBidProject{}
	err := h.db.Unsafe().Select(&projects, "SELECT * FROM tech_bid_projects WHERE company_id = ? ORDER BY created_at DESC", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, projects)
}

func (h *TechBidProjectHandler) GetProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	var project model.TechBidProject
	err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	h.healStructurePlanState(id, &project)

	plans := []model.TechBidChapterPlan{}
	h.db.Unsafe().Select(&plans, "SELECT * FROM tech_bid_chapter_plans WHERE project_id = ? ORDER BY chapter_order ASC", id)

	files := []model.BidProjectFile{}
	err = h.db.Unsafe().Select(&files, `
		SELECT tbf.*, ? as company_id, fa.parse_status, fa.id as asset_id
		FROM tech_bid_tender_files tbf
		LEFT JOIN file_asset fa ON tbf.file_asset_id = fa.id
		WHERE tbf.project_id = ?`, companyID, id)
	if err != nil {
		log.Printf("[TechBid] Error selecting files: %v", err)
	}

	var latestVersion model.BidProjectVersion
	vErr := h.db.Get(&latestVersion, "SELECT * FROM tech_bid_versions WHERE project_id = ? ORDER BY version_no DESC LIMIT 1", id)

	var sharedTender model.SharedTender
	if project.SharedTenderID != nil {
		h.db.Get(&sharedTender, "SELECT * FROM shared_tenders WHERE id = ?", *project.SharedTenderID)
	}

	var profile model.TechBidProjectProfile
	pErr := h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)

	var facts []model.TechBidOutlineFact
	h.db.Select(&facts, "SELECT * FROM tech_bid_outline_facts WHERE project_id = ?", id)

	var latestAudit model.TechBidOutlineAudit
	aErr := h.db.Get(&latestAudit, "SELECT * FROM tech_bid_outline_audits WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)

	db.EnsureTechBidOutlineVerificationsSchema(h.db)
	var latestVerification model.TechBidOutlineVerification
	v5Err := h.db.Get(&latestVerification, "SELECT * FROM tech_bid_outline_verifications WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)

	detail := model.TechBidProjectDetail{
		TechBidProject: project,
		ChapterPlans:   plans,
		Files:          files,
		Facts:          facts,
	}

	if vErr == nil {
		detail.LatestVersion = &latestVersion
	}
	if project.SharedTenderID != nil {
		detail.SharedTender = &sharedTender
	}
	if pErr == nil {
		detail.Profile = &profile
	}
	if aErr == nil {
		detail.LatestAudit = &latestAudit
	}
	if v5Err == nil {
		detail.LatestVerification = &latestVerification
	}

	c.JSON(200, detail)
}

func (h *TechBidProjectHandler) CreateProject(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		ProjectName string `json:"project_name" binding:"required"`
		TenderCode  string `json:"tender_code"`
		ProjectType string `json:"project_type"`
		Profession  string `json:"profession"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	query := `INSERT INTO tech_bid_projects (id, company_id, project_name, tender_code, project_type, profession, project_status, current_step) 
              VALUES (?, ?, ?, ?, ?, ?, 'created', 'tender_parse')`
	_, err := h.db.Exec(query, id, companyID, input.ProjectName, input.TenderCode, input.ProjectType, input.Profession)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(201, gin.H{"id": id})
}

func (h *TechBidProjectHandler) RunStep(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var project model.TechBidProject
	err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Normalize: empty current_step defaults to tender_parse
	currentStep := derefString(project.CurrentStep)
	if currentStep == "" {
		currentStep = "tender_parse"
	}

	// 1. Mark status as running using audited helper
	err = h.updateProjectState(id, &currentStep, &currentStep, derefString(project.CurrentStepStatus), "running", "user", &cid, "", "", "用户启动处理步", nil)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 2. Trigger background process if in tender_parse step
	if currentStep == "tender_parse" {
		go h.processTenderExtraction(id, cid)
	} else {
		// Mock other steps for now
		go func() {
			time.Sleep(3 * time.Second)
			h.db.Exec("UPDATE tech_bid_projects SET project_status = 'success', current_step_status = 'success', updated_at = ? WHERE id = ?", time.Now(), id)
		}()
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *TechBidProjectHandler) persistProfileExtractionSnapshot(projectID, profileID, stage string, chunkIndex int, payload interface{}, runID string, fileID string) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[TechBid] Snapshot marshal failed (%s): %v", stage, err)
		return
	}
	if _, err := h.db.Exec(`
		INSERT INTO tech_bid_profile_extraction_snapshots (id, project_id, profile_id, stage, chunk_index, payload_json, run_id, file_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), projectID, profileID, stage, chunkIndex, string(payloadJSON), runID, fileID, time.Now()); err != nil {
		log.Printf("[TechBid] Snapshot save failed (%s): %v", stage, err)
	}
}

func (h *TechBidProjectHandler) processTenderExtraction(projectID, companyID string) {
	log.Printf("[TechBid] Starting background extraction for project: %s", projectID)

	// 1. Get the tender file
	var tenderFile struct {
		FileAssetID string `db:"file_asset_id"`
	}
	// Preserve current step across all state updates in this goroutine
	tenderParse := "tender_parse"

	err := h.db.Get(&tenderFile, "SELECT file_asset_id FROM tech_bid_tender_files WHERE project_id = ? AND file_role = 'tender' LIMIT 1", projectID)
	if err != nil {
		log.Printf("[TechBid] No tender file found for project %s", projectID)
		h.updateProjectState(projectID, &tenderParse, &tenderParse, "running", "warning", "system", nil, "", "", "未找到关联的招标文件", nil)
		return
	}

	// 2. Get the content (Markdown)
	var content struct {
		MarkdownText *string `db:"markdown_text"`
	}
	err = h.db.Get(&content, `
		SELECT markdown_text FROM file_content 
		WHERE file_asset_id = ? 
		ORDER BY created_at DESC LIMIT 1`, tenderFile.FileAssetID)

	if err != nil || content.MarkdownText == nil || *content.MarkdownText == "" {
		log.Printf("[TechBid] No parsed content found for file %s", tenderFile.FileAssetID)
		h.updateProjectState(projectID, &tenderParse, &tenderParse, "running", "warning", "system", nil, "", "", "招标文件尚未完成解析", nil)
		return
	}

	// 3. Call AI digitization
	results, err := h.digitizeService.DigitizeTenderFile(context.Background(), tenderFile.FileAssetID, *content.MarkdownText, "tech", func(percent int) {
		h.db.Exec("UPDATE tech_bid_projects SET current_progress = ?, updated_at = ? WHERE id = ?", percent, time.Now(), projectID)
	})
	if err != nil {
		log.Printf("[TechBid] AI Extraction failed: %v", err)
		h.updateProjectState(projectID, &tenderParse, &tenderParse, "running", "failed", "ai", nil, "", "ai", "标书画像提取失败: "+err.Error(), nil)
		return
	}

	// 4. Persist snapshots and save typed profile result
	profileID := uuid.New().String()
	runID := uuid.New().String()
	fileID := tenderFile.FileAssetID
	h.persistProfileExtractionSnapshot(projectID, profileID, "raw_text", 0, map[string]interface{}{
		"file_asset_id": tenderFile.FileAssetID,
		"content":       *content.MarkdownText,
	}, runID, fileID)
	h.persistProfileExtractionSnapshot(projectID, profileID, "chunks", 0, results.Chunks, runID, fileID)
	for idx, chunkOutput := range results.ChunkOutputs {
		chunkType := "narrative"
		if idx < len(results.ChunkTypes) {
			chunkType = results.ChunkTypes[idx]
		}
		h.persistProfileExtractionSnapshot(projectID, profileID, "chunk_output", idx, map[string]interface{}{
			"chunk_text":   results.Chunks[idx],
			"chunk_type":   chunkType,
			"raw_output":   chunkOutput,
			"normalized":   results.ChunkResults[idx],
			"processed_at": results.ProcessedAt,
		}, runID, fileID)
	}
	// Save prompt inputs if available
	for idx, promptInput := range results.PromptInputs {
		if promptInput != "" {
			h.persistProfileExtractionSnapshot(projectID, profileID, "prompt_input", idx, map[string]interface{}{
				"prompt": promptInput,
			}, runID, fileID)
		}
	}
	h.persistProfileExtractionSnapshot(projectID, profileID, "merge_after", 0, results.MergedProfile, runID, fileID)
	if len(results.MergeDiffs) > 0 {
		h.persistProfileExtractionSnapshot(projectID, profileID, "merge_diff", 0, results.MergeDiffs, runID, fileID)
	}

	// 5. Supplementary extraction for high-risk fields
	supplementCtx, supplementCancel := context.WithTimeout(context.Background(), 300*time.Second)
	supplementResult, supplementErr := h.digitizeService.SupplementaryProfileExtraction(supplementCtx, fileID, *content.MarkdownText, &results.MergedProfile)
	supplementCancel()
	if supplementErr != nil {
		log.Printf("[TechBid] Supplementary extraction warning: %v (continuing with original profile)", supplementErr)
	} else {
		// Save supplement snapshots
		h.persistProfileExtractionSnapshot(projectID, profileID, "supplement_check", 0, map[string]interface{}{
			"categories_checked":   supplementResult.CategoriesChecked,
			"categories_extracted": supplementResult.CategoriesExtracted,
		}, runID, fileID)
		for key, rawOutput := range supplementResult.RawOutputs {
			h.persistProfileExtractionSnapshot(projectID, profileID, "supplement_output_"+key, 0, map[string]interface{}{
				"category":   key,
				"raw_output": rawOutput,
			}, runID, fileID)
		}
		h.persistProfileExtractionSnapshot(projectID, profileID, "supplement_merged", 0, supplementResult.UpdatedProfile, runID, fileID)
		results.MergedProfile = supplementResult.UpdatedProfile
	}

	// 6. Keyword audit finalize with raw text
	h.digitizeService.FinalizeWithKeywordAudit(&results.MergedProfile, *content.MarkdownText)

	extractionMeta := h.digitizeService.BuildProjectProfileExtractionMeta(results.MergedProfile)
	if results.DetectedIndustry != "" {
		extractionMeta["detected_industry"] = results.DetectedIndustry
	}
	if supplementErr == nil && supplementResult != nil {
		extractionMeta["supplement_applied"] = true
		extractionMeta["supplement_categories_checked"] = supplementResult.CategoriesChecked
		extractionMeta["supplement_categories_extracted"] = supplementResult.CategoriesExtracted
	} else {
		extractionMeta["supplement_applied"] = false
	}

	finalPayload := map[string]interface{}{
		"schema_version":  "v2",
		"processed_at":    results.ProcessedAt,
		"profile":         results.MergedProfile,
		"extraction_meta": extractionMeta,
		"views":           service.BuildProfileViews(results.MergedProfile),
	}
	h.persistProfileExtractionSnapshot(projectID, profileID, "final_payload", 0, finalPayload, runID, fileID)

	resultsJSON, _ := json.Marshal(results.MergedProfile)
	extractionMetaJSON, _ := json.Marshal(extractionMeta)

	_, err = h.db.Exec(`
		INSERT INTO tech_bid_project_profiles (id, project_id, profile_json, summary_text, schema_version, extraction_meta_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		profileID, projectID, string(resultsJSON), "AI 专家已根据招标文件完成多维度画像提取", "v2", string(extractionMetaJSON))

	if err != nil {
		log.Printf("[TechBid] Save profile failed: %v", err)
	}

	// 6. Mark step as success
	h.updateProjectState(projectID, &tenderParse, &tenderParse, "running", "success", "system", nil, "", "ai", "标书画像提取成功", nil)
	log.Printf("[TechBid] Project extraction success: %s", projectID)
}

func (h *TechBidProjectHandler) mergeProjectProfiles(chunks []string) map[string]interface{} {
	merged := make(map[string]interface{})
	dimensions := []string{
		"project_base_info",
		"construction_core_requirements",
		"difficulty_and_focus",
		"bidder_requirements",
		"evaluation_and_performance_rules",
		"other_technical_requirements",
	}

	// Initialize dimensions in merged map
	for _, dim := range dimensions {
		merged[dim] = make(map[string]interface{})
	}

	for _, chunkStr := range chunks {
		var chunkMap map[string]interface{}
		// Basic clean up of markdown code blocks if any
		cleanStr := chunkStr
		startIdx := -1
		endIdx := -1

		for i := 0; i < len(chunkStr); i++ {
			if chunkStr[i] == '{' {
				startIdx = i
				break
			}
		}
		for i := len(chunkStr) - 1; i >= 0; i-- {
			if chunkStr[i] == '}' {
				endIdx = i
				break
			}
		}

		if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			cleanStr = chunkStr[startIdx : endIdx+1]
		}

		if err := json.Unmarshal([]byte(cleanStr), &chunkMap); err != nil {
			log.Printf("[TechBid] Partial JSON unmarshal failed: %v", err)
			continue
		}

		// Merge each dimension
		for _, dim := range dimensions {
			if data, ok := chunkMap[dim].(map[string]interface{}); ok {
				targetDim := merged[dim].(map[string]interface{})
				for k, v := range data {
					existing, exists := targetDim[k]
					if !exists || existing == "" || existing == "无" || existing == nil {
						targetDim[k] = v
					} else {
						// Strategies for merging:
						// If both are arrays, concat
						// If both are strings, check if they are the same
						switch val := v.(type) {
						case []interface{}:
							if existingArr, ok := existing.([]interface{}); ok {
								// Simple append and deduplicate (dedup skipped for now)
								targetDim[k] = append(existingArr, val...)
							}
						case string:
							if existingStr, ok := existing.(string); ok {
								if existingStr == "" || existingStr == "无" || len(val) > len(existingStr) {
									targetDim[k] = val
								}
							}
						}
					}
				}
			}
		}
	}
	return merged
}

func (h *TechBidProjectHandler) ConfirmStep(c *gin.Context) {
	id := c.Param("id")

	var project model.TechBidProject
	err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Normalize: empty current_step defaults to tender_parse
	currentStep := derefString(project.CurrentStep)
	if currentStep == "" {
		currentStep = "tender_parse"
	}

	nextStep := ""
	switch currentStep {
	case "tender_parse":
		nextStep = "project_profile"
	case "project_profile":
		nextStep = "route_planning"
	case "route_planning":
		nextStep = "outline_generation"
	case "outline_generation":
		nextStep = "outline_verification"
	case "outline_verification":
		nextStep = "content_generation"
	case "content_generation":
		nextStep = "risk_review"
	case "risk_review":
		nextStep = "output_finalize"
	}

	// Guard: Step 4 → 终审（完全响应硬门槛 + 合并决策必须为 PASS；Step5 强制放行 / Step4 专用放行 除外）
	if currentStep == "outline_generation" {
		if project.OverrideEnabled == 0 && project.Step4OverrideEnabled == 0 {
			if project.FinalDecision != nil && *project.FinalDecision != "PASS" {
				Error(c, http.StatusForbidden, "目录未通过 Step4 自检（含完全响应硬门槛与审计）。请重试目录生成、优化或申请 Step4 放行 / 强制放行。")
				return
			}
		}
	}

	// Guard: Step 5 Experts Gate Loop Enforcement (P0-2)
	if currentStep == "outline_verification" {
		if project.FinalDecision != nil && (*project.FinalDecision == "BLOCK" || *project.FinalDecision == "REVISE") {
			if project.OverrideEnabled == 0 {
				Error(c, http.StatusForbidden, "标书核验未通过 (REVISE/BLOCK)，请先进行修正并重审，或申请强制放行。")
				return
			}
		}
	}
	if currentStep == "content_generation" {
		if _, err := syncTechStep5StatusIfComplete(h.db, id); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	err = h.updateProjectState(id, &currentStep, &nextStep, derefString(project.CurrentStepStatus), "waiting", "user", nil, "", "", "用户确认通过，进入下一步", nil)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *TechBidProjectHandler) GoBackStep(c *gin.Context) {
	id := c.Param("id")

	var project model.TechBidProject
	err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Normalize: empty current_step defaults to tender_parse
	currentStep := derefString(project.CurrentStep)
	if currentStep == "" {
		currentStep = "tender_parse"
	}

	prevStep := ""
	switch currentStep {
	case "project_profile":
		prevStep = "tender_parse"
	case "route_planning":
		prevStep = "project_profile"
	case "outline_generation":
		prevStep = "route_planning"
	case "outline_verification":
		prevStep = "outline_generation"
	case "content_generation":
		prevStep = "outline_verification"
	case "risk_review":
		prevStep = "content_generation"
	case "output_finalize":
		prevStep = "risk_review"
	}

	if prevStep != "" {
		err = h.updateProjectState(id, project.CurrentStep, &prevStep, derefString(project.CurrentStepStatus), "waiting", "user", nil, "", "", "用户回退步骤", nil)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *TechBidProjectHandler) SelectRoute(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[TechBid] SelectRoute called for project: %s", id)

	var input struct {
		RouteID  string   `json:"routeId"`
		Chapters []string `json:"chapters"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[TechBid] SelectRoute bind JSON failed: %v", err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[TechBid] Route selected: %s with %d chapters", input.RouteID, len(input.Chapters))

	// --- PHASE 1: Initial Cleanup and Status Update (Short Transaction) ---
	tx1 := h.db.MustBegin()
	_, _ = tx1.Exec("DELETE FROM tech_bid_chapter_contents WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_chapter_plans WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_outline_facts WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_fact_mappings WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_outline_coverage_checks WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_requirement_register WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_requirement_response_checks WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_step4_gate_overrides WHERE project_id = ?", id)

	// Update project and set current_step to outline_generation, status to 'running'
	_, err := tx1.Exec(`UPDATE tech_bid_projects SET current_step = 'outline_generation', current_step_status = 'running', project_status = 'waiting', step4_status = 'idle', last_error_message = NULL, step4_override_enabled = 0, step4_override_reason = NULL, step4_override_by = NULL, step4_override_at = NULL, history_similarity_hint = NULL, outline_fingerprint = NULL, outline_titles_json = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		tx1.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx1.Commit(); err != nil {
		log.Printf("[TechBid] Initial commit failed: %v", err)
		Error(c, http.StatusInternalServerError, "Database update failed")
		return
	}
	// --- DB LOCK RELEASED ---

	var project model.TechBidProject
	_ = h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)

	var profile model.TechBidProjectProfile
	_ = h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
	if profile.ProfileJSON == nil || *profile.ProfileJSON == "" || *profile.ProfileJSON == "{}" {
		Error(c, http.StatusBadRequest, "项目画像 (Step 1-3) 缺失，请先运行或确认标书解析。")
		return
	}
	profileData := *profile.ProfileJSON

	tenderContent, tenderErr := h.loadTenderContentForStep4(id)
	if tenderErr != nil {
		Error(c, http.StatusBadRequest, tenderErr.Error())
		return
	}

	// Frontend (TechBidProjectWorkbench) uses std_conv / green_std / smart_site; legacy r1–r3 kept for old clients.
	routeName := "常规施工组织路线"
	switch input.RouteID {
	case "std_conv":
		routeName = "常规施工组织路线"
	case "green_std":
		routeName = "绿色低碳示范路线"
	case "smart_site":
		routeName = "智慧工地数字化路线"
	case "r1":
		routeName = "稳健型施工路线"
	case "r2":
		routeName = "竞争型施工路线"
	case "r3":
		routeName = "精专型（智慧工地）路线"
	}

	companyIDAny, _ := c.Get("companyID")
	cid, _ := companyIDAny.(string)

	_ = h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)
	store := service.NewStep4Store(h.db)
	runID, runErr := store.CreateRun(id, "route_select", cid, "running", "requirements_extracting")
	if runErr != nil {
		log.Printf("[TechBid] CreateRun failed: %v", runErr)
		Error(c, http.StatusInternalServerError, runErr.Error())
		return
	}
	professionStr := ""
	if project.Profession != nil {
		professionStr = *project.Profession
	}
	projectTypeStr := ""
	if project.ProjectType != nil {
		projectTypeStr = *project.ProjectType
	}
	outlineStep := "outline_generation"

	// 读取用户确认的骨架（优先使用）
	selectedSkeletonID := ""
	var skeletonSelection struct {
		SkeletonID   string `db:"skeleton_id"`
		SkeletonName string `db:"skeleton_name"`
	}
	if err := h.db.Get(&skeletonSelection, `SELECT skeleton_id, skeleton_name FROM tech_bid_skeleton_selections WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id); err == nil {
		selectedSkeletonID = skeletonSelection.SkeletonID
		log.Printf("[TechBid] SelectRoute: Found user-confirmed skeleton: %s (%s)", skeletonSelection.SkeletonName, selectedSkeletonID)
	}

	// 确定使用 skeleton 模式（当有用户确认的骨架时）或 direct 模式
	outlineGenMode := "direct"
	if selectedSkeletonID != "" {
		outlineGenMode = "skeleton"
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		service.RunOutlinePipeline(&service.OutlinePipelineInput{
			Ctx:                   ctx,
			DB:                    h.db,
			Store:                 store,
			Digitize:              h.digitizeService,
			RunID:                 runID,
			ProjectID:             id,
			CompanyID:             cid,
			RouteName:             routeName,
			ProfileData:           profileData,
			TenderContent:         tenderContent,
			ProfessionStr:         professionStr,
			ProjectTypeStr:        projectTypeStr,
			ProjectStep:           &outlineStep,
			SelectedSkeletonID:    selectedSkeletonID,
			OutlineGenerationMode: outlineGenMode, // 使用 skeleton 模式当有用户确认骨架时
			Hooks: service.OutlinePipelineHooks{
				UpdateProjectState:         h.updateProjectState,
				UpsertPendingStructurePlan: h.upsertPendingStructurePlan,
			},
		})
	}()

	c.JSON(200, gin.H{"success": true, "status": "running", "data": gin.H{"run_id": runID}})
}

// PostOutlineRun matches CTO POST /outline/run; body same as SelectRoute (routeId, chapters).
func (h *TechBidProjectHandler) PostOutlineRun(c *gin.Context) {
	h.SelectRoute(c)
}

// PostOutlineRegenerate handles "重新生成目录" button click.
// Unlike SelectRoute, this endpoint does NOT require routeId parameter.
// It uses the route from the latest successful run or falls back to a default route.
func (h *TechBidProjectHandler) PostOutlineRegenerate(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[TechBid] PostOutlineRegenerate called for project: %s", id)

	companyIDAny, _ := c.Get("companyID")
	cid, _ := companyIDAny.(string)

	// Get current project
	var project model.TechBidProject
	_ = h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)

	// Get profile data
	var profile model.TechBidProjectProfile
	_ = h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
	if profile.ProfileJSON == nil || *profile.ProfileJSON == "" || *profile.ProfileJSON == "{}" {
		Error(c, http.StatusBadRequest, "项目画像 (Step 1-3) 缺失，请先运行或确认标书解析。")
		return
	}
	profileData := *profile.ProfileJSON

	// Get tender content
	tenderContent, tenderErr := h.loadTenderContentForStep4(id)
	if tenderErr != nil {
		Error(c, http.StatusBadRequest, tenderErr.Error())
		return
	}

	// Get route name from latest successful run's metadata, or use default
	routeName := "常规施工组织路线" // Default route for regeneration

	// Try to get route info from latest run's agent_runs metadata
	store := service.NewStep4Store(h.db)
	var lastRun struct {
		ID            int64  `db:"id"`
		TriggerSource string `db:"trigger_source"`
	}
	if err := h.db.Get(&lastRun, `SELECT id, trigger_source FROM step4_runs WHERE project_id = ? ORDER BY id DESC LIMIT 1`, id); err == nil {
		// Extract route name from trigger source if available
		// trigger_source format: "route_select:green_std" or just "route_select"
		if strings.HasPrefix(lastRun.TriggerSource, "route_select:") {
			parts := strings.Split(lastRun.TriggerSource, ":")
			if len(parts) >= 2 {
				switch parts[1] {
				case "std_conv":
					routeName = "常规施工组织路线"
				case "green_std":
					routeName = "绿色低碳示范路线"
				case "smart_site":
					routeName = "智慧工地数字化路线"
				}
			}
		}
	}

	// 读取用户确认的骨架（优先使用）
	selectedSkeletonID := ""
	var skeletonSelection struct {
		SkeletonID   string `db:"skeleton_id"`
		SkeletonName string `db:"skeleton_name"`
	}
	if err := h.db.Get(&skeletonSelection, `SELECT skeleton_id, skeleton_name FROM tech_bid_skeleton_selections WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id); err == nil {
		selectedSkeletonID = skeletonSelection.SkeletonID
		log.Printf("[TechBid] Found user-confirmed skeleton: %s (%s)", skeletonSelection.SkeletonName, selectedSkeletonID)
	}

	// 确定使用 skeleton 模式（当有用户确认的骨架时）或 direct 模式
	outlineGenMode := "direct"
	if selectedSkeletonID != "" {
		outlineGenMode = "skeleton"
	}

	log.Printf("[TechBid] Regenerate outline for project: %s, using route: %s, skeleton: %s, mode: %s", id, routeName, selectedSkeletonID, outlineGenMode)

	// Clean up existing chapter data before regeneration
	tx1 := h.db.MustBegin()
	_, _ = tx1.Exec("DELETE FROM tech_bid_chapter_contents WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_chapter_plans WHERE project_id = ?", id)
	_, _ = tx1.Exec("DELETE FROM tech_bid_step4_gate_overrides WHERE project_id = ?", id)

	// Update project status
	_, err := tx1.Exec(`UPDATE tech_bid_projects SET current_step_status = 'running', project_status = 'waiting', step4_status = 'idle', last_error_message = NULL, step4_override_enabled = 0, step4_override_reason = NULL, step4_override_by = NULL, step4_override_at = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		tx1.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx1.Commit(); err != nil {
		log.Printf("[TechBid] Regenerate commit failed: %v", err)
		Error(c, http.StatusInternalServerError, "Database update failed")
		return
	}

	// Create new run
	runID, runErr := store.CreateRun(id, "outline_regenerate", cid, "running", "requirements_extracting")
	if runErr != nil {
		log.Printf("[TechBid] CreateRun failed: %v", runErr)
		Error(c, http.StatusInternalServerError, runErr.Error())
		return
	}

	professionStr := ""
	if project.Profession != nil {
		professionStr = *project.Profession
	}
	projectTypeStr := ""
	if project.ProjectType != nil {
		projectTypeStr = *project.ProjectType
	}
	outlineStep := "outline_generation"

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		service.RunOutlinePipeline(&service.OutlinePipelineInput{
			Ctx:                   ctx,
			DB:                    h.db,
			Store:                 store,
			Digitize:              h.digitizeService,
			RunID:                 runID,
			ProjectID:             id,
			CompanyID:             cid,
			RouteName:             routeName,
			ProfileData:           profileData,
			TenderContent:         tenderContent,
			ProfessionStr:         professionStr,
			ProjectTypeStr:        projectTypeStr,
			ProjectStep:           &outlineStep,
			SelectedSkeletonID:    selectedSkeletonID,
			OutlineGenerationMode: outlineGenMode, // 使用 skeleton 模式当有用户确认骨架时
			Hooks: service.OutlinePipelineHooks{
				UpdateProjectState:         h.updateProjectState,
				UpsertPendingStructurePlan: h.upsertPendingStructurePlan,
			},
		})
	}()

	c.JSON(200, gin.H{"success": true, "status": "running", "data": gin.H{"run_id": runID, "route": routeName}})
}

// PostOutlineChaptersGenerate 发击 Phase A：生成一级章草案
func (h *TechBidProjectHandler) PostOutlineChaptersGenerate(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[TechBid] PostOutlineChaptersGenerate called for project: %s", id)

	var project model.TechBidProject
	if err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id); err != nil {
		Error(c, http.StatusNotFound, "项目未找到")
		return
	}

	var profile model.TechBidProjectProfile
	_ = h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
	if profile.ProfileJSON == nil || *profile.ProfileJSON == "" {
		Error(c, http.StatusBadRequest, "项目画像缺失")
		return
	}

	professionStr := derefString(project.Profession)
	projectTypeStr := derefString(project.ProjectType)

	// 更新状态为正在生成一级章
	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'running', step4_status = 'generating_chapters' WHERE id = ?`, id)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		chapters, err := h.digitizeService.GenerateOutlineChapterDraft(ctx, id, *profile.ProfileJSON, professionStr, projectTypeStr)
		if err != nil {
			log.Printf("[TechBid] GenerateOutlineChapterDraft failed: %v", err)
			_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'failed', last_error_message = ? WHERE id = ?`, "一级章生成失败: "+err.Error(), id)
			return
		}

		// 轻量校验
		ok, msg := h.digitizeService.LightValidateChapters(chapters)
		chaptersJSON, _ := json.Marshal(chapters)
		chapterDraft := string(chaptersJSON)

		status := "pending_approval"
		if !ok {
			log.Printf("[TechBid] Light validation warning: %s", msg)
			// 即使校验不通过，也允许用户看到并修改，但在状态上体现
		}

		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET chapter_draft_json = ?, current_step_status = ?, step4_status = 'outline_chapter_confirm_pending', last_error_message = ? WHERE id = ?`,
			chapterDraft, status, msg, id)
	}()

	c.JSON(200, gin.H{"success": true, "message": "已开始生成一级章草案"})
}

// PostOutlineChaptersConfirm 确认并保存一级章骨架
func (h *TechBidProjectHandler) PostOutlineChaptersConfirm(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Chapters []string `json:"chapters" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	chaptersJSON, _ := json.Marshal(req.Chapters)
	confirmedChapters := string(chaptersJSON)

	_, err := h.db.Exec(`UPDATE tech_bid_projects SET confirmed_chapters_json = ?, step4_status = 'chapter_confirmed' WHERE id = ?`, confirmedChapters, id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "一级章已确认"})
}

// PostOutlineExpand 发起 Phase B：基于已确认骨架展开全文目录
func (h *TechBidProjectHandler) PostOutlineExpand(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[TechBid] PostOutlineExpand called for project: %s", id)

	companyIDAny, _ := c.Get("companyID")
	cid, _ := companyIDAny.(string)

	var project model.TechBidProject
	if err := h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id); err != nil {
		Error(c, http.StatusNotFound, "项目未找到")
		return
	}

	if project.ConfirmedChaptersJSON == nil || *project.ConfirmedChaptersJSON == "" {
		Error(c, http.StatusBadRequest, "一级章尚未确认，无法展开")
		return
	}

	var profile model.TechBidProjectProfile
	_ = h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
	profileData := derefString(profile.ProfileJSON)

	// 启动 Phase B
	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'running', step4_status = 'expanding_structure' WHERE id = ?`, id)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		var confirmedChapters []string
		_ = json.Unmarshal([]byte(*project.ConfirmedChaptersJSON), &confirmedChapters)

		outline, err := h.digitizeService.ExpandOutlineStructure(ctx, id, confirmedChapters, profileData)
		if err != nil {
			log.Printf("[TechBid] ExpandOutlineStructure failed: %v", err)
			_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'failed', last_error_message = ? WHERE id = ?`, "目录扩展失败: "+err.Error(), id)
			return
		}

		// 展开后执行完整审计（复用原有逻辑）
		// 为了简化，这里直接调用原有的 Audit 和后续流程
		h.finalizeFullOutline(ctx, id, cid, outline, project.Profession, project.ProjectType)
	}()

	c.JSON(200, gin.H{"success": true, "message": "已开始展开全文目录结构"})
}

// finalizeFullOutline 用于两步走流程的最后收尾：持久化目录、执行审计并更新状态
func (h *TechBidProjectHandler) finalizeFullOutline(ctx context.Context, projectID string, companyID string, outline []map[string]interface{}, profession *string, projectType *string) {
	log.Printf("[TechBid] Finalizing full outline for project: %s", projectID)

	// 1. 持久化到 tech_bid_chapter_plans
	tx := h.db.MustBegin()
	_, _ = tx.Exec(`DELETE FROM tech_bid_chapter_plans WHERE project_id = ?`, projectID)
	plans := h.mapAIOutlineToChapterPlans(projectID, outline)
	for i := range plans {
		p := plans[i]
		if _, err := tx.NamedExec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, requirement_ids_json, outline_version) VALUES (:id, :project_id, :parent_id, :chapter_name, :chapter_order, :node_level, 'completed', :requirement_ids_json, 1)`, &p); err != nil {
			log.Printf("[TechBid] finalizeFullOutline insert failed: %v", err)
		}
	}
	_ = tx.Commit()

	// 2. 触发完整审计
	// 获取事实
	facts, _ := h.loadPersistedFacts(projectID)
	outlineJSON, _ := json.Marshal(outline)
	audit, err := h.digitizeService.AuditOutlineCoverage(ctx, projectID, facts, string(outlineJSON))

	if err != nil {
		log.Printf("[TechBid] Final audit failed: %v", err)
		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'failed', last_error_message = ? WHERE id = ?`, "后期审计失败: "+err.Error(), projectID)
		return
	}

	// 3. 更新项目最终状态
	finalDecision := "PASS"
	if audit.CoverageScore < 80 {
		finalDecision = "REVISE"
	}

	riskLevel := "LOW"
	if finalDecision == "REVISE" {
		riskLevel = "MEDIUM"
	}

	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET current_step_status = 'completed', step4_status = 'outline_ready', coverage_score = ?, final_decision = ?, risk_level = ?, updated_at = ? WHERE id = ?`,
		audit.CoverageScore, finalDecision, riskLevel, time.Now(), projectID)
}

func (h *TechBidProjectHandler) GetOutlineRunStatus(c *gin.Context) {
	id := c.Param("id")
	store := service.NewStep4Store(h.db)
	data, err := store.RunStatusForAPI(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}

// GetOutlineAgentRuns returns step4_agent_runs for the latest run.
func (h *TechBidProjectHandler) GetOutlineAgentRuns(c *gin.Context) {
	id := c.Param("id")
	store := service.NewStep4Store(h.db)
	r, err := store.GetLatestRunByProject(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(200, gin.H{"success": true, "data": gin.H{"runs": []interface{}{}}})
			return
		}
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	agents, _ := store.ListAgentRuns(r.ID)
	c.JSON(200, gin.H{"success": true, "data": gin.H{"runs": agents}})
}

// GetOutlineRunHistory returns recent Step4 runs for the project.
func (h *TechBidProjectHandler) GetOutlineRunHistory(c *gin.Context) {
	id := c.Param("id")
	store := service.NewStep4Store(h.db)
	runs, err := store.ListRunsByProject(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"runs": runs}})
}

// GetOutlineApprovalLogs returns approval log history for the project.
func (h *TechBidProjectHandler) GetOutlineApprovalLogs(c *gin.Context) {
	id := c.Param("id")
	store := service.NewStep4Store(h.db)
	logs, err := store.ListApprovalLogs(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"logs": logs}})
}

// GetOutlineVersions lists step4_outline_versions for the project's latest run (Phase 2).
func (h *TechBidProjectHandler) GetOutlineVersions(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	versions, err := store.ListOutlineVersionsForLatestRun(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if versions == nil {
		versions = []map[string]interface{}{}
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"versions": versions}})
}

// GetOutlineVersionDetail returns outline JSON rebuilt from step4_outline_nodes.
func (h *TechBidProjectHandler) GetOutlineVersionDetail(c *gin.Context) {
	id := c.Param("id")
	vid := c.Param("versionId")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	var n int
	if err := h.db.Get(&n, `SELECT COUNT(*) FROM step4_outline_versions WHERE id = ? AND project_id = ?`, vid, id); err != nil || n == 0 {
		Error(c, http.StatusNotFound, "版本不存在")
		return
	}
	store := service.NewStep4Store(h.db)
	nodes, err := store.ListNodesForVersion(vid)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	outline := service.OutlineNodesToOutlineJSON(nodes)
	c.JSON(200, gin.H{"success": true, "data": gin.H{"outline": outline, "nodes": nodes}})
}

// SelectOutlineVersion marks a version recommended and replaces tech_bid_chapter_plans from its snapshot.
func (h *TechBidProjectHandler) SelectOutlineVersion(c *gin.Context) {
	id := c.Param("id")
	vid := c.Param("versionId")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	var row struct {
		VersionNo int `db:"version_no"`
	}
	if err := h.db.Get(&row, `SELECT version_no FROM step4_outline_versions WHERE id = ? AND project_id = ?`, vid, id); err != nil {
		Error(c, http.StatusNotFound, "版本不存在")
		return
	}
	var req struct {
		OperatorID string `json:"operator_id"`
	}
	_ = c.ShouldBindJSON(&req)
	store := service.NewStep4Store(h.db)
	if err := store.SetRecommendedOutlineVersion(id, vid); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	nodes, err := store.ListNodesForVersion(vid)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	outline := service.OutlineNodesToOutlineJSON(nodes)
	if err := service.ReplaceChapterPlansFromOutline(h.db, id, outline, row.VersionNo); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.logStep4Approval(id, "outline_version", "select", req.OperatorID, "select outline version "+vid)
	c.JSON(200, gin.H{"success": true, "message": "已切换目录版本并同步章节计划"})
}

func (h *TechBidProjectHandler) mapAIOutlineToChapterPlans(projectID string, outline []map[string]interface{}) []model.TechBidChapterPlan {
	var plans []model.TechBidChapterPlan
	for i, ch := range outline {
		chName, _ := ch["name"].(string)
		chID := uuid.New().String()
		plans = append(plans, model.TechBidChapterPlan{
			ID:           chID,
			ProjectID:    projectID,
			ChapterName:  chName,
			ChapterOrder: i + 1,
			NodeLevel:    ptr("chapter"),
		})

		units, _ := ch["units"].([]interface{})
		for j, u := range units {
			uMap, _ := u.(map[string]interface{})
			uName, _ := uMap["name"].(string)
			uID := uuid.New().String()
			plans = append(plans, model.TechBidChapterPlan{
				ID:           uID,
				ProjectID:    projectID,
				ParentID:     &chID,
				ChapterName:  uName,
				ChapterOrder: j + 1,
				NodeLevel:    ptr("unit"),
			})

			subs, _ := uMap["subsections"].([]interface{})
			for k, s := range subs {
				var sName string
				var reqIDs []string
				if sMap, ok := s.(map[string]interface{}); ok {
					sName, _ = sMap["name"].(string)
					if r, ok := sMap["requirement_ids"].([]interface{}); ok {
						for _, rid := range r {
							if rs, ok := rid.(string); ok {
								reqIDs = append(reqIDs, rs)
							}
						}
					}
				} else {
					sName, _ = s.(string)
				}
				reqJSON, _ := json.Marshal(reqIDs)
				reqStr := string(reqJSON)
				plans = append(plans, model.TechBidChapterPlan{
					ID:                 uuid.New().String(),
					ProjectID:          projectID,
					ParentID:           &uID,
					ChapterName:        sName,
					ChapterOrder:       k + 1,
					NodeLevel:          ptr("subsection"),
					RequirementIdsJSON: &reqStr,
				})
			}
		}
	}
	return plans
}

func (h *TechBidProjectHandler) mapChapterPlansToAIOutline(plans []model.TechBidChapterPlan) []map[string]interface{} {
	chapterMap := make(map[string]map[string]interface{})
	unitMap := make(map[string]map[string]interface{})
	var rootChapters []map[string]interface{}

	for _, p := range plans {
		if p.NodeLevel != nil && *p.NodeLevel == "chapter" {
			ch := map[string]interface{}{
				"name":  p.ChapterName,
				"units": []interface{}{},
			}
			chapterMap[p.ID] = ch
			rootChapters = append(rootChapters, ch)
		}
	}

	for _, p := range plans {
		if p.NodeLevel != nil && *p.NodeLevel == "unit" && p.ParentID != nil {
			un := map[string]interface{}{
				"name":        p.ChapterName,
				"subsections": []interface{}{},
			}
			unitMap[p.ID] = un
			if ch, ok := chapterMap[*p.ParentID]; ok {
				ch["units"] = append(ch["units"].([]interface{}), un)
			}
		}
	}

	for _, p := range plans {
		if p.NodeLevel != nil && *p.NodeLevel == "subsection" && p.ParentID != nil {
			reqIDs := []string{}
			if p.RequirementIdsJSON != nil {
				_ = json.Unmarshal([]byte(*p.RequirementIdsJSON), &reqIDs)
			}
			sub := map[string]interface{}{
				"name":            p.ChapterName,
				"requirement_ids": reqIDs,
			}
			if un, ok := unitMap[*p.ParentID]; ok {
				un["subsections"] = append(un["subsections"].([]interface{}), sub)
			}
		}
	}

	return rootChapters
}

func ptr(s string) *string { return &s }

func (h *TechBidProjectHandler) loadPersistedFacts(projectID string) (*service.FactExtractResult, error) {
	rows := []struct {
		FactCode        string  `db:"fact_code"`
		FactType        string  `db:"fact_type"`
		FactName        string  `db:"fact_name"`
		FactContent     string  `db:"fact_content"`
		SourceText      string  `db:"source_text"`
		SourceChapter   string  `db:"source_chapter"`
		PageNumber      int     `db:"page_number"`
		LineNumber      int     `db:"line_number"`
		Priority        string  `db:"priority"`
		ScoreValue      float64 `db:"score_value"`
		EvidenceCount   int     `db:"evidence_count"`
		ExtractedByView string  `db:"extracted_by_view"`
	}{}
	if err := h.db.Select(&rows, `SELECT fact_code, fact_type, COALESCE(fact_name, '') AS fact_name, COALESCE(fact_content, '') AS fact_content, COALESCE(source_text, '') AS source_text, COALESCE(source_section, '') AS source_chapter, COALESCE(source_page, 0) AS page_number, COALESCE(source_line, 0) AS line_number, COALESCE(priority, 'medium') AS priority, COALESCE(score_value, 0) AS score_value, COALESCE(evidence_count, 0) AS evidence_count, COALESCE(extracted_by_view, '') AS extracted_by_view FROM tech_bid_outline_facts WHERE project_id = ? ORDER BY created_at ASC`, projectID); err != nil {
		return nil, err
	}
	res := &service.FactExtractResult{}
	for _, row := range rows {
		item := service.FactItem{
			ID:              row.FactCode,
			Name:            row.FactName,
			Content:         row.FactContent,
			SourceText:      row.SourceText,
			SourceChapter:   row.SourceChapter,
			PageNumber:      row.PageNumber,
			LineNumber:      row.LineNumber,
			Priority:        row.Priority,
			ScoreValue:      row.ScoreValue,
			EvidenceCount:   row.EvidenceCount,
			ExtractedByView: row.ExtractedByView,
		}
		switch row.FactType {
		case "score_item":
			res.ScoreItems = append(res.ScoreItems, item)
		case "mandatory_spec":
			res.MandatorySpecs = append(res.MandatorySpecs, item)
		case "project_characteristic":
			res.ProjectCharacteristics = append(res.ProjectCharacteristics, item)
		case "special_topic":
			res.SpecialTopics = append(res.SpecialTopics, item)
		}
	}
	return res, nil
}

func (h *TechBidProjectHandler) loadPersistedRequirementRegister(projectID string) ([]service.RequirementRegisterEntry, error) {
	rows := []model.TechBidRequirementRegister{}
	if err := h.db.Select(&rows, `SELECT * FROM tech_bid_requirement_register WHERE project_id = ? ORDER BY created_at ASC`, projectID); err != nil {
		return nil, err
	}
	out := make([]service.RequirementRegisterEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, service.RequirementRegisterEntry{
			RequirementID:         row.RequirementID,
			RequirementType:       row.RequirementType,
			SourceText:            row.SourceText,
			SourceLocation:        row.SourceLocation,
			Priority:              row.Priority,
			MustBeExplicit:        row.MustBeExplicit,
			ExpectedResponseLevel: row.ExpectedResponseLevel,
			Domain:                row.Domain,
			ResponseTier:          row.ResponseTier,
			Summary:               row.Summary,
		})
	}
	return out, nil
}

func (h *TechBidProjectHandler) loadPersistedMappings(projectID string) ([]service.FactOutlineMapping, error) {
	rows := []model.TechBidFactMapping{}
	if err := h.db.Select(&rows, `SELECT * FROM tech_bid_fact_mappings WHERE project_id = ? ORDER BY created_at ASC`, projectID); err != nil {
		return nil, err
	}
	out := make([]service.FactOutlineMapping, 0, len(rows))
	for _, row := range rows {
		var path []string
		_ = json.Unmarshal([]byte(row.TargetPathJSON), &path)
		out = append(out, service.FactOutlineMapping{
			FactID:        row.FactID,
			FactType:      row.FactType,
			FactName:      row.FactName,
			TargetLevel:   row.TargetLevel,
			TargetPath:    path,
			Required:      row.Required == 1,
			Priority:      row.Priority,
			MappingReason: derefString(row.MappingReason),
		})
	}
	return out, nil
}

func (h *TechBidProjectHandler) upsertPendingStructurePlan(projectID string, adjustmentsJSON string, profileJSON string, personalizationScore float64, rationale string) (string, error) {
	var existingID string
	_ = h.db.Get(&existingID, `SELECT id FROM tech_bid_structure_plan WHERE project_id = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, projectID)
	if strings.TrimSpace(existingID) != "" {
		_, err := h.db.Exec(`UPDATE tech_bid_structure_plan SET adjustments_json = ?, tender_profile_json = ?, personalization_score = ?, rationale = ?, reject_reason = NULL WHERE id = ?`, adjustmentsJSON, profileJSON, personalizationScore, rationale, existingID)
		if err == nil {
			_, _ = h.db.Exec(`UPDATE step4_structure_plans SET adjustments_json = ?, rationale = ?, status = 'pending' WHERE id = ?`, adjustmentsJSON, rationale, existingID)
		}
		return existingID, err
	}
	planID := uuid.New().String()
	tx, err := h.db.Beginx()
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE tech_bid_structure_plan SET status = 'rejected', reject_reason = COALESCE(reject_reason, 'superseded by newer pending plan') WHERE project_id = ? AND status = 'pending'`, projectID); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	if _, err = tx.Exec(`INSERT INTO tech_bid_structure_plan (id, project_id, adjustments_json, tender_profile_json, personalization_score, rationale, status) VALUES (?, ?, ?, ?, ?, ?, 'pending')`, planID, projectID, adjustmentsJSON, profileJSON, personalizationScore, rationale); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	if _, err = tx.Exec(`UPDATE tech_bid_projects SET structure_plan_status = 'pending', current_step_status = 'waiting_for_approval', step4_status = 'generating_outline', updated_at = ? WHERE id = ?`, time.Now(), projectID); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	var runID sql.NullInt64
	if err := h.db.Get(&runID, `SELECT active_step4_run_id FROM tech_bid_projects WHERE id = ?`, projectID); err == nil && runID.Valid && runID.Int64 > 0 {
		_, _ = h.db.Exec(`INSERT INTO step4_structure_plans (id, run_id, project_id, adjustments_json, rationale, status) VALUES (?, ?, ?, ?, ?, 'pending')`,
			planID, runID.Int64, projectID, adjustmentsJSON, rationale)
	}
	return planID, nil
}

func (h *TechBidProjectHandler) healStructurePlanState(projectID string, project *model.TechBidProject) {
	if project == nil || strings.ToLower(strings.TrimSpace(derefString(project.StructurePlanStatus))) != "pending" {
		return
	}
	var pendingCount int
	if err := h.db.Get(&pendingCount, `SELECT COUNT(*) FROM tech_bid_structure_plan WHERE project_id = ? AND status = 'pending'`, projectID); err != nil || pendingCount > 0 {
		return
	}
	status := "failed"
	step4Status := project.Step4Status
	message := "结构计划状态异常：主表显示待审批，但未找到有效待审批计划，请重新生成目录。"
	if derefString(project.CurrentStep) != "outline_generation" || derefString(project.CurrentStepStatus) != "waiting_for_approval" {
		status = derefString(project.CurrentStepStatus)
		if status == "" {
			status = "waiting"
		}
		if step4Status == "" {
			step4Status = "idle"
		}
	}
	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET structure_plan_status = 'rejected', current_step_status = ?, step4_status = ?, last_error_message = ?, updated_at = ? WHERE id = ?`, status, step4Status, message, time.Now(), projectID)
	project.StructurePlanStatus = ptr("rejected")
	project.CurrentStepStatus = &status
	project.Step4Status = step4Status
	project.LastErrorMessage = &message
}

func (h *TechBidProjectHandler) continueOutlineGenerationFromApprovedPlan(projectID string, routeName string, approvedPlanJSON string) error {
	// G2: Always use direct generation mode - skeleton approval flow now triggers direct generation
	facts, err := h.loadPersistedFacts(projectID)
	if err != nil {
		return err
	}
	requirementRegister, err := h.loadPersistedRequirementRegister(projectID)
	if err != nil {
		return err
	}
	// Note: mappings no longer required for direct generation mode (G2)
	// Validation functions will receive empty mappings
	var mappings []service.FactOutlineMapping

	var project model.TechBidProject
	if err := h.db.Unsafe().Get(&project, `SELECT * FROM tech_bid_projects WHERE id = ?`, projectID); err != nil {
		return err
	}
	var profile model.TechBidProjectProfile
	_ = h.db.Unsafe().Get(&profile, `SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, projectID)
	profileData := derefString(profile.ProfileJSON)
	professionStr := derefString(project.Profession)
	projectTypeStr := derefString(project.ProjectType)
	cid := project.CompanyID
	if strings.TrimSpace(routeName) == "" {
		routeName = "常规施工组织路线"
	}

	// Get tender content for direct generation
	var tenderFile struct {
		FileAssetID string `db:"file_asset_id"`
	}
	tenderContent := ""
	if err := h.db.Get(&tenderFile, `SELECT file_asset_id FROM tech_bid_tender_files WHERE project_id = ? AND file_role = 'tender' LIMIT 1`, projectID); err == nil {
		var content struct {
			MarkdownText *string `db:"markdown_text"`
		}
		if err := h.db.Get(&content, `SELECT markdown_text FROM file_content WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1`, tenderFile.FileAssetID); err == nil && content.MarkdownText != nil {
			tenderContent = *content.MarkdownText
		}
	}
	// Enrich tender content with domain hints
	tenderContent = service.EnrichTenderForStep4(tenderContent, professionStr, projectTypeStr)

	// G2: Use GenerateOutlineDirectly instead of GenerateOutlineFromFacts
	subCtxGen, cancelGen := context.WithTimeout(context.Background(), 300*time.Second)
	draftOutline, err := h.digitizeService.GenerateOutlineDirectly(
		subCtxGen, projectID, tenderContent, facts, requirementRegister,
		profileData, routeName, projectTypeStr, professionStr,
	)
	cancelGen()
	if err != nil {
		log.Printf("[TechBid] Draft generation failed: %v", err)
		h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录生成失败: "+err.Error(), projectID)
		h.updateProjectState(projectID, nil, nil, "running", "failed", "ai", nil, "", "ai", "目录生成失败: "+err.Error(), nil)
		return err
	}
	h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'outline_ready' WHERE id = ?", projectID)

	// === 清理目录中重复的章节编号 ===
	draftOutline = service.NormalizeOutlineNames(draftOutline)
	log.Printf("[TechBid] 已清理目录中的重复章节编号")

	// === 评分表必出项硬校验 + 循环补章（最多3轮）===
	mandatoryChapters := service.ExtractMandatoryChapters(facts)
	if len(mandatoryChapters) > 0 {
		missing, ok := service.ValidateMandatoryChapters(draftOutline, mandatoryChapters)
		if !ok {
			log.Printf("[TechBid] 必出项缺失: %v，启动循环补章", missing)
			draftOutline, _ = service.PatchMissingMandatoryChaptersWithLoop(draftOutline, mandatoryChapters, 3)
			log.Printf("[TechBid] 循环补章后目录共 %d 个一级章", len(draftOutline))
		} else {
			log.Printf("[TechBid] 必出项校验通过，无需补章")
		}
	}

	structCov := h.digitizeService.ValidateOutlineSemanticCoverage(draftOutline, facts, mappings)
	fullRes := h.digitizeService.ValidateFullRequirementResponse(draftOutline, requirementRegister, professionStr, projectTypeStr)
	draftJSON, _ := json.Marshal(draftOutline)
	factsJSON, _ := json.Marshal(facts)
	_, _ = service.PersistCoverageCheck(h.db, projectID, 1, structCov)
	_, _ = service.PersistFullResponseCheck(h.db, projectID, 1, fullRes)

	h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'auditing_outline' WHERE id = ?", projectID)
	subCtxAudit, cancelAudit := context.WithTimeout(context.Background(), 300*time.Second)
	audit, err := h.digitizeService.AuditOutlineCoverage(subCtxAudit, projectID, facts, string(draftJSON))
	cancelAudit()
	if err != nil {
		log.Printf("[TechBid] Initial audit failed: %v", err)
		h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录审计失败: "+err.Error(), projectID)
		h.updateProjectState(projectID, nil, nil, "running", "failed", "ai", nil, "", "ai", "目录审计失败: "+err.Error(), nil)
		return err
	}

	finalDecision, reason := h.digitizeService.DecideNextFlowWithFullResponse(audit, structCov, fullRes)
	mergedScore := audit.CoverageScore
	if structCov != nil && structCov.CoverageRate < mergedScore {
		mergedScore = structCov.CoverageRate
	}
	if fullRes != nil && fullRes.RequirementTotal > 0 && fullRes.FullResponseRate < mergedScore {
		mergedScore = fullRes.FullResponseRate
	}

	if finalDecision == "REVISE" {
		// === 防污染：提取骨架快照 ===
		frozenSkeleton := service.ExtractSkeleton(draftOutline)
		log.Printf("[TechBid] 防污染：已冻结骨架，共 %d 个一级章", len(frozenSkeleton))

		h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'refining_outline' WHERE id = ?", projectID)
		subCtxOpt, cancelOpt := context.WithTimeout(context.Background(), 300*time.Second)
		finalOutline, optErr := h.digitizeService.OptimizeOutlineByCoverage(subCtxOpt, projectID, string(draftJSON), audit, facts, mappings, fullRes)
		cancelOpt()
		if optErr == nil {
			// === 防污染：强制恢复骨架 ===
			if len(frozenSkeleton) > 0 {
				finalOutline = service.EnforceSkeleton(finalOutline, frozenSkeleton)
				log.Printf("[TechBid] 防污染：已强制恢复骨架（%d 个章）", len(frozenSkeleton))
			}

			// === 防污染：骨架偏移检测 ===
			if len(frozenSkeleton) > 0 {
				drift := service.ComputeChapterDrift(draftOutline, finalOutline)
				log.Printf("[TechBid] 防污染：骨架偏移 drift=%.2f", drift)
				if drift > 0.3 {
					log.Printf("[TechBid] ⚠️ 防污染：骨架偏移过大 (drift=%.2f > 0.3)，回滚到原始目录", drift)
					finalOutline = draftOutline
				}
			}

			draftOutline = finalOutline
			draftJSON, _ = json.Marshal(draftOutline)
			structCov = h.digitizeService.ValidateOutlineSemanticCoverage(draftOutline, facts, mappings)
			fullRes = h.digitizeService.ValidateFullRequirementResponse(draftOutline, requirementRegister, professionStr, projectTypeStr)
			_, _ = service.PersistCoverageCheck(h.db, projectID, 1, structCov)
			_, _ = service.PersistFullResponseCheck(h.db, projectID, 1, fullRes)
			mergedScore = audit.CoverageScore
			if structCov != nil && structCov.CoverageRate < mergedScore {
				mergedScore = structCov.CoverageRate
			}
			if fullRes != nil && fullRes.RequirementTotal > 0 && fullRes.FullResponseRate < mergedScore {
				mergedScore = fullRes.FullResponseRate
			}
			finalDecision, reason = h.digitizeService.DecideNextFlowWithFullResponse(audit, structCov, fullRes)
		}
		h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'refine_ready' WHERE id = ?", projectID)
	} else {
		h.db.Exec("UPDATE tech_bid_projects SET step4_status = 'audit_ready' WHERE id = ?", projectID)
	}

	riskLevel := "LOW"
	if finalDecision != "PASS" {
		riskLevel = "MEDIUM"
	}
	if finalDecision == "BLOCK" {
		riskLevel = "HIGH"
	}
	db.EnsureTechBidOutlineAuditsSchema(h.db)
	auditID := uuid.New().String()
	_, _ = h.db.Exec(`INSERT INTO tech_bid_outline_audits (id, project_id, outline_version, outline_snapshot_json, facts_snapshot_json, coverage_score, audit_summary, final_decision, risk_level, can_proceed, audit_version) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
		auditID, projectID, string(draftJSON), string(factsJSON), mergedScore, audit.AuditSummary, finalDecision, riskLevel, map[bool]int{true: 1, false: 0}[finalDecision == "PASS"], "v2")

	gateRate := 0.0
	gateRes := "PASS"
	gateReasonText := ""
	if fullRes != nil {
		gateRate = fullRes.FullResponseRate
		gateRes = fullRes.Result
		gateReasonText = fullRes.Summary
	}
	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET coverage_score = ?, final_decision = ?, risk_level = ?, full_response_rate = ?, step4_gate_result = ?, step4_gate_reason = ?, last_error_message = ?, structure_plan_status = 'approved' WHERE id = ?`,
		mergedScore, finalDecision, riskLevel, gateRate, gateRes, gateReasonText, reason, projectID)

	tx := h.db.MustBegin()
	_, _ = tx.Exec(`DELETE FROM tech_bid_chapter_plans WHERE project_id = ?`, projectID)
	for i, ch := range draftOutline {
		chapterID := uuid.New().String()
		chapterName := ""
		if n, ok := ch["name"].(string); ok {
			chapterName = n
		}
		if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, 'chapter', 'completed', 1)`, chapterID, projectID, chapterName, i+1); dbErr != nil {
			_ = tx.Rollback()
			return dbErr
		}
		units, _ := ch["units"].([]interface{})
		for j, u := range units {
			uMap, _ := u.(map[string]interface{})
			unitID := uuid.New().String()
			unitName, _ := uMap["name"].(string)
			if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, ?, 'unit', 'completed', 1)`, unitID, projectID, chapterID, unitName, j+1); dbErr != nil {
				_ = tx.Rollback()
				return dbErr
			}
			subs, _ := uMap["subsections"].([]interface{})
			for k, s := range subs {
				subName := ""
				reqIDs := ""
				if sMap, ok := s.(map[string]interface{}); ok {
					subName, _ = sMap["name"].(string)
					if r, ok := sMap["requirement_ids"].([]interface{}); ok {
						rJSON, _ := json.Marshal(r)
						reqIDs = string(rJSON)
					}
				} else {
					subName, _ = s.(string)
				}
				if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version, requirement_ids_json) VALUES (?, ?, ?, ?, ?, 'subsection', 'not_started', 1, ?)`, uuid.New().String(), projectID, unitID, subName, k+1, reqIDs); dbErr != nil {
					_ = tx.Rollback()
					return dbErr
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	fp := h.digitizeService.OutlineFingerprint(draftOutline)
	outlineTitles := service.CollectOutlineSubsectionTitles(draftOutline)
	titlesJSON, _ := json.Marshal(outlineTitles)
	var dup int
	hint := ""
	if cid != "" {
		if err := h.db.Get(&dup, `SELECT COUNT(*) FROM tech_bid_projects WHERE company_id = ? AND outline_fingerprint = ? AND id != ?`, cid, fp, projectID); err == nil && dup > 0 {
			hint = "与本企业其他项目目录指纹重复，请复核雷同风险"
		}
	}
	jaccardHint := ""
	if cid != "" {
		jaccardHint = service.BestHistoryJaccardHint(h.db, cid, projectID, outlineTitles)
	}
	mergedHint := strings.TrimSpace(strings.Join([]string{hint, jaccardHint}, " "))
	var hintArg interface{}
	if mergedHint != "" {
		hintArg = mergedHint
	}
	_, _ = h.db.Exec(`UPDATE tech_bid_projects SET outline_fingerprint = ?, history_similarity_hint = ?, outline_titles_json = ?, step4_status = 'audit_ready' WHERE id = ?`, fp, hintArg, string(titlesJSON), projectID)
	h.updateProjectState(projectID, nil, nil, "running", "success", "system", nil, "", "ai", "目录生成与审计完成", map[string]interface{}{"coverage": mergedScore})
	return nil
}

func latestFullResponseCheckJSON(db *sqlx.DB, projectID string) string {
	var row model.TechBidRequirementResponseCheck
	err := db.Get(&row, `SELECT * FROM tech_bid_requirement_response_checks WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, projectID)
	if err != nil {
		return "{}"
	}
	parseStr := func(p *string) []string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []string
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	payload := map[string]interface{}{
		"requirement_total":            row.RequirementTotal,
		"requirement_mapped":           row.RequirementMapped,
		"requirement_fully_responded":  row.RequirementFullyResponded,
		"requirement_weakly_responded": row.RequirementWeaklyResponded,
		"requirement_only_tagged":      row.RequirementOnlyTagged,
		"full_response_rate":           row.FullResponseRate,
		"weak_response_rate":           row.WeakResponseRate,
		"response_quality_score":       row.ResponseQualityScore,
		"missing_requirement_ids":      parseStr(row.MissingRequirementIDsJSON),
		"weak_requirement_ids":         parseStr(row.WeakRequirementIDsJSON),
		"only_tagged_requirement_ids":  parseStr(row.OnlyTaggedRequirementIDsJSON),
		"shell_title_hints":            parseStr(row.ShellTitleHintsJSON),
		"hard_rule_warnings":           parseStr(row.HardRuleWarningsJSON),
		"result":                       row.Result,
		"summary":                      summary,
	}
	var proj struct {
		FullResponseRate float64 `db:"full_response_rate"`
		Step4GateResult  *string `db:"step4_gate_result"`
		Step4GateReason  *string `db:"step4_gate_reason"`
	}
	if err := db.Get(&proj, `SELECT full_response_rate, step4_gate_result, step4_gate_reason FROM tech_bid_projects WHERE id = ?`, projectID); err == nil {
		payload["project_full_response_rate"] = proj.FullResponseRate
		if proj.Step4GateResult != nil {
			payload["project_step4_gate_result"] = *proj.Step4GateResult
		}
		if proj.Step4GateReason != nil {
			payload["project_step4_gate_reason"] = *proj.Step4GateReason
		}
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

func (h *TechBidProjectHandler) GetTechStep6Payload(c *gin.Context) {
	id := c.Param("id")
	var row struct {
		Step6Status      sql.NullString `db:"step6_status"`
		Step6PayloadJSON sql.NullString `db:"step6_payload_json"`
	}
	if err := h.db.Get(&row, `SELECT step6_status, step6_payload_json FROM tech_bid_projects WHERE id = ?`, id); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	status := "idle"
	if row.Step6Status.Valid && strings.TrimSpace(row.Step6Status.String) != "" {
		status = row.Step6Status.String
	}
	var payload interface{}
	if row.Step6PayloadJSON.Valid && strings.TrimSpace(row.Step6PayloadJSON.String) != "" {
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(row.Step6PayloadJSON.String), &parsed) == nil {
			payload = parsed
		}
	}
	resp := gin.H{"success": true, "status": status, "data": payload}
	if latest := h.latestTechStep6OutputPath(id); latest != "" {
		resp["latest_download_url"] = fmt.Sprintf("/api/tech-bid/projects/%s/step6/download?file=%s", id, filepath.Base(latest))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *TechBidProjectHandler) GenerateTechStep6Payload(c *gin.Context) {
	id := c.Param("id")
	if err := h.ensureTechStep6CanRun(id); err != nil {
		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'error', last_error_message = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now(), id)
		Error(c, http.StatusForbidden, err.Error())
		return
	}

	_, err := h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'generating', last_error_message = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	payload, err := h.buildTechStep6Payload(id)
	if err != nil {
		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'error', last_error_message = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now(), id)
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	payloadBytes, _ := json.Marshal(payload)
	_, err = h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'success', step6_payload_json = ?, last_error_message = NULL, updated_at = ? WHERE id = ?`, string(payloadBytes), time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "status": "success", "data": payload})
}

func (h *TechBidProjectHandler) ExportTechFinalWord(c *gin.Context) {
	id := c.Param("id")
	if err := h.ensureTechStep6CanRun(id); err != nil {
		Error(c, http.StatusForbidden, err.Error())
		return
	}
	var input struct {
		PayloadJSON      string `json:"payload_json"`
		TemplateFilePath string `json:"template_file_path"`
	}
	_ = c.ShouldBindJSON(&input)
	if strings.TrimSpace(input.PayloadJSON) == "" {
		var payload sql.NullString
		if err := h.db.Get(&payload, `SELECT step6_payload_json FROM tech_bid_projects WHERE id = ?`, id); err == nil && payload.Valid {
			input.PayloadJSON = payload.String
		}
	}
	if strings.TrimSpace(input.PayloadJSON) == "" {
		payload, err := h.buildTechStep6Payload(id)
		if err != nil {
			_, _ = h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'error', last_error_message = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now(), id)
			Error(c, http.StatusBadRequest, "Step6 payload is empty and could not be generated: "+err.Error())
			return
		}
		payloadBytes, _ := json.Marshal(payload)
		input.PayloadJSON = string(payloadBytes)
		if _, err := h.db.Exec(`UPDATE tech_bid_projects SET step6_status = 'success', step6_payload_json = ?, last_error_message = NULL, updated_at = ? WHERE id = ?`, input.PayloadJSON, time.Now(), id); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	exportPath, err := service.NewStep6ExporterService(h.db).ExecuteSafeWordExportToDir(c.Request.Context(), input.PayloadJSON, input.TemplateFilePath, techStep6ExportRoot())
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to execute technical export: "+err.Error())
		return
	}
	if err := h.recordTechStep6Output(id, exportPath); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to record technical export: "+err.Error())
		return
	}
	downloadURL := fmt.Sprintf("/api/tech-bid/projects/%s/step6/download?file=%s", id, filepath.Base(exportPath))
	c.JSON(http.StatusOK, gin.H{"success": true, "export_path": exportPath, "download_url": downloadURL})
}

func (h *TechBidProjectHandler) DownloadTechStep6Word(c *gin.Context) {
	id := c.Param("id")
	fileName := c.Query("file")
	if fileName == "" || strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
		Error(c, http.StatusBadRequest, "Invalid file name")
		return
	}
	filePath, err := resolveTechStep6DownloadPath(id, fileName)
	if err != nil {
		Error(c, http.StatusNotFound, "File not found or expired")
		return
	}
	c.FileAttachment(filePath, "技术标最终文档.docx")
}

func (h *TechBidProjectHandler) ensureTechStep6CanRun(projectID string) error {
	var project model.TechBidProject
	if err := h.db.Get(&project, `SELECT id, final_decision, can_enter_content_generation, step4_override_enabled, override_enabled FROM tech_bid_projects WHERE id = ?`, projectID); err != nil {
		return fmt.Errorf("project not found")
	}
	decision := ""
	if project.FinalDecision != nil {
		decision = strings.TrimSpace(*project.FinalDecision)
	}
	if decision == "PASS" || project.CanEnterContentGeneration == 1 || project.Step4OverrideEnabled == 1 || project.OverrideEnabled == 1 {
		return nil
	}
	return fmt.Errorf("STEP4_NOT_PASSED: 技术标目录未通过 Step4 自检，不能生成最终文档")
}

func (h *TechBidProjectHandler) GenerateTechStep5Content(c *gin.Context) {
	id := c.Param("id")
	if err := h.ensureTechStep6CanRun(id); err != nil {
		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET step5_status = 'error', last_error_message = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now(), id)
		Error(c, http.StatusForbidden, err.Error())
		return
	}

	_, err := h.db.Exec(`UPDATE tech_bid_projects SET step5_status = 'generating', last_error_message = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	generated, err := h.generateTechStep5Content(id)
	if err != nil {
		_, _ = h.db.Exec(`UPDATE tech_bid_projects SET step5_status = 'error', last_error_message = ?, updated_at = ? WHERE id = ?`, err.Error(), time.Now(), id)
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = h.db.Exec(`UPDATE tech_bid_projects SET step5_status = 'success', step6_status = 'idle', last_error_message = NULL, updated_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "status": "success", "generated_count": generated})
}

func (h *TechBidProjectHandler) generateTechStep5Content(projectID string) (int, error) {
	var rows []struct {
		ID                 string         `db:"id"`
		ChapterName        string         `db:"chapter_name"`
		ChapterOrder       int            `db:"chapter_order"`
		RequirementIdsJSON sql.NullString `db:"requirement_ids_json"`
	}
	if err := h.db.Select(&rows, `
		SELECT id, chapter_name, chapter_order, requirement_ids_json
		FROM tech_bid_chapter_plans
		WHERE project_id = ? AND node_level = 'subsection'
		ORDER BY chapter_order ASC, id ASC`, projectID); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("no approved Step4 subsections found for Step5 generation")
	}

	requirements := h.techRequirementSummaryMap(projectID)
	tx := h.db.MustBegin()
	now := time.Now()
	for _, row := range rows {
		reqIDs := parseRequirementIDsJSON(row.RequirementIdsJSON)
		content := buildTechStep5ChapterContent(row.ChapterName, reqIDs, requirements)
		var maxVersion int
		_ = tx.Get(&maxVersion, `SELECT COALESCE(MAX(version_no), 0) FROM tech_bid_chapter_contents WHERE chapter_id = ?`, row.ID)
		sourceRefs, _ := json.Marshal(reqIDs)
		_, err := tx.Exec(`
			INSERT INTO tech_bid_chapter_contents
				(id, project_id, chapter_id, version_no, content_md, content_html, source_refs_json, generation_model, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), projectID, row.ID, maxVersion+1, content, "", string(sourceRefs), "deterministic-step5-v1", "final", now, now)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE tech_bid_chapter_plans SET generation_status = 'completed', updated_at = ? WHERE id = ?`, now, row.ID); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (h *TechBidProjectHandler) buildTechStep6Payload(projectID string) (*agent.BidActionList, error) {
	var rows []struct {
		ID                 string         `db:"id"`
		ChapterName        string         `db:"chapter_name"`
		ChapterOrder       int            `db:"chapter_order"`
		NodeLevel          sql.NullString `db:"node_level"`
		RequirementIdsJSON sql.NullString `db:"requirement_ids_json"`
		ContentMD          sql.NullString `db:"content_md"`
	}
	err := h.db.Select(&rows, `
		SELECT
			p.id,
			p.chapter_name,
			p.chapter_order,
			p.node_level,
			p.requirement_ids_json,
			COALESCE((
				SELECT c.content_md
				FROM tech_bid_chapter_contents c
				WHERE c.chapter_id = p.id
				ORDER BY c.version_no DESC, c.updated_at DESC
				LIMIT 1
			), '') AS content_md
		FROM tech_bid_chapter_plans p
		WHERE p.project_id = ?
		ORDER BY p.chapter_order ASC, p.id ASC`, projectID)
	if err != nil {
		return nil, err
	}
	requirements := h.techRequirementSummaryMap(projectID)
	slots := make([]agent.BidActionSlot, 0, len(rows))
	var original strings.Builder
	for _, row := range rows {
		content := strings.TrimSpace(row.ContentMD.String)
		if content == "" && strings.TrimSpace(row.NodeLevel.String) == "subsection" {
			content = buildTechStep6FallbackContent(row.ChapterName, parseRequirementIDsJSON(row.RequirementIdsJSON), requirements)
		}
		if content == "" {
			continue
		}
		if original.Len() > 0 {
			original.WriteString("\n\n")
		}
		original.WriteString(content)
		slots = append(slots, agent.BidActionSlot{
			SlotID:           "tech_chapter_" + sanitizeStep6PathPart(row.ID),
			ChapterPath:      []string{row.ChapterName},
			SlotContextTitle: row.ChapterName,
			TargetField:      "技术标章节_" + row.ChapterName,
			SlotType:         agent.SlotTypeText,
			AISuggestedValue: content,
			Reason:           "由技术标 Step4/Step5 已通过的章节内容生成。",
			Status:           agent.StatusApproved,
		})
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("no completed technical chapter content found")
	}
	return &agent.BidActionList{
		ProjectID:        projectID,
		Chapter:          "技术标最终文档装配",
		Slots:            slots,
		OriginalMarkdown: original.String(),
	}, nil
}

func buildTechStep5ChapterContent(chapterName string, reqIDs []string, requirements map[string]string) string {
	title := strings.TrimSpace(chapterName)
	if title == "" {
		title = "技术标响应措施"
	}
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("### 一、响应目标\n")
	b.WriteString("本节围绕招标文件对应条款展开，明确施工组织、过程控制、验收配合和资料闭环要求，确保投标文件对评分项和强制技术要求形成可追溯响应。\n\n")
	if len(reqIDs) > 0 {
		b.WriteString("### 二、招标要求依据\n")
		for _, id := range reqIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			detail := strings.TrimSpace(requirements[id])
			if detail == "" {
				detail = id
			}
			b.WriteString("- ")
			b.WriteString(strings.ReplaceAll(detail, "\n", "；"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("### 三、实施措施\n")
	b.WriteString("项目实施过程中，将由项目经理牵头组织技术负责人、安全负责人、质量负责人和资料负责人进行分工落实。施工前完成技术交底和作业条件确认，施工中按工序开展自检、互检和专检，关键节点形成检查记录、影像资料和验收资料。对涉及安全、质量、进度、资源投入和现场文明施工的要求，均纳入日常巡检和问题闭环台账，确保发现问题后及时整改、复核、归档。\n\n")
	b.WriteString("### 四、质量与验收闭环\n")
	b.WriteString("本节对应工作完成后，将按照招标文件、施工图设计、现行验收规范及建设单位管理要求组织验收配合。所有过程资料与验收资料同步整理，做到责任人明确、检查记录完整、整改闭环可查、交付成果满足招标文件要求。")
	return b.String()
}

func (h *TechBidProjectHandler) techRequirementSummaryMap(projectID string) map[string]string {
	var rows []struct {
		RequirementID  string         `db:"requirement_id"`
		Summary        sql.NullString `db:"summary"`
		SourceText     sql.NullString `db:"source_text"`
		SourceLocation sql.NullString `db:"source_location"`
	}
	out := make(map[string]string)
	if err := h.db.Select(&rows, `SELECT requirement_id, summary, source_text, source_location FROM tech_bid_requirement_register WHERE project_id = ?`, projectID); err != nil {
		return out
	}
	for _, row := range rows {
		id := strings.TrimSpace(row.RequirementID)
		if id == "" {
			continue
		}
		parts := []string{}
		if strings.TrimSpace(row.Summary.String) != "" {
			parts = append(parts, strings.TrimSpace(row.Summary.String))
		}
		if strings.TrimSpace(row.SourceLocation.String) != "" {
			parts = append(parts, "来源："+strings.TrimSpace(row.SourceLocation.String))
		}
		if strings.TrimSpace(row.SourceText.String) != "" {
			parts = append(parts, "原文依据："+strings.TrimSpace(row.SourceText.String))
		}
		out[id] = strings.Join(parts, "\n")
	}
	return out
}

func parseRequirementIDsJSON(value sql.NullString) []string {
	raw := strings.TrimSpace(value.String)
	if !value.Valid || raw == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		return ids
	}
	var anyIDs []interface{}
	if err := json.Unmarshal([]byte(raw), &anyIDs); err != nil {
		return nil
	}
	for _, item := range anyIDs {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			ids = append(ids, strings.TrimSpace(s))
		}
	}
	return ids
}

func buildTechStep6FallbackContent(chapterName string, reqIDs []string, requirements map[string]string) string {
	title := strings.TrimSpace(chapterName)
	if title == "" {
		title = "技术标响应措施"
	}
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("本节按照已通过 Step4 自检的技术标目录和招标要求编制，作为最终文档装配内容。\n\n")
	if len(reqIDs) > 0 {
		b.WriteString("### 招标要求响应\n")
		for _, id := range reqIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			detail := strings.TrimSpace(requirements[id])
			if detail == "" {
				detail = id
			}
			b.WriteString("- ")
			b.WriteString(strings.ReplaceAll(detail, "\n", "；"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("### 实施措施\n")
	b.WriteString("围绕本节对应的招标条款，投标文件将从组织安排、过程控制、质量安全、验收配合和资料归档等方面逐项响应，确保章节内容与招标要求保持一致并具备可追溯依据。")
	return b.String()
}

func (h *TechBidProjectHandler) recordTechStep6Output(projectID string, exportPath string) error {
	var versionNo interface{}
	var activeVersionNo int
	if err := h.db.Get(&activeVersionNo, `SELECT active_version_no FROM tech_bid_projects WHERE id = ?`, projectID); err == nil {
		versionNo = activeVersionNo
	}
	now := time.Now()
	_, err := h.db.Exec(`INSERT INTO tech_bid_outputs
		(id, project_id, version_no, output_type, file_name, file_path, mime_type, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		projectID,
		versionNo,
		"technical_word",
		filepath.Base(exportPath),
		exportPath,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"available",
		now,
	)
	if err != nil {
		return err
	}
	_, err = h.db.Exec(`UPDATE tech_bid_projects
		SET current_step = 'output_finalize',
			current_step_status = 'success',
			step6_status = 'success',
			last_error_message = NULL,
			updated_at = ?
		WHERE id = ?`, now, projectID)
	return err
}

func (h *TechBidProjectHandler) latestTechStep6OutputPath(projectID string) string {
	var filePaths []string
	err := h.db.Select(&filePaths, `SELECT file_path FROM tech_bid_outputs
		WHERE project_id = ? AND output_type = 'technical_word' AND status = 'available'
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

func resolveTechStep6DownloadPath(projectID string, fileName string) (string, error) {
	if fileName == "" || strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
		return "", fmt.Errorf("invalid file name")
	}
	projectPath := filepath.Join(techStep6ExportRoot(), sanitizeStep6PathPart(projectID), fileName)
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath, nil
	}
	commercePath := filepath.Join(step6ExportRoot(), sanitizeStep6PathPart(projectID), fileName)
	if _, err := os.Stat(commercePath); err == nil {
		return commercePath, nil
	}
	legacyTmpPath := filepath.Join(os.TempDir(), fileName)
	if _, err := os.Stat(legacyTmpPath); err == nil {
		return legacyTmpPath, nil
	}
	return "", os.ErrNotExist
}

func techStep6ExportRoot() string {
	if root := os.Getenv("TECH_BID_EXPORT_DIR"); root != "" {
		return root
	}
	return filepath.Join("data", "exports", "tech_bid_projects")
}

func sToJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (h *TechBidProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var input struct {
		ProjectName string `json:"project_name"`
		ProjectType string `json:"project_type"`
		Profession  string `json:"profession"`
		TenderCode  string `json:"tender_code"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	query := `UPDATE tech_bid_projects SET project_name = ?, project_type = ?, profession = ?, tender_code = ?, updated_at = ? 
              WHERE id = ? AND company_id = ?`
	_, err := h.db.Exec(query, input.ProjectName, input.ProjectType, input.Profession, input.TenderCode, time.Now(), id, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *TechBidProjectHandler) DeleteProject(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	tx := h.db.MustBegin()

	// Cleanup all project-dependent data
	tables := []string{
		"tech_bid_chapter_contents",
		"tech_bid_chapter_plans",
		"tech_bid_outline_facts",
		"tech_bid_fact_mappings",
		"tech_bid_outline_coverage_checks",
		"tech_bid_requirement_register",
		"tech_bid_requirement_response_checks",
		"tech_bid_step4_gate_overrides",
		"tech_bid_outline_audits",
		"tech_bid_outline_verifications",
		"tech_bid_tender_files",
		"tech_bid_versions",
		"tech_bid_risk_records",
		"tech_bid_project_profiles",
		"tech_bid_outputs",
	}

	for _, table := range tables {
		_, _ = tx.Exec("DELETE FROM "+table+" WHERE project_id = ?", id)
	}

	// Delete the main project
	_, err := tx.Exec("DELETE FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID)
	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, "删除项目失败: "+err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, "提交删除事务失败: "+err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}
func (h *TechBidProjectHandler) AddProjectFile(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		FileAssetID string `json:"file_asset_id" binding:"required"`
		FileRole    string `json:"file_role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Fetch file info from file_asset to sync names and paths
	var asset struct {
		FileName   string `db:"file_name"`
		StoredPath string `db:"stored_path"`
	}
	err := h.db.Get(&asset, "SELECT file_name, stored_path FROM file_asset WHERE id = ?", input.FileAssetID)
	if err != nil {
		Error(c, http.StatusNotFound, "File asset not found in library: "+err.Error())
		return
	}

	fid := uuid.New().String()
	query := `INSERT INTO tech_bid_tender_files (
		id, project_id, file_asset_id, file_name, stored_path, file_role, 
		parse_status, source_type, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, 'pending', 'upload', ?, ?)`

	now := time.Now()
	_, err = h.db.Exec(query, fid, id, input.FileAssetID, asset.FileName, asset.StoredPath, input.FileRole, now, now)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to associate file: "+err.Error())
		return
	}

	c.JSON(201, gin.H{"id": fid, "file_id": fid})
}
func (h *TechBidProjectHandler) RunVerification(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[TechBid] RunVerification called for project: %s", id)

	// Update status and CLEAR error message
	_, err := h.db.Exec("UPDATE tech_bid_projects SET current_step_status = 'running', last_error_message = '', updated_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	go func() {
		// 1. Get context
		var project model.TechBidProject
		_ = h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)

		plans := []model.TechBidChapterPlan{}
		h.db.Unsafe().Select(&plans, "SELECT * FROM tech_bid_chapter_plans WHERE project_id = ? ORDER BY chapter_order ASC", id)
		outlineJSON, _ := json.Marshal(plans)

		// 2. Load context needed for facts extraction if missing
		var profile model.TechBidProjectProfile
		h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
		profileJSON := "{}"
		if profile.ProfileJSON != nil {
			profileJSON = *profile.ProfileJSON
		}

		var tenderFile struct {
			FileAssetID string `db:"file_asset_id"`
		}
		h.db.Get(&tenderFile, "SELECT file_asset_id FROM tech_bid_tender_files WHERE project_id = ? AND file_role = 'tender' LIMIT 1", id)
		tenderContent := ""
		if tenderFile.FileAssetID != "" {
			var content struct {
				MarkdownText *string `db:"markdown_text"`
			}
			h.db.Get(&content, "SELECT markdown_text FROM file_content WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1", tenderFile.FileAssetID)
			if content.MarkdownText != nil {
				tenderContent = *content.MarkdownText
			}
		}

		// 3. Get Facts from DB (or re-extract if missing)
		var facts []model.TechBidOutlineFact
		err = h.db.Select(&facts, "SELECT * FROM tech_bid_outline_facts WHERE project_id = ?", id)
		if err != nil || len(facts) == 0 {
			log.Printf("[TechBid] Facts missing for project %s. Attempting auto-recovery extraction.", id)
			h.updateProjectState(id, nil, nil, "running", "running", "ai", nil, "", "ai", "正在自动补全核验事实库 (Auto-Recovery)...", nil)

			// Use a context with timeout for safety
			extCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			recoveredFacts, extErr := h.digitizeService.ExtractOutlineFacts(extCtx, id, tenderContent)
			cancel()

			if extErr != nil {
				log.Printf("[TechBid] Auto-recovery extraction failed: %v", extErr)
				h.updateProjectState(id, nil, nil, "running", "failed", "system", nil, "", "", "核验执行失败: OUTLINE_FACTS_MISSING (事实库缺失且自动补全失败)", nil)
				return
			}

			// Save Recovered Facts to DB
			h.db.Exec("DELETE FROM tech_bid_outline_facts WHERE project_id = ?", id)
			for _, item := range recoveredFacts.ScoreItems {
				h.db.Exec(`INSERT INTO tech_bid_outline_facts (id, project_id, fact_code, fact_type, fact_name, fact_content, source_text, priority, score_value) VALUES (?, ?, ?, 'score_item', ?, ?, ?, ?, ?)`, uuid.New().String(), id, item.ID, item.Name, item.Content, item.SourceText, item.Priority, item.ScoreValue)
			}
			for _, item := range recoveredFacts.MandatorySpecs {
				h.db.Exec(`INSERT INTO tech_bid_outline_facts (id, project_id, fact_code, fact_type, fact_name, fact_content, source_text, priority) VALUES (?, ?, ?, 'mandatory_spec', ?, ?, ?, ?)`, uuid.New().String(), id, item.ID, item.Name, item.Content, item.SourceText, item.Priority)
			}
			for _, item := range recoveredFacts.ProjectCharacteristics {
				h.db.Exec(`INSERT INTO tech_bid_outline_facts (id, project_id, fact_code, fact_type, fact_name, fact_content, priority) VALUES (?, ?, ?, 'project_characteristic', ?, ?, ?)`, uuid.New().String(), id, item.ID, item.Name, item.Content, item.Priority)
			}
			for _, item := range recoveredFacts.SpecialTopics {
				h.db.Exec(`INSERT INTO tech_bid_outline_facts (id, project_id, fact_code, fact_type, fact_name, fact_content, priority) VALUES (?, ?, ?, 'special_topic', ?, ?, ?)`, uuid.New().String(), id, item.ID, item.Name, item.Content, item.Priority)
			}

			// Reload facts after recovery
			h.db.Select(&facts, "SELECT * FROM tech_bid_outline_facts WHERE project_id = ?", id)
		}

		// Convert model facts to service facts structure
		factResult := &service.FactExtractResult{}
		for _, f := range facts {
			fid := f.ID
			if f.FactCode != nil && strings.TrimSpace(*f.FactCode) != "" {
				fid = *f.FactCode
			}
			item := service.FactItem{
				ID:         fid,
				Name:       derefString(f.FactName),
				Content:    derefString(f.FactContent),
				SourceText: derefString(f.SourceText),
				Priority:   derefString(f.Priority),
			}
			if f.ScoreValue != nil {
				item.ScoreValue = *f.ScoreValue
			}

			switch f.FactType {
			case "score_item":
				factResult.ScoreItems = append(factResult.ScoreItems, item)
			case "mandatory_spec":
				factResult.MandatorySpecs = append(factResult.MandatorySpecs, item)
			case "project_characteristic":
				factResult.ProjectCharacteristics = append(factResult.ProjectCharacteristics, item)
			case "special_topic":
				factResult.SpecialTopics = append(factResult.SpecialTopics, item)
			}
		}

		// Note: Immortality Rule (P0-4) - We NO LONGER delete old data here.
		h.db.Exec("UPDATE tech_bid_projects SET step5_status = 'verifying' WHERE id = ?", id)

		// 4. Get Latest Audit from Step 4 for context
		var priorAudit service.CoverageAuditResult
		var auditModel model.TechBidOutlineAudit
		aErr := h.db.Get(&auditModel, "SELECT * FROM tech_bid_outline_audits WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
		if aErr == nil {
			if auditModel.MissingItemsJSON != nil {
				json.Unmarshal([]byte(*auditModel.MissingItemsJSON), &priorAudit.MissingItems)
			}
			if auditModel.WeakItemsJSON != nil {
				json.Unmarshal([]byte(*auditModel.WeakItemsJSON), &priorAudit.WeakItems)
			}
			priorAudit.CoverageScore = auditModel.CoverageScore
			if auditModel.AuditSummary != nil {
				priorAudit.AuditSummary = *auditModel.AuditSummary
			}
		}

		frJSON := latestFullResponseCheckJSON(h.db, id)

		// Call Final Verification Service (Doubao Expert)，并传入 Step4 完全响应硬门槛 JSON
		verifyResult, err := h.digitizeService.VerifyChapterOutline(context.Background(), id, factResult, string(outlineJSON), &priorAudit, profileJSON, tenderContent, "", "", "doubao-pro-32k", frJSON)
		if err != nil {
			log.Printf("[TechBid] Verification audit failed: %v", err)
			h.updateProjectState(id, nil, nil, "running", "failed", "ai", nil, "", "ai", "专家终审执行失败: "+err.Error(), nil)
			return
		}

		// 系统钳制：未强制放行时，Step4 完全响应硬门槛非 PASS 则不得终审放行
		var projGate model.TechBidProject
		_ = h.db.Get(&projGate, "SELECT step4_gate_result, override_enabled, step4_override_enabled FROM tech_bid_projects WHERE id = ?", id)
		if projGate.OverrideEnabled == 0 && projGate.Step4OverrideEnabled == 0 && projGate.Step4GateResult != nil {
			g := strings.TrimSpace(*projGate.Step4GateResult)
			if g == "BLOCK" {
				verifyResult.FinalDecision = "BLOCK"
				verifyResult.CanProceed = false
				if verifyResult.Summary != "" {
					verifyResult.Summary += " "
				}
				verifyResult.Summary += "（系统：Step4 完全响应硬门槛为 BLOCK，终审不得放行。）"
			} else if g == "REVISE" && verifyResult.FinalDecision == "PASS" {
				verifyResult.FinalDecision = "REVISE"
				verifyResult.CanProceed = false
				if verifyResult.Summary != "" {
					verifyResult.Summary += " "
				}
				verifyResult.Summary += "（系统：Step4 完全响应硬门槛未达 PASS，已降级为 REVISE。）"
			}
		}

		// 7. Save structured verification result
		verificationID := uuid.New().String()
		critJSON, _ := json.Marshal(verifyResult.CriticalIssues)
		majorJSON, _ := json.Marshal(verifyResult.MajorIssues)
		suggestJSON, _ := json.Marshal(verifyResult.SuggestedActions)

		var auditIDPtr *string
		if aErr == nil {
			auditIDPtr = &auditModel.ID
		}

		db.EnsureTechBidOutlineVerificationsSchema(h.db)
		_, err = h.db.Exec(`INSERT INTO tech_bid_outline_verifications 
				(id, project_id, audit_id, final_decision, risk_level, summary, critical_issues_json, major_issues_json, suggested_actions_json, can_proceed, verification_method, verification_model) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'ai', ?)`,
			verificationID, id, auditIDPtr, verifyResult.FinalDecision, verifyResult.RiskLevel, verifyResult.Summary,
			string(critJSON), string(majorJSON), string(suggestJSON),
			map[bool]int{true: 1, false: 0}[verifyResult.CanProceed], "doubao-pro-32k")
		if err != nil {
			log.Printf("[TechBid] Persist outline verification failed: %v", err)
			h.updateProjectState(id, nil, nil, "running", "failed", "system", nil, "", "db", "专家终审结果保存失败: "+err.Error(), nil)
			return
		}

		// 8. Update project status based on result using audited helper
		step5Status := "verified_pass"
		canProceed := 0
		reason := "专家终审完成"
		if verifyResult.FinalDecision == "PASS" {
			canProceed = 1
		} else if verifyResult.FinalDecision == "REVISE" {
			step5Status = "verified_revise"
			reason = "专家审核建议: REVISE"
		} else if verifyResult.FinalDecision == "BLOCK" {
			step5Status = "verified_block"
			reason = "专家审核建议: BLOCK (严重风险)"
		}

		h.updateProjectState(id, project.CurrentStep, project.CurrentStep, "running", "success", "ai", nil, "", "ai", reason, map[string]interface{}{
			"final_decision": verifyResult.FinalDecision,
			"risk_level":     verifyResult.RiskLevel,
			"can_proceed":    canProceed,
		})

		h.db.Exec(`UPDATE tech_bid_projects 
			SET verification_result = ?, 
			    verification_summary = ?,
			    final_decision = ?,
			    risk_level = ?,
			    coverage_score = ?, 
			    step5_status = ?, 
			    can_enter_content_generation = ?,
			    updated_at = ? 
			WHERE id = ?`,
			verifyResult.Summary, verifyResult.Summary, verifyResult.FinalDecision, verifyResult.RiskLevel,
			priorAudit.CoverageScore, step5Status, canProceed, time.Now(), id)
	}()

	c.JSON(200, gin.H{"success": true})
}

func (h *TechBidProjectHandler) ManualOverrideVerification(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason     string `json:"reason"`
		OperatorID string `json:"operator_id"` // Optional from frontend
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	// 1. Get snapshot before override
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id); err != nil {
		c.JSON(404, gin.H{"error": "project not found"})
		return
	}
	snapshotJSON, _ := json.Marshal(project)
	origStatus := derefString(project.ProjectStatus)

	// 2. Perform override update using helper for consistency and audit trail
	result := fmt.Sprintf("[人工强制解锁] %s", req.Reason)
	h.updateProjectState(id, project.CurrentStep, project.CurrentStep, origStatus, "success", "manual", &req.OperatorID, "Expert", "manual_override", result, nil)

	now := time.Now()
	_, err := h.db.Exec(`UPDATE tech_bid_projects 
		SET step5_status = 'verified_override', 
		    verification_method = 'manual_override',
		    override_enabled = 1,
		    override_reason = ?,
		    override_at = ?,
		    verification_result = ?, 
		    manual_lock = 0,
		    can_enter_content_generation = 1,
		    updated_at = ? 
		WHERE id = ?`,
		req.Reason, now, result, now, id)

	if err != nil {
		c.JSON(500, gin.H{"error": "failed to update project state"})
		return
	}

	// 3. Log to manual_overrides (Historical Table)
	overrideID := uuid.New().String()
	h.db.Exec(`INSERT INTO tech_bid_manual_overrides 
		(id, project_id, original_status, target_status, operator_id, reason, snapshot_before_override_json) 
		VALUES (?, ?, ?, 'verified_override', ?, ?, ?)`,
		overrideID, id, origStatus, req.OperatorID, req.Reason, string(snapshotJSON))

	// 5. Also insert a trace record in audits (Step 4 history)
	auditID := uuid.New().String()
	h.db.Exec(`INSERT INTO tech_bid_outline_audits 
		(id, project_id, coverage_score, audit_summary, final_decision, risk_level, can_proceed) 
		VALUES (?, ?, 100, ?, 'PASS', 'LOW', 1)`,
		auditID, id, result)

	c.JSON(200, gin.H{"success": true, "message": "Manual override successful"})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *TechBidProjectHandler) OptimizeOutline(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Suggestions string `json:"suggestions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Update status
	_, err := h.db.Exec("UPDATE tech_bid_projects SET current_step_status = 'running', updated_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	go func() {
		// 1. Get context
		var project model.TechBidProject
		_ = h.db.Unsafe().Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ?", id)

		var profile model.TechBidProjectProfile
		_ = h.db.Unsafe().Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)
		profileData := ""
		if profile.ProfileJSON != nil {
			profileData = *profile.ProfileJSON
		}

		plans := []model.TechBidChapterPlan{}
		h.db.Unsafe().Select(&plans, "SELECT * FROM tech_bid_chapter_plans WHERE project_id = ? ORDER BY chapter_order ASC", id)
		outlineJSON, _ := json.Marshal(plans)

		var tenderFile struct {
			FileAssetID string `db:"file_asset_id"`
		}
		fErr := h.db.Get(&tenderFile, "SELECT file_asset_id FROM tech_bid_tender_files WHERE project_id = ? AND file_role = 'tender' LIMIT 1", id)

		tenderContent := ""
		if fErr == nil {
			var content struct {
				MarkdownText *string `db:"markdown_text"`
			}
			h.db.Get(&content, "SELECT markdown_text FROM file_content WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1", tenderFile.FileAssetID)
			if content.MarkdownText != nil {
				tenderContent = *content.MarkdownText
			}
		}

		// 2. Call optimization
		optimized, err := h.digitizeService.OptimizeChapterOutline(context.Background(), id, string(outlineJSON), input.Suggestions, profileData, tenderContent)
		if err != nil {
			log.Printf("[TechBid] Optimization failed: %v", err)
			h.db.Exec("UPDATE tech_bid_projects SET current_step_status = 'failed', last_error_message = ? WHERE id = ?", err.Error(), id)
			return
		}

		// 3. Save optimized outline (Chapter Plans)
		tx := h.db.MustBegin()
		_, _ = tx.Exec("DELETE FROM tech_bid_chapter_contents WHERE project_id = ?", id)
		_, _ = tx.Exec("DELETE FROM tech_bid_chapter_plans WHERE project_id = ?", id)

		for i, ch := range optimized {
			chapterID := uuid.New().String()
			chapterName := ""
			if n, ok := ch["name"].(string); ok {
				chapterName = n
			}
			_, _ = tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, chapter_name, chapter_order, node_level, generation_status) VALUES (?, ?, ?, ?, 'chapter', 'completed')`, chapterID, id, chapterName, i+1)

			units, _ := ch["units"].([]interface{})
			for j, u := range units {
				uMap, _ := u.(map[string]interface{})
				unitID := uuid.New().String()
				unitName, _ := uMap["name"].(string)
				_, _ = tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status) VALUES (?, ?, ?, ?, ?, 'unit', 'completed')`, unitID, id, chapterID, unitName, j+1)

				subs, _ := uMap["subsections"].([]interface{})
				for k, s := range subs {
					subName, _ := s.(string)
					_, _ = tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status) VALUES (?, ?, ?, ?, ?, 'subsection', 'not_started')`, uuid.New().String(), id, unitID, subName, k+1)
				}
			}
		}

		_, _ = tx.Exec("UPDATE tech_bid_projects SET current_step_status = 'success', verification_result = ?, updated_at = ? WHERE id = ?", input.Suggestions, time.Now(), id)

		if err := tx.Commit(); err != nil {
			log.Printf("[TechBid] Optimization commit failed: %v", err)
		}
	}()

	c.JSON(200, gin.H{"success": true})
}

// GetOutlineFactMappings 返回 Step4 已持久化的 facts→目录映射（强映射表）
func (h *TechBidProjectHandler) GetOutlineFactMappings(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	if runID, err := store.ResolveAuthoritativeStep4RunID(id); err == nil {
		if out, err2 := store.ListFactMappingsForAPI(runID, id); err2 == nil && len(out) > 0 {
			c.JSON(200, gin.H{"success": true, "source": "step4", "run_id": runID, "mappings": out})
			return
		}
	}

	type mappingWithEvidence struct {
		model.TechBidFactMapping
		SourceChapter  *string `db:"source_chapter"`
		PageNumber     *int    `db:"page_number"`
		LineNumber     *int    `db:"line_number"`
		SourceLocation *string `db:"source_location"`
	}

	var rows []mappingWithEvidence
	query := `
		SELECT
			m.id,
			m.project_id,
			m.fact_id,
			m.fact_type,
			COALESCE(f.fact_name, m.fact_name) AS fact_name,
			m.target_level,
			m.target_path_json,
			m.required,
			m.priority,
			m.mapping_reason,
			m.mapping_source,
			m.created_at,
			f.source_location,
			f.source_section AS source_chapter,
			f.source_page AS page_number,
			f.source_line AS line_number
		FROM tech_bid_fact_mappings m
		LEFT JOIN tech_bid_outline_facts f ON m.project_id = f.project_id AND m.fact_id = f.fact_code
		WHERE m.project_id = ?
		ORDER BY m.fact_id ASC`

	if err := h.db.Select(&rows, query, id); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		var path []string
		if strings.TrimSpace(r.TargetPathJSON) != "" {
			_ = json.Unmarshal([]byte(r.TargetPathJSON), &path)
		}
		mr := ""
		if r.MappingReason != nil {
			mr = *r.MappingReason
		}

		res := map[string]interface{}{
			"id":              r.ID,
			"fact_id":         r.FactID,
			"fact_type":       r.FactType,
			"fact_name":       r.FactName,
			"target_level":    r.TargetLevel,
			"target_path":     path,
			"required":        r.Required != 0,
			"priority":        r.Priority,
			"mapping_reason":  mr,
			"mapping_source":  r.MappingSource,
			"created_at":      r.CreatedAt,
			"source_chapter":  r.SourceChapter,
			"page_number":     r.PageNumber,
			"line_number":     r.LineNumber,
			"source_location": r.SourceLocation,
		}
		out = append(out, res)
	}
	for i := range out {
		out[i]["source"] = "legacy"
	}
	c.JSON(200, gin.H{"success": true, "source": "legacy", "mappings": out})
}

// GetOutlineCoverageLatest 返回最近一次结构化覆盖率校验结果
func (h *TechBidProjectHandler) GetOutlineCoverageLatest(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	if cov, err := store.LatestCompletedStep4Coverage(id); err == nil {
		c.JSON(200, gin.H{"success": true, "coverage": cov})
		return
	}
	var row model.TechBidOutlineCoverageCheck
	err := h.db.Get(&row, `SELECT * FROM tech_bid_outline_coverage_checks WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id)
	if err != nil {
		c.JSON(200, gin.H{"success": true, "coverage": nil})
		return
	}
	parseStrArr := func(p *string) []string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []string
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	c.JSON(200, gin.H{"success": true, "coverage": gin.H{
		"id":                   row.ID,
		"outline_version":      row.OutlineVersion,
		"fact_total":           row.FactTotal,
		"fact_mapped":          row.FactMapped,
		"coverage_rate":        row.CoverageRate,
		"missing_fact_ids":     parseStrArr(row.MissingFactIDsJSON),
		"weak_fact_ids":        parseStrArr(row.WeakFactIDsJSON),
		"duplicate_node_hints": parseStrArr(row.DuplicateNodeIDsJSON),
		"result":               row.Result,
		"summary":              summary,
		"created_at":           row.CreatedAt,
		"source":               "legacy",
	}})
}

// GetRequirementRegister 返回 Step4 招标要求总表（持久化行）
func (h *TechBidProjectHandler) GetRequirementRegister(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	if runID, err := store.ResolveAuthoritativeStep4RunID(id); err == nil {
		if out, err2 := store.ListRequirementsAPISnapshot(runID); err2 == nil && len(out) > 0 {
			c.JSON(200, gin.H{"success": true, "source": "step4", "run_id": runID, "requirements": out})
			return
		}
	}
	var rows []model.TechBidRequirementRegister
	if err := h.db.Select(&rows, `SELECT * FROM tech_bid_requirement_register WHERE project_id = ? ORDER BY requirement_id ASC`, id); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":                      r.ID,
			"requirement_id":          r.RequirementID,
			"requirement_type":        r.RequirementType,
			"source_text":             r.SourceText,
			"source_location":         r.SourceLocation,
			"priority":                r.Priority,
			"must_be_explicit":        r.MustBeExplicit,
			"expected_response_level": r.ExpectedResponseLevel,
			"domain":                  r.Domain,
			"response_tier":           r.ResponseTier,
			"summary":                 r.Summary,
			"created_at":              r.CreatedAt,
			"source":                  "legacy",
		})
	}
	c.JSON(200, gin.H{"success": true, "source": "legacy", "requirements": out})
}

// GetOutlineFactCandidates 返回当前权威 run 的 step4_fact_candidates（CTO Fact Agent 工件）。
func (h *TechBidProjectHandler) GetOutlineFactCandidates(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	runID, err := store.ResolveAuthoritativeStep4RunID(id)
	if err != nil {
		c.JSON(200, gin.H{"success": true, "source": "none", "run_id": nil, "candidates": []interface{}{}})
		return
	}
	list, err := store.ListFactCandidatesForRun(runID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true, "source": "step4", "run_id": runID, "candidates": list})
}

// GetFullRequirementResponseLatest 返回最近一次「完全响应率」校验结果
func (h *TechBidProjectHandler) GetFullRequirementResponseLatest(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	if fr, err := store.LatestCompletedStep4FullResponse(id); err == nil {
		c.JSON(200, gin.H{"success": true, "full_response": fr})
		return
	}
	var row model.TechBidRequirementResponseCheck
	err := h.db.Get(&row, `SELECT * FROM tech_bid_requirement_response_checks WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id)
	if err != nil {
		c.JSON(200, gin.H{"success": true, "full_response": nil})
		return
	}
	parseStrArr := func(p *string) []string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []string
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	c.JSON(200, gin.H{"success": true, "full_response": gin.H{
		"id":                           row.ID,
		"outline_version":              row.OutlineVersion,
		"requirement_total":            row.RequirementTotal,
		"requirement_mapped":           row.RequirementMapped,
		"requirement_fully_responded":  row.RequirementFullyResponded,
		"requirement_weakly_responded": row.RequirementWeaklyResponded,
		"requirement_only_tagged":      row.RequirementOnlyTagged,
		"full_response_rate":           row.FullResponseRate,
		"weak_response_rate":           row.WeakResponseRate,
		"response_quality_score":       row.ResponseQualityScore,
		"missing_requirement_ids":      parseStrArr(row.MissingRequirementIDsJSON),
		"weak_requirement_ids":         parseStrArr(row.WeakRequirementIDsJSON),
		"only_tagged_requirement_ids":  parseStrArr(row.OnlyTaggedRequirementIDsJSON),
		"shell_title_hints":            parseStrArr(row.ShellTitleHintsJSON),
		"high_priority_missing_ids":    parseStrArr(row.HighPriorityMissingIDsJSON),
		"mandatory_missing_ids":        parseStrArr(row.MandatoryMissingIDsJSON),
		"mandatory_insufficient_ids":   parseStrArr(row.MandatoryInsufficientIDsJSON),
		"hard_rule_warnings":           parseStrArr(row.HardRuleWarningsJSON),
		"result":                       row.Result,
		"summary":                      summary,
		"created_at":                   row.CreatedAt,
		"source":                       "legacy",
	}})
}

// ManualOverrideStep4Gate Step4 完全响应硬门槛人工放行（独立留痕，与 Step5 强制放行区分）
func (h *TechBidProjectHandler) ManualOverrideStep4Gate(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)
	var req struct {
		Reason     string `json:"reason" binding:"required"`
		OperatorID string `json:"operator_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, cid); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	summary := ""
	if project.Step4GateReason != nil {
		summary = *project.Step4GateReason
	}
	snap := map[string]interface{}{
		"step4_gate_result":  derefString(project.Step4GateResult),
		"full_response_rate": project.FullResponseRate,
		"step4_gate_reason":  summary,
		"final_decision":     derefString(project.FinalDecision),
	}
	snapJSON, _ := json.Marshal(snap)
	now := time.Now()
	ovID := uuid.New().String()
	if _, err := h.db.Exec(`INSERT INTO tech_bid_step4_gate_overrides (id, project_id, company_id, operator_id, reason, gate_snapshot_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ovID, id, cid, req.OperatorID, req.Reason, string(snapJSON), now); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	op := req.OperatorID
	if op == "" {
		op = "unknown"
	}
	if _, err := h.db.Exec(`UPDATE tech_bid_projects SET step4_override_enabled = 1, step4_override_reason = ?, step4_override_by = ?, step4_override_at = ?, updated_at = ? WHERE id = ?`,
		req.Reason, op, now, now, id); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.logStep4Approval(id, "step4_gate", "override", op, req.Reason)
	c.JSON(200, gin.H{"success": true, "message": "已记录 Step4 门槛人工放行"})
}

// GetOutlineConflictAuditLatest 返回最近一次「逻辑冲突审计」结果
func (h *TechBidProjectHandler) GetOutlineConflictAuditLatest(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	store := service.NewStep4Store(h.db)
	if aud, err := store.LatestCompletedStep4Conflict(id); err == nil {
		c.JSON(200, gin.H{"success": true, "audit": aud})
		return
	}
	var row model.TechBidConflictAudit
	if err := h.db.Get(&row, `SELECT * FROM tech_bid_conflict_audit WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id); err != nil {
		c.JSON(200, gin.H{"success": true, "audit": nil})
		return
	}

	parseConflicts := func(p *string) []interface{} {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []interface{}
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}

	c.JSON(200, gin.H{"success": true, "audit": gin.H{
		"id":         row.ID,
		"project_id": row.ProjectID,
		"has_block":  row.HasBlock == 1,
		"conflicts":  parseConflicts(row.ConflictJSON),
		"summary":    row.Summary,
		"created_at": row.CreatedAt,
		"source":     "legacy",
	}})
}

// GetStructurePlan 返回最近生成的结构调整计划
func (h *TechBidProjectHandler) GetStructurePlan(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var cnt int
	if err := h.db.Get(&cnt, "SELECT COUNT(*) FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil || cnt == 0 {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}
	var row struct {
		ID                string    `db:"id"`
		AdjustmentsJSON   string    `db:"adjustments_json"`
		TenderProfileJSON *string   `db:"tender_profile_json"`
		Rationale         *string   `db:"rationale"`
		Status            string    `db:"status"`
		RejectReason      *string   `db:"reject_reason"`
		CreatedAt         time.Time `db:"created_at"`
	}
	err := h.db.Get(&row, `SELECT id, adjustments_json, tender_profile_json, rationale, status, reject_reason, created_at FROM tech_bid_structure_plan WHERE project_id = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, id)
	if err != nil {
		c.JSON(200, gin.H{"plan": nil})
		return
	}

	var adjustments []interface{}
	_ = json.Unmarshal([]byte(row.AdjustmentsJSON), &adjustments)
	var profile interface{}
	if row.TenderProfileJSON != nil {
		_ = json.Unmarshal([]byte(*row.TenderProfileJSON), &profile)
	}

	c.JSON(200, gin.H{"plan": gin.H{
		"id":            row.ID,
		"adjustments":   adjustments,
		"profile":       profile,
		"rationale":     derefString(row.Rationale),
		"status":        row.Status,
		"created_at":    row.CreatedAt,
		"reject_reason": derefString(row.RejectReason),
	}})
}

// ApproveStructurePlan 用户审核通过，触发后续目录生成
func (h *TechBidProjectHandler) ApproveStructurePlan(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	var plan struct {
		ID              string `db:"id"`
		AdjustmentsJSON string `db:"adjustments_json"`
	}
	if err := h.db.Get(&plan, `SELECT id, adjustments_json FROM tech_bid_structure_plan WHERE project_id = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, id); err != nil {
		Error(c, http.StatusConflict, "没有可审批的结构计划")
		return
	}

	res, err := h.db.Exec(`UPDATE tech_bid_structure_plan SET status = 'approved', approve_reason = COALESCE(approve_reason, 'approved by reviewer') WHERE id = ? AND status = 'pending'`, plan.ID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update plan status")
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusConflict, "结构计划已失效，请刷新后重试")
		return
	}
	now := time.Now()
	_, _ = h.db.Exec(`UPDATE step4_structure_plans SET status = 'approved', approved_at = ? WHERE id = ?`, now, plan.ID)

	if _, err = h.db.Exec(`UPDATE tech_bid_projects SET structure_plan_status = 'approved', current_step_status = 'running', step4_status = 'generating_outline', updated_at = ? WHERE id = ?`, now, id); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update project status")
		return
	}

	go func() {
		if err := h.continueOutlineGenerationFromApprovedPlan(id, "", plan.AdjustmentsJSON); err != nil {
			log.Printf("[TechBid] Resume outline generation after approval failed for %s: %v", id, err)
		}
	}()

	h.logStep4Approval(id, "structure_plan", "approve", "", "approved pending structure plan")
	c.JSON(200, gin.H{"success": true, "message": "已批准结构计划，正在恢复目录生成..."})
}

// RejectStructurePlan 用户拒绝结构调整计划，触发重规划
func (h *TechBidProjectHandler) RejectStructurePlan(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		req.Reason = "未填写拒绝原因"
	}

	var plan struct {
		ID string `db:"id"`
	}
	if err := h.db.Get(&plan, `SELECT id FROM tech_bid_structure_plan WHERE project_id = ? AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, id); err != nil {
		Error(c, http.StatusConflict, "没有可驳回的结构计划")
		return
	}

	res, err := h.db.Exec(`UPDATE tech_bid_structure_plan SET status = 'rejected', reject_reason = ? WHERE id = ? AND status = 'pending'`, req.Reason, plan.ID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update plan status")
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusConflict, "结构计划已失效，请刷新后重试")
		return
	}

	_, err = h.db.Exec(`UPDATE tech_bid_projects SET structure_plan_status = 'rejected', current_step_status = 'running', step4_status = 'generating_outline', last_error_message = ?, last_structure_reject_reason = ?, updated_at = ? WHERE id = ? AND company_id = ?`,
		"用户拒绝了上一个结构计划: "+req.Reason, req.Reason, time.Now(), id, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to reset project status")
		return
	}

	h.logStep4Approval(id, "structure_plan", "reject", "", req.Reason)
	c.JSON(200, gin.H{"success": true, "message": "已拒绝计划，请重新选择路线或重新触发目录生成。"})
}

// setProfileFieldByPath sets a profile field value by its dot-path.
func setProfileFieldByPath(profile *service.ProjectProfileResult, path string, value string, sourceText string, sourceLocation string) (service.ProjectProfileField, bool) {
	newField := func(old service.ProjectProfileField) service.ProjectProfileField {
		f := service.ProjectProfileField{
			Value:          value,
			SourceText:     sourceText,
			SourceLocation: sourceLocation,
			Confidence:     1.0,
			Missing:        false,
			Notes:          "人工补录",
		}
		if f.SourceText == "" {
			f.SourceText = old.SourceText
		}
		if f.SourceLocation == "" {
			f.SourceLocation = old.SourceLocation
		}
		return f
	}

	switch path {
	case "project_base_info.project_name":
		old := profile.ProjectBaseInfo.ProjectName
		profile.ProjectBaseInfo.ProjectName = newField(old)
		return old, true
	case "project_base_info.owner_unit":
		old := profile.ProjectBaseInfo.OwnerUnit
		profile.ProjectBaseInfo.OwnerUnit = newField(old)
		return old, true
	case "project_base_info.location":
		old := profile.ProjectBaseInfo.Location
		profile.ProjectBaseInfo.Location = newField(old)
		return old, true
	case "project_base_info.category_and_scope":
		old := profile.ProjectBaseInfo.CategoryAndScope
		profile.ProjectBaseInfo.CategoryAndScope = newField(old)
		return old, true
	case "project_base_info.duration_requirements":
		old := profile.ProjectBaseInfo.DurationRequirements
		profile.ProjectBaseInfo.DurationRequirements = newField(old)
		return old, true
	case "project_base_info.quality_standard":
		old := profile.ProjectBaseInfo.QualityStandard
		profile.ProjectBaseInfo.QualityStandard = newField(old)
		return old, true
	case "construction_core_requirements.material_equipment_rules":
		old := profile.ConstructionCoreRequirements.MaterialEquipmentRules
		profile.ConstructionCoreRequirements.MaterialEquipmentRules = newField(old)
		return old, true
	case "construction_core_requirements.technical_specifications":
		old := profile.ConstructionCoreRequirements.TechnicalSpecifications
		profile.ConstructionCoreRequirements.TechnicalSpecifications = newField(old)
		return old, true
	case "construction_core_requirements.site_management":
		old := profile.ConstructionCoreRequirements.SiteManagement
		profile.ConstructionCoreRequirements.SiteManagement = newField(old)
		return old, true
	case "construction_core_requirements.acceptance_requirements":
		old := profile.ConstructionCoreRequirements.AcceptanceRequirements
		profile.ConstructionCoreRequirements.AcceptanceRequirements = newField(old)
		return old, true
	case "construction_core_requirements.special_operations":
		old := profile.ConstructionCoreRequirements.SpecialOperations
		profile.ConstructionCoreRequirements.SpecialOperations = newField(old)
		return old, true
	case "construction_core_requirements.procurement_boundary":
		old := profile.ConstructionCoreRequirements.ProcurementBoundary
		profile.ConstructionCoreRequirements.ProcurementBoundary = newField(old)
		return old, true
	case "construction_core_requirements.schedule_constraints":
		old := profile.ConstructionCoreRequirements.ScheduleConstraints
		profile.ConstructionCoreRequirements.ScheduleConstraints = newField(old)
		return old, true
	case "bidder_requirements.qualification_certificates":
		old := profile.BidderRequirements.QualificationCertificates
		profile.BidderRequirements.QualificationCertificates = newField(old)
		return old, true
	case "bidder_requirements.performance_requirements":
		old := service.ProjectProfileField{Value: fmt.Sprintf("%d items", len(profile.BidderRequirements.PerformanceRequirements))}
		profile.BidderRequirements.PerformanceRequirements = []service.ProjectProfileListItem{
			{Name: value, Value: value, Confidence: 1.0, Notes: "人工补录"},
		}
		return old, true
	case "bidder_requirements.qualification_requirements":
		old := service.ProjectProfileField{Value: fmt.Sprintf("%d items", len(profile.BidderRequirements.QualificationRequirements))}
		profile.BidderRequirements.QualificationRequirements = []service.ProjectProfileListItem{
			{Name: value, Value: value, Confidence: 1.0, Notes: "人工补录"},
		}
		return old, true
	case "evaluation_and_performance_rules.method_and_score_weights":
		old := profile.EvaluationAndPerformanceRules.MethodAndScoreWeights
		profile.EvaluationAndPerformanceRules.MethodAndScoreWeights = newField(old)
		return old, true
	case "evaluation_and_performance_rules.technical_evaluation_dimensions":
		old := profile.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions
		profile.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions = newField(old)
		return old, true
	case "evaluation_and_performance_rules.payment_method":
		old := profile.EvaluationAndPerformanceRules.PaymentMethod
		profile.EvaluationAndPerformanceRules.PaymentMethod = newField(old)
		return old, true
	case "evaluation_and_performance_rules.settlement_rules":
		old := profile.EvaluationAndPerformanceRules.SettlementRules
		profile.EvaluationAndPerformanceRules.SettlementRules = newField(old)
		return old, true
	case "evaluation_and_performance_rules.total_duration":
		old := profile.EvaluationAndPerformanceRules.TotalDuration
		profile.EvaluationAndPerformanceRules.TotalDuration = newField(old)
		return old, true
	default:
		return service.ProjectProfileField{}, false
	}
}

// PatchProfileField allows manual correction of a single profile field.
func (h *TechBidProjectHandler) PatchProfileField(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		FieldPath      string `json:"field_path" binding:"required"`
		NewValue       string `json:"new_value" binding:"required"`
		SourceText     string `json:"source_text"`
		SourceLocation string `json:"source_location"`
		OperatorName   string `json:"operator_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Fetch current profile
	var profile model.TechBidProjectProfile
	err := h.db.Get(&profile, `SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id)
	if err != nil {
		Error(c, http.StatusNotFound, "Profile not found")
		return
	}

	if profile.ProfileJSON == nil || *profile.ProfileJSON == "" {
		Error(c, http.StatusNotFound, "Profile JSON is empty")
		return
	}

	var profileResult service.ProjectProfileResult
	if err := json.Unmarshal([]byte(*profile.ProfileJSON), &profileResult); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to parse profile JSON")
		return
	}

	oldField, ok := setProfileFieldByPath(&profileResult, req.FieldPath, req.NewValue, req.SourceText, req.SourceLocation)
	if !ok {
		Error(c, http.StatusBadRequest, "Unknown field path: "+req.FieldPath)
		return
	}

	// Save edit history
	oldJSON, _ := json.Marshal(oldField)
	newJSON, _ := json.Marshal(map[string]string{"value": req.NewValue})
	_, _ = h.db.Exec(`INSERT INTO tech_bid_profile_edit_history (id, project_id, profile_id, field_path, old_value_json, new_value_json, edit_source, operator_name) VALUES (?, ?, ?, ?, ?, ?, 'manual', ?)`,
		uuid.New().String(), id, profile.ID, req.FieldPath, string(oldJSON), string(newJSON), req.OperatorName)

	// Update profile JSON
	updatedJSON, _ := json.Marshal(profileResult)
	_, err = h.db.Exec(`UPDATE tech_bid_project_profiles SET profile_json = ?, edit_count = COALESCE(edit_count, 0) + 1, updated_at = ? WHERE id = ?`,
		string(updatedJSON), time.Now(), profile.ID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to update profile: "+err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true, "field_path": req.FieldPath, "new_value": req.NewValue})
}

// ConfirmProfile marks a profile as manually confirmed.
func (h *TechBidProjectHandler) ConfirmProfile(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		OperatorName string `json:"operator_name"`
	}
	_ = c.ShouldBindJSON(&req)

	result, err := h.db.Exec(`UPDATE tech_bid_project_profiles SET confirmed_at = ?, confirmed_by = ? WHERE project_id = ?`,
		time.Now(), req.OperatorName, id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to confirm profile: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		Error(c, http.StatusNotFound, "Profile not found")
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "画像已标记为人工确认"})
}

// GetProfileEditHistory returns the edit history for a project's profile.
func (h *TechBidProjectHandler) GetProfileEditHistory(c *gin.Context) {
	id := c.Param("id")

	var history []struct {
		ID           string  `db:"id" json:"id"`
		ProjectID    string  `db:"project_id" json:"project_id"`
		ProfileID    string  `db:"profile_id" json:"profile_id"`
		FieldPath    string  `db:"field_path" json:"field_path"`
		OldValueJSON *string `db:"old_value_json" json:"old_value_json"`
		NewValueJSON string  `db:"new_value_json" json:"new_value_json"`
		EditSource   string  `db:"edit_source" json:"edit_source"`
		OperatorName *string `db:"operator_name" json:"operator_name"`
		CreatedAt    string  `db:"created_at" json:"created_at"`
	}

	err := h.db.Select(&history, `SELECT id, project_id, profile_id, field_path, old_value_json, new_value_json, edit_source, operator_name, created_at FROM tech_bid_profile_edit_history WHERE project_id = ? ORDER BY created_at DESC`, id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch edit history: "+err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true, "data": history})
}

// GetProfileExtractionSnapshots returns all extraction snapshots for a project,
// supporting the evidence drawer (Task 9) and run replay (Task 13).
func (h *TechBidProjectHandler) GetProfileExtractionSnapshots(c *gin.Context) {
	id := c.Param("id")
	runID := c.Query("run_id")
	stage := c.Query("stage")

	var snapshots []struct {
		ID          string  `db:"id" json:"id"`
		ProjectID   string  `db:"project_id" json:"project_id"`
		ProfileID   string  `db:"profile_id" json:"profile_id"`
		Stage       string  `db:"stage" json:"stage"`
		ChunkIndex  int     `db:"chunk_index" json:"chunk_index"`
		PayloadJSON string  `db:"payload_json" json:"payload_json"`
		RunID       *string `db:"run_id" json:"run_id"`
		FileID      *string `db:"file_id" json:"file_id"`
		ChunkType   *string `db:"chunk_type" json:"chunk_type"`
		CreatedAt   string  `db:"created_at" json:"created_at"`
	}

	query := `SELECT id, project_id, profile_id, stage, chunk_index, payload_json, run_id, file_id, chunk_type, created_at
		FROM tech_bid_profile_extraction_snapshots WHERE project_id = ?`
	args := []interface{}{id}
	if runID != "" {
		query += ` AND run_id = ?`
		args = append(args, runID)
	}
	if stage != "" {
		query += ` AND stage = ?`
		args = append(args, stage)
	}
	query += ` ORDER BY created_at ASC, chunk_index ASC`

	err := h.db.Select(&snapshots, query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch extraction snapshots: "+err.Error())
		return
	}

	// Parse payload_json into proper JSON objects for the frontend
	type snapshotResponse struct {
		ID         string      `json:"id"`
		ProjectID  string      `json:"project_id"`
		ProfileID  string      `json:"profile_id"`
		Stage      string      `json:"stage"`
		ChunkIndex int         `json:"chunk_index"`
		Payload    interface{} `json:"payload"`
		RunID      *string     `json:"run_id"`
		FileID     *string     `json:"file_id"`
		ChunkType  *string     `json:"chunk_type"`
		CreatedAt  string      `json:"created_at"`
	}

	result := make([]snapshotResponse, 0, len(snapshots))
	for _, s := range snapshots {
		var payload interface{}
		if err := json.Unmarshal([]byte(s.PayloadJSON), &payload); err != nil {
			payload = s.PayloadJSON
		}
		result = append(result, snapshotResponse{
			ID:         s.ID,
			ProjectID:  s.ProjectID,
			ProfileID:  s.ProfileID,
			Stage:      s.Stage,
			ChunkIndex: s.ChunkIndex,
			Payload:    payload,
			RunID:      s.RunID,
			FileID:     s.FileID,
			ChunkType:  s.ChunkType,
			CreatedAt:  s.CreatedAt,
		})
	}

	// Also collect distinct run_ids for the replay selector
	var runIDs []struct {
		RunID     string `db:"run_id" json:"run_id"`
		CreatedAt string `db:"created_at" json:"created_at"`
	}
	_ = h.db.Select(&runIDs, `SELECT DISTINCT run_id, MIN(created_at) as created_at FROM tech_bid_profile_extraction_snapshots WHERE project_id = ? AND run_id IS NOT NULL AND run_id != '' GROUP BY run_id ORDER BY created_at DESC`, id)

	c.JSON(200, gin.H{
		"success": true,
		"data":    result,
		"run_ids": runIDs,
	})
}

// ============================================================================
// Skeleton Candidate Selection APIs (骨架候选选择 API)
// These APIs support human-in-the-loop skeleton confirmation before outline generation
// ============================================================================

// GetSkeletonCandidates returns scored skeleton candidates for the project.
// This should be called before starting Step4 outline generation to allow human confirmation.
func (h *TechBidProjectHandler) GetSkeletonCandidates(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	// Verify project exists and belongs to company
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Get project profile for scoring
	var profile model.TechBidProjectProfile
	_ = h.db.Get(&profile, "SELECT * FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", id)

	// Get tender files for keyword extraction
	var files []model.BidProjectFile
	h.db.Select(&files, `SELECT tbf.*, fa.parse_status FROM tech_bid_tender_files tbf LEFT JOIN file_asset fa ON tbf.file_asset_id = fa.id WHERE tbf.project_id = ?`, id)

	// Load all available skeletons from database
	var skeletons []model.IndustrySkeletonDB
	err := h.db.Select(&skeletons, "SELECT * FROM tech_bid_industry_skeletons ORDER BY industry_name")
	if err != nil || len(skeletons) == 0 {
		// Fallback to hardcoded skeletons
		scorer := service.NewSkeletonScorer()
		fallbackSkeletons := scorer.GetHardcodedSkeletons()
		c.JSON(200, gin.H{
			"success":    true,
			"candidates": fallbackSkeletons,
			"source":     "fallback",
		})
		return
	}

	// Build project profile for scoring
	projProfile := service.ProjectProfile{
		ProjectType: derefString(project.ProjectType),
		Profession:  derefString(project.Profession),
	}

	// Extract keywords from project type
	if project.ProjectType != nil {
		projProfile.Keywords = extractKeywordsFromProjectType(*project.ProjectType)
	}

	// Score all skeletons
	scorer := service.NewSkeletonScorer()
	candidates := scorer.ScoreSkeletons(skeletons, projProfile)

	// Find the recommended skeleton
	var recommended *service.SkeletonCandidate
	for i := range candidates {
		if candidates[i].Recommended {
			recommended = &candidates[i]
			break
		}
	}

	c.JSON(200, gin.H{
		"success":     true,
		"candidates":  candidates,
		"recommended": recommended,
		"source":      "database",
	})
}

// ConfirmSkeleton confirms the selected skeleton and stores it for outline generation.
func (h *TechBidProjectHandler) ConfirmSkeleton(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	var req struct {
		SkeletonID   string `json:"skeleton_id" binding:"required"`
		OperatorName string `json:"operator_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request: skeleton_id is required")
		return
	}

	// Verify project exists
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Verify skeleton exists
	var skeleton model.IndustrySkeletonDB
	if err := h.db.Get(&skeleton, "SELECT * FROM tech_bid_industry_skeletons WHERE id = ?", req.SkeletonID); err != nil {
		Error(c, http.StatusNotFound, "Skeleton not found")
		return
	}

	// Store the confirmed skeleton selection in project metadata or dedicated table
	now := time.Now()

	// Update project with selected skeleton info
	_, err := h.db.Exec(`UPDATE tech_bid_projects SET 
		updated_at = ?,
		last_error_message = NULL
		WHERE id = ?`,
		now, id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to confirm skeleton: "+err.Error())
		return
	}

	// Log the confirmation
	h.logStep4Approval(id, "skeleton_selection", "confirm", "", "selected: "+skeleton.IndustryName)

	// Store in skeleton selection history table (create if not exists)
	db.EnsureSkeletonSelectionSchema(h.db) // Ensure table exists

	_, _ = h.db.Exec(`INSERT INTO tech_bid_skeleton_selections (id, project_id, skeleton_id, skeleton_name, operator_name, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), id, req.SkeletonID, skeleton.IndustryName, req.OperatorName, now)

	c.JSON(200, gin.H{
		"success":       true,
		"message":       "骨架已确认",
		"skeleton_id":   req.SkeletonID,
		"skeleton_name": skeleton.IndustryName,
	})
}

// GetConfirmedSkeleton returns the confirmed skeleton for a project if any.
func (h *TechBidProjectHandler) GetConfirmedSkeleton(c *gin.Context) {
	id := c.Param("id")

	var selection struct {
		ID           string    `db:"id"`
		ProjectID    string    `db:"project_id"`
		SkeletonID   string    `db:"skeleton_id"`
		SkeletonName string    `db:"skeleton_name"`
		OperatorName *string   `db:"operator_name"`
		CreatedAt    time.Time `db:"created_at"`
	}
	err := h.db.Get(&selection, `SELECT * FROM tech_bid_skeleton_selections WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, id)
	if err != nil {
		c.JSON(200, gin.H{
			"success":  true,
			"selected": false,
		})
		return
	}

	// Also fetch the full skeleton data
	var skeleton model.IndustrySkeletonDB
	_ = h.db.Get(&skeleton, "SELECT * FROM tech_bid_industry_skeletons WHERE id = ?", selection.SkeletonID)

	c.JSON(200, gin.H{
		"success":  true,
		"selected": true,
		"selection": gin.H{
			"id":            selection.ID,
			"skeleton_id":   selection.SkeletonID,
			"skeleton_name": selection.SkeletonName,
			"operator_name": selection.OperatorName,
			"created_at":    selection.CreatedAt,
		},
		"skeleton": skeleton,
	})
}

// GetSkeletonCandidatesFromFacts generates skeleton candidates based on extracted facts.
// This is called after Step4 facts are extracted to refine skeleton selection.
func (h *TechBidProjectHandler) GetSkeletonCandidatesFromFacts(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	// Verify project exists
	var project model.TechBidProject
	if err := h.db.Get(&project, "SELECT * FROM tech_bid_projects WHERE id = ? AND company_id = ?", id, companyID); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	// Get facts from latest run
	store := service.NewStep4Store(h.db)
	runID, err := store.ResolveAuthoritativeStep4RunID(id)
	if err != nil {
		Error(c, http.StatusBadRequest, "Facts not yet extracted. Please run outline generation first.")
		return
	}

	// Get fact candidates for keyword analysis
	var factCandidates []struct {
		FactType    string  `db:"fact_type"`
		FactContent *string `db:"fact_content"`
		SourceText  *string `db:"source_text"`
	}
	_ = h.db.Select(&factCandidates, `SELECT fact_type, fact_content, source_text FROM step4_fact_candidates WHERE run_id = ?`, runID)

	// Extract keywords from facts
	keywords := extractKeywordsFromFacts(factCandidates)

	// Load all skeletons
	var skeletons []model.IndustrySkeletonDB
	h.db.Select(&skeletons, "SELECT * FROM tech_bid_industry_skeletons ORDER BY industry_name")

	// Build enhanced profile with facts
	projProfile := service.ProjectProfile{
		ProjectType: derefString(project.ProjectType),
		Profession:  derefString(project.Profession),
		Keywords:    keywords,
	}

	// Detect special chapters from facts
	projProfile.HasSpecials = detectSpecialChaptersFromFacts(factCandidates)

	scorer := service.NewSkeletonScorer()
	candidates := scorer.ScoreSkeletons(skeletons, projProfile)

	c.JSON(200, gin.H{
		"success":    true,
		"candidates": candidates,
		"run_id":     runID,
		"keywords":   keywords,
	})
}

// Helper function: extract keywords from project type string
func extractKeywordsFromProjectType(projectType string) []string {
	if projectType == "" {
		return nil
	}

	keywords := []string{projectType}

	// Add related keywords based on common industry terms
	relatedMap := map[string][]string{
		"水利": {"堤防", "河道", "大坝", "闸站", "灌溉", "防洪", "度汛"},
		"房建": {"建筑", "结构", "装修", "安装", "基础", "主体"},
		"公路": {"道路", "路基", "路面", "桥涵", "隧道", "交通"},
		"市政": {"管网", "道路", "桥梁", "排水", "给水"},
		"铁路": {"轨道", "站场", "隧道", "桥梁", "电气化"},
		"电力": {"输电", "变电", "配电", "线路", "电气"},
	}

	projectTypeLower := strings.ToLower(projectType)
	for industry, related := range relatedMap {
		if strings.Contains(projectTypeLower, industry) {
			keywords = append(keywords, related...)
			break
		}
	}

	return keywords
}

// Helper function: extract keywords from fact candidates
func extractKeywordsFromFacts(facts []struct {
	FactType    string  `db:"fact_type"`
	FactContent *string `db:"fact_content"`
	SourceText  *string `db:"source_text"`
}) []string {
	keywordSet := make(map[string]bool)

	industryTerms := []string{
		"水利", "房建", "公路", "市政", "铁路", "电力", "港口", "航道",
		"堤防", "河道", "大坝", "闸站", "桥梁", "隧道", "管网", "道路",
		"基坑", "桩基", "混凝土", "钢结构", "防水", "绿化", "机电",
	}

	specialTerms := []string{
		"隧道", "盾构", "顶管", "深基坑", "大体积混凝土", "预应力",
		"悬索", "斜拉", "钢箱梁", "波形护栏", "沥青", "改性沥青",
		"顶管", "不开挖", "地下管线", "海绵城市", "综合管廊",
	}

	for _, fact := range facts {
		content := derefString(fact.FactContent)
		source := derefString(fact.SourceText)
		text := content + " " + source

		for _, term := range industryTerms {
			if strings.Contains(text, term) {
				keywordSet[term] = true
			}
		}

		for _, term := range specialTerms {
			if strings.Contains(text, term) {
				keywordSet[term] = true
			}
		}
	}

	keywords := make([]string, 0, len(keywordSet))
	for kw := range keywordSet {
		keywords = append(keywords, kw)
	}

	return keywords
}

// Helper function: detect special chapters from facts
func detectSpecialChaptersFromFacts(facts []struct {
	FactType    string  `db:"fact_type"`
	FactContent *string `db:"fact_content"`
	SourceText  *string `db:"source_text"`
}) []string {
	specials := make([]string, 0)
	detected := make(map[string]bool)

	specialMappings := map[string][]string{
		"隧道":  {"隧道", "掘进", "盾构", "开挖"},
		"桥梁":  {"桥梁", "桥涵", "上部结构", "下部结构", "箱梁", "T梁"},
		"基坑":  {"基坑", "支护", "降水", "深基坑"},
		"管网":  {"管网", "管道", "顶管", "不开挖"},
		"电气":  {"电气", "机电", "配电", "照明", "电力"},
		"绿化":  {"绿化", "景观", "种植", "养护"},
		"防洪":  {"防洪", "防汛", "度汛"},
		"航道":  {"航道", "疏浚", "码头"},
		"装饰":  {"装饰", "装修", "幕墙", "涂料"},
		"钢结构": {"钢结构", "钢箱梁", "网架", "桁架"},
	}

	for _, fact := range facts {
		text := derefString(fact.FactContent) + " " + derefString(fact.SourceText)

		for special, keywords := range specialMappings {
			if detected[special] {
				continue
			}
			for _, kw := range keywords {
				if strings.Contains(text, kw) {
					specials = append(specials, special)
					detected[special] = true
					break
				}
			}
		}
	}

	return specials
}
