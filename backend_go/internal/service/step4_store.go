package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"backend_go/internal/model"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Step4Store persists Coordinator run / agent rows and optional snapshots.
type Step4Store struct {
	DB *sqlx.DB
}

func NewStep4Store(db *sqlx.DB) *Step4Store { return &Step4Store{DB: db} }

// CreateRun inserts a new step4_runs row and sets tech_bid_projects.active_step4_run_id.
func (s *Step4Store) CreateRun(projectID, triggerSource, operatorID, initialStatus, stage string) (int64, error) {
	now := time.Now()
	var op interface{}
	if strings.TrimSpace(operatorID) != "" {
		op = operatorID
	}
	res, err := s.DB.Exec(`INSERT INTO step4_runs (project_id, trigger_source, operator_id, status, current_stage, started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, triggerSource, op, initialStatus, stage, now, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, _ = s.DB.Exec(`UPDATE tech_bid_projects SET active_step4_run_id = ?, updated_at = ? WHERE id = ?`, id, now, projectID)
	return id, nil
}

// UpdateRunStatus updates run status and optional stage / gate / error.
func (s *Step4Store) UpdateRunStatus(runID int64, status, stage string, gate *string, errMsg *string) error {
	now := time.Now()
	q := `UPDATE step4_runs SET status = ?, current_stage = ?, updated_at = ?`
	args := []interface{}{status, stage, now}
	if gate != nil {
		q += `, gate_result = ?`
		args = append(args, *gate)
	}
	if errMsg != nil {
		q += `, error_message = ?`
		args = append(args, *errMsg)
	}
	q += ` WHERE id = ?`
	args = append(args, runID)
	_, err := s.DB.Exec(q, args...)
	return err
}

// FinishRun marks run finished.
func (s *Step4Store) FinishRun(runID int64, status string, gate *string, errMsg *string) error {
	now := time.Now()
	if errMsg != nil {
		_, err := s.DB.Exec(`UPDATE step4_runs SET status = ?, gate_result = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ?`,
			status, gate, *errMsg, now, now, runID)
		return err
	}
	_, err := s.DB.Exec(`UPDATE step4_runs SET status = ?, gate_result = ?, finished_at = ?, updated_at = ? WHERE id = ?`,
		status, gate, now, now, runID)
	return err
}

// GetLatestRunByProject returns the most recent run for a project.
func (s *Step4Store) GetLatestRunByProject(projectID string) (*model.Step4Run, error) {
	var r model.Step4Run
	err := s.DB.Get(&r, `SELECT * FROM step4_runs WHERE project_id = ? ORDER BY id DESC LIMIT 1`, projectID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRunsByProject returns recent Step4 runs for a project.
func (s *Step4Store) ListRunsByProject(projectID string) ([]model.Step4Run, error) {
	var rows []model.Step4Run
	err := s.DB.Select(&rows, `SELECT * FROM step4_runs WHERE project_id = ? ORDER BY id DESC LIMIT 20`, projectID)
	return rows, err
}

// ListAgentRuns for a run.
func (s *Step4Store) ListAgentRuns(runID int64) ([]model.Step4AgentRun, error) {
	var rows []model.Step4AgentRun
	err := s.DB.Select(&rows, `SELECT * FROM step4_agent_runs WHERE run_id = ? ORDER BY created_at ASC`, runID)
	return rows, err
}

// StartAgentRun inserts a running agent row; returns agent row id.
func (s *Step4Store) StartAgentRun(runID int64, projectID, agentName, stage, inputSummary string) (string, error) {
	id := uuid.New().String()
	now := time.Now()
	_, err := s.DB.Exec(`INSERT INTO step4_agent_runs (id, run_id, project_id, agent_name, stage, status, input_summary, started_at, created_at)
		VALUES (?, ?, ?, ?, ?, 'running', ?, ?, ?)`, id, runID, projectID, agentName, stage, inputSummary, now, now)
	return id, err
}

// CompleteAgentRun marks agent done or failed.
func (s *Step4Store) CompleteAgentRun(agentRowID string, status, outputSummary string, errMsg *string) error {
	now := time.Now()
	var started time.Time
	_ = s.DB.Get(&started, `SELECT started_at FROM step4_agent_runs WHERE id = ?`, agentRowID)
	durationMs := int(now.Sub(started).Milliseconds())
	if errMsg != nil {
		_, err := s.DB.Exec(`UPDATE step4_agent_runs SET status = ?, output_summary = ?, error_message = ?, finished_at = ?, duration_ms = ? WHERE id = ?`,
			status, outputSummary, *errMsg, now, durationMs, agentRowID)
		return err
	}
	_, err := s.DB.Exec(`UPDATE step4_agent_runs SET status = ?, output_summary = ?, finished_at = ?, duration_ms = ? WHERE id = ?`,
		status, outputSummary, now, durationMs, agentRowID)
	return err
}

// SnapshotRequirementsFromLegacy copies tech_bid_requirement_register into step4_requirements for this run.
func (s *Step4Store) SnapshotRequirementsFromLegacy(runID int64, projectID string) error {
	rows := []struct {
		RequirementID         string `db:"requirement_id"`
		RequirementType       string `db:"requirement_type"`
		SourceText            string `db:"source_text"`
		SourceLocation        string `db:"source_location"`
		Priority              string `db:"priority"`
		MustBeExplicit        int    `db:"must_be_explicit"`
		ExpectedResponseLevel string `db:"expected_response_level"`
		Domain                string `db:"domain"`
		ResponseTier          string `db:"response_tier"`
		Summary               string `db:"summary"`
	}{}
	if err := s.DB.Select(&rows, `SELECT requirement_id, requirement_type, COALESCE(source_text,'') AS source_text, COALESCE(source_location,'') AS source_location,
		COALESCE(priority,'normal') AS priority, COALESCE(must_be_explicit,0) AS must_be_explicit,
		COALESCE(expected_response_level,'') AS expected_response_level, COALESCE(domain,'') AS domain, COALESCE(response_tier,'') AS response_tier, COALESCE(summary,'') AS summary
		FROM tech_bid_requirement_register WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	_, _ = s.DB.Exec(`DELETE FROM step4_requirements WHERE run_id = ?`, runID)
	for _, r := range rows {
		id := uuid.New().String()
		_, err := s.DB.Exec(`INSERT INTO step4_requirements (id, run_id, project_id, requirement_id, requirement_type, source_text, source_location, priority, must_be_explicit, expected_response_level, domain, response_tier, summary)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, runID, projectID, r.RequirementID, r.RequirementType, r.SourceText, r.SourceLocation, r.Priority, r.MustBeExplicit, r.ExpectedResponseLevel, r.Domain, r.ResponseTier, r.Summary)
		if err != nil {
			return err
		}
	}
	return nil
}

// SnapshotFactCandidatesFromOutlineFacts copies tech_bid_outline_facts into step4_fact_candidates (CTO Fact Agent 工件).
func (s *Step4Store) SnapshotFactCandidatesFromOutlineFacts(runID int64, projectID string) error {
	_, _ = s.DB.Exec(`DELETE FROM step4_fact_candidates WHERE run_id = ?`, runID)
	rows := []struct {
		FactCode      string `db:"fact_code"`
		FactType      string `db:"fact_type"`
		FactName      string `db:"fact_name"`
		SourceSection string `db:"source_section"`
		FactContent   string `db:"fact_content"`
	}{}
	if err := s.DB.Select(&rows, `SELECT fact_code, fact_type, COALESCE(fact_name,'') AS fact_name, COALESCE(source_section,'') AS source_section, COALESCE(fact_content,'') AS fact_content FROM tech_bid_outline_facts WHERE project_id = ? ORDER BY created_at ASC`, projectID); err != nil {
		return err
	}
	for _, r := range rows {
		id := uuid.New().String()
		snippet := r.FactContent
		if len(snippet) > 2000 {
			snippet = snippet[:2000]
		}
		_, err := s.DB.Exec(`INSERT INTO step4_fact_candidates (id, run_id, project_id, fact_id, fact_type, fact_name, source_library, source_location, confidence_score, snippet)
			VALUES (?, ?, ?, ?, ?, ?, 'tender_extract', ?, 0.85, ?)`,
			id, runID, projectID, r.FactCode, r.FactType, r.FactName, r.SourceSection, snippet)
		if err != nil {
			return err
		}
	}
	return nil
}

// SnapshotFactMappingsFromLegacy copies tech_bid_fact_mappings into step4_fact_mappings (requirement_id 使用 legacy:fact_id 占位以兼容表结构).
func (s *Step4Store) SnapshotFactMappingsFromLegacy(runID int64, projectID string) error {
	_, _ = s.DB.Exec(`DELETE FROM step4_fact_mappings WHERE run_id = ?`, runID)
	rows := []struct {
		FactID         string  `db:"fact_id"`
		FactType       string  `db:"fact_type"`
		FactName       string  `db:"fact_name"`
		TargetLevel    string  `db:"target_level"`
		TargetPathJSON string  `db:"target_path_json"`
		Required       int     `db:"required"`
		Priority       string  `db:"priority"`
		MappingReason  *string `db:"mapping_reason"`
		MappingSource  string  `db:"mapping_source"`
	}{}
	if err := s.DB.Select(&rows, `SELECT fact_id, fact_type, COALESCE(fact_name,'') AS fact_name, COALESCE(target_level,'') AS target_level, COALESCE(target_path_json,'[]') AS target_path_json,
		COALESCE(required,0) AS required, COALESCE(priority,'normal') AS priority, mapping_reason, COALESCE(mapping_source,'ai') AS mapping_source
		FROM tech_bid_fact_mappings WHERE project_id = ? ORDER BY created_at ASC`, projectID); err != nil {
		return err
	}
	for _, r := range rows {
		id := uuid.New().String()
		reqID := "legacy:" + r.FactID
		strength := "weak"
		if r.Required != 0 {
			strength = "strong"
		}
		mr := ""
		if r.MappingReason != nil {
			mr = *r.MappingReason
		}
		_, err := s.DB.Exec(`INSERT INTO step4_fact_mappings (id, run_id, project_id, requirement_id, fact_id, target_level, target_path_json, mapping_reason, mapping_source, support_strength)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, runID, projectID, reqID, r.FactID, r.TargetLevel, r.TargetPathJSON, mr, r.MappingSource, strength)
		if err != nil {
			return err
		}
	}
	return nil
}

// ResolveAuthoritativeStep4RunID picks run_id for requirement/mapping/candidate reads: active snapshot first, else latest completed with data.
func (s *Step4Store) ResolveAuthoritativeStep4RunID(projectID string) (int64, error) {
	var active sql.NullInt64
	_ = s.DB.Get(&active, `SELECT active_step4_run_id FROM tech_bid_projects WHERE id = ?`, projectID)
	if active.Valid && active.Int64 > 0 {
		var n int
		_ = s.DB.Get(&n, `SELECT COUNT(*) FROM step4_requirements WHERE run_id = ?`, active.Int64)
		if n > 0 {
			return active.Int64, nil
		}
	}
	var rid sql.NullInt64
	err := s.DB.Get(&rid, `
		SELECT r.id FROM step4_runs r
		WHERE r.project_id = ? AND r.status = 'completed'
		AND EXISTS (SELECT 1 FROM step4_requirements q WHERE q.run_id = r.id)
		ORDER BY r.id DESC LIMIT 1`, projectID)
	if err != nil {
		return 0, err
	}
	if !rid.Valid {
		return 0, sql.ErrNoRows
	}
	return rid.Int64, nil
}

// ListRequirementsAPISnapshot returns requirement rows from step4_requirements for a run (CTO 真相层读路径).
func (s *Step4Store) ListRequirementsAPISnapshot(runID int64) ([]map[string]interface{}, error) {
	var rows []struct {
		ID                    string `db:"id"`
		RequirementID         string `db:"requirement_id"`
		RequirementType       string `db:"requirement_type"`
		SourceText            string `db:"source_text"`
		SourceLocation        string `db:"source_location"`
		Priority              string `db:"priority"`
		MustBeExplicit        int    `db:"must_be_explicit"`
		ExpectedResponseLevel string `db:"expected_response_level"`
		Domain                string `db:"domain"`
		ResponseTier          string `db:"response_tier"`
		Summary               string `db:"summary"`
		CreatedAt             string `db:"created_at"`
	}
	if err := s.DB.Select(&rows, `SELECT id, requirement_id, requirement_type, source_text, COALESCE(source_location,'') AS source_location, priority, must_be_explicit,
		COALESCE(expected_response_level,'') AS expected_response_level, COALESCE(domain,'') AS domain, COALESCE(response_tier,'') AS response_tier, COALESCE(summary,'') AS summary, created_at
		FROM step4_requirements WHERE run_id = ? ORDER BY requirement_id ASC`, runID); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":                      r.ID,
			"requirement_id":          r.RequirementID,
			"requirement_type":        r.RequirementType,
			"source_text":             r.SourceText,
			"source_location":         r.SourceLocation,
			"priority":                r.Priority,
			"must_be_explicit":        r.MustBeExplicit,
			"expected_response_level": r.ExpectedResponseLevel,
			"domain":                  r.Domain,
			"response_tier":           r.ResponseTier,
			"summary":                 r.Summary,
			"created_at":              r.CreatedAt,
			"source":                  "step4",
		})
	}
	return out, nil
}

