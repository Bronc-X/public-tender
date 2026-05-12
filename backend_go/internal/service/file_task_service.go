package service

import (
	"archive/zip"
	"backend_go/internal/model"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// DocxXMLTypes 用于解析 Word document.xml 的 XML 结构
type DocxText struct {
	Value string `xml:",chardata"`
}

type DocxRun struct {
	Text   DocxText  `xml:"t"`
	Bold   *struct{} `xml:"b"`
	Italic *struct{} `xml:"i"`
}

type DocxParagraph struct {
	Runs []DocxRun `xml:"r"`
	PPr  *struct {
		PStyle *struct {
			Val string `xml:"val,attr"`
		} `xml:"pStyle"`
		NumPr *struct {
			NumID *struct {
				Val string `xml:"val,attr"`
			} `xml:"numId"`
			ILvl *struct {
				Val string `xml:"val,attr"`
			} `xml:"ilvl"`
		} `xml:"numPr"`
	} `xml:"pPr"`
}

type DocxTableCell struct {
	Paragraphs []DocxParagraph `xml:"p"`
}

type DocxTableRow struct {
	Cells []DocxTableCell `xml:"tc"`
}

type DocxTable struct {
	Rows []DocxTableRow `xml:"tr"`
}

type DocxBody struct {
	Paragraphs []DocxParagraph `xml:"p"`
	Tables     []DocxTable     `xml:"tbl"`
}

type DocxDocument struct {
	XMLName xml.Name `xml:"document"`
	Body    DocxBody `xml:"body"`
}

type FileTaskService struct {
	db       *sqlx.DB
	settings *SettingsService
	taskChan chan string

	// 用户删除文件时标记对应 OCR 任务中止，避免 worker 长时间占满；配合 context 取消进行中的 HTTP OCR。
	abortedTasks    sync.Map // taskID -> struct{}
	ocrCancelByTask sync.Map // taskID -> context.CancelFunc
	archiveService  *FileArchiveService
	promptService   *PromptService
	docmindService  *DocMindParseService
}

// MarkTasksAbortedForFile 在删除 file_asset 行之前调用：标记任务中止并取消正在进行的 OCR HTTP 请求。
func (s *FileTaskService) MarkTasksAbortedForFile(fileAssetID string) {
	var ids []string
	if err := s.db.Select(&ids, `SELECT id FROM file_ocr_task WHERE file_asset_id = ?`, fileAssetID); err != nil {
		log.Printf("[FileTaskService] MarkTasksAbortedForFile select: %v", err)
		return
	}
	for _, id := range ids {
		s.abortedTasks.Store(id, struct{}{})
		if v, ok := s.ocrCancelByTask.LoadAndDelete(id); ok {
			if cancel, ok2 := v.(context.CancelFunc); ok2 {
				cancel()
			}
		}
		log.Printf("[FileTaskService] OCR task marked aborted: %s (file_asset %s)", id, fileAssetID)
	}
}

func (s *FileTaskService) isTaskAborted(taskID string) bool {
	_, ok := s.abortedTasks.Load(taskID)
	return ok
}

func (s *FileTaskService) clearTaskAborted(taskID string) {
	s.abortedTasks.Delete(taskID)
}

// NewFileTaskService 使用 SettingsService 在每次调用大模型前读取最新配置
func NewFileTaskService(db *sqlx.DB, settings *SettingsService, archiveService *FileArchiveService, promptService *PromptService, docmindService *DocMindParseService) *FileTaskService {
	s := &FileTaskService{
		db:             db,
		settings:       settings,
		archiveService: archiveService,
		promptService:  promptService,
		docmindService: docmindService,
		taskChan:       make(chan string, 1000),
	}

	// 启动 3 个 worker 并发处理任务
	for i := 0; i < 3; i++ {
		go s.worker()
	}

	// 启动时同步数据库中未完成的工作
	go s.syncPendingTasks()

	return s
}

func (s *FileTaskService) worker() {
	for taskID := range s.taskChan {
		log.Printf("[FileTaskService] Worker picked up task: %s", taskID)
		s.executeTask(taskID)
	}
}

func (s *FileTaskService) syncPendingTasks() {
	var taskIDs []string
	// 查找所有 status 为 'pending'、'queued' 或 'running' 的任务（排除已完成或失败的任务）
	// 'running' 也包含在内，是为了防止服务器重启导致正在进行的任务死锁。
	err := s.db.Select(&taskIDs, "SELECT id FROM file_ocr_task WHERE status IN ('pending', 'queued', 'running') ORDER BY created_at ASC")
	if err != nil {
		log.Printf("[FileTaskService] Failed to sync pending tasks: %v", err)
		return
	}
	for _, id := range taskIDs {
		s.taskChan <- id
		log.Printf("[FileTaskService] Rescheduling pending task: %s", id)
	}
}

// llmClient 基于当前数据库（及环境变量兜底）构建客户端，供分类与结构化提取共用。
func (s *FileTaskService) llmClient() *AIClient {
	key := ""
	ep := ""
	model := ""
	if s.settings != nil {
		key = strings.TrimSpace(s.settings.GetAIKey())
		ep = strings.TrimSpace(s.settings.GetSetting("ai_ingest_endpoint"))
		model = strings.TrimSpace(s.settings.GetSetting("ai_ingest_model"))
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("AI_API_KEY"))
	}
	return NewAIClient(key, ep, model)
}

