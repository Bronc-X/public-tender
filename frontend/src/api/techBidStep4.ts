import axios from 'axios';

export interface Step4FactMapping {
    id: string;
    fact_id: string;
    fact_type: string;
    fact_name: string;
    target_level: string;
    target_path: string[];
    required: boolean;
    priority: string;
    mapping_reason?: string;
    mapping_source?: string;
    target_node_id?: string;
    created_at?: string;
    // Traceability fields
    source_chapter?: string;
    page_number?: number;
    line_number?: number;
    source_location?: string;
}

export interface Step4Coverage {
    id: string;
    outline_version: number;
    fact_total: number;
    fact_mapped: number;
    coverage_rate: number;
    missing_fact_ids: string[];
    weak_fact_ids: string[];
    duplicate_node_hints: string[];
    result: 'PASS' | 'REVISE' | 'BLOCK' | string;
    summary: string;
    created_at?: string;
    /** step4 = Coordinator 快照；legacy = 旧表 */
    source?: string;
}

/** 招标要求总表单行（Step4 真相层） */
export interface Step4RequirementRow {
    id: string;
    requirement_id: string;
    requirement_type: string;
    source_text: string;
    source_location: string;
    priority: string;
    must_be_explicit: number;
    expected_response_level: string;
    domain: string;
    response_tier: string;
    summary: string;
    created_at?: string;
}

/** 完全响应率校验（超越仅挂 requirement_ids） */
export interface Step4FullResponse {
    id: string;
    outline_version: number;
    requirement_total: number;
    requirement_mapped: number;
    requirement_fully_responded: number;
    requirement_weakly_responded: number;
    requirement_only_tagged: number;
    full_response_rate: number;
    weak_response_rate: number;
    response_quality_score: number;
    missing_requirement_ids: string[];
    weak_requirement_ids: string[];
    only_tagged_requirement_ids: string[];
    shell_title_hints: string[];
    high_priority_missing_ids?: string[];
    mandatory_missing_ids?: string[];
    mandatory_insufficient_ids?: string[];
    hard_rule_warnings?: string[];
    result: 'PASS' | 'REVISE' | 'BLOCK' | string;
    summary: string;
    created_at?: string;
}

export interface Step4Conflict {
    conflict_id: string;
    conflict_type: string;
    field_name: string;
    source_a: string;
    source_b: string;
    description: string;
    severity: 'high' | 'medium' | 'low';
}

export interface Step4ConflictAudit {
    id: string;
    project_id: string;
    has_block: boolean;
    conflicts: Step4Conflict[];
    summary: string;
    created_at?: string;
}

export async function fetchStep4Mappings(projectId: string, companyId: string): Promise<Step4FactMapping[]> {
    const res = await axios.get<{ mappings: Step4FactMapping[] }>(`/api/tech-bid/projects/${projectId}/outline/mappings`, {
        headers: { 'X-Company-Id': companyId },
    });
    return Array.isArray(res.data?.mappings) ? res.data.mappings : [];
}

export async function fetchStep4Coverage(projectId: string, companyId: string): Promise<Step4Coverage | null> {
    const res = await axios.get<{ coverage: Step4Coverage | null }>(`/api/tech-bid/projects/${projectId}/outline/coverage`, {
        headers: { 'X-Company-Id': companyId },
    });
    return res.data?.coverage ?? null;
}

export async function fetchStep4Requirements(projectId: string, companyId: string): Promise<Step4RequirementRow[]> {
    const res = await axios.get<{ requirements: Step4RequirementRow[] }>(`/api/tech-bid/projects/${projectId}/outline/requirements`, {
        headers: { 'X-Company-Id': companyId },
    });
    return Array.isArray(res.data?.requirements) ? res.data.requirements : [];
}

export async function fetchStep4FullResponse(projectId: string, companyId: string): Promise<Step4FullResponse | null> {
    const res = await axios.get<{ full_response: Step4FullResponse | null }>(`/api/tech-bid/projects/${projectId}/outline/full-response`, {
        headers: { 'X-Company-Id': companyId },
    });
    return res.data?.full_response ?? null;
}

export async function fetchStep4ConflictAudit(projectId: string, companyId: string): Promise<Step4ConflictAudit | null> {
    const res = await axios.get<{ audit: Step4ConflictAudit | null }>(`/api/tech-bid/projects/${projectId}/outline/conflict-audit`, {
        headers: { 'X-Company-Id': companyId },
    });
    return res.data?.audit ?? null;
}

