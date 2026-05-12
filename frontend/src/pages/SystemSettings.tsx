import React, { useState, useEffect } from 'react';
import {
  Card, Form, Input, Button, Switch, Space, Typography, Tag,
  message, Alert, Row, Col, Select, Table, Modal, Tabs, Popconfirm, Tooltip
} from 'antd';
import {
  SafetyCertificateOutlined, RobotOutlined, CloudServerOutlined,
  SaveOutlined, PlusOutlined, BankOutlined, EditOutlined, DeleteOutlined,
  TranslationOutlined, DownloadOutlined, SyncOutlined,
  RocketOutlined, ApiOutlined, CheckCircleOutlined,
  ControlOutlined, PartitionOutlined, ThunderboltOutlined,
} from '@ant-design/icons';
import axios from 'axios';
import { useSearchParams } from 'react-router-dom';
import { useCompany } from '../context/CompanyContext';
import { Badge, Divider, InputNumber } from 'antd';
import PromptCenter from '../components/PromptCenter';
import { SystemSettingsSkeletonTab } from '../components/SystemSettingsSkeletonTab';

const { Title, Text, Paragraph } = Typography;

const SETTINGS_TAB_KEYS = new Set(['llm', 'ai', 'prompt', 'storage', 'company', 'auth', 'skeleton', 'techbid']);

interface SettingItem {
  key: string;
  value: string;
  description: string;
}

interface OcrProviderInfo {
  provider: string;
  endpoint: string;
  healthy: boolean | null;
  isDefault: boolean;
  pythonPath: string;
  scriptPath: string;
}

interface OCRSettingsData {
  mode: string;
  service_url: string;
  service_port: string;
  api_key: string;
  token: string;
  default_strategy: string;
  max_concurrency: number;
  timeout_seconds: number;
  retry_times: number;
  confidence_threshold?: number;
  allow_auto_download?: boolean;
  model_version?: string;
}

