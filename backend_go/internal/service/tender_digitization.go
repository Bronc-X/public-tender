package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type FactItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name" validate:"required"`
	Content         string   `json:"content" validate:"required"`
	SourceText      string   `json:"source_text"`
	SourceLocation  string   `json:"source_location"`
	SourceChapter   string   `json:"source_chapter"`
	PageNumber      int      `json:"page_number"`
	LineNumber      int      `json:"line_number"`
	Priority        string   `json:"priority" validate:"required,oneof=high medium low"`
	ScoreValue      float64  `json:"score_value"`
	PenaltyLevel    string   `json:"penalty_level"`
	Tags            []string `json:"tags"`
	EvidenceCount   int      `json:"evidence_count"`
	ExtractedByView string   `json:"extracted_by_view"`
}

type FactExtractResult struct {
	ScoreItems             []FactItem `json:"score_items" validate:"dive"`
	MandatorySpecs         []FactItem `json:"mandatory_specs" validate:"dive"`
	ProjectCharacteristics []FactItem `json:"project_characteristics" validate:"dive"`
	SpecialTopics          []FactItem `json:"special_topics" validate:"dive"`
}

type AuditIssue struct {
	RequirementID  string `json:"requirement_id"`
	Description    string `json:"description"`
	Priority       string `json:"priority"`
	Reason         string `json:"reason"`
	ActionType     string `json:"action_type"`     // insert, expand, merge, reorder, rewrite
	TargetSection  string `json:"target_section"`  // Suggested location
	ExpectedEffect string `json:"expected_effect"` // Goal of the change
}

type ConflictItem struct {
	ID                   string `json:"conflict_id"`
	Type                 string `json:"conflict_type"` // schedule, scope, material, term, numeric
	FieldName            string `json:"field_name"`
	SourceA              string `json:"source_a"`
	SourceB              string `json:"source_b"`
	Reason               string `json:"conflict_reason"`
	Severity             string `json:"severity"` // high, medium, low
	ManualReviewRequired bool   `json:"manual_review_required"`
}

type ConflictAuditResult struct {
	Conflicts []ConflictItem `json:"conflicts" validate:"dive"`
	Summary   string         `json:"summary"`
	HasBlock  bool           `json:"has_block"`
}

type TenderChapter struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Index   int    `json:"index"`
}

type ProjectGlobalContext struct {
	ProjectName    string   `json:"project_name"`
	Location       string   `json:"location"`
	Duration       string   `json:"duration"`
	Scope          string   `json:"scope"`
	BudgetInfo     string   `json:"budget_info"`
	KeyRedFlags    []string `json:"key_red_flags"`
	ScoringSummary string   `json:"scoring_summary"`
}

type CoverageAuditResult struct {
	CoverageScore  float64      `json:"coverage_score" validate:"gte=0,lte=100"` // not "required": 0 is valid for float64
	AuditSummary   string       `json:"audit_summary" validate:"required"`
	MissingItems   []AuditIssue `json:"missing_items" validate:"dive"`
	WeakItems      []AuditIssue `json:"weak_items" validate:"dive"`
	DuplicateItems []AuditIssue `json:"duplicate_items" validate:"dive"`
}

type VerificationResult struct {
	FinalDecision    string   `json:"final_decision" validate:"required,oneof=PASS REVISE BLOCK"`
	RiskLevel        string   `json:"risk_level" validate:"required,oneof=LOW MEDIUM HIGH"`
	Summary          string   `json:"summary" validate:"required"`
	CriticalIssues   []string `json:"critical_issues"`
	MajorIssues      []string `json:"major_issues"`
	SuggestedActions []string `json:"suggested_actions"`
	CanProceed       bool     `json:"can_proceed"`
}

type AISection struct {
	Name  string   `json:"name" validate:"required"`
	Units []AIUnit `json:"units" validate:"required,dive"`
}

type AIUnit struct {
	Name        string         `json:"name" validate:"required"`
	Subsections []AISubsection `json:"subsections" validate:"required,dive"`
}

type AISubsection struct {
	Name           string   `json:"name" validate:"required"`
	RequirementIDs []string `json:"requirement_ids"`
}

type OutlineOptimizationSuggestion struct {
	InsertUnder    string   `json:"insert_under"`
	NewUnit        string   `json:"new_unit"`
	NewSubsections []string `json:"new_subsections"`
	RequirementID  string   `json:"requirement_id"`
	Reason         string   `json:"reason"`
}

type ProjectProfileField struct {
	Value          string   `json:"value"`
	SourceText     string   `json:"source_text,omitempty"`
	SourceLocation string   `json:"source_location,omitempty"`
	Confidence     float64  `json:"confidence,omitempty"`
	Missing        bool     `json:"missing"`
	Notes          string   `json:"notes,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
}

type ProjectProfileListItem struct {
	Name             string  `json:"name,omitempty"`
	Value            string  `json:"value,omitempty"`
	SourceText       string  `json:"source_text,omitempty"`
	SourceLocation   string  `json:"source_location,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
	Missing          bool    `json:"missing"`
	Notes            string  `json:"notes,omitempty"`
	RequiresEvidence *bool   `json:"requires_evidence,omitempty"`
}

type ProjectBaseInfoProfile struct {
	ProjectName          ProjectProfileField `json:"project_name"`
	OwnerUnit            ProjectProfileField `json:"owner_unit"`
	Location             ProjectProfileField `json:"location"`
	CategoryAndScope     ProjectProfileField `json:"category_and_scope"`
	DurationRequirements ProjectProfileField `json:"duration_requirements"`
	QualityStandard      ProjectProfileField `json:"quality_standard"`
}

type ConstructionCoreRequirementsProfile struct {
	MaterialEquipmentRules  ProjectProfileField      `json:"material_equipment_rules"`
	TechnicalSpecifications ProjectProfileField      `json:"technical_specifications"`
	SiteManagement          ProjectProfileField      `json:"site_management"`
	AcceptanceRequirements  ProjectProfileField      `json:"acceptance_requirements"`
	SpecialOperations       ProjectProfileField      `json:"special_operations"`
	ProcurementBoundary     ProjectProfileField      `json:"procurement_boundary"`
	OwnerSuppliedItems      []ProjectProfileListItem `json:"owner_supplied_items"`
	ContractorSuppliedItems []ProjectProfileListItem `json:"contractor_supplied_items"`
	ScheduleConstraints     ProjectProfileField      `json:"schedule_constraints"`
}

type BidderRequirementsProfile struct {
	QualificationCertificates  ProjectProfileField      `json:"qualification_certificates"`
	PerformanceRequirements    []ProjectProfileListItem `json:"performance_requirements"`
	PersonnelRequirements      []ProjectProfileListItem `json:"personnel_requirements"`
	FinancialRequirements      []ProjectProfileListItem `json:"financial_requirements"`
	CreditRequirements         []ProjectProfileListItem `json:"credit_requirements"`
	BonusItems                 []ProjectProfileListItem `json:"bonus_items"`
	QualificationRequirements  []ProjectProfileListItem `json:"qualification_requirements"`
	OtherMandatoryRequirements []ProjectProfileListItem `json:"other_mandatory_requirements"`
}

type EvaluationAndPerformanceRulesProfile struct {
	MethodAndScoreWeights         ProjectProfileField      `json:"method_and_score_weights"`
	TechnicalEvaluationDimensions ProjectProfileField      `json:"technical_evaluation_dimensions"`
	PaymentMethod                 ProjectProfileField      `json:"payment_method"`
	SettlementRules               ProjectProfileField      `json:"settlement_rules"`
	ScoringItems                  []ProjectProfileListItem `json:"scoring_items"`
	DisqualificationRules         []ProjectProfileListItem `json:"disqualification_rules"`
	TotalDuration                 ProjectProfileField      `json:"total_duration"`
}

type KeywordAuditHit struct {
	Group       string   `json:"group"`
	Keywords    []string `json:"keywords"`
	FieldLabels []string `json:"field_labels"`
	Severity    string   `json:"severity"` // warning | error
	HitCount    int      `json:"hit_count"`
}

// MergeDiffEntry records a single merge decision for traceability.
type MergeDiffEntry struct {
	FieldLabel string  `json:"field_label"`
	ChunkIndex int     `json:"chunk_index"`
	Action     string  `json:"action"` // skip_empty/replace_empty/append_evidence/replace_better/keep_existing/close_score
	OldValue   string  `json:"old_value,omitempty"`
	NewValue   string  `json:"new_value,omitempty"`
	ScoreOld   float64 `json:"score_old,omitempty"`
	ScoreNew   float64 `json:"score_new,omitempty"`
	Reason     string  `json:"reason"`
}