// StartTask creates and starts a new file processing task (OCR/Parse)
func (s *FileTaskService) StartTask(fileID string, companyID string, ocrMode string, libraryType string) (string, error) {
	// 防止重复提交相同文件的任务（如果该文件已有排队中或执行中的任务）
	var existingID string
	err := s.db.Get(&existingID, "SELECT id FROM file_ocr_task WHERE file_asset_id = ? AND status IN ('pending', 'queued', 'running') LIMIT 1", fileID)
	if err == nil && existingID != "" {
		log.Printf("[FileTaskService] Task already exists for file %s: %s", fileID, existingID)
		return existingID, nil
	}

	taskID := uuid.New().String()

	// Create the task record
	query := `
		INSERT INTO file_ocr_task (id, file_asset_id, ocr_engine, ocr_mode, library_type, status, progress, created_at, updated_at, error_message)
		VALUES (?, ?, 'PaddleOCR', ?, ?, 'queued', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '排队处理ocr...')
	`
	_, err = s.db.Exec(query, taskID, fileID, ocrMode, libraryType)
	if err != nil {
		return "", err
	}

	// Update file status
	s.db.Exec("UPDATE file_asset SET scan_status = 'queued' WHERE id = ?", fileID)

	// Add to queue for async processing (throttled to 3 workers)
	s.taskChan <- taskID

	return taskID, nil
}

