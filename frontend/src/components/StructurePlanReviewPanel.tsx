import React from 'react';
import { 
  CheckCircleOutlined, 
  ExclamationCircleOutlined, 
  ArrowUpOutlined, 
  BlockOutlined, 
  AppstoreAddOutlined, 
  CheckOutlined 
} from '@ant-design/icons';
import type { StructurePlan } from '../api/techBidStep4';

interface Props {
  plan: StructurePlan;
  onApprove: () => void;
  onReject: (reason: string) => void;
  loading?: boolean;
}

const actionIcons: Record<string, React.ReactNode> = {
  promote: <ArrowUpOutlined style={{ color: '#1890ff' }} />,
  split: <BlockOutlined style={{ color: '#722ed1' }} />,
  insert: <AppstoreAddOutlined style={{ color: '#52c41a' }} />,
  move: <ArrowUpOutlined style={{ color: '#fa8c16', transform: 'rotate(90deg)' }} />,
  keep: <CheckCircleOutlined style={{ color: '#8c8c8c' }} />,
};

const actionLabels: Record<string, string> = {
  promote: '建议升格为章',
  split: '建议拆分章节',
  insert: '建议新增专项章',
  move: '建议调整顺序',
  keep: '保留原结构',
};

export const StructurePlanReviewPanel: React.FC<Props> = ({ plan, onApprove, onReject, loading }) => {
  const [rejectReason, setRejectReason] = React.useState('');
  const [showRejectModal, setShowRejectModal] = React.useState(false);

  // Calculate a mock score based on adjustments vs keep
  const total = plan.adjustments?.length || 0;
  const changed = plan.adjustments?.filter(a => a.action !== 'keep').length || 0;
  const personalizationScore = total > 0 ? (changed / total * 100).toFixed(0) : '0';

  return (
    <div style={{ padding: '20px', background: '#fff', borderRadius: '12px', border: '1px solid #f0f0f0', boxShadow: '0 4px 12px rgba(0,0,0,0.05)' }}>
      <div style={{ marginBottom: '24px', display: 'flex', justifyContent: 'space-between', alignItems: 'start' }}>
        <div>
          <h3 style={{ fontSize: '20px', fontWeight: 600, margin: 0, color: '#1a1a1a', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <ExclamationCircleOutlined style={{ color: '#1890ff' }} />
            弹性骨架：AI 结构优化建议
            <span style={{ 
              fontSize: '12px', 
              fontWeight: 500, 
              padding: '2px 8px', 
              borderRadius: '12px', 
              background: '#e6f7ff', 
              color: '#1890ff',
              marginLeft: '8px'
            }}>
              个性化适配度: {personalizationScore}%
            </span>
          </h3>
          <p style={{ color: '#8c8c8c', marginTop: '8px', fontSize: '14px', maxWidth: '600px' }}>
            系统已深度解析招标文件（Tender Profile），识别出标准行业骨架与本项目事实的偏差，并生成以下调整计划。
          </p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <button 
            onClick={() => setShowRejectModal(true)}
            disabled={loading || plan.status !== 'pending'}
            style={{
              background: '#fff',
              color: '#ff4d4f',
              border: '1px solid #ff4d4f',
              padding: '8px 20px',
              borderRadius: '6px',
              fontSize: '14px',
              fontWeight: 500,
              cursor: loading || plan.status !== 'pending' ? 'not-allowed' : 'pointer',
              transition: 'all 0.3s'
            }}
          >
            拒绝并重规划
          </button>
          <button 
            onClick={onApprove}
            disabled={loading || plan.status !== 'pending'}
            style={{
              background: plan.status === 'approved' ? '#52c41a' : '#1890ff',
              color: '#fff',
              border: 'none',
              padding: '8px 24px',
              borderRadius: '6px',
              fontSize: '14px',
              fontWeight: 500,
              cursor: loading || plan.status !== 'pending' ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              boxShadow: '0 2px 4px rgba(24,144,255,0.2)',
              transition: 'all 0.3s'
            }}
          >
            {plan.status === 'approved' ? <CheckOutlined /> : null}
            {plan.status === 'approved' ? '已应用计划' : '批准并应用结构计划'}
          </button>
        </div>
      </div>

      {showRejectModal && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, 
          background: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000
        }}>
          <div style={{ background: '#fff', padding: '24px', borderRadius: '8px', width: '400px' }}>
            <h4 style={{ margin: '0 0 16px 0' }}>拒绝说明</h4>
            <textarea 
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder="请输入拒绝理由或调整建议..."
              style={{ width: '100%', height: '100px', padding: '8px', marginBottom: '16px', borderRadius: '4px', border: '1px solid #d9d9d9' }}
            />
            <div style={{ display: 'flex', justifyContent: 'end', gap: '8px' }}>
              <button onClick={() => setShowRejectModal(false)} style={{ padding: '4px 12px', border: '1px solid #d9d9d9', background: '#fff', borderRadius: '4px' }}>取消</button>
              <button 
                onClick={() => {
                  onReject(rejectReason);
                  setShowRejectModal(false);
                }} 
                style={{ padding: '4px 12px', background: '#ff4d4f', color: '#fff', border: 'none', borderRadius: '4px' }}
              >
                确认拒绝
              </button>
            </div>
          </div>
        </div>
      )}

      <div style={{ background: '#f9f9f9', padding: '16px', borderRadius: '6px', marginBottom: '20px' }}>
        <div style={{ fontWeight: 500, color: '#595959', marginBottom: '8px', fontSize: '14px' }}>优化策略理由 (Rationale):</div>
        <div style={{ color: '#262626', fontSize: '14px', lineHeight: '1.6' }}>
          {plan.rationale || "AI 正在分析招标文件结构，已根据提取到的核心事实自动识别出该项目需要突破通用骨架的部分。"}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '16px' }}>
        {plan.adjustments?.map((adj, idx) => (
          <div 
            key={idx} 
            style={{ 
              border: '1px solid #e8e8e8', 
              borderRadius: '6px', 
              padding: '16px', 
              background: adj.action === 'keep' ? '#fafafa' : '#fff',
              position: 'relative'
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
              <span style={{ fontSize: '18px' }}>{actionIcons[adj.action] || actionIcons['keep']}</span>
              <span style={{ 
                background: adj.action === 'promote' ? '#e6f7ff' : adj.action === 'split' ? '#f9f0ff' : '#f5f5f5',
                color: adj.action === 'promote' ? '#1890ff' : adj.action === 'split' ? '#722ed1' : '#595959',
                padding: '2px 8px',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: 500
              }}>
                {actionLabels[adj.action] || '其他'}
              </span>
            </div>
            
            <div style={{ fontWeight: 600, fontSize: '15px', color: '#262626', marginBottom: '8px' }}>
              {adj.target_name}
            </div>
            
            <div style={{ fontSize: '13px', color: '#595959', lineHeight: '1.5' }}>
              {adj.reason}
            </div>

            {adj.priority > 70 && (
              <div style={{ 
                position: 'absolute', 
                top: '12px', 
                right: '12px',
                width: '6px',
                height: '6px',
                borderRadius: '50%',
                background: '#ff4d4f'
              }} title="高优先级调整" />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