type FormatTemplateBoundary struct {
	Detected   bool   `json:"detected"`
	StartPage  int    `json:"start_page"`
	EndPage    int    `json:"end_page"`
	SourceText string `json:"source_text,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type ProjectProfileResult struct {
	ProjectBaseInfo               ProjectBaseInfoProfile               `json:"project_base_info"`
	ConstructionCoreRequirements  ConstructionCoreRequirementsProfile  `json:"construction_core_requirements"`
	BidderRequirements            BidderRequirementsProfile            `json:"bidder_requirements"`
	EvaluationAndPerformanceRules EvaluationAndPerformanceRulesProfile `json:"evaluation_and_performance_rules"`
	ExtractionGaps                []ProjectProfileListItem             `json:"extraction_gaps"`
	UncertainItems                []ProjectProfileListItem             `json:"uncertain_items"`
	RequiresManualReview          ProjectProfileField                  `json:"requires_manual_review"`
	KeywordAuditHits              []KeywordAuditHit                    `json:"keyword_audit_hits,omitempty"`
	RuleEngineHits                []RuleHit                            `json:"rule_engine_hits,omitempty"`
	FormatTemplateBoundary        FormatTemplateBoundary               `json:"format_template_boundary,omitempty"`
}

type ProjectProfileDigitizationResult struct {
	RawText          string                 `json:"raw_text"`
	Chunks           []string               `json:"chunks"`
	ChunkTypes       []string               `json:"chunk_types"`
	ChunkOutputs     []string               `json:"chunk_outputs"`
	PromptInputs     []string               `json:"prompt_inputs"`
	ChunkResults     []ProjectProfileResult `json:"chunk_results"`
	MergedProfile    ProjectProfileResult   `json:"merged_profile"`
	MergeDiffs       []MergeDiffEntry       `json:"merge_diffs,omitempty"`
	DetectedIndustry string                 `json:"detected_industry,omitempty"`
	ProcessedAt      string                 `json:"processed_at"`
}

type TenderDigitizationService struct {
	aiClient      *AIClient
	promptService *PromptService
	cacheMetrics  *CacheMetricsService
	validate      *validator.Validate
	db            *sqlx.DB
	ElasticEngine *ElasticOutlineEngine
}

func NewTenderDigitizationService(aiClient *AIClient, promptService *PromptService, cacheMetrics *CacheMetricsService, db *sqlx.DB, elastic *ElasticOutlineEngine) *TenderDigitizationService {
	return &TenderDigitizationService{
		aiClient:      aiClient,
		promptService: promptService,
		cacheMetrics:  cacheMetrics,
		validate:      validator.New(),
		db:            db,
		ElasticEngine: elastic,
	}
}

func normalizeFactPriority(p string) string {
	p = strings.TrimSpace(strings.ToLower(p))
	switch p {
	case "", "未知", "n/a", "na":
		return "medium"
	case "高", "h":
		return "high"
	case "中", "m":
		return "medium"
	case "低", "l":
		return "low"
	}
	if p == "high" || p == "medium" || p == "low" {
		return p
	}
	if strings.HasPrefix(p, "high") {
		return "high"
	}
	if strings.HasPrefix(p, "low") {
		return "low"
	}
	return "medium"
}

func (s *TenderDigitizationService) normalizeFactExtractResult(r *FactExtractResult) {
	seenIDs := map[string]int{}
	fix := func(prefix string, items []FactItem) []FactItem {
		for i := range items {
			items[i].ID = normalizeUniqueFactID(items[i].ID, prefix, i+1, seenIDs)
			items[i].Priority = normalizeFactPriority(items[i].Priority)
			if strings.TrimSpace(items[i].Name) == "" {
				items[i].Name = "未命名条目"
			}
			if strings.TrimSpace(items[i].Content) == "" {
				items[i].Content = "（内容待补充）"
			}
		}
		return items
	}
	r.ScoreItems = fix("score", r.ScoreItems)
	r.MandatorySpecs = fix("ms", r.MandatorySpecs)
	r.ProjectCharacteristics = fix("pc", r.ProjectCharacteristics)
	r.SpecialTopics = fix("st", r.SpecialTopics)
}

func normalizeUniqueFactID(raw, prefix string, index int, seen map[string]int) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		id = fmt.Sprintf("%s_%02d", prefix, index)
	}
	id = strings.ReplaceAll(id, " ", "_")
	if _, ok := seen[id]; !ok {
		seen[id] = 1
		return id
	}
	seen[id]++
	return fmt.Sprintf("%s_%02d_%d", prefix, index, seen[id])
}

func (s *TenderDigitizationService) coerceFactMapToResult(row map[string]interface{}, r *FactExtractResult) {
	parser := func(keys ...string) []FactItem {
		for _, k := range keys {
			if v, ok := row[k]; ok {
				var items []FactItem
				b, _ := json.Marshal(v)
				if err := json.Unmarshal(b, &items); err == nil && len(items) > 0 {
					return items
				}
				// Try individual item mapping if pure array unmarshal fails
				if arr, ok := v.([]interface{}); ok {
					var res []FactItem
					for _, it := range arr {
						if m, ok := it.(map[string]interface{}); ok {
							res = append(res, s.cleanExtractionItem(m))
						}
					}
					if len(res) > 0 {
						return res
					}
				}
			}
		}
		return nil
	}

	if len(r.ScoreItems) == 0 {
		r.ScoreItems = parser("score_items", "points", "scoring_points", "score", "ScoringItems")
	}
	if len(r.MandatorySpecs) == 0 {
		r.MandatorySpecs = parser("mandatory_specs", "mandatory", "specs", "requirements")
	}
	if len(r.ProjectCharacteristics) == 0 {
		r.ProjectCharacteristics = parser("project_characteristics", "characteristics", "features")
	}
	if len(r.SpecialTopics) == 0 {
		r.SpecialTopics = parser("special_topics", "topics", "others")
	}
}

func (s *TenderDigitizationService) cleanExtractionItem(m map[string]interface{}) FactItem {
	return FactItem{
		Name:     firstStringFromMap(m, "fact_name", "name", "title", "项", "名称"),
		Content:  firstStringFromMap(m, "fact_content", "content", "summary", "详细", "内容"),
		ID:       firstStringFromMap(m, "fact_code", "code", "id", "编号"),
		Priority: normalizeFactPriority(firstStringFromMap(m, "priority", "level", "重要性")),
	}
}

func newMissingProjectProfileField(note string) ProjectProfileField {
	note = strings.TrimSpace(note)
	if note == "" {
		note = "未提取"
	}
	return ProjectProfileField{Missing: true, Notes: note}
}

func newBooleanProjectProfileField(flag bool, note string) ProjectProfileField {
	return ProjectProfileField{
		Value:      map[bool]string{true: "是", false: "否"}[flag],
		Confidence: 1,
		Missing:    false,
		Notes:      strings.TrimSpace(note),
	}
}

func isEmptyProjectProfileValue(value string) bool {
	normalized := strings.TrimSpace(strings.ToLower(value))
	return normalized == "" || normalized == "无" || normalized == "未提取" || normalized == "未知" || normalized == "n/a" || normalized == "na" || normalized == "null"
}

func normalizeProjectProfileConfidence(value interface{}) float64 {
	conf := flexibleFloat64(value)
	if conf > 1 {
		conf = conf / 100
	}
	if conf < 0 {
		return 0
	}
	if conf > 1 {
		return 1
	}
	return conf
}

func normalizeProjectProfileMissing(value interface{}) bool {
	switch t := value.(type) {
	case bool:
		return t
	case string:
		normalized := strings.TrimSpace(strings.ToLower(t))
		return normalized == "true" || normalized == "yes" || normalized == "1" || normalized == "是" || normalized == "缺失" || normalized == "missing"
	default:
		return false
	}
}

func lookupProjectProfileValue(m map[string]interface{}, keys ...string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v, true
		}
	}
	for existingKey, value := range m {
		normalizedExisting := strings.TrimSpace(strings.ToLower(existingKey))
		for _, key := range keys {
			if normalizedExisting == strings.TrimSpace(strings.ToLower(key)) {
				return value, true
			}
		}
	}
	return nil, false
}

func pickProjectProfileRaw(section map[string]interface{}, root map[string]interface{}, keys ...string) interface{} {
	if v, ok := lookupProjectProfileValue(section, keys...); ok {
		return v
	}
	if root != nil {
		if v, ok := lookupProjectProfileValue(root, keys...); ok {
			return v
		}
	}
	return nil
}

func coerceProjectProfileMap(raw interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}
	if m, ok := raw.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func unwrapProjectProfileRoot(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	for _, key := range []string{"data", "result", "profile", "output", "response"} {
		inner, ok := m[key].(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := inner["project_base_info"]; ok {
			return inner
		}
		if _, ok := inner["construction_core_requirements"]; ok {
			return inner
		}
	}
	return m
}

func normalizeProjectProfileListItem(raw interface{}) ProjectProfileListItem {
	switch t := raw.(type) {
	case nil:
		return ProjectProfileListItem{Missing: true, Notes: "未提取"}
	case string:
		value := strings.TrimSpace(t)
		if isEmptyProjectProfileValue(value) {
			return ProjectProfileListItem{Missing: true, Notes: "未提取"}
		}
		defaultTrue := true
		return ProjectProfileListItem{Name: value, Value: value, Confidence: 0.65, RequiresEvidence: &defaultTrue}
	case map[string]interface{}:
		name := pickStringFromMap(t, "name", "title", "label", "item")
		value := pickStringFromMap(t, "value", "content", "text", "summary", "description", "requirement_text", "requirement")
		if value == "" {
			value = name
		}
		if name == "" {
			name = value
		}
		missing := normalizeProjectProfileMissing(t["missing"])
		if isEmptyProjectProfileValue(value) && isEmptyProjectProfileValue(name) {
			missing = true
		}
		confidence := normalizeProjectProfileConfidence(t["confidence"])
		if confidence == 0 && !missing {
			confidence = 0.65
		}
		var reqEv *bool
		if evRaw, ok := t["requires_evidence"]; ok {
			if evBool, isBool := evRaw.(bool); isBool {
				reqEv = &evBool
			}
		}
		if reqEv == nil {
			defaultTrue := true
			reqEv = &defaultTrue
		}

		item := ProjectProfileListItem{
			Name:             strings.TrimSpace(name),
			Value:            strings.TrimSpace(value),
			SourceText:       pickStringFromMap(t, "source_text", "evidence", "quote", "source", "excerpt"),
			SourceLocation:   pickStringFromMap(t, "source_location", "location", "page", "source_section", "section"),
			Confidence:       confidence,
			Missing:          missing,
			Notes:            pickStringFromMap(t, "notes", "note", "reason", "comment"),
			RequiresEvidence: reqEv,
		}
		if item.Missing {
			item.Confidence = 0
			if strings.TrimSpace(item.Notes) == "" {
				item.Notes = "未提取"
			}
		}
		return item
	default:
		value := strings.TrimSpace(fmt.Sprint(t))
		if isEmptyProjectProfileValue(value) {
			return ProjectProfileListItem{Missing: true, Notes: "未提取"}
		}
		defaultTrue := true
		return ProjectProfileListItem{Name: value, Value: value, Confidence: 0.65, RequiresEvidence: &defaultTrue}
	}
}

func splitProjectProfileListText(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '\n':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeProjectProfileList(raw interface{}) []ProjectProfileListItem {
	if raw == nil {
		return []ProjectProfileListItem{}
	}
	items := []ProjectProfileListItem{}
	switch t := raw.(type) {
	case []interface{}:
		for _, item := range t {
			normalized := normalizeProjectProfileListItem(item)
			if !normalized.Missing || strings.TrimSpace(normalized.Name) != "" || strings.TrimSpace(normalized.Value) != "" {
				items = append(items, normalized)
			}
		}
	case map[string]interface{}:
		for _, key := range []string{"items", "list", "values", "data"} {
			if nested, ok := lookupProjectProfileValue(t, key); ok {
				return normalizeProjectProfileList(nested)
			}
		}
		normalized := normalizeProjectProfileListItem(t)
		if !normalized.Missing || strings.TrimSpace(normalized.Name) != "" || strings.TrimSpace(normalized.Value) != "" {
			items = append(items, normalized)
		}
	case string:
		text := strings.TrimSpace(t)
		if text == "" || isEmptyProjectProfileValue(text) {
			return []ProjectProfileListItem{}
		}
		for _, part := range splitProjectProfileListText(text) {
			items = append(items, ProjectProfileListItem{Name: part, Value: part, Confidence: 0.65})
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(t))
		if text != "" && !isEmptyProjectProfileValue(text) {
			items = append(items, ProjectProfileListItem{Name: text, Value: text, Confidence: 0.65})
		}
	}
	return mergeProjectProfileLists(nil, items)
}

func normalizeProjectProfileField(raw interface{}) ProjectProfileField {
	switch t := raw.(type) {
	case nil:
		return newMissingProjectProfileField("")
	case string:
		value := strings.TrimSpace(t)
		if isEmptyProjectProfileValue(value) {
			return newMissingProjectProfileField("")
		}
		return ProjectProfileField{Value: value, Confidence: 0.65}
	case []interface{}:
		items := normalizeProjectProfileList(t)
		if len(items) == 0 {
			return newMissingProjectProfileField("")
		}
		parts := make([]string, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(item.Value) != "" {
				parts = append(parts, item.Value)
			}
		}
		if len(parts) == 0 {
			return newMissingProjectProfileField("")
		}
		return ProjectProfileField{Value: strings.Join(parts, "；"), Confidence: 0.65}
	case map[string]interface{}:
		value := pickStringFromMap(t, "value", "content", "text", "summary", "name", "title", "description")
		if value == "" {
			if nested, ok := lookupProjectProfileValue(t, "items", "list", "values"); ok {
				return normalizeProjectProfileField(nested)
			}
		}
		missing := normalizeProjectProfileMissing(t["missing"])
		if isEmptyProjectProfileValue(value) {
			value = ""
			missing = true
		}
		confidence := normalizeProjectProfileConfidence(t["confidence"])
		if confidence == 0 && !missing {
			confidence = 0.65
		}
		field := ProjectProfileField{
			Value:          strings.TrimSpace(value),
			SourceText:     pickStringFromMap(t, "source_text", "evidence", "quote", "source", "excerpt"),
			SourceLocation: pickStringFromMap(t, "source_location", "location", "page", "source_section", "section"),
			Confidence:     confidence,
			Missing:        missing,
			Notes:          pickStringFromMap(t, "notes", "note", "reason", "comment"),
		}
		if field.Missing {
			field.Confidence = 0
			if strings.TrimSpace(field.Notes) == "" {
				field.Notes = "未提取"
			}
		}
		return field
	default:
		value := strings.TrimSpace(fmt.Sprint(t))
		if isEmptyProjectProfileValue(value) {
			return newMissingProjectProfileField("")
		}
		return ProjectProfileField{Value: value, Confidence: 0.65}
	}
}

func newProjectProfileAuditItem(name, value, note string) ProjectProfileListItem {
	if strings.TrimSpace(value) == "" {
		value = name
	}
	return ProjectProfileListItem{
		Name:       strings.TrimSpace(name),
		Value:      strings.TrimSpace(value),
		Confidence: 1,
		Missing:    false,
		Notes:      strings.TrimSpace(note),
	}
}

func projectProfileListIdentity(item ProjectProfileListItem) string {
	name := strings.TrimSpace(strings.ToLower(item.Name))
	value := strings.TrimSpace(strings.ToLower(item.Value))
	if name == "" {
		name = value
	}
	if value == "" {
		value = name
	}
	return name + "|" + value
}

func mergeProjectProfileListItem(current, candidate ProjectProfileListItem) ProjectProfileListItem {
	if current.Missing && !candidate.Missing {
		return candidate
	}
	if candidate.Missing {
		return current
	}
	if current.Missing {
		return candidate
	}
	if candidate.Confidence > current.Confidence {
		current.Confidence = candidate.Confidence
	}
	current.SourceText = appendSourceEvidence(current.SourceText, candidate.SourceText, "；")
	current.SourceLocation = appendSourceEvidence(current.SourceLocation, candidate.SourceLocation, "；")
	if len(candidate.Value) > len(current.Value) {
		current.Value = candidate.Value
	}
	if len(candidate.Name) > len(current.Name) {
		current.Name = candidate.Name
	}
	current.Notes = appendSourceEvidence(current.Notes, candidate.Notes, "；")

	if current.RequiresEvidence == nil && candidate.RequiresEvidence != nil {
		current.RequiresEvidence = candidate.RequiresEvidence
	} else if current.RequiresEvidence != nil && candidate.RequiresEvidence != nil {
		if *current.RequiresEvidence || *candidate.RequiresEvidence {
			defaultTrue := true
			current.RequiresEvidence = &defaultTrue
		} else {
			defaultFalse := false
			current.RequiresEvidence = &defaultFalse
		}
	}

	return current
}

func mergeProjectProfileLists(current, candidate []ProjectProfileListItem) []ProjectProfileListItem {
	merged := make([]ProjectProfileListItem, 0, len(current)+len(candidate))
	indexByKey := map[string]int{}
	appendItem := func(item ProjectProfileListItem) {
		key := projectProfileListIdentity(item)
		if key == "|" {
			return
		}
		if idx, ok := indexByKey[key]; ok {
			merged[idx] = mergeProjectProfileListItem(merged[idx], item)
			return
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	for _, item := range current {
		appendItem(item)
	}
	for _, item := range candidate {
		appendItem(item)
	}
	return merged
}

func projectProfileComparableValue(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.ToLower(value))), "")
}

func projectProfileFieldScore(field ProjectProfileField) float64 {
	if field.Missing || isEmptyProjectProfileValue(field.Value) {
		return 0
	}
	score := 1.0 + field.Confidence
	if strings.TrimSpace(field.SourceText) != "" {
		score += 0.3
	}
	loc := strings.TrimSpace(field.SourceLocation)
	if loc != "" {
		score += 0.2
		if regexp.MustCompile(`第[一二三四五六七八九十\d]+章`).MatchString(loc) {
			score += 0.15
		}
	}
	return score
}

// appendSourceEvidence merges two evidence strings by deduplicating fragments.
func appendSourceEvidence(current, candidate, sep string) string {
	if strings.TrimSpace(candidate) == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return candidate
	}
	seen := make(map[string]struct{})
	var parts []string
	for _, s := range strings.Split(current, sep) {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			parts = append(parts, t)
		}
	}
	for _, s := range strings.Split(candidate, sep) {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			parts = append(parts, t)
		}
	}
	result := strings.Join(parts, sep)
	if len(result) > 2000 {
		result = result[:1997] + "..."
	}
	return result
}

// isHighRiskField returns true for fields where candidate preservation matters.
func isHighRiskField(label string) bool {
	switch label {
	case "采购边界", "工期节点", "资格要求", "评标方法", "总工期", "评分项", "废标规则":
		return true
	}
	return false
}

func projectProfileFieldToAuditItem(label string, field ProjectProfileField, note string) ProjectProfileListItem {
	return ProjectProfileListItem{
		Name:           label,
		Value:          field.Value,
		SourceText:     field.SourceText,
		SourceLocation: field.SourceLocation,
		Confidence:     field.Confidence,
		Missing:        false,
		Notes:          strings.TrimSpace(note),
	}
}

func mergeProjectProfileField(label string, current, candidate ProjectProfileField, uncertain *[]ProjectProfileListItem) (ProjectProfileField, MergeDiffEntry) {
	diff := MergeDiffEntry{FieldLabel: label}

	if current.Missing && !candidate.Missing {
		diff.Action = "replace_empty"
		diff.NewValue = candidate.Value
		diff.Reason = "当前为空，采用候选值"
		return candidate, diff
	}
	if candidate.Missing {
		diff.Action = "skip_empty"
		diff.Reason = "候选值为空，保留当前值"
		return current, diff
	}
	if current.Missing {
		diff.Action = "replace_empty"
		diff.NewValue = candidate.Value
		diff.Reason = "当前为空，采用候选值"
		return candidate, diff
	}
	currentValue := projectProfileComparableValue(current.Value)
	candidateValue := projectProfileComparableValue(candidate.Value)

	// Values are semantically identical — keep the longer raw form, append evidence
	if currentValue == candidateValue {
		if len(candidate.Value) > len(current.Value) {
			current.Value = candidate.Value
		}
		if candidate.Confidence > current.Confidence {
			current.Confidence = candidate.Confidence
		}
		current.SourceText = appendSourceEvidence(current.SourceText, candidate.SourceText, "；")
		current.SourceLocation = appendSourceEvidence(current.SourceLocation, candidate.SourceLocation, "；")
		current.Notes = appendSourceEvidence(current.Notes, candidate.Notes, "；")
		diff.Action = "append_evidence"
		diff.OldValue = current.Value
		diff.Reason = "值相同，合并证据"
		return current, diff
	}

	// Values differ — use score-based selection
	currentScore := projectProfileFieldScore(current)
	candidateScore := projectProfileFieldScore(candidate)
	diff.ScoreOld = currentScore
	diff.ScoreNew = candidateScore
	diff.OldValue = current.Value
	diff.NewValue = candidate.Value

	// Clear winner: candidate significantly better
	if candidateScore > currentScore+0.15 {
		if uncertain != nil {
			*uncertain = mergeProjectProfileLists(*uncertain, []ProjectProfileListItem{projectProfileFieldToAuditItem(label, current, "与其他分块结果存在冲突，当前值未被采纳")})
		}
		candidate.SourceText = appendSourceEvidence(candidate.SourceText, current.SourceText, "；")
		candidate.SourceLocation = appendSourceEvidence(candidate.SourceLocation, current.SourceLocation, "；")
		diff.Action = "replace_better"
		diff.Reason = fmt.Sprintf("候选值评分更高 (%.2f > %.2f)", candidateScore, currentScore)
		return candidate, diff
	}

	// Clear winner: current significantly better
	if currentScore > candidateScore+0.15 {
		if uncertain != nil {
			*uncertain = mergeProjectProfileLists(*uncertain, []ProjectProfileListItem{projectProfileFieldToAuditItem(label, candidate, "与其他分块结果存在冲突，当前值未被采纳")})
		}
		current.SourceText = appendSourceEvidence(current.SourceText, candidate.SourceText, "；")
		current.SourceLocation = appendSourceEvidence(current.SourceLocation, candidate.SourceLocation, "；")
		diff.Action = "keep_existing"
		diff.Reason = fmt.Sprintf("当前值评分更高 (%.2f > %.2f)", currentScore, candidateScore)
		return current, diff
	}

	// Scores are close — record both as uncertain
	if uncertain != nil {
		*uncertain = mergeProjectProfileLists(*uncertain, []ProjectProfileListItem{
			projectProfileFieldToAuditItem(label, current, "多分块抽取结果接近，建议人工确认"),
			projectProfileFieldToAuditItem(label, candidate, "多分块抽取结果接近，建议人工确认"),
		})
	}

	// For high-risk fields, preserve the loser as a candidate value
	var winner, loser ProjectProfileField
	if candidateScore >= currentScore {
		winner, loser = candidate, current
	} else {
		winner, loser = current, candidate
	}
	if isHighRiskField(label) && strings.TrimSpace(loser.Value) != "" {
		winner.Candidates = append(winner.Candidates, loser.Value)
	}
	winner.SourceText = appendSourceEvidence(winner.SourceText, loser.SourceText, "；")
	winner.SourceLocation = appendSourceEvidence(winner.SourceLocation, loser.SourceLocation, "；")
	diff.Action = "close_score"
	diff.Reason = fmt.Sprintf("评分接近 (%.2f vs %.2f)，取较高者", currentScore, candidateScore)
	return winner, diff
}

func projectProfileFieldAffirmative(field ProjectProfileField) bool {
	normalized := strings.TrimSpace(strings.ToLower(field.Value))
	return normalized == "是" || normalized == "true" || normalized == "yes" || normalized == "1"
}

func NewEmptyProjectProfileResult() ProjectProfileResult {
	return ProjectProfileResult{
		ProjectBaseInfo: ProjectBaseInfoProfile{
			ProjectName:          newMissingProjectProfileField("项目名称未提取"),
			OwnerUnit:            newMissingProjectProfileField("招标单位未提取"),
			Location:             newMissingProjectProfileField("工程地点未提取"),
			CategoryAndScope:     newMissingProjectProfileField("施工范围未提取"),
			DurationRequirements: newMissingProjectProfileField("工期要求未提取"),
			QualityStandard:      newMissingProjectProfileField("质量标准未提取"),
		},
		ConstructionCoreRequirements: ConstructionCoreRequirementsProfile{
			MaterialEquipmentRules:  newMissingProjectProfileField("材料设备要求未提取"),
			TechnicalSpecifications: newMissingProjectProfileField("施工技术规范未提取"),
			SiteManagement:          newMissingProjectProfileField("现场管理要求未提取"),
			AcceptanceRequirements:  newMissingProjectProfileField("验收要求未提取"),
			SpecialOperations:       newMissingProjectProfileField("专项作业要求未提取"),
			ProcurementBoundary:     newMissingProjectProfileField("采购边界未提取"),
			OwnerSuppliedItems:      []ProjectProfileListItem{},
			ContractorSuppliedItems: []ProjectProfileListItem{},
			ScheduleConstraints:     newMissingProjectProfileField("工期节点约束未提取"),
		},
		BidderRequirements: BidderRequirementsProfile{
			QualificationCertificates:  newMissingProjectProfileField("资质证书要求未提取"),
			PerformanceRequirements:    []ProjectProfileListItem{},
			PersonnelRequirements:      []ProjectProfileListItem{},
			FinancialRequirements:      []ProjectProfileListItem{},
			CreditRequirements:         []ProjectProfileListItem{},
			BonusItems:                 []ProjectProfileListItem{},
			QualificationRequirements:  []ProjectProfileListItem{},
			OtherMandatoryRequirements: []ProjectProfileListItem{},
		},
		EvaluationAndPerformanceRules: EvaluationAndPerformanceRulesProfile{
			MethodAndScoreWeights:         newMissingProjectProfileField("评标方法未提取"),
			TechnicalEvaluationDimensions: newMissingProjectProfileField("技术标评分维度未提取"),
			PaymentMethod:                 newMissingProjectProfileField("支付说明未提取"),
			SettlementRules:               newMissingProjectProfileField("结算规则未提取"),
			ScoringItems:                  []ProjectProfileListItem{},
			DisqualificationRules:         []ProjectProfileListItem{},
			TotalDuration:                 newMissingProjectProfileField("总工期未提取"),
		},
		ExtractionGaps:       []ProjectProfileListItem{},
		UncertainItems:       []ProjectProfileListItem{},
		RequiresManualReview: newBooleanProjectProfileField(false, ""),
	}
}

// keywordAuditFieldCheck maps a field label to a function checking if it's deficient.
type keywordAuditFieldCheck struct {
	Label       string
	IsDeficient func(p *ProjectProfileResult) bool
}

// keywordAuditGroup defines a group of keywords and associated profile fields.
type keywordAuditGroup struct {
	Name     string
	Keywords []string
	Fields   []keywordAuditFieldCheck
}

func getKeywordAuditGroups() []keywordAuditGroup {
	fieldDeficient := func(f ProjectProfileField) bool {
		return f.Missing || isEmptyProjectProfileValue(f.Value)
	}
	listDeficient := func(items []ProjectProfileListItem) bool {
		return len(items) == 0
	}
	return []keywordAuditGroup{
		{
			Name:     "采购边界",
			Keywords: []string{"甲供", "乙供", "招标人采购", "中标人采购", "自行采购", "供货范围", "责任界面"},
			Fields: []keywordAuditFieldCheck{
				{"采购边界", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.ConstructionCoreRequirements.ProcurementBoundary)
				}},
				{"甲供项", func(p *ProjectProfileResult) bool {
					return listDeficient(p.ConstructionCoreRequirements.OwnerSuppliedItems)
				}},
				{"乙供项", func(p *ProjectProfileResult) bool {
					return listDeficient(p.ConstructionCoreRequirements.ContractorSuppliedItems)
				}},
			},
		},
		{
			Name:     "工期",
			Keywords: []string{"工期", "节点", "里程碑", "完工", "交付", "延误", "奖罚"},
			Fields: []keywordAuditFieldCheck{
				{"工期要求", func(p *ProjectProfileResult) bool { return fieldDeficient(p.ProjectBaseInfo.DurationRequirements) }},
				{"工期节点", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.ConstructionCoreRequirements.ScheduleConstraints)
				}},
				{"总工期", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.EvaluationAndPerformanceRules.TotalDuration)
				}},
			},
		},
		{
			Name:     "质量",
			Keywords: []string{"验收", "检验", "试验", "规范", "标准", "合格率"},
			Fields: []keywordAuditFieldCheck{
				{"质量标准", func(p *ProjectProfileResult) bool { return fieldDeficient(p.ProjectBaseInfo.QualityStandard) }},
				{"验收要求", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.ConstructionCoreRequirements.AcceptanceRequirements)
				}},
			},
		},
		{
			Name:     "安全",
			Keywords: []string{"安全", "文明施工", "环保", "危大工程", "专项方案"},
			Fields: []keywordAuditFieldCheck{
				{"专项作业", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.ConstructionCoreRequirements.SpecialOperations)
				}},
				{"现场管理", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.ConstructionCoreRequirements.SiteManagement)
				}},
			},
		},
		{
			Name:     "评分废标",
			Keywords: []string{"评分", "加分", "扣分", "否决", "废标", "不得分", "必须响应"},
			Fields: []keywordAuditFieldCheck{
				{"评标方法", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.EvaluationAndPerformanceRules.MethodAndScoreWeights)
				}},
				{"评分项", func(p *ProjectProfileResult) bool { return listDeficient(p.EvaluationAndPerformanceRules.ScoringItems) }},
				{"废标规则", func(p *ProjectProfileResult) bool {
					return listDeficient(p.EvaluationAndPerformanceRules.DisqualificationRules)
				}},
			},
		},
		{
			Name:     "资格人员",
			Keywords: []string{"资质", "证书", "业绩", "项目经理", "技术负责人"},
			Fields: []keywordAuditFieldCheck{
				{"资格要求", func(p *ProjectProfileResult) bool {
					return listDeficient(p.BidderRequirements.QualificationRequirements)
				}},
				{"业绩要求", func(p *ProjectProfileResult) bool {
					return listDeficient(p.BidderRequirements.PerformanceRequirements)
				}},
				{"人员要求", func(p *ProjectProfileResult) bool { return listDeficient(p.BidderRequirements.PersonnelRequirements) }},
				{"资质证书", func(p *ProjectProfileResult) bool {
					return fieldDeficient(p.BidderRequirements.QualificationCertificates)
				}},
			},
		},
	}
}

// keywordAuditCheck scans rawText for keywords and flags fields that are empty despite keyword presence.
func keywordAuditCheck(rawText string, profile *ProjectProfileResult) []KeywordAuditHit {
	if rawText == "" {
		return nil
	}
	lines := strings.Split(rawText, "\n")
	groups := getKeywordAuditGroups()

	// Append industry-specific audit groups if industry is detected
	detectedIndustry := DetectIndustry(rawText)
	if industryGroups := GetIndustryAuditGroups(detectedIndustry); len(industryGroups) > 0 {
		groups = append(groups, industryGroups...)
	}

	var hits []KeywordAuditHit

	for _, group := range groups {
		// Count keyword hits in the document
		hitKeywords := make(map[string]int)
		totalHits := 0
		for _, line := range lines {
			for _, kw := range group.Keywords {
				if strings.Contains(line, kw) {
					hitKeywords[kw]++
					totalHits++
				}
			}
		}
		if totalHits == 0 {
			continue
		}

		// Check which associated fields are deficient
		var deficientLabels []string
		for _, fc := range group.Fields {
			if fc.IsDeficient(profile) {
				deficientLabels = append(deficientLabels, fc.Label)
			}
		}
		if len(deficientLabels) == 0 {
			continue
		}

		// Determine severity
		severity := "warning"
		uniqueKeywordCount := len(hitKeywords)
		if uniqueKeywordCount >= 2 && len(deficientLabels) == len(group.Fields) {
			severity = "error"
		}

		var matchedKWs []string
		for kw := range hitKeywords {
			matchedKWs = append(matchedKWs, kw)
		}

		hits = append(hits, KeywordAuditHit{
			Group:       group.Name,
			Keywords:    matchedKWs,
			FieldLabels: deficientLabels,
			Severity:    severity,
			HitCount:    totalHits,
		})
	}
	return hits
}

// boostIndustryLowConfidence checks low-confidence fields against industry dictionary keywords.
// If the field value contains industry-relevant terms, confidence is boosted by 0.1.
func boostIndustryLowConfidence(rawText string, profile *ProjectProfileResult) {
	industry := DetectIndustry(rawText)
	if industry == "" {
		return
	}
	var allKeywords []string
	for _, dict := range GetIndustryDictionaries() {
		if dict.Name == industry {
			for _, kws := range dict.Keywords {
				allKeywords = append(allKeywords, kws...)
			}
			break
		}
	}
	if len(allKeywords) == 0 {
		return
	}

	boostIfMatch := func(field *ProjectProfileField) {
		if field.Missing || isEmptyProjectProfileValue(field.Value) {
			return
		}
		if field.Confidence <= 0 || field.Confidence >= 0.6 {
			return
		}
		val := field.Value
		for _, kw := range allKeywords {
			if strings.Contains(val, kw) {
				field.Confidence += 0.1
				if field.Notes != "" {
					field.Notes += "；"
				}
				field.Notes += "行业词典匹配，置信度已提升"
				return
			}
		}
	}

	boostIfMatch(&profile.ConstructionCoreRequirements.TechnicalSpecifications)
	boostIfMatch(&profile.ConstructionCoreRequirements.SpecialOperations)
	boostIfMatch(&profile.ConstructionCoreRequirements.MaterialEquipmentRules)
	boostIfMatch(&profile.ConstructionCoreRequirements.SiteManagement)
	boostIfMatch(&profile.ConstructionCoreRequirements.AcceptanceRequirements)
	boostIfMatch(&profile.BidderRequirements.QualificationCertificates)
}

func (s *TenderDigitizationService) finalizeProjectProfileResult(profile *ProjectProfileResult, rawText ...string) {
	appendGapIfMissing := func(label string, field ProjectProfileField) {
		if field.Missing || isEmptyProjectProfileValue(field.Value) {
			profile.ExtractionGaps = mergeProjectProfileLists(profile.ExtractionGaps, []ProjectProfileListItem{newProjectProfileAuditItem(label, label, "字段缺失")})
		}
	}
	appendUncertainIfLowConfidence := func(label string, field ProjectProfileField) {
		if !field.Missing && field.Confidence > 0 && field.Confidence < 0.6 {
			profile.UncertainItems = mergeProjectProfileLists(profile.UncertainItems, []ProjectProfileListItem{projectProfileFieldToAuditItem(label, field, "置信度偏低，建议人工确认")})
		}
	}
	appendGapIfEmptyList := func(label string, items []ProjectProfileListItem) {
		if len(items) == 0 {
			profile.ExtractionGaps = mergeProjectProfileLists(profile.ExtractionGaps, []ProjectProfileListItem{newProjectProfileAuditItem(label, label, "列表项未提取")})
		}
	}

	appendGapIfMissing("项目名称", profile.ProjectBaseInfo.ProjectName)
	appendGapIfMissing("招标单位", profile.ProjectBaseInfo.OwnerUnit)
	appendGapIfMissing("工程地点", profile.ProjectBaseInfo.Location)
	appendGapIfMissing("施工范围", profile.ProjectBaseInfo.CategoryAndScope)
	appendGapIfMissing("工期要求", profile.ProjectBaseInfo.DurationRequirements)
	appendGapIfMissing("质量标准", profile.ProjectBaseInfo.QualityStandard)
	appendGapIfMissing("采购边界", profile.ConstructionCoreRequirements.ProcurementBoundary)
	appendGapIfMissing("工期节点", profile.ConstructionCoreRequirements.ScheduleConstraints)
	appendGapIfEmptyList("资格要求", profile.BidderRequirements.QualificationRequirements)
	appendGapIfMissing("评标方法", profile.EvaluationAndPerformanceRules.MethodAndScoreWeights)
	appendGapIfMissing("技术标评分维度", profile.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions)
	appendGapIfMissing("总工期", profile.EvaluationAndPerformanceRules.TotalDuration)
	appendGapIfEmptyList("甲供项", profile.ConstructionCoreRequirements.OwnerSuppliedItems)
	appendGapIfEmptyList("乙供项", profile.ConstructionCoreRequirements.ContractorSuppliedItems)
	appendGapIfEmptyList("人员要求", profile.BidderRequirements.PersonnelRequirements)
	appendGapIfEmptyList("评分项", profile.EvaluationAndPerformanceRules.ScoringItems)
	appendGapIfEmptyList("废标规则", profile.EvaluationAndPerformanceRules.DisqualificationRules)

	appendUncertainIfLowConfidence("项目名称", profile.ProjectBaseInfo.ProjectName)
	appendUncertainIfLowConfidence("招标单位", profile.ProjectBaseInfo.OwnerUnit)
	appendUncertainIfLowConfidence("工程地点", profile.ProjectBaseInfo.Location)
	appendUncertainIfLowConfidence("施工范围", profile.ProjectBaseInfo.CategoryAndScope)
	appendUncertainIfLowConfidence("工期要求", profile.ProjectBaseInfo.DurationRequirements)
	appendUncertainIfLowConfidence("采购边界", profile.ConstructionCoreRequirements.ProcurementBoundary)
	appendUncertainIfLowConfidence("工期节点", profile.ConstructionCoreRequirements.ScheduleConstraints)
	// QualificationRequirements is []ProjectProfileListItem, skip single-field uncertainty check
	appendUncertainIfLowConfidence("评标方法", profile.EvaluationAndPerformanceRules.MethodAndScoreWeights)
	appendUncertainIfLowConfidence("总工期", profile.EvaluationAndPerformanceRules.TotalDuration)

	// Industry dictionary boost: if industry detected, boost low-confidence fields that contain industry keywords
	if len(rawText) > 0 && rawText[0] != "" {
		boostIndustryLowConfidence(rawText[0], profile)
	}

	// Keyword audit: scan raw text for keywords and flag empty fields
	if len(rawText) > 0 && rawText[0] != "" {
		auditHits := keywordAuditCheck(rawText[0], profile)
		if len(auditHits) > 0 {
			profile.KeywordAuditHits = auditHits
			for _, hit := range auditHits {
				gapNote := fmt.Sprintf("关键词反查：原文出现「%s」相关关键词 %d 次，但 [%s] 字段为空",
					hit.Group, hit.HitCount, strings.Join(hit.FieldLabels, "、"))
				profile.ExtractionGaps = mergeProjectProfileLists(profile.ExtractionGaps, []ProjectProfileListItem{
					newProjectProfileAuditItem("关键词反查:"+hit.Group, hit.Group, gapNote),
				})
			}
		}
	}

	// Rule engine: scan raw text with regex rules and integrate hits
	if len(rawText) > 0 && rawText[0] != "" {
		ruleHits := RunRuleEngine(rawText[0], profile)
		if len(ruleHits) > 0 {
			profile.RuleEngineHits = ruleHits
			ApplyRuleEngineHits(ruleHits, profile)
		}
	}

	manualReview := projectProfileFieldAffirmative(profile.RequiresManualReview) || len(profile.ExtractionGaps) > 0 || len(profile.UncertainItems) > 0 || len(profile.KeywordAuditHits) > 0 || len(profile.RuleEngineHits) > 0
	note := strings.TrimSpace(profile.RequiresManualReview.Notes)
	if manualReview && note == "" {
		note = "存在缺失字段、低置信度或分块冲突，建议人工复核。"
	}
	if !manualReview {
		note = ""
	}
	profile.RequiresManualReview = newBooleanProjectProfileField(manualReview, note)
}

// FinalizeWithKeywordAudit runs keyword audit on a profile that has already been finalized.
// It only adds keyword audit hits and updates manual review status if needed.
func (s *TenderDigitizationService) FinalizeWithKeywordAudit(profile *ProjectProfileResult, rawText string) {
	if rawText == "" {
		return
	}
	auditHits := keywordAuditCheck(rawText, profile)
	if len(auditHits) == 0 {
		return
	}
	profile.KeywordAuditHits = auditHits
	for _, hit := range auditHits {
		gapNote := fmt.Sprintf("关键词反查：原文出现「%s」相关关键词 %d 次，但 [%s] 字段为空",
			hit.Group, hit.HitCount, strings.Join(hit.FieldLabels, "、"))
		profile.ExtractionGaps = mergeProjectProfileLists(profile.ExtractionGaps, []ProjectProfileListItem{
			newProjectProfileAuditItem("关键词反查:"+hit.Group, hit.Group, gapNote),
		})
	}
	// Update manual review if keyword audit found issues
	if !projectProfileFieldAffirmative(profile.RequiresManualReview) {
		note := "关键词反查发现可能遗漏，建议人工复核。"
		profile.RequiresManualReview = newBooleanProjectProfileField(true, note)
	}
}

func (s *TenderDigitizationService) NormalizeProjectProfileChunk(raw string) ProjectProfileResult {
	result := NewEmptyProjectProfileResult()
	cleanJSON := s.extractJSON(raw)
	var payload interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		result.ExtractionGaps = mergeProjectProfileLists(result.ExtractionGaps, []ProjectProfileListItem{newProjectProfileAuditItem("chunk_parse_failed", "chunk_parse_failed", "模型输出无法解析为 JSON")})
		result.RequiresManualReview = newBooleanProjectProfileField(true, "模型输出无法解析，建议人工复核。")
		return result
	}

	var root map[string]interface{}
	switch v := payload.(type) {
	case map[string]interface{}:
		root = unwrapProjectProfileRoot(v)
	case []interface{}:
		if len(v) == 1 {
			if m, ok := v[0].(map[string]interface{}); ok {
				root = unwrapProjectProfileRoot(m)
			}
		}
	}
	if root == nil {
		result.ExtractionGaps = mergeProjectProfileLists(result.ExtractionGaps, []ProjectProfileListItem{newProjectProfileAuditItem("chunk_schema_invalid", "chunk_schema_invalid", "模型输出根结构不符合预期")})
		result.RequiresManualReview = newBooleanProjectProfileField(true, "模型输出根结构不符合预期，建议人工复核。")
		return result
	}

	projectBaseInfo := coerceProjectProfileMap(pickProjectProfileRaw(root, root, "project_base_info"))
	constructionCore := coerceProjectProfileMap(pickProjectProfileRaw(root, root, "construction_core_requirements"))
	bidderRequirements := coerceProjectProfileMap(pickProjectProfileRaw(root, root, "bidder_requirements"))
	evaluationRules := coerceProjectProfileMap(pickProjectProfileRaw(root, root, "evaluation_and_performance_rules"))

	templateBoundaryMap := coerceProjectProfileMap(pickProjectProfileRaw(root, root, "format_template_boundary", "bid_format_template", "template_boundary"))
	if templateBoundaryMap != nil {
		detectedRaw, _ := templateBoundaryMap["detected"]
		startRaw, _ := templateBoundaryMap["start_page"]
		endRaw, _ := templateBoundaryMap["end_page"]
		sourceTextRaw, _ := templateBoundaryMap["source_text"]
		notesRaw, _ := templateBoundaryMap["notes"]

		if detectedBool, ok := detectedRaw.(bool); ok {
			result.FormatTemplateBoundary.Detected = detectedBool
		} else {
			result.FormatTemplateBoundary.Detected = true
		}
		result.FormatTemplateBoundary.StartPage = int(flexibleFloat64(startRaw))
		result.FormatTemplateBoundary.EndPage = int(flexibleFloat64(endRaw))

		if sourceTextRaw != nil {
			result.FormatTemplateBoundary.SourceText = fmt.Sprint(sourceTextRaw)
		}
		if notesRaw != nil {
			result.FormatTemplateBoundary.Notes = fmt.Sprint(notesRaw)
		}
	}

	result.ProjectBaseInfo.ProjectName = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "project_name", "name", "project"))
	result.ProjectBaseInfo.OwnerUnit = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "owner_unit", "tender_unit", "owner", "招标单位"))
	result.ProjectBaseInfo.Location = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "location", "project_location", "construction_location", "工程地点"))
	result.ProjectBaseInfo.CategoryAndScope = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "category_and_scope", "scope", "project_scope", "construction_scope", "施工范围"))
	result.ProjectBaseInfo.DurationRequirements = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "duration_requirements", "duration", "schedule_requirements", "工期要求"))
	result.ProjectBaseInfo.QualityStandard = normalizeProjectProfileField(pickProjectProfileRaw(projectBaseInfo, root, "quality_standard", "quality_requirements", "质量标准"))

	result.ConstructionCoreRequirements.MaterialEquipmentRules = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "material_equipment_rules", "material_equipment", "materials", "材料设备要求"))
	result.ConstructionCoreRequirements.TechnicalSpecifications = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "technical_specifications", "technical_requirements", "技术规范"))
	result.ConstructionCoreRequirements.SiteManagement = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "site_management", "site_management_requirements", "现场管理要求"))
	result.ConstructionCoreRequirements.AcceptanceRequirements = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "acceptance_requirements", "acceptance", "验收要求"))
	result.ConstructionCoreRequirements.SpecialOperations = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "special_operations", "special_operation_requirements", "专项作业要求"))
	result.ConstructionCoreRequirements.ProcurementBoundary = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "procurement_boundary", "采购边界"))
	result.ConstructionCoreRequirements.OwnerSuppliedItems = normalizeProjectProfileList(pickProjectProfileRaw(constructionCore, root, "owner_supplied_items", "owner_supplied_materials", "甲供项", "甲供材料"))
	result.ConstructionCoreRequirements.ContractorSuppliedItems = normalizeProjectProfileList(pickProjectProfileRaw(constructionCore, root, "contractor_supplied_items", "contractor_supplied_materials", "乙供项", "乙供材料"))
	result.ConstructionCoreRequirements.ScheduleConstraints = normalizeProjectProfileField(pickProjectProfileRaw(constructionCore, root, "schedule_constraints", "schedule_nodes", "milestone_requirements", "工期节点"))

	result.BidderRequirements.QualificationCertificates = normalizeProjectProfileField(pickProjectProfileRaw(bidderRequirements, root, "qualification_certificates", "qualification_certificate", "资质证书"))
	result.BidderRequirements.PerformanceRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "performance_requirements", "performance_requirement", "业绩要求"))
	result.BidderRequirements.PersonnelRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "personnel_requirements", "key_personnel", "人员要求"))
	result.BidderRequirements.FinancialRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "financial_requirements", "financials", "财务要求"))
	result.BidderRequirements.CreditRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "credit_requirements", "credits", "信誉要求"))
	result.BidderRequirements.BonusItems = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "bonus_items", "bonus_points", "加分项"))
	result.BidderRequirements.QualificationRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "qualification_requirements", "qualifications", "资格要求"))
	result.BidderRequirements.OtherMandatoryRequirements = normalizeProjectProfileList(pickProjectProfileRaw(bidderRequirements, root, "other_mandatory_requirements", "other_requirements", "其它关联要求"))

	result.EvaluationAndPerformanceRules.MethodAndScoreWeights = normalizeProjectProfileField(pickProjectProfileRaw(evaluationRules, root, "method_and_score_weights", "evaluation_method", "bid_evaluation_method", "评标方法"))
	result.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions = normalizeProjectProfileField(pickProjectProfileRaw(evaluationRules, root, "technical_evaluation_dimensions", "technical_scoring_dimensions", "技术标评分维度"))
	result.EvaluationAndPerformanceRules.PaymentMethod = normalizeProjectProfileField(pickProjectProfileRaw(evaluationRules, root, "payment_method", "payment_terms", "支付方式"))
	result.EvaluationAndPerformanceRules.SettlementRules = normalizeProjectProfileField(pickProjectProfileRaw(evaluationRules, root, "settlement_rules", "settlement_rule", "结算规则"))
	result.EvaluationAndPerformanceRules.ScoringItems = normalizeProjectProfileList(pickProjectProfileRaw(evaluationRules, root, "scoring_items", "score_items", "评分项"))
	result.EvaluationAndPerformanceRules.DisqualificationRules = normalizeProjectProfileList(pickProjectProfileRaw(evaluationRules, root, "disqualification_rules", "invalid_bid_rules", "废标规则"))
	result.EvaluationAndPerformanceRules.TotalDuration = normalizeProjectProfileField(pickProjectProfileRaw(evaluationRules, root, "total_duration", "duration", "total_schedule", "总工期"))

	result.ExtractionGaps = mergeProjectProfileLists(result.ExtractionGaps, normalizeProjectProfileList(pickProjectProfileRaw(root, root, "extraction_gaps", "missing_items")))
	result.UncertainItems = mergeProjectProfileLists(result.UncertainItems, normalizeProjectProfileList(pickProjectProfileRaw(root, root, "uncertain_items", "review_items", "conflicts")))
	if rawManualReview := pickProjectProfileRaw(root, root, "requires_manual_review", "manual_review_required"); rawManualReview != nil {
		result.RequiresManualReview = normalizeProjectProfileField(rawManualReview)
	}

	s.finalizeProjectProfileResult(&result)
	return result
}

func (s *TenderDigitizationService) MergeProjectProfileChunks(chunks []ProjectProfileResult) (ProjectProfileResult, []MergeDiffEntry) {
	merged := NewEmptyProjectProfileResult()
	var allDiffs []MergeDiffEntry

	for chunkIdx, chunk := range chunks {
		collectDiff := func(field *ProjectProfileField, label string, candidate ProjectProfileField) {
			result, diff := mergeProjectProfileField(label, *field, candidate, &merged.UncertainItems)
			diff.ChunkIndex = chunkIdx
			if diff.Action != "skip_empty" {
				allDiffs = append(allDiffs, diff)
			}
			*field = result
		}

		collectDiff(&merged.ProjectBaseInfo.ProjectName, "项目名称", chunk.ProjectBaseInfo.ProjectName)
		collectDiff(&merged.ProjectBaseInfo.OwnerUnit, "招标单位", chunk.ProjectBaseInfo.OwnerUnit)
		collectDiff(&merged.ProjectBaseInfo.Location, "工程地点", chunk.ProjectBaseInfo.Location)
		collectDiff(&merged.ProjectBaseInfo.CategoryAndScope, "施工范围", chunk.ProjectBaseInfo.CategoryAndScope)
		collectDiff(&merged.ProjectBaseInfo.DurationRequirements, "工期要求", chunk.ProjectBaseInfo.DurationRequirements)
		collectDiff(&merged.ProjectBaseInfo.QualityStandard, "质量标准", chunk.ProjectBaseInfo.QualityStandard)

		collectDiff(&merged.ConstructionCoreRequirements.MaterialEquipmentRules, "材料设备要求", chunk.ConstructionCoreRequirements.MaterialEquipmentRules)
		collectDiff(&merged.ConstructionCoreRequirements.TechnicalSpecifications, "施工技术规范", chunk.ConstructionCoreRequirements.TechnicalSpecifications)
		collectDiff(&merged.ConstructionCoreRequirements.SiteManagement, "现场管理要求", chunk.ConstructionCoreRequirements.SiteManagement)
		collectDiff(&merged.ConstructionCoreRequirements.AcceptanceRequirements, "验收要求", chunk.ConstructionCoreRequirements.AcceptanceRequirements)
		collectDiff(&merged.ConstructionCoreRequirements.SpecialOperations, "专项作业要求", chunk.ConstructionCoreRequirements.SpecialOperations)
		collectDiff(&merged.ConstructionCoreRequirements.ProcurementBoundary, "采购边界", chunk.ConstructionCoreRequirements.ProcurementBoundary)
		merged.ConstructionCoreRequirements.OwnerSuppliedItems = mergeProjectProfileLists(merged.ConstructionCoreRequirements.OwnerSuppliedItems, chunk.ConstructionCoreRequirements.OwnerSuppliedItems)
		merged.ConstructionCoreRequirements.ContractorSuppliedItems = mergeProjectProfileLists(merged.ConstructionCoreRequirements.ContractorSuppliedItems, chunk.ConstructionCoreRequirements.ContractorSuppliedItems)
		collectDiff(&merged.ConstructionCoreRequirements.ScheduleConstraints, "工期节点", chunk.ConstructionCoreRequirements.ScheduleConstraints)

		collectDiff(&merged.BidderRequirements.QualificationCertificates, "资质证书", chunk.BidderRequirements.QualificationCertificates)
		merged.BidderRequirements.PerformanceRequirements = mergeProjectProfileLists(merged.BidderRequirements.PerformanceRequirements, chunk.BidderRequirements.PerformanceRequirements)
		merged.BidderRequirements.PersonnelRequirements = mergeProjectProfileLists(merged.BidderRequirements.PersonnelRequirements, chunk.BidderRequirements.PersonnelRequirements)
		merged.BidderRequirements.FinancialRequirements = mergeProjectProfileLists(merged.BidderRequirements.FinancialRequirements, chunk.BidderRequirements.FinancialRequirements)
		merged.BidderRequirements.CreditRequirements = mergeProjectProfileLists(merged.BidderRequirements.CreditRequirements, chunk.BidderRequirements.CreditRequirements)
		merged.BidderRequirements.BonusItems = mergeProjectProfileLists(merged.BidderRequirements.BonusItems, chunk.BidderRequirements.BonusItems)
		merged.BidderRequirements.QualificationRequirements = mergeProjectProfileLists(merged.BidderRequirements.QualificationRequirements, chunk.BidderRequirements.QualificationRequirements)
		merged.BidderRequirements.OtherMandatoryRequirements = mergeProjectProfileLists(merged.BidderRequirements.OtherMandatoryRequirements, chunk.BidderRequirements.OtherMandatoryRequirements)

		collectDiff(&merged.EvaluationAndPerformanceRules.MethodAndScoreWeights, "评标方法", chunk.EvaluationAndPerformanceRules.MethodAndScoreWeights)
		collectDiff(&merged.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions, "技术标评分维度", chunk.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions)
		collectDiff(&merged.EvaluationAndPerformanceRules.PaymentMethod, "支付说明", chunk.EvaluationAndPerformanceRules.PaymentMethod)
		collectDiff(&merged.EvaluationAndPerformanceRules.SettlementRules, "结算规则", chunk.EvaluationAndPerformanceRules.SettlementRules)
		merged.EvaluationAndPerformanceRules.ScoringItems = mergeProjectProfileLists(merged.EvaluationAndPerformanceRules.ScoringItems, chunk.EvaluationAndPerformanceRules.ScoringItems)
		merged.EvaluationAndPerformanceRules.DisqualificationRules = mergeProjectProfileLists(merged.EvaluationAndPerformanceRules.DisqualificationRules, chunk.EvaluationAndPerformanceRules.DisqualificationRules)
		collectDiff(&merged.EvaluationAndPerformanceRules.TotalDuration, "总工期", chunk.EvaluationAndPerformanceRules.TotalDuration)

		if chunk.FormatTemplateBoundary.Detected {
			if !merged.FormatTemplateBoundary.Detected || (chunk.FormatTemplateBoundary.EndPage-chunk.FormatTemplateBoundary.StartPage > merged.FormatTemplateBoundary.EndPage-merged.FormatTemplateBoundary.StartPage) {
				merged.FormatTemplateBoundary = chunk.FormatTemplateBoundary
			}
		}

		merged.ExtractionGaps = mergeProjectProfileLists(merged.ExtractionGaps, chunk.ExtractionGaps)
		merged.UncertainItems = mergeProjectProfileLists(merged.UncertainItems, chunk.UncertainItems)
	}
	s.finalizeProjectProfileResult(&merged)
	return merged, allDiffs
}

func (s *TenderDigitizationService) BuildProjectProfileExtractionMeta(profile ProjectProfileResult) map[string]interface{} {
	fields := []ProjectProfileField{
		profile.ProjectBaseInfo.ProjectName,
		profile.ProjectBaseInfo.OwnerUnit,
		profile.ProjectBaseInfo.Location,
		profile.ProjectBaseInfo.CategoryAndScope,
		profile.ProjectBaseInfo.DurationRequirements,
		profile.ProjectBaseInfo.QualityStandard,
		profile.ConstructionCoreRequirements.MaterialEquipmentRules,
		profile.ConstructionCoreRequirements.TechnicalSpecifications,
		profile.ConstructionCoreRequirements.SiteManagement,
		profile.ConstructionCoreRequirements.AcceptanceRequirements,
		profile.ConstructionCoreRequirements.SpecialOperations,
		profile.ConstructionCoreRequirements.ProcurementBoundary,
		profile.ConstructionCoreRequirements.ScheduleConstraints,
		profile.BidderRequirements.QualificationCertificates,
		profile.EvaluationAndPerformanceRules.MethodAndScoreWeights,
		profile.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions,
		profile.EvaluationAndPerformanceRules.PaymentMethod,
		profile.EvaluationAndPerformanceRules.SettlementRules,
		profile.EvaluationAndPerformanceRules.TotalDuration,
	}
	missingCount := 0
	lowConfidenceCount := 0
	for _, field := range fields {
		if field.Missing || isEmptyProjectProfileValue(field.Value) {
			missingCount++
			continue
		}
		if field.Confidence > 0 && field.Confidence < 0.6 {
			lowConfidenceCount++
		}
	}
	return map[string]interface{}{
		"schema_version":       "v2",
		"field_total":          len(fields),
		"field_missing":        missingCount,
		"low_confidence_count": lowConfidenceCount,
		"gaps_count":           len(profile.ExtractionGaps),
		"uncertain_count":      len(profile.UncertainItems),
		"manual_review":        projectProfileFieldAffirmative(profile.RequiresManualReview),
		"rule_engine_hits":     len(profile.RuleEngineHits),
		"array_item_counts": map[string]int{
			"owner_supplied_items":      len(profile.ConstructionCoreRequirements.OwnerSuppliedItems),
			"contractor_supplied_items": len(profile.ConstructionCoreRequirements.ContractorSuppliedItems),
			"personnel_requirements":    len(profile.BidderRequirements.PersonnelRequirements),
			"financial_requirements":    len(profile.BidderRequirements.FinancialRequirements),
			"credit_requirements":       len(profile.BidderRequirements.CreditRequirements),
			"bonus_items":               len(profile.BidderRequirements.BonusItems),
			"scoring_items":             len(profile.EvaluationAndPerformanceRules.ScoringItems),
			"disqualification_rules":    len(profile.EvaluationAndPerformanceRules.DisqualificationRules),
		},
	}
}

// BuildProfileViews generates 7 specialized views from a profile for frontend consumption.
func BuildProfileViews(profile ProjectProfileResult) []map[string]interface{} {
	type viewField struct {
		Label string      `json:"label"`
		Field interface{} `json:"field"`
	}
	type viewList struct {
		Label string                   `json:"label"`
		Items []ProjectProfileListItem `json:"items"`
	}

	buildView := func(key, label string, fields []viewField, lists []viewList) map[string]interface{} {
		total := len(fields) + len(lists)
		filled := 0
		for _, f := range fields {
			if pf, ok := f.Field.(ProjectProfileField); ok {
				if !pf.Missing && !isEmptyProjectProfileValue(pf.Value) {
					filled++
				}
			}
		}
		for _, l := range lists {
			if len(l.Items) > 0 {
				filled++
			}
		}
		completeness := 0.0
		if total > 0 {
			completeness = float64(filled) / float64(total)
		}
		return map[string]interface{}{
			"view_key":     key,
			"view_label":   label,
			"fields":       fields,
			"lists":        lists,
			"completeness": completeness,
		}
	}

	p := profile
	return []map[string]interface{}{
		buildView("overview", "总画像",
			[]viewField{
				{"项目名称", p.ProjectBaseInfo.ProjectName},
				{"招标单位", p.ProjectBaseInfo.OwnerUnit},
				{"工程地点", p.ProjectBaseInfo.Location},
				{"施工范围", p.ProjectBaseInfo.CategoryAndScope},
				{"工期要求", p.ProjectBaseInfo.DurationRequirements},
				{"质量标准", p.ProjectBaseInfo.QualityStandard},
			}, nil),
		buildView("material_equipment", "材料设备",
			[]viewField{
				{"材料设备要求", p.ConstructionCoreRequirements.MaterialEquipmentRules},
				{"采购边界", p.ConstructionCoreRequirements.ProcurementBoundary},
			},
			[]viewList{
				{"甲供项", p.ConstructionCoreRequirements.OwnerSuppliedItems},
				{"乙供项", p.ConstructionCoreRequirements.ContractorSuppliedItems},
			}),
		buildView("schedule", "工期",
			[]viewField{
				{"工期要求", p.ProjectBaseInfo.DurationRequirements},
				{"工期节点", p.ConstructionCoreRequirements.ScheduleConstraints},
				{"总工期", p.EvaluationAndPerformanceRules.TotalDuration},
			}, nil),
		buildView("quality", "质量",
			[]viewField{
				{"质量标准", p.ProjectBaseInfo.QualityStandard},
				{"验收要求", p.ConstructionCoreRequirements.AcceptanceRequirements},
				{"施工技术要求", p.ConstructionCoreRequirements.TechnicalSpecifications},
			}, nil),
		buildView("safety", "安全文明",
			[]viewField{
				{"专项作业", p.ConstructionCoreRequirements.SpecialOperations},
				{"现场管理", p.ConstructionCoreRequirements.SiteManagement},
			}, nil),
		buildView("scoring", "评分",
			[]viewField{
				{"评标方法", p.EvaluationAndPerformanceRules.MethodAndScoreWeights},
				{"技术评分维度", p.EvaluationAndPerformanceRules.TechnicalEvaluationDimensions},
				{"支付方式", p.EvaluationAndPerformanceRules.PaymentMethod},
				{"结算规则", p.EvaluationAndPerformanceRules.SettlementRules},
			},
			[]viewList{
				{"评分项", p.EvaluationAndPerformanceRules.ScoringItems},
				{"废标规则", p.EvaluationAndPerformanceRules.DisqualificationRules},
				{"加分项", p.BidderRequirements.BonusItems},
			}),
		buildView("risk", "风险",
			[]viewField{
				{"人工复核", p.RequiresManualReview},
			},
			[]viewList{
				{"缺失项", p.ExtractionGaps},
				{"不确定项", p.UncertainItems},
			}),
	}
}

func (s *TenderDigitizationService) DigitizeTenderFile(ctx context.Context, fileID string, content string, mode string, onProgress func(percent int)) (*ProjectProfileDigitizationResult, error) {
	log.Printf("[Digitize] Starting digitization for file: %s", fileID)

	chunkSize := 12000
	overlap := 1500
	chunks := s.splitText(content, chunkSize, overlap)
	log.Printf("[Digitize] Split text into %d chunks", len(chunks))

	// Classify each chunk type
	if onProgress != nil {
		onProgress(8)
	}
	chunkTypes := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTypes[i] = classifyChunkType(chunk)
		log.Printf("[Digitize] Chunk %d/%d classified as: %s", i+1, len(chunks), chunkTypes[i])
	}

	if onProgress != nil {
		onProgress(15)
	}

	// Detect industry from full content
	detectedIndustry := DetectIndustry(content)
	if detectedIndustry != "" {
		log.Printf("[Digitize] Detected industry: %s", detectedIndustry)
	}
	industryHint := GetIndustryPromptHint(detectedIndustry)

	chunkOutputs := make([]string, len(chunks))
	promptInputs := make([]string, len(chunks))
	normalizedResults := make([]ProjectProfileResult, len(chunks))
	var wg sync.WaitGroup
	errCh := make(chan error, len(chunks))
	sem := make(chan struct{}, 4)
	var completedChunks int32
	var mu sync.Mutex

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, text string, cType string, indHint string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			log.Printf("[Digitize] Processing chunk %d/%d (type: %s)", idx+1, len(chunks), cType)

			promptBody, sysPrompt := s.getExtractionPromptByChunkType(cType, mode)
			if indHint != "" {
				promptBody = indHint + "\n\n" + promptBody
			}
			prompt := fmt.Sprintf("%s\n待分析文本段:\n\"\"\"\n%s\n\"\"\"", promptBody, text)

			messages := []LLMMessage{
				{Role: "system", Content: sysPrompt},
				{Role: "user", Content: prompt},
			}
			promptInputs[idx] = prompt
			res, err := s.aiClient.CallLLMWithContext(ctx, messages, 0.1)
			if err != nil {
				errCh <- fmt.Errorf("chunk %d failed: %w", idx+1, err)
				return
			}
			chunkOutputs[idx] = res
			normalizedResults[idx] = s.NormalizeProjectProfileChunk(res)

			mu.Lock()
			completedChunks++
			if onProgress != nil {
				// Chunk processing covers 20% to 85%
				percent := 20 + int(float64(completedChunks)/float64(len(chunks))*65)
				onProgress(percent)
			}
			mu.Unlock()
		}(i, chunk, chunkTypes[i], industryHint)
	}

	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		return nil, <-errCh
	}

	if onProgress != nil {
		onProgress(88)
	}

	log.Printf("[Digitize] All chunks processed, merging results")
	if onProgress != nil {
		onProgress(92)
	}
	mergedProfile, mergeDiffs := s.MergeProjectProfileChunks(normalizedResults)
	processedAt := time.Now().Format(time.RFC3339)
	return &ProjectProfileDigitizationResult{
		RawText:          content,
		Chunks:           chunks,
		ChunkTypes:       chunkTypes,
		ChunkOutputs:     chunkOutputs,
		PromptInputs:     promptInputs,
		ChunkResults:     normalizedResults,
		MergedProfile:    mergedProfile,
		MergeDiffs:       mergeDiffs,
		DetectedIndustry: detectedIndustry,
		ProcessedAt:      processedAt,
	}, nil
}

// ExtractOutlineFacts performs segmented multi-view extraction from a tender document.
func (s *TenderDigitizationService) ExtractOutlineFacts(ctx context.Context, projectID string, tenderContent string) (*FactExtractResult, error) {
	log.Printf("[Digitize] Layer 1: Starting segmented multi-view extraction for project: %s", projectID)

	// Step 1: Get Project Global Context (Module A)
	globalCtxCtx, cancelGlobalCtx := context.WithTimeout(ctx, 60*time.Second)
	globalCtx, err := s.GetProjectGlobalContext(globalCtxCtx, tenderContent, "")
	cancelGlobalCtx()
	if err != nil {
		log.Printf("[Digitize] Warning: Failed to get global context: %v", err)
		globalCtx = &ProjectGlobalContext{ProjectName: "未知项目"}
	}
	globalCtxJSON, _ := json.Marshal(globalCtx)

	// Step 2: Split Document to Chapters (Module A)
	chapters := s.SplitDocumentToChapters(tenderContent)
	log.Printf("[Digitize] Split document into %d chapters", len(chapters))

	// Step 3: Multi-View Extraction per Chapter (Module B)
	// Views: Audit, Scoring, Technical, Cost. Sequence: audit -> scoring -> technical -> cost
	views := []string{"audit_view", "scoring_view", "technical_view", "cost_view"}

	type chapterTask struct {
		chapter TenderChapter
		view    string
	}

	taskCh := make(chan chapterTask, len(chapters)*len(views))
	resultCh := make(chan FactExtractResult, len(chapters)*len(views))

	var wg sync.WaitGroup
	workerCount := 3 // 降低并发，避免 API 限流
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				res, err := s.extractFactsFromChapter(ctx, projectID, string(globalCtxJSON), task.chapter, task.view)
				if err != nil {
					// 5. 失败兜底: Record error but continue other views
					log.Printf("[FactExtraction] Chapter %s View %s failed: %v", task.chapter.Title, task.view, err)
					continue
				}
				if res != nil {
					resultCh <- *res
				}
			}
		}()
	}

	for _, ch := range chapters {
		for _, view := range views {
			taskCh <- chapterTask{chapter: ch, view: view}
		}
	}
	close(taskCh)
	wg.Wait()
	close(resultCh)

	// Step 4: Merge & Deduplicate (Layer 2)
	var allExtractedResults []FactExtractResult
	successCount := 0
	emptyCount := 0
	for res := range resultCh {
		if len(res.ScoreItems) > 0 || len(res.MandatorySpecs) > 0 || len(res.ProjectCharacteristics) > 0 || len(res.SpecialTopics) > 0 {
			successCount++
		} else {
			emptyCount++
		}
		allExtractedResults = append(allExtractedResults, res)
	}

	log.Printf("[Digitize] Analysis summary: %d chap/view tasks returned items, %d returned empty results", successCount, emptyCount)

	if len(allExtractedResults) == 0 {
		return nil, fmt.Errorf("no facts extracted from any chapter/view")
	}

	finalFacts := s.MergeAndDeduplicateFacts(allExtractedResults)
	s.normalizeFactExtractResult(finalFacts)

	log.Printf("[Digitize] Layer 2: Fact extraction completed. Total items: %d Score, %d Mandatory, %d Proc, %d Special",
		len(finalFacts.ScoreItems), len(finalFacts.MandatorySpecs), len(finalFacts.ProjectCharacteristics), len(finalFacts.SpecialTopics))

	if len(finalFacts.ScoreItems) == 0 && len(finalFacts.MandatorySpecs) == 0 && len(finalFacts.ProjectCharacteristics) == 0 && len(finalFacts.SpecialTopics) == 0 {
		return nil, fmt.Errorf("事实提取阶段产出为 0，请检查章节切分是否太细或模型输出是否异常")
	}

	return finalFacts, nil
}

func (s *TenderDigitizationService) extractFactsFromChapter(ctx context.Context, projectID, globalCtxJSON string, chapter TenderChapter, view string) (*FactExtractResult, error) {
	// 1. 接入调用逻辑: Get prompt from DB
	promptKey := "tech_bid_fact_extraction_" + view
	promptBody, sysPrompt := s.promptService.GetPromptFull(promptKey)
	if promptBody == "" && sysPrompt == "" {
		// Log and fail-safe return empty instead of error to keep other views running
		log.Printf("[FactExtraction] Warning: Prompt template not found or empty for key: %s", promptKey)
		return &FactExtractResult{}, nil
	}

	dynamicInstruction := fmt.Sprintf("当前分析视角: 【%s】, 当前分析章节: 【%s】。请结合全局画像, 提取相关事实。%s", view, chapter.Title, promptBody)

	cacheContext := fmt.Sprintf("### 全局项目画像\n%s\n\n### 章节标题\n%s\n\n### 章节内容\n%s",
		globalCtxJSON, chapter.Title, chapter.Content)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt + "\n重要提示: 你必须严格返回 FactExtractResult JSON 结构。"),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	// 添加子上下文超时，防止单个章节提取阻塞整个流程
	subCtx, cancel := context.WithTimeout(ctx, 120*time.Second) // 增加超时到 120 秒
	defer cancel()
	res, _, err := s.aiClient.CallLLMV2WithContext(subCtx, messages, 0.1)
	if err != nil {
		return nil, err
	}

	var result FactExtractResult
	cleanJSON := s.extractJSON(res)

	// MEGA DEBUG: Log first 2 chapter/view combinations to see raw output
	if chapter.Index < 2 && (view == "audit_view" || view == "scoring_view") {
		log.Printf("[DEBUG_RAW_AI] Project %s Chapter %d View %s. Raw AI Body (truncated 1000): %s",
			projectID, chapter.Index, view, func(s string) string {
				if len(s) > 1000 {
					return s[:1000]
				}
				return s
			}(res))
		log.Printf("[DEBUG_CLEAN_JSON] Cleaned JSON for unmarshal: %s", func(s string) string {
			if len(s) > 500 {
				return s[:500]
			}
			return s
		}(cleanJSON))
	}

	// First pass: try direct unmarshal
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		// Second pass: try lenient unmarshal with key mapping if direct fails
		log.Printf("[FactExtraction] Strict JSON Unmarshal failed, attempting lenient coercion for %s", chapter.Title)
	}

	// Third pass (Robustness): If the structural fields are still empty, the model might have used synonyms.
	if len(result.ScoreItems) == 0 && len(result.MandatorySpecs) == 0 {
		var rawMap map[string]interface{}
		if err := json.Unmarshal([]byte(cleanJSON), &rawMap); err == nil {
			// Map synonyms like "points" or "score" to "score_items"
			s.coerceFactMapToResult(rawMap, &result)
		}
	}

	// 3. 保留视角标签 & 溯源信息
	tagger := func(items []FactItem) []FactItem {
		for i := range items {
			items[i].SourceChapter = chapter.Title
			items[i].ExtractedByView = view
			// If LLM didn't provide location/page, we could hint it here if we had precise mapping,
			// but for now we preserve what LLM extracted.
			if items[i].ID == "" {
				// Use fact code pattern to ensure uniqueness before deduplication
				items[i].ID = fmt.Sprintf("F-%s-%d", uuid.New().String()[:6], i)
			}
		}
		return items
	}
	result.ScoreItems = tagger(result.ScoreItems)
	result.MandatorySpecs = tagger(result.MandatorySpecs)
	result.ProjectCharacteristics = tagger(result.ProjectCharacteristics)
	result.SpecialTopics = tagger(result.SpecialTopics)

	return &result, nil
}

func (s *TenderDigitizationService) MergeAndDeduplicateFacts(allFacts []FactExtractResult) *FactExtractResult {
	final := &FactExtractResult{
		ScoreItems:             []FactItem{},
		MandatorySpecs:         []FactItem{},
		ProjectCharacteristics: []FactItem{},
		SpecialTopics:          []FactItem{},
	}

	merger := func(target *[]FactItem, source []FactItem) {
		seen := make(map[string]int) // local per-category seen map
		// First, populate seen map with existing items in target
		for i, it := range *target {
			key := it.Name + "|" + it.Content
			if len(it.Content) > 100 {
				key = it.Name + "|" + it.Content[:100]
			}
			seen[key] = i
		}

		for _, it := range source {
			contentKey := it.Content
			if len(contentKey) > 100 {
				contentKey = contentKey[:100]
			}
			key := fmt.Sprintf("%s|%s", it.Name, contentKey)

			if idx, exists := seen[key]; !exists {
				if it.EvidenceCount == 0 {
					it.EvidenceCount = 1
				}
				seen[key] = len(*target)
				*target = append(*target, it)
			} else {
				// Update existing fact
				(*target)[idx].EvidenceCount++
				// Append view source if not already present
				if it.ExtractedByView != "" && !strings.Contains((*target)[idx].ExtractedByView, it.ExtractedByView) {
					if (*target)[idx].ExtractedByView == "" {
						(*target)[idx].ExtractedByView = it.ExtractedByView
					} else {
						(*target)[idx].ExtractedByView += ", " + it.ExtractedByView
					}
				}
			}
		}
	}

	// 4. 合并去重: Process each result
	for _, f := range allFacts {
		merger(&final.ScoreItems, f.ScoreItems)
		merger(&final.MandatorySpecs, f.MandatorySpecs)
		merger(&final.ProjectCharacteristics, f.ProjectCharacteristics)
		merger(&final.SpecialTopics, f.SpecialTopics)
	}

	return final
}

// GenerateOutlineFromFacts generates outline using the legacy skeleton-dominant approach.
// DEPRECATED: This function is kept for backward compatibility only.
// It is only invoked when OutlineGenerationMode = "skeleton" (deprecated mode).
// For new projects, use GenerateOutlineDirectly() instead.
func (s *TenderDigitizationService) GenerateOutlineFromFacts(ctx context.Context, projectID string, facts *FactExtractResult, profileJSON string, routeName string, projectType string, profession string, mappings []FactOutlineMapping, structurePlanJSON string) ([]map[string]interface{}, error) {
	log.Printf("[Digitize] ⚠️ DEPRECATED: Generating outline from extracted facts for project: %s (Industry: %s)", projectID, projectType)

	skeleton := s.LoadIndustrySkeleton(projectType, profession)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_generation")
	if promptBody == "" {
		promptBody = `你是一个顶级技术标书架构师。请根据《行业骨架》《facts→目录映射表》和《核验事实》生成专业三级目录。

### 核心生成原则：
1. **骨架驱动**：只能在骨架定义的逻辑章与节池范围内组织内容，不得另起与行业无关。
2. **强映射（最高级指令）**：必须严格落实《facts→目录映射表》中的 target_path。映射表要求的 [章, 节, 小节] 必须在输出中完整体现，不得随意归并到“其他”或“主体内容”。
3. **节池强制性**：节标题（Level 2）必须从骨架定义的 UnitPool 中选择最贴切的项。
4. **禁止泛标题**：严禁在三级目录中使用「主体内容」「技术要求响应」「相关措施」「其他说明」「施工方案」「关键技术」等无实际业务语义的泛化词汇。标题必须包含具体工艺、设备、部位或业务名词。
5. **事实驱动**：每个小节必须包含至少一个 requirement_ids，且 ID 必须来源于事实库。
6. **高优先级独立性**：priority 为 high 的事实必须对应独立的小节。
7. **专业术语**：使用行业标准术语。

### 行业骨架 (Industry Skeleton)：
{{skeleton}}

### facts → 目录节点映射（必须落实）：
{{mappings}}

### 标题候选思路 (Title Candidates)：
{{title_pool}}

### 核验事实 (Facts JSON)：
{{facts}}

### 项目背景：
- 行业类型: {{projectType}}
- 施工路线: {{routeName}}
- 项目画像: {{profile}}`
	}

	if sysPrompt == "" {
		sysPrompt = "只返回 JSON 数组。根为章数组；每章含 units；每 unit 含 subsections；每个 subsection 必须含非空 requirement_ids 数组。禁止 chapters/title 等非标准根字段。"
	}

	factsJSON, _ := json.Marshal(facts)
	skeletonJSON, _ := json.Marshal(skeleton.LogicalChapters)
	titlePoolJSON, _ := json.Marshal(skeleton.TitleCandidatePool)
	mappingsJSON, _ := json.Marshal(mappings)

	// --- Cacheable message construction ---
	cacheContext := fmt.Sprintf("### 行业骨架 (JSON)\n%s\n\n### 批准的弹性结构调整计划 (必须执行!!)\n%s\n\n### facts→目录映射\n%s\n\n### 标题候选思路\n%s\n\n### 核验事实 (JSON)\n%s",
		string(skeletonJSON), structurePlanJSON, string(mappingsJSON), string(titlePoolJSON), string(factsJSON))

	dynamicInstruction := fmt.Sprintf("请作为 %s 专家，严格执行已审批的【弹性结构调整计划】，在该结构框架内生成三级目录。确保落实映射表落点。根必须是 JSON 数组。不要带 Markdown 标记。", skeleton.IndustryName)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.3)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "generate_outline",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	outline, err := s.parseAndNormalizeOutlineJSON(cleanJSON)
	if err != nil {
		return nil, fmt.Errorf("目录生成结果解析失败: %w", err)
	}
	outline = s.EnforceRequirementIDsFromMappings(outline, mappings)
	outline = s.BackfillRequirementIDs(outline, facts)

	return outline, nil
}

func (s *TenderDigitizationService) ValidateOutlineStructure(outline []map[string]interface{}) error {
	// Re-marshal and unmarshal into validation DTO for strict checking
	b, _ := json.Marshal(outline)
	var typedOutline []AISection
	if err := json.Unmarshal(b, &typedOutline); err != nil {
		return fmt.Errorf("目录结构不完整 (章/节缺失): %v", err)
	}

	if len(typedOutline) == 0 {
		return fmt.Errorf("目录内容不能为空")
	}

	// 检查泛化标题
	forbidden := []string{"主体内容", "技术要求响应", "相关措施", "其他说明", "未命名小节", "未命名单元"}
	for _, ch := range typedOutline {
		for _, u := range ch.Units {
			for _, sub := range u.Subsections {
				for _, f := range forbidden {
					if strings.Contains(sub.Name, f) {
						return fmt.Errorf("生成结果包含违禁泛化标题: %s", sub.Name)
					}
				}
			}
		}
	}

	return s.validate.Var(typedOutline, "dive")
}

// parseAndNormalizeOutlineJSON unmarshals LLM output (often inconsistent keys), maps aliases to name/units/subsections, and validates.
func (s *TenderDigitizationService) parseAndNormalizeOutlineJSON(cleanJSON string) ([]map[string]interface{}, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &raw); err != nil {
		return nil, err
	}
	sections, err := s.coerceOutlineRoot(raw)
	if err != nil {
		return nil, err
	}
	out := s.normalizeOutlineCanonical(sections)
	if err := s.ValidateOutlineStructure(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *TenderDigitizationService) coerceOutlineRoot(raw interface{}) ([]map[string]interface{}, error) {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, it := range v {
			if m, ok := it.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("目录 JSON 中未找到有效的章对象")
		}
		return out, nil
	case map[string]interface{}:
		for _, key := range []string{"chapters", "outline", "sections", "data", "items", "目录"} {
			if arr, ok := v[key].([]interface{}); ok {
				return s.coerceOutlineRoot(arr)
			}
		}
		return nil, fmt.Errorf("目录 JSON 根对象中缺少 chapters/outline/sections 数组")
	default:
		return nil, fmt.Errorf("目录 JSON 根类型无效: %T", raw)
	}
}

func pickStringFromMap(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s != "" {
				return s
			}
		case float64:
			return fmt.Sprintf("%.0f", t)
		case json.Number:
			f, err := t.Float64()
			if err == nil {
				return fmt.Sprintf("%.0f", f)
			}
		}
	}
	return ""
}

func toInterfaceSlice(raw interface{}) []interface{} {
	if raw == nil {
		return nil
	}
	switch t := raw.(type) {
	case []interface{}:
		return t
	case map[string]interface{}:
		return []interface{}{t}
	default:
		return nil
	}
}

func (s *TenderDigitizationService) collectRequirementIDs(t map[string]interface{}) []string {
	raw := t["requirement_ids"]
	if raw == nil {
		raw = t["requirementIds"]
	}
	if raw == nil {
		raw = t["ids"]
	}
	if raw == nil {
		raw = t["fact_ids"]
	}
	var out []string
	switch r := raw.(type) {
	case []string:
		for _, v := range r {
			if strings.TrimSpace(v) != "" {
				out = append(out, strings.TrimSpace(v))
			}
		}
	case []interface{}:
		for _, x := range r {
			switch v := x.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					out = append(out, strings.TrimSpace(v))
				}
			case float64:
				out = append(out, fmt.Sprintf("%.0f", v))
			}
		}
	case string:
		if strings.TrimSpace(r) != "" {
			out = []string{strings.TrimSpace(r)}
		}
	}
	return out
}

func (s *TenderDigitizationService) normalizeSubsectionsSlice(u map[string]interface{}) []interface{} {
	raw := u["subsections"]
	if raw == nil {
		raw = u["subsection"]
	}
	if raw == nil {
		raw = u["items"]
	}
	if raw == nil {
		raw = u["children"]
	}
	if raw == nil {
		raw = u["小节"]
	}
	arr := toInterfaceSlice(raw)
	out := make([]interface{}, 0, len(arr))
	for _, it := range arr {
		switch t := it.(type) {
		case map[string]interface{}:
			nm := pickStringFromMap(t, "name", "title", "label", "小节标题")
			if nm == "" {
				nm = "未命名小节"
			}
			ids := s.collectRequirementIDs(t)
			out = append(out, map[string]interface{}{"name": nm, "requirement_ids": ids})
		case string:
			st := strings.TrimSpace(t)
			if st != "" {
				out = append(out, map[string]interface{}{"name": st, "requirement_ids": []string{}})
			}
		}
	}
	return out
}

func (s *TenderDigitizationService) normalizeUnitsSlice(raw interface{}) []interface{} {
	arr := toInterfaceSlice(raw)
	out := make([]interface{}, 0, len(arr))
	for _, it := range arr {
		u, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		uname := pickStringFromMap(u, "name", "title", "unit_name", "label", "节标题")
		if uname == "" {
			uname = "未命名单元"
		}
		subs := s.normalizeSubsectionsSlice(u)
		if len(subs) == 0 {
			subs = []interface{}{
				map[string]interface{}{"name": uname + "方案", "requirement_ids": []string{}},
			}
		}
		out = append(out, map[string]interface{}{
			"name":        uname,
			"subsections": subs,
		})
	}
	return out
}

func (s *TenderDigitizationService) normalizeOutlineCanonical(sections []map[string]interface{}) []map[string]interface{} {
	for i := range sections {
		sec := sections[i]
		name := pickStringFromMap(sec, "name", "title", "chapter_name", "chapter", "label", "章名", "章节名称")
		if name == "" {
			name = fmt.Sprintf("第%d章", i+1)
		}
		unitsRaw := sec["units"]
		if unitsRaw == nil {
			unitsRaw = sec["units_list"]
		}
		if unitsRaw == nil {
			unitsRaw = sec["children"]
		}
		if unitsRaw == nil {
			unitsRaw = sec["单元"]
		}
		units := s.normalizeUnitsSlice(unitsRaw)
		if len(units) == 0 {
			units = []interface{}{
				map[string]interface{}{
					"name": "主要施工内容",
					"subsections": []interface{}{
						map[string]interface{}{"name": name + "专项方案", "requirement_ids": []string{}},
					},
				},
			}
		}
		sections[i] = map[string]interface{}{
			"name":  name,
			"units": units,
		}
	}
	return sections
}

func unwrapAuditRoot(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	for _, k := range []string{"data", "result", "audit", "output", "response"} {
		inner, ok := m[k].(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := inner["coverage_score"]; ok {
			return inner
		}
		if _, ok := inner["audit_summary"]; ok {
			return inner
		}
		if _, ok := inner["missing_items"]; ok {
			return inner
		}
	}
	return m
}

func flexibleFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

// valueToAuditSummaryString coerces LLM output: audit_summary may be string, object, or array.
func valueToAuditSummaryString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	case map[string]interface{}:
		if s := pickStringFromMap(t, "text", "overview", "summary", "content", "conclusion", "description"); s != "" {
			return s
		}
		parts := []string{}
		for _, k := range []string{"title", "headline", "brief"} {
			if s := pickStringFromMap(t, k); s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " — ")
		}
		b, _ := json.Marshal(t)
		return string(b)
	case []interface{}:
		var parts []string
		for _, it := range t {
			parts = append(parts, strings.TrimSpace(valueToAuditSummaryString(it)))
		}
		return strings.TrimSpace(strings.Join(parts, "；"))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

func parseAuditIssueSlice(raw interface{}) []AuditIssue {
	arr, ok := raw.([]interface{})
	if !ok || raw == nil {
		return []AuditIssue{}
	}
	out := make([]AuditIssue, 0, len(arr))
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, AuditIssue{
			RequirementID:  pickStringFromMap(m, "requirement_id", "id", "requirementId"),
			Description:    pickStringFromMap(m, "description", "desc", "message", "text"),
			Priority:       pickStringFromMap(m, "priority", "level"),
			Reason:         pickStringFromMap(m, "reason", "cause"),
			ActionType:     pickStringFromMap(m, "action_type", "action"),
			TargetSection:  pickStringFromMap(m, "target_section", "section"),
			ExpectedEffect: pickStringFromMap(m, "expected_effect", "effect"),
		})
	}
	return out
}

func (s *TenderDigitizationService) parseCoverageAuditResultJSON(cleanJSON string) (*CoverageAuditResult, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &raw); err != nil {
		return nil, err
	}
	var root map[string]interface{}
	switch v := raw.(type) {
	case map[string]interface{}:
		root = v
	case []interface{}:
		if len(v) == 1 {
			if m, ok := v[0].(map[string]interface{}); ok {
				root = m
			}
		}
		if root == nil {
			return nil, fmt.Errorf("审计结果须为 JSON 对象或单元素对象数组")
		}
	default:
		return nil, fmt.Errorf("审计结果 JSON 根类型无效: %T", raw)
	}
	m := unwrapAuditRoot(root)
	if m == nil {
		return nil, fmt.Errorf("审计结果 JSON 根对象无效")
	}

	summary := valueToAuditSummaryString(m["audit_summary"])
	if summary == "" {
		summary = pickStringFromMap(m, "summary", "audit_summary_text", "结论", "审计结论")
	}
	score := flexibleFloat64(m["coverage_score"])
	if score == 0 && m["coverage_score"] == nil {
		score = flexibleFloat64(m["score"])
	}

	audit := &CoverageAuditResult{
		CoverageScore:  score,
		AuditSummary:   summary,
		MissingItems:   parseAuditIssueSlice(m["missing_items"]),
		WeakItems:      parseAuditIssueSlice(m["weak_items"]),
		DuplicateItems: parseAuditIssueSlice(m["duplicate_items"]),
	}
	if strings.TrimSpace(audit.AuditSummary) == "" {
		audit.AuditSummary = "目录与事实库对照完成（模型未提供文字摘要）。"
	}

	return audit, nil
}

func (s *TenderDigitizationService) AuditOutlineCoverage(ctx context.Context, projectID string, facts *FactExtractResult, outlineJSON string) (*CoverageAuditResult, error) {
	log.Printf("[Digitize] Auditing outline coverage for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_audit")
	if promptBody == "" {
		promptBody = `你是一个严谨的【事实核验审计官】。你的唯一任务是对比《核验事实库》与《当前生成目录》，找出事实覆盖层面的漏洞，并给出精准的“修补指令”。

### 审计职责：
1. **事实对齐**：逐条核对事实库中的得分项、废标项是否在目录中有所体现。
2. **缺口分析**：识别完全缺失的项 (Missing) 或响应程度不足的项 (Weak)。
3. **修补分诊**：针对每个 Gap，给出具体的 action_type (insert/expand/merge/reorder/rewrite) 和 target_section。

### 输入信息：
- 核验事实库: {{facts}}
- 目录结构 JSON: {{outline}}

### 输出要求：
必须严格以单个 JSON 对象格式返回：
{
  "coverage_score": 0-100的得分,
  "audit_summary": "简明扼要的审计结论",
  "missing_items": [
     {
       "requirement_id": "ID", 
       "description": "缺失描述", 
       "priority": "high/medium/low",
       "action_type": "insert/expand/merge/rewrite",
       "target_section": "建议操作的章/节名称",
       "reason": "审计理由",
       "expected_effect": "预期达到的合规效果"
     }
  ],
  "weak_items": [...],
  "duplicate_items": [...]
}`
	}

	if sysPrompt == "" {
		sysPrompt = "你是一个铁面无私的质量审计官。只找出漏洞并给出结构化修补指令，不要自行修复。"
	}

	factsJSON, _ := json.Marshal(facts)

	// --- Cacheable message construction (3-layer design) ---
	// Layer 1 (Stable): system prompt
	// Layer 2 (Stable): facts JSON (reused from extract step)
	// Layer 3 (Dynamic): outline to audit + audit instruction
	cacheContext := fmt.Sprintf("### 核验事实库\n%s", string(factsJSON))

	dynamicInstruction := fmt.Sprintf("### 目录结构 JSON\n%s\n\n请对比事实库与目录，找出缺失、薄弱和重复项。输出单个 JSON 对象：coverage_score(0-100 数字)、audit_summary(必须是字符串段落)、missing_items、weak_items、duplicate_items 为数组。audit_summary 不要写成对象或嵌套 JSON。", outlineJSON)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.1)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "audit_coverage",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	audit, err := s.parseCoverageAuditResultJSON(cleanJSON)
	if err != nil {
		return nil, fmt.Errorf("核验结果解析失败: %w", err)
	}

	if err := s.validate.Struct(audit); err != nil {
		return nil, fmt.Errorf("审计结果 Schema 校验未通过: %v", err)
	}

	return audit, nil
}

func (s *TenderDigitizationService) DecideNextFlow(audit *CoverageAuditResult) (string, string) {
	// Rule 1: Mandatory block if score is too low
	if audit.CoverageScore < 75 {
		return "BLOCK", "评分低于阈值 (75)，建议进行全局优化 (Optimize)"
	}

	// Rule 2: Automatic flow to Refine if score is in middle range
	if audit.CoverageScore >= 75 && audit.CoverageScore < 90 {
		return "REVISE", "评分良好但存在遗漏，建议进行补丁式修复 (Refine)"
	}

	// Rule 3: High priority missing items block Proceed even if score is high
	for _, item := range audit.MissingItems {
		if item.Priority == "high" {
			return "REVISE", fmt.Sprintf("存在高优先级缺失项 (%s)，必须修复后方可通过", item.Description)
		}
	}

	return "PASS", "质量达标，允许进入终审核验"
}

func (s *TenderDigitizationService) OptimizeOutlineByCoverage(ctx context.Context, projectID string, originalOutlineJSON string, audit *CoverageAuditResult, facts *FactExtractResult, mappings []FactOutlineMapping, full *FullRequirementResponseResult) ([]map[string]interface{}, error) {
	log.Printf("[Digitize] Optimizing outline based on structured audit for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_refine")
	if promptBody == "" {
		promptBody = `你是一个资深标书修订专家。请作为【定点修补器 (Patcher)】，根据审计出的《核心修补指令》，在《原始目录》中进行精准补全。

### 输入：
- 原始目录: {{outline}}
- 审计修补指令: {{audit}}
- 原始事实依据: {{facts}}
- Step4 完全响应率校验（硬门槛缺口）: {{full_response}}

### 核心修订原则：
1. **最小扰动**：严禁大规模重写已有结构。保持 80% 以上的目录章/节名称和顺序不变。
2. **定点修补**：严格按照 audit 中的 target_section 进行 insert 或 expand 操作；同时优先消除「完全响应」中的缺失 ID、仅挂标签、弱响应项。
3. **数据一致性**：确保补全的小节正确打上对应的 requirement_ids。
4. **防泛化**：补全的内容必须使用具体的行业工艺术语，不得使用“拟采取的措施”等空泛标题；对 only_tagged 项须将泛标题改为能体现 requirement 语义的具体标题。

### 输出：
返回修订后的完整三级目录 JSON 数组。`
	}

	if sysPrompt == "" {
		sysPrompt = "你是一个冷静的标书修补专家。严格遵循最小扰动原则，按指令定点修补，不破坏原有专业结构。"
	}

	auditJSON, _ := json.Marshal(audit)
	factsJSON, _ := json.Marshal(facts)
	fullJSON := []byte("{}")
	if full != nil {
		fullJSON, _ = json.Marshal(full)
	}

	// --- Cacheable message construction ---
	// Layer 1 (Stable): system prompt
	// Layer 2 (Stable): original outline + audit result + facts + full response gate
	// Layer 3 (Dynamic): refine instruction
	cacheContext := fmt.Sprintf("### 原始目录\n%s\n\n### 缺失/薄弱项\n%s\n\n### 原始事实依据\n%s\n\n### Step4 完全响应率校验（含 missing/only_tagged/weak）\n%s",
		originalOutlineJSON, string(auditJSON), string(factsJSON), string(fullJSON))

	dynamicInstruction := "请根据以上缺失项与完全响应缺口，在原始目录中进行精准补全。优先为缺失的 requirement_id 增补小节或改写泛标题。只插入和修正必要的部分，保持原有结构不变。严格返回 JSON 数组格式。"

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.2)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "optimize_outline",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	outline, err := s.parseAndNormalizeOutlineJSON(cleanJSON)
	if err != nil {
		return nil, fmt.Errorf("优化目录解析失败: %w", err)
	}
	outline = s.EnforceRequirementIDsFromMappings(outline, mappings)
	outline = s.BackfillRequirementIDs(outline, facts)

	return outline, nil
}

func (s *TenderDigitizationService) GenerateOutlineChapterDraft(ctx context.Context, projectID string, profileJSON string, profession string, projectType string) ([]string, error) {
	log.Printf("[Digitize] Phase A: Generating chapter draft for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_chapter_generation")
	if promptBody == "" {
		return nil, fmt.Errorf("未找到一级章生成 Prompt 模板 (tech_bid_outline_chapter_generation)")
	}

	cacheContext := fmt.Sprintf("### 项目画像\n%s\n\n### 行业/专业\n类型：%s，专业：%s", profileJSON, projectType, profession)
	log.Printf("[Digitize] Phase A: cacheContext len=%d, promptBody len=%d", len(cacheContext), len(promptBody))
	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(promptBody),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.3)
	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	var chapters []string
	if err := json.Unmarshal([]byte(cleanJSON), &chapters); err != nil {
		return nil, fmt.Errorf("解析一级章草案失败: %w", err)
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("一级章生成结果为空")
	}

	return chapters, nil
}

func (s *TenderDigitizationService) LightValidateChapters(chapters []string) (bool, string) {
	if len(chapters) < 5 {
		return false, "章节数量过少（建议不少于 8 章），可能遗漏核心管理或技术环节。"
	}

	// 核心章节子集（模糊匹配）
	mandatoryKeywords := []string{"编制依据", "工程概况", "进度", "质量", "安全", "施工方案"}
	missing := []string{}
	for _, kw := range mandatoryKeywords {
		found := false
		for _, ch := range chapters {
			if strings.Contains(ch, kw) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, kw)
		}
	}

	if len(missing) > 0 {
		return false, fmt.Sprintf("目录完整度较低，可能缺失以下维度：【%s】", strings.Join(missing, "、"))
	}

	return true, "一级章结构基本合理。"
}

func (s *TenderDigitizationService) ExpandOutlineStructure(ctx context.Context, projectID string, confirmedChapters []string, profileJSON string) ([]map[string]interface{}, error) {
	log.Printf("[Digitize] Phase B: Expanding structure for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_units_subsections_generation")
	if promptBody == "" {
		return nil, fmt.Errorf("未找到目录展开 Prompt 模板 (tech_bid_outline_units_subsections_generation)")
	}

	chaptersJSON, _ := json.Marshal(confirmedChapters)
	cacheContext := fmt.Sprintf("### 项目画像\n%s\n\n### 已确认一级章\n%s", profileJSON, string(chaptersJSON))

	instruction := strings.ReplaceAll(promptBody, "{{confirmed_chapters}}", string(chaptersJSON))

	log.Printf("[Digitize] Phase B: cacheContext len=%d, instruction len=%d", len(cacheContext), len(instruction))

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(instruction),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.3)
	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	return s.parseAndNormalizeOutlineJSON(cleanJSON)
}

func (s *TenderDigitizationService) extractJSON(res string) string {
	start := strings.Index(res, "{")
	startArr := strings.Index(res, "[")

	if start == -1 && startArr == -1 {
		return res
	}

	first := start
	if startArr != -1 && (start == -1 || startArr < start) {
		first = startArr
	}

	last := strings.LastIndex(res, "}")
	lastArr := strings.LastIndex(res, "]")

	finalLast := last
	if lastArr != -1 && (last == -1 || lastArr > last) {
		finalLast = lastArr
	}

	if first != -1 && finalLast != -1 && finalLast > first {
		return res[first : finalLast+1]
	}
	return res
}

func (s *TenderDigitizationService) VerifyChapterOutline(ctx context.Context, projectID string, facts *FactExtractResult, outlineJSON string, auditResult *CoverageAuditResult, profileJSON string, tenderContent string, apiKey, endpoint, model string, step4FullResponseJSON string) (*VerificationResult, error) {
	log.Printf("[Digitize] Final Verification by Expert Agent for project: %s (model: %s)", projectID, model)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_verify")
	if promptBody == "" {
		promptBody = `你是一个拥有票据否决权的【技术标终审专家】。你站在项目总工的高度，对当前标书目录的合规性与竞争力做最终裁决。

### 终审原则：
1. **基于审计报告**：深度结合 Step 4 产出的覆盖审计结果。若存在高优先级缺失项，务必审慎裁定。
2. **完全响应硬门槛**：必须参考 Step4 产出的「完全响应率」结果（missing / only_tagged / 质量分）。若该结果仍为 REVISE/BLOCK 级别缺口，终审不得轻易给 PASS。
3. **全局逻辑评估**：评价目录是否符合行业规范，是否能支撑后续高质量正文编写。
4. **裁决产出**：输出确定的终审结论（PASS/REVISE/BLOCK）及对应的风险等级。

### 审核输入：
- **核验事实库**：{{facts}}
- **Step 4 结构化审计报告**：{{audit}}
- **Step4 完全响应硬门槛（full_response_rate 等）**：{{full_response}}
- **当前目录结构 JSON**：{{outline}}
- **项目背景/画像**：{{profile}}

### 输出要求：
必须严格以 JSON 格式返回：
{
  "final_decision": "PASS | REVISE | BLOCK",
  "risk_level": "LOW | MEDIUM | HIGH",
  "summary": "专家终审意见摘要",
  "critical_issues": ["核心漏洞1", "核心漏洞2"],
  "major_issues": ["主要改进建议1", "等"],
  "suggested_actions": ["下一步落地的操作方案", "等"],
  "can_proceed": true | false
}`
	}

	if sysPrompt == "" {
		sysPrompt = "你是一个拥有票据否决权的终审专家。你的结论必须基于证据。"
	}

	factsJSON, _ := json.Marshal(facts)
	auditJSON, _ := json.Marshal(auditResult)
	fr := strings.TrimSpace(step4FullResponseJSON)
	if fr == "" {
		fr = "{}"
	}

	// --- Cacheable message construction (3-layer design) ---
	// Layer 1 (Stable): system prompt
	// Layer 2 (Stable): facts JSON + audit result + Step4 full response gate
	// Layer 3 (Dynamic): outline + profile + verification instruction
	cacheContext := fmt.Sprintf("### 核验事实库\n%s\n\n### Step 4 结构化审计报告\n%s\n\n### Step4 完全响应硬门槛（full_response_rate / missing / only_tagged）\n%s",
		string(factsJSON), string(auditJSON), fr)

	dynamicInstruction := fmt.Sprintf("### 当前目录结构 JSON\n%s\n\n### 项目背景/画像\n%s\n\n请对当前目录进行最终审核，严格以 JSON 格式返回终审结论。", outlineJSON, profileJSON)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithConfig(messages, 0.2, model, endpoint, apiKey)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "verify_chapter_outline",
			Model:                    model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	var result VerificationResult
	cleanJSON := s.extractJSON(res)
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("终审结果解析失败: %v", err)
	}

	// Schema Validation
	if err := s.validate.Struct(result); err != nil {
		return nil, fmt.Errorf("专家终审 Schema 校验未通过: %v", err)
	}

	return &result, nil
}

func (s *TenderDigitizationService) OptimizeChapterOutline(ctx context.Context, projectID string, originalOutlineJSON string, suggestions string, profileJSON string, tenderContent string) ([]map[string]interface{}, error) {
	log.Printf("[Digitize] Optimizing chapter outline for project: %s", projectID)

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_outline_optimize")
	if promptBody == "" {
		promptBody = `
你是一个顶级的技术标书专家。请根据以下《核验建议》对《原始目录大纲》进行深度优化和调整。

### 核心任务：
1. **整合建议**：将核验建议中提到的遗漏项、改进点完美融合进目录中。
2. **保持结构**：依然维持 章 -> 单元节 -> 小节 的三层级结构。
3. **深度响应**：确保目录能 100% 覆盖评分标准和技术规范。

### 输入信息：
- **项目画像**：{{profile}}
- **原始目录**：{{outline}}
- **核验与优化建议**：{{suggestions}}
- **招标文件参考**：{{content}}

### 返回格式：
必须返回优化后的完整 JSON 数组。
`
	}

	trimmedContent := tenderContent
	if len(tenderContent) > 50000 {
		trimmedContent = tenderContent[:50000] + "... [截断]"
	}

	if sysPrompt == "" {
		sysPrompt = "你是一个顶级标书架构师。请以 JSON 数组形式输出优化后的目录结构。"
	}

	// --- Cacheable message construction ---
	cacheContext := fmt.Sprintf("### 项目画像\n%s\n\n### 原始目录\n%s",
		profileJSON, originalOutlineJSON)

	dynamicInstruction := fmt.Sprintf("### 核验与优化建议\n%s\n\n### 招标文件参考\n%s\n\n请根据以上建议对原始目录进行深度优化，严格返回 JSON 数组格式。", suggestions, trimmedContent)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.2)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "optimize_chapter_outline",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	// Clean JSON
	cleanJSON := res
	if start := strings.Index(res, "["); start != -1 {
		if end := strings.LastIndex(res, "]"); end != -1 && end > start {
			cleanJSON = res[start : end+1]
		}
	}

	var outline []map[string]interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &outline); err != nil {
		return nil, fmt.Errorf("优化后的目录格式解析失败: %v", err)
	}

	return outline, nil
}

func (s *TenderDigitizationService) GenerateChapterContent(ctx context.Context, projectID string, chapterName string, profileJSON string, tenderContent string) (string, error) {
	log.Printf("[Digitize] Generating content for chapter: %s (project: %s, content size: %d)", chapterName, projectID, len(tenderContent))

	promptBody, sysPrompt := s.promptService.GetPromptFull("tech_bid_content_generation")
	if promptBody == "" {
		promptBody = "请根据项目画像和招标文件为指定章节编写高质量技术标正文内容。"
	}

	// 1. Filter relevant segments from the tender document to save tokens (limit to ~30k chars)
	trimmedContent := s.filterRelevantSegments(chapterName, tenderContent, 30000)

	if sysPrompt == "" {
		sysPrompt = `你是一个身经百战的建筑工程总工程师助理。请直接返回 Markdown 格式的技术标正文内容，不要包含任何「好的」「没问题」等垃圾话。`
	}

	// --- Cacheable message construction ---
	// Layer 1 (Stable): system prompt (shared across all chapters)
	// Layer 2 (Stable): project profile (shared across all chapters)
	// Layer 3 (Dynamic): chapter name + filtered tender content
	cacheContext := fmt.Sprintf("### 项目画像\n%s", profileJSON)

	dynamicInstruction := fmt.Sprintf("### 当前章节名称\n%s\n\n### 招标文件相关事实摘要\n%s\n\n请为以上章节编写完整的技术标正文内容。", chapterName, trimmedContent)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(sysPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.4)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "generate_chapter_content",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return "", err
	}

	return res, nil
}

func (s *TenderDigitizationService) filterRelevantSegments(chapterName string, fullContent string, limit int) string {
	if len(fullContent) <= limit {
		return fullContent
	}

	// Simple keyword extraction (including handling for Chinese)
	// We use a combination of splitting by common separators
	cleanName := strings.NewReplacer("/", " ", "-", " ", "（", " ", "）", " ", "(", " ", ")", " ").Replace(chapterName)
	keywords := strings.Fields(cleanName)

	// Split into segments (paragraphs)
	segments := strings.Split(fullContent, "\n")
	type scoredSegment struct {
		text  string
		score int
		index int
	}

	var scored []scoredSegment
	for i, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if len(trimmed) < 10 {
			continue
		}

		score := 0
		lowerSeg := strings.ToLower(trimmed)
		for _, kw := range keywords {
			if len(kw) < 1 {
				continue
			}
			if strings.Contains(lowerSeg, strings.ToLower(kw)) {
				score += 10
			}
		}
		// Boost for section headers (usually start with # or numbers)
		if score > 0 {
			if strings.HasPrefix(trimmed, "#") || (len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9') {
				score += 5
			}
			scored = append(scored, scoredSegment{trimmed, score, i})
		}
	}

	if len(scored) == 0 {
		// Fallback: If no matches, return the first part of the document (Overview/Basics)
		return fullContent[:limit]
	}

	// Sort by score? No, we should maintain document order for context coherence
	// But we only want the TOP relevant ones if there are too many.
	// For now, let's just pick segments with score > 0 until limit is reached.

	var result strings.Builder
	currentLen := 0

	// Add a prefix to let AI know this is filtered
	result.WriteString("> [提取自招标文件的相关片段]\n\n")

	for _, ss := range scored {
		if currentLen+len(ss.text) > limit {
			break
		}
		result.WriteString(ss.text)
		result.WriteString("\n\n")
		currentLen += len(ss.text)
	}

	return result.String()
}

func (s *TenderDigitizationService) splitText(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}
	var chunks []string
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
		start = end - overlap
	}
	return chunks
}

// classifyChunkType classifies a text chunk into one of: narrative, table, appendix, list.
func classifyChunkType(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return "narrative"
	}

	// Check appendix: keywords in first 100 chars or first few lines
	prefix := text
	if len(prefix) > 200 {
		prefix = prefix[:200]
	}
	appendixRe := regexp.MustCompile(`(?i)(附录|附件|附表|APPENDIX)`)
	if appendixRe.MatchString(prefix) {
		return "appendix"
	}

	// Check table: markdown pipe tables (|...|...|) on 3+ consecutive lines
	pipeLineCount := 0
	maxConsecutivePipes := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 3 && strings.Count(trimmed, "|") >= 2 {
			pipeLineCount++
			if pipeLineCount > maxConsecutivePipes {
				maxConsecutivePipes = pipeLineCount
			}
		} else {
			pipeLineCount = 0
		}
	}
	if maxConsecutivePipes >= 3 {
		return "table"
	}

	// Check list: 5+ consecutive lines starting with numbering patterns, each <200 chars
	listRe := regexp.MustCompile(`^\s*(\d+[.、)）]|[•◆\-－·]|（\d+）|\(\d+\))`)
	consecutiveList := 0
	maxConsecutiveList := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(trimmed) < 200 && listRe.MatchString(trimmed) {
			consecutiveList++
			if consecutiveList > maxConsecutiveList {
				maxConsecutiveList = consecutiveList
			}
		} else {
			consecutiveList = 0
		}
	}
	if maxConsecutiveList >= 5 {
		return "list"
	}

	return "narrative"
}

// getExtractionPromptByChunkType returns type-specific prompt body and system prompt.
func (s *TenderDigitizationService) getExtractionPromptByChunkType(chunkType string, mode string) (string, string) {
	baseSys := "你是一个专业的标书分析助手，只返回 JSON 格式的结构化提取结果。不要解释。"

	promptKey := "tender_rule_extraction"
	defaultPrompt := "你是一个资深的工程标书分析专家。请根据以下文本提取标书中关于资格、技术和评标的综合规则。"
	if mode == "commerce" {
		promptKey = "commerce_rule_extraction"
		defaultPrompt = "你是一个资深的工程商务标分析专家。请根据以下文本提取标书中与商务相关的规则（包括但不限于：最高限价、付款比例、资质要求、业绩要求、人员要求、财务要求、信誉要求、保证金要求等），严格排除施工组织设计等纯技术标内容。"
	}

	promptBody, sysPrompt := s.promptService.GetPromptFull(promptKey)
	if promptBody == "" {
		promptBody = defaultPrompt
	}
	if sysPrompt == "" {
		sysPrompt = baseSys
	}

	hint := ""
	switch chunkType {
	case "table":
		hint = "\n\n【附加提示】：当前文本段落包含大量表格。请务必结合表格的表头与行列结构，将表格内的明细要求（尤其是评分表、人员设备清单、各硬性指标体系）全面提取出来，绝不遗漏关键数值。"
	case "appendix":
		hint = "\n\n【附加提示】：当前文本段落来自附录/附件。请重点提取其中的补充性约束条款、技术偏差要求、材料分配细则等边界条件。"
	case "list":
		hint = "\n\n【附加提示】：当前文本段落包含大量清单或列表数据。请逐条梳理带序号的条款，尤其是隐藏在列表中的强制性废标条件、特殊的业绩与资格约束，切勿因为是列表项就忽略提取！"
	}

	return promptBody + hint, sysPrompt
}

func (s *TenderDigitizationService) mapArrayToResult(payload []map[string]interface{}) FactExtractResult {
	res := FactExtractResult{}
	for _, item := range payload {
		factType, _ := item["fact_type"].(string)
		fi := s.mapToFactItem(item)
		switch factType {
		case "score_item", "score_items":
			res.ScoreItems = append(res.ScoreItems, fi)
		case "mandatory_spec", "mandatory_specs":
			res.MandatorySpecs = append(res.MandatorySpecs, fi)
		case "project_characteristic", "project_characteristics":
			res.ProjectCharacteristics = append(res.ProjectCharacteristics, fi)
		case "special_topic", "special_topics":
			res.SpecialTopics = append(res.SpecialTopics, fi)
		default:
			// If no fact_type, heuristic mapping based on keys? Or just put into other?
			res.ScoreItems = append(res.ScoreItems, fi) // Default to score items
		}
	}
	return res
}

func (s *TenderDigitizationService) mapItemsToResult(items []interface{}) FactExtractResult {
	payload := []map[string]interface{}{}
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			payload = append(payload, m)
		}
	}
	return s.mapArrayToResult(payload)
}

func (s *TenderDigitizationService) mapKeysToResult(payload map[string]interface{}, result *FactExtractResult) {
	if items, ok := payload["score_items"].([]interface{}); ok {
		for _, it := range items {
			if m, ok := it.(map[string]interface{}); ok {
				result.ScoreItems = append(result.ScoreItems, s.mapToFactItem(m))
			}
		}
	}
	if items, ok := payload["mandatory_specs"].([]interface{}); ok {
		for _, it := range items {
			if m, ok := it.(map[string]interface{}); ok {
				result.MandatorySpecs = append(result.MandatorySpecs, s.mapToFactItem(m))
			}
		}
	}
	if items, ok := payload["project_characteristics"].([]interface{}); ok {
		for _, it := range items {
			if m, ok := it.(map[string]interface{}); ok {
				result.ProjectCharacteristics = append(result.ProjectCharacteristics, s.mapToFactItem(m))
			}
		}
	}
	if items, ok := payload["special_topics"].([]interface{}); ok {
		for _, it := range items {
			if m, ok := it.(map[string]interface{}); ok {
				result.SpecialTopics = append(result.SpecialTopics, s.mapToFactItem(m))
			}
		}
	}
}

func (s *TenderDigitizationService) mapToFactItem(item map[string]interface{}) FactItem {
	name, _ := item["name"].(string)
	content, _ := item["content"].(string)
	sourceText, _ := item["source_text"].(string)
	priority, _ := item["priority"].(string)
	priority = normalizeFactPriority(priority)
	scoreValue := 0.0
	if v, ok := item["score_value"].(float64); ok {
		scoreValue = v
	}
	fi := FactItem{Name: name, Content: content, SourceText: sourceText, Priority: priority, ScoreValue: scoreValue}
	if id, ok := item["id"].(string); ok {
		fi.ID = id
	}
	return fi
}

// PatchOutlineByMissingRequirements 按完全响应缺口做定点修补（与 OptimizeOutlineByCoverage 等价，语义化入口）
func (s *TenderDigitizationService) PatchOutlineByMissingRequirements(ctx context.Context, projectID string, originalOutlineJSON string, audit *CoverageAuditResult, facts *FactExtractResult, mappings []FactOutlineMapping, full *FullRequirementResponseResult) ([]map[string]interface{}, error) {
	return s.OptimizeOutlineByCoverage(ctx, projectID, originalOutlineJSON, audit, facts, mappings, full)
}

// OutlineFingerprint 目录小节标题 + requirement_id 的稳定哈希，用于同企业雷同风险提示
func (s *TenderDigitizationService) OutlineFingerprint(outline []map[string]interface{}) string {
	var lines []string
	for _, ch := range outline {
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				title := strings.TrimSpace(fmt.Sprint(sm["name"]))
				ids := s.collectRequirementIDs(sm)
				sort.Strings(ids)
				lines = append(lines, title+"#"+strings.Join(ids, ","))
			}
		}
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// SplitDocumentToChapters identifies chapter boundaries semantically.
func (s *TenderDigitizationService) SplitDocumentToChapters(content string) []TenderChapter {
	lines := strings.Split(content, "\n")
	var chapters []TenderChapter
	var currentTitle string = "前言/其他"
	var currentContent strings.Builder
	currentIndex := 0

	// Optimized chapter markers: focus on "Chapter" level blocks to prevent over-fragmentation.
	// We no longer treat "1." or "一、" as top-level chapters unless they are explicitly "第X章".
	chapterRegex := regexp.MustCompile(`^第[一二三四五六七八九十\d]+章`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 100 && chapterRegex.MatchString(trimmed) {
			// Found a new chapter
			if currentContent.Len() > 0 {
				chapters = append(chapters, TenderChapter{
					Title:   currentTitle,
					Content: currentContent.String(),
					Index:   currentIndex,
				})
				currentIndex++
			}
			currentTitle = trimmed
			currentContent.Reset()
			currentContent.WriteString(line + "\n")
		} else {
			currentContent.WriteString(line + "\n")
		}
	}

	// Add the last chapter
	if currentContent.Len() > 0 {
		chapters = append(chapters, TenderChapter{
			Title:   currentTitle,
			Content: currentContent.String(),
			Index:   currentIndex,
		})
	}

	// Optimization: If the content is large but we found few semantic chapters, fallback to sliding window chunking.
	// We use a larger chunk size (12,000 chars) to ensure the AI has enough surrounding context for scoring items.
	if (len(chapters) <= 2 && len(content) > 10000) || (len(chapters) > 40 && len(content) < 100000) {
		log.Printf("[Splitter] Custom triggered chunking (Chapters: %d, Length: %d)", len(chapters), len(content))
		chunks := s.splitText(content, 12000, 2000)
		chapters = nil
		for i, c := range chunks {
			chapters = append(chapters, TenderChapter{
				Title:   fmt.Sprintf("片段 %d", i+1),
				Content: c,
				Index:   i,
			})
		}
	}

	return chapters
}

// GetProjectGlobalContext extracts a high-level summary for downstream chapter extractions.
func (s *TenderDigitizationService) GetProjectGlobalContext(ctx context.Context, tenderContent string, profileJSON string) (*ProjectGlobalContext, error) {
	prompt := `你是一个资深的审标专家。请阅读以下信息，输出项目的全局语境摘要。
