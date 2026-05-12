import React, { useMemo } from 'react';
import {
    Card, Tabs, Table, Tag, Space, Typography, Progress, Row, Col, Statistic, Empty, Spin, Tooltip, Alert, Button, Drawer,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
    ApartmentOutlined,
    PieChartOutlined,
    BranchesOutlined,
    SafetyCertificateOutlined,
    FileProtectOutlined,
    UnorderedListOutlined,
    WarningOutlined,
    DatabaseOutlined,
} from '@ant-design/icons';
import type { Step4FactMapping, Step4Coverage, Step4RequirementRow, Step4FullResponse, Step4ConflictAudit, Step4Conflict, Step4FactCandidate, OutlineVersionDetailResponse, OutlineVersionRow } from '../api/techBidStep4';

const { Text, Paragraph } = Typography;

const countOutlineLevels = (nodes?: Array<Record<string, any>> | null) => {
    const list = Array.isArray(nodes) ? nodes : [];
    return {
        chapters: list.filter((n) => n?.node_level === 1).length,
        units: list.filter((n) => n?.node_level === 2).length,
        sections: list.filter((n) => n?.node_level === 3).length,
        total: list.length,
    };
};

const factTypeColor: Record<string, string> = {
    score_item: 'blue',
    mandatory_spec: 'volcano',
    project_characteristic: 'geekblue',
    special_topic: 'purple',
};

const factTypeLabel: Record<string, string> = {
    score_item: '评分项',
    mandatory_spec: '强制规范',
    project_characteristic: '项目特征',
    special_topic: '专项主题',
};

const priorityColor = (p: string) => {
    const x = (p || '').toLowerCase();
    if (x === 'high') return 'red';
    if (x === 'low') return 'default';
    return 'gold';
};

const resultColor = (r: string) => {
    if (r === 'PASS') return 'success';
    if (r === 'BLOCK') return 'error';
    if (r === 'REVISE') return 'warning';
    return 'default';
};

const mergeGateStatus = (coverageResult?: string | null, fullResponseResult?: string | null) => {
    const values = [coverageResult, fullResponseResult].filter(Boolean) as string[];
    if (values.includes('BLOCK')) return 'BLOCK';
    if (values.includes('REVISE')) return 'REVISE';
    if (values.includes('PASS') && values.length > 0 && values.every((v) => v === 'PASS')) return 'PASS';
    return 'UNKNOWN';
};

const tierLabel: Record<string, string> = {
    must_standalone: '须独立小节',
    mergeable: '可合并',
    background: '背景支撑',
};

export interface Step4MappingCoveragePanelProps {
    loading: boolean;
    mappings: Step4FactMapping[];
    coverage: Step4Coverage | null;
    /** 招标要求总表（可选，与后端 tech_bid_requirement_register 一致） */
    requirements?: Step4RequirementRow[];
    /** 完全响应率校验（可选） */
    fullResponse?: Step4FullResponse | null;
    /** 逻辑冲突审计结果（可选） */
    conflictAudit?: Step4ConflictAudit | null;
    factCandidates?: Step4FactCandidate[];
    outlineVersions?: OutlineVersionRow[];
    selectedVersionId?: string | null;
    selectedVersionDetail?: OutlineVersionDetailResponse | null;
    recommendedVersionDetail?: OutlineVersionDetailResponse | null;
    compact?: boolean;
    onViewDetail?: () => void;
    highlightFactId?: string | null;
    drawerOpen?: boolean;
    onDrawerClose?: () => void;
    gateStatus?: string | null;
    gateReason?: string | null;
    onOpenDrawer?: () => void;
}

