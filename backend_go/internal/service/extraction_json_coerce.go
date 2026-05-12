package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CoerceExtractionJSONArray 将 LLM 返回的多种字段习惯统一为 id/title/content/confidence/source_page，
// 解决「核心摘要」为空（模型使用 详细信息、内容、value 等键名）的问题。
func CoerceExtractionJSONArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	var rawItems []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &rawItems); err != nil {
		return raw
	}
	out := make([]map[string]interface{}, 0, len(rawItems))
	for i, m := range rawItems {
		if m == nil {
			continue
		}
		title := cleanExtractionTitle(firstStringFromMap(m,
			"title", "名称", "类目", "类目名称", "字段", "field", "label", "key", "项"))
		content := firstStringFromMap(m,
			"content", "summary", "详细信息", "详细内容", "内容", "value", "text", "detail", "描述",
			"extracted_value", "说明", "提取内容", "核心内容", "摘要信息", "data")
		id := firstStringFromMap(m, "id")
		if id == "" {
			id = strconv.Itoa(i + 1)
		}
		sp := firstStringFromMap(m, "source_page", "page", "页码", "页")
		if sp == "" {
			sp = "1"
		}
		conf := 0.9
		if c, ok := m["confidence"]; ok {
			switch v := c.(type) {
			case float64:
				conf = v
			case json.Number:
				f, _ := v.Float64()
				conf = f
			}
		}
		out = append(out, map[string]interface{}{
			"id":           id,
			"title":        title,
			"content":      content,
			"confidence":   conf,
			"source_page":  sp,
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return string(b)
}

func cleanExtractionTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "：:")
	s = strings.TrimSpace(s)
	return s
}

func firstStringFromMap(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case float64, bool, json.Number:
			s := strings.TrimSpace(fmt.Sprint(t))
			if s != "" && s != "<nil>" {
				return s
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(t))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