输出 JSON：
{
  "project_name": "...",
  "location": "...",
  "duration": "...",
  "scope": "承包范围...",
  "budget_info": "控制价/计价方式...",
  "key_red_flags": ["关键红线..."],
  "scoring_summary": "评分总原则摘要..."
}

项目画像：
%s

招标文件（头 30k 字符）：
%s`

	tc := tenderContent
	if len(tc) > 30000 {
		tc = tc[:30000]
	}

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage("你是一个专业的招标文件分析师。只返回 JSON。"),
		BuildDynamicUserBlock(fmt.Sprintf(prompt, profileJSON, tc)),
	}

	res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.1)
	if err != nil {
		return nil, err
	}

	var result ProjectGlobalContext
	cleanJSON := s.extractJSON(res)
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		// Fallback blank if LLM fails
		return &ProjectGlobalContext{ProjectName: "未知项目"}, nil
	}

	return &result, nil
}

// FinalDigitizationValidation combines coverage, logic conflict, and requirement response results into a final decision.
func (s *TenderDigitizationService) FinalDigitizationValidation(matrix *CoverageAuditMatrix, conflict *ConflictAuditResult) (string, string) {
	if conflict != nil && (conflict.HasBlock || len(conflict.Conflicts) > 5) {
		return "BLOCK", fmt.Sprintf("【冲突审计阻断】检测到 %d 条逻辑冲突：%s", len(conflict.Conflicts), conflict.Summary)
	}

	if matrix != nil {
		if matrix.MissingCount > 0 {
			return "BLOCK", fmt.Sprintf("【要求缺失阻断】尚有 %d 条核心招标要求未提取到证据事实，需触发补抽。", matrix.MissingCount)
		}
		if matrix.CoverageScore < 70 {
			return "REVISE", fmt.Sprintf("【覆盖度门槛】语义覆盖分 %.1f%% 低于 70%% 放行线。", matrix.CoverageScore)
		}
	}

	return "PASS", "标书技术事实数字化程度已达标。"
}

// ── P0-5 Supplementary Profile Extraction ──────────────────────────────────

// SupplementaryExtractionResult holds the output of the supplementary extraction pass.
type SupplementaryExtractionResult struct {
	UpdatedProfile      ProjectProfileResult `json:"updated_profile"`
	CategoriesChecked   []string             `json:"categories_checked"`
	CategoriesExtracted []string             `json:"categories_extracted"`
	RawOutputs          map[string]string    `json:"raw_outputs"`
}

// supplementaryCategory describes one high-risk field category for targeted re-extraction.
type supplementaryCategory struct {
	Key            string
	Label          string
	NeedsRescan    func(p *ProjectProfileResult) bool
	FallbackPrompt string
}

func profileFieldDeficient(f ProjectProfileField) bool {
	return f.Missing || isEmptyProjectProfileValue(f.Value) || (f.Confidence > 0 && f.Confidence < 0.6)
}

// getSupplementaryCategories returns the 4 high-risk categories for P0-5.
func getSupplementaryCategories() []supplementaryCategory {
	return []supplementaryCategory{
		{
			Key:   "procurement_boundary",
			Label: "采购边界",
			NeedsRescan: func(p *ProjectProfileResult) bool {
				return profileFieldDeficient(p.ConstructionCoreRequirements.ProcurementBoundary) ||
					len(p.ConstructionCoreRequirements.OwnerSuppliedItems) == 0 ||
					len(p.ConstructionCoreRequirements.ContractorSuppliedItems) == 0
			},
			FallbackPrompt: `请从以下招标文件中精准提取【采购边界】信息，填入 construction_core_requirements 下对应字段：
