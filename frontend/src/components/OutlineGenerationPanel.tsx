/**
 * 目录生成面板
 * 从 TechBidProjectWorkbench.tsx 中拆分（任务10 模块化）
 */
import React, { lazy, Suspense } from 'react';
import {
    Typography, Button, Card, Space, Tag, Alert, Input,
    Empty, List, Select, Tooltip, Timeline, Row, Col, Spin, Modal, Result,
} from 'antd';
import {
    SafetyCertificateOutlined, AuditOutlined,
    WarningOutlined, EditOutlined, InfoCircleOutlined,
    PlusCircleOutlined, BuildOutlined,
} from '@ant-design/icons';

// P1-2: 懒加载重型组件，减少首屏 bundle 体积
const Step4MappingCoveragePanel = lazy(() => import('./Step4MappingCoveragePanel'));
import { getChapterLabel } from '../utils/numberToChinese';
import {
    type Step4FactMapping,
    type Step4Coverage,
    type Step4RequirementRow,
    type Step4FullResponse,
    type Step4ConflictAudit,
    type Step4FactCandidate,
    type Step4AgentRunRow,
    type Step4ApprovalLogRow,
    type Step4RunRow,
    type OutlineVersionRow,
    type OutlineVersionDetailResponse,
} from '../api/techBidStep4';

const { Title, Text, Paragraph } = Typography;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
interface OutlineGenerationPanelProps {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    project: any;
    /** 更新章节名称回调 */
    onUpdateChapterName?: (chapterId: string, newName: string) => void;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    displayChapters: any[];
    structurePlan: unknown;
    step4ArtifactsWarning: string | null;
    onOpenAuditFact: (node: unknown) => void;
    // Outline versions
    outlineVersions: OutlineVersionRow[];
    selectedOutlineVerId: string | null;
    onSelectOutlineVerId: (id: string) => void;
    selectingOutlineVer: boolean;
    onSwitchOutlineVersion: () => Promise<void>;
    // Step4 data
    step4ArtifactsLoading: boolean;
    step4Mappings: Step4FactMapping[];
    step4Coverage: Step4Coverage | null;
    step4Requirements: Step4RequirementRow[];
    step4FullResponse: Step4FullResponse | null;
    step4ConflictAudit: Step4ConflictAudit | null;
    step4FactCandidates: Step4FactCandidate[];
    selectedOutlineVersionDetail: OutlineVersionDetailResponse | null;
    recommendedOutlineVersionDetail: OutlineVersionDetailResponse | null;
    step4DrawerOpen: boolean;
    onOpenDrawer: () => void;
    onDrawerClose: () => void;
    step4HighlightFactId: string | null;
    step4AgentRuns: Step4AgentRunRow[];
    step4ApprovalLogs: Step4ApprovalLogRow[];
    step4RunHistory: Step4RunRow[];
    // 新增两步走回调
    onGenerateChapters?: () => Promise<void>;
    onConfirmChapters?: (chapters: string[]) => Promise<void>;
    onExpandStructure?: () => Promise<void>;
}

