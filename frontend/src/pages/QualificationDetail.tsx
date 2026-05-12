import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { Card, Typography, Button, Space, Descriptions, Spin, Result, Tag, Image, Modal, Tooltip } from 'antd';
import { LeftOutlined, EditOutlined, EyeOutlined } from '@ant-design/icons';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

interface QualificationDetailData {
  id: string;
  qualification_name: string;
  qualification_level?: string;
  qualification_type?: string;
  owner_type?: string;
  owner_id?: string;
  certificate_no?: string;
  registration_no?: string;
  issuing_authority?: string;
  valid_from?: string;
  valid_to?: string;
  bid_usable_status?: string;
  risk_status?: string;
  stored_path?: string;
  file_asset_id?: string;
}

function labelBidUsable(v?: string) {
  if (v === 'usable') return '正常 (库内可用)';
  if (v === 'restricted') return '受限 (不推荐使用)';
  if (v === 'expired') return '过期';
  return v || '-';
}

function validStatusTag(validTo?: string) {
  if (!validTo) return <Tag color="processing">长期或未填</Tag>;
  const today = dayjs();
  const expiry = dayjs(validTo);
  const diffDays = expiry.diff(today, 'day');
  if (diffDays < 0) return <Tag color="error">已过期</Tag>;
  if (diffDays <= 90) return <Tag color="warning">即将到期 ({diffDays} 天)</Tag>;
  return <Tag color="success">正常</Tag>;
}

const QualificationDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { currentCompanyId } = useCompany();
  const [loading, setLoading] = useState(true);
  const [qual, setQual] = useState<QualificationDetailData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      if (!id) return;
      setLoading(true);
      setError(null);
      try {
        const response = await axios.get<QualificationDetailData>(`/api/qualifications/${id}`, {
          headers: { 'x-company-id': currentCompanyId },
        });
        setQual(response.data);
      } catch (err: unknown) {
        console.error('Failed to fetch qualification detail:', err);
        const apiErr = err as { response?: { data?: { error?: string } } };
        setError(apiErr.response?.data?.error || '无法获取资质详情');
        setQual(null);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [id, currentCompanyId]);

  const showOriginal = () => {
    if (!qual) return;
    const previewUrl = qual.file_asset_id
      ? `/api/files/download/${qual.file_asset_id}`
      : (qual.stored_path?.includes('data/files/')
          ? `/files/${qual.stored_path.split('data/files/')[1]}`
          : '');
    if (!previewUrl) return;
    const isPdf = (qual.stored_path || '').toLowerCase().endsWith('.pdf');
    Modal.info({
      title: `原件预览 — ${qual.qualification_name}`,
      width: 800,
      centered: true,
      maskClosable: true,
      icon: null,
      content: (
        <div style={{ textAlign: 'center', padding: '10px 0', maxHeight: '75vh', overflowY: 'auto' }}>
          {isPdf ? (
            <iframe
              src={`${previewUrl}#view=FitH`}
              title="原件预览"
              style={{ width: '100%', height: '70vh', border: 'none', borderRadius: 4 }}
            />
          ) : (
            <Image
              src={previewUrl}
              alt="原件预览"
              style={{ maxWidth: '100%', borderRadius: 4, boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
            />
          )}
        </div>
      ),
      okText: '关闭',
    });
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入资质数据..." />
      </div>
    );
  }

  if (error || !qual) {
    return (
      <Result
        status="error"
        title="获取详情失败"
        subTitle={error}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/qualifications')}>
            返回列表
          </Button>,
        ]}
      />
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }} className="animate-in fade-in duration-500">
      <div className="flex justify-between items-center w-full">
        <Space size="middle" style={{ flex: 1, minWidth: 0, alignItems: 'center' }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate(-1)} />
          <Title level={3} style={{ margin: 0 }}>
            {qual.qualification_name}
          </Title>
          <Tooltip title="编辑资质">
            <Link to={`/library/qualifications/${qual.id}/edit`}>
              <Button type="text" shape="circle" icon={<EditOutlined />} style={{ color: 'rgba(0,0,0,0.45)' }} />
            </Link>
          </Tooltip>
        </Space>
      </div>

      <Card className="shadow-sm border-none bg-white" title="证书信息">
        <Descriptions column={{ xs: 1, sm: 2 }} bordered size="middle">
          <Descriptions.Item label="证书名称" span={2}>
            {qual.qualification_name}
          </Descriptions.Item>
          <Descriptions.Item label="证书编号">{qual.certificate_no || '-'}</Descriptions.Item>
          <Descriptions.Item label="资质等级/类别">
            {(qual.qualification_level === '有限责任公司（自然人投资或控股）' && qual.qualification_name === '营业执照') 
              ? '营业执照' 
              : (qual.qualification_level || '-')}
          </Descriptions.Item>
          <Descriptions.Item label="颁发/起始日期">{qual.valid_from || '-'}</Descriptions.Item>
          <Descriptions.Item label="有效期至">
            <Space>
              <Text>{qual.valid_to || '长期 / 未填'}</Text>
              {validStatusTag(qual.valid_to)}
            </Space>
          </Descriptions.Item>
          <Descriptions.Item label="发证机关" span={2}>{qual.issuing_authority || '-'}</Descriptions.Item>
          <Descriptions.Item label="投标可用性">{labelBidUsable(qual.bid_usable_status)}</Descriptions.Item>
          <Descriptions.Item label="查看原件">
            {(qual.stored_path || qual.file_asset_id) ? (
              <Button type="link" icon={<EyeOutlined />} onClick={showOriginal} style={{ paddingInline: 0 }}>
                查看原件
              </Button>
            ) : (
              '-'
            )}
          </Descriptions.Item>
          {qual.owner_type === 'person' && qual.owner_id ? (
            <Descriptions.Item label="关联人员" span={2}>
              <Link to={`/library/persons/${qual.owner_id}`} className="text-blue-600 hover:underline">
                查看人员详情
              </Link>
            </Descriptions.Item>
          ) : null}
        </Descriptions>
      </Card>
    </div>
  );
};

export default QualificationDetail;