// ListFactCandidatesForRun returns step4_fact_candidates rows for API.
func (s *Step4Store) ListFactCandidatesForRun(runID int64) ([]map[string]interface{}, error) {
	var rows []struct {
		ID              string  `db:"id"`
		FactID          string  `db:"fact_id"`
		FactType        string  `db:"fact_type"`
		FactName        string  `db:"fact_name"`
		SourceLibrary   *string `db:"source_library"`
		SourceLocation  *string `db:"source_location"`
		ConfidenceScore float64 `db:"confidence_score"`
		Snippet         *string `db:"snippet"`
		CreatedAt       string  `db:"created_at"`
	}
	if err := s.DB.Select(&rows, `SELECT id, fact_id, fact_type, fact_name, source_library, source_location, confidence_score, snippet, created_at FROM step4_fact_candidates WHERE run_id = ? ORDER BY created_at ASC`, runID); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":               r.ID,
			"fact_id":          r.FactID,
			"fact_type":        r.FactType,
			"fact_name":        r.FactName,
			"source_library":   derefStringPtr(r.SourceLibrary),
			"source_location":  derefStringPtr(r.SourceLocation),
			"confidence_score": r.ConfidenceScore,
			"snippet":          derefStringPtr(r.Snippet),
			"created_at":       r.CreatedAt,
		})
	}
	return out, nil
}

func derefStringPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListFactMappingsForAPI builds the same shape as GetOutlineFactMappings from step4_fact_mappings + outline_facts.
func (s *Step4Store) ListFactMappingsForAPI(runID int64, projectID string) ([]map[string]interface{}, error) {
	type row struct {
		ID             string  `db:"id"`
		FactID         string  `db:"fact_id"`
		FactType       string  `db:"fact_type"`
		FactName       string  `db:"fact_name"`
		TargetLevel    string  `db:"target_level"`
		TargetPathJSON string  `db:"target_path_json"`
		Required       int     `db:"required"`
		Priority       string  `db:"priority"`
		MappingReason  *string `db:"mapping_reason"`
		MappingSource  string  `db:"mapping_source"`
		CreatedAt      string  `db:"created_at"`
		SourceChapter  *string `db:"source_chapter"`
		PageNumber     *int    `db:"page_number"`
		LineNumber     *int    `db:"line_number"`
		SourceLocation *string `db:"source_location"`
	}
	var rows []row
	q := `
		SELECT
			m.id,
			m.fact_id,
			COALESCE(f.fact_type, 'fact') AS fact_type,
			COALESCE(f.fact_name, m.fact_id) AS fact_name,
			m.target_level,
			m.target_path_json,
			CASE WHEN m.support_strength = 'strong' THEN 1 ELSE 0 END AS required,
			COALESCE(lm.priority, 'normal') AS priority,
			m.mapping_reason,
			COALESCE(m.mapping_source, 'ai') AS mapping_source,
			m.created_at,
			f.source_section AS source_chapter,
			f.source_page AS page_number,
			f.source_line AS line_number,
			f.source_location
		FROM step4_fact_mappings m
		LEFT JOIN tech_bid_outline_facts f ON m.project_id = f.project_id AND m.fact_id = f.fact_code
		LEFT JOIN tech_bid_fact_mappings lm ON lm.project_id = m.project_id AND lm.fact_id = m.fact_id
		WHERE m.run_id = ? AND m.project_id = ?
		ORDER BY m.fact_id ASC`
	if err := s.DB.Select(&rows, q, runID, projectID); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		var path []string
		if strings.TrimSpace(r.TargetPathJSON) != "" {
			_ = json.Unmarshal([]byte(r.TargetPathJSON), &path)
		}
		mr := ""
		if r.MappingReason != nil {
			mr = *r.MappingReason
		}
		out = append(out, map[string]interface{}{
			"id":              r.ID,
			"fact_id":         r.FactID,
			"fact_type":       r.FactType,
			"fact_name":       r.FactName,
			"target_level":    r.TargetLevel,
			"target_path":     path,
			"required":        r.Required != 0,
			"priority":        r.Priority,
			"mapping_reason":  mr,
			"mapping_source":  r.MappingSource,
			"created_at":      r.CreatedAt,
			"source_chapter":  r.SourceChapter,
			"page_number":     r.PageNumber,
			"line_number":     r.LineNumber,
			"source_location": r.SourceLocation,
			"source":          "step4",
		})
	}
	return out, nil
}

