# 技术标骨架目录 AI 生成 - 接口草稿代码方案

## 1. 目标

本文提供“技术标骨架目录 AI 生成”功能的接口与后端代码草稿思路，便于工程师快速落地。

项目路径：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

---

## 2. 推荐新增接口

### 路由
`POST /api/settings/industry-skeletons/:id/generate`

### 说明
- 输入：二级分类 ID
- 输出：AI 生成的 4 个字段草案
- 不直接落库

---

## 3. 请求与响应结构

## 3.1 请求结构

```go
type GenerateSkeletonDraftRequest struct {
    Overwrite bool   `json:"overwrite"`
    Mode      string `json:"mode"` // draft / refine
}
```

## 3.2 响应结构

```go
type GenerateSkeletonDraftResponse struct {
    ID                       string              `json:"id"`
    IndustryName             string              `json:"industry_name"`
    ParentIndustryName       string              `json:"parent_industry_name"`
    LogicalChaptersMarkdown  string              `json:"logical_chapters_markdown"`
    IndustryKeywords         []string            `json:"industry_keywords"`
    TitleCandidatePool       map[string][]string `json:"title_candidate_pool"`
    CommonSectionPool        []string            `json:"common_section_pool"`
    PromptVersion            string              `json:"prompt_version"`
    RawModelOutput           string              `json:"raw_model_output,omitempty"`
}
```

---

## 4. 推荐新增 Service 文件

### 文件
`backend_go/internal/service/industry_skeleton_generator.go`

### 推荐结构

```go
package service

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "backend_go/internal/model"
    "github.com/jmoiron/sqlx"
)

type SkeletonDraft struct {
    LogicalChaptersMarkdown string              `json:"logical_chapters_markdown"`
    IndustryKeywords        []string            `json:"industry_keywords"`
    TitleCandidatePool      map[string][]string `json:"title_candidate_pool"`
    CommonSectionPool       []string            `json:"common_section_pool"`
}

type IndustrySkeletonGenerator struct {
    db *sqlx.DB
    ai *AIClient
}

func NewIndustrySkeletonGenerator(db *sqlx.DB, ai *AIClient) *IndustrySkeletonGenerator {
    return &IndustrySkeletonGenerator{db: db, ai: ai}
}
```

---

## 5. 推荐核心方法

## 5.1 GenerateSkeletonDraft

```go
func (s *IndustrySkeletonGenerator) GenerateSkeletonDraft(ctx context.Context, categoryID string, overwrite bool, mode string) (*GenerateSkeletonDraftResponse, error) {
    var row model.IndustrySkeletonDB
    if err := s.db.Get(&row, "SELECT * FROM tech_bid_industry_skeletons WHERE id = ? LIMIT 1", categoryID); err != nil {
        return nil, fmt.Errorf("分类不存在: %w", err)
    }

    var parentName string
    if row.ParentID != nil && strings.TrimSpace(*row.ParentID) != "" {
        _ = s.db.Get(&parentName, "SELECT industry_name FROM tech_bid_industry_skeletons WHERE id = ? LIMIT 1", *row.ParentID)
    }

    prompt := s.BuildPrompt(parentName, row, overwrite, mode)

    messages := []LLMMessage{
        {Role: "system", Content: s.BuildSystemPrompt()},
        {Role: "user", Content: prompt},
    }

    resp, err := s.ai.CallLLMWithContext(ctx, messages, 0.3)
    if err != nil {
        return nil, err
    }

    draft, err := s.ParseAndValidateDraft(resp)
    if err != nil {
        return nil, err
    }

    return &GenerateSkeletonDraftResponse{
        ID:                      row.ID,
        IndustryName:            row.IndustryName,
        ParentIndustryName:      parentName,
        LogicalChaptersMarkdown: draft.LogicalChaptersMarkdown,
        IndustryKeywords:        draft.IndustryKeywords,
        TitleCandidatePool:      draft.TitleCandidatePool,
        CommonSectionPool:       draft.CommonSectionPool,
        PromptVersion:           "skeleton_gen_v1",
        RawModelOutput:          resp,
    }, nil
}
```

---

## 5.2 BuildSystemPrompt

```go
func (s *IndustrySkeletonGenerator) BuildSystemPrompt() string {
    return `你是中国建筑工程投标领域的技术标目录架构专家。
你的任务不是写正文，而是为某个行业分类生成可复用的技术标骨架模板。
你必须输出纯 JSON，不允许输出解释，不允许输出 Markdown 代码块。`
}
```

---

## 5.3 BuildPrompt

