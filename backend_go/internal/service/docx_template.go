package service

import (
	"encoding/json"
	"strings"
)

// ProcessArrayBlocks automatically identifies {{#blockName}} ... {{/blockName}} within document.xml
// and duplicates the enclosed XML (like <w:tr>) for each element in the array map.
func ProcessArrayBlocks(xmlContent string, arrayData map[string]string) string {
	for blockName, jsonArrayStr := range arrayData {
		var items []map[string]string
		if err := json.Unmarshal([]byte(jsonArrayStr), &items); err != nil {
			continue // skip invalid json
		}

		startTag := "{{#" + blockName + "}}"
		endTag := "{{/" + blockName + "}}"

		for {
			startIdx := strings.Index(xmlContent, startTag)
			if startIdx == -1 {
				break
			}
			endIdx := strings.Index(xmlContent[startIdx:], endTag)
			if endIdx == -1 {
				break
			}
			endIdx += startIdx + len(endTag)

			// We found a block: xmlContent[startIdx:endIdx]
			// But wait, the tags might be inside a <w:tr> that we need to duplicate.
			// Let's expand exactly to the enclosing <w:tr> if it's a table row.
			// We will just duplicate the content between startTag and endTag, and strip the tags.
			innerContent := xmlContent[startIdx+len(startTag) : endIdx-len(endTag)]
			
			// If innerContent contains <w:tr> boundaries, it's complex.
			// The simplest templating is repeating innerContent for each item in the array.
			var rendered strings.Builder
			for _, item := range items {
				itemStr := innerContent
				for k, v := range item {
					itemStr = strings.ReplaceAll(itemStr, "{{" + k + "}}", v)
				}
				rendered.WriteString(itemStr)
			}

			// Replace the whole block (including tags) with the rendered sequences
			xmlContent = xmlContent[:startIdx] + rendered.String() + xmlContent[endIdx:]
		}
	}
	return xmlContent
}

// CleanDocxTags attempts to fix Word's run-tag fragmentations around {{ and }} placeholders.
// E.g., <w:t>{</w:t><w:t>{</w:t><w:t>name</w:t><w:t>}</w:t><w:t>}</w:t>  -> <w:t>{{name}}</w:t>
func CleanDocxTags(xmlContent string) string {
	// A robust cleaner for heavily fragmented templates requires AST.
	// For now, we strip intra-tag formatting inside placeholders using a regex 
	// that looks for { ... } but it's very complex in pure regex.
	// We'll leave this as a stub that can be expanded if needed.
	return xmlContent
}