// SnapshotOutlineVersionFromDraft inserts one outline version + flat nodes from AI outline JSON.
func (s *Step4Store) SnapshotOutlineVersionFromDraft(runID int64, projectID string, versionNo int, outline []map[string]interface{}) (string, error) {
	verID := uuid.New().String()
	_, err := s.DB.Exec(`INSERT INTO step4_outline_versions (id, run_id, project_id, version_no, version_source, status, rationale, created_by, created_at)
		VALUES (?, ?, ?, ?, 'agent', 'draft', '', 'coordinator', CURRENT_TIMESTAMP)`, verID, runID, projectID, versionNo)
	if err != nil {
		return "", err
	}
	_, _ = s.DB.Exec(`DELETE FROM step4_outline_nodes WHERE outline_version_id = ?`, verID)
	order := 0
	for _, ch := range outline {
		chName, _ := ch["name"].(string)
		chID := uuid.New().String()
		order++
		_ = s.insertOutlineNode(chID, verID, nil, chName, 1, order, nil, nil)
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			uMap, _ := u.(map[string]interface{})
			uName, _ := uMap["name"].(string)
			uID := uuid.New().String()
			order++
			_ = s.insertOutlineNode(uID, verID, &chID, uName, 2, order, nil, nil)
			subs, _ := uMap["subsections"].([]interface{})
			for _, sub := range subs {
				var sName string
				var reqJSON string
				if sMap, ok := sub.(map[string]interface{}); ok {
					sName, _ = sMap["name"].(string)
					reqJSON = marshalRequirementIDsJSON(sMap["requirement_ids"])
				} else {
					sName, _ = sub.(string)
				}
				sid := uuid.New().String()
				order++
				rs := reqJSON
				_ = s.insertOutlineNode(sid, verID, &uID, sName, 3, order, &rs, nil)
			}
		}
	}
	return verID, nil
}

