package handler

import (
	"backend_go/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// CacheStatsHandler provides cache metrics analytics endpoints.
type CacheStatsHandler struct {
	db                *sqlx.DB
	cacheMetricsSvc   *service.CacheMetricsService
}

// NewCacheStatsHandler creates a new cache stats handler.
func NewCacheStatsHandler(db *sqlx.DB, cacheMetricsSvc *service.CacheMetricsService) *CacheStatsHandler {
	return &CacheStatsHandler{db: db, cacheMetricsSvc: cacheMetricsSvc}
}

// GetCacheStats returns aggregated cache metrics.
// GET /api/cache/stats?project_id=xxx&hours=24
func (h *CacheStatsHandler) GetCacheStats(c *gin.Context) {
	projectID := c.Query("project_id")
	hours := c.DefaultQuery("hours", "24")

	// Overall stats
	query := `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
			COALESCE(SUM(cached_tokens), 0) as total_cached_tokens,
			COALESCE(SUM(cache_creation_input_tokens), 0) as total_cache_creation_tokens,
			COALESCE(SUM(completion_tokens), 0) as total_completion_tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(SUM(CASE WHEN cache_mode = 'explicit' THEN 1 ELSE 0 END), 0) as explicit_count,
			COALESCE(SUM(CASE WHEN cache_mode = 'implicit' THEN 1 ELSE 0 END), 0) as implicit_count,
			COALESCE(SUM(CASE WHEN cache_mode = 'none' THEN 1 ELSE 0 END), 0) as none_count
		FROM cache_metrics
		WHERE created_at >= datetime('now', '-' || ? || ' hours')`

	args := []interface{}{hours}
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	var stats struct {
		TotalRequests           int     `json:"total_requests" db:"total_requests"`
		TotalPromptTokens       int     `json:"total_prompt_tokens" db:"total_prompt_tokens"`
		TotalCachedTokens       int     `json:"total_cached_tokens" db:"total_cached_tokens"`
		TotalCacheCreationTokens int    `json:"total_cache_creation_tokens" db:"total_cache_creation_tokens"`
		TotalCompletionTokens   int     `json:"total_completion_tokens" db:"total_completion_tokens"`
		AvgLatencyMs            float64 `json:"avg_latency_ms" db:"avg_latency_ms"`
		ExplicitCount           int     `json:"explicit_count" db:"explicit_count"`
		ImplicitCount           int     `json:"implicit_count" db:"implicit_count"`
		NoneCount               int     `json:"none_count" db:"none_count"`
	}

	if err := h.db.Get(&stats, query, args...); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Cache hit rate
	hitRate := 0.0
	if stats.TotalPromptTokens > 0 {
		hitRate = float64(stats.TotalCachedTokens) / float64(stats.TotalPromptTokens) * 100
	}

	// Per-step breakdown
	stepQuery := `
		SELECT
			step,
			COUNT(*) as request_count,
			COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
			COALESCE(SUM(cached_tokens), 0) as total_cached_tokens,
			COALESCE(SUM(cache_creation_input_tokens), 0) as total_cache_creation_tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM cache_metrics
		WHERE created_at >= datetime('now', '-' || ? || ' hours')`

	stepArgs := []interface{}{hours}
	if projectID != "" {
		stepQuery += " AND project_id = ?"
		stepArgs = append(stepArgs, projectID)
	}
	stepQuery += " GROUP BY step ORDER BY step"

	var stepStats []struct {
		Step                    string  `json:"step" db:"step"`
		RequestCount            int     `json:"request_count" db:"request_count"`
		TotalPromptTokens       int     `json:"total_prompt_tokens" db:"total_prompt_tokens"`
		TotalCachedTokens       int     `json:"total_cached_tokens" db:"total_cached_tokens"`
		TotalCacheCreationTokens int    `json:"total_cache_creation_tokens" db:"total_cache_creation_tokens"`
		AvgLatencyMs            float64 `json:"avg_latency_ms" db:"avg_latency_ms"`
	}

	if err := h.db.Select(&stepStats, stepQuery, stepArgs...); err != nil {
		// Non-fatal: return empty breakdown
		stepStats = []struct {
			Step                    string  `json:"step" db:"step"`
			RequestCount            int     `json:"request_count" db:"request_count"`
			TotalPromptTokens       int     `json:"total_prompt_tokens" db:"total_prompt_tokens"`
			TotalCachedTokens       int     `json:"total_cached_tokens" db:"total_cached_tokens"`
			TotalCacheCreationTokens int    `json:"total_cache_creation_tokens" db:"total_cache_creation_tokens"`
			AvgLatencyMs            float64 `json:"avg_latency_ms" db:"avg_latency_ms"`
		}{}
	}

	c.JSON(200, gin.H{
		"summary": gin.H{
			"total_requests":            stats.TotalRequests,
			"total_prompt_tokens":       stats.TotalPromptTokens,
			"total_cached_tokens":       stats.TotalCachedTokens,
			"total_cache_creation_tokens": stats.TotalCacheCreationTokens,
			"total_completion_tokens":   stats.TotalCompletionTokens,
			"avg_latency_ms":            stats.AvgLatencyMs,
			"cache_hit_rate_pct":        hitRate,
			"explicit_count":            stats.ExplicitCount,
			"implicit_count":            stats.ImplicitCount,
			"none_count":                stats.NoneCount,
		},
		"by_step": stepStats,
	})
}
