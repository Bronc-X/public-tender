-- Migration: Technical Bid Workbench Optimization (P0/P1)
-- Added at: 2026-04-05

-- 1. Extend tech_bid_projects with granular step status (State Machine)
ALTER TABLE tech_bid_projects ADD COLUMN step4_status TEXT DEFAULT 'idle'; -- idle, facts_extracting, facts_ready, generating, outline_ready, auditing, audit_ready, refining, refine_ready, optimizing, optimized_ready, failed
ALTER TABLE tech_bid_projects ADD COLUMN step5_status TEXT DEFAULT 'idle'; -- idle, waiting_for_step4, verifying, verified_pass, verified_revise, verified_block, failed
ALTER TABLE tech_bid_projects ADD COLUMN manual_lock INTEGER DEFAULT 0; -- 0: Normal, 1: Locked by human

-- 2. Extend tech_bid_outline_audits with structured decision fields
ALTER TABLE tech_bid_outline_audits ADD COLUMN final_decision TEXT DEFAULT 'REVISE'; -- PASS, REVISE, BLOCK
ALTER TABLE tech_bid_outline_audits ADD COLUMN risk_level TEXT DEFAULT 'MEDIUM'; -- LOW, MEDIUM, HIGH
ALTER TABLE tech_bid_outline_audits ADD COLUMN can_proceed INTEGER DEFAULT 0; -- 0: No, 1: Yes
ALTER TABLE tech_bid_outline_audits ADD COLUMN detail_json TEXT; -- For storing expanded audit/verification results