export interface Step4FactCandidate {
    id: string;
    fact_id: string;
    fact_type: string;
    fact_name: string;
    source_library?: string | null;
    source_location?: string | null;
    confidence_score: number;
    snippet?: string | null;
    created_at?: string | null;
}

export async function fetchStep4FactCandidates(projectId: string, companyId: string): Promise<Step4FactCandidate[]> {
    const res = await axios.get<{ success: boolean; data: { candidates: Step4FactCandidate[] } }>(`/api/tech-bid/projects/${projectId}/outline/fact-candidates`, {
        headers: { 'X-Company-Id': companyId },
    });
    return Array.isArray(res.data?.data?.candidates) ? res.data.data.candidates : [];
}

export interface Step4RunRow {
    id: number;
    project_id: string;
    outline_version?: number | null;
    trigger_source: string;
    operator_id?: string | null;
    status: string;
    current_stage?: string | null;
    gate_result?: string | null;
    started_at?: string | null;
    finished_at?: string | null;
    error_message?: string | null;
    retry_count?: number;
    created_at?: string;
    updated_at?: string;
}

export async function fetchOutlineRunHistory(projectId: string, companyId: string): Promise<Step4RunRow[]> {
    const res = await axios.get<{ success: boolean; data: { runs: Step4RunRow[] } }>(`/api/tech-bid/projects/${projectId}/outline/run-history`, {
        headers: { 'X-Company-Id': companyId },
    });
    return Array.isArray(res.data?.data?.runs) ? res.data.data.runs : [];
}

export interface Step4ApprovalLogRow {
    id: string;
    run_id: number;
    stage: string;
    action: string;
    operator_id?: string | null;
    reason?: string | null;
    snapshot_version?: string | null;
    created_at?: string | null;
}

export async function fetchStep4ApprovalLogs(projectId: string, companyId: string): Promise<Step4ApprovalLogRow[]> {
    const res = await axios.get<{ success: boolean; data: { logs: Step4ApprovalLogRow[] } }>(`/api/tech-bid/projects/${projectId}/outline/approval-logs`, {
        headers: { 'X-Company-Id': companyId },
    });
    return Array.isArray(res.data?.data?.logs) ? res.data.data.logs : [];
}

export async function postStep4GateOverride(
    projectId: string,
    companyId: string,
    body: { reason: string; operator_id?: string },
): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/outline/step4-gate/override`, body, {
        headers: { 'X-Company-Id': companyId },
    });
}

export interface StructureAdjustment {
    action: 'keep' | 'move' | 'split' | 'merge' | 'promote' | 'insert';
    target_name: string;
    new_index?: number;
    reason: string;
    priority: number;
}

export interface StructurePlan {
    id: string;
    adjustments: StructureAdjustment[];
    profile: any;
    rationale: string;
    status: 'pending' | 'approved' | 'rejected';
    created_at: string;
}

export async function fetchStructurePlan(projectId: string, companyId: string): Promise<StructurePlan | null> {
    const res = await axios.get<{ plan: StructurePlan | null }>(`/api/tech-bid/projects/${projectId}/structure-plan`, {
        headers: { 'X-Company-Id': companyId },
    });
    return res.data?.plan ?? null;
}

export async function approveStructurePlan(projectId: string, companyId: string): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/structure-plan/approve`, {}, {
        headers: { 'X-Company-Id': companyId },
    });
}

