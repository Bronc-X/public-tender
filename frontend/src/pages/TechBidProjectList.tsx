import React, { useEffect, useState, useCallback } from 'react';
import { Table, Button, Space, Card, Tag, Modal, Form, Input, message, Typography, Radio, Divider, List, Avatar } from 'antd';
import { PlusOutlined, SafetyCertificateOutlined, DeleteOutlined, EditOutlined, SyncOutlined, FileTextOutlined, SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios, { isAxiosError } from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

const TechBidProjectList: React.FC = () => {
    const navigate = useNavigate();
    const { currentCompanyId } = useCompany();
    const [projects, setProjects] = useState<any[]>([]);
    const [searchText, setSearchText] = useState('');
    const [loading, setLoading] = useState(false);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingId, setEditingId] = useState<string | null>(null);
    const [createMode, setCreateMode] = useState<'new' | 'sync'>('new');
    const [candidates, setCandidates] = useState<any[]>([]);
    const [candidateSearch, setCandidateSearch] = useState('');
    const [selectedCandidate, setSelectedCandidate] = useState<any>(null);
    const [form] = Form.useForm();
    
    const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

    const highlightText = (text: string | null | undefined, search: string) => {
        const t = text ?? '';
        if (!search || !t) return t || '—';
        let parts: string[];
        try {
            parts = t.split(new RegExp(`(${escapeRegExp(search)})`, 'gi'));
        } catch {
            return t;
        }
        return (
            <span>
                {parts.map((part, i) =>
                    part.toLowerCase() === search.toLowerCase() ? (
                        <span key={i} style={{ color: '#f5222d', fontWeight: 'bold' }}>{part}</span>
                    ) : (
                        <span key={i}>{part}</span>
                    )
                )}
            </span>
        );
    };

    const fetchProjects = async () => {
        if (!currentCompanyId) return;
        setLoading(true);
        try {
            const res = await axios.get('/api/tech-bid/projects', {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            setProjects(res.data);
        } catch (err) {
            const detail =
                isAxiosError(err) && err.response?.data && typeof (err.response.data as { error?: string }).error === 'string'
                    ? (err.response.data as { error: string }).error
                    : null;
            console.error('tech-bid projects list failed', err);
            message.error(detail ? `获取项目列表失败：${detail}` : '获取项目列表失败');
        } finally {
            setLoading(false);
        }
    };

    const fetchCandidates = useCallback(async (search: string = '') => {
        if (!currentCompanyId) return;
        try {
            const res = await axios.get('/api/shared-tenders/candidates', {
                params: { search },
                headers: { 'X-Company-Id': currentCompanyId }
            });
            setCandidates(Array.isArray(res.data) ? res.data : []);
        } catch (err) {
            console.error('Failed to fetch candidates', err);
        }
    }, [currentCompanyId]);

    useEffect(() => {
        if (createMode === 'sync' && isModalOpen) {
            const timer = setTimeout(() => {
                fetchCandidates(candidateSearch);
            }, 300);
            return () => clearTimeout(timer);
        }
    }, [candidateSearch, createMode, isModalOpen, fetchCandidates]);

    useEffect(() => {
        fetchProjects();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentCompanyId]);

    const handleDelete = (id: string) => {
        Modal.confirm({
            title: '删除项目',
            content: '确定要删除该技术标项目吗？此操作不可恢复。',
            okText: '确认',
            cancelText: '取消',
            okButtonProps: { danger: true },
            onOk: async () => {
                try {
                    await axios.delete(`/api/tech-bid/projects/${id}`, {
                        headers: { 'X-Company-Id': currentCompanyId }
                    });
                    message.success('技术标项目已删除');
                    fetchProjects();
                } catch (err: any) {
                    message.error('删除失败');
                }
            }
        });
    };

    const handleCreate = async (values: any) => {
        try {
            const payload = { ...values };
            if (createMode === 'sync' && selectedCandidate) {
                payload.shared_tender_id = selectedCandidate.mode === 'registry' ? selectedCandidate.id : null;
                payload.sync_source_project_id = selectedCandidate.source_project_id;
                payload.sync_source_module = selectedCandidate.source_module;
                payload.tender_code = selectedCandidate.tender_code;
                if (!payload.project_name) payload.project_name = selectedCandidate.project_name;
            }

            if (editingId) {
                await axios.patch(`/api/tech-bid/projects/${editingId}`, payload, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                message.success('技术标项目更新成功');
            } else {
                const res = await axios.post('/api/tech-bid/projects', payload, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                message.success('技术标项目创建成功');
                navigate(`/tech-bid-projects/${res.data.id}`);
            }
            setIsModalOpen(false);
            setEditingId(null);
            setSelectedCandidate(null);
            setCreateMode('new');
            form.resetFields();
            fetchProjects();
        } catch (err: any) {
            message.error((editingId ? '更新' : '创建') + '失败: ' + err.message);
        }
    };

    const showEditModal = (record: any) => {
        setEditingId(record.id);
        form.setFieldsValue({
            project_name: record.project_name,
            project_type: record.project_type,
            profession: record.profession
        });
        setIsModalOpen(true);
    };

    const columns = [
        {
            title: '项目名称',
            dataIndex: 'project_name',
            key: 'project_name',
            render: (text: string, record: any) => (
                <Text 
                    style={{ fontSize: '14px', color: '#1890ff', cursor: 'pointer', fontWeight: 400 }}
                    onClick={() => navigate(`/tech-bid-projects/${record.id}`)}
                >
                    {highlightText(text, searchText)}
                </Text>
            ),
        },
        {
            title: '项目业主',
            dataIndex: 'owner_name',
            key: 'owner_name',
            render: (text: string) => highlightText(text, searchText)
        },
        {
            title: '当前状态',
            dataIndex: 'current_step',
            key: 'current_step',
            render: (step: string, record: any) => {
                const stepMap: any = {
                    'tender_parse': '招标文件解析',
                    'project_profile': '项目特征画像',
                    'route_planning': '施工路线规划',
                    'chapter_generation': '章节内容生成',
                    'risk_review': '风控与合规检查',
                    'output_finalize': '导出完成'
                };
                const stepName = stepMap[step] || step || '未知阶段';
                const isFailed = record.project_status === 'failed';
                const isCompleted = record.project_status === 'completed';
                return (
                    <Tag color={isFailed ? 'error' : isCompleted ? 'success' : 'processing'} style={{ borderRadius: '4px' }}>
                        {stepName}
                    </Tag>
                );
            }
        },
        {
            title: '关联状态',
            dataIndex: 'shared_tender_id',
            key: 'shared_tender_id',
            render: (id: string, record: any) => (
                id ? <Tag color="purple" icon={<SyncOutlined spin={record.current_step_status === 'running'} />}>已同步</Tag> : <Tag>独立项目</Tag>
            )
        },
        {
            title: '创建时间',
            dataIndex: 'created_at',
            key: 'created_at',
            render: (date: string | undefined) =>
                date ? new Date(date).toLocaleString() : '—'
        },
        {
            title: '操作',
            key: 'action',
            width: 120,
            render: (_: any, record: any) => (
                <Space size="middle">
                    <Button 
                        type="text" 
                        icon={<EditOutlined />} 
                        onClick={() => showEditModal(record)}
                        style={{ color: 'rgba(0, 0, 0, 0.45)' }}
                    />
                    <Button 
                        type="text" 
                        icon={<DeleteOutlined />} 
                        onClick={() => handleDelete(record.id)}
                        style={{ color: 'rgba(0, 0, 0, 0.45)' }}
                    />
                </Space>
            ),
        },
    ];

    return (
        <div style={{ padding: '0 0 24px 0' }}>
            <div style={{ display: 'flex', justifyContent: 'flex-start', alignItems: 'center', marginBottom: 24 }}>
                <Space size={16}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => {
                        setEditingId(null);
                        setCreateMode('new');
                        setSelectedCandidate(null);
                        setCandidateSearch('');
                        form.resetFields();
                        fetchCandidates('');
                        setIsModalOpen(true);
                    }} style={{ borderRadius: '8px' }}>
                        新建
                    </Button>
                    <Input
                      placeholder="搜索项目名称、业主..."
                      prefix={<SearchOutlined />}
                      allowClear
                      value={searchText}
                      onChange={e => setSearchText(e.target.value)}
                      style={{ width: 400 }}
                    />
                </Space>
            </div>

            <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
                <Table 
                    columns={columns} 
                    dataSource={projects.filter(item => 
                        item.project_name?.toLowerCase().includes(searchText.toLowerCase()) ||
                        item.owner_name?.toLowerCase().includes(searchText.toLowerCase())
                    )} 
                    loading={loading} 
                    rowKey="id" 
                    pagination={{ pageSize: 10 }}
                    style={{ margin: 0 }}
                />
            </Card>

            <Modal
                title={editingId ? "编辑技术标项目" : "新建技术标项目"}
                open={isModalOpen}
                onCancel={() => {
                    setIsModalOpen(false);
                    setEditingId(null);
                    form.resetFields();
                }}
                footer={null}
                width={600}
                destroyOnClose
            >
                <div style={{ marginBottom: 24 }}>
                    <Radio.Group 
                        value={createMode} 
                        onChange={e => setCreateMode(e.target.value)} 
                        optionType="button" 
                        buttonStyle="solid"
                        disabled={!!editingId}
                    >
                        <Radio.Button value="new">新文件创建</Radio.Button>
                        <Radio.Button value="sync">从已有项目同步</Radio.Button>
                    </Radio.Group>
                </div>

                {createMode === 'sync' ? (
                    <div>
                        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
                            检测到以下已有招标项目，同步后可复用招标文件与解析结果，避免重复计费。
                        </Text>
                        <Input.Search 
                            placeholder="输入项目标题或招标编号搜索" 
                            style={{ marginBottom: 16 }} 
                            onChange={e => setCandidateSearch(e.target.value)}
                            value={candidateSearch}
                            allowClear
                        />
                        <List
                            pagination={{
                                pageSize: 5,
                                size: 'small',
                                simple: true,
                                hideOnSinglePage: true
                            }}
                            dataSource={candidates}
                            renderItem={item => (
                                <List.Item 
                                    style={{ 
                                        cursor: 'pointer', 
                                        borderRadius: '8px', 
                                        padding: '12px',
                                        marginBottom: '8px',
                                        transition: 'all 0.3s',
                                        border: selectedCandidate?.id === item.id ? '2px solid #1890ff' : '1px solid #f0f0f0',
                                        background: selectedCandidate?.id === item.id ? '#f0f5ff' : 'white'
                                    }}
                                    onClick={() => {
                                        setSelectedCandidate(item);
                                        form.setFieldsValue({ project_name: item.project_name, owner_name: item.owner_name });
                                    }}
                                >
                                    <List.Item.Meta
                                        avatar={<Avatar icon={<FileTextOutlined />} style={{ backgroundColor: '#1890ff' }} />}
                                        title={highlightText(item.project_name, candidateSearch)}
                                        description={
                                            <Space split={<Divider type="vertical" />}>
                                                <Text type="secondary">编号: {highlightText(item.tender_code || '无', candidateSearch)}</Text>
                                                <Tag color="cyan">{item.source_module === 'bid' ? '商务标来源' : '技术标来源'}</Tag>
                                            </Space>
                                        }
                                    />
                                </List.Item>
                            )}
                        />
                        <Form form={form} layout="vertical" onFinish={handleCreate} style={{ marginTop: 24 }}>
                            <Form.Item name="project_name" label="技术标项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
                                <Input placeholder="同步后的技术标项目名称" size="large" />
                            </Form.Item>
                            <Form.Item name="owner_name" label="项目业主" rules={[{ required: true, message: '请输入业主名称' }]}>
                                <Input placeholder="业主单位名称" size="large" />
                            </Form.Item>
                            <Button type="primary" htmlType="submit" block size="large" style={{ height: '48px' }} disabled={!selectedCandidate}>
                                确认同步并创建
                            </Button>
                        </Form>
                    </div>
                ) : (
                    <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ project_name: '', owner_name: '', profession: '土建施工' }}>
                        <Form.Item name="project_name" label="项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
                            <Input placeholder="例如：某市中心医院技术标制作" size="large" />
                        </Form.Item>
                        <Form.Item name="owner_name" label="项目业主 (Owner)" rules={[{ required: true, message: '请输入业主名称' }]}>
                            <Input placeholder="例如：某市城市建设投资有限公司" size="large" />
                        </Form.Item>
                        {/* 工程大类字段已移除：骨架分类由 AI 根据招标文件内容自动推荐，人工确认后再生成目录 */}
                        <Form.Item style={{ marginTop: 24, marginBottom: 0 }}>
                            <Button type="primary" htmlType="submit" block size="large" style={{ height: '48px' }}>
                                {editingId ? "确认更改" : "确认创建"}
                            </Button>
                        </Form.Item>
                    </Form>
                )}
            </Modal>
        </div>
    );
};

export default TechBidProjectList;
