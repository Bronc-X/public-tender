package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/jmoiron/sqlx"
)

// RunTemplateLoader simulates fetching the template markdown based on boundaries.
func RunTemplateLoader(ctx context.Context, input FormGenerationInput, db *sqlx.DB) (*ExtractPayload, error) {
	log.Printf("[Eino TemplateLoader] Loading Template for ProjectID: %s, Chapter: %s (Pages: %d-%d)\n",
		input.ProjectID, input.Chapter, input.StartPage, input.EndPage)

	var templatePath *string
	err := db.Get(&templatePath, "SELECT template_markdown_path FROM bid_projects WHERE id = ?", input.ProjectID)
	
	// Fallback simulated markdown just in case
	simulatedMarkdown := fmt.Sprintf(`
# %s

根据招标文件要求，投标人需要提供以下信息：
1. 项目经理姓名：__________
2. 项目经理身份证号：(   )
3. 投标总价（大写）：_______________
4. 投标单位：_______________

附送资料：
在此插入营业执照副本原件扫描件：[资质复印件]
`, input.Chapter)

	contentData := simulatedMarkdown
	if err == nil && templatePath != nil && *templatePath != "" {
		importBytes, err := os.ReadFile(*templatePath)
		if err == nil && len(importBytes) > 0 {
			contentData = string(importBytes)
			log.Printf("[Eino TemplateLoader] Successfully loaded physical template chunk: %s", *templatePath)
		} else {
			log.Printf("[Eino TemplateLoader] Warning: template_markdown_path found but failed to read: %v", err)
		}
	}

	return &ExtractPayload{
		ProjectID:       input.ProjectID,
		Chapter:         input.Chapter,
		MarkdownText:    contentData,
		ChapterBindings: input.ChapterBindings,
	}, nil
}

// RunSlotExtractor uses an LLM to parse the Markdown and identify blanks/slots.
func RunSlotExtractor(ctx context.Context, input *ExtractPayload, db *sqlx.DB) (*FillPayload, error) {
	log.Printf("[Eino SlotExtractor] Analyzing markdown (%d bytes) for blanks...", len(input.MarkdownText))

	endpoint, apiKey, modelID := GenerateEinoLLMClient(ctx, db)
	conf := &openai.ChatModelConfig{
		BaseURL: endpoint,
		APIKey:  apiKey,
		Model:   modelID,
	}
	llm, err := openai.NewChatModel(ctx, conf)
	if err != nil {
		log.Printf("[Eino SlotExtractor] Failed to init LLM: %v", err)
		return nil, err
	}

	prompt := fmt.Sprintf(`你是自动化标书装配 AI。请分析以下模板文件，识别其中需要的复合“业务数据块（DataBlock）”以及零散的“个性化词条（文本/贴图）”。
切忌将表格碎片化（例如不要把人员简历表拆成姓名、年龄等几十个空），当发现表格或结构化区域时，请将其整体提取为一个数据块。

文件内容：
%s

请返回纯 JSON 对象，严格遵循格式：
{
  "slots": [
    {
      "slot_id": "随机唯一的字符串例如 b_1",
      "chapter_path": ["所属大章", "所属小节"],
      "slot_context_title": "所需业务块大标题",
      "target_field": "填空题的名称或数据块标识，例如 '项目经理简历表' 或 '投标总价'",
      "slot_type": "必须是以下之一: text, image, personnel_table (人员表), performance_table (业绩表), company_profile (企业档案), certificate_list (资质证书池)"
    }
  ]
}
不要输出任何 markdown 标记如 \x60\x60\x60json，只输出原生的大括号。`, input.MarkdownText)

	msg := schema.UserMessage(prompt)
	resp, err := llm.Generate(ctx, []*schema.Message{msg})
	if err != nil {
		log.Printf("[Eino SlotExtractor] Generation failed: %v", err)
		return nil, err
	}

	var parsed struct {
		Slots []BidActionSlot `json:"slots"`
	}
	
	rawContent := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(resp.Content), "\x60\x60\x60"), "\x60\x60\x60json")
	if err := json.Unmarshal([]byte(rawContent), &parsed); err != nil {
		log.Printf("[Eino SlotExtractor] Failed to parse JSON (%v): %s", err, rawContent)
		return nil, err
	}

	// Default status
	for i := range parsed.Slots {
		parsed.Slots[i].Status = StatusPending
	}

	return &FillPayload{
		ProjectID:       input.ProjectID,
		Chapter:         input.Chapter,
		MarkdownText:    input.MarkdownText,
		Slots:           parsed.Slots,
		ChapterBindings: input.ChapterBindings,
	}, nil
}