export async function rejectStructurePlan(projectId: string, companyId: string, reason: string): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/structure-plan/reject`, { reason }, {
        headers: { 'X-Company-Id': companyId },
    });
}

/** GET /outline/run-status — Coordinator 编排进度 */
export interface Step4RunStatusPayload {
    run_id: number | null;
    project_id?: string;
    status: string;
    gate_result?: string | null;
    current_stage?: string | null;
    current_agent?: string | null;
    progress?: number;
    started_at?: string | null;
    last_error?: string | null;
    stages?: Array<{
        stage: string;
        agent: string;
        status: string;
        duration_ms: number;
    }>;
    message?: string;
}

export async function fetchOutlineRunStatus(projectId: string, companyId: string): Promise<Step4RunStatusPayload | null> {
    const res = await axios.get<{ success: boolean; data: Step4RunStatusPayload }>(
        `/api/tech-bid/projects/${projectId}/outline/run-status`,
        { headers: { 'X-Company-Id': companyId } },
    );
    return res.data?.data ?? null;
}

/** POST /outline/regenerate — 重新生成目录（不需要 routeId 参数） */
export async function regenerateOutline(projectId: string, companyId: string): Promise<{ run_id: number; route: string }> {
    const res = await axios.post<{ success: boolean; status: string; data: { run_id: number; route: string } }>(
        `/api/tech-bid/projects/${projectId}/outline/regenerate`,
        {},
        { headers: { 'X-Company-Id': companyId } },
    );
    return res.data.data;
}

export interface Step4AgentRunRow {
    id: string;
    run_id: number;
    project_id: string;
    agent_name: string;
    stage: string;
    status: string;
    input_summary?: string | null;
    output_summary?: string | null;
    error_message?: string | null;
    started_at?: string | null;
    finished_at?: string | null;
    duration_ms?: number | null;
}

export async function fetchOutlineAgentRuns(projectId: string, companyId: string): Promise<Step4AgentRunRow[]> {
    const res = await axios.get<{ success: boolean; data: { runs: Step4AgentRunRow[] } }>(
        `/api/tech-bid/projects/${projectId}/outline/agent-runs`,
        { headers: { 'X-Company-Id': companyId } },
    );
    return Array.isArray(res.data?.data?.runs) ? res.data.data.runs : [];
}

/** Phase 2：同一 run 下的候选目录版本 */
export interface OutlineVersionRow {
    id: string;
    version_no: number;
    version_source: string;
    status: string;
    rationale: string;
    created_by: string;
    created_at: string;
    run_id: number;
}

export interface OutlineVersionDetailResponse {
    outline: Array<Record<string, unknown>>;
    nodes: Array<Record<string, unknown>>;
}

export async function fetchOutlineVersions(projectId: string, companyId: string): Promise<OutlineVersionRow[]> {
    const res = await axios.get<{ success: boolean; data: { versions: OutlineVersionRow[] } }>(
        `/api/tech-bid/projects/${projectId}/outline/versions`,
        { headers: { 'X-Company-Id': companyId } },
    );
    return Array.isArray(res.data?.data?.versions) ? res.data.data.versions : [];
}

export async function fetchOutlineVersionDetail(
    projectId: string,
    companyId: string,
    versionId: string,
): Promise<OutlineVersionDetailResponse | null> {
    const res = await axios.get<{ success: boolean; data: OutlineVersionDetailResponse }>(
        `/api/tech-bid/projects/${projectId}/outline/versions/${versionId}`,
        { headers: { 'X-Company-Id': companyId } },
    );
    return res.data?.data ?? null;
}

export async function selectOutlineVersion(
    projectId: string,
    companyId: string,
    versionId: string,
    operatorId?: string,
): Promise<void> {
    await axios.post(
        `/api/tech-bid/projects/${projectId}/outline/versions/${versionId}/select`,
        { operator_id: operatorId ?? '' },
        { headers: { 'X-Company-Id': companyId } },
    );
}

// Profile manual editing APIs

export async function patchProfileField(
    projectId: string,
    body: { field_path: string; new_value: string; source_text?: string; source_location?: string; operator_name?: string },
): Promise<any> {
    const res = await axios.patch(`/api/tech-bid/projects/${projectId}/profile/fields`, body);
    return res.data;
}

export async function confirmProfile(
    projectId: string,
    operatorName?: string,
): Promise<any> {
    const res = await axios.post(`/api/tech-bid/projects/${projectId}/profile/confirm`, { operator_name: operatorName ?? '' });
    return res.data;
}

export async function fetchProfileEditHistory(
    projectId: string,
): Promise<any> {
    const res = await axios.get(`/api/tech-bid/projects/${projectId}/profile/edit-history`);
    return res.data;
}

// ─── Extraction Snapshots (Evidence Traceability + Run Replay) ───

export interface ExtractionSnapshot {
    id: string;
    project_id: string;
    profile_id: string;
    stage: string;
    chunk_index: number;
    payload: unknown;
    run_id: string | null;
    file_id: string | null;
    chunk_type: string | null;
    created_at: string;
}

export interface ExtractionRunRef {
    run_id: string;
    created_at: string;
}

export async function fetchProfileExtractionSnapshots(
    projectId: string,
    runId?: string,
    stage?: string,
): Promise<{ snapshots: ExtractionSnapshot[]; runIds: ExtractionRunRef[] }> {
    const params: Record<string, string> = {};
    if (runId) params.run_id = runId;
    if (stage) params.stage = stage;
    const res = await axios.get(`/api/tech-bid/projects/${projectId}/profile/extraction-snapshots`, { params });
    return {
        snapshots: Array.isArray(res.data?.data) ? res.data.data : [],
        runIds: Array.isArray(res.data?.run_ids) ? res.data.run_ids : [],
    };
}

// ============================================================================
// Skeleton Candidate Selection APIs (骨架候选选择 API)
// ============================================================================

/** 骨架评分明细 */
export interface SkeletonScoreBreakdown {
    keyword_score: number;        // 关键词匹配 30%
    structure_score: number;       // 结构相似度 30%
    special_chapter_score: number; // 特殊章节匹配 25%
    history_score: number;         // 历史相似度 15%
}

/** 骨架摘要信息 */
export interface SkeletonSummary {
    industry_name: string;
    chapter_count: number;
    chapters: string[];
    keywords: string[];
}

/** 骨架候选 */
export interface SkeletonCandidate {
    skeleton_id: string;
    industry_name: string;
    match_score: number;           // 总分 0-100
    score_breakdown: SkeletonScoreBreakdown;
    match_reasons: string[];
    confidence: 'high' | 'medium' | 'low';
    recommended: boolean;
    skeleton_data?: SkeletonSummary;
}

/** 获取骨架候选列表 */
export async function fetchSkeletonCandidates(projectId: string, companyId: string): Promise<{
    candidates: SkeletonCandidate[];
    recommended: SkeletonCandidate | null;
    source: string;
}> {
    const res = await axios.get(`/api/tech-bid/projects/${projectId}/skeleton/candidates`, {
        headers: { 'X-Company-Id': companyId },
    });
    return {
        candidates: res.data?.candidates || [],
        recommended: res.data?.recommended || null,
        source: res.data?.source || 'database',
    };
}

/** 基于事实的骨架候选（facts 提取后） */
export async function fetchSkeletonCandidatesFromFacts(projectId: string, companyId: string): Promise<{
    candidates: SkeletonCandidate[];
    run_id: number;
    keywords: string[];
}> {
    const res = await axios.get(`/api/tech-bid/projects/${projectId}/skeleton/candidates-from-facts`, {
        headers: { 'X-Company-Id': companyId },
    });
    return {
        candidates: res.data?.candidates || [],
        run_id: res.data?.run_id || 0,
        keywords: res.data?.keywords || [],
    };
}

/** 确认骨架选择 */
export async function confirmSkeleton(
    projectId: string,
    companyId: string,
    skeletonId: string,
    operatorName?: string,
): Promise<{ skeleton_id: string; skeleton_name: string }> {
    const res = await axios.post(
        `/api/tech-bid/projects/${projectId}/skeleton/confirm`,
        { skeleton_id: skeletonId, operator_name: operatorName || '' },
        { headers: { 'X-Company-Id': companyId } },
    );
    return {
        skeleton_id: res.data?.skeleton_id,
        skeleton_name: res.data?.skeleton_name,
    };
}

/** 获取已确认的骨架 */
export async function fetchConfirmedSkeleton(projectId: string, companyId: string): Promise<{
    selected: boolean;
    selection?: {
        id: string;
        skeleton_id: string;
        skeleton_name: string;
        operator_name?: string;
        created_at: string;
    };
}> {
    const res = await axios.get(`/api/tech-bid/projects/${projectId}/skeleton/confirmed`, {
        headers: { 'X-Company-Id': companyId },
    });
    return {
        selected: res.data?.selected || false,
        selection: res.data?.selection,
    };
}

// ============================================================================
// Two-Step Outline Generation APIs (两步走目录生成 API)
// ============================================================================

/** 发起 Phase A：生成一级章草案 */
export async function generateOutlineChapters(projectId: string, companyId: string): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/outline/chapters/generate`, {}, {
        headers: { 'X-Company-Id': companyId },
    });
}

/** 确认及保存一级章骨架 */
export async function confirmOutlineChapters(projectId: string, companyId: string, chapters: string[]): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/outline/chapters/confirm`, { chapters }, {
        headers: { 'X-Company-Id': companyId },
    });
}

/** 发起 Phase B：结构展开 */
export async function expandOutlineStructure(projectId: string, companyId: string): Promise<void> {
    await axios.post(`/api/tech-bid/projects/${projectId}/outline/expand`, {}, {
        headers: { 'X-Company-Id': companyId },
    });
}
