/**
 * 项目画像展示面板
 * 从 TechBidProjectWorkbench.tsx 中拆分（CTO P0-2）
 */
import React, { useState } from 'react';
import {
    Typography, Button, Card, Space, Tag, Alert, Row, Col,
    Tabs, Tooltip, Modal, Input, message,
} from 'antd';
import {
    BuildOutlined, EditOutlined, BulbOutlined, CloudSyncOutlined,
    FireOutlined, SafetyCertificateOutlined, CheckCircleOutlined,
    FileSearchOutlined, PlayCircleOutlined,
} from '@ant-design/icons';
import {
    type ProfileData, type EvidenceField, type EvidenceListItem,
    type KeywordAuditHit, type RuleEngineHit,
    isEvidenceField, isEvidenceList, getFieldText,
    PROFILE_FIELD_PATH_MAP,
} from '../types/projectProfile';
import { computeProfileScoring } from '../utils/profileScoring';
import { patchProfileField, confirmProfile } from '../api/techBidStep4';
import EvidenceDrawer from './EvidenceDrawer';
import RunReplayPanel from './RunReplayPanel';

const { Title, Text } = Typography;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
interface ProjectProfilePanelProps {
    project: any;
    onConfirm: () => void;
    onRefresh: () => void;
}

const ProjectProfilePanel: React.FC<ProjectProfilePanelProps> = ({ project, onConfirm, onRefresh }) => {
    const profileData: ProfileData | null = project.profile?.profile_json
        ? JSON.parse(project.profile.profile_json)
        : null;

    const [editingField, setEditingField] = useState<{ label: string; value: string; path: string } | null>(null);
    const [editValue, setEditValue] = useState('');
    const [editSaving, setEditSaving] = useState(false);
    const [evidenceDrawer, setEvidenceDrawer] = useState<{ label: string; value: unknown; path?: string } | null>(null);
    const [runReplayOpen, setRunReplayOpen] = useState(false);

    // ─── 编辑保存 ─────────────────────────────────────
    const handleEditSave = async () => {
        if (!editingField || !project?.id) return;
        setEditSaving(true);
        try {
            await patchProfileField(project.id, {
                field_path: editingField.path,
                new_value: editValue,
            });
            message.success('字段已更新');
            setEditingField(null);
            onRefresh();
        } catch (e: unknown) {
            const err = e as { response?: { data?: { error?: string } }; message?: string };
            message.error('更新失败: ' + (err?.response?.data?.error || err?.message));
        } finally {
            setEditSaving(false);
        }
    };

    const handleConfirmProfile = async () => {
        if (!project?.id) return;
        try {
            await confirmProfile(project.id);
            message.success('画像已标记为人工确认');
            onRefresh();
        } catch (e: unknown) {
            const err = e as { response?: { data?: { error?: string } }; message?: string };
            message.error('确认失败: ' + (err?.response?.data?.error || err?.message));
        }
    };

    // ─── 字段状态标签 ─────────────────────────────────
    const renderStatusTags = (value: unknown) => {
        if (!isEvidenceField(value)) return null;
        const tags = [] as React.ReactNode[];
        if (value.missing) {
            tags.push(<Tag color="default" key="missing">缺失</Tag>);
        } else {
            tags.push(<Tag color="success" key="ok">已提取</Tag>);
            if (typeof value.confidence === 'number' && value.confidence > 0 && value.confidence < 0.6) {
                tags.push(<Tag color="warning" key="low">低置信度</Tag>);
            }
        }
        return tags.length > 0 ? <Space size={[4, 4]} wrap>{tags}</Space> : null;
    };

    // ─── 单字段渲染 ───────────────────────────────────
    const renderField = (label: string, value: unknown) => {
        const displayValue = getFieldText(value);
        const editablePath = PROFILE_FIELD_PATH_MAP[label];
        return (
            <div key={label} style={{ marginBottom: 12, borderBottom: '1px solid #f0f0f0', paddingBottom: 8 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, marginBottom: 4 }}>
                    <div style={{ color: '#8c8c8c', fontSize: 12 }}>{label}</div>
                    <Space size={4}>
                        {renderStatusTags(value)}
                        <Tooltip title="查看证据溯源">
                            <FileSearchOutlined
                                style={{ color: '#722ed1', cursor: 'pointer', fontSize: 12 }}
                                onClick={() => setEvidenceDrawer({ label, value, path: editablePath })}
                            />
                        </Tooltip>
                        {editablePath && isEvidenceField(value) && (
                            <Tooltip title="编辑此字段">
                                <EditOutlined
                                    style={{ color: '#1890ff', cursor: 'pointer', fontSize: 12 }}
                                    onClick={() => {
                                        setEditingField({ label, value: (value as EvidenceField)?.value || '', path: editablePath });
                                        setEditValue((value as EvidenceField)?.value || '');
                                    }}
                                />
                            </Tooltip>
                        )}
                    </Space>
                </div>
                <div style={{ fontWeight: 500, fontSize: 14 }}>{displayValue}</div>
                {isEvidenceField(value) && !value.missing && (value.source_text || value.source_location) && (
                    <div style={{ marginTop: 6 }}>
                        {value.source_text && <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>依据：{value.source_text}</Text>}
                        {value.source_location && <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>位置：{value.source_location}</Text>}
                    </div>
                )}
                {isEvidenceList(value) && value.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                        <Space direction="vertical" size={6} style={{ width: '100%' }}>
                            {value.map((item: EvidenceListItem, index: number) => (
                                <div key={`${label}-${index}`} style={{ background: '#fafafa', borderRadius: 6, padding: '8px 10px' }}>
                                    <div style={{ fontWeight: 500 }}>{item?.value || item?.name || '无'}</div>
                                    {(item?.source_text || item?.source_location) && (
                                        <Text type="secondary" style={{ fontSize: 12 }}>
                                            {[item?.source_text, item?.source_location].filter(Boolean).join('｜')}
                                        </Text>
                                    )}
                                </div>
                            ))}
                        </Space>
                    </div>
                )}
            </div>
        );
    };

    // ─── 构建 Tab 项 ──────────────────────────────────
    const profileViews = profileData?.views;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const renderViewTab = (view: any) => {
        if (!view) return null;
        return (
            <div style={{ padding: '8px 0' }}>
                {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                {Array.isArray(view.fields) && view.fields.map((f: any) => renderField(f.label, f.field))}
                {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                {Array.isArray(view.lists) && view.lists.map((l: any) => renderField(l.label, l.items))}
            </div>
        );
    };

    const viewTabItems = profileViews && profileViews.length > 0
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ? profileViews.map((view: any) => ({
            key: view.view_key,
            label: (
                <span>
                    {view.view_label}
                    {typeof view.completeness === 'number' && (
                        <Tag color={view.completeness >= 0.8 ? 'success' : view.completeness >= 0.5 ? 'blue' : 'default'}
                            style={{ marginLeft: 6, fontSize: 11 }}>
                            {Math.round(view.completeness * 100)}%
                        </Tag>
                    )}
                </span>
            ),
            children: renderViewTab(view),
        }))
        : null;

    const tabItems = viewTabItems || (profileData ? [
        {
            key: 'base',
            label: '项目基础',
            children: (
                <div style={{ padding: '8px 0' }}>
                    {renderField('项目名称', profileData.project_base_info?.project_name)}
                    {renderField('招标单位', profileData.project_base_info?.owner_unit || profileData.project_base_info?.tender_unit)}
                    {renderField('工程地点', profileData.project_base_info?.location || profileData.project_base_info?.project_location)}
                    {renderField('施工范围', profileData.project_base_info?.category_and_scope || profileData.project_base_info?.tender_scope || profileData.project_base_info?.project_scope)}
                    {renderField('工期要求', profileData.project_base_info?.duration_requirements || profileData.project_base_info?.duration_requirement)}
                    {renderField('质量标准', profileData.project_base_info?.quality_standard || profileData.project_base_info?.quality_target)}
                    {renderField('项目类型', profileData.project_base_info?.project_type)}
                    {renderField('建设规模', profileData.project_base_info?.construction_scale)}
                    {renderField('安全目标', profileData.project_base_info?.safety_target)}
                </div>
            )
        },
        {
            key: 'construction',
            label: '施工核心',
            children: (
                <div style={{ padding: '8px 0' }}>
                    {renderField('材料设备要求', profileData.construction_core_requirements?.material_equipment_rules || profileData.construction_core_requirements?.material_requirements)}
                    {renderField('施工技术规范', profileData.construction_core_requirements?.technical_specifications || profileData.construction_core_requirements?.technical_requirements)}
                    {renderField('现场管理要求', profileData.construction_core_requirements?.site_management || profileData.construction_core_requirements?.site_management_requirements)}
                    {renderField('验收要求', profileData.construction_core_requirements?.acceptance_requirements || profileData.construction_core_requirements?.acceptance)}
                    {renderField('专项作业要求', profileData.construction_core_requirements?.special_operations || profileData.construction_core_requirements?.special_operation_requirements)}
                    {renderField('采购边界', profileData.construction_core_requirements?.procurement_boundary)}
                    {renderField('甲供项', profileData.construction_core_requirements?.owner_supplied_items || profileData.construction_core_requirements?.owner_supplied_materials)}
                    {renderField('乙供项', profileData.construction_core_requirements?.contractor_supplied_items || profileData.construction_core_requirements?.contractor_supplied_materials)}
                    {renderField('工期节点', profileData.construction_core_requirements?.schedule_constraints || profileData.construction_core_requirements?.schedule_nodes)}
                    {renderField('关键施工方法', profileData.construction_core_requirements?.key_construction_methods)}
                    {renderField('难点工程', profileData.construction_core_requirements?.difficult_works)}
                    {renderField('文明施工要求', profileData.construction_core_requirements?.civilized_construction_requirements)}
                    {renderField('环境保护要求', profileData.construction_core_requirements?.environmental_protection_requirements)}
                    {renderField('现场条件约束', profileData.construction_core_requirements?.site_condition_constraints)}
                </div>
            )
        },
        {
            key: 'bidder',
            label: '投标人要求',
            children: (
                <div style={{ padding: '8px 0' }}>
                    {renderField('资质证书', profileData.bidder_requirements?.qualification_certificates)}
                    {renderField('业绩要求', profileData.bidder_requirements?.performance_requirements)}
                    {renderField('人员要求', profileData.bidder_requirements?.personnel_requirements)}
                    {renderField('加分项', profileData.bidder_requirements?.bonus_items)}
                    {renderField('资格要求', profileData.bidder_requirements?.qualification_requirements)}
                </div>
            )
        },
        {
            key: 'rules',
            label: '评标规则',
            children: (
                <div style={{ padding: '8px 0' }}>
                    {renderField('评标方法', profileData.evaluation_and_performance_rules?.method_and_score_weights)}
                    {renderField('技术标评分维度', profileData.evaluation_and_performance_rules?.technical_evaluation_dimensions)}
                    {renderField('支付说明', profileData.evaluation_and_performance_rules?.payment_method)}
                    {renderField('结算规则', profileData.evaluation_and_performance_rules?.settlement_rules)}
                    {renderField('评分项', profileData.evaluation_and_performance_rules?.scoring_items)}
                    {renderField('废标规则', profileData.evaluation_and_performance_rules?.disqualification_rules)}
                    {renderField('总工期', profileData.evaluation_and_performance_rules?.total_duration)}
                </div>
            )
        }
    ] : []);

    // ─── 审计与规则数据 ───────────────────────────────
    const extractionGaps = profileData?.extraction_gaps || [];
    const uncertainItems = profileData?.uncertain_items || [];
    const requiresManualReview = profileData?.requires_manual_review;
    const keywordAuditHits = profileData?.keyword_audit_hits || [];
    const ruleEngineHits = profileData?.rule_engine_hits || [];

    // ─── 评分计算 ─────────────────────────────────────
    const scoring = profileData ? computeProfileScoring(profileData) : null;

    return (
        <div style={{ padding: '20px' }}>
            <Alert
                message="项目画像生成成功"
                description={project.profile?.summary_text || "AI 专家已根据招标文件深度解析了项目特征。"}
                type="success"
                showIcon
                style={{ marginBottom: 24 }}
            />
            <Row gutter={24}>
                <Col span={16}>
                    <Card title={<Space><BulbOutlined style={{ color: '#faad14' }} />多维度提取结果</Space>} className="profile-card">
                        <Tabs
                            defaultActiveKey="base"
                            items={tabItems}
                            tabPosition="left"
                            style={{ minHeight: 400 }}
                        />
                    </Card>
                </Col>
                <Col span={8}>
                    <Space direction="vertical" size={16} style={{ width: '100%' }}>
                        {scoring && (
                            <>
                                <Card title={<Space><BuildOutlined style={{ color: '#1890ff' }} />CTO 编制倾向</Space>} className="profile-card">
                                    <div style={{ background: '#f8fafc', padding: 16, borderRadius: 8 }}>
                                        <Text type="secondary">基于当前画像特征，AI 建议采用：</Text>
                                        <Title level={5} style={{ margin: '8px 0', color: '#1890ff' }}>{scoring.scores[0].route}</Title>
                                        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>{scoring.scores[0].desc}</Text>
                                        <div style={{ marginTop: 12 }}>
                                            <Tag color="blue">匹配度 {scoring.matchRate}%</Tag>
                                            <Tag color={scoring.difficultyLevel.color}>{scoring.difficultyLevel.label}</Tag>
                                            <Tag color={scoring.profileCoverageRate >= 0.7 ? 'success' : scoring.profileCoverageRate >= 0.4 ? 'blue' : 'default'}>
                                                {scoring.profileCoverageRate >= 0.7 ? '画像数据充分' : scoring.profileCoverageRate >= 0.4 ? '画像数据基本' : '数据不足'}
                                            </Tag>
                                            {requiresManualReview?.value === '是' && <Tag color="warning">待人工确认</Tag>}
                                        </div>
                                    </div>
                                </Card>
                                {scoring.scores.length >= 2 && (
                                    <Card title="其他可选路线" size="small" className="profile-card">
                                        <div style={{ marginTop: 8 }}>
                                            {scoring.scores.slice(1).map((s, idx) => {
                                                const iconMap: Record<string, React.ReactNode> = {
                                                    volcano: <FireOutlined />,
                                                    green: <SafetyCertificateOutlined />,
                                                    blue: <CloudSyncOutlined />,
                                                    gold: <BuildOutlined />,
                                                };
                                                return (
                                                    <Space key={idx} style={{ marginBottom: 6, display: 'flex' }} align="start">
                                                        {iconMap[s.color] || <BuildOutlined />}
                                                        <div>
                                                            <Text strong style={{ fontSize: 13 }}>{s.route}</Text>
                                                            <br />
                                                            <Text type="secondary" style={{ fontSize: 12 }}>{s.desc}</Text>
                                                        </div>
                                                    </Space>
                                                );
                                            })}
                                        </div>
                                    </Card>
                                )}
                            </>
                        )}
                        <Card title="提取质量摘要" className="profile-card">
                            <Space direction="vertical" size={10} style={{ width: '100%' }}>
                                <div>
                                    <Text strong>缺失项</Text>
                                    <div style={{ marginTop: 6 }}>
                                        {Array.isArray(extractionGaps) && extractionGaps.length > 0
                                            ? extractionGaps.map((item, index) => <Tag key={`gap-${index}`} color="default">{item?.name || item?.value}</Tag>)
                                            : <Text type="secondary">无</Text>}
                                    </div>
                                </div>
                                <div>
                                    <Text strong>不确定项</Text>
                                    <div style={{ marginTop: 6 }}>
                                        {Array.isArray(uncertainItems) && uncertainItems.length > 0
                                            ? uncertainItems.map((item, index) => <Tag key={`uncertain-${index}`} color="warning">{item?.name || item?.value}</Tag>)
                                            : <Text type="secondary">无</Text>}
                                    </div>
                                </div>
                                {Array.isArray(keywordAuditHits) && keywordAuditHits.length > 0 && (
                                    <div>
                                        <Text strong>关键词反查</Text>
                                        <div style={{ marginTop: 6 }}>
                                            {keywordAuditHits.map((hit: KeywordAuditHit, index: number) => (
                                                <Tag key={`kw-${index}`} color={hit.severity === 'error' ? 'red' : 'orange'} style={{ marginBottom: 4 }}>
                                                    {hit.group}: 命中 {hit.hit_count} 次, [{(hit.field_labels || []).join(', ')}] 为空
                                                </Tag>
                                            ))}
                                        </div>
                                    </div>
                                )}
                                {Array.isArray(ruleEngineHits) && ruleEngineHits.length > 0 && (
                                    <div>
                                        <Text strong>规则引擎命中</Text>
                                        <div style={{ marginTop: 6 }}>
                                            {ruleEngineHits.map((hit: RuleEngineHit, index: number) => {
                                                const colorMap: Record<string, string> = { duration: 'blue', qualification: 'green', scoring: 'orange', procurement: 'purple', personnel: 'cyan' };
                                                return (
                                                    <Tag key={`rule-${index}`} color={colorMap[hit.category] || 'default'} style={{ marginBottom: 4 }}>
                                                        {hit.rule_name}: {(hit.matches || []).slice(0, 3).join('、')}{(hit.matches || []).length > 3 ? '...' : ''} → {hit.field_path}
                                                    </Tag>
                                                );
                                            })}
                                        </div>
                                    </div>
                                )}
                                <div>
                                    <Text strong>人工复核</Text>
                                    <div style={{ marginTop: 6 }}>
                                        <Tag color={requiresManualReview?.value === '是' ? 'warning' : 'success'}>
                                            {requiresManualReview?.value === '是' ? '需要复核' : '当前无需复核'}
                                        </Tag>
                                    </div>
                                    {requiresManualReview?.notes && (
                                        <Text type="secondary" style={{ fontSize: 12 }}>{requiresManualReview.notes}</Text>
                                    )}
                                </div>
                            </Space>
                        </Card>
                    </Space>
                </Col>
            </Row>
            <div style={{ marginTop: 24, textAlign: 'center' }}>
                <Space size={16}>
                    <Button icon={<PlayCircleOutlined />} onClick={() => setRunReplayOpen(true)}>
                        抽取过程回放
                    </Button>
                    {project.profile?.confirmed_at ? (
                        <Tag color="success" style={{ fontSize: 14, padding: '4px 12px' }}>
                            <CheckCircleOutlined /> 人工已确认 {project.profile.confirmed_by ? `(${project.profile.confirmed_by})` : ''}
                        </Tag>
                    ) : (
                        <Button onClick={handleConfirmProfile}>标记人工已确认</Button>
                    )}
                    <Button type="primary" size="large" onClick={onConfirm}>确认画像进入路线规划</Button>
                </Space>
            </div>
            <Modal
                title={`编辑字段: ${editingField?.label || ''}`}
                open={!!editingField}
                onCancel={() => setEditingField(null)}
                onOk={handleEditSave}
                confirmLoading={editSaving}
                okText="保存"
                cancelText="取消"
            >
                <div style={{ marginBottom: 12 }}>
                    <Text type="secondary">当前值: {editingField?.value || '(空)'}</Text>
                </div>
                <Input.TextArea
                    value={editValue}
                    onChange={(e) => setEditValue(e.target.value)}
                    rows={4}
                    placeholder="输入新值..."
                />
            </Modal>
            <EvidenceDrawer
                open={!!evidenceDrawer}
                onClose={() => setEvidenceDrawer(null)}
                projectId={project?.id || ''}
                fieldLabel={evidenceDrawer?.label || ''}
                fieldValue={evidenceDrawer?.value}
                fieldPath={evidenceDrawer?.path}
            />
            <RunReplayPanel
                open={runReplayOpen}
                onClose={() => setRunReplayOpen(false)}
                projectId={project?.id || ''}
            />
        </div>
    );
};

export default ProjectProfilePanel;
