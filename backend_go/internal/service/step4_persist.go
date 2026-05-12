package service

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ReplaceChapterPlansFromOutline replaces chapter plans (and draft contents) from outline JSON.
func ReplaceChapterPlansFromOutline(db *sqlx.DB, projectID string, outline []map[string]interface{}, outlineVersion int) error {
	tx := db.MustBegin()
	if _, err := tx.Exec(`DELETE FROM tech_bid_chapter_contents WHERE project_id = ?`, projectID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tech_bid_chapter_plans WHERE project_id = ?`, projectID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for i, ch := range outline {
		chapterID := uuid.New().String()
		chapterName := ""
		if n, ok := ch["name"].(string); ok {
			chapterName = n
		}
		if _, err := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, 'chapter', 'completed', ?)`,
			chapterID, projectID, chapterName, i+1, outlineVersion); err != nil {
			_ = tx.Rollback()
			return err
		}
		units, _ := ch["units"].([]interface{})
		for j, u := range units {
			uMap, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			unitID := uuid.New().String()
			unitName, _ := uMap["name"].(string)
			if _, err := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, ?, 'unit', 'completed', ?)`,
				unitID, projectID, chapterID, unitName, j+1, outlineVersion); err != nil {
				_ = tx.Rollback()
				return err
			}
			subs, _ := uMap["subsections"].([]interface{})
			for k, s := range subs {
				subName := ""
				reqIDs := ""
				if sMap, ok := s.(map[string]interface{}); ok {
					subName, _ = sMap["name"].(string)
					reqIDs = marshalRequirementIDsJSON(sMap["requirement_ids"])
				} else {
					subName, _ = s.(string)
				}
				if _, err := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version, requirement_ids_json) VALUES (?, ?, ?, ?, ?, 'subsection', 'not_started', ?, ?)`,
					uuid.New().String(), projectID, unitID, subName, k+1, outlineVersion, reqIDs); err != nil {
					_ = tx.Rollback()
					return err
				}
			}
		}
	}
	return tx.Commit()
}

func marshalRequirementIDsJSON(raw interface{}) string {
	switch r := raw.(type) {
	case nil:
		return ""
	case []string:
		if len(r) == 0 {
			return ""
		}
		b, _ := json.Marshal(r)
		return string(b)
	case []interface{}:
		ids := make([]string, 0, len(r))
		for _, it := range r {
			if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
				ids = append(ids, strings.TrimSpace(s))
			}
		}
		if len(ids) == 0 {
			return ""
		}
		b, _ := json.Marshal(ids)
		return string(b)
	case string:
		if strings.TrimSpace(r) == "" {
			return ""
		}
		b, _ := json.Marshal([]string{strings.TrimSpace(r)})
		return string(b)
	default:
		b, _ := json.Marshal(r)
		return string(b)
	}
}

// PersistStep4FactMappings writes tech_bid_fact_mappings (legacy table).
func PersistStep4FactMappings(db *sqlx.DB, projectID string, mappings []FactOutlineMapping) error {
	if _, err := db.Exec(`DELETE FROM tech_bid_fact_mappings WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, m := range mappings {
		pathJSON, _ := json.Marshal(m.TargetPath)
		req := 0
		if m.Required {
			req = 1
		}
		_, err := db.Exec(`INSERT INTO tech_bid_fact_mappings (id, project_id, fact_id, fact_type, fact_name, target_level, target_path_json, required, priority, mapping_reason, mapping_source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'ai')`,
			uuid.New().String(), projectID, m.FactID, m.FactType, m.FactName, m.TargetLevel, string(pathJSON), req, m.Priority, m.MappingReason)
		if err != nil {
			return err
		}
	}
	return nil
}

// PersistCoverageCheck writes tech_bid_outline_coverage_checks.
func PersistCoverageCheck(db *sqlx.DB, projectID string, outlineVersion int, cov *OutlineCoverageResult) (string, error) {
	if cov == nil {
		return "", nil
	}
	if _, err := db.Exec(`DELETE FROM tech_bid_outline_coverage_checks WHERE project_id = ?`, projectID); err != nil {
		return "", err
	}
	id := uuid.New().String()
	missJ, _ := json.Marshal(cov.MissingFactIDs)
	weakJ, _ := json.Marshal(cov.WeakFactIDs)
	dupJ, _ := json.Marshal(cov.DuplicateNodeHints)
	_, err := db.Exec(`INSERT INTO tech_bid_outline_coverage_checks (id, project_id, outline_version, fact_total, fact_mapped, coverage_rate, missing_fact_ids_json, weak_fact_ids_json, duplicate_node_ids_json, result, summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, outlineVersion, cov.FactTotal, cov.FactMapped, cov.CoverageRate, string(missJ), string(weakJ), string(dupJ), cov.Result, cov.Summary)
	return id, err
}

// PersistRequirementRegister writes tech_bid_requirement_register.
func PersistRequirementRegister(db *sqlx.DB, projectID string, entries []RequirementRegisterEntry) error {
	if _, err := db.Exec(`DELETE FROM tech_bid_requirement_register WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, e := range entries {
		rid := strings.TrimSpace(e.RequirementID)
		if rid == "" {
			continue
		}
		_, err := db.Exec(`INSERT INTO tech_bid_requirement_register 
			(id, project_id, requirement_id, requirement_type, source_text, source_location, priority, must_be_explicit, expected_response_level, domain, response_tier, summary) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), projectID, rid, e.RequirementType, e.SourceText, e.SourceLocation, e.Priority, e.MustBeExplicit, e.ExpectedResponseLevel, e.Domain, e.ResponseTier, e.Summary)
		if err != nil {
			return err
		}
	}
	return nil
}

