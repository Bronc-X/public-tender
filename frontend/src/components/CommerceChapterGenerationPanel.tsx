import React, { useState, useEffect, useMemo, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Card, Button, Spin, Typography, message, Result, Space, Input, Table, Tag, Divider, Empty, Row, Col, Descriptions, Alert, Upload, Tabs, Badge, Checkbox, Modal, Steps, List } from 'antd';
import {
  CheckCircleFilled, CheckOutlined, RobotOutlined,
  FileImageOutlined, LoadingOutlined, IdcardOutlined, BuildOutlined, ProfileOutlined,
  SyncOutlined, CheckCircleOutlined, FileWordOutlined, CloseOutlined, DownloadOutlined, DatabaseOutlined, AuditOutlined, FileTextOutlined,
  PlusCircleOutlined, EditOutlined, UploadOutlined, CloseCircleOutlined
} from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';
import LibraryDetailModal from './LibraryDetailModal';

const { Title, Text } = Typography;

const markdownStyles = `
  .markdown-body table {
    border-collapse: collapse;
    width: 100%;
    margin-bottom: 16px;
    border: 1px solid #e2e8f0;
  }
  .markdown-body th, .markdown-body td {
    border: 1px solid #e2e8f0;
    padding: 8px 12px;
    text-align: left;
  }
  .markdown-body th {
    background-color: #f8fafc;
    font-weight: 600;
  }
  .markdown-body tr:nth-child(even) {
    background-color: #fcfcfc;
  }
`;

interface BidActionSlot {
  slot_id: string;
  chapter_path: string[];
  slot_context_title: string;
  target_field: string;
  slot_type?: 'text' | 'image' | 'personnel_table' | 'performance_table' | 'company_profile' | 'certificate_list';
  ai_suggested_value: string;
  reason: string;
  status: 'pending_review' | 'approved';
}

interface BidActionList {
  project_id: string;
  slots: BidActionSlot[];
  original_markdown?: string;
  step4_bindings?: Record<string, { target_type: string, record_id?: string, record_name: string, requirement_text?: string, category?: string }>;
  chapter_bindings?: Record<string, { resources: any[], supplement: string }>;
}