- procurement_boundary（采购边界总体描述：甲乙方采购责任划分、供货范围界面）
- owner_supplied_items（甲供材料/设备清单，每项包含名称、规格型号、数量等）
- contractor_supplied_items（乙供材料/设备清单，每项包含名称、规格型号、数量等）

特别注意：甲乙供划分常出现在"材料设备"章节、合同条款、专用条款、或技术规格书中。请逐条列出每个甲供/乙供项，而不是笼统描述。`,
		},
		{
			Key:   "schedule_nodes",
			Label: "工期节点",
			NeedsRescan: func(p *ProjectProfileResult) bool {
				return profileFieldDeficient(p.EvaluationAndPerformanceRules.TotalDuration) ||
					profileFieldDeficient(p.ConstructionCoreRequirements.ScheduleConstraints)
			},
			FallbackPrompt: `请从以下招标文件中精准提取【工期与进度节点】信息：
- total_duration（总工期，放入 evaluation_and_performance_rules.total_duration）
- schedule_constraints（里程碑节点、关键工期约束、节点罚则，放入 construction_core_requirements.schedule_constraints）

特别注意：工期信息可能分散在"投标人须知"、"合同条款"、"技术要求"等多处。请汇总所有相关条款，特别是带具体日期或天数的约束。`,
		},
		{
			Key:   "scoring_disqualification",
			Label: "评分废标",
			NeedsRescan: func(p *ProjectProfileResult) bool {
				return len(p.EvaluationAndPerformanceRules.ScoringItems) == 0 ||
					len(p.EvaluationAndPerformanceRules.DisqualificationRules) == 0
			},
			FallbackPrompt: `请从以下招标文件中精准提取【评分项与废标规则】：
