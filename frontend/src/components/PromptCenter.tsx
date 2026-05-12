import React, { useState, useEffect, useMemo } from 'react';
import {
  Table, Card, Button, Space, Tag, Modal, Form, Input,
  Select, Tree, Row, Col, message, List, Typography, Popconfirm, InputNumber
} from 'antd';
import {
  PlusOutlined, EditOutlined, HistoryOutlined, DeleteOutlined,
  FolderOutlined, FileTextOutlined, UndoOutlined, SearchOutlined
} from '@ant-design/icons';
import axios from 'axios';

const { Text } = Typography;
const { TextArea } = Input;

interface PromptCategory {
  id: number;
  name: string;
  parent_id: number;
}

interface PromptTemplate {
  id: number;
  prompt_key: string;
  prompt_name: string;
  category_id: number;
  content: string;
  system_content: string;
  variables: string;
  status: number;
  version: number;
  updated_at: string;
}

interface PromptVersion {
  id: number;
  version: number;
  content: string;
  system_content: string;
  change_summary: string;
  created_at: string;
}

const PromptCenter: React.FC = () => {
  const [categories, setCategories] = useState<PromptCategory[]>([]);
  const [selectedCategoryId, setSelectedCategoryId] = useState<number | null>(null);
  const [prompts, setPrompts] = useState<PromptTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [showPromptModal, setShowPromptModal] = useState(false);
  const [showVersionModal, setShowVersionModal] = useState(false);
  const [showCategoryModal, setShowCategoryModal] = useState(false);
  const [currentPrompt, setCurrentPrompt] = useState<PromptTemplate | null>(null);
  const [currentCategory, setCurrentCategory] = useState<PromptCategory | null>(null);
  const [versions, setVersions] = useState<PromptVersion[]>([]);
  const [form] = Form.useForm();
  const [catForm] = Form.useForm();
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([0]);
  const [searchKeyword, setSearchKeyword] = useState('');

  const fetchCategories = async () => {
    try {
      const res = await axios.get('/api/prompt/category/list');
      if (res.data.success) {
        setCategories(res.data.data);
        // 默认展开所有：包含根节点 0 和所有分类 ID
        const keys = [0, ...res.data.data.map((c: any) => c.id)];
        setExpandedKeys(keys);
      }
    } catch (err) {
      console.error('Fetch categories failed', err);
    }
  };

  const fetchPrompts = async (catId?: number) => {
    setLoading(true);
    try {
      const url = catId ? `/api/prompt/list?category_id=${catId}` : '/api/prompt/list';
      const res = await axios.get(url);
      if (res.data.success) {
        setPrompts(res.data.data);
      }
    } catch (err) {
      console.error('Fetch prompts failed', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCategories();
    fetchPrompts();
  }, []);

  const handleEditPrompt = (record: PromptTemplate) => {
    setCurrentPrompt(record);
    form.setFieldsValue(record);
    setShowPromptModal(true);
  };

  const handleAddPrompt = () => {
    setCurrentPrompt(null);
    form.resetFields();
    if (selectedCategoryId) {
      form.setFieldsValue({ category_id: selectedCategoryId });
    }
    setShowPromptModal(true);
  };

  const handleSavePrompt = async () => {
    try {
      const values = await form.validateFields();
      const payload = {
        ...values,
        category_id: currentPrompt?.category_id || 0,
        id: currentPrompt?.id || 0,
        status: 1
      };
      const res = await axios.post('/api/prompt/save', payload);
      if (res.data.success) {
        message.success('保存成功');
        setShowPromptModal(false);
        fetchPrompts(selectedCategoryId || undefined);
      }
    } catch (err) {
      console.error('Save prompt failed', err);
    }
  };

  const handleShowHistory = async (record: PromptTemplate) => {
    try {
      const res = await axios.get(`/api/prompt/versions/${record.id}`);
      if (res.data.success) {
        setVersions(res.data.data);
        setCurrentPrompt(record);
        setShowVersionModal(true);
      }
    } catch (err) {
      console.error('Fetch history failed', err);
    }
  };

  const handleRollback = async (versionId: number) => {
    if (!currentPrompt) return;
    try {
      const res = await axios.post('/api/prompt/rollback', {
        template_id: currentPrompt.id,
        version_id: versionId
      });
      if (res.data.success) {
        message.success('回滚成功');
        setShowVersionModal(false);
        fetchPrompts(selectedCategoryId || undefined);
      }
    } catch (err) {
      console.error('Rollback failed', err);
    }
  };

  const handleAddCategory = () => {
    setCurrentCategory(null);
    catForm.resetFields();
    catForm.setFieldsValue({ parent_id: 0 });
    setShowCategoryModal(true);
  };

  const handleEditCategory = (cat: PromptCategory) => {
    setCurrentCategory(cat);
    catForm.setFieldsValue(cat);
    setShowCategoryModal(true);
  };

  const handleSaveCategory = async () => {
    try {
      const values = await catForm.validateFields();
      const payload = {
        ...values,
        id: currentCategory?.id || 0
      };
      const res = await axios.post('/api/prompt/category/save', payload);
      if (res.data.success) {
        message.success('分类保存成功');
        setShowCategoryModal(false);
        fetchCategories();
      }
    } catch (err) {
      console.error('Save category failed', err);
    }
  };

  const handleDeleteCategory = async (id: number) => {
    try {
      const res = await axios.delete(`/api/prompt/category/${id}`);
      if (res.data.success) {
        message.success('分类已删除');
        fetchCategories();
      }
    } catch (err: unknown) {
      const msg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
      message.error(msg || '删除失败');
    }
  };

  const renderTitle = (c: PromptCategory) => (
    <div className="flex justify-between items-center group w-full" style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
      <span>{c.name}</span>
      <Space size={4} className="opacity-0 group-hover:opacity-100" style={{ marginLeft: '12px' }}>
        <EditOutlined 
          style={{ color: '#1890ff', fontSize: '12px' }} 
          onClick={(e: React.MouseEvent) => { e.stopPropagation(); handleEditCategory(c); }} 
        />
        <Popconfirm title="确定删除吗？" onConfirm={(e?: React.MouseEvent) => { e?.stopPropagation(); handleDeleteCategory(c.id); }}>
          <DeleteOutlined 
            style={{ color: '#ff4d4f', fontSize: '12px' }} 
            onClick={(e: React.MouseEvent) => e.stopPropagation()} 
          />
        </Popconfirm>
      </Space>
    </div>
  );

  const treeData = categories
    .filter(c => c.parent_id === 0)
    .map(c => ({
      title: renderTitle(c),
      key: c.id,
      children: categories
        .filter(sub => sub.parent_id === c.id)
        .map(sub => ({ title: renderTitle(sub), key: sub.id }))
    }));

  // 根据关键词过滤提示词
  const filteredPrompts = useMemo(() => {
    if (!searchKeyword.trim()) return prompts;
    const keyword = searchKeyword.trim().toLowerCase();
    return prompts.filter(p =>
      p.prompt_name.toLowerCase().includes(keyword) ||
      p.prompt_key.toLowerCase().includes(keyword)
    );
  }, [prompts, searchKeyword]);

  // 高亮匹配的关键词
  const highlightText = (text: string, keyword: string): React.ReactNode => {
    if (!keyword.trim()) return text;
    const lowerKeyword = keyword.trim().toLowerCase();
    const lowerText = text.toLowerCase();
    const parts: React.ReactNode[] = [];
    let lastIndex = 0;
    let index = lowerText.indexOf(lowerKeyword);

    while (index !== -1) {
      if (index > lastIndex) {
        parts.push(text.slice(lastIndex, index));
      }
      parts.push(
        <span key={index} style={{ color: '#ff4d4f', fontWeight: 600, backgroundColor: '#fff1f0', padding: '0 2px', borderRadius: 2 }}>
          {text.slice(index, index + keyword.length)}
        </span>
      );
      lastIndex = index + keyword.length;
      index = lowerText.indexOf(lowerKeyword, lastIndex);
    }
    if (lastIndex < text.length) {
      parts.push(text.slice(lastIndex));
    }
    return <>{parts}</>;
  };

  const columns = [
    {
      title: '提示词名称',
      dataIndex: 'prompt_name',
      key: 'prompt_name',
      render: (text: string, record: PromptTemplate) => (
        <Space direction="vertical" size={0}>
          <Text strong>{highlightText(text, searchKeyword)}</Text>
          <Text type="secondary" style={{ fontSize: '12px' }}>{highlightText(record.prompt_key, searchKeyword)}</Text>
        </Space>
      )
    },
    {
      title: '当前版本',
      dataIndex: 'version',
      key: 'version',
      render: (v: number) => <Tag color="blue">v{v}</Tag>
    },
    {
      title: '最后更新',
      dataIndex: 'updated_at',
      key: 'updated_at',
      render: (d: string) => new Date(d).toLocaleString()
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: PromptTemplate) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => handleEditPrompt(record)}>编辑</Button>
          <Button icon={<HistoryOutlined />} onClick={() => handleShowHistory(record)}>历史</Button>
        </Space>
      )
    }
  ];

  return (
    <div style={{ padding: '0 0', display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAddPrompt}>新增提示词</Button>
      </div>
      <Row gutter={24}>
        <Col span={24}>
          <Card 
            size="small"
            className="shadow-sm border-none"
            style={{ minHeight: '500px' }}
          >
            <Input.Search
              placeholder="搜索提示词名称或唯一 Key"
              allowClear
              enterButton={<><SearchOutlined /> 搜索</>}
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
              onSearch={(value) => setSearchKeyword(value)}
              style={{ marginBottom: 16 }}
            />
            <Table 
              dataSource={filteredPrompts} 
              columns={columns} 
              rowKey="id" 
              loading={loading}
              pagination={{ pageSize: 10 }}
              locale={{ emptyText: searchKeyword ? '未找到匹配的提示词' : '暂无提示词' }}
            />
          </Card>
        </Col>
      </Row>

      <Modal
        title={currentPrompt ? "编辑提示词" : "新增提示词"}
        open={showPromptModal}
        onOk={handleSavePrompt}
        onCancel={() => setShowPromptModal(false)}
        width={900}
        style={{ top: 40 }}
        styles={{ body: { maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' } }}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="prompt_name" label="名称" rules={[{ required: true }]}>
                <Input placeholder="例如：通用 OCR 提取" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="prompt_key" label="唯一 Key" rules={[{ required: true }]}>
                <Input placeholder="例如：ocr_general_extraction" disabled={!!currentPrompt} />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="content" label="业务内容 (User Role)" rules={[{ required: true }]}>
            <TextArea rows={6} placeholder="输入具体的业务提取指令，支持 {{variable}} 语法" />
          </Form.Item>
          <Form.Item name="system_content" label="指令约束 (System Role)">
            <TextArea rows={3} placeholder="输入给模型的系统级约束，如：你是一个数据提取专家，请只返回 JSON。不要包含 Markdown 包隔符。" />
          </Form.Item>
          <Form.Item name="variables" label="变量定义 (JSON 格式)">
            <TextArea rows={2} placeholder='例如：{"name": "人员姓名", "id": "身份证号"}' />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={currentCategory ? "编辑分类" : "新增分类"}
        open={showCategoryModal}
        onOk={handleSaveCategory}
        onCancel={() => setShowCategoryModal(false)}
        destroyOnClose
      >
        <Form form={catForm} layout="vertical">
          <Form.Item name="name" label="分类名称" rules={[{ required: true }]}>
            <Input placeholder="输入分类名称" />
          </Form.Item>
          <Form.Item name="parent_id" label="上级分类" initialValue={0}>
            <Select>
              <Select.Option value={0}>顶级分类</Select.Option>
              {categories.filter(c => c.parent_id === 0).map(c => (
                <Select.Option key={c.id} value={c.id}>{c.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="sort" label="排序" initialValue={0}>
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="版本历史"
        open={showVersionModal}
        footer={null}
        onCancel={() => setShowVersionModal(false)}
        width={700}
      >
        <List
          itemLayout="horizontal"
          dataSource={versions}
          renderItem={(item) => (
            <List.Item
              actions={[
                <Popconfirm title="确定回滚到此版本吗？" onConfirm={() => handleRollback(item.id)}>
                  <Button icon={<UndoOutlined />}>回滚</Button>
                </Popconfirm>
              ]}
            >
              <List.Item.Meta
                title={<Space><Tag color="blue">v{item.version}</Tag> <Text type="secondary">{new Date(item.created_at).toLocaleString()}</Text></Space>}
                description={
                  <div style={{ backgroundColor: '#f5f5f5', padding: '8px', borderRadius: '4px', marginTop: '8px', maxHeight: '100px', overflow: 'hidden' }}>
                    <Text type="secondary" style={{ fontSize: '12px', whiteSpace: 'pre-wrap' }}>
                      {item.content.length > 200 ? item.content.substring(0, 200) + '...' : item.content}
                    </Text>
                  </div>
                }
              />
            </List.Item>
          )}
        />
      </Modal>
    </div>
  );
};

export default PromptCenter;
