import React, { useState } from 'react';
import { Modal, Upload, Button, Table, Select, Space, Typography, message, Steps, Alert } from 'antd';
import type { UploadFile } from 'antd/es/upload/interface';
import { InboxOutlined, CheckCircleOutlined, RocketOutlined } from '@ant-design/icons';
import axios from 'axios';

const { Dragger } = Upload;
const { Text, Title } = Typography;

interface MappingRow {
  targetField: string;
  targetLabel: string;
  sourceHeader: string | null;
  required: boolean;
}

interface ImportResult {
  total: number;
  success: number;
  failed: number;
  errors?: string[];
  batchId: string;
}

interface PerformanceImportModalProps {
  visible: boolean;
  onCancel: () => void;
  onSuccess: () => void;
}

const PerformanceImportModal: React.FC<PerformanceImportModalProps> = ({ visible, onCancel, onSuccess }) => {
  const [currentStep, setCurrentStep] = useState(0);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [mapping, setMapping] = useState<MappingRow[]>([]);
  const [availableHeaders, setAvailableHeaders] = useState<string[]>([]);
  const [fileId, setFileId] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);

  const reset = () => {
    setCurrentStep(0);
    setFileList([]);
    setMapping([]);
    setFileId(null);
    setImportResult(null);
  };

  const handleUpload = async () => {
    if (fileList.length === 0) return;
    setLoading(true);
    try {
      const formData = new FormData();
      if (fileList[0]?.originFileObj) {
        formData.append('file', fileList[0].originFileObj);
      } else {
        throw new Error('No file selected');
      }
      formData.append('source_module', 'performance_import');

      const uploadResp = await axios.post('/api/files/upload', formData);
      const uploadedFileId = uploadResp.data.id;
      setFileId(uploadedFileId);

      const analyzeResp = await axios.post('/api/imports/analyze', { file_id: uploadedFileId });
      setAvailableHeaders(analyzeResp.data.headers);
      setMapping(analyzeResp.data.suggestedMapping);
      setCurrentStep(1);
    } catch (err: unknown) {
      message.error('上传或解析文件失败');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleExecute = async () => {
    if (!fileId) return;
    setLoading(true);
    try {
      const resp = await axios.post('/api/imports/execute', {
        fileId,
        targetType: 'performance',
        mapping: mapping.map(m => ({
          targetField: m.targetField,
          sourceHeader: m.sourceHeader
        }))
      });
      setImportResult(resp.data);
      setCurrentStep(2);
      onSuccess();
    } catch (err: unknown) {
      message.error('执行导入失败');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: '业绩库字段',
      dataIndex: 'targetLabel',
      key: 'targetLabel',
      render: (text: string, record: MappingRow) => (
        <span>
          {text} {record.required && <Text type="danger">*</Text>}
        </span>
      ),
    },
    {
      title: 'Excel 表头列',
      dataIndex: 'sourceHeader',
      key: 'sourceHeader',
      render: (val: string, record: MappingRow) => (
        <Select
          style={{ width: '100%' }}
          placeholder="请选择对应的列"
          value={val || undefined}
          onChange={(v) => {
            const newMapping = mapping.map(m => 
              m.targetField === record.targetField ? { ...m, sourceHeader: v } : m
            );
            setMapping(newMapping);
          }}
          allowClear
        >
          {availableHeaders.map(h => (
            <Select.Option key={h} value={h}>{h}</Select.Option>
          ))}
        </Select>
      ),
    }
  ];

  return (
    <Modal
      title="批量导入业绩"
      open={visible}
      onCancel={() => { onCancel(); reset(); }}
      width={800}
      footer={null}
      destroyOnClose
    >
      <Steps
        current={currentStep}
        style={{ marginBottom: 32 }}
        items={[
          { title: '上传表格' },
          { title: '字段对齐' },
          { title: '完成导入' },
        ]}
      />

      {currentStep === 0 && (
        <div style={{ padding: '20px 0' }}>
          <Dragger
            beforeUpload={(file) => {
              setFileList([file]);
              return false;
            }}
            fileList={fileList}
            onRemove={() => setFileList([])}
            accept=".xlsx,.xls"
            maxCount={1}
          >
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">点击或拖拽 Excel 文件到此处上传</p>
            <p className="ant-upload-hint">支持 .xlsx, .xls 格式。请确保第一行为表头。</p>
          </Dragger>
          <div style={{ marginTop: 24, textAlign: 'center' }}>
            <Button 
                type="primary" 
                size="large" 
                disabled={fileList.length === 0} 
                loading={loading}
                onClick={handleUpload}
                style={{ width: 160 }}
            >
              下一步：对齐字段
            </Button>
          </div>
        </div>
      )}

      {currentStep === 1 && (
        <div>
          <Alert
            message="字段映射确认"
            description="请核对 Excel 表头与业绩库字段的对应关系，系统已为您自动尝试匹配。"
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />
          <Table
            dataSource={mapping}
            columns={columns}
            pagination={false}
            rowKey="targetField"
            size="middle"
            scroll={{ y: 400 }}
          />
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setCurrentStep(0)}>上一步</Button>
              <Button 
                type="primary" 
                loading={loading} 
                icon={<RocketOutlined />}
                onClick={handleExecute}
                disabled={mapping.some(m => m.required && !m.sourceHeader)}
              >
                立即开始导入
              </Button>
            </Space>
          </div>
        </div>
      )}

      {currentStep === 2 && importResult && (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <CheckCircleOutlined style={{ fontSize: 64, color: '#52c41a', marginBottom: 24 }} />
          <Title level={3}>导入完成</Title>
          <div style={{ marginBottom: 32 }}>
            <Text>成功导入 <Text strong type="success">{importResult.success}</Text> 条，</Text>
            <Text>失败 <Text strong type="danger">{importResult.failed}</Text> 条。</Text>
          </div>
          {importResult.errors && importResult.errors.length > 0 && (
            <div style={{ textAlign: 'left', background: '#fff1f0', padding: 16, borderRadius: 8, maxHeight: 200, overflowY: 'auto', marginBottom: 24 }}>
              {importResult.errors.map((err: string, i: number) => (
                <div key={i}><Text type="danger" style={{ fontSize: 12 }}>• {err}</Text></div>
              ))}
            </div>
          )}
          <Button type="primary" onClick={() => { onCancel(); reset(); }}>
            返回列表
          </Button>
        </div>
      )}
    </Modal>
  );
};

export default PerformanceImportModal;
