import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Form, Input, Button, Space, Typography, Tag,
  message, Row, Col, Select, Table, Modal, Tabs, Popconfirm, Tooltip, Badge,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PartitionOutlined, ArrowLeftOutlined, ExperimentOutlined, LoadingOutlined,
} from '@ant-design/icons';
import axios from 'axios';

const { Text, Paragraph } = Typography;

interface LogicalChapter {
  id: string;
  name: string;
  description: string;
  is_mandatory: boolean;
  unit_pool: string[];
  subsection_pool: Record<string, string[]>;
  is_core_chapter: boolean;
  can_reorder: boolean;
  can_split: boolean;
  can_merge: boolean;
  can_insert_before: boolean;
  can_insert_after: boolean;
  priority_range: string[];
  fact_type_preference: string[];
}

export interface IndustrySkeletonRecord {
  id: string;
  industry_name: string;
  parent_id?: string;
  logical_chapters_json: string;
  common_section_pool_json?: string;
  industry_keywords_json?: string;
  title_candidate_pool_json?: string;
  matching_rules_json?: string;
  updated_at: string;
  children?: IndustrySkeletonRecord[];
}

function isRootSkeletonItem(item: IndustrySkeletonRecord): boolean {
  return item.parent_id === null || item.parent_id === undefined || item.parent_id === '';
}

const mdToChapters = (newMd: string): LogicalChapter[] => {
  const lines = newMd.split('\n');
  const chapters: LogicalChapter[] = [];
  let currentChapter: LogicalChapter | null = null;
  let currentUnit: string | null = null;

  lines.forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed) return;

    // Detect Levels
    const isL1 = /^#+\s+第[一二三四五六七八九十百]+章/.test(trimmed) || /^##\s+[^第]+/.test(trimmed);
    const isL2 = !isL1 && (/^#+\s+第[一二三四五六七八九十百]+节/.test(trimmed) || trimmed.startsWith('- ') || (trimmed.startsWith('## ') && trimmed.includes('节')));
    const isL3 = !isL1 && !isL2 && (trimmed.startsWith('### ') || trimmed.startsWith('#### ') || line.startsWith('  - ') || /^[\s-]*（[一二三四五六七八九十百]+）/.test(trimmed));

    if (isL1) {
      if (currentChapter) chapters.push(currentChapter);
      const namePart = trimmed.replace(/^#+\s+/, '');
      const isMandatory = namePart.includes('(必选)');
      const flagMatch = namePart.match(/\{([^}]+)\}/);
      const flags = flagMatch ? flagMatch[1].split(',').map((f) => f.trim().toLowerCase()) : [];
      const name = namePart.replace('(必选)', '').replace(/\{[^}]+\}/, '').trim();

      currentChapter = {
        id: `CH${chapters.length + 1}`,
        name,
        description: '',
        is_mandatory: isMandatory,
        unit_pool: [],
        subsection_pool: {},
        is_core_chapter: flags.includes('core'),
        can_reorder: !flags.includes('fix'),
        can_split: flags.includes('split'),
        can_merge: flags.includes('merge'),
        can_insert_before: true,
        can_insert_after: true,
        priority_range: [],
        fact_type_preference: [],
      };
      currentUnit = null;
    } else if (isL2 && currentChapter) {
      currentUnit = trimmed.replace(/^#+\s+/, '').replace(/^-+\s+/, '').trim();
      currentChapter.unit_pool.push(currentUnit);
    } else if (isL3 && currentChapter && currentUnit) {
      const subName = trimmed.replace(/^#+\s+/, '').replace(/^-+\s+/, '').trim();
      if (!currentChapter.subsection_pool[currentUnit]) {
        currentChapter.subsection_pool[currentUnit] = [];
      }
      currentChapter.subsection_pool[currentUnit].push(subName);
    } else if (currentChapter) {
      currentChapter.description += (currentChapter.description ? '\n' : '') + trimmed;
    }
  });

  if (currentChapter) chapters.push(currentChapter);
  return chapters;
};
interface ChapterMarkdownBuilderProps {
  value: string;
  onChange: (v: string) => void;
  label: string;
}

