/**
 * 项目画像相关统一类型定义
 * 解决 CTO P0-3（前后端契约对齐）+ P1-1（统一类型）
 */

// ─── 证据字段类型 ───────────────────────────────────────

export interface EvidenceField {
    value?: string;
    missing?: boolean;
    confidence?: number;
    source_text?: string;
    source_location?: string;
    notes?: string;
}

export interface EvidenceListItem {
    value?: string;
    name?: string;
    source_text?: string;
    source_location?: string;
}

// ─── 画像主结构 ───────────────────────────────────────

export interface ProjectBaseInfo {
    project_name?: EvidenceField;
    project_type?: EvidenceField;
    project_location?: EvidenceField;
    owner_unit?: EvidenceField;
    tender_unit?: EvidenceField;
    location?: EvidenceField;
    category_and_scope?: EvidenceField;
    tender_scope?: EvidenceField;
    project_scope?: EvidenceField;
    contract_scope_summary?: EvidenceField;
    total_duration?: EvidenceField;
    duration_requirements?: EvidenceField;
    duration_requirement?: EvidenceField;
    quality_target?: EvidenceField;
    quality_standard?: EvidenceField;
    safety_target?: EvidenceField;
    construction_scale?: EvidenceField;
}

export interface ConstructionCoreRequirements {
    construction_scope?: EvidenceField;
    material_equipment_rules?: EvidenceField;
    material_requirements?: EvidenceField;
    technical_specifications?: EvidenceField;
    technical_requirements?: EvidenceField;
    site_management?: EvidenceField;
    site_management_requirements?: EvidenceField;
    acceptance_requirements?: EvidenceField;
    acceptance?: EvidenceField;
    special_operations?: EvidenceField;
    special_operation_requirements?: EvidenceField;
    procurement_boundary?: EvidenceField;
    owner_supplied_items?: EvidenceField | EvidenceListItem[];
    owner_supplied_materials?: EvidenceField | EvidenceListItem[];
    contractor_supplied_items?: EvidenceField | EvidenceListItem[];
    contractor_supplied_materials?: EvidenceField | EvidenceListItem[];
    schedule_constraints?: EvidenceField;
    schedule_nodes?: EvidenceField;
    safety_civilization_rules?: EvidenceField;
    quality_acceptance_rules?: EvidenceField;
    key_construction_methods?: EvidenceField;
    difficult_works?: EvidenceField;
    civilized_construction_requirements?: EvidenceField;
    environmental_protection_requirements?: EvidenceField;
    site_condition_constraints?: EvidenceField;
}

export interface BidderRequirements {
    qualification_requirements?: EvidenceField;
    qualification_certificates?: EvidenceField;
    personnel_requirements?: EvidenceField;
    performance_requirements?: EvidenceField;
    certificate_requirements?: EvidenceField;
    bonus_items?: EvidenceField;
}

export interface EvaluationAndPerformanceRules {
    scoring_items?: EvidenceField | EvidenceListItem[];
    bonus_rules?: EvidenceField | EvidenceListItem[];
    deduction_rules?: EvidenceField | EvidenceListItem[];
    disqualification_rules?: EvidenceField | EvidenceListItem[];
    mandatory_response_items?: EvidenceField | EvidenceListItem[];
    method_and_score_weights?: EvidenceField;
    technical_evaluation_dimensions?: EvidenceField;
    payment_method?: EvidenceField;
    settlement_rules?: EvidenceField;
    total_duration?: EvidenceField;
}

export interface DifficultyAndFocus {
    technical_process_difficulty?: EvidenceField;
    site_management_difficulty?: EvidenceField;
    engineering_characteristics?: EvidenceField;
}

export interface ProfileView {
    view_key: string;
    view_label: string;
    completeness?: number;
    fields?: Array<{ label: string; field: EvidenceField }>;
    lists?: Array<{ label: string; items: EvidenceListItem[] }>;
}

