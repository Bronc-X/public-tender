/**
 * 风控合规面板（risk_review 步骤）
 * 从 TechBidProjectWorkbench.tsx 中拆分（CTO P0-2）
 */
import React, { useState } from 'react';
import {
    Typography, Button, Card, Space, Tag, Empty, message,
} from 'antd';
import { SafetyCertificateOutlined } from '@ant-design/icons';
import axios from 'axios';

const { Title, Text, Paragraph } = Typography;

interface RiskReviewPanelProps {
    projectId: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    risks: any[] | null | undefined;
    auditLoading: boolean;
    onAuditLoadingChange: (loading: boolean) => void;
    onRefreshRisks: () => void;
    onConfirm: () => void;
}

const RiskReviewPanel: React.FC<RiskReviewPanelProps> = ({
    projectId,
    risks,
    auditLoading,
    onAuditLoadingChange,
    onRefreshRisks,
    onConfirm,
}) => {
    const safeRisks = Array.isArray(risks) ? risks : [];
    const [auditCompleted, setAuditCompleted] = useState(false);

    const handleRunAudit = async () => {
        onAuditLoadingChange(true);
        try {
            await axios.post(`/api/tech-bid/risk/projects/${projectId}/run`);
            setAuditCompleted(true);
            message.success('质检任务已完成');
            onRefreshRisks();
        } catch {
            message.error('质检失败');
        } finally {
            onAuditLoadingChange(false);
        }
    };

    return (
        <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
                <div>
                    <Title level={4}>标书风控与合规质检</Title>
                    <Text type="secondary">基于 AI 专家的全量质检。</Text>
                </div>
                <Button
                    type="primary"
                    icon={<SafetyCertificateOutlined />}
                    loading={auditLoading}
                    onClick={handleRunAudit}
                >
                    执行全量 AI 质检
                </Button>
            </div>
            {safeRisks.length === 0 ? (
                <Empty description="暂无风险记录" />
            ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                    {safeRisks.map((risk) => (
                        <Card
                            key={risk.id || `${risk.risk_type}-${risk.risk_detail}`}
                            style={{ width: '100%', borderLeft: `4px solid ${risk.risk_level === 'high' ? '#ff4d4f' : '#faad14'}` }}
                        >
                            <Space>
                                <Tag color={risk.risk_type === 'similarity' ? 'orange' : 'red'}>{risk.risk_type}</Tag>
                                <Text strong>{risk.chapter_name}</Text>
                            </Space>
                            <Paragraph style={{ marginTop: 8 }}>{risk.risk_detail}</Paragraph>
                        </Card>
                    ))}
                </div>
            )}
            {((auditCompleted && safeRisks.length === 0) || (safeRisks.length > 0 && safeRisks.every((r) => r.status !== 'open'))) && (
                <div style={{ marginTop: 32, textAlign: 'center' }}>
                    <Button type="primary" size="large" onClick={onConfirm}>质检通过，准备定稿导出</Button>
                </div>
            )}
        </div>
    );
};

export default RiskReviewPanel;
