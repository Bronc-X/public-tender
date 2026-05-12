import React, { useState, useEffect, useLayoutEffect, useCallback, useRef, useMemo } from 'react';
import { Table, Button, Space, Typography, Card, Modal, Form, Input, Select, message, Spin, Tag, Divider } from 'antd';
import KnowledgeExtractWizard, { type ExtractStartedPayload } from '../components/KnowledgeExtractWizard';
import {
    DeleteOutlined, EditOutlined, BookOutlined, CarOutlined, SafetyOutlined,
    HistoryOutlined, AlertOutlined, TrophyOutlined, TeamOutlined, SolutionOutlined,
    CloudUploadOutlined, FolderOutlined, RobotOutlined
} from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;
const { TextArea } = Input;

type PendingExtractPhase = 'running' | 'completed' | 'failed' | 'cancelled';

function pendingExtractsStorageKey(companyId: string) {
    return `bid_knowledge_pending_extracts_v1_${companyId}`;
}

function loadPendingExtractsFromStorage(companyId: string): PendingExtractRow[] {
    if (!companyId || typeof window === 'undefined') return [];
    try {
        const raw = sessionStorage.getItem(pendingExtractsStorageKey(companyId));
        if (!raw) return [];
        const parsed = JSON.parse(raw) as unknown;
        if (!Array.isArray(parsed)) return [];
        return parsed.filter(
            (x): x is PendingExtractRow =>
                x != null &&
                typeof x === 'object' &&
                typeof (x as PendingExtractRow).taskId === 'string' &&
                typeof (x as PendingExtractRow).title === 'string' &&
                ['running', 'completed', 'failed', 'cancelled'].includes((x as PendingExtractRow).phase)
        );
    } catch {
        return [];
    }
}

function savePendingExtractsToStorage(companyId: string, rows: PendingExtractRow[]) {
    if (!companyId || typeof window === 'undefined') return;
    try {
        sessionStorage.setItem(pendingExtractsStorageKey(companyId), JSON.stringify(rows));
    } catch {
        /* quota */
    }
}

interface PendingExtractRow {
    taskId: string;
    title: string;
    phase: PendingExtractPhase;
    errorMessage?: string;
    /** 发起提炼时所在知识库 type；旧会话可能缺省，按 method 处理 */
    knowledgeType?: string;
}

interface KnowledgeItem {
    id: string;
    item_type: string;
    item_name: string;
    item_content: string;
    tags_json: string;
    source_desc?: string;
    source_project_id?: string;
    source_file_id?: string;
    source_reference?: string;
    created_at: string;
    updated_at: string;
    /** 来自历史项目 AI 提炼入库时关联的任务 id */
    extract_task_id?: string;
    /** 列表中的异步提炼占位行 */
    _pendingExtract?: PendingExtractRow;
}

export interface KnowledgeSectionDisplayOverride {
    title: string;
    icon: React.ReactNode;
    itemLabel: string;
    subTitle: string;
}

