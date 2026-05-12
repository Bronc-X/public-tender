package model

import (
	"time"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Company struct {
	ID                      string    `db:"id" json:"id"`
	CompanyName             string    `db:"company_name" json:"company_name"`
	UnifiedSocialCreditCode *string   `db:"unified_social_credit_code" json:"unified_social_credit_code"`
	LegalPerson             *string   `db:"legal_person" json:"legal_person"`
	LegalPersonIdCard       *string   `db:"legal_person_id_card" json:"legal_person_id_card"`
	Address                 *string   `db:"address" json:"address"`
	CreatedAt               time.Time `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time `db:"updated_at" json:"updated_at"`
}

type Person struct {
	ID                   string          `db:"id" json:"id"`
	Name                 string          `db:"name" json:"name"`
	Gender               *string         `db:"gender" json:"gender"`
	JoinDate             *string         `db:"join_date" json:"join_date"`
	IdNumberMasked       *string         `db:"id_number_masked" json:"id_number_masked"`
	IdCardNo             *string         `db:"id_card_no" json:"id_card_no"`
	RegistrationNo       *string         `db:"registration_no" json:"registration_no"`
	RegDate              *string         `db:"reg_date" json:"reg_date"`
	ExpiryDate           *string         `db:"expiry_date" json:"expiry_date"`
	IssuingAuthority     *string         `db:"issuing_authority" json:"issuing_authority"`
	CompanyName          *string         `db:"company_name" json:"company_name"`
	SocialSecurityStatus *string         `db:"social_security_status" json:"social_security_status"`
	OnJobStatus          *string         `db:"on_job_status" json:"on_job_status"`
	BidUsableStatus      *string         `db:"bid_usable_status" json:"bid_usable_status"`
	RiskStatus           *string         `db:"risk_status" json:"risk_status"`
	CompanyID            *string         `db:"company_id" json:"company_id"`
	CreatedAt            *time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt            *time.Time      `db:"updated_at" json:"updated_at"`
	Certificates         []Qualification            `db:"-" json:"certificates"`
	Proofs               []PersonProof              `db:"-" json:"proofs"`
	Performances         []PersonRelatedPerformance `db:"-" json:"performances"`
	Educations           []PersonEducation          `db:"-" json:"educations"`
	WorkExperiences      []PersonWorkExperience     `db:"-" json:"work_experiences"`
}

type PersonEducation struct {
	ID        string     `db:"id" json:"id"`
	PersonID  string     `db:"person_id" json:"person_id"`
	StartDate *string    `db:"start_date" json:"start_date"`
	EndDate   *string    `db:"end_date" json:"end_date"`
	School    *string    `db:"school" json:"school"`
	Degree    *string    `db:"degree" json:"degree"`
	CreatedAt *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
}

type PersonWorkExperience struct {
	ID        string     `db:"id" json:"id"`
	PersonID  string     `db:"person_id" json:"person_id"`
	StartDate *string    `db:"start_date" json:"start_date"`
	EndDate   *string    `db:"end_date" json:"end_date"`
	Company   *string    `db:"company" json:"company"`
	Position  *string    `db:"position" json:"position"`
	CreatedAt *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
}

type PersonRelatedPerformance struct {
	ID                 string   `json:"id"`
	ProjectName        string   `json:"project_name"`
	RoleName           string   `json:"role_name"`
	ProjectManagerName *string  `json:"project_manager_name"`
	WinningDate        *string  `json:"winning_date"`
	CompletionDate     *string  `json:"completion_date"`
	AmountValue        *float64 `json:"amount_value"`
}

type Qualification struct {
	ID                 string     `db:"id" json:"id"`
	QualificationName  string     `db:"qualification_name" json:"qualification_name"`
	QualificationLevel *string    `db:"qualification_level" json:"qualification_level"`
	QualificationType  *string    `db:"qualification_type" json:"qualification_type"`
	Specialty          *string    `db:"specialty" json:"specialty"`
	OwnerType          *string    `db:"owner_type" json:"owner_type"`
	OwnerID            *string    `db:"owner_id" json:"owner_id"`
	CertificateNo      *string    `db:"certificate_no" json:"certificate_no"`
	RegistrationNo     *string    `db:"registration_no" json:"registration_no"`
	IssuingAuthority   *string    `db:"issuing_authority" json:"issuing_authority"`
	ValidFrom          *string    `db:"valid_from" json:"valid_from"`
	ValidTo            *string    `db:"valid_to" json:"valid_to"`
	BidUsableStatus    *string    `db:"bid_usable_status" json:"bid_usable_status"`
	RiskStatus         *string    `db:"risk_status" json:"risk_status"`
	VersionStatus      *string    `db:"version_status" json:"version_status"`
	FileAssetID        *string    `db:"file_asset_id" json:"file_asset_id"`
	StoredPath         *string    `db:"stored_path" json:"stored_path"` // Joined from file_asset
	Ext                *string    `db:"ext" json:"ext"`                 // Joined from file_asset
	CompanyID          *string    `db:"company_id" json:"company_id"`
	FileID             *string    `db:"file_id" json:"file_id"`
	CreatedAt          *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          *time.Time `db:"updated_at" json:"updated_at"`
}

type FinancialReportFolder struct {
	ID         string    `db:"id" json:"id"`
	CompanyID  string    `db:"company_id" json:"company_id"`
	FolderName string    `db:"folder_name" json:"folder_name"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type Performance struct {
	ID                       string             `db:"id" json:"id"`
	ProjectName              string             `db:"project_name" json:"project_name"`
	ProjectLocation          *string            `db:"project_location" json:"project_location"`
	OwnerOrg                 *string            `db:"owner_org" json:"owner_org"`
	ProjectManagerName       *string            `db:"project_manager_name" json:"project_manager_name"`
	PmID                     *string            `db:"pm_id" json:"pm_id"`
	TechnicalLeaderName      *string            `db:"technical_leader_name" json:"technical_leader_name"`
	TechLeaderID             *string            `db:"tech_leader_id" json:"tech_leader_id"`
	SafetyLeaderName         *string            `db:"safety_leader_name" json:"safety_leader_name"`
	SafetyLeaderID           *string            `db:"safety_leader_id" json:"safety_leader_id"`
	CompletionDate           *string            `db:"completion_date" json:"completion_date"`
	WinningDate              *string            `db:"winning_date" json:"winning_date"`
	AmountValue              *float64           `db:"amount_value" json:"amount_value"`
	BidAmountValue           *float64           `db:"bid_amount_value" json:"bid_amount_value"`
	ScaleDesc                *string            `db:"scale_desc" json:"scale_desc"`
	ScoreStatus              *string            `db:"score_status" json:"score_status"`
	BidUsableStatus          *string            `db:"bid_usable_status" json:"bid_usable_status"`
	ProofChainStatus         *string            `db:"proof_chain_status" json:"proof_chain_status"`
	RiskStatus               *string            `db:"risk_status" json:"risk_status"`
	SourceChannel            *string            `db:"source_channel" json:"source_channel"`
	SourceBatchID            *string            `db:"source_batch_id" json:"source_batch_id"`
	ConstructionPeriod       *string            `db:"construction_period" json:"construction_period"`
	DocumentationOfficerName *string            `db:"documentation_officer_name" json:"documentation_officer_name"`
	DocumentationOfficerID   *string            `db:"documentation_officer_id" json:"documentation_officer_id"`
	MaterialsOfficerName     *string            `db:"materials_officer_name" json:"materials_officer_name"`
	MaterialsOfficerID       *string            `db:"materials_officer_id" json:"materials_officer_id"`
	QualityInspectorName     *string            `db:"quality_inspector_name" json:"quality_inspector_name"`
	QualityInspectorID       *string            `db:"quality_inspector_id" json:"quality_inspector_id"`
	ConstructionOfficerName  *string            `db:"construction_officer_name" json:"construction_officer_name"`
	ConstructionOfficerID    *string            `db:"construction_officer_id" json:"construction_officer_id"`
	StandardsOfficerName     *string            `db:"standards_officer_name" json:"standards_officer_name"`
	StandardsOfficerID       *string            `db:"standards_officer_id" json:"standards_officer_id"`
	MechanicalOfficerName    *string            `db:"mechanical_officer_name" json:"mechanical_officer_name"`
	MechanicalOfficerID      *string            `db:"mechanical_officer_id" json:"mechanical_officer_id"`
	LaborOfficerName         *string            `db:"labor_officer_name" json:"labor_officer_name"`
	LaborOfficerID           *string            `db:"labor_officer_id" json:"labor_officer_id"`
	ConfirmedAt              *time.Time         `db:"confirmed_at" json:"confirmed_at"`
	CompanyID                *string            `db:"company_id" json:"company_id"`
	CreatedAt                *time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt                *time.Time         `db:"updated_at" json:"updated_at"`
	Proofs                   []PerformanceProof `db:"-" json:"proofs"`
}

type PerformanceProof struct {
	ID                   string    `db:"id" json:"id"`
	ProjectPerformanceID string    `db:"project_performance_id" json:"project_performance_id"`
	ProofType            *string   `db:"proof_type" json:"proof_type"`
	FileAssetID          *string   `db:"file_asset_id" json:"file_asset_id"`
	IsPrimary            int       `db:"is_primary" json:"is_primary"`
	ExtractedFieldsJSON  *string   `db:"extracted_fields_json" json:"extracted_fields_json"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
	// Joined fields
	FileName     *string `db:"file_name" json:"file_name"`
	Ext          *string `db:"ext" json:"ext"`
	MarkdownText *string `db:"markdown_text" json:"markdown_text"`
}

type PersonProof struct {
	ID          string    `db:"id" json:"id"`
	PersonID    string    `db:"person_id" json:"person_id"`
	ProofType   *string   `db:"proof_type" json:"proof_type"`
	FileAssetID *string   `db:"file_asset_id" json:"file_asset_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	// Joined fields
	FileName     *string `db:"file_name" json:"file_name"`
	Ext          *string `db:"ext" json:"ext"`
	MarkdownText *string `db:"markdown_text" json:"markdown_text"`
}

type HonorRecord struct {
	ID              string     `db:"id" json:"id"`
	HonorName       string     `db:"honor_name" json:"honor_name"`
	HonorLevel      *string    `db:"honor_level" json:"honor_level"`
	OwnerOrg        *string    `db:"owner_org" json:"owner_org"`
	OwnerPersonName *string    `db:"owner_person_name" json:"owner_person_name"`
	AwardDate       *string    `db:"award_date" json:"award_date"`
	IssueAuthority  *string    `db:"issue_authority" json:"issue_authority"`
	RiskStatus      *string    `db:"risk_status" json:"risk_status"`
	FileAssetID     *string    `db:"file_asset_id" json:"file_asset_id"`
	StoredPath      *string    `db:"stored_path" json:"stored_path"` // Joined from file_asset
	CompanyID       *string    `db:"company_id" json:"company_id"`
	CreatedAt       *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       *time.Time `db:"updated_at" json:"updated_at"`
}

type FileAsset struct {
	ID                string     `db:"id" json:"id"`
	FileName          string     `db:"file_name" json:"file_name"`
	Ext               *string    `db:"ext" json:"ext"`
	MimeType          *string    `db:"mime_type" json:"mime_type"`
	FileSize          *int64     `db:"file_size" json:"file_size"`
	Sha256            *string    `db:"sha256" json:"sha256"`
	SourcePath        *string    `db:"source_path" json:"source_path"`
	StoredPath        *string    `db:"stored_path" json:"stored_path"`
	SourceType        *string    `db:"source_type" json:"source_type"`
	ImportBatchID     *string    `db:"import_batch_id" json:"import_batch_id"`
	ScanStatus        *string    `db:"scan_status" json:"scan_status"`
	ParseStatus       *string    `db:"parse_status" json:"parse_status"`
	ArchiveStatus     *string    `db:"archive_status" json:"archive_status"`
	CompanyID         *string    `db:"company_id" json:"company_id"`
	CreatedAt         *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         *time.Time `db:"updated_at" json:"updated_at"`
	PlainText         *string    `db:"plain_text" json:"plain_text"`
	MarkdownText      *string    `db:"markdown_text" json:"markdown_text"`
	Status            *string    `db:"status" json:"status"`
	SourceModule      *string    `db:"source_module" json:"source_module"`
	SourceProjectID   *string    `db:"source_project_id" json:"source_project_id"`
	LastTaskID        *string    `db:"last_task_id" json:"last_task_id"`
	LastErrorMessage  *string    `db:"last_error_message" json:"last_error_message"`
	AuditID           *string    `db:"audit_id" json:"audit_id"`
	ObjectType        *string    `db:"object_type" json:"object_type"`
	ArchiveTargetType *string    `db:"archive_target_type" json:"archive_target_type"`
	ArchiveTargetID   *string    `db:"archive_target_id" json:"archive_target_id"`
}

type SharedTender struct {
	ID                 string     `db:"id" json:"id"`
	CompanyID          string     `db:"company_id" json:"company_id"`
	TenderCode         *string    `db:"tender_code" json:"tender_code"`
	ProjectName        *string    `db:"project_name" json:"project_name"`
	OwnerName          *string    `db:"owner_name" json:"owner_name"`
	TenderHash         *string    `db:"tender_hash" json:"tender_hash"`
	PrimaryFileAssetID *string    `db:"primary_file_asset_id" json:"primary_file_asset_id"`
	ParseStatus        *string    `db:"parse_status" json:"parse_status"`
	ParseResultJSON    *string    `db:"parse_result_json" json:"parse_result_json"`
	SourceModule       *string    `db:"source_module" json:"source_module"`
	SourceProjectID    *string    `db:"source_project_id" json:"source_project_id"`
	CreatedAt          *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          *time.Time `db:"updated_at" json:"updated_at"`
}

type BidProject struct {
	ID                   string     `db:"id" json:"id"`
	CompanyID            string     `db:"company_id" json:"company_id"`
	ProjectName          string     `db:"project_name" json:"project_name"`
	TenderCode           *string    `db:"tender_code" json:"tender_code"`
	ConstructionLocation *string    `db:"construction_location" json:"construction_location"`
	OwnerName            *string    `db:"owner_name" json:"owner_name"`
	ProjectStatus        *string    `db:"project_status" json:"project_status"`
	CurrentStep          *string    `db:"current_step" json:"current_step"`
	CurrentStepStatus    *string    `db:"current_step_status" json:"current_step_status"`
	AdaptationConclusion *string    `db:"adaptation_conclusion" json:"adaptation_conclusion"`
	RiskLevel            *string    `db:"risk_level" json:"risk_level"`
	BlockingReason       *string    `db:"blocking_reason" json:"blocking_reason"`
	LastErrorMessage     *string    `db:"last_error_message" json:"last_error_message"`
	LastConfirmType      *string    `db:"last_confirm_type" json:"last_confirm_type"`
	ActiveVersionNo      *int       `db:"active_version_no" json:"active_version_no"`
	CreatedSource        *string    `db:"created_source" json:"created_source"`
	CreatedBy            *string    `db:"created_by" json:"created_by"`
	CreatedAt            *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            *time.Time `db:"updated_at" json:"updated_at"`
	SharedTenderID       *string    `db:"shared_tender_id" json:"shared_tender_id"`
	SyncSourceProjectID  *string    `db:"sync_source_project_id" json:"sync_source_project_id"`
	SyncSourceModule     *string    `db:"sync_source_module" json:"sync_source_module"`
	CurrentProgress      int        `db:"current_progress" json:"current_progress"`
	CurrentStageMessage  *string    `db:"current_stage_message" json:"current_stage_message"`
	RuleMarkdownPath     *string    `db:"rule_markdown_path" json:"rule_markdown_path"`
	TemplateMarkdownPath *string    `db:"template_markdown_path" json:"template_markdown_path"`
	TemplateStartPage    *int       `db:"template_start_page" json:"template_start_page"`
	TemplateEndPage      *int       `db:"template_end_page" json:"template_end_page"`
	Step6Status          *string    `db:"step6_status" json:"step6_status"`
	Step6PayloadJSON     *string    `db:"step6_payload_json" json:"step6_payload_json"`
}

type BidProjectStep struct {
	ID             string     `db:"id" json:"id"`
	ProjectID      string     `db:"project_id" json:"project_id"`
	StepName       string     `db:"step_name" json:"step_name"`
	StepOrder      int        `db:"step_order" json:"step_order"`
	StepStatus     *string    `db:"step_status" json:"step_status"`
	WarningCount   int        `db:"warning_count" json:"warning_count"`
	RetryCount     int        `db:"retry_count" json:"retry_count"`
	BlockingReason *string    `db:"blocking_reason" json:"blocking_reason"`
	ErrorMessage   *string    `db:"error_message" json:"error_message"`
	StartedAt      *time.Time `db:"started_at" json:"started_at"`
	FinishedAt     *time.Time `db:"finished_at" json:"finished_at"`
	CreatedAt      *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at" json:"updated_at"`
}

type BidProjectFile struct {
	ID              string     `db:"id" json:"id"`
	ProjectID       string     `db:"project_id" json:"project_id"`
	CompanyID       string     `db:"company_id" json:"company_id"`
	FileAssetID     *string    `db:"file_asset_id" json:"file_asset_id"`
	FileRole        *string    `db:"file_role" json:"file_role"`
	FileName        *string    `db:"file_name" json:"file_name"`
	OriginalPath    *string    `db:"original_path" json:"original_path"`
	StoredPath      *string    `db:"stored_path" json:"stored_path"`
	MimeType        *string    `db:"mime_type" json:"mime_type"`
	FileSize        *int64     `db:"file_size" json:"file_size"`
	Sha256          *string    `db:"sha256" json:"sha256"`
	IsPrimaryTender int        `db:"is_primary_tender" json:"is_primary_tender"`
	SharedTenderID  *string    `db:"shared_tender_id" json:"shared_tender_id"`
	ReuseSource     *string    `db:"reuse_source" json:"reuse_source"`
	CreatedAt       *time.Time `db:"created_at" json:"created_at"`
	ParseStatus     *string    `db:"parse_status" json:"parse_status"`
	AssetID         *string    `db:"asset_id" json:"asset_id"`
}

type BidProjectVersion struct {
	ID           string     `db:"id" json:"id"`
	ProjectID    string     `db:"project_id" json:"project_id"`
	VersionNo    int        `db:"version_no" json:"version_no"`
	VersionType  string     `db:"version_type" json:"version_type"`
	SnapshotJSON string     `db:"snapshot_json" json:"snapshot_json"`
	SummaryJSON  string     `db:"summary_json" json:"summary_json"`
	CreatedBy    string     `db:"created_by" json:"created_by"`
	CreatedAt    *time.Time `db:"created_at" json:"created_at"`
}

type BidProjectDetail struct {
	BidProject
	Steps                   []BidProjectStep   `json:"steps"`
	Files                   []BidProjectFile   `json:"files"`
	LatestVersion           *BidProjectVersion `json:"latestVersion"`
	LatestRuleParse           interface{}        `json:"latestRuleParse"`
	LatestCompanyAdaptation   interface{}        `json:"latestCompanyAdaptation"`
	LatestResourceCombination interface{}        `json:"latestResourceCombination"`
	SharedTender            *SharedTender      `json:"sharedTender"`
}

type TechBidProject struct {
	ID                        string     `db:"id" json:"id"`
	CompanyID                 string     `db:"company_id" json:"company_id"`
	ProjectName               string     `db:"project_name" json:"project_name"`
	TenderCode                *string    `db:"tender_code" json:"tender_code"`
	ProjectType               *string    `db:"project_type" json:"project_type"`
	Profession                *string    `db:"profession" json:"profession"`
	ProjectLocation           *string    `db:"project_location" json:"project_location"`
	ContractDuration          *string    `db:"contract_duration" json:"contract_duration"`
	QualityTarget             *string    `db:"quality_target" json:"quality_target"`
	SafetyTarget              *string    `db:"safety_target" json:"safety_target"`
	ProjectStatus             *string    `db:"project_status" json:"project_status"`
	CurrentStep               *string    `db:"current_step" json:"current_step"`
	CurrentStepStatus         *string    `db:"current_step_status" json:"current_step_status"`
	StructurePlanStatus       *string    `db:"structure_plan_status" json:"structure_plan_status"`
	Step4Status               string     `db:"step4_status" json:"step4_status"`
	Step5Status               string     `db:"step5_status" json:"step5_status"`
	CoverageScore             float64    `db:"coverage_score" json:"coverage_score"`
	FullResponseRate          float64    `db:"full_response_rate" json:"full_response_rate"`
	Step4GateResult           *string    `db:"step4_gate_result" json:"step4_gate_result"`
	Step4GateReason           *string    `db:"step4_gate_reason" json:"step4_gate_reason"`
	Step4OverrideEnabled      int        `db:"step4_override_enabled" json:"step4_override_enabled"`
	Step4OverrideReason       *string    `db:"step4_override_reason" json:"step4_override_reason"`
	Step4OverrideBy           *string    `db:"step4_override_by" json:"step4_override_by"`
	Step4OverrideAt           *time.Time `db:"step4_override_at" json:"step4_override_at"`
	OutlineFingerprint        *string    `db:"outline_fingerprint" json:"outline_fingerprint"`
	HistorySimilarityHint     *string    `db:"history_similarity_hint" json:"history_similarity_hint"`
	OutlineTitlesJSON         *string    `db:"outline_titles_json" json:"outline_titles_json"`
	FinalDecision             *string    `db:"final_decision" json:"final_decision"`
	CanEnterContentGeneration int        `db:"can_enter_content_generation" json:"can_enter_content_generation"`
	VerificationMethod        string     `db:"verification_method" json:"verification_method"`
	VerificationSummary       *string    `db:"verification_summary" json:"verification_summary"`
	OverrideEnabled           int        `db:"override_enabled" json:"override_enabled"`
	OverrideReason            *string    `db:"override_reason" json:"override_reason"`
	OverrideBy                *string    `db:"override_by" json:"override_by"`
	OverrideAt                *time.Time `db:"override_at" json:"override_at"`
	ManualReviewRequired      int        `db:"manual_review_required" json:"manual_review_required"`
	ManualReviewResult        *string    `db:"manual_review_result" json:"manual_review_result"`
	ManualReviewReason        *string    `db:"manual_review_reason" json:"manual_review_reason"`
	ManualReviewBy            *string    `db:"manual_review_by" json:"manual_review_by"`
	ManualReviewAt            *time.Time `db:"manual_review_at" json:"manual_review_at"`
	RiskLevel                 *string    `db:"risk_level" json:"risk_level"`
	BlockingReason            *string    `db:"blocking_reason" json:"blocking_reason"`
	LastErrorMessage          *string    `db:"last_error_message" json:"last_error_message"`
	ActiveVersionNo           *int       `db:"active_version_no" json:"active_version_no"`
	CreatedBy                 *string    `db:"created_by" json:"created_by"`
	CreatedAt                 *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt                 *time.Time `db:"updated_at" json:"updated_at"`
	SharedTenderID            *string    `db:"shared_tender_id" json:"shared_tender_id"`
	SyncSourceProjectID       *string    `db:"sync_source_project_id" json:"sync_source_project_id"`
	SyncSourceModule          *string    `db:"sync_source_module" json:"sync_source_module"`
	VerificationResult        *string    `db:"verification_result" json:"verification_result"`
	ManualLock                int        `db:"manual_lock" json:"manual_lock"`
	CurrentProgress           int        `db:"current_progress" json:"current_progress"`
	PersonalizationScore      float64    `db:"personalization_score" json:"personalization_score"`
	StructureProfileJSON      *string    `db:"structure_profile_json" json:"structure_profile_json"`
	LastStructureRejectReason *string    `db:"last_structure_reject_reason" json:"last_structure_reject_reason"`
	ActiveStep4RunID          *int64     `db:"active_step4_run_id" json:"active_step4_run_id,omitempty"`
	ChapterDraftJSON          *string    `db:"chapter_draft_json" json:"chapter_draft_json"`
	ConfirmedChaptersJSON     *string    `db:"confirmed_chapters_json" json:"confirmed_chapters_json"`
	Step6Status               *string    `db:"step6_status" json:"step6_status"`
	Step6PayloadJSON          *string    `db:"step6_payload_json" json:"step6_payload_json"`
}

type TechBidOutlineVerification struct {
	ID                   string     `db:"id" json:"id"`
	ProjectID            string     `db:"project_id" json:"project_id"`
	AuditID              *string    `db:"audit_id" json:"audit_id"`
	FinalDecision        *string    `db:"final_decision" json:"final_decision"`
	RiskLevel            *string    `db:"risk_level" json:"risk_level"`
	Summary              *string    `db:"summary" json:"summary"`
	CriticalIssuesJSON   *string    `db:"critical_issues_json" json:"critical_issues_json"`
	MajorIssuesJSON      *string    `db:"major_issues_json" json:"major_issues_json"`
	SuggestedActionsJSON *string    `db:"suggested_actions_json" json:"suggested_actions_json"`
	CanProceed           int        `db:"can_proceed" json:"can_proceed"`
	VerificationMethod   string     `db:"verification_method" json:"verification_method"`
	VerificationModel    *string    `db:"verification_model" json:"verification_model"`
	CreatedAt            *time.Time `db:"created_at" json:"created_at"`
}

type TechBidManualOverride struct {
	ID                         string     `db:"id" json:"id"`
	ProjectID                  string     `db:"project_id" json:"project_id"`
	OriginalStatus             *string    `db:"original_status" json:"original_status"`
	TargetStatus               *string    `db:"target_status" json:"target_status"`
	OperatorID                 *string    `db:"operator_id" json:"operator_id"`
	OperatorName               *string    `db:"operator_name" json:"operator_name"`
	Reason                     string     `db:"reason" json:"reason"`
	SnapshotBeforeOverrideJSON *string    `db:"snapshot_before_override_json" json:"snapshot_before_override_json"`
	CreatedAt                  *time.Time `db:"created_at" json:"created_at"`
}

type TechBidStateTransition struct {
	ID                 int64      `db:"id" json:"id"`
	ProjectID          string     `db:"project_id" json:"project_id"`
	FromStatus         *string    `db:"from_status" json:"from_status"`
	ToStatus           *string    `db:"to_status" json:"to_status"`
	FromStep           *string    `db:"from_step" json:"from_step"`
	ToStep             *string    `db:"to_step" json:"to_step"`
	TriggerType        string     `db:"trigger_type" json:"trigger_type"` // ai, manual, system
	TriggerReason      *string    `db:"trigger_reason" json:"trigger_reason"`
	VerificationMethod string     `db:"verification_method" json:"verification_method"`
	OperatorID         *string    `db:"operator_id" json:"operator_id"`
	OperatorName       *string    `db:"operator_name" json:"operator_name"`
	CreatedAt          *time.Time `db:"created_at" json:"created_at"`
}

type TechBidProjectDetail struct {
	TechBidProject
	ChapterPlans       []TechBidChapterPlan        `json:"chapterPlans"`
	Files              []BidProjectFile            `json:"tenderFiles"`
	LatestVersion      *BidProjectVersion          `json:"latestVersion"`
	SharedTender       *SharedTender               `json:"sharedTender"`
	Profile            *TechBidProjectProfile      `json:"profile"`
	Facts              []TechBidOutlineFact        `json:"facts"`
	LatestAudit        *TechBidOutlineAudit        `json:"latestAudit"`
	LatestVerification *TechBidOutlineVerification `json:"latestVerification"`
}

type TechBidProjectProfile struct {
	ID                 string     `db:"id" json:"id"`
	ProjectID          string     `db:"project_id" json:"project_id"`
	ProfileJSON        *string    `db:"profile_json" json:"profile_json"`
	SummaryText        *string    `db:"summary_text" json:"summary_text"`
	SchemaVersion      *string    `db:"schema_version" json:"schema_version"`
	ExtractionMetaJSON *string    `db:"extraction_meta_json" json:"extraction_meta_json"`
	GeneratedBy        *string    `db:"generated_by" json:"generated_by"`
	ConfirmedAt        *time.Time `db:"confirmed_at" json:"confirmed_at"`
	ConfirmedBy        *string    `db:"confirmed_by" json:"confirmed_by"`
	EditCount          *int       `db:"edit_count" json:"edit_count"`
	CreatedAt          *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          *time.Time `db:"updated_at" json:"updated_at"`
}

type TechBidRiskRecord struct {
	ID          string     `db:"id" json:"id"`
	ProjectID   string     `db:"project_id" json:"project_id"`
	TaskID      *string    `db:"task_id" json:"task_id"`
	RiskGroup   *string    `db:"risk_group" json:"risk_group"`
	Title       string     `db:"title" json:"title"`
	Description *string    `db:"description" json:"description"`
	Level       *string    `db:"level" json:"level"`
	Status      *string    `db:"status" json:"status"`
	FixAdvice   *string    `db:"fix_advice" json:"fix_advice"`
	Evidence    *string    `db:"evidence" json:"evidence"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at" json:"updated_at"`
}

type TechBidChapterPlan struct {
	ID                  string     `db:"id" json:"id"`
	ProjectID           string     `db:"project_id" json:"project_id"`
	ChapterKey          *string    `db:"chapter_key" json:"chapter_key"`
	ChapterName         string     `db:"chapter_name" json:"chapter_name"`
	ChapterOrder        int        `db:"chapter_order" json:"chapter_order"`
	ParentID            *string    `db:"parent_id" json:"parent_id"`
	NodeLevel           *string    `db:"node_level" json:"node_level"`
	WordMin             *int       `db:"word_min" json:"word_min"`
	WordMax             *int       `db:"word_max" json:"word_max"`
	StyleID             *string    `db:"style_id" json:"style_id"`
	FocusPoints         *string    `db:"focus_points" json:"focus_points"`
	ForbiddenItems      *string    `db:"forbidden_items" json:"forbidden_items"`
	ReferenceSources    *string    `db:"reference_sources" json:"reference_sources"`
	GenerationStatus    *string    `db:"generation_status" json:"generation_status"`
	CreatedAt           *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           *time.Time `db:"updated_at" json:"updated_at"`
	ContentMD           *string    `db:"content_md" json:"content_md"`
	ContentHTML         *string    `db:"content_html" json:"content_html"`
	ContentUpdatedAt    *time.Time `db:"content_updated_at" json:"content_updated_at"`
	CoverageSourcesJSON *string    `db:"coverage_sources_json" json:"coverage_sources_json"`
	RequirementIdsJSON  *string    `db:"requirement_ids_json" json:"requirement_ids_json"`
	CoverageLevel       *string    `db:"coverage_level" json:"coverage_level"`
	OutlineVersion      int        `db:"outline_version" json:"outline_version"`
	MustHave            int        `db:"must_have" json:"must_have"`
	ScoreRelated        int        `db:"score_related" json:"score_related"`
}

type ProjectOutput struct {
	ID         string     `db:"id" json:"id"`
	ProjectID  string     `db:"project_id" json:"project_id"`
	VersionNo  *int       `db:"version_no" json:"version_no"`
	OutputType *string    `db:"output_type" json:"output_type"`
	FileName   *string    `db:"file_name" json:"file_name"`
	FilePath   *string    `db:"file_path" json:"file_path"`
	MimeType   *string    `db:"mime_type" json:"mime_type"`
	Status     *string    `db:"status" json:"status"`
	CreatedAt  *time.Time `db:"created_at" json:"created_at"`
}

type AuditDetail struct {
	ID            string  `json:"id" db:"id"`
	FileName      string  `json:"file_name" db:"file_name"`
	MimeType      string  `json:"mime_type" db:"mime_type"`
	StoredPath    string  `json:"stored_path" db:"stored_path"`
	ObjectType    string  `json:"object_type" db:"object_type"`
	AuditStatus   string  `json:"audit_status" db:"audit_status"`
	ExtractedData string  `json:"extracted_data" db:"extracted_data"`
	OCRText       string  `json:"ocr_text" db:"ocr_text"`
	AICleanText   string  `json:"ai_clean_text" db:"ai_clean_text"`
	FileID        string  `json:"file_id" db:"file_id"`
	ReviewerID    *string `json:"reviewer_id" db:"reviewer_id"`
	ReviewerName  *string `json:"reviewer_name" db:"reviewer_name"`
	ArchiveType   *string `json:"archive_target_type" db:"archive_target_type"`
	ArchiveID     *string `json:"archive_target_id" db:"archive_target_id"`
}

type TechBidKnowledgeItem struct {
	ID              string     `db:"id" json:"id"`
	ItemType        string     `db:"item_type" json:"item_type"`
	Title           string     `db:"title" json:"title"`
	Summary         *string    `db:"summary" json:"summary"`
	Content         *string    `db:"content" json:"content"`
	OriginalText    *string    `db:"original_text" json:"original_text"`
	SourcePage      *string    `db:"source_page" json:"source_page"`
	SourceParagraph *string    `db:"source_paragraph" json:"source_paragraph"`
	SourceSnippet   *string    `db:"source_snippet" json:"source_snippet"`
	Confidence      *float64   `db:"confidence" json:"confidence"`
	MatchStatus     *string    `db:"match_status" json:"match_status"`
	ReviewStatus    *string    `db:"review_status" json:"review_status"`
	MergedToID      *string    `db:"merged_to_id" json:"merged_to_id"`
	TagsJSON        *string    `db:"tags_json" json:"tags_json"`
	FileAssetID     *string    `db:"file_asset_id" json:"file_asset_id"`
	CreatedAt       *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       *time.Time `db:"updated_at" json:"updated_at"`
}
type FileContent struct {
	ID              string     `db:"id" json:"id"`
	FileAssetID     string     `db:"file_asset_id" json:"file_asset_id"`
	PlainText       *string    `db:"plain_text" json:"plain_text"`
	MarkdownText    *string    `db:"markdown_text" json:"markdown_text"`
	PageMappingJSON *string    `db:"page_mapping_json" json:"page_mapping_json"`
	ContentType     *string    `db:"content_type" json:"content_type"`
	Version         int        `db:"version" json:"version"`
	CreatedAt       *time.Time `db:"created_at" json:"created_at"`
}

type OCRSettings struct {
	ID                  string     `db:"id" json:"id"`
	Mode                string     `db:"mode" json:"mode"` // cloud / local / private / auto
	ServiceURL          *string    `db:"service_url" json:"service_url"`
	ServicePort         *string    `db:"service_port" json:"service_port"`
	APIKey              *string    `db:"api_key" json:"api_key"`
	Token               *string    `db:"token" json:"token"`
	DefaultStrategy     *string    `db:"default_strategy" json:"default_strategy"`
	AllowAutoDownload   int        `db:"allow_auto_download" json:"allow_auto_download"`
	ConfidenceThreshold float64    `db:"confidence_threshold" json:"confidence_threshold"`
	MaxConcurrency      int        `db:"max_concurrency" json:"max_concurrency"`
	TimeoutSeconds      int        `db:"timeout_seconds" json:"timeout_seconds"`
	RetryTimes          int        `db:"retry_times" json:"retry_times"`
	ModelVersion        *string    `db:"model_version" json:"model_version"`
	Status              string     `db:"status" json:"status"`
	CompanyID           *string    `db:"company_id" json:"company_id"`
	CreatedAt           *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           *time.Time `db:"updated_at" json:"updated_at"`
}

type IssueRecord struct {
	ID             string    `db:"id" json:"id"`
	IssueType      string    `db:"issue_type" json:"issue_type"`
	ObjectType     string    `db:"object_type" json:"object_type"`
	ObjectID       string    `db:"object_id" json:"object_id"`
	SourceID       *string   `db:"source_id" json:"source_id"`
	Severity       string    `db:"severity" json:"severity"`
	Status         string    `db:"status" json:"status"`
	IssueMessage   string    `db:"issue_message" json:"issue_message"`
	ResolutionNote *string   `db:"resolution_note" json:"resolution_note"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
	CompanyID      string    `db:"company_id" json:"company_id"`
	ProjectID      *string   `db:"project_id" json:"project_id"`
}

type TechBidOutlineFact struct {
	ID             string     `db:"id" json:"id"`
	ProjectID      string     `db:"project_id" json:"project_id"`
	FactCode       *string    `db:"fact_code" json:"fact_code"` // 业务事实 ID，如 S1、M1（与 requirement_ids 对齐）
	FactType       string     `db:"fact_type" json:"fact_type"` // score_item, mandatory_spec, project_characteristic, special_topic
	FactName       *string    `db:"fact_name" json:"fact_name"`
	FactContent    *string    `db:"fact_content" json:"fact_content"`
	SourceText     *string    `db:"source_text" json:"source_text"`
	SourceLocation *string    `db:"source_location" json:"source_location"`
	Priority       *string    `db:"priority" json:"priority"`
	ScoreValue     *float64   `db:"score_value" json:"score_value"`
	PenaltyLevel   *string    `db:"penalty_level" json:"penalty_level"`
	TagsJSON       *string    `db:"tags_json" json:"tags_json"`
	CreatedAt      *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at" json:"updated_at"`
}

// TechBidFactMapping 对应表 tech_bid_fact_mappings（Step4 facts→目录强映射）
type TechBidFactMapping struct {
	ID             string     `db:"id" json:"id"`
	ProjectID      string     `db:"project_id" json:"project_id"`
	FactID         string     `db:"fact_id" json:"fact_id"`
	FactType       string     `db:"fact_type" json:"fact_type"`
	FactName       string     `db:"fact_name" json:"fact_name"`
	TargetLevel    string     `db:"target_level" json:"target_level"`
	TargetPathJSON string     `db:"target_path_json" json:"target_path_json,omitempty"`
	Required       int        `db:"required" json:"required"`
	Priority       string     `db:"priority" json:"priority"`
	MappingReason  *string    `db:"mapping_reason" json:"mapping_reason,omitempty"`
	MappingSource  string     `db:"mapping_source" json:"mapping_source"`
	CreatedAt      *time.Time `db:"created_at" json:"created_at,omitempty"`
}

// TechBidRequirementRegister 招标要求总表（Step4 真相层）
type TechBidRequirementRegister struct {
	ID                    string     `db:"id" json:"id"`
	ProjectID             string     `db:"project_id" json:"project_id"`
	RequirementID         string     `db:"requirement_id" json:"requirement_id"`
	RequirementType       string     `db:"requirement_type" json:"requirement_type"`
	SourceText            string     `db:"source_text" json:"source_text"`
	SourceLocation        string     `db:"source_location" json:"source_location"`
	Priority              string     `db:"priority" json:"priority"`
	MustBeExplicit        int        `db:"must_be_explicit" json:"must_be_explicit"`
	ExpectedResponseLevel string     `db:"expected_response_level" json:"expected_response_level"`
	Domain                string     `db:"domain" json:"domain"`
	ResponseTier          string     `db:"response_tier" json:"response_tier"`
	Summary               string     `db:"summary" json:"summary"`
	CreatedAt             *time.Time `db:"created_at" json:"created_at,omitempty"`
}

// TechBidRequirementResponseCheck 完全响应率校验结果
type TechBidRequirementResponseCheck struct {
	ID                           string     `db:"id" json:"id"`
	ProjectID                    string     `db:"project_id" json:"project_id"`
	OutlineVersion               int        `db:"outline_version" json:"outline_version"`
	RequirementTotal             int        `db:"requirement_total" json:"requirement_total"`
	RequirementMapped            int        `db:"requirement_mapped" json:"requirement_mapped"`
	RequirementFullyResponded    int        `db:"requirement_fully_responded" json:"requirement_fully_responded"`
	RequirementWeaklyResponded   int        `db:"requirement_weakly_responded" json:"requirement_weakly_responded"`
	RequirementOnlyTagged        int        `db:"requirement_only_tagged" json:"requirement_only_tagged"`
	FullResponseRate             float64    `db:"full_response_rate" json:"full_response_rate"`
	WeakResponseRate             float64    `db:"weak_response_rate" json:"weak_response_rate"`
	ResponseQualityScore         float64    `db:"response_quality_score" json:"response_quality_score"`
	MissingRequirementIDsJSON    *string    `db:"missing_requirement_ids_json" json:"-"`
	WeakRequirementIDsJSON       *string    `db:"weak_requirement_ids_json" json:"-"`
	OnlyTaggedRequirementIDsJSON *string    `db:"only_tagged_requirement_ids_json" json:"-"`
	ShellTitleHintsJSON          *string    `db:"shell_title_hints_json" json:"-"`
	HighPriorityMissingIDsJSON   *string    `db:"high_priority_missing_ids_json" json:"-"`
	MandatoryMissingIDsJSON      *string    `db:"mandatory_missing_ids_json" json:"-"`
	MandatoryInsufficientIDsJSON *string    `db:"mandatory_insufficient_ids_json" json:"-"`
	HardRuleWarningsJSON         *string    `db:"hard_rule_warnings_json" json:"-"`
	Result                       string     `db:"result" json:"result"`
	Summary                      *string    `db:"summary" json:"summary,omitempty"`
	CreatedAt                    *time.Time `db:"created_at" json:"created_at,omitempty"`
}

// TechBidOutlineCoverageCheck 对应表 tech_bid_outline_coverage_checks
type TechBidOutlineCoverageCheck struct {
	ID                   string     `db:"id" json:"id"`
	ProjectID            string     `db:"project_id" json:"project_id"`
	OutlineVersion       int        `db:"outline_version" json:"outline_version"`
	FactTotal            int        `db:"fact_total" json:"fact_total"`
	FactMapped           int        `db:"fact_mapped" json:"fact_mapped"`
	CoverageRate         float64    `db:"coverage_rate" json:"coverage_rate"`
	MissingFactIDsJSON   *string    `db:"missing_fact_ids_json" json:"-"`
	WeakFactIDsJSON      *string    `db:"weak_fact_ids_json" json:"-"`
	DuplicateNodeIDsJSON *string    `db:"duplicate_node_ids_json" json:"-"`
	Result               string     `db:"result" json:"result"`
	Summary              *string    `db:"summary" json:"summary,omitempty"`
	CreatedAt            *time.Time `db:"created_at" json:"created_at,omitempty"`
}

type TechBidOutlineAudit struct {
	ID                  string     `db:"id" json:"id"`
	ProjectID           string     `db:"project_id" json:"project_id"`
	OutlineSnapshotJSON *string    `db:"outline_snapshot_json" json:"outline_snapshot_json"`
	FactsSnapshotJSON   *string    `db:"facts_snapshot_json" json:"facts_snapshot_json"`
	CoverageScore       float64    `db:"coverage_score" json:"coverage_score"`
	AuditSummary        *string    `db:"audit_summary" json:"audit_summary"`
	MissingItemsJSON    *string    `db:"missing_items_json" json:"missing_items_json"`
	WeakItemsJSON       *string    `db:"weak_items_json" json:"weak_items_json"`
	DuplicateItemsJSON  *string    `db:"duplicate_items_json" json:"duplicate_items_json"`
	FinalDecision       *string    `db:"final_decision" json:"final_decision"` // PASS, REVISE, BLOCK
	RiskLevel           *string    `db:"risk_level" json:"risk_level"`         // LOW, MEDIUM, HIGH
	CanProceed          int        `db:"can_proceed" json:"can_proceed"`
	DetailJSON          *string    `db:"detail_json" json:"detail_json"`
	AuditModel          *string    `db:"audit_model" json:"audit_model"`
	AuditVersion        string     `db:"audit_version" json:"audit_version"`
	CreatedAt           *time.Time `db:"created_at" json:"created_at"`
}
type IndustrySkeletonDB struct {
	ID                     string     `db:"id" json:"id"`
	IndustryName           string     `db:"industry_name" json:"industry_name"`
	ParentID               *string    `db:"parent_id" json:"parent_id,omitempty"` // New field for categorization
	LogicalChaptersJSON    string     `db:"logical_chapters_json" json:"logical_chapters_json"`
	CommonSectionPoolJSON  *string    `db:"common_section_pool_json" json:"common_section_pool_json,omitempty"`
	IndustryKeywordsJSON   *string    `db:"industry_keywords_json" json:"industry_keywords_json,omitempty"`
	TitleCandidatePoolJSON *string    `db:"title_candidate_pool_json" json:"title_candidate_pool_json,omitempty"`
	MatchingRulesJSON      *string    `db:"matching_rules_json" json:"matching_rules_json,omitempty"`
	CreatedAt              *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt              *time.Time `db:"updated_at" json:"updated_at"`
}

type LogicalChapter struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsMandatory bool     `json:"is_mandatory"`
	UnitPool    []string `json:"unit_pool"`
	// Elasticity Controls
	IsCoreChapter      bool     `json:"is_core_chapter"`
	CanReorder         bool     `json:"can_reorder"`
	CanSplit           bool     `json:"can_split"`
	CanMerge           bool     `json:"can_merge"`
	CanInsertBefore    bool     `json:"can_insert_before"`
	CanInsertAfter     bool     `json:"can_insert_after"`
	PriorityRange      []string `json:"priority_range"`
	FactTypePreference []string `json:"fact_type_preference"`
}

// TechBidConflictAudit 逻辑冲突审计结果
type TechBidConflictAudit struct {
	ID           string     `db:"id" json:"id"`
	ProjectID    string     `db:"project_id" json:"project_id"`
	HasBlock     int        `db:"has_block" json:"has_block"`
	ConflictJSON *string    `db:"conflict_json" json:"conflict_json"`
	Summary      *string    `db:"summary" json:"summary"`
	CreatedAt    *time.Time `db:"created_at" json:"created_at"`
}

type OutlineTitlesJSON struct {
	Nodes []OutlineNode `json:"nodes"`
}

type OutlineNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Level int    `json:"level"`
}
