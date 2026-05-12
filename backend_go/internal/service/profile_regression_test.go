package service

import (
	"strings"
	"testing"
)

// profileTestCase defines a single regression test scenario.
type profileTestCase struct {
	Name                string
	DocType             string            // narrative / table_heavy / appendix / mixed
	InputMarkdown       string            // Simulated markdown text for rule engine + keyword audit + industry detection
	ChunkOutputJSONs    []string          // Pre-baked LLM output JSONs for normalize
	ExpectedFields      map[string]string // field_path → expected substring in value
	ExpectedNonEmpty    []string          // field paths that should be non-empty after merge
	ExpectedAuditGroups []string          // keyword audit group names expected to fire
	ExpectedRuleHits    []string          // rule engine categories expected to fire
	ExpectedIndustry    string            // expected industry detection result
}

func getProfileTestCases() []profileTestCase {
	return []profileTestCase{
		// ── Case 1: Narrative document with duration and quality ──
		{
			Name:    "正文型_工期质量",
			DocType: "narrative",
			InputMarkdown: `# 第一章 招标公告
本项目为某市政道路改造工程，位于某市某区。
工期要求：总工期为180日历天，自开工令下达之日起计算。
竣工日期不得晚于2025年12月31日。
质量标准：合格，符合国家现行施工质量验收规范。
投标人须具备市政公用工程施工总承包二级及以上资质。
项目经理须具备二级注册建造师资格。`,
			ChunkOutputJSONs: []string{`{
				"project_base_info": {
					"project_name": {"value": "某市政道路改造工程", "source_text": "本项目为某市政道路改造工程", "confidence": 0.9},
					"location": {"value": "某市某区", "source_text": "位于某市某区", "confidence": 0.85},
					"duration_requirements": {"value": "180日历天", "source_text": "总工期为180日历天", "source_location": "第一章", "confidence": 0.9},
					"quality_standard": {"value": "合格", "source_text": "质量标准：合格", "confidence": 0.85}
				},
				"evaluation_and_performance_rules": {
					"total_duration": {"value": "180日历天", "source_text": "总工期为180日历天", "confidence": 0.9}
				},
				"bidder_requirements": {
					"qualification_requirements": {"value": "市政公用工程施工总承包二级及以上", "source_text": "须具备市政公用工程施工总承包二级及以上资质", "confidence": 0.85}
				}
			}`},
			ExpectedFields: map[string]string{
				"total_duration":             "180",
				"duration_requirements":      "180",
				"qualification_requirements": "二级",
			},
			ExpectedNonEmpty: []string{"total_duration", "duration_requirements", "qualification_requirements", "quality_standard"},
			ExpectedRuleHits: []string{"duration", "qualification"},
		},
		// ── Case 2: Table-heavy document with scoring ──
		{
			Name:    "表格型_评分",
			DocType: "table_heavy",
			InputMarkdown: `# 评标办法
本次招标采用综合评分法。
| 评审因素 | 分值 |
|---------|------|
| 技术方案 | 40分 |
| 商务报价 | 30分 |
| 企业信誉 | 20分 |
| 项目管理 | 10分 |
技术标权重占比60%，商务标权重占比40%。
加分项：具有类似业绩加2分。
废标条款：未按要求密封的投标文件按废标处理。`,
			ChunkOutputJSONs: []string{`{
				"evaluation_and_performance_rules": {
					"method_and_score_weights": {"value": "综合评分法，技术标60%，商务标40%", "source_text": "采用综合评分法", "confidence": 0.85},
					"scoring_items": [
						{"name": "技术方案", "value": "40分"},
						{"name": "商务报价", "value": "30分"},
						{"name": "企业信誉", "value": "20分"},
						{"name": "项目管理", "value": "10分"}
					],
					"disqualification_rules": [
						{"name": "密封要求", "value": "未按要求密封的投标文件按废标处理"}
					]
				},
				"bidder_requirements": {
					"bonus_items": [
						{"name": "类似业绩", "value": "加2分"}
					]
				}
			}`},
			ExpectedNonEmpty:    []string{"method_and_score_weights", "scoring_items", "disqualification_rules"},
			ExpectedRuleHits:    []string{"scoring"},
			ExpectedAuditGroups: []string{},
		},
		// ── Case 3: Narrative with procurement boundary ──
		{
			Name:    "正文型_采购边界",
			DocType: "narrative",
			InputMarkdown: `# 第三章 技术要求
一、材料供应
甲供材料：钢材、水泥由招标人提供。
乙供材料：砂石料、砖块由中标人自行采购。
供货范围详见附表。
二、施工要求
施工现场应设置围挡，高度不低于2.5米。
安全文明施工费不得低于工程造价的3%。`,
			ChunkOutputJSONs: []string{`{
				"construction_core_requirements": {
					"procurement_boundary": {"value": "甲供：钢材、水泥；乙供：砂石料、砖块", "source_text": "甲供材料：钢材、水泥由招标人提供", "confidence": 0.85},
					"owner_supplied_items": [
						{"name": "钢材", "value": "招标人提供"},
						{"name": "水泥", "value": "招标人提供"}
					],
					"contractor_supplied_items": [
						{"name": "砂石料", "value": "中标人自行采购"},
						{"name": "砖块", "value": "中标人自行采购"}
					],
					"site_management": {"value": "围挡高度不低于2.5米，安全文明施工费不低于3%", "confidence": 0.8}
				}
			}`},
			ExpectedFields: map[string]string{
				"procurement_boundary": "甲供",
			},
			ExpectedNonEmpty: []string{"procurement_boundary", "owner_supplied_items", "contractor_supplied_items", "site_management"},
			ExpectedRuleHits: []string{"procurement"},
		},
		// ── Case 4: Document with appendix/list ──
		{
			Name:    "附录型_设备清单",
			DocType: "appendix",
			InputMarkdown: `# 附录A 主要材料设备清单
1. PE管 DN200 约500米
2. 球阀 DN200 10个
3. 调压器 RTZ-50 2台
4. 燃气报警器 20套
5. 阴极保护设备 1套

施工需满足GB50028城镇燃气设计规范。
带气作业须编制专项方案。`,
			ChunkOutputJSONs: []string{`{
				"construction_core_requirements": {
					"material_equipment_rules": {"value": "PE管DN200、球阀、调压器、燃气报警器、阴极保护设备", "source_text": "附录A 主要材料设备清单", "confidence": 0.8},
					"special_operations": {"value": "带气作业须编制专项方案", "source_text": "带气作业须编制专项方案", "confidence": 0.75},
					"technical_specifications": {"value": "符合GB50028城镇燃气设计规范", "source_text": "施工需满足GB50028", "confidence": 0.8}
				}
			}`},
			ExpectedNonEmpty: []string{"material_equipment_rules", "special_operations", "technical_specifications"},
			ExpectedRuleHits: []string{},
			ExpectedIndustry: "能源燃气",
		},
		// ── Case 5: Mixed document with personnel requirements ──
		{
			Name:    "混合型_人员资格",
			DocType: "mixed",
			InputMarkdown: `# 投标人资格要求
投标人须具备建筑工程施工总承包一级及以上资质。
项目经理须具备一级注册建造师资格，且近5年有不少于2项类似工程业绩。
技术负责人1名，安全员2人，质检员1名。
至少配备5名持证上岗人员。`,
			ChunkOutputJSONs: []string{`{
				"bidder_requirements": {
					"qualification_requirements": {"value": "建筑工程施工总承包一级及以上", "source_text": "须具备建筑工程施工总承包一级及以上资质", "confidence": 0.9},
					"qualification_certificates": {"value": "一级注册建造师", "source_text": "项目经理须具备一级注册建造师资格", "confidence": 0.85},
					"performance_requirements": {"value": "近5年不少于2项类似工程业绩", "confidence": 0.8},
					"personnel_requirements": [
						{"name": "项目经理", "value": "一级注册建造师"},
						{"name": "技术负责人", "value": "1名"},
						{"name": "安全员", "value": "2人"},
						{"name": "质检员", "value": "1名"}
					]
				}
			}`},
			ExpectedFields: map[string]string{
				"qualification_requirements": "一级",
			},
			ExpectedNonEmpty: []string{"qualification_requirements", "qualification_certificates", "personnel_requirements", "performance_requirements"},
			ExpectedRuleHits: []string{"qualification", "personnel"},
		},
		// ── Case 6: Table-heavy equipment procurement ──
		{
			Name:    "表格型_材料采购",
			DocType: "table_heavy",
			InputMarkdown: `# 材料采购分工表
本项目为市政道路排水工程，含给水管道铺设及桥梁附属设施，雨污分流改造。
| 序号 | 材料名称 | 供应方 |
|------|---------|--------|
| 1 | 排水管 | 招标人采购 |
| 2 | 给水管 | 招标人采购 |
| 3 | 沥青 | 中标人采购 |
| 4 | 混凝土 | 中标人采购 |
路基处理完成后方可进行管道开挖，交通导改方案需报批。
总工期120个日历天。
工期节点：开工日期2025年3月1日。`,
			ChunkOutputJSONs: []string{`{
				"construction_core_requirements": {
					"procurement_boundary": {"value": "甲供：排水管、给水管；乙供：沥青、混凝土", "confidence": 0.85},
					"owner_supplied_items": [
						{"name": "排水管", "value": "招标人采购"},
						{"name": "给水管", "value": "招标人采购"}
					],
					"contractor_supplied_items": [
						{"name": "沥青", "value": "中标人采购"},
						{"name": "混凝土", "value": "中标人采购"}
					]
				},
				"evaluation_and_performance_rules": {
					"total_duration": {"value": "120个日历天", "confidence": 0.9}
				}
			}`},
			ExpectedNonEmpty: []string{"procurement_boundary", "owner_supplied_items", "contractor_supplied_items", "total_duration"},
			ExpectedRuleHits: []string{"duration", "procurement"},
			ExpectedIndustry: "市政",
		},
		// ── Case 7: Appendix with disqualification rules ──
		{
			Name:    "附录型_废标规则",
			DocType: "appendix",
			InputMarkdown: `# 附件：废标条款
1. 投标文件未按要求签字盖章的
2. 投标报价超出最高限价的
3. 投标人资质不满足要求的
4. 未提供投标保证金的

评标采用经评审的最低投标价法。
技术标30分，商务标70分。`,
			ChunkOutputJSONs: []string{`{
				"evaluation_and_performance_rules": {
					"method_and_score_weights": {"value": "经评审的最低投标价法，技术30分商务70分", "confidence": 0.85},
					"disqualification_rules": [
						{"name": "签字盖章", "value": "未按要求签字盖章"},
						{"name": "最高限价", "value": "投标报价超出最高限价"},
						{"name": "资质要求", "value": "投标人资质不满足要求"},
						{"name": "保证金", "value": "未提供投标保证金"}
					]
				}
			}`},
			ExpectedNonEmpty: []string{"method_and_score_weights", "disqualification_rules"},
			ExpectedRuleHits: []string{"scoring"},
		},
		// ── Case 8: Multi-chunk merge test ──
		{
			Name:    "多chunk_合并测试",
			DocType: "mixed",
			InputMarkdown: `总工期240日历天。质量标准合格。
投标人须具备特级资质。采购边界：甲供钢材。`,
			ChunkOutputJSONs: []string{
				`{
					"project_base_info": {
						"project_name": {"value": "多分块测试项目", "confidence": 0.8},
						"duration_requirements": {"value": "240日历天", "source_text": "总工期240日历天", "confidence": 0.85}
					},
					"evaluation_and_performance_rules": {
						"total_duration": {"value": "240日历天", "confidence": 0.85}
					}
				}`,
				`{
					"project_base_info": {
						"project_name": {"value": "多分块测试项目", "confidence": 0.9},
						"quality_standard": {"value": "合格", "confidence": 0.8}
					},
					"bidder_requirements": {
						"qualification_requirements": {"value": "特级资质", "confidence": 0.9}
					},
					"construction_core_requirements": {
						"procurement_boundary": {"value": "甲供钢材", "confidence": 0.8}
					}
				}`,
			},
			ExpectedFields: map[string]string{
				"total_duration":             "240",
				"qualification_requirements": "特级",
				"procurement_boundary":       "甲供",
			},
			ExpectedNonEmpty: []string{"total_duration", "duration_requirements", "qualification_requirements", "procurement_boundary", "quality_standard"},
			ExpectedRuleHits: []string{"duration", "qualification", "procurement"},
		},
		// ── Case 9: Water conservancy project ──
		{
			Name:    "水利工程",
			DocType: "narrative",
			InputMarkdown: `# 某水库除险加固工程
大坝防渗灌浆处理，帷幕灌浆深度30米。
围堰施工方案需专家论证。泵站改造含闸门更换。
总工期365日历天。
投标人须具备水利水电工程施工总承包二级及以上资质。`,
			ChunkOutputJSONs: []string{`{
				"project_base_info": {
					"project_name": {"value": "某水库除险加固工程", "confidence": 0.9},
					"duration_requirements": {"value": "365日历天", "confidence": 0.85}
				},
				"construction_core_requirements": {
					"special_operations": {"value": "帷幕灌浆深度30米，围堰施工需专家论证", "confidence": 0.8},
					"technical_specifications": {"value": "大坝防渗灌浆处理，泵站改造含闸门更换", "confidence": 0.75}
				},
				"evaluation_and_performance_rules": {
					"total_duration": {"value": "365日历天", "confidence": 0.85}
				},
				"bidder_requirements": {
					"qualification_requirements": {"value": "水利水电工程施工总承包二级及以上", "confidence": 0.9}
				}
			}`},
			ExpectedNonEmpty: []string{"total_duration", "special_operations", "qualification_requirements"},
			ExpectedRuleHits: []string{"duration", "qualification"},
			ExpectedIndustry: "水利",
		},
		// ── Case 10: Building construction project ──
		{
			Name:    "房建工程",
			DocType: "mixed",
			InputMarkdown: `# 某住宅楼工程
基坑深度12米，采用基坑支护方案。剪力墙结构。
精装修标准交付。消防安装需同步施工。
项目经理1人，技术负责人1名，安全员3人。
总工期540日历天。
技术标50分，商务标50分。`,
			ChunkOutputJSONs: []string{`{
				"project_base_info": {
					"project_name": {"value": "某住宅楼工程", "confidence": 0.9},
					"duration_requirements": {"value": "540日历天", "confidence": 0.85}
				},
				"construction_core_requirements": {
					"special_operations": {"value": "深基坑支护，基坑深度12米", "confidence": 0.8},
					"technical_specifications": {"value": "剪力墙结构，精装修标准交付，消防安装同步施工", "confidence": 0.75}
				},
				"evaluation_and_performance_rules": {
					"total_duration": {"value": "540日历天", "confidence": 0.85},
					"method_and_score_weights": {"value": "技术标50分，商务标50分", "confidence": 0.85}
				},
				"bidder_requirements": {
					"personnel_requirements": [
						{"name": "项目经理", "value": "1人"},
						{"name": "技术负责人", "value": "1名"},
						{"name": "安全员", "value": "3人"}
					]
				}
			}`},
			ExpectedNonEmpty: []string{"total_duration", "special_operations", "method_and_score_weights", "personnel_requirements"},
			ExpectedRuleHits: []string{"duration", "scoring", "personnel"},
			ExpectedIndustry: "房建",
		},
	}
}

