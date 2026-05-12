package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// DetectTenderConflicts performs a logic audit for internal contradictions in the tender document.
func (s *TenderDigitizationService) DetectTenderConflicts(ctx context.Context, projectID string, tenderContent string) (*ConflictAuditResult, error) {
	log.Printf("[Digitize] Module C: Starting logic conflict audit for project: %s", projectID)

	prompt := `你是一个极其严谨的招标方案审计专家。请寻找以下招标文件中的【内部逻辑矛盾】。你需要特别关注以下三个维度的交叉核验：

### 审计要点：
1. **工期与关键节点 (Schedule)**：总工期是否等于各阶段完成时间之和？开竣工日期是否矛盾？
2. **施工内容与界限 (Scope)**：图纸说明与工程量说明是否一致？甲供与乙供范围是否有重叠或遗漏？
3. **关键材料与价格 (Material)**：主要材料规格在不同章节描述是否统一？暂估金/暂列金额度是否前后一致？
4. **数字与条款 (Numeric/Terms)**：保证金比例、误期赔偿限额等数字在合同条款与前附表中是否矛盾？

只输出 JSON 格式的 ConflictAuditResult，不要任何解释文字：
{
  "conflicts": [
    {
      "conflict_id": "L1",
      "conflict_type": "schedule|scope|material|term|numeric",
      "field_name": "内容字段名（如：总工期）",
      "source_a": "证据 A（章节+原文）",
      "source_b": "证据 B（章节+原文）",
      "conflict_reason": "矛盾理由（简练描述）",
      "severity": "high|medium|low",
      "manual_review_required": true
    }
  ],
  "summary": "审计总述，总结矛盾分布情况...",
  "has_block": true/false (对于高风险且不可自动修订的情况设为 true)
}

招标文件全文（前 80k 字符）：
%s`

	tc := tenderContent
	if len(tc) > 80000 {
		tc = tc[:80000]
	}

	messages := []LLMMessageV2{
		BuildCacheableSystemMessage("你是一个专业的招标文件审计员。只返回 JSON。"),
		BuildDynamicUserBlock(fmt.Sprintf(prompt, tc)),
	}

	rescanCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	res, _, err := s.aiClient.CallLLMV2WithContext(rescanCtx, messages, 0.1)
	if err != nil {
		log.Printf("[Digitize] Conflict audit failed or timed out: %v", err)
		return &ConflictAuditResult{Summary: "审计超时或调用失败，已跳过逻辑矛盾核查"}, nil
	}

	var result ConflictAuditResult
	cleanJSON := s.extractJSON(res)
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return &ConflictAuditResult{Summary: "审计解析失败"}, nil
	}

	return &result, nil
}
