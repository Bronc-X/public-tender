import React, { useState, useEffect } from 'react';
import { Table, Button, Space, Input, Tag, Card, Row, Col, Typography, message, Modal, Form, DatePicker, Select, Image, Tooltip, Popconfirm } from 'antd';
import { PlusOutlined, SearchOutlined, TrophyOutlined, EditOutlined, EyeOutlined, DeleteOutlined } from '@ant-design/icons';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title, Text } = Typography;
const { Option } = Select;

interface Honor {
  id: string;
  honor_name: string;
  honor_level: string;
  owner_org?: string;
  owner_person_name?: string;
  award_date?: string;
  issue_authority?: string;
  stored_path?: string;
}

const HonorLibrary: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [honors, setHonors] = useState<Honor[]>([]);
  const [searchText, setSearchText] = useState('');
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [form] = Form.useForm();
  const { currentCompanyId } = useCompany();

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    try {
      const response = await axios.get('/api/honors', {
        headers: { 'x-company-id': currentCompanyId }
      });
      setHonors(response.data);
    } catch (err) {
      console.error('Failed to fetch honors:', err);
    } finally {
      setLoading(false);
    }
  }, [currentCompanyId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const onFinish = async (values: { honor_name: string, honor_level: string, award_date: dayjs.Dayjs | null, owner_org?: string, issue_authority?: string }) => {
    try {
      const payload = {
        ...values,
        award_date: values.award_date?.format('YYYY-MM-DD'),
      };
      await axios.post('/api/honors', payload, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('荣誉记录录入成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error('录入失败: ' + msg);
    }
  };

  const columns = [
    {
      title: '荣誉名称',
      dataIndex: 'honor_name',
      key: 'honor_name',
      render: (text: string, record: Honor) => (
        <Link to={`/library/honors/${record.id}`} className="text-blue-600 hover:underline">
          {text}
        </Link>
      ),
    },
    {
      title: '荣誉级别',
      dataIndex: 'honor_level',
      key: 'honor_level',
      render: (level: string) => {
        const colors: Record<string, string> = { 'national': 'red', 'provincial': 'orange', 'municipal': 'blue', 'district': 'cyan' };
        const labels: Record<string, string> = { 'national': '国家级', 'provincial': '省级', 'municipal': '市级', 'district': '区级' };
        return <Tag color={colors[level] || 'default'}>{labels[level] || level}</Tag>;
      },
    },
    {
      title: '获奖主体 (单位/个人)',
      key: 'owner',
      render: (_: unknown, record: Honor) => (
        <span>{record.owner_org || record.owner_person_name || '未指定'}</span>
      ),
    },
    {
      title: '颁发日期',
      dataIndex: 'award_date',
      key: 'award_date',
      sorter: (a: Honor, b: Honor) => (a.award_date || '').localeCompare(b.award_date || ''),
    },
    {
      title: '发证机关',
      dataIndex: 'issue_authority',
      key: 'issue_authority',
      ellipsis: true,
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      fixed: 'right' as const,
      align: 'center' as const,
      render: (_: unknown, record: Honor) => (
        <Space size="small">
          <Tooltip title="详情/修改">
            <Button 
               type="text" 
               size="small" 
               icon={<EditOutlined style={{ color: '#1890ff' }} />} 
               onClick={() => navigate(`/library/honors/${record.id}/edit`)} 
            />
          </Tooltip>
          
          <Tooltip title={record.stored_path ? "查看原件" : "暂无原件"}>
            <Button 
              type="text" 
              size="small" 
              icon={<EyeOutlined style={{ color: record.stored_path ? '#52c41a' : '#bfbfbf' }} />}
              disabled={!record.stored_path}
              onClick={() => {
                const previewUrl = `/files/${record.stored_path!.split('data/files/')[1]}`;
                Modal.info({
                  title: `原件预览 - ${record.honor_name}`,
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
                  okText: '关闭'
                });
              }}
            />
          </Tooltip>

          <Popconfirm
            title="确认删除？"
            description={`确定删除该荣誉记录吗？`}
            onConfirm={async () => {
              try {
                await axios.delete(`/api/honors/${record.id}`, { headers: { 'x-company-id': currentCompanyId } });
                message.success('已删除');
                fetchData();
              } catch {
                message.error('删除失败');
              }
            }}
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
              placeholder="搜索荣誉名称、发证机关..."
              prefix={<SearchOutlined />}
              onChange={e => setSearchText(e.target.value)}
              style={{ width: 400 }}
            />
            <Select defaultValue="all" style={{ width: 150 }}>
              <Option value="all">全部荣誉评级</Option>
              <Option value="national">国家级</Option>
              <Option value="provincial">省级</Option>
            </Select>
          </Space>
        </div>

      <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table
          columns={columns}
          dataSource={honors.filter(h => 
            h.honor_name?.includes(searchText) || 
            h.issue_authority?.includes(searchText)
          )}
          loading={loading}
          rowKey="id"
          pagination={{ 
            pageSizeOptions: ['10', '20', '50', '100'], 
            showSizeChanger: true, 
            defaultPageSize: 10,
            showTotal: (total) => `共 ${total} 条荣誉`
          }}
          size="middle"
        />
      </Card>

      <Modal
        title="新增荣誉项"
        open={isModalVisible}
        onCancel={() => setIsModalVisible(false)}
        footer={null}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item name="honor_name" label="荣誉名称" rules={[{ required: true }]}>
            <Input placeholder="请填写奖项或荣誉全称" />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="honor_level" label="荣誉级别" rules={[{ required: true }]}>
                <Select placeholder="选择级别">
                  <Option value="national">国家级</Option>
                  <Option value="provincial">省级</Option>
                  <Option value="municipal">市级</Option>
                  <Option value="district">区级</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="award_date" label="获奖日期">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="owner_org" label="获奖单位 (所有者)" initialValue="本单位">
                <Input placeholder="输入获奖单位或所有者" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="issue_authority" label="颁发部门/机关">
                <Input placeholder="如：中华人民共和国住房和城乡建设部" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item className="text-right mt-4 mb-0">
            <Space>
              <Button onClick={() => setIsModalVisible(false)}>取消</Button>
              <Button type="primary" htmlType="submit">保存并存档</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default HonorLibrary;
