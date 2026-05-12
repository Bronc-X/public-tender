-- Migration for Step 4 Directory Strong Mapping
-- Created: 2024-04-05

-- 1. tech_bid_fact_mappings
CREATE TABLE IF NOT EXISTS tech_bid_fact_mappings (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    fact_type TEXT,
    fact_name TEXT,
    target_level TEXT, -- chapter, unit, subsection
    target_chapter TEXT,
    target_unit TEXT,
    target_subsection TEXT,
    required INTEGER DEFAULT 1,
    priority TEXT, -- high, medium, low
    mapping_reason TEXT,
    mapping_source TEXT, -- rule, ai, manual
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fact_mapping_project_id ON tech_bid_fact_mappings(project_id);

-- 2. tech_bid_outline_coverage_checks
CREATE TABLE IF NOT EXISTS tech_bid_outline_coverage_checks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    outline_version INTEGER NOT NULL,
    fact_total INTEGER,
    fact_mapped INTEGER,
    coverage_rate REAL,
    missing_fact_ids_json TEXT,
    weak_fact_ids_json TEXT,
    duplicate_node_ids_json TEXT,
    result TEXT, -- PASS, REVISE, BLOCK
    summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coverage_check_project_id ON tech_bid_outline_coverage_checks(project_id);

-- 3. tech_bid_outline_similarity_checks
CREATE TABLE IF NOT EXISTS tech_bid_outline_similarity_checks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    outline_version INTEGER NOT NULL,
    compare_scope TEXT, -- same_project, same_industry, global
    similarity_score REAL,
    matched_reference_id TEXT,
    risk_level TEXT,
    action_taken TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_similarity_check_project_id ON tech_bid_outline_similarity_checks(project_id);

-- 4. Extend tech_bid_chapter_plans with more analytical fields
-- (Assuming the columns don't exist, we use ALTER TABLE in the app code or here)
-- In SQLite, we can't add multiple columns in one ALTER TABLE easily in some versions,
-- but the app code has a dynamic EnsureTable logic. 
-- However, for a formal migration:

-- We'll add these columns if they don't exist via the app's schema_patch.go logic later,
-- but for now let's add them here as well for completeness if the DB supports it.
ALTER TABLE tech_bid_chapter_plans ADD COLUMN mapping_fact_ids_json TEXT;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN node_role TEXT; -- chapter, unit, subsection
ALTER TABLE tech_bid_chapter_plans ADD COLUMN node_quality_score REAL;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN is_required_node INTEGER DEFAULT 0;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN similarity_flag INTEGER DEFAULT 0;
