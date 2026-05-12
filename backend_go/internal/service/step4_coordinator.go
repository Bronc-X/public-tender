package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"backend_go/internal/db"
	"backend_go/internal/model"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// OutlinePipelineHooks callbacks into handler (state transitions + structure plan persistence).
type OutlinePipelineHooks struct {
	UpdateProjectState         func(projectID string, fromStep, toStep *string, fromStatus, toStatus string, transitionType string, operatorID *string, operatorName string, verificationMethod string, reason string, metadata interface{}) error
	UpsertPendingStructurePlan func(projectID string, adjustmentsJSON string, profileJSON string, personalizationScore float64, rationale string) (string, error)
}

// OutlinePipelineInput parameters for RunOutlinePipeline.
// OutlineGenerationMode controls which outline generation path is used:
//
// # Mode Summary
//
//	"direct"   (default): LLM generates outline directly from tender content + facts + profile + route plan.
//	                     This is the recommended production mode (G1+G2 of the simplification plan).
//	"skeleton"            : Legacy skeleton-dominant path (elastic planning + user approval loop).
//	                     DEPRECATED: This mode is kept for backward compatibility only.
//	                     Will be removed in future versions. Please use "direct" mode.
type OutlinePipelineInput struct {
	Ctx                   context.Context
	DB                    *sqlx.DB
	Store                 *Step4Store
	Digitize              *TenderDigitizationService
	RunID                 int64
	ProjectID             string
	CompanyID             string
	RouteName             string
	ProfileData           string
	TenderContent         string
	ProfessionStr         string
	ProjectTypeStr        string
	OutlineGenerationMode string  // "direct" or "skeleton", default "direct". DEPRECATED: skeleton mode will be removed.
	ProjectStep           *string // current_step pointer for state updates
	Hooks                 OutlinePipelineHooks
	SelectedSkeletonID    string // 用户确认的骨架ID，优先使用此骨架生成目录
}

func wrapAgent(store *Step4Store, runID int64, projectID, agentName, stage, inputSummary string, fn func() (outputSummary string, err error)) {
	rowID, err := store.StartAgentRun(runID, projectID, agentName, stage, inputSummary)
	if err != nil {
		log.Printf("[Step4Coordinator] StartAgentRun %s: %v", stage, err)
		return
	}
	out, err := fn()
	if err != nil {
		em := err.Error()
		_ = store.CompleteAgentRun(rowID, "failed", "", &em)
		return
	}
	_ = store.CompleteAgentRun(rowID, "done", out, nil)
}

