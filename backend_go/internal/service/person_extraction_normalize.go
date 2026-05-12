package service

import (
	"encoding/json"
	"strconv"
	"strings"
)

// personExtractionCanonicalTitles 人员档案结构化提取的固定类目顺序（与前端、归档逻辑一致）。
var personExtractionCanonicalTitles = []string{"姓名", "身份证号", "资格类型", "证书类别", "专业", "证书编号", "注册编号", "注册时间", "有效期截止", "颁发单位"}

type personRow struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Summary     string  `json:"summary,omitempty"`
	Content     string  `json:"content"`
	Confidence  float64 `json:"confidence"`
	SourcePage  string  `json:"source_page"`
}

func normalizePersonTitle(t string) string {
	t = strings.TrimSpace(t)
	switch t {
	case "职称":
		return "类别"
	case "注册专业", "执业专业", "专业类别":
		return "专业"
	default:
		return t
	}
}

// matchCanonicalPersonTitle 将 OCR/模型产生的标题归一到固定类目（cleanExtractionTitle 在 extraction_json_coerce.go）。
func matchCanonicalPersonTitle(raw string) string {
	t := cleanExtractionTitle(raw)
	if t == "" {
		return ""
	}
	if x := normalizePersonTitle(t); x != t || t == "职称" {
		return x
	}
	for _, canon := range personExtractionCanonicalTitles {
		if t == canon {
			return canon
		}
	}
	for _, canon := range personExtractionCanonicalTitles {
		if strings.Contains(t, canon) && len([]rune(t)) <= len([]rune(canon))+10 {
			return canon
		}
	}
	return t
}

func isPersonCanonicalTitle(t string) bool {
	for _, c := range personExtractionCanonicalTitles {
		if t == c {
			return true
		}
	}
	return false
}

func primaryText(it personRow) string {
	c := strings.TrimSpace(it.Content)
	if c != "" && c != "null" {
		return c
	}
	s := strings.TrimSpace(it.Summary)
	if s != "" && s != "null" {
		return s
	}
	return ""
}

// NormalizePersonExtractedDataJSON 将人员档案的 LLM 输出规范为多个固定 6 条类目的组（支持一个文件多个人）。
func NormalizePersonExtractedDataJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return buildEmptyPersonExtractionJSON()
	}
	var items []personRow
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return buildEmptyPersonExtractionJSON()
	}

	var groups []map[string]personRow
	currentGroup := make(map[string]personRow)

	for _, it := range items {
		t := matchCanonicalPersonTitle(it.Title)
		if t == "" || !isPersonCanonicalTitle(t) {
			continue
		}
		
		// 如果该组已经有了该标题（特别是“姓名”），通常意味着开始了新的一位人员
		if _, exists := currentGroup[t]; exists {
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = make(map[string]personRow)
		}

		it.Title = t
		it.Content = primaryText(it)
		it.Summary = ""
		currentGroup[t] = it
	}
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	if len(groups) == 0 {
		return buildEmptyPersonExtractionJSON()
	}

	var finalItems []personRow
	globalIdx := 1
	for _, group := range groups {
		for _, title := range personExtractionCanonicalTitles {
			id := strconv.Itoa(globalIdx)
			globalIdx++
			if it, ok := group[title]; ok {
				it.ID = id
				it.Title = title
				if strings.TrimSpace(it.SourcePage) == "" {
					it.SourcePage = "1"
				}
				finalItems = append(finalItems, it)
			} else {
				finalItems = append(finalItems, personRow{
					ID:         id,
					Title:      title,
					Content:    "",
					SourcePage: "1",
				})
			}
		}
	}

	b, err := json.Marshal(finalItems)
	if err != nil {
		return buildEmptyPersonExtractionJSON()
	}
	return string(b)
}

func buildEmptyPersonExtractionJSON() string {
	out := make([]personRow, 0, len(personExtractionCanonicalTitles))
	for i, title := range personExtractionCanonicalTitles {
		out = append(out, personRow{
			ID:         strconv.Itoa(i + 1),
			Title:      title,
			Content:    "",
			SourcePage: "1",
		})
	}
	b, _ := json.Marshal(out)
	return string(b)
}
