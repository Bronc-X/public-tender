package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type KnowledgeHandler struct {
	db *sqlx.DB
}

func NewKnowledgeHandler(db *sqlx.DB) *KnowledgeHandler {
	return &KnowledgeHandler{db: db}
}

type KnowledgeItem struct {
	ID                 string    `db:"id" json:"id"`
	CompanyID          string    `db:"company_id" json:"company_id"`
	ItemType           string    `db:"item_type" json:"item_type"`
	ItemName           string    `db:"item_name" json:"item_name"`
	ItemContent        string    `db:"item_content" json:"item_content"`
	TagsJSON           string    `db:"tags_json" json:"tags_json"`
	ApplicableChapters string    `db:"applicable_chapters" json:"applicable_chapters"`
	SourceDesc         string    `db:"source_desc" json:"source_desc"`
	CredibilityLevel   string    `db:"credibility_level" json:"credibility_level"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
	SourceProjectID    *string   `db:"source_project_id" json:"source_project_id"`
	SourceFileID       *string   `db:"source_file_id" json:"source_file_id"`
	SourceResultID     *string   `db:"source_result_id" json:"source_result_id"`
	SourceReference    *string   `db:"source_reference" json:"source_reference"`
	ExtractTaskID      *string   `db:"extract_task_id" json:"extract_task_id,omitempty"`
}

func (h *KnowledgeHandler) ListKnowledge(c *gin.Context) {
	companyID := c.MustGet("companyID").(string)
	itemType := c.Query("type")

	query := `SELECT 
		id, company_id, item_type, item_name, item_content, tags_json,
		IFNULL(applicable_chapters,'') AS applicable_chapters,
		IFNULL(source_desc,'') AS source_desc,
		IFNULL(credibility_level,'') AS credibility_level,
		created_at, updated_at,
		source_project_id, source_file_id, source_result_id, source_reference,
		extract_task_id
		FROM tech_bid_knowledge_items WHERE company_id = ?`
	args := []interface{}{companyID}

	if itemType != "" {
		query += ` AND item_type = ?`
		args = append(args, itemType)
	}

	query += ` ORDER BY updated_at DESC`

	var items []KnowledgeItem
	err := h.db.Select(&items, query, args...)
	if err != nil {
		log.Printf("Error listing knowledge: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch knowledge items"})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *KnowledgeHandler) CreateKnowledge(c *gin.Context) {
	companyID := c.MustGet("companyID").(string)
	var item KnowledgeItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item.ID = uuid.New().String()
	item.CompanyID = companyID
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	query := `INSERT INTO tech_bid_knowledge_items (id, company_id, item_type, item_name, item_content, tags_json, source_desc, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, item.ID, item.CompanyID, item.ItemType, item.ItemName, item.ItemContent, item.TagsJSON, item.SourceDesc, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		log.Printf("Error creating knowledge: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create knowledge item"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *KnowledgeHandler) UpdateKnowledge(c *gin.Context) {
	companyID := c.MustGet("companyID").(string)
	id := c.Param("id")
	var item KnowledgeItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `UPDATE tech_bid_knowledge_items SET item_name = ?, item_content = ?, tags_json = ?, updated_at = ? WHERE id = ? AND company_id = ?`
	_, err := h.db.Exec(query, item.ItemName, item.ItemContent, item.TagsJSON, time.Now(), id, companyID)
	if err != nil {
		log.Printf("Error updating knowledge: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update knowledge item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *KnowledgeHandler) DeleteKnowledge(c *gin.Context) {
	companyID := c.MustGet("companyID").(string)
	id := c.Param("id")

	_, err := h.db.Exec(`DELETE FROM tech_bid_knowledge_items WHERE id = ? AND company_id = ?`, id, companyID)
	if err != nil {
		log.Printf("Error deleting knowledge: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete knowledge item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *KnowledgeHandler) GetKnowledgeItem(c *gin.Context) {
	companyID := c.MustGet("companyID").(string)
	id := c.Param("id")

	var item KnowledgeItem
	query := `SELECT 
		id, company_id, item_type, item_name, item_content, tags_json,
		IFNULL(applicable_chapters,'') AS applicable_chapters,
		IFNULL(source_desc,'') AS source_desc,
		IFNULL(credibility_level,'') AS credibility_level,
		created_at, updated_at,
		source_project_id, source_file_id, source_result_id, source_reference,
		extract_task_id
		FROM tech_bid_knowledge_items WHERE id = ? AND company_id = ?`
	
	err := h.db.Get(&item, query, id, companyID)
	if err != nil {
		log.Printf("Error getting knowledge item: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Knowledge item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}