- scoring_items（评分项明细：每项包含名称/评分标准/分值/权重，放入 evaluation_and_performance_rules.scoring_items）
- disqualification_rules（废标/无效标条款：每条包含具体触发条件，放入 evaluation_and_performance_rules.disqualification_rules）

特别注意：评分项通常在"评标办法"或"评分标准"章节。废标条款可能分散在"投标人须知"、"开标评标"等多处。请确保列出所有条目，不遗漏。`,
		},
		{
			Key:   "bidder_qualification",
			Label: "资格人员",
			NeedsRescan: func(p *ProjectProfileResult) bool {
				return len(p.BidderRequirements.QualificationRequirements) == 0 ||
					len(p.BidderRequirements.PerformanceRequirements) == 0 ||
					len(p.BidderRequirements.PersonnelRequirements) == 0
			},
			FallbackPrompt: `请从以下招标文件中精准提取【资格要求与人员配置】：
- qualification_requirements（投标人资格条件总述，放入 bidder_requirements.qualification_requirements）
- performance_requirements（业绩要求：需要什么类型、规模、数量的业绩，放入 bidder_requirements.performance_requirements。如果发现文本提到“详见综合评分明细表”等引用信息，请务必在下文中找到评分表并提取具体的项目类型、规模、造价、数量等指标要求，不要只提取一句“详情见xxx”，要提取出具体实质性要求内容！）
- personnel_requirements（人员要求清单：每人包含岗位名称、资质证书要求、数量要求、经验要求，放入 bidder_requirements.personnel_requirements）
- financial_requirements（财务状况要求：如要求提供近几年审计报告、营业额、利润要求、资不抵债将被否决等，放入 bidder_requirements.financial_requirements）
- credit_requirements（信誉要求：如没有处于被责令停业状态、未被列入严重违法失信企业名单或失信被执行人等，放入 bidder_requirements.credit_requirements）

