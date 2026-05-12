-- New tables for Multi-Agent Bid Outline Generation

-- 1. Fact Extraction Layer
CREATE TABLE IF NOT EXISTS tech_bid_outline_facts (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES tech_bid_projects(id),
  fact_type TEXT NOT NULL, -- score_item, mandatory_spec, project_characteristic, special_topic
  fact_name TEXT,
  fact_content TEXT,
  source_text TEXT,
  source_location TEXT,
  priority TEXT, -- high, medium, low
  score_value REAL,
  penalty_level TEXT,
  tags_json TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. Structured Audits
CREATE TABLE IF NOT EXISTS tech_bid_outline_audits (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES tech_bid_projects(id),
  outline_version INTEGER NOT NULL,
  coverage_score REAL NOT NULL,
  audit_summary TEXT,
  missing_items_json TEXT, -- JSON structure of gaps
  weak_items_json TEXT,
  duplicate_items_json TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 3. Enhance Chapter Plans Table
-- Using ADD COLUMN (SQLite 3.25+ supports this)
ALTER TABLE tech_bid_chapter_plans ADD COLUMN coverage_sources_json TEXT;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN requirement_ids_json TEXT;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN coverage_level TEXT;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN outline_version INTEGER DEFAULT 1;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN must_have INTEGER DEFAULT 0;
ALTER TABLE tech_bid_chapter_plans ADD COLUMN score_related INTEGER DEFAULT 0;

-- 4. Enhance Project Table
ALTER TABLE tech_bid_projects ADD COLUMN coverage_score REAL DEFAULT 0;