// PersistConflictAudit writes tech_bid_conflict_audit.
func PersistConflictAudit(db *sqlx.DB, projectID string, audit *ConflictAuditResult) error {
	if audit == nil {
		return nil
	}
	if _, err := db.Exec(`DELETE FROM tech_bid_conflict_audit WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, c := range audit.Conflicts {
		manual := 0
		if c.ManualReviewRequired {
			manual = 1
		}
		_, err := db.Exec(`INSERT INTO tech_bid_conflict_audit 
			(id, project_id, conflict_type, field_name, source_a, source_b, conflict_reason, severity, manual_review_required) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), projectID, c.Type, c.FieldName, c.SourceA, c.SourceB, c.Reason, c.Severity, manual)
		if err != nil {
			return err
		}
	}
	return nil
}

// PersistFullResponseCheck writes tech_bid_requirement_response_checks.
func PersistFullResponseCheck(db *sqlx.DB, projectID string, outlineVersion int, res *FullRequirementResponseResult) (string, error) {
	if res == nil {
		return "", nil
	}
	if _, err := db.Exec(`DELETE FROM tech_bid_requirement_response_checks WHERE project_id = ?`, projectID); err != nil {
		return "", err
	}
	id := uuid.New().String()
	missJ, _ := json.Marshal(res.MissingRequirementIDs)
	weakJ, _ := json.Marshal(res.WeakRequirementIDs)
	tagJ, _ := json.Marshal(res.OnlyTaggedRequirementIDs)
	shellJ, _ := json.Marshal(res.ShellTitleHints)
	highJ, _ := json.Marshal(res.HighPriorityMissingIDs)
	mandMissJ, _ := json.Marshal(res.MandatoryMissingIDs)
	mandInsJ, _ := json.Marshal(res.MandatoryInsufficientIDs)
	hardJ, _ := json.Marshal(res.HardRuleWarnings)
	_, err := db.Exec(`INSERT INTO tech_bid_requirement_response_checks (id, project_id, outline_version, requirement_total, requirement_mapped, requirement_fully_responded, requirement_weakly_responded, requirement_only_tagged, full_response_rate, weak_response_rate, response_quality_score, missing_requirement_ids_json, weak_requirement_ids_json, only_tagged_requirement_ids_json, shell_title_hints_json, high_priority_missing_ids_json, mandatory_missing_ids_json, mandatory_insufficient_ids_json, hard_rule_warnings_json, result, summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, outlineVersion, res.RequirementTotal, res.RequirementMapped, res.RequirementFullyResponded, res.RequirementWeaklyResponded, res.RequirementOnlyTagged, res.FullResponseRate, res.WeakResponseRate, res.ResponseQualityScore, string(missJ), string(weakJ), string(tagJ), string(shellJ), string(highJ), string(mandMissJ), string(mandInsJ), string(hardJ), res.Result, res.Summary)
	return id, err
}

// PersistFactsWithEvidence writes tech_bid_outline_facts.
func PersistFactsWithEvidence(db *sqlx.DB, projectID string, facts *FactExtractResult) error {
	if _, err := db.Exec(`DELETE FROM tech_bid_outline_facts WHERE project_id = ?`, projectID); err != nil {
		return err
	}

	saveItems := func(items []FactItem, fType string) error {
		for _, it := range items {
			_, err := db.Exec(`INSERT INTO tech_bid_outline_facts
				(id, project_id, fact_code, fact_type, fact_name, fact_content, source_text, source_section, source_page, source_line, evidence_count, extracted_by_view, priority, score_value)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), projectID, it.ID, fType, it.Name, it.Content, it.SourceText, it.SourceChapter, it.PageNumber, it.LineNumber, it.EvidenceCount, it.ExtractedByView, it.Priority, it.ScoreValue)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if err := saveItems(facts.ScoreItems, "score_item"); err != nil {
		return err
	}
	if err := saveItems(facts.MandatorySpecs, "mandatory_spec"); err != nil {
		return err
	}
	if err := saveItems(facts.ProjectCharacteristics, "project_characteristic"); err != nil {
		return err
	}
	if err := saveItems(facts.SpecialTopics, "special_topic"); err != nil {
		return err
	}

	return nil
}
