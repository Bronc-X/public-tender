import React, { useState, useEffect, useCallback } from 'react';
import { Card, Steps, Upload, Button, Table, Select, Space, Typography, message, Tag, Progress, Badge, Drawer, Empty, Input, Modal, Tooltip, Image } from 'antd';
import { 
  CloudUploadOutlined,
  TableOutlined,
  RocketOutlined,
  FileExcelOutlined,
  FilePdfOutlined,
  FileImageOutlined,
  CheckCircleOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  FolderOpenOutlined,
  FileSearchOutlined as AuditOutlined,
  SyncOutlined,
  EyeOutlined,
  FileTextOutlined,
  InfoCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
  ThunderboltOutlined,
  DeleteOutlined,
  SearchOutlined
} from '@ant-design/icons';
import axios from 'axios';
import { useNavigate, useSearchParams, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useCompany } from '../context/CompanyContext';
import fileCenterApi from '../api/fileCenter';
import LibraryDetailModal from '../components/LibraryDetailModal';
import type { FileAsset, AuditItem } from '../api/fileCenter';
import { labelForAuditObjectType } from '../constants/auditObjectTypes';
const { Title, Text, Link } = Typography;

function highlightKeyword(text: string, keyword: string): React.ReactNode {
  const raw = (text ?? '').toString();
  const kw = (keyword ?? '').trim();
  if (!kw) return raw;

  const lowerText = raw.toLowerCase();
  const lowerKw = kw.toLowerCase();

  const parts: React.ReactNode[] = [];
  let start = 0;
  let idx = lowerText.indexOf(lowerKw, start);
  if (idx === -1) return raw;

  while (idx !== -1) {
    if (idx > start) parts.push(raw.slice(start, idx));
    parts.push(
      <span key={`${idx}-${kw}`} style={{ color: '#ff4d4f', fontWeight: 'bold' }}>
        {raw.slice(idx, idx + kw.length)}
      </span>
    );
    start = idx + kw.length;
    idx = lowerText.indexOf(lowerKw, start);
  }
  if (start < raw.length) parts.push(raw.slice(start));

  return <>{parts}</>;
}

// --- Interfaces ---
interface TaskInfo {
    id: string;
    status: 'pending' | 'running' | 'completed' | 'failed' | 'finished';
    progress: number;
    message: string;
}

interface FileCenterProps {
    independentTab?: 'repository' | 'import' | 'audit';
}

/**
 * 状态映射表
 */
const STATUS_MAP: Record<string, { label: string; color: string; icon: React.ReactNode }> = {
  'uploaded': { label: '已上传/待预处理', color: 'default', icon: <ClockCircleOutlined /> },
  'processing': { label: '正在提取 OCR...', color: 'processing', icon: <SyncOutlined spin /> },
  'analyzing': { label: 'AI 智能解析中...', color: 'blue', icon: <SyncOutlined spin /> },
  'queued': { label: '排队处理ocr...', color: 'cyan', icon: <ClockCircleOutlined /> },
  'running': { label: '正在识别提取...', color: 'processing', icon: <SyncOutlined spin /> },
  'waiting_review': { label: '待人工审核', color: 'warning', icon: <AuditOutlined /> },
  'pending': { label: '待人工核对', color: 'warning', icon: <AuditOutlined /> },
  'reviewing': { label: '正在审核中', color: 'orange', icon: <EyeOutlined /> },
  'approved': { label: '审核已通过', color: 'success', icon: <CheckCircleOutlined /> },
  'archived': { label: '已入库', color: 'success', icon: <CheckCircleOutlined /> },
  'failed': { label: '识别处理失败', color: 'error', icon: <CloseCircleOutlined /> },
  'ignored': { label: '已忽略处理', color: 'default', icon: <ExclamationCircleOutlined /> },
};

