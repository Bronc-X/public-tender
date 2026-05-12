package handler

import (
	"backend_go/internal/model"
	"backend_go/internal/service"
	"database/sql"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type FileHandler struct {
	db          *sqlx.DB
	fileService *service.FileService
	taskService *service.FileTaskService
	docMind     *service.DocMindParseService
}

func NewFileHandler(db *sqlx.DB, fileService *service.FileService, taskService *service.FileTaskService, docMind *service.DocMindParseService) *FileHandler {
	return &FileHandler{
		db:          db,
		fileService: fileService,
		taskService: taskService,
		docMind:     docMind,
	}
}

func (h *FileHandler) GetFileAsset(c *gin.Context) {
	id := c.Param("id")
	var file model.FileAsset
	err := h.db.Get(&file, "SELECT * FROM file_asset WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "File asset not found")
		return
	}
	c.JSON(200, file)
}

func (h *FileHandler) ListFiles(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	files, err := h.fileService.ListFiles(cid)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to list files")
		return
	}
	c.JSON(200, files)
}
func (h *FileHandler) RouteFile(c *gin.Context) {
	id := c.Param("id")
	var file model.FileAsset
	err := h.db.Get(&file, "SELECT * FROM file_asset WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	// Recommendations based on file type
	var recommendedEngine = "paddle_ocr"
	var recommendedMode = "fast"
	var deployType = "local"
	var confidence = 0.95

	ext := ""
	if file.Ext != nil {
		ext = *file.Ext
	}
	mime := ""
	if file.MimeType != nil {
		mime = *file.MimeType
	}

	if ext == ".pdf" || mime == "application/pdf" {
		recommendedMode = "fast"      // Default for PDFs (assuming digital)
		recommendedEngine = "glm_ocr" // For better structure in documents
	} else if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		recommendedMode = "accurate"
		recommendedEngine = "paddle_ocr"
	}

	c.JSON(200, gin.H{
		"file_id":            id,
		"recommended_engine": recommendedEngine,
		"recommended_mode":   recommendedMode,
		"deploy_type":        deployType,
		"confidence":         confidence,
	})
}

