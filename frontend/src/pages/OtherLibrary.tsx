import React, { useState, useEffect, useRef } from 'react';
import { Typography, Row, Col, Modal, Form, Input, message, Spin, Empty, Button, Dropdown, Space, Card } from 'antd';
import { FolderFilled, PlusOutlined, EllipsisOutlined, ExclamationCircleOutlined, FileOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;

interface Folder {
  id: string;
  folder_name: string;
  created_at: string;
}

const OtherLibrary: React.FC = () => {
  const [folders, setFolders] = useState<Folder[]>([]);
  const [loading, setLoading] = useState(true);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [editingFolderId, setEditingFolderId] = useState<string | null>(null);
  const [editingFolderName, setEditingFolderName] = useState('');
  const renamingRef = useRef(false);
  const { currentCompanyId } = useCompany();
  const navigate = useNavigate();
  const [form] = Form.useForm();

  const fetchFolders = async () => {
    setLoading(true);
    try {
      const res = await axios.get('/api/others/folders');
      setFolders(res.data || []);
    } catch (err) {
      console.error(err);
      message.error('加载其他文件夹失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchFolders();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentCompanyId]);

  const handleCreateFolder = async (values: { folder_name: string }) => {
    setSubmitting(true);
    try {
      await axios.post('/api/others/folders', values);
      message.success('创建成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchFolders();
    } catch (err) {
      console.error(err);
      message.error('创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    Modal.confirm({
      title: '确定要删除该文件夹吗？',
      icon: <ExclamationCircleOutlined />,
      content: '删除后，文件夹内的所有图片档案关系将解除。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await axios.delete(`/api/others/folders/${id}`);
          message.success('删除成功');
          fetchFolders();
        } catch (err) {
          console.error(err);
          message.error('删除失败');
        }
      },
    });
  };

  const handleRename = async (id: string) => {
    // Guard against double-fire (Enter + blur both trigger this)
    if (renamingRef.current) return;
    if (!editingFolderName.trim()) {
      setEditingFolderId(null);
      return;
    }
    renamingRef.current = true;
    try {
      await axios.patch(`/api/others/folders/${id}`, { folder_name: editingFolderName.trim() });
      message.success('重命名成功');
      setEditingFolderId(null);
      fetchFolders();
    } catch (err) {
      console.error(err);
      message.error('重命名失败');
    } finally {
      renamingRef.current = false;
    }
  };

  const cancelRename = () => {
    setEditingFolderId(null);
    setEditingFolderName('');
  };

  return (
    <div style={{ padding: '0 0 24px 0' }}>

      <Card
        bordered={false}
        style={{ borderRadius: 12, boxShadow: 'none' }}
        styles={{ body: { padding: '0 0 24px 0' } }}
      >
        <Spin spinning={loading}>
          {folders.length === 0 && !loading ? (
            <Empty description="暂无文件夹" style={{ marginTop: 60 }}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
                新建文件夹
              </Button>
            </Empty>
          ) : (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '32px' }}>
              <div
                style={{
                  width: 120,
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  cursor: 'pointer',
                  opacity: 0.8,
                  transition: 'opacity 0.2s'
                }}
                onClick={() => setIsModalVisible(true)}
                onMouseEnter={(e) => (e.currentTarget.style.opacity = '1')}
                onMouseLeave={(e) => (e.currentTarget.style.opacity = '0.8')}
              >
                <div style={{
                  width: 80, height: 80, background: '#f1f5f9', borderRadius: 12,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  marginBottom: 8, border: '2px dashed #cbd5e1'
                }}>
                  <PlusOutlined style={{ fontSize: 24, color: '#94a3b8' }} />
                </div>
                <Text strong style={{ color: '#64748b' }}>新建</Text>
              </div>

              {folders.map(folder => (
                <div
                  key={folder.id}
                  className="group"
                  style={{
                    width: 120,
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    cursor: 'pointer',
                    position: 'relative'
                  }}
                >
                  <div style={{ position: 'relative' }} onClick={() => navigate(`/library/others/${folder.id}?name=${encodeURIComponent(folder.folder_name)}`)}>
                    <FolderFilled className="drop-shadow-sm transition-all duration-200 group-hover:scale-105" style={{ fontSize: 80, color: '#fbbf24' }} />
                    <div
                      className="absolute right-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200"
                      style={{ top: 2 }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Dropdown 
                        menu={{ 
                          items: [
                            { key: 'rename', label: '重命名', onClick: (e) => { e.domEvent.stopPropagation(); setEditingFolderId(folder.id); setEditingFolderName(folder.folder_name); } },
                            { key: 'delete', danger: true, label: '删除文件夹', onClick: (e) => { e.domEvent.stopPropagation(); handleDelete(folder.id, e.domEvent as any); } }
                          ] 
                        }} 
                        trigger={['click']}
                      >
                        <Button type="text" size="small" icon={<EllipsisOutlined style={{ fontSize: 18 }} />} style={{ color: '#0ea5e9' }} />
                      </Dropdown>
                    </div>
                  </div>
                  <div style={{ marginTop: 8, textAlign: 'center', width: '100%' }} onClick={(e) => e.stopPropagation()}>
                    {editingFolderId === folder.id ? (
                      <Input
                        size="small"
                        value={editingFolderName}
                        onChange={(e) => setEditingFolderName(e.target.value)}
                        onPressEnter={() => handleRename(folder.id)}
                        onBlur={() => handleRename(folder.id)}
                        onKeyDown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); cancelRename(); } }}
                        autoFocus
                        style={{ textAlign: 'center', width: 120 }}
                      />
                    ) : (
                      <Text style={{ wordBreak: 'break-all', fontWeight: 500 }} ellipsis={{ tooltip: folder.folder_name }}>
                        {folder.folder_name}
                      </Text>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </Spin>
      </Card>

      <Modal
        title="新建其他资源文件夹"
        open={isModalVisible}
        onOk={() => form.submit()}
        onCancel={() => setIsModalVisible(false)}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} onFinish={handleCreateFolder} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="folder_name" label="文件夹名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="输入文件夹名称，如：2023年度汇算" autoFocus />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default OtherLibrary;