// getFieldValueByPath extracts a field value from a profile result by path.
func getFieldValueByPath(profile ProjectProfileResult, path string) string {
	field := getProfileFieldByPath(&profile, path)
	if field != nil {
		return field.Value
	}
	if items := getProfileListByPath(&profile, path); items != nil && len(*items) > 0 {
		var parts []string
		for _, item := range *items {
			if strings.TrimSpace(item.Value) != "" {
				parts = append(parts, item.Value)
				continue
			}
			if strings.TrimSpace(item.Name) != "" {
				parts = append(parts, item.Name)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "；")
		}
	}
	// Check list fields
	switch path {
	case "owner_supplied_items":
		if len(profile.ConstructionCoreRequirements.OwnerSuppliedItems) > 0 {
			return "non_empty"
		}
	case "contractor_supplied_items":
		if len(profile.ConstructionCoreRequirements.ContractorSuppliedItems) > 0 {
			return "non_empty"
		}
	case "personnel_requirements":
		if len(profile.BidderRequirements.PersonnelRequirements) > 0 {
			return "non_empty"
		}
	case "scoring_items":
		if len(profile.EvaluationAndPerformanceRules.ScoringItems) > 0 {
			return "non_empty"
		}
	case "disqualification_rules":
		if len(profile.EvaluationAndPerformanceRules.DisqualificationRules) > 0 {
			return "non_empty"
		}
	case "bonus_items":
		if len(profile.BidderRequirements.BonusItems) > 0 {
			return "non_empty"
		}
	}
	return ""
}