func (h *FileHandler) UploadFile(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	file, err := c.FormFile("file")
	if err != nil {
		Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	// Create directory if not exists
	uploadDir := "data/files/incoming/"
	err = os.MkdirAll(uploadDir, 0755)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate UUID filename
	ext := filepath.Ext(file.Filename)
	id := uuid.New().String()
	saveName := id + ext
	savePath := filepath.Join(uploadDir, saveName)

	// Save file
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		Error(c, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// Get absolute path
	absPath, _ := filepath.Abs(savePath)

	// Register in DB（浏览器常把 PDF 标成 application/octet-stream，需按扩展名纠正，否则审核页误判为图片、且无法触发 PDF OCR 队列）
	mimeType := c.DefaultPostForm("mime_type", "application/octet-stream")
	size := file.Size
	extStr := strings.ToLower(ext)
	mimeLower := strings.ToLower(mimeType)
	switch extStr {
	case ".pdf":
		if !strings.Contains(mimeLower, "pdf") {
			mimeType = "application/pdf"
		}
	case ".jpg", ".jpeg":
		if mimeType == "application/octet-stream" || mimeType == "" {
			mimeType = "image/jpeg"
		}
	case ".png":
		if mimeType == "application/octet-stream" || mimeType == "" {
			mimeType = "image/png"
		}
	case ".gif":
		if mimeType == "application/octet-stream" || mimeType == "" {
			mimeType = "image/gif"
		}
	case ".webp":
		if mimeType == "application/octet-stream" || mimeType == "" {
			mimeType = "image/webp"
		}
	}

	sourceModule := c.DefaultPostForm("source_module", "general")
	sourceProjectID := c.PostForm("source_project_id")

	fileAsset := &model.FileAsset{
		ID:              id,
		CompanyID:       &cid,
		FileName:        file.Filename,
		StoredPath:      &absPath,
		FileSize:        &size,
		MimeType:        &mimeType,
		Ext:             &extStr,
		SourceModule:    &sourceModule,
		SourceProjectID: &sourceProjectID,
		ScanStatus:      stringPtr("uploaded"),
	}

	err = h.fileService.CreateFileAsset(fileAsset)
	if err != nil {
		log.Printf("[File] DB insert error: %v", err)
		Error(c, http.StatusInternalServerError, "Failed to register file")
		return
	}

	// TRIGGER AUTO-TASK for Image/PDF/Word（同时认扩展名，避免 octet-stream 漏掉 PDF）
	if strings.Contains(strings.ToLower(mimeType), "image") || strings.Contains(strings.ToLower(mimeType), "pdf") ||
		extStr == ".pdf" || extStr == ".jpg" || extStr == ".jpeg" || extStr == ".png" || extStr == ".gif" || extStr == ".webp" ||
		extStr == ".doc" || extStr == ".docx" {
		h.taskService.StartTask(id, cid, "auto", sourceModule)
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        id,
		"file_name": file.Filename,
		"status":    "uploaded",
	})
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	log.Printf("[File] Deleting file %s (company %s)", id, cid)

	// 先中止可能正在跑的 OCR（释放 worker、取消进行中的 HTTP），再删库
	h.taskService.MarkTasksAbortedForFile(id)

	// 1. Get file path to delete from disk and actually delete it
	var filePath sql.NullString
	if queryErr := h.db.Get(&filePath, "SELECT stored_path FROM file_asset WHERE id = ? AND company_id = ?", id, cid); queryErr == nil && filePath.Valid {
		log.Printf("[File] Removing physical file: %s", filePath.String)
		os.Remove(filePath.String)
	}

	// 删除按页切割衍生出来的图片切片目录
	pagesDir := filepath.Join("data", "files", "pages", id)
	log.Printf("[File] Removing pages directory: %s", pagesDir)
	os.RemoveAll(pagesDir)

	// 2. Delete related records
	tx, err := h.db.Begin()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}

	// Delete content
	tx.Exec("DELETE FROM file_content WHERE file_asset_id = ?", id)
	// Delete OCR results
	tx.Exec("DELETE FROM file_ocr_result WHERE file_asset_id = ?", id)
	// Delete audit items
	tx.Exec("DELETE FROM audit_item WHERE file_id = ?", id)
	// Delete tasks
	tx.Exec("DELETE FROM file_ocr_task WHERE file_asset_id = ?", id)

	// --- CASCADING DELETION FOR LIBRARY RECORDS ---
	// 1. Delete person_certificate linkages for qualifications that came from this file
	tx.Exec("DELETE FROM person_certificate WHERE qualification_id IN (SELECT id FROM qualification WHERE file_asset_id = ?)", id)
	// 2. Delete qualification records
	tx.Exec("DELETE FROM qualification WHERE file_asset_id = ?", id)
	// 3. Delete person_education
	tx.Exec("DELETE FROM person_education WHERE file_asset_id = ?", id)
	// 4. Delete person_work_experience
	tx.Exec("DELETE FROM person_work_experience WHERE file_asset_id = ?", id)
	// 5. Delete project_performance records
	tx.Exec("DELETE FROM project_performance WHERE file_asset_id = ?", id)
	// 6. Delete document_record records
	tx.Exec("DELETE FROM document_record WHERE file_asset_id = ?", id)
	// ------------------------------------------------

	// Delete asset
	_, err = tx.Exec("DELETE FROM file_asset WHERE id = ? AND company_id = ?", id, cid)

	if err != nil {
		tx.Rollback()
		Error(c, http.StatusInternalServerError, "Failed to delete file asset")
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *FileHandler) RenameFile(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")

	var req struct {
		FileName string `json:"file_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.db.Exec("UPDATE file_asset SET file_name = ? WHERE id = ? AND company_id = ?", req.FileName, id, companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to rename file")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		Error(c, http.StatusNotFound, "File not found or unauthorized")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "renamed", "new_name": req.FileName})
}

// ServeFilePage 供前端读取拆分后的页面图片，路径 /api/files/:id/pages/:num
func (h *FileHandler) ServeFilePage(c *gin.Context) {
	id := c.Param("id")
	pageNum := c.Param("num")

	// 图片路径：data/files/pages/<id>/page-<num>.png
	imagePath := filepath.Join("data", "files", "pages", id, fmt.Sprintf("page-%s.png", pageNum))

	// 检查文件是否存在
	if _, err := os.Stat(imagePath); err != nil {
		Error(c, http.StatusNotFound, "页面图片未找到或尚未拆分")
		return
	}

	c.File(imagePath)
}

func stringPtr(s string) *string {
	return &s
}

// GetFileParsed 返回 file_content 中的 Markdown（纯文本响应，兼容现有前端直接把 res.data 交给 ReactMarkdown）。
func (h *FileHandler) GetFileParsed(c *gin.Context) {
	h.writeFileParsedPlain(c, c.Param("id"))
}

// ServeFileParsed 与 GetFileParsed 相同，路径为 GET /api/file-parsed/:id，避免 /files/:id/parsed 落入 NoRoute 代理产生 502。
func (h *FileHandler) ServeFileParsed(c *gin.Context) {
	h.writeFileParsedPlain(c, c.Param("id"))
}

func (h *FileHandler) writeFileParsedPlain(c *gin.Context, fileAssetID string) {
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var md, pt sql.NullString
	err := h.db.QueryRow(`
		SELECT c.markdown_text, c.plain_text
		FROM file_content c
		INNER JOIN file_asset f ON f.id = c.file_asset_id
		WHERE c.file_asset_id = ? AND IFNULL(f.company_id, '') = ?
		ORDER BY c.created_at DESC
		LIMIT 1
	`, fileAssetID, cid).Scan(&md, &pt)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.String(http.StatusOK, "")
			return
		}
		log.Printf("[File] writeFileParsedPlain asset=%s: %v", fileAssetID, err)
		Error(c, http.StatusInternalServerError, "读取解析内容失败")
		return
	}
	out := ""
	if md.Valid && strings.TrimSpace(md.String) != "" {
		out = md.String
	} else if pt.Valid {
		out = pt.String
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, out)
}

// NormalizeDocMindStoredMarkdown converts stored docmind JSON to readable markdown without calling cloud parsing.
func (h *FileHandler) NormalizeDocMindStoredMarkdown(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	var contentID string
	var md, pt sql.NullString
	err := h.db.QueryRow(`
		SELECT c.id, c.markdown_text, c.plain_text
		FROM file_content c
		INNER JOIN file_asset f ON f.id = c.file_asset_id
		WHERE c.file_asset_id = ? AND IFNULL(f.company_id, '') = ?
		ORDER BY c.created_at DESC
		LIMIT 1
	`, id, cid).Scan(&contentID, &md, &pt)
	if err != nil {
		if err == sql.ErrNoRows {
			Error(c, http.StatusNotFound, "未找到可转换的解析内容")
			return
		}
		Error(c, http.StatusInternalServerError, "读取解析内容失败")
		return
	}

	raw := ""
	if md.Valid && strings.TrimSpace(md.String) != "" {
		raw = md.String
	} else if pt.Valid && strings.TrimSpace(pt.String) != "" {
		raw = pt.String
	}
	if strings.TrimSpace(raw) == "" {
		Error(c, http.StatusBadRequest, "当前内容为空，无法转换")
		return
	}

	normalized, convErr := service.MarkdownFromDocMindStoredJSON(raw)
	if convErr != nil {
		msg := convErr.Error()
		if strings.Contains(msg, "invalid docmind json") {
			Error(c, http.StatusBadRequest, "当前内容不是 JSON，或已是 Markdown 正文，无需转换")
			return
		}
		Error(c, http.StatusBadRequest, "无法从当前 JSON 中提取 Markdown")
		return
	}
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		Error(c, http.StatusBadRequest, "转换结果为空")
		return
	}
	if strings.TrimSpace(raw) == normalized {
		c.JSON(http.StatusOK, gin.H{"ok": true, "markdown_length": len(normalized), "updated": false})
		return
	}
	_, err = h.db.Exec(`UPDATE file_content SET markdown_text = ?, plain_text = ?, content_type = ? WHERE id = ?`,
		normalized, normalized, "docmind_json_normalized", contentID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "保存转换结果失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "markdown_length": len(normalized), "updated": true})
}

// AliyunDocParse 调用阿里云文档智能解析当前文件并写入 file_content（同步轮询，耗时可能数分钟）。
func (h *FileHandler) AliyunDocParse(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	cid := companyID.(string)

	if h.docMind == nil || !h.docMind.Enabled() {
		Error(c, http.StatusBadRequest, "未配置阿里云文档解析密钥，请在系统设置中填写 doc_parser_access_key / doc_parser_access_secret")
		return
	}

	var asset model.FileAsset
	err := h.db.Get(&asset, "SELECT * FROM file_asset WHERE id = ? AND IFNULL(company_id, '') = ?", id, cid)
	if err != nil {
		Error(c, http.StatusNotFound, "文件不存在或无权访问")
		return
	}
	if asset.StoredPath == nil || *asset.StoredPath == "" {
		Error(c, http.StatusBadRequest, "文件未存储在磁盘")
		return
	}
	ext := ""
	if asset.Ext != nil {
		ext = strings.ToLower(strings.TrimSpace(*asset.Ext))
	}
	if ext != ".pdf" && ext != ".doc" && ext != ".docx" {
		Error(c, http.StatusBadRequest, "仅支持 PDF / Word 的阿里云文档解析")
		return
	}

	resolvedPath, resErr := h.resolveStoredPathOnDisk(*asset.StoredPath)
	if resErr != nil {
		Error(c, http.StatusNotFound, "文件在磁盘上不存在或路径已失效，请重新上传")
		return
	}

	md, plain, err := h.docMind.ParseLocalFile(resolvedPath, asset.FileName)
	if err != nil {
		log.Printf("[File] Aliyun doc parse failed for %s: %v", id, err)
		Error(c, http.StatusBadGateway, err.Error())
		return
	}

	if err := h.upsertFileContent(id, plain, md, "aliyun_docmind"); err != nil {
		log.Printf("[File] upsert file_content: %v", err)
		Error(c, http.StatusInternalServerError, "保存解析结果失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"markdown_text": md,
		"plain_text":    plain,
		"status":        "ok",
	})
}

func (h *FileHandler) upsertFileContent(fileAssetID, plain, md, contentType string) error {
	var existing string
	err := h.db.Get(&existing, "SELECT id FROM file_content WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1", fileAssetID)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		existing = ""
	}
	if existing != "" {
		_, err = h.db.Exec(`UPDATE file_content SET plain_text = ?, markdown_text = ?, content_type = ? WHERE id = ?`,
			plain, md, contentType, existing)
		return err
	}
	cid := uuid.New().String()
	_, err = h.db.Exec(`INSERT INTO file_content (id, file_asset_id, plain_text, markdown_text, content_type) VALUES (?, ?, ?, ?, ?)`,
		cid, fileAssetID, plain, md, contentType)
	return err
}

// resolveStoredPathOnDisk 在库中路径指向的文件不存在时，按工作目录与 data/files/incoming  basename 回退查找（解决进程 cwd 与上传时不一致）。
func (h *FileHandler) resolveStoredPathOnDisk(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", fmt.Errorf("empty stored_path")
	}
	try := func(p string) (string, bool) {
		p = strings.TrimSpace(p)
		if p == "" {
			return "", false
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			return "", false
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return p, true
		}
		return abs, true
	}
	if p, ok := try(stored); ok {
		return p, nil
	}
	wd, wdErr := os.Getwd()
	base := filepath.Base(stored)
	candidates := []string{}
	if wdErr == nil {
		if !filepath.IsAbs(stored) {
			candidates = append(candidates, filepath.Join(wd, stored))
		}
		candidates = append(candidates,
			filepath.Join(wd, "data/files/incoming", base),
			filepath.Join(wd, "backend_go", "data/files/incoming", base),
		)
	}
	candidates = append(candidates,
		filepath.Join("data/files/incoming", base),
		filepath.Join("backend_go", "data/files/incoming", base),
	)
	for _, p := range candidates {
		if p, ok := try(p); ok {
			log.Printf("[File] resolved stored_path via fallback: %s -> %s", stored, p)
			return p, nil
		}
	}
	return "", fmt.Errorf("file not on disk for stored_path=%s", stored)
}

// serveFileContent 仅按 id 读取 stored_path 并输出文件，避免将整行扫描进 FileAsset 时因可空字段/列不一致导致扫描失败而误报「File not found」。
func (h *FileHandler) serveFileContent(c *gin.Context, id string) {
	if strings.TrimSpace(id) == "" {
		Error(c, http.StatusBadRequest, "File ID is required")
		return
	}

	var storedPath sql.NullString
	err := h.db.Get(&storedPath, "SELECT stored_path FROM file_asset WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			// --- RESILIENCE FALLBACK: Check for stray files on disk if missing from DB ---
			base := strings.TrimSpace(id)
			candidates := []string{
				filepath.Join("data/files/incoming", base+".pdf"),
				filepath.Join("backend_go/data/files/incoming", base+".pdf"),
			}
			wd, wdErr := os.Getwd()
			if wdErr == nil {
				candidates = append(candidates,
					filepath.Join(wd, "data/files/incoming", base+".pdf"),
					filepath.Join(wd, "backend_go/data/files/incoming", base+".pdf"),
				)
			}
			for _, p := range candidates {
				if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
					log.Printf("[File] serveFileContent: %s missing from DB, but found on disk: %s", id, p)
					ctype := "application/pdf"
					c.Header("Content-Type", ctype)
					c.Header("Content-Disposition", "inline")
					c.File(p)
					return
				}
			}
			Error(c, http.StatusNotFound, "File not found")
		} else {
			log.Printf("[File] serveFileContent lookup id=%s: %v", id, err)
			Error(c, http.StatusInternalServerError, "Failed to load file")
		}
		return
	}
	if !storedPath.Valid || strings.TrimSpace(storedPath.String) == "" {
		Error(c, http.StatusNotFound, "Storage path is empty")
		return
	}

	resolved, resErr := h.resolveStoredPathOnDisk(storedPath.String)
	if resErr != nil {
		log.Printf("[File] serveFileContent id=%s resolve: %v", id, resErr)
		Error(c, http.StatusNotFound, "文件在磁盘上不存在或路径已失效，请重新上传")
		return
	}

	f, err := os.Open(resolved)
	if err != nil {
		log.Printf("[File] serveFileContent open %s: %v", resolved, err)
		Error(c, http.StatusInternalServerError, "无法打开文件")
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		Error(c, http.StatusInternalServerError, "无法读取文件信息")
		return
	}

	if strings.EqualFold(filepath.Ext(resolved), ".pdf") {
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "inline")
	} else {
		ctype := mime.TypeByExtension(filepath.Ext(resolved))
		if ctype != "" {
			c.Header("Content-Type", ctype)
		}
	}

	http.ServeContent(c.Writer, c.Request, filepath.Base(resolved), fi.ModTime(), f)
}

func (h *FileHandler) DownloadFile(c *gin.Context) {
	h.serveFileContent(c, c.Param("id"))
}

// DownloadFileBinary 与 DownloadFile 相同逻辑，路径为 GET /api/files/:id/binary，避免部分环境下对 /files/download/:id 的误匹配或代理问题。
func (h *FileHandler) DownloadFileBinary(c *gin.Context) {
	h.serveFileContent(c, c.Param("id"))
}

// ServeFileBinary 路径 GET /api/file-binary/:id，与 /files/:id 树完全分离，避免未匹配路由被 NoRoute 代理到 Node 返回 502。
func (h *FileHandler) ServeFileBinary(c *gin.Context) {
	h.serveFileContent(c, c.Param("id"))
}