const FileCenter: React.FC<FileCenterProps> = ({ independentTab }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { currentCompanyId } = useCompany();
  
  // Tab State: repository | import | audit
  const activeTab = searchParams.get('tab') || independentTab || 'repository';
  const setActiveTab = useCallback((tab: string) => setSearchParams({ tab }), [setSearchParams]);

  const [loading, setLoading] = useState(false);
  const [fileList, setFileList] = useState<FileAsset[]>([]);
  const [fileFilter, setFileFilter] = useState({ type: 'all', name: '' });
  
  // Projects/Context for Ingest
  const [projects, setProjects] = useState<{id: string, name: string}[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [importType, setImportType] = useState<'doc' | 'tech' | 'excel'>('doc');

  // Multi-step Process States
  const [currentStep, setCurrentStep] = useState(0);
  const [uploadFileList, setUploadFileList] = useState<any[]>([]);
  const [currentTask, setCurrentTask] = useState<TaskInfo | null>(null);

  // Audit States
  const [auditData, setAuditData] = useState<AuditItem[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);

  // OCR Status
  const [ocrStatus, setOcrStatus] = useState<{ success: boolean; message: string; version?: string } | null>(null);

  // Preview States
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileAsset | null>(null);
  const [pdfPreviewUrl, setPdfPreviewUrl] = useState<string>('');
  const [pdfPreviewLoading, setPdfPreviewLoading] = useState(false);
  const [pdfPreviewError, setPdfPreviewError] = useState<string>('');

  // Detail Drawer
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedFile, setSelectedFile] = useState<FileAsset | null>(null);

  // Library Item Details Modal
  const [libDetailVisible, setLibDetailVisible] = useState(false);
  const [libDetailType, setLibDetailType] = useState<string | undefined>(undefined);
  const [libDetailId, setLibDetailId] = useState<string | undefined>(undefined);

  // Batch Selection
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [pageSize, setPageSize] = useState(10);

  // Quick Upload Ref
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  const handleQuickUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    const count = files.length;
    message.loading({ content: `正在快传 ${count} 个文件...`, key: 'quick-upload' });
    setLoading(true);

    try {
        const uploadPromises = Array.from(files).map(file => 
            fileCenterApi.uploadFile(file, { source_module: 'general' })
        );
        await Promise.all(uploadPromises);
        message.success({ content: `成功上传 ${count} 个文件！已进入自动解析排队。`, key: 'quick-upload', duration: 4 });
        fetchFiles();
    } catch (err) {
        console.error('Quick upload error:', err);
        message.error({ content: '文件快传出现部分失败，请重试。', key: 'quick-upload' });
    } finally {
        setLoading(false);
        if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  // --- Data Fetching ---
  const fetchFiles = useCallback(async () => {
    setLoading(true);
    try {
        const data = await fileCenterApi.listFiles();
        const processed = data.map((item, index) => ({
            ...item,
            __unique_key__: item.id ? `${item.id}-idx${index}` : `noid-idx${index}`
        }));
        setFileList((processed as any));
    } catch (e) {
        console.error(e);
        message.error('加载文件列表失败');
    } finally {
        setLoading(false);
    }
  }, [currentCompanyId]);

  // Polling logic for In-Progress files
  useEffect(() => {
    // 把 waiting_review/pending/reviewing 也纳入轮询，避免后端最终写回 approved/archived 前 UI 卡住。
    const hasIncomplete = (fileList || []).some(f =>
      ['uploaded', 'processing', 'analyzing', 'queued', 'running', 'waiting_review', 'pending', 'reviewing'].includes(
        f.status || ''
      )
    );

    let intervalId: any = null;
    if (hasIncomplete) {
        intervalId = setInterval(() => {
            // Fetch silently without full component loading state
            fileCenterApi.listFiles().then(data => {
                const processed = data.map((item, index) => ({
                    ...item,
                    __unique_key__: item.id ? `${item.id}-idx${index}` : `noid-idx${index}`
                }));
                setFileList((processed as any));
            }).catch(() => {});
        }, 4000); // 4 seconds
    }

    return () => {
        if (intervalId) clearInterval(intervalId);
    };
  }, [fileList]);

  const fetchAuditData = useCallback(async () => {
    setAuditLoading(true);
    try {
      const data = await fileCenterApi.listAudits();
      setAuditData(data);
    } catch (err) {
      console.error('Failed to fetch audits:', err);
      message.error('无法加载待审核列表');
    } finally {
      setAuditLoading(false);
    }
  }, []);

  const fetchProjects = useCallback(async () => {
    try {
        const res = await axios.get('/api/tech-bid/projects');
        setProjects(res.data);
    } catch (e) {
        console.error(e);
    }
  }, []);

  const checkOcrStatus = useCallback(async () => {
    try {
      const response = await axios.get('/api/imports/ocr/provider');
      const data = response.data;
      setOcrStatus({
        success: data.healthy,
        message: data.healthy ? `已连通 PaddleOCR (${data.endpoint.replace('http://', '')})` : 'OCR 引擎未就绪 (请检查 18082 端口)',
        version: data.provider
      });
    } catch (e) {
      setOcrStatus({ success: false, message: '无法连接到检测服务' });
    }
  }, []);

  useEffect(() => {
    if (activeTab === 'repository') fetchFiles();
    if (activeTab === 'audit') fetchAuditData();
    if (activeTab === 'import') {
        fetchProjects();
        checkOcrStatus();
    }
  }, [activeTab, location.key, currentCompanyId, fetchFiles, fetchAuditData, fetchProjects, checkOcrStatus]);

  // PDF preview: fetch blob through axios (keeps auth / X-Company-Id headers), then render via blob URL.
  useEffect(() => {
    let cancelled = false;
    const revoke = (url: string) => {
      try {
        if (url) URL.revokeObjectURL(url);
      } catch {
        /* ignore */
      }
    };

    // cleanup if modal closed / file changed / non-pdf
    const clear = () => {
      setPdfPreviewLoading(false);
      setPdfPreviewError('');
      setPdfPreviewUrl((prev) => {
        revoke(prev);
        return '';
      });
    };

    if (!previewVisible || !previewFile) {
      clear();
      return () => {};
    }

    const isPdf = previewFile.file_name?.toLowerCase().endsWith('.pdf');
    if (!isPdf) {
      clear();
      return () => {};
    }

    setPdfPreviewLoading(true);
    setPdfPreviewError('');
    setPdfPreviewUrl((prev) => {
      revoke(prev);
      return '';
    });

    (async () => {
      try {
        const resp = await axios.get(`/api/files/download/${previewFile.id}`, { responseType: 'blob' });
        if (cancelled) return;
        const blob = resp.data as Blob;
        const url = URL.createObjectURL(blob);
        setPdfPreviewUrl(url);
      } catch (e) {
        console.error('PDF preview fetch failed:', e);
        if (cancelled) return;
        setPdfPreviewError('PDF 加载失败，请稍后重试或使用新窗口打开');
      } finally {
        if (!cancelled) setPdfPreviewLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [previewVisible, previewFile]);

  // Task Polling Effect
  useEffect(() => {
    let timer: any;
    if (currentTask && currentTask.status === 'running') {
      timer = setInterval(async () => {
        try {
          const data = await fileCenterApi.getTaskStatus(currentTask.id);
          if (data.status === 'finished' || data.status === 'completed' || data.status === 'success') {
            setCurrentTask(prev => prev ? { ...prev, status: 'completed', progress: 100 } : null);
            message.success('AI 识别完成，数据已全自动归库！');
            if (independentTab) {
                navigate('/file-center/repository');
            } else {
                setActiveTab('repository');
            }
            fetchAuditData();
            setCurrentStep(0);
            setCurrentTask(null);
            setUploadFileList([]);
            clearInterval(timer);
          } else {
            setCurrentTask(prev => prev ? { ...prev, progress: data.progress } : null);
          }
        } catch (e) {
          console.error('Poll error:', e);
          clearInterval(timer);
        }
      }, 2000);
    }
    return () => clearInterval(timer);
  }, [currentTask, setActiveTab, independentTab, navigate, fetchAuditData]);

  // --- Handlers ---
  const handleDocBatchUpload = async () => {
    if (uploadFileList.length === 0) {
      message.warning('请先选择文件');
      return;
    }
    setLoading(true);
    try {
      setCurrentStep(1);
      let completedCount = 0;
      const total = uploadFileList.length;
      const mode = importType === 'doc' ? 'standard' : 'advanced';

      for (const fileItem of uploadFileList) {
        // 1. Upload file
        const uploadRes = await fileCenterApi.uploadFile(fileItem, {
           source_module: importType === 'tech' ? 'tech_bid' : 'library'
        });
        const fileId = uploadRes.id;

        // 2. Start Task (Integration point with FileTaskService)
        const taskRes = await axios.post('/api/tech-bid/import/tasks', {
            task_name: `AI 识别: ${fileItem.name}`,
            source_file_id: fileId,
            source_project_id: selectedProjectId,
            target_library_type: importType === 'doc' ? 'qualification' : (importType === 'tech' ? 'method' : 'performance'),
            ocr_mode: mode
        });

        completedCount++;
        setCurrentTask({
            id: taskRes.data.id,
            status: 'running',
            progress: Math.round((completedCount / total) * 100),
            message: `正在识别 (${completedCount}/${total}): ${fileItem.name}`
        });
      }

    } catch (err: any) {
      console.error('Upload flow error:', err);
      message.error('上传或识别任务创建失败');
      setCurrentStep(0);
    } finally {
      setLoading(false);
    }
  };

  const onIgnoreAudit = async (auditId: string) => {
      try {
          await fileCenterApi.ignoreAudit(auditId);
          message.success('已忽略该任务');
          fetchAuditData();
      } catch (err) {
          message.error('操作失败');
      }
  };

  const handleStartOcr = async (fileId: string) => {
    try {
        setLoading(true);
        await fileCenterApi.startOcrTask(fileId);
        message.success('已成功启动 OCR 识别任务，请稍候查看进度。');
        // Refresh list to show 'Processing' status
        fetchFiles();
    } catch (err) {
        console.error('Start OCR error:', err);
        message.error('启动识别任务失败，请检查 OCR 服务配置。');
    } finally {
        setLoading(false);
    }
  };

  /** 与智能审核案台「提交确认」相同：用当前 extracted_data + object_type 归档入库 */
  const handleQuickConfirmAudit = useCallback(
    async (auditId: string) => {
      if (!auditId) return;
      const msgKey = 'quick-confirm-audit';
      message.loading({ content: '正在按提取结果入库…', key: msgKey, duration: 0 });
      try {
        const detail = await fileCenterApi.getAuditDetail(auditId);
        if (detail.audit_status === 'processing') {
          message.destroy(msgKey);
          message.warning('AI 仍在解析中，请稍后再试或进入审核台查看');
          return;
        }
        let raw: unknown[] = [];
        if (detail.extracted_data) {
          try {
            const parsed = JSON.parse(detail.extracted_data) as unknown;
            if (Array.isArray(parsed)) raw = parsed;
          } catch {
            /* ignore */
          }
        }
        const items = raw.map((it: unknown, i: number) => {
          const o = it && typeof it === 'object' ? (it as Record<string, unknown>) : {};
          return {
            title: typeof o.title === 'string' ? o.title : `条目${i + 1}`,
            summary: typeof o.summary === 'string' ? o.summary : '',
            content: typeof o.content === 'string' ? o.content : String(o.content ?? ''),
            confidence: typeof o.confidence === 'number' ? o.confidence : 0.8,
            source_page: typeof o.source_page === 'string' ? o.source_page : '1',
          };
        });
        let finalItems = items;
        if (finalItems.length === 0) {
          const t = detail.ocr_text?.trim();
          if (!t) {
            message.destroy(msgKey);
            message.error('暂无可入库的提取内容，请进入审核台处理');
            return;
          }
          finalItems = [{ title: '全文摘录', summary: '', content: t, confidence: 0.7, source_page: '1' }];
        }
        await fileCenterApi.confirmAudit(auditId, {
          file_id: detail.file_id,
          extracted_items: finalItems,
          object_type: detail.object_type || 'general',
          confirmed_text: detail.ocr_text || '',
        });
        message.destroy(msgKey);
        message.success('已按当前提取结果完成入库（等同审核台「提交确认」）');
        fetchFiles();
        fetchAuditData();
      } catch (e) {
        console.error(e);
        message.destroy(msgKey);
        message.error('入库失败，请进入审核台手动确认');
      }
    },
    [fetchFiles, fetchAuditData]
  );

  const handleDeleteFile = (id: string, fileName: string) => {
    Modal.confirm({
      title: '确认删除资产',
      icon: <ExclamationCircleOutlined />,
      content: `确定要永久删除文件 "${fileName}" 吗？此操作不可撤销，且会同步删除相关的解析和审核记录。`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await fileCenterApi.deleteFile(id);
          message.success('文件资产已永久删除');
          fetchFiles();
        } catch (err) {
          message.error('删除文件失败');
        }
      },
    });
  };

  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) return;
    Modal.confirm({
      title: '确认批量删除资产',
      icon: <ExclamationCircleOutlined />,
      content: `确定要永久删除选中的 ${selectedRowKeys.length} 个文件吗？此操作不可撤销，且会同步删除相关的解析和审核记录。`,
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
            return fileCenterApi.deleteFile(actualId);
          });
          await Promise.all(promises);
          message.success(`成功删除选中文件`);
          setSelectedRowKeys([]);
          fetchFiles();
        } catch (err) {
          message.error('批量删除出现部分或全部失败，请刷新查看');
          setSelectedRowKeys([]);
          fetchFiles();
        }
      },
    });
  };

  // --- Render Helpers ---

  const renderStatusTag = (status: string | undefined) => {
    const config = STATUS_MAP[status || 'uploaded'] || { label: status, color: 'default', icon: <InfoCircleOutlined /> };
    return (
      <Tag color={config.color} icon={config.icon}>
        {config.label}
      </Tag>
    );
  };

  const renderRepository = () => (
    <div style={{ padding: '0 0 24px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-start', alignItems: 'center', marginBottom: 24 }}>
            <Space>
                {independentTab === 'repository' && (
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => fileInputRef.current?.click()} loading={loading}>上传</Button>
                )}
                <Select defaultValue="all" style={{ width: 140 }} onChange={v => setFileFilter(f => ({ ...f, type: v }))}>
                    <Select.Option value="all">所有文件类型</Select.Option>
                    <Select.Option value=".pdf">PDF 文档</Select.Option>
                    <Select.Option value=".xlsx">Excel 表格</Select.Option>
                    <Select.Option value="image">图片素材</Select.Option>
                </Select>
                <Input 
                    placeholder="搜索文件名..." 
                    prefix={<SearchOutlined />} 
                    allowClear 
                    onChange={e => setFileFilter(f => ({ ...f, name: e.target.value }))}
                    style={{ width: 220 }}
                />
            </Space>
        </div>
        <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table 
            rowSelection={{
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys),
            }}
            pagination={{
                pageSize,
                onChange: (page, size) => setPageSize(size),
                onShowSizeChange: (current, size) => setPageSize(size),
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
                        <Text style={{ marginLeft: 16 }}>共 {total} 条</Text>
                    </Space>
                )
            }}
            dataSource={(fileList || []).filter(f => {
                const name = f.file_name || '';
                if (fileFilter.type !== 'all' && (
                    (fileFilter.type === 'image' && !name.match(/\.(jpg|jpeg|png|gif)$/i)) ||
                    (fileFilter.type !== 'image' && !name.toLowerCase().endsWith(fileFilter.type))
                )) return false;
                if (fileFilter.name && !name.toLowerCase().includes(fileFilter.name.toLowerCase())) return false;
                return true;
            })} 
            loading={loading}
            size="middle"
            rowKey="__unique_key__"
            columns={[
                { title: '文件名', dataIndex: 'file_name', key: 'name', render: (t, record) => (
                    <Space>
                        {t.endsWith('.pdf') ? <FilePdfOutlined style={{ color: '#ff4d4f' }} /> : 
                         t.match(/\.(jpg|jpeg|png)$/i) ? <FileImageOutlined style={{ color: '#1890ff' }} /> :
                         <FileExcelOutlined style={{ color: '#52c41a' }} />}
                        <Link 
                            strong 
                            onClick={() => {
                                setPreviewFile(record);
                                setPreviewVisible(true);
                            }}
                        >
                            {highlightKeyword(t, fileFilter.name)}
                        </Link>
                    </Space>
                )},
                { title: '大小', dataIndex: 'file_size', key: 'size', render: (s) => s ? (s / 1024 / 1024).toFixed(2) + ' MB' : '--' },
                { title: '创建时间', dataIndex: 'created_at', key: 'time', render: (t) => dayjs(t).format('YYYY-MM-DD HH:mm') },
                { title: '状态', key: 'status', render: (_: unknown, record: FileAsset) => {
                    const status = record.status || record.scan_status || 'uploaded';
                    const isArchived = (status === 'approved' || status === 'archived') && !!record.archive_target_id;
                    const tag = isArchived 
                        ? <Tag color="success" icon={<CheckCircleOutlined />}>已入库</Tag>
                        : renderStatusTag(status);
                    if (isArchived) {
                        return (
                            <Link 
                                onClick={() => {
                                    setLibDetailType(record.archive_target_type);
                                    setLibDetailId(record.archive_target_id);
                                    setLibDetailVisible(true);
                                }}
                            >
                                {tag}
                            </Link>
                        );
                    }
                    return tag;
                }},
                {
                  title: '分类',
                  dataIndex: 'object_type',
                  key: 'object_type',
                  width: 120,
                  render: (_: unknown, record: FileAsset) => {
                    const label = labelForAuditObjectType(record.object_type);
                    return label === '—' ? <Text type="secondary">—</Text> : <Tag color="blue">{label}</Tag>;
                  },
                },
                { title: '操作', key: 'action', width: 220, render: (_, record) => {
                    const status = record.status || record.scan_status || 'uploaded';
                    const dimIcon = 'rgba(0, 0, 0, 0.55)';
                    const openPreviewOrAudit = () => {
                        if (record.audit_id) {
                            navigate(`/audits/${record.audit_id}`, { state: { background: location } });
                        } else {
                            setSelectedFile(record);
                            setDetailVisible(true);
                        }
                    };
                    const eyeBtn = (
                        <Tooltip key="eye" title="查看详情">
                            <Button
                                type="text"
                                size="small"
                                icon={<EyeOutlined style={{ color: dimIcon, fontSize: 16 }} />}
                                aria-label="查看详情"
                                onClick={openPreviewOrAudit}
                            />
                        </Tooltip>
                    );

                    const deleteBtn = (
                        <Tooltip key="del" title="删除数据">
                            <Button
                                size="small"
                                type="text"
                                icon={<DeleteOutlined style={{ color: 'rgba(0, 0, 0, 0.55)', fontSize: 16 }} />}
                                aria-label="删除数据"
                                onClick={() => handleDeleteFile(record.id, record.file_name)}
                            />
                        </Tooltip>
                    );

                    const quickAuditBtn = (
                        <Button
                            key="quick-audit"
                            size="small"
                            type="primary"
                            ghost
                            icon={<AuditOutlined />}
                            onClick={() => {
                                if (record.audit_id) {
                                    void handleQuickConfirmAudit(record.audit_id);
                                } else {
                                    setActiveTab('audit');
                                }
                            }}
                        >
                            审核入库
                        </Button>
                    );

                    const isUploadOrFailed = status === 'uploaded' || status === 'failed';
                    const isPerformance = record.source_module === 'performance';
                    const actionNodes: React.ReactNode[] = [eyeBtn, deleteBtn];
                    
                    if (isUploadOrFailed && !isPerformance) {
                        actionNodes.push(
                            <Button key="ocr" size="small" type="primary" icon={<ThunderboltOutlined />} onClick={() => handleStartOcr(record.id)}>
                                识别提取
                            </Button>
                        );
                    }
                    if (status === 'waiting_review' || status === 'pending') {
                        actionNodes.push(quickAuditBtn);
                    }

                    return <Space>{actionNodes}</Space>;
                }},
            ]}
        />
        </Card>
    </div>
  );

  const renderImport = () => (
    <div style={{ paddingTop: 8 }}>
      <Steps 
        current={currentStep} 
        size="small"
        items={[
            { title: '批量上传', icon: <CloudUploadOutlined /> },
            { title: 'AI 识别解析', icon: <RocketOutlined /> },
            { title: '人工审核归档', icon: <CheckCircleOutlined /> }
        ]} 
        style={{ marginBottom: 40, maxWidth: 800, margin: '0 auto 40px' }} 
      />

      {currentStep === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
            <Card className="bg-slate-50 border-none">
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
                    <Text type="secondary">OCR 解析引擎状态: </Text>
                    {ocrStatus ? (
                        <Space>
                            <Badge status={ocrStatus.success ? 'success' : 'error'} />
                            <Text type={ocrStatus.success ? 'success' : 'danger'}>
                                {ocrStatus.success ? `已就绪 (${ocrStatus.version || '本地 OCR'})` : ocrStatus.message}
                            </Text>
                        </Space>
                    ) : (
                        <Space>
                            <SyncOutlined spin />
                            <Text type="secondary">正在探测中...</Text>
                        </Space>
                    )}
                </div>
                <Space size={32} align="start" style={{ width: '100%' }}>
                    <div style={{ flex: 1 }}>
                        <Text strong style={{ display: 'block', marginBottom: 12 }}>业务归口</Text>
                        <div style={{ display: 'flex', gap: 12 }}>
                            {[
                                { k: 'doc', l: '证照资质', i: <SafetyCertificateOutlined /> },
                                { k: 'tech', l: '标书知识', i: <FileTextOutlined /> },
                                { k: 'excel', l: '结构化表', i: <TableOutlined /> }
                            ].map(item => (
                                <Card 
                                    key={item.k}
                                    size="small" 
                                    hoverable 
                                    className={importType === item.k ? 'border-primary bg-primary-50' : ''}
                                    onClick={() => setImportType(item.k as any)}
                                    style={{ width: 140, textAlign: 'center', cursor: 'pointer', borderColor: importType === item.k ? '#1890ff' : undefined }}
                                >
                                    <div style={{ fontSize: 24, marginBottom: 8, color: importType === item.k ? '#1890ff' : undefined }}>{item.i}</div>
                                    <div>{item.l}</div>
                                </Card>
                            ))}
                        </div>
                    </div>
                    
                    <div style={{ width: 300 }}>
                        <Text strong style={{ display: 'block', marginBottom: 12 }}>关联项目 (可选)</Text>
                        <Select 
                            placeholder="请选择所属投标项目" 
                            style={{ width: '100%' }}
                            allowClear
                            showSearch
                            optionFilterProp="children"
                            onChange={v => setSelectedProjectId(v)}
                        >
                            {projects.map(p => (
                                <Select.Option key={p.id} value={p.id}>{p.name}</Select.Option>
                            ))}
                        </Select>
                    </div>
                </Space>
            </Card>

            <div style={{ 
                padding: '60px 0', 
                textAlign: 'center', 
                backgroundColor: '#fff', 
                border: '2px dashed #e2e8f0', 
                borderRadius: 16 
            }}>
                <Upload.Dragger
                    beforeUpload={(file) => {
                        setUploadFileList(prev => [...prev, file]);
                        return false;
                    }}
                    multiple
                    showUploadList
                    accept=".pdf,.jpg,.jpeg,.png,.xlsx,.xls"
                    style={{ border: 'none', background: 'transparent' }}
                >
                    <div style={{ marginBottom: 20 }}>
                        <CloudUploadOutlined style={{ fontSize: 48, color: '#1890ff' }} />
                    </div>
                    <Title level={4}>点击或拖拽附件到此处</Title>
                    <Text type="secondary">支持 PDF、图片或 Excel。上传后系统将自动按照队列执行 OCR 预处理。</Text>
                </Upload.Dragger>
            </div>

            <div style={{ textAlign: 'center' }}>
                <Button 
                    type="primary" 
                    size="large" 
                    icon={<RocketOutlined />}
                    disabled={uploadFileList.length === 0}
                    loading={loading}
                    onClick={handleDocBatchUpload}
                    style={{ width: 220, height: 48, borderRadius: 8 }}
                >
                    提交并开始识别
                </Button>
            </div>
        </div>
      )}

      {currentStep === 1 && (
        <div style={{ padding: '60px 0', textAlign: 'center' }}>
            <div style={{ maxWidth: 460, margin: '0 auto' }}>
                <Progress percent={currentTask?.progress || 0} status="active" />
                <div style={{ marginTop: 24 }}>
                    <Title level={4}>AI 正在深度解析文件...</Title>
                    <Text type="secondary">{currentTask?.message || '正在执行 OCR 识别与语义分析'}</Text>
                </div>
                <Button 
                    style={{ marginTop: 40 }} 
                    type="primary" 
                    icon={<AuditOutlined />}
                    onClick={() => setActiveTab('audit')}
                >
                    跳过等待，去审核台
                </Button>
            </div>
        </div>
      )}
    </div>
  );

  const renderAudit = () => (
    <div style={{ padding: '0 0 24px 0' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
            <Badge count={auditData.length} offset={[10, 0]} color="#1890ff">
                <Text strong style={{ fontSize: 16 }}>待审核识别任务</Text>
            </Badge>
            <Button icon={<SyncOutlined />} onClick={fetchAuditData} loading={auditLoading}>刷新状态</Button>
        </div>

        <Card styles={{ body: { padding: 0 } }} bordered={false} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
        <Table
          columns={[
            {
              title: '文件',
              dataIndex: 'file_name',
              key: 'file_name',
              render: (text: string, record: AuditItem) => (
                <Space>
                  {record.mime_type?.includes('pdf') ? <FilePdfOutlined style={{ color: '#ff4d4f' }} /> : <FileImageOutlined style={{ color: '#1890ff' }} />}
                  <Link onClick={() => navigate(`/audits/${record.id}`)}>{text || '未知文档'}</Link>
                </Space>
              ),
            },
            {
              title: '状态',
              dataIndex: 'audit_status',
              key: 'status',
              render: (st: string) => renderStatusTag(st)
            },
            {
              title: '推荐入库',
              dataIndex: 'object_type',
              key: 'object_type',
              render: (type: string) => {
                const types: Record<string, string> = {
                  'performance': '项目业绩',
                  'person': '人员档案',
                  'qualification': '资质证书',
                  'honor': '荣誉奖项',
                  'method': '工法亮点',
                };
                return <Tag color="blue">{types[type] || type}</Tag>;
              }
            },
            {
              title: '置信度',
              dataIndex: 'confidence_score',
              width: 140,
              render: (score: number) => (
                <div style={{ width: 100 }}>
                  <Progress size="small" percent={Math.round((score || 0.8) * 100)} status={(score || 0.8) > 0.6 ? 'active' : 'exception'} />
                </div>
              )
            },
            {
              title: '风险等级',
              dataIndex: 'risk_level',
              key: 'risk',
              render: (level: string) => (
                <Tag color={level === 'high' ? 'red' : level === 'medium' ? 'orange' : 'green'}>
                  {level === 'high' ? '高风险' : level === 'medium' ? '中风险' : '低风险'}
                </Tag>
              )
            },
            {
              title: '操作',
              render: (_: any, record: AuditItem) => (
                <Space>
                  <Button type="primary" size="small" onClick={() => navigate(`/audits/${record.id}`)}>极速审核</Button>
                  <Button size="small" onClick={() => onIgnoreAudit(record.id)}>忽略</Button>
                </Space>
              ),
            },
          ]}
          dataSource={auditData}
          loading={auditLoading}
          rowKey="id"
          size="middle"
        />
        </Card>
    </div>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {!independentTab && (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
            <Title level={2} style={{ margin: 0 }}>
                <FolderOpenOutlined style={{ marginRight: 12, color: '#1890ff' }} /> 文件库
            </Title>
            <Text type="secondary">集成 OCR/AI 解析的全生命周期文件资产门户。</Text>
            </div>
            {activeTab !== 'import' && (
                <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setActiveTab('import')}>
                    上传文件
                </Button>
            )}
        </div>
      )}

      {independentTab === 'repository' && (
        <input 
            type="file" 
            multiple 
            ref={fileInputRef} 
            style={{ display: 'none' }} 
            onChange={handleQuickUpload} 
            accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg"
        />
      )}

      <div style={{ padding: '0 0 24px 0' }}>
          {activeTab === 'repository' ? renderRepository() : 
           activeTab === 'import' ? renderImport() : 
           activeTab === 'audit' ? renderAudit() : 
           renderRepository()}
      </div>

      {/* Drawer: Detail */}
      <Drawer
        title={<Space><FileTextOutlined /> 文件检查: {selectedFile?.file_name}</Space>}
        width={720}
        onClose={() => setDetailVisible(false)}
        open={detailVisible}
      >
        {selectedFile && (
           <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
              <Card size="small" title="AI 分析摘要">
                 {selectedFile.markdown_text ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{selectedFile.markdown_text}</ReactMarkdown> : <Empty description="暂无分析结果" />}
              </Card>
              <Card size="small" title="OCR 原始文本">
                 <Input.TextArea value={selectedFile.plain_text || ''} rows={15} readOnly style={{ fontFamily: 'monospace' }} />
              </Card>
           </div>
        )}
      </Drawer>

      {/* Modal: Preview */}
      <Modal
        title={<span>文件预览: {previewFile?.file_name}</span>}
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        footer={null}
        width="80vw"
        styles={{ body: { height: '70vh', padding: 0 } }}
      >
        {previewFile && (
          previewFile.file_name?.toLowerCase().endsWith('.pdf') ? (
            <div style={{ width: '100%', height: '100%', background: '#fff' }}>
              {pdfPreviewLoading ? (
                <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Space>
                    <SyncOutlined spin />
                    <span>正在加载 PDF…</span>
                  </Space>
                </div>
              ) : pdfPreviewUrl ? (
                <iframe
                  key={previewFile.id}
                  src={pdfPreviewUrl}
                  style={{ width: '100%', height: '100%', border: 'none', background: '#fff' }}
                  title="PDF Preview"
                />
              ) : (
                <div style={{ padding: 16 }}>
                  <p style={{ marginBottom: 8 }}>{pdfPreviewError || '当前环境无法预览 PDF。'}</p>
                  <a href={`/api/files/download/${previewFile.id}`} target="_blank" rel="noreferrer">
                    点击在新窗口打开
                  </a>
                </div>
              )}
            </div>
          ) : previewFile.file_name?.match(/\.(jpg|jpeg|png|gif|webp)$/i) ? (
            <div style={{ width: '100%', height: '100%', background: '#f5f5f5', overflow: 'auto' }}>
                <Image
                  src={`/api/files/download/${previewFile.id}`}
                  alt="Preview"
                  style={{ width: '100%', display: 'block' }}
                  preview={false}
                />
            </div>
          ) : (
            <iframe 
              src={`/api/files/download/${previewFile.id}`} 
              style={{ width: '100%', height: '100%', border: 'none' }} 
              title="Preview"
            />
          )
        )}
      </Modal>
      <LibraryDetailModal 
        visible={libDetailVisible} 
        onCancel={() => setLibDetailVisible(false)}
        targetType={libDetailType}
        targetId={libDetailId}
      />
    </div>
  );
};

export default FileCenter;