// ── Test 1: Normalize + Merge + Finalize ──
func TestNormalizeAndMerge(t *testing.T) {
	svc := &TenderDigitizationService{}
	cases := getProfileTestCases()

	totalRecallChecks := 0
	totalRecallHits := 0
	totalTraceChecks := 0
	totalTraceHits := 0
	totalValueChecks := 0
	totalValueHits := 0

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Normalize each chunk
			var chunks []ProjectProfileResult
			for _, rawJSON := range tc.ChunkOutputJSONs {
				normalized := svc.NormalizeProjectProfileChunk(rawJSON)
				chunks = append(chunks, normalized)
			}

			// Merge
			merged, diffs := svc.MergeProjectProfileChunks(chunks)

			// Finalize with raw text for keyword audit + rule engine
			svc.finalizeProjectProfileResult(&merged, tc.InputMarkdown)

			// Check recall (non-empty fields)
			for _, path := range tc.ExpectedNonEmpty {
				totalRecallChecks++
				val := getFieldValueByPath(merged, path)
				if val != "" {
					totalRecallHits++
				} else {
					t.Errorf("[%s] Expected non-empty field %s but got empty", tc.Name, path)
				}
			}

			// Check traceability (non-empty fields should have source_text)
			allFieldPaths := []string{
				"project_name", "owner_unit", "location", "category_and_scope",
				"duration_requirements", "quality_standard",
				"procurement_boundary", "schedule_constraints",
				"qualification_requirements", "qualification_certificates",
				"method_and_score_weights", "total_duration",
			}
			for _, path := range allFieldPaths {
				field := getProfileFieldByPath(&merged, path)
				if field != nil && !field.Missing && field.Value != "" {
					totalTraceChecks++
					if field.SourceText != "" {
						totalTraceHits++
					}
				}
			}

			// Check value matches
			for path, expectedSubstr := range tc.ExpectedFields {
				totalValueChecks++
				val := getFieldValueByPath(merged, path)
				if strings.Contains(val, expectedSubstr) {
					totalValueHits++
				} else {
					t.Errorf("[%s] Field %s: expected to contain '%s', got '%s'", tc.Name, path, expectedSubstr, val)
				}
			}

			// Verify merge diffs are generated for multi-chunk cases
			if len(tc.ChunkOutputJSONs) > 1 && len(diffs) == 0 {
				t.Errorf("[%s] Expected merge diffs for multi-chunk case but got none", tc.Name)
			}
		})
	}

	// Summary metrics
	recallRate := float64(0)
	if totalRecallChecks > 0 {
		recallRate = float64(totalRecallHits) / float64(totalRecallChecks) * 100
	}
	traceRate := float64(0)
	if totalTraceChecks > 0 {
		traceRate = float64(totalTraceHits) / float64(totalTraceChecks) * 100
	}
	valueRate := float64(0)
	if totalValueChecks > 0 {
		valueRate = float64(totalValueHits) / float64(totalValueChecks) * 100
	}
	t.Logf("=== Regression Metrics ===")
	t.Logf("Recall Rate:       %.1f%% (%d/%d)", recallRate, totalRecallHits, totalRecallChecks)
	t.Logf("Traceability Rate: %.1f%% (%d/%d)", traceRate, totalTraceHits, totalTraceChecks)
	t.Logf("Value Match Rate:  %.1f%% (%d/%d)", valueRate, totalValueHits, totalValueChecks)
}

