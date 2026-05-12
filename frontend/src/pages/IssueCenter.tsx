import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Space, Tag, Card, Typography, Tabs, message, Tooltip, Empty } from 'antd';
import { 
  WarningOutlined, 
  ExclamationCircleOutlined, 
  ClockCircleOutlined,
  SolutionOutlined,
  CheckOutlined,
  EyeOutlined
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

interface Issue {
    id: string;
    issue_type: string;
    object_type: string;
    object_id: string;
    severity: string;
    issue_message: string;
    created_at: string;
}

const IssueCenter: React.FC = () => {
    const navigate = useNavigate();
    const { currentCompanyId, loading: companyLoading } = useCompany();
    const [loading, setLoading] = useState(true);
    const [issues, setIssues] = useState<Issue[]>([]);
    const [activeTab, setActiveTab] = useState('all');

    const fetchIssues = useCallback(async () => {
        if (companyLoading || !currentCompanyId) return;
        setLoading(true);
        try {
            const response = await axios.get('/api/issues');
            setIssues(response.data || []);
        } catch (err) {
            console.error('Fetch issues error:', err);
            message.error('加载异常数据失败');
        } finally {
            setLoading(false);
        }
    }, [currentCompanyId, companyLoading]);

    useEffect(() => {
        fetchIssues();
    }, [fetchIssues]);

    const handleResolve = async (id: string) => {
        try {
            await axios.patch(`/api/issues/${id}`, { 
                status: 'resolved',
                resolution_note: '用户在中心标记为已解决'
            });
            message.success('问题已标记为解决');
            fetchIssues();
        } catch (err) {
            console.error('Resolve issue error:', err);
            message.error('操作失败');
        }
    };

    const columns = [
        {
            title: '严重性',
            dataIndex: 'severity',
            key: 'severity',
            width: 100,
            render: (sev: string) => {
                const colors: Record<string, string> = {
                    'fatal': 'red',
                    'error': 'volcano',
                    'warning': 'orange',
                    'info': 'blue'
                };
                return <Tag color={colors[sev] || 'default'}>{sev.toUpperCase()}</Tag>;
            }
        },
        {
            title: '异常描述',
            dataIndex: 'issue_message',
            key: 'issue_message',
            render: (text: string, record: Issue) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                    <Text strong>{text}</Text>
                    <Text type="secondary" style={{ fontSize: '12px' }}>
                        对象类型：{record.object_type} | ID: {record.object_id}
                    </Text>
                </div>
            )
        },
        {
            title: '类型',
            dataIndex: 'issue_type',
            key: 'issue_type',
            render: (type: string) => {
                const types: Record<string, string> = {
                    'missing_field': '基础资料缺失',
                    'expiring': '即将过期',
                    'duplicate': '疑似重复数据',
                    'conflict': '逻辑冲突'
                };
                return <Tag>{types[type] || '其他'}</Tag>;
            }
        },
        {
            title: '发现时间',
            dataIndex: 'created_at',
            key: 'created_at',
            render: (date: string) => dayjs(date).format('YYYY-MM-DD HH:mm')
        },
        {
            title: '操作',
            key: 'action',
            render: (_: any, record: Issue) => (
                <Space size="middle">
                    <Tooltip title="跳转查看详情">
                        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => {
                             if (record.object_type === 'performance') navigate('/library/performances');
                             else if (record.object_type === 'qualification') navigate('/library/qualifications');
                             else if (record.object_type === 'person') navigate('/library/persons');
                        }}>去完善</Button>
                    </Tooltip>
                    <Button size="small" icon={<CheckOutlined />} onClick={() => handleResolve(record.id)}>忽略</Button>
                </Space>
            ),
        },
    ];

    const tabItems = [
        { key: 'all', label: '全部异常' },
        { key: 'missing_field', label: '基础资料缺失' },
        { key: 'expiring', label: '证书过期预警' },
        { key: 'conflict', label: '合规性冲突' },
    ];

    const filteredIssues = activeTab === 'all' ? issues : issues.filter(i => i.issue_type === activeTab);

    return (
        <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <div style={{ background: '#fff', padding: '0 24px', margin: '-24px -24px 24px -24px' }}>
                <Tabs 
                    activeKey={activeTab} 
                    onChange={setActiveTab}
                    items={tabItems.map(tab => ({
                        key: tab.key,
                        label: (
                            <span>
                                {tab.label}
                                {issues.filter(i => (tab.key === 'all' || i.issue_type === tab.key)).length > 0 && (
                                    <span style={{ marginLeft: 4, color: '#ff4d4f' }}>({issues.filter(i => (tab.key === 'all' || i.issue_type === tab.key)).length})</span>
                                )}
                            </span>
                        ),
                    }))}
                    style={{ marginBottom: -1 }} 
                />
            </div>

            <div style={{ flex: 1, padding: '0 0 24px 0' }}>
                <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden', marginBottom: 24 }}>
                    <Table
                        columns={columns}
                        dataSource={filteredIssues}
                        loading={loading}
                        rowKey="id"
                        pagination={{ pageSize: 12 }}
                        locale={{ emptyText: <Empty description="当前分类下暂无异常项" /> }}
                    />
                </Card>

                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: '24px' }}>
                    <Card size="small" style={{ borderLeft: '4px solid #ff4d4f' }}>
                        <StatisticCard title="紧急处理" count={issues.filter(i => i.severity === 'fatal' || i.severity === 'error').length} />
                    </Card>
                    <Card size="small" style={{ borderLeft: '4px solid #faad14' }}>
                        <StatisticCard title="待补全业绩" count={issues.filter(i => i.object_type === 'performance').length} />
                    </Card>
                    <Card size="small" style={{ borderLeft: '4px solid #1890ff' }}>
                        <StatisticCard title="失效人员证书" count={issues.filter(i => i.object_type === 'person').length} />
                    </Card>
                    <Card size="small" style={{ borderLeft: '4px solid #d9d9d9' }}>
                        <StatisticCard title="低风险提示" count={issues.filter(i => i.severity === 'info').length} />
                    </Card>
                </div>
            </div>
        </div>
    );
};

const StatisticCard = ({ title, count }: { title: string, count: number }) => (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '4px 8px' }}>
        <Text type="secondary">{title}</Text>
        <Title level={4} style={{ margin: 0 }}>{count}</Title>
    </div>
);

export default IssueCenter;
