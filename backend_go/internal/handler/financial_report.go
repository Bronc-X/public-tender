package handler

import (
	"backend_go/internal/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type FinancialReportHandler struct {
	db *sqlx.DB
}

func NewFinancialReportHandler(db *sqlx.DB) *FinancialReportHandler {
	return &FinancialReportHandler{db: db}
}

// ListFolders returns all financial report folders for the current company
func (h *FinancialReportHandler) ListFolders(c *gin.Context) {
	companyID, exists := c.Get("companyID")
	if !exists {
		Error(c, http.StatusUnauthorized, "Missing company context")
		return
	}
	cid := companyID.(string)

	var folders []model.FinancialReportFolder
	err := h.db.Select(&folders, "SELECT * FROM financial_report_folder WHERE company_id = ? ORDER BY created_at DESC", cid)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to retrieve folders")
		return
	}

	c.JSON(http.StatusOK, folders)
}

// CreateFolder creates a new folder
func (h *FinancialReportHandler) CreateFolder(c *gin.Context) {
	companyID, exists := c.Get("companyID")
	if !exists {
		Error(c, http.StatusUnauthorized, "Missing company context")
		return
	}
	cid := companyID.(string)

	var req struct {
		FolderName string `json:"folder_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	id := uuid.New().String()
	_, err := h.db.Exec("INSERT INTO financial_report_folder (id, company_id, folder_name) VALUES (?, ?, ?)", id, cid, req.FolderName)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to create folder")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          id,
		"company_id":  cid,
		"folder_name": req.FolderName,
	})
}

// DeleteFolder deletes a folder by id
func (h *FinancialReportHandler) DeleteFolder(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	_, err := h.db.Exec("DELETE FROM financial_report_folder WHERE id = ? AND company_id = ?", id, cid)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to delete folder")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// RenameFolder updates the name of a folder by id
func (h *FinancialReportHandler) RenameFolder(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var req struct {
		FolderName string `json:"folder_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.db.Exec("UPDATE financial_report_folder SET folder_name = ? WHERE id = ? AND company_id = ?", req.FolderName, id, cid)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to rename folder")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		Error(c, http.StatusNotFound, "Folder not found or unauthorized")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "renamed", "new_name": req.FolderName})
}

// ListFilesByFolder lists file_asset items restricted by folder id
func (h *FinancialReportHandler) ListFilesByFolder(c *gin.Context) {
	folderID := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var files = make([]model.FileAsset, 0)
	query := `
		SELECT 
			f.id, f.file_name, f.ext, f.mime_type, f.file_size, f.sha256, 
			f.source_path, f.stored_path, f.source_type, f.import_batch_id, 
			f.scan_status, f.parse_status, f.archive_status, f.company_id, 
			f.created_at, f.updated_at,
			f.source_module, f.source_project_id, f.last_task_id, f.last_error_message,
			f.scan_status as status,
			f.archive_target_type, f.archive_target_id,
			a.id as audit_id,
			a.object_type as object_type,
			COALESCE(c.plain_text, '') as plain_text, 
			COALESCE(c.markdown_text, '') as markdown_text 
		FROM file_asset f
		LEFT JOIN file_content c ON f.id = c.file_asset_id
		LEFT JOIN audit_item a ON f.id = a.file_id
		WHERE f.company_id = ? 
		  AND f.source_module = 'financial_report'
		  AND f.source_project_id = ?
		ORDER BY f.created_at DESC 
	`
	err := h.db.Select(&files, query, cid, folderID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch files")
		return
	}

	c.JSON(http.StatusOK, files)
}
