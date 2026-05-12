package service

import "testing"

func TestValidateOutlineSemanticCoverageRequiresMappedPathMatch(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		SpecialTopics: []FactItem{
			{ID: "bid_opening_rule", Name: "开标唱标规则", Content: "报价表单独密封，大写金额优先。", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第一章 施工组织设计",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第三节 质量、安全与HSE管理",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、西南油气田分公司HSE标准现场落实措施",
							"requirement_ids": []string{"bid_opening_rule"},
						},
					},
				},
			},
		},
	}
	mappings := []FactOutlineMapping{
		{
			FactID:     "bid_opening_rule",
			TargetPath: []string{"第三章 商务及合同响应措施", "第三节 投标合规与报价策略", "四、开标唱标大写优先与单价汇总优先规则响应"},
		},
	}

	res := svc.ValidateOutlineSemanticCoverage(outline, facts, mappings)

	if res.Result == "PASS" {
		t.Fatalf("semantic mismatch passed coverage: %+v", res)
	}
	if res.FactMapped != 0 {
		t.Fatalf("FactMapped = %d, want 0 for mismatched mapped path", res.FactMapped)
	}
	if len(res.WeakFactIDs) == 0 || res.WeakFactIDs[0] != "bid_opening_rule" {
		t.Fatalf("WeakFactIDs = %v, want bid_opening_rule", res.WeakFactIDs)
	}
}

func TestValidateOutlineSemanticCoveragePassesMappedPathMatch(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		SpecialTopics: []FactItem{
			{ID: "bid_opening_rule", Name: "开标唱标规则", Content: "报价表单独密封，大写金额优先。", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第三章 商务及合同响应措施",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第三节 投标合规与报价策略",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "四、开标唱标大写优先与单价汇总优先规则响应",
							"requirement_ids": []string{"bid_opening_rule"},
						},
					},
				},
			},
		},
	}
	mappings := []FactOutlineMapping{
		{
			FactID:     "bid_opening_rule",
			TargetPath: []string{"第三章 商务及合同响应措施", "第三节 投标合规与报价策略", "四、开标唱标大写优先与单价汇总优先规则响应"},
		},
	}

	res := svc.ValidateOutlineSemanticCoverage(outline, facts, mappings)

	if res.Result != "PASS" {
		t.Fatalf("Result = %s, want PASS: %+v", res.Result, res)
	}
	if res.FactMapped != 1 {
		t.Fatalf("FactMapped = %d, want 1", res.FactMapped)
	}
}

func TestNormalizeRequirementIDsFromFactsRewritesFactAliases(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "ms_01", Name: "目录节点追溯要求", Content: "目录节点须可追溯到招标条款。", Priority: "high"},
			{ID: "ms_02", Name: "编制约束", Content: "事实引用须带来源位置。", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第一章 目录节点追溯要求响应",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 招标条款与评分点对应机制",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、目录节点与招标条款追溯表",
							"requirement_ids": []interface{}{"fact_1", "unknown"},
						},
						map[string]interface{}{
							"name":            "二、事实引用来源位置标注规范",
							"requirement_ids": []interface{}{"fact_2"},
						},
					},
				},
			},
		},
	}

	normalized := svc.NormalizeRequirementIDsFromFacts(outline, facts)
	units := normalized[0]["units"].([]interface{})
	subs := units[0].(map[string]interface{})["subsections"].([]interface{})
	firstIDs := subs[0].(map[string]interface{})["requirement_ids"].([]string)
	secondIDs := subs[1].(map[string]interface{})["requirement_ids"].([]string)

	if len(firstIDs) != 1 || firstIDs[0] != "ms_01" {
		t.Fatalf("first requirement_ids = %v, want [ms_01]", firstIDs)
	}
	if len(secondIDs) != 1 || secondIDs[0] != "ms_02" {
		t.Fatalf("second requirement_ids = %v, want [ms_02]", secondIDs)
	}
}

func TestValidateOutlineSemanticCoverageAllowsFactTitleContextMatch(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "ms_01", Name: "目录节点追溯要求", Content: "目录节点须可追溯到招标条款。", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第一章 目录节点追溯要求响应",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 招标条款与评分点对应机制",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、目录节点与招标条款追溯表",
							"requirement_ids": []string{"ms_01"},
						},
					},
				},
			},
		},
	}
	mappings := []FactOutlineMapping{
		{
			FactID:     "ms_01",
			TargetPath: []string{"CH1", "项目理解与总体分析"},
		},
	}

	res := svc.ValidateOutlineSemanticCoverage(outline, facts, mappings)

	if res.Result != "PASS" {
		t.Fatalf("Result = %s, want PASS via fact title context match: %+v", res.Result, res)
	}
}

