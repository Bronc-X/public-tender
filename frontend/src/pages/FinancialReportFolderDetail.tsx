import React, { useState, useEffect } from 'react';
import { Typography, Upload, Image, Button, Space, message, Spin, Empty, Popconfirm, Input, Card } from 'antd';
import { UploadOutlined, LeftOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

const FinancialReportFolderDetail: React.FC = () => {
  const { folderId } = useParams<{ folderId: string }>();
  const [searchParams] = useSearchParams();
  const folderName = searchParams.get('name') || '文件夹详情';
  const { currentCompanyId } = useCompany();
  const navigate = useNavigate();

  const [files, setFiles] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingFileId, setEditingFileId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');

  const fetchFiles = async () => {
    setLoading(true);
    try {
      const res = await axios.get(`/api/financial-reports/folders/${folderId}/files`);
      setFiles(res.data || []);
    } catch (err) {
      console.error(err);
      message.error('加载图片失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchFiles();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folderId, currentCompanyId]);

  const handleDelete = async (fileId: string) => {
    try {
      await axios.delete(`/api/files/${fileId}`);
      message.success('已删除');
      fetchFiles();
    } catch (err) {
      console.error(err);
      message.error('删除失败');
    }
  };

  const handleRename = async (fileId: string) => {
    if (!editingName.trim()) {
      message.warning('文件名不能为空');
      return;
    }
    try {
      await axios.patch(`/api/files/${fileId}`, { file_name: editingName.trim() });
      message.success('重命名成功');
      setEditingFileId(null);
      fetchFiles();
    } catch (err) {
      console.error(err);
      message.error('重命名失败');
    }
  };

  const uploadProps = {
    action: '/api/files/upload',
    headers: {
      'X-Company-Id': currentCompanyId,
      Authorization: `Bearer ${localStorage.getItem('token')}`,
    },
    data: {
      source_module: 'financial_report',
      source_project_id: folderId,
    },
    accept: 'image/*',
    showUploadList: false,
    multiple: true,
    onChange: (info: any) => {
      if (info.file.status === 'done') {
        message.success(`${info.file.name} 上传成功`);
        fetchFiles();
      } else if (info.file.status === 'error') {
        message.error(`${info.file.name} 上传失败`);
      }
    },
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24, padding: '0 0 24px 0' }} className="animate-in fade-in duration-500">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
        <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate('/library/financial-reports')} />
          <Title level={3} style={{ margin: 0 }}>
            {folderName}
          </Title>
        </Space>
        <Space size="middle" style={{ marginLeft: 24, flexShrink: 0 }}>
          <Upload {...uploadProps}>
            <Button type="primary" icon={<UploadOutlined />}>上传扫描件</Button>
          </Upload>
        </Space>
      </div>

      <Card
        bordered={false}
        style={{ borderRadius: 12, boxShadow: 'none' }}
        styles={{ body: { padding: '0 0 24px 0' } }}
      >
        <Spin spinning={loading}>
          {files.length === 0 && !loading ? (
            <Empty description="暂无图片数据，请点击右上角上传" style={{ marginTop: 60 }} />
          ) : (
            <Image.PreviewGroup>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24 }}>
                {files.map(file => (
                  <div key={file.id} style={{ display: 'flex', flexDirection: 'column', width: 200 }}>
                    <div style={{ position: 'relative', width: 200, height: 260, borderRadius: 8, overflow: 'hidden', border: '1px solid #e2e8f0', background: '#f8fafc', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <Image
                        src={`/api/files/${file.id}/binary`}
                        alt={file.file_name}
                        style={{ maxWidth: '100%', maxHeight: 260, objectFit: 'contain' }}
                        fallback="https://gw.alipayobjects.com/zos/antfincdn/aPkFc8Sj7n/method-draw-image.svg"
                      />
                      <div style={{ position: 'absolute', top: 8, right: 8 }}>
                        <Popconfirm title="确定要删除这张图片吗？" onConfirm={() => handleDelete(file.id)}>
                          <Button type="text" shape="circle" icon={<DeleteOutlined style={{ color: '#94a3b8' }} />} size="small" />
                        </Popconfirm>
                      </div>
                    </div>
                    <div style={{ marginTop: 8, textAlign: 'center' }}>
                      {editingFileId === file.id ? (
                        <Input
                          size="small"
                          value={editingName}
                          onChange={(e) => setEditingName(e.target.value)}
                          onPressEnter={() => handleRename(file.id)}
                          onBlur={() => handleRename(file.id)}
                          autoFocus
                          style={{ textAlign: 'center' }}
                        />
                      ) : (
                        <Text
                          ellipsis={{ tooltip: file.file_name }}
                          style={{ fontWeight: 500, color: '#334155', cursor: 'pointer' }}
                          onClick={() => {
                            setEditingFileId(file.id);
                            setEditingName(file.file_name || '');
                          }}
                        >
                          {file.file_name} <EditOutlined style={{ fontSize: 12, color: '#94a3b8' }} />
                        </Text>
                      )}
                    </div>
                    <Text type="secondary" style={{ fontSize: 12, textAlign: 'center' }}>
                      {new Date(file.created_at).toLocaleDateString()}
                    </Text>
                  </div>
                ))}
              </div>
            </Image.PreviewGroup>
          )}
        </Spin>
      </Card>
    </div>
  );
};

export default FinancialReportFolderDetail;
