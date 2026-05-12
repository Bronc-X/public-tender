/**
 * 证据抽屉 / 溯源视图 (任务9)
 * 允许用户点击字段后查看完整证据来源：
 * - value / source_text / source_location / confidence / chunk_index / human_confirmed
 * - 若有多个候选值，支持切换查看
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
    Drawer, Typography, Descriptions, Tag, Space, Spin, Empty,
    Tabs, Card, List, Badge,
} from 'antd';
import {
    FileSearchOutlined, CheckCircleOutlined,
} from '@ant-design/icons';
import {
    type EvidenceField, type EvidenceListItem,
    isEvidenceField, isEvidenceList,
} from '../types/projectProfile';
import {
    fetchProfileExtractionSnapshots,
    type ExtractionSnapshot,
} from '../api/techBidStep4';

const { Text, Paragraph } = Typography;

// ─── Types ─────────────────────────────────────────────

interface EvidenceDrawerProps {
    open: boolean;
    onClose: () => void;
    projectId: string;
    fieldLabel: string;
    fieldValue: unknown;
    fieldPath?: string;
}

// ─── Helper: render single evidence field ──────────────

const renderEvidenceDetail = (value: EvidenceField, label: string) => (
    <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
        <Descriptions.Item label="字段">{label}</Descriptions.Item>
        <Descriptions.Item label="值">
            <Text strong>{value.missing ? <Tag color="default">缺失</Tag> : (value.value || '无')}</Text>
        </Descriptions.Item>
        <Descriptions.Item label="置信度">
            {typeof value.confidence === 'number' ? (
                <Tag color={value.confidence >= 0.8 ? 'success' : value.confidence >= 0.5 ? 'blue' : 'warning'}>
                    {(value.confidence * 100).toFixed(0)}%
                </Tag>
            ) : <Text type="secondary">未知</Text>}
        </Descriptions.Item>
        <Descriptions.Item label="原文依据">
            {value.source_text ? (
                <Paragraph style={{ margin: 0, fontSize: 13, background: '#fffbe6', padding: '8px 12px', borderRadius: 6, border: '1px solid #ffe58f' }}>
                    {value.source_text}
                </Paragraph>
            ) : <Text type="secondary">无</Text>}
        </Descriptions.Item>
        <Descriptions.Item label="位置">
            {value.source_location || <Text type="secondary">未标注</Text>}
        </Descriptions.Item>
        <Descriptions.Item label="备注">
            {value.notes || <Text type="secondary">无</Text>}
        </Descriptions.Item>
    </Descriptions>
);

const renderEvidenceListDetail = (items: EvidenceListItem[], label: string) => (
    <div style={{ marginBottom: 16 }}>
        <Text strong style={{ marginBottom: 8, display: 'block' }}>{label} ({items.length} 条)</Text>
        <List
            size="small"
            bordered
            dataSource={items}
            renderItem={(item, idx) => (
                <List.Item>
                    <div style={{ width: '100%' }}>
                        <Space style={{ marginBottom: 4 }}>
                            <Badge count={idx + 1} style={{ backgroundColor: '#1890ff' }} />
                            <Text strong>{item.value || item.name || '无'}</Text>
                        </Space>
                        {(item.source_text || item.source_location) && (
                            <div style={{ marginTop: 4, paddingLeft: 28 }}>
                                {item.source_text && (
                                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                                        依据：{item.source_text}
                                    </Text>
                                )}
                                {item.source_location && (
                                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                                        位置：{item.source_location}
                                    </Text>
                                )}
                            </div>
                        )}
                    </div>
                </List.Item>
            )}
        />
    </div>
);

// ─── Component ─────────────────────────────────────────

const EvidenceDrawer: React.FC<EvidenceDrawerProps> = ({
    open,
    onClose,
    projectId,
    fieldLabel,
    fieldValue,
    fieldPath,
}) => {
    const [loading, setLoading] = useState(false);
    const [chunkSnapshots, setChunkSnapshots] = useState<ExtractionSnapshot[]>([]);

    const loadChunkEvidence = useCallback(async () => {
        if (!open || !projectId) return;
        setLoading(true);
        try {
            const { snapshots } = await fetchProfileExtractionSnapshots(projectId, undefined, 'chunk_output');
            setChunkSnapshots(snapshots);
        } catch {
            setChunkSnapshots([]);
        } finally {
            setLoading(false);
        }
    }, [open, projectId]);

    useEffect(() => {
        void loadChunkEvidence();
    }, [loadChunkEvidence]);

    // Find chunks that mention this field's value in their payload
    const relevantChunks = chunkSnapshots.filter((snap) => {
        if (!fieldPath) return false;
        try {
            const payload = snap.payload as Record<string, unknown>;
            const raw = payload?.raw || payload?.parsed_result || payload;
            const jsonStr = JSON.stringify(raw);
            // Simple heuristic: check if the field path or its last segment appears in the chunk output
            const lastSegment = fieldPath.split('.').pop() || '';
            return jsonStr.includes(lastSegment);
        } catch {
            return false;
        }
    });

    return (
        <Drawer
            title={
                <Space>
                    <FileSearchOutlined />
                    <span>证据溯源：{fieldLabel}</span>
                </Space>
            }
            placement="right"
            width={560}
            onClose={onClose}
            open={open}
        >
            <Tabs
                defaultActiveKey="evidence"
                items={[
                    {
                        key: 'evidence',
                        label: '字段证据',
                        children: (
                            <div>
                                {isEvidenceField(fieldValue) ? (
                                    renderEvidenceDetail(fieldValue, fieldLabel)
                                ) : isEvidenceList(fieldValue) ? (
                                    renderEvidenceListDetail(fieldValue, fieldLabel)
                                ) : (
                                    <Descriptions bordered size="small" column={1}>
                                        <Descriptions.Item label="字段">{fieldLabel}</Descriptions.Item>
                                        <Descriptions.Item label="值">
                                            <Text>{typeof fieldValue === 'string' ? fieldValue : JSON.stringify(fieldValue)}</Text>
                                        </Descriptions.Item>
                                    </Descriptions>
                                )}
                            </div>
                        ),
                    },
                    {
                        key: 'chunks',
                        label: (
                            <span>
                                Chunk 溯源
                                {relevantChunks.length > 0 && (
                                    <Badge
                                        count={relevantChunks.length}
                                        style={{ backgroundColor: '#1890ff', marginLeft: 6 }}
                                        size="small"
                                    />
                                )}
                            </span>
                        ),
                        children: loading ? (
                            <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
                        ) : relevantChunks.length > 0 ? (
                            <List
                                size="small"
                                dataSource={relevantChunks}
                                renderItem={(snap) => {
                                    const payload = snap.payload as Record<string, unknown>;
                                    return (
                                        <List.Item>
                                            <Card size="small" style={{ width: '100%' }}>
                                                <Space style={{ marginBottom: 8 }}>
                                                    <Tag color="blue">Chunk #{snap.chunk_index}</Tag>
                                                    {snap.chunk_type && <Tag>{snap.chunk_type}</Tag>}
                                                    <Text type="secondary" style={{ fontSize: 11 }}>{snap.created_at}</Text>
                                                </Space>
                                                <Paragraph
                                                    style={{ fontSize: 12, margin: 0, maxHeight: 120, overflow: 'auto' }}
                                                    ellipsis={{ rows: 4, expandable: true }}
                                                >
                                                    {typeof payload?.raw === 'string'
                                                        ? payload.raw
                                                        : JSON.stringify(payload, null, 2).slice(0, 500)}
                                                </Paragraph>
                                            </Card>
                                        </List.Item>
                                    );
                                }}
                            />
                        ) : (
                            <Empty description="暂无匹配的 Chunk 抽取记录" />
                        ),
                    },
                    {
                        key: 'status',
                        label: '状态信息',
                        children: (
                            <Descriptions bordered size="small" column={1}>
                                <Descriptions.Item label="字段路径">
                                    <Text code>{fieldPath || '未映射'}</Text>
                                </Descriptions.Item>
                                <Descriptions.Item label="数据类型">
                                    {isEvidenceField(fieldValue) ? (
                                        <Tag>EvidenceField</Tag>
                                    ) : isEvidenceList(fieldValue) ? (
                                        <Tag>EvidenceList[{(fieldValue as EvidenceListItem[]).length}]</Tag>
                                    ) : (
                                        <Tag>原始值</Tag>
                                    )}
                                </Descriptions.Item>
                                <Descriptions.Item label="是否缺失">
                                    {isEvidenceField(fieldValue)
                                        ? (fieldValue.missing ? <Tag color="error">是</Tag> : <Tag color="success">否</Tag>)
                                        : <Text type="secondary">不适用</Text>}
                                </Descriptions.Item>
                                <Descriptions.Item label="人工确认">
                                    {isEvidenceField(fieldValue) && (fieldValue as Record<string, unknown>).human_confirmed
                                        ? <Tag color="success" icon={<CheckCircleOutlined />}>已确认</Tag>
                                        : <Tag color="default">未确认</Tag>}
                                </Descriptions.Item>
                            </Descriptions>
                        ),
                    },
                ]}
            />
        </Drawer>
    );
};

export default EvidenceDrawer;
