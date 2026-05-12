package service

import "unicode/utf8"

// TruncateMarkdownForConflictAgent limits tender markdown for Conflict Auditor Agent (CTO: 局部上下文最小化).
func TruncateMarkdownForConflictAgent(markdown string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 40000
	}
	if utf8.RuneCountInString(markdown) <= maxRunes {
		return markdown
	}
	runes := []rune(markdown)
	return string(runes[:maxRunes]) + "\n\n<!-- step4_context_truncated_for_conflict_agent -->"
}
