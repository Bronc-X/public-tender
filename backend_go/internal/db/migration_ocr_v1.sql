-- CTO OCR 技术实施方案 - 数据表扩展

-- 1. 文件内容表 (分层存储：原文、Markdown、页码映射)
CREATE TABLE IF NOT EXISTS file_content (
  id TEXT PRIMARY KEY,
  file_asset_id TEXT NOT NULL REFERENCES file_asset(id),
  plain_text TEXT,
  markdown_text TEXT,
  page_mapping_json TEXT, -- 存储每页在原文中的偏移或元数据
  content_type TEXT DEFAULT 'full', -- full / snippet
  version INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. OCR 任务表 (精细化任务追踪)
CREATE TABLE IF NOT EXISTS file_ocr_task (
  id TEXT PRIMARY KEY,
  file_asset_id TEXT NOT NULL REFERENCES file_asset(id),
  ocr_engine TEXT, -- paddle_ocr, glm_ocr, tencent_ocr
  ocr_mode TEXT, -- fast, accurate, structured
  recommended_mode TEXT,
  selected_mode TEXT,
  status TEXT DEFAULT 'pending', -- pending, queued, running, succeeded, failed, canceled
  progress INTEGER DEFAULT 0,
  error_message TEXT,
  retry_count INTEGER DEFAULT 0,
  started_at DATETIME,
  completed_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 3. OCR 结果原始表 (保留 OCR 引擎最原始的输出和置信度)
CREATE TABLE IF NOT EXISTS file_ocr_result (
  id TEXT PRIMARY KEY,
  task_id TEXT REFERENCES file_ocr_task(id),
  file_asset_id TEXT REFERENCES file_asset(id),
  ocr_text TEXT,
  confidence REAL,
  engine_meta_json TEXT, -- 存储 OCR 引擎返回的原始全量 JSON
  review_status TEXT DEFAULT 'unreviewed', -- unreviewed, reviewing, revised, approved, rejected
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 4. 结构化提炼结果表 (最终业务消费层)
CREATE TABLE IF NOT EXISTS file_extraction (
  id TEXT PRIMARY KEY,
  file_asset_id TEXT REFERENCES file_asset(id),
  content_id TEXT REFERENCES file_content(id),
  library_type TEXT, -- equipment, person, performance, method, system, risk, etc.
  title TEXT,
  summary TEXT,
  structured_json TEXT, -- 提炼出的业务字段
  source_snippet TEXT, -- 来源文本片段
  confidence_score REAL,
  approved_by TEXT,
  approved_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 5. OCR 服务全局配置表
CREATE TABLE IF NOT EXISTS ocr_settings (
  id TEXT PRIMARY KEY,
  mode TEXT DEFAULT 'auto', -- cloud / local / private / auto
  service_url TEXT,
  service_port TEXT,
  api_key TEXT,
  token TEXT,
  default_strategy TEXT DEFAULT 'balanced',
  allow_auto_download INTEGER DEFAULT 1, -- 0-false, 1-true
  max_concurrency INTEGER DEFAULT 2,
  timeout_seconds INTEGER DEFAULT 60,
  retry_times INTEGER DEFAULT 3,
  model_version TEXT,
  status TEXT DEFAULT 'configured', -- not_configured, configured, available, unavailable
  company_id TEXT, -- 支持租户级配置
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 增加必要的索引
CREATE INDEX IF NOT EXISTS idx_file_content_asset ON file_content(file_asset_id);
CREATE INDEX IF NOT EXISTS idx_ocr_task_status ON file_ocr_task(status);
CREATE INDEX IF NOT EXISTS idx_ocr_result_task ON file_ocr_result(task_id);
CREATE INDEX IF NOT EXISTS idx_file_extraction_asset ON file_extraction(file_asset_id);
CREATE INDEX IF NOT EXISTS idx_file_extraction_library ON file_extraction(library_type);