const SystemSettings: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const { companies, refreshCompanies } = useCompany();
  const [showCompanyModal, setShowCompanyModal] = useState(false);
  const [showModelModal, setShowModelModal] = useState(false);
  const [editingModelType, setEditingModelType] = useState<'openai' | 'aliyun' | 'doubao' | null>(null);
  const [loading, setLoading] = useState(false);
  const [incomingDir, setIncomingDir] = useState<string>('');
  const [openingIncoming, setOpeningIncoming] = useState(false);
  const [dbDir, setDbDir] = useState<string>('');
  const [openingDb, setOpeningDb] = useState(false);
  const [ocrChecking, setOcrChecking] = useState(false);
  const [testingAI, setTestingAI] = useState(false);
  const [testingDoubao, setTestingDoubao] = useState(false);
  const [testingOCR, setTestingOCR] = useState(false);
  const [ocrTestResult, setOcrTestResult] = useState<{ success: boolean; message: string; version?: string } | null>(null);
  const [providerInfo, setProviderInfo] = useState<OcrProviderInfo | null>(null);
  const [form] = Form.useForm();
  const [settings, setSettings] = useState<SettingItem[]>([]);

  const tabParam = searchParams.get('tab');
  const settingsActiveTab =
    tabParam && SETTINGS_TAB_KEYS.has(tabParam) ? tabParam : 'llm';

  const fetchProviderInfo = React.useCallback(async () => {
    try {
      const response = await axios.get('/api/imports/ocr/provider');
      setProviderInfo(response.data as OcrProviderInfo);
    } catch (err) {
      console.error('Failed to fetch OCR provider info:', err);
    }
  }, []);

  const fetchOCRSettings = React.useCallback(async (): Promise<OCRSettingsData | null> => {
    try {
      const response = await axios.get('/api/settings/ocr');
      return response.data as OCRSettingsData;
    } catch (err) {
      console.error('Failed to fetch OCR settings:', err);
      return null;
    }
  }, []);

  const fetchSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const standardResponse = await axios.get('/api/imports/settings');
      const standardData = standardResponse.data as SettingItem[];
      setSettings(standardData);

      const ocrData = await fetchOCRSettings();

      const formValues: Record<string, string | number | boolean | undefined> = {};
      standardData.forEach(item => {
        formValues[item.key] = item.value;
      });

      // Integrate OCR settings into the form
      if (ocrData) {
        Object.assign(formValues, ocrData);
      }

      // Defaults
      if (!formValues.mode) formValues.mode = 'local';
      if (!formValues.default_strategy) formValues.default_strategy = 'balanced';
      if (!formValues.service_port) formValues.service_port = 18082;
      if (!formValues.ai_provider) formValues.ai_provider = 'paddle_ocr';
      if (!formValues.local_ai_endpoint) formValues.local_ai_endpoint = 'http://127.0.0.1:18082';
      if (!formValues.ai_ingest_endpoint) formValues.ai_ingest_endpoint = 'http://127.0.0.1:18081';
      if (!formValues.ai_ingest_model) formValues.ai_ingest_model = 'glm-4';
      if (formValues.tech_bid_full_response_gate_config === undefined || formValues.tech_bid_full_response_gate_config === '') {
        formValues.tech_bid_full_response_gate_config = '{}';
      }
      if (formValues.tech_bid_industry_hard_rules === undefined || formValues.tech_bid_industry_hard_rules === '') {
        formValues.tech_bid_industry_hard_rules = '{"rules":[]}';
      }
      if (formValues.tech_bid_outline_similarity_config === undefined || formValues.tech_bid_outline_similarity_config === '') {
        formValues.tech_bid_outline_similarity_config = '{"jaccard_warn_threshold":0.85}';
      }

      form.setFieldsValue(formValues);
      await fetchProviderInfo();
    } catch (err) {
      console.error('Failed to fetch settings:', err);
    } finally {
      setLoading(false);
    }
  }, [form, fetchProviderInfo, fetchOCRSettings]);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  const fetchStorageInfo = React.useCallback(async () => {
    try {
      const res = await axios.get('/api/storage/info');
      setIncomingDir(String(res.data?.incoming_dir || ''));
      setDbDir(String(res.data?.db_dir || ''));
    } catch (err) {
      console.error('Failed to fetch storage info:', err);
      setIncomingDir('');
      setDbDir('');
    }
  }, []);

  useEffect(() => {
    if (settingsActiveTab === 'storage') {
      fetchStorageInfo();
    }
  }, [settingsActiveTab, fetchStorageInfo]);

  const onFinish = async (values: Record<string, string | number | boolean | undefined>) => {
    setLoading(true);
    try {
      // 1. Prepare standard settings
      const standardKeys = settings.map(s => s.key);
      // Some default keys might not be in settings yet
      const essentialKeys = [
        'ai_provider', 'local_ai_endpoint', 'ai_ingest_endpoint', 'ai_ingest_model', 'ai_api_key', 
        'doc_parser_access_key', 'doc_parser_access_secret', 'doc_parser_endpoint', 'doc_parser_structure_type', 
        'doubao_api_key', 'doubao_endpoint', 'doubao_model_id',
        'storage_path', 'export_template', 'auto_parse', 'auto_ingest_enabled',
        'tech_bid_full_response_gate_config', 'tech_bid_industry_hard_rules',
        'tech_bid_outline_similarity_config',
      ];

      const allKeys = Array.from(new Set([...standardKeys, ...essentialKeys]));

      const settingsToUpdate = allKeys
        .filter(key => values[key] !== undefined)
        .map(key => {
          const existing = settings.find(s => s.key === key);
          return {
            key,
            value: typeof values[key] === 'boolean' ? (values[key] ? 'true' : 'false') : String(values[key]),
            description: existing ? existing.description : ''
          };
        });

      await axios.post('/api/imports/settings', { settings: settingsToUpdate });

      // 2. Prepare and save OCR settings (Go backend)
      const ocrKeys = ['mode', 'service_url', 'service_port', 'api_key', 'token', 'default_strategy', 'max_concurrency', 'timeout_seconds', 'retry_times', 'confidence_threshold', 'allow_auto_download', 'model_version'];
      const ocrValues: Record<string, string | number | boolean | undefined> = {};
      ocrKeys.forEach(key => {
        if (values[key] !== undefined) {
          ocrValues[key] = values[key];
        }
      });

      if (Object.keys(ocrValues).length > 0) {
        await axios.put('/api/settings/ocr', ocrValues);

        // Sync ai_provider with mode for legacy system
        if (ocrValues.mode) {
          const providerValue = ocrValues.mode === 'private' ? 'local' : String(ocrValues.mode);
          const existing = settings.find(s => s.key === 'ai_provider');
          await axios.post('/api/imports/settings', {
            settings: [{
              key: 'ai_provider',
              value: providerValue,
              description: existing ? existing.description : 'OCR Provider Sync'
            }]
          });
        }
      }

      message.success('系统配置与 OCR 引擎设置已同步保存');
      await fetchProviderInfo();
      await fetchSettings();
    } catch (err) {
      console.error('Save error:', err);
      message.error('保存配置失败，请检查网络连接');
    } finally {
      setLoading(false);
    }
  };

  const handleTestAI = async () => {
    setTestingAI(true);
    try {
      const res = await axios.post('/api/imports/settings/test-ai');
      if (res.data.success) {
        message.success({
          content: `连接成功！模型回复：${res.data.message}`,
          duration: 5
        });
      } else {
        message.error(`连接失败：${res.data.error}`);
      }
    } catch (err: unknown) {
      const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
      message.error(`测试失败：${errorMsg}`);
    } finally {
      setTestingAI(false);
    }
  };

  const handleTestDoubao = async () => {
    setTestingDoubao(true);
    try {
      const res = await axios.post('/api/imports/settings/test-doubao');
      if (res.data.success) {
        message.success({
          content: `豆包连接成功！模型回复：${res.data.message}`,
          duration: 5
        });
      } else {
        message.error(`豆包连接失败：${res.data.error}`);
      }
    } catch (err: unknown) {
      const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
      message.error(`豆包测试失败：${errorMsg}`);
    } finally {
      setTestingDoubao(false);
    }
  };

  const handleCheckOcr = async () => {
    setOcrChecking(true);
    try {
      const response = await axios.post('/api/imports/ocr/health-check');
      if (response.data?.ok) {
        message.success(response.data.started ? 'PaddleOCR 已自动启动并可用' : 'PaddleOCR 服务可用');
      } else {
        message.warning('PaddleOCR 健康检查未通过');
      }
      await fetchProviderInfo();
    } catch (err: unknown) {
      const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
      message.error(`PaddleOCR 检查失败：${errorMsg}`);
    } finally {
      setOcrChecking(false);
    }
  };

  const handleTestOCRConnection = async () => {
    const values = form.getFieldsValue();
    if (!values.service_url) {
      message.warning('请先填写高级 OCR 引擎服务地址');
      return;
    }

    setTestingOCR(true);
    setOcrTestResult(null);
    try {
      const response = await axios.post('/api/settings/ocr/test', {
        service_url: values.service_url
      });
      setOcrTestResult({
        success: true,
        message: response.data.message,
        version: response.data.version
      });
      message.success('高级 OCR 引擎连通测试成功');
    } catch (err: unknown) {
      const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : '连接超时或服务未响应';
      setOcrTestResult({
        success: false,
        message: errorMsg
      });
      message.error('连通测试失败');
    } finally {
      setTestingOCR(false);
    }
  };

  const handleAddCompany = async (values: Record<string, string | undefined>) => {
    try {
      const payload = {
        company_name: values.company_name?.trim(),
        unified_social_credit_code: values.unified_social_credit_code?.trim() || undefined,
        legal_person: values.legal_person?.trim() || undefined,
      };
      await axios.post('/api/companies', payload);
      message.success('新公司注册成功');
      setShowCompanyModal(false);
      refreshCompanies();
    } catch (err: unknown) {
      const ax = err as { response?: { data?: { error?: string } }; message?: string };
      const detail = ax.response?.data?.error || ax.message || '请检查网络或后端日志';
      message.error(`注册失败：${detail}`);
    }
  };

  const handleDeleteCompany = async (id: string, name: string) => {
    if (companies.length <= 1) {
      message.warning('至少需保留一家公司主体');
      return;
    }
    try {
      await axios.delete(`/api/companies/${id}`);
      message.success(`已删除「${name}」`);
      refreshCompanies();
    } catch (err: unknown) {
      const ax = err as { response?: { data?: { error?: string } }; message?: string };
      const detail = ax.response?.data?.error || ax.message || '请稍后重试';
      message.error(`删除失败：${detail}`);
    }
  };

  const onSettingsTabChange = (key: string) => {
    if (key === 'llm') {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('tab');
        return next;
      }, { replace: true });
    } else {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set('tab', key);
          return next;
        },
        { replace: true },
      );
    }
  };

  const AIServiceTab = (
    <div className="space-y-6">
      {/* 模块 A: OCR 识别核心 */}
      <Card className="shadow-sm border-none">
        <Paragraph>
          <Text type="secondary">该模块用于从资质证书、项目经理身份证、业绩合同等 **图片或 PDF 扫描件** 中提取原始文字信息。</Text>
        </Paragraph>

        <div style={{ marginBottom: 24 }}>
          <Divider plain><Text style={{ color: '#004dc7', fontSize: '12px' }}><PartitionOutlined /> 核心架构设置</Text></Divider>

          <Row gutter={24}>
            <Col span={12}>
              <Form.Item name="mode" label="识别架构模式" tooltip="决定系统如何接入解析引擎">
                <Select placeholder="选择架构模式">
                  <Select.Option value="local">本地部署 (Local / 零成本 / 数据私有)</Select.Option>
                  <Select.Option value="cloud">云端服务 (Cloud API / 百度/阿里)</Select.Option>
                  <Select.Option value="private">企业私有云 (Private Cloud)</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="default_strategy" label="默认识别策略">
                <Select>
                  <Select.Option value="fast">性能优先 (Fast)</Select.Option>
                  <Select.Option value="balanced">平衡模式 (Balanced)</Select.Option>
                  <Select.Option value="accurate">精度优先 (Accurate)</Select.Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.mode !== curr.mode}>
            {({ getFieldValue }) => {
              const mode = getFieldValue('mode');
              return (
                <>
                  {/* 如果是本地模式或私有云，显示健康检查和 Endpoint */}
                  {(mode === 'local' || !mode) && (
                    <div style={{ marginBottom: 24 }}>
                      <Alert
                        showIcon
                        type={providerInfo?.healthy ? 'success' : 'info'}
                        message={providerInfo?.isDefault ? '当前默认识别工具：PaddleOCR' : '服务状态监测'}
                        description={
                          <div style={{ fontSize: '12px' }}>
                            <div>状态：{providerInfo?.healthy === null ? '未检测' : providerInfo?.healthy ? '可用' : '未启动/不可用'}</div>
                            <div>节点：{providerInfo?.endpoint || 'http://127.0.0.1:18082'}</div>
                          </div>
                        }
                        action={<Button size="small" onClick={handleCheckOcr} loading={ocrChecking}>自动启动/检查</Button>}
                        style={{ marginBottom: 16 }}
                      />
                    </div>
                  )}

                  {(mode === 'cloud' || mode === 'private') && (
                    <>
                      <Divider plain><Text style={{ color: '#004dc7', fontSize: '12px' }}><ApiOutlined /> 连接与鉴权 ({mode === 'cloud' ? '云端授权' : '接入网关'})</Text></Divider>

                      {mode === 'private' && (
                        <Row gutter={16} align="bottom" style={{ marginBottom: 24 }}>
                          <Col span={14}>
                            <Form.Item name="service_url" label="服务 Endpoint" rules={[{ required: true, message: '请输入服务地址' }]}>
                              <Input placeholder="http://127.0.0.1" addonAfter=":port" />
                            </Form.Item>
                          </Col>
                          <Col span={5}>
                            <Form.Item name="service_port" label="端口">
                              <InputNumber style={{ width: '100%' }} placeholder="18082" />
                            </Form.Item>
                          </Col>
                          <Col span={5}>
                            <Form.Item label=" ">
                              <Button
                                icon={<RocketOutlined />}
                                onClick={handleTestOCRConnection}
                                loading={testingOCR}
                                type="dashed"
                                block
                              >
                                测试
                              </Button>
                            </Form.Item>
                          </Col>
                        </Row>
                      )}

                      <Row gutter={24}>
                        <Col span={12}>
                          <Form.Item name="api_key" label="API Key">
                            <Input.Password placeholder="用于云端或私有部署鉴权" />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item name="token" label="Access Token">
                            <Input.Password placeholder="临时访问令牌" />
                          </Form.Item>
                        </Col>
                      </Row>
                    </>
                  )}

                  {ocrTestResult && (
                    <div style={{ marginBottom: 20 }}>
                      <Alert
                        message={ocrTestResult.success ? "配置正常" : "连接失败"}
                        description={
                          <Space direction="vertical" style={{ width: '100%' }}>
                            <Text style={{ fontSize: '12px' }}>{ocrTestResult.message}</Text>
                            {ocrTestResult.version && <Tag icon={<CheckCircleOutlined />} color="success" style={{ fontSize: '11px' }}>版本: {ocrTestResult.version}</Tag>}
                          </Space>
                        }
                        type={ocrTestResult.success ? "success" : "error"}
                        showIcon
                        style={{ borderRadius: '8px' }}
                      />
                    </div>
                  )}
                </>
              );
            }}
          </Form.Item>

          <Divider plain><Text style={{ color: '#004dc7', fontSize: '12px' }}><ControlOutlined /> 负载与精度控制</Text></Divider>

          <Row gutter={16}>
            <Col span={6}>
              <Form.Item name="max_concurrency" label="最大并发">
                <InputNumber min={1} max={16} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="timeout_seconds" label="超时(秒)">
                <InputNumber min={10} max={600} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="retry_times" label="自动重试">
                <InputNumber min={0} max={5} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="confidence_threshold" label="置信阈值" tooltip="0-1">
                <InputNumber min={0} max={1} step={0.1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={24} style={{ marginTop: 8 }}>
            <Col span={12}>
              <Form.Item name="allow_auto_download" label="允许自动下载模型" valuePropName="checked">
                <Switch size="small" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="model_version" label="锁定版本">
                <Input placeholder="latest" size="small" />
              </Form.Item>
            </Col>
          </Row>

          <Divider />
          <div style={{ display: 'flex', gap: 24, padding: '8px 16px', backgroundColor: '#f9fafb', borderRadius: '8px' }}>
            <Badge status="processing" text={<Text type="secondary" style={{ fontSize: '11px' }}>Local Node: 活跃</Text>} />
            <Badge status="warning" text={<Text type="secondary" style={{ fontSize: '11px' }}>Cloud Gateway: 待命</Text>} />
            <Badge status="default" text={<Text type="secondary" style={{ fontSize: '11px' }}>Cluster Mesh: 未配置</Text>} />
          </div>
        </div>
      </Card>
    </div>
  );

  const openModelEditor = (type: 'openai' | 'aliyun' | 'doubao') => {
    setEditingModelType(type);
    setShowModelModal(true);
  };

  const handleModelSave = async (values: Record<string, string | number | boolean | undefined>) => {
    setLoading(true);
    try {
      const currentValues = form.getFieldsValue();
      const updatedValues = { ...currentValues, ...values };
      await onFinish(updatedValues);
      setShowModelModal(false);
    } catch (err) {
      console.error('Failed to save model configuration:', err);
    } finally {
      setLoading(false);
    }
  };

  const LLMServiceTab = (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div>
        <Button 
          type="primary" 
          icon={<PlusOutlined />} 
          onClick={() => {
            const vals = form.getFieldsValue();
            if (!vals.doubao_api_key) openModelEditor('doubao');
            else if (!vals.doc_parser_access_key) openModelEditor('aliyun');
            else message.info('所有大模型服务已配置，请在对应卡片中选择“修改”');
          }}
        >
          添加大模型服务
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Card 
            title={<Space><TranslationOutlined style={{ color: '#52c41a' }} /> 线上大模型服务 (OpenAI/Qwen)</Space>}
            extra={<Button size="small" type="link" onClick={() => openModelEditor('openai')}>修改参数</Button>}
            className="shadow-sm border-none"
          >
            <Table
              className="read-only-settings-table"
              pagination={false}
              size="small"
              showHeader={false}
              columns={[{ dataIndex: 'label', width: 200, className: 'text-secondary' }, { dataIndex: 'value' }]}
              dataSource={[
                { key: '1', label: '接口地址 (Endpoint)', value: form.getFieldValue('ai_ingest_endpoint') || <Text type="secondary">未设置</Text> },
                { key: '2', label: '模型版本 (Model Name)', value: form.getFieldValue('ai_ingest_model') || <Text type="secondary">未设置</Text> },
                { key: '3', label: 'API Key', value: form.getFieldValue('ai_api_key') ? '••••••••••••••••' : <Text type="secondary">未设置</Text> },
              ]}
            />
            <div style={{ marginTop: 16 }}>
              <Button
                onClick={handleTestAI}
                loading={testingAI}
                size="small"
                icon={<SyncOutlined spin={testingAI} />}
                type="dashed"
              >
                连通性测试
              </Button>
            </div>
          </Card>
        </Col>

        <Col span={24}>
          <Card 
            title={<Space><ApiOutlined style={{ color: '#1890ff' }} /> 阿里云文档深度解析</Space>}
            extra={<Button size="small" type="link" onClick={() => openModelEditor('aliyun')}>修改参数</Button>}
            className="shadow-sm border-none"
          >
            <Table
              className="read-only-settings-table"
              pagination={false}
              size="small"
              showHeader={false}
              columns={[{ dataIndex: 'label', width: 200, className: 'text-secondary' }, { dataIndex: 'value' }]}
              dataSource={[
                { key: '1', label: 'AccessKey ID', value: form.getFieldValue('doc_parser_access_key') || <Text type="secondary">未设置</Text> },
                { key: '2', label: 'AccessKey Secret', value: form.getFieldValue('doc_parser_access_secret') ? '••••••••••••••••' : <Text type="secondary">未设置</Text> },
                { key: '3', label: 'Endpoint', value: form.getFieldValue('doc_parser_endpoint') || <Text type="secondary">未设置</Text> },
                { key: '4', label: '结构化类型', value: <Tag color="blue">{form.getFieldValue('doc_parser_structure_type') || 'default'}</Tag> },
              ]}
            />
          </Card>
        </Col>

        <Col span={24}>
          <Card 
            title={<Space><RobotOutlined style={{ color: '#eb2f96' }} /> 火山引擎 / 豆包大模型</Space>}
            extra={<Button size="small" type="link" onClick={() => openModelEditor('doubao')}>修改参数</Button>}
            className="shadow-sm border-none"
          >
            <Table
              className="read-only-settings-table"
              pagination={false}
              size="small"
              showHeader={false}
              columns={[{ dataIndex: 'label', width: 200, className: 'text-secondary' }, { dataIndex: 'value' }]}
              dataSource={[
                { key: '1', label: '服务地址 (Endpoint)', value: form.getFieldValue('doubao_endpoint') || <Text type="secondary">未设置</Text> },
                { key: '2', label: '推理接入点 ID (Endpoint ID)', value: form.getFieldValue('doubao_model_id') || <Text type="secondary">未设置</Text> },
                { key: '3', label: 'API Key', value: form.getFieldValue('doubao_api_key') ? '••••••••••••••••' : <Text type="secondary">未设置</Text> },
              ]}
            />
            <div style={{ marginTop: 16 }}>
              <Button
                onClick={handleTestDoubao}
                loading={testingDoubao}
                size="small"
                icon={<SyncOutlined spin={testingDoubao} />}
                type="dashed"
              >
                连通性测试
              </Button>
            </div>
          </Card>
        </Col>
      </Row>

      <Modal
        title={
          editingModelType === 'openai' ? '修改 OpenAI / Qwen 配置' : 
          editingModelType === 'aliyun' ? '修改阿里云解析配置' : 
          '修改豆包配置'
        }
        open={showModelModal}
        onCancel={() => setShowModelModal(false)}
        footer={null}
        destroyOnClose
      >
        <Form
          layout="vertical"
          initialValues={form.getFieldsValue()}
          onFinish={handleModelSave}
        >
          {editingModelType === 'openai' && (
            <>
              <Form.Item label="服务地址" name="ai_ingest_endpoint" rules={[{ required: true }]}>
                <Input placeholder="https://api.openai.com/v1" />
              </Form.Item>
              <Form.Item label="模型名称" name="ai_ingest_model" rules={[{ required: true }]}>
                <Input placeholder="qwen-max" />
              </Form.Item>
              <Form.Item label="API Key" name="ai_api_key" rules={[{ required: true }]}>
                <Input.Password />
              </Form.Item>
            </>
          )}

          {editingModelType === 'aliyun' && (
            <>
              <Form.Item label="AccessKey ID" name="doc_parser_access_key" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
              <Form.Item label="AccessKey Secret" name="doc_parser_access_secret" rules={[{ required: true }]}>
                <Input.Password />
              </Form.Item>
              <Form.Item label="Endpoint" name="doc_parser_endpoint">
                <Input placeholder="docmind-api.cn-hangzhou.aliyuncs.com" />
              </Form.Item>
              <Form.Item label="结构化类型" name="doc_parser_structure_type">
                <Select>
                  <Select.Option value="default">default</Select.Option>
                  <Select.Option value="doctree">doctree</Select.Option>
                  <Select.Option value="layout">layout</Select.Option>
                </Select>
              </Form.Item>
            </>
          )}

          {editingModelType === 'doubao' && (
            <>
              <Form.Item label="服务地址 (Endpoint)" name="doubao_endpoint" rules={[{ required: true }]}>
                <Input placeholder="https://ark.cn-beijing.volces.com/api/v3" />
              </Form.Item>
              <Form.Item label="模型 ID" name="doubao_model_id" rules={[{ required: true }]}>
                <Input placeholder="ep-2024..." />
              </Form.Item>
              <Form.Item label="API Key" name="doubao_api_key" rules={[{ required: true }]}>
                <Input.Password />
              </Form.Item>
            </>
          )}

          <Form.Item style={{ marginBottom: 0, marginTop: 24 }}>
            <div className="flex justify-end gap-2">
              <Button onClick={() => setShowModelModal(false)}>取消</Button>
              <Button type="primary" htmlType="submit">保存配置</Button>
            </div>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );

  return (
    <div className="max-w-6xl mx-auto pb-12 pt-8">


      <style>{`
        /* Premium Vertical Tabs Styling */
        .settings-tabs-vertical .ant-tabs-nav {
          width: 240px;
          border-right: 1px solid #edf2f7;
          margin-right: 32px !important;
          background: rgba(247, 250, 252, 0.5);
          border-radius: 16px;
          padding: 12px 0;
          height: fit-content;
        }
        
        .settings-tabs-vertical .ant-tabs-tab {
          margin: 4px 0 !important;
          padding: 12px 16px !important;
          border-radius: 10px !important;
          transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
          border: none !important;
          background: transparent !important;
          justify-content: flex-start !important;
        }
        
        .settings-tabs-vertical .ant-tabs-tab:hover {
          background: #f1f5f9 !important;
          color: #2563eb !important;
        }
        
        .settings-tabs-vertical .ant-tabs-tab-active {
          background: #eff6ff !important;
          box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
        }
        
        .settings-tabs-vertical .ant-tabs-tab-active .ant-tabs-tab-btn {
          color: #2563eb !important;
          font-weight: 600 !important;
        }
        
        .settings-tabs-vertical .ant-tabs-ink-bar {
          display: none !important;
        }
        
        .settings-tabs-vertical .ant-tabs-content-holder {
          padding-left: 8px;
          border-left: none !important;
        }
        
        /* Modern Card Adjustments */
        .ant-card {
          border-radius: 16px !important;
          border: 1px solid #f1f5f9 !important;
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02), 0 1px 2px rgba(0, 0, 0, 0.03) !important;
        }
        
        .ant-card-head {
          border-bottom: 1px solid #f1f5f9 !important;
          padding: 0 24px !important;
          min-height: 56px !important;
        }
        
        .ant-card-body {
          padding: 24px !important;
        }
        
        /* Save Section Enhancement */
        .premium-save-bar {
          background: #fff;
          border: 1px solid #e2e8f0;
          border-radius: 20px;
          padding: 24px;
          margin-top: 48px;
          box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05);
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 12px;
          transition: transform 0.2s;
        }
        
        .premium-save-bar:hover {
          transform: translateY(-2px);
        }
      `}</style>

      <Form
        form={form}
        layout="vertical"
        onFinish={onFinish}
        onValuesChange={() => form.submit()}
        requiredMark="optional"
        initialValues={{
          mode: 'local',
          default_strategy: 'balanced',
          service_port: 18082
        }}
      >
        <Tabs
          tabPosition="left"
          className="settings-tabs-vertical"
          activeKey={settingsActiveTab}
          onChange={onSettingsTabChange}
          items={[
            {
              key: 'llm',
              label: <Space><TranslationOutlined /> 大模型配置</Space>,
              children: LLMServiceTab,
            },
            {
              key: 'ai',
              label: <Space><RobotOutlined /> OCR配置</Space>,
              children: AIServiceTab,
            },
            {
              key: 'prompt',
              label: <Space><RocketOutlined /> 提示词配置</Space>,
              children: <PromptCenter />,
            },
            {
              key: 'storage',
              label: <Space><CloudServerOutlined /> 存储与工作流</Space>,
              children: (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
                  <Card title="存储路径设置" className="shadow-sm border-none">
                    <Form.Item label="文件库落盘目录（后端实际存储路径）">
                      <Space.Compact style={{ width: '100%' }}>
                        <Input
                          value={incomingDir || '—'}
                          readOnly
                          placeholder="正在读取..."
                        />
                        <Button
                          type="primary"
                          onClick={async () => {
                            setOpeningIncoming(true);
                            try {
                              await axios.post('/api/storage/open');
                              message.success('已打开文件库目录');
                            } catch (err: unknown) {
                              const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
                              message.error(`打开失败：${errorMsg}`);
                            } finally {
                              setOpeningIncoming(false);
                            }
                          }}
                          loading={openingIncoming}
                        >
                          打开
                        </Button>
                      </Space.Compact>
                      <Text type="secondary" className="text-xs">
                        说明：该目录为上传文件的真实落盘位置（`backend_go/data/files/incoming/` 的绝对路径）。
                      </Text>
                    </Form.Item>
                    <Form.Item label="数据库存储目录">
                      <Space.Compact style={{ width: '100%' }}>
                        <Input
                          value={dbDir || '—'}
                          readOnly
                          placeholder="正在读取..."
                        />
                        <Button
                          type="primary"
                          onClick={async () => {
                            setOpeningDb(true);
                            try {
                              await axios.post('/api/storage/open-db');
                              message.success('已打开数据库目录');
                            } catch (err: unknown) {
                              const errorMsg = axios.isAxiosError(err) ? (err.response?.data?.error || err.message) : String(err);
                              message.error(`打开失败：${errorMsg}`);
                            } finally {
                              setOpeningDb(false);
                            }
                          }}
                          loading={openingDb}
                        >
                          打开
                        </Button>
                      </Space.Compact>
                      <Text type="secondary" className="text-xs">
                        说明：该目录包含 SQLite 数据库文件（`backend_go/data/app.db`）。
                      </Text>
                    </Form.Item>
                  </Card>
                  <Card title="自动化开关" className="shadow-sm border-none">
                    <Row gutter={24}>
                      <Col span={12}>
                        <Form.Item label="开启自动识别入库" name="auto_parse" valuePropName="checked">
                          <Switch checkedChildren="开启" unCheckedChildren="关闭" />
                        </Form.Item>
                        <Text type="secondary" className="text-xs">上传资料后自动触发 AI 抽取</Text>
                      </Col>
                      <Col span={12}>
                        <Form.Item label="开启全自动静默入库" name="auto_ingest_enabled" valuePropName="checked">
                          <Switch />
                        </Form.Item>
                        <Text type="secondary" className="text-xs">跳过人工复核直接入库（慎用）</Text>
                      </Col>
                    </Row>
                  </Card>
                </div>
              ),
            },
            {
              key: 'company',
              label: <Space><BankOutlined /> 多主体管理</Space>,
              children: (
                <Card
                  title="多公司主体清单"
                  extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setShowCompanyModal(true)}>新增主体</Button>}
                  className="shadow-sm border-none"
                >
                  <Table
                    dataSource={companies}
                    rowKey="id"
                    size="small"
                    pagination={false}
                    columns={[
                      { title: '公司名称', dataIndex: 'company_name', render: (t) => <Text strong>{t}</Text> },
                      { title: '信用代码', dataIndex: 'unified_social_credit_code', render: (t) => t || '-' },
                      {
                        title: '操作',
                        key: 'action',
                        width: 120,
                        render: (_: unknown, record: { id: string; company_name: string }) => (
                          <Space size="small">
                            <Tooltip title="修改">
                              <Button type="text" size="small" icon={<EditOutlined style={{ color: '#1890ff' }} />} />
                            </Tooltip>
                            {companies.length <= 1 ? (
                              <Tooltip title="至少需保留一家公司主体">
                                <Button
                                  type="text"
                                  size="small"
                                  danger
                                  icon={<DeleteOutlined />}
                                  disabled
                                />
                              </Tooltip>
                            ) : (
                              <Popconfirm
                                title="确定删除此公司主体？"
                                description="删除后不可恢复；该公司下的业务数据仍将留在库中，但切换主体后可能无法从当前界面直接访问。"
                                okText="删除"
                                cancelText="取消"
                                okButtonProps={{ danger: true }}
                                onConfirm={() => handleDeleteCompany(record.id, record.company_name)}
                              >
                                <Tooltip title="删除">
                                  <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                                </Tooltip>
                              </Popconfirm>
                            )}
                          </Space>
                        ),
                      },
                    ]}
                  />
                </Card>
              ),
            },
            {
              key: 'auth',
              label: <Space><SafetyCertificateOutlined /> 授权与安全</Space>,
              children: (
                <Card className="shadow-sm border-none">
                  <Row align="middle" justify="space-between" className="mb-6">
                    <Col>
                      <Space direction="vertical">
                        <Text strong>当前授权状态：<Tag color="success">SVIP 已激活</Tag></Text>
                        <Text type="secondary">版本限制：不限制项目数量</Text>
                      </Space>
                    </Col>
                    <Col>
                      <Button icon={<DownloadOutlined />}>同步云端备份</Button>
                    </Col>
                  </Row>
                  <Alert
                    message="安全提示"
                    description="您的所有 API Key 均以加密形式存储在本地数据库中。"
                    type="info"
                    showIcon
                  />
                </Card>
              ),
            },
            {
              key: 'skeleton',
              label: <Space><PartitionOutlined /> 技术标骨架目录</Space>,
              children: <SystemSettingsSkeletonTab searchParams={searchParams} setSearchParams={setSearchParams} />,
            },
            {
              key: 'techbid',
              label: <Space><ThunderboltOutlined /> 技术标 Step4 规则</Space>,
              children: (
                <div className="space-y-6">
                  <Card title="完全响应门槛与行业硬规则" className="shadow-sm border-none">
                    <Alert
                      type="info"
                      showIcon
                      className="mb-4"
                      message="以 system_settings 键保存"
                      description="「完全响应门槛」键 tech_bid_full_response_gate_config；「行业硬规则」键 tech_bid_industry_hard_rules（rules 数组）；「目录雷同」键 tech_bid_outline_similarity_config，字段 jaccard_warn_threshold 为 0～1，超过则提示与历史项目标题集雷同。"
                    />
                    <Form.Item
                      label="tech_bid_full_response_gate_config"
                      name="tech_bid_full_response_gate_config"
                      rules={[
                        {
                          validator: async (_, v) => {
                            if (v === undefined || v === '') return;
                            try {
                              JSON.parse(String(v));
                            } catch {
                              throw new Error('须为合法 JSON');
                            }
                          },
                        },
                      ]}
                    >
                      <Input.TextArea rows={16} className="font-mono text-sm" placeholder="{}" />
                    </Form.Item>
                    <Form.Item
                      label="tech_bid_industry_hard_rules"
                      name="tech_bid_industry_hard_rules"
                      rules={[
                        {
                          validator: async (_, v) => {
                            if (v === undefined || v === '') return;
                            try {
                              JSON.parse(String(v));
                            } catch {
                              throw new Error('须为合法 JSON');
                            }
                          },
                        },
                      ]}
                    >
                      <Input.TextArea rows={12} className="font-mono text-sm" placeholder='{"rules":[]}' />
                    </Form.Item>
                    <Form.Item
                      label="tech_bid_outline_similarity_config"
                      name="tech_bid_outline_similarity_config"
                      rules={[
                        {
                          validator: async (_, v) => {
                            if (v === undefined || v === '') return;
                            try {
                              const o = JSON.parse(String(v)) as { jaccard_warn_threshold?: number };
                              if (o.jaccard_warn_threshold !== undefined) {
                                const x = Number(o.jaccard_warn_threshold);
                                if (Number.isNaN(x) || x <= 0 || x > 1) {
                                  throw new Error('jaccard_warn_threshold 须在 (0,1]');
                                }
                              }
                            } catch (e) {
                              if (e instanceof Error && e.message.includes('jaccard')) throw e;
                              throw new Error('须为合法 JSON');
                            }
                          },
                        },
                      ]}
                    >
                      <Input.TextArea rows={4} className="font-mono text-sm" placeholder='{"jaccard_warn_threshold":0.85}' />
                    </Form.Item>
                  </Card>
                </div>
              ),
            },
          ]}
        />

      </Form>

      <Modal
        title="新增公司主体"
        open={showCompanyModal}
        onCancel={() => setShowCompanyModal(false)}
        footer={null}
        destroyOnClose
      >
        <Form layout="vertical" onFinish={handleAddCompany}>
          <Form.Item label="公司名称" name="company_name" required rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如：中铁XX局第一工程有限公司" />
          </Form.Item>
          <Form.Item label="社会信用代码" name="unified_social_credit_code">
            <Input placeholder="91XXXXXXXXXXXXXXXX" />
          </Form.Item>
          <Form.Item label="法人代表" name="legal_person">
            <Input placeholder="张三" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" block htmlType="submit">立即创建</Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SystemSettings;
