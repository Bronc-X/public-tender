package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"backend_go/internal/model"
)

// ProcessCompanyAdaptation runs the Hybrid Engine for Step 3
func (s *TenderDigitizationService) ProcessCompanyAdaptation(ctx context.Context, projectID, companyID string, progressCb func(int, string)) (string, int, error) {
	progressCb(10, "正在收集企业库资料用于比对 (Qualifications/Performances/Persons)...")

	// 1. Fetch Company Inventory Data
	var qualifications []model.Qualification
	err := s.db.SelectContext(ctx, &qualifications, "SELECT * FROM qualification WHERE company_id = ?", companyID)
	if err != nil {
		log.Printf("Failed to load qualifications: %v", err)
	}

	var persons []model.Person
	err = s.db.SelectContext(ctx, &persons, "SELECT * FROM person WHERE company_id = ?", companyID)
	if err != nil {
		log.Printf("Failed to load persons: %v", err)
	}

	var performances []model.Performance
	err = s.db.SelectContext(ctx, &performances, "SELECT * FROM project_performance WHERE company_id = ? LIMIT 100", companyID)
	if err != nil {
		log.Printf("Failed to load performances: %v", err)
	}

	// Data Distillation for LLM context
	distilledQualifications := []map[string]interface{}{}
	for _, q := range qualifications {
		if q.QualificationName != "" {
			dist := map[string]interface{}{
				"id":       q.ID,
				"证书名称": q.QualificationName,
				"有效期至": "",
			}
			if q.QualificationType != nil && *q.QualificationType != "" {
				dist["资质类别"] = *q.QualificationType
			}
			if q.QualificationLevel != nil && *q.QualificationLevel != "" {
				dist["资质等级"] = *q.QualificationLevel
			}
			if q.ValidTo != nil {
				dist["有效期至"] = *q.ValidTo
			}
			distilledQualifications = append(distilledQualifications, dist)
		}
	}

	distilledPerformances := []map[string]interface{}{}
	for _, p := range performances {
		if p.ProjectName != "" {
			dist := map[string]interface{}{"id": p.ID, "项目名称": p.ProjectName, "中标金额": p.AmountValue, "竣工时间": ""}
			if p.CompletionDate != nil {
				dist["竣工时间"] = *p.CompletionDate
			}
			if p.ProjectManagerName != nil && *p.ProjectManagerName != "" {
				dist["项目经理"] = *p.ProjectManagerName
			}
			if p.ScaleDesc != nil && *p.ScaleDesc != "" {
				dist["建设规模"] = *p.ScaleDesc
			}
			distilledPerformances = append(distilledPerformances, dist)
		}
	}

	distilledPersons := []map[string]interface{}{}
	for _, p := range persons {
		if p.Name != "" {
			dist := map[string]interface{}{"id": p.ID, "姓名": p.Name, "职务角色": "", "执业号": ""}
			if p.RegistrationNo != nil {
				dist["执业号"] = *p.RegistrationNo
			}
			distilledPersons = append(distilledPersons, dist)
		}
	}

	progressCb(30, "加载第二步提取的规则库...")
	var latestRuleParseJSON string
	err = s.db.GetContext(ctx, &latestRuleParseJSON, "SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = ? AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", projectID, "extract_rules")

	if err != nil || latestRuleParseJSON == "" {
		return "", 0, fmt.Errorf("找不到上一步拆解的招标规则，请返回第二步重新解析, err = %v", err)
	}

	var parsed struct {
		Eligibility []map[string]interface{} `json:"eligibility"`
		Scoring     []map[string]interface{} `json:"scoring"`
	}
	json.Unmarshal([]byte(latestRuleParseJSON), &parsed)

	progressCb(50, "启动混合判定引擎 (本地审查 + AI 智能推断)...")

	passedCount := 0
	results := []map[string]interface{}{}

	// Combine all rules
	allRules := make([]map[string]interface{}, 0)
	allRules = append(allRules, parsed.Eligibility...)
	allRules = append(allRules, parsed.Scoring...)

	var unresolvedRules []map[string]interface{}

	for _, rule := range allRules {
		category, _ := rule["category"].(string)
		requirementText, _ := rule["requirement_text"].(string)

		res := map[string]interface{}{
			"id":                  rule["id"],
			"category":            category,
			"category_group":      rule["category_group"],
			"source_type":         rule["source_type"],
			"requirement_text":    requirementText,
			"confidence":          1.0,
			"matched_record_name": "",
			"matched_record_id":   "",
			"reason":              "",
			"status":              "warning",
		}

		if reqEv, ok := rule["requires_evidence"].(bool); ok && !reqEv {
			// 本地短路截杀，直接扔给 UI
			res["status"] = "ignored"
			res["matched_record_name"] = "无需提供实物证明材料"
			res["reason"] = "大模型前置提取判定：通用规范或承诺性质条款"
			results = append(results, res)
			continue
		}

		if category == "qualification" {
			// Local Engine (Route A): exact string match
			found := false
			for _, q := range qualifications {
				if q.QualificationName != "" && stringContains(requirementText, q.QualificationName) {
					res["status"] = "success"
					res["matched_record_name"] = q.QualificationName
					res["matched_record_id"] = q.ID
					res["reason"] = "本地引擎匹配成功：企业资质库中直接包含此资质关键字。"
					passedCount++
					found = true
					break
				}
			}
			if !found {
				res["status"] = "warning"
				res["reason"] = "未能精确定位匹配项，需人工或 AI 复核"
				unresolvedRules = append(unresolvedRules, res)
			}
		} else if category == "project_performance" || category == "person" {
			res["status"] = "warning"
			res["reason"] = "复杂语义条件，交由 AI 引擎进行推理验证..."
			unresolvedRules = append(unresolvedRules, res)
		} else {
			res["status"] = "warning"
			res["reason"] = "其他类别项，交由 AI 推理..."
			unresolvedRules = append(unresolvedRules, res)
		}
		results = append(results, res)
	}

	// ── AI Semantic Engine: single-request approach (proven stable) ──
	if len(unresolvedRules) > 0 {
		progressCb(70, fmt.Sprintf("调用大模型对 %d 条疑难条款进行深度语义判定...", len(unresolvedRules)))
		companyCtxBytes, _ := json.Marshal(map[string]interface{}{
			"qualifications": distilledQualifications,
			"performances":   distilledPerformances,
			"persons":        distilledPersons,
		})

		type AIResult struct {
			ID                string `json:"id"`
			Status            string `json:"status"`
			MatchedRecordName string `json:"matched_record_name"`
			MatchedRecordID   string `json:"matched_record_id"`
			Reason            string `json:"reason"`
		}

		sysPrompt := "你是一个严格的建筑招投标合规审查专家。你会收到公司的家底资料（JSON格式，含资质、业绩、人员），以及一批用本地过滤后无法解决的疑难招标要求。"
		
		chunkSize := 20
		var aiParsed []AIResult
		var hasError bool

		for i := 0; i < len(unresolvedRules); i += chunkSize {
			end := i + chunkSize
			if end > len(unresolvedRules) {
				end = len(unresolvedRules)
			}
			chunkRules := unresolvedRules[i:end]
			
			progressCb(70, fmt.Sprintf("调用大模型对疑难条款进行深度语义判定 (批次 %d/%d)...", (i/chunkSize)+1, (len(unresolvedRules)+chunkSize-1)/chunkSize))
			
			rulesBytes, _ := json.Marshal(chunkRules)
			userPrompt := fmt.Sprintf("【公司家底资料】\n%s\n\n【待验证的要求】\n%s\n\n请针对每条要求，在公司家底中寻找逻辑最匹配的证据。", string(companyCtxBytes), string(rulesBytes))
			userPrompt += "\n注意等级泛化推理，例如：如果公司具备\"一级资质\"，必须能够满足\"要求二级及以上资质\"。"
			userPrompt += "\n\n【判定逻辑优化与极速压缩指令】"
			userPrompt += "\n1. 若条款只是单纯的【纪律声明、合规承诺、一般性大话空话】（如：不得串通投标、不接受联合体、无违法记录等），且无需提供指定的硬性实体证明或加盖公章的专项承诺书函件，判定 status 为 'ignored'。"
			userPrompt += "\n   ★ 核心压缩指令：一旦判定为 'ignored'，必须强制将 `matched_record_name` 和 `reason` 输出为空字符串 `\"\"`！以节省输出空间！"
			userPrompt += "\n2. 若必须提供实体响应证明、或专项盖章承诺信件，请在公司家底匹配，若匹配成功设 'success'，否则 'warning'。只有遇到 success 或 warning 时，才需要写出精简的 `matched_record_name` 和推理逻辑 `reason`。"
			userPrompt += "\n\n请返回严格的 JSON 数组，每个元素格式如下：\n"
			userPrompt += "[{\"id\": \"被验证要求的内部id\", \"status\": \"success、warning、或ignored\", \"matched_record_id\": \"最吻合证据在JSON中的id/空\", \"matched_record_name\": \"最吻合的证据名/空\", \"reason\": \"推理逻辑/空\"}]"

			messages := []LLMMessageV2{
				BuildCacheableSystemMessage(sysPrompt),
				BuildDynamicUserBlock(userPrompt),
			}

			// 使用专用 4 分钟超时每批
			aiCtx, aiCancel := context.WithTimeout(ctx, 4*time.Minute)
			resJSONStr, _, err := s.aiClient.CallLLMV2WithContext(aiCtx, messages, 0.1)
			aiCancel()

			if err == nil && resJSONStr != "" {
				cleanJSON := s.extractJSON(resJSONStr)
				var batchParsed []AIResult
				if err := json.Unmarshal([]byte(cleanJSON), &batchParsed); err == nil {
					aiParsed = append(aiParsed, batchParsed...)
				} else {
					log.Printf("AI Result Parse Failed on chunk: %v, raw(first500): %.500s", err, cleanJSON)
					hasError = true
				}
			} else {
				log.Printf("AI Call Failed or empty on chunk: %v", err)
				hasError = true
			}
		}

		if hasError {
			log.Printf("警告：部分大模型推断批次发生错误或超时")
		}

		for _, ar := range aiParsed {
			for i, r := range results {
				idStr, _ := r["id"].(string)
				if idStr == ar.ID {
					results[i]["status"] = ar.Status
					if ar.Status == "ignored" {
						results[i]["matched_record_name"] = "无需提供证明材料"
						results[i]["reason"] = "大模型判定：这是通用的合规与纪律声明"
					} else {
						results[i]["matched_record_name"] = ar.MatchedRecordName
						results[i]["matched_record_id"] = ar.MatchedRecordID
						results[i]["reason"] = "AI 引擎判定：" + ar.Reason
					}
					if ar.Status == "success" {
						passedCount++
					}
					break
				}
			}
		}
	}

	progressCb(90, "正在组装验证结果...")
	summary := map[string]interface{}{
		"passed_count": passedCount,
		"status":       "warning",
	}
	if passedCount == len(allRules) && len(allRules) > 0 {
		summary["status"] = "success"
	}

	finalOutput := map[string]interface{}{
		"summary": summary,
		"results": results,
	}

	outJson, _ := json.Marshal(finalOutput)
	return string(outJson), passedCount, nil
}

func stringContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
