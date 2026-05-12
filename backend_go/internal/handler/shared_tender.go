package handler

import (
	"backend_go/internal/model"
	"backend_go/pkg/common"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type SharedTenderHandler struct {
	db *sqlx.DB
}

func NewSharedTenderHandler(db *sqlx.DB) *SharedTenderHandler {
	return &SharedTenderHandler{db: db}
}

func (h *SharedTenderHandler) ListCandidates(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	search := strings.TrimSpace(c.Query("search"))

	type Candidate struct {
		ID              string  `db:"id" json:"id"`
		ProjectName     *string `db:"project_name" json:"project_name"`
		TenderCode      *string `db:"tender_code" json:"tender_code"`
		OwnerName       *string `db:"owner_name" json:"owner_name"`
		SourceModule    string  `db:"source_module" json:"source_module"`
		SourceProjectID string  `db:"source_project_id" json:"source_project_id"`
		Mode            string  `db:"mode" json:"mode"`
	}

	// Non-nil slice so JSON is always [] — never null (frontend List crashes on null dataSource).
	candidates := make([]Candidate, 0)

	// Registry
	var registry []Candidate
	if err := h.db.Select(&registry, "SELECT id, project_name, tender_code, owner_name, source_module, source_project_id, 'registry' as mode FROM shared_tenders WHERE company_id = ?", companyID); err == nil {
		candidates = append(candidates, registry...)
	}

	// Bid：列出本公司全部商务标项目（含已关联共享招标的），便于「再建一条」同步自现有项目
	var bidCandidates []Candidate
	if err := h.db.Select(&bidCandidates, `
		SELECT id as id, project_name, tender_code, owner_name, 'bid' as source_module, id as source_project_id, 'project' as mode 
		FROM bid_projects 
		WHERE company_id = ?`, companyID); err == nil {
		candidates = append(candidates, bidCandidates...)
	}

	// Tech：同上，技术标也可作为商务标同步来源
	var techCandidates []Candidate
	if err := h.db.Select(&techCandidates, `
		SELECT id as id, project_name, tender_code, NULL as owner_name, 'tech' as source_module, id as source_project_id, 'project' as mode 
		FROM tech_bid_projects 
		WHERE company_id = ?`, companyID); err == nil {
		candidates = append(candidates, techCandidates...)
	}

	if search != "" {
		q := strings.ToLower(search)
		filtered := make([]Candidate, 0, len(candidates))
		for _, c := range candidates {
			pn := ""
			if c.ProjectName != nil {
				pn = strings.ToLower(*c.ProjectName)
			}
			tc := ""
			if c.TenderCode != nil {
				tc = strings.ToLower(*c.TenderCode)
			}
			on := ""
			if c.OwnerName != nil {
				on = strings.ToLower(*c.OwnerName)
			}
			if strings.Contains(pn, q) || strings.Contains(tc, q) || strings.Contains(on, q) {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	c.JSON(200, candidates)
}

func (h *SharedTenderHandler) Resolve(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		FileAssetID string `json:"file_asset_id"`
		TenderCode  string `json:"tender_code"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var hash string
	if input.FileAssetID != "" {
		var storedPath string
		err := h.db.Get(&storedPath, "SELECT stored_path FROM file_asset WHERE id = ?", input.FileAssetID)
		if err == nil {
			hash, _ = common.CalculateFileHash(storedPath)
		}
	}

	var existing model.SharedTender
	found := false
	if hash != "" {
		err := h.db.Get(&existing, "SELECT * FROM shared_tenders WHERE company_id = ? AND tender_hash = ?", companyID, hash)
		if err == nil {
			found = true
		}
	}
	if !found && input.TenderCode != "" {
		err := h.db.Get(&existing, "SELECT * FROM shared_tenders WHERE company_id = ? AND tender_code = ?", companyID, input.TenderCode)
		if err == nil {
			found = true
		}
	}

	if found {
		matchType := "tender_code"
		if hash != "" && existing.TenderHash != nil && *existing.TenderHash == hash {
			matchType = "hash"
		}

		var parseResultJSON interface{}
		if existing.ParseResultJSON != nil {
			json.Unmarshal([]byte(*existing.ParseResultJSON), &parseResultJSON)
		}

		c.JSON(200, gin.H{
			"match_type":       matchType,
			"can_reuse":        true,
			"shared_tender_id": existing.ID,
			"parse_status":     existing.ParseStatus,
			"parse_result_json": parseResultJSON,
			"project_name":     existing.ProjectName,
		})
	} else {
		c.JSON(200, gin.H{"can_reuse": false})
	}
}

func (h *SharedTenderHandler) GetSharedTender(c *gin.Context) {
	id := c.Param("id")
	var tender model.SharedTender
	err := h.db.Get(&tender, "SELECT * FROM shared_tenders WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Shared tender not found")
		return
	}

	// Linked projects
	var bidProjects []struct{ ID, ProjectName string }
	h.db.Select(&bidProjects, "SELECT id, project_name FROM bid_projects WHERE shared_tender_id = ?", id)

	var techProjects []struct{ ID, ProjectName string }
	h.db.Select(&techProjects, "SELECT id, project_name FROM tech_bid_projects WHERE shared_tender_id = ?", id)

	c.JSON(200, gin.H{
		"tender": tender,
		"linked_projects": gin.H{
			"bid":  bidProjects,
			"tech": techProjects,
		},
	})
}

func (h *SharedTenderHandler) BindProject(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		ProjectType string `json:"project_type" binding:"required"`
		ProjectID   string `json:"project_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	table := "bid_projects"
	if input.ProjectType == "tech" {
		table = "tech_bid_projects"
	}

	tx, _ := h.db.Beginx()
	_, err := tx.Exec("UPDATE "+table+" SET shared_tender_id = ? WHERE id = ?", id, input.ProjectID)
	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Also update file links (simplified)
	// ...
	
	tx.Commit()
	c.JSON(200, gin.H{"success": true})
}