特别注意：资格条件可能分散在"资格审查"、"投标人须知"、"评分标准"中。人员要求可能在"投标人须知"和"合同条款"中有不同表述。业绩要求经常详列在“综合评分明细表”中，切勿漏提。`,
		},
	}
}

const supplementSystemPrompt = `你是一个精准的招标文件补漏专家。你的任务是从给定招标文件中提取特定类别的结构化信息。

输出要求：
1. 只返回 JSON 对象，遵循 ProjectProfileResult schema
2. 只填充指定类别的字段，其他类别可以留空或设为 missing
3. 不要解释，不要输出 Markdown 标记
4. 每个对象字段必须包含 value, source_text, source_location, confidence, missing 属性
5. 数组项必须包含 name, value, source_text, source_location, confidence, missing 属性
6. source_text 必须是原文摘录，不是你的改写`

// SupplementaryProfileExtraction runs targeted second-pass extraction for high-risk field categories.
func (s *TenderDigitizationService) SupplementaryProfileExtraction(
	ctx context.Context,
	fileID string,
	fullText string,
	mainProfile *ProjectProfileResult,
) (*SupplementaryExtractionResult, error) {
	log.Printf("[Supplement] Starting supplementary extraction for file: %s", fileID)

	categories := getSupplementaryCategories()
	result := &SupplementaryExtractionResult{
		CategoriesChecked:   make([]string, 0, len(categories)),
		CategoriesExtracted: make([]string, 0),
		RawOutputs:          make(map[string]string),
	}

	// Start with a copy of the main profile
	updated := *mainProfile
	// Clear gaps/uncertain so finalize rebuilds them fresh after supplements
	updated.ExtractionGaps = []ProjectProfileListItem{}
	updated.UncertainItems = []ProjectProfileListItem{}

	// Truncate full text if too long (80k chars ~27k tokens for Chinese)
	docText := fullText
	if len(docText) > 80000 {
		head := docText[:40000]
		tail := docText[len(docText)-40000:]
		docText = head + "\n\n[... 文件截断，中间内容省略 ...]\n\n" + tail
		log.Printf("[Supplement] Document truncated from %d to 80k chars (head+tail)", len(fullText))
	}

	// Build cacheable context (shared across all 4 categories)
	sysPrompt := supplementSystemPrompt
	dbSysBody, dbSys := s.promptService.GetPromptFull("tender_profile_supplement_system")
	if dbSys != "" {
		sysPrompt = dbSys
	} else if dbSysBody != "" {
		sysPrompt = dbSysBody
	}

	for _, cat := range categories {
		result.CategoriesChecked = append(result.CategoriesChecked, cat.Key)

		if !cat.NeedsRescan(mainProfile) {
			log.Printf("[Supplement] Category %s (%s): no deficiency, skipping", cat.Key, cat.Label)
			continue
		}

		log.Printf("[Supplement] Category %s (%s): deficiency detected, running targeted extraction", cat.Key, cat.Label)

		// Get prompt from DB or use fallback
		promptKey := "tender_profile_supplement_" + cat.Key
		promptBody, _ := s.promptService.GetPromptFull(promptKey)
		if promptBody == "" {
			promptBody = cat.FallbackPrompt
		}

		// Build messages with caching: system+doc as cacheable, category instructions as dynamic
		messages := []LLMMessageV2{
			BuildCacheableSystemMessage(sysPrompt),
			BuildCacheableUserBlock("### 招标文件全文\n" + docText),
			BuildDynamicUserBlock(promptBody),
		}

		res, _, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.1)
		if err != nil {
			log.Printf("[Supplement] Category %s extraction failed: %v (continuing)", cat.Key, err)
			continue
		}

		result.RawOutputs[cat.Key] = res
		result.CategoriesExtracted = append(result.CategoriesExtracted, cat.Key)

		// Normalize and merge
		categoryResult := s.NormalizeProjectProfileChunk(res)
		mergeSupplementaryCategory(&updated, categoryResult, cat.Key)

		log.Printf("[Supplement] Category %s (%s): extraction and merge completed", cat.Key, cat.Label)
	}

	// Re-finalize to rebuild gaps/uncertain based on updated data
	s.finalizeProjectProfileResult(&updated)
	result.UpdatedProfile = updated

	log.Printf("[Supplement] Supplementary extraction complete: checked %d categories, extracted %d",
		len(result.CategoriesChecked), len(result.CategoriesExtracted))
	return result, nil
}

// mergeSupplementaryCategory merges only the fields relevant to a specific category,
// leaving all other fields untouched.
func mergeSupplementaryCategory(main *ProjectProfileResult, supplement ProjectProfileResult, categoryKey string) {
	var uncertain []ProjectProfileListItem

	switch categoryKey {
	case "procurement_boundary":
		main.ConstructionCoreRequirements.ProcurementBoundary, _ = mergeProjectProfileField(
			"采购边界",
			main.ConstructionCoreRequirements.ProcurementBoundary,
			supplement.ConstructionCoreRequirements.ProcurementBoundary,
			&uncertain,
		)
		main.ConstructionCoreRequirements.OwnerSuppliedItems = mergeProjectProfileLists(
			main.ConstructionCoreRequirements.OwnerSuppliedItems,
			supplement.ConstructionCoreRequirements.OwnerSuppliedItems,
		)
		main.ConstructionCoreRequirements.ContractorSuppliedItems = mergeProjectProfileLists(
			main.ConstructionCoreRequirements.ContractorSuppliedItems,
			supplement.ConstructionCoreRequirements.ContractorSuppliedItems,
		)

	case "schedule_nodes":
		main.EvaluationAndPerformanceRules.TotalDuration, _ = mergeProjectProfileField(
			"总工期",
			main.EvaluationAndPerformanceRules.TotalDuration,
			supplement.EvaluationAndPerformanceRules.TotalDuration,
			&uncertain,
		)
		main.ConstructionCoreRequirements.ScheduleConstraints, _ = mergeProjectProfileField(
			"工期节点",
			main.ConstructionCoreRequirements.ScheduleConstraints,
			supplement.ConstructionCoreRequirements.ScheduleConstraints,
			&uncertain,
		)

	case "scoring_disqualification":
		main.EvaluationAndPerformanceRules.ScoringItems = mergeProjectProfileLists(
			main.EvaluationAndPerformanceRules.ScoringItems,
			supplement.EvaluationAndPerformanceRules.ScoringItems,
		)
		main.EvaluationAndPerformanceRules.DisqualificationRules = mergeProjectProfileLists(
			main.EvaluationAndPerformanceRules.DisqualificationRules,
			supplement.EvaluationAndPerformanceRules.DisqualificationRules,
		)

	case "bidder_qualification":
		main.BidderRequirements.QualificationRequirements = mergeProjectProfileLists(
			main.BidderRequirements.QualificationRequirements,
			supplement.BidderRequirements.QualificationRequirements,
		)
		main.BidderRequirements.PerformanceRequirements = mergeProjectProfileLists(
			main.BidderRequirements.PerformanceRequirements,
			supplement.BidderRequirements.PerformanceRequirements,
		)
		main.BidderRequirements.PersonnelRequirements = mergeProjectProfileLists(
			main.BidderRequirements.PersonnelRequirements,
			supplement.BidderRequirements.PersonnelRequirements,
		)
		main.BidderRequirements.FinancialRequirements = mergeProjectProfileLists(
			main.BidderRequirements.FinancialRequirements,
			supplement.BidderRequirements.FinancialRequirements,
		)
		main.BidderRequirements.CreditRequirements = mergeProjectProfileLists(
			main.BidderRequirements.CreditRequirements,
			supplement.BidderRequirements.CreditRequirements,
		)
	}

	// Append any conflicts from supplementary merge to uncertain items
	if len(uncertain) > 0 {
		main.UncertainItems = mergeProjectProfileLists(main.UncertainItems, uncertain)
	}
}

// GenerateOutlineDirectly generates a 3-level technical bid outline directly from the tender document
// and its extracted facts, without relying on industry skeleton matching as the primary driver.
// This is the new "direct" generation mode (G1 of the outline pipeline simplification plan).
//
// Inputs:
//   - tenderContent: Full enriched tender document (may include domain hints)
//   - facts: Extracted facts from Step 4 facts extraction phase
//   - requirements: Requirement register built from facts
//   - profileJSON: Project profile JSON from Step 2
//   - routeName: Route plan name from Step 3
//   - projectType / profession: Project classification (used only as soft hints, not structural constraints)
func (s *TenderDigitizationService) GenerateOutlineDirectly(
	ctx context.Context,
	projectID string,
	tenderContent string,
	facts *FactExtractResult,
	requirements []RequirementRegisterEntry,
	profileJSON string,
	routeName string,
	projectType string,
	profession string,
) ([]map[string]interface{}, error) {
	log.Printf("[Digitize] GenerateOutlineDirectly for project: %s (Type: %s, Route: %s)", projectID, projectType, routeName)
	if outline, ok := s.BuildDeterministicTechnicalOutline(facts, requirements); ok {
		log.Printf("[Digitize] GenerateOutlineDirectly using deterministic requirement outline for project %s: %d chapters", projectID, len(outline))
		return outline, nil
	}

	promptBundle := s.promptService.GetPromptBundle("tech_bid_outline_direct_generation")
	promptBody, sysPrompt := promptBundle.Content, promptBundle.SystemContent
	if promptBody == "" {
		promptBody = `你是一个拥有20年国家级重点工程中标经验的"首席标书架构师"与"高级评标专家"。你的任务是根据《招标文件全文》生成一份技术标（施工组织设计）目录大纲。