const OutlineGenerationPanel: React.FC<OutlineGenerationPanelProps> = ({
    project,
    displayChapters,
    structurePlan,
    step4ArtifactsWarning,
    onOpenAuditFact,
    outlineVersions,
    selectedOutlineVerId,
    onSelectOutlineVerId,
    selectingOutlineVer,
    onSwitchOutlineVersion,
    step4ArtifactsLoading,
    step4Mappings,
    step4Coverage,
    step4Requirements,
    step4FullResponse,
    step4ConflictAudit,
    step4FactCandidates,
    selectedOutlineVersionDetail,
    recommendedOutlineVersionDetail,
    step4DrawerOpen,
    onOpenDrawer,
    onDrawerClose,
    step4HighlightFactId,
    step4AgentRuns,
    step4ApprovalLogs,
    step4RunHistory,
    onUpdateChapterName,
    onGenerateChapters,
    onConfirmChapters,
    onExpandStructure,
}) => {
    const [editingChapterId, setEditingChapterId] = React.useState<string | null>(null);
    const [editingChapterName, setEditingChapterName] = React.useState('');
    const [missingModalVisible, setMissingModalVisible] = React.useState(false);
    const [weakModalVisible, setWeakModalVisible] = React.useState(false);

    // 两步走：骨架编辑状态
    const [draftChapters, setDraftChapters] = React.useState<string[]>([]);
    const [confirmingChapters, setConfirmingChapters] = React.useState(false);

    // 同步后端返回的草案到本地编辑状态
    React.useEffect(() => {
        if (project?.chapter_draft_json) {
            try {
                const parsed = JSON.parse(project.chapter_draft_json);
                if (Array.isArray(parsed)) {
                    setDraftChapters(parsed);
                }
            } catch (e) {
                console.error('Failed to parse chapter_draft_json', e);
            }
        }
    }, [project?.chapter_draft_json]);

    // 根据 fact_id 查找详情，优先从 mappings 查找，其次从 factCandidates 查找
    const findFactDetails = (factId: string) => {
        // 1. 优先从 step4Mappings 查找
        const mapping = step4Mappings.find(m => m.fact_id === factId);
        if (mapping) {
            return {
                fact_name: mapping.fact_name,
                fact_type: mapping.fact_type,
                source_location: mapping.source_location,
                source_chapter: mapping.source_chapter,
                snippet: null,
                priority: mapping.priority,
            };
        }
        // 2. 从 factCandidates 查找
        const candidate = step4FactCandidates.find(f => f.fact_id === factId);
        if (candidate) {
            return {
                fact_name: candidate.fact_name,
                fact_type: candidate.fact_type,
                source_location: candidate.source_location,
                source_chapter: null,
                snippet: candidate.snippet,
                priority: null,
            };
        }
        return null;
    };

    // 获取缺失项的详情列表
    const missingFacts = (step4Coverage?.missing_fact_ids || []).map(id => {
        const detail = findFactDetails(id);
        return {
            id,
            fact_name: detail?.fact_name || id,
            fact_type: detail?.fact_type || 'unknown',
            source_location: detail?.source_location,
            source_chapter: detail?.source_chapter,
            snippet: detail?.snippet,
            priority: detail?.priority,
        };
    });

    // 获取弱覆盖项的详情列表
    const weakFacts = (step4Coverage?.weak_fact_ids || []).map(id => {
        const detail = findFactDetails(id);
        return {
            id,
            fact_name: detail?.fact_name || id,
            fact_type: detail?.fact_type || 'high_priority',
            source_location: detail?.source_location,
            source_chapter: detail?.source_chapter,
            snippet: detail?.snippet,
            priority: 'high',
        };
    });

    const handleStartEdit = (chapterId: string, currentName: string) => {
        setEditingChapterId(chapterId);
        setEditingChapterName(currentName);
    };

    const handleSaveEdit = () => {
        if (editingChapterId && onUpdateChapterName && editingChapterName.trim()) {
            onUpdateChapterName(editingChapterId, editingChapterName.trim());
        }
        setEditingChapterId(null);
        setEditingChapterName('');
    };

    const handleCancelEdit = () => {
        setEditingChapterId(null);
        setEditingChapterName('');
    };

    // Compute missing items summary from step4Coverage and step4FullResponse
    const missingItemCount = (step4Coverage?.missing_fact_ids?.length ?? 0)
        + (step4FullResponse?.missing_requirement_ids?.length ?? 0);
    const weakItemCount = (step4Coverage?.weak_fact_ids?.length ?? 0)
        + (step4FullResponse?.weak_requirement_ids?.length ?? 0);
    const hasRisk = missingItemCount > 0 || weakItemCount > 0;
    const riskLevel = missingItemCount > 5 || project?.risk_level === 'HIGH'
        ? 'HIGH' : missingItemCount > 0 || weakItemCount > 0 || project?.risk_level === 'MEDIUM'
            ? 'MEDIUM' : 'LOW';

    const hasChapters = displayChapters.filter((c: unknown) => !(c as Record<string, unknown>).parent_id).length > 0;

    // 两步走：草案编辑回调
    const handleUpdateDraft = (index: number, val: string) => {
        const newDraft = [...draftChapters];
        newDraft[index] = val;
        setDraftChapters(newDraft);
    };

    const handleAddDraft = () => {
        setDraftChapters([...draftChapters, '新章节名称']);
    };

    const handleRemoveDraft = (index: number) => {
        const newDraft = draftChapters.filter((_, i) => i !== index);
        setDraftChapters(newDraft);
    };

    const handleConfirmDraft = async () => {
        if (!onConfirmChapters || draftChapters.length === 0) return;
        setConfirmingChapters(true);
        try {
            await onConfirmChapters(draftChapters);
        } finally {
            setConfirmingChapters(false);
        }
    };

    const renderEmptyOrTwoStep = () => {
        const s4 = project?.step4_status;
        
        // Phase A: 正在生成以及草案确认阶段
        if (s4 === 'generating_chapters') {
            return (
                <div style={{ textAlign: 'center', padding: '100px 0' }}>
                    <Spin size="large" tip="AI 专家正在分析招标文件并生成一级章骨架..." />
                    <div style={{ marginTop: 16 }}>
                        <Text type="secondary">这通常需要 10-20 秒，请稍后控制并发刷新查看</Text>
                    </div>
                </div>
            );
        }

        if (s4 === 'outline_chapter_confirm_pending') {
            return (
                <div style={{ padding: 24, background: '#f8fafc', borderRadius: 8 }}>
                    <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div>
                            <Title level={4} style={{ marginBottom: 4 }}>
                                第一步：确认目录骨架 (Skeleton)
                            </Title>
                            <Text type="secondary">
                                AI 已根据招标文件生成以下一级章。您可以调整名称、增加或删除章节。确认后将进入详细结构展开。
                            </Text>
                        </div>
                        <Button
                            type="primary"
                            icon={<SafetyCertificateOutlined />}
                            onClick={handleConfirmDraft}
                            loading={confirmingChapters}
                            disabled={draftChapters.length === 0}
                        >
                            确认骨架并进入第二步
                        </Button>
                    </div>
                    <List
                        size="small"
                        bordered
                        style={{ background: '#fff' }}
                        dataSource={draftChapters}
                        renderItem={(item, index) => (
                            <List.Item
                                actions={[
                                    <Button key="delete" type="text" danger size="small" onClick={() => handleRemoveDraft(index)}>删除</Button>
                                ]}
                            >
                                <Space style={{ width: '100%' }}>
                                    <Text type="secondary" style={{ width: 40 }}>{index + 1}.</Text>
                                    <Input
                                        value={item}
                                        onChange={(e) => handleUpdateDraft(index, e.target.value)}
                                        style={{ border: 'none', background: 'transparent', width: '100%' }}
                                    />
                                </Space>
                            </List.Item>
                        )}
                        footer={
                            <Button type="dashed" block icon={<PlusCircleOutlined />} onClick={handleAddDraft}>
                                添加新章节
                            </Button>
                        }
                    />
                </div>
            );
        }

        // Phase B: 骨架已确认，等待展开
        if (s4 === 'chapter_confirmed') {
            return (
                <div style={{ textAlign: 'center', padding: '100px 0' }}>
                    <Result
                        status="success"
                        title="目录骨架已确认"
                        subTitle="接下来 AI 将基于此骨架，深度结合招标文件内容，通过两轮分析将其展开为完整的三级目录结构。"
                        extra={[
                            <Button
                                type="primary"
                                key="expand"
                                size="large"
                                onClick={onExpandStructure}
                            >
                                发起详细结构展开 (Phase B)
                            </Button>,
                            <Button key="reset" onClick={onGenerateChapters}>
                                重新生成骨架
                            </Button>
                        ]}
                    />
                </div>
            );
        }

        if (s4 === 'expanding_structure') {
            return (
                <div style={{ textAlign: 'center', padding: '100px 0' }}>
                    <Spin size="large" tip="AI 专家正在进行深度结构展开 (Phase B)..." />
                    <div style={{ marginTop: 24 }}>
                        <Text strong>执行中：第一轮逻辑推演 + 第二轮需求对齐</Text>
                        <br />
                        <Text type="secondary" style={{ fontSize: 13 }}>
                            此过程通常需要 40-60 秒，完成后目录将自动同步并进行一次全量合规审计。
                        </Text>
                    </div>
                </div>
            );
        }

        return (
            <div style={{ paddingTop: 100 }}>
                <Empty
                    description={
                        <span>
                            {project?.current_step_status === 'waiting_for_approval' && !structurePlan
                                ? (
                                    <div style={{ color: '#d97706' }}>
                                        <Text strong style={{ color: '#d97706' }}>结构计划状态异常</Text>
                                        <br />
                                        <Text type="secondary" style={{ fontSize: 13 }}>{step4ArtifactsWarning || '当前没有待审批结构计划，请刷新或重新生成目录。'}</Text>
                                    </div>
                                )
                                : project?.current_step_status === 'running' || ['facts_extracting', 'generating_outline', 'auditing_outline', 'refining_outline', 'outline_generating_direct'].includes(project?.step4_status || '')
                                    ? 'AI 专家正在基于招标文件直接生成三级目录，请稍后刷新查看...'
                                    : project?.current_step_status === 'failed' || project?.step4_status === 'failed'
                                        ? (
                                            <div style={{ color: '#ff4d4f' }}>
                                                <Text type="danger" strong>目录生成失败</Text>
                                                <br />
                                                <Text type="secondary" style={{ fontSize: 13 }}>{project?.last_error_message || '发生了未知错误，建议尝试重新选择路线。'}</Text>
                                            </div>
                                        )
                                        : (
                                            <Space direction="vertical">
                                                <Text>暂无目录结构数据。请选择生成模式：</Text>
                                                <Space style={{ marginTop: 12 }}>
                                                    <Button type="primary" icon={<BuildOutlined />} onClick={onGenerateChapters}>
                                                        分步生成 (推荐：先定骨架后内容)
                                                    </Button>
                                                </Space>
                                            </Space>
                                        )}
                        </span>
                    }
                />
            </div>
        );
    };

    return (
        <div style={{ padding: 24 }}>
            <Row gutter={24}>
                <Col span={18}>
                    {/* G4: 漏项提示横幅（目录生成后展示）- 移至白色背景区域外部 */}
                    {hasChapters && hasRisk && (
                        <Alert
                            type={riskLevel === 'HIGH' ? 'error' : 'warning'}
                            showIcon
                            icon={<WarningOutlined />}
                            message={
                                <Space>
                                    <Text strong>漏项风险提示</Text>
                                    {missingItemCount > 0 && (
                                        <Tag color="error" style={{ cursor: 'pointer' }} onClick={() => setMissingModalVisible(true)}>
                                            {missingItemCount} 条缺失项（点击查看）
                                        </Tag>
                                    )}
                                    {weakItemCount > 0 && (
                                        <Tag color="orange" style={{ cursor: 'pointer' }} onClick={() => setWeakModalVisible(true)}>
                                            {weakItemCount} 条弱覆盖（点击查看）
                                        </Tag>
                                    )}
                                </Space>
                            }
                            description={
                                <Text type="secondary" style={{ fontSize: 12 }}>
                                    目录基于招标文件直接生成，部分要求项尚未完全覆盖。建议通过右侧「审计看板」查看详情，或点击「重新生成目录」重新生成完整目录。
                                </Text>
                            }
                            style={{ marginBottom: 16 }}
                        />
                    )}

                    {/* 缺失项 Modal */}
                    <Modal
                        title={<Text type="danger">缺失项详情（共 {missingFacts.length} 条）</Text>}
                        open={missingModalVisible}
                        onCancel={() => setMissingModalVisible(false)}
                        footer={null}
                        width={800}
                    >
                        <List
                            size="small"
                            dataSource={missingFacts}
                            renderItem={(fact) => (
                                <List.Item>
                                    <div style={{ width: '100%' }}>
                                        <Space>
                                            <Tag color="red">{fact.fact_type}</Tag>
                                            <Text strong>{fact.fact_name}</Text>
                                            {fact.priority && <Tag color="purple">{fact.priority}</Tag>}
                                        </Space>
                                        {fact.source_chapter && (
                                            <div style={{ marginTop: 4 }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    招标文件来源：{fact.source_chapter}
                                                </Text>
                                            </div>
                                        )}
                                        {fact.source_location && (
                                            <div style={{ marginTop: 2 }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    位置：{fact.source_location}
                                                </Text>
                                            </div>
                                        )}
                                        {fact.snippet && (
                                            <div style={{ marginTop: 4, padding: 8, background: '#fff1f0', borderRadius: 4, borderLeft: '3px solid #ff4d4f' }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    {fact.snippet}
                                                </Text>
                                            </div>
                                        )}
                                    </div>
                                </List.Item>
                            )}
                        />
                    </Modal>

                    {/* 弱覆盖项 Modal */}
                    <Modal
                        title={<Text type="warning">弱覆盖项详情（共 {weakFacts.length} 条）</Text>}
                        open={weakModalVisible}
                        onCancel={() => setWeakModalVisible(false)}
                        footer={null}
                        width={800}
                    >
                        <List
                            size="small"
                            dataSource={weakFacts}
                            renderItem={(fact) => (
                                <List.Item>
                                    <div style={{ width: '100%' }}>
                                        <Space>
                                            <Tag color="orange">高优先级</Tag>
                                            <Text strong>{fact.fact_name}</Text>
                                        </Space>
                                        {fact.source_chapter && (
                                            <div style={{ marginTop: 4 }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    招标文件来源：{fact.source_chapter}
                                                </Text>
                                            </div>
                                        )}
                                        {fact.source_location && (
                                            <div style={{ marginTop: 2 }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    位置：{fact.source_location}
                                                </Text>
                                            </div>
                                        )}
                                        {fact.snippet && (
                                            <div style={{ marginTop: 4, padding: 8, background: '#fff7e6', borderRadius: 4, borderLeft: '3px solid #faad14' }}>
                                                <Text type="secondary" style={{ fontSize: 12 }}>
                                                    {fact.snippet}
                                                </Text>
                                            </div>
                                        )}
                                    </div>
                                </List.Item>
                            )}
                        />
                    </Modal>

                    <div style={{ background: '#fff', borderRadius: 8, padding: 24, border: '1px solid #e2e8f0', minHeight: 600 }}>

                        {displayChapters.filter((c: unknown) => !(c as Record<string, unknown>).parent_id).length === 0 ? renderEmptyOrTwoStep() : (
                            <div className="elastic-outline-container">
                                {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                                {displayChapters.filter((c: unknown) => !(c as Record<string, unknown>).parent_id).map((chapter: any, cIdx: number) => (
                                    <div key={chapter.id} className="elastic-outline-item">
                                        <div className="chapter-node">
                                            <Space>
                                                <Text style={{ fontSize: 20, fontWeight: 700, color: '#1e293b' }}>{getChapterLabel(cIdx + 1, 'chapter')} {chapter.chapter_name}</Text>
                                                {editingChapterId === chapter.id ? (
                                                    <Space.Compact>
                                                        <Input
                                                            size="small"
                                                            value={editingChapterName}
                                                            onChange={(e) => setEditingChapterName(e.target.value)}
                                                            onPressEnter={handleSaveEdit}
                                                            style={{ width: 200 }}
                                                            autoFocus
                                                        />
                                                        <Button size="small" type="primary" onClick={handleSaveEdit}>保存</Button>
                                                        <Button size="small" onClick={handleCancelEdit}>取消</Button>
                                                    </Space.Compact>
                                                ) : (
                                                    onUpdateChapterName && (
                                                        <Button type="text" size="small" icon={<EditOutlined />} onClick={() => handleStartEdit(chapter.id, chapter.chapter_name)} />
                                                    )
                                                )}
                                            </Space>
                                        </div>
                                        <div style={{ paddingLeft: 20 }}>
                                            {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                                            {displayChapters.filter((c: unknown) => (c as Record<string, unknown>).parent_id === chapter.id).map((unit: any, uIdx: number) => (
                                                <div key={unit.id} style={{ marginBottom: 20 }}>
                                                    <div className="section-node">
                                                        <Text>{getChapterLabel(uIdx + 1, 'unit')} {unit.chapter_name}</Text>
                                                    </div>
                                                    <div className="subsection-list">
                                                        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                                                        {displayChapters.filter((c: unknown) => (c as Record<string, unknown>).parent_id === unit.id).map((sub: any, sIdx: number) => {
                                                            // 提取标题中的引用信息（括号内容）
                                                            const citationMatch = sub.chapter_name?.match(/（(.+?)）$/);
                                                            const citation = citationMatch ? citationMatch[1] : null;
                                                            const cleanTitle = citation ? sub.chapter_name.replace(/（.+?）$/, '') : sub.chapter_name;
                                                            
                                                            return (
                                                                <div key={sub.id} className="subsection-node" style={{ paddingRight: 8 }}>
                                                                    <Space align="center" style={{ flex: 1 }}>
                                                                        <Text>{getChapterLabel(sIdx + 1, 'subsection')} {cleanTitle}</Text>
                                                                        {citation && (
                                                                            <Tooltip title={<>引用来源：<br />{citation}</>}>
                                                                                <span style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', color: '#1890ff' }}>
                                                                                    <InfoCircleOutlined />
                                                                                </span>
                                                                            </Tooltip>
                                                                        )}
                                                                        {sub.requirement_ids_json && JSON.parse(sub.requirement_ids_json).length > 0 && (
                                                                            <Tooltip title={`对应需求: ${JSON.parse(sub.requirement_ids_json).join(', ')}`}>
                                                                                <span
                                                                                    style={{ cursor: 'pointer', display: 'flex', alignItems: 'center' }}
                                                                                    onClick={() => onOpenAuditFact(sub)}
                                                                                >
                                                                                    <SafetyCertificateOutlined style={{ color: '#52c41a', fontSize: 13 }} />
                                                                                </span>
                                                                            </Tooltip>
                                                                        )}
                                                                        {sub.must_have === 1 && (
                                                                            <Tag color="red" style={{ fontSize: 10, margin: 0, padding: '0 4px', height: 16, lineHeight: '14px' }}>必选</Tag>
                                                                        )}
                                                                    </Space>
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                </Col>
                <Col span={6}>
                    <div style={{ position: 'sticky', top: 24 }}>
                        {outlineVersions.length > 0 && (
                            <Card size="small" style={{ marginBottom: 16 }} title="目录候选版本（Step4）" bordered={false}>
                                <Space direction="vertical" style={{ width: '100%' }} size="small">
                                    <Text type="secondary" style={{ fontSize: 12 }}>
                                        {outlineVersions.length > 1
                                            ? '存在多个快照时可对比后选择其一，同步到章节计划（供正文生成）。'
                                            : '当前运行生成的目录版本。'}
                                    </Text>
                                    <Select
                                        style={{ width: '100%' }}
                                        placeholder="选择版本"
                                        value={selectedOutlineVerId ?? undefined}
                                        onChange={(v) => onSelectOutlineVerId(v)}
                                        options={outlineVersions.map((v) => ({
                                            value: v.id,
                                            label: `版本 ${v.version_no}${v.status === 'recommended' ? ' · 推荐' : ''}${v.rationale ? ` — ${v.rationale}` : ''}`,
                                        }))}
                                    />
                                    <Button
                                        type="primary"
                                        size="small"
                                        loading={selectingOutlineVer}
                                        disabled={!selectedOutlineVerId}
                                        onClick={onSwitchOutlineVersion}
                                    >
                                        同步到章节计划
                                    </Button>
                                </Space>
                            </Card>
                        )}
                        <Suspense fallback={<div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}><Spin tip="加载审计看板..." /></div>}>
                            <Step4MappingCoveragePanel
                                compact
                                loading={step4ArtifactsLoading}
                                mappings={step4Mappings}
                                coverage={step4Coverage}
                                requirements={step4Requirements}
                                fullResponse={step4FullResponse}
                                conflictAudit={step4ConflictAudit}
                                factCandidates={step4FactCandidates}
                                outlineVersions={outlineVersions}
                                selectedVersionId={selectedOutlineVerId}
                                selectedVersionDetail={selectedOutlineVersionDetail}
                                recommendedVersionDetail={recommendedOutlineVersionDetail}
                                drawerOpen={step4DrawerOpen}
                                onOpenDrawer={onOpenDrawer}
                                onDrawerClose={onDrawerClose}
                                highlightFactId={step4HighlightFactId}
                                gateStatus={project.step4_gate_result}
                                gateReason={project.step4_gate_reason}
                            />
                        </Suspense>

                        {step4AgentRuns.length > 0 && (
                            <Card size="small" style={{ marginTop: 16 }} bordered={false} title="Step4 Agent 历史">
                                <Timeline
                                    items={step4AgentRuns.slice(0, 6).map((run) => ({
                                        color: run.status === 'done' ? 'green' : run.status === 'failed' ? 'red' : run.status === 'running' ? 'blue' : 'gray',
                                        children: (
                                            <div>
                                                <Text strong>{run.agent_name}</Text>
                                                <Text type="secondary"> · {run.stage}</Text>
                                                <div style={{ fontSize: 12, color: '#94a3b8' }}>
                                                    {run.status}
                                                    {typeof run.duration_ms === 'number' ? ` · ${run.duration_ms} ms` : ''}
                                                </div>
                                            </div>
                                        ),
                                    }))}
                                />
                            </Card>
                        )}

                        {step4FactCandidates.length > 0 && (
                            <Card size="small" style={{ marginTop: 16 }} bordered={false} title="Step4 事实候选">
                                <List
                                    size="small"
                                    dataSource={step4FactCandidates.slice(0, 5)}
                                    renderItem={(item) => (
                                        <List.Item>
                                            <List.Item.Meta
                                                title={<Space><Text strong>{item.fact_name}</Text><Tag>{item.fact_type}</Tag></Space>}
                                                description={item.snippet || item.source_location || item.fact_id}
                                            />
                                        </List.Item>
                                    )}
                                />
                            </Card>
                        )}

                        {step4ApprovalLogs.length > 0 && (
                            <Card size="small" style={{ marginTop: 16 }} bordered={false} title="Step4 审批留痕">
                                <List
                                    size="small"
                                    dataSource={step4ApprovalLogs.slice(0, 5)}
                                    renderItem={(item) => (
                                        <List.Item>
                                            <List.Item.Meta
                                                title={<Space><Text strong>{item.stage}</Text><Tag>{item.action}</Tag></Space>}
                                                description={`${item.reason || '—'}${item.operator_id ? ` · ${item.operator_id}` : ''}`}
                                            />
                                        </List.Item>
                                    )}
                                />
                            </Card>
                        )}

                        {step4RunHistory.length > 0 && (
                            <Card size="small" style={{ marginTop: 16 }} bordered={false} title="Step4 运行历史">
                                <List
                                    size="small"
                                    dataSource={step4RunHistory.slice(0, 5)}
                                    renderItem={(run) => (
                                        <List.Item>
                                            <List.Item.Meta
                                                title={<Space><Text strong>Run #{run.id}</Text><Tag>{run.status}</Tag></Space>}
                                                description={`${run.current_stage || '—'}${run.gate_result ? ` · ${run.gate_result}` : ''}`}
                                            />
                                        </List.Item>
                                    )}
                                />
                            </Card>
                        )}

                        <Card size="small" style={{ marginTop: 16, background: '#f8fafc' }} bordered={false}>
                            <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
                                <Title level={5} style={{ fontSize: 13, marginBottom: 8 }}>操作贴士</Title>
                                1. 点击目录节点右侧的 <AuditOutlined /> 图标，查看该章节的原文支持证据。<br />
                                2. 右侧「审计健康看板」显示映射率与响应率，BLOCK/REVISE 时建议查看详情并「补漏重生」。<br />
                                3. 当前目录基于招标文件直接生成，前三步产物作为辅助参考，不再依赖骨架匹配。
                            </Paragraph>
                        </Card>
                    </div>
                </Col>
            </Row>
        </div>
    );
};

export default OutlineGenerationPanel;