// RunOutlinePipeline executes the full Step4 closed-loop (same semantics as legacy SelectRoute goroutine).
func RunOutlinePipeline(p *OutlinePipelineInput) {
	ctx := p.Ctx
	id := p.ProjectID
	cid := p.CompanyID
	h := p.Hooks
	store := p.Store
	runID := p.RunID
	d := p.Digitize
	dbx := p.DB
	tenderContent := EnrichTenderForStep4(p.TenderContent, p.ProfessionStr, p.ProjectTypeStr)

	fail := func(msg string) {
		em := msg
		_ = store.FinishRun(runID, "failed", nil, &em)
	}

	up := func(fromSt, toSt string, reason string, meta interface{}) {
		_ = h.UpdateProjectState(id, p.ProjectStep, p.ProjectStep, fromSt, toSt, "ai", nil, "", "ai", reason, meta)
	}

	_ = store.UpdateRunStatus(runID, "running", "requirements_extracting", nil, nil)

	// 1 Facts
	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'facts_extracting', last_error_message = NULL WHERE id = ?", id)
	up("running", "running", "开始多视角章节化事实提取与废标项审查", nil)

	var facts *FactExtractResult
	var err error
	var requirementRegister []RequirementRegisterEntry
	deterministicFastPath := false
	deterministicRegister := BuildDeterministicRequirementRegister(tenderContent)
	wrapAgent(store, runID, id, "requirement_agent", "requirements_extracting", "招标文件 Markdown", func() (string, error) {
		if shouldUseDeterministicStep4FastPath(deterministicRegister) {
			deterministicFastPath = true
			requirementRegister = filterValidRequirements(deterministicRegister)
			facts = d.EnsureFactsForRequirements(&FactExtractResult{}, requirementRegister)
			if err := PersistFactsWithEvidence(dbx, id, facts); err != nil {
				return "", err
			}
			return fmt.Sprintf("确定性抽取要求 %d 条，已跳过慢速多视角 LLM 提取", len(requirementRegister)), nil
		}
		subCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
		facts, err = d.ExtractOutlineFacts(subCtx, id, tenderContent)
		if err != nil {
			return "", err
		}
		if err := PersistFactsWithEvidence(dbx, id, facts); err != nil {
			return "", err
		}
		n := 0
		if facts != nil {
			n = len(facts.ScoreItems) + len(facts.MandatorySpecs) + len(facts.ProjectCharacteristics) + len(facts.SpecialTopics)
		}
		return fmt.Sprintf("抽取事实条目约 %d 条", n), nil
	})
	if err != nil {
		log.Printf("[TechBid] Fact extraction failed: %v", err)
		dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "事实提取失败: "+err.Error(), id)
		up("running", "failed", "事实提取失败: "+err.Error(), nil)
		fail("事实提取失败: " + err.Error())
		return
	}

	_ = store.UpdateRunStatus(runID, "running", "facts_mapping", nil, nil)
	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'facts_ready' WHERE id = ?", id)

	wrapAgent(store, runID, id, "fact_agent", "facts_mapping", "requirement register + facts", func() (string, error) {
		if len(requirementRegister) == 0 {
			subCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
			defer cancel()
			requirementRegister, _ = d.BuildRequirementRegister(subCtx, id, facts, p.ProfileData, tenderContent)
		}
		// 过滤掉空 ID 的无效条目
		var validEntries []RequirementRegisterEntry
		for _, e := range requirementRegister {
			if strings.TrimSpace(e.RequirementID) != "" {
				validEntries = append(validEntries, e)
			}
		}
		if len(validEntries) < len(requirementRegister) {
			log.Printf("[Step4] ⚠️ 需求登记表中有 %d 条空ID条目已过滤", len(requirementRegister)-len(validEntries))
		}
		requirementRegister = validEntries
		facts = d.EnsureFactsForRequirements(facts, requirementRegister)
		_ = PersistFactsWithEvidence(dbx, id, facts)
		_ = PersistRequirementRegister(dbx, id, requirementRegister)
		_ = store.SnapshotRequirementsFromLegacy(runID, id)
		return fmt.Sprintf("requirement %d 条（有效）", len(requirementRegister)), nil
	})

	countFacts := func(r *FactExtractResult) int {
		if r == nil {
			return 0
		}
		return len(r.ScoreItems) + len(r.MandatorySpecs) + len(r.ProjectCharacteristics) + len(r.SpecialTopics)
	}
	matrix := d.BuildRequirementCoverageMatrix(facts, requirementRegister)
	rescanWarnings := make([]string, 0, 2)
	for i := 0; i < 2 && matrix.MissingCount > 0; i++ {
		missingBefore := matrix.MissingCount
		factCountBefore := countFacts(facts)
		up("running", "running", fmt.Sprintf("发现 %d 条漏项，正在执行第 %d 轮回扫补抽...", matrix.MissingCount, i+1), nil)
		subCtx, cancel := context.WithTimeout(ctx, 240*time.Second)
		newFacts, rescanErr := d.RescanMissingRequirements(subCtx, id, matrix, requirementRegister, tenderContent)
		cancel()
		if rescanErr != nil {
			warn := fmt.Sprintf("回扫补抽第 %d 轮失败，已使用当前 facts 继续：%v", i+1, rescanErr)
			rescanWarnings = append(rescanWarnings, warn)
			break
		}
		if newFacts == nil {
			warn := fmt.Sprintf("回扫补抽第 %d 轮未返回结果，已使用当前 facts 继续", i+1)
			rescanWarnings = append(rescanWarnings, warn)
			break
		}
		mergedFacts := d.MergeAndDeduplicateFacts([]FactExtractResult{*facts, *newFacts})
		matrixAfter := d.BuildRequirementCoverageMatrix(mergedFacts, requirementRegister)
		factCountAfter := countFacts(mergedFacts)
		if factCountAfter <= factCountBefore || matrixAfter.MissingCount >= missingBefore {
			warn := fmt.Sprintf("回扫补抽第 %d 轮未产生有效进展，沿用当前 facts 继续", i+1)
			rescanWarnings = append(rescanWarnings, warn)
			break
		}
		facts = mergedFacts
		matrix = matrixAfter
		_ = PersistFactsWithEvidence(dbx, id, facts)
	}
	if len(rescanWarnings) > 0 {
		_, _ = dbx.Exec("UPDATE tech_bid_projects SET last_error_message = ? WHERE id = ?", strings.Join(rescanWarnings, "；"), id)
	}

	_ = store.SnapshotFactCandidatesFromOutlineFacts(runID, id)

	_ = store.UpdateRunStatus(runID, "running", "conflict_auditing", nil, nil)
	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'auditing_outline', last_error_message = NULL WHERE id = ?", id)
	up("running", "running", "初步目录已生成，正在执行行业语义审计与冲突核查...", nil)

	var conflicts *ConflictAuditResult
	wrapAgent(store, runID, id, "conflict_auditor_agent", "conflict_auditing", "招标文件全文", func() (string, error) {
		if deterministicFastPath {
			conflicts = &ConflictAuditResult{Summary: "确定性 Step4 快路已跳过慢速模型冲突审计；未发现结构化硬阻断冲突。"}
			_ = PersistConflictAudit(dbx, id, conflicts)
			_ = store.AppendConflictSnapshot(runID, id, 1, conflicts)
			return "确定性快路跳过模型冲突审计", nil
		}
		subCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
		conflictInput := TruncateMarkdownForConflictAgent(tenderContent, 40000)
		conflicts, _ = d.DetectTenderConflicts(subCtx, id, conflictInput)
		cancel()
		_ = PersistConflictAudit(dbx, id, conflicts)
		_ = store.AppendConflictSnapshot(runID, id, 1, conflicts)
		if conflicts != nil {
			return fmt.Sprintf("冲突条目 %d", len(conflicts.Conflicts)), nil
		}
		return "无冲突", nil
	})

	// Determine which outline generation path to use
	// Default to "direct" mode (LLM generates outline directly from tender content)
	// "skeleton" mode preserves the legacy skeleton-dominant pipeline for rollback testing
	mode := strings.TrimSpace(p.OutlineGenerationMode)
	if mode == "" {
		mode = "direct" // Default to direct generation mode (G1+G2 of the simplification plan)
	}

	// P1-3: Governance - skeleton mode boundary enforcement
	if mode == "skeleton" {
		// DEPRECATED: Log deprecation warning to track skeleton mode usage
		log.Printf("[Step4Coordinator] ⚠️ DEPRECATED: Skeleton mode is deprecated and will be removed in future versions.")
		log.Printf("[Step4Coordinator] ⚠️ Skeleton mode usage - ProjectID: %s, CompanyID: %s", id, cid)
		log.Printf("[Step4Coordinator] ⚠️ Please migrate to direct mode (default) for production use.")
	} else if mode != "direct" {
		// Invalid mode fallback to direct
		log.Printf("[Step4Coordinator] Invalid OutlineGenerationMode '%s', falling back to 'direct' mode", mode)
		mode = "direct"
	}

	gateDecision, gateReason := d.FinalDigitizationValidation(matrix, conflicts)

	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'generating_outline' WHERE id = ?", id)

	var draftOutline []map[string]interface{}
	var mappings []FactOutlineMapping
	var genErr error

	if mode == "skeleton" {
		// Legacy skeleton-dominant path (G6 bypass mode)
		log.Printf("[Step4Coordinator] Running in skeleton mode (legacy path)")

		// 优先使用用户确认的骨架
		var skDB *model.IndustrySkeletonDB
		var sk *IndustrySkeleton
		if p.SelectedSkeletonID != "" {
			log.Printf("[Step4Coordinator] Using user-selected skeleton: %s", p.SelectedSkeletonID)
			skDB, _ = d.LoadSkeletonByID(p.SelectedSkeletonID)
			if skDB != nil {
				sk = d.mapDBToSkeleton(*skDB)
			}
		}
		// 如果没有用户选择的骨架，回退到自动匹配
		if skDB == nil {
			skDB, _ = d.LoadIndustrySkeletonDB(p.ProjectTypeStr, p.ProfessionStr)
			sk = d.LoadIndustrySkeleton(p.ProjectTypeStr, p.ProfessionStr)
		}

		_ = store.UpdateRunStatus(runID, "running", "outline_planning", nil, nil)

		wrapAgent(store, runID, id, "outline_planner_agent", "outline_planning", "facts + skeleton", func() (string, error) {
			subCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
			mappings, _ = d.BuildFactToOutlineMappings(subCtx, id, facts, sk)
			cancel()
			_ = PersistStep4FactMappings(dbx, id, mappings)
			_ = store.SnapshotFactMappingsFromLegacy(runID, id)
			return fmt.Sprintf("映射 %d 条", len(mappings)), nil
		})

		var approvedPlanJSON string
		for {
			globalCtx, _ := d.GetProjectGlobalContext(ctx, tenderContent, p.ProfileData)
			if skDB == nil || globalCtx == nil {
				log.Printf("[TechBid] Skipping elastic planning due to nil context")
				break
			}
			subCtx, cancel := context.WithTimeout(ctx, 240*time.Second)
			structurePlan, err := d.ElasticEngine.BuildOutlineStructurePlan(subCtx, id, *skDB, *facts, requirementRegister, *globalCtx)
			cancel()
			if err != nil {
				log.Printf("[TechBid] Structure planning failed: %v", err)
				break
			}
			adjJSON, _ := json.Marshal(structurePlan.Adjustments)
			profJSON, _ := json.Marshal(structurePlan.Profile)
			pScore := d.ElasticEngine.CalculatePersonalizationScore(structurePlan)
			planID, planErr := h.UpsertPendingStructurePlan(id, string(adjJSON), string(profJSON), pScore, structurePlan.Rationale)
			if planErr != nil {
				log.Printf("[TechBid] Persist structure planning failed: %v", planErr)
				break
			}
			_, _ = dbx.Exec(`UPDATE tech_bid_projects SET personalization_score = ?, structure_profile_json = ?, updated_at = ? WHERE id = ?`, pScore, string(profJSON), time.Now(), id)
			up("running", "waiting_for_approval", "弹性结构计划已生成，等待人工审核", map[string]interface{}{
				"personalization_score": pScore,
				"profile_type":          structurePlan.Profile.Type,
			})

			approved := false
			rejected := false
			ticker := time.NewTicker(2 * time.Second)
			timeoutChan := time.After(30 * time.Minute)
			for {
				select {
				case <-ctx.Done():
					ticker.Stop()
					dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "人工审批结构计划超时", id)
					up("running", "failed", "人工审批结构计划超时", nil)
					fail("人工审批结构计划超时")
					return
				case <-timeoutChan:
					ticker.Stop()
					dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "人工审批结构计划超时", id)
					up("running", "failed", "人工审批结构计划超时", nil)
					fail("人工审批结构计划超时")
					return
				case <-ticker.C:
					var status string
					var rejReason *string
					_ = dbx.Get(&status, "SELECT status FROM tech_bid_structure_plan WHERE id = ?", planID)
					if status == "approved" {
						approved = true
						approvedPlanJSON = string(adjJSON)
					}
					if status == "rejected" {
						rejected = true
						_ = dbx.Get(&rejReason, "SELECT reject_reason FROM tech_bid_structure_plan WHERE id = ?", planID)
						if rejReason != nil {
							dbx.Exec("UPDATE tech_bid_projects SET last_structure_reject_reason = ?, last_error_message = ? WHERE id = ?", *rejReason, "用户拒绝了结构计划: "+*rejReason, id)
						}
					}
				}
				if approved || rejected {
					ticker.Stop()
					break
				}
			}
			if approved {
				break
			}
			if rejected {
				up("waiting_for_approval", "running", "正在根据反馈重新生成结构计划...", nil)
				continue
			}
		}

		// Generate outline using skeleton path
		genRowID, _ := store.StartAgentRun(runID, id, "outline_planner_agent", "outline_planning", "GenerateOutlineFromFacts")
		subCtxGen, cancelGen := context.WithTimeout(ctx, 300*time.Second)
		draftOutline, genErr = d.GenerateOutlineFromFacts(subCtxGen, id, facts, p.ProfileData, p.RouteName, p.ProjectTypeStr, p.ProfessionStr, mappings, approvedPlanJSON)
		cancelGen()
		if genErr != nil {
			em := genErr.Error()
			_ = store.CompleteAgentRun(genRowID, "failed", "", &em)
			log.Printf("[TechBid] Draft generation (skeleton mode) failed: %v", genErr)
			dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录生成失败: "+em, id)
			up("running", "failed", "目录生成失败: "+em, nil)
			fail("目录生成失败: " + em)
			return
		}
		_ = store.CompleteAgentRun(genRowID, "done", "目录 JSON 已生成 (骨架模式)", nil)
	} else {
		// Direct generation mode (G1+G2 of the simplification plan)
		// No skeleton dependency; LLM generates outline directly from tender content + facts + profile + route plan
		log.Printf("[Step4Coordinator] Running in direct mode (LLM-direct outline generation)")
		_ = store.UpdateRunStatus(runID, "running", "outline_generating_direct", nil, nil)

		genRowID, _ := store.StartAgentRun(runID, id, "outline_planner_agent", "outline_generating_direct", "tender + facts + profile + route")
		subCtxGen, cancelGen := context.WithTimeout(ctx, 300*time.Second)
		draftOutline, genErr = d.GenerateOutlineDirectly(
			subCtxGen, id, tenderContent, facts, requirementRegister,
			p.ProfileData, p.RouteName, p.ProjectTypeStr, p.ProfessionStr,
		)
		cancelGen()
		if genErr != nil {
			em := genErr.Error()
			_ = store.CompleteAgentRun(genRowID, "failed", "", &em)
			log.Printf("[TechBid] Direct outline generation failed: %v", genErr)
			dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录生成失败: "+em, id)
			up("running", "failed", "目录生成失败: "+em, nil)
			fail("目录生成失败: " + em)
			return
		}
		draftOutline = d.EnsureProjectOverviewSection(draftOutline, facts)
		_ = store.CompleteAgentRun(genRowID, "done", "目录 JSON 已生成 (直生模式)", nil)

		// P1: 构建事实到目录的映射（用于后置审计）
		// 即使在 direct 模式下，也需要 mapping 数据来支持语义覆盖验证
		sk := d.LoadIndustrySkeleton(p.ProjectTypeStr, p.ProfessionStr)
		wrapAgent(store, runID, id, "outline_planner_agent", "outline_mapping", "facts + outline", func() (string, error) {
			if deterministicFastPath {
				mappings = nil
				return "确定性目录已自带 requirement_ids，跳过慢速模型落点映射", nil
			}
			var mapErr error
			mappings, mapErr = d.BuildFactToOutlineMappings(ctx, id, facts, sk)
			if mapErr != nil {
				log.Printf("[TechBid] Mapping build after direct generation failed: %v", mapErr)
			} else {
				mappings = d.AlignFactMappingsToOutline(draftOutline, facts, mappings)
				_ = PersistStep4FactMappings(dbx, id, mappings)
				_ = store.SnapshotFactMappingsFromLegacy(runID, id)
			}
			return fmt.Sprintf("映射 %d 条", len(mappings)), nil
		})
	}

	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'outline_ready' WHERE id = ?", id)

	// === 清理目录中重复的章节编号 ===
	draftOutline = NormalizeOutlineNames(draftOutline)
	log.Printf("[Step4] 已清理目录中的重复章节编号")

	// === 评分表必出项硬校验 + 循环补章（最多3轮）===
	mandatoryChapters := ExtractMandatoryChapters(facts)
	if len(mandatoryChapters) > 0 {
		missing, ok := ValidateMandatoryChapters(draftOutline, mandatoryChapters)
		if !ok {
			log.Printf("[Step4] 必出项缺失: %v，启动循环补章", missing)
			draftOutline, _ = PatchMissingMandatoryChaptersWithLoop(draftOutline, mandatoryChapters, 3)
			log.Printf("[Step4] 循环补章后目录共 %d 个一级章", len(draftOutline))
		} else {
			log.Printf("[Step4] 必出项校验通过，无需补章")
		}
	}
	draftOutline = d.EnforceRequirementIDsFromMappings(draftOutline, mappings)
	draftOutline = d.BackfillRequirementIDs(draftOutline, facts)

	structCov := d.ValidateOutlineSemanticCoverage(draftOutline, facts, mappings)
	fullRes := d.ValidateFullRequirementResponse(draftOutline, requirementRegister, p.ProfessionStr, p.ProjectTypeStr)
	draftJSON, _ := json.Marshal(draftOutline)
	_, _ = PersistCoverageCheck(dbx, id, 1, structCov)
	_, _ = PersistFullResponseCheck(dbx, id, 1, fullRes)
	_ = store.SnapshotCoverageAndFullResponse(runID, id, 1, structCov, fullRes)

	_ = store.UpdateRunStatus(runID, "running", "coverage_auditing", nil, nil)
	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'auditing_outline' WHERE id = ?", id)
	factsJSON, _ := json.Marshal(facts)

	covRowID, _ := store.StartAgentRun(runID, id, "coverage_auditor_agent", "coverage_auditing", "AuditOutlineCoverage")
	var audit *CoverageAuditResult
	if deterministicFastPath {
		audit = BuildStructuralCoverageAudit(structCov, fullRes)
		_ = store.CompleteAgentRun(covRowID, "done", "结构化 coverage 审计完成（跳过慢速模型审计）", nil)
	} else {
		subCtxAudit, cancelAudit := context.WithTimeout(ctx, 300*time.Second)
		var auditErr error
		audit, auditErr = d.AuditOutlineCoverage(subCtxAudit, id, facts, string(draftJSON))
		cancelAudit()
		if auditErr != nil {
			em := auditErr.Error()
			_ = store.CompleteAgentRun(covRowID, "failed", "", &em)
			log.Printf("[TechBid] Initial audit failed: %v", auditErr)
			dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录审计失败: "+em, id)
			up("running", "failed", "目录审计失败: "+em, nil)
			fail("目录审计失败: " + em)
			return
		}
		_ = store.CompleteAgentRun(covRowID, "done", "coverage 审计完成", nil)
	}

	finalDecision, reason := d.DecideNextFlowWithFullResponse(audit, structCov, fullRes)
	if gateDecision == "BLOCK" {
		finalDecision = "BLOCK"
		reason = "【硬审计阻断】" + gateReason
	} else if gateDecision == "REVISE" && finalDecision == "PASS" {
		finalDecision = "REVISE"
		reason = "【数字化建议修订】" + gateReason
	}

	mergedScore := audit.CoverageScore
	if structCov != nil && structCov.CoverageRate < mergedScore {
		mergedScore = structCov.CoverageRate
	}
	if fullRes != nil && fullRes.RequirementTotal > 0 && fullRes.FullResponseRate < mergedScore {
		mergedScore = fullRes.FullResponseRate
	}

	var draftBeforeRevise []map[string]interface{}
	var frozenSkeleton []SkeletonEntry // 骨架快照（用于防污染）

	if finalDecision == "REVISE" {
		// === 防污染：提取骨架快照 ===
		if b, mErr := json.Marshal(draftOutline); mErr == nil {
			_ = json.Unmarshal(b, &draftBeforeRevise)
		}
		frozenSkeleton = ExtractSkeleton(draftOutline)
		log.Printf("[Step4Coordinator] 防污染：已冻结骨架，共 %d 个一级章", len(frozenSkeleton))

		dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'refining_outline' WHERE id = ?", id)
		subCtxOpt, cancelOpt := context.WithTimeout(ctx, 300*time.Second)
		finalOutline, optErr := d.OptimizeOutlineByCoverage(subCtxOpt, id, string(draftJSON), audit, facts, mappings, fullRes)
		cancelOpt()
		if optErr == nil {
			// === 防污染：强制恢复骨架 ===
			if len(frozenSkeleton) > 0 {
				finalOutline = EnforceSkeleton(finalOutline, frozenSkeleton)
				log.Printf("[Step4Coordinator] 防污染：已强制恢复骨架（%d 个章）", len(frozenSkeleton))
			}

			// === 防污染：骨架偏移检测 ===
			if len(frozenSkeleton) > 0 {
				drift := ComputeChapterDrift(draftOutline, finalOutline)
				log.Printf("[Step4Coordinator] 防污染：骨架偏移 drift=%.2f", drift)
				if drift > 0.3 {
					log.Printf("[Step4Coordinator] ⚠️ 防污染：骨架偏移过大 (drift=%.2f > 0.3)，回滚到原始目录", drift)
					finalOutline = draftOutline
					log.Printf("[Step4Coordinator] 防污染：已回滚到优化前版本")
				}
			}

			draftOutline = finalOutline
			draftJSON, _ = json.Marshal(draftOutline)
			structCov = d.ValidateOutlineSemanticCoverage(draftOutline, facts, mappings)
			fullRes = d.ValidateFullRequirementResponse(draftOutline, requirementRegister, p.ProfessionStr, p.ProjectTypeStr)
			if len(draftBeforeRevise) > 0 && (structCov == nil || structCov.CoverageRate < 90 || (fullRes != nil && fullRes.RequirementTotal > 0 && fullRes.FullResponseRate < 90)) {
				log.Printf("[Step4Coordinator] 优化后覆盖率退化，回滚到优化前目录")
				draftOutline = draftBeforeRevise
				draftOutline = d.BackfillRequirementIDs(draftOutline, facts)
				draftJSON, _ = json.Marshal(draftOutline)
				structCov = d.ValidateOutlineSemanticCoverage(draftOutline, facts, mappings)
				fullRes = d.ValidateFullRequirementResponse(draftOutline, requirementRegister, p.ProfessionStr, p.ProjectTypeStr)
			}
			_, _ = PersistCoverageCheck(dbx, id, 1, structCov)
			_, _ = PersistFullResponseCheck(dbx, id, 1, fullRes)
			_ = store.SnapshotCoverageAndFullResponse(runID, id, 1, structCov, fullRes)
			mergedScore = audit.CoverageScore
			if structCov != nil && structCov.CoverageRate < mergedScore {
				mergedScore = structCov.CoverageRate
			}
			if fullRes != nil && fullRes.RequirementTotal > 0 && fullRes.FullResponseRate < mergedScore {
				mergedScore = fullRes.FullResponseRate
			}
			finalDecision, reason = d.DecideNextFlowWithFullResponse(audit, structCov, fullRes)
		}
		dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'refine_ready' WHERE id = ?", id)
	} else {
		dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'audit_ready' WHERE id = ?", id)
	}

	riskLevel := "LOW"
	if finalDecision != "PASS" {
		riskLevel = "MEDIUM"
	}
	if finalDecision == "BLOCK" {
		riskLevel = "HIGH"
	}

	db.EnsureTechBidOutlineAuditsSchema(dbx)
	auditID := uuid.New().String()
	_, _ = dbx.Exec(`INSERT INTO tech_bid_outline_audits 
		(id, project_id, outline_version, outline_snapshot_json, facts_snapshot_json, coverage_score, audit_summary, final_decision, risk_level, can_proceed, audit_version) 
		VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
		auditID, id, string(draftJSON), string(factsJSON), mergedScore, audit.AuditSummary, finalDecision, riskLevel, map[bool]int{true: 1, false: 0}[finalDecision == "PASS"], "v2")

	gateRate := 0.0
	gateRes := "PASS"
	gateReasonText := ""
	if fullRes != nil {
		gateRate = fullRes.FullResponseRate
		gateRes = fullRes.Result
		gateReasonText = fullRes.Summary
	}

	dbx.Exec(`UPDATE tech_bid_projects SET coverage_score = ?, final_decision = ?, risk_level = ?, can_enter_content_generation = ?, full_response_rate = ?, step4_gate_result = ?, step4_gate_reason = ?, last_error_message = ? WHERE id = ?`,
		mergedScore, finalDecision, riskLevel, map[bool]int{true: 1, false: 0}[finalDecision == "PASS"], gateRate, gateRes, gateReasonText, reason, id)

	outlineVerForChapters := 1
	if len(draftBeforeRevise) > 0 {
		if _, snapErr := store.SnapshotOutlineVersionFromDraft(runID, id, 1, draftBeforeRevise); snapErr != nil {
			log.Printf("[Step4] snapshot outline v1: %v", snapErr)
		}
		if _, snapErr := store.SnapshotOutlineVersionFromDraft(runID, id, 2, draftOutline); snapErr != nil {
			log.Printf("[Step4] snapshot outline v2: %v", snapErr)
		}
		_, _ = dbx.Exec(`UPDATE step4_outline_versions SET status = 'archived', rationale = '优化前候选' WHERE run_id = ? AND version_no = 1`, runID)
		_, _ = dbx.Exec(`UPDATE step4_outline_versions SET status = 'recommended', rationale = '覆盖率优化后' WHERE run_id = ? AND version_no = 2`, runID)
		outlineVerForChapters = 2
	} else {
		if _, snapErr := store.SnapshotOutlineVersionFromDraft(runID, id, 1, draftOutline); snapErr != nil {
			log.Printf("[Step4] snapshot outline version: %v", snapErr)
		}
		_, _ = dbx.Exec(`UPDATE step4_outline_versions SET status = 'recommended', rationale = '主版本' WHERE run_id = ? AND version_no = 1`, runID)
	}

	tx := dbx.MustBegin()
	for i, ch := range draftOutline {
		chapterID := uuid.New().String()
		chapterName := ""
		if n, ok := ch["name"].(string); ok {
			chapterName = n
		}
		if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, 'chapter', 'completed', ?)`, chapterID, id, chapterName, i+1, outlineVerForChapters); dbErr != nil {
			log.Printf("[TechBid] Save chapter plan failed: %v", dbErr)
			_ = tx.Rollback()
			dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录结构保存失败: "+dbErr.Error(), id)
			up("running", "failed", "目录结构保存失败: "+dbErr.Error(), map[string]interface{}{"coverage": mergedScore})
			fail("目录结构保存失败: " + dbErr.Error())
			return
		}
		units, _ := ch["units"].([]interface{})
		for j, u := range units {
			uMap, _ := u.(map[string]interface{})
			unitID := uuid.New().String()
			unitName, _ := uMap["name"].(string)
			if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version) VALUES (?, ?, ?, ?, ?, 'unit', 'completed', ?)`, unitID, id, chapterID, unitName, j+1, outlineVerForChapters); dbErr != nil {
				_ = tx.Rollback()
				dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录结构保存失败: "+dbErr.Error(), id)
				up("running", "failed", "目录结构保存失败: "+dbErr.Error(), map[string]interface{}{"coverage": mergedScore})
				fail("目录结构保存失败: " + dbErr.Error())
				return
			}
			subs, _ := uMap["subsections"].([]interface{})
			for k, s := range subs {
				subName := ""
				reqIDs := ""
				if sMap, ok := s.(map[string]interface{}); ok {
					subName, _ = sMap["name"].(string)
					reqIDs = marshalRequirementIDsJSON(sMap["requirement_ids"])
				} else {
					subName, _ = s.(string)
				}
				if _, dbErr := tx.Exec(`INSERT INTO tech_bid_chapter_plans (id, project_id, parent_id, chapter_name, chapter_order, node_level, generation_status, outline_version, requirement_ids_json) VALUES (?, ?, ?, ?, ?, 'subsection', 'not_started', ?, ?)`, uuid.New().String(), id, unitID, subName, k+1, outlineVerForChapters, reqIDs); dbErr != nil {
					_ = tx.Rollback()
					dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录结构保存失败: "+dbErr.Error(), id)
					up("running", "failed", "目录结构保存失败: "+dbErr.Error(), map[string]interface{}{"coverage": mergedScore})
					fail("目录结构保存失败: " + dbErr.Error())
					return
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[TechBid] Final outline commit failed: %v", err)
		dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'failed', last_error_message = ? WHERE id = ?", "目录结构保存失败: "+err.Error(), id)
		up("running", "failed", "目录结构保存失败: "+err.Error(), map[string]interface{}{"coverage": mergedScore})
		fail("目录结构保存失败: " + err.Error())
		return
	}

	fp := d.OutlineFingerprint(draftOutline)
	outlineTitles := CollectOutlineSubsectionTitles(draftOutline)
	titlesJSON, _ := json.Marshal(outlineTitles)
	var dup int
	hint := ""
	if cid != "" {
		if err := dbx.Get(&dup, `SELECT COUNT(*) FROM tech_bid_projects WHERE company_id = ? AND outline_fingerprint = ? AND id != ?`, cid, fp, id); err == nil && dup > 0 {
			hint = "与本企业其他项目目录指纹重复，请复核雷同风险"
		}
	}
	jaccardHint := ""
	if cid != "" {
		jaccardHint = BestHistoryJaccardHint(dbx, cid, id, outlineTitles)
	}
	mergedParts := []string{}
	if hint != "" {
		mergedParts = append(mergedParts, hint)
	}
	if jaccardHint != "" {
		mergedParts = append(mergedParts, jaccardHint)
	}
	merged := strings.Join(mergedParts, " ")
	var hintArg interface{}
	if merged != "" {
		hintArg = merged
	} else {
		hintArg = nil
	}
	_, _ = dbx.Exec(`UPDATE tech_bid_projects SET outline_fingerprint = ?, history_similarity_hint = ?, outline_titles_json = ? WHERE id = ?`, fp, hintArg, string(titlesJSON), id)

	dbx.Exec("UPDATE tech_bid_projects SET step4_status = 'audit_ready' WHERE id = ?", id)
	up("running", "success", "目录生成与审计完成", map[string]interface{}{"coverage": mergedScore})

	gatePtr := finalDecision
	_ = store.FinishRun(runID, "completed", &gatePtr, nil)
	_ = store.UpdateRunStatus(runID, "completed", "coordinator", &gatePtr, nil)
}