你的工作原则是：
评分标准搭骨架，技术要求填血肉，行业骨架做修饰。
你必须先看评分标准，再看技术要求，最后再参考行业骨架。
输出必须是一个适合直接用于技术标编写的目录结构，且每个层级都尽量有招标文件依据。`
	}
	if sysPrompt == "" {
		sysPrompt = "只返回纯 JSON 数组，不得输出任何解释、前言、备注、Markdown 标记。"
	}

	// Truncate tender content to avoid excessive tokens
	// Keep ~50k chars (~13k tokens for Chinese text)
	// Truncate to preserve both the top and bottom (where scoring tables often reside)
	truncatedContent := tenderContent
	if len(truncatedContent) > 50000 {
		head := truncatedContent[:25000]
		tail := truncatedContent[len(truncatedContent)-25000:]
		truncatedContent = head + "\n\n[... 文件截断，中间内容省略保留首尾 50,000 字符 ...]\n\n" + tail
	}

	// --- 只提取评分项和强制项摘要，不喂全量 facts/requirements/profile ---
	keyFactsSummary := buildKeyFactsSummary(facts)

	// 将 prompt_template.variables 中的模板变量注入 prompt（如果存在）
	promptVars := map[string]interface{}{}
	if promptBundle.Variables != "" {
		if err := json.Unmarshal([]byte(promptBundle.Variables), &promptVars); err != nil {
			log.Printf("[Digitize] Warning: failed to parse prompt variables for tech_bid_outline_direct_generation: %v", err)
			promptVars = map[string]interface{}{}
		}
	}

	// 渲染 variables 中的结构化配置
	renderPrompt := promptBody
	if promptBundle.Variables != "" {
		for _, varKey := range []string{"industry_skeleton", "scoreing_focus", "technical_focus", "output_requirements"} {
			if b, err := json.Marshal(promptVars[varKey]); err == nil {
				renderPrompt = strings.ReplaceAll(renderPrompt, "{{"+varKey+"}}", string(b))
			}
		}
	}
	// content 变量始终注入招标文件正文
	renderPrompt = strings.ReplaceAll(renderPrompt, "{{content}}", truncatedContent)
	renderPrompt = strings.ReplaceAll(renderPrompt, "{{招标文件全文}}", truncatedContent)

	// --- 精简版 message 拼装：强提示词 + 招标文件 紧贴在 user message ---
	// System: 只放短硬约束
	// User (cacheable): 完整提示词主体 + 招标文件正文
	// User (dynamic): 仅评分项/强制项摘要
	cacheContext := fmt.Sprintf("%s\n\n%s\n\n### 招标文件全文\n%s",
		sysPrompt, renderPrompt, truncatedContent)

	// system message 缩短为纯硬约束
	shortSystemPrompt := "只返回纯 JSON 数组，不得输出任何解释、前言、备注、Markdown 标记。禁止臆造招标文件中不存在的专项。"

	// === 评分表必出项：作为硬约束喂给模型 ===
	mandatoryChapters := ExtractMandatoryChapters(facts)
	mandatoryText := ""
	if len(mandatoryChapters) > 0 {
		mandatoryText = "【强制独立成章项（不得合并到其他章节，必须独立列为一级章）】\n"
		for i, ch := range mandatoryChapters {
			mandatoryText += fmt.Sprintf("%d. %s\n", i+1, ch)
		}
		mandatoryText += "\n重要提示：以上每一项必须在目录中独立成为一级章（一级标题），不得合并到其他章节中。评委在一级目录中找不到对应章名，即判定为缺漏项，直接扣分。宁可多一章，不可少一章。\n"
	}

	dynamicInstruction := fmt.Sprintf(`### 以下为评分表中必须独立成章的条目（硬性约束，不得合并）
%s

### 以下为从招标文件中预抽取的评分项和强制项摘要（仅供辅助参考，招标文件全文才是唯一真相源）
%s

### 输出结构硬约束
每个三级小节必须是对象，不能是字符串；必须显式填写 requirement_ids，且 ID 必须来自上方事实摘要。
正确格式：
[
  {
    "name": "第一章 xxx",
    "units": [
      {
        "name": "第一节 xxx",
        "subsections": [
          {"name": "一、xxx", "requirement_ids": ["fact_id_1"]}
        ]
      }
    ]
  }
]
禁止把无关 ID 挂到小节上；找不到事实依据的小节 requirement_ids 必须为空数组。