```go
func (s *IndustrySkeletonGenerator) BuildPrompt(parentName string, row model.IndustrySkeletonDB, overwrite bool, mode string) string {
    existing := map[string]interface{}{
        "logical_chapters_json":    row.LogicalChaptersJSON,
        "common_section_pool_json": row.CommonSectionPoolJSON,
        "industry_keywords_json":   row.IndustryKeywordsJSON,
        "title_candidate_pool_json": row.TitleCandidatePoolJSON,
    }
    b, _ := json.MarshalIndent(existing, "", "  ")

    return fmt.Sprintf(`请为以下行业分类生成技术标骨架目录模板：

一级分类：%s
二级分类：%s
模式：%s
是否允许覆盖已有值：%v

当前已有配置：
%s

请输出严格 JSON：
{
  "logical_chapters_markdown": "...",
  "industry_keywords": ["..."],
  "title_candidate_pool": {
    "CH1": ["...", "..."]
  },
  "common_section_pool": ["...", "..."]
}`,
        parentName,
        row.IndustryName,
        mode,
        overwrite,
        string(b),
    )
}
```

---

## 5.4 ParseAndValidateDraft

```go
func (s *IndustrySkeletonGenerator) ParseAndValidateDraft(raw string) (*SkeletonDraft, error) {
    cleaned := strings.TrimSpace(raw)

    var draft SkeletonDraft
    if err := json.Unmarshal([]byte(cleaned), &draft); err != nil {
        return nil, fmt.Errorf("模型输出 JSON 解析失败: %w", err)
    }

    if strings.TrimSpace(draft.LogicalChaptersMarkdown) == "" {
        return nil, fmt.Errorf("缺少 logical_chapters_markdown")
    }
    if len(draft.IndustryKeywords) == 0 {
        return nil, fmt.Errorf("缺少 industry_keywords")
    }
    if len(draft.TitleCandidatePool) == 0 {
        return nil, fmt.Errorf("缺少 title_candidate_pool")
    }
    if len(draft.CommonSectionPool) == 0 {
        return nil, fmt.Errorf("缺少 common_section_pool")
    }
    if strings.Count(draft.LogicalChaptersMarkdown, "## ") < 3 {
        return nil, fmt.Errorf("章节数量不足")
    }

    return &draft, nil
}
```

---

## 6. 推荐 Handler 草稿

建议放在：
`backend_go/internal/handler/settings.go`

```go
func (h *SettingsHandler) GenerateIndustrySkeletonDraft(c *gin.Context) {
    id := c.Param("id")

    var req struct {
        Overwrite bool   `json:"overwrite"`
        Mode      string `json:"mode"`
    }
    _ = c.ShouldBindJSON(&req)
    if strings.TrimSpace(req.Mode) == "" {
        req.Mode = "draft"
    }

    generator := service.NewIndustrySkeletonGenerator(
        h.db,
        service.NewAIClient(h.apiKey, h.endpoint, h.model),
    )

    ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
    defer cancel()

    result, err := generator.GenerateSkeletonDraft(ctx, id, req.Overwrite, req.Mode)
    if err != nil {
        Error(c, http.StatusInternalServerError, err.Error())
        return
    }

    c.JSON(200, gin.H{
        "success": true,
        "data": result,
    })
}
```

---

## 7. 前端调用草稿

建议在 `SystemSettingsSkeletonTab.tsx` 增加：

```ts
const handleGenerate = async (record: IndustrySkeletonRecord) => {
  try {
    setGenerateLoadingId(record.id);
    const res = await axios.post(`/api/settings/industry-skeletons/${record.id}/generate`, {
      overwrite: false,
      mode: 'draft',
    });
    setGeneratedDraft(res.data?.data || null);
    setPreviewVisible(true);
  } catch (err) {
    message.error('AI 生成失败');
  } finally {
    setGenerateLoadingId(null);
  }
};
```

---

## 8. 预览弹窗回填草稿

```ts
const applyDraftToForm = () => {
  if (!generatedDraft) return;
  skeletonForm.setFieldsValue({
    logical_chapters_json: generatedDraft.logical_chapters_markdown,
    industry_keywords_json: JSON.stringify(generatedDraft.industry_keywords, null, 2),
    title_candidate_pool_json: JSON.stringify(generatedDraft.title_candidate_pool, null, 2),
    common_section_pool_json: JSON.stringify(generatedDraft.common_section_pool, null, 2),
  });
  setPreviewVisible(false);
};
```

---

## 9. 路由注册提醒

后端还需要补路由，例如：

```go
router.POST("/api/settings/industry-skeletons/:id/generate", settingsHandler.GenerateIndustrySkeletonDraft)
```

---

## 10. 建议事项

### 第一阶段建议先这样做
- 单次生成
- 单次返回 4 字段
- 用户审核后保存

### 先不要做
- 自动直接覆盖数据库
- 批量生成所有行业
- 多轮自动修复

---

## 11. 一句话结论

这套接口草稿方案的核心目标是：

**最小改动现有系统，新增一个“生成草案”的闭环能力。**

这样既能快速上线，又便于后续继续迭代 prompt 和行业增强能力。 