const ChapterMarkdownBuilder: React.FC<ChapterMarkdownBuilderProps> = ({
  value,
  onChange,
  label,
}) => {
  const [mode, setMode] = useState<'visual' | 'raw'>('visual');
  const [internalMd, setInternalMd] = useState('');

  // Helper to convert JSON value to Markdown string
  const jsonToMd = useCallback((jsonValue: string) => {
    try {
      const data = JSON.parse(jsonValue || '[]');
      if (Array.isArray(data)) {
        return data
          .map((item: LogicalChapter) => {
            const sections = (item.unit_pool || [])
              .map((unit) => {
                const subs = (item.subsection_pool?.[unit] || [])
                  .map((s) => `### ${s}`)
                  .join('\n');
                return `## ${unit}${subs ? '\n' + subs : ''}`;
              })
              .join('\n');

            const flags = [
              item.is_core_chapter ? 'core' : '',
              item.can_reorder === false ? 'fix' : '',
              item.can_split ? 'split' : '',
              item.can_merge ? 'merge' : '',
            ]
              .filter(Boolean)
              .join(', ');
            const flagStr = flags ? ` {${flags}}` : '';
            return `## ${item.name}${item.is_mandatory ? ' (必选)' : ''}${flagStr}\n${item.description || ''}${sections ? '\n' + sections : ''}`;
          })
          .join('\n\n');
      }
    } catch {
      return jsonValue;
    }
    return '';
  }, []);

  // Sync prop value to internal state only on mount or when mode changes to visual
  // This prevents typing being interrupted by the controlled 'value' prop
  useEffect(() => {
    if (mode === 'visual') {
      const md = jsonToMd(value);
      setInternalMd((prev) => (prev === md ? prev : md));
    }
  }, [value, mode, jsonToMd]);

  const handleMdChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newMd = e.target.value;
    setInternalMd(newMd);
    const chapters = mdToChapters(newMd);
    onChange(JSON.stringify(chapters, null, 2));
  };

  return (
    <div className="border rounded-md overflow-hidden bg-white shadow-sm">
      <div className="bg-gray-50 px-4 py-2 border-b flex justify-between items-center">
        <Text strong style={{ fontSize: '12px' }}>
          <PartitionOutlined /> {label}
        </Text>
        <Tabs
          size="small"
          activeKey={mode}
          onChange={(k) => setMode(k as 'visual' | 'raw')}
          className="mb-[-10px] chapters-builder-tabs"
          items={[
            { key: 'visual', label: 'Markdown 快速构建' },
            { key: 'raw', label: '底层 JSON 源码' },
          ]}
        />
      </div>
      <div className="p-4">
        {mode === 'visual' ? (
          <>
            <div className="mb-2 bg-blue-50 p-2 rounded text-[11px] text-blue-600">
              支持弹性语法：<b>## 章节 {`{core, split, fix}`}</b>。core: 核心主章，split: 允许拆分，fix: 禁止重排。<b>(必选)</b> 设为必填。
            </div>
            <Input.TextArea
              value={internalMd}
              onChange={handleMdChange}
              rows={10}
              autoSize={{ minRows: 8, maxRows: 16 }}
              placeholder={`示例：\n## 第一章 施工组织策划 (必选) {core, fix}\n- 编制说明\n\n## 停气碰口专项 {split}\n- 碰口工艺`}
              style={{ fontFamily: 'monospace', fontSize: '13px', lineHeight: '1.6' }}
            />
          </>
        ) : (
          <Input.TextArea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            rows={10}
            autoSize={{ minRows: 8, maxRows: 16 }}
            style={{ fontFamily: 'monospace', fontSize: '12px', backgroundColor: '#fdfdfd' }}
          />
        )}
      </div>
    </div>
  );
};

export interface SystemSettingsSkeletonTabProps {
  searchParams: URLSearchParams;
  setSearchParams: (
    nextInit: URLSearchParams | ((prev: URLSearchParams) => URLSearchParams),
    navigateOptions?: { replace?: boolean; state?: unknown }
  ) => void;
}

