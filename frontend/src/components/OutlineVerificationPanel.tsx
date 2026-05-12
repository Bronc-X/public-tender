/**
 * 终审核查面板（outline_verification 步骤）
 * 从 TechBidProjectWorkbench.tsx 中拆分（CTO P0-2）
 */
import React from 'react';
import {
    Typography, Button, Card, Space, Tag, Alert, Row, Col,
    Statistic, Badge, List, Tabs,
} from 'antd';
import {
    CheckCircleOutlined, WarningOutlined, BulbOutlined,
    SafetyCertificateOutlined, CloudSyncOutlined, TeamOutlined,
    ToolOutlined, PlayCircleOutlined, EditOutlined, ReloadOutlined,
} from '@ant-design/icons';
import Step4MappingCoveragePanel from './Step4MappingCoveragePanel';
import type {
    Step4FactMapping, Step4Coverage, Step4RequirementRow,
    Step4FullResponse, Step4ConflictAudit, Step4FactCandidate,
    OutlineVersionRow,
} from '../api/techBidStep4';

const { Title, Text, Paragraph } = Typography;

interface OutlineVerificationPanelProps {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    project: any;
    verifying: boolean;
    onConfirm: () => void;
    onRunVerification: () => void;
    onStartManualAudit: () => void;
    onForceUnlock: () => void;
    onOpenFactsDrawer: () => void;
    // Step4 data
    step4ArtifactsLoading: boolean;
    step4Mappings: Step4FactMapping[];
    step4Coverage: Step4Coverage | null;
    step4Requirements: Step4RequirementRow[];
    step4FullResponse: Step4FullResponse | null;
    step4ConflictAudit: Step4ConflictAudit | null;
    step4FactCandidates: Step4FactCandidate[];
    outlineVersions: OutlineVersionRow[];
    selectedOutlineVerId: string | null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    selectedOutlineVersionDetail: any;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    recommendedOutlineVersionDetail: any;
    step4DrawerOpen: boolean;
    onOpenDrawer: () => void;
    onDrawerClose: () => void;
    step4HighlightFactId: string | null;
}

