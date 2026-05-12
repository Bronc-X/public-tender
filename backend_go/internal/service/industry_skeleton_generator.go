package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
)

type SkeletonTask struct {
	ID           string
	IndustryName string
	ParentName   string
}

type IndustrySkeletonGenerator struct {
	db            *sqlx.DB
	aiClient      *AIClient
	promptService *PromptService
	
	taskQueue    chan SkeletonTask
	isStarted    bool
	mu           sync.Mutex
}

func NewIndustrySkeletonGenerator(db *sqlx.DB, aiClient *AIClient, promptService *PromptService) *IndustrySkeletonGenerator {
	return &IndustrySkeletonGenerator{
		db:            db,
		aiClient:      aiClient,
		promptService: promptService,
		taskQueue:    make(chan SkeletonTask, 100),
	}
}

func (s *IndustrySkeletonGenerator) StartWorker() {
	s.mu.Lock()
	if s.isStarted {
		s.mu.Unlock()
		return
	}
	s.isStarted = true
	s.mu.Unlock()

	log.Println("[SkeletonWorker] Starting background worker...")
	
	// Recover pending tasks from DB
	var pendingTasks []struct {
		ID           string  `db:"id"`
		IndustryName string  `db:"industry_name"`
		ParentID     *string `db:"parent_id"`
	}
	err := s.db.Select(&pendingTasks, "SELECT id, industry_name, parent_id FROM tech_bid_industry_skeletons WHERE generation_status IN ('queued', 'processing')")
	if err == nil && len(pendingTasks) > 0 {
		log.Printf("[SkeletonWorker] Recovering %d pending tasks", len(pendingTasks))
		for _, pt := range pendingTasks {
			parentName := ""
			if pt.ParentID != nil {
				var pName string
				s.db.Get(&pName, "SELECT industry_name FROM tech_bid_industry_skeletons WHERE id = ?", *pt.ParentID)
				parentName = pName
			}
			s.taskQueue <- SkeletonTask{ID: pt.ID, IndustryName: pt.IndustryName, ParentName: parentName}
		}
	}

	go func() {
		for task := range s.taskQueue {
			s.processTask(task)
		}
	}()
}

func (s *IndustrySkeletonGenerator) EnqueueTask(id, industryName, parentName string) error {
	if id == "" || id == "undefined" {
		return fmt.Errorf("请先保存行业分类后再使用 AI 生成")
	}

	// 1. Verify existence and update status to queued
	res, err := s.db.Exec("UPDATE tech_bid_industry_skeletons SET generation_status = ? WHERE id = ?", "queued", id)
	if err != nil {
		return err
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("找不到对应的行业记录 (ID: %s)，请确认是否已保存", id)
	}

	// 2. Add to queue for sequential processing
	s.taskQueue <- SkeletonTask{ID: id, IndustryName: industryName, ParentName: parentName}
	
	// Ensure worker is running
	s.StartWorker()
	return nil
}

func (s *IndustrySkeletonGenerator) processTask(task SkeletonTask) {
	log.Printf("[SkeletonWorker] Processing task: %s (%s)", task.ID, task.IndustryName)

	// Update status to processing
	s.db.Exec("UPDATE tech_bid_industry_skeletons SET generation_status = ? WHERE id = ?", "processing", task.ID)

	draft, err := s.GenerateSkeletonDraft(context.Background(), task.IndustryName, task.ParentName)
	if err != nil {
		log.Printf("[SkeletonWorker] Task failed: %s, err: %v", task.ID, err)
		s.db.Exec("UPDATE tech_bid_industry_skeletons SET generation_status = ? WHERE id = ?", "error", task.ID)
		return
	}

	// Save draft to DB
	err = s.saveDraftToDB(task.ID, draft)
	if err != nil {
		log.Printf("[SkeletonWorker] Failed to save draft: %s, err: %v", task.ID, err)
		s.db.Exec("UPDATE tech_bid_industry_skeletons SET generation_status = ? WHERE id = ?", "error", task.ID)
		return
	}

	// Update status to done
	s.db.Exec("UPDATE tech_bid_industry_skeletons SET generation_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", "done", task.ID)
	log.Printf("[SkeletonWorker] Task completed: %s", task.ID)
}

func (s *IndustrySkeletonGenerator) saveDraftToDB(id string, draft *SkeletonDraft) error {
	kw, _ := json.Marshal(draft.IndustryKeywords)
	tp, _ := json.Marshal(draft.TitleCandidatePool)
	cp, _ := json.Marshal(draft.CommonSectionPool)

	// Convert chapters to JSON array
	chapters := mdToChapters(draft.LogicalChaptersMarkdown)
	chaptersJson, _ := json.Marshal(chapters)

	_, err := s.db.Exec(`UPDATE tech_bid_industry_skeletons SET 
		logical_chapters_json = ?, 
		industry_keywords_json = ?, 
		title_candidate_pool_json = ?, 
		common_section_pool_json = ? 
		WHERE id = ?`,
		string(chaptersJson), string(kw), string(tp), string(cp), id)
	return err
}

// mdToChapters is a helper to parse AI markdown back to structured JSON
func mdToChapters(md string) []LogicalChapter {
	lines := strings.Split(md, "\n")
	var chapters []LogicalChapter

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "##") {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "##"))
			chapter := LogicalChapter{
				ID:          fmt.Sprintf("ch_%d", len(chapters)+1),
				Name:        name,
				Description: "",
				IsMandatory: true,
				UnitPool:    []string{},
			}
			chapters = append(chapters, chapter)
		}
	}
	return chapters
}

