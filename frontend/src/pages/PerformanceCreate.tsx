import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, DatePicker, Row, Col, Space, Typography, InputNumber, Select, message } from 'antd';
import { LeftOutlined, SaveOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import type { Dayjs } from 'dayjs';

const { Title } = Typography;
const { Option } = Select;

interface Person {
  id: string;
  name: string;
  role_type?: string;
  specialty?: string;
}

interface CreatePerformanceValues {
  project_name: string;
  pm_id?: string;
  tech_leader_id?: string;
  safety_leader_id?: string;
  winning_date?: Dayjs; 
  completion_date?: Dayjs; 
  bid_amount_value?: number;
  amount_value?: number;
  owner_org?: string;
  project_location?: string;
  scale_desc?: string;
  construction_period?: string;
  documentation_officer_id?: string;
  materials_officer_id?: string;
  quality_inspector_id?: string;
  construction_officer_id?: string;
  standards_officer_id?: string;
  mechanical_officer_id?: string;
  labor_officer_id?: string;
}

const PerformanceCreate: React.FC = () => {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [persons, setPersons] = useState<Person[]>([]);
  const [loadingPersons, setLoadingPersons] = useState(false);

  useEffect(() => {
    const fetchPersons = async () => {
      setLoadingPersons(true);
      try {
        const response = await axios.get('/api/persons');
        setPersons(response.data);
      } catch (err) {
        console.error('Failed to fetch persons:', err);
      } finally {
        setLoadingPersons(false);
      }
    };
    fetchPersons();
  }, []);

  const onFinish = async (values: CreatePerformanceValues) => {
    try {
      const pm = persons.find(p => p.id === values.pm_id);
      const tech = persons.find(p => p.id === values.tech_leader_id);
      const safety = persons.find(p => p.id === values.safety_leader_id);
      const docs = persons.find(p => p.id === values.documentation_officer_id);
      const materials = persons.find(p => p.id === values.materials_officer_id);
      const quality = persons.find(p => p.id === values.quality_inspector_id);
      const construction = persons.find(p => p.id === values.construction_officer_id);
      const standards = persons.find(p => p.id === values.standards_officer_id);
      const mechanical = persons.find(p => p.id === values.mechanical_officer_id);
      const labor = persons.find(p => p.id === values.labor_officer_id);

      const payload = {
        ...values,
        project_manager_name: pm?.name || '',
        technical_leader_name: tech?.name || '',
        safety_leader_name: safety?.name || '',
        documentation_officer_name: docs?.name || '',
        materials_officer_name: materials?.name || '',
        quality_inspector_name: quality?.name || '',
        construction_officer_name: construction?.name || '',
        standards_officer_name: standards?.name || '',
        mechanical_officer_name: mechanical?.name || '',
        labor_officer_name: labor?.name || '',
        completion_date: values.completion_date ? values.completion_date.format('YYYY-MM-DD') : null,
        winning_date: values.winning_date ? values.winning_date.format('YYYY-MM-DD') : null,
      };
      await axios.post('/api/performances', payload);
      message.success('业绩档案创建成功');
      navigate('/library/performances');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error('创建失败：' + msg);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center w-full">
        <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate(-1)} />
          <Title level={3} style={{ margin: 0 }}>
            新增项目业绩
          </Title>
        </Space>
      </div>

      <Card className="shadow-sm border-none bg-white p-4" style={{ marginTop: 12 }}>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{ bid_amount_value: 0, amount_value: 0 }}
          className="max-w-4xl mx-auto"
        >
          <div className="border-b mb-6 pb-2">
            <Title level={5}>1. 基本信息</Title>
          </div>
          <Row gutter={24}>
            <Col span={24}>
              <Form.Item 
                name="project_name" 
                label="项目全称" 
                rules={[{ required: true, message: '请输入完整的项目名称' }]}
              >
                <Input placeholder="请以此处为主项填写完整项目名称，用于后续标书生成" size="large" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="owner_org" label="业主单位/建设单位">
                <Input placeholder="建设单位全称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="project_location" label="项目地点">
                <Input placeholder="省/市/区" />
              </Form.Item>
            </Col>
          </Row>

          <div className="border-b mb-6 mt-8 pb-2">
            <Title level={5}>2. 管理团队</Title>
          </div>
          <Row gutter={24}>
            <Col span={8}>
              <Form.Item name="pm_id" label="项目经理">
                <Select
                  showSearch
                  placeholder="选择项目经理"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="tech_leader_id" label="技术负责人">
                <Select
                  showSearch
                  placeholder="选择技术负责人"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="safety_leader_id" label="安全负责人">
                <Select
                  showSearch
                  placeholder="选择安全负责人"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="construction_officer_id" label="施工员">
                <Select
                  showSearch
                  placeholder="选择施工员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="quality_inspector_id" label="质量员">
                <Select
                  showSearch
                  placeholder="选择质量员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="documentation_officer_id" label="资料员">
                <Select
                  showSearch
                  placeholder="选择资料员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="materials_officer_id" label="材料员">
                <Select
                  showSearch
                  placeholder="选择材料员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="standards_officer_id" label="标准员">
                <Select
                  showSearch
                  placeholder="选择标准员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="mechanical_officer_id" label="机械员">
                <Select
                  showSearch
                  placeholder="选择机械员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="labor_officer_id" label="劳务员">
                <Select
                  showSearch
                  placeholder="选择劳务员"
                  loading={loadingPersons}
                  optionFilterProp="label"
                >
                  {persons.map(p => (
                    <Option key={p.id} value={p.id} label={`${p.name} (${p.role_type || '无角色'})`}>
                      {p.name} <span style={{ color: '#999', fontSize: '12px' }}>{p.specialty}</span>
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <div className="border-b mb-6 mt-8 pb-2">
            <Title level={5}>3. 关键节点与金额</Title>
          </div>
          <Row gutter={24}>
            <Col span={12}>
              <Form.Item name="winning_date" label="中标录入时间">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="completion_date" label="完工验收日期">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="bid_amount_value" label="中标金额 (万元)">
                <InputNumber style={{ width: '100%' }} precision={2} min={0} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="amount_value" label="合同金额 (万元)">
                <InputNumber style={{ width: '100%' }} precision={2} min={0} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="construction_period" label="建设工期 (汉字/文本)">
                <Input placeholder="例如：300日历天 / 12个月" />
              </Form.Item>
            </Col>
          </Row>

          <div className="border-b mb-6 mt-8 pb-2">
            <Title level={5}>4. 规模及附属描述</Title>
          </div>
          <Col span={24}>
            <Form.Item name="scale_desc" label="工程规模/详细内容描述">
              <Input.TextArea rows={4} placeholder="如：建筑面积、跨度、层数、特殊工艺等关键参数描述" />
            </Form.Item>
          </Col>

          <Form.Item className="mt-10 border-t pt-6 text-center">
            <Space size="large">
              <Button size="large" onClick={() => navigate('/library/performances')}>取消返回</Button>
              <Button type="primary" htmlType="submit" size="large" icon={<SaveOutlined />}>
                确认创建并入库
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default PerformanceCreate;

