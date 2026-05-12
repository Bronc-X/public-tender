import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Space, Tag, Card, Typography, Progress, message, Tooltip } from 'antd';
import { 
  FileSearchOutlined as AuditOutlined, 
  FilePdfOutlined, 
  FileImageOutlined,
  SyncOutlined,
  EyeOutlined
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text, Link } = Typography;

interface AuditItem {
    id: string;
    file_name: string;
    mime_type: string;
    object_type: string;
    confidence_score: number;
    audit_status: string;
    risk_level: string;
    created_at: string;
}

const AuditCenter: React.FC = () => {
  const navigate = useNavigate();
  const { currentCompanyId, loading: companyLoading } = useCompany();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<AuditItem[]>([]);

  const fetchAudits = useCallback(async () => {
    if (companyLoading || !currentCompanyId) return;
    setLoading(true);
    try {
      const response = await axios.get('/api/audits');
      setData(response.data);
    } catch (err) {
      console.error('Failed to fetch audits:', err);
      message.error('无法加载待审核列表');
    } finally {
      setLoading(false);
    }
  }, [currentCompanyId, companyLoading]);

  useEffect(() => {
    fetchAudits();
  }, [fetchAudits]);

  const columns = [
    {
      title: '文件名',
      dataIndex: 'file_name',
      key: 'file_name',
      width: 320,
      render: (text: string, record: AuditItem) => (
        <Space style={{ width: '100%' }}>
          {record.mime_type?.includes('pdf') ? <FilePdfOutlined style={{ color: '#ff4d4f' }} /> : <FileImageOutlined style={{ color: '#1890ff' }} />}
          <Tooltip title={text} mouseEnterDelay={0.5}>
            <Link
              onClick={() => navigate(`/audits/${record.id}`)}
              style={{ maxWidth: 260, display: 'inline-block' }}
              ellipsis
            >
              {text}
            </Link>
          </Tooltip>
        </Space>
      ),
    },
    {
      title: 'AI 识别建议',
      dataIndex: 'object_type',
      key: 'object_type',
      width: 120,
      render: (type: string) => {
        const types: Record<string, string> = {
          'performance': '项目业绩',
          'person': '人员档案',
          'qualification': '资质证书',
          'honor': '荣誉奖项',
          'method': '工法亮点',
          'equipment': '施工设备',
          'standard': '制度规范',
          'risk': '技术风险',
          'tech_bid_import': '技术标提炼',
          'unknown': '待分类'
        };
        return <Tag color="blue">{types[type] || '未知'}</Tag>;
      }
    },
    {
      title: '置信度',
      dataIndex: 'confidence_score',
      key: 'confidence_score',
      width: 150,
      render: (score: number) => (
        <Space direction="vertical" style={{ width: 100 }}>
          <Progress 
            percent={Math.round((score || 0.85) * 100)} 
            size="small" 
            status={score < 0.6 ? 'exception' : 'active'}
            showInfo={false}
          />
          <Text type="secondary" style={{ fontSize: '10px' }}>{Math.round((score || 0.85) * 100)}% 匹配</Text>
        </Space>
      )
    },
    {
      title: '风险级别',
      dataIndex: 'risk_level',
      key: 'risk_level',
      width: 100,
      render: (level: string) => (
        <Tag color={level === 'high' ? 'red' : level === 'warning' ? 'orange' : 'green'}>
          {level === 'high' ? '高风险' : level === 'warning' ? '需注意' : '正常'}
        </Tag>
      )
    },
    {
      title: '上传时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (date: string) => dayjs(date).format('YYYY-MM-DD HH:mm')
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: AuditItem) => (
        <Space size="middle">
          <Button type="primary" size="small" icon={<EyeOutlined />} onClick={() => navigate(`/audits/${record.id}`)}>
            开始审核
          </Button>
          <Button size="small" onClick={async () => {
            try {
              await axios.post(`/api/audits/${record.id}/ignore`);
              message.success('已忽略该任务');
              fetchAudits();
            } catch (err) {
              console.error('Ignore error:', err);
              message.error('操作失败');
            }
          }}>忽略</Button>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={3} style={{ margin: 0 }}>
            <AuditOutlined style={{ marginRight: 8, color: '#1890ff' }} /> 文件审核台
          </Title>
          <Text type="secondary">对 AI 自动识别出的字段进行最后的人工校验。核对无误后点击“确认入库”即正式存入企业资料库。</Text>
        </div>
        <Button icon={<SyncOutlined />} onClick={fetchAudits} loading={loading}>刷新任务</Button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: '24px' }}>
          <Card size="small" style={{ backgroundColor: '#f0faff', border: '1px solid #e6f7ff', borderRadius: '8px' }}>
              <Space direction="vertical">
                  <Text type="secondary" style={{ fontSize: '13px' }}>待处理审核</Text>
                  <Title level={2} style={{ margin: 0 }}>{data.length}</Title>
              </Space>
          </Card>
          <Card size="small" style={{ backgroundColor: '#f6ffed', border: '1px solid #d9f7be', borderRadius: '8px' }}>
              <Space direction="vertical">
                  <Text type="secondary" style={{ fontSize: '13px' }}>今日已入库</Text>
                  <Title level={2} style={{ margin: 0 }}>12</Title>
              </Space>
          </Card>
          <Card size="small" style={{ backgroundColor: '#fff7e6', border: '1px solid #ffd591', borderRadius: '8px' }}>
              <Space direction="vertical">
                  <Text type="secondary" style={{ fontSize: '13px' }}>识别异常项</Text>
                  <Title level={2} style={{ margin: 0 }}>{data.filter(i => i.risk_level === 'high').length}</Title>
              </Space>
          </Card>
      </div>

      <Card style={{ boxShadow: '0 1px 2px 0 rgba(0, 0, 0, 0.03)', border: 'none' }}>
        <Table
          columns={columns}
          dataSource={data}
          loading={loading}
          rowKey="id"
          pagination={{ pageSize: 15 }}
        />
      </Card>
    </div>
  );
};

export default AuditCenter;