/** 技术标骨架目录 Tab：须为模块级组件，避免在父组件内联定义导致每次父级重渲染时子树被卸载/白屏 */
export const SystemSettingsSkeletonTab: React.FC<SystemSettingsSkeletonTabProps> = ({ searchParams, setSearchParams }) => {
  const [skeletonData, setSkeletonData] = useState<IndustrySkeletonRecord[]>([]);
  const [allSkeletonData, setAllSkeletonData] = useState<IndustrySkeletonRecord[]>([]);
  const [skeletonLoading, setSkeletonLoading] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [editingSkeleton, setEditingSkeleton] = useState<IndustrySkeletonRecord | null>(null);
  const [skeletonForm] = Form.useForm();
  /** 任务队列状态 */
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [aiQueue, setAiQueue] = useState<string[]>([]);
  const [currentProcessingId, setCurrentProcessingId] = useState<string | null>(null);
  const [generationStatus, setGenerationStatus] = useState<Record<string, 'idle' | 'queued' | 'processing' | 'done' | 'error'>>({});

  /** 打开「添加二级分类」时捕获的一级 id；onFinish 不包含 disabled 的 parent_id，须用此值保证 POST 必带 parent_id */
  const parentIdForNewSubRef = React.useRef<string | null>(null);

  const categoryParam = searchParams.get('categoryId');
  const currentParentId = categoryParam || null;

  const currentParent = allSkeletonData.find((cat) => cat.id === currentParentId);

  const updateCategoryId = (id: string | null) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (id) next.set('categoryId', id);
        else next.delete('categoryId');
        return next;
      },
      { replace: true },
    );
  };

  const fetchSkeletons = React.useCallback(async () => {
    setSkeletonLoading(true);
    try {
      const res = await axios.get<unknown>('/api/settings/industry-skeletons');
      const raw = res.data;
      const rawData: IndustrySkeletonRecord[] = Array.isArray(raw) ? (raw as IndustrySkeletonRecord[]) : [];

      setAllSkeletonData(rawData);

      // Filtering logic based on currentParentId
      if (currentParentId) {
        const subCategories = rawData.filter((item) => item.parent_id === currentParentId);
        setSkeletonData(subCategories);
      } else {
        const rootCategories = rawData.filter(isRootSkeletonItem);
        setSkeletonData(rootCategories);
      }
    } catch {
      message.error('加载骨架数据失败');
    } finally {
      setSkeletonLoading(false);
    }
  }, [currentParentId]);

  useEffect(() => {
    fetchSkeletons();
  }, [fetchSkeletons, currentParentId]);

  const handleEdit = (record: IndustrySkeletonRecord) => {
    parentIdForNewSubRef.current = null;
    setEditingSkeleton(record);
    skeletonForm.setFieldsValue(record);
    setEditModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await axios.delete(`/api/settings/industry-skeletons/${id}`);
      message.success('已删除骨架');
      await fetchSkeletons();
    } catch {
      message.error('删除失败');
    }
  };

  const handleSave = async (values: Partial<IndustrySkeletonRecord>) => {
    try {
      let parentIdToSend: string | undefined;
      // CRITICAL FIX: Ensure parentId is correctly identified for L2 items
      if (!editingSkeleton) {
        // If creating new: use the ref if set, or the current view's parentId
        parentIdToSend = parentIdForNewSubRef.current || currentParentId || undefined;
      } else {
        // If editing: preserve existing parent_id
        parentIdToSend = editingSkeleton.parent_id || undefined;
      }

      const payload: Record<string, unknown> = {
        industry_name: values.industry_name,
        logical_chapters_json:
          values.logical_chapters_json ?? editingSkeleton?.logical_chapters_json ?? '[]',
        common_section_pool_json:
          values.common_section_pool_json ?? editingSkeleton?.common_section_pool_json ?? null,
        industry_keywords_json:
          values.industry_keywords_json ?? editingSkeleton?.industry_keywords_json ?? null,
        title_candidate_pool_json:
          values.title_candidate_pool_json ?? editingSkeleton?.title_candidate_pool_json ?? null,
        matching_rules_json:
          values.matching_rules_json ?? editingSkeleton?.matching_rules_json ?? null,
      };

      if (editingSkeleton?.id) {
        payload.id = editingSkeleton.id;
      }
      if (parentIdToSend) {
        payload.parent_id = parentIdToSend;
      }

      // Query params and Header fallback for backend compatibility
      const requestConfig: { params?: Record<string, string>; headers?: Record<string, string> } = {};
      if (parentIdToSend) {
        requestConfig.params = { categoryId: parentIdToSend };
        requestConfig.headers = { 'X-Skeleton-Parent-Id': parentIdToSend };
      }

      await axios.post('/api/settings/industry-skeletons', payload, requestConfig);
      message.success('骨架已更新');
      parentIdForNewSubRef.current = null;
      setEditModalVisible(false);
      await fetchSkeletons();
    } catch (err) {
      console.error('Save failed:', err);
      message.error('保存失败，请检查数据格式或网络连接');
    }
  };

  const handleGenerateAI = async () => {
    const values = skeletonForm.getFieldsValue();
    const industryName = values.industry_name;
    const parentId = values.parent_id || currentParentId;

    if (!industryName) {
      message.warning('请先输入分类名称后再使用 AI 生成');
      return;
    }

    setAiLoading(true);
    const hide = message.loading('AI 正在思考中，请稍候...', 0);
    try {
      // Use ID if available, otherwise pass names in body
      const url = editingSkeleton?.id
        ? `/api/settings/industry-skeletons/${editingSkeleton.id}/generate`
        : `/api/settings/industry-skeletons/undefined/generate`;

      const res = await axios.post(url, {
        industryName,
        parentId,
      });
      const draft = res.data;

      // Parse Markdown into JSON before saving
      const chaptersJson = JSON.stringify(mdToChapters(draft.logical_chapters_markdown), null, 2);

      // Update form fields with correctly formatted data
      skeletonForm.setFieldsValue({
        logical_chapters_json: chaptersJson,
        industry_keywords_json: JSON.stringify(draft.industry_keywords, null, 2),
        title_candidate_pool_json: JSON.stringify(draft.title_candidate_pool, null, 2),
        common_section_pool_json: JSON.stringify(draft.common_section_pool, null, 2),
      });

      message.success('AI 已生成骨架草案，请审核后保存');
    } catch (err: unknown) {
      const errorMsg = err instanceof Error ? err.message : String(err);
      console.error('AI Generation failed:', err);
      message.error(errorMsg || 'AI 生成失败，请重试');
    } finally {
      setAiLoading(false);
      hide();
    }
  };

  const handleAdd = () => {
    parentIdForNewSubRef.current = null;
    setEditingSkeleton(null);
    skeletonForm.resetFields();
    skeletonForm.setFieldsValue({
      logical_chapters_json: '[]',
      common_section_pool_json: '[]',
      industry_keywords_json: '[]',
      title_candidate_pool_json: '{}',
      matching_rules_json: '{}',
    });
    setEditModalVisible(true);
  };

  /** 处理单项 AI 生成（带自动保存逻辑） */
  const runAIGeneration = useCallback(async (id: string, industryName: string, parentId?: string) => {
    setCurrentProcessingId(id);
    setGenerationStatus(prev => ({ ...prev, [id]: 'processing' }));
    try {
      const url = `/api/settings/industry-skeletons/${id}/generate`;
      const res = await axios.post(url, { industryName, parentId });
      const draft = res.data;

      // 1. 解析结果
      const chaptersJson = JSON.stringify(mdToChapters(draft.logical_chapters_markdown), null, 2);

      // 2. 自动存入数据库
      const payload = {
        id,
        industry_name: industryName,
        parent_id: parentId,
        logical_chapters_json: chaptersJson,
        industry_keywords_json: JSON.stringify(draft.industry_keywords, null, 2),
        title_candidate_pool_json: JSON.stringify(draft.title_candidate_pool, null, 2),
        common_section_pool_json: JSON.stringify(draft.common_section_pool, null, 2),
      };

      await axios.post('/api/settings/industry-skeletons', payload);
      setGenerationStatus(prev => ({ ...prev, [id]: 'done' }));
      fetchSkeletons(); // 刷新表格
    } catch (err) {
      console.error('AI Gen failed:', err);
      setGenerationStatus(prev => ({ ...prev, [id]: 'error' }));
      message.error(`[${industryName}] AI 生成失败`);
    } finally {
      setCurrentProcessingId(null);
    }
  }, [fetchSkeletons]);

  /** 队列调度器 */
  useEffect(() => {
    if (!currentProcessingId && aiQueue.length > 0) {
      const nextId = aiQueue[0];
      const record = allSkeletonData.find(item => item.id === nextId);
      if (record) {
        setAiQueue(prev => prev.slice(1));
        runAIGeneration(record.id, record.industry_name, record.parent_id);
      } else {
        // ID 不在当前列表，跳过
        setAiQueue(prev => prev.slice(1));
      }
    }
  }, [aiQueue, currentProcessingId, allSkeletonData, runAIGeneration]);

  const addToQueue = (ids: string[]) => {
    const validIds = ids.filter(id => {
      const status = generationStatus[id];
      return status !== 'queued' && status !== 'processing';
    });

    if (validIds.length === 0) return;

    setGenerationStatus(prev => {
      const update: Record<string, 'queued'> = {};
      validIds.forEach(id => { update[id] = 'queued'; });
      return { ...prev, ...update };
    });
    setAiQueue(prev => [...prev, ...validIds]);
    message.info(`已将 ${validIds.length} 项加入生成队列`);
  };

  return (
    <Card
      title={
        <Space>
          {currentParentId && (
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => updateCategoryId(null)} />
          )}
          <Text strong>
            {currentParentId ? `二级分类：${currentParent?.industry_name || '加载中...'}` : '行业目录骨架库 (一级大类)'}
          </Text>
        </Space>
      }
      className="shadow-sm border-none"
      extra={
        <Space>
          <Button
            type="primary"
            icon={currentParentId ? <PlusOutlined /> : undefined}
            onClick={() => {
              if (currentParentId) {
                parentIdForNewSubRef.current = currentParentId;
                setEditingSkeleton(null);
                skeletonForm.resetFields();
                skeletonForm.setFieldsValue({
                  parent_id: currentParentId,
                  logical_chapters_json: currentParent?.logical_chapters_json || '[]',
                  common_section_pool_json: currentParent?.common_section_pool_json || '[]',
                  industry_keywords_json: currentParent?.industry_keywords_json || '[]',
                  title_candidate_pool_json: currentParent?.title_candidate_pool_json || '{}',
                });
                setEditModalVisible(true);
              } else {
                handleAdd();
              }
            }}
          >
            {currentParentId ? '添加二级分类' : '添加一级大类'}
          </Button>

          {currentParentId && selectedRowKeys.length > 0 && (
            <Button
              icon={<ExperimentOutlined />}
              onClick={() => addToQueue(selectedRowKeys.map(k => String(k)))}
              disabled={!!currentProcessingId || aiQueue.length > 0}
              style={{ backgroundColor: '#f0faff', color: '#096dd9', borderColor: '#91d5ff' }}
            >
              批量 AI 生成 ({selectedRowKeys.length})
            </Button>
          )}
        </Space>
      }
    >
      <Paragraph>
        <Text type="secondary">
          {currentParentId
            ? `管理「${currentParent?.industry_name}」下的具体行业细分与技术模板。`
            : '管理不同行业的标准目录骨架与资源池。点击大类可进入管理二级细分行业。'}
        </Text>
      </Paragraph>

      <Table
        dataSource={currentParentId ? allSkeletonData.filter((item) => item.parent_id === currentParentId) : skeletonData}
        loading={skeletonLoading}
        rowKey="id"
        size="middle"
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        className="skeleton-drilldown-table"
        rowSelection={currentParentId ? {
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        } : undefined}
        onRow={(record) => ({
          style: { cursor: !currentParentId ? 'pointer' : 'default' },
          onClick: () => {
            if (!currentParentId) {
              updateCategoryId(record.id);
            }
          },
        })}
        columns={[
          {
            title: currentParentId ? '二级分类名称' : '行业大类名称',
            dataIndex: 'industry_name',
            key: 'industry_name',
            render: (val, record) => (
              <Space>
                {!currentParentId ? (
                  <Tag color="geekblue" style={{ borderRadius: '4px', fontWeight: 600, padding: '2px 8px' }}>
                    {val}
                  </Tag>
                ) : (
                  <Space size="middle">
                    <Text
                      role="button"
                      tabIndex={0}
                      onClick={(e) => {
                        e.stopPropagation();
                        handleEdit(record);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          e.stopPropagation();
                          handleEdit(record);
                        }
                      }}
                      style={{
                        fontSize: '14px',
                        fontWeight: 500,
                        color: '#1677ff',
                        cursor: 'pointer',
                      }}
                    >
                      {val}
                    </Text>
                  </Space>
                )}
              </Space>
            ),
          },
          {
            title: currentParentId ? '业务配置详情' : '子类数量',
            key: 'details',
            align: currentParentId ? undefined : 'center',
            render: (_, record) => {
              if (!currentParentId) {
                const subCount = allSkeletonData.filter((item) => item.parent_id === record.id).length;
                return (
                  <Text style={{ fontSize: '14px', fontVariantNumeric: 'tabular-nums' }}>
                    {subCount}
                  </Text>
                );
              }

              const status = generationStatus[record.id];
              if (status === 'processing') {
                return <Badge status="processing" text={<Text className="animate-pulse" style={{ color: '#1677ff' }}>AI 生成中...</Text>} />;
              }
              if (status === 'queued') {
                return <Badge status="warning" text={<Text style={{ color: '#fa8c16' }}>排队中</Text>} />;
              }
              if (status === 'done') {
                return <Badge status="success" text={<Text style={{ color: '#52c41a' }}>已完成生成</Text>} />;
              }

              try {
                const parsed = JSON.parse(record.logical_chapters_json);
                const chaptersLen = Array.isArray(parsed) ? parsed.length : 0;
                let kwLen = 0;
                if (record.industry_keywords_json) {
                  try {
                    const kw = JSON.parse(record.industry_keywords_json);
                    kwLen = Array.isArray(kw) ? kw.length : 0;
                  } catch {
                    kwLen = 0;
                  }
                }
                return (
                  <Space size="large">
                    <Badge status={chaptersLen > 0 ? 'processing' : 'default'} text={`${chaptersLen} 逻辑章节`} />
                    {kwLen > 0 && (
                      <Text type="secondary" style={{ fontSize: '12px' }}>
                        词库: {kwLen}
                      </Text>
                    )}
                  </Space>
                );
              } catch {
                return <Tag color="error">配置错误</Tag>;
              }
            },
          },
          {
            title: currentParentId ? '修改时间' : '最近修改',
            dataIndex: 'updated_at',
            render: (val) => {
              if (!val) return '-';
              const d = new Date(val);
              if (currentParentId) {
                return (
                  <Text type="secondary" style={{ fontSize: '12px', fontVariantNumeric: 'tabular-nums' }}>
                    {d.toLocaleString('zh-CN', {
                      year: 'numeric',
                      month: '2-digit',
                      day: '2-digit',
                      hour: '2-digit',
                      minute: '2-digit',
                      second: '2-digit',
                      hour12: false,
                    })}
                  </Text>
                );
              }
              return (
                <Tooltip title={d.toLocaleString()}>
                  <Text type="secondary" style={{ fontSize: '12px' }}>
                    {d.toLocaleDateString()}
                  </Text>
                </Tooltip>
              );
            },
          },
          {
            title: '管理',
            key: 'action',
            width: currentParentId ? 200 : 120,
            render: (_, record) => {
              const grayIcon = !currentParentId ? { color: '#8c8c8c' as const } : undefined;
              const status = generationStatus[record.id];
              const isLocked = status === 'processing' || status === 'queued';

              return (
                <Space onClick={(e) => e.stopPropagation()}>
                  {currentParentId && (
                    <Tooltip title={status === 'queued' ? '任务排队中...' : status === 'processing' ? 'AI 正在生成数据...' : status === 'done' ? '重新生成' : 'AI 生成大纲与资源池'}>
                      <Button
                        type="text"
                        size="small"
                        disabled={isLocked}
                        icon={
                          status === 'processing' ? <LoadingOutlined style={{ color: '#1677ff' }} /> :
                            status === 'queued' ? <LoadingOutlined style={{ color: '#fa8c16' }} /> :
                              <ExperimentOutlined style={{ color: '#722ed1' }} />
                        }
                        onClick={() => addToQueue([record.id])}
                      />
                    </Tooltip>
                  )}
                  <Button
                    type="text"
                    size="small"
                    disabled={isLocked}
                    icon={<EditOutlined style={grayIcon} />}
                    style={grayIcon}
                    onClick={() => handleEdit(record)}
                  />
                  <Popconfirm title="永久删除该分类？" onConfirm={() => handleDelete(record.id)} disabled={isLocked}>
                    <Button type="text" size="small" disabled={isLocked} icon={<DeleteOutlined style={grayIcon} />} style={grayIcon} />
                  </Popconfirm>
                </Space>
              );
            },
          },
        ]}
      />

      <Modal
        title={
          editingSkeleton
            ? editingSkeleton.parent_id
              ? (() => {
                const l1 = currentParent?.industry_name;
                const l2 = editingSkeleton.industry_name;
                return l1 ? `编辑二级分类：${l1} / ${l2}` : `编辑二级分类：${l2}`;
              })()
              : `修改一级大类名称`
            : currentParentId
              ? '添加二级分类'
              : '添加一级大类'
        }
        open={editModalVisible}
        onCancel={() => {
          parentIdForNewSubRef.current = null;
          setEditModalVisible(false);
        }}
        onOk={() => skeletonForm.submit()}
        footer={[
          <Button key="cancel" onClick={() => setEditModalVisible(false)}>
            取消
          </Button>,
          (editingSkeleton?.parent_id || currentParentId) && (
            <Button
              key="ai"
              icon={<ExperimentOutlined />}
              loading={aiLoading}
              onClick={handleGenerateAI}
              style={{ backgroundColor: '#f0faff', color: '#096dd9', borderColor: '#91d5ff' }}
              title="根据一级大类和当前分类名称，使用 AI 生成三层骨架目录草案"
            >
              AI 生成草案 (🧪)
            </Button>
          ),
          <Button key="submit" type="primary" onClick={() => skeletonForm.submit()}>
            保存配置
          </Button>,
        ]}
        width={editingSkeleton?.parent_id || currentParentId ? 800 : 450}
        destroyOnClose
      >
        <Form form={skeletonForm} layout="vertical" onFinish={handleSave}>
          <Form.Item noStyle shouldUpdate>
            {() => {
              const isL1Mode = editingSkeleton ? !editingSkeleton.parent_id : !currentParentId;
              const isAddingSub = !editingSkeleton && !!currentParentId;
              const isEditingSub = !!editingSkeleton && !!editingSkeleton.parent_id;

              if (isL1Mode) {
                return (
                  <>
                    <Form.Item name="industry_name" label="一级大类名称" rules={[{ required: true, message: '请输入名称' }]}>
                      <Input placeholder="如：水利工程 / 房建工程" />
                    </Form.Item>
                    <Form.Item name="matching_rules_json" label="匹配规则" help={<Text type="secondary" style={{ fontSize: 12 }}>定义该行业大类的招标文件匹配规则，支持 JSON 格式。例如：<code>{"{"}"keywords": ["燃气", "管道"], "excluded": ["房建"]{"}"}</code></Text>}>
                      <Input.TextArea
                        rows={4}
                        placeholder={'{"keywords": ["燃气", "管道"], "excluded": ["房建"], "minScore": 60}'}
                        style={{ fontFamily: 'monospace', fontSize: '12px' }}
                      />
                    </Form.Item>
                  </>
                );
              }

              return (
                <>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="industry_name" label="分类名称" rules={[{ required: true, message: '请输入名称' }]}>
                        <Input placeholder={isAddingSub ? '二级细分名称，如：住宅建筑' : '名称'} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="parent_id" label="所属一级分类">
                        <Select placeholder="选择一级分类" allowClear disabled={isAddingSub || isEditingSub}>
                          {allSkeletonData.filter((item) => isRootSkeletonItem(item)).map((cat) => (
                            <Select.Option key={cat.id} value={cat.id}>
                              {cat.industry_name}
                            </Select.Option>
                          ))}
                        </Select>
                      </Form.Item>
                    </Col>
                  </Row>

                  <Form.Item name="logical_chapters_json" label="章节与结构配置" rules={[{ required: true, message: '请配置逻辑章节' }]}>
                    <Form.Item noStyle dependencies={['logical_chapters_json']}>
                      {({ getFieldValue, setFieldsValue }) => (
                        <ChapterMarkdownBuilder
                          label="章节大纲构建器"
                          value={getFieldValue('logical_chapters_json')}
                          onChange={(v) => setFieldsValue({ logical_chapters_json: v })}
                        />
                      )}
                    </Form.Item>
                  </Form.Item>

                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Item name="industry_keywords_json" label="行业关键词池">
                        <Input.TextArea
                          rows={4}
                          placeholder="['关键词A', '关键词B']"
                          style={{ fontFamily: 'monospace', fontSize: '12px' }}
                        />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="title_candidate_pool_json" label="差异化标题池">
                        <Input.TextArea
                          rows={4}
                          placeholder="{'章节ID': ['候选标题1', '候选标题2']}"
                          style={{ fontFamily: 'monospace', fontSize: '12px' }}
                        />
                      </Form.Item>
                    </Col>
                  </Row>

                  <Form.Item name="common_section_pool_json" label="通用可选子节池 (JSON Array)">
                    <Input.TextArea rows={3} style={{ fontFamily: 'monospace', fontSize: '12px' }} />
                  </Form.Item>
                </>
              );
            }}
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};