请严格按上述提示词要求和招标文件内容生成三级目录。只返回 JSON 数组。`, mandatoryText, keyFactsSummary)

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage(shortSystemPrompt),
		BuildCacheableUserBlock(cacheContext),
		BuildDynamicUserBlock(dynamicInstruction),
	}

	start := time.Now()
	res, cacheUsage, err := s.aiClient.CallLLMV2WithContext(ctx, messages, 0.3)
	latencyMs := time.Since(start).Milliseconds()

	cacheMode := "none"
	if cacheUsage != nil {
		if cacheUsage.CachedTokens > 0 && cacheUsage.CacheCreationInputTokens > 0 {
			cacheMode = "explicit"
		} else if cacheUsage.CachedTokens > 0 {
			cacheMode = "implicit"
		}
		s.cacheMetrics.Log(CacheLogEntry{
			ProjectID:                projectID,
			Step:                     "generate_outline_direct",
			Model:                    s.aiClient.Model,
			CacheMode:                cacheMode,
			PromptTokens:             cacheUsage.PromptTokens,
			CachedTokens:             cacheUsage.CachedTokens,
			CacheCreationInputTokens: cacheUsage.CacheCreationInputTokens,
			CompletionTokens:         cacheUsage.CompletionTokens,
			LatencyMs:                latencyMs,
			RequestSuccess:           err == nil,
			PromptVersion:            "",
		})
	}

	if err != nil {
		return nil, err
	}

	cleanJSON := s.extractJSON(res)
	outline, parseErr := s.parseAndNormalizeOutlineJSON(cleanJSON)
	if parseErr != nil {
		return nil, fmt.Errorf("目录生成结果解析失败: %w", parseErr)
	}
	outline = s.NormalizeRequirementIDsFromFacts(outline, facts)

	log.Printf("[Digitize] GenerateOutlineDirectly completed for project %s: %d chapters", projectID, len(outline))
	return outline, nil
}

func buildKeyFactsSummary(facts *FactExtractResult) string {
	if facts == nil {
		return "无"
	}

	trim := func(s string, n int) string {
		s = strings.TrimSpace(s)
		r := []rune(s)
		if len(r) <= n {
			return s
		}
		return string(r[:n]) + "..."
	}

	var lines []string
	appendFacts := func(title string, items []FactItem, limit int) {
		if len(items) == 0 {
			return
		}
		lines = append(lines, title)
		for i, item := range items {
			if i >= limit {
				break
			}
			name := trim(item.Name, 60)
			content := trim(item.Content, 120)
			if content != "" {
				lines = append(lines, fmt.Sprintf("- ID=%s；名称=%s；内容=%s", item.ID, name, content))
			} else {
				lines = append(lines, fmt.Sprintf("- ID=%s；名称=%s", item.ID, name))
			}
		}
	}

	appendFacts("【评分项摘要】", facts.ScoreItems, 8)
	appendFacts("【强制项摘要】", facts.MandatorySpecs, 8)
	appendFacts("【特殊技术点摘要】", facts.SpecialTopics, 6)

	if len(lines) == 0 {
		return "无"
	}
	return strings.Join(lines, "\n")
}

// ============================================================================
// 后链路防污染：骨架冻结与恢复
// ============================================================================

// SkeletonEntry 保存一级章和二级节的骨架结构
type SkeletonEntry struct {
	ChapterName string         `json:"chapter_name"` // 一级章名称，如 "第一章 施工组织总体策划"
	ChapterIdx  int            `json:"chapter_idx"`  // 一级章序号（从0开始）
	Units       []SkeletonUnit `json:"units"`        // 二级节列表
}

// SkeletonUnit 保存二级节的骨架结构
type SkeletonUnit struct {
	UnitName string `json:"unit_name"` // 二级节名称，如 "第一节 施工部署"
	UnitIdx  int    `json:"unit_idx"`  // 二级节序号（从0开始）
}

// ExtractSkeleton 从目录 outline 中提取一级章和二级节骨架
// 只保留主骨架，不含三级小节
func ExtractSkeleton(outline []map[string]interface{}) []SkeletonEntry {
	if len(outline) == 0 {
		return nil
	}

	var skeleton []SkeletonEntry
	for chapIdx, chapter := range outline {
		entry := SkeletonEntry{
			ChapterIdx: chapIdx,
		}

		// 提取一级章名称
		if name, ok := chapter["name"].(string); ok {
			entry.ChapterName = name
		}

		// 提取二级节列表
		if unitsRaw, ok := chapter["units"].([]interface{}); ok {
			for unitIdx, unitRaw := range unitsRaw {
				if unitMap, ok := unitRaw.(map[string]interface{}); ok {
					unit := SkeletonUnit{
						UnitIdx: unitIdx,
					}
					if name, ok := unitMap["name"].(string); ok {
						unit.UnitName = name
					}
					entry.Units = append(entry.Units, unit)
				}
			}
		}

		skeleton = append(skeleton, entry)
	}

	return skeleton
}

// enforceSkeleton 强制恢复主骨架
// 规则：
// 1. 一级章的名称和顺序必须和 frozenSkeleton 一致
// 2. 二级节的名称和顺序必须和 frozenSkeleton 一致
// 3. 三级小节允许增删改（这是优化的主要空间）
// 4. 如果优化后新增了章/节（frozenSkeleton 里没有的），追加到末尾
func EnforceSkeleton(optimized []map[string]interface{}, frozen []SkeletonEntry) []map[string]interface{} {
	if len(frozen) == 0 || len(optimized) == 0 {
		return optimized
	}

	// 构建骨架名称集合，用于快速查找
	frozenChapterSet := make(map[string]bool)
	frozenUnitSet := make(map[string]bool)
	for _, entry := range frozen {
		frozenChapterSet[entry.ChapterName] = true
		for _, unit := range entry.Units {
			key := entry.ChapterName + "||" + unit.UnitName
			frozenUnitSet[key] = true
		}
	}

	// 分类章：骨架内的 vs 优化新增的
	var skeletonChapters []map[string]interface{}
	var extraChapters []map[string]interface{}

	for _, chapter := range optimized {
		name := ""
		if n, ok := chapter["name"].(string); ok {
			name = n
		}

		if frozenChapterSet[name] {
			// 这是骨架内的章，按骨架顺序重建
			skeletonChapters = append(skeletonChapters, RebuildChapterFromSkeleton(chapter, name, frozen))
		} else {
			// 这是优化新增的章，保留到末尾
			extraChapters = append(extraChapters, chapter)
		}
	}

	// 按骨架顺序排列
	sort.Slice(skeletonChapters, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if n, ok := skeletonChapters[i]["name"].(string); ok {
			nameI = n
		}
		if n, ok := skeletonChapters[j]["name"].(string); ok {
			nameJ = n
		}
		// 找到在骨架中的位置
		posI := -1
		posJ := -1
		for idx, entry := range frozen {
			if entry.ChapterName == nameI {
				posI = idx
			}
			if entry.ChapterName == nameJ {
				posJ = idx
			}
		}
		return posI < posJ
	})

	// 合并：骨架章 + 新增章
	result := append(skeletonChapters, extraChapters...)
	return result
}

// rebuildChapterFromSkeleton 重建单个章节，按骨架恢复二级节顺序
func RebuildChapterFromSkeleton(chapter map[string]interface{}, chapterName string, frozen []SkeletonEntry) map[string]interface{} {
	// 找到对应的骨架条目
	var skeletonEntry *SkeletonEntry
	for i := range frozen {
		if frozen[i].ChapterName == chapterName {
			skeletonEntry = &frozen[i]
			break
		}
	}

	if skeletonEntry == nil {
		return chapter
	}

	// 构建骨架单元名称集合
	frozenUnitSet := make(map[string]bool)
	for _, unit := range skeletonEntry.Units {
		frozenUnitSet[unit.UnitName] = true
	}

	// 分类单元：骨架内的 vs 新增的
	var skeletonUnits []map[string]interface{}
	var extraUnits []map[string]interface{}

	if unitsRaw, ok := chapter["units"].([]interface{}); ok {
		for _, unitRaw := range unitsRaw {
			if unitMap, ok := unitRaw.(map[string]interface{}); ok {
				unitName := ""
				if n, ok := unitMap["name"].(string); ok {
					unitName = n
				}

				if frozenUnitSet[unitName] {
					skeletonUnits = append(skeletonUnits, unitMap)
				} else {
					extraUnits = append(extraUnits, unitMap)
				}
			}
		}
	}

	// 按骨架顺序排列
	sort.Slice(skeletonUnits, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if n, ok := skeletonUnits[i]["name"].(string); ok {
			nameI = n
		}
		if n, ok := skeletonUnits[j]["name"].(string); ok {
			nameJ = n
		}
		posI := -1
		posJ := -1
		for idx, unit := range skeletonEntry.Units {
			if unit.UnitName == nameI {
				posI = idx
			}
			if unit.UnitName == nameJ {
				posJ = idx
			}
		}
		return posI < posJ
	})

	// 重建章节
	result := make(map[string]interface{})
	for k, v := range chapter {
		result[k] = v
	}
	result["units"] = append(skeletonUnits, extraUnits...)

	return result
}

// computeChapterDrift 计算骨架偏移程度
// 返回 0.0 ~ 1.0，值越大表示骨架变化越大
// 规则：
// - 一级章名称变化：权重 0.6
// - 一级章顺序变化：权重 0.2
// - 二级节名称变化：权重 0.15
// - 二级节顺序变化：权重 0.05
func ComputeChapterDrift(original, optimized []map[string]interface{}) float64 {
	if len(original) == 0 || len(optimized) == 0 {
		return 1.0 // 完全无效
	}

	// 提取原始骨架
	origSkeleton := ExtractSkeleton(original)
	optSkeleton := ExtractSkeleton(optimized)

	// 计算一级章名称变化
	origChapterNames := make([]string, len(origSkeleton))
	optChapterNames := make([]string, len(optSkeleton))
	for i, entry := range origSkeleton {
		origChapterNames[i] = entry.ChapterName
	}
	for i, entry := range optSkeleton {
		optChapterNames[i] = entry.ChapterName
	}

	// 一级章名称变化
	nameChanges := 0
	maxChapters := max(len(origChapterNames), len(optChapterNames))
	commonChapters := min(len(origChapterNames), len(optChapterNames))
	for i := 0; i < commonChapters; i++ {
		if origChapterNames[i] != optChapterNames[i] {
			nameChanges++
		}
	}
	// 新增的章也算变化
	nameChanges += abs(len(optChapterNames) - len(origChapterNames))

	nameDrift := float64(nameChanges) / float64(maxChapters)

	// 一级章顺序变化（只计算共同章节的顺序变化）
	orderChanges := 0
	if len(origChapterNames) > 1 && len(optChapterNames) > 1 {
		// 构建原始位置的映射
		origPos := make(map[string]int)
		for i, name := range origChapterNames {
			origPos[name] = i
		}
		// 计算逆序对
		optCommon := make([]string, 0)
		for _, name := range optChapterNames {
			if _, exists := origPos[name]; exists {
				optCommon = append(optCommon, name)
			}
		}
		for i := 0; i < len(optCommon)-1; i++ {
			for j := i + 1; j < len(optCommon); j++ {
				if origPos[optCommon[i]] > origPos[optCommon[j]] {
					orderChanges++
				}
			}
		}
	}
	maxPairs := len(origChapterNames) * (len(origChapterNames) - 1) / 2
	orderDrift := 0.0
	if maxPairs > 0 {
		orderDrift = float64(orderChanges) / float64(maxPairs)
	}

	// 计算二级节变化
	unitNameChanges := 0
	totalUnits := 0
	for i := range origSkeleton {
		if i < len(optSkeleton) {
			origUnits := origSkeleton[i].Units
			optUnits := optSkeleton[i].Units

			// 名称变化
			commonUnits := min(len(origUnits), len(optUnits))
			totalUnits += commonUnits
			for j := 0; j < commonUnits; j++ {
				if origUnits[j].UnitName != optUnits[j].UnitName {
					unitNameChanges++
				}
			}
			// 新增的节
			unitNameChanges += abs(len(optUnits) - len(origUnits))
		}
	}
	if totalUnits > 0 {
		unitNameChanges += abs(totalUnits - countTotalUnits(optSkeleton))
	}

	unitNameDrift := 0.0
	if totalUnits > 0 {
		unitNameDrift = float64(unitNameChanges) / float64(max(totalUnits, countTotalUnits(optSkeleton)))
	}

	// 综合计算 drift
	// 一级章名称变化权重 0.6，二级节名称变化权重 0.25，一级章顺序变化权重 0.15
	totalDrift := nameDrift*0.6 + unitNameDrift*0.25 + orderDrift*0.15

	log.Printf("[SkeletonProtection] Drift analysis: name=%.2f, unit_name=%.2f, order=%.2f, total=%.2f",
		nameDrift, unitNameDrift, orderDrift, totalDrift)

	return totalDrift
}

// countTotalUnits 计算骨架中的总单元数
func countTotalUnits(skeleton []SkeletonEntry) int {
	count := 0
	for _, entry := range skeleton {
		count += len(entry.Units)
	}
	return count
}

// abs 返回绝对值
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// max 返回最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min 返回最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// 评分表必出项强制校验
// ============================================================================

// ExtractMandatoryChapters 从 facts 中筛选必须独立成章的条目
// 这些条目对应评标办法中明确列出的独立评分维度
func ExtractMandatoryChapters(facts *FactExtractResult) []string {
	if facts == nil {
		return nil
	}

	// 定义必出项关键词（任意匹配即收录）
	requiredKeywords := []string{
		"施工方案", "技术措施",
		"质量管理",
		"安全文明", "安全管理",
		"环境保护",
		"工期保证", "工期计划", "工期",
		"资源配备", "资源配置",
		"施工总平面", "平面布置",
		"项目管理机构",
	}

	mandatorySet := make(map[string]bool)
	seen := make(map[string]bool)

	// 从 ScoreItems 筛选
	for _, item := range facts.ScoreItems {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		// 检查是否包含必出关键词
		for _, kw := range requiredKeywords {
			if strings.Contains(name, kw) && !seen[name] {
				mandatorySet[name] = true
				seen[name] = true
				break
			}
		}
	}

	// 从 MandatorySpecs 中筛选 high priority 且名称明确指向独立章节的
	for _, spec := range facts.MandatorySpecs {
		if spec.Priority != "high" {
			continue
		}
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		for _, kw := range requiredKeywords {
			if strings.Contains(name, kw) && !seen[name] {
				mandatorySet[name] = true
				seen[name] = true
				break
			}
		}
	}

	// 转成 slice
	var result []string
	for name := range mandatorySet {
		result = append(result, name)
	}

	// 排序保证顺序稳定
	sort.Strings(result)
	return result
}

// cleanOutlineTitle 移除标题中可能干扰 AI 的编号前缀（如 “第1章”、“一、”等）
func cleanOutlineTitle(title string) string {
	// 匹配常见编号模式：第X章、第X节、一、(一)、1.、1、 等
	re := regexp.MustCompile(`^(第[一二三四五六七八九十\d]+[章节]\s*|[一二三四五六七八九十]+\s*[、\.]\s*|[\(\（][一二三四五六七八九十]+[\)\）]\s*|\d+[\.、\s]+|[\(\（]\d+[\)\）]\s*)`)
	cleaned := re.ReplaceAllString(title, "")
	return strings.TrimSpace(cleaned)
}

// ValidateMandatoryChapters 校验必出项是否全部独立成章
// 返回缺失项列表和是否全部通过
func ValidateMandatoryChapters(outline []map[string]interface{}, mandatoryChapters []string) (missing []string, ok bool) {
	if len(mandatoryChapters) == 0 {
		return nil, true
	}
	if len(outline) == 0 {
		return mandatoryChapters, false
	}

	// 提取一级章名称
	chapterNames := make([]string, len(outline))
	for i, ch := range outline {
		if name, ok := ch["name"].(string); ok {
			chapterNames[i] = name
		}
	}

	// 检查每个必出项
	for _, mandatory := range mandatoryChapters {
		found := false
		// 检查是否在某个一级章中命中
		for _, chName := range chapterNames {
			if chName == "" {
				continue
			}
			// 宽松匹配：必出项名称包含于章名，或章名包含必出项关键词
			if strings.Contains(chName, mandatory) || MatchChapterWithMandatory(chName, mandatory) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, mandatory)
		}
	}

	return missing, len(missing) == 0
}

// ============================================================================
// 评分表必出项终极补强
// ============================================================================

// MandatoryChapterRule 结构化映射规则
type MandatoryChapterRule struct {
	CanonicalName string   // 标准名称
	Aliases       []string // 别名列表
	InsertAfter   string   // 应插入在哪个标准名之后（空表示末尾）
}

// getMandatoryChapterRules 返回必出项的结构化映射规则
func getMandatoryChapterRules() []MandatoryChapterRule {
	return []MandatoryChapterRule{
		{CanonicalName: "施工方案和技术措施", Aliases: []string{"施工方案", "技术措施", "技术方案"}, InsertAfter: ""},
		{CanonicalName: "质量管理体系与措施", Aliases: []string{"质量管理", "质量保证", "质量保障"}, InsertAfter: "施工方案和技术措施"},
		{CanonicalName: "安全文明施工管理体系与措施", Aliases: []string{"安全文明", "安全施工", "安全管理"}, InsertAfter: "质量管理体系与措施"},
		{CanonicalName: "环境保护管理体系与措施", Aliases: []string{"环境保护", "环保管理", "环境管理", "绿色施工"}, InsertAfter: "安全文明施工管理体系与措施"},
		{CanonicalName: "工期保证措施", Aliases: []string{"工期保证", "工期计划", "工期管理", "进度控制"}, InsertAfter: "环境保护管理体系与措施"},
		{CanonicalName: "资源配备计划", Aliases: []string{"资源配备", "资源配置", "人机料", "设备材料"}, InsertAfter: "工期保证措施"},
		{CanonicalName: "施工总平面布置", Aliases: []string{"施工总平面", "总平面布置", "平面布置", "现场布置"}, InsertAfter: "资源配备计划"},
		{CanonicalName: "项目管理机构配置", Aliases: []string{"项目管理机构", "项目组织", "管理架构", "组织机构"}, InsertAfter: "施工总平面布置"},
	}
}

// MatchChapterWithMandatory 判断章名是否命中某个必出项
// 使用结构化映射，更准确地判断
func MatchChapterWithMandatory(chapterName, mandatory string) bool {
	rules := getMandatoryChapterRules()

	// 直接匹配标准名称
	for _, rule := range rules {
		if rule.CanonicalName == mandatory || strings.Contains(mandatory, rule.CanonicalName) {
			// 检查章名是否包含该标准名或其别名
			for _, alias := range rule.Aliases {
				if strings.Contains(chapterName, alias) {
					return true
				}
			}
		}
	}

	// 模糊匹配：必出项包含某关键词，章名也包含
	keywords := []string{
		"施工方案", "技术措施", "质量管理", "安全文明", "安全管理",
		"环境保护", "工期保证", "资源配备", "平面布置", "项目管理机构",
	}
	for _, kw := range keywords {
		if strings.Contains(mandatory, kw) && strings.Contains(chapterName, kw) {
			return true
		}
	}

	return false
}

// FindChapterPosition 找到某标准章节在目录中的位置
// 返回第一个匹配的索引，-1 表示未找到
func FindChapterPosition(outline []map[string]interface{}, canonicalName string) int {
	for i, ch := range outline {
		if name, ok := ch["name"].(string); ok {
			if name == canonicalName || strings.Contains(name, canonicalName) {
				return i
			}
			// 也检查别名
			rules := getMandatoryChapterRules()
			for _, rule := range rules {
				if rule.CanonicalName == canonicalName {
					for _, alias := range rule.Aliases {
						if strings.Contains(name, alias) {
							return i
						}
					}
				}
			}
		}
	}
	return -1
}

// InsertAfterCanonical 找到某标准章节应该插入到哪个位置之后
func InsertAfterCanonical(mandatoryName string) string {
	rules := getMandatoryChapterRules()
	for _, rule := range rules {
		if rule.CanonicalName == mandatoryName || strings.Contains(mandatoryName, rule.CanonicalName) {
			return rule.InsertAfter
		}
	}
	return "" // 默认末尾
}

// PatchMissingMandatoryChapters 对缺失的必出项自动补章
// 按合理位置插入（非一律末尾），每个占位一个章+节+小节
func PatchMissingMandatoryChapters(outline []map[string]interface{}, missing []string) []map[string]interface{} {
	if len(missing) == 0 {
		return outline
	}

	result := make([]map[string]interface{}, len(outline))
	copy(result, outline)

	for _, chName := range missing {
		// 创建占位章节
		newChapter := map[string]interface{}{
			"name": chName,
			"units": []map[string]interface{}{
				{
					"name": "第一节 总体措施",
					"subsections": []map[string]interface{}{
						{"name": "一、待补充内容", "requirement_ids": []string{}},
					},
				},
			},
		}

		// 找到应该插入的位置
		insertAfter := InsertAfterCanonical(chName)
		if insertAfter == "" {
			// 插入到末尾
			result = append(result, newChapter)
		} else {
			// 找到插入位置
			pos := FindChapterPosition(result, insertAfter)
			if pos >= 0 {
				// 插入到该位置之后
				insertIndex := pos + 1
				result = append(result[:insertIndex], append([]map[string]interface{}{newChapter}, result[insertIndex:]...)...)
			} else {
				// 未找到插入位置，追加到末尾
				result = append(result, newChapter)
			}
		}
	}

	return result
}

// PatchMissingMandatoryChaptersWithLoop 循环补章直到稳定（最多3轮）
func PatchMissingMandatoryChaptersWithLoop(outline []map[string]interface{}, mandatoryChapters []string, maxLoops int) ([]map[string]interface{}, bool) {
	currentOutline := outline
	for i := 0; i < maxLoops; i++ {
		missing, ok := ValidateMandatoryChapters(currentOutline, mandatoryChapters)
		if ok {
			return currentOutline, true
		}
		log.Printf("[Step4] 循环校验第 %d 轮：仍有缺失项 %v，继续补章", i+1, missing)
		currentOutline = PatchMissingMandatoryChapters(currentOutline, missing)
	}
	// 最后再校验一次
	missing, ok := ValidateMandatoryChapters(currentOutline, mandatoryChapters)
	if !ok {
		log.Printf("[Step4] ⚠️ 循环补章后仍有缺失项: %v", missing)
	}
	return currentOutline, ok
}

// NormalizeOutlineNames 清理目录中重复的章节编号
// 处理如 "第一章 第一章 XXX" -> "第一章 XXX" 的重复情况
func NormalizeOutlineNames(outline []map[string]interface{}) []map[string]interface{} {
	for i, chapter := range outline {
		if name, ok := chapter["name"].(string); ok {
			chapter["name"] = renumberChapterName(normalizeChapterName(name), i+1)
		}
		// 处理节
		if units, ok := chapter["units"].([]interface{}); ok {
			for _, u := range units {
				if unit, ok := u.(map[string]interface{}); ok {
					if name, ok := unit["name"].(string); ok {
						unit["name"] = normalizeSectionName(name)
					}
					// 处理小节
					if subsections, ok := unit["subsections"].([]interface{}); ok {
						for _, s := range subsections {
							if sub, ok := s.(map[string]interface{}); ok {
								if name, ok := sub["name"].(string); ok {
									sub["name"] = normalizeSubsectionName(name)
								}
							}
						}
					}
				}
			}
		}
	}
	return outline
}

// normalizeChapterName 清理章名中的重复编号
// 如 "第一章 第一章 工程概况" -> "第一章 工程概况"
func normalizeChapterName(name string) string {
	// 使用字符串处理而非反向引用（Go regexp 不支持 \1）
	// 匹配 "第X章 第X章" 模式
	chineseNums := "[一二三四五六七八九十百千万]+"

	// 尝试匹配中文数字模式
	re1 := regexp.MustCompile(`^(第` + chineseNums + `章)(\s+第` + chineseNums + `章)\s*`)
	if re1.MatchString(name) {
		return re1.ReplaceAllString(name, "$1 ")
	}

	// 尝试匹配阿拉伯数字模式 (使用 [0-9]+ 代替 \d+)
	re2 := regexp.MustCompile(`^(第[0-9]+章)(\s+第[0-9]+章)\s*`)
	if re2.MatchString(name) {
		return re2.ReplaceAllString(name, "$1 ")
	}

	return name
}

func renumberChapterName(name string, index int) string {
	title := strings.TrimSpace(stripLeadingChapterPrefix(name))
	if title == "" {
		title = "目录章节"
	}
	return fmt.Sprintf("第%s章 %s", chineseNumber(index), title)
}

func stripLeadingChapterPrefix(name string) string {
	name = strings.TrimSpace(name)
	reChinese := regexp.MustCompile(`^第[一二三四五六七八九十百千万]+章\s*`)
	name = reChinese.ReplaceAllString(name, "")
	reArabic := regexp.MustCompile(`^第[0-9]+章\s*`)
	name = reArabic.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

func chineseNumber(n int) string {
	if n <= 0 {
		return "一"
	}
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if n < 10 {
		return digits[n]
	}
	if n == 10 {
		return "十"
	}
	if n < 20 {
		return "十" + digits[n%10]
	}
	if n < 100 {
		tens := n / 10
		ones := n % 10
		if ones == 0 {
			return digits[tens] + "十"
		}
		return digits[tens] + "十" + digits[ones]
	}
	return strconv.Itoa(n)
}

// normalizeSectionName 清理节名中的重复编号
// 如 "第一节 第一节 项目情况" -> "第一节 项目情况"
func normalizeSectionName(name string) string {
	// 使用字符串处理而非反向引用（Go regexp 不支持 \1）
	chineseNums := "[一二三四五六七八九十百千万]+"

	// 尝试匹配中文数字模式
	re1 := regexp.MustCompile(`^(第` + chineseNums + `节)(\s+第` + chineseNums + `节)\s*`)
	if re1.MatchString(name) {
		return re1.ReplaceAllString(name, "$1 ")
	}

	// 尝试匹配阿拉伯数字模式 (使用 [0-9]+ 代替 \d+)
	re2 := regexp.MustCompile(`^(第[0-9]+节)(\s+第[0-9]+节)\s*`)
	if re2.MatchString(name) {
		return re2.ReplaceAllString(name, "$1 ")
	}

	return name
}

// normalizeSubsectionName 清理小节名中的重复编号
// 如 "(一) 一、工程概况" -> "(一) 工程概况"
func normalizeSubsectionName(name string) string {
	// 匹配 "(X) X、" 模式，保留括号格式
	patterns := []string{
		`^([（(][一二三四五六七八九十][)）])\s*[一二三四五六七八九十]、?\s*`,
		`^([（(]\d+[)）])\s*\d+[、.]\s*`,
		`^([（(][a-zA-Z][)）])\s*[a-zA-Z][、.]\s*`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(name) {
			return re.ReplaceAllString(name, "$1 ")
		}
	}
	// 处理 "一、一、" 重复（不使用反向引用）
	re4 := regexp.MustCompile(`^([一二三四五六七八九十])[、.]\s*[一二三四五六七八九十][、.]\s*`)
	if re4.MatchString(name) {
		return re4.ReplaceAllString(name, "$1、")
	}
	// 处理 "(一) (一)" 重复（不使用反向引用）
	re5 := regexp.MustCompile(`^([（(][一二三四五六七八九十][)）])\s*[（(][一二三四五六七八九十][)）]\s*`)
	if re5.MatchString(name) {
		return re5.ReplaceAllString(name, "$1 ")
	}
	return name
}