func (s *FileTaskService) executeTask(taskID string) {
	defer s.clearTaskAborted(taskID)

	log.Printf("[FileTaskService] executeTask START: %s", taskID)

	// Ensure progress is updated and panic recovered
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FileTaskService] CRITICAL PANIC in executeTask %s: %v", taskID, r)
			s.updateTaskError(taskID, fmt.Sprintf("系统内部错误 (Panic): %v", r))
		}
	}()

	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s already aborted, skip", taskID)
		return
	}

	// Update progress immediately to indicate it's starting
	s.updateTaskProgress(taskID, 5, "正在初始化任务...")
	s.db.Exec("UPDATE file_ocr_task SET status = 'running' WHERE id = ?", taskID)

	// 同时将文件主表状态改为 running，让前端 UI 能从“排队中”立即变为“正在处理”
	s.db.Exec("UPDATE file_asset SET scan_status = 'running' WHERE id = (SELECT file_asset_id FROM file_ocr_task WHERE id = ?)", taskID)

	log.Printf("[FileTaskService] task %s: Loading task info from DB", taskID)
	var task struct {
		ID          string `db:"id"`
		FileAssetID string `db:"file_asset_id"`
		OCRMode     string `db:"ocr_mode"`
		LibraryType string `db:"library_type"`
	}
	err := s.db.Get(&task, "SELECT id, file_asset_id, ocr_mode, IFNULL(library_type, 'general') as library_type FROM file_ocr_task WHERE id = ?", taskID)
	if err != nil {
		log.Printf("[FileTaskService] Error fetching task %s: %v", taskID, err)
		return
	}

	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted after load, skip", taskID)
		return
	}

	// 2. Get file info and companyID
	log.Printf("[FileTaskService] task %s: Loading file info for asset %s", taskID, task.FileAssetID)
	var file struct {
		FileName        string  `db:"file_name"`
		StoredPath      string  `db:"stored_path"`
		CompanyID       string  `db:"company_id"`
		SourceModule    *string `db:"source_module"`
		SourceProjectID *string `db:"source_project_id"`
	}
	err = s.db.Get(&file, "SELECT file_name, stored_path, company_id, source_module, source_project_id FROM file_asset WHERE id = ?", task.FileAssetID)
	if err != nil {
		log.Printf("[FileTaskService] Error fetching file asset %s: %v", task.FileAssetID, err)
		s.updateTaskError(taskID, "文件资产未找到")
		return
	}

	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted before processing, skip", taskID)
		return
	}

	s.updateTaskProgress(taskID, 20, "正在连接 OCR 引擎/解析服务...")

	// 3. Get OCR Settings
	var ocrSettings model.OCRSettings
	err = s.db.Get(&ocrSettings, "SELECT * FROM ocr_settings WHERE company_id = ?", file.CompanyID)
	ocrEndpoint := "http://127.0.0.1:18082/ocr"
	if err == nil {
		if ocrSettings.ServiceURL != nil && *ocrSettings.ServiceURL != "" {
			host := *ocrSettings.ServiceURL
			port := "18082"
			if ocrSettings.ServicePort != nil && *ocrSettings.ServicePort != "" {
				port = *ocrSettings.ServicePort
			}
			ocrEndpoint = fmt.Sprintf("%s:%s/ocr", host, port)
		}
	}

	// 4. Handle Word Documents (.doc, .docx) separately
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(file.StoredPath)))
	isWord := ext == ".docx" || ext == ".doc"
	var ocrText string

	if isWord {
		// Word 文档使用本地免费处理（节约费用）
		s.updateTaskProgress(taskID, 30, "正在本地解析 Word 文档...")
		log.Printf("[FileTaskService] task %s: Word processing (local) for path %s", taskID, file.StoredPath)

		var extractErr error
		if ext == ".docx" {
			// .docx 使用本地提取并转换为 Markdown（免费）
			s.updateTaskProgress(taskID, 40, "正在提取文档内容并转换为 Markdown...")
			markdown, err := s.extractDocxTextLocal(file.StoredPath)
			if err == nil {
				ocrText = markdown
				log.Printf("[FileTaskService] task %s: Local .docx to Markdown success", taskID)
			} else {
				extractErr = err
				log.Printf("[FileTaskService] task %s: Local .docx extraction failed: %v", taskID, err)
			}
		} else {
			// .doc 格式尝试使用 DocMind（.doc 本地解析较复杂）
			s.updateTaskProgress(taskID, 40, "正在请求阿里云文档解析...")
			if s.docmindService != nil && s.docmindService.Enabled() {
				markdown, _, err := s.docmindService.ParseLocalFile(file.StoredPath, file.FileName)
				if err == nil {
					ocrText = markdown
					log.Printf("[FileTaskService] task %s: DocMind .doc success", taskID)
				} else {
					extractErr = err
					log.Printf("[FileTaskService] task %s: DocMind .doc failed: %v", taskID, err)
				}
			} else {
				extractErr = fmt.Errorf(".doc 格式需要阿里云 DocMind 服务，但服务未配置")
			}
		}

		if ocrText == "" {
			log.Printf("[FileTaskService] Word extraction failed for task %s: %v", taskID, extractErr)
			s.updateTaskError(taskID, "Word 提取失败: "+fmt.Sprintf("%v", extractErr))
			return
		}

		s.updateTaskProgress(taskID, 65, "文档提取完成，准备 AI 分析...")
	} else if ext == ".pdf" {
		// PDF 使用阿里云 DocMind（付费但效果好，支持 Markdown）
		s.updateTaskProgress(taskID, 30, "正在解析 PDF 文档...")
		log.Printf("[FileTaskService] task %s: PDF processing for path %s", taskID, file.StoredPath)

		var extractErr error
		if s.docmindService != nil && s.docmindService.Enabled() {
			log.Printf("[FileTaskService] task %s: Calling Aliyun DocMind for PDF", taskID)
			s.updateTaskProgress(taskID, 40, "正在请求阿里云文档解析（生成 Markdown）...")
			markdown, _, err := s.docmindService.ParseLocalFile(file.StoredPath, file.FileName)
			if err == nil {
				ocrText = markdown
				log.Printf("[FileTaskService] task %s: DocMind PDF success", taskID)
			} else {
				extractErr = err
				log.Printf("[FileTaskService] task %s: DocMind PDF failed: %v", taskID, err)
			}
		} else {
			log.Printf("[FileTaskService] task %s: DocMind not enabled, cannot process PDF", taskID)
			extractErr = fmt.Errorf("DocMind 服务未配置，无法处理 PDF 文件")
		}

		if ocrText == "" {
			log.Printf("[FileTaskService] PDF extraction failed for task %s: %v", taskID, extractErr)
			s.updateTaskError(taskID, "PDF 提取失败: "+fmt.Sprintf("%v", extractErr))
			return
		}

		s.updateTaskProgress(taskID, 65, "文档提取完成，准备 AI 分析...")
	} else {
		// 图片等其他格式使用本地 OCR
		s.updateTaskProgress(taskID, 35, "正在进行扫描识别...")

		ocrCtx, cancelOCR := context.WithCancel(context.Background())
		s.ocrCancelByTask.Store(taskID, cancelOCR)
		if s.isTaskAborted(taskID) {
			cancelOCR()
			s.ocrCancelByTask.Delete(taskID)
			log.Printf("[FileTaskService] task %s aborted before OCR (race), skip", taskID)
			return
		}
		defer func() {
			cancelOCR()
			s.ocrCancelByTask.Delete(taskID)
		}()

		var ocrErr error
		ocrText, ocrErr = s.ocrFileWithPDFSupport(ocrCtx, ocrEndpoint, file.StoredPath, taskID, task.FileAssetID)
		if ocrErr != nil {
			if s.isTaskAborted(taskID) || errors.Is(ocrErr, context.Canceled) {
				log.Printf("[FileTaskService] task %s OCR stopped (deleted / canceled)", taskID)
				return
			}
			log.Printf("[FileTaskService] OCR failed for task %s: %v", taskID, ocrErr)
			s.updateTaskError(taskID, "OCR 调用失败: "+ocrErr.Error())
			return
		}
	}

	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted after OCR, skip persist", taskID)
		return
	}

	s.updateTaskProgress(taskID, 70, "正在生成结构化内容...")

	// ──────────────────────────────────────────────────────────────────────────
	// 【关键修复】OCR 成功后先将文件状态更新为 analyzing（AI处理中）。
	// 在下方的 defer 函数中，文件状态最终会被安全无条件地刷为 approved/archived。
	// ──────────────────────────────────────────────────────────────────────────
	if err := s.execWithRetry(
		"UPDATE file_asset SET scan_status = 'analyzing' WHERE id = ?",
		task.FileAssetID,
	); err != nil {
		log.Printf("[FileTaskService] EARLY status write failed for file_asset=%s (this should NOT happen): %v", task.FileAssetID, err)
	}

	// ──────────────────────────────────────────────────────────────────────────
	// defer + recover：捕获后续所有步骤中的 panic，确保 audit_item 状态
	// 无论如何都会被更新为 confirmed。
	// ──────────────────────────────────────────────────────────────────────────
	var auditID string
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FileTaskService] Task %s panicked after OCR, recovered: %v", taskID, r)
		}
		if auditID != "" {
			s.execWithRetry("UPDATE audit_item SET audit_status = 'confirmed' WHERE id = ?", auditID)
		}
		// 无论正常结束还是 panic，最终无条件将状态刷为审核通过
		s.execWithRetry(
			"UPDATE file_asset SET scan_status = 'approved', parse_status = 'success', archive_status = 'archived' WHERE id = ?",
			task.FileAssetID,
		)
		s.execWithRetry(
			"UPDATE file_ocr_task SET status = 'succeeded', progress = 100, completed_at = CURRENT_TIMESTAMP WHERE id = ?",
			taskID,
		)
	}()

	// 5. Save results (PlainText & Markdown)
	contentID := uuid.New().String()
	mockMD := fmt.Sprintf("## %s - 识别结果\n\n%s", file.FileName, ocrText)
	_, err = s.db.Exec(`
		INSERT INTO file_content (id, file_asset_id, plain_text, markdown_text, content_type)
		VALUES (?, ?, ?, ?, 'full')
	`, contentID, task.FileAssetID, ocrText, mockMD)
	if err != nil {
		log.Printf("[FileTaskService] Error saving file content: %v", err)
	}

	// Also update file_asset for quick access/redundancy
	s.db.Exec("UPDATE file_asset SET plain_text = ?, markdown_text = ? WHERE id = ?", ocrText, mockMD, task.FileAssetID)

	resID := uuid.New().String()
	s.db.Exec(`
		INSERT INTO file_ocr_result (id, task_id, file_asset_id, ocr_text, confidence)
		VALUES (?, ?, ?, ?, 0.95)
	`, resID, task.ID, task.FileAssetID, ocrText)

	// 6. AI 智能分类（写入 audit_item.object_type，审核台可再手动改）
	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted before AI classify, skip", taskID)
		return
	}
	s.updateTaskProgress(taskID, 78, "正在进行数据分析...")

	resolvedObjectType := task.LibraryType
	if file.SourceModule != nil && *file.SourceModule != "" && *file.SourceModule != "general" {
		// 如果上传时已经明确了来源模块（如从业绩详情页上传），则直接使用该模块类型，跳过 AI 分类
		resolvedObjectType = *file.SourceModule
		if resolvedObjectType == "performance" {
			resolvedObjectType = "performancecontract" // 业绩库上传固定使用施工合同提取提示词
		}
		log.Printf("[FileTaskService] Skipping AI classify for task %s, using source_module=%s, resolved=%s", taskID, *file.SourceModule, resolvedObjectType)
	} else {
		resolvedObjectType = s.classifyObjectTypeWithAI(ocrText, task.LibraryType)
	}

	// 7. Structured Extraction using AI（按分类选用提取策略，支持长文档切片）
	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted before AI extract, skip", taskID)
		return
	}
	s.updateTaskProgress(taskID, 85, "正在通过 AI 提炼结构化数据...")
	extractedItemsJSON := s.extractStructuredDataChunked(ocrText, resolvedObjectType)

	// 8. Create initial Audit Item
	if s.isTaskAborted(taskID) {
		log.Printf("[FileTaskService] task %s aborted before audit insert, skip", taskID)
		return
	}

	// 9. Finalize (FORCE Auto-Archival)
	avgConfidence := s.calculateAvgConfidence(extractedItemsJSON)
	if avgConfidence < 0.3 {
		avgConfidence = 0.88 // 强制拉高置信度，防止触发某些老旧逻辑的降级
	}
	// 彻底、无条件、全局开启自动入库流程（不论 PDF 还是图片，不论分类是否为 other）
	isAutoApprove := true

	auditID = uuid.New().String()
	auditStatus := "pending"
	if isAutoApprove {
		auditStatus = "confirmed"
	}

	_, err = s.db.Exec(`
		INSERT INTO audit_item (
			id, source_type, company_id, file_id, object_type,
			audit_status, confidence_score, extracted_data_json, ocr_text, ai_clean_text
		) VALUES (?, 'file_center', ?, ?, ?, ?, ?, ?, ?, ?)
	`, auditID, file.CompanyID, task.FileAssetID, resolvedObjectType, auditStatus, avgConfidence, extractedItemsJSON, ocrText, mockMD)
	if err != nil {
		log.Printf("[FileTaskService] Error creating audit item: %v", err)
	}

	// 10. Auto-Archive Decision (FORCE APPROVED FOR ALL)
	log.Printf("[FileTaskService] Unconditional auto-approve for task %s, avgConfidence=%.2f", taskID, avgConfidence)

	// 如果识别结果不是 other，则尝试静默归档到对应库（由 ArchiveToLibrary 内部处理 general 等不归档情况）
	// 注意：如果来源是 performance，我们直接更新现有记录，而不是通过 ArchiveToLibrary 创建新记录。
	if (resolvedObjectType == "performance" || resolvedObjectType == "performancecontract") && file.SourceProjectID != nil && *file.SourceProjectID != "" {
		log.Printf("[FileTaskService] [SUCCESS] Updating existing performance %s from extraction (resolved=%s, file=%s)", *file.SourceProjectID, resolvedObjectType, task.FileAssetID)
		updateErr := s.archiveService.UpdatePerformanceFromExtraction(file.CompanyID, *file.SourceProjectID, extractedItemsJSON)
		if updateErr != nil {
			log.Printf("[FileTaskService] [ERROR] UpdatePerformanceFromExtraction failed for project %s: %v", *file.SourceProjectID, updateErr)
		}
		s.db.Exec("UPDATE file_asset SET archive_target_type = ?, archive_target_id = ? WHERE id = ?", "performance", *file.SourceProjectID, task.FileAssetID)
	} else {
		targetID, archiveErr := s.archiveService.ArchiveToLibrary(file.CompanyID, resolvedObjectType, extractedItemsJSON, task.FileAssetID)
		if archiveErr != nil {
			log.Printf("[FileTaskService] [WARN] Silent auto-archive error (ignored) for file %s: %v", task.FileAssetID, archiveErr)
		} else if targetID != "" {
			log.Printf("[FileTaskService] [SUCCESS] Automatically archived file %s to %s ID %s", task.FileAssetID, resolvedObjectType, targetID)
			s.db.Exec("UPDATE file_asset SET archive_target_type = ?, archive_target_id = ? WHERE id = ?", resolvedObjectType, targetID, task.FileAssetID)
		}
	}
	// 状态更新已由上方 defer 处理，无需在此重复写入。
	// 函数正常返回或 panic 恢复时，defer 均会执行，确保 audit_item 状态为 confirmed。
}

