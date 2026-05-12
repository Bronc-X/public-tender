package handler

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type techStep5ContentStats struct {
	SubsectionCount int
	ContentCount    int
}

func getTechStep5ContentStats(db *sqlx.DB, projectID string) (techStep5ContentStats, error) {
	var stats techStep5ContentStats
	if err := db.Get(&stats.SubsectionCount, `SELECT COUNT(*) FROM tech_bid_chapter_plans WHERE project_id = ? AND node_level = 'subsection'`, projectID); err != nil {
		return stats, err
	}
	if err := db.Get(&stats.ContentCount, `
		SELECT COUNT(DISTINCT chapter_id)
		FROM tech_bid_chapter_contents
		WHERE project_id = ? AND COALESCE(content_md, '') != ''`, projectID); err != nil {
		return stats, err
	}
	return stats, nil
}

func syncTechStep5StatusIfComplete(db *sqlx.DB, projectID string) (bool, error) {
	stats, err := getTechStep5ContentStats(db, projectID)
	if err != nil {
		return false, err
	}
	if stats.SubsectionCount == 0 || stats.ContentCount < stats.SubsectionCount {
		return false, nil
	}
	_, err = db.Exec(`
		UPDATE tech_bid_projects
		SET step5_status = 'success',
			step6_status = CASE WHEN COALESCE(step6_status, '') = '' THEN 'idle' ELSE step6_status END,
			last_error_message = NULL,
			updated_at = ?
		WHERE id = ?
			AND COALESCE(step5_status, '') NOT IN ('success', 'verified_pass')`,
		time.Now(), projectID)
	if err != nil {
		return false, err
	}
	return true, nil
}