export interface ProfileData {
    project_base_info?: ProjectBaseInfo;
    construction_core_requirements?: ConstructionCoreRequirements;
    bidder_requirements?: BidderRequirements;
    evaluation_and_performance_rules?: EvaluationAndPerformanceRules;
    difficulty_and_focus?: DifficultyAndFocus;
    extraction_gaps?: Array<{ name?: string; value?: string }>;
    uncertain_items?: Array<{ name?: string; value?: string }>;
    requires_manual_review?: EvidenceField;
    keyword_audit_hits?: KeywordAuditHit[];
    rule_engine_hits?: RuleEngineHit[];
    views?: ProfileView[];
}

// ─── 审计与规则引擎 ───────────────────────────────────

export interface KeywordAuditHit {
    group: string;
    hit_count: number;
    field_labels: string[];
    severity: 'error' | 'warning' | string;
}

export interface RuleEngineHit {
    rule_name: string;
    category: 'duration' | 'qualification' | 'scoring' | 'procurement' | 'personnel' | string;
    matches: string[];
    field_path: string;
    source_lines?: string[];
    confidence?: number;
}

// ─── 类型守卫 ─────────────────────────────────────────

export const isRecord = (value: unknown): value is Record<string, unknown> =>
    !!value && typeof value === 'object' && !Array.isArray(value);

export const isEvidenceField = (value: unknown): value is EvidenceField =>
    isRecord(value) && Object.prototype.hasOwnProperty.call(value, 'missing');

export const isEvidenceListItem = (value: unknown): value is EvidenceListItem =>
    isRecord(value);

export const isEvidenceList = (value: unknown): value is EvidenceListItem[] =>
    Array.isArray(value) && value.length > 0 && value.every(isEvidenceListItem);

// ─── 字段值解析 ───────────────────────────────────────

export const getFieldText = (value: unknown): string => {
    if (isEvidenceField(value)) {
        return value.missing ? '无' : (value.value || '无');
    }
    if (isEvidenceList(value)) {
        const texts = value
            .map((item: EvidenceListItem) => item?.value || item?.name)
            .filter(Boolean);
        return texts.length > 0 ? texts.join('；') : '无';
    }
    if (Array.isArray(value)) {
        return value.length > 0 ? value.join('；') : '无';
    }
    return (value as string) || '无';
};

/** 从 EvidenceField 或 EvidenceListItem[] 中提取纯文本用于评分计算 */
export const resolveFieldText = (val: unknown): string => {
    if (val == null) return '';
    if (isEvidenceField(val)) return val.missing ? '' : (val.value || '');
    if (Array.isArray(val)) {
        return val.length > 0
            ? (val.map((i: Record<string, unknown>) => (i?.value as string) || (i?.name as string)).filter(Boolean).join('') || '')
            : '';
    }
    return String(val);
};

// ─── 字段路径映射（画像编辑用） ──────────────────────

export const PROFILE_FIELD_PATH_MAP: Record<string, string> = {
    '项目名称': 'project_base_info.project_name',
    '招标单位': 'project_base_info.owner_unit',
    '工程地点': 'project_base_info.location',
    '施工范围': 'project_base_info.category_and_scope',
    '工期要求': 'project_base_info.duration_requirements',
    '质量标准': 'project_base_info.quality_standard',
    '材料设备要求': 'construction_core_requirements.material_equipment_rules',
    '施工技术规范': 'construction_core_requirements.technical_specifications',
    '现场管理要求': 'construction_core_requirements.site_management',
    '验收要求': 'construction_core_requirements.acceptance_requirements',
    '专项作业要求': 'construction_core_requirements.special_operations',
    '采购边界': 'construction_core_requirements.procurement_boundary',
    '工期节点': 'construction_core_requirements.schedule_constraints',
    '资质证书': 'bidder_requirements.qualification_certificates',
    '业绩要求': 'bidder_requirements.performance_requirements',
    '资格要求': 'bidder_requirements.qualification_requirements',
    '评标方法': 'evaluation_and_performance_rules.method_and_score_weights',
    '技术标评分维度': 'evaluation_and_performance_rules.technical_evaluation_dimensions',
    '支付说明': 'evaluation_and_performance_rules.payment_method',
    '结算规则': 'evaluation_and_performance_rules.settlement_rules',
    '总工期': 'evaluation_and_performance_rules.total_duration',
};
