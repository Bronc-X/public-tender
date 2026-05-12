import React, { useState, useEffect, useCallback } from 'react';
import { Input, Button, Space, Typography, Tag, message, Spin, Empty, Image, Tooltip, Table, Alert, Select } from 'antd';
import {
  LeftOutlined,
  CheckCircleOutlined,
  ZoomInOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  SyncOutlined
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import fileCenterApi from '../api/fileCenter';
import type { AuditItem } from '../api/fileCenter';
import { AUDIT_OBJECT_TYPE_OPTIONS } from '../constants/auditObjectTypes';

const { Text, Title } = Typography;

interface ExtractionItem {
  id: string;
  title: string;
  summary: string;
  content: string;
  confidence: number;
  source_page: string;
}

interface AuditDetailProps {
  auditId?: string;
  isDrawer?: boolean;
  onAuditSuccess?: () => void;
}

const AuditDetail: React.FC<AuditDetailProps> = ({ auditId, isDrawer, onAuditSuccess }) => {
  const { id: paramId } = useParams<{ id: string }>();
  const activeId = auditId || paramId;
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<AuditItem | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [confirmedText, setConfirmedText] = useState("");
  const [extractionResults, setExtractionResults] = useState<ExtractionItem[]>([]);
  const [showPreview, setShowPreview] = useState(false);
  const [targetType, setTargetType] = useState<string>("general");

  const fetchDetail = useCallback(async () => {
    if (!activeId) return;
    setLoading(true);
    try {
      const d = await fileCenterApi.getAuditDetail(activeId);
      setData(d);
      setConfirmedText(d.ocr_text || "");
      setTargetType(d.object_type || "general");

      if (d.extracted_data) {
        try {
          const parsed = JSON.parse(d.extracted_data);
          let rows = Array.isArray(parsed) ? parsed : [];
          // 人员档案：直接解析，后端已完成归一化
          if (d.object_type === 'person') {
            rows = Array.isArray(parsed) ? parsed : [];
          }
          setExtractionResults(rows);
        } catch {
          setExtractionResults([]);
        }
      } else {
        setExtractionResults([]);
      }
    } catch (err) {
      console.error('Failed to fetch audit detail:', err);
      message.error('加载审核详情失败');
    } finally {
      setLoading(false);
    }
  }, [activeId]);

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  const onConfirm = useCallback(async () => {
    if (!activeId || !data) return;
    setConfirming(true);
    try {
      await fileCenterApi.confirmAudit(activeId, {
        file_id: data.file_id,
        extracted_items: extractionResults,
        object_type: targetType,
        confirmed_text: confirmedText
      });
      message.success('审核已通过，数据资产已成功入库沉淀！');
      if (isDrawer && onAuditSuccess) {
        onAuditSuccess();
      } else {
        navigate('/file-center/audit');
      }
    } catch (err) {
      console.error('Finalize error:', err);
      message.error('审核入库失败');
    } finally {
      setConfirming(false);
    }
  }, [activeId, data, extractionResults, confirmedText, targetType, navigate, isDrawer, onAuditSuccess]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: isDrawer ? '400px' : '100vh' }}><Spin size="large" /></div>;
  if (!data) return <Empty description="任务数据不存在" />;

  const isKnowledgeType = ['method', 'equipment', 'standard', 'performance', 'risk', 'tech_bid_import', 'qualification', 'person', 'laborcontract'].includes(targetType);
  const isPersonAudit = targetType === 'person' || data.object_type === 'person';
  const previewUrl = `/api/files/download/${data.file_id}/view.pdf`;
  /** 上传阶段常把 PDF 记成 application/octet-stream，仅靠 mime 会误走 <Image>，PDF 无法显示 */
  const isPdfOriginal =
    (data.mime_type && data.mime_type.toLowerCase().includes('pdf')) ||
    (data.file_name && data.file_name.toLowerCase().endsWith('.pdf')) ||
    (data.stored_path && data.stored_path.toLowerCase().endsWith('.pdf'));

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: isDrawer ? 'calc(100vh - 100px)' : '100vh',
      overflow: 'hidden',
      margin: isDrawer ? '0' : '-24px',
      background: '#fff'
    }}>
      {!isDrawer && (
        <div style={{
          flexShrink: 0,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '16px 24px',
          borderBottom: '1px solid #e8e8e8',
          background: '#fff',
          zIndex: 10,
          boxShadow: '0 1px 4px rgba(0,0,0,0.06)'
        }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <Space size="middle">
              <Button
                icon={<LeftOutlined />}
                onClick={() => navigate('/file-center/audit')}
                style={{ borderRadius: 6 }}
              />
              <Title level={3} style={{ margin: 0 }}>
                智能审核校验案台
              </Title>
            </Space>
            <div style={{ paddingLeft: 48 }}>
              <Text type="secondary" style={{ fontSize: 13 }}>待核件：</Text>
              <Tooltip title={data.file_name}>
                <Text
                  type="secondary"
                  style={{ fontSize: 13, fontWeight: 500 }}
                >
                  {data.file_name && data.file_name.length > 50 ? `${data.file_name.slice(0, 50)}...` : data.file_name}
                </Text>
              </Tooltip>
              <Select
                size="small"
                value={targetType}
                onChange={setTargetType}
                style={{ width: 120, marginLeft: 12 }}
                options={[...AUDIT_OBJECT_TYPE_OPTIONS]}
              />
            </div>
          </div>
          <Space size="middle">
            <Button
              type="primary"
              size="large"
              icon={<CheckCircleOutlined />}
              loading={confirming}
              onClick={onConfirm}
              disabled={data.audit_status === 'processing'}
              style={{ borderRadius: 8, paddingLeft: 24, paddingRight: 24 }}
            >
              {data.audit_status === 'processing' ? '正在智能识别...' : '完成确认并同步入库'}
            </Button>
          </Space>
        </div>
      )}

      {isDrawer && (
        <div style={{ padding: '0 0 16px 0', borderBottom: '1px solid #f0f0f0', marginBottom: 0, flexShrink: 0, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <Text strong style={{ marginRight: 12 }}>{data.file_name}</Text>
            <Select
              size="small"
              value={targetType}
              onChange={setTargetType}
              style={{ width: 120 }}
              options={[...AUDIT_OBJECT_TYPE_OPTIONS]}
            />
          </div>
          <Button
            type="primary"
            size="large"
            icon={<CheckCircleOutlined />}
            loading={confirming}
            onClick={onConfirm}
            disabled={data.audit_status === 'processing'}
            style={{ borderRadius: 8 }}
          >
            {data.audit_status === 'processing' ? '识别中...' : '提交确认'}
          </Button>
        </div>
      )}

      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'row',
        overflow: 'hidden'
      }}>

        {/* Left: Original Preview */}
        <div style={{
          width: '50%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          borderRight: '1px solid #e8e8e8',
          background: '#ffffff',
          overflow: 'hidden'
        }}>
          <div style={{
            flexShrink: 0,
            padding: '10px 16px',
            background: '#eef0f2',
            borderBottom: '1px solid #e0e0e0',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center'
          }}>
            <Text type="secondary" style={{ fontSize: 11, letterSpacing: 2, fontWeight: 600, textTransform: 'uppercase' }}>
              原件对照
            </Text>
            <Tag icon={<ZoomInOutlined />} color="default" style={{ fontSize: 11 }}>
              在线预览 (PDF/图片)
            </Tag>
          </div>
          <div style={{
            flex: 1,
            overflow: 'hidden',
            minHeight: 0,
            padding: isPdfOriginal ? 0 : 32,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: isPdfOriginal ? 'stretch' : 'flex-start'
          }}>
            {isPdfOriginal ? (
              <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#fff' }}>
                <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
                  <embed
                    src={previewUrl}
                    type="application/pdf"
                    style={{
                      width: '100%',
                      height: '100%',
                      border: 'none',
                      position: 'absolute',
                      top: 0,
                      left: 0
                    }}
                  />
                </div>
                <div style={{ padding: '8px 16px', background: '#fff', borderTop: '1px solid #f0f0f0', display: 'flex', justifyContent: 'center', gap: '12px' }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>无法预览？</Text>
                  <Button size="small" type="link" onClick={() => window.open(previewUrl)} style={{ fontSize: 12, padding: 0 }}>在新窗口打开</Button>
                  <Button size="small" type="link" onClick={() => window.open(previewUrl)} style={{ fontSize: 12, padding: 0 }}>下载原件</Button>
                </div>
              </div>
            ) : (
              <Image
                src={previewUrl}
                alt="原件预览"
                style={{
                  maxWidth: '100%',
                  maxHeight: '80vh',
                  objectFit: 'contain',
                  borderRadius: 4,
                  boxShadow: '0 6px 24px rgba(0,0,0,0.12)',
                  border: '1px solid #e0e0e0'
                }}
                preview={{
                  mask: <div style={{ fontSize: 12 }}><ZoomInOutlined style={{ marginRight: 4 }} />查看原图</div>
                }}
              />
            )}
          </div>
        </div>

        {/* Right: AI Results & Verification */}
        <div style={{
          width: '50%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          background: '#fff',
          overflow: 'hidden'
        }}>
          <div style={{
            flexShrink: 0,
            padding: '10px 16px',
            background: '#fafafa',
            borderBottom: '1px solid #e8e8e8',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center'
          }}>
            <Text type="secondary" style={{ fontSize: 11, letterSpacing: 2, fontWeight: 600, textTransform: 'uppercase' }}>
              {isKnowledgeType ? '结构化提取结果' : '文本内容校准'}
            </Text>
            <Space>
              <Button.Group size="small">
                <Button
                  type={!showPreview ? "primary" : "default"}
                  onClick={() => setShowPreview(false)}
                >
                  编辑
                </Button>
                <Button
                  type={showPreview ? "primary" : "default"}
                  onClick={() => setShowPreview(true)}
                >
                  预览
                </Button>
              </Button.Group>
            </Space>
          </div>

          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', padding: (isKnowledgeType || showPreview || data.audit_status === 'processing') ? 0 : 24, overflow: 'auto' }}>
            {data.audit_status === 'processing' ? (
              <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', height: '100%', gap: 20, background: '#fafafa' }}>
                <Spin size="large" />
                <div style={{ textAlign: 'center' }}>
                  <Title level={4}>AI 正在深度解析中...</Title>
                  <Text type="secondary">正在提取文档结构化数据，请稍候。解析完成后列表将自动刷新。</Text>
                  <div style={{ marginTop: 16 }}>
                    <Button icon={<SyncOutlined />} onClick={fetchDetail}>手动刷新</Button>
                  </div>
                </div>
              </div>
            ) : isKnowledgeType && !showPreview ? (
              <div style={{ padding: 24 }}>
                <Alert
                  message={isPersonAudit
                    ? 'AI 建议：请核对提取出的 9 项人员/证书核心信息（姓名、身份证、类别、编号等）。如有缺失项，请手动补充。'
                    : 'AI 建议：已自动提炼以下核心资产。请核对并修正字段信息。'}
                  type="info"
                  showIcon
                  icon={<InfoCircleOutlined />}
                  style={{ marginBottom: 20 }}
                />
                <Table
                  dataSource={extractionResults}
                  pagination={false}
                  rowKey="id"
                  size="small"
                  columns={[
                    {
                      title: isPersonAudit ? '类目名称' : '条目名称',
                      dataIndex: 'title',
                      width: '30%',
                      render: (v, record) => (
                        <Input
                          defaultValue={v}
                          variant="filled"
                          onChange={(e) => {
                            const newResults = [...extractionResults];
                            const idx = newResults.findIndex(r => r.id === record.id);
                            newResults[idx].title = e.target.value;
                            setExtractionResults(newResults);
                          }}
                        />
                      )
                    },
                    {
                      title: '核心摘要',
                      dataIndex: 'content',
                      render: (v, record) => (
                        <Input.TextArea
                          defaultValue={v}
                          variant="filled"
                          autoSize={{ minRows: 2, maxRows: 10 }}
                          onChange={(e) => {
                            const newResults = [...extractionResults];
                            const idx = newResults.findIndex(r => r.id === record.id);
                            newResults[idx].content = e.target.value;
                            setExtractionResults(newResults);
                          }}
                        />
                      )
                    },
                    {
                      title: '页码',
                      dataIndex: 'source_page',
                      width: 70,
                      render: (p) => <Tag color="blue">P{p}</Tag>
                    },
                    {
                      title: '',
                      width: 50,
                      render: (_, record) => (
                        <Button
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={() => {
                            setExtractionResults(extractionResults.filter(r => r.id !== record.id));
                          }}
                        />
                      )
                    }
                  ]}
                />
              </div>
            ) : (
              showPreview ? (
                <div className="markdown-preview" style={{ padding: 40, lineHeight: '2' }}>
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>
                    {data.ai_clean_text || confirmedText}
                  </ReactMarkdown>
                </div>
              ) : (
                <Input.TextArea
                  value={confirmedText}
                  onChange={(e) => setConfirmedText(e.target.value)}
                  variant="borderless"
                  style={{
                    flex: 1,
                    resize: 'none',
                    lineHeight: '1.8',
                    fontSize: 15,
                    fontFamily: '"JetBrains Mono", Menlo, Monaco, Consolas, monospace',
                    padding: 30,
                    backgroundColor: '#fafafa'
                  }}
                  placeholder="在此校对识别文字..."
                />
              )
            )}
          </div>

          <div style={{ flexShrink: 0, background: '#fff', minHeight: 64, display: 'flex', flexDirection: 'column', borderTop: '1px solid #f0f0f0' }}>
            <div style={{ padding: '12px 24px' }}>
              <Text style={{ fontSize: 12, color: '#8c8c8c' }}>
                <span style={{ fontWeight: 600, color: '#1890ff', marginRight: 8 }}>智能审核指南：</span>
                确认通过后内容将自动归档至对应库。单条记录支持 Markdown 语法。
              </Text>
            </div>
          </div>

        </div>

      </div>
    </div>
  );
};

export default AuditDetail;
