package model

import "time"

type PromptCategory struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	ParentID  int64     `json:"parent_id" db:"parent_id"`
	Sort      int       `json:"sort" db:"sort"`
	Remark    string    `json:"remark" db:"remark"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type PromptTemplate struct {
	ID         int64     `json:"id" db:"id"`
	PromptKey  string    `json:"prompt_key" db:"prompt_key"`
	PromptName string    `json:"prompt_name" db:"prompt_name"`
	CategoryID int64     `json:"category_id" db:"category_id"`
	Scenario   string    `json:"scenario" db:"scenario"`
	Content       string    `json:"content" db:"content"`
	SystemContent string    `json:"system_content" db:"system_content"`
	Variables     string    `json:"variables" db:"variables"`
	Status        int       `json:"status" db:"status"`     // 1: enabled, 0: disabled
	Version    int       `json:"version" db:"version"`   // current version number
	Remark     string    `json:"remark" db:"remark"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type PromptVersion struct {
	ID            int64     `json:"id" db:"id"`
	TemplateID    int64     `json:"template_id" db:"template_id"`
	PromptKey     string    `json:"prompt_key" db:"prompt_key"`
	Version       int       `json:"version" db:"version"`
	Content       string    `json:"content" db:"content"`
	SystemContent string    `json:"system_content" db:"system_content"`
	ChangeSummary string    `json:"change_summary" db:"change_summary"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
