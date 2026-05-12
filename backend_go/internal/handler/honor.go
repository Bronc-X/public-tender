package handler

import (
	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strings"
	"time"
)

type HonorHandler struct {
	db *sqlx.DB
}

func NewHonorHandler(db *sqlx.DB) *HonorHandler {
	return &HonorHandler{db: db}
}

func (h *HonorHandler) ListHonors(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	honors := []model.HonorRecord{}
	query := `
		SELECT h.*, fa.stored_path
		FROM honor_record h
		LEFT JOIN file_asset fa ON h.file_asset_id = fa.id
		WHERE h.company_id = ?
		ORDER BY h.created_at DESC
	`
	err := h.db.Select(&honors, query, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, honors)
}

func (h *HonorHandler) GetHonor(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var honor model.HonorRecord
	query := `
		SELECT h.*, fa.stored_path
		FROM honor_record h
		LEFT JOIN file_asset fa ON h.file_asset_id = fa.id
		WHERE h.id = ? AND h.company_id = ?
	`
	err := h.db.Get(&honor, query, id, companyID)
	if err != nil {
		Error(c, http.StatusNotFound, "Honor not found")
		return
	}
	c.JSON(200, honor)
}

func (h *HonorHandler) CreateHonor(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		HonorName       string `json:"honor_name" binding:"required"`
		HonorLevel      string `json:"honor_level"`
		OwnerOrg        string `json:"owner_org"`
		OwnerPersonName string `json:"owner_person_name"`
		AwardDate       string `json:"award_date"`
		IssueAuthority  string `json:"issue_authority"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	query := `INSERT INTO honor_record (id, honor_name, honor_level, owner_org, owner_person_name, award_date, issue_authority, company_id) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, id, input.HonorName, input.HonorLevel, input.OwnerOrg, input.OwnerPersonName, input.AwardDate, input.IssueAuthority, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(201, gin.H{"id": id})
}

func (h *HonorHandler) UpdateHonor(c *gin.Context) {
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
	query := "UPDATE honor_record SET " + strings.Join(setClauses, ", ") + ", updated_at = ? WHERE id = ?"
	
	result, err := h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Honor not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *HonorHandler) DeleteHonor(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM honor_record WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Honor not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
