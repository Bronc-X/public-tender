import React, { useEffect, useState } from 'react';
import { Row, Col, Card, Statistic, Table, Tag, List, Typography, Space, Button, Empty, Skeleton, Descriptions, Modal, Form, Input, message, Upload, Divider } from 'antd';
import {
  ProjectOutlined,
  UserOutlined,
  SolutionOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  ArrowRightOutlined,
  BankOutlined,
  EditOutlined,
  UploadOutlined
} from '@ant-design/icons';
import axios from 'axios';
import { useNavigate } from 'react-router-dom';
import { useCompany } from '../context/CompanyContext';

interface Project {
  id: string;
  project_name: string;
  completion_date: string;
  amount_value: number;
  risk_status: string;
}


interface DashboardState {
  projectCount: number;
  personCount: number;
  qualificationCount: number;
  honorCount: number;
  issueCount: number;
  auditCount: number;
  recentProjects: Project[];
}

const { Title, Text } = Typography;

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const { currentCompanyId, companies, refreshCompanies } = useCompany();
  const [loading, setLoading] = useState(true);
  const [issuesCount, setIssuesCount] = useState<number | null>(null);
  
  // Edit Modal States
  const [isEditingCompany, setIsEditingCompany] = useState(false);
  const [form] = Form.useForm();
  const [savingSettings, setSavingSettings] = useState(false);

  // Attachment Viewer States
  const [isViewingAttachments, setIsViewingAttachments] = useState(false);
  const [loadingAttachments, setLoadingAttachments] = useState(false);
  const [companyAttachments, setCompanyAttachments] = useState<any[]>([]);

  const companyInfo = companies.find(c => c.id === currentCompanyId) || null;
  const [data, setData] = useState<DashboardState>({
    projectCount: 0,
    personCount: 0,
    qualificationCount: 0,
    honorCount: 0,
    issueCount: 0,
    auditCount: 0,
    recentProjects: []
  });

  useEffect(() => {
    let isMounted = true;
    const load = async () => {
      try {
        const summaryRes = await axios.get('/api/dashboard/summary');
        if (isMounted) {
          setData(summaryRes.data);
          setLoading(false);
        }
      } catch (err) {
        console.error('Failed to fetch dashboard data:', err);
        if (isMounted) {
          setLoading(false);
        }
      }
    };
    load();
    return () => { isMounted = false; };
  }, []);

  useEffect(() => {
    let isMounted = true;
    const loadIssuesCount = async () => {
      try {
        const res = await axios.get('/api/issues');
        const arr = Array.isArray(res.data) ? res.data : [];
        if (isMounted) setIssuesCount(arr.length);
      } catch (err) {
        console.error('Failed to fetch issues count:', err);
        if (isMounted) setIssuesCount(null);
      }
    };
    loadIssuesCount();
    return () => { isMounted = false; };
  }, []);

  const handleEditCompany = () => {
    if (companyInfo) {
      form.setFieldsValue({
        company_name: companyInfo.company_name,
        unified_social_credit_code: companyInfo.unified_social_credit_code,
        legal_person: companyInfo.legal_person,
        legal_person_id_card: companyInfo.legal_person_id_card,
        address: companyInfo.address,
      });
      setIsEditingCompany(true);
    }
  };

  const handleSaveCompany = async () => {
    try {
      const values = await form.validateFields();
      setSavingSettings(true);
      await axios.patch(`/api/companies/${companyInfo?.id}`, values);
      message.success('公司信息已更新');
      setIsEditingCompany(false);
      refreshCompanies();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) {
        // Validation failed, do nothing
      } else {
        const errorMsg = axios.isAxiosError(err) ? err.response?.data?.error || err.message : '更新失败';
        message.error(`保存失败：${errorMsg}`);
      }
    } finally {
      setSavingSettings(false);
    }
  };

  const handleViewAttachments = async () => {
    setIsViewingAttachments(true);
    setLoadingAttachments(true);
    try {
      const res = await axios.get('/api/files');
      const allFiles = Array.isArray(res.data) ? res.data : [];
      // 严格过滤：只展示明确打上了 company_profile 标签的专属核心归档文件
      const companyFiles = allFiles.filter(f => f.source_module === 'company_profile');
      setCompanyAttachments(companyFiles);
    } catch (err) {
      console.error(err);
      message.error('无法加载档案附件');
    } finally {
      setLoadingAttachments(false);
    }
  };

  const projectColumns = [
    {
      title: '项目名称',
      dataIndex: 'project_name',
      key: 'project_name',
      render: (text: string, record: Project) => (
        <Typography.Link onClick={() => navigate(`/library/performances/${record.id}`)}>
          {text}
        </Typography.Link>
      ),
    },
    {
      title: '中标日期',
      dataIndex: 'winning_date',
      key: 'winning_date',
      width: 120,
      render: (val: string) => <Text style={{ whiteSpace: 'nowrap' }}>{val || '-'}</Text>,
    },
    {
      title: '金额 (万元)',
      dataIndex: 'amount_value',
      key: 'amount_value',
      width: 120,
      align: 'right' as const,
      render: (val: number) => <Text style={{ whiteSpace: 'nowrap' }}>{(val || 0).toLocaleString()}</Text>,
    },
  ];

  if (loading) {
    return <Skeleton active paragraph={{ rows: 20 }} />;
  }

  return (
    <div className="space-y-6">
      {companyInfo && (
        <div style={{
          marginBottom: 32,
          padding: 32,
          borderRadius: 16,
          background: 'linear-gradient(135deg, #f8fafc 0%, #eff6ff 100%)',
          boxShadow: '0 4px 20px -2px rgba(59, 130, 246, 0.05)',
          position: 'relative',
          overflow: 'hidden'
        }}>
          {/* Decorative Background Blur Elements */}
          <div style={{
            position: 'absolute',
            top: -80,
            right: -50,
            width: 300,
            height: 300,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(99,102,241,0.08) 0%, rgba(99,102,241,0) 70%)',
            pointerEvents: 'none'
          }} />
          <div style={{
            position: 'absolute',
            bottom: -60,
            left: 200,
            width: 200,
            height: 200,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(56,189,248,0.08) 0%, rgba(56,189,248,0) 70%)',
            pointerEvents: 'none'
          }} />

          <Space align="center" size={20} style={{ marginBottom: 32, position: 'relative', zIndex: 1 }}>
            <div style={{
              width: 56,
              height: 56,
              borderRadius: 14,
              background: 'linear-gradient(135deg, #4f46e5 0%, #3b82f6 100%)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 8px 16px rgba(79, 70, 229, 0.25)'
            }}>
              <BankOutlined style={{ fontSize: 26, color: '#fff' }} />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', fontSize: 26, fontWeight: 700, color: '#0f172a', letterSpacing: '-0.5px' }}>
                {companyInfo.company_name || '未设置公司名称'}
                <Button 
                  type="text" 
                  icon={<EditOutlined />} 
                  onClick={handleEditCompany}
                  style={{ marginLeft: 6, color: '#64748b' }}
                  title="编辑公司信息"
                />
              </div>
            </div>
          </Space>

          <Row gutter={[16, 16]} style={{ position: 'relative', zIndex: 1 }}>
            {[
              { label: '统一社会信用代码', value: companyInfo.unified_social_credit_code },
              { label: '法定代表人', value: companyInfo.legal_person },
              { label: '法人身份证号', value: companyInfo.legal_person_id_card },
              { label: '注册地址', value: companyInfo.address }
            ].map((item, idx) => (
              <Col xs={24} sm={12} md={5} key={idx}>
                <div style={{
                  padding: '16px 20px',
                  background: 'rgba(255, 255, 255, 0.7)',
                  borderRadius: 12,
                  backdropFilter: 'blur(12px)',
                  border: '1px solid rgba(255, 255, 255, 0.9)',
                  boxShadow: '0 2px 8px rgba(0,0,0,0.02)',
                  transition: 'all 0.3s ease',
                  cursor: 'default',
                  height: '100%'
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.transform = 'translateY(-2px)';
                  e.currentTarget.style.boxShadow = '0 6px 16px rgba(0,0,0,0.04)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.transform = 'translateY(0)';
                  e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.02)';
                }}>
                  <div style={{ fontSize: 13, color: '#64748b', marginBottom: 6 }}>{item.label}</div>
                  <div style={{ fontSize: 13, color: '#1e293b' }}>{item.value || '-'}</div>
                </div>
              </Col>
            ))}
            
            <Col xs={24} sm={12} md={4}>
              <div style={{
                padding: '16px 20px',
                background: 'rgba(255, 255, 255, 0.7)',
                borderRadius: 12,
                backdropFilter: 'blur(12px)',
                border: '1px solid rgba(255, 255, 255, 0.9)',
                boxShadow: '0 2px 8px rgba(0,0,0,0.02)',
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'center',
                height: '100%',
                transition: 'all 0.3s ease'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-2px)';
                e.currentTarget.style.boxShadow = '0 6px 16px rgba(0,0,0,0.04)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)';
                e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.02)';
              }}>
                <div style={{ fontSize: 13, color: '#64748b', marginBottom: 6 }}>企业附件档案</div>
                <Button 
                  type="link" 
                  size="small" 
                  onClick={handleViewAttachments}
                  style={{ padding: 0, height: 'auto', textAlign: 'left', fontWeight: 600, fontSize: 14 }}
                >
                  查看公司附件 &rarr;
                </Button>
              </div>
            </Col>
          </Row>

          <Modal
            title={<span><BankOutlined style={{ marginRight: 8, color: '#4f46e5' }} /> 编辑企业核心档案</span>}
            open={isEditingCompany}
            onCancel={() => setIsEditingCompany(false)}
            onOk={handleSaveCompany}
            confirmLoading={savingSettings}
            destroyOnClose
            width={640}
            okText="保存档案"
            cancelText="取消"
          >
            <Form
              form={form}
              layout="vertical"
              style={{ marginTop: 24 }}
            >
              <Form.Item label="公司名称" name="company_name" rules={[{ required: true, message: '必须输入主体名称' }]}>
                <Input placeholder="例如：四川宏远建筑工程有限公司" size="large" />
              </Form.Item>
              
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item label="统一社会信用代码" name="unified_social_credit_code">
                    <Input placeholder="输入 18 位统一社会信用码" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item label="法定代表人" name="legal_person">
                    <Input placeholder="如：张三" />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item label="法人身份证号" name="legal_person_id_card">
                    <Input placeholder="输入 18 位身份证号码" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item label="注册地址" name="address">
                    <Input placeholder="填写营业执照上的注册地..." />
                  </Form.Item>
                </Col>
              </Row>

              <Divider dashed style={{ margin: '16px 0' }} />
              
              <Form.Item label={<span>企业电子附件归档 <Text type="secondary" style={{ fontSize: 12, fontWeight: 'normal' }}>(支持拖拽营业执照、资质原件扫描件等)</Text></span>}>
                <Upload 
                  action="/api/upload" 
                  data={{ source_module: 'company_profile', source_project_id: currentCompanyId }}
                  listType="picture-card"
                  maxCount={10}
                  multiple
                >
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
                    <UploadOutlined style={{ fontSize: '20px', color: '#94a3b8' }} />
                    <div style={{ marginTop: 8, color: '#64748b', fontSize: 12 }}>点击上传档案</div>
                  </div>
                </Upload>
              </Form.Item>
            </Form>
          </Modal>

          <Modal
            title={<span><BankOutlined style={{ marginRight: 8, color: '#4f46e5' }} /> 电子附件与档案柜</span>}
            open={isViewingAttachments}
            onCancel={() => setIsViewingAttachments(false)}
            footer={[
              <Button key="close" onClick={() => setIsViewingAttachments(false)}>关闭</Button>,
              <Button key="manage" type="primary" onClick={() => {
                setIsViewingAttachments(false);
                navigate('/file-center/repository');
              }}>前往文件库管理</Button>
            ]}
            width={600}
            destroyOnClose
          >
            {loadingAttachments ? (
              <Skeleton active paragraph={{ rows: 4 }} />
            ) : companyAttachments.length > 0 ? (
              <List
                itemLayout="horizontal"
                dataSource={companyAttachments}
                renderItem={(file: any) => (
                  <List.Item
                    actions={[
                      <Button type="link" size="small" href={`/api/file-binary/${file.id}`} target="_blank">预览/下载</Button>
                    ]}
                  >
                    <List.Item.Meta
                      avatar={
                        <div style={{ width: 40, height: 40, background: '#f1f5f9', borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                          {file.ext === '.pdf' ? (
                            <span style={{ color: '#ef4444', fontWeight: 'bold', fontSize: 12 }}>PDF</span>
                          ) : (
                            <UploadOutlined style={{ color: '#94a3b8' }} />
                          )}
                        </div>
                      }
                      title={<span style={{ fontWeight: 500 }}>{file.file_name || '未命名附件'}</span>}
                      description={`上传时间: ${new Date(file.created_at).toLocaleString()}`}
                    />
                  </List.Item>
                )}
              />
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <span>当前公司暂无核心附件。<br/><Text type="secondary">请点击上方“编辑”按钮上传营业执照或扫描件。</Text></span>
                }
              />
            )}
          </Modal>

        </div>
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card 
            className="stat-card" 
            hoverable 
            onClick={() => navigate('/library/performances')}
            style={{ cursor: 'pointer' }}
          >
            <Statistic
              title="业绩总量"
              value={data.projectCount}
              prefix={<ProjectOutlined className="text-blue-500 mr-2" />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card 
            className="stat-card" 
            hoverable 
            onClick={() => navigate('/library/persons')}
            style={{ cursor: 'pointer' }}
          >
            <Statistic
              title="人员总量"
              value={data.personCount}
              prefix={<UserOutlined className="text-green-500 mr-2" />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card 
            className="stat-card" 
            hoverable 
            onClick={() => navigate('/library/qualifications')}
            style={{ cursor: 'pointer' }}
          >
            <Statistic
              title="资质证书"
              value={data.qualificationCount}
              prefix={<SolutionOutlined className="text-purple-500 mr-2" />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card 
            className="stat-card" 
            hoverable 
            onClick={() => navigate('/issues')}
            style={{ cursor: 'pointer' }}
          >
            <Statistic
              title="待审核/异常"
              value={issuesCount ?? data.issueCount}
              prefix={<WarningOutlined className="text-red-500 mr-2" />}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[24, 24]} style={{ marginTop: 48 }}>
        <Col span={16}>
          <Card 
            title={<Space><ProjectOutlined /> 最近导入业绩</Space>} 
            extra={<Button type="link" onClick={() => navigate('/library/performances')}>查看全部</Button>}
          >
            <Table
              columns={projectColumns}
              dataSource={data.recentProjects}
              pagination={false}
              size="middle"
              rowKey="id"
              locale={{ emptyText: <Empty description="暂无最近项目" /> }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card
            title={<Space><CheckCircleOutlined /> 待处理提醒</Space>}
            className="h-full"
          >
            <List
              dataSource={[
                { title: '业绩附件待确认', count: data.auditCount, type: 'audit', color: 'blue' },
                { title: '资料异常/冲突', count: data.issueCount, type: 'issue', color: 'red' },
                { title: '荣誉资料缺失', count: 4, type: 'honor', color: 'orange' },
              ]}
              renderItem={(item) => (
                <List.Item
                  className="px-4 hover:bg-gray-50 cursor-pointer rounded-lg transition-colors"
                  actions={[<ArrowRightOutlined key="arrow" />]}
                  onClick={() => {
                    if (item.type === 'audit') navigate('/file-center/audit');
                    else if (item.type === 'issue') navigate('/issues');
                    else if (item.type === 'honor') navigate('/library/honors');
                  }}
                >
                  <List.Item.Meta
                    title={item.title}
                    description={
                      <Space size="middle">
                        <Tag color={item.color}>{item.count} 项待办</Tag>
                        <Text type="secondary">需人工确认</Text>
                      </Space>
                    }
                  />
                </List.Item>
              )}
            />
            <Button
              type="dashed"
              block
              className="mt-6 border-dashed"
              style={{ height: '50px' }}
            >
              + 自定义提醒
            </Button>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