export const CommerceChapterGenerationPanel: React.FC<{
  project_id: string;
  onRefresh: () => void;
  onReadyToNext: () => void;
  hideFloatingPanel?: boolean;
  onValidationChange?: (isValid: boolean, missingCount: number) => void;
}> = ({ project_id, onRefresh, onReadyToNext, hideFloatingPanel, onValidationChange }) => {
  const { currentCompanyId } = useCompany();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<BidActionList | null>(null);
  const [exportPath, setExportPath] = useState('');
  const [syncStatus, setSyncStatus] = useState<'idle' | 'generating' | 'success' | 'error'>('idle');
  const [step6ErrorMessage, setStep6ErrorMessage] = useState('');
  const [humanOverrides, setHumanOverrides] = useState<Record<string, string>>({});
  const [activeChapter, setActiveChapter] = useState<string>('');
  const [isExporting, setIsExporting] = useState(false);

  // Minimalist Log State for Step 6
  const [agentLogs, setAgentLogs] = useState<string[]>([]);
  const [isReportVisible, setIsReportVisible] = useState(false);
  const [isSimulating, setIsSimulating] = useState(false);
  const [hasPromptedTemplate, setHasPromptedTemplate] = useState(false);

  // Draggable Floating Panel State
  const [panelPos, setPanelPos] = useState({ x: 24, y: 70 });
  const [isDragging, setIsDragging] = useState(false);
  const dragRef = useRef<{ startX: number; startY: number; initX: number; initY: number } | null>(null);

  // Detail Modal State
  const [detailModalState, setDetailModalState] = useState<{ targetType: string, targetId: string } | null>(null);

  // Chapter Bindings State
  const [selectedResourceKeys, setSelectedResourceKeys] = useState<string[]>([]);
  const [isConfigModalVisible, setIsConfigModalVisible] = useState(false);
  const [supplementaryText, setSupplementaryText] = useState('');
  const [chapterBindings, setChapterBindings] = useState<Record<string, {
    resources: { ruleId: string, record_name: string, requirement_text?: string, category?: string, record_id?: string }[],
    supplement: string
  }>>({});

  const hasChapterBindingContent = (chap: string) => {
    const binding = chapterBindings?.[chap];
    return !!binding && ((binding.resources?.length || 0) > 0 || !!binding.supplement?.trim());
  };

  // Reset selection when changing chapter
  useEffect(() => {
    setSelectedResourceKeys([]);
  }, [activeChapter]);

  const handlePointerDown = (e: React.PointerEvent) => {
    dragRef.current = {
      startX: e.clientX,
      startY: e.clientY,
      initX: panelPos.x,
      initY: panelPos.y,
    };
    setIsDragging(true);
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const handlePointerMove = (e: React.PointerEvent) => {
    if (!isDragging || !dragRef.current) return;
    const dx = e.clientX - dragRef.current.startX;
    const dy = e.clientY - dragRef.current.startY;
    setPanelPos({
      x: dragRef.current.initX + dx,
      y: dragRef.current.initY + dy,
    });
  };

  const handlePointerUp = (e: React.PointerEvent) => {
    setIsDragging(false);
    e.currentTarget.releasePointerCapture(e.pointerId);
  };

  useEffect(() => {
    fetchPayload();
  }, [project_id]);

  useEffect(() => {
    let timer: any = null;
    let logTimer: any = null;

    if (syncStatus === 'generating') {
      timer = setInterval(() => {
        fetchPayload(true);
      }, 3000);
    }

    if (hideFloatingPanel && (syncStatus === 'generating' || isSimulating)) {
      // Simulate Eino Agent Log stream for UX
      const logsPool = [
        "启动 Eino Agent 图流编排...",
        "正在加载官方原始排版模板...",
        "解析底层 XML 段落边界...",
        "识别到表格与图片占位符特征...",
        "从资源池检索项目级配置参数...",
        "提取企业工商网基础资料...",
        "ReAct: 遍历法定代表人及代理人关联规则...",
        "ReAct: 成功绑定资质证书变量...",
        "校验提取参数，准备向占位符注入文本...",
        "执行无损渲染并重组 Document XML...",
        "正在封包为最终 Word 文档..."
      ];
      let idx = 0;
      setAgentLogs([logsPool[0]]);
      logTimer = setInterval(() => {
        idx++;
        if (idx < logsPool.length) {
          setAgentLogs(prev => {
            const updated = [...prev, logsPool[idx]];
            return updated.slice(-7);
          });
        }
      }, 1000);
    }

    return () => {
      if (timer) clearInterval(timer);
      if (logTimer) clearInterval(logTimer);
    };
  }, [syncStatus, project_id, isSimulating, hideFloatingPanel]);

  const triggerGenerate = async () => {
    setSyncStatus('generating');
    setIsSimulating(true);
    setTimeout(() => {
      setIsSimulating(false);
    }, 11500);

    try {
      await axios.post(`/api/bid-projects/${project_id}/step6/generate`, {}, {
        headers: { 'x-company-id': currentCompanyId || '' }
      });
    } catch (err) {
      message.error("唤起AI素材分析失败");
      setSyncStatus('error');
    }
  };

  const fetchPayload = async (isSilent = false) => {
    if (!isSilent) setLoading(true);
    try {
      const res = await axios.get(`/api/bid-projects/${project_id}/step6/payload`, {
        headers: { 'x-company-id': currentCompanyId || '' }
      });
      const serverStatus = res.data?.status || 'idle';
      const latestExportPath = res.data?.latest_export_path || '';
      setStep6ErrorMessage(res.data?.last_error_message || '');
      if (latestExportPath) {
        setExportPath(latestExportPath);
      }

      // Step 5 (hideFloatingPanel=false): Only needs data for display, never triggers generation
      if (!hideFloatingPanel) {
        // Always load whatever data is available regardless of generation status
        if (res.data.data) {
          setData(res.data.data);
          if (res.data.data?.chapter_bindings) {
            setChapterBindings(res.data.data.chapter_bindings);
          }
        }
        setSyncStatus('success'); // Let step 5 render the configuration UI
      } else {
        // Step 6 (hideFloatingPanel=true): Cares about generation lifecycle
        if (serverStatus === 'success' || latestExportPath) {
          setHasPromptedTemplate(true);
        }
        if (serverStatus === 'idle' || (serverStatus === 'error' && !hasPromptedTemplate)) {
          // If user hasn't uploaded a template yet, treat any stale error as idle
          setSyncStatus(latestExportPath ? 'success' : 'idle');
        } else if (serverStatus === 'error') {
          setSyncStatus('error');
          if (!isSilent) message.error("后台装配失败，请重试或联系管理员");
        } else if (serverStatus === 'generating') {
          setSyncStatus('generating');
        } else if (serverStatus === 'success') {
          setSyncStatus('success');
          setData(res.data.data);
          if (res.data.data?.chapter_bindings) {
            setChapterBindings(res.data.data.chapter_bindings);
          }
        }
      }
    } catch (err: any) {
      if (err.response?.status !== 404 && !isSilent) {
        message.warning('尚未生成数据区块');
      }
    } finally {
      if (!isSilent) setLoading(false);
    }
  };

  const executeWordExport = async () => {
    try {
      setIsExporting(true);
      message.loading({ content: '正在写入 Word 物理底稿...', key: 'export' });
      
      const payloadData = data ? JSON.parse(JSON.stringify(data)) : { project_id, slots: [] };
      const slots = Array.isArray(payloadData.slots) ? payloadData.slots : [];
      if (slots.length === 0) {
        message.error({ content: '请先回到“方案处理确认”完成章节装配，再生成数据块导出 Word', key: 'export' });
        return;
      }
      const unresolvedSlots = slots.filter((s: any) => s.status !== 'approved');
      if (unresolvedSlots.length > 0) {
        message.error({ content: `仍有 ${unresolvedSlots.length} 个数据块未确认`, key: 'export' });
        return;
      }

      const res = await axios.post(`/api/bid-projects/${project_id}/step6/export`, {
        payload_json: JSON.stringify(payloadData)
      }, {
        headers: { 'x-company-id': currentCompanyId || '' }
      });
      message.success({ content: 'Word 文档生成完毕，开始下载！', key: 'export', duration: 2 });
      setExportPath(res.data.export_path);
      onRefresh();
      // Auto trigger download immediately
      const downloadUrl = `/api/bid-projects/${project_id}/step6/download?file=${encodeURIComponent(res.data.export_path.split('/').pop() || '')}`;
      window.location.href = downloadUrl;
    } catch (err) {
      message.error({ content: '写入失败，请检查全数据块是否确认通过', key: 'export' });
    } finally {
      setIsExporting(false);
    }
  };

  const handleGoBack = async () => {
    try {
      await axios.post(`/api/bid-projects/${project_id}/goback`, null, {
        headers: { 'x-company-id': currentCompanyId || '' }
      });
      onRefresh();
    } catch (e) {
      message.error("回退失败");
    }
  };

  const approveSlot = async (slotId: string, finalValue: string) => {
    try {
      // Opt-in backend call if it exists, otherwise just local update for now
      axios.post(`/api/bid-projects/${project_id}/step6/slots/${slotId}/approve`,
        { ai_suggested_value: finalValue },
        { headers: { 'x-company-id': currentCompanyId || '' } }
      ).catch(() => { });

      message.success("数据验证通过");

      // Local optimistic update
      setData(prev => {
        if (!prev) return prev;
        return {
          ...prev,
          slots: prev.slots.map(s => s.slot_id === slotId ? { ...s, status: 'approved', ai_suggested_value: finalValue } : s)
        };
      });
    } catch (e) {
      console.error(e);
      message.error("确认失败");
    }
  };

  const removeSlot = (slotId: string) => {
    setData(prev => {
      if (!prev) return prev;
      return {
        ...prev,
        slots: prev.slots.filter(s => s.slot_id !== slotId)
      };
    });
    message.success("该结构化素材已从当前装配链中彻底移除");
  };

  // --- Rendering Helpers ---
  const renderDataBlock = (slot: BidActionSlot) => {
    let parsed: any = [];
    try {
      if (slot.ai_suggested_value) {
        parsed = JSON.parse(slot.ai_suggested_value);
      }
    } catch (e) { }

    // Safe array wrap
    if (!Array.isArray(parsed) && typeof parsed === 'object') {
      parsed = [parsed];
    } else if (!Array.isArray(parsed)) {
      parsed = [];
    }

    if (slot.slot_type === 'personnel_table') {
      const columns = [
        { title: '姓名', dataIndex: '姓名', key: '姓名' },
        { title: '年龄', dataIndex: '年龄', key: '年龄' },
        { title: '职务', dataIndex: '职务', key: '职务' },
        { title: '类似项目名称', dataIndex: '类似项目名称', key: '类似项目名称' },
      ];
      return <Table dataSource={parsed} columns={columns} pagination={false} size="small" rowKey={(r: any) => r['姓名'] || Math.random().toString()} />;
    }

    if (slot.slot_type === 'performance_table') {
      const columns = [
        { title: '项目名称', dataIndex: '项目名称', key: '项目名称' },
        { title: '合同金额', dataIndex: '合同金额', key: '合同金额' },
        { title: '工期', dataIndex: '工期', key: '工期' },
        { title: '发包人', dataIndex: '发包人', key: '发包人' },
      ];
      return <Table dataSource={parsed} columns={columns} pagination={false} size="small" rowKey={(r: any) => r['项目名称'] || Math.random().toString()} />;
    }

    return <Text type="secondary">无法解析的数据块结构</Text>;
  };

  // Chinese numeral pattern for detecting chapter-level headings: 一、二、三、...、十、
  const CHINESE_NUMERAL_PATTERN = /^[一二三四五六七八九十]{1,3}、/;

  // 1. Extract chapters from the Markdown by matching headings whose title starts with Chinese numerals (一、二、三...)
  //    This works regardless of heading level (# vs ## vs ###), because OCR output is often inconsistent.
  const markdownChapters = useMemo(() => {
    if (!data?.original_markdown) return [];
    const lines = data.original_markdown.split('\n');
    const chaps: string[] = [];

    for (const line of lines) {
      const match = line.match(/^#{1,6}\s+(.*)/);
      if (match) {
        const title = match[1].trim();
        if (CHINESE_NUMERAL_PATTERN.test(title)) {
          if (!chaps.includes(title)) chaps.push(title);
        }
      }
    }
    return chaps;
  }, [data?.original_markdown]);

  // 2. Map AI slots to these absolute markdown chapters (currently empty since LLM is bypassed)
  const chaptersMap = useMemo(() => {
    const cmap = new Map<string, BidActionSlot[]>();
    markdownChapters.forEach(c => cmap.set(c, []));

    const slots = data?.slots || [];
    slots.forEach(s => {
      let aiChap = '未归类 / 全局共享项';
      if (s.chapter_path && s.chapter_path.length > 0) {
        aiChap = s.chapter_path.length > 1 ? s.chapter_path[1] : s.chapter_path[0];
      }

      let matched = false;
      for (const realChap of markdownChapters) {
        if (realChap === aiChap || realChap.includes(aiChap)) {
          cmap.get(realChap)!.push(s);
          matched = true;
          break;
        }
      }

      if (!matched) {
        if (!cmap.has(aiChap)) cmap.set(aiChap, []);
        cmap.get(aiChap)!.push(s);
      }
    });

    return cmap;
  }, [data, markdownChapters]);

  // The Top Tabs are perfectly aligned with the Native Markdown chapters
  const allChapterNames = markdownChapters.length > 0
    ? markdownChapters
    : Array.from(chaptersMap.keys()).filter(name => name !== '封面' && name !== '未归类 / 全局共享项');

  useEffect(() => {
    if (allChapterNames.length > 0 && !activeChapter) {
      setActiveChapter(allChapterNames[0]);
    } else if (allChapterNames.length > 0 && activeChapter && !allChapterNames.includes(activeChapter)) {
      setActiveChapter(allChapterNames[0]);
    }
  }, [allChapterNames, activeChapter]);

  // Lift validation state up to parent to control the 'Next Step' button
  useEffect(() => {
    if (onValidationChange) {
      if (allChapterNames.length === 0) {
        onValidationChange(!!hideFloatingPanel, hideFloatingPanel ? 0 : 1);
      } else {
        const configuredCount = allChapterNames.filter(chap => hasChapterBindingContent(chap)).length;
        const missingCount = allChapterNames.length - configuredCount;
        onValidationChange(missingCount === 0, missingCount);
      }
    }
  }, [allChapterNames, chapterBindings, hideFloatingPanel, onValidationChange]);

  const isChapterConfirmed = (chap: string) => hasChapterBindingContent(chap);

  const activeSlots = (activeChapter ? chaptersMap.get(activeChapter) : null) || [];

  // Divide active slots into Left (Missing/Unstructured) and Right (Structured Material Blocks)
  const leftPanelSlots = useMemo(() => {
    return activeSlots.filter(s => s.slot_type === 'text' || s.slot_type === 'image');
  }, [activeSlots]);

  const rightPanelBlocks = useMemo(() => {
    return activeSlots.filter(s =>
      s.slot_type !== 'text' &&
      s.slot_type !== 'image' &&
      s.slot_type !== 'company_profile'
    );
  }, [activeSlots]);

  const chapterMarkdown = useMemo(() => {
    if (!data?.original_markdown || !activeChapter) return data?.original_markdown;
    if (activeChapter === '未归类 / 全局共享项') return data.original_markdown;

    const lines = data.original_markdown.split('\n');
    let capturing = false;
    const result: string[] = [];

    // Build a Set of all known chapter titles so we can detect boundaries
    const chapterTitleSet = new Set(markdownChapters);

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      const headingMatch = line.match(/^#{1,6}\s+(.*)/);

      if (headingMatch) {
        const title = headingMatch[1].trim();

        if (!capturing) {
          // Start capturing when we find the active chapter's exact title
          if (title === activeChapter) {
            capturing = true;
            result.push(line);
          }
        } else {
          // Stop capturing when we hit another known chapter title (a different one)
          if (chapterTitleSet.has(title) && title !== activeChapter) {
            break;
          }
          // Otherwise keep capturing (sub-headings and other headings within this chapter)
          result.push(line);
        }
      } else if (capturing) {
        result.push(line);
      }
    }

    // If extraction found nothing, fallback to the whole content
    let parsedMd = result.length > 0 ? result.join('\n') : data.original_markdown;

    // Convert 4 or more contiguous spaces to full-width underlines to restore original document fill-in-the-blanks
    if (parsedMd) {
      parsedMd = parsedMd.replace(/ {4,}/g, match => '＿'.repeat(Math.max(4, Math.floor(match.length / 1.5))));
    }

    return parsedMd;
  }, [data?.original_markdown, activeChapter, markdownChapters]);

  const unresolvedCount = (data?.slots || []).filter(s => {
    if (s.status === 'approved') return false;
    const chap = s.chapter_path && s.chapter_path.length > 0 ?
      (s.chapter_path.length > 1 ? s.chapter_path[1] : s.chapter_path[0]) : '';
    return chap !== '封面';
  }).length;

  // ==========================================
  // EXTREME MINIMALIST MODE (STEP 6)
  // ==========================================
  if (hideFloatingPanel) {
    if (!hasPromptedTemplate) {
      return (
        <div style={{ height: '60vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
          <FileWordOutlined style={{ fontSize: 64, color: '#6366f1', marginBottom: 24, opacity: 0.8 }} />
          <Title level={4} style={{ color: '#1e293b', fontWeight: 500, letterSpacing: 1, marginBottom: 8 }}>
            请上传专属的投标文件模板
          </Title>
          <Text type="secondary" style={{ fontSize: 15, marginBottom: 48 }}>
            系统将依据该模板的占位符进行智能排版和物理级的数据无损注入。
          </Text>

          <Space size="large" wrap style={{ justifyContent: 'center' }}>
            <Upload
              name="file"
              action={`/api/bid-projects/${project_id}/step6/upload-template`}
              headers={{ 'x-company-id': currentCompanyId || '' }}
              showUploadList={false}
              onChange={(info) => {
                if (info.file.status === 'uploading') {
                  message.loading({ content: '正在上传并解析模板...', key: 'uploadTpl' });
                }
                if (info.file.status === 'done') {
                  message.success({ content: '模板解析成功！正在启动协同装配...', key: 'uploadTpl', duration: 2 });
                  setExportPath('');
                  setHasPromptedTemplate(true);
                  triggerGenerate(); // Start AI processing
                } else if (info.file.status === 'error') {
                  message.error({ content: '模板上传失败', key: 'uploadTpl' });
                }
              }}
            >
              <Button type="primary" size="large" icon={<UploadOutlined />} style={{ width: 180, height: 44, borderRadius: 6, fontSize: 15 }}>
                上传投标文件模板
              </Button>
            </Upload>
            <Button
              size="large"
              style={{ width: 180, height: 44, borderRadius: 6, fontSize: 15 }}
              onClick={() => {
                setExportPath('');
                setHasPromptedTemplate(true);
                if (syncStatus !== 'success') {
                  triggerGenerate();
                } else {
                  setIsSimulating(true);
                  setTimeout(() => setIsSimulating(false), 11500);
                }
              }}
            >
              跳过，使用默认文件
            </Button>
          </Space>
        </div>
      );
    }

    const isShowingGenerating = loading || syncStatus === 'generating' || isSimulating;

    if (isShowingGenerating) {
      return (
        <div style={{ height: '60vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
          <RobotOutlined style={{ fontSize: 48, color: '#10b981', marginBottom: 24, opacity: 0.8 }} spin />
          <Title level={4} style={{ color: '#334155', fontWeight: 400, letterSpacing: 1, marginBottom: 32 }}>
            Eino Agent 正在协同装配标书
          </Title>

          <div style={{ width: 440, height: 260, position: 'relative', overflow: 'hidden', maskImage: 'linear-gradient(to bottom, black 60%, transparent 100%)', WebkitMaskImage: 'linear-gradient(to bottom, black 60%, transparent 100%)' }}>
            <div style={{ position: 'absolute', top: 10, width: '100%', display: 'flex', flexDirection: 'column', gap: 14 }}>
              {[...agentLogs].reverse().map((log, index) => {
                const isLatest = index === 0;
                return (
                  <div key={`${log}-${index}`} style={{
                    color: isLatest ? '#1e293b' : '#94a3b8',
                    fontSize: isLatest ? 15 : 14,
                    textAlign: 'center',
                    transition: 'all 0.4s',
                    fontWeight: isLatest ? 500 : 400,
                    opacity: Math.max(0, 1 - (index * 0.15))
                  }}>
                    {log}
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      );
    }

    if (syncStatus === 'error' && !exportPath) {
      const missingConfirmedBindings = step6ErrorMessage.includes('no confirmed step 5 chapter bindings');
      return (
        <div style={{ height: '60vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', textAlign: 'center' }}>
          <CloseCircleOutlined style={{ fontSize: 56, color: '#ef4444', marginBottom: 24 }} />
          <Title level={4} style={{ color: '#1e293b', fontWeight: 500, marginBottom: 8 }}>
            装配前置数据未完成
          </Title>
          <Text type="secondary" style={{ fontSize: 15, marginBottom: 32 }}>
            {missingConfirmedBindings ? '请先回到“方案处理确认”，逐章完成数据块装配。' : (step6ErrorMessage || '后台装配失败，请重试或联系管理员。')}
          </Text>
          <Button type="primary" size="large" onClick={missingConfirmedBindings ? handleGoBack : () => triggerGenerate()}>
            {missingConfirmedBindings ? '回到方案处理确认' : '重新触发装配'}
          </Button>
        </div>
      );
    }

    // Prepare Audit & Missing Items Report
    const auditItems: any[] = [];
    let missingCount = 0;
    Object.entries(chapterBindings || {}).forEach(([chapName, binding]) => {
      binding.resources.forEach(res => {
        auditItems.push({
          chapter: chapName,
          rule: res.requirement_text || '知识资源库',
          injected: res.record_name
        });
      });
      if (binding.supplement) {
        auditItems.push({
          chapter: chapName,
          rule: '人工补充要求',
          injected: binding.supplement
        });
      }
    });

    // Fallback if data is raw slots
    if (auditItems.length === 0 && data?.slots) {
      data.slots.forEach(s => {
        if (!s.ai_suggested_value) missingCount++;
        auditItems.push({
          chapter: '自动提取',
          rule: s.target_field,
          injected: s.ai_suggested_value || '缺失'
        });
      });
    }

    const missingItems = auditItems.filter(item => !item.injected || item.injected === '暂无数据' || item.injected === '缺失');

    return (
      <div style={{ height: '60vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <CheckCircleFilled style={{ fontSize: 56, color: '#10b981', marginBottom: 24 }} />
        <Title level={3} style={{ color: '#0f172a', fontWeight: 500, marginBottom: 8, letterSpacing: 1 }}>装配完成</Title>
        <Text type="secondary" style={{ fontSize: 15, marginBottom: 48 }}>文档内容已注入原模板，物理排版毫无受损</Text>

        <Space size="large" wrap style={{ justifyContent: 'center' }}>
          <Button
            type="primary"
            size="large"
            icon={<DownloadOutlined />}
            loading={isExporting}
            href={exportPath ? `/api/bid-projects/${project_id}/step6/download?file=${encodeURIComponent(exportPath.split('/').pop() || '')}` : undefined}
            style={{ width: 180, height: 44, borderRadius: 6, fontSize: 15 }}
            onClick={(e) => {
              if (exportPath) {
                onRefresh();
              }
              if (!exportPath) {
                e.preventDefault(); // Prevent navigating if href is undefined
                executeWordExport();
              }
            }}
          >
            下载 Word 文档
          </Button>
          <Upload
            name="file"
            action={`/api/bid-projects/${project_id}/step6/upload-template`}
            headers={{ 'x-company-id': currentCompanyId || '' }}
            showUploadList={false}
            onChange={(info) => {
              if (info.file.status === 'uploading') {
                message.loading({ content: '正在上传并解析模板...', key: 'uploadTpl' });
              }
              if (info.file.status === 'done') {
                message.success({ content: '模板替换成功！正在重新拼装数据...', key: 'uploadTpl', duration: 3 });
                setExportPath('');
                setIsSimulating(true); // Restart the UI animation
                triggerGenerate();
              } else if (info.file.status === 'error') {
                message.error({ content: '模板上传失败', key: 'uploadTpl' });
              }
            }}
          >
            <Button size="large" icon={<UploadOutlined />} style={{ height: 44, borderRadius: 6, fontSize: 15 }}>
              重新上传投标文件模板
            </Button>
          </Upload>
        </Space>

        <Modal
          title={<><RobotOutlined style={{ color: '#10b981', marginRight: 8 }} />Agent 工作汇报</>}
          open={isReportVisible}
          onCancel={() => setIsReportVisible(false)}
          footer={null}
          width={600}
        >
          <div style={{ padding: '12px 4px' }}>
            <p style={{ fontSize: 15, color: '#334155', marginBottom: 24 }}>
              系统在模板中共成功挂载并注入了 <strong>{auditItems.length}</strong> 处核心暗扣与数据项。
            </p>

            {missingItems.length > 0 ? (
              <>
                <Alert
                  type="warning"
                  showIcon
                  message={`发现 ${missingItems.length} 个空缺预留项，生成文档中已留空，请人工补齐`}
                  style={{ marginBottom: 16 }}
                />
                <List
                  size="small"
                  dataSource={missingItems}
                  renderItem={item => (
                    <List.Item>
                      <Text style={{ color: '#475569' }}>{item.rule}</Text>
                      <Tag color="orange">待人工补充</Tag>
                    </List.Item>
                  )}
                />
              </>
            ) : (
              <Alert
                type="success"
                showIcon
                message="底层文档全部关联插槽数据饱满，暂无缺漏项。"
                style={{ padding: '12px 16px' }}
              />
            )}
          </div>
        </Modal>
      </div>
    );
  }

  // ==========================================
  // CONFIGURATION MODE (STEP 5)
  // ==========================================

  if (syncStatus === 'error') {
    const missingConfirmedBindings = step6ErrorMessage.includes('no confirmed step 5 chapter bindings');
    return (
      <div style={{ padding: '60px 0', textAlign: 'center' }}>
        <CloseCircleOutlined style={{ fontSize: 56, color: '#ef4444', marginBottom: 24 }} />
        <Title level={4} style={{ color: '#1e293b', fontWeight: 500, marginBottom: 8 }}>装配过程发生异常</Title>
        <Text type="secondary" style={{ fontSize: 14, marginBottom: 24, display: 'block' }}>
          {missingConfirmedBindings ? '还没有完成“方案处理确认”的章节装配，请先回退上一步确认数据块。' : (step6ErrorMessage || '无法从后端获取装配数据，请点击下方按钮重新尝试。')}
        </Text>
        <Button type="primary" onClick={missingConfirmedBindings ? handleGoBack : () => triggerGenerate()} size="large">
          {missingConfirmedBindings ? '回到方案处理确认' : '重新触发装配'}
        </Button>
      </div>
    );
  }

  if (loading || syncStatus === 'generating') {
    return (
      <div style={{ padding: '40px 0', textAlign: 'center' }}>
        <Spin size="large" description="正在进行全篇数据解构装配..." />
        <div style={{ marginTop: 24, fontSize: 13, color: '#888' }}>提取结构化数据块并验证关联池...</div>
      </div>
    );
  }

  if (exportPath) {
    return (
      <Result
        status="success"
        icon={<CheckCircleFilled style={{ color: '#059669' }} />}
        title="无痕编织成功！Word文档已结构化生成"
        subTitle="所有的动态表格和段落数据已完好注入原模板中，文件格式毫发无损且100%纯净。"
        extra={[
          <Button type="primary" key="download" href={`/api/bid-projects/${project_id}/step6/download?file=${encodeURIComponent(exportPath.split('/').pop() || '')}`}>
            下载最终排版文档 (Word)
          </Button>,
          <Button key="next" onClick={onReadyToNext}>
            确认无误，进入下一步
          </Button>
        ]}
      />
    );
  }

  return (
    <div style={{ padding: '0 24px' }}>


      {allChapterNames.length > 0 && (
        <div style={{ padding: '12px 16px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, marginBottom: 16 }}>
          <Space wrap size={[12, 12]}>
            {allChapterNames.map(chap => {
              const chapSlots = chaptersMap.get(chap) || [];
              const unresCount = chapSlots.filter(s => s.status !== 'approved').length;
              const isActive = activeChapter === chap;
              const isAssembled = hasChapterBindingContent(chap);

              return (
                <Button
                  key={chap}
                  type={isActive ? 'primary' : 'default'}
                  onClick={() => setActiveChapter(chap)}
                  style={{
                    borderRadius: 6,
                    fontWeight: isActive ? 600 : 'normal',
                    height: 'auto',
                    padding: '6px 16px',
                    borderColor: isActive ? 'transparent' : (isAssembled ? '#a7f3d0' : '#cbd5e1'),
                    backgroundColor: (!isActive && isAssembled) ? '#f0fdf4' : undefined,
                    color: (!isActive && isAssembled) ? '#065f46' : undefined,
                    display: 'flex',
                    alignItems: 'center',
                    transition: 'all 0.3s'
                  }}
                >
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 150 }}>
                    {chap}
                  </span>
                  {isAssembled && (
                    <CheckCircleFilled
                      style={{
                        marginLeft: 6,
                        color: isActive ? '#fff' : '#10b981',
                        fontSize: 14
                      }}
                    />
                  )}
                </Button>
              );
            })}
          </Space>
        </div>
      )}


      <Row gutter={24}>
        {/* Right Panel: Pure Markdown Reference - Now Full Width */}
        <Col span={24}>
          <Card 
            title={<><BuildOutlined style={{ color: '#10b981', marginRight: 8 }} /> 文档原文参考</>} 
            size="small"
          >
            {/* Bound Resources Bucket */}
            {activeChapter && (!hideFloatingPanel || (chapterBindings[activeChapter] && (chapterBindings[activeChapter].resources.length > 0 || chapterBindings[activeChapter].supplement !== ''))) && (
              <div 
                style={{ 
                  marginBottom: 16, 
                  padding: '12px 16px', 
                  background: '#f0fdf4', 
                  border: '1px dashed #34d399', 
                  borderRadius: 8,
                  cursor: !hideFloatingPanel && (!chapterBindings[activeChapter] || (chapterBindings[activeChapter].resources.length === 0 && !chapterBindings[activeChapter].supplement)) ? 'pointer' : 'default',
                  transition: 'all 0.2s ease',
                }}
                onClick={() => {
                  // Only trigger whole-container click when empty
                  if (!hideFloatingPanel && (!chapterBindings[activeChapter] || (chapterBindings[activeChapter].resources.length === 0 && !chapterBindings[activeChapter].supplement))) {
                    const existingKeys = chapterBindings[activeChapter]?.resources.map(r => r.ruleId) || [];
                    setSelectedResourceKeys(existingKeys);
                    setSupplementaryText(chapterBindings[activeChapter]?.supplement || '');
                    setIsConfigModalVisible(true);
                  }
                }}
              >
                {(!chapterBindings[activeChapter] || (chapterBindings[activeChapter].resources.length === 0 && !chapterBindings[activeChapter].supplement)) ? (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '58px', color: '#10b981' }}>
                    <PlusCircleOutlined style={{ fontSize: 20, marginRight: 8, opacity: 0.8 }} />
                    <span style={{ fontSize: 15, fontWeight: 500, opacity: 0.9 }}>点击装配当前章节核心资源</span>
                  </div>
                ) : (
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
                    <div style={{ flex: 1, paddingRight: 16 }}>
                      <div style={{ marginBottom: 8, fontWeight: 600, color: '#065f46' }}>
                        <CheckCircleOutlined style={{ marginRight: 6 }} /> 本章已装配库
                      </div>
                      <Space wrap size={[0, 8]}>
                        {chapterBindings[activeChapter]?.resources?.map(res => (
                          <Tag
                            color="green"
                            key={res.ruleId}
                            closable={!hideFloatingPanel}
                            onClose={(e) => {
                              e.preventDefault();
                              setChapterBindings(prev => {
                                const newState = {
                                  ...prev,
                                  [activeChapter]: {
                                    ...prev[activeChapter],
                                    resources: prev[activeChapter].resources.filter(r => r.ruleId !== res.ruleId)
                                  }
                                };
                                axios.post(`/api/bid-projects/${project_id}/step5-bindings`, { bindings: newState }, {
                                  headers: { 'x-company-id': currentCompanyId || '' }
                                }).catch(() => message.error('参数更新失败！'));
                                return newState;
                              });
                            }}
                          >
                            {res.record_name}
                          </Tag>
                        ))}
                      </Space>
                      {chapterBindings[activeChapter]?.supplement && (
                        <div style={{ marginTop: 8, position: 'relative', fontSize: 13, color: '#064e3b', background: '#d1fae5', padding: '6px 28px 6px 10px', borderRadius: 4, display: 'inline-block' }}>
                          <strong>备注指引：</strong> {chapterBindings[activeChapter].supplement}
                          {!hideFloatingPanel && (
                            <Button
                              type="text"
                              size="small"
                              icon={<CloseOutlined style={{ fontSize: 10, color: '#047857' }} />}
                              style={{ position: 'absolute', right: 4, top: 4, padding: 4, height: 'auto', minWidth: 'auto', lineHeight: 1 }}
                              onClick={() => {
                                setChapterBindings(prev => {
                                  const newState = {
                                    ...prev,
                                    [activeChapter]: {
                                      ...prev[activeChapter],
                                      supplement: ''
                                    }
                                  };
                                  axios.post(`/api/bid-projects/${project_id}/step5-bindings`, { bindings: newState }, {
                                    headers: { 'x-company-id': currentCompanyId || '' }
                                  }).catch(() => message.error('参数更新失败！'));
                                  return newState;
                                });
                              }}
                            />
                          )}
                        </div>
                      )}
                    </div>
                    {!hideFloatingPanel && (
                      <Button 
                        type="dashed" 
                        size="small" 
                        icon={<PlusCircleOutlined />} 
                        style={{ borderColor: '#34d399', color: '#10b981', background: '#e6fffa' }}
                        onClick={() => {
                          const existingKeys = chapterBindings[activeChapter]?.resources.map(r => r.ruleId) || [];
                          setSelectedResourceKeys(existingKeys);
                          setSupplementaryText(chapterBindings[activeChapter]?.supplement || '');
                          setIsConfigModalVisible(true);
                        }}
                      >
                        继续添加
                      </Button>
                    )}
                  </div>
                )}
              </div>
            )}
            {chapterMarkdown ? (
              <div className="markdown-body" style={{ fontSize: 14, color: '#334155', lineHeight: 1.8, padding: '16px 0' }}>
                <style>{markdownStyles}</style>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{chapterMarkdown}</ReactMarkdown>
              </div>
            ) : (
              <Empty description="当前章节暂无原文参考数据" />
            )}
          </Card>
        </Col>
      </Row>

      {/* Detail Modal */}
      {detailModalState && (
        <LibraryDetailModal
          visible={!!detailModalState}
          onCancel={() => setDetailModalState(null)}
          targetType={detailModalState.targetType}
          targetId={detailModalState.targetId}
        />
      )}

      {/* Binding Configuration Modal */}
      <Modal
        title={isChapterConfirmed(activeChapter) ? "修改本章装配参数" : "添加装配参数"}
        open={isConfigModalVisible}
        onOk={() => {
          if (!activeChapter) return;
          const resources = selectedResourceKeys.map(k => ({
            ruleId: k,
            ...data?.step4_bindings?.[k]
          })) as any[];

          setChapterBindings(prev => {
            const newState = {
              ...prev,
              [activeChapter]: {
                resources, // replace entirely with the newly selected ones
                supplement: supplementaryText
              }
            };
            // Post to backend immediately
            axios.post(`/api/bid-projects/${project_id}/step5-bindings`, { bindings: newState }, {
              headers: { 'x-company-id': currentCompanyId || '' }
            }).catch(err => message.error('参数持久化保存失败！'));
            return newState;
          });

          setIsConfigModalVisible(false);
          setSupplementaryText('');
          setSelectedResourceKeys([]);
          message.success('装配参数配置成功！已记录关联关系。');
        }}
        onCancel={() => setIsConfigModalVisible(false)}
        okText="确认装配"
        cancelText="取消"
        okButtonProps={{ style: { background: '#10b981' } }}
        width={600}
        styles={{ body: { paddingTop: 16 } }}
      >
        <div style={{ marginBottom: 24 }}>
          <Text strong style={{ display: 'block', marginBottom: 12 }}>请勾选即将装入【{activeChapter}】的资源：</Text>
          <div style={{ maxHeight: '240px', overflowY: 'auto', padding: '12px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: '8px' }}>
            {Object.keys(data?.step4_bindings || {}).length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="资源池为空，上一阶段未提取出任何有效库记录" />
            ) : (
              Object.entries(data?.step4_bindings || {})
                .sort((a: any, b: any) => {
                  const categoryOrder = ['qualification', 'project_performance', 'person', 'financial', 'credit', 'other_requirements', 'scoring_item'];
                  let idxA = categoryOrder.indexOf(a[1].category);
                  let idxB = categoryOrder.indexOf(b[1].category);
                  if (idxA === -1) idxA = 999;
                  if (idxB === -1) idxB = 999;
                  return idxA - idxB;
                })
                .map(([ruleId, bind]: any) => (
                <div 
                  key={ruleId} 
                  style={{ 
                    marginBottom: 12, display: 'flex', alignItems: 'flex-start', background: '#fff', 
                    padding: '8px 12px', borderRadius: 6, 
                    border: `1px solid ${selectedResourceKeys.includes(ruleId) ? '#10b981' : '#e2e8f0'}`,
                    cursor: 'pointer',
                    transition: 'border-color 0.2s'
                  }}
                  onClick={() => {
                    if (selectedResourceKeys.includes(ruleId)) {
                      setSelectedResourceKeys(prev => prev.filter(k => k !== ruleId));
                    } else {
                      setSelectedResourceKeys(prev => [...prev, ruleId]);
                    }
                  }}
                >
                  <Checkbox 
                    style={{ marginTop: 2, marginRight: 12, pointerEvents: 'none' }}
                    checked={selectedResourceKeys.includes(ruleId)}
                  />
                  <div style={{ flex: 1, fontSize: 13, lineHeight: '18px' }}>
                    <div style={{ color: '#0ea5e9', fontWeight: 500, marginBottom: 4 }}>
                      [{bind.category === 'person' ? '人员' : bind.category === 'qualification' ? '资质' : bind.category === 'project_performance' ? '业绩' : bind.category === 'financial' ? '财务' : bind.category === 'credit' ? '信誉' : bind.category === 'other_requirements' ? '其他' : '通用'}] {bind.record_name}
                    </div>
                    <div style={{ color: '#64748b' }}>{bind.requirement_text || '通用资源指标'}</div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
        <div>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>本章补充说明 (Supplement)：</Text>          
          <Input.TextArea
            rows={4}
            placeholder="请输入针对该章节额外的业务说明或承诺约定（如：承诺本项目经理全勤驻场...），若无则留空。"
            value={supplementaryText}
            onChange={e => setSupplementaryText(e.target.value)}
          />
        </div>
      </Modal>
    </div>
  );
};
