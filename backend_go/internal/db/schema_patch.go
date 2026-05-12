package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
)

// ApplySchemaPatches 在启动时追加列等轻量变更（SQLite 无 IF NOT EXISTS 列语法，重复执行时忽略 duplicate column）。
func ApplySchemaPatches(database *sqlx.DB) {
	execCreate := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if strings.Contains(em, "already exists") {
				return
			}
			qPreview := q
			if len(qPreview) > 80 {
				qPreview = qPreview[:80] + "..."
			}
			log.Printf("[db] schema create: %v — %s", err, qPreview)
		} else {
			log.Printf("[db] schema create applied")
		}
	}

	ensureClosedLoopOptimizationSchema(database)

	execCreate(`CREATE TABLE IF NOT EXISTS knowledge_extract_task (
		id TEXT PRIMARY KEY,
		company_id TEXT NOT NULL,
		knowledge_type TEXT NOT NULL,
		source_origin TEXT NOT NULL,
		source_project_id TEXT NOT NULL,
		source_project_name TEXT,
		source_file_id TEXT NOT NULL,
		extract_scope TEXT NOT NULL,
		selected_sections_json TEXT,
		prompt_template_id INTEGER,
		prompt_key TEXT,
		prompt_version INTEGER,
		prompt_override TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	execCreate(`CREATE TABLE IF NOT EXISTS knowledge_extract_result (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		title TEXT,
		content_json TEXT NOT NULL,
		source_section TEXT,
		selected_flag INTEGER DEFAULT 1,
		saved_knowledge_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	execCreate(`CREATE INDEX IF NOT EXISTS idx_knowledge_extract_task_company ON knowledge_extract_task(company_id, status)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_knowledge_extract_result_task ON knowledge_extract_result(task_id)`)

	execCreate(`CREATE TABLE IF NOT EXISTS person_education (
		id TEXT PRIMARY KEY,
		person_id TEXT NOT NULL,
		start_date TEXT,
		end_date TEXT,
		school TEXT,
		degree TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_person_education_person_id ON person_education(person_id)`)

	execCreate(`CREATE TABLE IF NOT EXISTS person_work_experience (
		id TEXT PRIMARY KEY,
		person_id TEXT NOT NULL,
		start_date TEXT,
		end_date TEXT,
		company TEXT,
		position TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_person_work_experience_person_id ON person_work_experience(person_id)`)

	execCreate(`CREATE TABLE IF NOT EXISTS financial_report_folder (
		id TEXT PRIMARY KEY,
		company_id TEXT NOT NULL,
		folder_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_financial_report_folder_company ON financial_report_folder(company_id)`)

	execCreate(`CREATE TABLE IF NOT EXISTS other_folder (
		id TEXT PRIMARY KEY,
		company_id TEXT NOT NULL,
		folder_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_other_folder_company ON other_folder(company_id)`)

	execCreate(`CREATE TABLE IF NOT EXISTS tech_bid_industry_skeletons (
		id TEXT PRIMARY KEY,
		industry_name TEXT NOT NULL UNIQUE,
		parent_id TEXT DEFAULT NULL,
		logical_chapters_json TEXT NOT NULL,
		common_section_pool_json TEXT,
		industry_keywords_json TEXT,
		title_candidate_pool_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	execCreate(`CREATE INDEX IF NOT EXISTS idx_skeleton_industry_name ON tech_bid_industry_skeletons(industry_name)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_skeleton_parent_id ON tech_bid_industry_skeletons(parent_id)`)

	// Ensure parent_id column exists on existing DBs; ALTER is safe to re-run (duplicate-column guard).
	skeletonPatches := []string{
		`ALTER TABLE tech_bid_industry_skeletons ADD COLUMN parent_id TEXT DEFAULT NULL`,
		`ALTER TABLE tech_bid_industry_skeletons ADD COLUMN matching_rules_json TEXT DEFAULT NULL`,
	}
	for _, q := range skeletonPatches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied (tech_bid_industry_skeletons.parent_id)")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}
	// Normalize legacy empty-string parent_id to proper NULL for strict L1 filtering
	database.Exec(`UPDATE tech_bid_industry_skeletons SET parent_id = NULL WHERE parent_id = ''`)

	patches := []string{
		`ALTER TABLE company_profile ADD COLUMN legal_person_id_card TEXT`,
		`ALTER TABLE person ADD COLUMN specialty TEXT`,
		`ALTER TABLE person ADD COLUMN gender TEXT`,
		`ALTER TABLE person ADD COLUMN join_date TEXT`,
		`ALTER TABLE tech_bid_knowledge_items ADD COLUMN knowledge_status TEXT DEFAULT 'published'`,
		`ALTER TABLE tech_bid_knowledge_items ADD COLUMN manually_edited INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_knowledge_items ADD COLUMN extract_task_id TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN final_decision TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN can_enter_content_generation INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN verification_method TEXT DEFAULT ''`,
		`ALTER TABLE tech_bid_projects ADD COLUMN verification_summary TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN override_enabled INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN override_reason TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN override_by TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN override_at DATETIME`,
		`ALTER TABLE tech_bid_projects ADD COLUMN manual_review_required INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN manual_review_result TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN manual_review_reason TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN manual_review_by TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN manual_review_at DATETIME`,
		`ALTER TABLE tech_bid_projects ADD COLUMN current_progress INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN full_response_rate REAL DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_gate_result TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_gate_reason TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_override_enabled INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_override_reason TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_override_by TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN step4_override_at DATETIME`,
		`ALTER TABLE tech_bid_projects ADD COLUMN outline_fingerprint TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN history_similarity_hint TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN outline_titles_json TEXT`,
		`ALTER TABLE tech_bid_requirement_response_checks ADD COLUMN high_priority_missing_ids_json TEXT`,
		`ALTER TABLE tech_bid_requirement_response_checks ADD COLUMN mandatory_missing_ids_json TEXT`,
		`ALTER TABLE tech_bid_requirement_response_checks ADD COLUMN mandatory_insufficient_ids_json TEXT`,
		`ALTER TABLE tech_bid_requirement_response_checks ADD COLUMN hard_rule_warnings_json TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN structure_plan_status TEXT DEFAULT 'pending'`,
		`ALTER TABLE tech_bid_projects ADD COLUMN structure_plan_version INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN personalization_score REAL DEFAULT 0`,
		`ALTER TABLE tech_bid_projects ADD COLUMN structure_profile_json TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN last_structure_reject_reason TEXT`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN schema_version TEXT DEFAULT 'v1'`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN extraction_meta_json TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN chapter_draft_json TEXT`,
		`ALTER TABLE tech_bid_projects ADD COLUMN confirmed_chapters_json TEXT`,
	}
	for _, q := range patches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}

	// tech_bid_outline_audits: older SQLite schema (multi_agent_outline) lacked snapshot columns; handler expects full model.
	outlineAuditPatches := []string{
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN outline_snapshot_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN facts_snapshot_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN audit_version TEXT DEFAULT 'v1'`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN audit_summary TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN final_decision TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN risk_level TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN can_proceed INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN duplicate_items_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN missing_items_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN weak_items_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN detail_json TEXT`,
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN audit_model TEXT`,
		// Refinement-style tables may omit outline_version; older tables require it — default makes INSERT safe.
		`ALTER TABLE tech_bid_outline_audits ADD COLUMN outline_version INTEGER DEFAULT 1`,
	}
	for _, q := range outlineAuditPatches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied (tech_bid_outline_audits)")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}

	// Idempotent: PRAGMA + ADD COLUMN only when missing (fixes cases where ALTER batch failed silently).
	EnsureTechBidOutlineAuditsSchema(database)
	EnsureTechBidOutlineVerificationsSchema(database)

	// Cache metrics table for observability
	execCreate(`CREATE TABLE IF NOT EXISTS cache_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		step TEXT NOT NULL,
		model TEXT NOT NULL,
		cache_mode TEXT NOT NULL DEFAULT 'none',
		prompt_tokens INTEGER DEFAULT 0,
		cached_tokens INTEGER DEFAULT 0,
		cache_creation_input_tokens INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		latency_ms INTEGER DEFAULT 0,
		request_success INTEGER DEFAULT 1,
		prompt_version TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_cache_metrics_project ON cache_metrics(project_id)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_cache_metrics_step ON cache_metrics(step)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_cache_metrics_created ON cache_metrics(created_at)`)

	ensureStep4StrongMappingSchema(database)
	ensureStep4MultiAgentSchema(database)
	ensureTechBidProjectProfileSchema(database)
	EnsureSkeletonSelectionSchema(database)
	seedDefaultKnowledgeExtractPrompts(database)
	seedTechBidFactOutlineMappingPrompt(database)
	seedTechBidStructureProfilerPrompt(database)
	seedTechBidOutlineReformPrompts(database)
}

func seedTechBidOutlineReformPrompts(database *sqlx.DB) {
	// 1. 一级章生成 Prompt
	var n1 int
	if err := database.Get(&n1, `SELECT COUNT(*) FROM prompt_template WHERE prompt_key = 'tech_bid_outline_chapter_generation'`); err == nil && n1 == 0 {
		const content = `你是建筑行业资深技术标总编，负责为项目构建顶级章节目录（一级章）。
你需要基于项目画像、招标文件要求和行业惯例，提炼出最符合本项目特点的一级章节。

### 输出要求
1. 只输出 JSON 数组，包含章节标题字符串。
2. 章节数量通常在 8-15 章之间，逻辑清晰，涵盖项目核心管理、技术施工、质量安全、资源保障等大项。
3. 必须包含招标文件明确要求的强制性章节。
4. 输出示例：["第一章 编制说明及依据", "第二章 工程概况及特点", ...]。

不要输出任何 Markdown 格式或额外解释，仅返回合法的 JSON 数组。`
		const sys = `你是一名技术标专家，只输出 JSON 数组。`
		_, _ = database.Exec(`
			INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, variables, status, version, remark, system_content)
			VALUES ('tech_bid_outline_chapter_generation', '技术标-两步走-一级章生成', 100, 'tech_bid_step4', ?, '', 1, 1, '新目录流程阶段 A', ?)
		`, content, sys)
	}

	// 2. 目录展开 Prompt
	var n2 int
	if err := database.Get(&n2, `SELECT COUNT(*) FROM prompt_template WHERE prompt_key = 'tech_bid_outline_units_subsections_generation'`); err == nil && n2 == 0 {
		const content = `你是建筑行业技术标编排专家。现在已确定了技术标的一级章骨架，你需要根据每个章的内容定位，为其补充二层（节）和三层（小节）。

### 输入上下文
1. 已确认的一级章：{{confirmed_chapters}}
2. 项目画像与招标文件事实。

### 输出要求
1. 返回完整的树形目录 JSON 结构。
2. 内部逻辑必须严密，二级节（units）应涵盖该章的所有核心要点，三级节（subsections）应对应具体的施工工艺、质量措施或规范要求。
3. 输出 JSON 格式：
[
  {
    "title": "第一章 XXX",
    "units": [
      {
        "title": "第一节 XXX",
        "subsections": [
          {"title": "1.1 XXX"},
          {"title": "1.2 XXX"}
        ]
      }
    ]
  },
  ...
]

不要输出任何 Markdown 格式或额外解释，仅返回合法的 JSON。`
		const sys = `你是一名技术标专家，只输出 JSON 完整目录结构。`
		_, _ = database.Exec(`
			INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, variables, status, version, remark, system_content)
			VALUES ('tech_bid_outline_units_subsections_generation', '技术标-两步走-目录树展开', 100, 'tech_bid_step4', ?, 'confirmed_chapters', 1, 1, '新目录流程阶段 B', ?)
		`, content, sys)
	}
}

func ensureTechBidProjectProfileSchema(database *sqlx.DB) {
	execCreate := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if !strings.Contains(em, "already exists") {
				qPreview := q
				if len(qPreview) > 80 {
					qPreview = qPreview[:80] + "..."
				}
				log.Printf("[db] project profile schema: %v — %s", err, qPreview)
			}
		} else {
			log.Printf("[db] project profile schema create applied")
		}
	}

	execCreate(`CREATE TABLE IF NOT EXISTS tech_bid_profile_extraction_snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		profile_id TEXT,
		stage TEXT NOT NULL,
		chunk_index INTEGER DEFAULT 0,
		payload_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_profile_snapshot_project ON tech_bid_profile_extraction_snapshots(project_id)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_profile_snapshot_profile ON tech_bid_profile_extraction_snapshots(profile_id)`)

	patches := []string{
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN schema_version TEXT DEFAULT 'v1'`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN extraction_meta_json TEXT`,
		`ALTER TABLE tech_bid_profile_extraction_snapshots ADD COLUMN run_id TEXT`,
		`ALTER TABLE tech_bid_profile_extraction_snapshots ADD COLUMN file_id TEXT`,
		`ALTER TABLE tech_bid_profile_extraction_snapshots ADD COLUMN chunk_type TEXT DEFAULT 'narrative'`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN confirmed_at DATETIME`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN confirmed_by TEXT`,
		`ALTER TABLE tech_bid_project_profiles ADD COLUMN edit_count INTEGER DEFAULT 0`,
	}
	for _, q := range patches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied (tech_bid_project_profiles)")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}

	// Edit history table for manual corrections
	execCreate(`CREATE TABLE IF NOT EXISTS tech_bid_profile_edit_history (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		profile_id TEXT NOT NULL,
		field_path TEXT NOT NULL,
		old_value_json TEXT,
		new_value_json TEXT NOT NULL,
		edit_source TEXT DEFAULT 'manual',
		operator_name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	execCreate(`CREATE INDEX IF NOT EXISTS idx_profile_edit_history_project ON tech_bid_profile_edit_history(project_id)`)
}

// ensureStep4StrongMappingSchema CTO 第四步：facts→目录映射表、覆盖率结果表、事实 fact_code
func ensureStep4StrongMappingSchema(database *sqlx.DB) {
	patches := []string{
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN fact_code TEXT`,
	}
	for _, q := range patches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied (step4 strong mapping)")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}

	exec := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if !strings.Contains(em, "already exists") {
				qPreview := q
				if len(qPreview) > 80 {
					qPreview = qPreview[:80] + "..."
				}
				log.Printf("[db] step4 schema: %v — %s", err, qPreview)
			}
		} else {
			log.Printf("[db] step4 schema create applied")
		}
	}
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_fact_mappings (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		fact_id TEXT NOT NULL,
		fact_type TEXT,
		fact_name TEXT,
		target_level TEXT,
		target_path_json TEXT,
		required INTEGER DEFAULT 1,
		priority TEXT,
		mapping_reason TEXT,
		mapping_source TEXT DEFAULT 'ai',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_fact_mappings_project ON tech_bid_fact_mappings(project_id)`)
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_outline_coverage_checks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		outline_version INTEGER DEFAULT 1,
		fact_total INTEGER,
		fact_mapped INTEGER,
		coverage_rate REAL,
		missing_fact_ids_json TEXT,
		weak_fact_ids_json TEXT,
		duplicate_node_ids_json TEXT,
		result TEXT,
		summary TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_coverage_checks_project ON tech_bid_outline_coverage_checks(project_id)`)
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_requirement_register (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		requirement_id TEXT NOT NULL,
		requirement_type TEXT,
		source_text TEXT,
		source_location TEXT,
		priority TEXT,
		must_be_explicit INTEGER DEFAULT 0,
		expected_response_level TEXT,
		domain TEXT,
		response_tier TEXT,
		summary TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_req_reg_proj_rid ON tech_bid_requirement_register(project_id, requirement_id)`)
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_requirement_response_checks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		outline_version INTEGER DEFAULT 1,
		requirement_total INTEGER,
		requirement_mapped INTEGER,
		requirement_fully_responded INTEGER,
		requirement_weakly_responded INTEGER,
		requirement_only_tagged INTEGER,
		full_response_rate REAL,
		weak_response_rate REAL,
		response_quality_score REAL,
		missing_requirement_ids_json TEXT,
		weak_requirement_ids_json TEXT,
		only_tagged_requirement_ids_json TEXT,
		shell_title_hints_json TEXT,
		high_priority_missing_ids_json TEXT,
		mandatory_missing_ids_json TEXT,
		mandatory_insufficient_ids_json TEXT,
		result TEXT,
		summary TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_req_resp_project ON tech_bid_requirement_response_checks(project_id)`)
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_step4_gate_overrides (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		company_id TEXT NOT NULL,
		operator_id TEXT,
		reason TEXT,
		gate_snapshot_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_gate_ov_proj ON tech_bid_step4_gate_overrides(project_id)`)
}

// ensureStep4MultiAgentSchema CTO Step4 Coordinator：运行记录、Agent 日志与各阶段快照表（SQLite）
func ensureStep4MultiAgentSchema(database *sqlx.DB) {
	exec := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if !strings.Contains(em, "already exists") {
				qPreview := q
				if len(qPreview) > 80 {
					qPreview = qPreview[:80] + "..."
				}
				log.Printf("[db] step4 multi-agent schema: %v — %s", err, qPreview)
			}
		} else {
			log.Printf("[db] step4 multi-agent schema create applied")
		}
	}

	exec(`CREATE TABLE IF NOT EXISTS step4_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		outline_version INTEGER DEFAULT NULL,
		trigger_source TEXT NOT NULL DEFAULT 'api',
		operator_id TEXT,
		status TEXT NOT NULL DEFAULT 'idle',
		current_stage TEXT,
		gate_result TEXT,
		started_at DATETIME,
		finished_at DATETIME,
		error_message TEXT,
		retry_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_runs_project ON step4_runs(project_id)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_runs_status ON step4_runs(status)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_requirements (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		requirement_id TEXT NOT NULL,
		requirement_type TEXT NOT NULL,
		source_text TEXT NOT NULL,
		source_location TEXT,
		priority TEXT NOT NULL DEFAULT 'normal',
		must_be_explicit INTEGER NOT NULL DEFAULT 0,
		expected_response_level TEXT,
		domain TEXT,
		response_tier TEXT,
		risk_level TEXT,
		summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_req_run ON step4_requirements(run_id)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_req_project ON step4_requirements(project_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_fact_candidates (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		fact_id TEXT NOT NULL,
		fact_type TEXT NOT NULL,
		fact_name TEXT NOT NULL,
		source_library TEXT,
		source_location TEXT,
		confidence_score REAL NOT NULL DEFAULT 0,
		snippet TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_fc_run ON step4_fact_candidates(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_fact_mappings (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		requirement_id TEXT NOT NULL,
		fact_id TEXT NOT NULL,
		target_level TEXT,
		target_path_json TEXT,
		mapping_reason TEXT,
		mapping_source TEXT,
		support_strength TEXT NOT NULL DEFAULT 'weak',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_fm_run ON step4_fact_mappings(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_outline_versions (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		version_no INTEGER NOT NULL,
		version_source TEXT NOT NULL DEFAULT 'agent',
		status TEXT NOT NULL DEFAULT 'draft',
		rationale TEXT,
		created_by TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE UNIQUE INDEX IF NOT EXISTS uk_step4_ov_run_ver ON step4_outline_versions(run_id, version_no)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_outline_nodes (
		id TEXT PRIMARY KEY,
		outline_version_id TEXT NOT NULL,
		parent_id TEXT,
		node_name TEXT NOT NULL,
		node_level INTEGER NOT NULL,
		node_order INTEGER NOT NULL DEFAULT 0,
		response_goal TEXT,
		linked_requirement_ids_json TEXT,
		linked_fact_ids_json TEXT,
		node_summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_on_version ON step4_outline_nodes(outline_version_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_coverage_audits (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		outline_version INTEGER,
		requirement_total INTEGER NOT NULL DEFAULT 0,
		requirement_mapped INTEGER NOT NULL DEFAULT 0,
		fact_total INTEGER NOT NULL DEFAULT 0,
		fact_mapped INTEGER NOT NULL DEFAULT 0,
		coverage_rate REAL NOT NULL DEFAULT 0,
		missing_requirement_ids_json TEXT,
		weak_requirement_ids_json TEXT,
		missing_fact_ids_json TEXT,
		weak_fact_ids_json TEXT,
		duplicate_node_hints_json TEXT,
		result TEXT NOT NULL DEFAULT 'REVISE',
		summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_cov_run ON step4_coverage_audits(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_full_response_audits (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		outline_version INTEGER,
		requirement_total INTEGER NOT NULL DEFAULT 0,
		requirement_mapped INTEGER NOT NULL DEFAULT 0,
		requirement_fully_responded INTEGER NOT NULL DEFAULT 0,
		requirement_weakly_responded INTEGER NOT NULL DEFAULT 0,
		requirement_only_tagged INTEGER NOT NULL DEFAULT 0,
		full_response_rate REAL NOT NULL DEFAULT 0,
		weak_response_rate REAL NOT NULL DEFAULT 0,
		response_quality_score REAL NOT NULL DEFAULT 0,
		missing_requirement_ids_json TEXT,
		weak_requirement_ids_json TEXT,
		only_tagged_requirement_ids_json TEXT,
		shell_title_hints_json TEXT,
		high_priority_missing_ids_json TEXT,
		mandatory_missing_ids_json TEXT,
		mandatory_insufficient_ids_json TEXT,
		hard_rule_warnings_json TEXT,
		result TEXT NOT NULL DEFAULT 'REVISE',
		summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_fr_run ON step4_full_response_audits(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_conflict_audits (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		outline_version INTEGER,
		has_block INTEGER NOT NULL DEFAULT 0,
		conflicts_json TEXT,
		duplicate_node_hints_json TEXT,
		summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_ca_run ON step4_conflict_audits(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_structure_plans (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		outline_version INTEGER,
		adjustments_json TEXT,
		rationale TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		approved_by TEXT,
		approved_at DATETIME,
		rejected_reason TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_sp_run ON step4_structure_plans(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_approval_logs (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		stage TEXT NOT NULL,
		action TEXT NOT NULL,
		operator_id TEXT,
		reason TEXT,
		snapshot_version TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_al_run ON step4_approval_logs(run_id)`)

	exec(`CREATE TABLE IF NOT EXISTS step4_agent_runs (
		id TEXT PRIMARY KEY,
		run_id INTEGER NOT NULL,
		project_id TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		stage TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'running',
		input_summary TEXT,
		output_summary TEXT,
		error_message TEXT,
		started_at DATETIME,
		finished_at DATETIME,
		duration_ms INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_ar_run ON step4_agent_runs(run_id)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_step4_ar_project ON step4_agent_runs(project_id)`)

	patches := []string{
		`ALTER TABLE tech_bid_projects ADD COLUMN active_step4_run_id INTEGER`,
	}
	for _, q := range patches {
		_, err := database.Exec(q)
		if err == nil {
			log.Printf("[db] schema patch applied (active_step4_run_id)")
			continue
		}
		em := strings.ToLower(err.Error())
		if strings.Contains(em, "duplicate column") {
			continue
		}
		log.Printf("[db] schema patch: %v — %s", err, q)
	}
}

// EnsureTechBidOutlineAuditsSchema creates the table if missing and adds any columns required by handlers.
// Safe to call on every request; call before INSERT into tech_bid_outline_audits.
func EnsureTechBidOutlineAuditsSchema(database *sqlx.DB) {
	const createSQL = `CREATE TABLE IF NOT EXISTS tech_bid_outline_audits (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		outline_version INTEGER DEFAULT 1,
		outline_snapshot_json TEXT,
		facts_snapshot_json TEXT,
		coverage_score REAL DEFAULT 0,
		audit_summary TEXT,
		missing_items_json TEXT,
		weak_items_json TEXT,
		duplicate_items_json TEXT,
		final_decision TEXT,
		risk_level TEXT,
		can_proceed INTEGER DEFAULT 0,
		audit_version TEXT DEFAULT 'v1',
		detail_json TEXT,
		audit_model TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := database.Exec(createSQL); err != nil {
		log.Printf("[db] EnsureTechBidOutlineAuditsSchema create: %v", err)
		return
	}

	rows, err := database.Query(`PRAGMA table_info(tech_bid_outline_audits)`)
	if err != nil {
		log.Printf("[db] EnsureTechBidOutlineAuditsSchema pragma: %v", err)
		return
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		existing[strings.ToLower(name)] = true
	}

	columns := []struct {
		name string
		ddl  string
	}{
		{"outline_snapshot_json", "TEXT"},
		{"facts_snapshot_json", "TEXT"},
		{"audit_version", "TEXT DEFAULT 'v1'"},
		{"audit_summary", "TEXT"},
		{"final_decision", "TEXT"},
		{"risk_level", "TEXT"},
		{"can_proceed", "INTEGER DEFAULT 0"},
		{"duplicate_items_json", "TEXT"},
		{"missing_items_json", "TEXT"},
		{"weak_items_json", "TEXT"},
		{"detail_json", "TEXT"},
		{"audit_model", "TEXT"},
		{"outline_version", "INTEGER DEFAULT 1"},
		{"coverage_score", "REAL DEFAULT 0"},
		{"created_at", "DATETIME DEFAULT CURRENT_TIMESTAMP"},
	}

	for _, col := range columns {
		if existing[strings.ToLower(col.name)] {
			continue
		}
		q := fmt.Sprintf("ALTER TABLE tech_bid_outline_audits ADD COLUMN %s %s", col.name, col.ddl)
		if _, err := database.Exec(q); err != nil {
			em := strings.ToLower(err.Error())
			if strings.Contains(em, "duplicate column") {
				continue
			}
			log.Printf("[db] EnsureTechBidOutlineAuditsSchema add %s: %v", col.name, err)
			continue
		}
		log.Printf("[db] EnsureTechBidOutlineAuditsSchema: added column %s", col.name)
	}
}

// EnsureTechBidOutlineVerificationsSchema creates the Step 5 verification history table for SQLite DBs.
// The original SQL migration used MySQL index syntax, so older local SQLite files can miss this table entirely.
func EnsureTechBidOutlineVerificationsSchema(database *sqlx.DB) {
	const createSQL = `CREATE TABLE IF NOT EXISTS tech_bid_outline_verifications (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		audit_id TEXT,
		final_decision TEXT,
		risk_level TEXT,
		summary TEXT,
		critical_issues_json TEXT,
		major_issues_json TEXT,
		suggested_actions_json TEXT,
		can_proceed INTEGER DEFAULT 0,
		verification_method TEXT DEFAULT 'ai',
		verification_model TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := database.Exec(createSQL); err != nil {
		log.Printf("[db] EnsureTechBidOutlineVerificationsSchema create: %v", err)
		return
	}
	if _, err := database.Exec(`CREATE INDEX IF NOT EXISTS idx_outline_verifications_project_id ON tech_bid_outline_verifications(project_id)`); err != nil {
		log.Printf("[db] EnsureTechBidOutlineVerificationsSchema index: %v", err)
	}

	rows, err := database.Query(`PRAGMA table_info(tech_bid_outline_verifications)`)
	if err != nil {
		log.Printf("[db] EnsureTechBidOutlineVerificationsSchema pragma: %v", err)
		return
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		existing[strings.ToLower(name)] = true
	}

	columns := []struct {
		name string
		ddl  string
	}{
		{"project_id", "TEXT"},
		{"audit_id", "TEXT"},
		{"final_decision", "TEXT"},
		{"risk_level", "TEXT"},
		{"summary", "TEXT"},
		{"critical_issues_json", "TEXT"},
		{"major_issues_json", "TEXT"},
		{"suggested_actions_json", "TEXT"},
		{"can_proceed", "INTEGER DEFAULT 0"},
		{"verification_method", "TEXT DEFAULT 'ai'"},
		{"verification_model", "TEXT"},
		{"created_at", "DATETIME DEFAULT CURRENT_TIMESTAMP"},
	}
	for _, col := range columns {
		if existing[strings.ToLower(col.name)] {
			continue
		}
		q := fmt.Sprintf("ALTER TABLE tech_bid_outline_verifications ADD COLUMN %s %s", col.name, col.ddl)
		if _, err := database.Exec(q); err != nil {
			em := strings.ToLower(err.Error())
			if strings.Contains(em, "duplicate column") {
				continue
			}
			log.Printf("[db] EnsureTechBidOutlineVerificationsSchema add %s: %v", col.name, err)
			continue
		}
		log.Printf("[db] EnsureTechBidOutlineVerificationsSchema: added column %s", col.name)
	}
}

// seedTechBidFactOutlineMappingPrompt 插入 Step4 facts→目录映射提示词（若库中尚无），便于运营在系统设置中编辑。
func seedTechBidFactOutlineMappingPrompt(database *sqlx.DB) {
	var n int
	if err := database.Get(&n, `SELECT COUNT(*) FROM prompt_template WHERE prompt_key = 'tech_bid_fact_outline_mapping'`); err != nil || n > 0 {
		return
	}
	const content = `你是技术标目录编排专家。根据行业骨架与核验事实，为每条事实生成「目录落点」映射。

### 输出要求
只输出 JSON 数组，不要 Markdown。每条对象字段：
- fact_id: 与事实中 id 一致（如 S1、M1）
- fact_type: score_item | mandatory_spec | project_characteristic | special_topic
- fact_name: 事实名称
- target_level: chapter | unit | subsection（专项工艺等建议 subsection）
- target_path: 字符串数组；建议 3 层 [章标题, 节标题, 小节标题]，须与骨架语义一致，可细化小节标题
- required: 是否必须在目录中出现
- priority: high | medium | low
- mapping_reason: 一句话落点理由（可选）

### 约束
- target_path 必须与行业骨架中的章、节逻辑一致，不得凭空编造与项目无关的章。
- 高优先级评分项、强制规范应 required=true 且优先落在施工方案、质量验收等相关节。`
	const sys = `只输出合法 JSON 数组，不要解释。`
	_, err := database.Exec(`
		INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, variables, status, version, remark, system_content)
		VALUES ('tech_bid_fact_outline_mapping', '技术标-facts→目录强映射', 100, 'tech_bid_step4', ?, '', 1, 1, 'Step4 映射层（仅首次种子；可在系统设置中改）', ?)
	`, content, sys)
	if err != nil {
		log.Printf("[db] seed tech_bid_fact_outline_mapping: %v", err)
		return
	}
	log.Printf("[db] seeded prompt_template tech_bid_fact_outline_mapping")
}

func seedDefaultKnowledgeExtractPrompts(database *sqlx.DB) {
	const content = `你是一名建筑工程领域的技术专家。请从以下标书文档内容中，提炼出与当前知识库类型相关的亮点内容。

文档内容：
{{markdown_content}}

请严格按照以下 JSON 数组格式输出，每条知识为一个对象，最多输出 20 条：
[
  {
    "name": "条目名称",
    "summary": "亮点摘要（150字以内）",
    "detail": "核心要点与正文（可多段）",
    "tags": ["标签1","标签2"],
    "source_section": "来源章节标题（若无法判断可填未知）"
  }
]

只输出 JSON 数组，不要输出其他文字。`

	var n int
	_ = database.Get(&n, `SELECT COUNT(*) FROM prompt_template WHERE prompt_key = 'knowledge_extract_default'`)
	if n > 0 {
		return
	}
	_, err := database.Exec(`
		INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, variables, status, version, remark, system_content)
		VALUES ('knowledge_extract_default', '知识库-历史项目提炼(通用兜底)', 5, 'knowledge_extract', ?, 'markdown_content', 1, 1, 'AI从历史项目提炼通用兜底模板', '')
	`, content)
	if err != nil {
		log.Printf("[db] seed knowledge_extract prompt: %v", err)
	}
}

func ensureClosedLoopOptimizationSchema(database *sqlx.DB) {
	exec := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if !strings.Contains(em, "already exists") {
				log.Printf("[db] closed_loop schema: %v — %s", err, q)
			}
		}
	}

	// Task 6: Logic Conflict Audit Table
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_conflict_audit (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		conflict_type TEXT,
		field_name TEXT,
		source_a TEXT,
		source_b TEXT,
		conflict_reason TEXT,
		severity TEXT,
		manual_review_required INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_conflict_audit_project ON tech_bid_conflict_audit(project_id)`)

	// Task 9: Evidence Tracking Columns for facts
	patches := []string{
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN source_section TEXT`,
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN source_page INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN source_line INTEGER DEFAULT 0`,
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN evidence_count INTEGER DEFAULT 1`,
		`ALTER TABLE tech_bid_outline_facts ADD COLUMN extracted_by_view TEXT`,
	}
	for _, q := range patches {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if strings.Contains(em, "duplicate column") {
				continue
			}
			log.Printf("[db] closed_loop patch: %v — %s", err, q)
		}
	}
	// Task 13.2: Elastic Skeleton Structure Plan
	exec(`CREATE TABLE IF NOT EXISTS tech_bid_structure_plan (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		plan_version INTEGER DEFAULT 1,
		adjustments_json TEXT NOT NULL,
		approve_reason TEXT, 
		reject_reason TEXT,
		tender_profile_json TEXT,
		personalization_score REAL DEFAULT 0,
		rationale TEXT,
		status TEXT DEFAULT 'pending', -- pending | approved | rejected
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_structure_plan_project ON tech_bid_structure_plan(project_id)`)
}

// seedTechBidStructureProfilerPrompt 插入 Step4 画像语义识别提示词
func seedTechBidStructureProfilerPrompt(database *sqlx.DB) {
	var n int
	if err := database.Get(&n, `SELECT COUNT(*) FROM prompt_template WHERE prompt_key = 'structure_profiler'`); err != nil || n > 0 {
		return
	}
	const content = `你是一个招标文件结构分析专家。请分析 Global Context 和 Extracted Facts，提取招标文件的「结构 DNA」画像。

### 输出要求
只输出 JSON 对象，不要 Markdown。字段：
- type: construction_org | special_workflow | score_centric | schedule_sensitive
- key_emphasis: 字符串数组（如：质量、工期、成本、安全、专项技术、资源保障）
- preferred_order: 核心章节关键字顺序（如：[概况, 部署, 工艺, 质量]）
- mandatory_tags: 必须特别强调的标签

### 分析逻辑
- 如果评分项中「专项施工方案」占比极高 -> type=special_workflow
- 如果「工期奖罚」或「进度控制」描述极重 -> type=schedule_sensitive
- 如果行业骨架中的核心章节在招标文件中有明确的「分节要求」-> 提取 preferred_order`
	const sys = `只输出合法 JSON 对象，不要解释。`
	_, err := database.Exec(`
		INSERT INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, variables, status, version, remark, system_content)
		VALUES ('structure_profiler', '技术标-结构画像识别', 100, 'tech_bid_step4', ?, '', 1, 1, 'Step4 画像层（仅首次种子）', ?)
	`, content, sys)
	if err != nil {
		log.Printf("[db] seed structure_profiler: %v", err)
		return
	}
	log.Printf("[db] seeded prompt_template structure_profiler")
}

// EnsureSkeletonSelectionSchema 创建骨架选择历史记录表（Human-in-the-loop 骨架确认）
func EnsureSkeletonSelectionSchema(database *sqlx.DB) {
	exec := func(q string) {
		_, err := database.Exec(q)
		if err != nil {
			em := strings.ToLower(err.Error())
			if !strings.Contains(em, "already exists") {
				log.Printf("[db] skeleton_selection schema: %v — %s", err, q)
			}
		} else {
			log.Printf("[db] skeleton_selection schema create applied")
		}
	}

	exec(`CREATE TABLE IF NOT EXISTS tech_bid_skeleton_selections (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		skeleton_id TEXT NOT NULL,
		skeleton_name TEXT NOT NULL,
		operator_name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	exec(`CREATE INDEX IF NOT EXISTS idx_skeleton_selection_project ON tech_bid_skeleton_selections(project_id)`)
}
