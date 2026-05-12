import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, DatePicker, Row, Col, Space, Typography, message, Spin, Result, Select } from 'antd';
import { LeftOutlined, TrophyOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import axios from 'axios';
import dayjs from 'dayjs';
import { useCompany } from '../context/CompanyContext';

const { Title } = Typography;
const { Option } = Select;

type FormValues = {
  honor_name: string;
  honor_level?: string;
  owner_org?: string;
  owner_person_name?: string;
  award_date?: dayjs.Dayjs;
  issue_authority?: string;
};

const HonorEdit: React.FC = () => {
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
        const resp = await axios.get(`/api/honors/${id}`, {
          headers: { 'x-company-id': currentCompanyId }
        });
        const p = resp.data || {};
        form.setFieldsValue({
          honor_name: p.honor_name,
          honor_level: p.honor_level,
          owner_org: p.owner_org,
          owner_person_name: p.owner_person_name,
          award_date: p.award_date ? dayjs(p.award_date) : undefined,
          issue_authority: p.issue_authority,
        });
      } catch (err: unknown) {
        console.error('Failed to fetch honor detail:', err);
        setError('无法获取荣誉详情');
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
        award_date: values.award_date ? values.award_date.format('YYYY-MM-DD') : null,
      };
      await axios.patch(`/api/honors/${id}`, payload, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('荣誉信息已更新');
      navigate('/library/honors');
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
        subTitle={error || '缺少荣誉 ID'}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/honors')}>
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
             编辑荣誉/奖项
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
                name="honor_name"
                label="荣誉/奖项名称"
                rules={[{ required: true, message: '请输入名称' }]}
              >
                <Input placeholder="例如：2023年度安全文明施工示范工地" size="large" prefix={<TrophyOutlined style={{ color: '#faad14' }} />} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="honor_level" label="级别">
                <Select placeholder="选择级别">
                  <Option value="national">国家级</Option>
                  <Option value="provincial">省级</Option>
                  <Option value="municipal">市级</Option>
                  <Option value="district">区县级</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="award_date" label="获奖/颁发日期">
                <DatePicker style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="issue_authority" label="颁发单位/机构">
                <Input placeholder="输入颁发单位名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
               <Form.Item name="owner_person_name" label="关联个人 (可选)">
                <Input placeholder="如果是个体奖项，请输入个人姓名" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item className="mt-10 border-t pt-6 text-center">
            <Space size="large">
              <Button size="large" onClick={() => navigate('/library/honors')}>取消返回</Button>
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

export default HonorEdit;