// ── Test 2: Keyword Audit Accuracy ──
func TestKeywordAuditAccuracy(t *testing.T) {
	cases := getProfileTestCases()

	for _, tc := range cases {
		if len(tc.ExpectedAuditGroups) == 0 {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			svc := &TenderDigitizationService{}
			var chunks []ProjectProfileResult
			for _, rawJSON := range tc.ChunkOutputJSONs {
				chunks = append(chunks, svc.NormalizeProjectProfileChunk(rawJSON))
			}
			merged, _ := svc.MergeProjectProfileChunks(chunks)
			auditHits := keywordAuditCheck(tc.InputMarkdown, &merged)

			hitGroups := make(map[string]bool)
			for _, hit := range auditHits {
				hitGroups[hit.Group] = true
			}

			for _, expected := range tc.ExpectedAuditGroups {
				if !hitGroups[expected] {
					t.Errorf("[%s] Expected keyword audit group '%s' to fire but it didn't", tc.Name, expected)
				}
			}
		})
	}
}

// ── Test 3: Rule Engine Hit Accuracy ──
func TestRuleEngineHits(t *testing.T) {
	cases := getProfileTestCases()

	totalExpected := 0
	totalHit := 0

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			svc := &TenderDigitizationService{}
			var chunks []ProjectProfileResult
			for _, rawJSON := range tc.ChunkOutputJSONs {
				chunks = append(chunks, svc.NormalizeProjectProfileChunk(rawJSON))
			}
			merged, _ := svc.MergeProjectProfileChunks(chunks)
			hits := RunRuleEngine(tc.InputMarkdown, &merged)

			hitCategories := make(map[string]bool)
			for _, hit := range hits {
				hitCategories[hit.Category] = true
			}

			for _, expected := range tc.ExpectedRuleHits {
				totalExpected++
				if hitCategories[expected] {
					totalHit++
				} else {
					t.Errorf("[%s] Expected rule category '%s' to hit but it didn't", tc.Name, expected)
				}
			}
		})
	}

	hitRate := float64(0)
	if totalExpected > 0 {
		hitRate = float64(totalHit) / float64(totalExpected) * 100
	}
	t.Logf("Rule Engine Hit Rate: %.1f%% (%d/%d)", hitRate, totalHit, totalExpected)
}