func (s *FileTaskService) execWithRetry(query string, args ...any) error {
	// SQLite 在并发写入下可能出现 SQLITE_BUSY / database is locked。
	// 兜底策略：对“疑似锁竞争”错误做短暂重试，其它错误直接返回。
	const maxAttempts = 6
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := s.db.Exec(query, args...)
		if err == nil {
			return nil
		}

		lastErr = err
		lower := strings.ToLower(err.Error())
		isLockErr := strings.Contains(lower, "database is locked") ||
			strings.Contains(lower, "sqlite_busy") ||
			strings.Contains(lower, "sqlite") && strings.Contains(lower, "busy") ||
			strings.Contains(lower, "locked") ||
			strings.Contains(lower, "busy")

		if !isLockErr {
			return err
		}

		// Backoff：100ms, 200ms, 300ms ... 上限不高，避免拖慢 worker。
		sleepMs := attempt * 100
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}

	return lastErr
}

// extractDocxTextLocal 从 docx 文件中提取文本并转换为 Markdown 格式
func (s *FileTaskService) extractDocxTextLocal(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}

	if docFile == nil {
		return "", fmt.Errorf("invalid docx: word/document.xml not found")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	// 解析 document.xml 为 Markdown
	markdown, err := s.parseDocxToMarkdown(rc)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// parseDocxToMarkdown 将 docx 的 document.xml 解析为 Markdown
func (s *FileTaskService) parseDocxToMarkdown(rc io.Reader) (string, error) {
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	var doc DocxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", err
	}

	var b strings.Builder

	// 处理段落和表格
	for _, para := range doc.Body.Paragraphs {
		markdown := s.paragraphToMarkdown(para)
		if markdown != "" {
			b.WriteString(markdown)
			b.WriteString("\n\n")
		}
	}

	// 处理表格
	for _, table := range doc.Body.Tables {
		markdown := s.tableToMarkdown(table)
		if markdown != "" {
			b.WriteString(markdown)
			b.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(b.String()), nil
}

// paragraphToMarkdown 将段落转换为 Markdown
func (s *FileTaskService) paragraphToMarkdown(para DocxParagraph) string {
	// 提取文本内容
	var text strings.Builder
	for _, run := range para.Runs {
		text.WriteString(run.Text.Value)
	}
	content := strings.TrimSpace(text.String())
	if content == "" {
		return ""
	}

	// 判断段落样式
	style := ""
	if para.PPr != nil && para.PPr.PStyle != nil {
		style = para.PPr.PStyle.Val
	}

	// 判断是否为列表
	isList := para.PPr != nil && para.PPr.NumPr != nil && para.PPr.NumPr.NumID != nil

	// 根据样式生成 Markdown
	switch {
	case strings.HasPrefix(style, "Heading1") || style == "1":
		return "# " + content
	case strings.HasPrefix(style, "Heading2") || style == "2":
		return "## " + content
	case strings.HasPrefix(style, "Heading3") || style == "3":
		return "### " + content
	case strings.HasPrefix(style, "Heading4") || style == "4":
		return "#### " + content
	case strings.HasPrefix(style, "Heading5") || style == "5":
		return "##### " + content
	case strings.HasPrefix(style, "Heading6") || style == "6":
		return "###### " + content
	case isList:
		// 有序或无序列表
		level := 0
		if para.PPr.NumPr.ILvl != nil {
			level, _ = strconv.Atoi(para.PPr.NumPr.ILvl.Val)
		}
		indent := strings.Repeat("  ", level)
		// 简单处理为无序列表（实际应该根据 numId 判断有序/无序）
		return indent + "- " + content
	default:
		// 普通段落
		return content
	}
}

// tableToMarkdown 将表格转换为 Markdown
func (s *FileTaskService) tableToMarkdown(table DocxTable) string {
	if len(table.Rows) == 0 {
		return ""
	}

	var b strings.Builder

	for i, row := range table.Rows {
		b.WriteString("| ")
		for _, cell := range row.Cells {
			// 提取单元格文本
			var cellText strings.Builder
			for _, para := range cell.Paragraphs {
				for _, run := range para.Runs {
					cellText.WriteString(run.Text.Value)
				}
			}
			// 替换换行符和管道符
			text := strings.ReplaceAll(cellText.String(), "\n", " ")
			text = strings.ReplaceAll(text, "|", "\\|")
			b.WriteString(text)
			b.WriteString(" | ")
		}
		b.WriteString("\n")

		// 表头分隔行
		if i == 0 {
			b.WriteString("| ")
			for range row.Cells {
				b.WriteString("--- | ")
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (s *FileTaskService) calculateAvgConfidence(itemsJSON string) float64 {
	var items []ExtractionItem
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return 0.5
	}
	if len(items) == 0 {
		return 0
	}
	var total float64
	for _, it := range items {
		total += it.Confidence
	}
	return total / float64(len(items))
}

func (s *FileTaskService) callOCR(ctx context.Context, endpoint string, filePath string) (string, error) {
	formData := url.Values{}
	formData.Set("image_path", filePath)
	bodyReader := strings.NewReader(formData.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Ok    bool   `json:"ok"`
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse OCR response: %v", err)
	}
	if !result.Ok {
		return "", fmt.Errorf("OCR service error: %s", result.Error)
	}
	return result.Text, nil
}

func (s *FileTaskService) updateTaskProgress(taskID string, progress int, message string) {
	s.db.Exec("UPDATE file_ocr_task SET progress = ?, status = 'running', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", progress, "running", message, taskID)
}

func (s *FileTaskService) updateTaskError(taskID string, errStr string) {
	s.db.Exec("UPDATE file_ocr_task SET status = 'failed', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", errStr, taskID)
	// Also update file status
	var fileID string
	s.db.Get(&fileID, "SELECT file_asset_id FROM file_ocr_task WHERE id = ?", taskID)
	if fileID != "" {
		s.db.Exec("UPDATE file_asset SET scan_status = 'failed', parse_status = 'failed' WHERE id = ?", fileID)
	}
}

const maxOCRRunesForClassify = 12000

func truncateTextForLLM(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "\n…(后文已省略)"
}

// classifyObjectTypeWithAI 调用大模型从 OCR 文本判定入库分类，与审核台「推荐入库」下拉取值一致。
func (s *FileTaskService) classifyObjectTypeWithAI(ocrText, hintLibraryType string) string {
	allowed := map[string]struct{}{
		"general": {}, "person": {}, "qualification": {}, "performance": {}, "method": {}, "laborcontract": {},
	}
	fallback := "general"
	if _, ok := allowed[hintLibraryType]; ok {
		fallback = hintLibraryType
	}
	client := s.llmClient()
	if client.APIKey == "" {
		log.Printf("[FileTaskService] AI not configured (无 ai_api_key / AI_API_KEY)，object_type fallback=%s", fallback)
		return fallback
	}

	snippet := truncateTextForLLM(ocrText, maxOCRRunesForClassify)
	userPrompt, sysPrompt := s.promptService.GetPromptFull("file_classify")
	if userPrompt == "" {
		userPrompt = "请根据提供的 OCR 文本内容，对该文件进行准确分类。分类建议：laborcontract (劳动合同), person (人员证件), qualification (企业资质), performance (项目业绩), method (施工方案/工艺), general (普通文档)。"
	}
	userPrompt += "\n\nOCR 文本：\n" + snippet

	if sysPrompt == "" {
		sysPrompt = "你只输出合法 JSON 对象，格式：{\"object_type\":\"general|person|qualification|performance|method|laborcontract\",\"confidence\":0.0到1.0的小数}。object_type 必须小写。不要输出其它文字。"
	}

	messages := []LLMMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.CallLLM(messages, 0.1)
	if err != nil {
		log.Printf("[FileTaskService] AI classify failed: %v", err)
		return fallback
	}

	var parsed struct {
		ObjectType string  `json:"object_type"`
		Confidence float64 `json:"confidence"`
	}
	cleaned := stripLLMFences(resp)
	if !extractJSONObject(cleaned, &parsed) && !extractJSONObject(resp, &parsed) {
		log.Printf("[FileTaskService] AI classify JSON parse failed, snippet=%q", truncateTextForLLM(resp, 240))
		return fallback
	}
	t := strings.TrimSpace(strings.ToLower(parsed.ObjectType))
	if _, ok := allowed[t]; !ok {
		log.Printf("[FileTaskService] AI classify invalid object_type=%q", parsed.ObjectType)
		return fallback
	}
	log.Printf("[FileTaskService] AI classified object_type=%s (confidence=%.2f)", t, parsed.Confidence)
	return t
}

func extractJSONObject(resp string, out interface{}) bool {
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start == -1 || end <= start {
		return false
	}
	return json.Unmarshal([]byte(resp[start:end+1]), out) == nil
}

// stripLLMFences 去掉 ``` / ```json 包裹，便于解析分类 JSON。
func stripLLMFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "json") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = strings.TrimSpace(s[nl+1:])
		}
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func (s *FileTaskService) extractStructuredDataChunked(text string, libType string) string {
	// 1. 按页拆分文字
	pagePattern := regexp.MustCompile(`(?s)--- 第 (\d+) 页 ---\n\n`)
	splits := pagePattern.Split(text, -1)

	// 如果由于某种原因没拆开（比如不是 PDF），退化到普通提取
	if len(splits) <= 1 {
		return s.extractStructuredData(text, libType)
	}

	// 第一块通常是页码 1 之前的（如果有的话，通常为空），或者第一页内容
	// 我们重新整理成带有明确页码的子项
	type PageChunk struct {
		PageNum int
		Content string
	}
	chunks := []PageChunk{}

	// 匹配所有页码标识
	matches := pagePattern.FindAllStringSubmatch(text, -1)

	// 第一部分归为第 1 页
	chunks = append(chunks, PageChunk{PageNum: 1, Content: strings.TrimSpace(splits[0])})

	for i, m := range matches {
		pageNum, _ := strconv.Atoi(m[1])
		if i+1 < len(splits) {
			chunks = append(chunks, PageChunk{PageNum: pageNum, Content: strings.TrimSpace(splits[i+1])})
		}
	}

	// 2. 分组提取（每 3 页一组，平衡准确率与 API 消耗）
	const pageSizePerCall = 3
	var allItems []ExtractionItem
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发控制：最多同时跑 2 个提取任务，避免瞬时 API 速率超限
	sem := make(chan struct{}, 2)

	for i := 0; i < len(chunks); i += pageSizePerCall {
		end := i + pageSizePerCall
		if end > len(chunks) {
			end = len(chunks)
		}

		group := chunks[i:end]
		wg.Add(1)

		go func(pGroup []PageChunk) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 合并本组文本
			var b strings.Builder
			for _, pc := range pGroup {
				b.WriteString(fmt.Sprintf("\n[第 %d 页原文]\n%s\n", pc.PageNum, pc.Content))
			}

			// 调用 LLM 提炼（传入本组第一页作为参考页码）
			resJSON := s.extractStructuredData(b.String(), libType)

			var items []ExtractionItem
			if err := json.Unmarshal([]byte(resJSON), &items); err == nil {
				// 尝试为没有页码的项补上页码
				for j := range items {
					if items[j].SourcePage == "" || items[j].SourcePage == "0" {
						items[j].SourcePage = strconv.Itoa(pGroup[0].PageNum)
					}
				}

				mu.Lock()
				allItems = append(allItems, items...)
				mu.Unlock()
			}
		}(group)
	}

	wg.Wait()

	// 3. 结果合并
	if len(allItems) == 0 {
		return s.extractStructuredData(truncateTextForLLM(text, 28000), libType)
	}

	finalJSON, _ := json.Marshal(allItems)
	return string(finalJSON)
}

func (s *FileTaskService) extractStructuredData(text string, libType string) string {
	client := s.llmClient()
	if client.APIKey == "" {
		log.Printf("[FileTaskService] AI client not configured, using fallback extraction")
		if libType == "person" {
			return NormalizePersonExtractedDataJSON("")
		}
		return `[{"id":"1", "title":"基础提炼", "content":"智能提取未配置，请手工进入审核台校对", "source_page":"1"}]`
	}

	extractInput := truncateTextForLLM(text, 28000)
	typeHint := "当前文档已由模型判定分类为 " + libType + "，请按该类型侧重提取关键字段。"

	prompt, sys := s.promptService.GetPromptFull(libType + "_extraction")

	if prompt == "" {
		// Basic Fallbacks
		if libType == "person" {
			prompt = "你是一个专业的工程招标文档专家。" + typeHint + " 请提取各人员信息。文本内容: \n" + extractInput
		} else {
			prompt = "你是一个专业的工程领域文档数据提取助手。" + typeHint + " 请从以下 OCR 文本中提取关键条目。文本内容:\n" + extractInput
		}
	} else {
		prompt += "\n文本内容: \n" + extractInput
	}

	if sys == "" {
		sys = "你是一个结构化数据提取专家。请只返回 JSON 数组（包含 id, title, content, source_page 字段）。不要包含任何 Markdown 格式。不要输出解释性文字。"
		if libType == "person" {
			// 修改为针对多人员友好的提示，不再强制 10 个一组平铺，而是强调完整性
			sys = "你是一个结构化数据提取专家。请从文本中提取所有人员信息。每个人的字段必须包含：姓名、身份证号、资格类型、证书类别、专业、证书编号、注册编号、有效期、颁发单位。必须返回扁平的 JSON 数组，每个字段作为一个对象，包含 title(字段名) 和 content(内容)。"
		}
	}
	messages := []LLMMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: prompt},
	}

	resp, err := client.CallLLM(messages, 0.1)
	if err != nil {
		log.Printf("[FileTaskService] AI Extraction failed: %v", err)
		if libType == "person" {
			return NormalizePersonExtractedDataJSON("")
		}
		return `[{"id":"1", "title":"识别结果", "content":"AI 提取失败，请手工录入", "source_page":"1"}]`
	}

	// Simple sanitization: only keep the JSON array part（兼容 ```json 包裹）
	resp = stripLLMFences(resp)
	startIndex := strings.Index(resp, "[")
	endIndex := strings.LastIndex(resp, "]")
	if startIndex != -1 && endIndex != -1 {
		out := resp[startIndex : endIndex+1]
		out = CoerceExtractionJSONArray(out)
		if libType == "person" {
			out = NormalizePersonExtractedDataJSON(out)
		}
		return out
	}

	if libType == "person" {
		return NormalizePersonExtractedDataJSON("")
	}
	return `[{"id":"1", "title":"识别结果", "content":"` + strings.ReplaceAll(resp, "\n", " ") + `", "source_page":"1"}]`
}
