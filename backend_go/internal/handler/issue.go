package handler

import (
	"log"
	"net/http"
	"time"

	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type IssueHandler struct {
	db *sqlx.DB
}

func NewIssueHandler(db *sqlx.DB) *IssueHandler {
	return &IssueHandler{db: db}
}

func (h *IssueHandler) ListIssues(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	log.Printf("[IssueCenter] Fetching issues for company %s", cid)

	issues := []model.IssueRecord{}
	// Fetch all open issues for the company
	err := h.db.Select(&issues, "SELECT * FROM issue_record WHERE company_id = ? AND status = 'open' ORDER BY created_at DESC", cid)
	if err != nil {
		log.Printf("Error list issues: %v", err)
		Error(c, http.StatusInternalServerError, "Failed to load issues")
		return
	}

	c.JSON(http.StatusOK, issues)
}

func (h *IssueHandler) ResolveIssue(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Status         string `json:"status" binding:"required"`
		ResolutionNote string `json:"resolution_note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec("UPDATE issue_record SET status = ?, resolution_note = ?, updated_at = ? WHERE id = ?", 
		input.Status, input.ResolutionNote, time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
