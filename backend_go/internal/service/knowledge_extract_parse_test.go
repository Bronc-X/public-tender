package service

import (
	"strings"
	"testing"
)

func TestParseKnowledgeExtractJSON_Array(t *testing.T) {
	raw := `[{"name":"A","detail":"x"},{"title":"B"}]`
	items, err := parseKnowledgeExtractJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d", len(items))
	}
}

func TestParseKnowledgeExtractJSON_Fenced(t *testing.T) {
	raw := "```json\n[{\"name\":\"N\"}]\n```"
	items, err := parseKnowledgeExtractJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["name"] != "N" {
		t.Fatalf("%v", items)
	}
}

func TestParseKnowledgeExtractJSON_SingleObject(t *testing.T) {
	raw := `{"name":"one","source_section":"§1"}`
	items, err := parseKnowledgeExtractJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["name"] != "one" {
		t.Fatalf("%v", items)
	}
}

func TestParseKnowledgeExtractJSON_BOM(t *testing.T) {
	raw := "\ufeff[{\"name\":\"b\"}]"
	items, err := parseKnowledgeExtractJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal(items)
	}
}

func TestParseKnowledgeExtractJSON_Invalid(t *testing.T) {
	_, err := parseKnowledgeExtractJSON("not json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("unexpected: %v", err)
	}
}
