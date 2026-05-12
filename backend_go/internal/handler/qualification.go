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

type QualificationHandler struct {
	db *sqlx.DB
}

func NewQualificationHandler(db *sqlx.DB) *QualificationHandler {
	return &QualificationHandler{db: db}
}

func (h *QualificationHandler) ListQualifications(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	qualifications := []model.Qualification{}
	query := `
		SELECT q.*, fa.stored_path, fa.ext
		FROM qualification q
		LEFT JOIN file_asset fa ON q.file_asset_id = fa.id
		WHERE q.company_id = ?
		ORDER BY q.created_at DESC
	`
	err := h.db.Select(&qualifications, query, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, qualifications)
}

func (h *QualificationHandler) GetQualification(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var qualification model.Qualification
	query := `
		SELECT q.*, fa.stored_path, fa.ext
		FROM qualification q
		LEFT JOIN file_asset fa ON q.file_asset_id = fa.id
		WHERE q.id = ? AND q.company_id = ?
	`
	err := h.db.Get(&qualification, query, id, companyID)
	if err != nil {
		Error(c, http.StatusNotFound, "Qualification not found")
		return
	}
	c.JSON(200, qualification)
}

func (h *QualificationHandler) CreateQualification(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		QualificationName  string `json:"qualification_name" binding:"required"`
		QualificationLevel string `json:"qualification_level"`
		QualificationType  string `json:"qualification_type"`
		OwnerType          string `json:"owner_type"`
		OwnerID            string `json:"owner_id"`
		CertificateNo      string `json:"certificate_no"`
		RegistrationNo     string `json:"registration_no"`
		IssuingAuthority   string `json:"issuing_authority"`
		ValidFrom          string `json:"valid_from"`
		ValidTo            string `json:"valid_to"`
		BidUsableStatus    string `json:"bid_usable_status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.New().String()
	query := `INSERT INTO qualification (id, qualification_name, qualification_level, qualification_type, owner_type, owner_id, certificate_no, registration_no, issuing_authority, valid_from, valid_to, bid_usable_status, company_id) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, id, input.QualificationName, input.QualificationLevel, input.QualificationType, input.OwnerType, input.OwnerID, input.CertificateNo, input.RegistrationNo, input.IssuingAuthority, input.ValidFrom, input.ValidTo, input.BidUsableStatus, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(201, gin.H{"id": id})
}

func (h *QualificationHandler) UpdateQualification(c *gin.Context) {
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
	query := "UPDATE qualification SET " + strings.Join(setClauses, ", ") + ", updated_at = ? WHERE id = ?"
	
	result, err := h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Qualification not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *QualificationHandler) DeleteQualification(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM qualification WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Qualification not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
