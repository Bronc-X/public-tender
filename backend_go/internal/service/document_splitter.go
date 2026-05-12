package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
)

type DocumentSplitterService struct {
	db *sqlx.DB
}

func NewDocumentSplitterService(db *sqlx.DB) *DocumentSplitterService {
	return &DocumentSplitterService{db: db}
}

// SplitMarkdownContent splits a full markdown string based on format_template_boundary detected.
func (s *DocumentSplitterService) SplitMarkdownContent(
	ctx context.Context,
	projectID string,
	content string,
	boundary DetectedBoundary,
) (rulePath string, templatePath string, err error) {
	if content == "" {
		return "", "", fmt.Errorf("markdown content is empty")
	}

	lines := strings.Split(content, "\n")

	splitIdx := -1

	// Strategy 1: If boundary startPage is useful, we try to find something like "[Page X]" or loosely matching "投标文件格式" headers in the latest 1/3 of document
	if boundary.Detected && boundary.StartPage > 0 {
		// Just a heuristic example: try to find a chapter heading like "第八章 投标文件格式"
		// The rule parsing from agent would give us "StartPage". If we have paging info in markdown we use it, otherwise we fall back to headers.
	}

	// Strategy 2: Regex Header Fallback
	if splitIdx == -1 {
		// Look for chapters indicating tender formats
		// commonly "第六章 投标文件格式", "第八章 投标文件格式", "第九章 附件格式"
		headerRegex := regexp.MustCompile(`(?m)^#+\s*(第[六七八九十][章部分]\s*)?(投标文件格式|附件格式|投标文件组成).*$`)
		loc := headerRegex.FindStringIndex(content)
		if loc != nil {
			// Find line index corresponding to loc[0]
			prefix := content[:loc[0]]
			splitIdx = len(strings.Split(prefix, "\n")) - 1
		}
	}

	// Strategy 3: No boundary found
	if splitIdx <= 0 || splitIdx >= len(lines) {
		// Fallback: don't split, just reuse the same or leave template empty
		splitIdx = len(lines) // rule gets everything
	}

	ruleLines := lines[:splitIdx]
	var templateLines []string
	if splitIdx < len(lines) {
		templateLines = lines[splitIdx:]
	}

	baseDir := filepath.Join("data", "projects", projectID)
	// ensure dir exists
	os.MkdirAll(baseDir, 0755)

	rulePath = filepath.Join(baseDir, "rule.md")
	templatePath = filepath.Join(baseDir, "template.md")

	err = os.WriteFile(rulePath, []byte(strings.Join(ruleLines, "\n")), 0644)
	if err != nil {
		return "", "", err
	}

	if len(templateLines) > 0 {
		err = os.WriteFile(templatePath, []byte(strings.Join(templateLines, "\n")), 0644)
		if err != nil {
			return "", "", err
		}
	} else {
		templatePath = "" // No template isolated
	}

	return rulePath, templatePath, nil
}

type DetectedBoundary struct {
	Detected  bool
	StartPage int
	EndPage   int
}
