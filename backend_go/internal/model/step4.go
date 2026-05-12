package model

import "time"

// Step4Run 对应 step4_runs
type Step4Run struct {
	ID             int64      `db:"id" json:"id"`
	ProjectID      string     `db:"project_id" json:"project_id"`
	OutlineVersion *int       `db:"outline_version" json:"outline_version,omitempty"`
	TriggerSource  string     `db:"trigger_source" json:"trigger_source"`
	OperatorID     *string    `db:"operator_id" json:"operator_id,omitempty"`
	Status         string     `db:"status" json:"status"`
	CurrentStage   *string    `db:"current_stage" json:"current_stage,omitempty"`
	GateResult     *string    `db:"gate_result" json:"gate_result,omitempty"`
	StartedAt      *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt     *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	ErrorMessage   *string    `db:"error_message" json:"error_message,omitempty"`
	RetryCount     int        `db:"retry_count" json:"retry_count"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// Step4AgentRun 对应 step4_agent_runs
type Step4AgentRun struct {
	ID            string     `db:"id" json:"id"`
	RunID         int64      `db:"run_id" json:"run_id"`
	ProjectID     string     `db:"project_id" json:"project_id"`
	AgentName     string     `db:"agent_name" json:"agent_name"`
	Stage         string     `db:"stage" json:"stage"`
	Status        string     `db:"status" json:"status"`
	InputSummary  *string    `db:"input_summary" json:"input_summary,omitempty"`
	OutputSummary *string    `db:"output_summary" json:"output_summary,omitempty"`
	ErrorMessage  *string    `db:"error_message" json:"error_message,omitempty"`
	StartedAt     *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt    *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	DurationMs    *int       `db:"duration_ms" json:"duration_ms,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
}
