package handler

import (
	"backend_go/internal/model"
	"backend_go/internal/service"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
	"strings"
)

type AuditHandler struct {
	db             *sqlx.DB
	reviewService  *service.FileReviewService
	archiveService *service.FileArchiveService
}

func NewAuditHandler(db *sqlx.DB, reviewService *service.FileReviewService, archiveService *service.FileArchiveService) *AuditHandler {
	return &AuditHandler{
		db:             db,
		reviewService:  reviewService,
		archiveService: archiveService,
	}
}

func (h *AuditHandler) ListAudits(c *gin.Context) {
	val, _ := c.Get("companyID")
	cid := val.(string)

	log.Printf("[Audit] Listing audits for company %s", cid)

	items := []struct {
		ID              string  `json:"id"`
		FileName        string  `json:"file_name"`
		MimeType        string  `json:"mime_type"`
		ObjectType      string  `json:"object_type"`
		AuditStatus     string  `json:"audit_status"`
		ConfidenceScore float64 `json:"confidence_score"`
		RiskLevel       string  `json:"risk_level"`
		CreatedAt       string  `json:"created_at"`
	}{}

	query := `
		SELECT 
            'audit:' || a.id as id, 
            COALESCE(f.file_name, '未知文件') as file_name, 
            COALESCE(f.mime_type, 'application/octet-stream') as mime_type,
            a.object_type, 
            a.audit_status, 
            a.confidence_score, 
            COALESCE(a.risk_level, 'low') as risk_level, 
            a.created_at
		FROM audit_item a
		LEFT JOIN file_asset f ON a.file_id = f.id
		WHERE (a.company_id = ? OR f.company_id = ?) 
		AND a.audit_status IN ('pending', 'processing')

        UNION ALL

        SELECT 
            'file:' || f.id as id,
            f.file_name,
            COALESCE(f.mime_type, 'image/jpeg') as mime_type,
            'general' as object_type,
            'processing' as audit_status,
            0.0 as confidence_score,
            'low' as risk_level,
            f.created_at
        FROM file_asset f
        WHERE f.company_id = ? 
        AND f.scan_status IN ('uploaded', 'processing', 'analyzing', 'queued', 'running')
        AND NOT EXISTS (SELECT 1 FROM audit_item WHERE file_id = f.id)

		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(query, cid, cid, cid)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to query audits")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var item struct {
			ID              string  `json:"id"`
			FileName        string  `json:"file_name"`
			MimeType        string  `json:"mime_type"`
			ObjectType      string  `json:"object_type"`
			AuditStatus     string  `json:"audit_status"`
			ConfidenceScore float64 `json:"confidence_score"`
			RiskLevel       string  `json:"risk_level"`
			CreatedAt       string  `json:"created_at"`
		}
		if err := rows.Scan(&item.ID, &item.FileName, &item.MimeType, &item.ObjectType, &item.AuditStatus, &item.ConfidenceScore, &item.RiskLevel, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (h *AuditHandler) GetAuditDetail(c *gin.Context) {
	id := c.Param("id")
	val, _ := c.Get("companyID")
	cid := val.(string)

	log.Printf("[Audit] Fetching detail for audit %s", id)

	if strings.HasPrefix(id, "file:") {
		// Handle virtual audit for processing file
		fileID := strings.TrimPrefix(id, "file:")
		var file model.FileAsset
		err := h.db.Get(&file, "SELECT * FROM file_asset WHERE id = ? AND company_id = ?", fileID, cid)
		if err != nil {
			Error(c, http.StatusNotFound, "Processing file not found")
			return
		}

		// Return a mock detail
		detail := &model.AuditDetail{
			ID:          id,
			FileName:    file.FileName,
			MimeType:    *file.MimeType,
			StoredPath:  *file.StoredPath,
			ObjectType:  "general",
			AuditStatus: "processing",
			OCRText:     "### 🔍 正在全力识别中...\n\n识别完成后，此处将自动同步识别出的结构化数据。您可以稍等片刻并刷新页面。",
			FileID:      file.ID,
		}
		c.JSON(http.StatusOK, detail)
		return
	}

    // Standard audit item
    realID := strings.TrimPrefix(id, "audit:")
	detail, err := h.reviewService.GetAuditDetail(realID, cid)
	if err != nil {
		Error(c, http.StatusNotFound, "Audit item not found")
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *AuditHandler) ConfirmAudit(c *gin.Context) {
	id := c.Param("id")
	val, _ := c.Get("companyID")
	cid := val.(string)

	var req struct {
		Text           string `json:"text"`
		FileID         string `json:"file_id"`
		ExtractedItems []struct {
			Title      string  `json:"title"`
			Summary    string  `json:"summary"`
			Content    string  `json:"content"`
			Confidence float64 `json:"confidence"`
			SourcePage string  `json:"source_page"`
		} `json:"extracted_items"`
		ObjectType     string `json:"object_type"`
		ConfirmedText  string `json:"confirmed_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	log.Printf("[Audit] Confirming audit %s with %d items", id, len(req.ExtractedItems))
	
	realID := strings.TrimPrefix(id, "audit:")
	log.Printf("[Audit] Looking up audit realID=%s cid=%s", realID, cid)

	// 1. Get current audit item to know the target
	detail, err := h.reviewService.GetAuditDetail(realID, cid)
	if err != nil {
		log.Printf("[Audit] Audit item not found: %v (ID: %s, CompanyID: %s)", err, realID, cid)
		Error(c, http.StatusNotFound, "Audit item not found")
		return
	}

	// 2. Archive to library
	targetType := detail.ObjectType
	if req.ObjectType != "" {
		targetType = req.ObjectType
	}
	itemsJSON, _ := json.Marshal(req.ExtractedItems)
	targetID, err := h.archiveService.ArchiveToLibrary(cid, targetType, string(itemsJSON), req.FileID)
	if err != nil {
		log.Printf("[Audit] Archiving failed: %v", err)
		Error(c, http.StatusInternalServerError, "Archive failed")
		return
	}

	// 3. Persist user-selected classification (与 AI 初判一致字段，便于列表与追溯)
	if req.ObjectType != "" {
		if _, err := h.db.Exec("UPDATE audit_item SET object_type = ? WHERE id = ?", req.ObjectType, realID); err != nil {
			log.Printf("[Audit] Update object_type failed: %v", err)
		}
	}

	// 4. Update status
	err = h.reviewService.UpdateAuditStatus(realID, "confirmed")
	if err != nil {
		log.Printf("[Audit] Status update failed: %v", err)
	}
	
	// 5. Update file status
	h.db.Exec("UPDATE file_asset SET scan_status = 'approved', archive_status = 'archived', archive_target_type = ?, archive_target_id = ? WHERE id = ?", targetType, targetID, req.FileID)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *AuditHandler) IgnoreAudit(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[Audit] Ignoring audit %s", id)

	realID := strings.TrimPrefix(id, "audit:")
	err := h.reviewService.UpdateAuditStatus(realID, "ignored")
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to ignore audit")
		return
	}

	h.db.Exec("UPDATE file_asset SET scan_status = 'ignored' WHERE id = (SELECT file_id FROM audit_item WHERE id = ?)", realID)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
