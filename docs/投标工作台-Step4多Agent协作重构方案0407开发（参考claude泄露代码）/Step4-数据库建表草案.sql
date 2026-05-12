-- Step4 数据库建表草案
-- 项目：投标工作台 / 技术标第4步多 Agent 协作
-- 说明：本 SQL 为结构草案，实际字段类型可根据现有数据库风格微调

-- 1. Step4 运行主表
CREATE TABLE IF NOT EXISTS step4_runs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    project_id BIGINT NOT NULL,
    outline_version BIGINT NULL,
    trigger_source VARCHAR(64) NOT NULL,
    operator_id VARCHAR(64) NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'idle',
    gate_result VARCHAR(16) NULL,
    started_at DATETIME NULL,
    finished_at DATETIME NULL,
    error_message TEXT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_runs_project_id (project_id),
    KEY idx_step4_runs_status (status),
    KEY idx_step4_runs_gate_result (gate_result)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 2. Requirement 真相层
CREATE TABLE IF NOT EXISTS step4_requirements (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    requirement_id VARCHAR(128) NOT NULL,
    requirement_type VARCHAR(64) NOT NULL,
    source_text TEXT NOT NULL,
    source_location VARCHAR(255) NULL,
    priority VARCHAR(32) NOT NULL DEFAULT 'normal',
    must_be_explicit TINYINT(1) NOT NULL DEFAULT 0,
    expected_response_level VARCHAR(32) NULL,
    domain VARCHAR(128) NULL,
    response_tier VARCHAR(64) NULL,
    risk_level VARCHAR(32) NULL,
    summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_requirements_run_id (run_id),
    KEY idx_step4_requirements_project_id (project_id),
    KEY idx_step4_requirements_requirement_id (requirement_id),
    KEY idx_step4_requirements_priority (priority),
    KEY idx_step4_requirements_risk_level (risk_level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 3. Fact 候选表
CREATE TABLE IF NOT EXISTS step4_fact_candidates (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    fact_id VARCHAR(128) NOT NULL,
    fact_type VARCHAR(64) NOT NULL,
    fact_name VARCHAR(255) NOT NULL,
    source_library VARCHAR(128) NULL,
    source_location VARCHAR(255) NULL,
    confidence_score DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    snippet TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_fact_candidates_run_id (run_id),
    KEY idx_step4_fact_candidates_project_id (project_id),
    KEY idx_step4_fact_candidates_fact_id (fact_id),
    KEY idx_step4_fact_candidates_fact_type (fact_type),
    KEY idx_step4_fact_candidates_confidence_score (confidence_score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 4. Fact 映射表
CREATE TABLE IF NOT EXISTS step4_fact_mappings (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    requirement_id VARCHAR(128) NOT NULL,
    fact_id VARCHAR(128) NOT NULL,
    target_level VARCHAR(64) NULL,
    target_path JSON NULL,
    mapping_reason TEXT NULL,
    mapping_source VARCHAR(255) NULL,
    support_strength VARCHAR(32) NOT NULL DEFAULT 'weak',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_fact_mappings_run_id (run_id),
    KEY idx_step4_fact_mappings_project_id (project_id),
    KEY idx_step4_fact_mappings_requirement_id (requirement_id),
    KEY idx_step4_fact_mappings_fact_id (fact_id),
    KEY idx_step4_fact_mappings_support_strength (support_strength)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 5. 候选目录版本表
CREATE TABLE IF NOT EXISTS step4_outline_versions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    version_no INT NOT NULL,
    version_source VARCHAR(64) NOT NULL DEFAULT 'agent',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    rationale TEXT NULL,
    created_by VARCHAR(64) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_step4_outline_versions_run_version (run_id, version_no),
    KEY idx_step4_outline_versions_project_id (project_id),
    KEY idx_step4_outline_versions_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 6. 目录节点表
CREATE TABLE IF NOT EXISTS step4_outline_nodes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    outline_version_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    node_name VARCHAR(255) NOT NULL,
    node_level INT NOT NULL,
    node_order INT NOT NULL DEFAULT 0,
    response_goal TEXT NULL,
    linked_requirement_ids JSON NULL,
    linked_fact_ids JSON NULL,
    node_summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_outline_nodes_outline_version_id (outline_version_id),
    KEY idx_step4_outline_nodes_parent_id (parent_id),
    KEY idx_step4_outline_nodes_node_level (node_level),
    KEY idx_step4_outline_nodes_node_order (node_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 7. 覆盖率审计表
CREATE TABLE IF NOT EXISTS step4_coverage_audits (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    outline_version BIGINT NULL,
    requirement_total INT NOT NULL DEFAULT 0,
    requirement_mapped INT NOT NULL DEFAULT 0,
    fact_total INT NOT NULL DEFAULT 0,
    fact_mapped INT NOT NULL DEFAULT 0,
    coverage_rate DECIMAL(6,4) NOT NULL DEFAULT 0.0000,
    missing_requirement_ids JSON NULL,
    weak_requirement_ids JSON NULL,
    missing_fact_ids JSON NULL,
    weak_fact_ids JSON NULL,
    duplicate_node_hints JSON NULL,
    result VARCHAR(16) NOT NULL DEFAULT 'REVISE',
    summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_coverage_audits_run_id (run_id),
    KEY idx_step4_coverage_audits_project_id (project_id),
    KEY idx_step4_coverage_audits_result (result)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 8. 完全响应率审计表
CREATE TABLE IF NOT EXISTS step4_full_response_audits (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    outline_version BIGINT NULL,
    requirement_total INT NOT NULL DEFAULT 0,
    requirement_mapped INT NOT NULL DEFAULT 0,
    requirement_fully_responded INT NOT NULL DEFAULT 0,
    requirement_weakly_responded INT NOT NULL DEFAULT 0,
    requirement_only_tagged INT NOT NULL DEFAULT 0,
    full_response_rate DECIMAL(6,4) NOT NULL DEFAULT 0.0000,
    weak_response_rate DECIMAL(6,4) NOT NULL DEFAULT 0.0000,
    response_quality_score DECIMAL(6,4) NOT NULL DEFAULT 0.0000,
    missing_requirement_ids JSON NULL,
    weak_requirement_ids JSON NULL,
    only_tagged_requirement_ids JSON NULL,
    shell_title_hints JSON NULL,
    high_priority_missing_ids JSON NULL,
    mandatory_missing_ids JSON NULL,
    mandatory_insufficient_ids JSON NULL,
    hard_rule_warnings JSON NULL,
    result VARCHAR(16) NOT NULL DEFAULT 'REVISE',
    summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_full_response_audits_run_id (run_id),
    KEY idx_step4_full_response_audits_project_id (project_id),
    KEY idx_step4_full_response_audits_result (result)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 9. 冲突审计表
CREATE TABLE IF NOT EXISTS step4_conflict_audits (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    outline_version BIGINT NULL,
    has_block TINYINT(1) NOT NULL DEFAULT 0,
    conflicts_json JSON NULL,
    duplicate_node_hints JSON NULL,
    summary TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_conflict_audits_run_id (run_id),
    KEY idx_step4_conflict_audits_project_id (project_id),
    KEY idx_step4_conflict_audits_has_block (has_block)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 10. 结构修正方案表
CREATE TABLE IF NOT EXISTS step4_structure_plans (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    outline_version BIGINT NULL,
    adjustments_json JSON NULL,
    rationale TEXT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    approved_by VARCHAR(64) NULL,
    approved_at DATETIME NULL,
    rejected_reason TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_structure_plans_run_id (run_id),
    KEY idx_step4_structure_plans_project_id (project_id),
    KEY idx_step4_structure_plans_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 11. 审批日志表
CREATE TABLE IF NOT EXISTS step4_approval_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    stage VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    operator_id VARCHAR(64) NULL,
    reason TEXT NULL,
    snapshot_version VARCHAR(64) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_step4_approval_logs_run_id (run_id),
    KEY idx_step4_approval_logs_project_id (project_id),
    KEY idx_step4_approval_logs_stage (stage),
    KEY idx_step4_approval_logs_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 12. Agent 运行日志表（建议）
CREATE TABLE IF NOT EXISTS step4_agent_runs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    agent_name VARCHAR(64) NOT NULL,
    stage VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    input_summary TEXT NULL,
    output_summary TEXT NULL,
    error_message TEXT NULL,
    started_at DATETIME NULL,
    finished_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_step4_agent_runs_run_id (run_id),
    KEY idx_step4_agent_runs_project_id (project_id),
    KEY idx_step4_agent_runs_agent_name (agent_name),
    KEY idx_step4_agent_runs_stage (stage),
    KEY idx_step4_agent_runs_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
