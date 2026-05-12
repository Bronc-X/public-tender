package handler

import (
	"log"
	"net/http"

	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type DashboardHandler struct {
	db *sqlx.DB
}

func NewDashboardHandler(db *sqlx.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

func (h *DashboardHandler) GetSummary(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	log.Printf("[Dashboard] Fetching summary for company %s", cid)

	var stats struct {
		ProjectCount       int `db:"project_count" json:"projectCount"`
		PersonCount        int `db:"person_count" json:"personCount"`
		QualificationCount int `db:"qualification_count" json:"qualificationCount"`
		HonorCount         int `db:"honor_count" json:"honorCount"`
		IssueCount         int `db:"issue_count" json:"issueCount"`
		AuditCount         int `db:"audit_count" json:"auditCount"`
	}

	// Individual counts for robust processing
	var projectCount, personCount, qualificationCount, honorCount, issueCount, auditCount int

	err := h.db.Get(&projectCount, "SELECT COUNT(*) FROM project_performance WHERE company_id = ?", cid)
	if err != nil { log.Printf("Error projectCount: %v", err) }
	
	err = h.db.Get(&personCount, "SELECT COUNT(*) FROM person WHERE company_id = ?", cid)
	if err != nil { log.Printf("Error personCount: %v", err) }
	
	err = h.db.Get(&qualificationCount, "SELECT COUNT(*) FROM qualification WHERE company_id = ? AND owner_type = 'company'", cid)
	if err != nil { log.Printf("Error qualificationCount: %v", err) }
	
	err = h.db.Get(&honorCount, "SELECT COUNT(*) FROM honor_record WHERE company_id = ?", cid)
	if err != nil { log.Printf("Error honorCount: %v", err) }
	
	err = h.db.Get(&issueCount, "SELECT COUNT(*) FROM issue_record WHERE status = 'open' AND company_id = ?", cid)
	if err != nil { log.Printf("Error issueCount: %v", err) }
	
	err = h.db.Get(&auditCount, "SELECT COUNT(*) FROM audit_item WHERE audit_status = 'pending' AND company_id = ?", cid)
	if err != nil { log.Printf("Error auditCount: %v", err) }

	stats.ProjectCount = projectCount
	stats.PersonCount = personCount
	stats.QualificationCount = qualificationCount
	stats.HonorCount = honorCount
	stats.IssueCount = issueCount
	stats.AuditCount = auditCount

	// Recent projects
	var recentProjects []model.Performance
	err = h.db.Select(&recentProjects, "SELECT * FROM project_performance WHERE company_id = ? ORDER BY created_at DESC LIMIT 5", cid)
	if err != nil {
		log.Printf("Error recentProjects: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"projectCount":       stats.ProjectCount,
		"personCount":        stats.PersonCount,
		"qualificationCount": stats.QualificationCount,
		"honorCount":         stats.HonorCount,
		"issueCount":         stats.IssueCount,
		"auditCount":         stats.AuditCount,
		"recentProjects":     recentProjects,
	})
}
