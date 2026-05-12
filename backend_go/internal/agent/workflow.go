package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/jmoiron/sqlx"
)

// FormGenerationInput is the input parameter when triggering the Agent Graph
type FormGenerationInput struct {
	ProjectID       string
	Chapter         string
	StartPage       int
	EndPage         int
	ChapterBindings string // Step 5 JSON mappings specific to this chapter
}

// BuildCommerceChapterGraph constructs the Eino Graph for extracting and filling document blanks.
// Flow: TemplateLoaderNode -> SlotExtractorNode -> SlotFillerNode
func BuildCommerceChapterGraph(ctx context.Context, db *sqlx.DB) (compose.Runnable[FormGenerationInput, BidActionList], error) {
	graph := compose.NewGraph[FormGenerationInput, BidActionList]()

	// 1. Template Loader Node (Lambda)
	// Fetches the markdown representation of the requested chapter using boundaries.
	graph.AddLambdaNode("TemplateLoaderNode", compose.InvokableLambda(func(ctx context.Context, input FormGenerationInput) (*ExtractPayload, error) {
		return RunTemplateLoader(ctx, input, db)
	}))

	// 2. Slot Extractor Node (Lambda wrapping an LLM)
	// Scans the text for blanks (_____) and parentheses ( ) to extract necessary fields.
	graph.AddLambdaNode("SlotExtractorNode", compose.InvokableLambda(func(ctx context.Context, input *ExtractPayload) (*FillPayload, error) {
		return RunSlotExtractor(ctx, input, db)
	}))

	// 3. Slot Filler Node (ReAct Agent / Tool Execution)
	// Uses the extracted slots, retrieves project data (PM names, Company info),
	// and generates the final filled form with reasons.
	graph.AddLambdaNode("SlotFillerNode", compose.InvokableLambda(func(ctx context.Context, input *FillPayload) (BidActionList, error) {
		return RunSlotFiller(ctx, input, db)
	}))

	// Define Edges
	graph.AddEdge(compose.START, "TemplateLoaderNode")
	graph.AddEdge("TemplateLoaderNode", "SlotExtractorNode")
	graph.AddEdge("SlotExtractorNode", "SlotFillerNode")
	graph.AddEdge("SlotFillerNode", compose.END)

	// Compile the graph
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile graph: %v", err)
	}

	return runnable, nil
}

// Payload structs for intermediate steps
type ExtractPayload struct {
	ProjectID       string
	Chapter         string
	MarkdownText    string
	ChapterBindings string
}

type FillPayload struct {
	ProjectID       string
	Chapter         string
	MarkdownText    string
	Slots           []BidActionSlot
	ChapterBindings string
}
