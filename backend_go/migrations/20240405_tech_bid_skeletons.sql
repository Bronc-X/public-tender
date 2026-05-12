-- Create tech_bid_industry_skeletons table
CREATE TABLE IF NOT EXISTS tech_bid_industry_skeletons (
    id TEXT PRIMARY KEY,
    industry_name TEXT NOT NULL UNIQUE,
    logical_chapters_json TEXT NOT NULL,
    common_section_pool_json TEXT,
    industry_keywords_json TEXT,
    title_candidate_pool_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_industry_name ON tech_bid_industry_skeletons(industry_name);
