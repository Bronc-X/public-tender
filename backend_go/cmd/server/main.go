package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"backend_go/internal/db"
	"backend_go/internal/handler"
	"backend_go/internal/middleware"
	"backend_go/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	dbPath := resolveDBPath()
	log.Printf("Using database at: %s", dbPath)

	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	db.ApplySchemaPatches(database)
	if _, err := database.Exec(`UPDATE tech_bid_projects
		SET current_step_status = 'failed',
			step4_status = 'failed',
			last_error_message = '服务重启导致任务中断，请重新选择路线',
			updated_at = ?
		WHERE current_step = 'outline_generation'
		  AND current_step_status IN ('running', 'waiting_for_approval')
		  AND updated_at < ?`, time.Now(), time.Now().Add(-10*time.Minute)); err != nil {
		log.Printf("Failed to recover stale tech-bid jobs: %v", err)
	}

	if _, err := database.Exec(`UPDATE bid_projects
		SET current_step_status = 'failed',
			last_error_message = '服务重启导致任务中断，请重新开始任务',
			updated_at = ?
		WHERE current_step_status = 'running'
		  AND updated_at < ?`, time.Now(), time.Now().Add(-60*time.Minute)); err != nil {
		log.Printf("Failed to recover stale bid-projects jobs: %v", err)
	}

	log.Printf("Successfully connected to database at: %s", dbPath)
	defer database.Close()

	r := gin.Default()

	r.Use(middleware.CORS())
	r.Use(middleware.CompanyID())

	// Services
	settingsService := service.NewSettingsService(database)
	promptService := service.NewPromptService(database)
	cacheMetricsService := service.NewCacheMetricsService(database)
	aiKey := settingsService.GetAIKey()
	endpoint := settingsService.GetSetting("ai_ingest_endpoint")
	modelName := settingsService.GetSetting("ai_ingest_model")

	aiClient := service.NewAIClient(aiKey, endpoint, modelName)
	elasticEngine := service.NewElasticOutlineEngine(database, aiClient, promptService)
	digitizeService := service.NewTenderDigitizationService(aiClient, promptService, cacheMetricsService, database, elasticEngine)
	service.InitDefaultSkeletons(database)
	fileService := service.NewFileService(database)
	fileArchiveService := service.NewFileArchiveService(database)
	docMindService := service.NewDocMindParseService(settingsService)
	fileTaskService := service.NewFileTaskService(database, settingsService, fileArchiveService, promptService, docMindService)
	fileReviewService := service.NewFileReviewService(database)
	step6ExporterService := service.NewStep6ExporterService(database)

	// Handlers
	companyHandler := handler.NewCompanyHandler(database)
	personHandler := handler.NewPersonHandler(database)
	qualificationHandler := handler.NewQualificationHandler(database)
	performanceHandler := handler.NewPerformanceHandler(database)
	honorHandler := handler.NewHonorHandler(database)
	fileHandler := handler.NewFileHandler(database, fileService, fileTaskService, docMindService)
	sharedTenderHandler := handler.NewSharedTenderHandler(database)
	bidProjectHandler := handler.NewBidProjectHandler(database, digitizeService, fileTaskService, step6ExporterService, docMindService)
	techBidProjectHandler := handler.NewTechBidProjectHandler(database, digitizeService)
	riskReviewHandler := handler.NewRiskReviewHandler(database)
	chapterHandler := handler.NewChapterHandler(database, digitizeService)
	exportHandler := handler.NewExportHandler(database)
	skeletonGenerator := service.NewIndustrySkeletonGenerator(database, aiClient, promptService)
	settingsHandler := handler.NewSettingsHandler(database, skeletonGenerator)
	auditHandler := handler.NewAuditHandler(database, fileReviewService, fileArchiveService)
	importHandler := handler.NewImportHandler(database, fileTaskService)
	dashboardHandler := handler.NewDashboardHandler(database)
	cacheStatsHandler := handler.NewCacheStatsHandler(database, cacheMetricsService)
	knowledgeHandler := handler.NewKnowledgeHandler(database)
	knowledgeExtractService := service.NewKnowledgeExtractService(database, aiClient, promptService)
	knowledgeExtractHandler := handler.NewKnowledgeExtractHandler(database, knowledgeExtractService)
	promptHandler := handler.NewPromptHandler(database, promptService)
	issueHandler := handler.NewIssueHandler(database)
	authHandler := handler.NewAuthHandler()
	storageHandler := handler.NewStorageHandler()
	financialReportHandler := handler.NewFinancialReportHandler(database)
	otherLibraryHandler := handler.NewOtherLibraryHandler(database)

	// Public Routes
	r.GET("/health", handler.HealthCheck)
	r.GET("/entry", authHandler.Entry)

	api := r.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		// Health check (duplicated but protected)
		api.GET("/health", handler.HealthCheck)

		// Settings
		api.GET("/settings", settingsHandler.GetSettings)
		api.POST("/settings", settingsHandler.UpdateSettingsBatch)
		api.GET("/imports/settings", settingsHandler.GetSettings)          // Legacy compatibility
		api.POST("/imports/settings", settingsHandler.UpdateSettingsBatch) // Legacy compatibility
		api.GET("/settings/ocr", settingsHandler.GetOCRSettings)
		api.PUT("/settings/ocr", settingsHandler.UpdateOCRSettings)
		api.POST("/settings/ocr/test", settingsHandler.TestOCRConnection)
		api.POST("/imports/settings/test-ai", settingsHandler.TestAIConnection)
		api.POST("/imports/settings/test-doubao", settingsHandler.TestDoubaoConnection)

		// Industry Skeleton routes
		api.GET("/settings/industry-skeletons", settingsHandler.ListIndustrySkeletons)
		api.POST("/settings/industry-skeletons", settingsHandler.UpdateIndustrySkeleton)
		api.DELETE("/settings/industry-skeletons/:id", settingsHandler.DeleteIndustrySkeleton)
		api.GET("/imports/ocr/provider", settingsHandler.GetOCRProviderInfo)
		api.POST("/imports/ocr/health-check", settingsHandler.HealthCheckOCR)

		// Dashboard
		api.GET("/dashboard/summary", dashboardHandler.GetSummary)

		// Cache Stats
		api.GET("/cache/stats", cacheStatsHandler.GetCacheStats)

		// Company routes
		api.GET("/companies", companyHandler.ListCompanies)
		api.POST("/companies", companyHandler.CreateCompany)
		api.GET("/companies/:id", companyHandler.GetCompany)
		api.PATCH("/companies/:id", companyHandler.UpdateCompany)
		api.DELETE("/companies/:id", companyHandler.DeleteCompany)

		// Person routes
		api.GET("/persons", personHandler.ListPersons)
		api.POST("/persons", personHandler.CreatePerson)
		api.GET("/persons/:id", personHandler.GetPerson)
		api.PATCH("/persons/:id", personHandler.UpdatePerson)
		api.DELETE("/persons/:id", personHandler.DeletePerson)
		api.POST("/persons/:id/attachments", personHandler.CreatePersonProof)
		api.DELETE("/persons/attachments/:proofID", personHandler.DeletePersonProof)

		api.POST("/persons/:id/educations", personHandler.CreateEducation)
		api.PATCH("/persons/educations/:eduID", personHandler.UpdateEducation)
		api.DELETE("/persons/educations/:eduID", personHandler.DeleteEducation)

		api.POST("/persons/:id/works", personHandler.CreateWorkExperience)
		api.PATCH("/persons/works/:workID", personHandler.UpdateWorkExperience)
		api.DELETE("/persons/works/:workID", personHandler.DeleteWorkExperience)

		// Qualification routes
		api.GET("/qualifications", qualificationHandler.ListQualifications)
		api.POST("/qualifications", qualificationHandler.CreateQualification)
		api.GET("/qualifications/:id", qualificationHandler.GetQualification)
		api.PATCH("/qualifications/:id", qualificationHandler.UpdateQualification)
		api.DELETE("/qualifications/:id", qualificationHandler.DeleteQualification)

		// Performance routes (with resilience mock if Java is down)
		api.GET("/performances", func(c *gin.Context) {
			// Try proxy to Java first, but if Java fails/timeout, return a Mock
			// For simplicity in this fix, we'll try to detect Java health
			httpClient := &http.Client{Timeout: 500 * time.Millisecond}
			javaHealth := "http://localhost:8889/health"
			_, err := httpClient.Get(javaHealth)

			if err != nil {
				log.Println("[Proxy] Java backend unavailable. Serving data from local Go database.")
				performanceHandler.ListPerformances(c)
				return
			}

			// If Java is up, use the regular handler
			performanceHandler.ListPerformances(c)
		})
		api.POST("/performances", performanceHandler.CreatePerformance)
		api.POST("/performances/match", performanceHandler.MatchPersonnel)
		api.GET("/performances/:id", performanceHandler.GetPerformance)
		api.PATCH("/performances/:id", performanceHandler.UpdatePerformance)
		api.DELETE("/performances/:id", performanceHandler.DeletePerformance)
		api.POST("/performances/:id/proofs", performanceHandler.AddPerformanceProof)
		api.DELETE("/performances/proofs/:proofID", performanceHandler.DeletePerformanceProof)

		// Honor routes
		api.GET("/honors", honorHandler.ListHonors)
		api.POST("/honors", honorHandler.CreateHonor)
		api.GET("/honors/:id", honorHandler.GetHonor)
		api.PATCH("/honors/:id", honorHandler.UpdateHonor)
		api.DELETE("/honors/:id", honorHandler.DeleteHonor)

		// Financial Report routes
		financialReportsGroup := api.Group("/financial-reports")
		{
			financialReportsGroup.GET("/folders", financialReportHandler.ListFolders)
			financialReportsGroup.POST("/folders", financialReportHandler.CreateFolder)
			financialReportsGroup.PATCH("/folders/:id", financialReportHandler.RenameFolder)
			financialReportsGroup.DELETE("/folders/:id", financialReportHandler.DeleteFolder)
			financialReportsGroup.GET("/folders/:id/files", financialReportHandler.ListFilesByFolder)
		}

		// Other Library routes
		otherLibraryGroup := api.Group("/others")
		{
			otherLibraryGroup.GET("/folders", otherLibraryHandler.ListFolders)
			otherLibraryGroup.POST("/folders", otherLibraryHandler.CreateFolder)
			otherLibraryGroup.PATCH("/folders/:id", otherLibraryHandler.RenameFolder)
			otherLibraryGroup.DELETE("/folders/:id", otherLibraryHandler.DeleteFolder)
			otherLibraryGroup.GET("/folders/:id/files", otherLibraryHandler.ListFilesByFolder)
		}

		// File Asset routes
		api.GET("/files", fileHandler.ListFiles)
		api.POST("/files/upload", fileHandler.UploadFile)
		api.GET("/files/download/:id/*filename", fileHandler.DownloadFile)
		// 技术标书库详情页专用路径（与 /files/:id/* 分离，避免未匹配时被 NoRoute 代理到 Node 返回 502）
		api.GET("/file-binary/:id", fileHandler.ServeFileBinary)
		api.GET("/file-parsed/:id", fileHandler.ServeFileParsed)
		api.POST("/file-normalize/:id", fileHandler.NormalizeDocMindStoredMarkdown)
		api.GET("/files/:id/parsed", fileHandler.GetFileParsed)
		api.GET("/files/:id/binary", fileHandler.DownloadFileBinary)
		api.POST("/files/:id/aliyun-doc-parse", fileHandler.AliyunDocParse)
		api.POST("/files/:id/normalize-docmind-markdown", fileHandler.NormalizeDocMindStoredMarkdown)
		api.GET("/files/:id", fileHandler.GetFileAsset)
		api.PATCH("/files/:id", fileHandler.RenameFile)
		api.DELETE("/files/:id", fileHandler.DeleteFile)
		api.GET("/files/:id/pages/:num", fileHandler.ServeFilePage)
		api.POST("/files/:id/route", fileHandler.RouteFile)

		// Shared Tender routes
		api.GET("/shared-tenders/candidates", sharedTenderHandler.ListCandidates)
		api.POST("/shared-tenders/resolve", sharedTenderHandler.Resolve)
		api.GET("/shared-tenders/:id", sharedTenderHandler.GetSharedTender)
		api.POST("/shared-tenders/:id/bind", sharedTenderHandler.BindProject)

		// Bid Project routes
		api.GET("/bid-projects", bidProjectHandler.ListProjects)
		api.POST("/bid-projects", bidProjectHandler.CreateProject)
		api.GET("/bid-projects/:id", bidProjectHandler.GetProject)
		api.PATCH("/bid-projects/:id", bidProjectHandler.UpdateProject)
		api.DELETE("/bid-projects/:id", bidProjectHandler.DeleteProject)
		api.GET("/bid-projects/:id/actions", bidProjectHandler.GetActions)
		api.POST("/bid-projects/:id/start-workflow", bidProjectHandler.StartWorkflow)
		api.POST("/bid-projects/:id/files", bidProjectHandler.AddProjectFile)
		api.POST("/bid-projects/:id/run", bidProjectHandler.RunWorkflow)
		api.POST("/bid-projects/:id/resource-combination", bidProjectHandler.SaveResourceCombination)
		api.POST("/bid-projects/:id/step5-bindings", bidProjectHandler.SaveStep5Bindings)
		api.POST("/bid-projects/:id/confirm", bidProjectHandler.ConfirmStep)
		api.POST("/bid-projects/:id/goback", bidProjectHandler.GoBackStep)
		api.PUT("/bid-projects/:id/rules", bidProjectHandler.UpdateRules)
		api.PUT("/bid-projects/:id/company-adaptation", bidProjectHandler.UpdateCompanyAdaptation)
		api.GET("/bid-projects/:id/outputs", exportHandler.GetProjectOutputs)

		// Bid Project Exporter (Step 6)
		api.GET("/bid-projects/:id/step6/payload", bidProjectHandler.GetStep6Payload)
		api.POST("/bid-projects/:id/step6/generate", bidProjectHandler.GenerateStep6Payload)
		api.POST("/bid-projects/:id/step6/upload-template", bidProjectHandler.UploadStep6Template)
		api.POST("/bid-projects/:id/step6/slots/:slot_id/regenerate", bidProjectHandler.RegenerateSlot)

		api.POST("/bid-projects/:id/step6/export", bidProjectHandler.ExportFinalWord)
		api.GET("/bid-projects/:id/step6/download", bidProjectHandler.DownloadStep6Word)
		api.HEAD("/bid-projects/:id/step6/download", bidProjectHandler.DownloadStep6Word)

		// Tech Bid Project routes
		api.GET("/tech-bid/projects", techBidProjectHandler.ListProjects)
		api.POST("/tech-bid/projects", techBidProjectHandler.CreateProject)
		api.GET("/tech-bid/projects/:id", techBidProjectHandler.GetProject)
		api.PATCH("/tech-bid/projects/:id", techBidProjectHandler.UpdateProject)
		api.DELETE("/tech-bid/projects/:id", techBidProjectHandler.DeleteProject)
		api.POST("/tech-bid/projects/:id/files", techBidProjectHandler.AddProjectFile)
		api.GET("/tech-bid/projects/:id/outputs", exportHandler.GetProjectOutputs)

		// Tech Knowledge routes
		api.GET("/tech-bid/knowledge", knowledgeHandler.ListKnowledge)
		api.POST("/tech-bid/knowledge", knowledgeHandler.CreateKnowledge)
		api.GET("/tech-bid/knowledge/:id", knowledgeHandler.GetKnowledgeItem)
		api.PATCH("/tech-bid/knowledge/:id", knowledgeHandler.UpdateKnowledge)
		api.DELETE("/tech-bid/knowledge/:id", knowledgeHandler.DeleteKnowledge)

		// Knowledge extract from history projects
		api.GET("/knowledge-extract/history-projects", knowledgeExtractHandler.ListHistoryProjects)
		api.GET("/knowledge-extract/projects/:origin/:id/files", knowledgeExtractHandler.ListProjectFiles)
		api.POST("/knowledge-extract/resolve-local-files", knowledgeExtractHandler.ResolveLocalFiles)
		api.GET("/knowledge-extract/prompt-templates", knowledgeExtractHandler.ListPromptTemplates)
		api.POST("/knowledge-extract/tasks", knowledgeExtractHandler.CreateTask)
		api.GET("/knowledge-extract/tasks/:id", knowledgeExtractHandler.GetTask)
		api.POST("/knowledge-extract/tasks/:id/commit", knowledgeExtractHandler.CommitTask)
		api.POST("/knowledge-extract/tasks/:id/cancel", knowledgeExtractHandler.CancelTask)

		// Risk Review routes (Tech Bid)
		api.GET("/tech-bid/risks", riskReviewHandler.ListTechRiskRecords)
		api.GET("/tech-bid/risk/projects/:id", riskReviewHandler.ListTechRiskRecordsByProject)
		api.POST("/tech-bid/risk/projects/:id/run", riskReviewHandler.RunTechRiskReview)
		api.PATCH("/tech-bid/risks/:id/status", riskReviewHandler.UpdateRiskStatus)

		// Chapter Plan routes (Tech Bid)
		api.GET("/tech-bid/chapters", chapterHandler.ListChapterPlans)
		api.GET("/tech-bid/chapters/project/:id", chapterHandler.ListChaptersByProject)
		api.PATCH("/tech-bid/chapters/:id", chapterHandler.UpdateChapterPlan)
		api.POST("/tech-bid/chapters/:id/generate", chapterHandler.GenerateChapterContent)
		api.PUT("/tech-bid/chapters/:id/content", chapterHandler.UpdateChapterContent)

		// Project Control routes
		api.POST("/tech-bid/projects/:id/run", techBidProjectHandler.RunStep)
		api.POST("/tech-bid/projects/:id/confirm", techBidProjectHandler.ConfirmStep)
		api.POST("/tech-bid/projects/:id/goback", techBidProjectHandler.GoBackStep)
		api.POST("/tech-bid/projects/:id/routes/select", techBidProjectHandler.SelectRoute)
		api.POST("/tech-bid/projects/:id/outline/run", techBidProjectHandler.PostOutlineRun)
		api.POST("/tech-bid/projects/:id/outline/regenerate", techBidProjectHandler.PostOutlineRegenerate)
		api.POST("/tech-bid/projects/:id/outline/chapters/generate", techBidProjectHandler.PostOutlineChaptersGenerate)
		api.POST("/tech-bid/projects/:id/outline/chapters/confirm", techBidProjectHandler.PostOutlineChaptersConfirm)
		api.POST("/tech-bid/projects/:id/outline/expand", techBidProjectHandler.PostOutlineExpand)
		api.GET("/tech-bid/projects/:id/outline/run-status", techBidProjectHandler.GetOutlineRunStatus)
		api.GET("/tech-bid/projects/:id/outline/run-history", techBidProjectHandler.GetOutlineRunHistory)
		api.GET("/tech-bid/projects/:id/outline/agent-runs", techBidProjectHandler.GetOutlineAgentRuns)
		api.GET("/tech-bid/projects/:id/outline/approval-logs", techBidProjectHandler.GetOutlineApprovalLogs)
		api.GET("/tech-bid/projects/:id/outline/versions", techBidProjectHandler.GetOutlineVersions)
		api.GET("/tech-bid/projects/:id/outline/versions/:versionId", techBidProjectHandler.GetOutlineVersionDetail)
		api.POST("/tech-bid/projects/:id/outline/versions/:versionId/select", techBidProjectHandler.SelectOutlineVersion)
		api.POST("/tech-bid/projects/:id/outline/verify", techBidProjectHandler.RunVerification)
		api.GET("/tech-bid/projects/:id/structure-plan", techBidProjectHandler.GetStructurePlan)
		api.POST("/tech-bid/projects/:id/structure-plan/approve", techBidProjectHandler.ApproveStructurePlan)
		api.POST("/tech-bid/projects/:id/structure-plan/reject", techBidProjectHandler.RejectStructurePlan)
		api.GET("/tech-bid/projects/:id/outline/mappings", techBidProjectHandler.GetOutlineFactMappings)
		api.GET("/tech-bid/projects/:id/outline/fact-candidates", techBidProjectHandler.GetOutlineFactCandidates)
		api.GET("/tech-bid/projects/:id/outline/coverage", techBidProjectHandler.GetOutlineCoverageLatest)
		api.GET("/tech-bid/projects/:id/outline/requirements", techBidProjectHandler.GetRequirementRegister)
		api.GET("/tech-bid/projects/:id/outline/full-response", techBidProjectHandler.GetFullRequirementResponseLatest)
		api.GET("/tech-bid/projects/:id/outline/conflict-audit", techBidProjectHandler.GetOutlineConflictAuditLatest)
		api.POST("/tech-bid/projects/:id/outline/step4-gate/override", techBidProjectHandler.ManualOverrideStep4Gate)
		api.POST("/tech-bid/projects/:id/outline/optimize", techBidProjectHandler.OptimizeOutline)
		api.POST("/tech-bid/projects/:id/outline/force-unlock", techBidProjectHandler.ManualOverrideVerification)
		api.POST("/tech-bid/projects/:id/step5/generate", techBidProjectHandler.GenerateTechStep5Content)
		api.GET("/tech-bid/projects/:id/step6/payload", techBidProjectHandler.GetTechStep6Payload)
		api.POST("/tech-bid/projects/:id/step6/generate", techBidProjectHandler.GenerateTechStep6Payload)
		api.POST("/tech-bid/projects/:id/step6/export", techBidProjectHandler.ExportTechFinalWord)
		api.GET("/tech-bid/projects/:id/step6/download", techBidProjectHandler.DownloadTechStep6Word)
		api.HEAD("/tech-bid/projects/:id/step6/download", techBidProjectHandler.DownloadTechStep6Word)

		// Profile manual editing routes
		api.PATCH("/tech-bid/projects/:id/profile/fields", techBidProjectHandler.PatchProfileField)
		api.POST("/tech-bid/projects/:id/profile/confirm", techBidProjectHandler.ConfirmProfile)
		api.GET("/tech-bid/projects/:id/profile/edit-history", techBidProjectHandler.GetProfileEditHistory)
		api.GET("/tech-bid/projects/:id/profile/extraction-snapshots", techBidProjectHandler.GetProfileExtractionSnapshots)

		// Skeleton candidate selection routes (Human-in-the-loop)
		api.GET("/tech-bid/projects/:id/skeleton/candidates", techBidProjectHandler.GetSkeletonCandidates)
		api.POST("/tech-bid/projects/:id/skeleton/confirm", techBidProjectHandler.ConfirmSkeleton)
		api.GET("/tech-bid/projects/:id/skeleton/confirmed", techBidProjectHandler.GetConfirmedSkeleton)
		api.GET("/tech-bid/projects/:id/skeleton/candidates-from-facts", techBidProjectHandler.GetSkeletonCandidatesFromFacts)

		// Export routes
		api.GET("/exports/projects/:id", exportHandler.GetProjectOutputs)
		api.POST("/exports/companies/excel", exportHandler.ExportCompanyExcel)

		// Audit Center routes
		api.GET("/audits", auditHandler.ListAudits)
		api.GET("/audits/:id", auditHandler.GetAuditDetail)
		api.POST("/audits/:id/confirm", auditHandler.ConfirmAudit)
		api.POST("/audits/:id/ignore", auditHandler.IgnoreAudit)

		// Issue Center routes
		api.GET("/issues", issueHandler.ListIssues)
		api.PATCH("/issues/:id", issueHandler.ResolveIssue)

		// Import center routes
		api.POST("/tech-bid/import/tasks", importHandler.CreateImportTask)
		api.GET("/tech-bid/import/tasks/:id", importHandler.GetTaskStatus)
		api.POST("/imports/analyze", importHandler.AnalyzeExcel)
		api.POST("/imports/execute", importHandler.ExecuteImport)

		// Storage (local disk paths)
		api.GET("/storage/info", storageHandler.GetStorageInfo)
		api.POST("/storage/open", storageHandler.OpenIncomingFolder)
		api.POST("/storage/open-db", storageHandler.OpenDatabaseFolder)

		// New: Digitize Route for Demo
		api.POST("/project/digitize/:id", func(c *gin.Context) {
			id := c.Param("id")
			log.Printf("[Digitize] Received request for project ID: %s", id)

			// Demo of using the service
			go digitizeService.DigitizeTenderFile(c.Request.Context(), id, "Demo content", "tech", nil)

			c.JSON(200, gin.H{"id": id, "status": "digitization_queued_go"})
		})

		// Prompt Configuration Center
		api.GET("/prompt/category/list", promptHandler.GetCategories)
		api.POST("/prompt/category/save", promptHandler.SaveCategory)
		api.DELETE("/prompt/category/:id", promptHandler.DeleteCategory)
		api.GET("/prompt/list", promptHandler.GetPrompts)
		api.GET("/prompt/detail/:id", promptHandler.GetPromptDetail)
		api.POST("/prompt/save", promptHandler.SavePrompt)
		api.GET("/prompt/versions/:id", promptHandler.GetVersionHistory)
		api.POST("/prompt/rollback", promptHandler.Rollback)
		api.GET("/prompt/get/:key", func(c *gin.Context) {
			key := c.Param("key")
			content := promptService.GetPrompt(key)
			if content == "" {
				handler.Error(c, http.StatusNotFound, "Prompt not found")
				return
			}
			handler.Success(c, gin.H{"key": key, "content": content})
		})
	}

	// NO-ROUTE PROXY (Fallback to Node.js)
	originalNodeHost := os.Getenv("NODE_BACKEND")
	if originalNodeHost == "" {
		originalNodeHost = "http://localhost:8889"
	}
	r.NoRoute(middleware.NoRouteProxy(originalNodeHost))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Go Backend Server starting on :%s (Proxying unhandled routes to %s)", port, originalNodeHost)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %s", err)
	}
}

func resolveDBPath() string {
	if dbPath := os.Getenv("APP_DB_PATH"); dbPath != "" {
		return dbPath
	}
	return "data/app.db"
}
