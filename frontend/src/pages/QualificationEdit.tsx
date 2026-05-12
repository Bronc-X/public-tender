import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, DatePicker, Row, Col, Space, Typography, message, Spin, Result, Select } from 'antd';
import { LeftOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title } = Typography;
const { Option } = Select;

type FormValues = {
  qualification_name: string;
  qualification_type?: string;
  owner_type?: string;
  certificate_no?: string;
  qualification_level?: string;
  issuing_authority?: string;
  valid_to?: dayjs.Dayjs;
  bid_usable_status?: string;
};

const QualificationEdit: React.FC = () => {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm<FormValues>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { currentCompanyId } = useCompany();

  useEffect(() => {
    const load = async () => {
      if (!id) return;
      setLoading(true);
      setError(null);
      try {
        const resp = await axios.get(`/api/qualifications/${id}`, {
          headers: { 'x-company-id': currentCompanyId }
        });
        const p = resp.data || {};
        form.setFieldsValue({
          qualification_name: p.qualification_name,
          qualification_type: p.qualification_type,
          owner_type: p.owner_type || 'company',
          certificate_no: p.certificate_no,
          qualification_level: p.qualification_level,
          issuing_authority: p.issuing_authority,
          valid_to: p.valid_to ? dayjs(p.valid_to) : undefined,
          bid_usable_status: p.bid_usable_status || 'usable',
        });
      } catch (err: unknown) {
        console.error('Failed to fetch qualification detail:', err);
        setError('无法获取资质详情');
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [id, form, currentCompanyId]);

  const onFinish = async (values: FormValues) => {
    if (!id) return;
    try {
      const payload = {
        ...values,
        valid_to: values.valid_to ? values.valid_to.format('YYYY-MM-DD') : null,
      };
      await axios.patch(`/api/qualifications/${id}`, payload, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('资质信息已更新');
      navigate('/library/qualifications');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error('更新失败：' + msg);
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入数据..." />
      </div>
    );
  }

  if (error || !id) {
    return (
      <Result
        status="error"
        title="无法编辑"
        subTitle={error || '缺少资质 ID'}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/qualifications')}>
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
            <SafetyCertificateOutlined style={{ marginRight: 8, color: '#1890ff' }} /> 编辑企业资质
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
          <Row gutter={24}>
            <Col span={24}>
              <Form.Item
                name="qualification_name"
                label="资质/证书名称"
                rules={[{ required: true, message: '请输入名称' }]}
              >
                <Input placeholder="例如：市政公用工程施工总承包一级资质" size="large" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="certificate_no" label="证书编号">
                <Input placeholder="请输入证书编号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="qualification_level" label="资质等级/专业分类">
                <Input placeholder="一级/二级/不分等级" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="issuing_authority" label="发证机关/发证单位">
                <Input placeholder="发证机关全称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="valid_to" label="有效期至">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="bid_usable_status" label="投标可用性">
                <Select>
                  <Option value="usable">正常 (库内可用)</Option>
                  <Option value="restricted">受限 (不推荐使用)</Option>
                  <Option value="expired">过期</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item className="mt-10 border-t pt-6 text-center">
            <Space size="large">
              <Button size="large" onClick={() => navigate('/library/qualifications')}>取消返回</Button>
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

export default QualificationEdit;
