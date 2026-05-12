package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// CollectOutlineSubsectionTitles 收集目录中所有小节标题（与完全响应/行业规则遍历一致）
func CollectOutlineSubsectionTitles(outline []map[string]interface{}) []string {
	var out []string
	for _, ch := range outline {
		units, _ := ch["units"].([]interface{})
		for _, u := range units {
			um, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			subs, _ := um["subsections"].([]interface{})
			for _, sub := range subs {
				sm, ok := sub.(map[string]interface{})
				if !ok {
					continue
				}
				t := pickStringFromMap(sm, "name", "title")
				if strings.TrimSpace(t) != "" {
					out = append(out, t)
				}
			}
		}
	}
	return out
}

// DefaultJaccardSimilarityWarnThreshold 默认 Jaccard 雷同提示线；可被 system_settings 键 tech_bid_outline_similarity_config 中的 jaccard_warn_threshold 覆盖
const DefaultJaccardSimilarityWarnThreshold = 0.85

// JaccardSimilarityWarnThreshold 兼容旧名，等于默认阈值
const JaccardSimilarityWarnThreshold = DefaultJaccardSimilarityWarnThreshold

type outlineSimilarityConfigFile struct {
	JaccardWarnThreshold float64 `json:"jaccard_warn_threshold"`
}

const outlineSimilarityConfigKey = "tech_bid_outline_similarity_config"

// LoadJaccardSimilarityWarnThreshold 从 system_settings 读取；缺失、非法或越界 (0,1] 时返回默认值
func LoadJaccardSimilarityWarnThreshold(db *sqlx.DB) float64 {
	def := DefaultJaccardSimilarityWarnThreshold
	if db == nil {
		return def
	}
	var raw string
	if err := db.Get(&raw, `SELECT value FROM system_settings WHERE key = ?`, outlineSimilarityConfigKey); err != nil || strings.TrimSpace(raw) == "" {
		return def
	}
	var f outlineSimilarityConfigFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return def
	}
	if f.JaccardWarnThreshold <= 0 || f.JaccardWarnThreshold > 1 {
		return def
	}
	return f.JaccardWarnThreshold
}

// JaccardStringSet 基于字符串集合的 Jaccard 相似度（0~1），用于目录小节标题集合对比
func JaccardStringSet(a, b []string) float64 {
	ma := make(map[string]struct{}, len(a))
	for _, x := range a {
		x = normalizeTitleForSimilarity(x)
		if x == "" {
			continue
		}
		ma[x] = struct{}{}
	}
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		x = normalizeTitleForSimilarity(x)
		if x == "" {
			continue
		}
		mb[x] = struct{}{}
	}
	if len(ma) == 0 && len(mb) == 0 {
		return 1
	}
	inter := 0
	for x := range ma {
		if _, ok := mb[x]; ok {
			inter++
		}
	}
	union := len(ma) + len(mb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func normalizeTitleForSimilarity(title string) string {
	t := stripEnumerationPrefix(strings.TrimSpace(title))
	t = strings.ToLower(t)
	t = strings.Join(strings.Fields(t), "")
	return t
}

// BestHistoryJaccardHint 与同企业其他已存目录标题集比对，超过阈值则返回提示文案（否则空串）
func BestHistoryJaccardHint(db *sqlx.DB, companyID, excludeProjectID string, titles []string) string {
	if db == nil || strings.TrimSpace(companyID) == "" || len(titles) == 0 {
		return ""
	}
	var rows []struct {
		ProjectName string `db:"project_name"`
		TitlesJSON  string `db:"outline_titles_json"`
	}
	q := `SELECT project_name, outline_titles_json FROM tech_bid_projects WHERE company_id = ? AND id != ? AND outline_titles_json IS NOT NULL AND TRIM(outline_titles_json) != '' AND outline_titles_json != '[]'`
	if err := db.Select(&rows, q, companyID, excludeProjectID); err != nil {
		return ""
	}
	best := 0.0
	bestName := ""
	for _, r := range rows {
		var ot []string
		if json.Unmarshal([]byte(r.TitlesJSON), &ot) != nil || len(ot) == 0 {
			continue
		}
		j := JaccardStringSet(titles, ot)
		if j > best {
			best = j
			bestName = strings.TrimSpace(r.ProjectName)
		}
	}
	th := LoadJaccardSimilarityWarnThreshold(db)
	if best >= th && bestName != "" {
		return fmt.Sprintf("与历史项目「%s」目录小节标题集合 Jaccard 相似度 %.0f%%，请复核雷同风险", bestName, best*100)
	}
	return ""
}
