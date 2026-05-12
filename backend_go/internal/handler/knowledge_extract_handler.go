package handler

import (
	"backend_go/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type KnowledgeExtractHandler struct {
	db      *sqlx.DB
	svc     *service.KnowledgeExtractService
}

func NewKnowledgeExtractHandler(db *sqlx.DB, svc *service.KnowledgeExtractService) *KnowledgeExtractHandler {
	return &KnowledgeExtractHandler{db: db, svc: svc}
}

// GET /api/knowledge-extract/history-projects?q=
func (h *KnowledgeExtractHandler) ListHistoryProjects(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	q := c.Query("q")
	rows, err := h.svc.ListTechBidHistoryProjects(cid, q)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, rows)
}

// GET /api/knowledge-extract/projects/:origin/:id/files
func (h *KnowledgeExtractHandler) ListProjectFiles(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	origin := c.Param("origin")
	id := c.Param("id")
	if origin != "tech_bid" {
		Error(c, http.StatusBadRequest, "unsupported origin")
		return
	}
	rows, err := h.svc.ListTechBidProjectFiles(cid, id)
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, rows)
}

type resolveLocalBody struct {
	ClientProjectID string                   `json:"client_project_id"`
	Files           []service.LocalFileInput `json:"files"`
}

// POST /api/knowledge-extract/resolve-local-files
func (h *KnowledgeExtractHandler) ResolveLocalFiles(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	var body resolveLocalBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.svc.ResolveLocalFiles(cid, body.Files)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"client_project_id": body.ClientProjectID, "files": rows})
}

// GET /api/knowledge-extract/prompt-templates?knowledge_type=method
func (h *KnowledgeExtractHandler) ListPromptTemplates(c *gin.Context) {
	kt := c.Query("knowledge_type")
	if kt == "" {
		kt = "method"
	}
	rows, err := h.svc.ListPromptTemplatesForType(kt)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, rows)
}

// POST /api/knowledge-extract/tasks — create task and run AI in background; returns task_id immediately.
func (h *KnowledgeExtractHandler) CreateTask(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	var req service.CreateExtractTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	taskID, err := h.svc.StartExtractTaskAsync(cid, req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID, "status": "running"})
}

// GET /api/knowledge-extract/tasks/:id
func (h *KnowledgeExtractHandler) GetTask(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	id := c.Param("id")
	meta, err := h.svc.GetTaskMeta(cid, id)
	if err != nil {
		Error(c, http.StatusNotFound, "not found")
		return
	}
	results, _ := h.svc.GetTaskResults(cid, id)
	c.JSON(http.StatusOK, gin.H{"meta": meta, "results": results})
}

type commitBody struct {
	Items []service.CommitExtractItem `json:"items" binding:"required"`
}

// POST /api/knowledge-extract/tasks/:id/commit
func (h *KnowledgeExtractHandler) CommitTask(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	id := c.Param("id")
	var body commitBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	n, err := h.svc.CommitTask(cid, id, body.Items)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved": n})
}

// POST /api/knowledge-extract/tasks/:id/cancel
func (h *KnowledgeExtractHandler) CancelTask(c *gin.Context) {
	cid := c.MustGet("companyID").(string)
	id := c.Param("id")
	if err := h.svc.CancelTask(cid, id); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}
