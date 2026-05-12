-- Migration: Add Document Split Paths (Rule vs Template)
-- Description: Split original markdown file into rules and templates
ALTER TABLE bid_projects ADD COLUMN rule_markdown_path TEXT;
ALTER TABLE bid_projects ADD COLUMN template_markdown_path TEXT;
ALTER TABLE bid_projects ADD COLUMN template_start_page INTEGER;
ALTER TABLE bid_projects ADD COLUMN template_end_page INTEGER;