// RunSlotFiller is the Decision Node that fetches context via Tools 
// and fills the expected slots using system data context.
func RunSlotFiller(ctx context.Context, input *FillPayload, db *sqlx.DB) (BidActionList, error) {
	log.Printf("[Eino SlotFiller] ReAct Agent triggered to answer %d slots...", len(input.Slots))

	endpoint, apiKey, modelID := GenerateEinoLLMClient(ctx, db)
	conf := &openai.ChatModelConfig{
		BaseURL: endpoint,
		APIKey:  apiKey,
		Model:   modelID,
	}
	llm, err := openai.NewChatModel(ctx, conf)
	if err != nil {
		log.Printf("[Eino SlotFiller] Failed to init LLM: %v", err)
		return BidActionList{}, err
	}

	// We pass the extracted Slots + Background DB Context to LLM to make the decision
	slotsJSON, _ := json.Marshal(input.Slots)
	prompt := fmt.Sprintf(`你是高度严谨的去幻觉自动化标书装配 AI。你需要填写的以下表单字段:
%s

【约束前提】：用户已经在上一步为你精准指派了以下强对应的人员/企业资质卡片和指令备注（如果是空的，说明没有任何有效输入），请你严格仅在此资料范围内进行匹配填充！
=== 第五步强绑定语料 ===
%s
========================

	任务：请依据上述第五步强绑定语料，严格为每一道填空题推导出合适的答案 (ai_suggested_value) 及推理理由 (reason)。
	【异常控制 - 极其重要】：如果在上述强绑定语料中，没有任何卡片能满足当前空缺逻辑，或者没有提供对应的材料，你【绝对不能】自行编造或尝试猜测任何内容！必须将该项的 ai_suggested_value 设置为 ""（空字符串），系统会在下一步进行警报预处理捕获！
	【图片类型的字段】：如果是要求上传图片（如营业执照)，请将 ai_suggested_value 设置为推断出的包含有效图片扩展名(.png/.jpg)的唯一资源标志名。
	【动态数组类型】：如 personnel_table (人员名单)、performance_table (业绩表)，请将匹配到的多条履历/业绩项直接格式化为一个包含对象属项的 JSON 原始字符串（例如 '[{"name":"张三","cert":"二级建造师"},{"name":"李四"...}]'），千万不要拆散结构。

	请返回纯 JSON 对象，格式如下：
{
  "filled_slots": [
    {
      "slot_id": "...",
      "ai_suggested_value": "真实抽取的答案或图片名（找不到给空字符串）",
      "reason": "依据..."
    }
  ]
}
不要输出 markdown 标记如 \x60\x60\x60json。`, string(slotsJSON), input.ChapterBindings)

	msg := schema.UserMessage(prompt)
	resp, err := llm.Generate(ctx, []*schema.Message{msg})
	if err != nil {
		log.Printf("[Eino SlotFiller] Generation failed: %v", err)
		return BidActionList{}, err
	}

	var parsed struct {
		FilledSlots []struct{
			SlotID string `json:"slot_id"`
			SuggestedValue string `json:"ai_suggested_value"`
			Reason string `json:"reason"`
		} `json:"filled_slots"`
	}

	rawContent := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(resp.Content), "\x60\x60\x60"), "\x60\x60\x60json")
	if err := json.Unmarshal([]byte(rawContent), &parsed); err != nil {
		log.Printf("[Eino SlotFiller] Failed to parse JSON (%v): %s", err, rawContent)
		return BidActionList{}, err
	}

	// Merge responses back into original array
	filledSlots := make([]BidActionSlot, len(input.Slots))
	for i, slot := range input.Slots {
		filledSlots[i] = slot
		for _, ans := range parsed.FilledSlots {
			if ans.SlotID == slot.SlotID {
				filledSlots[i].AISuggestedValue = ans.SuggestedValue
				filledSlots[i].Reason = ans.Reason
				if filledSlots[i].AISuggestedValue == "" {
					filledSlots[i].Status = StatusMissing
				} else {
					filledSlots[i].Status = StatusApproved // Auto approve for now if we found it
				}
				break
			}
		}
	}

	return BidActionList{
		ProjectID:        input.ProjectID,
		Chapter:          input.Chapter,
		Slots:            filledSlots,
		OriginalMarkdown: input.MarkdownText,
	}, nil
}