const TechKnowledgeLibrary: React.FC<{
    type?: string;
    /** 自定义平级分类时的展示文案（内置库不需要传） */
    displayOverride?: KnowledgeSectionDisplayOverride;
}> = ({ type = 'method', displayOverride }) => {
    const { currentCompanyId } = useCompany();
    const [items, setItems] = useState<KnowledgeItem[]>([]);
    const [loading, setLoading] = useState(false);
    const [isModalVisible, setIsModalVisible] = useState(false);
    const [editingId, setEditingId] = useState<string | null>(null);
    const [editingRecord, setEditingRecord] = useState<KnowledgeItem | null>(null);
    const [form] = Form.useForm();
    const [detailVisible, setDetailVisible] = useState(false);
    const [detailRecord, setDetailRecord] = useState<KnowledgeItem | null>(null);
    const [extractWizardOpen, setExtractWizardOpen] = useState(false);
    const [resumeTaskId, setResumeTaskId] = useState<string | null>(null);
    const [pendingExtracts, setPendingExtracts] = useState<PendingExtractRow[]>(() =>
        loadPendingExtractsFromStorage(
            typeof window !== 'undefined' ? localStorage.getItem('current_company_id') || '' : ''
        )
    );
    const pendingRef = useRef(pendingExtracts);
    pendingRef.current = pendingExtracts;
    const companyIdRef = useRef(currentCompanyId);
    companyIdRef.current = currentCompanyId;

    /** 切换公司或从其它页面回到知识库（组件重挂载）时，同步恢复待确认提炼行 */
    useLayoutEffect(() => {
        if (!currentCompanyId) return;
        setPendingExtracts(loadPendingExtractsFromStorage(currentCompanyId));
    }, [currentCompanyId]);

    useEffect(() => {
        const cid = companyIdRef.current;
        if (!cid) return;
        savePendingExtractsToStorage(cid, pendingExtracts);
    }, [pendingExtracts]);
    /** 刚入库成功的提炼任务 id：同任务多条置顶 + 高亮 + 展开 */
    const [highlightExtractTaskId, setHighlightExtractTaskId] = useState<string | null>(null);

    const typeConfig: Record<string, { title: string; icon: React.ReactNode; itemLabel: string; subTitle: string }> = {
        method: { title: '工法库', icon: <BookOutlined />, itemLabel: '工法名称', subTitle: '沉淀企业标准化施工工艺与技术亮点' },
        equipment: { title: '设备库', icon: <CarOutlined />, itemLabel: '设备名称', subTitle: '管理企业机械设备能力与技术参数' },
        system: { title: '制度与规范库', icon: <SafetyOutlined />, itemLabel: '制度名称', subTitle: '支撑安全与质量标准化管理' },
        performance: { title: '业绩库（技术）', icon: <HistoryOutlined />, itemLabel: '项目名称', subTitle: '沉淀过往项目的技术难点与解决方案' },
        risks: { title: '风控库', icon: <AlertOutlined />, itemLabel: '风险场景', subTitle: '识别行业常见技术风险与规避对策' },
        regions: { title: '企业优势库', icon: <TrophyOutlined />, itemLabel: '优势条目名称', subTitle: '沉淀企业资质、业绩亮点、技术与品牌优势等' },
        subcontractors: { title: '分包库', icon: <TeamOutlined />, itemLabel: '单位名称', subTitle: '管理优质分包资源与劳务组织能力' },
        costs: { title: '风险应对措施库', icon: <SolutionOutlined />, itemLabel: '措施名称', subTitle: '针对各类风险的预案、应对流程与处置要点' },
    };

    const config =
        displayOverride ??
        typeConfig[type || 'method'] ?? {
            title: '知识库',
            icon: <FolderOutlined />,
            itemLabel: '条目名称',
            subTitle: '在此分类下管理知识条目',
        };

    const fetchItems = useCallback(async (opts?: { silent?: boolean }): Promise<KnowledgeItem[]> => {
        if (!currentCompanyId) return [];
        if (!opts?.silent) setLoading(true);
        try {
            const res = await axios.get(`/api/tech-bid/knowledge?type=${encodeURIComponent(type)}`, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            const data = res.data;
            const list = Array.isArray(data) ? data : [];
            setItems(list);
            return list;
        } catch (err) {
            console.error('Fetch error:', err);
            message.error('加载列表失败');
            return [];
        } finally {
            if (!opts?.silent) setLoading(false);
        }
    }, [currentCompanyId, type]);

    useEffect(() => {
        fetchItems();
    }, [fetchItems]);

    const hasRunningExtract = pendingExtracts.some((p) => p.phase === 'running');

    useEffect(() => {
        if (!currentCompanyId || !hasRunningExtract) return;
        const tick = async () => {
            const runners = pendingRef.current.filter((p) => p.phase === 'running');
            for (const p of runners) {
                try {
                    const res = await axios.get<{
                        meta?: { status?: string; error_message?: string };
                    }>(`/api/knowledge-extract/tasks/${p.taskId}`, {
                        headers: { 'X-Company-Id': currentCompanyId },
                    });
                    const status = res.data.meta?.status;
                        if (status === 'completed') {
                            // Instantly refresh and remove the pending row so the real item takes over
                            void fetchItems({ silent: true }).then(() => {
                                setPendingExtracts((prev) => prev.filter((x) => x.taskId !== p.taskId));
                            });
                        } else if (status === 'failed' || status === 'cancelled') {
                        const em = res.data.meta?.error_message;
                        setPendingExtracts((prev) =>
                            prev.map((x) =>
                                x.taskId === p.taskId
                                    ? { ...x, phase: 'failed' as const, errorMessage: em || (status === 'cancelled' ? '已取消' : '提炼失败') }
                                    : x
                            )
                        );
                    }
                } catch {
                    /* 单次轮询失败忽略，下次再试 */
                }
            }
        };
        void tick();
        const id = window.setInterval(() => void tick(), 2000);
        return () => window.clearInterval(id);
    }, [currentCompanyId, hasRunningExtract, fetchItems]);

    const onExtractStarted = useCallback((payload: ExtractStartedPayload) => {
        setPendingExtracts((prev) => {
            if (prev.some((x) => x.taskId === payload.taskId)) return prev;
            return [
                {
                    taskId: payload.taskId,
                    title: payload.displayName,
                    phase: 'running',
                    knowledgeType: payload.knowledgeType,
                },
                ...prev,
            ];
        });
    }, []);

    const pendingAsItems: KnowledgeItem[] = pendingExtracts
        .filter((p) => (p.knowledgeType ?? 'method') === type)
        .map((p) => ({
            id: `extract-pending-${p.taskId}`,
            item_type: type,
            item_name: p.title,
            item_content: '',
            tags_json: '[]',
            created_at: '',
            updated_at: '',
            source_desc: 'AI从历史项目提炼（进行中或待确认）',
            _pendingExtract: p,
        }));

    const orderedItems = useMemo(() => {
        if (!highlightExtractTaskId) return items;
        const hi = items.filter((i) => i.extract_task_id === highlightExtractTaskId);
        if (hi.length === 0) return items;
        const rest = items.filter((i) => i.extract_task_id !== highlightExtractTaskId);
        return [...hi, ...rest];
    }, [items, highlightExtractTaskId]);

    const tableDataSource = [...pendingAsItems, ...orderedItems];

    function formatItemContentPreview(raw: string): string {
        const t = raw?.trim() ?? '';
        if (!t) return '';
        try {
            const o = JSON.parse(t) as unknown;
            return JSON.stringify(o, null, 2);
        } catch {
            return t;
        }
    }

    const handleSubmit = async (values: Record<string, unknown>) => {
        try {
            const payload = {
                ...values,
                item_type: type,
                tags_json: JSON.stringify((values.tags as string[]) || [])
            };

            if (editingId) {
                await axios.patch(`/api/tech-bid/knowledge/${editingId}`, payload, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                message.success('更新成功');
            } else {
                await axios.post('/api/tech-bid/knowledge', payload, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                message.success('创建成功');
            }
            setIsModalVisible(false);
            setEditingId(null);
            setEditingRecord(null);
            form.resetFields();
            fetchItems();
        } catch (err: unknown) {
            console.error('Submit error:', err);
            message.error('操作失败');
        }
    };

    const handleDelete = (id: string) => {
        Modal.confirm({
            title: '删除确认',
            content: '确定要删除此条目吗？',
            onOk: async () => {
                try {
                    await axios.delete(`/api/tech-bid/knowledge/${id}`, {
                        headers: { 'X-Company-Id': currentCompanyId }
                    });
                    message.success('已删除');
                    fetchItems();
                } catch (err) {
                    console.error('Delete error:', err);
                    message.error('删除失败');
                }
            }
        });
    };

    const showEditModal = (record: KnowledgeItem) => {
        setEditingId(record.id);
        setEditingRecord(record);
        let tagList: string[] = [];
        if (record.tags_json) {
            try {
                const parsed = JSON.parse(record.tags_json) as unknown;
                tagList = Array.isArray(parsed) ? (parsed as string[]) : [];
            } catch {
                tagList = [];
            }
        }
        form.setFieldsValue({
            item_name: record.item_name,
            item_content: record.item_content,
            tags: tagList,
        });
        setIsModalVisible(true);
    };

    const columns = [
        {
            title: config.itemLabel,
            dataIndex: 'item_name',
            key: 'item_name',
            render: (text: string, record: KnowledgeItem) => {
                const isPending = !!record._pendingExtract;
                const isAIExtracted = !!record.extract_task_id || isPending;
                
                let displayText = text || '';
                // 彻底清理标题后缀，确保展示纯净
                const aiSuffixes = [' · AI提炼', ' · AI 提炼', ' - AI提炼', ' AI提炼', '·AI提炼', '· AI提炼'];
                for (const suffix of aiSuffixes) {
                    if (displayText.endsWith(suffix)) {
                        displayText = displayText.slice(0, -suffix.length).trim();
                        break;
                    }
                }

                return (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <Typography.Link 
                            onClick={() => {
                                setDetailRecord(record);
                                setDetailVisible(true);
                            }}
                            style={{ 
                                fontWeight: 400, 
                                fontSize: '13.5px',
                                color: '#2563eb',
                                cursor: 'pointer'
                            }}
                        >
                            {displayText}
                        </Typography.Link>
                        {isAIExtracted && (
                            <div style={{
                                padding: '1px 6px',
                                borderRadius: '6px',
                                background: 'linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%)',
                                border: '1px solid #ddd6fe',
                                color: '#7c3aed',
                                fontSize: '11px',
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: '3px',
                                fontWeight: 500,
                                userSelect: 'none',
                                flexShrink: 0
                            }}>
                                <RobotOutlined style={{ fontSize: '12px' }} />
                                <span>AI提炼</span>
                            </div>
                        )}
                    </div>
                )
            }
        },
        {
            title: '操作',
            key: 'action',
            width: 120,
            align: 'right' as const,
            render: (_: unknown, record: KnowledgeItem) => {
                const pe = record._pendingExtract;
                if (pe) {
                    if (pe.phase === 'running') {
                        return (
                            <Space style={{ color: '#1890ff' }}>
                                <Spin size="small" />
                                <Text>AI正在提取中</Text>
                            </Space>
                        );
                    }
                    if (pe.phase === 'failed' || pe.phase === 'cancelled') {
                        return (
                            <Text type="danger" style={{ fontSize: 12 }}>
                                {pe.errorMessage || '提炼失败'}
                            </Text>
                        );
                    }
                }
                return (
                    <Space size="middle">
                        <Button type="text" icon={<EditOutlined />} style={{ color: '#94a3b8' }} onClick={() => showEditModal(record)} />
                        <Button type="text" icon={<DeleteOutlined />} style={{ color: '#94a3b8' }} onClick={() => handleDelete(record.id)} />
                    </Space>
                );
            }
        }
    ];

    return (
        <div style={{ padding: '0 0 24px 0' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
                <Space align="center" size="large">
                    <div style={{ 
                        background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)', 
                        padding: '16px', 
                        borderRadius: '16px', 
                        fontSize: '32px', 
                        color: '#2563eb', 
                        display: 'flex',
                        boxShadow: '0 4px 12px rgba(37, 99, 235, 0.08)',
                        border: '1px solid #bfdbfe'
                    }}>
                        {config.icon}
                    </div>
                    <div>
                        <Title level={2} style={{ margin: 0, fontWeight: 700, color: '#0f172a' }}>{config.title}</Title>
                        <Text style={{ color: '#64748b', fontSize: 15 }}>{config.subTitle}</Text>
                    </div>
                </Space>
                <Space size="middle">
                    <Button
                        icon={<CloudUploadOutlined />}
                        onClick={() => {
                            setResumeTaskId(null);
                            setExtractWizardOpen(true);
                        }}
                        disabled={!currentCompanyId}
                        style={{ 
                            height: '48px', 
                            borderRadius: '12px', 
                            background: '#f0fdf4', 
                            color: '#16a34a', 
                            borderColor: '#bbf7d0',
                            fontWeight: 600,
                            padding: '0 20px',
                            boxShadow: '0 2px 4px rgba(22, 163, 74, 0.05)'
                        }}
                    >
                        AI 智能提炼
                    </Button>
                    <Button 
                        type="primary" 
                        size="large" 
                        onClick={() => {
                            setEditingId(null);
                            setEditingRecord(null);
                            form.resetFields();
                            setIsModalVisible(true);
                        }} 
                        style={{ 
                            height: '48px', 
                            borderRadius: '12px', 
                            padding: '0 24px',
                            background: '#2563eb',
                            boxShadow: '0 4px 12px rgba(37, 99, 235, 0.2)',
                            border: 'none',
                            fontWeight: 600
                        }}
                    >
                        新增条目
                    </Button>
                </Space>
            </div>

            <Card 
                bordered={false} 
                styles={{ body: { padding: 0 } }} 
                style={{ 
                    boxShadow: '0 10px 15px -3px rgba(0,0,0,0.05), 0 4px 6px -4px rgba(0,0,0,0.05)', 
                    borderRadius: '20px', 
                    overflow: 'hidden',
                    border: '1px solid #f1f5f9'
                }}
            >
                <Table
                    columns={columns}
                    dataSource={tableDataSource}
                    loading={loading}
                    rowKey="id"
                    pagination={{ 
                        pageSize: 10,
                        showSizeChanger: false,
                        style: { padding: '16px 24px' }
                    }}
                    rowClassName={(record) => record._pendingExtract ? 'bg-slate-50/50' : 'hover:bg-slate-50 transition-colors'}
                    style={{ borderRadius: '20px' }}
                    onRow={(record) => ({
                        style:
                            record.extract_task_id &&
                            highlightExtractTaskId &&
                            record.extract_task_id === highlightExtractTaskId
                                ? {
                                      background: '#f6ffed',
                                      boxShadow: 'inset 3px 0 0 #52c41a',
                                  }
                                : {},
                    })}
                />
            </Card>

            <Modal
                title="条目详情"
                open={detailVisible}
                onCancel={() => {
                    setDetailVisible(false);
                    setDetailRecord(null);
                }}
                footer={[
                    <Button key="close" type="primary" onClick={() => setDetailVisible(false)}>
                        确定
                    </Button>
                ]}
                width={800}
                destroyOnClose
            >
                {detailRecord && (
                    <div style={{ padding: '8px 0' }}>
                        <div style={{ marginBottom: 24 }}>
                            <Title level={4}>{detailRecord.item_name}</Title>
                            {detailRecord.source_desc && (
                                <Tag color="blue" icon={<BookOutlined />}>{detailRecord.source_desc}</Tag>
                            )}
                        </div>
                        <Divider style={{ margin: '12px 0' }} />
                        <div style={{ marginTop: 16 }}>
                            <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>详细内容：</Text>
                            <pre
                                style={{
                                    margin: 0,
                                    maxHeight: 500,
                                    overflow: 'auto',
                                    padding: 20,
                                    background: '#f8fafc',
                                    borderRadius: 12,
                                    fontSize: 14,
                                    lineHeight: 1.6,
                                    whiteSpace: 'pre-wrap',
                                    wordBreak: 'break-word',
                                    border: '1px solid #e2e8f0'
                                }}
                            >
                                {formatItemContentPreview(detailRecord.item_content)}
                            </pre>
                        </div>
                    </div>
                )}
            </Modal>

            <Modal
                title={editingId ? "编辑条目" : "新增条目"}
                open={isModalVisible}
                onCancel={() => {
                    setIsModalVisible(false);
                    setEditingRecord(null);
                }}
                onOk={() => form.submit()}
                width={700}
                destroyOnClose
            >
                {editingId && editingRecord?.source_desc && (
                    <div style={{ marginBottom: 16, padding: '8px 12px', background: '#f6ffed', borderRadius: 8, fontSize: 13, border: '1px solid #b7eb8f' }}>
                        <Text type="secondary">来源追溯：</Text> {editingRecord.source_desc}
                    </div>
                )}
                <Form form={form} layout="vertical" onFinish={handleSubmit}>
                    <Form.Item name="item_name" label={config.itemLabel} rules={[{ required: true }]}>
                        <Input placeholder={`请输入${config.itemLabel}`} size="large" />
                    </Form.Item>
                    <Form.Item name="tags" label="标签 (Search Keywords)">
                        <Select mode="tags" style={{ width: '100%' }} placeholder="输入标签按回车" size="large" />
                    </Form.Item>
                    <Form.Item name="item_content" label="核心内容 (Markdown/Text)" rules={[{ required: true }]}>
                        <TextArea rows={12} placeholder="输入核心正文、要点或 MD 格式内容..." />
                    </Form.Item>
                </Form>
            </Modal>

            <KnowledgeExtractWizard
                open={extractWizardOpen && !!currentCompanyId}
                onClose={() => {
                    setExtractWizardOpen(false);
                    setResumeTaskId(null);
                }}
                knowledgeType={type || 'method'}
                currentCompanyId={currentCompanyId || ''}
                resumeTaskId={resumeTaskId}
                onExtractStarted={onExtractStarted}
                onCommitted={async (committedTaskId) => {
                    setResumeTaskId(null);
                    const list = await fetchItems({ silent: true });
                    if (committedTaskId && list.length) {
                        const savedRows = list.filter((i) => i.extract_task_id === committedTaskId);
                        if (savedRows.length) {
                            // Only remove the pending row after we can see saved rows in the real list.
                            setPendingExtracts((prev) => prev.filter((x) => x.taskId !== committedTaskId));
                            setHighlightExtractTaskId(committedTaskId);
                        } else {
                            message.warning('已提交入库，但列表尚未返回该条目；保留此行，稍后可再次确认入库');
                        }
                    } else if (committedTaskId) {
                        message.warning('已提交入库，但列表刷新为空；保留此行，稍后请刷新重试');
                    }
                }}
            />
        </div>
    );
};

export default TechKnowledgeLibrary;
