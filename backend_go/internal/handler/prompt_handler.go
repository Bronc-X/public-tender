package handler

import (
	"backend_go/internal/model"
	"backend_go/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type PromptHandler struct {
	db            *sqlx.DB
	promptService *service.PromptService
}

func NewPromptHandler(db *sqlx.DB, ps *service.PromptService) *PromptHandler {
	return &PromptHandler{db: db, promptService: ps}
}

// GetCategories 获取分类列表
func (h *PromptHandler) GetCategories(c *gin.Context) {
	var categories []model.PromptCategory
	err := h.db.Select(&categories, "SELECT * FROM prompt_category ORDER BY sort ASC")
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, categories)
}

// SaveCategory 新增或编辑分类
func (h *PromptHandler) SaveCategory(c *gin.Context) {
	var input model.PromptCategory
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if input.ID > 0 {
		_, err := h.db.Exec("UPDATE prompt_category SET name = ?, parent_id = ?, sort = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			input.Name, input.ParentID, input.Sort, input.Remark, input.ID)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		_, err := h.db.Exec("INSERT INTO prompt_category (name, parent_id, sort, remark) VALUES (?, ?, ?, ?)",
			input.Name, input.ParentID, input.Sort, input.Remark)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	Success(c, nil)
}

// DeleteCategory 删除分类
func (h *PromptHandler) DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	// 检查是否有提示词使用该分类
	var count int
	err := h.db.Get(&count, "SELECT COUNT(*) FROM prompt_template WHERE category_id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		Error(c, http.StatusBadRequest, "该分类下仍有提示词，不允许删除")
		return
	}

	_, err = h.db.Exec("DELETE FROM prompt_category WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, nil)
}

// GetPrompts 获取提示词列表
func (h *PromptHandler) GetPrompts(c *gin.Context) {
	categoryID := c.Query("category_id")
	keyword := c.Query("keyword")

	query := "SELECT * FROM prompt_template WHERE 1=1"
	args := []interface{}{}

	if categoryID != "" {
		query += " AND category_id = ?"
		args = append(args, categoryID)
	}
	if keyword != "" {
		query += " AND (prompt_name LIKE ? OR prompt_key LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	query += " ORDER BY updated_at DESC"

	var prompts []model.PromptTemplate
	err := h.db.Select(&prompts, query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, prompts)
}

// GetPromptDetail 获取提示词详情
func (h *PromptHandler) GetPromptDetail(c *gin.Context) {
	id := c.Param("id")
	var prompt model.PromptTemplate
	err := h.db.Get(&prompt, "SELECT * FROM prompt_template WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "提示词未找到")
		return
	}
	Success(c, prompt)
}

// SavePrompt 保存提示词（包括版本生成）
func (h *PromptHandler) SavePrompt(c *gin.Context) {
	var input model.PromptTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var oldTemplate model.PromptTemplate
	isNew := input.ID == 0
	newVersion := 1

	if !isNew {
		err = tx.Get(&oldTemplate, "SELECT * FROM prompt_template WHERE id = ?", input.ID)
		if err != nil {
			Error(c, http.StatusNotFound, "原提示词未找到")
			return
		}
		newVersion = oldTemplate.Version + 1
	}

	// 1. 更新/插入主表
	if isNew {
		res, err := tx.Exec("INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, system_content, variables, status, version, remark) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			input.PromptKey, input.PromptName, input.CategoryID, input.Scenario, input.Content, input.SystemContent, input.Variables, input.Status, 1, input.Remark)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		lastID, _ := res.LastInsertId()
		input.ID = lastID
	} else {
		_, err = tx.Exec("UPDATE prompt_template SET prompt_name = ?, category_id = ?, scenario = ?, content = ?, system_content = ?, variables = ?, status = ?, version = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			input.PromptName, input.CategoryID, input.Scenario, input.Content, input.SystemContent, input.Variables, input.Status, newVersion, input.Remark, input.ID)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 2. 生成版本记录
	_, err = tx.Exec("INSERT INTO prompt_template_version (template_id, prompt_key, version, content, system_content, change_summary) VALUES (?, ?, ?, ?, ?, ?)",
		input.ID, input.PromptKey, newVersion, input.Content, input.SystemContent, "自动保存版本")
	if err != nil {
		Error(c, http.StatusInternalServerError, "生成版本失败: "+err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 3. 失效缓存
	h.promptService.InvalidateCache(input.PromptKey)

	Success(c, input.ID)
}

// GetVersionHistory 获取版本历史
func (h *PromptHandler) GetVersionHistory(c *gin.Context) {
	id := c.Param("id")
	var versions []model.PromptVersion
	err := h.db.Select(&versions, "SELECT * FROM prompt_template_version WHERE template_id = ? ORDER BY version DESC", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, versions)
}

// Rollback 回滚版本
func (h *PromptHandler) Rollback(c *gin.Context) {
	var input struct {
		TemplateID int64 `json:"template_id" binding:"required"`
		VersionID  int64 `json:"version_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var version model.PromptVersion
	err := h.db.Get(&version, "SELECT * FROM prompt_template_version WHERE id = ?", input.VersionID)
	if err != nil {
		Error(c, http.StatusNotFound, "版本记录未找到")
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var template model.PromptTemplate
	err = tx.Get(&template, "SELECT * FROM prompt_template WHERE id = ?", input.TemplateID)
	if err != nil {
		Error(c, http.StatusNotFound, "提示词主记录未找到")
		return
	}

	newVersionNo := template.Version + 1
	_, err = tx.Exec("UPDATE prompt_template SET content = ?, system_content = ?, version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		version.Content, version.SystemContent, newVersionNo, input.TemplateID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = tx.Exec("INSERT INTO prompt_template_version (template_id, prompt_key, version, content, system_content, change_summary) VALUES (?, ?, ?, ?, ?, ?)",
		input.TemplateID, template.PromptKey, newVersionNo, version.Content, version.SystemContent, "从版本 v"+strconv.Itoa(version.Version)+" 回滚")
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.promptService.InvalidateCache(template.PromptKey)

	Success(c, nil)
}
