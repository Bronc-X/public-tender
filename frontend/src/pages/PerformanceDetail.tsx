import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Typography, Button, Space, Descriptions, Spin, Result, Divider, Empty, Upload, Modal, List, message, Popconfirm, Tag, Tooltip, Alert } from 'antd';
import { LeftOutlined, EditOutlined, HomeOutlined, ProjectOutlined, CalendarOutlined, EnvironmentOutlined, DollarOutlined, UserOutlined, UploadOutlined, FilePdfOutlined, FileImageOutlined, DeleteOutlined, EyeOutlined, FileWordOutlined, DownloadOutlined } from '@ant-design/icons';
import axios from 'axios';
import dayjs from 'dayjs';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Title, Text } = Typography;

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
  documentation_officer_name?: string;
  documentation_officer_id?: string;
  materials_officer_name?: string;
  materials_officer_id?: string;
  quality_inspector_name?: string;
  quality_inspector_id?: string;
  construction_officer_name?: string;
  construction_officer_id?: string;
  standards_officer_name?: string;
  standards_officer_id?: string;
  mechanical_officer_name?: string;
  mechanical_officer_id?: string;
  labor_officer_name?: string;
  labor_officer_id?: string;
  risk_status: string;
  created_at: string;
  proofs?: PerformanceProof[];
}

interface PerformanceProof {
  id: string;
  project_performance_id: string;
  proof_type?: string;
  file_asset_id?: string;
  is_primary?: number;
  file_name?: string;
  ext?: string;
  created_at?: string;
  markdown_text?: string;
}

const PerformanceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewUrl, setPreviewUrl] = useState('');
  const [previewTitle, setPreviewTitle] = useState('');
  const [previewType, setPreviewType] = useState<'image' | 'pdf' | 'word' | 'other'>('other');
  const [previewMarkdown, setPreviewMarkdown] = useState<string | null>(null);

  const { Link } = Typography;

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        const response = await axios.get(`/api/performances/${id}`);
        setProject(response.data);
      } catch (err: unknown) {
        console.error('Failed to fetch performance detail:', err);
        const apiErr = err as { response?: { data?: { error?: string } } };
        setError(apiErr.response?.data?.error || '无法获取业绩详情');
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [id]);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入业绩数据..." />
      </div>
    );
  }

  if (error || !project) {
    return (
      <Result
        status="error"
        title="获取详情失败"
        subTitle={error}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/performances')}>
            返回列表
          </Button>
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
            {project.project_name}
          </Title>
          <Tooltip title="编辑业绩">
            <Button
              type="text"
              shape="circle"
              icon={<EditOutlined />}
              onClick={() => navigate(`/library/performances/${project.id}/edit`)}
              style={{ color: 'rgba(0,0,0,0.45)' }}
            />
          </Tooltip>
        </Space>
      </div>

      <Card className="shadow-sm border-none">
        <Descriptions
          title="基本信息"
          bordered
          column={{ xxl: 3, xl: 2, lg: 2, md: 1, sm: 1, xs: 1 }}
          labelStyle={{ width: 120, whiteSpace: 'nowrap' }}
        >
          <Descriptions.Item label={<Space><ProjectOutlined /> 项目名称</Space>} span={2}>
            <Text strong>{project.project_name}</Text>
          </Descriptions.Item>
          <Descriptions.Item label={<Space><EnvironmentOutlined /> 建设地点</Space>}>
            {project.project_location || '暂无地址'}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><HomeOutlined /> 发包人 (业主)</Space>} span={3}>
            {project.owner_org || '暂无信息'}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><DollarOutlined /> 中标金额 (万元)</Space>}>
            <Text>{(project.bid_amount_value || 0).toLocaleString()}</Text>
          </Descriptions.Item>
          <Descriptions.Item label={<Space><DollarOutlined /> 合同金额 (万元)</Space>}>
            <Text type="warning" strong>{(project.amount_value || 0).toLocaleString()} 万元</Text>
          </Descriptions.Item>
          <Descriptions.Item label={<Space><CalendarOutlined /> 中标日期</Space>}>
            {project.winning_date ? dayjs(project.winning_date).format('YYYY-MM-DD') : '-'}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><CalendarOutlined /> 完工日期</Space>}>
            {project.completion_date ? dayjs(project.completion_date).format('YYYY-MM-DD') : '-'}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><CalendarOutlined /> 建设工期</Space>}>
            {project.construction_period || '--'}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 项目经理</Space>}>
            {project.pm_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.pm_id}`)} strong>
                {project.project_manager_name}
              </Link>
            ) : (
              <Text strong>{project.project_manager_name || '--'}</Text>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><ProjectOutlined /> 建设规模/指标</Space>} span={3}>
            <div style={{ whiteSpace: 'pre-wrap' }}>{project.scale_desc || '--'}</div>
          </Descriptions.Item>
        </Descriptions>

        <Divider style={{ margin: '28px 0' }} />

        <Descriptions title="关键人员信息" bordered column={{ xxl: 3, xl: 3, lg: 2, md: 1, sm: 1, xs: 1 }}>
          <Descriptions.Item label={<Space><UserOutlined /> 技术负责人</Space>}>
            {project.tech_leader_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.tech_leader_id}`)}>
                {project.technical_leader_name}
              </Link>
            ) : (
              project.technical_leader_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 安全员</Space>}>
            {project.safety_leader_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.safety_leader_id}`)}>
                {project.safety_leader_name}
              </Link>
            ) : (
              project.safety_leader_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 施工员</Space>}>
            {project.construction_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.construction_officer_id}`)} strong>
                {project.construction_officer_name}
              </Link>
            ) : (
              <Text strong>{project.construction_officer_name || '--'}</Text>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 质量员</Space>}>
            {project.quality_inspector_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.quality_inspector_id}`)}>
                {project.quality_inspector_name}
              </Link>
            ) : (
              project.quality_inspector_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 资料员</Space>}>
            {project.documentation_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.documentation_officer_id}`)}>
                {project.documentation_officer_name}
              </Link>
            ) : (
              project.documentation_officer_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 材料员</Space>}>
            {project.materials_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.materials_officer_id}`)}>
                {project.materials_officer_name}
              </Link>
            ) : (
              project.materials_officer_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 标准员</Space>}>
            {project.standards_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.standards_officer_id}`)}>
                {project.standards_officer_name}
              </Link>
            ) : (
              project.standards_officer_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 机械员</Space>}>
            {project.mechanical_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.mechanical_officer_id}`)}>
                {project.mechanical_officer_name}
              </Link>
            ) : (
              project.mechanical_officer_name || '--'
            )}
          </Descriptions.Item>
          <Descriptions.Item label={<Space><UserOutlined /> 劳务员</Space>}>
            {project.labor_officer_id ? (
              <Link onClick={() => navigate(`/library/persons/${project.labor_officer_id}`)}>
                {project.labor_officer_name}
              </Link>
            ) : (
              project.labor_officer_name || '--'
            )}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        title={<Space><ProjectOutlined /> 合同附件</Space>}
        className="shadow-sm border-none"
        extra={
          <Upload
            showUploadList={false}
            customRequest={async (options) => {
              const { file, onSuccess, onError } = options;
              const formData = new FormData();
              formData.append('file', file);
              formData.append('source_module', 'performance');
              formData.append('source_project_id', id || '');

              try {
                // 1. Upload file asset
                const uploadRes = await axios.post('/api/files/upload', formData);
                const fileAssetId = uploadRes.data.id;

                // 2. Bind to performance
                await axios.post(`/api/performances/${id}/proofs`, {
                  file_asset_id: fileAssetId,
                  proof_type: 'contract'
                });

                message.success('附件上传成功');

                // Refresh data
                const response = await axios.get(`/api/performances/${id}`);
                setProject(response.data);

                if (onSuccess) onSuccess("ok");
              } catch (err) {
                console.error('Upload failed:', err);
                message.error('附件上传失败');
                if (onError) onError(err as Error);
              }
            }}
          >
            <Button icon={<UploadOutlined />} type="primary" ghost>上传附件</Button>
          </Upload>
        }
      >
        {!project.proofs || project.proofs.length === 0 ? (
          <Empty description="暂无关联图片或文档" />
        ) : (
          <List
            grid={{ gutter: 16, xxl: 4, xl: 3, lg: 2, md: 2, sm: 1, xs: 1 }}
            dataSource={project.proofs}
            renderItem={(item) => (
              <List.Item>
                <Card
                  size="small"
                  hoverable
                  className="bg-gray-50 border-gray-200"
                  actions={[
                    <Tooltip title="预览" key="view">
                      <EyeOutlined onClick={() => handlePreview(item)} />
                    </Tooltip>,
                    <Popconfirm
                      key="delete"
                      title="确定删除此附件吗？"
                      onConfirm={() => handleDeleteProof(item.id)}
                      okText="确定"
                    >
                      <DeleteOutlined style={{ color: 'rgba(0, 0, 0, 0.45)' }} />
                    </Popconfirm>,
                  ]}
                >
                  <Card.Meta
                    avatar={
                      item.ext === '.pdf' ? (
                        <FilePdfOutlined style={{ fontSize: 32, color: '#ff4d4f' }} />
                      ) : item.ext?.includes('.doc') ? (
                        <FileWordOutlined style={{ fontSize: 32, color: '#1890ff' }} />
                      ) : (
                        <FileImageOutlined style={{ fontSize: 32, color: '#1890ff' }} />
                      )
                    }
                    title={
                      <div className="truncate" style={{ maxWidth: '100%' }}>
                        <Text strong onClick={() => handlePreview(item)} style={{ cursor: 'pointer' }}>
                          {item.file_name}
                        </Text>
                      </div>
                    }
                    description={
                      <Space direction="vertical" size={0}>
                        <Tag color="blue">合同附件</Tag>
                        <Text type="secondary" style={{ fontSize: '12px' }}>
                          上传于 {dayjs(item.created_at).format('YYYY-MM-DD')}
                        </Text>
                      </Space>
                    }
                  />
                </Card>
              </List.Item>
            )}
          />
        )}
      </Card>

      <Modal
        open={previewVisible}
        title={previewTitle}
        footer={null}
        onCancel={() => setPreviewVisible(false)}
        width={previewType === 'pdf' ? '80%' : 1000}
        style={{ top: 20 }}
        styles={{
          body: {
            height: previewType === 'pdf' ? 'calc(100vh - 150px)' : 'calc(90vh - 150px)',
            overflow: 'auto',
            textAlign: 'center',
            padding: previewType === 'word' ? 0 : 24,
            backgroundColor: '#ffffff'
          }
        }}
      >
        {previewType === 'image' && (
          <div style={{ backgroundColor: '#ffffff', minHeight: '300px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
            <img alt="preview" style={{ maxWidth: '100%', display: 'block' }} src={previewUrl} />
          </div>
        )}
        {previewType === 'pdf' && (
          <div style={{ backgroundColor: '#ffffff', height: '100%', display: 'flex', flexDirection: 'column' }}>
            <div style={{ flex: 1, minHeight: 0 }}>
              <iframe
                src={previewUrl}
                style={{ width: '100%', height: '100%', border: 'none' }}
                title="pdf-preview"
              />
            </div>
            <div style={{ padding: '12px 0', borderTop: '1px solid #f0f0f0', display: 'flex', justifyContent: 'center', gap: '16px' }}>
              <Text type="secondary">预览异常？</Text>
              <Button size="small" type="link" onClick={() => window.open(previewUrl)}>在新标签页打开</Button>
              <Button size="small" type="link" onClick={() => window.open(previewUrl)}>下载文件</Button>
            </div>
          </div>
        )}
        {previewType === 'word' && (
          <div className="text-left py-10 px-8 bg-white min-h-full" style={{ backgroundColor: '#ffffff' }}>
            <Alert
              message="Word 文档智能预览说明"
              description="当前系统对 Word 文档采用智能文本模式展示内容，如需查看原始精美格式，请点击下方下载原件。"
              type="info"
              showIcon
              className="mb-8"
              style={{ marginBottom: '2rem' }}
            />
            <div className="prose max-w-none p-10 rounded-lg shadow-sm border border-gray-100" style={{ backgroundColor: '#ffffff', minHeight: '600px', color: '#1a1a1a' }}>
              {previewMarkdown ? (
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {previewMarkdown}
                </ReactMarkdown>
              ) : (
                <div style={{ padding: '80px 0', textAlign: 'center' }}>
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={
                      <Space direction="vertical" size="middle">
                        <Text type="secondary" style={{ fontSize: '16px' }}>该文档尚未完成智能解析或内容为空</Text>
                        <Button size="large" icon={<DownloadOutlined />} onClick={() => window.open(previewUrl)}>下载原件查看</Button>
                      </Space>
                    }
                  />
                </div>
              )}
            </div>
            {previewMarkdown && (
              <div className="mt-12 text-center sticky bottom-0 bg-white py-6 border-t border-gray-100 flex justify-center gap-4">
                <Button
                  size="large"
                  type="primary"
                  icon={<DownloadOutlined />}
                  onClick={() => window.open(previewUrl)}
                >
                  下载原始文档 (.{previewUrl.split('.').pop()})
                </Button>
                <Button size="large" onClick={() => setPreviewVisible(false)}>
                  关闭预览
                </Button>
              </div>
            )}
          </div>
        )}
        {previewType === 'other' && (
          <div className="py-24 text-center bg-white" style={{ backgroundColor: '#ffffff' }}>
            <Empty
              description={
                <Space direction="vertical" size="large">
                  <Text type="secondary" style={{ fontSize: '18px' }}>暂不支持该格式的在线预览</Text>
                  <Button size="large" icon={<DownloadOutlined />} type="primary" onClick={() => window.open(previewUrl)}>
                    下载附件查看
                  </Button>
                </Space>
              }
            />
          </div>
        )}
      </Modal>
    </div>
  );

  function handlePreview(item: PerformanceProof) {
    const url = `/api/files/download/${item.file_asset_id}/view.pdf`;
    const ext = item.ext?.toLowerCase();

    setPreviewUrl(url);
    setPreviewTitle(item.file_name || '文件预览');

    if (ext === '.pdf') {
      setPreviewType('pdf');
    } else if (ext?.includes('.doc')) {
      setPreviewType('word');
      setPreviewMarkdown(item.markdown_text || null);
    } else if (['.jpg', '.jpeg', '.png', '.gif', '.webp'].includes(ext || '')) {
      setPreviewType('image');
    } else {
      setPreviewType('other');
    }

    setPreviewVisible(true);
  }

  async function handleDeleteProof(proofId: string) {
    try {
      await axios.delete(`/api/performances/proofs/${proofId}`);
      message.success('附件已删除');

      // Refresh data
      const response = await axios.get(`/api/performances/${id}`);
      setProject(response.data);
    } catch (err) {
      console.error('Delete failed:', err);
      message.error('删除失败');
    }
  }
};

export default PerformanceDetail;