func TestAlignFactMappingsToOutlineRelocatesMissingSkeletonPath(t *testing.T) {
	svc := &TenderDigitizationService{}
	outline := []map[string]interface{}{
		{
			"name": "第一章 施工方案和技术措施",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 天然气管道工程专项施工方案",
					"subsections": []interface{}{
						map[string]interface{}{"name": "一、气源管道工程及集中小区工程管道安装工艺"},
					},
				},
			},
		},
	}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "ms_01", Name: "天然气管道安装要求", Content: "天然气管道安装施工方案须完整。", Priority: "high"},
		},
	}
	mappings := []FactOutlineMapping{
		{FactID: "ms_01", FactType: "mandatory_spec", FactName: "天然气管道安装要求", TargetPath: []string{"CH1", "施工组织总体思路"}},
	}

	aligned := svc.AlignFactMappingsToOutline(outline, facts, mappings)
	if len(aligned) != 1 {
		t.Fatalf("expected one mapping, got %d", len(aligned))
	}
	if !pathMatchesOutline(aligned[0].TargetPath, "第一章 施工方案和技术措施", "第一节 天然气管道工程专项施工方案", "一、气源管道工程及集中小区工程管道安装工艺") {
		t.Fatalf("mapping was not relocated to generated outline path: %+v", aligned[0].TargetPath)
	}

	enforced := svc.EnforceRequirementIDsFromMappings(outline, aligned)
	got := svc.ValidateOutlineSemanticCoverage(enforced, facts, aligned)
	if got.CoverageRate != 100 || got.Result != "PASS" {
		t.Fatalf("expected aligned mapping to cover fact, got %+v", got)
	}
}

func TestEnsureProjectOverviewSectionAddsLandingZoneForProjectFacts(t *testing.T) {
	svc := &TenderDigitizationService{}
	outline := []map[string]interface{}{
		{
			"name": "第一章 施工方案和技术措施",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 天然气管道工程专项施工方案",
					"subsections": []interface{}{
						map[string]interface{}{"name": "一、气源管道工程及集中小区工程管道安装工艺"},
					},
				},
			},
		},
	}
	facts := &FactExtractResult{
		ProjectCharacteristics: []FactItem{
			{ID: "pc_01", Name: "项目名称", Content: "2026—2027年度天然气安装工程招标", Priority: "high"},
		},
		MandatorySpecs: []FactItem{
			{ID: "ms_01", Name: "目录节点追溯要求", Content: "目录节点须可追溯到招标条款/评分点。", Priority: "high"},
		},
	}

	withOverview := svc.EnsureProjectOverviewSection(outline, facts)
	if len(withOverview) != 2 {
		t.Fatalf("expected overview chapter to be prepended, got %d chapters", len(withOverview))
	}
	if got := withOverview[0]["name"]; got != "第一章 项目理解与招标要求响应总览" {
		t.Fatalf("overview name = %v", got)
	}
	units := withOverview[0]["units"].([]interface{})
	subs := units[0].(map[string]interface{})["subsections"].([]interface{})
	if len(subs) != 2 {
		t.Fatalf("expected project and traceability subsections, got %d", len(subs))
	}
}

func TestNormalizeOutlineNamesRenumbersPrependedOverview(t *testing.T) {
	outline := []map[string]interface{}{
		{"name": "第一章 项目理解与招标要求响应总览"},
		{"name": "第一章 施工组织设计"},
		{"name": "第二章 材料堆放场地"},
	}

	normalized := NormalizeOutlineNames(outline)

	if got := normalized[0]["name"]; got != "第一章 项目理解与招标要求响应总览" {
		t.Fatalf("overview name = %v", got)
	}
	if got := normalized[1]["name"]; got != "第二章 施工组织设计" {
		t.Fatalf("second chapter should be renumbered, got %v", got)
	}
	if got := normalized[2]["name"]; got != "第三章 材料堆放场地" {
		t.Fatalf("third chapter should be renumbered, got %v", got)
	}
}

