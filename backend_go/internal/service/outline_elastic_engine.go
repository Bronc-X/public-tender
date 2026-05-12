package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
	"backend_go/internal/model"
)

// StructureAction represents an atomic adjustment to the skeleton
type StructureAction string

const (
	ActionKeep      StructureAction = "keep"
	ActionMove      StructureAction = "move"
	ActionSplit     StructureAction = "split"
	ActionMerge     StructureAction = "merge"
	ActionPromote   StructureAction = "promote" // Section/Topic to Chapter
	ActionInsert    StructureAction = "insert"
	ActionWeaken    StructureAction = "weaken"
)

// StructureAdjustment details a specific change to a chapter or section
type StructureAdjustment struct {
	Action      StructureAction `json:"action"`
	TargetID    string          `json:"target_id,omitempty"`   // Original Chapter ID if applicable
	TargetName  string          `json:"target_name"`
	NewName     string          `json:"new_name,omitempty"`
	Reason      string          `json:"reason"`
	Evidence    []string        `json:"evidence"`
	Priority    int             `json:"priority"` // 1-100, higher moves forward
	NewParentID string          `json:"new_parent_id,omitempty"`
}

// OutlineStructurePlan is the output of the decision engine
type OutlineStructurePlan struct {
	ProjectID   string                `json:"project_id"`
	Adjustments []StructureAdjustment `json:"adjustments"`
	Profile     TenderStructureProfile `json:"tender_profile"`
	Rationale   string                `json:"rationale"`
}

// TenderStructureProfile identifies the "Structural DNA" of the tender
type TenderStructureProfile struct {
	Type          string   `json:"type"` // e.g., "construction_org", "special_workflow", "score_centric"
	KeyEmphasis   []string `json:"key_emphasis"`
	ChapterOrder  []string `json:"preferred_order"`
	MandatoryTags []string `json:"mandatory_tags"`
}

type ElasticOutlineEngine struct {
	db     *sqlx.DB
	ai     *AIClient
	prompt *PromptService
}

func NewElasticOutlineEngine(db *sqlx.DB, ai *AIClient, prompt *PromptService) *ElasticOutlineEngine {
	return &ElasticOutlineEngine{db: db, ai: ai, prompt: prompt}
}

// BuildOutlineStructurePlan analyzes all inputs to decide the directory structure
func (s *ElasticOutlineEngine) BuildOutlineStructurePlan(
	ctx context.Context,
	projectID string,
	skeleton model.IndustrySkeletonDB,
	facts FactExtractResult,
	requirements []RequirementRegisterEntry,
	globalContext ProjectGlobalContext,
) (*OutlineStructurePlan, error) {
	
	// 1. Parse Skeleton Chapters
	var chapters []model.LogicalChapter
	if err := json.Unmarshal([]byte(skeleton.LogicalChaptersJSON), &chapters); err != nil {
		return nil, fmt.Errorf("failed to parse skeleton chapters: %w", err)
	}

	// 2. Identify Tender Structural DNA (Profile)
	profile, err := s.IdentifyTenderStructureProfile(ctx, globalContext, facts)
	if err != nil {
		log.Printf("Warning: failed to identify tender profile: %v", err)
	}

	// 3. Score Fact Structure Weights
	weights := s.ScoreFactStructureWeights(facts, requirements)

	// 4. Generate Adjustments via AI Decision Logic
	// Logic: Core chapters must stay, non-core can move/merge/split based on weights.
	// High weight special topics get promoted to chapters.
	adjustments := s.decideAdjustments(chapters, weights, profile)

	return &OutlineStructurePlan{
		ProjectID:   projectID,
		Adjustments: adjustments,
		Profile:     *profile,
		Rationale:   "Automatically optimized based on tender profile and fact weights.",
	}, nil
}

// ScoreFactStructureWeights calculates the relative importance of topics for structural positioning
func (s *ElasticOutlineEngine) ScoreFactStructureWeights(facts FactExtractResult, reqs []RequirementRegisterEntry) map[string]float64 {
	weights := make(map[string]float64)
	
	// Process mandatory specs (high weight)
	for _, f := range facts.MandatorySpecs {
		weight := 50.0
		if f.Priority == "high" { weight += 30 }
		weights[f.Name] += weight
	}

	// Process special topics (promotion candidates)
	for _, f := range facts.SpecialTopics {
		weights[f.Name] += 60.0
		if f.Priority == "high" { weights[f.Name] += 20 }
	}

	// Process scoring items
	for _, f := range facts.ScoreItems {
		weights[f.Name] += 40.0
		if f.ScoreValue > 10 { weights[f.Name] += 20 }
	}

	return weights
}

