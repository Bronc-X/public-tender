import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Space, Typography, Tag, Card, Modal, Form, Input, message, Radio, List, Avatar, Divider } from 'antd';
import { PlusOutlined, DeleteOutlined, AuditOutlined, EditOutlined, SyncOutlined, FileTextOutlined, SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

interface Project {
  id: string;
  project_name: string;
  owner_name: string;
  project_status: string;
  current_step: string;
  created_at: string;
  updated_at: string;
  shared_tender_id?: string;
  current_step_status?: string;
}

const STEP_NAMES: Record<string, string> = {
  tender_detail_extract: '项目详情提取',
  rule_parse: '招标规则解析',
  company_adaptation: '各项指标适配',
  resource_combination: '资源方案组合',
  user_confirmation: '方案人工确认',
  chapter_generation: '标书章节生成',
  attachment_assembly: '附件自动装配',
  risk_review: '合规风险审查',
  output_finalize: '成果定稿输出'
};

const BidProjectList: React.FC = () => {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchText, setSearchText] = useState('');
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [createMode, setCreateMode] = useState<'new' | 'sync'>('new');
  const [candidates, setCandidates] = useState<any[]>([]);
  const [candidateSearch, setCandidateSearch] = useState('');
  const [selectedCandidate, setSelectedCandidate] = useState<any>(null);
  const [form] = Form.useForm();

  const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  const highlightText = (text: string | null | undefined, search: string) => {
    const t = text ?? '';
    if (!search || !t) return t || '—';
    let parts: string[];
    try {
      parts = t.split(new RegExp(`(${escapeRegExp(search)})`, 'gi'));
    } catch {
      return t;
    }
    return (
      <span>
        {parts.map((part, i) =>
          part.toLowerCase() === search.toLowerCase() ? (
            <span key={i} style={{ color: '#f5222d', fontWeight: 'bold' }}>{part}</span>
          ) : (
            <span key={i}>{part}</span>
          )
        )}
      </span>
    );
  };
  const navigate = useNavigate();
  const { currentCompanyId, loading: companyLoading } = useCompany();

  const fetchProjects = useCallback(async () => {
    if (companyLoading || !currentCompanyId) return;
    setLoading(true);
    try {
      const response = await axios.get('/api/bid-projects', {
        headers: { 'x-company-id': currentCompanyId }
      });
      setProjects(response.data);
    } catch (err) {
      console.error('Failed to fetch projects:', err);
      message.error('加载项目列表失败');
    } finally {
      setLoading(false);
    }
  }, [currentCompanyId, companyLoading]);

  const fetchCandidates = useCallback(async (search: string = '') => {
    if (!currentCompanyId) return;
    try {
      const res = await axios.get('/api/shared-tenders/candidates', {
        params: { search },
        headers: { 'X-Company-Id': currentCompanyId }
      });
      setCandidates(Array.isArray(res.data) ? res.data : []);
    } catch (err) {
      console.error('Failed to fetch candidates', err);
    }
  }, [currentCompanyId]);

  useEffect(() => {
    if (createMode === 'sync' && isModalVisible) {
      const timer = setTimeout(() => {
        fetchCandidates(candidateSearch);
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [candidateSearch, createMode, isModalVisible, fetchCandidates]);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  const handleSubmit = async (values: any) => {
    try {
      const payload = { ...values };
      if (createMode === 'sync' && selectedCandidate) {
        payload.shared_tender_id = selectedCandidate.mode === 'registry' ? selectedCandidate.id : null;
        payload.sync_source_project_id = selectedCandidate.source_project_id;
        payload.sync_source_module = selectedCandidate.source_module;
        if (!payload.project_name) payload.project_name = selectedCandidate.project_name;
        if (!payload.owner_name) payload.owner_name = selectedCandidate.owner_name;
      }

      if (editingId) {
        await axios.patch(`/api/bid-projects/${editingId}`, payload);
        message.success('项目信息已更新');
      } else {
        const res = await axios.post('/api/bid-projects', payload, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('项目创建成功');
        navigate(`/bid-projects/${res.data.id}`);
      }
      setIsModalVisible(false);
      setEditingId(null);
      setSelectedCandidate(null);
      setCreateMode('new');
      form.resetFields();
      fetchProjects();
    } catch (err) {
      console.error('Submit error:', err);
      message.error(editingId ? '更新失败' : '创建失败');
    }
  };

  const showEditModal = (record: Project) => {
    setEditingId(record.id);
    form.setFieldsValue({
      project_name: record.project_name,
      owner_name: record.owner_name
    });
    setIsModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    Modal.confirm({
      title: '删除项目',
      content: '确定要删除该商务标制作项目吗？此操作不可恢复。',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          await axios.delete(`/api/bid-projects/${id}`);
          message.success('已删除');
          fetchProjects();
        } catch (err) {
          console.error('Delete error:', err);
          message.error('删除失败');
        }
      }
    });
  };

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'project_name',
      key: 'project_name',
      render: (text: string, record: Project) => (
        <span
          role="link"
          tabIndex={0}
          style={{ color: '#1890ff', cursor: 'pointer', fontSize: 15, fontWeight: 400 }}
          onClick={() => navigate(`/bid-projects/${record.id}`)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              navigate(`/bid-projects/${record.id}`);
            }
          }}
        >
          {highlightText(text, searchText)}
        </span>
      ),
    },
    {
      title: '项目业主',
      dataIndex: 'owner_name',
      key: 'owner_name',
      render: (text: string) => highlightText(text, searchText)
    },
    {
      title: '当前状态',
      key: 'status',
      render: (_: any, record: Project) => {
        const stepName = STEP_NAMES[record.current_step] || record.current_step || '未知阶段';
        const isFailed = record.project_status === 'failed';
        const isCompleted = record.project_status === 'completed';

        return (
          <Tag color={isFailed ? 'error' : isCompleted ? 'success' : 'processing'} style={{ borderRadius: '4px' }}>
            {stepName}
          </Tag>
        );
      }
    },
    {
      title: '关联状态',
      dataIndex: 'shared_tender_id',
      key: 'shared_tender_id',
      render: (id: string, record: Project) => (
        id ? <Tag color="purple" icon={<SyncOutlined spin={record.current_step_status === 'running'} />}>已同步</Tag> : <Tag>独立项目</Tag>
      )
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: Project) => (
        <Space size="middle">
          <Button
            type="text"
            icon={<EditOutlined />}
            onClick={() => showEditModal(record)}
            style={{ color: 'rgba(0, 0, 0, 0.45)' }}
          />
          <Button
            type="text"
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record.id)}
            style={{ color: 'rgba(0, 0, 0, 0.45)' }}
          />
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: '0 0 24px 0' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-start', alignItems: 'center', marginBottom: 24 }}>
        <Space size={16}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditingId(null);
              setCreateMode('new');
              setSelectedCandidate(null);
              setCandidateSearch('');
              form.resetFields();
              fetchCandidates('');
              setIsModalVisible(true);
            }}
            style={{ borderRadius: '8px' }}
          >
            新建
          </Button>
          <Input
            placeholder="搜索项目名称、业主..."
            prefix={<SearchOutlined />}
            allowClear
            value={searchText}
            onChange={e => setSearchText(e.target.value)}
            style={{ width: 400 }}
          />
        </Space>
      </div>

      <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table
          columns={columns}
          dataSource={projects.filter(item => 
            item.project_name?.toLowerCase().includes(searchText.toLowerCase()) || 
            item.owner_name?.toLowerCase().includes(searchText.toLowerCase())
          )}
          loading={loading}
          rowKey="id"
          pagination={{ pageSize: 10 }}
          style={{ margin: 0 }}
        />
      </Card>

      <Modal
        title={editingId ? "编辑项目信息" : "创建商务标制作项目"}
        open={isModalVisible}
        onCancel={() => {
          setIsModalVisible(false);
          setEditingId(null);
          form.resetFields();
        }}
        footer={null}
        destroyOnClose
        width={600}
      >
        <div style={{ marginBottom: 24 }}>
          <Radio.Group
            value={createMode}
            onChange={e => setCreateMode(e.target.value)}
            optionType="button"
            buttonStyle="solid"
            disabled={!!editingId}
          >
            <Radio.Button value="new">新文件创建</Radio.Button>
            <Radio.Button value="sync">从已有项目同步</Radio.Button>
          </Radio.Group>
        </div>

        {createMode === 'sync' ? (
          <div>
            <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
              检测到以下已有招标项目，同步后可复用招标文件与解析结果，避免重复计费。
            </Text>
            <Input.Search
              placeholder="输入项目标题或招标编号搜索"
              style={{ marginBottom: 16 }}
              onChange={e => setCandidateSearch(e.target.value)}
              value={candidateSearch}
              allowClear
            />
            <List
              locale={{ emptyText: '暂无可同步的项目' }}
              pagination={{
                pageSize: 5,
                size: 'small',
                simple: true,
                hideOnSinglePage: true
              }}
              dataSource={candidates}
              renderItem={item => (
                <List.Item
                  style={{
                    cursor: 'pointer',
                    borderRadius: '8px',
                    padding: '12px',
                    marginBottom: '8px',
                    transition: 'all 0.3s',
                    border: selectedCandidate?.id === item.id ? '2px solid #1890ff' : '1px solid #f0f0f0',
                    background: selectedCandidate?.id === item.id ? '#f0f5ff' : 'white'
                  }}
                  onClick={() => {
                    setSelectedCandidate(item);
                    form.setFieldsValue({
                      project_name: item.project_name ?? '',
                      owner_name: item.owner_name ?? '',
                    });
                  }}
                >
                  <List.Item.Meta
                    avatar={<Avatar icon={<FileTextOutlined />} style={{ backgroundColor: '#1890ff' }} />}
                    title={highlightText(item.project_name, candidateSearch)}
                    description={
                      <Space split={<Divider type="vertical" />}>
                        <Text type="secondary">编号: {highlightText(item.tender_code || '无', candidateSearch)}</Text>
                        <Tag color="cyan">{item.source_module === 'bid' ? '商务标来源' : '技术标来源'}</Tag>
                      </Space>
                    }
                  />
                </List.Item>
              )}
            />
            <Form form={form} layout="vertical" onFinish={handleSubmit} style={{ marginTop: 24 }}>
              <Form.Item name="project_name" label="商务标项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
                <Input placeholder="同步后的商务标项目名称" size="large" />
              </Form.Item>
              <Form.Item name="owner_name" label="项目业主" rules={[{ required: true, message: '请输入业主名称' }]}>
                <Input placeholder="业主单位名称" size="large" />
              </Form.Item>
              <Button type="primary" htmlType="submit" block size="large" style={{ height: '48px' }} disabled={!selectedCandidate}>
                确认同步并创建
              </Button>
            </Form>
          </div>
        ) : (
          <Form
            form={form}
            layout="vertical"
            onFinish={handleSubmit}
          >
            <Form.Item
              name="project_name"
              label="项目名称"
              rules={[{ required: true, message: '请输入项目名称' }]}
            >
              <Input placeholder="例如：某市市民中心办公楼装修工程" size="large" />
            </Form.Item>

            <Form.Item
              name="owner_name"
              label="项目业主 (Owner)"
              rules={[{ required: true, message: '请输入业主名称' }]}
            >
              <Input placeholder="例如：某市城市建设投资有限公司" size="large" />
            </Form.Item>
            <Form.Item style={{ marginTop: 24, marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" block size="large" style={{ height: '48px' }}>
                {editingId ? "确认修改" : "确认创建"}
              </Button>
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  );
};

export default BidProjectList;
