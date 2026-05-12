package service

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// CacheLogEntry captures all cache-related metrics for a single LLM call.
type CacheLogEntry struct {
	ProjectID                string `json:"project_id"`
	Step                    string `json:"step"`            // e.g. "extract_facts", "generate_outline", "audit_coverage"
	Model                    string `json:"model"`
	CacheMode                string `json:"cache_mode"`      // "explicit" | "implicit" | "none"
	PromptTokens             int    `json:"prompt_tokens"`
	CachedTokens             int    `json:"cached_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	CompletionTokens         int    `json:"completion_tokens"`
	LatencyMs                int64  `json:"latency_ms"`
	RequestSuccess           bool   `json:"request_success"`
	PromptVersion            string `json:"prompt_version"`  // prompt template version for cache consistency analysis
}

// CacheMetricsService persists cache metrics to the database and logs them.
type CacheMetricsService struct {
	db *sqlx.DB
}

// NewCacheMetricsService creates a new cache metrics service.
func NewCacheMetricsService(db *sqlx.DB) *CacheMetricsService {
	return &CacheMetricsService{db: db}
}

// Log persists a cache metrics entry to DB and outputs a structured log line.
func (s *CacheMetricsService) Log(entry CacheLogEntry) {
	// 1. Persist to database for analytics
	if s.db != nil {
		success := 0
		if entry.RequestSuccess {
			success = 1
		}
		_, err := s.db.Exec(`
			INSERT INTO cache_metrics
				(project_id, step, model, cache_mode, prompt_tokens, cached_tokens,
				 cache_creation_input_tokens, completion_tokens, latency_ms, request_success, prompt_version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.ProjectID, entry.Step, entry.Model, entry.CacheMode,
			entry.PromptTokens, entry.CachedTokens, entry.CacheCreationInputTokens,
			entry.CompletionTokens, entry.LatencyMs, success, entry.PromptVersion, time.Now())
		if err != nil {
			log.Printf("[CacheMetrics] DB persist failed: %v", err)
		}
	}

	// 2. Output structured log
	log.Printf("[CacheMetrics] project_id=%s step=%s model=%s cache_mode=%s prompt_tokens=%d cached_tokens=%d cache_creation_input_tokens=%d completion_tokens=%d latency_ms=%d success=%v prompt_version=%s",
		entry.ProjectID,
		entry.Step,
		entry.Model,
		entry.CacheMode,
		entry.PromptTokens,
		entry.CachedTokens,
		entry.CacheCreationInputTokens,
		entry.CompletionTokens,
		entry.LatencyMs,
		entry.RequestSuccess,
		entry.PromptVersion,
	)
}

// LogCacheMetrics is the legacy package-level function for backward compatibility.
// It only logs (no DB persistence). Use CacheMetricsService.Log() for full functionality.
func LogCacheMetrics(entry CacheLogEntry) {
	log.Printf("[CacheMetrics] project_id=%s step=%s model=%s cache_mode=%s prompt_tokens=%d cached_tokens=%d cache_creation_input_tokens=%d completion_tokens=%d latency_ms=%d success=%v prompt_version=%s",
		entry.ProjectID,
		entry.Step,
		entry.Model,
		entry.CacheMode,
		entry.PromptTokens,
		entry.CachedTokens,
		entry.CacheCreationInputTokens,
		entry.CompletionTokens,
		entry.LatencyMs,
		entry.RequestSuccess,
		entry.PromptVersion,
	)
}