// ── Test 4: Merge Diff Log Correctness ──
func TestMergeDiffLog(t *testing.T) {
	svc := &TenderDigitizationService{}

	// Use multi-chunk test case (#8)
	tc := getProfileTestCases()[7] // "多chunk_合并测试"
	if tc.Name != "多chunk_合并测试" {
		t.Skip("Test case index mismatch")
	}

	var chunks []ProjectProfileResult
	for _, rawJSON := range tc.ChunkOutputJSONs {
		chunks = append(chunks, svc.NormalizeProjectProfileChunk(rawJSON))
	}

	_, diffs := svc.MergeProjectProfileChunks(chunks)

	if len(diffs) == 0 {
		t.Fatal("Expected merge diffs but got none")
	}

	// Verify diff structure
	for _, diff := range diffs {
		if diff.FieldLabel == "" {
			t.Error("Diff entry has empty FieldLabel")
		}
		if diff.Action == "" {
			t.Error("Diff entry has empty Action")
		}
		if diff.Reason == "" {
			t.Error("Diff entry has empty Reason")
		}
		// Validate action values
		validActions := map[string]bool{
			"skip_empty": true, "replace_empty": true, "append_evidence": true,
			"replace_better": true, "keep_existing": true, "close_score": true,
		}
		if !validActions[diff.Action] {
			t.Errorf("Invalid diff action: %s", diff.Action)
		}
	}

	t.Logf("Merge produced %d diff entries for %d chunks", len(diffs), len(tc.ChunkOutputJSONs))
}

// ── Test 5: Industry Detection Accuracy ──
func TestIndustryDetection(t *testing.T) {
	cases := getProfileTestCases()

	for _, tc := range cases {
		if tc.ExpectedIndustry == "" {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			detected := DetectIndustry(tc.InputMarkdown)
			if detected != tc.ExpectedIndustry {
				t.Errorf("[%s] Expected industry '%s', got '%s'", tc.Name, tc.ExpectedIndustry, detected)
			}
		})
	}
}