func TestBackfillRequirementIDsFillsEmptySubsections(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "quality_acceptance", Name: "质量验收标准", Content: "质量验收合格标准", Priority: "high"},
			{ID: "safety_hse", Name: "HSE 安全管理", Content: "安全文明及 HSE 标准", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第二章 施工组织设计",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第二节 质量管理体系和措施",
					"subsections": []interface{}{
						map[string]interface{}{"name": "一、满足国家相关法律及现行验收规范合格标准"},
					},
				},
			},
		},
	}

	filled := svc.BackfillRequirementIDs(outline, facts)
	sub := filled[0]["units"].([]interface{})[0].(map[string]interface{})["subsections"].([]interface{})[0].(map[string]interface{})
	ids, ok := sub["requirement_ids"].([]string)
	if !ok || len(ids) == 0 {
		t.Fatalf("expected requirement_ids to be filled, got %#v", sub["requirement_ids"])
	}
}

func TestBackfillRequirementIDsAvoidsProjectFactsOutsideOverview(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "ms_quality", Name: "质量验收标准", Content: "验收规范合格标准", Priority: "high"},
		},
		ProjectCharacteristics: []FactItem{
			{ID: "pc_project", Name: "项目名称", Content: "天然气安装工程", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第二章 施工方案和技术措施",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 质量管理体系和措施",
					"subsections": []interface{}{
						map[string]interface{}{"name": "一、国家现行验收规范合格标准落实措施"},
					},
				},
			},
		},
	}

	filled := svc.BackfillRequirementIDs(outline, facts)
	sub := filled[0]["units"].([]interface{})[0].(map[string]interface{})["subsections"].([]interface{})[0].(map[string]interface{})
	ids := sub["requirement_ids"].([]string)
	if ids[0] == "pc_project" {
		t.Fatalf("technical subsection should not be backfilled with project fact: %v", ids)
	}
}

func TestBackfillRequirementIDsCreatesSubsectionForMatchingEmptyChapter(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		MandatorySpecs: []FactItem{
			{ID: "req_tech安全文明施工HSE标准", Name: "安全文明施工与 HSE 标准", Content: "按西南油气田分公司HSE有关规定达到安全文明施工标准。", Priority: "high"},
			{ID: "req_tech工期2026至2027", Name: "工期 2026 年至 2027 年截止 2027 年 12 月 31 日", Content: "工期：2026年-2027年（截止2027年12月31日）。", Priority: "high"},
		},
	}
	outline := []map[string]interface{}{
		{"name": "第八章 安全文明施工与 HSE 标准"},
		{"name": "第九章 工期 2026 年至 2027 年截止 2027 年 12 月 31 日"},
	}

	filled := svc.BackfillRequirementIDs(outline, facts)

	for i, wantID := range []string{"req_tech安全文明施工HSE标准", "req_tech工期2026至2027"} {
		units, ok := filled[i]["units"].([]interface{})
		if !ok || len(units) != 1 {
			t.Fatalf("chapter %d should get one unit, got %#v", i, filled[i]["units"])
		}
		subs := units[0].(map[string]interface{})["subsections"].([]interface{})
		ids := subs[0].(map[string]interface{})["requirement_ids"].([]string)
		if len(ids) != 1 || ids[0] != wantID {
			t.Fatalf("chapter %d ids = %v, want %s", i, ids, wantID)
		}
	}
}

func TestValidateOutlineSemanticCoverageAcceptsOverviewIndex(t *testing.T) {
	svc := &TenderDigitizationService{}
	facts := &FactExtractResult{
		ProjectCharacteristics: []FactItem{
			{ID: "pc_agent", Name: "招标代理机构", Content: "四川恒大建设项目管理有限公司", Priority: "medium"},
		},
	}
	outline := []map[string]interface{}{
		{
			"name": "第一章 项目理解与招标要求响应总览",
			"units": []interface{}{
				map[string]interface{}{
					"name": "第一节 项目基本情况与响应索引",
					"subsections": []interface{}{
						map[string]interface{}{
							"name":            "一、项目名称、招标人及招标代理机构响应",
							"requirement_ids": []string{"pc_agent"},
						},
					},
				},
			},
		},
	}
	cov := svc.ValidateOutlineSemanticCoverage(outline, facts, []FactOutlineMapping{
		{FactID: "pc_agent", TargetPath: []string{"第二章 施工组织设计", "第一节 施工方案", "一、专项措施"}},
	})
	if cov.CoverageRate != 100 {
		t.Fatalf("overview index should cover project facts, got %.1f", cov.CoverageRate)
	}
}

func TestMarshalRequirementIDsJSONAcceptsStringSlice(t *testing.T) {
	got := marshalRequirementIDsJSON([]string{"ms_quality", "pc_agent"})
	if got != `["ms_quality","pc_agent"]` {
		t.Fatalf("unexpected marshaled ids: %s", got)
	}
}
