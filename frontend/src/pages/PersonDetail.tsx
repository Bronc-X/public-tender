import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { Card, Typography, Button, Space, Descriptions, Spin, Result, Tag, Divider, Image, Modal, Upload, List, message, Popconfirm, Tooltip, Empty, Table, Tabs, Form, Input, Select, Row, Col, DatePicker } from 'antd';
import { LeftOutlined, EditOutlined, UploadOutlined, EyeOutlined, DeleteOutlined, FilePdfOutlined, FileImageOutlined, FileWordOutlined, ProjectOutlined, PlusOutlined } from '@ant-design/icons';
import axios from 'axios';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import zhCN from 'antd/es/date-picker/locale/zh_CN';
import { useCompany } from '../context/CompanyContext';

dayjs.locale('zh-cn');

const { Title, Text } = Typography;
const { Option } = Select;

interface Qualification {
  id: string;
  qualification_name?: string;
  qualification_type?: string;
  specialty?: string;
  registration_no?: string;
  certificate_no?: string;
  valid_from?: string;
  valid_to?: string;
  issuing_authority?: string;
  file_asset_id?: string;
  stored_path?: string;
}

interface PersonDetailData {
  id: string;
  name: string;
  gender?: string;
  join_date?: string;
  reg_date?: string;
  specialty?: string;
  id_number_masked?: string;
  id_card_no?: string;
  company_name?: string;
  social_security_status?: string;
  on_job_status?: string;
  bid_usable_status?: string;
  risk_status?: string;
  certificates?: Qualification[];
  proofs?: PersonProof[];
  performances?: PersonRelatedPerformance[];
  educations?: PersonEducation[];
  work_experiences?: PersonWorkExperience[];
}

interface PersonEducation {
  id: string;
  person_id: string;
  start_date?: string;
  end_date?: string;
  school?: string;
  degree?: string;
}

interface PersonWorkExperience {
  id: string;
  person_id: string;
  start_date?: string;
  end_date?: string;
  company?: string;
  position?: string;
}

interface PersonRelatedPerformance {
  id: string;
  project_name: string;
  role_name: string;
  project_manager_name?: string;
  winning_date?: string;
  completion_date?: string;
  amount_value?: number;
}

interface PersonProof {
  id: string;
  person_id: string;
  proof_type?: string;
  file_asset_id?: string;
  file_name?: string;
  ext?: string;
  created_at?: string;
  markdown_text?: string;
}

function labelSocial(v?: string) {
  if (v === 'active') return '已缴纳 (正常)';
  if (v === 'none') return '未缴纳 / 异常';
  return v || '-';
}

function labelOnJob(v?: string) {
  if (v === 'active') return '在职';
  if (v === 'resigned') return '离职';
  return v || '-';
}

function labelBidUsable(v?: string) {
  if (v === 'usable') return '可用 (未锁定)';
  if (v === 'locked') return '锁定 (项目执行中)';
  if (v === 'restricted') return '受限';
  return v || '-';
}

const PersonDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { currentCompanyId } = useCompany();
  const [loading, setLoading] = useState(true);
  const [person, setPerson] = useState<PersonDetailData | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Edit states
  const [isBasicInfoModalOpen, setIsBasicInfoModalOpen] = useState(false);
  const [basicInfoForm] = Form.useForm();
  const [savingBasicInfo, setSavingBasicInfo] = useState(false);

  // Certificate edit states
  const [editingCert, setEditingCert] = useState<Qualification | Partial<Qualification> | null>(null);
  const [certForm] = Form.useForm();
  const [savingCert, setSavingCert] = useState(false);

  // Preview states
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewUrl, setPreviewUrl] = useState('');
  const [previewTitle, setPreviewTitle] = useState('');
  const [previewType, setPreviewType] = useState<'image' | 'pdf' | 'word' | 'other'>('other');

  // Education edit states
  const [editingEdu, setEditingEdu] = useState<PersonEducation | Partial<PersonEducation> | null>(null);
  const [eduForm] = Form.useForm();
  const [savingEdu, setSavingEdu] = useState(false);

  // Work edit states
  const [editingWork, setEditingWork] = useState<PersonWorkExperience | Partial<PersonWorkExperience> | null>(null);
  const [workForm] = Form.useForm();
  const [savingWork, setSavingWork] = useState(false);

  const fetchData = React.useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const response = await axios.get<PersonDetailData>(`/api/persons/${id}`, {
        headers: { 'x-company-id': currentCompanyId },
      });
      setPerson(response.data);
    } catch (err: unknown) {
      console.error('Failed to fetch person detail:', err);
      const apiErr = err as { response?: { data?: { error?: string } } };
      setError(apiErr.response?.data?.error || '无法获取人员详情');
      setPerson(null);
    } finally {
      setLoading(false);
    }
  }, [id, currentCompanyId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Handlers
  const openBasicInfoEdit = () => {
    if (person) {
      basicInfoForm.setFieldsValue({
        name: person.name,
        gender: person.gender,
        join_date: person.join_date ? dayjs(person.join_date) : undefined,
        id_card_no: person.id_card_no,
        company_name: person.company_name,
        social_security_status: person.social_security_status || 'none',
        on_job_status: person.on_job_status || 'active',
        bid_usable_status: person.bid_usable_status || 'usable',
      });
      setIsBasicInfoModalOpen(true);
    }
  };

  const handleBasicInfoSubmit = async () => {
    try {
      const values = await basicInfoForm.validateFields();
      if (values.join_date) {
        values.join_date = values.join_date.format('YYYY-MM-DD');
      }
      setSavingBasicInfo(true);
      await axios.patch(`/api/persons/${id}`, values, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('基本信息已更新');
      setIsBasicInfoModalOpen(false);
      fetchData(); 
    } catch (err: unknown) {
      if (err instanceof Error) {
        message.error('更新失败: ' + err.message);
      }
    } finally {
      setSavingBasicInfo(false);
    }
  };

  const openCertEdit = (cert?: Qualification) => {
    if (cert) {
      setEditingCert(cert);
      certForm.setFieldsValue({
        ...cert,
        valid_from: cert.valid_from || person?.reg_date
      });
    } else {
      setEditingCert({ id: 'new' });
      certForm.resetFields();
    }
  };

  const handleCertSubmit = async () => {
    try {
      const values = await certForm.validateFields();
      setSavingCert(true);
      if (editingCert?.id && editingCert.id !== 'new') {
        // Update existing certificate
        await axios.patch(`/api/qualifications/${editingCert.id}`, values, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('证书已更新');
      } else {
        // Create new certificate
        await axios.post(`/api/qualifications`, { 
          ...values, 
          owner_id: id, 
          owner_type: 'person' 
        }, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('证书已添加');
      }
      setEditingCert(null);
      fetchData();
    } catch (err: unknown) {
      if (err instanceof Error) {
         message.error('保存失败: ' + err.message);
      }
    } finally {
      setSavingCert(false);
    }
  };

  const handleDeleteCert = async (certId: string) => {
      try {
        await axios.delete(`/api/qualifications/${certId}`, {
           headers: { 'x-company-id': currentCompanyId }
        });
        message.success('证书已删除');
        fetchData();
      } catch (err) {
        message.error('删除证书失败');
      }
  };

  const openEduEdit = (edu?: PersonEducation) => {
    if (edu) {
      setEditingEdu(edu);
      eduForm.setFieldsValue({
        ...edu,
        start_date: edu.start_date ? dayjs(edu.start_date) : undefined,
        end_date: edu.end_date ? dayjs(edu.end_date) : undefined,
      });
    } else {
      setEditingEdu({ id: 'new', person_id: id });
      eduForm.resetFields();
    }
  };

  const handleEduSubmit = async () => {
    try {
      const values = await eduForm.validateFields();
      const formattedValues = {
        ...values,
        start_date: values.start_date ? values.start_date.format('YYYY-MM') : undefined,
        end_date: values.end_date ? values.end_date.format('YYYY-MM') : undefined,
      };
      setSavingEdu(true);
      if (editingEdu?.id && editingEdu.id !== 'new') {
        await axios.patch(`/api/persons/educations/${editingEdu.id}`, formattedValues, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('学习经历已更新');
      } else {
        await axios.post(`/api/persons/${id}/educations`, formattedValues, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('学习经历已添加');
      }
      setEditingEdu(null);
      fetchData();
    } catch (err) {
      message.error('保存失败');
    } finally {
      setSavingEdu(false);
    }
  };

  const handleDeleteEdu = async (eduId: string) => {
    try {
      await axios.delete(`/api/persons/educations/${eduId}`, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('已删除学习经历');
      fetchData();
    } catch (err) {
      message.error('删除失败');
    }
  };

  const openWorkEdit = (work?: PersonWorkExperience) => {
    if (work) {
      setEditingWork(work);
      workForm.setFieldsValue({
        ...work,
        start_date: work.start_date ? dayjs(work.start_date) : undefined,
        end_date: (work.end_date && work.end_date !== '至今') ? dayjs(work.end_date) : undefined,
      });
    } else {
      setEditingWork({ id: 'new', person_id: id });
      workForm.resetFields();
    }
  };

  const handleWorkSubmit = async () => {
    try {
      const values = await workForm.validateFields();
      const formattedValues = {
        ...values,
        start_date: values.start_date ? values.start_date.format('YYYY-MM') : undefined,
        end_date: values.end_date ? values.end_date.format('YYYY-MM') : '至今',
      };
      setSavingWork(true);
      if (editingWork?.id && editingWork.id !== 'new') {
        await axios.patch(`/api/persons/works/${editingWork.id}`, formattedValues, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('工作经历已更新');
      } else {
        await axios.post(`/api/persons/${id}/works`, formattedValues, {
          headers: { 'x-company-id': currentCompanyId }
        });
        message.success('工作经历已添加');
      }
      setEditingWork(null);
      fetchData();
    } catch (err) {
      message.error('保存失败');
    } finally {
      setSavingWork(false);
    }
  };

  const handleDeleteWork = async (workId: string) => {
    try {
      await axios.delete(`/api/persons/works/${workId}`, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('已删除工作经历');
      fetchData();
    } catch (err) {
      message.error('删除失败');
    }
  };

  const handleRenameProof = async (fileAssetId: string | undefined, newName: string) => {
    if (!fileAssetId || !newName.trim()) return;
    try {
      await axios.patch(`/api/files/${fileAssetId}`, { file_name: newName }, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('附件已重命名');
      fetchData();
    } catch (err) {
      if (err instanceof Error) {
        message.error('重命名失败: ' + err.message);
      } else {
        message.error('重命名失败');
      }
    }
  };



  if (loading && !person) {
    return (
      <div className="flex justify-center items-center h-64">
        <Spin size="large" tip="正在载入人员数据..." />
      </div>
    );
  }

  if (error || !person) {
    return (
      <Result
        status="error"
        title="获取详情失败"
        subTitle={error}
        extra={[
          <Button type="primary" key="back" onClick={() => navigate('/library/persons')}>
            返回列表
          </Button>,
        ]}
      />
    );
  }

  const tabItems = [
    {
      key: 'basic',
      label: '基本信息',
      children: (
        <Card className="shadow-none border-none bg-transparent"
          title={
            <Button type="primary" icon={<EditOutlined />} onClick={openBasicInfoEdit}>
              编辑信息
            </Button>
          }
        >
          <Descriptions column={{ xs: 1, sm: 2, md: 2 }} bordered size="middle">
            <Descriptions.Item label="人员ID">{person.id}</Descriptions.Item>
            <Descriptions.Item label="姓名">{person.name}</Descriptions.Item>
            <Descriptions.Item label="性别">{person.gender || '-'}</Descriptions.Item>
            <Descriptions.Item label="入职时间">{person.join_date || '-'}</Descriptions.Item>
            <Descriptions.Item label="身份证号">{person.id_card_no || '-'}</Descriptions.Item>
            <Descriptions.Item label="所属单位">{person.company_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="在职状态">{labelOnJob(person.on_job_status)}</Descriptions.Item>
            <Descriptions.Item label="社保状态">{labelSocial(person.social_security_status)}</Descriptions.Item>
            <Descriptions.Item label="投标可用性">{labelBidUsable(person.bid_usable_status)}</Descriptions.Item>
          </Descriptions>
        </Card>
      )
    },
    {
      key: 'certs',
      label: '相关证书',
      children: (
        <Card className="shadow-none border-none bg-transparent"
          title={
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openCertEdit()}>
              添加证书
            </Button>
          }
        >
          {person.certificates && person.certificates.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
              {person.certificates.map((cert, index) => (
                <div key={cert.id} className="p-4 border rounded-md">
                  <Descriptions 
                    title={
                      <div className="flex justify-between items-center w-full">
                        <span>{person.certificates!.length >= 1 ? `证书 ${index + 1} - ${cert.qualification_name || '未知证书'}` : undefined}</span>
                        <Space>
                          <Button type="text" icon={<EditOutlined />} style={{ color: '#8c8c8c' }} onClick={() => openCertEdit(cert)} />
                          <Popconfirm title="确定删除该证书吗？" onConfirm={() => handleDeleteCert(cert.id)}>
                            <Button type="text" icon={<DeleteOutlined />} style={{ color: '#8c8c8c' }} />
                          </Popconfirm>
                        </Space>
                      </div>
                    }
                    column={{ xs: 1, sm: 2 }} 
                    bordered 
                    size="middle"
                  >
                    <Descriptions.Item label="资格类型">{cert.qualification_type || '-'}</Descriptions.Item>
                    <Descriptions.Item label="证书类别">{cert.qualification_name || '-'}</Descriptions.Item>
                    <Descriptions.Item label="专业">{cert.specialty || person.specialty || '-'}</Descriptions.Item>
                    <Descriptions.Item label="证书编号">{cert.certificate_no || '-'}</Descriptions.Item>
                    <Descriptions.Item label="注册编号">{cert.registration_no || '-'}</Descriptions.Item>
                    <Descriptions.Item label="颁发单位">{cert.issuing_authority || '-'}</Descriptions.Item>
                    <Descriptions.Item label="注册时间">{person.reg_date || cert.valid_from || '-'}</Descriptions.Item>
                    <Descriptions.Item label="有效期截止">{cert.valid_to || '-'}</Descriptions.Item>
                    <Descriptions.Item label="证书图片">
                      {cert.file_asset_id ? (
                        <Button 
                          type="link" 
                          style={{ padding: 0 }} 
                          onClick={() => {
                            Modal.info({
                              title: `原件预览 - ${person.name} (${cert.qualification_name || '证书'})`,
                              width: 800,
                              centered: true,
                              maskClosable: true,
                              icon: null,
                              content: (
                                <div style={{ textAlign: 'center', padding: '10px 0', maxHeight: '75vh', overflowY: 'auto' }}>
                                  {cert.stored_path?.toLowerCase().endsWith('.pdf') ? (
                                    <iframe
                                      src={`/api/files/download/${cert.file_asset_id}`}
                                      style={{ width: '100%', height: '600px', border: '1px solid #eee', borderRadius: 4 }}
                                      title={cert.qualification_name}
                                    />
                                  ) : (
                                    <Image
                                      src={`/api/files/download/${cert.file_asset_id}`}
                                      style={{ maxWidth: '100%', borderRadius: 4, boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
                                    />
                                  )}
                                </div>
                              ),
                              okText: '关闭'
                            });
                          }}
                        >
                          查看原图
                        </Button>
                      ) : (
                        '--'
                      )}
                    </Descriptions.Item>
                  </Descriptions>
                </div>
              ))}
            </div>
          ) : (
            <Empty description="暂无关联证书" />
          )}
        </Card>
      )
    },
    {
      key: 'performances',
      label: '相关业绩',
      children: (
        <Card className="shadow-none border-none bg-transparent">
          <Table
            dataSource={person.performances || []}
            columns={[
              {
                title: '项目名称',
                dataIndex: 'project_name',
                key: 'project_name',
                render: (text: string, record: PersonRelatedPerformance) => (
                  <Link to={`/library/performances/${record.id}`}>{text}</Link>
                ),
              },
              {
                title: '岗位名称',
                dataIndex: 'role_name',
                key: 'role_name',
              },
              {
                title: '项目经理',
                dataIndex: 'project_manager_name',
                key: 'project_manager_name',
                render: (text: string) => text || '-',
              },
              {
                title: '中标时间',
                dataIndex: 'winning_date',
                key: 'winning_date',
                render: (text: string) => text ? dayjs(text).format('YYYY-MM-DD') : '-',
              },
              {
                title: '完工时间',
                dataIndex: 'completion_date',
                key: 'completion_date',
                render: (text: string) => text ? dayjs(text).format('YYYY-MM-DD') : '-',
              },
              {
                title: '合同金额',
                dataIndex: 'amount_value',
                key: 'amount_value',
                render: (val: number) => val != null ? `${val.toLocaleString()} 元` : '-',
              },
            ]}
            rowKey="id"
            pagination={false}
            locale={{ emptyText: '暂无相关业绩' }}
          />
        </Card>
      )
    },
    {
      key: 'educations',
      label: '学习经历',
      children: (
        <Card className="shadow-none border-none bg-transparent"
          title={
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openEduEdit()}>
              添加学习经历
            </Button>
          }
        >
          <Table
            dataSource={person.educations || []}
            rowKey="id"
            pagination={false}
            size="small"
            columns={[
              { title: '起止时间', key: 'time', render: (_, r) => `${r.start_date || '-'} 至 ${r.end_date || '-'}` },
              { title: '毕业院校', dataIndex: 'school', key: 'school' },
              { title: '学历学位', dataIndex: 'degree', key: 'degree' },
              { 
                title: '操作', 
                key: 'action', 
                width: 100,
                render: (_, r) => (
                  <Space size="small">
                    <Button type="text" size="small" icon={<EditOutlined />} style={{ color: '#8c8c8c' }} onClick={() => openEduEdit(r)} />
                    <Popconfirm title="确定删除？" onConfirm={() => handleDeleteEdu(r.id)}>
                      <Button type="text" size="small" icon={<DeleteOutlined />} style={{ color: '#8c8c8c' }} />
                    </Popconfirm>
                  </Space>
                )
              }
            ]}
            locale={{ emptyText: '暂无学习经历' }}
          />
        </Card>
      )
    },
    {
      key: 'work_experiences',
      label: '工作经历',
      children: (
        <Card className="shadow-none border-none bg-transparent"
          title={
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openWorkEdit()}>
              添加工作经历
            </Button>
          }
        >
          <Table
            dataSource={person.work_experiences || []}
            rowKey="id"
            pagination={false}
            size="small"
            columns={[
              { title: '起止时间', key: 'time', render: (_, r) => `${r.start_date || '-'} 至 ${r.end_date || '-'}` },
              { title: '工作单位', dataIndex: 'company', key: 'company' },
              { title: '担任职务', dataIndex: 'position', key: 'position' },
              { 
                title: '操作', 
                key: 'action', 
                width: 100,
                render: (_, r) => (
                  <Space size="small">
                    <Button type="text" size="small" icon={<EditOutlined />} style={{ color: '#8c8c8c' }} onClick={() => openWorkEdit(r)} />
                    <Popconfirm title="确定删除？" onConfirm={() => handleDeleteWork(r.id)}>
                      <Button type="text" size="small" icon={<DeleteOutlined />} style={{ color: '#8c8c8c' }} />
                    </Popconfirm>
                  </Space>
                )
              }
            ]}
            locale={{ emptyText: '暂无工作经历' }}
          />
        </Card>
      )
    },
    {
      key: 'attachments',
      label: '相关附件',
      children: (
        <Card
          className="shadow-none border-none bg-transparent"
          title={
            <Upload
              showUploadList={false}
              customRequest={async (options) => {
                const { file, onSuccess, onError } = options;
                const formData = new FormData();
                formData.append('file', file);
                formData.append('source_module', 'person');
                formData.append('source_project_id', id || '');

                try {
                  const uploadRes = await axios.post('/api/files/upload', formData);
                  const fileAssetId = uploadRes.data.id;

                  await axios.post(`/api/persons/${id}/attachments`, {
                    file_asset_id: fileAssetId,
                    proof_type: 'contract'
                  });

                  message.success('附件上传成功');
                  fetchData();
                  if (onSuccess) onSuccess("ok");
                } catch (err) {
                  message.error('附件上传失败');
                  if (onError) onError(err as Error);
                }
              }}
            >
              <Button icon={<UploadOutlined />} type="primary">上传附件</Button>
            </Upload>
          }
        >
          {!person.proofs || person.proofs.length === 0 ? (
            <Empty description="暂无相关附件" />
          ) : (
            <List
              grid={{ gutter: 16, xxl: 4, xl: 3, lg: 2, md: 2, sm: 1, xs: 1 }}
              dataSource={person.proofs}
              renderItem={(item) => (
                <List.Item>
                  <Card
                    size="small"
                    hoverable
                    className="bg-gray-50 border-gray-200"
                    actions={[
                      <Tooltip title="预览" key="view">
                        <EyeOutlined onClick={() => handlePreview(item)} />
                      </Tooltip>,
                      <Popconfirm
                        key="delete"
                        title="确定删除此附件吗？"
                        onConfirm={() => handleDeleteProof(item.id)}
                        okText="确定"
                        cancelText="取消"
                      >
                        <DeleteOutlined style={{ color: 'rgba(0, 0, 0, 0.45)' }} />
                      </Popconfirm>
                    ]}
                  >
                    <Card.Meta
                      avatar={
                        item.ext === '.pdf' ? (
                          <FilePdfOutlined style={{ fontSize: 32, color: '#ff4d4f' }} />
                        ) : item.ext?.includes('.doc') ? (
                          <FileWordOutlined style={{ fontSize: 32, color: '#1890ff' }} />
                        ) : (
                          <FileImageOutlined style={{ fontSize: 32, color: '#1890ff' }} />
                        )
                      }
                      title={
                        <div className="truncate" style={{ maxWidth: '100%' }}>
                          <Text 
                            strong 
                            editable={{
                              tooltip: '重命名',
                              onChange: (val) => handleRenameProof(item.file_asset_id, val),
                            }}
                          >
                            {item.file_name}
                          </Text>
                        </div>
                      }
                      description={
                        <Space direction="vertical" size={0}>
                          <Tag color="blue">相关附件</Tag>
                          <Text type="secondary" style={{ fontSize: '12px' }}>
                            上传于 {dayjs(item.created_at).format('YYYY-MM-DD')}
                          </Text>
                        </Space>
                      }
                    />
                  </Card>
                </List.Item>
              )}
            />
          )}
        </Card>
      )
    }
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }} className="animate-in fade-in duration-500">
      <div className="flex justify-between items-center w-full bg-white p-4 rounded-lg shadow-sm border border-gray-100">
        <Space size="middle" style={{ flex: 1, minWidth: 0, alignItems: 'center' }}>
          <Button icon={<LeftOutlined />} onClick={() => navigate(-1)} />
          <Title level={3} style={{ margin: '0 0 4px 0' }}>
            {person.name}
          </Title>
          <div style={{ marginTop: '0px' }}>
             {person.risk_status === 'warning' && <Tag color="orange" style={{ marginLeft: 8 }}>资料异常</Tag>}
          </div>
        </Space>
      </div>

      <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-100">
         <Tabs items={tabItems} defaultActiveKey="basic" />
      </div>

      {/* Edit Basic Info Modal */}
      <Modal
        title="编辑基本信息"
        open={isBasicInfoModalOpen}
        onOk={handleBasicInfoSubmit}
        onCancel={() => setIsBasicInfoModalOpen(false)}
        confirmLoading={savingBasicInfo}
        destroyOnClose
        width={700}
      >
         <Form form={basicInfoForm} layout="vertical" className="mt-4">
            <Row gutter={16}>
               <Col span={12}>
                  <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
                     <Input placeholder="输入姓名" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="gender" label="性别">
                    <Select placeholder="选择性别">
                      <Option value="男">男</Option>
                      <Option value="女">女</Option>
                    </Select>
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="join_date" label="入职时间">
                     <DatePicker style={{width: '100%'}} format="YYYY-MM-DD" placeholder="选择入职时间" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="id_card_no" label="身份证号">
                     <Input placeholder="输入身份证" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="company_name" label="所属单位">
                     <Input placeholder="输入单位名称" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="social_security_status" label="社保状态">
                    <Select placeholder="选择状态">
                      <Option value="active">已缴纳 (正常)</Option>
                      <Option value="none">未缴纳 / 异常</Option>
                    </Select>
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="on_job_status" label="在职状态">
                    <Select placeholder="选择状态">
                      <Option value="active">在职</Option>
                      <Option value="resigned">离职</Option>
                    </Select>
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="bid_usable_status" label="投标可用性">
                    <Select placeholder="选择可用性">
                      <Option value="usable">可用 (未锁定)</Option>
                      <Option value="locked">锁定 (项目执行中)</Option>
                      <Option value="restricted">受限</Option>
                    </Select>
                  </Form.Item>
               </Col>
            </Row>
         </Form>
      </Modal>

      {/* Edit Certificate Modal */}
      <Modal
        title={editingCert?.id && editingCert.id !== 'new' ? '编辑证书' : '添加证书'}
        open={!!editingCert}
        onOk={handleCertSubmit}
        onCancel={() => setEditingCert(null)}
        confirmLoading={savingCert}
        destroyOnClose
        width={700}
      >
         <Form form={certForm} layout="vertical" className="mt-4">
            <Row gutter={16}>
               <Col span={12}>
                  <Form.Item name="qualification_type" label="资格类型">
                     <Input placeholder="如：住建部人员资格" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="qualification_name" label="证书类别">
                     <Input placeholder="如：注册建造师_二级" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="specialty" label="专业">
                     <Input placeholder="如：建筑工程" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="certificate_no" label="证书编号">
                     <Input placeholder="输入证书编号" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="registration_no" label="注册编号">
                     <Input placeholder="输入注册编号" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="issuing_authority" label="颁发单位">
                     <Input placeholder="颁发单位名称" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="valid_from" label="注册时间">
                     <Input placeholder="如：YYYY-MM-DD" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="valid_to" label="有效期截止">
                     <Input placeholder="如：YYYY-MM-DD" />
                  </Form.Item>
               </Col>
            </Row>
         </Form>
      </Modal>

      {/* Edit Education Modal */}
      <Modal
        title={editingEdu?.id && editingEdu.id !== 'new' ? '编辑学习经历' : '添加学习经历'}
        open={!!editingEdu}
        onOk={handleEduSubmit}
        onCancel={() => setEditingEdu(null)}
        confirmLoading={savingEdu}
        destroyOnClose
        width={600}
      >
         <Form form={eduForm} layout="vertical" className="mt-4">
            <Row gutter={16}>
               <Col span={12}>
                  <Form.Item name="start_date" label="开始时间" rules={[{ required: true, message: '请选择开始时间' }]}>
                     <DatePicker picker="month" format="YYYY-MM" placeholder="如：2016-09" style={{ width: '100%' }} locale={zhCN} />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="end_date" label="结束时间" rules={[{ required: true, message: '请选择结束时间' }]}>
                     <DatePicker picker="month" format="YYYY-MM" placeholder="如：2020-06" style={{ width: '100%' }} locale={zhCN} />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="school" label="毕业院校" rules={[{ required: true }]}>
                     <Input placeholder="输入毕业院校" />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="degree" label="学历学位" rules={[{ required: true }]}>
                     <Input placeholder="如：本科 / 学士" />
                  </Form.Item>
               </Col>
            </Row>
         </Form>
      </Modal>

      {/* Edit Work Experience Modal */}
      <Modal
        title={editingWork?.id && editingWork.id !== 'new' ? '编辑工作经历' : '添加工作经历'}
        open={!!editingWork}
        onOk={handleWorkSubmit}
        onCancel={() => setEditingWork(null)}
        confirmLoading={savingWork}
        destroyOnClose
        width={600}
      >
         <Form form={workForm} layout="vertical" className="mt-4">
            <Row gutter={16}>
               <Col span={12}>
                  <Form.Item name="start_date" label="开始时间" rules={[{ required: true, message: '请选择开始时间' }]}>
                     <DatePicker picker="month" format="YYYY-MM" placeholder="如：2020-07" style={{ width: '100%' }} locale={zhCN} />
                  </Form.Item>
               </Col>
               <Col span={12}>
                  <Form.Item name="end_date" label="结束时间">
                     <DatePicker picker="month" format="YYYY-MM" placeholder="不填表示至今" style={{ width: '100%' }} allowClear locale={zhCN} />
                  </Form.Item>
               </Col>
               <Col span={24}>
                  <Form.Item name="company" label="工作单位" rules={[{ required: true }]}>
                     <Input placeholder="输入单位名称" />
                  </Form.Item>
               </Col>
               <Col span={24}>
                  <Form.Item name="position" label="担任职务" rules={[{ required: true }]}>
                     <Input placeholder="输入职务" />
                  </Form.Item>
               </Col>
            </Row>
         </Form>
      </Modal>

      <Modal
        open={previewVisible}
        title={previewTitle}
        footer={null}
        onCancel={() => setPreviewVisible(false)}
        width={previewType === 'pdf' ? '80%' : 1000}
        style={{ top: 20 }}
        styles={{
          body: {
            height: previewType === 'pdf' ? 'calc(100vh - 150px)' : 'calc(90vh - 150px)',
            overflow: 'auto',
            textAlign: 'center',
            padding: 24,
            backgroundColor: '#ffffff'
          }
        }}
      >
        {previewType === 'image' && (
          <div style={{ backgroundColor: '#ffffff', minHeight: '300px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
            <img alt="preview" style={{ maxWidth: '100%', display: 'block' }} src={previewUrl} />
          </div>
        )}
        {previewType === 'pdf' && (
          <div style={{ backgroundColor: '#ffffff', height: '100%', display: 'flex', flexDirection: 'column' }}>
            <div style={{ flex: 1, minHeight: 0 }}>
              <iframe
                src={previewUrl}
                style={{ width: '100%', height: '100%', border: 'none' }}
                title="pdf-preview"
              />
            </div>
            <div style={{ padding: '12px 0', borderTop: '1px solid #f0f0f0', display: 'flex', justifyContent: 'center', gap: '16px' }}>
              <Text type="secondary">预览异常？</Text>
              <Button size="small" type="link" onClick={() => window.open(previewUrl)}>在新标签页打开</Button>
              <Button size="small" type="link" onClick={() => window.open(previewUrl)}>下载文件</Button>
            </div>
          </div>
        )}
        {previewType === 'other' && (
          <div className="py-24 text-center bg-white" style={{ backgroundColor: '#ffffff' }}>
            <Empty
              description={
                <Space direction="vertical" size="large">
                  <Text type="secondary" style={{ fontSize: '18px' }}>暂不支持该格式的在线预览</Text>
                  <Button size="large" type="primary" onClick={() => window.open(previewUrl)}>
                    下载附件查看
                  </Button>
                </Space>
              }
            />
          </div>
        )}
      </Modal>
    </div>
  );

  function handlePreview(item: PersonProof) {
    const url = `/api/files/download/${item.file_asset_id}/view.pdf`;
    const ext = item.ext?.toLowerCase();

    setPreviewUrl(url);
    setPreviewTitle(item.file_name || '文件预览');

    if (ext === '.pdf') {
      setPreviewType('pdf');
    } else if (['.jpg', '.jpeg', '.png', '.gif', '.webp'].includes(ext || '')) {
      setPreviewType('image');
    } else {
      setPreviewType('other');
    }

    setPreviewVisible(true);
  }

  async function handleDeleteProof(proofId: string) {
    try {
      await axios.delete(`/api/persons/attachments/${proofId}`, {
        headers: { 'x-company-id': currentCompanyId }
      });
      message.success('附件已删除');
      fetchData();
    } catch (err) {
      console.error('Delete failed:', err);
      message.error('删除失败');
    }
  }
};

export default PersonDetail;
