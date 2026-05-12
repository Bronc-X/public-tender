/**
 * 项目画像评分与路线计算逻辑
 * 从 TechBidProjectWorkbench.tsx 中抽离（CTO P1-2）
 */

import { resolveFieldText, type ProfileData } from '../types/projectProfile';

export interface ProfileSignal {
    technicalDifficulty: string;
    siteManagementDifficulty: string;
    engineeringCharacteristics: string;
    greenEnv: string;
    qualityStandard: string;
    schedulePressure: string;
    procurementComplexity: string;
    ownerSuppliedItems: string;
    scoringRules: string;
    qualificationReq: string;
    performanceReq: string;
    personnelReq: string;
    materialEquip: string;
    bonusItems: string;
}

export interface RouteScore {
    route: string;
    score: number;
    maxScore: number;
    color: 'volcano' | 'green' | 'blue' | 'gold';
    desc: string;
}

export interface DifficultyLevel {
    label: string;
    color: string;
}

export interface ProfileScoringResult {
    signal: ProfileSignal;
    filledFieldCount: number;
    profileFieldCount: number;
    profileCoverageRate: number;
    scores: RouteScore[];
    difficultyNormalized: number;
    difficultyLevel: DifficultyLevel;
    matchRate: number;
}

/** 从 profileData 中提取评分信号 */
export function computeProfileSignal(profileData: ProfileData): ProfileSignal {
    const r = resolveFieldText;
    return {
        technicalDifficulty: r(profileData?.difficulty_and_focus?.technical_process_difficulty) + r(profileData?.construction_core_requirements?.special_operations),
        siteManagementDifficulty: r(profileData?.difficulty_and_focus?.site_management_difficulty),
        engineeringCharacteristics: r(profileData?.difficulty_and_focus?.engineering_characteristics),
        greenEnv: r(profileData?.construction_core_requirements?.acceptance_requirements) + r(profileData?.construction_core_requirements?.site_management),
        qualityStandard: r(profileData?.project_base_info?.quality_standard),
        schedulePressure: r(profileData?.construction_core_requirements?.schedule_constraints) + r(profileData?.project_base_info?.duration_requirements),
        procurementComplexity: r(profileData?.construction_core_requirements?.procurement_boundary),
        ownerSuppliedItems: r(profileData?.construction_core_requirements?.owner_supplied_items),
        scoringRules: r(profileData?.evaluation_and_performance_rules?.scoring_items) + r(profileData?.evaluation_and_performance_rules?.method_and_score_weights),
        qualificationReq: r(profileData?.bidder_requirements?.qualification_requirements),
        performanceReq: r(profileData?.bidder_requirements?.performance_requirements),
        personnelReq: r(profileData?.bidder_requirements?.personnel_requirements),
        materialEquip: r(profileData?.construction_core_requirements?.material_equipment_rules),
        bonusItems: r(profileData?.bidder_requirements?.bonus_items),
    };
}

/** 基于信号计算完整评分结果 */
export function computeProfileScoring(profileData: ProfileData): ProfileScoringResult {
    const signal = computeProfileSignal(profileData);

    const filledFieldCount = Object.values(signal).filter(Boolean).length;
    const profileFieldCount = Object.keys(signal).length;
    const profileCoverageRate = profileFieldCount > 0 ? filledFieldCount / profileFieldCount : 0;

    let technicalScore = 0;
    if (signal.technicalDifficulty) technicalScore += 25;
    if (signal.siteManagementDifficulty) technicalScore += 10;
    if (signal.procurementComplexity) technicalScore += 10;
    if (signal.ownerSuppliedItems) technicalScore += 10;
    if (signal.materialEquip) technicalScore += 5;

    const gk = ['绿色', '环保', '节能', '减排', '扬尘', '降噪', '污水', '固废', '生态', '文明', '环境', '低碳'];
    const greenContent = signal.greenEnv + signal.engineeringCharacteristics + signal.qualityStandard;
    const gHits = gk.filter(kw => greenContent.includes(kw)).length;
    let greenScore = 0;
    if (gHits >= 3) greenScore += 25;
    else if (gHits >= 1) greenScore += 15;

    const sk = ['BIM', 'bim', '数字化', '智慧', '智能', '信息化', '物联网', '自动化', '传感器', '云平台', '模型', '三维', '数字孪生'];
    const smartContent = signal.engineeringCharacteristics + signal.materialEquip + signal.technicalDifficulty + signal.siteManagementDifficulty;
    const sHits = sk.filter(kw => smartContent.includes(kw)).length;
    let smartScore = 0;
    if (sHits >= 2) smartScore += 25;
    else if (sHits >= 1) smartScore += 15;

    let standardScore = 0;
    if (!signal.technicalDifficulty && !signal.siteManagementDifficulty) standardScore += 20;
    standardScore += Math.round(profileCoverageRate * 15);

    let difficultyScore = 0;
    if (signal.technicalDifficulty) difficultyScore += 30;
    if (signal.siteManagementDifficulty) difficultyScore += 15;
    if (signal.schedulePressure) difficultyScore += 10;
    if (signal.scoringRules) difficultyScore += 10;
    if (signal.qualificationReq) difficultyScore += 10;
    if (signal.performanceReq) difficultyScore += 10;
    if (signal.personnelReq) difficultyScore += 10;
    if (signal.bonusItems) difficultyScore += 5;
    const difficultyNormalized = Math.min(difficultyScore / 1.2, 100);

    const scores: RouteScore[] = [
        { route: '专项工艺驱动 + 深度难点解析', score: technicalScore, maxScore: 60, color: 'volcano', desc: '招标文件包含专项工艺要求和复杂技术难点，建议以难点拆解为核心，逐项输出专项施工方案与技术保障措施。' },
        { route: '绿色低碳施工示范', score: greenScore, maxScore: 25, color: 'green', desc: '招标文件强调绿色施工与环境保护要求，建议增设节能减排、扬尘治理、文明施工专项章节，展现企业绿色建造能力。' },
        { route: '智慧工地数字化管理', score: smartScore, maxScore: 25, color: 'blue', desc: '招标文件提及BIM或数字化管理要求，建议引入智慧工地、信息化管控、数字化审计等前沿章节，提升技术标先进性。' },
        { route: '常规标准施工组织', score: standardScore, maxScore: 35, color: 'gold', desc: '项目特征以常规施工为主，建议采用企业标准模板，重点突出工期管控、质量保障和安全文明施工。' },
    ];
    scores.sort((a, b) => b.score - a.score);

    const topRoute = scores[0];
    const matchRate = Math.min(Math.round((topRoute.score / topRoute.maxScore) * 100), 99);

    let difficultyLevel: DifficultyLevel = { label: '常规难度', color: 'success' };
    if (difficultyNormalized >= 75) difficultyLevel = { label: '高难度', color: 'red' };
    else if (difficultyNormalized >= 50) difficultyLevel = { label: '中等偏难', color: 'orange' };
    else if (difficultyNormalized >= 25) difficultyLevel = { label: '中等难度', color: 'gold' };

    return {
        signal,
        filledFieldCount,
        profileFieldCount,
        profileCoverageRate,
        scores,
        difficultyNormalized,
        difficultyLevel,
        matchRate,
    };
}
