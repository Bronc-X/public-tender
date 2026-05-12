/**
 * 正文生成面板（content_generation 步骤）
 * 从 TechBidProjectWorkbench.tsx 中拆分（CTO P0-2）
 */
import React from 'react';
import {
    Typography, Button, Card, Space, Tag, Row, Col, Empty, List, Progress,
} from 'antd';
import {
    FileTextOutlined, SyncOutlined, EditOutlined,
    CheckCircleOutlined, ClockCircleOutlined, FileSearchOutlined,
    PlayCircleOutlined,
} from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { getChapterLabel } from '../utils/numberToChinese';

const { Title, Text } = Typography;

interface ContentGenerationPanelProps {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    displayChapters: any[];
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    editingChapter: any;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    onSelectChapter: (chapter: any) => void;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    onGenerateChapter: (chapter: any) => void;
    onEditChapter: (contentMd: string) => void;
    onConfirm: () => void;
    onBatchGenerate: () => void;
    batchGeneration: {
        running: boolean;
        total: number;
        completed: number;
        failed: number;
        currentName: string;
    };
}

const ContentGenerationPanel: React.FC<ContentGenerationPanelProps> = ({
    displayChapters,
    editingChapter,
    onSelectChapter,
    onGenerateChapter,
    onEditChapter,
    onConfirm,
    onBatchGenerate,
    batchGeneration,
}) => {
    const contentChapters = displayChapters.filter((c: unknown) => {
        const ch = c as { node_level?: string; parent_id?: string; id?: string };
        return ch.node_level === 'subsection' || (!ch.parent_id && displayChapters.filter((child: unknown) => (child as { parent_id?: string }).parent_id === ch.id).length === 0);
    });
    const completedCount = contentChapters.filter((c: unknown) => {
        const ch = c as { generation_status?: string; content_md?: string };
        return ch.generation_status === 'completed' || Boolean(ch.content_md);
    }).length;
    const totalCount = contentChapters.length;
    const overallPercent = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;
    const queuePercent = batchGeneration.total > 0
        ? Math.round(((batchGeneration.completed + batchGeneration.failed) / batchGeneration.total) * 100)
        : overallPercent;

    const renderStatusTag = (chapter: { generation_status?: string; content_md?: string }) => {
        if (chapter.generation_status === 'generating') return <Tag color="processing">生成中</Tag>;
        if (chapter.generation_status === 'error') return <Tag color="error">失败</Tag>;
        if (chapter.generation_status === 'completed' || chapter.content_md) return <Tag color="success">已完成</Tag>;
        return <Tag>待生成</Tag>;
    };

    return (
        <div style={{ padding: 24 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
                <div>
                    <Title level={4}>技术标内容工厂</Title>
                    <Text type="secondary">按小节粒度逐步生成正文。这种方式能确保超长文档的内容质量与稳定性。</Text>
                </div>
                <Space>
                    <Button
                        type="primary"
                        icon={<PlayCircleOutlined />}
                        loading={batchGeneration.running}
                        disabled={batchGeneration.running || totalCount === 0}
                        onClick={onBatchGenerate}
                    >
                        {batchGeneration.running ? '排队生成中' : '全书排队生成'}
                    </Button>
                </Space>
            </div>
            <div style={{ marginBottom: 24, padding: '16px 20px', border: '1px solid #e5e7eb', borderRadius: 8, background: '#fff' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, marginBottom: 8 }}>
                    <Text strong>{batchGeneration.running ? '全书生成进度' : '正文完成进度'}</Text>
                    <Text type="secondary">
                        已完成 {completedCount}/{totalCount}
                        {batchGeneration.running && `，本轮 ${batchGeneration.completed + batchGeneration.failed}/${batchGeneration.total}`}
                    </Text>
                </div>
                <Progress percent={batchGeneration.running ? queuePercent : overallPercent} status={batchGeneration.failed > 0 ? 'exception' : 'active'} />
                {batchGeneration.running && (
                    <div style={{ marginTop: 8, display: 'flex', justifyContent: 'space-between', gap: 16 }}>
                        <Text type="secondary">正在生成：{batchGeneration.currentName || '准备队列'}</Text>
                        <Text type={batchGeneration.failed > 0 ? 'danger' : 'secondary'}>失败 {batchGeneration.failed}</Text>
                    </div>
                )}
            </div>
            <Row gutter={24}>
                <Col span={7}>
                    <Card title="目录导航" size="small" style={{ height: 600, overflow: 'auto' }}>
                        <List
                            dataSource={contentChapters}
                            renderItem={(chapter: unknown) => {
                                const ch = chapter as { id?: string; generation_status?: string; chapter_order?: number; chapter_name?: string; content_md?: string };
                                return (
                                    <div
                                        style={{
                                            padding: '12px 16px',
                                            cursor: 'pointer',
                                            borderBottom: '1px solid #f0f0f0',
                                            background: editingChapter?.id === ch.id ? '#e6f7ff' : 'transparent',
                                        }}
                                        onClick={() => onSelectChapter(chapter)}
                                    >
                                        <Space>
                                            {ch.generation_status === 'generating' ? <SyncOutlined spin style={{ color: '#1677ff' }} /> : (ch.generation_status === 'completed' || ch.content_md) ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <ClockCircleOutlined style={{ color: '#d9d9d9' }} />}
                                            <Text strong={editingChapter?.id === ch.id}>
                                                {getChapterLabel(ch.chapter_order || 0, 'subsection')} {ch.chapter_name}
                                            </Text>
                                        </Space>
                                    </div>
                                );
                            }}
                        />
                    </Card>
                </Col>
                <Col span={17}>
                    {editingChapter ? (
                        <Card
                            title={<Space><FileTextOutlined style={{ color: '#1890ff' }} /> {editingChapter.chapter_name}</Space>}
                            extra={<Space>{renderStatusTag(editingChapter)}</Space>}
                        >
                            <div style={{ minHeight: 400, backgroundColor: '#f8fafc', padding: 24, borderRadius: 8, marginBottom: 20, border: '1px solid #e2e8f0', maxHeight: 500, overflow: 'auto' }}>
                                {editingChapter.content_md ? (
                                    <div className="markdown-content">
                                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{editingChapter.content_md}</ReactMarkdown>
                                    </div>
                                ) : (
                                    <div style={{ textAlign: 'center', color: '#94a3b8', marginTop: 140 }}>
                                        <Empty description="该小节尚未生成内容" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                                        <Button type="primary" icon={<SyncOutlined />} style={{ marginTop: 16 }} onClick={() => onGenerateChapter(editingChapter)}>开始 AI 生成</Button>
                                    </div>
                                )}
                            </div>
                            <Space>
                                <Button type="primary" icon={<SyncOutlined />} onClick={() => onGenerateChapter(editingChapter)}>AI 重写</Button>
                                <Button icon={<EditOutlined />} onClick={() => onEditChapter(editingChapter.content_md || '')}>手动编辑</Button>
                            </Space>
                        </Card>
                    ) : (
                        <div style={{ textAlign: 'center', background: '#f8fafc', padding: '160px 0', borderRadius: 8, border: '1px dashed #cbd5e1' }}>
                            <FileSearchOutlined style={{ fontSize: 64, color: '#94a3b8' }} />
                            <Title level={4} style={{ marginTop: 24, color: '#64748b' }}>内容预览区</Title>
                            <Text type="secondary">请点击左侧目录节点，进行内容生成或编辑</Text>
                        </div>
                    )}
                </Col>
            </Row>
            <div style={{ marginTop: 32, textAlign: 'center' }}>
                <Button type="primary" size="large" onClick={onConfirm}>内容编写完毕，进入风控合规检查</Button>
            </div>
        </div>
    );
};

export default ContentGenerationPanel;
