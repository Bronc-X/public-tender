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

type CompanyHandler struct {
	db *sqlx.DB
}

func NewCompanyHandler(db *sqlx.DB) *CompanyHandler {
	return &CompanyHandler{db: db}
}

func (h *CompanyHandler) ListCompanies(c *gin.Context) {
	var companies []model.Company
	err := h.db.Select(&companies, `
		SELECT id, company_name, unified_social_credit_code, legal_person, legal_person_id_card, address, created_at, updated_at
		FROM company_profile
		ORDER BY created_at DESC`)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, companies)
}

func (h *CompanyHandler) GetCompany(c *gin.Context) {
	id := c.Param("id")
	var company model.Company
	err := h.db.Get(&company, `
		SELECT id, company_name, unified_social_credit_code, legal_person, legal_person_id_card, address, created_at, updated_at
		FROM company_profile WHERE id = ?`, id)
	if err != nil {
		Error(c, http.StatusNotFound, "Company not found")
		return
	}
	c.JSON(200, company)
}

func (h *CompanyHandler) CreateCompany(c *gin.Context) {
	var input struct {
		CompanyName               string  `json:"company_name" binding:"required"`
		UnifiedSocialCreditCode   *string `json:"unified_social_credit_code"`
		LegalPerson               *string `json:"legal_person"`
		Address                   *string `json:"address"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	_, err := h.db.Exec(`
		INSERT INTO company_profile (id, company_name, unified_social_credit_code, legal_person, address)
		VALUES (?, ?, ?, ?, ?)`,
		id, input.CompanyName, input.UnifiedSocialCreditCode, input.LegalPerson, input.Address)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(201, gin.H{"id": id})
}

func (h *CompanyHandler) UpdateCompany(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Filter out forbidden fields
	forbidden := map[string]bool{"id": true, "created_at": true, "updated_at": true, "company_id": true}
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
	query := "UPDATE company_profile SET " + strings.Join(setClauses, ", ") + ", updated_at = ? WHERE id = ?"
	
	result, err := h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Company not found")
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *CompanyHandler) DeleteCompany(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM company_profile WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Company not found")
		return
	}

	c.JSON(200, gin.H{"success": true})
}