const OutlineVerificationPanel: React.FC<OutlineVerificationPanelProps> = ({
    project,
    verifying,
    onConfirm,
    onRunVerification,
    onStartManualAudit,
    onForceUnlock,
    onOpenFactsDrawer,
    step4ArtifactsLoading,
    step4Mappings,
    step4Coverage,
    step4Requirements,
    step4FullResponse,
    step4ConflictAudit,
    step4FactCandidates,
    outlineVersions,
    selectedOutlineVerId,
    selectedOutlineVersionDetail,
    recommendedOutlineVersionDetail,
    step4DrawerOpen,
    onOpenDrawer,
    onDrawerClose,
    step4HighlightFactId,
}) => {
    const finalDecision = project.latestVerification?.final_decision || project.latestAudit?.final_decision;
    const canProceedGate = project.can_enter_content_generation === 1;
    const isStep5Blocked = (finalDecision === 'BLOCK' || finalDecision === 'REVISE') && !canProceedGate;
    const hasResult = project.current_step_status === 'success' || (project.verification_result && project.current_step_status !== 'running');

    return (
        <div style={{ padding: 24 }}>
            <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div>
                    <Title level={4} style={{ marginBottom: 4 }}>最终核查与专家签批 (Step 5 终审大关)</Title>
                    <Text type="secondary">由高级 AI 终审专家对全量事实覆盖情况、编制逻辑与风险点进行最终裁决。</Text>
                </div>
                <Space>
                    <Button size="large" type="primary" onClick={onConfirm} disabled={!!isStep5Blocked} icon={<CheckCircleOutlined />}>
                        通过终审，开始生成正文
                    </Button>
                    <Button size="large" icon={<ReloadOutlined />} onClick={onRunVerification} loading={verifying}>
                        重新发起终审
                    </Button>
                </Space>
            </div>

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

            {project.current_step_status === 'failed' && (
                <Alert message="核验执行失败" description={project.last_error_message} type="error" showIcon style={{ marginBottom: 24 }} />
            )}

            {hasResult ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
                    {project.latestVerification ? (
                        <Card bordered={false} style={{ background: project.latestVerification.final_decision === 'PASS' ? '#f6ffed' : project.latestVerification.final_decision === 'BLOCK' ? '#fff1f0' : '#fff7e6', borderRadius: 12, border: '1px solid #d9d9d9' }} className="shadow-sm">
                            <Row align="middle" gutter={24}>
                                <Col span={4}>
                                    <div style={{ textAlign: 'center' }}>
                                        {project.latestVerification.final_decision === 'PASS' ? (
                                            <CheckCircleOutlined style={{ fontSize: 64, color: '#52c41a' }} />
                                        ) : project.latestVerification.final_decision === 'BLOCK' ? (
                                            <WarningOutlined style={{ fontSize: 64, color: '#ff4d4f' }} />
                                        ) : (
                                            <BulbOutlined style={{ fontSize: 64, color: '#faad14' }} />
                                        )}
                                        <div style={{ marginTop: 8, fontWeight: 800, fontSize: 18 }}>
                                            {project.latestVerification.final_decision}
                                        </div>
                                    </div>
                                </Col>
                                <Col span={15}>
                                    <Title level={5}>
                                        专家终核：{project.latestVerification.final_decision === 'PASS' ? '建议通过' : project.latestVerification.final_decision === 'BLOCK' ? '强力阻断' : '建议修订'}
                                        <Tag style={{ marginLeft: 12 }} color="blue">{project.latestVerification.verification_model || '豆包-Pro'}</Tag>
                                    </Title>
                                    <Paragraph style={{ fontSize: 15 }}>{project.latestVerification.summary || '等待终核签批报告...'}</Paragraph>
                                    <Space>
                                        {project.latestVerification.risk_level && (
                                            <Space>
                                                <Text type="secondary">风险等级:</Text>
                                                <Tag color={project.latestVerification.risk_level === 'HIGH' ? 'red' : project.latestVerification.risk_level === 'MEDIUM' ? 'orange' : 'green'}>{project.latestVerification.risk_level}</Tag>
                                            </Space>
                                        )}
                                        {project.verification_method === 'manual_override' ? (
                                            <Tag icon={<TeamOutlined />} color="purple">管理专家人工放行</Tag>
                                        ) : (
                                            <Tag icon={<CloudSyncOutlined />} color="cyan">AI 自动核验</Tag>
                                        )}
                                        {project.latestVerification.final_decision === 'BLOCK' && (
                                            <Button danger type="dashed" size="small" icon={<ToolOutlined />} onClick={onForceUnlock}>
                                                申请强制解锁 (Override)
                                            </Button>
                                        )}
                                    </Space>
                                    {(project.step4_override_enabled === 1 || project.override_enabled === 1) && (
                                        <div style={{ marginTop: 12 }}>
                                            <Alert
                                                message="已解锁放行"
                                                description={<Text type="secondary">理由: {project.step4_override_reason || project.override_reason || '未填写'} (由 {project.step4_override_by || project.operator_name || '管理专家'} 于 {project.step4_override_at ? new Date(project.step4_override_at).toLocaleString() : (project.override_at ? new Date(project.override_at).toLocaleString() : '最近')} 确认)</Text>}
                                                type="info"
                                                showIcon
                                                icon={<SafetyCertificateOutlined />}
                                            />
                                        </div>
                                    )}
                                </Col>
                                <Col span={5} style={{ borderLeft: '1px solid #e8e8e8', textAlign: 'center' }}>
                                    <Statistic title="最后审计覆盖率" value={project.coverage_score || 0} precision={1} suffix="%" />
                                    <Button type="link" size="small" style={{ marginTop: 8 }} onClick={onOpenFactsDrawer}>追溯事实依据</Button>
                                </Col>
                            </Row>
                        </Card>
                    ) : project.latestAudit && (
                        <Card bordered={false} style={{ background: project.latestAudit.final_decision === 'PASS' ? '#f6ffed' : project.latestAudit.final_decision === 'BLOCK' ? '#fff1f0' : '#fff7e6', borderRadius: 12 }} className="shadow-sm">
                            <Row align="middle" gutter={24}>
                                <Col span={4}>
                                    <div style={{ textAlign: 'center' }}>
                                        {project.latestAudit.final_decision === 'PASS' ? (
                                            <CheckCircleOutlined style={{ fontSize: 64, color: '#52c41a' }} />
                                        ) : project.latestAudit.final_decision === 'BLOCK' ? (
                                            <WarningOutlined style={{ fontSize: 64, color: '#ff4d4f' }} />
                                        ) : (
                                            <BulbOutlined style={{ fontSize: 64, color: '#faad14' }} />
                                        )}
                                        <div style={{ marginTop: 8, fontWeight: 800, fontSize: 18 }}>
                                            {project.latestAudit.final_decision}
                                        </div>
                                    </div>
                                </Col>
                                <Col span={15}>
                                    <Title level={5}>初步审计结论：{project.latestAudit.final_decision === 'PASS' ? '完成无缝覆盖' : project.latestAudit.final_decision === 'BLOCK' ? '存在严重审计漏洞' : '建议补充事实响应'}</Title>
                                    <Paragraph>{project.verification_result || '等待专家签核...'}</Paragraph>
                                    <Space>
                                        {project.latestAudit.risk_level && (
                                            <Space>
                                                <Text type="secondary">风险等级:</Text>
                                                <Tag color={project.latestAudit.risk_level === 'HIGH' ? 'red' : 'green'}>{project.latestAudit.risk_level}</Tag>
                                            </Space>
                                        )}
                                    </Space>
                                </Col>
                                <Col span={5} style={{ borderLeft: '1px solid #e8e8e8', textAlign: 'center' }}>
                                    <Statistic title="结构响应度" value={project.coverage_score || 0} precision={1} suffix="%" />
                                </Col>
                            </Row>
                        </Card>
                    )}

                    {project.latestVerification ? (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
                            <Card title={<span><WarningOutlined style={{ color: '#ff4d4f' }} /> 终审核查结论 (Critical Issues)</span>} bordered={false} style={{ borderRadius: 12 }} className="shadow-sm">
                                <List
                                    dataSource={JSON.parse(project.latestVerification.critical_issues_json || '[]')}
                                    renderItem={(item: string) => (
                                        <List.Item>
                                            <List.Item.Meta
                                                avatar={<Badge status="error" />}
                                                title={<Text strong type="danger">{item}</Text>}
                                            />
                                        </List.Item>
                                    )}
                                    locale={{ emptyText: '未发现致命合规项' }}
                                />
                            </Card>
                            <Row gutter={24}>
                                <Col span={12}>
                                    <Card title="主要改进点 (Major Issues)" bordered={false} style={{ borderRadius: 12, height: '100%' }} className="shadow-sm">
                                        <List
                                            size="small"
                                            dataSource={JSON.parse(project.latestVerification.major_issues_json || '[]')}
                                            renderItem={(item: string) => (
                                                <List.Item>
                                                    <Text type="secondary"><Badge status="warning" style={{ marginRight: 8 }} />{item}</Text>
                                                </List.Item>
                                            )}
                                        />
                                    </Card>
                                </Col>
                                <Col span={12}>
                                    <Card title="调整行动建议" bordered={false} style={{ borderRadius: 12, height: '100%' }} className="shadow-sm">
                                        <List
                                            size="small"
                                            dataSource={JSON.parse(project.latestVerification.suggested_actions_json || '[]')}
                                            renderItem={(item: string) => (
                                                <List.Item>
                                                    <Text><CheckCircleOutlined style={{ color: '#1890ff', marginRight: 8 }} />{item}</Text>
                                                </List.Item>
                                            )}
                                        />
                                    </Card>
                                </Col>
                            </Row>
                        </div>
                    ) : project.latestAudit && (
                        <Card title="初步审计 Gap 分析 (Step 4 Evidence)" bordered={false} style={{ borderRadius: 12 }} className="shadow-sm">
                            <Tabs defaultActiveKey="missing">
                                <Tabs.TabPane tab={<Badge count={JSON.parse(project.latestAudit.missing_items_json || '[]').length} offset={[10, 0]} color="red"><span>严重缺失项</span></Badge>} key="missing">
                                    <List
                                        dataSource={JSON.parse(project.latestAudit.missing_items_json || '[]')}
                                        renderItem={(item: unknown) => {
                                            const i = item as { requirement_id?: string; description?: string };
                                            return (
                                                <List.Item>
                                                    <List.Item.Meta
                                                        avatar={<Badge status="error" />}
                                                        title={<Space><Text strong>{i.requirement_id || '未知ID'}</Text><Tag color="red">高优先级</Tag></Space>}
                                                        description={<Text type="secondary">{i.description}</Text>}
                                                    />
                                                </List.Item>
                                            );
                                        }}
                                    />
                                </Tabs.TabPane>
                                <Tabs.TabPane tab={<Badge count={JSON.parse(project.latestAudit.weak_items_json || '[]').length} offset={[10, 0]} color="orange"><span>响应薄弱项</span></Badge>} key="weak">
                                    <List
                                        dataSource={JSON.parse(project.latestAudit.weak_items_json || '[]')}
                                        renderItem={(item: unknown) => {
                                            const i = item as { requirement_id?: string; description?: string };
                                            return (
                                                <List.Item>
                                                    <List.Item.Meta
                                                        avatar={<Badge status="warning" />}
                                                        title={<Space><Text strong>{i.requirement_id || '未知ID'}</Text><Tag color="orange">中优先级</Tag></Space>}
                                                        description={<Text type="secondary">{i.description}</Text>}
                                                    />
                                                </List.Item>
                                            );
                                        }}
                                    />
                                </Tabs.TabPane>
                            </Tabs>
                        </Card>
                    )}
                </div>
            ) : (
                <div style={{ textAlign: 'center', padding: '100px 0', background: '#fff', borderRadius: 12 }}>
                    <SafetyCertificateOutlined style={{ fontSize: 64, color: '#bae7ff', marginBottom: 24 }} />
                    <Title level={4}>待开启检查核验环节</Title>
                    <Paragraph type="secondary">本环节将采用豆包大模型对当前目录进行深度审计。您也可以直接进行人工核验。</Paragraph>
                    <Space size="middle">
                        <Button type="primary" size="large" icon={<PlayCircleOutlined />} onClick={onRunVerification} loading={verifying}>
                            启动 AI 终审核查
                        </Button>
                        <Button size="large" icon={<EditOutlined />} onClick={onStartManualAudit}>
                            直接开始人工核验
                        </Button>
                    </Space>
                </div>
            )}
        </div>
    );
};

export default OutlineVerificationPanel;
