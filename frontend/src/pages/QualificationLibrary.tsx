import React, { useState, useEffect } from 'react';
import { Table, Button, Space, Input, Tag, Card, Row, Col, Typography, message, Modal, Form, DatePicker, Select, Image, Tooltip, Popconfirm, Spin } from 'antd';
import { PlusOutlined, SearchOutlined, SafetyCertificateOutlined, CalendarOutlined, EditOutlined, EyeOutlined, DeleteOutlined } from '@ant-design/icons';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;
const { Option } = Select;

interface Qualification {
  id: string;
  qualification_name: string;
  qualification_level?: string;
  qualification_type: string;
  owner_type?: string;
  owner_id?: string;
  person_owner_name?: string;
  company_owner_name?: string;
  certificate_no: string;
  issuing_authority: string;
  valid_to: string;
  risk_status: string;
  stored_path?: string;
  ext?: string;
  file_asset_id?: string;
  __unique_key__?: string;
  certificates?: {
    id: string;
    qualification_name: string;
    valid_to?: string;
    stored_path?: string;
    ext?: string;
    file_asset_id?: string;
  }[];
}

const QualificationLibrary: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<Qualification[]>([]);
  const [persons, setPersons] = useState<{ id: string, name: string }[]>([]);
  const [searchText, setSearchText] = useState('');
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [form] = Form.useForm();
  const { currentCompanyId } = useCompany();
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [pageSize, setPageSize] = useState(10);

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [qualRes, personRes] = await Promise.all([
        axios.get('/api/qualifications', { headers: { 'x-company-id': currentCompanyId } }),
        axios.get('/api/persons', { headers: { 'x-company-id': currentCompanyId } })
      ]);
      const processed = qualRes.data.map((item: any, index: number) => ({
          ...item,
          __unique_key__: item.id ? `${item.id}-idx${index}` : `noid-idx${index}`
      }));
      setData(processed);
      setPersons(personRes.data);
    } catch (err) {
      console.error('Failed to fetch qualification data:', err);
    } finally {
      setLoading(false);
    }
  }, [currentCompanyId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const onFinish = async (values: any) => {
    try {
      const payload = {
        ...values,
        owner_type: 'company',
        owner_id: currentCompanyId,
        valid_from: values.valid_from ? values.valid_from.format('YYYY-MM-DD') : undefined,
        valid_to: values.valid_to ? values.valid_to.format('YYYY-MM-DD') : undefined,
      };
      await axios.post('/api/qualifications', payload, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('资质证书录入成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error('录入失败: ' + msg);
    }
  };

  const getStatusTag = (validTo: string) => {
    const today = dayjs();
    const expiry = dayjs(validTo);
    const diffDays = expiry.diff(today, 'day');
    if (diffDays < 0) return <Tag color="error">已过期</Tag>;
    if (diffDays <= 90) return <Tag color="warning">即将到期 ({diffDays}天)</Tag>;
    return <Tag color="success">正常</Tag>;
  };

  const handleDelete = async (id: string, name: string) => {
    try {
      await axios.delete(`/api/qualifications/${id}`, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success(`资质证书“${name}”已删除`);
      fetchData();
    } catch (err) {
      console.error('Delete error:', err);
      message.error('删除过程中发生错误');
    }
  };

  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) return;
    Modal.confirm({
      title: '确认批量删除',
      content: `确定要删除选中的 ${selectedRowKeys.length} 个资质吗？此操作不可撤销。`,
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
            return axios.delete(`/api/qualifications/${actualId}`, {
              headers: { 'x-company-id': currentCompanyId }
            });
          });
          await Promise.all(promises);
          message.success(`成功删除选中资质`);
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

  const highlightText = (text: string) => {
    if (!searchText) return text;
    const regex = new RegExp(`(${searchText})`, 'gi');
    const parts = String(text || '').split(regex);
    return (
      <span>
        {parts.map((part, i) => 
          regex.test(part) ? (
            <span key={i} style={{ color: '#ff4d4f', fontWeight: 'bold' }}>{part}</span>
          ) : (
            part
          )
        )}
      </span>
    );
  };

  const getPreviewUrl = (record: Qualification) => {
    if (record.file_asset_id) {
        return `/api/files/download/${record.file_asset_id}`;
    }
    if (!record.stored_path) return '';
    return `/api/files/download/${record.id}`;
  };

  const isPdfAsset = (record: Qualification) => {
    const ext = (record.ext || '').toLowerCase();
    const path = (record.stored_path || '').toLowerCase();
    const name = (record.qualification_name || '').toLowerCase();
    return ext === '.pdf' || path.endsWith('.pdf') || name.includes('.pdf');
  };

  const columns = [
    {
      title: '资质类别',
      dataIndex: 'qualification_level',
      key: 'qualification_level',
      render: (text: string, record: Qualification) => {
        const displayText = (text === '有限责任公司（自然人投资或控股）' && record.qualification_name === '营业执照') 
          ? '营业执照' 
          : text;
        return (
          <Link to={`/library/qualifications/${record.id}`} className="text-blue-600 hover:underline">
            {highlightText(displayText) || '-'}
          </Link>
        );
      },
    },
    {
      title: '证书名称',
      dataIndex: 'qualification_name',
      key: 'qualification_name',
      render: (text: string) => highlightText(text),
    },

    {
      title: '有效期至',
      dataIndex: 'valid_to',
      key: 'valid_to',
      render: (date: string) => {
        const d = dayjs(date);
        const displayDate = (date && d.isValid() && date !== '0001-01-01') ? d.format('YYYY-M-D') : (date || '长期');
        return (
          <Space>
            <CalendarOutlined className="text-gray-400" />
            <Text>{displayDate}</Text>
            {date && d.isValid() && date !== '0001-01-01' && getStatusTag(date)}
          </Space>
        );
      },
      sorter: (a: Qualification, b: Qualification) => dayjs(a.valid_to).unix() - dayjs(b.valid_to).unix(),
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      fixed: 'right' as const,
      align: 'center' as const,
      render: (_: unknown, record: Qualification) => (
        <Space size="small">
          <Tooltip title="详情/修改">
            <Button 
               type="text" 
               size="small" 
               icon={<EditOutlined style={{ color: '#1890ff' }} />} 
               onClick={() => navigate(`/library/qualifications/${record.id}/edit`)} 
            />
          </Tooltip>
          
          <Tooltip title={(record.file_asset_id || record.stored_path || (record.certificates && record.certificates.length > 0)) ? "查看原件" : "暂无原件"}>
            <Button 
              type="text" 
              size="small" 
              icon={<EyeOutlined style={{ color: (record.file_asset_id || record.stored_path || (record.certificates && record.certificates.length > 0)) ? '#52c41a' : '#bfbfbf' }} />}
              disabled={!(record.file_asset_id || record.stored_path || (record.certificates && record.certificates.length > 0))}
              onClick={() => {
                const previewUrl = record.file_asset_id 
                  ? `/api/files/download/${record.file_asset_id}` 
                  : (record.stored_path ? getPreviewUrl(record) : '');
                
                if (!previewUrl) {
                  message.warning('暂无电子原件可预览');
                  return;
                }

                const isPdf = isPdfAsset(record);
                
                Modal.info({
                  title: `原件预览 - ${record.qualification_name}`,
                  width: 1000,
                  centered: true,
                  maskClosable: true,
                  icon: null,
                  content: (
                    <div style={{ textAlign: 'center', padding: '10px 0' }}>
                      <div style={{ marginBottom: 16, textAlign: 'right' }}>
                         <Space>
                            <Button size="small" onClick={() => window.open(previewUrl, '_blank')}>在新窗口打开</Button>
                            <Button size="small" type="primary" href={previewUrl} download>下载原件</Button>
                         </Space>
                      </div>
                      <div style={{ maxHeight: '75vh', overflowY: 'auto', border: '1px solid #f0f0f0', borderRadius: 8, padding: 8, background: '#fafafa' }}>
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
                            placeholder={<div style={{ padding: 50 }}><Spin tip="图片加载中..." /></div>}
                            fallback="https://via.placeholder.com/400?text=图片加载失败，请尝试下载查看"
                            style={{ maxWidth: '100%', borderRadius: 4, boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
                          />
                        )}
                      </div>
                    </div>
                  ),
                  okText: '关闭'
                });
              }}
            />
          </Tooltip>

          <Popconfirm
            title="狠心删除？"
            description={`确定删除资质证书“${record.qualification_name}”吗？此操作不可撤销。`}
            onConfirm={() => handleDelete(record.id, record.qualification_name)}
            okText="删除"
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
        <div className="flex items-center" style={{ justifyContent: 'flex-start', marginBottom: 24 }}>
          <Space size={16}>
            <Button icon={<PlusOutlined />} type="primary" onClick={() => setIsModalVisible(true)}>新增</Button>
            <Input
              placeholder="搜证书名、编号、类别..."
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
          dataSource={data.filter(item => {
            const matchSearch = 
              item.qualification_name?.toLowerCase().includes(searchText.toLowerCase()) || 
              item.certificate_no?.toLowerCase().includes(searchText.toLowerCase()) ||
              item.qualification_level?.toLowerCase().includes(searchText.toLowerCase());
            
            // 资质库仅展示企业资质；人员证书在人员库中维护
            return matchSearch && (!item.owner_type || item.owner_type === 'company');
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
                        <Text>已选 {selectedRowKeys.length}/{pageSize} 条</Text>
                        <Button 
                            size="small" 
                            disabled={selectedRowKeys.length === 0}
                            onClick={handleBatchDelete}
                        >
                            批量删除
                        </Button>
                    </Space>
                    <Text style={{ marginLeft: 16 }}>共 {total} 条资质</Text>
                </Space>
            )
          }}
          size="middle"
        />
      </Card>

      <Modal
        title="录入资质证书"
        open={isModalVisible}
        onCancel={() => setIsModalVisible(false)}
        footer={null}
        width={700}
      >
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="qualification_name" label="证书名称" rules={[{ required: true }]}>
                <Input placeholder="输入证书名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="certificate_no" label="证书编号">
                <Input placeholder="输入证书编号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="qualification_level" label="资质等级/类别">
                <Input placeholder="如：甲级、一级等" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="issuing_authority" label="发证机关">
                <Input placeholder="输入发证机关名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="valid_from" label="颁发/起始日期">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="valid_to" label="有效期至">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item className="text-right mt-4 mb-0">
            <Space>
              <Button onClick={() => setIsModalVisible(false)}>取消</Button>
              <Button type="primary" htmlType="submit">保存资质记录</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default QualificationLibrary;
