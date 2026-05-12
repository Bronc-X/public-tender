import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, DatePicker, Row, Col, Space, Typography, InputNumber, Select, message, Spin, Result } from 'antd';
import { LeftOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

const { Title } = Typography;
const { Option } = Select;

interface Person {
  id: string;
  name: string;
  role_type?: string;
  specialty?: string;
}

type FormValues = {
  project_name: string;
  project_location?: string;
  owner_org?: string;
  pm_id?: string;
  tech_leader_id?: string;
  safety_leader_id?: string;
  winning_date?: Dayjs;
  completion_date?: Dayjs;
  bid_amount_value?: number;
  amount_value?: number;
  scale_desc?: string;
  construction_period?: string;
  documentation_officer_id?: string;
  materials_officer_id?: string;
  quality_inspector_id?: string;
  construction_officer_id?: string;
  standards_officer_id?: string;
  mechanical_officer_id?: string;
  labor_officer_id?: string;
};

const PerformanceEdit: React.FC = () => {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm<FormValues>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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

  useEffect(() => {
    const load = async () => {
      if (!id) return;
      setLoading(true);
      setError(null);
      try {
        const resp = await axios.get(`/api/performances/${id}`);
        const p = resp.data || {};
        form.setFieldsValue({
          project_name: p.project_name,
          owner_org: p.owner_org,
          project_location: p.project_location,
          pm_id: p.pm_id || p.project_manager_name || undefined,
          tech_leader_id: p.tech_leader_id || p.technical_leader_name || undefined,
          safety_leader_id: p.safety_leader_id || p.safety_leader_name || undefined,
          winning_date: p.winning_date ? dayjs(p.winning_date) : undefined,
          completion_date: p.completion_date ? dayjs(p.completion_date) : undefined,
          bid_amount_value: typeof p.bid_amount_value === 'number' ? p.bid_amount_value : Number(p.bid_amount_value || 0),
          amount_value: typeof p.amount_value === 'number' ? p.amount_value : Number(p.amount_value || 0),
          scale_desc: p.scale_desc,
          construction_period: p.construction_period,
          documentation_officer_id: p.documentation_officer_id || p.documentation_officer_name || undefined,
          materials_officer_id: p.materials_officer_id || p.materials_officer_name || undefined,
          quality_inspector_id: p.quality_inspector_id || p.quality_inspector_name || undefined,
          construction_officer_id: p.construction_officer_id || p.construction_officer_name || undefined,
          standards_officer_id: p.standards_officer_id || p.standards_officer_name || undefined,
          mechanical_officer_id: p.mechanical_officer_id || p.mechanical_officer_name || undefined,
          labor_officer_id: p.labor_officer_id || p.labor_officer_name || undefined,
        });
      } catch (err: unknown) {
        console.error('Failed to fetch performance detail:', err);
        const apiErr = err as { response?: { data?: { error?: string } } };
        setError(apiErr.response?.data?.error || '无法获取业绩详情');
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [id, form]);

  const onFinish = async (values: FormValues) => {
    if (!id) return;
    try {
      const resolvePerson = (selectedVal: string | undefined) => {
        if (!selectedVal) return { name: '', id: null };
        const p = persons.find(per => per.id === selectedVal);
        if (p) return { name: p.name, id: p.id };
        return { name: selectedVal, id: null };
      };

      const pm = resolvePerson(values.pm_id);
      const tech = resolvePerson(values.tech_leader_id);
      const safety = resolvePerson(values.safety_leader_id);
      const docs = resolvePerson(values.documentation_officer_id);
      const materials = resolvePerson(values.materials_officer_id);
      const quality = resolvePerson(values.quality_inspector_id);
      const construction = resolvePerson(values.construction_officer_id);
      const standards = resolvePerson(values.standards_officer_id);
      const mechanical = resolvePerson(values.mechanical_officer_id);
      const labor = resolvePerson(values.labor_officer_id);

      const payload = {
        ...values,
        pm_id: pm.id,
        project_manager_name: pm.name,
        tech_leader_id: tech.id,
        technical_leader_name: tech.name,
        safety_leader_id: safety.id,
        safety_leader_name: safety.name,
        documentation_officer_id: docs.id,
        documentation_officer_name: docs.name,
        materials_officer_id: materials.id,
        materials_officer_name: materials.name,
        quality_inspector_id: quality.id,
        quality_inspector_name: quality.name,
        construction_officer_id: construction.id,
        construction_officer_name: construction.name,
        standards_officer_id: standards.id,
        standards_officer_name: standards.name,
        mechanical_officer_id: mechanical.id,
        mechanical_officer_name: mechanical.name,
        labor_officer_id: labor.id,
        labor_officer_name: labor.name,
        completion_date: values.completion_date ? values.completion_date.format('YYYY-MM-DD') : null,
        winning_date: values.winning_date ? values.winning_date.format('YYYY-MM-DD') : null,
      };
      await axios.patch(`/api/performances/${id}`, payload);
      message.success('业绩已更新');
      navigate(`/library/performances/${id}`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error('更新失败：' + msg);
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入业绩数据..." />
      </div>
    );
  }

  if (error || !id) {
    return (
      <Result
        status="error"
        title="无法编辑"
        subTitle={error || '缺少业绩 ID'}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/performances')}>
            返回列表
          </Button>,
        ]}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center w-full">
        <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate(-1)} />
          <Title level={3} style={{ margin: 0 }}>
            编辑业绩
          </Title>
        </Space>
      </div>

      <Card className="shadow-sm border-none bg-white p-4" style={{ marginTop: 12 }}>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
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
                <Input placeholder="项目名称" size="large" />
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
              <Input.TextArea rows={4} placeholder="规模描述" />
            </Form.Item>
          </Col>

          <Form.Item className="mt-10 border-t pt-6 text-center">
            <Space size="large">
              <Button size="large" onClick={() => navigate(`/library/performances/${id}`)}>取消返回</Button>
              <Button type="primary" htmlType="submit" size="large">
                保存修改
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default PerformanceEdit;


