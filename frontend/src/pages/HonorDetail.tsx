import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { Card, Typography, Button, Space, Descriptions, Spin, Result, Tag, Image, Modal, Tooltip } from 'antd';
import { LeftOutlined, EditOutlined, EyeOutlined } from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

interface HonorDetailData {
  id: string;
  honor_name: string;
  honor_level?: string;
  owner_org?: string;
  owner_person_name?: string;
  award_date?: string;
  issue_authority?: string;
  stored_path?: string;
  risk_status?: string;
}

const LEVEL_COLORS: Record<string, string> = {
  national: 'red',
  provincial: 'orange',
  municipal: 'blue',
  district: 'cyan',
};
const LEVEL_LABELS: Record<string, string> = {
  national: '国家级',
  provincial: '省级',
  municipal: '市级',
  district: '区级',
};

const HonorDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { currentCompanyId } = useCompany();
  const [loading, setLoading] = useState(true);
  const [honor, setHonor] = useState<HonorDetailData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      if (!id) return;
      setLoading(true);
      setError(null);
      try {
        const response = await axios.get<HonorDetailData>(`/api/honors/${id}`, {
          headers: { 'x-company-id': currentCompanyId },
        });
        setHonor(response.data);
      } catch (err: unknown) {
        console.error('Failed to fetch honor detail:', err);
        const apiErr = err as { response?: { data?: { error?: string } } };
        setError(apiErr.response?.data?.error || '无法获取荣誉详情');
        setHonor(null);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [id, currentCompanyId]);

  const showOriginal = () => {
    if (!honor?.stored_path) return;
    const previewUrl = `/files/${honor.stored_path.split('data/files/')[1]}`;
    Modal.info({
      title: `原件预览 — ${honor.honor_name}`,
      width: 800,
      centered: true,
      maskClosable: true,
      icon: null,
      content: (
        <div style={{ textAlign: 'center', padding: '10px 0', maxHeight: '75vh', overflowY: 'auto' }}>
          <Image
            src={previewUrl}
            alt="原件预览"
            style={{ maxWidth: '100%', borderRadius: 4, boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
          />
        </div>
      ),
      okText: '关闭',
    });
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入荣誉数据..." />
      </div>
    );
  }

  if (error || !honor) {
    return (
      <Result
        status="error"
        title="获取详情失败"
        subTitle={error}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/honors')}>
            返回列表
          </Button>,
        ]}
      />
    );
  }

  const level = honor.honor_level || '';
  const ownerLabel = honor.owner_org || honor.owner_person_name || '未指定';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }} className="animate-in fade-in duration-500">
      <div className="flex justify-between items-center w-full">
        <Space size="middle" style={{ flex: 1, minWidth: 0, alignItems: 'center' }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate(-1)} />
          <Title level={3} style={{ margin: 0 }}>
            {honor.honor_name}
          </Title>
          <Tooltip title="编辑荣誉">
            <Link to={`/library/honors/${honor.id}/edit`}>
              <Button type="text" shape="circle" icon={<EditOutlined />} style={{ color: 'rgba(0,0,0,0.45)' }} />
            </Link>
          </Tooltip>
        </Space>
        {honor.stored_path && (
          <Space size="middle" style={{ marginLeft: 24, flexShrink: 0 }}>
            <Button icon={<EyeOutlined />} onClick={showOriginal}>
              查看原件
            </Button>
          </Space>
        )}
      </div>

      <Card className="shadow-sm border-none bg-white" title="荣誉信息">
        <Descriptions column={{ xs: 1, sm: 2 }} bordered size="middle">
          <Descriptions.Item label="荣誉名称" span={2}>
            {honor.honor_name}
          </Descriptions.Item>
          <Descriptions.Item label="荣誉级别">
            <Tag color={LEVEL_COLORS[level] || 'default'}>{LEVEL_LABELS[level] || level || '-'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="颁发日期">{honor.award_date || '-'}</Descriptions.Item>
          <Descriptions.Item label="获奖主体 (单位/个人)" span={2}>
            {ownerLabel}
          </Descriptions.Item>
          <Descriptions.Item label="颁发部门/机关" span={2}>
            {honor.issue_authority || '-'}
          </Descriptions.Item>
          {honor.risk_status ? (
            <Descriptions.Item label="风险状态" span={2}>
              <Text type="warning">{honor.risk_status}</Text>
            </Descriptions.Item>
          ) : null}
        </Descriptions>
      </Card>
    </div>
  );
};

export default HonorDetail;