func (s *Step4Store) insertOutlineNode(id, verID string, parentID *string, name string, level, ord int, reqJSON, factJSON *string) error {
	_, err := s.DB.Exec(`INSERT INTO step4_outline_nodes (id, outline_version_id, parent_id, node_name, node_level, node_order, linked_requirement_ids_json, linked_fact_ids_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, verID, parentID, name, level, ord, deref(reqJSON), deref(factJSON))
	return err
}

func deref(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// SnapshotCoverageAndFullResponse stores step4_coverage_audits / step4_full_response_audits from service results.
func (s *Step4Store) SnapshotCoverageAndFullResponse(runID int64, projectID string, outlineVer int, cov *OutlineCoverageResult, fr *FullRequirementResponseResult) error {
	if cov != nil {
		id := uuid.New().String()
		mj, _ := json.Marshal(cov.MissingFactIDs)
		wj, _ := json.Marshal(cov.WeakFactIDs)
		dj, _ := json.Marshal(cov.DuplicateNodeHints)
		_, err := s.DB.Exec(`INSERT INTO step4_coverage_audits (id, run_id, project_id, outline_version, requirement_total, requirement_mapped, fact_total, fact_mapped, coverage_rate,
			missing_requirement_ids_json, weak_requirement_ids_json, missing_fact_ids_json, weak_fact_ids_json, duplicate_node_hints_json, result, summary)
			VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, runID, projectID, outlineVer, cov.FactTotal, cov.FactMapped, cov.CoverageRate, "[]", "[]", string(mj), string(wj), string(dj), cov.Result, cov.Summary)
		if err != nil {
			return err
		}
	}
	if fr != nil {
		id := uuid.New().String()
		missJ, _ := json.Marshal(fr.MissingRequirementIDs)
		weakJ, _ := json.Marshal(fr.WeakRequirementIDs)
		tagJ, _ := json.Marshal(fr.OnlyTaggedRequirementIDs)
		shellJ, _ := json.Marshal(fr.ShellTitleHints)
		highJ, _ := json.Marshal(fr.HighPriorityMissingIDs)
		mandM, _ := json.Marshal(fr.MandatoryMissingIDs)
		mandI, _ := json.Marshal(fr.MandatoryInsufficientIDs)
		hardJ, _ := json.Marshal(fr.HardRuleWarnings)
		_, err := s.DB.Exec(`INSERT INTO step4_full_response_audits (id, run_id, project_id, outline_version, requirement_total, requirement_mapped, requirement_fully_responded, requirement_weakly_responded, requirement_only_tagged,
			full_response_rate, weak_response_rate, response_quality_score, missing_requirement_ids_json, weak_requirement_ids_json, only_tagged_requirement_ids_json, shell_title_hints_json, high_priority_missing_ids_json, mandatory_missing_ids_json, mandatory_insufficient_ids_json, hard_rule_warnings_json, result, summary)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, runID, projectID, outlineVer, fr.RequirementTotal, fr.RequirementMapped, fr.RequirementFullyResponded, fr.RequirementWeaklyResponded, fr.RequirementOnlyTagged,
			fr.FullResponseRate, fr.WeakResponseRate, fr.ResponseQualityScore, string(missJ), string(weakJ), string(tagJ), string(shellJ), string(highJ), string(mandM), string(mandI), string(hardJ), fr.Result, fr.Summary)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunStatusForAPI builds payload for GET outline/run-status.
func (s *Step4Store) RunStatusForAPI(projectID string) (map[string]interface{}, error) {
	r, err := s.GetLatestRunByProject(projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]interface{}{
				"run_id":        nil,
				"project_id":    projectID,
				"status":        "idle",
				"gate_result":   nil,
				"current_stage": nil,
				"current_agent": nil,
				"progress":      0,
				"started_at":    nil,
				"last_error":    nil,
				"stages":        []interface{}{},
				"message":       "no run yet",
			}, nil
		}
		return nil, err
	}
	agents, _ := s.ListAgentRuns(r.ID)
	stages := make([]map[string]interface{}, 0, len(agents))
	progress := 0
	stageOrder := []string{"requirements_extracting", "facts_mapping", "outline_planning", "coverage_auditing", "conflict_auditing", "coordinator"}
	stageDone := map[string]bool{}
	for _, a := range agents {
		st := "pending"
		if a.Status == "done" {
			st = "done"
			stageDone[a.Stage] = true
		} else if a.Status == "failed" {
			st = "failed"
		} else if a.Status == "running" {
			st = "running"
		}
		dur := 0
		if a.DurationMs != nil {
			dur = *a.DurationMs
		}
		stages = append(stages, map[string]interface{}{
			"stage":       a.Stage,
			"agent":       a.AgentName,
			"status":      st,
			"duration_ms": dur,
		})
	}
	for i, name := range stageOrder {
		if stageDone[name] {
			progress = (i + 1) * 100 / len(stageOrder)
		}
	}
	curAgent := ""
	if r.CurrentStage != nil {
		for _, a := range agents {
			if a.Status == "running" {
				curAgent = a.AgentName
				break
			}
		}
	}
	gate := interface{}(nil)
	if r.GateResult != nil {
		gate = *r.GateResult
	}
	lastErr := interface{}(nil)
	if r.ErrorMessage != nil {
		lastErr = *r.ErrorMessage
	}
	return map[string]interface{}{
		"run_id":        r.ID,
		"project_id":    projectID,
		"status":        r.Status,
		"gate_result":   gate,
		"current_stage": r.CurrentStage,
		"current_agent": curAgent,
		"progress":      progress,
		"started_at":    r.StartedAt,
		"last_error":    lastErr,
		"stages":        stages,
	}, nil
}

// LogApproval writes step4_approval_logs.
func (s *Step4Store) LogApproval(runID int64, projectID, stage, action, operatorID, reason string) error {
	id := uuid.New().String()
	_, err := s.DB.Exec(`INSERT INTO step4_approval_logs (id, run_id, project_id, stage, action, operator_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		id, runID, projectID, stage, action, nullIfEmpty(operatorID), reason)
	return err
}

// ListApprovalLogs returns recent approval actions for a project.
func (s *Step4Store) ListApprovalLogs(projectID string) ([]map[string]interface{}, error) {
	var rows []struct {
		ID              string     `db:"id"`
		RunID           int64      `db:"run_id"`
		Stage           string     `db:"stage"`
		Action          string     `db:"action"`
		OperatorID      *string    `db:"operator_id"`
		Reason          *string    `db:"reason"`
		SnapshotVersion *string    `db:"snapshot_version"`
		CreatedAt       *time.Time `db:"created_at"`
	}
	if err := s.DB.Select(&rows, `SELECT id, run_id, stage, action, operator_id, reason, snapshot_version, created_at FROM step4_approval_logs WHERE project_id = ? ORDER BY created_at DESC LIMIT 50`, projectID); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		var createdAt interface{}
		if r.CreatedAt != nil {
			createdAt = r.CreatedAt
		}
		out = append(out, map[string]interface{}{
			"id":               r.ID,
			"run_id":           r.RunID,
			"stage":            r.Stage,
			"action":           r.Action,
			"operator_id":      derefStringPtr(r.OperatorID),
			"reason":           derefStringPtr(r.Reason),
			"snapshot_version": derefStringPtr(r.SnapshotVersion),
			"created_at":       createdAt,
		})
	}
	return out, nil
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// AppendConflictSnapshot stores conflict audit in step4_conflict_audits
func (s *Step4Store) AppendConflictSnapshot(runID int64, projectID string, outlineVer int, audit *ConflictAuditResult) error {
	if audit == nil {
		return nil
	}
	cj, _ := json.Marshal(audit.Conflicts)
	id := uuid.New().String()
	hasBlock := 0
	if audit.HasBlock {
		hasBlock = 1
	}
	_, err := s.DB.Exec(`INSERT INTO step4_conflict_audits (id, run_id, project_id, outline_version, has_block, conflicts_json, summary) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, runID, projectID, outlineVer, hasBlock, string(cj), audit.Summary)
	return err
}

// Step4OutlineNodeRow is one row from step4_outline_nodes (ordered list walk).
type Step4OutlineNodeRow struct {
	ID            string  `db:"id" json:"id"`
	ParentID      *string `db:"parent_id" json:"parent_id,omitempty"`
	NodeName      string  `db:"node_name" json:"node_name"`
	NodeLevel     int     `db:"node_level" json:"node_level"`
	NodeOrder     int     `db:"node_order" json:"node_order"`
	LinkedReqJSON *string `db:"linked_requirement_ids_json" json:"linked_requirement_ids_json,omitempty"`
}

// ListNodesForVersion returns nodes in DFS order for rebuilding outline JSON.
func (s *Step4Store) ListNodesForVersion(versionID string) ([]Step4OutlineNodeRow, error) {
	var rows []Step4OutlineNodeRow
	err := s.DB.Select(&rows, `SELECT id, parent_id, node_name, node_level, node_order, linked_requirement_ids_json FROM step4_outline_nodes WHERE outline_version_id = ? ORDER BY node_order ASC`, versionID)
	return rows, err
}

// OutlineNodesToOutlineJSON rebuilds []map from flat node rows (matches SnapshotOutlineVersionFromDraft shape).
func OutlineNodesToOutlineJSON(nodes []Step4OutlineNodeRow) []map[string]interface{} {
	var chapters []map[string]interface{}
	var curCh map[string]interface{}
	var curUnit map[string]interface{}
	for _, n := range nodes {
		switch n.NodeLevel {
		case 1:
			ch := map[string]interface{}{"name": n.NodeName, "units": []interface{}{}}
			chapters = append(chapters, ch)
			curCh = ch
			curUnit = nil
		case 2:
			u := map[string]interface{}{"name": n.NodeName, "subsections": []interface{}{}}
			if curCh != nil {
				curCh["units"] = append(curCh["units"].([]interface{}), u)
			}
			curUnit = u
		case 3:
			var reqIDs []string
			if n.LinkedReqJSON != nil && strings.TrimSpace(*n.LinkedReqJSON) != "" {
				_ = json.Unmarshal([]byte(*n.LinkedReqJSON), &reqIDs)
			}
			sub := map[string]interface{}{"name": n.NodeName, "requirement_ids": reqIDs}
			if curUnit != nil {
				curUnit["subsections"] = append(curUnit["subsections"].([]interface{}), sub)
			}
		}
	}
	return chapters
}

// ListOutlineVersionsForLatestRun lists step4_outline_versions for the latest run of a project.
func (s *Step4Store) ListOutlineVersionsForLatestRun(projectID string) ([]map[string]interface{}, error) {
	var runID sql.NullInt64
	if err := s.DB.Get(&runID, `SELECT id FROM step4_runs WHERE project_id = ? ORDER BY id DESC LIMIT 1`, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	if !runID.Valid {
		return nil, nil
	}
	var rows []struct {
		ID            string `db:"id"`
		VersionNo     int    `db:"version_no"`
		VersionSource string `db:"version_source"`
		Status        string `db:"status"`
		Rationale     string `db:"rationale"`
		CreatedBy     string `db:"created_by"`
		CreatedAt     string `db:"created_at"`
	}
	if err := s.DB.Select(&rows, `SELECT id, version_no, version_source, status, COALESCE(rationale,'') AS rationale, COALESCE(created_by,'') AS created_by, created_at FROM step4_outline_versions WHERE run_id = ? ORDER BY version_no ASC`, runID.Int64); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":             r.ID,
			"version_no":     r.VersionNo,
			"version_source": r.VersionSource,
			"status":         r.Status,
			"rationale":      r.Rationale,
			"created_by":     r.CreatedBy,
			"created_at":     r.CreatedAt,
			"run_id":         runID.Int64,
		})
	}
	return out, nil
}

// SetRecommendedOutlineVersion marks one version recommended and archives siblings in the same run.
func (s *Step4Store) SetRecommendedOutlineVersion(projectID, versionID string) error {
	var runID int64
	if err := s.DB.Get(&runID, `SELECT run_id FROM step4_outline_versions WHERE id = ? AND project_id = ?`, versionID, projectID); err != nil {
		return err
	}
	if _, err := s.DB.Exec(`UPDATE step4_outline_versions SET status = 'archived' WHERE run_id = ? AND project_id = ? AND id != ?`, runID, projectID, versionID); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE step4_outline_versions SET status = 'recommended' WHERE id = ?`, versionID)
	return err
}

// LatestCompletedStep4Coverage returns newest step4_coverage_audits row for a completed run, if any.
func (s *Step4Store) LatestCompletedStep4Coverage(projectID string) (map[string]interface{}, error) {
	var row struct {
		ID                 string  `db:"id"`
		OutlineVersion     *int    `db:"outline_version"`
		FactTotal          int     `db:"fact_total"`
		FactMapped         int     `db:"fact_mapped"`
		CoverageRate       float64 `db:"coverage_rate"`
		MissingFactIDsJSON *string `db:"missing_fact_ids_json"`
		WeakFactIDsJSON    *string `db:"weak_fact_ids_json"`
		DuplicateHintsJSON *string `db:"duplicate_node_hints_json"`
		Result             string  `db:"result"`
		Summary            *string `db:"summary"`
		CreatedAt          string  `db:"created_at"`
	}
	err := s.DB.Get(&row, `
		SELECT c.id, c.outline_version, c.fact_total, c.fact_mapped, c.coverage_rate,
			c.missing_fact_ids_json, c.weak_fact_ids_json, c.duplicate_node_hints_json, c.result, c.summary, c.created_at
		FROM step4_coverage_audits c
		INNER JOIN step4_runs r ON r.id = c.run_id
		WHERE r.project_id = ? AND r.status = 'completed'
		ORDER BY c.created_at DESC LIMIT 1`, projectID)
	if err != nil {
		return nil, err
	}
	parseArr := func(p *string) []string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []string
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	ov := 1
	if row.OutlineVersion != nil {
		ov = *row.OutlineVersion
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	return map[string]interface{}{
		"id":                   row.ID,
		"outline_version":      ov,
		"fact_total":           row.FactTotal,
		"fact_mapped":          row.FactMapped,
		"coverage_rate":        row.CoverageRate,
		"missing_fact_ids":     parseArr(row.MissingFactIDsJSON),
		"weak_fact_ids":        parseArr(row.WeakFactIDsJSON),
		"duplicate_node_hints": parseArr(row.DuplicateHintsJSON),
		"result":               row.Result,
		"summary":              summary,
		"created_at":           row.CreatedAt,
		"source":               "step4",
	}, nil
}

// LatestCompletedStep4FullResponse returns newest step4_full_response_audits for a completed run.
func (s *Step4Store) LatestCompletedStep4FullResponse(projectID string) (map[string]interface{}, error) {
	var row struct {
		ID                           string  `db:"id"`
		OutlineVersion               *int    `db:"outline_version"`
		RequirementTotal             int     `db:"requirement_total"`
		RequirementMapped            int     `db:"requirement_mapped"`
		RequirementFullyResponded    int     `db:"requirement_fully_responded"`
		RequirementWeaklyResponded   int     `db:"requirement_weakly_responded"`
		RequirementOnlyTagged        int     `db:"requirement_only_tagged"`
		FullResponseRate             float64 `db:"full_response_rate"`
		WeakResponseRate             float64 `db:"weak_response_rate"`
		ResponseQualityScore         float64 `db:"response_quality_score"`
		MissingRequirementIDsJSON    *string `db:"missing_requirement_ids_json"`
		WeakRequirementIDsJSON       *string `db:"weak_requirement_ids_json"`
		OnlyTaggedRequirementIDsJSON *string `db:"only_tagged_requirement_ids_json"`
		ShellTitleHintsJSON          *string `db:"shell_title_hints_json"`
		HighPriorityMissingIDsJSON   *string `db:"high_priority_missing_ids_json"`
		MandatoryMissingIDsJSON      *string `db:"mandatory_missing_ids_json"`
		MandatoryInsufficientIDsJSON *string `db:"mandatory_insufficient_ids_json"`
		HardRuleWarningsJSON         *string `db:"hard_rule_warnings_json"`
		Result                       string  `db:"result"`
		Summary                      *string `db:"summary"`
		CreatedAt                    string  `db:"created_at"`
	}
	err := s.DB.Get(&row, `
		SELECT f.id, f.outline_version, f.requirement_total, f.requirement_mapped, f.requirement_fully_responded, f.requirement_weakly_responded, f.requirement_only_tagged,
			f.full_response_rate, f.weak_response_rate, f.response_quality_score,
			f.missing_requirement_ids_json, f.weak_requirement_ids_json, f.only_tagged_requirement_ids_json, f.shell_title_hints_json,
			f.high_priority_missing_ids_json, f.mandatory_missing_ids_json, f.mandatory_insufficient_ids_json, f.hard_rule_warnings_json,
			f.result, f.summary, f.created_at
		FROM step4_full_response_audits f
		INNER JOIN step4_runs r ON r.id = f.run_id
		WHERE r.project_id = ? AND r.status = 'completed'
		ORDER BY f.created_at DESC LIMIT 1`, projectID)
	if err != nil {
		return nil, err
	}
	parseArr := func(p *string) []string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []string
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	ov := 1
	if row.OutlineVersion != nil {
		ov = *row.OutlineVersion
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	return map[string]interface{}{
		"id":                           row.ID,
		"outline_version":              ov,
		"requirement_total":            row.RequirementTotal,
		"requirement_mapped":           row.RequirementMapped,
		"requirement_fully_responded":  row.RequirementFullyResponded,
		"requirement_weakly_responded": row.RequirementWeaklyResponded,
		"requirement_only_tagged":      row.RequirementOnlyTagged,
		"full_response_rate":           row.FullResponseRate,
		"weak_response_rate":           row.WeakResponseRate,
		"response_quality_score":       row.ResponseQualityScore,
		"missing_requirement_ids":      parseArr(row.MissingRequirementIDsJSON),
		"weak_requirement_ids":         parseArr(row.WeakRequirementIDsJSON),
		"only_tagged_requirement_ids":  parseArr(row.OnlyTaggedRequirementIDsJSON),
		"shell_title_hints":            parseArr(row.ShellTitleHintsJSON),
		"high_priority_missing_ids":    parseArr(row.HighPriorityMissingIDsJSON),
		"mandatory_missing_ids":        parseArr(row.MandatoryMissingIDsJSON),
		"mandatory_insufficient_ids":   parseArr(row.MandatoryInsufficientIDsJSON),
		"hard_rule_warnings":           parseArr(row.HardRuleWarningsJSON),
		"result":                       row.Result,
		"summary":                      summary,
		"created_at":                   row.CreatedAt,
		"source":                       "step4",
	}, nil
}

// LatestCompletedStep4Conflict returns newest step4_conflict_audits for a completed run.
func (s *Step4Store) LatestCompletedStep4Conflict(projectID string) (map[string]interface{}, error) {
	var row struct {
		ID            string  `db:"id"`
		HasBlock      int     `db:"has_block"`
		ConflictsJSON *string `db:"conflicts_json"`
		Summary       *string `db:"summary"`
		CreatedAt     string  `db:"created_at"`
	}
	err := s.DB.Get(&row, `
		SELECT c.id, c.has_block, c.conflicts_json, c.summary, c.created_at
		FROM step4_conflict_audits c
		INNER JOIN step4_runs r ON r.id = c.run_id
		WHERE r.project_id = ? AND r.status = 'completed'
		ORDER BY c.created_at DESC LIMIT 1`, projectID)
	if err != nil {
		return nil, err
	}
	parseConflicts := func(p *string) []interface{} {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		var xs []interface{}
		if json.Unmarshal([]byte(*p), &xs) != nil {
			return nil
		}
		return xs
	}
	summary := ""
	if row.Summary != nil {
		summary = *row.Summary
	}
	return map[string]interface{}{
		"id":         row.ID,
		"has_block":  row.HasBlock != 0,
		"conflicts":  parseConflicts(row.ConflictsJSON),
		"summary":    summary,
		"created_at": row.CreatedAt,
		"source":     "step4",
	}, nil
}
