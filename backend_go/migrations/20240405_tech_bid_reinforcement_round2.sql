-- Technical Bid Workbench Hardening Phase - Round 2 Reinforcement
-- SQLite Compatible Migration

-- 1. Update tech_bid_state_transitions to include step-level and operator details
ALTER TABLE `tech_bid_state_transitions` ADD COLUMN `from_step` VARCHAR(64) DEFAULT NULL;
ALTER TABLE `tech_bid_state_transitions` ADD COLUMN `to_step` VARCHAR(64) DEFAULT NULL;
ALTER TABLE `tech_bid_state_transitions` ADD COLUMN `verification_method` VARCHAR(64) DEFAULT NULL;
ALTER TABLE `tech_bid_state_transitions` ADD COLUMN `operator_id` VARCHAR(64) DEFAULT NULL;
ALTER TABLE `tech_bid_state_transitions` ADD COLUMN `operator_name` VARCHAR(128) DEFAULT NULL;

-- 2. Ensure tech_bid_projects has all standard hardening fields (if missing)
-- (Using IF NOT EXISTS logic via a script or just standard ALTERs if we know they are likely missing in some environments)

-- Note: TechBidProject fields like verification_method, override_enabled, etc. 
-- were already in the previous migration, so we only add if they might have been skipped.
-- SQLite doesn't support IF NOT EXISTS for columns easily in raw SQL, 
-- but we already verified they are in the model.
