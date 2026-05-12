import React, { useState, useEffect } from 'react';
import { Table, Button, Space, Input, Tag, Card, Row, Col, Typography, message, Modal, Form, Select, Tooltip, Image, Popconfirm, Spin } from 'antd';
import { PlusOutlined, SearchOutlined, FilterOutlined, UserOutlined, AlertOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import { useNavigate, Link, useSearchParams } from 'react-router-dom';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;
const { Option } = Select;

interface Person {
  id: string;
  name: string;
  role_type?: string;
  specialty?: string;
  id_number_masked?: string;
  id_card_no?: string;
  company_name?: string;
  social_security_status?: string;
  on_job_status?: string;
  bid_usable_status?: string;
  risk_status: string;
  created_at: string;
  __unique_key__?: string;
  certificates?: {
    id: string;
    qualification_name: string;
    qualification_type?: string;
    specialty?: string;
    registration_no?: string;
    certificate_no?: string;
    valid_from?: string;
    valid_to?: string;
    stored_path?: string;
    ext?: string;
    file_asset_id?: string;
  }[];
}

const PersonLibrary: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<Person[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchText, setSearchText] = useState(searchParams.get('q') || '');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [pageSize, setPageSize] = useState(10);
  const [socialFilter, setSocialFilter] = useState('all');

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchParams(prev => {
        if (searchText) prev.set('q', searchText);
        else prev.delete('q');
        return prev;
      }, { replace: true });
    }, 300);
    return () => clearTimeout(timer);
  }, [searchText, setSearchParams]);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [form] = Form.useForm();
  const { currentCompanyId } = useCompany();

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const response = await axios.get('/api/persons', {
        headers: { 'x-company-id': currentCompanyId }
      });
      const processed = response.data.map((item: any, index: number) => ({
        ...item,
        __unique_key__: item.id ? `${item.id}-idx${index}` : `noid-idx${index}`
      }));
      setData(processed);
    } catch (err) {
      console.error('Failed to fetch persons:', err);
    } finally {
      setLoading(false);
    }
  }, [currentCompanyId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleAdd = () => setIsModalVisible(true);

  const handleDelete = async (id: string, name: string) => {
    try {
      await axios.delete(`/api/persons/${id}`, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success(`人员“${name}”已从库中删除`);
      fetchData();
    } catch (err: unknown) {
      console.error('Delete error:', err);
      message.error('删除人员失败');
    }
  };

  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) return;
    Modal.confirm({
      title: '确认批量删除',
      content: `确定要删除选中的 ${selectedRowKeys.length} 个人员吗？此操作不可撤销。`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const promises = selectedRowKeys.map(key => {
            const strKey = String(key);
            if (strKey.startsWith('noid-')) return Promise.resolve();
            const parts = strKey.split('-idx');
            const actualId = parts[0];
            return axios.delete(`/api/persons/${actualId}`, {
              headers: { 'x-company-id': currentCompanyId }
            });
          });
          await Promise.all(promises);
          message.success(`成功删除选中人员`);
          setSelectedRowKeys([]);
          fetchData();
        } catch (err) {
          message.error('批量删除出现部分或全部失败，请刷新查看');
          setSelectedRowKeys([]);
          fetchData();
        }
      },
    });
  };

  const handleCancel = () => {
    setIsModalVisible(false);
    form.resetFields();
  };

  const onFinish = async (values: Partial<Person>) => {
    try {
      await axios.post('/api/persons', values, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('人员信息创建成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchData();
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      message.error('创建失败：' + errorMessage);
    }
  };

  const renderHighlightedText = (text: string | null | undefined, highlight: string) => {
    if (!text) return '-';
    if (!highlight) return <>{text}</>;

    // Escape special regex characters in the search text
    const escapedHighlight = highlight.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const parts = text.split(new RegExp(`(${escapedHighlight})`, 'gi'));

    return (
      <span style={{ display: 'inline-block' }}>
        {parts.map((part, index) =>
          part.toLowerCase() === highlight.toLowerCase() ? (
            <span key={index} style={{ color: '#ff4d4f', fontWeight: 'bold' }}>
              {part}
            </span>
          ) : (
            <span key={index}>{part}</span>
          )
        )}
      </span>
    );
  };

  const columns = [
    {
      title: '人员 ID',
      dataIndex: 'id',
      key: 'id',
      width: 100,
      render: (id: string) => <Text type="secondary" style={{ fontSize: 11, fontFamily: 'monospace' }}>{id}</Text>,
    },
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: Person) => (
        <Space>
          <Link to={`/library/persons/${record.id}`} className="text-blue-600 hover:underline">
            {renderHighlightedText(text, searchText)}
          </Link>
          {record.risk_status === 'warning' && (
            <Tooltip title="资料存在异常">
              <AlertOutlined className="text-orange-500" />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: '人员类别/资格证书',
      key: 'role_certificates',
      render: (_: any, record: Person) => {
        if (!record.certificates || record.certificates.length === 0) {
          return <Tag color="default">未设置</Tag>;
        }
        return (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            {record.certificates.map((cert, idx) => {
              const fullText = [
                cert.qualification_name,
                cert.specialty || record.specialty // 如果证书库里还没存专业（旧数据），就拿人员的大专业兜底
              ].filter(Boolean).join(' / ');
              return (
                <div key={cert.id || idx}>
                  <Tag color="blue" style={{ margin: 0 }}>
                    {renderHighlightedText(fullText, searchText)}
                  </Tag>
                </div>
              );
            })}
          </div>
        );
      }
    },
    {
      title: '身份证号',
      dataIndex: 'id_card_no',
      key: 'id_card_no',
      render: (text: string) => renderHighlightedText(text || '-', searchText),
    },
    {
      title: '社保状态',
      dataIndex: 'social_security_status',
      key: 'social_security_status',
      render: (status: string) => {
        if (status === 'active') return <Tag color="green">已缴纳</Tag>;
        if (status === 'none') return <Tag color="orange">未缴纳</Tag>;
        return <Tag color="default">{status || '未知'}</Tag>;
      }
    },
    {
      title: '投标可用性',
      dataIndex: 'bid_usable_status',
      key: 'bid_usable_status',
      render: (status: string) => (
        <Tag color={status === 'usable' ? 'green' : 'red'}>
          {status === 'usable' ? '可用' : '锁定中'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 130,
      fixed: 'right' as const,
      align: 'center' as const,
      render: (_: unknown, record: Person) => (
        <Space size="small">
          <Tooltip title={record.certificates && record.certificates.length > 0 ? "查看电子原件" : "暂无原件"}>
            <Button
              type="text"
              size="small"
              icon={<EyeOutlined style={{ color: record.certificates && record.certificates.length > 0 ? '#52c41a' : '#bfbfbf' }} />}
              disabled={!record.certificates || record.certificates.length === 0}
              onClick={() => {
                const certs = record.certificates?.filter(c => c.stored_path || c.file_asset_id) || [];
                if (certs.length === 0) {
                  message.info('暂无电子原件');
                  return;
                }
                Modal.info({
                  title: `原件预览 - ${record.name}`,
                  width: 1000,
                  centered: true,
                  maskClosable: true,
                  icon: null,
                  content: (
                    <div style={{ textAlign: 'center', padding: '10px 0' }}>
                      <div style={{ marginBottom: 16, textAlign: 'left', color: '#888', fontSize: 12 }}>
                        提示：点击预览下方证书原件，支持 PDF 和 图片 格式。
                      </div>
                      {certs.map((c, i) => {
                        const previewUrl = c.file_asset_id
                          ? `/api/files/download/${c.file_asset_id}`
                          : `/api/files/download/${c.id}`;

                        const isPdf = (c.ext || '').toLowerCase() === '.pdf' ||
                          (c.stored_path || '').toLowerCase().endsWith('.pdf') ||
                          (c.qualification_name || '').toLowerCase().includes('.pdf');

                        return (
                          <div key={i} style={{ marginBottom: 32, border: '1px solid #f0f0f0', borderRadius: 12, overflow: 'hidden', background: '#fff' }}>
                            <div style={{ padding: '12px 20px', background: '#fafafa', borderBottom: '1px solid #f0f0f0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                              <Text strong>{c.qualification_name}</Text>
                              <Space>
                                <Button size="small" onClick={() => window.open(previewUrl, '_blank')}>在新窗口打开</Button>
                                <Button size="small" type="primary" href={previewUrl} download>下载</Button>
                              </Space>
                            </div>
                            <div style={{ padding: 16, background: '#fff' }}>
                              {isPdf ? (
                                <iframe
                                  src={`${previewUrl}#view=FitH`}
                                  style={{ width: '100%', height: '600px', border: 'none', borderRadius: 4 }}
                                  title={c.qualification_name}
                                />
                              ) : (
                                <Image
                                  src={previewUrl}
                                  placeholder={<div style={{ padding: 50 }}><Spin tip="图片加载中..." /></div>}
                                  style={{ maxWidth: '100%', borderRadius: 4, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
                                />
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ),
                  okText: '关闭'
                });
              }}
            />
          </Tooltip>

          <Popconfirm
            title="狠心删除？"
            description={`确定删除人员“${record.name}”吗？此操作不可撤销。`}
            onConfirm={() => handleDelete(record.id, record.name)}
            okText="确定"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Tooltip title="删除数据">
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined style={{ color: 'rgba(0,0,0,0.45)' }} />}
              />
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
            placeholder="搜索姓名、人员类别/资格证书"
            prefix={<SearchOutlined />}
            allowClear
            value={searchText}
            onChange={e => setSearchText(e.target.value)}
            style={{ width: 320 }}
          />
          <Select value={socialFilter} onChange={v => setSocialFilter(v)} style={{ width: 120 }}>
            <Option value="all">社保状态</Option>
            <Option value="active">已缴纳</Option>
            <Option value="none">未缴纳</Option>
            <Option value="unknown">未知</Option>
          </Select>
        </Space>
      </div>

      <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table
          columns={columns}
          dataSource={data.filter(item => {
            if (socialFilter !== 'all') {
              if (socialFilter === 'unknown' && item.social_security_status) return false;
              if (socialFilter !== 'unknown' && item.social_security_status !== socialFilter) return false;
            }
            if (!searchText) return true;
            const text = searchText.toLowerCase();
            return (
              item.name?.toLowerCase().includes(text) ||
              item.company_name?.toLowerCase().includes(text) ||
              item.role_type?.toLowerCase().includes(text) ||
              item.specialty?.toLowerCase().includes(text) ||
              item.certificates?.some(cert =>
                cert.qualification_name?.toLowerCase().includes(text) ||
                cert.specialty?.toLowerCase().includes(text)
              )
            );
          })}
          loading={loading}
          rowKey="__unique_key__"
          rowSelection={{
            selectedRowKeys,
            onChange: (keys) => setSelectedRowKeys(keys),
          }}
          pagination={{
            pageSize,
            onChange: (page, size) => setPageSize(size),
            onShowSizeChange: (current, size) => setPageSize(size),
            pageSizeOptions: ['10', '20', '50', '100'],
            showSizeChanger: true,
            showTotal: (total) => (
              <Space style={{ marginRight: 16 }}>
                <Space size="middle">
                  <Text>已选 {selectedRowKeys.length}/{pageSize} 人</Text>
                  <Button
                    size="small"
                    disabled={selectedRowKeys.length === 0}
                    onClick={handleBatchDelete}
                  >
                    批量删除
                  </Button>
                </Space>
                <Text style={{ marginLeft: 16 }}>共 {total} 人</Text>
              </Space>
            )
          }}
          size="middle"
        />
      </Card>

      <Modal
        title="新增库内人员"
        open={isModalVisible}
        onCancel={handleCancel}
        footer={null}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{ on_job_status: 'active', social_security_status: 'none', bid_usable_status: 'usable' }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="id" label="人员ID">
                <Input placeholder="无需填写，系统后自动生成" disabled />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="name" label="姓名" rules={[{ required: true }]}>
                <Input placeholder="请输入姓名" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="id_card_no" label="身份证号">
                <Input placeholder="请输入真实身份证号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="on_job_status" label="在职状态">
                <Select>
                  <Option value="active">在职</Option>
                  <Option value="resigned">离职</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="social_security_status" label="社保状态">
                <Select>
                  <Option value="active">已缴纳 (正常)</Option>
                  <Option value="none">未缴纳 / 异常</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="bid_usable_status" label="投标可用性">
                <Select>
                  <Option value="usable">可用 (未锁定)</Option>
                  <Option value="locked">锁定 (项目执行中)</Option>
                  <Option value="restricted">受限</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          <Form.Item className="mb-0 text-right mt-4">
            <Space>
              <Button onClick={handleCancel}>取消</Button>
              <Button type="primary" htmlType="submit">创建入库</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default PersonLibrary;
