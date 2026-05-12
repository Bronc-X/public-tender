package service

import (
	"strings"
	"testing"
)

func TestExtractMarkdownFromDocParserData_RootMarkdown(t *testing.T) {
	s := extractMarkdownFromDocParserData(map[string]interface{}{
		"markdown": "# Title",
	})
	if s != "# Title" {
		t.Fatalf("got %q", s)
	}
}

func TestExtractMarkdownFromDocParserData_LayoutsMarkdownContent(t *testing.T) {
	data := map[string]interface{}{
		"Layouts": []interface{}{
			map[string]interface{}{"index": 0, "markdownContent": "第一段  \n\n"},
			map[string]interface{}{"index": 1, "markdownContent": "第二段  \n\n"},
		},
	}
	s := extractMarkdownFromDocParserData(data)
	if !strings.Contains(s, "第一段") || !strings.Contains(s, "第二段") {
		t.Fatalf("got %q", s)
	}
}

func TestExtractMarkdownFromDocParserData_NoLongerDumpsRawJSON(t *testing.T) {
	data := map[string]interface{}{
		"Layouts": []interface{}{
			map[string]interface{}{"index": 0, "foo": "bar"},
		},
	}
	s := extractMarkdownFromDocParserData(data)
	if strings.Contains(s, `"foo"`) || strings.Contains(s, `"Layouts"`) {
		t.Fatalf("unexpected JSON dump: %q", s)
	}
	if s != "" {
		t.Fatalf("expected empty, got %q", s)
	}
}

func TestMarkdownFromDocMindStoredJSON_Success(t *testing.T) {
	raw := `{"Layouts":[{"markdownContent":"# 标题\n\n"},{"markdownContent":"正文A  \n\n"}]}`
	md, err := MarkdownFromDocMindStoredJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "# 标题") || !strings.Contains(md, "正文A") {
		t.Fatalf("unexpected markdown: %q", md)
	}
}

func TestMarkdownFromDocMindStoredJSON_Invalid(t *testing.T) {
	_, err := MarkdownFromDocMindStoredJSON("not-json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkdownFromDocMindStoredJSON_EmptyExtract(t *testing.T) {
	raw := `{"Layouts":[{"index":1,"foo":"bar"}]}`
	_, err := MarkdownFromDocMindStoredJSON(raw)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkdownFromDocMindStoredJSON_RootArray(t *testing.T) {
	raw := `[{"markdownContent":"A"},{"markdownContent":"B"}]`
	md, err := MarkdownFromDocMindStoredJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "A") || !strings.Contains(md, "B") {
		t.Fatalf("got %q", md)
	}
}

func TestMarkdownFromDocMindStoredJSON_NestedData(t *testing.T) {
	raw := `{"Data":{"Layouts":[{"markdownContent":"嵌套"}]}}`
	md, err := MarkdownFromDocMindStoredJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "嵌套") {
		t.Fatalf("got %q", md)
	}
}