func (s *ElasticOutlineEngine) CalculatePersonalizationScore(plan *OutlineStructurePlan) float64 {
	score := 0.0
	for _, adj := range plan.Adjustments {
		switch adj.Action {
		case ActionPromote:
			score += 15.0 // High impact
		case ActionSplit:
			score += 10.0
		case ActionMove:
			score += 5.0
		case ActionInsert:
			score += 10.0
		case ActionMerge:
			score += 8.0
		}
	}
	// Normalize or cap if necessary. Here we just return the sum of weights.
	return score
}

func (s *ElasticOutlineEngine) decideAdjustments(
	chapters []model.LogicalChapter,
	weights map[string]float64,
	profile *TenderStructureProfile,
) []StructureAdjustment {
	var results []StructureAdjustment

	// A. Handle Skeleton Chapters
	for _, ch := range chapters {
		adj := StructureAdjustment{
			Action:     ActionKeep,
			TargetID:   ch.ID,
			TargetName: ch.Name,
			Reason:     "Base industry skeleton chapter",
		}

		// Check if it should be moved forward based on profile preference
		for i, pref := range profile.ChapterOrder {
			if strings.Contains(ch.Name, pref) {
				adj.Action = ActionMove
				adj.Priority = 100 - i
				adj.Reason = fmt.Sprintf("Moved forward to match tender preference: %s", pref)
				break
			}
		}

		// Check for splitting (e.g. Technical measures chapter is often split if facts are heavy)
		if ch.CanSplit {
			heavyFacts := 0
			for name, w := range weights {
				if w > 80 && strings.Contains(name, ch.Name) { // Simplified matching
					heavyFacts++
				}
			}
			if heavyFacts > 3 {
				adj.Action = ActionSplit
				adj.Reason = "Splitting due to high volume of critical facts"
			}
		}

		results = append(results, adj)
	}

	// B. Handle Promotion (Facts/Topics to Chapters)
	for name, w := range weights {
		if w >= 100 { // Threshold for promotion to top-level chapter
			results = append(results, StructureAdjustment{
				Action:     ActionPromote,
				TargetName: name,
				Reason:     "High structural weight (Critical topic/Technical difficulty)",
				Priority:   80, // High priority insertion
			})
		}
	}

	return results
}

func (s *ElasticOutlineEngine) IdentifyTenderStructureProfile(
	ctx context.Context,
	globalContext ProjectGlobalContext,
	facts FactExtractResult,
) (*TenderStructureProfile, error) {
	// AI-Driven Profiling
	prompt, system := s.prompt.GetPromptFull("structure_profiler")
	if prompt == "" {
		// Fallback to heuristic if prompt template missing
		return s.identifyTenderStructureProfileHeuristic(facts), nil
	}

	factsJSON, _ := json.Marshal(facts)
	globalJSON, _ := json.Marshal(globalContext)
	
	input := fmt.Sprintf("Global Context: %s\n\nExtracted Facts: %s", string(globalJSON), string(factsJSON))
	
	resp, err := s.ai.CallLLMWithContext(ctx, []LLMMessage{
		{Role: "system", Content: system},
		{Role: "user",   Content: prompt + "\n\n" + input},
	}, 0.2)
	
	if err != nil {
		return s.identifyTenderStructureProfileHeuristic(facts), err
	}

	var profile TenderStructureProfile
	if err := json.Unmarshal([]byte(resp), &profile); err != nil {
		return s.identifyTenderStructureProfileHeuristic(facts), nil
	}

	return &profile, nil
}

func (s *ElasticOutlineEngine) identifyTenderStructureProfileHeuristic(facts FactExtractResult) *TenderStructureProfile {
	profile := &TenderStructureProfile{
		Type: "construction_org",
		KeyEmphasis: []string{"quality", "safety"},
		ChapterOrder: []string{"概况", "核心技术", "进度"},
	}

	if len(facts.SpecialTopics) > 2 {
		profile.Type = "special_workflow"
	}
	return profile
}

// ValidateOutlineAgainstStructurePlan ensures the generated outline respects the core structural decisions
func (s *ElasticOutlineEngine) ValidateOutlineAgainstStructurePlan(
	outline model.OutlineTitlesJSON, // Assuming this is defined or passed as raw list
	plan *OutlineStructurePlan,
) (bool, string) {
	// check if promoted chapters exist
	for _, adj := range plan.Adjustments {
		if adj.Action == ActionPromote {
			found := false
			for _, node := range outline.Nodes {
				if strings.Contains(node.Title, adj.TargetName) {
					found = true
					break
				}
			}
			if !found {
				return false, fmt.Sprintf("Missing critical promoted chapter: %s", adj.TargetName)
			}
		}
	}
	return true, "Consistency check passed"
}
