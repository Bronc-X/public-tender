/**
 * 运行结果可回放面板 (任务13)
 * 支持按 run 回放一次完整抽取过程：
 * - 原始输入 (raw_text)
 * - chunk 列表 (chunks)
 * - 每 chunk 输出 (chunk_output)
 * - prompt 输入 (prompt_input)
 * - merge diff (merge_diff)
 * - 补充抽取 (supplement_*)
 * - 最终输出 (final_payload)
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
    Drawer, Typography, Select, Space, Spin, Empty,
    Timeline, Card, Tag, Collapse, Descriptions, Badge,
} from 'antd';
import {
    PlayCircleOutlined, DatabaseOutlined,
    FileTextOutlined, MergeCellsOutlined,
    CheckCircleOutlined, ExperimentOutlined,
} from '@ant-design/icons';
import {
    fetchProfileExtractionSnapshots,
    type ExtractionSnapshot,
    type ExtractionRunRef,
} from '../api/techBidStep4';

const { Text } = Typography;

interface RunReplayPanelProps {
    open: boolean;
    onClose: () => void;
    projectId: string;
}

const STAGE_META: Record<string, { label: string; color: string; icon: React.ReactNode }> = {
    raw_text: { label: '原始文本', color: 'cyan', icon: <FileTextOutlined /> },
    chunks: { label: 'Chunk 列表', color: 'blue', icon: <DatabaseOutlined /> },
    chunk_output: { label: 'Chunk 抽取输出', color: 'geekblue', icon: <ExperimentOutlined /> },
    prompt_input: { label: 'Prompt 输入', color: 'purple', icon: <FileTextOutlined /> },
    merge_after: { label: 'Merge 后结果', color: 'orange', icon: <MergeCellsOutlined /> },
    merge_diff: { label: 'Merge Diff', color: 'volcano', icon: <MergeCellsOutlined /> },
    supplement_check: { label: '补充抽取检查', color: 'gold', icon: <ExperimentOutlined /> },
    supplement_merged: { label: '补充抽取合并', color: 'lime', icon: <MergeCellsOutlined /> },
    final_payload: { label: '最终输出', color: 'green', icon: <CheckCircleOutlined /> },
};

const getStageLabel = (stage: string): string => {
    if (STAGE_META[stage]) return STAGE_META[stage].label;
    if (stage.startsWith('supplement_output_')) return `补充抽取: ${stage.replace('supplement_output_', '')}`;
    return stage;
};

const getStageColor = (stage: string): string => {
    if (STAGE_META[stage]) return STAGE_META[stage].color;
    if (stage.startsWith('supplement_output_')) return 'gold';
    return 'default';
};

const getStageIcon = (stage: string): React.ReactNode => {
    if (STAGE_META[stage]) return STAGE_META[stage].icon;
    return <ExperimentOutlined />;
};

// Render a JSON payload in a collapsible card
const PayloadViewer: React.FC<{ payload: unknown; maxHeight?: number }> = ({ payload, maxHeight = 300 }) => {
    const text = typeof payload === 'string'
        ? payload
        : JSON.stringify(payload, null, 2);

    // For raw text, show as-is; for JSON, show formatted
    if (typeof payload === 'string' && !payload.startsWith('{') && !payload.startsWith('[')) {
        return (
            <div style={{ maxHeight, overflow: 'auto', background: '#fafafa', padding: 12, borderRadius: 6, fontSize: 12, lineHeight: 1.7, whiteSpace: 'pre-wrap' }}>
                {text.length > 2000 ? text.slice(0, 2000) + '\n...(已截断)' : text}
            </div>
        );
    }

    return (
        <pre style={{ maxHeight, overflow: 'auto', background: '#f6f8fa', padding: 12, borderRadius: 6, fontSize: 11, lineHeight: 1.5, margin: 0 }}>
            {text.length > 5000 ? text.slice(0, 5000) + '\n...(已截断)' : text}
        </pre>
    );
};

// Group snapshots by stage for timeline rendering
interface StageGroup {
    stage: string;
    snapshots: ExtractionSnapshot[];
}

const groupByStage = (snapshots: ExtractionSnapshot[]): StageGroup[] => {
    const order = ['raw_text', 'chunks', 'prompt_input', 'chunk_output', 'merge_after', 'merge_diff', 'supplement_check'];
    const groups: Map<string, ExtractionSnapshot[]> = new Map();

    for (const snap of snapshots) {
        const key = snap.stage;
        if (!groups.has(key)) groups.set(key, []);
        groups.get(key)!.push(snap);
    }

    // Sort: known stages first in order, then unknown, then final_payload last
    const sorted = Array.from(groups.entries()).sort(([a], [b]) => {
        const idxA = order.indexOf(a);
        const idxB = order.indexOf(b);
        if (a === 'final_payload') return 1;
        if (b === 'final_payload') return -1;
        if (idxA >= 0 && idxB >= 0) return idxA - idxB;
        if (idxA >= 0) return -1;
        if (idxB >= 0) return 1;
        return a.localeCompare(b);
    });

    return sorted.map(([stage, snaps]) => ({ stage, snapshots: snaps }));
};

const RunReplayPanel: React.FC<RunReplayPanelProps> = ({ open, projectId, onClose }) => {
    const [loading, setLoading] = useState(false);
    const [runIds, setRunIds] = useState<ExtractionRunRef[]>([]);
    const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
    const [snapshots, setSnapshots] = useState<ExtractionSnapshot[]>([]);

    // Load available run IDs
    const loadRunIds = useCallback(async () => {
        if (!open || !projectId) return;
        setLoading(true);
        try {
            const { runIds: ids } = await fetchProfileExtractionSnapshots(projectId);
            setRunIds(ids);
            if (ids.length > 0 && !selectedRunId) {
                setSelectedRunId(ids[0].run_id);
            }
        } catch {
            setRunIds([]);
        } finally {
            setLoading(false);
        }
    }, [open, projectId, selectedRunId]);

    // Load snapshots for selected run
    const loadSnapshots = useCallback(async () => {
        if (!selectedRunId || !projectId) return;
        setLoading(true);
        try {
            const { snapshots: snaps } = await fetchProfileExtractionSnapshots(projectId, selectedRunId);
            setSnapshots(snaps);
        } catch {
            setSnapshots([]);
        } finally {
            setLoading(false);
        }
    }, [selectedRunId, projectId]);

    useEffect(() => { void loadRunIds(); }, [loadRunIds]);
    useEffect(() => { void loadSnapshots(); }, [loadSnapshots]);

    const stageGroups = groupByStage(snapshots);

    return (
        <Drawer
            title={<Space><PlayCircleOutlined />运行结果回放</Space>}
            placement="right"
            width={700}
            onClose={onClose}
            open={open}
            extra={
                <Select
                    style={{ width: 260 }}
                    placeholder="选择运行批次"
                    value={selectedRunId ?? undefined}
                    onChange={(v) => setSelectedRunId(v)}
                    options={runIds.map((r) => ({
                        value: r.run_id,
                        label: `Run ${r.run_id.slice(0, 8)}... · ${r.created_at}`,
                    }))}
                    loading={loading}
                />
            }
        >
            {loading ? (
                <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" tip="加载抽取快照..." /></div>
            ) : snapshots.length === 0 ? (
                <Empty description={selectedRunId ? '该批次无快照数据' : '暂无运行记录'} />
            ) : (
                <div>
                    <Descriptions size="small" bordered column={2} style={{ marginBottom: 16 }}>
                        <Descriptions.Item label="Run ID"><Text code>{selectedRunId}</Text></Descriptions.Item>
                        <Descriptions.Item label="快照数量"><Badge count={snapshots.length} style={{ backgroundColor: '#1890ff' }} /></Descriptions.Item>
                        <Descriptions.Item label="阶段数">{stageGroups.length}</Descriptions.Item>
                        <Descriptions.Item label="Chunk 数">
                            {snapshots.filter(s => s.stage === 'chunk_output').length}
                        </Descriptions.Item>
                    </Descriptions>

                    <Timeline>
                        {stageGroups.map((group) => (
                            <Timeline.Item
                                key={group.stage}
                                color={getStageColor(group.stage)}
                                dot={getStageIcon(group.stage)}
                            >
                                <Collapse
                                    size="small"
                                    items={[{
                                        key: group.stage,
                                        label: (
                                            <Space>
                                                <Tag color={getStageColor(group.stage)}>{getStageLabel(group.stage)}</Tag>
                                                {group.snapshots.length > 1 && (
                                                    <Text type="secondary" style={{ fontSize: 12 }}>
                                                        {group.snapshots.length} 条记录
                                                    </Text>
                                                )}
                                                <Text type="secondary" style={{ fontSize: 11 }}>
                                                    {group.snapshots[0]?.created_at}
                                                </Text>
                                            </Space>
                                        ),
                                        children: (
                                            <div>
                                                {group.snapshots.map((snap) => (
                                                    <Card
                                                        key={snap.id}
                                                        size="small"
                                                        style={{ marginBottom: 8 }}
                                                        title={
                                                            group.snapshots.length > 1 ? (
                                                                <Space>
                                                                    <Text style={{ fontSize: 12 }}>
                                                                        #{snap.chunk_index}
                                                                    </Text>
                                                                    {snap.chunk_type && <Tag style={{ fontSize: 11 }}>{snap.chunk_type}</Tag>}
                                                                </Space>
                                                            ) : undefined
                                                        }
                                                    >
                                                        <PayloadViewer payload={snap.payload} />
                                                    </Card>
                                                ))}
                                            </div>
                                        ),
                                    }]}
                                />
                            </Timeline.Item>
                        ))}
                    </Timeline>
                </div>
            )}
        </Drawer>
    );
};

export default RunReplayPanel;