const Step4MappingCoveragePanel: React.FC<Step4MappingCoveragePanelProps> = ({
    loading,
    mappings = [],
    coverage = null,
    requirements = [],
    fullResponse = null,
    conflictAudit = null,
    factCandidates = [],
    outlineVersions = [],
    selectedVersionId = null,
    selectedVersionDetail = null,
    recommendedVersionDetail = null,
    compact = false,
    highlightFactId = null,
    drawerOpen = false,
    onDrawerClose,
    gateStatus: propGateStatus = null,
    gateReason = null,
    onOpenDrawer,
}) => {
    const derivedGateStatus = mergeGateStatus(coverage?.result, fullResponse?.result);
    const finalGateStatus = propGateStatus || derivedGateStatus;
    
    // Use internal state if not controlled externally
    const [internalDrawerOpen, setInternalDrawerOpen] = React.useState(false);
    const [activeTab, setActiveTab] = React.useState<string>('coverage');

    const isShowingDrawer = onDrawerClose ? drawerOpen : internalDrawerOpen;

    // Aggressively switch to mappings tab when highlight context is provided
    React.useEffect(() => {
        if (isShowingDrawer && highlightFactId) {
            setActiveTab('mappings');
        }
    }, [isShowingDrawer, highlightFactId]);

    // Scroll highlighted row into view
    React.useEffect(() => {
        if (highlightFactId && isShowingDrawer && activeTab === 'mappings') {
            const timer = setTimeout(() => {
                const row = document.querySelector(`.step4-mapping-highlight-row`);
                if (row) {
                    row.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }
            }, 300); // Wait for Tab transition and Table render
            return () => clearTimeout(timer);
        }
    }, [highlightFactId, isShowingDrawer, activeTab]);
    const toggleDrawer = (open: boolean) => {
        if (open) {
            if (onOpenDrawer) {
                onOpenDrawer();
            } else {
                setInternalDrawerOpen(true);
            }
        } else {
            if (onDrawerClose) {
                onDrawerClose();
            } else {
                setInternalDrawerOpen(false);
            }
        }
    };
    const mappingColumns: ColumnsType<Step4FactMapping> = useMemo(
        () => [
            {
                title: '事实 ID',
                dataIndex: 'fact_id',
                width: 100,
                fixed: 'left',
                render: (v: string) => <Text code>{v}</Text>,
            },
            {
                title: '类型',
                dataIndex: 'fact_type',
                width: 108,
                render: (t: string) => (
                    <Tag color={factTypeColor[t] || 'default'}>{factTypeLabel[t] || t}</Tag>
                ),
            },
            {
                title: '名称',
                dataIndex: 'fact_name',
                ellipsis: true,
                render: (v: string) => (
                    <Tooltip title={v}>
                        <span>{v || '—'}</span>
                    </Tooltip>
                ),
            },
            {
                title: '溯源 (章节/页码)',
                width: 180,
                render: (_, r) => (
                    <Tooltip title={r.source_location || '无位置信息'}>
                        <Space direction="vertical" size={0}>
                            <Text type="secondary" style={{ fontSize: 11 }}>
                                {r.source_chapter || '未知章节'}
                            </Text>
                            {r.page_number !== undefined && (
                                <Tag color="blue" style={{ fontSize: 10 }}>
                                    P{r.page_number}{r.line_number ? `:L${r.line_number}` : ''}
                                </Tag>
                            )}
                        </Space>
                    </Tooltip>
                ),
            },
            {
                title: '落点层级',
                dataIndex: 'target_level',
                width: 110,
                render: (v: string) => <Tag>{v || '—'}</Tag>,
            },
            {
                title: '目标路径',
                dataIndex: 'target_path',
                width: 280,
                render: (path: string[]) =>
                    path?.length ? (
                        <Text type="secondary" style={{ fontSize: 13 }}>
                            {path.map((p, i) => (
                                <span key={i}>
                                    {i > 0 && <span style={{ color: '#94a3b8', margin: '0 4px' }}>›</span>}
                                    <span style={{ color: '#334155' }}>{p}</span>
                                </span>
                            ))}
                        </Text>
                    ) : (
                        '—'
                    ),
            },
            {
                title: '优先级',
                dataIndex: 'priority',
                width: 88,
                render: (p: string) => <Tag color={priorityColor(p)}>{p || '—'}</Tag>,
            },
            {
                title: '必选',
                dataIndex: 'required',
                width: 72,
                render: (v: boolean) => (v ? <Tag color="red">必选</Tag> : <Text type="secondary">—</Text>),
            },
        ],
        []
    );

    const requirementColumns: ColumnsType<Step4RequirementRow> = useMemo(
        () => [
            {
                title: 'ID',
                dataIndex: 'requirement_id',
                width: 88,
                fixed: 'left',
                render: (v: string) => <Text code>{v}</Text>,
            },
            {
                title: '类型',
                dataIndex: 'requirement_type',
                width: 100,
                render: (t: string) => <Tag>{t || '—'}</Tag>,
            },
            {
                title: '分级',
                dataIndex: 'response_tier',
                width: 108,
                render: (t: string) => <Tag color="geekblue">{tierLabel[t] || t || '—'}</Tag>,
            },
            {
                title: '摘要',
                dataIndex: 'summary',
                ellipsis: true,
                render: (v: string, row: Step4RequirementRow) => (
                    <Tooltip title={row.source_text || v}>
                        <span>{v || '—'}</span>
                    </Tooltip>
                ),
            },
            {
                title: '优先级',
                dataIndex: 'priority',
                width: 88,
                render: (p: string) => <Tag color={priorityColor(p)}>{p || '—'}</Tag>,
            },
        ],
        []
    );

    const conflictColumns: ColumnsType<Step4Conflict> = useMemo(
        () => [
            {
                title: '类型',
                dataIndex: 'conflict_type',
                width: 120,
                render: (v: string) => <Tag color="volcano">{v}</Tag>,
            },
            {
                title: '字段/维度',
                dataIndex: 'field_name',
                width: 120,
                render: (v: string) => <Text strong>{v}</Text>,
            },
            {
                title: '冲突详情',
                dataIndex: 'description',
                render: (v: string, r) => (
                    <Space direction="vertical" size={4}>
                        <Text>{v}</Text>
                        <div style={{ fontSize: 12 }}>
                            <Tag color="default">来源 A: {r.source_a}</Tag>
                            <Tag color="default">来源 B: {r.source_b}</Tag>
                        </div>
                    </Space>
                ),
            },
            {
                title: '严重程度',
                dataIndex: 'severity',
                width: 100,
                render: (v: string) => <Tag color={v === 'high' ? 'red' : v === 'medium' ? 'orange' : 'blue'}>{v.toUpperCase()}</Tag>,
            },
        ],
        []
    );

    const candidateColumns: ColumnsType<Step4FactCandidate> = useMemo(
        () => [
            {
                title: '事实',
                dataIndex: 'fact_name',
                ellipsis: true,
                render: (v: string, row) => (
                    <Space direction="vertical" size={0}>
                        <Text strong>{v || row.fact_id}</Text>
                        <Text code style={{ fontSize: 11 }}>{row.fact_id}</Text>
                    </Space>
                ),
            },
            {
                title: '类型',
                dataIndex: 'fact_type',
                width: 112,
                render: (v: string) => <Tag color={factTypeColor[v] || 'blue'}>{factTypeLabel[v] || v || '—'}</Tag>,
            },
            {
                title: '来源',
                dataIndex: 'source_location',
                ellipsis: true,
                render: (v: string | null | undefined, row) => v || row.source_library || '—',
            },
            {
                title: '置信度',
                dataIndex: 'confidence_score',
                width: 96,
                render: (v: number) => <Tag color={v >= 0.8 ? 'green' : v >= 0.6 ? 'gold' : 'red'}>{Math.round((v || 0) * 100)}%</Tag>,
            },
        ],
        []
    );

    const versionColumns: ColumnsType<OutlineVersionRow> = useMemo(
        () => [
            { title: '版本', dataIndex: 'version_no', width: 76, render: (v: number) => <Text strong>{v}</Text> },
            { title: '状态', dataIndex: 'status', width: 110, render: (v: string) => <Tag color={v === 'recommended' ? 'green' : 'default'}>{v}</Tag> },
            { title: '来源', dataIndex: 'version_source', width: 120, render: (v: string) => <Tag>{v || '—'}</Tag> },
            { title: '说明', dataIndex: 'rationale', ellipsis: true },
            { title: '创建人', dataIndex: 'created_by', width: 100, render: (v: string) => v || '—' },
        ],
        []
    );

    const coverageTab = coverage ? (
        <div className="step4-coverage-inner">
            <Row gutter={[20, 20]} align="middle">
                <Col xs={24} sm={8} style={{ textAlign: 'center' }}>
                    <Progress
                        type="dashboard"
                        percent={Math.min(100, Math.round(coverage.coverage_rate * 10) / 10)}
                        strokeColor={{
                            '0%': '#3b82f6',
                            '100%': '#22c55e',
                        }}
                        gapDegree={4}
                        size={compact ? 120 : 140}
                    />
                    <div style={{ marginTop: 8 }}>
                        <Tag color={resultColor(coverage.result)} style={{ fontSize: 13, padding: '2px 10px' }}>
                            结构化 {coverage.result}
                        </Tag>
                    </div>
                </Col>
                <Col xs={24} sm={16}>
                    <Row gutter={16}>
                        <Col span={12}>
                            <Statistic title="已映射 / 事实总数" value={coverage.fact_mapped} suffix={`/ ${coverage.fact_total}`} />
                        </Col>
                        <Col span={12}>
                            <Statistic
                                title="覆盖率"
                                value={coverage.coverage_rate}
                                precision={1}
                                suffix="%"
                                valueStyle={{ color: '#2563eb' }}
                            />
                        </Col>
                    </Row>
                    {coverage.summary && (
                        <Alert
                            style={{ marginTop: 16 }}
                            type={coverage.result === 'PASS' ? 'success' : coverage.result === 'BLOCK' ? 'error' : 'warning'}
                            showIcon
                            message="校验摘要"
                            description={<Paragraph style={{ marginBottom: 0 }}>{coverage.summary}</Paragraph>}
                        />
                    )}
                    {(coverage.missing_fact_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 16 }}>
                            <Text type="secondary">未进入目录的事实 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {coverage.missing_fact_ids.map((id) => (
                                        <Tag key={id} color="error">
                                            {id}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(coverage.weak_fact_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">高优先级待补强：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {coverage.weak_fact_ids.map((id) => (
                                        <Tag key={id} color="orange">
                                            {id}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(coverage.duplicate_node_hints?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">结构提示：</Text>
                            <ul style={{ margin: '8px 0 0', paddingLeft: 18, color: '#64748b', fontSize: 13 }}>
                                {coverage.duplicate_node_hints.map((h, i) => (
                                    <li key={i}>{h}</li>
                                ))}
                            </ul>
                        </div>
                    )}
                </Col>
            </Row>
        </div>
    ) : (
        <Empty description="暂无结构化覆盖率记录（目录生成完成后将写入）" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const fullResponseTab = fullResponse ? (
        <div className="step4-coverage-inner">
            <Row gutter={[20, 20]} align="middle">
                <Col xs={24} sm={8} style={{ textAlign: 'center' }}>
                    <Progress
                        type="dashboard"
                        percent={Math.min(100, Math.round(fullResponse.full_response_rate * 10) / 10)}
                        strokeColor={{
                            '0%': '#7c3aed',
                            '100%': '#22c55e',
                        }}
                        gapDegree={4}
                        size={compact ? 120 : 140}
                    />
                    <div style={{ marginTop: 8 }}>
                        <Tag color={resultColor(fullResponse.result)} style={{ fontSize: 13, padding: '2px 10px' }}>
                            完全响应 {fullResponse.result}
                        </Tag>
                    </div>
                </Col>
                <Col xs={24} sm={16}>
                    <Row gutter={16}>
                        <Col span={8}>
                            <Statistic
                                title="充分 / 总要求"
                                value={fullResponse.requirement_fully_responded}
                                suffix={`/ ${fullResponse.requirement_total}`}
                            />
                        </Col>
                        <Col span={8}>
                            <Statistic
                                title="完全响应率"
                                value={fullResponse.full_response_rate}
                                precision={1}
                                suffix="%"
                                valueStyle={{ color: '#7c3aed' }}
                            />
                        </Col>
                        <Col span={8}>
                            <Statistic
                                title="质量分"
                                value={fullResponse.response_quality_score}
                                precision={1}
                                suffix="/ 100"
                                valueStyle={{ color: '#0d9488' }}
                            />
                        </Col>
                    </Row>
                    <Row gutter={16} style={{ marginTop: 8 }}>
                        <Col span={8}>
                            <Statistic title="弱响应" value={fullResponse.requirement_weakly_responded} />
                        </Col>
                        <Col span={8}>
                            <Statistic title="仅挂 ID" value={fullResponse.requirement_only_tagged} />
                        </Col>
                        <Col span={8}>
                            <Statistic
                                title="弱响应率"
                                value={fullResponse.weak_response_rate}
                                precision={1}
                                suffix="%"
                            />
                        </Col>
                    </Row>
                    {fullResponse.summary && (
                        <Alert
                            style={{ marginTop: 16 }}
                            type={
                                fullResponse.result === 'PASS'
                                    ? 'success'
                                    : fullResponse.result === 'BLOCK'
                                      ? 'error'
                                      : 'warning'
                            }
                            showIcon
                            message="完全响应校验摘要"
                            description={<Paragraph style={{ marginBottom: 0 }}>{fullResponse.summary}</Paragraph>}
                        />
                    )}
                    {(fullResponse.missing_requirement_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 16 }}>
                            <Text type="secondary">未响应要求 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {fullResponse.missing_requirement_ids.map((rid) => (
                                        <Tag key={rid} color="error">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.high_priority_missing_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">高优先级缺失 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {(fullResponse.high_priority_missing_ids || []).map((rid) => (
                                        <Tag key={rid} color="magenta">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.mandatory_missing_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">强制规范未映射 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {(fullResponse.mandatory_missing_ids || []).map((rid) => (
                                        <Tag key={rid} color="volcano">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.mandatory_insufficient_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">强制规范未充分展开 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {(fullResponse.mandatory_insufficient_ids || []).map((rid) => (
                                        <Tag key={rid} color="purple">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.weak_requirement_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">弱响应要求 ID：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {fullResponse.weak_requirement_ids.map((rid) => (
                                        <Tag key={rid} color="orange">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.only_tagged_requirement_ids?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">仅挂标签（标题未体现语义）：</Text>
                            <div style={{ marginTop: 8 }}>
                                <Space wrap size={[4, 8]}>
                                    {fullResponse.only_tagged_requirement_ids.map((rid) => (
                                        <Tag key={rid} color="default">
                                            {rid}
                                        </Tag>
                                    ))}
                                </Space>
                            </div>
                        </div>
                    )}
                    {(fullResponse.shell_title_hints?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">疑似空壳/泛标题小节：</Text>
                            <ul style={{ margin: '8px 0 0', paddingLeft: 18, color: '#64748b', fontSize: 13 }}>
                                {fullResponse.shell_title_hints.map((h, i) => (
                                    <li key={i}>{h}</li>
                                ))}
                            </ul>
                        </div>
                    )}
                    {(fullResponse.hard_rule_warnings?.length ?? 0) > 0 && (
                        <div style={{ marginTop: 12 }}>
                            <Text type="secondary">行业硬规则提示：</Text>
                            <ul style={{ margin: '8px 0 0', paddingLeft: 18, color: '#b45309', fontSize: 13 }}>
                                {(fullResponse.hard_rule_warnings ?? []).map((h, i) => (
                                    <li key={i}>{h}</li>
                                ))}
                            </ul>
                        </div>
                    )}
                </Col>
            </Row>
            {requirements.length > 0 && (
                <div style={{ marginTop: 24 }}>
                    <Text strong>
                        <UnorderedListOutlined style={{ marginRight: 8 }} />
                        招标要求总表
                    </Text>
                    <Table<Step4RequirementRow>
                        style={{ marginTop: 12 }}
                        size="small"
                        rowKey="id"
                        columns={requirementColumns}
                        dataSource={requirements}
                        pagination={requirements.length > 8 ? { pageSize: 8, showSizeChanger: false } : false}
                        scroll={{ x: 720 }}
                    />
                </div>
            )}
        </div>
    ) : (
        <Empty description="暂无完全响应率记录（目录生成完成后将写入）" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const conflictTab = conflictAudit ? (
        <div className="step4-coverage-inner">
            <Alert
                type={conflictAudit.has_block ? 'error' : 'warning'}
                showIcon
                message={conflictAudit.has_block ? '发现严重逻辑冲突' : '冲突审计摘要'}
                description={conflictAudit.summary || '未发现显著逻辑冲突'}
                style={{ marginBottom: 20 }}
            />
            <Table<Step4Conflict>
                size="small"
                rowKey="conflict_id"
                columns={conflictColumns}
                dataSource={conflictAudit.conflicts}
                pagination={false}
            />
        </div>
    ) : (
        <Empty description="暂无逻辑冲突审计记录" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const factCandidatesTab = factCandidates.length > 0 ? (
        <div className="step4-coverage-inner">
            <Alert
                type="info"
                showIcon
                message="事实候选池"
                description="展示 Step4 Coordinator 从资料库/解析结果中提取出的待映射事实候选。"
                style={{ marginBottom: 20 }}
            />
            <Table<Step4FactCandidate>
                size="small"
                rowKey="id"
                columns={candidateColumns}
                dataSource={factCandidates}
                pagination={factCandidates.length > 8 ? { pageSize: 8, showSizeChanger: false } : false}
                scroll={{ x: 720 }}
            />
        </div>
    ) : (
        <Empty description="暂无事实候选记录" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const versionDetailTab = selectedVersionDetail ? (
        <div className="step4-coverage-inner">
            <Alert
                type="success"
                showIcon
                message={selectedVersionId ? `目录版本 ${selectedVersionId}` : '目录版本详情'}
                description={`共 ${selectedVersionDetail.nodes?.length || 0} 个节点，已按服务端快照重建。`}
                style={{ marginBottom: 20 }}
            />
            <Table
                size="small"
                rowKey={(row: any) => row.id || `${row.node_name}-${row.node_order}`}
                dataSource={selectedVersionDetail.nodes}
                pagination={false}
                columns={[
                    { title: '层级', dataIndex: 'node_level', width: 72 },
                    { title: '名称', dataIndex: 'node_name', ellipsis: true },
                    { title: '顺序', dataIndex: 'node_order', width: 72 },
                    {
                        title: '关联要求',
                        dataIndex: 'linked_requirement_ids_json',
                        ellipsis: true,
                        render: (v: string | null | undefined) => v || '—',
                    },
                ]}
            />
        </div>
    ) : (
        <Empty description="请选择一个目录版本查看详情" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const selectedVersionSummary = countOutlineLevels(selectedVersionDetail?.nodes ?? null);
    const recommendedVersionSummary = countOutlineLevels(recommendedVersionDetail?.nodes ?? null);
    const versionDiffTab = outlineVersions.length >= 2 ? (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
            <Alert
                type="info"
                showIcon
                message="目录版本对比"
                description="对比当前选择版本与推荐版本的结构规模，先看差异，再决定是否同步。"
            />
            <Row gutter={16}>
                <Col span={12}>
                    <Card size="small" title="推荐版本" bordered={false}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                            <Space>
                                <Text strong>版本 {outlineVersions.find((v) => v.status === 'recommended')?.version_no ?? '—'}</Text>
                                <Tag color="green">Recommended</Tag>
                            </Space>
                            <Text type="secondary">{recommendedVersionDetail ? `${recommendedVersionSummary.total} 个节点` : '暂无详情'}</Text>
                            <Space wrap>
                                <Tag color="blue">章 {recommendedVersionSummary.chapters}</Tag>
                                <Tag color="geekblue">节 {recommendedVersionSummary.units}</Tag>
                                <Tag color="purple">小节 {recommendedVersionSummary.sections}</Tag>
                            </Space>
                        </Space>
                    </Card>
                </Col>
                <Col span={12}>
                    <Card size="small" title="当前选择版本" bordered={false}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                            <Space>
                                <Text strong>版本 {outlineVersions.find((v) => v.id === selectedVersionId)?.version_no ?? '—'}</Text>
                                <Tag color={selectedVersionDetail ? 'blue' : 'default'}>{selectedVersionDetail ? 'Selected' : 'Pending'}</Tag>
                            </Space>
                            <Text type="secondary">{selectedVersionDetail ? `${selectedVersionSummary.total} 个节点` : '请选择版本查看详情'}</Text>
                            <Space wrap>
                                <Tag color="blue">章 {selectedVersionSummary.chapters}</Tag>
                                <Tag color="geekblue">节 {selectedVersionSummary.units}</Tag>
                                <Tag color="purple">小节 {selectedVersionSummary.sections}</Tag>
                            </Space>
                        </Space>
                    </Card>
                </Col>
            </Row>
            <Row gutter={16}>
                <Col span={8}>
                    <Card size="small" title="节点总数差异" bordered={false}>
                        <Statistic
                            value={selectedVersionSummary.total - recommendedVersionSummary.total}
                            precision={0}
                            valueStyle={{ color: selectedVersionSummary.total >= recommendedVersionSummary.total ? '#16a34a' : '#dc2626' }}
                            suffix="个节点"
                        />
                    </Card>
                </Col>
                <Col span={8}>
                    <Card size="small" title="章节层差异" bordered={false}>
                        <Statistic
                            value={selectedVersionSummary.chapters - recommendedVersionSummary.chapters}
                            precision={0}
                            valueStyle={{ color: '#2563eb' }}
                            suffix="章"
                        />
                    </Card>
                </Col>
                <Col span={8}>
                    <Card size="small" title="小节层差异" bordered={false}>
                        <Statistic
                            value={selectedVersionSummary.sections - recommendedVersionSummary.sections}
                            precision={0}
                            valueStyle={{ color: '#7c3aed' }}
                            suffix="小节"
                        />
                    </Card>
                </Col>
            </Row>
        </Space>
    ) : (
        <Empty description="暂无足够版本进行对比" image={Empty.PRESENTED_IMAGE_SIMPLE} />
    );

    const mappingTab =
        mappings.length === 0 && !loading ? (
            <Empty description="暂无映射数据（生成完成后将展示每条事实的目录落点）" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
            <Table<Step4FactMapping>
                size="small"
                rowKey="fact_id"
                loading={loading}
                columns={mappingColumns}
                dataSource={mappings}
                pagination={mappings.length > 12 ? { pageSize: 12, showSizeChanger: false } : false}
                scroll={{ x: 1100 }}
                rowClassName={(record) => 
                    record.fact_id === highlightFactId ? 'step4-mapping-highlight-row' : ''
                }
            />
        );

    const tabItems = [
        {
            key: 'mappings',
            label: (
                <span>
                    <BranchesOutlined /> 事实映射 (溯源)
                    {mappings.length > 0 && (
                        <Tag style={{ marginLeft: 8 }}>{mappings.length}</Tag>
                    )}
                </span>
            ),
            children: mappingTab,
        },
        {
            key: 'candidates',
            label: (
                <span>
                    <DatabaseOutlined /> 事实候选
                    {factCandidates.length > 0 && (
                        <Tag color="blue" style={{ marginLeft: 8 }}>{factCandidates.length}</Tag>
                    )}
                </span>
            ),
            children: factCandidatesTab,
        },
        {
            key: 'coverage',
            label: (
                <span>
                    <PieChartOutlined /> 映射覆盖率
                    {coverage && (
                        <Tag color={resultColor(coverage.result)} style={{ marginLeft: 8 }}>
                            {coverage.result}
                        </Tag>
                    )}
                </span>
            ),
            children: coverageTab,
        },
        {
            key: 'full_response',
            label: (
                <span>
                    <FileProtectOutlined /> 完全响应 (要求总表)
                    {fullResponse && (
                        <Tag color={resultColor(fullResponse.result)} style={{ marginLeft: 8 }}>
                            {fullResponse.result}
                        </Tag>
                    )}
                </span>
            ),
            children: fullResponseTab,
        },
        {
            key: 'conflicts',
            label: (
                <span>
                    <SafetyCertificateOutlined /> 逻辑冲突审计
                    {conflictAudit && conflictAudit.conflicts?.length > 0 && (
                        <Tag color="red" style={{ marginLeft: 8 }}>{conflictAudit.conflicts.length}</Tag>
                    )}
                </span>
            ),
            children: conflictTab,
        },
        {
            key: 'versions',
            label: (
                <span>
                    <UnorderedListOutlined /> 目录版本
                    {outlineVersions.length > 0 && (
                        <Tag style={{ marginLeft: 8 }}>{outlineVersions.length}</Tag>
                    )}
                </span>
            ),
            children: (
                <Tabs
                    size="small"
                    items={[
                        {
                            key: 'version-list',
                            label: '版本列表',
                            children: (
                                <Space direction="vertical" style={{ width: '100%' }} size="large">
                                    <Table<OutlineVersionRow>
                                        size="small"
                                        rowKey="id"
                                        columns={versionColumns}
                                        dataSource={outlineVersions}
                                        pagination={false}
                                    />
                                    {versionDetailTab}
                                </Space>
                            ),
                        },
                        {
                            key: 'version-diff',
                            label: '版本对比',
                            children: versionDiffTab,
                        },
                    ]}
                />
            ),
        },
    ];

    if (compact) {
        const borderColors = {
            'PASS': '#22c55e',
            'BLOCK': '#ff4d4f',
            'REVISE': '#faad14',
            'UNKNOWN': '#d1d5db'
        };

        return (
            <div className="step4-audit-hud">
                <Card 
                    size="small" 
                    bordered={false} 
                    style={{ borderTop: `3px solid ${borderColors[finalGateStatus as keyof typeof borderColors] || '#d1d5db'}` }}
                    title={<Text strong style={{ fontSize: 13 }}><ApartmentOutlined /> 审计健康看板</Text>} 
                    extra={<Button type="link" size="small" onClick={() => toggleDrawer(true)}>明细</Button>}
                >
                    <Space direction="vertical" style={{ width: '100%' }} size={16}>
                        <Row gutter={12}>
                            <Col span={12}>
                                <div style={{ textAlign: 'center' }}>
                                    <Progress type="circle" percent={Math.round((coverage?.coverage_rate || 0))} size={45} strokeWidth={8} />
                                    <div style={{ fontSize: 10, marginTop: 4, color: '#64748b' }}>映射率</div>
                                </div>
                            </Col>
                            <Col span={12}>
                                <div style={{ textAlign: 'center' }}>
                                    <Progress type="circle" percent={Math.round((fullResponse?.full_response_rate || 0))} size={45} strokeWidth={8} strokeColor="#7c3aed" />
                                    <div style={{ fontSize: 10, marginTop: 4, color: '#64748b' }}>响应率</div>
                                </div>
                            </Col>
                        </Row>
                        
                        <div className="audit-signals">
                            <Space direction="vertical" size={8} style={{ width: '100%' }}>
                                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                                    <Text type="secondary">门禁状态</Text>
                                    <Tag color={resultColor(finalGateStatus)} style={{ margin: 0 }}>{finalGateStatus}</Tag>
                                </div>
                                {conflictAudit && conflictAudit.conflicts?.length > 0 && (
                                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                                        <Text type="secondary">逻辑冲突</Text>
                                        <Tag color="red" style={{ margin: 0 }}>{conflictAudit.conflicts.length}项</Tag>
                                    </div>
                                )}
                            </Space>
                        </div>

                        {finalGateStatus !== 'PASS' && gateReason && (
                            <div style={{ 
                                background: finalGateStatus === 'BLOCK' ? '#fff1f0' : '#fff7e6',
                                border: `1px solid ${finalGateStatus === 'BLOCK' ? '#ffa39e' : '#ffe58f'}`,
                                borderRadius: 4,
                                padding: '8px 12px',
                                fontSize: 12,
                                display: 'flex',
                                gap: 8,
                                alignItems: 'flex-start'
                            }}>
                                <WarningOutlined style={{ color: finalGateStatus === 'BLOCK' ? '#ff4d4f' : '#faad14', marginTop: 3 }} />
                                <div style={{ lineHeight: 1.5, color: '#434343' }}>
                                    <Text strong style={{ fontSize: 11, display: 'block', marginBottom: 2, color: finalGateStatus === 'BLOCK' ? '#cf1322' : '#d48806' }}>
                                        {finalGateStatus === 'BLOCK' ? '生成质量预警' : '建议人工核查'}
                                    </Text>
                                    {gateReason}
                                </div>
                            </div>
                        )}

                        {finalGateStatus === 'PASS' && coverage?.summary && (
                            <Alert 
                                type="success" 
                                message={<div style={{ fontSize: 11 }}>自检通过</div>}
                                description={<div style={{ fontSize: 11, lineHeight: 1.4 }}>{coverage.summary}</div>}
                                style={{ padding: '8px' }}
                                showIcon
                            />
                        )}
                    </Space>
                </Card>

                <Drawer
                    title={<Space><SafetyCertificateOutlined /> 事实溯源与闭环审计报告</Space>}
                    placement="right"
                    width={950}
                    onClose={() => toggleDrawer(false)}
                    open={isShowingDrawer}
                    extra={
                        <Space>
                            <Tag color={resultColor(finalGateStatus)}>{finalGateStatus}</Tag>
                        </Space>
                    }
                >
                    <style>{`
                        .step4-mapping-highlight-row {
                            background-color: #fffbe6 !important;
                            border: 1px solid #ffe58f;
                        }
                        .step4-mapping-highlight-row td {
                            background-color: #fffbe6 !important;
                        }
                    `}</style>
                    {highlightFactId && (
                        <Alert 
                            type="info"
                            showIcon
                            message={<Text strong>正在溯源证据链</Text>}
                            description={`已为您定位并高亮 Fact ID: ${highlightFactId} 及其对应的目录落点。`}
                            style={{ marginBottom: 16 }}
                            closable
                        />
                    )}
                    <Tabs 
                        activeKey={activeTab} 
                        onChange={(key) => setActiveTab(key)}
                        items={tabItems} 
                        size="middle" 
                    />
                </Drawer>
            </div>
        );
    }

    return (
        <Card
            className="step4-artifacts-card"
            bordered={false}
            title={
                <Row justify="space-between" align="middle" style={{ width: '100%' }}>
                    <Col>
                        <Space>
                            <ApartmentOutlined style={{ color: '#4f46e5', fontSize: 18 }} />
                            <span style={{ fontWeight: 600 }}>事实溯源与闭环审计</span>
                            <Tooltip title="包含事实源文本溯源、招标要求映射覆盖率及逻辑冲突审计结果">
                                <SafetyCertificateOutlined style={{ color: '#94a3b8', fontSize: 14 }} />
                            </Tooltip>
                        </Space>
                    </Col>
                    <Col>
                        <Space>
                            <Text type="secondary">验证门禁状态:</Text>
                            <Tag color={resultColor(finalGateStatus)} style={{ fontWeight: 600 }}>
                                {finalGateStatus}
                            </Tag>
                        </Space>
                    </Col>
                </Row>
            }
        >
            <Spin spinning={loading}>
                <Tabs defaultActiveKey="mappings" items={tabItems} size="middle" />
            </Spin>
        </Card>
    );
};

export default Step4MappingCoveragePanel;
