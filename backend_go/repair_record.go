//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type LogicalChapter struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	IsMandatory        bool                `json:"is_mandatory"`
	UnitPool           []string            `json:"unit_pool"`
	SubsectionPool     map[string][]string `json:"subsection_pool"`
	IsCoreChapter      bool                `json:"is_core_chapter"`
	CanReorder         bool                `json:"can_reorder"`
	CanSplit           bool                `json:"can_split"`
	CanMerge           bool                `json:"can_merge"`
	CanInsertBefore    bool                `json:"can_insert_before"`
	CanInsertAfter     bool                `json:"can_insert_after"`
	PriorityRange      []string            `json:"priority_range"`
	FactTypePreference []string            `json:"fact_type_preference"`
}

func parseMarkdown(md string) []LogicalChapter {
	lines := strings.Split(md, "\n")
	var chapters []LogicalChapter
	var currentChapter *LogicalChapter
	var currentUnit string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isSub := strings.HasPrefix(line, "  - ")

		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			if currentChapter != nil {
				chapters = append(chapters, *currentChapter)
			}
			namePart := strings.TrimPrefix(strings.TrimPrefix(trimmed, "## "), "# ")
			isMandatory := strings.Contains(namePart, "(必选)")

			// Parse flags like {core, fix}
			flags := []string{}
			if start := strings.Index(namePart, "{"); start != -1 {
				if end := strings.Index(namePart, "}"); end != -1 {
					flagStr := namePart[start+1 : end]
					for _, f := range strings.Split(flagStr, ",") {
						flags = append(flags, strings.ToLower(strings.TrimSpace(f)))
					}
					namePart = namePart[:start] + namePart[end+1:]
				}
			}
			name := strings.TrimSpace(strings.ReplaceAll(namePart, "(必选)", ""))

			currentChapter = &LogicalChapter{
				ID:              fmt.Sprintf("CH%d", len(chapters)+1),
				Name:            name,
				IsMandatory:     isMandatory,
				UnitPool:        []string{},
				SubsectionPool:  make(map[string][]string),
				CanReorder:      true,
				CanInsertBefore: true,
				CanInsertAfter:  true,
			}
			for _, f := range flags {
				switch f {
				case "core":
					currentChapter.IsCoreChapter = true
				case "fix":
					currentChapter.CanReorder = false
				case "split":
					currentChapter.CanSplit = true
				case "merge":
					currentChapter.CanMerge = true
				}
			}
			currentUnit = ""
		} else if isSub && currentChapter != nil && currentUnit != "" {
			subName := strings.TrimPrefix(trimmed, "- ")
			currentChapter.SubsectionPool[currentUnit] = append(currentChapter.SubsectionPool[currentUnit], subName)
		} else if strings.HasPrefix(trimmed, "- ") && currentChapter != nil {
			currentUnit = strings.TrimPrefix(trimmed, "- ")
			currentChapter.UnitPool = append(currentChapter.UnitPool, currentUnit)
		} else if trimmed != "" && currentChapter != nil {
			if currentChapter.Description != "" {
				currentChapter.Description += "\n"
			}
			currentChapter.Description += trimmed
		}
	}
	if currentChapter != nil {
		chapters = append(chapters, *currentChapter)
	}
	return chapters
}

func main() {
	db, err := sqlx.Open("sqlite3", "/Users/raoyi/.openclaw/workspace/hudi/bid_data_management/backend_go/data/app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var record struct {
		ID                  string `db:"id"`
		LogicalChaptersJSON string `db:"logical_chapters_json"`
	}

	err = db.Get(&record, "SELECT id, logical_chapters_json FROM tech_bid_industry_skeletons WHERE industry_name LIKE '%城镇公共厕所%'")
	if err != nil {
		log.Fatal("Could not find record: ", err)
	}

	if !strings.HasPrefix(record.LogicalChaptersJSON, "##") {
		fmt.Println("Record already seems to be JSON or not Markdown. No repair needed.")
		return
	}

	chapters := parseMarkdown(record.LogicalChaptersJSON)
	jsonBytes, err := json.MarshalIndent(chapters, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("UPDATE tech_bid_industry_skeletons SET logical_chapters_json = ? WHERE id = ?", string(jsonBytes), record.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Successfully repaired record %s\n", record.ID)
}
