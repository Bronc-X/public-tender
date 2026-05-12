import React, { useState, useEffect, useCallback } from 'react';
import { Button, Space, Tag, Card, Typography, Form, Select, DatePicker, Checkbox, message, Progress, List, Avatar, Input, Empty } from 'antd';
import { 
  CloudDownloadOutlined as ExportOutlined, 
  FileExcelOutlined, 
  FileZipOutlined,
  CheckCircleOutlined,
  SyncOutlined,
  SearchOutlined
} from '@ant-design/icons';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text, Paragraph } = Typography;
const { RangePicker } = DatePicker;

interface ExportTemplate {
    id: string;
    name: string;
    format: string;
    description: string;
}

interface ExportTask {
    id: string;
    status: string;
    progress: number;
    message: string;
    created_at: string;
    result_json?: string;
}

const ExportCenter: React.FC = () => {
    const { currentCompanyId, loading: companyLoading } = useCompany();
    const [loading, setLoading] = useState(false);
    const [templates, setTemplates] = useState<ExportTemplate[]>([]);
    const [tasks, setTasks] = useState<ExportTask[]>([]);
    const [form] = Form.useForm();

    const fetchTemplates = useCallback(async () => {
        if (companyLoading || !currentCompanyId) return;
        try {
            const resp = await axios.get('/api/exports/templates');
            setTemplates(resp.data);
        } catch (err) {
            console.error('Failed to fetch templates:', err);
        }
    }, [currentCompanyId, companyLoading]);

    const fetchTasks = useCallback(async () => {
        if (companyLoading || !currentCompanyId) return;
        try {
            const resp = await axios.get('/api/exports/tasks');
            setTasks(resp.data);
        } catch (err) {
            console.error('Failed to fetch tasks:', err);
        }
    }, [currentCompanyId, companyLoading]);

    useEffect(() => {
        fetchTemplates();
        fetchTasks();
        const timer = setInterval(fetchTasks, 5000);
        return () => clearInterval(timer);
    }, [fetchTemplates, fetchTasks]);

    const onFinish = async (values: any) => {
        setLoading(true);
        try {
            const selectedTemplate = templates.find(t => t.id === values.template_id);
            await axios.post('/api/exports', {
                template_id: values.template_id,
                name: selectedTemplate?.name || '自定义导出',
                filters: values
            });
            message.success('导出任务已提交，系统正在后台处理中...');
            fetchTasks();
        } catch (err) {
            console.error('Export failed:', err);
            message.error('导出失败，请重试');
        } finally {
            setLoading(false);
        }
    };

    const getStatusTag = (status: string) => {
        switch (status) {
            case 'completed': return <Tag color="success" icon={<CheckCircleOutlined />}>完成</Tag>;
            case 'running': return <Tag color="processing" icon={<SyncOutlined spin />}>处理中</Tag>;
            case 'failed': return <Tag color="error">失败</Tag>;
            default: return <Tag color="default">排队中</Tag>;
        }
    };

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div>
                    <Title level={3} style={{ margin: 0 }}>
                        <ExportOutlined style={{ marginRight: 8, color: '#1890ff' }} /> 导出中心 (Export Center)
                    </Title>
                    <Text type="secondary">在此挑选您的投标模板，快速打包业绩、人员、资质文件为 Excel 或 ZIP 格式。</Text>
                </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(12, minmax(0, 1fr))', gap: '32px' }}>
                {/* Left: Configuration Panel */}
                <Card style={{ gridColumn: 'span 8', boxShadow: '0 1px 2px 0 rgba(0, 0, 0, 0.03)', border: 'none' }} title="🚀 新建导出任务">
                    <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ template_id: 'v15_standard' }}>
                        <Form.Item label="导出模板" name="template_id" required>
                            <Select size="large">
                                {templates.map(t => (
                                    <Select.Option key={t.id} value={t.id}>
                                        <Space>
                                            {t.format === 'xlsx' ? <FileExcelOutlined style={{ color: '#52c41a' }} /> : <FileZipOutlined style={{ color: '#faad14' }} />}
                                            {t.name}
                                        </Space>
                                    </Select.Option>
                                ))}
                            </Select>
                        </Form.Item>

                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '16px' }}>
                            <Form.Item label="时间范围" name="date_range">
                                <RangePicker style={{ width: '100%' }} />
                            </Form.Item>
                            <Form.Item label="导出模块">
                                <Checkbox.Group options={['业绩', '人员', '资质', '附件']} defaultValue={['业绩', '人员']} />
                            </Form.Item>
                        </div>

                        <Form.Item label="附加筛选 (选填)" name="keyword">
                            <Input 
                                prefix={<SearchOutlined style={{ color: '#bfbfbf' }} />} 
                                placeholder="输入项目名称、经理名或业主等关键词进行过滤" 
                            />
                        </Form.Item>

                        <Form.Item shouldUpdate={(prevVal, curVal) => prevVal.template_id !== curVal.template_id}>
                            {({ getFieldValue }) => (
                                <div style={{ marginTop: 16, padding: 16, backgroundColor: '#f0faff', borderRadius: 4, border: '1px solid #e6f7ff', marginBottom: 24 }}>
                                    <Paragraph style={{ margin: 0, fontSize: '14px' }}>
                                        <Text strong>当前已选：</Text> 
                                        {templates.find(t => t.id === getFieldValue('template_id'))?.description || '请先选择模板'}
                                    </Paragraph>
                                </div>
                            )}
                        </Form.Item>

                        <Button type="primary" size="large" block icon={<ExportOutlined />} loading={loading} htmlType="submit" style={{ height: '48px', borderRadius: '8px' }}>
                            立即开始生成文件
                        </Button>
                    </Form>
                </Card>

                {/* Right: Task History */}
                <div style={{ gridColumn: 'span 4', display: 'flex', flexDirection: 'column', gap: '24px' }}>
                    <Title level={4}>🕒 最近导出任务</Title>
                    <Card style={{ boxShadow: '0 1px 2px 0 rgba(0, 0, 0, 0.03)', border: 'none', overflow: 'hidden' }} styles={{ body: { padding: 0 } }}>
                        <List
                            itemLayout="horizontal"
                            dataSource={tasks}
                            loading={tasks.length === 0 && loading}
                            renderItem={task => (
                                <List.Item style={{ paddingLeft: 16, paddingRight: 16 }} actions={[
                                    task.status === 'completed' ? <Button type="link" size="small" key="download">下载</Button> : null
                                ]}>
                                    <List.Item.Meta
                                        avatar={
                                            <Avatar 
                                                style={{ backgroundColor: task.status === 'completed' ? '#52c41a' : '#1890ff' }} 
                                                icon={task.status === 'completed' ? <CheckCircleOutlined /> : <SyncOutlined spin={task.status === 'running'} />} 
                                            />
                                        }
                                        title={<Text style={{ fontSize: '13px' }}>{dayjs(task.created_at).format('MM-DD HH:mm')}</Text>}
                                        description={
                                            <div style={{ marginTop: 4 }}>
                                                <div style={{ marginBottom: 4, fontSize: '12px' }}>{getStatusTag(task.status)}</div>
                                                <div style={{ fontSize: '12px', color: '#8c8c8c' }}>{task.message}</div>
                                                {task.status === 'running' && <Progress percent={task.progress} size="small" style={{ marginTop: 4 }} />}
                                            </div>
                                        }
                                    />
                                </List.Item>
                            )}
                        />
                        {tasks.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无历史任务" style={{ padding: '40px 0' }} />}
                    </Card>
                </div>
            </div>
        </div>
    );
};

export default ExportCenter;
