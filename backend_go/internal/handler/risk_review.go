package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RiskReviewHandler struct {
	db *sqlx.DB
}

func NewRiskReviewHandler(db *sqlx.DB) *RiskReviewHandler {
	return &RiskReviewHandler{db: db}
}

func (h *RiskReviewHandler) ListTechRiskRecords(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		Error(c, http.StatusBadRequest, "missing project_id")
		return
	}
	h.listRisks(c, projectID)
}

func (h *RiskReviewHandler) ListTechRiskRecordsByProject(c *gin.Context) {
	projectID := c.Param("id")
	h.listRisks(c, projectID)
}

func (h *RiskReviewHandler) listRisks(c *gin.Context, projectID string) {
	risks := make([]techRiskRecordRow, 0)
	err := h.db.Select(&risks, `
		SELECT
			r.id,
			r.project_id,
			r.chapter_id,
			COALESCE(p.chapter_name, '') AS chapter_name,
			r.risk_type,
			r.risk_level,
			r.risk_source,
			r.risk_detail,
			r.similarity_score,
			r.check_result_json,
			r.status,
			r.created_at,
			r.updated_at
		FROM tech_bid_risk_records r
		LEFT JOIN tech_bid_chapter_plans p ON p.id = r.chapter_id
		WHERE r.project_id = ?
		ORDER BY r.created_at DESC`, projectID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, risks)
}

func (h *RiskReviewHandler) RunTechRiskReview(c *gin.Context) {
	projectID := c.Param("id")
	if _, err := syncTechStep5StatusIfComplete(h.db, projectID); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var project struct {
		ID            string         `db:"id"`
		FinalDecision sql.NullString `db:"final_decision"`
		Step5Status   sql.NullString `db:"step5_status"`
	}
	if err := h.db.Get(&project, `SELECT id, final_decision, step5_status FROM tech_bid_projects WHERE id = ?`, projectID); err != nil {
		Error(c, http.StatusNotFound, "Project not found")
		return
	}

	riskRows, err := h.buildDeterministicRiskReview(projectID, strings.TrimSpace(project.FinalDecision.String), strings.TrimSpace(project.Step5Status.String))
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(`DELETE FROM tech_bid_risk_records WHERE project_id = ? AND risk_source = 'deterministic-risk-review-v1'`, projectID); err != nil {
		_ = tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now()
	for _, row := range riskRows {
		if _, err := tx.Exec(`
			INSERT INTO tech_bid_risk_records
				(id, project_id, chapter_id, risk_type, risk_level, risk_source, risk_detail, similarity_score, check_result_json, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(),
			projectID,
			row.ChapterID,
			row.RiskType,
			row.RiskLevel,
			"deterministic-risk-review-v1",
			row.RiskDetail,
			0,
			row.CheckResultJSON,
			row.Status,
			now,
			now,
		); err != nil {
			_ = tx.Rollback()
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if _, err := tx.Exec(`UPDATE tech_bid_projects SET last_error_message = NULL, updated_at = ? WHERE id = ?`, now, projectID); err != nil {
		_ = tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "risk_count": len(riskRows), "risks": riskRows})
}

type techRiskRecordRow struct {
	ID              string     `db:"id" json:"id"`
	ProjectID       string     `db:"project_id" json:"project_id"`
	ChapterID       *string    `db:"chapter_id" json:"chapter_id,omitempty"`
	ChapterName     string     `db:"chapter_name" json:"chapter_name"`
	RiskType        *string    `db:"risk_type" json:"risk_type"`
	RiskLevel       *string    `db:"risk_level" json:"risk_level"`
	RiskSource      *string    `db:"risk_source" json:"risk_source"`
	RiskDetail      *string    `db:"risk_detail" json:"risk_detail"`
	SimilarityScore *float64   `db:"similarity_score" json:"similarity_score"`
	CheckResultJSON *string    `db:"check_result_json" json:"check_result_json"`
	Status          *string    `db:"status" json:"status"`
	CreatedAt       *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       *time.Time `db:"updated_at" json:"updated_at"`
}

type deterministicRiskRow struct {
	ChapterID       interface{}
	RiskType        string
	RiskLevel       string
	RiskDetail      string
	CheckResultJSON string
	Status          string
}

func (h *RiskReviewHandler) buildDeterministicRiskReview(projectID, finalDecision, step5Status string) ([]deterministicRiskRow, error) {
	out := make([]deterministicRiskRow, 0)
	if finalDecision != "PASS" {
		out = append(out, deterministicRiskRow{
			RiskType:        "step4_gate",
			RiskLevel:       "high",
			RiskDetail:      fmt.Sprintf("Step4 尚未通过，当前 final_decision=%s。", firstNonEmptyString(finalDecision, "unknown")),
			CheckResultJSON: `{"gate":"step4"}`,
			Status:          "open",
		})
	}

	stats, err := getTechStep5ContentStats(h.db, projectID)
	if err != nil {
		return nil, err
	}
	contentComplete := stats.SubsectionCount > 0 && stats.ContentCount >= stats.SubsectionCount
	if stats.SubsectionCount > 0 && !contentComplete {
		out = append(out, deterministicRiskRow{
			RiskType:        "content_completeness",
			RiskLevel:       "high",
			RiskDetail:      fmt.Sprintf("正文生成不完整：应生成 %d 个小节，当前仅有 %d 个小节正文。", stats.SubsectionCount, stats.ContentCount),
			CheckResultJSON: fmt.Sprintf(`{"subsection_count":%d,"content_count":%d}`, stats.SubsectionCount, stats.ContentCount),
			Status:          "open",
		})
	}
	if !contentComplete && step5Status != "" && step5Status != "success" && step5Status != "verified_pass" {
		out = append(out, deterministicRiskRow{
			RiskType:        "step5_status",
			RiskLevel:       "medium",
			RiskDetail:      fmt.Sprintf("Step5 状态不是 success，当前 step5_status=%s。", step5Status),
			CheckResultJSON: `{"gate":"step5"}`,
			Status:          "open",
		})
	}
	return out, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *RiskReviewHandler) UpdateRiskStatus(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec("UPDATE tech_bid_risk_records SET status = ?, updated_at = ? WHERE id = ?", input.Status, time.Now(), id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}
