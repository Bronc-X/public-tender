import React, { useState, useEffect } from 'react';
import { Table, Button, Space, Input, Card, Typography, Popconfirm, Tooltip, message } from 'antd';
import { PlusOutlined, SearchOutlined, ProjectOutlined, DeleteOutlined, EditOutlined, UploadOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';
import PerformanceImportModal from '../components/PerformanceImportModal';


const { Title, Text, Link } = Typography;

function highlightKeyword(text: string, keyword: string): React.ReactNode {
  const raw = (text ?? '').toString();
  const kw = (keyword ?? '').trim();
  if (!kw) return raw;

  const lowerText = raw.toLowerCase();
  const lowerKw = kw.toLowerCase();

  const parts: React.ReactNode[] = [];
  let start = 0;
  let idx = lowerText.indexOf(lowerKw, start);
  if (idx === -1) return raw;

  while (idx !== -1) {
    if (idx > start) parts.push(raw.slice(start, idx));
    parts.push(
      <span key={`${idx}-${kw}`} style={{ color: '#ff4d4f' }}>
        {raw.slice(idx, idx + kw.length)}
      </span>
    );
    start = idx + kw.length;
    idx = lowerText.indexOf(lowerKw, start);
  }
  if (start < raw.length) parts.push(raw.slice(start));

  return <>{parts}</>;
}

interface Project {
  id: string;
  project_name: string;
  project_location?: string;
  owner_org?: string;
  project_manager_name?: string;
  pm_id?: string;
  technical_leader_name?: string;
  tech_leader_id?: string;
  safety_leader_name?: string;
  safety_leader_id?: string;
  completion_date?: string;
  winning_date?: string;
  bid_amount_value?: number;
  amount_value?: number;
  scale_desc?: string;
  construction_period?: string;
}

const PerformanceLibrary: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<Project[]>([]);
  const [searchText, setSearchText] = useState('');
  const [isImportModalVisible, setIsImportModalVisible] = useState(false);
  const { currentCompanyId } = useCompany();

  const fetchData = async () => {
    setLoading(true);
    try {
      const response = await axios.get('/api/performances');
      // 兼容直接返回数组和封装后的 {data: [], total: ...} 结构
      const rawList: Record<string, any>[] = Array.isArray(response.data) ? response.data : (response.data?.data || []);
      
      // 数据归一化：由于后端 Mock 数据可能使用 camelCase，这里统一转为 snake_case 供前端使用
      const normalized = rawList.map((item: Record<string, any>) => ({
        ...item,
        id: (item.id || '').toString(),
        project_name: item.project_name || item.projectName || '-',
        project_manager_name: item.project_manager_name || item.projectManagerName || item.pmName || '-',
        amount_value: item.amount_value || item.amount || 0,
        completion_date: item.completion_date || item.completionDate || '-',
        winning_date: item.winning_date || item.winningDate || '-',
        owner_org: item.owner_org || item.owner || '-',
      }));
      
      setData(normalized);
    } catch (err) {
      console.error('Failed to fetch performances:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [currentCompanyId]);

  const handleAdd = () => {
    navigate('/library/performances/create');
  };

  const handleMatch = async () => {
    setLoading(true);
    try {
      const resp = await axios.post('/api/performances/match');
      const { updated } = resp.data;
      if (updated > 0) {
        message.success(`匹配完成！已成功关联 ${updated} 条业绩。`);
        fetchData();
      } else {
        message.info('扫描完成，未发现可关联的新人员。');
      }
    } catch (err) {
      console.error('Match failed:', err);
      message.error('匹配失败，请检查后端服务。');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await axios.delete(`/api/performances/${id}`);
      message.success('已删除');
      fetchData();
    } catch (err: unknown) {
      console.error('Failed to delete performance:', err);
      const msg = err instanceof Error ? err.message : String(err);
      message.error('删除失败：' + msg);
    }
  };

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'project_name',
      key: 'project_name',
      width: 360,
      render: (text: string, record: Project) => (
        <Link onClick={() => navigate(`/library/performances/${record.id}`)}>
          {highlightKeyword(text, searchText)}
        </Link>
      ),
    },
    {
      title: '项目经理',
      key: 'team',
      width: 140,
      render: (_: unknown, record: Project) => (
        <Space direction="vertical" size={0}>
          {record.pm_id ? (
            <Link onClick={(e) => { e.stopPropagation(); navigate(`/library/persons/${record.pm_id}`); }}>
              {highlightKeyword(record.project_manager_name || '', searchText)}
            </Link>
          ) : (
            <Text type="secondary" className="text-xs">
              {record.project_manager_name ? highlightKeyword(record.project_manager_name, searchText) : '-'}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: '中标日期',
      key: 'dates',
      width: 140,
      render: (_: unknown, record: Project) => (
        <Text>{record.winning_date || '-'}</Text>
      ),
    },
    {
      title: '完工日期',
      key: 'completion_date',
      width: 140,
      render: (_: unknown, record: Project) => (
        <Text>{record.completion_date || '-'}</Text>
      ),
    },

    {
      title: '合同金额（万元）',
      key: 'amounts',
      width: 140,
      align: 'right' as const,
      render: (_: unknown, record: Project) => (
        <Text>{(record.amount_value || 0).toLocaleString()}</Text>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      fixed: 'right' as const,
      align: 'center' as const,
      render: (_: unknown, record: Project) => (
        <Space size="middle">
          <Tooltip title="编辑">
            <Button type="text" icon={<EditOutlined />} onClick={() => navigate(`/library/performances/${record.id}/edit`)} style={{ color: '#1890ff' }} />
          </Tooltip>
          <Popconfirm
            title="确认删除该业绩？"
            description="删除后无法恢复。"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={() => handleDelete(record.id)}
          >
            <Tooltip title="删除">
              <Button type="text" icon={<DeleteOutlined />} style={{ color: 'rgba(0,0,0,0.45)' }} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: '0 0 24px 0' }}>
        <div className="flex items-center" style={{ justifyContent: 'space-between', marginBottom: 24 }}>
          <Space size={16}>
            <Button icon={<PlusOutlined />} type="primary" onClick={handleAdd}>新增</Button>
            <Input
              placeholder="搜索项目名称、负责人、业主..."
              prefix={<SearchOutlined />}
              value={searchText}
              allowClear
              onChange={e => setSearchText(e.target.value)}
              style={{ width: 400 }}
            />
          </Space>
          <Space size={16} style={{ marginLeft: 32 }}>
            <Button icon={<UploadOutlined />} onClick={() => setIsImportModalVisible(true)}>批量导入</Button>
            <Button icon={<ProjectOutlined />} onClick={handleMatch}>全库人员对齐</Button>
          </Space>
        </div>

      <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table
          columns={columns}
          dataSource={data.filter((item: Project) => 
            item.project_name?.includes(searchText) || 
            item.project_manager_name?.includes(searchText) ||
            item.technical_leader_name?.includes(searchText) ||
            item.safety_leader_name?.includes(searchText) ||
            item.owner_org?.includes(searchText)
          )}
          loading={loading}
          rowKey="id"
          scroll={{ x: 980 }}
          pagination={{ 
            pageSizeOptions: ['10', '20', '50', '100'], 
            showSizeChanger: true, 
            defaultPageSize: 10,
            showTotal: (total) => `共 ${total} 条业绩`
          }}
          size="middle"
        />
      </Card>

      <PerformanceImportModal 
        visible={isImportModalVisible} 
        onCancel={() => setIsImportModalVisible(false)}
        onSuccess={() => {
          fetchData();
          setIsImportModalVisible(false);
        }}
      />
    </div>
  );
};

export default PerformanceLibrary;
