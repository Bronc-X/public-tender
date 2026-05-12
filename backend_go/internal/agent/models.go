package agent

// SlotType indicates whether the slot expects text or an image.
type SlotType string

const (
	SlotTypeText            SlotType = "text"
	SlotTypeImage           SlotType = "image"
	SlotTypePersonnelTable  SlotType = "personnel_table"
	SlotTypePerformance     SlotType = "performance_table"
	SlotTypeCompanyProfile  SlotType = "company_profile"
	SlotTypeCertificateList SlotType = "certificate_list"
)

// SlotStatus indicates the human-in-the-loop review status.
type SlotStatus string

const (
	StatusPending  SlotStatus = "pending_review"
	StatusApproved SlotStatus = "approved"
	StatusMissing  SlotStatus = "missing"
)

// BidActionSlot represents a single blank to be filled by the AI and reviewed by the human.
// This is strictly modeled to be output by the Eino Agent.
type BidActionSlot struct {
	SlotID           string     `json:"slot_id" description:"Unique identifier for this fillable slot"`
	ChapterPath      []string   `json:"chapter_path" description:"Hierarchical path for UI tree rendering"`
	SlotContextTitle string     `json:"slot_context_title" description:"Context or section title in the original document"`
	TargetField      string     `json:"target_field" description:"Human-readable name of the field to fill (e.g., Project Manager Name)"`
	SlotType         SlotType   `json:"slot_type" description:"Type of the slot, e.g., 'text', 'personnel_table'"`
	AISuggestedValue string     `json:"ai_suggested_value" description:"The auto-filled answer populated by the Context Fetching Tool"`
	Reason           string     `json:"reason" description:"Traceable reasoning behind the suggested value based on context"`
	Status           SlotStatus `json:"status" description:"Current status of human review"`
}

// BidActionList represents the full array of slots extracted from a document sub-section.
type BidActionList struct {
	ProjectID        string          `json:"project_id"`
	Chapter          string          `json:"chapter"`
	Slots            []BidActionSlot `json:"slots"`
	OriginalMarkdown string          `json:"original_markdown,omitempty"`
}