type SkeletonDraft struct {
	IndustryName            string              `json:"industry_name"`
	LogicalChaptersMarkdown string              `json:"logical_chapters_markdown"`
	IndustryKeywords        []string            `json:"industry_keywords"`
	TitleCandidatePool      map[string][]string `json:"title_candidate_pool"`
	CommonSectionPool       []string            `json:"common_section_pool"`
}

func (s *IndustrySkeletonGenerator) GenerateSkeletonDraft(ctx context.Context, industryName, parentName string) (*SkeletonDraft, error) {
	// 1. Get prompts from DB
	promptKey := "industry_skeleton_generator"
	promptBody, sysPrompt := s.promptService.GetPromptFull(promptKey)

	// Default prompts if not in DB (as a fail-safe)
	if sysPrompt == "" {
		sysPrompt = "你是一个专业的建筑工程技术标书架构专家。你的任务是根据行业名称生成技术标目录骨架模版。只返回 JSON 格式结果。不要解释。"
	}
	if promptBody == "" {
		promptBody = `请为以下行业分类生成技术标骨架目录模板：
一级分类：{{parentIndustryName}}
二级分类：{{industryName}}

要求输出包含以下 4 个字段：
1. logical_chapters_markdown (使用 ## 章节名 {flags} 格式)
2. industry_keywords (30-80 个专业词汇)
3. title_candidate_pool (差异化标题候选)
4. common_section_pool (通用可选子节)

输出必须是纯 JSON，不要带 Markdown 标记块。`
	}

	// 2. Build Dynamic Prompt
	userPrompt := strings.ReplaceAll(promptBody, "{{parentIndustryName}}", parentName)
	userPrompt = strings.ReplaceAll(userPrompt, "{{industryName}}", industryName)

	// Append industry specific hints
	lowerParent := strings.ToLower(parentName)
	lowerChild := strings.ToLower(industryName)
	if strings.Contains(lowerParent, "水利") || strings.Contains(lowerChild, "水利") {
		userPrompt += "\n\n补充要求（水利类）：优先考虑导流围堰、度汛方案、水工建筑物、大体积混凝土、河道治理等专业章节。"
	} else if strings.Contains(lowerParent, "房建") || strings.Contains(lowerChild, "房建") {
		userPrompt += "\n\n补充要求（房建类）：优先考虑基坑支护、总进度计划、质量样板、智慧工地、绿色施工等专业章节。"
	} else if strings.Contains(lowerParent, "市政") || strings.Contains(lowerChild, "市政") {
		userPrompt += "\n\n补充要求（市政类）：优先考虑交通导改、管网保护、地下非开挖工艺、既有路口通行组织等章节。"
	} else if strings.Contains(lowerParent, "公路") || strings.Contains(lowerChild, "公路") {
		userPrompt += "\n\n补充要求（公路类）：优先考虑路基路面、线性组织、沥青摊铺、保通方案、测量试验等章节。"
	}
	// 3. Append global formatting and professional constraints
	userPrompt += "\n\n### 【极重要】格式清洗指令："
	userPrompt += "\n1. **标题规范**: 章用 ## 第一章，节用 ## 第一节。严禁使用列表符号。"
	userPrompt += "\n2. **专业度**: 严禁通用标题。必须结合【{{industryName}}】行业场景生成具体章节。"

	// 5. Call LLM
	messages := []LLMMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Use Doubao settings if available from DB
	var doubaoSettings []struct {
		Key   string  `db:"key"`
		Value *string `db:"value"`
	}
	s.db.Select(&doubaoSettings, "SELECT key, value FROM system_settings WHERE key IN ('doubao_endpoint', 'doubao_model_id', 'doubao_api_key')")

	ep, mid, key := "", "", ""
	for _, setting := range doubaoSettings {
		if setting.Value == nil {
			continue
		}
		switch setting.Key {
		case "doubao_endpoint":
			ep = *setting.Value
		case "doubao_model_id":
			mid = *setting.Value
		case "doubao_api_key":
			key = *setting.Value
		}
	}

	var res string
	var callErr error
	if key != "" && ep != "" && mid != "" {
		res, callErr = s.aiClient.CallLLMWithConfig(messages, 0.3, mid, ep, key)
	} else {
		res, callErr = s.aiClient.CallLLM(messages, 0.3)
	}

	if callErr != nil {
		return nil, fmt.Errorf("AI generation failed: %w", callErr)
	}

	// 6. Parse result
	cleanJSON := s.extractJSON(res)

	var draft SkeletonDraft
	if err := json.Unmarshal([]byte(cleanJSON), &draft); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &draft, nil
}

func (s *IndustrySkeletonGenerator) extractJSON(res string) string {
	res = strings.TrimSpace(res)
	if strings.HasPrefix(res, "```json") {
		res = strings.TrimPrefix(res, "```json")
		res = strings.TrimSuffix(res, "```")
	} else if strings.HasPrefix(res, "```") {
		res = strings.TrimPrefix(res, "```")
		res = strings.TrimSuffix(res, "```")
	}
	return strings.TrimSpace(res)
}
