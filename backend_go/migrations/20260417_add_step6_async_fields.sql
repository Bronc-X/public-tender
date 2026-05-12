-- +goose Up
-- +goose StatementBegin
ALTER TABLE tech_bid_projects
ADD COLUMN IF NOT EXISTS step6_status VARCHAR(64) DEFAULT 'idle' COMMENT 'Step 6 Agent Execution Status: idle, generating, success, error',
ADD COLUMN IF NOT EXISTS step6_payload_json LONGTEXT COMMENT 'Generated Payload Cache for Step 6 Form';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tech_bid_projects
DROP COLUMN IF EXISTS step6_status,
DROP COLUMN IF EXISTS step6_payload_json;
-- +goose StatementEnd
