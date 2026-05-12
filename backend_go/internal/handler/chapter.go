package handler

import (
	"backend_go/internal/model"
	"backend_go/internal/service"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
	"strings"
	"time"
)

type ChapterHandler struct {
	db              *sqlx.DB
	digitizeService *service.TenderDigitizationService
}

func NewChapterHandler(db *sqlx.DB, digitizeService *service.TenderDigitizationService) *ChapterHandler {
	return &ChapterHandler{db: db, digitizeService: digitizeService}
}

func (h *ChapterHandler) ListChapterPlans(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		Error(c, http.StatusBadRequest, "missing project_id")
		return
	}
	h.listChapters(c, projectID)
}

func (h *ChapterHandler) ListChaptersByProject(c *gin.Context) {
	projectID := c.Param("id")
	h.listChapters(c, projectID)
}

func (h *ChapterHandler) listChapters(c *gin.Context, projectID string) {
	plans := []model.TechBidChapterPlan{}

	// Latest content per chapter without window functions (broader SQLite compatibility).
	query := `
		SELECT p.*,
			(SELECT c2.content_md FROM tech_bid_chapter_contents c2
			 WHERE c2.chapter_id = p.id AND c2.project_id = ?
			 ORDER BY c2.version_no DESC, c2.updated_at DESC LIMIT 1) AS content_md,
			(SELECT c3.content_html FROM tech_bid_chapter_contents c3
			 WHERE c3.chapter_id = p.id AND c3.project_id = ?
			 ORDER BY c3.version_no DESC, c3.updated_at DESC LIMIT 1) AS content_html,
			(SELECT c4.updated_at FROM tech_bid_chapter_contents c4
			 WHERE c4.chapter_id = p.id AND c4.project_id = ?
			 ORDER BY c4.version_no DESC, c4.updated_at DESC LIMIT 1) AS content_updated_at
		FROM tech_bid_chapter_plans p
		WHERE p.project_id = ?
		ORDER BY p.chapter_order ASC
	`

	err := h.db.Select(&plans, query, projectID, projectID, projectID, projectID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, plans)
}

func (h *ChapterHandler) GenerateChapterContent(c *gin.Context) {
	id := c.Param("id")

	var chapter model.TechBidChapterPlan
	err := h.db.Get(&chapter, "SELECT * FROM tech_bid_chapter_plans WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Chapter not found")
		return
	}

	projectID := chapter.ProjectID

	// --- Context Retrieval ---

	// 1. Get Project Profile
	var profile struct {
		ProfileJSON *string `db:"profile_json"`
	}
	_ = h.db.Get(&profile, "SELECT profile_json FROM tech_bid_project_profiles WHERE project_id = ? ORDER BY created_at DESC LIMIT 1", projectID)
	profileData := "{}"
	if profile.ProfileJSON != nil {
		profileData = *profile.ProfileJSON
	}

	// 2. Get Tender Content
	var tenderFile struct {
		FileAssetID string `db:"file_asset_id"`
	}
	fErr := h.db.Get(&tenderFile, "SELECT file_asset_id FROM tech_bid_tender_files WHERE project_id = ? AND file_role = 'tender' LIMIT 1", projectID)

	tenderContent := ""
	if fErr == nil {
		var content struct {
			MarkdownText *string `db:"markdown_text"`
		}
		cErr := h.db.Get(&content, "SELECT markdown_text FROM file_content WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1", tenderFile.FileAssetID)
		if cErr == nil && content.MarkdownText != nil {
			tenderContent = *content.MarkdownText
		}
	}

	// --- AI Generation (Long-running) ---
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	log.Printf("[Chapter] Generating AI content for: %s", chapter.ChapterName)
	contentMD, genErr := h.digitizeService.GenerateChapterContent(ctx, projectID, chapter.ChapterName, profileData, tenderContent)
	if genErr != nil {
		log.Printf("[Chapter] AI generation failed: %v", genErr)
		// Fallback
		contentMD = fmt.Sprintf("# %s\n\n[ AI 生成失败: %v ]\n请手动编写或重新尝试。", chapter.ChapterName, genErr)
	}

	contentHTML := "" // Front-end rendering preferred

	tx := h.db.MustBegin()

	// Increment version or create first
	var maxVersion int
	_ = tx.Get(&maxVersion, "SELECT COALESCE(MAX(version_no), 0) FROM tech_bid_chapter_contents WHERE chapter_id = ?", id)

	_, err = tx.Exec(`
		INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, content_html, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), projectID, id, maxVersion+1, contentMD, contentHTML, time.Now())

	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = tx.Exec("UPDATE tech_bid_chapter_plans SET generation_status = 'completed', updated_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := syncTechStep5StatusIfComplete(h.db, projectID); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *ChapterHandler) UpdateChapterContent(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		ContentMD string `json:"content_md"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var chapter model.TechBidChapterPlan
	err := h.db.Get(&chapter, "SELECT * FROM tech_bid_chapter_plans WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Chapter not found")
		return
	}

	tx := h.db.MustBegin()

	var maxVersion int
	_ = tx.Get(&maxVersion, "SELECT COALESCE(MAX(version_no), 0) FROM tech_bid_chapter_contents WHERE chapter_id = ?", id)

	_, err = tx.Exec(`
		INSERT INTO tech_bid_chapter_contents (id, project_id, chapter_id, version_no, content_md, content_html, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), chapter.ProjectID, id, maxVersion+1, input.ContentMD, "", time.Now())

	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err = tx.Exec("UPDATE tech_bid_chapter_plans SET generation_status = 'completed', updated_at = ? WHERE id = ?", time.Now(), id); err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := syncTechStep5StatusIfComplete(h.db, chapter.ProjectID); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *ChapterHandler) UpdateChapterPlan(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		ChapterName  *string `json:"chapter_name"`
		ChapterOrder *int    `json:"chapter_order"`
		MustHave     *int    `json:"must_have"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var chapter model.TechBidChapterPlan
	err := h.db.Get(&chapter, "SELECT * FROM tech_bid_chapter_plans WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "Chapter not found")
		return
	}

	// Build update query dynamically based on provided fields
	setParts := []string{}
	args := []interface{}{}
	if input.ChapterName != nil {
		setParts = append(setParts, "chapter_name = ?")
		args = append(args, *input.ChapterName)
	}
	if input.ChapterOrder != nil {
		setParts = append(setParts, "chapter_order = ?")
		args = append(args, *input.ChapterOrder)
	}
	if input.MustHave != nil {
		setParts = append(setParts, "must_have = ?")
		args = append(args, *input.MustHave)
	}

	if len(setParts) == 0 {
		Error(c, http.StatusBadRequest, "No fields to update")
		return
	}

	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE tech_bid_chapter_plans SET %s WHERE id = ?", strings.Join(setParts, ", "))
	_, err = h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, gin.H{"success": true})
}
