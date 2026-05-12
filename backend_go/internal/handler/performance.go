package handler

import (
	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
	"strings"
	"time"
)

type PerformanceHandler struct {
	db *sqlx.DB
}

func NewPerformanceHandler(db *sqlx.DB) *PerformanceHandler {
	return &PerformanceHandler{db: db}
}

func (h *PerformanceHandler) ListPerformances(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	performances := []model.Performance{}
	err := h.db.Select(&performances, "SELECT * FROM project_performance WHERE company_id = ? ORDER BY created_at DESC", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, performances)
}

func (h *PerformanceHandler) GetPerformance(c *gin.Context) {
	id := c.Param("id")
	var performance model.Performance
	err := h.db.Get(&performance, "SELECT * FROM project_performance WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Performance not found")
		return
	}

	// Fetch proofs
	proofs := []model.PerformanceProof{}
	query := `SELECT pp.*, f.file_name, f.ext, f.markdown_text 
	          FROM performance_proof pp
			  LEFT JOIN file_asset f ON pp.file_asset_id = f.id
			  WHERE pp.project_performance_id = ?`
	if err := h.db.Select(&proofs, query, id); err != nil {
		log.Printf("[Performance] Error fetching proofs for %s: %v", id, err)
	}
	performance.Proofs = proofs

	c.JSON(200, performance)
}

func (h *PerformanceHandler) AddPerformanceProof(c *gin.Context) {
	id := c.Param("id") // performance id
	var input struct {
		FileAssetID string `json:"file_asset_id"`
		ProofType   string `json:"proof_type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	proofID := uuid.New().String()
	_, err := h.db.Exec(`INSERT INTO performance_proof (id, project_performance_id, file_asset_id, proof_type) VALUES (?, ?, ?, ?)`,
		proofID, id, input.FileAssetID, input.ProofType)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(201, gin.H{"id": proofID})
}

func (h *PerformanceHandler) DeletePerformanceProof(c *gin.Context) {
	proofID := c.Param("proofID")
	_, err := h.db.Exec("DELETE FROM performance_proof WHERE id = ?", proofID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PerformanceHandler) CreatePerformance(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		ProjectName         string  `json:"project_name" binding:"required"`
		ProjectLocation     string  `json:"project_location"`
		OwnerOrg            string  `json:"owner_org"`
		ProjectManagerName  string  `json:"project_manager_name"`
		PmID                string  `json:"pm_id"`
		TechnicalLeaderName string  `json:"technical_leader_name"`
		TechLeaderID        string  `json:"tech_leader_id"`
		SafetyLeaderName    string  `json:"safety_leader_name"`
		SafetyLeaderID      string  `json:"safety_leader_id"`
		CompletionDate      string  `json:"completion_date"`
		WinningDate         string  `json:"winning_date"`
		AmountValue         float64 `json:"amount_value"`
		BidAmountValue      float64 `json:"bid_amount_value"`
		ScaleDesc           string  `json:"scale_desc"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	query := `INSERT INTO project_performance (
				id, project_name, project_location, owner_org, 
				project_manager_name, pm_id, 
				technical_leader_name, tech_leader_id, 
				safety_leader_name, safety_leader_id, 
				completion_date, winning_date, amount_value, bid_amount_value, scale_desc, company_id
			  ) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, 
		id, input.ProjectName, input.ProjectLocation, input.OwnerOrg, 
		input.ProjectManagerName, input.PmID, 
		input.TechnicalLeaderName, input.TechLeaderID, 
		input.SafetyLeaderName, input.SafetyLeaderID, 
		input.CompletionDate, input.WinningDate, input.AmountValue, input.BidAmountValue, input.ScaleDesc, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(201, gin.H{"id": id})
}

func (h *PerformanceHandler) UpdatePerformance(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	forbidden := map[string]bool{"id": true, "created_at": true, "updated_at": true}
	var setClauses []string
	var args []interface{}

	for k, v := range input {
		if !forbidden[strings.ToLower(k)] {
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}

	if len(setClauses) == 0 {
		Error(c, http.StatusBadRequest, "no valid fields to update")
		return
	}

	args = append(args, time.Now(), id)
	query := "UPDATE project_performance SET " + strings.Join(setClauses, ", ") + ", updated_at = ? WHERE id = ?"
	
	result, err := h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Performance not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PerformanceHandler) DeletePerformance(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM project_performance WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Performance not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// MatchPersonnel processes all performances for a company and attempts to link names to personnel IDs
func (h *PerformanceHandler) MatchPersonnel(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	
	// 1. Get all persons for this company to build a lookup map
	var persons []model.Person
	err := h.db.Select(&persons, "SELECT id, name FROM person WHERE company_id = ?", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch persons: "+err.Error())
		return
	}
	
	nameToID := make(map[string]string)
	for _, p := range persons {
		nameToID[strings.TrimSpace(p.Name)] = p.ID
	}
	
	// 2. Get all performances with missing IDs
	var performances []model.Performance
	query := `SELECT id, project_manager_name, technical_leader_name, safety_leader_name 
	          FROM project_performance 
			  WHERE company_id = ? 
			  AND (pm_id IS NULL OR pm_id = '' OR tech_leader_id IS NULL OR tech_leader_id = '' OR safety_leader_id IS NULL OR safety_leader_id = '')`
	
	err = h.db.Select(&performances, query, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to fetch performances: "+err.Error())
		return
	}
	
	updatedCount := 0
	for _, p := range performances {
		updated := false
		updateQuery := "UPDATE project_performance SET "
		var setClauses []string
		var args []interface{}
		
		if p.PmID == nil || *p.PmID == "" {
			if id, ok := nameToID[strings.TrimSpace(strings.TrimSpace(ptrToString(p.ProjectManagerName)))]; ok {
				setClauses = append(setClauses, "pm_id = ?")
				args = append(args, id)
				updated = true
			}
		}
		if p.TechLeaderID == nil || *p.TechLeaderID == "" {
			if id, ok := nameToID[strings.TrimSpace(ptrToString(p.TechnicalLeaderName))]; ok {
				setClauses = append(setClauses, "tech_leader_id = ?")
				args = append(args, id)
				updated = true
			}
		}
		if p.SafetyLeaderID == nil || *p.SafetyLeaderID == "" {
			if id, ok := nameToID[strings.TrimSpace(ptrToString(p.SafetyLeaderName))]; ok {
				setClauses = append(setClauses, "safety_leader_id = ?")
				args = append(args, id)
				updated = true
			}
		}
		
		if updated {
			updateQuery += strings.Join(setClauses, ", ") + " WHERE id = ?"
			args = append(args, p.ID)
			_, err = h.db.Exec(updateQuery, args...)
			if err == nil {
				updatedCount++
			}
		}
	}
	
	c.JSON(200, gin.H{
		"processed": len(performances),
		"updated":   updatedCount,
	})
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

