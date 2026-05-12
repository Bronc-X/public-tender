-- Technical Bid Workbench Refinement - March 2024 Migration
-- Adds granular status tracking, audit result storage, and administrative override logs.

-- 1. Updates to tech_bid_projects table
ALTER TABLE `tech_bid_projects` 
    ADD COLUMN `step4_status` VARCHAR(64) DEFAULT 'idle' AFTER `project_status`,
    ADD COLUMN `step5_status` VARCHAR(64) DEFAULT 'idle' AFTER `step4_status`,
    ADD COLUMN `coverage_score` DECIMAL(5,2) DEFAULT 0.00 AFTER `step5_status`,
    ADD COLUMN `final_decision` VARCHAR(32) DEFAULT NULL AFTER `coverage_score`,
    ADD COLUMN `can_enter_content_generation` TINYINT(1) DEFAULT 0 AFTER `final_decision`,
    ADD COLUMN `verification_method` VARCHAR(64) DEFAULT 'ai' AFTER `can_enter_content_generation`,
    ADD COLUMN `verification_summary` TEXT DEFAULT NULL AFTER `verification_method`,
    ADD COLUMN `override_enabled` TINYINT(1) DEFAULT 0 AFTER `verification_summary`,
    ADD COLUMN `override_reason` TEXT DEFAULT NULL AFTER `override_enabled`,
    ADD COLUMN `override_by` VARCHAR(128) DEFAULT NULL AFTER `override_reason`,
    ADD COLUMN `override_at` DATETIME DEFAULT NULL AFTER `override_by`,
    ADD COLUMN `manual_review_required` TINYINT(1) DEFAULT 0 AFTER `override_at`,
    ADD COLUMN `manual_review_result` TEXT DEFAULT NULL AFTER `manual_review_required`,
    ADD COLUMN `manual_review_reason` TEXT DEFAULT NULL AFTER `manual_review_result`,
    ADD COLUMN `manual_review_by` VARCHAR(128) DEFAULT NULL AFTER `manual_review_reason`,
    ADD COLUMN `manual_review_at` DATETIME DEFAULT NULL AFTER `manual_review_by`;

-- 2. New table: Historical Step 4 Audit Results
CREATE TABLE IF NOT EXISTS `tech_bid_outline_audits` (
    `id` VARCHAR(64) PRIMARY KEY,
    `project_id` VARCHAR(64) NOT NULL,
    `outline_snapshot_json` LONGTEXT DEFAULT NULL,
    `facts_snapshot_json` LONGTEXT DEFAULT NULL,
    `coverage_score` DECIMAL(5,2) DEFAULT 0.00,
    `missing_items_json` LONGTEXT DEFAULT NULL,
    `weak_items_json` LONGTEXT DEFAULT NULL,
    `duplicate_items_json` LONGTEXT DEFAULT NULL,
    `audit_model` VARCHAR(128) DEFAULT NULL,
    `audit_version` VARCHAR(64) DEFAULT 'v1',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_outline_audits_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. New table: Historical Step 5 Verification Results
CREATE TABLE IF NOT EXISTS `tech_bid_outline_verifications` (
    `id` VARCHAR(64) PRIMARY KEY,
    `project_id` VARCHAR(64) NOT NULL,
    `audit_id` VARCHAR(64) DEFAULT NULL,
    `final_decision` VARCHAR(32) DEFAULT NULL,
    `risk_level` VARCHAR(32) DEFAULT NULL,
    `summary` TEXT DEFAULT NULL,
    `critical_issues_json` LONGTEXT DEFAULT NULL,
    `major_issues_json` LONGTEXT DEFAULT NULL,
    `suggested_actions_json` LONGTEXT DEFAULT NULL,
    `can_proceed` TINYINT(1) DEFAULT 0,
    `verification_method` VARCHAR(64) DEFAULT 'ai',
    `verification_model` VARCHAR(128) DEFAULT NULL,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_outline_verifications_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. New table: Administrative Override Logs (Audit Trail)
CREATE TABLE IF NOT EXISTS `tech_bid_manual_overrides` (
    `id` VARCHAR(64) PRIMARY KEY,
    `project_id` VARCHAR(64) NOT NULL,
    `original_status` VARCHAR(64) DEFAULT NULL,
    `target_status` VARCHAR(64) DEFAULT NULL,
    `operator_id` VARCHAR(64) DEFAULT NULL,
    `operator_name` VARCHAR(128) DEFAULT NULL,
    `reason` TEXT NOT NULL,
    `snapshot_before_override_json` LONGTEXT DEFAULT NULL,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_manual_overrides_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. New table: System-wide State Transition Logs
CREATE TABLE IF NOT EXISTS `tech_bid_state_transitions` (
    `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
    `project_id` VARCHAR(64) NOT NULL,
    `from_status` VARCHAR(64) DEFAULT NULL,
    `to_status` VARCHAR(64) DEFAULT NULL,
    `trigger_type` VARCHAR(32) NOT NULL, -- system/ai/manual
    `trigger_reason` TEXT DEFAULT NULL,
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_state_transitions_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
