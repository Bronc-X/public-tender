import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Typography, Button, Steps, Card, Space, Tag, Divider,
  Empty, Spin, message, Badge, Upload, Input,
  Alert, Result, Row, Col, Modal, Collapse, Tooltip, Popover, Progress, Grid
} from 'antd';
import {
  LeftOutlined, CheckCircleOutlined, FileWordOutlined, RollbackOutlined,
  SyncOutlined, CheckCircleFilled,
  ExclamationCircleFilled,
  CloseCircleFilled,
  CloseCircleOutlined,
  WarningOutlined,
  FileTextOutlined,
  ProjectOutlined, SafetyCertificateOutlined,
  FileDoneOutlined, HistoryOutlined,
  DownloadOutlined, CloudUploadOutlined,
  InboxOutlined, SearchOutlined,
  SafetyOutlined, SolutionOutlined, FileSearchOutlined,
  BranchesOutlined, EditOutlined, DeleteOutlined
} from '@ant-design/icons';
import axios from 'axios';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useCompany } from '../context/CompanyContext';
import { ResourceCombinationPanel } from '../components/ResourceCombinationPanel';
import { CommerceChapterGenerationPanel } from '../components/CommerceChapterGenerationPanel';

const { Title, Text, Paragraph } = Typography;
const { Dragger } = Upload;

const STEP_ORDER = [
  'tender_detail_extract', 'rule_parse', 'company_adaptation', 'resource_combination', 'user_confirmation',
  'chapter_generation', 'attachment_assembly', 'risk_review', 'output_finalize'
] as const;

const STEP_DETAILS: Record<string, { label: string; description: string; icon: React.ReactNode }> = {
  tender_detail_extract: { label: '提取招标详情', description: '从招标文件提取项目基本信息与结构', icon: <SearchOutlined /> },
  rule_parse: { label: '招标规则解析', description: '解析资格、否决与评分规则', icon: <FileTextOutlined /> },
  company_adaptation: { label: '各项指标适配', description: '匹配企业资质、人员与业绩', icon: <ProjectOutlined /> },
  resource_combination: { label: '资源方案组合', description: '组合资源与附件形成投标方案', icon: <SyncOutlined /> },
  user_confirmation: { label: '方案处理确认', description: '确认方案与资源匹配结果', icon: <CheckCircleOutlined /> },
  chapter_generation: { label: '标书章节生成', description: '自动生成商务标章节正文', icon: <FileTextOutlined /> },
  attachment_assembly: { label: '附件自动装配', description: '装配表格、证明与附件', icon: <SafetyCertificateOutlined /> },
  risk_review: { label: '合规风险审查', description: '合规检查与风险识别', icon: <WarningOutlined /> },
  output_finalize: { label: '成果定稿输出', description: '导出定稿标书与成果包', icon: <FileDoneOutlined /> }
};

const BidProjectWorkbench: React.FC = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { currentCompanyId } = useCompany();
  const screens = Grid.useBreakpoint();
  const [project, setProject] = useState<Record<string, any> | null>(null);
  const [loading, setLoading] = useState(true);
  const [resourceBindings, setResourceBindings] = useState<Record<string, any>>({});
  const [running, setRunning] = useState(false);
  const [activeStepData, setActiveStepData] = useState<Record<string, any> | null>(null);
  const [apiKeyInput, setApiKeyInput] = useState('');
  const [hasApiKey, setHasApiKey] = useState(false);
  const [editedRules, setEditedRules] = useState<Record<string, any> | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [isGenericRulesModalVisible, setIsGenericRulesModalVisible] = useState(false);
  
  // Step 5 Validation State
  const [isChapterGenerationValid, setIsChapterGenerationValid] = useState(true);
  const [missingChapterCount, setMissingChapterCount] = useState(0);

  const fetchProjectDetail = useCallback(async (isSilent = false) => {
    if (!currentCompanyId) return;
    if (!isSilent) setLoading(true);
    try {
      const headers = { 'x-company-id': currentCompanyId };
      const response = await axios.get(`/api/bid-projects/${id}`, { headers });
      setProject(response.data);

      // Check system settings for API Key
      const settingsRes = await axios.get('/api/settings');
      setHasApiKey(!!settingsRes.data.ai_api_key);

      if (response.data.latestRuleParse && response.data.current_step === 'rule_parse') {
        const parsed = response.data.latestRuleParse;
        const effectiveCount =
          (Array.isArray(parsed?.eligibility) ? parsed.eligibility.length : 0) +
          (Array.isArray(parsed?.scoring) ? parsed.scoring.length : 0);
        const data = effectiveCount > 0 ? parsed : (response.data.current_step_status === 'success' ? parsed : null);
        setActiveStepData(data);
        setEditedRules(data);
      } else if (response.data.latestCompanyAdaptation && response.data.current_step === 'company_adaptation') {
        const parsed = response.data.latestCompanyAdaptation;
        const effectiveCount = Array.isArray(parsed?.results) ? parsed.results.length : 0;
        const data = effectiveCount > 0 ? parsed : (response.data.current_step_status === 'success' ? parsed : null);
        setActiveStepData(data);
        setEditedRules(parsed);
      } else if (response.data.current_step) {
        const actionRes = await axios.get(`/api/bid-projects/${id}/actions`, {
          params: { step_name: response.data.current_step, status: 'success' },
          headers
        });
        const firstAction = Array.isArray(actionRes.data) ? actionRes.data[0] : null;
        const parsed = firstAction?.result_json ? JSON.parse(firstAction.result_json) : null;
        const effectiveCount =
          (Array.isArray(parsed?.eligibility) ? parsed.eligibility.length : 0) +
          (Array.isArray(parsed?.scoring) ? parsed.scoring.length : 0) +
          (Array.isArray(parsed?.results) ? parsed.results.length : 0);
        const data = effectiveCount > 0 ? parsed : (response.data.current_step_status === 'success' ? parsed : null);
        setActiveStepData(data);
        setEditedRules(data);
      }
      setIsDirty(false);
    } catch (err: any) {
      console.error('Failed to fetch project detail:', err);
      // If project doesn't belong to company, redirect to list
      if (err.response?.status === 404) {
        navigate('/bid-projects');
      } else if (!isSilent) {
        message.error('加载项目详情失败，请重试');
      }
      if (!isSilent) setProject(null);
    } finally {
      if (!isSilent) setLoading(false);
    }
  }, [id, currentCompanyId, navigate]);

  useEffect(() => {
    fetchProjectDetail();
  }, [fetchProjectDetail]);

  // Polling for project status when running
  useEffect(() => {
    let timer: any;
    if (project?.current_step_status === 'running') {
      timer = setInterval(() => {
        fetchProjectDetail(true);
      }, 2000);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [project?.current_step_status, fetchProjectDetail]);

  const handleRunWorkflow = useCallback(async (force = false) => {
    setRunning(true);
    setActiveStepData(null); // Reset data while running
    try {
      const headers = { 'x-company-id': currentCompanyId || '' };
      await axios.post(`/api/bid-projects/${id}/run`, { force }, { headers });
      message.success('工作流已启动');
      await fetchProjectDetail();
    } catch (err: any) {
      console.error('Run error:', err);
      if (err.response?.data?.error?.includes('MISSING_API_KEY')) {
        message.warning('请先配置大模型 API Key');
        fetchProjectDetail();
      } else {
        message.error('启动失败');
      }
    } finally {
      setRunning(false);
    }
  }, [id, currentCompanyId, fetchProjectDetail]);

  // 自动触发提取招标规则：当抵达 rule_parse 且状态为 waiting 且没有解析过数据时自动运行！
  useEffect(() => {
    if (project?.current_step === 'rule_parse' && project?.current_step_status === 'waiting') {
      const hasPreviousParse = !!project?.latestRuleParse;
      if (!hasPreviousParse && !running) {
        handleRunWorkflow();
      }
    }
  }, [project?.current_step, project?.current_step_status, project?.latestRuleParse, running, handleRunWorkflow]);

  // 自动触发公司指标适配：当抵达 company_adaptation 且状态为 waiting 且没有解析过数据时自动运行！
  useEffect(() => {
    if (project?.current_step === 'company_adaptation' && project?.current_step_status === 'waiting') {
      const hasPreviousAdaptation = !!(project as any)?.latestCompanyAdaptation;
      if (!hasPreviousAdaptation && !running) {
        handleRunWorkflow();
      }
    }
  }, [project?.current_step, project?.current_step_status, (project as any)?.latestCompanyAdaptation, running, handleRunWorkflow]);

  const handleSaveApiKey = async () => {
    if (!apiKeyInput) return message.error('请输入 API Key');
    try {
      await axios.post('/api/settings', { key: 'ai_api_key', value: apiKeyInput });
      message.success('API Key 已保存');
      setHasApiKey(true);
      handleRunWorkflow();
    } catch (err) {
      message.error('保存失败');
    }
  };

  const handleConfirm = async () => {
    try {
      setRunning(true);
      if (curStepKey === 'tender_detail_extract') {
        const payload = {
          confirm_type: 'auto',
          project_data: {
            profile: Object.keys(editedRules || {}).length > 0 ? editedRules : activeStepData
          }
        };
        await axios.put(`/api/bid-projects/${id}/rules`, payload, { headers: { 'x-company-id': currentCompanyId || '' } });
        await axios.post(`/api/bid-projects/${id}/confirm`, { confirm_type: 'auto' }, { headers: { 'x-company-id': currentCompanyId || '' } });
      } else if (curStepKey === 'rule_parse') {
        await axios.put(`/api/bid-projects/${id}/rules`, { project_data: { latestRuleParse: editedRules } }, { headers: { 'x-company-id': currentCompanyId || '' } });
        await axios.post(`/api/bid-projects/${id}/confirm`, { confirm_type: 'auto' }, { headers: { 'x-company-id': currentCompanyId || '' } });
      } else if (curStepKey === 'resource_combination') {
        // Submit the custom resource binding mappings
        await axios.post(`/api/bid-projects/${id}/resource-combination`, { bindings: resourceBindings }, { headers: { 'x-company-id': currentCompanyId || '' } });
        // Manually move forward after saving
        await axios.post(`/api/bid-projects/${id}/confirm`, { confirm_type: 'auto' }, { headers: { 'x-company-id': currentCompanyId || '' } });
      } else {
        await axios.post(`/api/bid-projects/${id}/confirm`, { confirm_type: 'auto' }, { headers: { 'x-company-id': currentCompanyId || '' } });
      }
      message.success('已确认，进入下一阶段');
      setIsDirty(false);
      fetchProjectDetail();
    } catch (e) {
      console.error(e);
      message.error('确认失败');
    } finally {
      setRunning(false);
    }
  };

  const handleGoBack = async () => {
    try {
      const headers = { 'x-company-id': currentCompanyId || '' };
      await axios.post(`/api/bid-projects/${id}/goback`, {}, { headers });
      message.success('已回退到上一个阶段');
      fetchProjectDetail();
    } catch (err) {
      message.error('操作失败');
    }
  };

  const handleReRun = () => {
    Modal.confirm({
      title: '重新提取规则',
      content: '再次提交将重复消耗 token 费用，且您当前的手动修改（如有）将被覆盖。是否继续？',
      okText: '继续',
      cancelText: '取消',
      onOk: () => {
        handleRunWorkflow(true);
      }
    });
  };

  const handleSaveRules = async () => {
    if (!editedRules) return;
    setRunning(true);
    try {
      const headers = { 'x-company-id': currentCompanyId || '' };
      await axios.put(`/api/bid-projects/${id}/rules`, { rules: editedRules }, { headers });
      message.success('规则已保存并同步至云端');
      setIsDirty(false);
      fetchProjectDetail(true);
    } catch (err) {
      message.error('保存失败');
    } finally {
      setRunning(false);
    }
  };

  const handleDeleteCategory = (sourceType: string, categoryName: string) => {
    if (!editedRules) return;
    const newRules = { ...editedRules };
    if (newRules.normalized_rules) {
      newRules.normalized_rules = newRules.normalized_rules.filter(
        (r: any) => !(r.source_type === sourceType && (r.category === categoryName || (!r.category && categoryName === '其他')))
      );
    }
    // Handle legacy format if still used
    if (Array.isArray(newRules[sourceType])) {
      newRules[sourceType] = newRules[sourceType].filter((c: any) => c.category !== categoryName);
    }
    setEditedRules(newRules);
    setIsDirty(true);
  };

  const handleDeleteRule = (ruleId: string, sourceType: string, categoryName: string, itemText: string) => {
    if (!editedRules) return;
    const newRules = { ...editedRules };
    let changed = false;

    if (newRules.normalized_rules) {
      const countBefore = newRules.normalized_rules.length;
      newRules.normalized_rules = newRules.normalized_rules.filter((r: any) => r.id !== ruleId);
      if (newRules.normalized_rules.length < countBefore) changed = true;
    }

    // Also handle legacy format if it matches the text and category
    if (newRules[sourceType] && Array.isArray(newRules[sourceType])) {
      newRules[sourceType] = newRules[sourceType].map((cat: any) => {
        if (cat.category === categoryName) {
          const newItems = cat.items.filter((it: any) => {
            const text = typeof it === 'string' ? it : (it.requirement_text || it.normalized_text || it.raw_text);
            return text !== itemText;
          });
          if (newItems.length < cat.items.length) changed = true;
          return { ...cat, items: newItems, count: newItems.length };
        }
        return cat;
      }).filter((cat: any) => cat.items.length > 0);
    }

    if (changed) {
      setEditedRules(newRules);
      setIsDirty(true);
    }
  };

  const handleTextChange = (ruleId: string, sourceType: string, categoryName: string, oldText: string, newText: string) => {
    if (!editedRules || oldText === newText) return;
    const newRules = { ...editedRules };
    let changed = false;

    if (newRules.normalized_rules) {
      newRules.normalized_rules = newRules.normalized_rules.map((r: any) => {
        if ((ruleId && r.id === ruleId) || (r.source_type === sourceType && r.category === categoryName && (r.requirement_text || r.normalized_text || r.raw_text) === oldText)) {
          changed = true;
          return { ...r, requirement_text: newText, normalized_text: newText, is_manual: true };
        }
        return r;
      });
    }

    // Also handle legacy format if it exists
    if (newRules[sourceType] && Array.isArray(newRules[sourceType])) {
      newRules[sourceType] = newRules[sourceType].map((cat: any) => {
        if (cat.category === categoryName) {
          const newItems = cat.items.map((it: any) => {
            const itText = typeof it === 'string' ? it : (it.requirement_text || it.normalized_text || it.raw_text);
            if (itText === oldText) {
              changed = true;
              if (typeof it === 'string') return newText;
              return { ...it, normalized_text: newText, is_manual: true };
            }
            return it;
          });
          return { ...cat, items: newItems, count: newItems.length };
        }
        return cat;
      });
    }

    if (changed) {
      setEditedRules(newRules);
      setIsDirty(true);
    }
  };

  const handlePreviewMarkdown = async () => {
    if (!tenderFile?.file_asset_id) return;
    try {
      const res = await axios.get(`/api/file-parsed/${tenderFile.file_asset_id}`);
      Modal.info({
        title: `招标文件原文解析 (${tenderFile.file_name})`,
        width: 1000,
        maskClosable: true,
        icon: null,
        style: { top: 40 },
        content: (
          <div style={{
            maxHeight: '70vh',
            overflowY: 'auto',
            padding: '24px',
            background: '#fcfcfd',
            borderRadius: 8,
            border: '1px solid #e2e8f0',
            lineHeight: 1.8
          }}>
            <style>
              {`
                .markdown-preview h1 { border-bottom: 2px solid #e2e8f0; padding-bottom: 8px; margin-top: 32px; font-weight: 800; }
                .markdown-preview h2 { border-bottom: 1px solid #cbd5e1; padding-bottom: 4px; margin-top: 24px; color: #1e293b; font-weight: 700; }
                .markdown-preview table { border-collapse: collapse; width: 100%; margin: 16px 0; }
                .markdown-preview th, .markdown-preview td { border: 1px solid #cbd5e1; padding: 12px; text-align: left; }
                .markdown-preview th { background-color: #f1f5f9; font-weight: 600; }
                .markdown-preview code { background-color: #f1f5f9; padding: 2px 4px; border-radius: 4px; color: #e11d48; }
                .markdown-preview strong { color: #0f172a; }
              `}
            </style>
            <div className="markdown-preview">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{res.data}</ReactMarkdown>
            </div>
          </div>
        ),
        okText: '关闭'
      });
    } catch (err) {
      message.error('获取解析内容失败，可能文件尚未解析完成');
    }
  };

  const handleFileUpload = async (info: any) => {
    const { status, response } = info.file;
    if (status === 'done') {
      try {
        await axios.post(`/api/bid-projects/${id}/files`, {
          file_asset_id: response.id,
          file_role: 'tender'
        }, {
          headers: { 'x-company-id': currentCompanyId || '' }
        });
        message.success('招标文件关联成功');
        await fetchProjectDetail();
        // Auto-run the rule parsing workflow after upload
        handleRunWorkflow();
      } catch (_err) {
        message.error('关联文件失败');
      }
    } else if (status === 'error') {
      message.error(`${info.file.name} 上传失败`);
    }
  };

  if (loading) return <div style={{ padding: 100, textAlign: 'center' }}><Spin size="large" /></div>;
  if (!project) return <Empty description="未找到项目" />;

  const currentStepIndex = STEP_ORDER.indexOf(
    (project.current_step as (typeof STEP_ORDER)[number]) || 'tender_detail_extract'
  );

  const tenderFile = project.files?.find((f: any) => f.file_role === 'tender');

  const renderStepContent = () => {
    const getGroupedData = (sourceType: string) => {
      const dataToUse = editedRules || activeStepData;
      if (!dataToUse) return [];

      const catMap: Record<string, string> = {
        'rejection_criteria': '否决投标规定',
        'eligibility': '资格要求',
        'qualification': '资质要求',
        'person': '人员要求',
        'project_performance': '业绩要求',
        'honor_record': '荣誉记录',
        'compliance_credit': '合规信用',
        'scoring_item': '评分细则',
        'scoring': '评分办法'
      };

      if (dataToUse.normalized_rules && Array.isArray(dataToUse.normalized_rules) && dataToUse.normalized_rules.length > 0) {
        const filtered = dataToUse.normalized_rules.filter((r: any) =>
          r.source_type === sourceType ||
          (sourceType === 'eligibility' && r.source_type === 'rejection_criteria') ||
          (sourceType === 'scoring' && (r.source_type === 'scoring' || r.source_type === 'scoring_item'))
        );
        const groups: Record<string, any[]> = {};
        filtered.forEach((r: any) => {
          let cat = '';
          if (r.source_type === 'rejection_criteria') {
            cat = '否决投标规定';
          } else {
            // Priority: category_group (from parsing) > category (from AI normalization)
            cat = r.category_group || (r.category && r.category !== 'other' ? (catMap[r.category] || r.category) : (catMap[r.source_type] || r.source_type || '其他'));
          }

          if (!groups[cat]) groups[cat] = [];
          groups[cat].push(r);
        });
        const sortOrder = ['资质要求', '业绩要求', '人员要求', '财务要求', '信誉要求', '其他要求'];
        return Object.entries(groups).map(([cat, items]) => ({
          category: cat,
          items: items,
          count: items.length
        })).sort((a, b) => {
          let idxA = sortOrder.indexOf(a.category);
          let idxB = sortOrder.indexOf(b.category);
          if (idxA === -1) idxA = 999;
          if (idxB === -1) idxB = 999;
          if (idxA !== idxB) return idxA - idxB;
          return a.category.localeCompare(b.category);
        });
      }
      if (dataToUse[sourceType] && Array.isArray(dataToUse[sourceType]) && dataToUse[sourceType].length > 0) {
        const sortOrder = ['资质要求', '业绩要求', '人员要求', '财务要求', '信誉要求', '其他要求'];
        const legacyData = [...dataToUse[sourceType]];
        return legacyData.sort((a, b) => {
          let idxA = sortOrder.indexOf(a.category);
          let idxB = sortOrder.indexOf(b.category);
          if (idxA === -1) idxA = 999;
          if (idxB === -1) idxB = 999;
          if (idxA !== idxB) return idxA - idxB;
          return a.category && b.category ? a.category.localeCompare(b.category) : 0;
        });
      }
      return [];
    };

    const getAdaptationGroupedData = (sourceType: string, results: any[]) => {
      const filtered = results.filter((r: any) =>
        r.status !== 'ignored' && (
          r.source_type === sourceType ||
          (sourceType === 'eligibility' && (r.source_type === 'rejection_criteria' || (!r.source_type && ['qualification', 'compliance_credit', 'person'].includes(r.category)))) ||
          (sourceType === 'scoring' && (r.source_type === 'scoring' || r.source_type === 'scoring_item' || (!r.source_type && ['project_performance', 'honor_record'].includes(r.category))))
        )
      );

      const catMap: Record<string, string> = {
        'rejection_criteria': '否决投标规定',
        'eligibility': '资格要求',
        'qualification': '资质要求',
        'person': '人员要求',
        'project_performance': '业绩要求',
        'honor_record': '荣誉记录',
        'compliance_credit': '合规信用',
        'scoring_item': '评分细则',
        'scoring': '评标方法'
      };

      const groups: Record<string, any[]> = {};
      filtered.forEach((r: any) => {
        let cat = '';
        // Strong priority for specific category_group to keep it distinct
        if (r.source_type === 'rejection_criteria') {
          cat = '否决投标规定';
        } else {
          // Priority: category_group (inherited from Step 2) > category_name (expert) > mapped category key > raw category
          cat = r.category_group || r.category_name || (r.category && catMap[r.category]) || r.category;

          // If category is other or missing, try source_type mapping
          if (!cat || cat === 'other' || cat === '未分类' || cat === '其他') {
            cat = catMap[r.source_type] || r.source_type || '其他';
          }
        }

        if (!groups[cat]) groups[cat] = [];
        groups[cat].push(r);
      });
      const sortOrder = ['资质要求', '业绩要求', '人员要求', '财务要求', '信誉要求', '其他要求'];
      return Object.entries(groups).map(([cat, items]) => ({
        category: cat,
        items: items,
        count: items.length
      })).sort((a, b) => {
        let idxA = sortOrder.indexOf(a.category);
        let idxB = sortOrder.indexOf(b.category);
        if (idxA === -1) idxA = 999;
        if (idxB === -1) idxB = 999;
        if (idxA !== idxB) return idxA - idxB;
        return a.category.localeCompare(b.category);
      });
    };

    const renderAdaptationSections = (dataList: any[]) => (
      <div style={{ padding: '0 8px 12px 12px' }}>
        {dataList.map((cat: any, idx: number) => (
          <div key={idx} style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 8 }}>
              <div style={{ width: 4, height: 14, backgroundColor: '#3b82f6', marginRight: 8, borderRadius: 2 }} />
              <span style={{ position: 'relative', paddingRight: 16 }}>
                <Text strong style={{ fontSize: 14, color: '#1e293b' }}>{cat.category}</Text>
                {cat.count > 0 && (
                  <span style={{ position: 'absolute', top: -2, right: -4, fontSize: 10, background: '#f1f5f9', color: '#64748b', padding: '0 5px', borderRadius: '10px', border: '1px solid #e2e8f0', lineHeight: '14px' }}>
                    {cat.count}
                  </span>
                )}
              </span>
            </div>
            <ul style={{ paddingLeft: 20, margin: 0, listStyleType: 'none' }}>
              {cat.items?.map((item: any, i: number) => {
                const requirementText = item.requirement_text || item.requirement || item.text;
                return (
                  <li key={i} style={{ padding: '8px 0', borderBottom: '1px solid #f1f5f9' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 8 }}>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <Text style={{ fontSize: 13, color: '#334155', display: 'block', marginBottom: 4, wordBreak: 'break-word' }}>
                          {requirementText}
                        </Text>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                          {item.matched_record_name ? (
                            <Tag color="cyan" style={{ fontSize: 11, borderRadius: 4, margin: 0, whiteSpace: 'normal', height: 'auto', padding: '2px 7px', wordBreak: 'break-word' }}>
                              证据：{item.matched_record_name}
                            </Tag>
                          ) : (
                            <Tag style={{ fontSize: 11, borderRadius: 4, margin: 0 }}>未匹配到具体证据</Tag>
                          )}
                          <Popover content={<div style={{ maxWidth: 300 }}>{item.reason}</div>} title="对标详情">
                            <Button type="link" size="small" style={{ fontSize: 11, padding: 0, height: 'auto' }}>
                              查看原因
                            </Button>
                          </Popover>
                        </div>
                      </div>
                      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', minWidth: 75, flexShrink: 0, whiteSpace: 'nowrap' }}>
                        <div style={{ marginBottom: 2 }}>
                          {item.status === 'success' && <CheckCircleFilled style={{ color: '#059669', fontSize: 18 }} />}
                          {item.status === 'warning' && <ExclamationCircleFilled style={{ color: '#d97706', fontSize: 18 }} />}
                          {item.status === 'failed' && <CloseCircleFilled style={{ color: '#dc2626', fontSize: 18 }} />}
                        </div>
                        {item.status === 'success' && <Text style={{ fontSize: 11, color: '#059669' }}>完全匹配</Text>}
                        {item.status === 'warning' && <Text style={{ fontSize: 11, color: '#d97706' }}>需人工核实</Text>}
                        {item.status === 'failed' && <Text style={{ fontSize: 11, color: '#dc2626' }}>未满足要求</Text>}
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </div>
    );

    if (project.current_step_status === 'running' || running) {
      const stageMsg = project.current_stage_message || project.last_error_message || '正在分析文件，请稍候...';
      const percent = project.current_progress || 0;
      return (
        <Result
          icon={<SyncOutlined spin />}
          title={`${STEP_DETAILS[project.current_step]?.label || '正在处理'}...`}
          subTitle={
            <Space orientation="vertical" align="center" style={{ width: '100%', marginTop: 8 }}>
              <Text type="secondary">{stageMsg}</Text>
              <div style={{ width: 400, marginTop: 16 }}>
                <Progress
                  percent={percent}
                  status="active"
                  strokeColor={{
                    '0%': '#108ee9',
                    '100%': '#87d068',
                  }}
                />
              </div>
            </Space>
          }
        />
      );
    }

    if (project.current_step_status === 'warning' || (project.current_step === 'rule_parse' && project.last_error_message?.includes('MISSING_API_KEY'))) {
      if (hasApiKey) {
        return (
          <div style={{ padding: '0 40px' }}>
            <Card style={{ borderRadius: 12 }}>
              <Result
                status="success"
                title="API Key 已检测"
                subTitle="您已在全局设置中配置了大模型密钥，随时可以开始深度招标文件解析。"
                extra={
                  <Button type="primary" size="large" onClick={() => handleRunWorkflow()}>
                    立即开始 AI 规则提取
                  </Button>
                }
              />
            </Card>
          </div>
        );
      }
      return (
        <div style={{ padding: '0 40px' }}>
          <Card
            title="需要配置大模型 API Key"
            variant="borderless"
            style={{ boxShadow: '0 4px 12px rgba(0,0,0,0.05)', borderRadius: 12 }}
            extra={<Tag color="warning">解析阻断</Tag>}
          >
            <Alert
              message="检测到尚未配置线上大模型 Key"
              description="招标文件解析需要调用云端大模型（如 GLM-4）进行语义抽提。请在下方输入您的 API Key 以继续。"
              type="warning"
              showIcon
              style={{ marginBottom: 24 }}
            />
            <Space.Compact style={{ width: '100%' }}>
              <Input.Password
                placeholder="请输入您的 API Key (如：747...)"
                value={apiKeyInput}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setApiKeyInput(e.target.value)}
                size="large"
                style={{ borderRadius: '10px 0 0 10px' }}
              />
              <Button
                type="primary"
                size="large"
                onClick={handleSaveApiKey}
                style={{ borderRadius: '0 10px 10px 0' }}
              >
                保存并继续解析
              </Button>
            </Space.Compact>
            <div style={{ marginTop: 16 }}>
              <Text type="secondary" style={{ fontSize: 13 }}>
                提示：您的 Key 将安全存储在本地数据库中，仅用于本项目的招标规则自动提取。
              </Text>
            </div>
          </Card>
        </div>
      );
    }

    if (project.current_step_status === 'failed') {
      return (
        <Result
          status="error"
          title="处理失败"
          subTitle={project.last_error_message}
          extra={<Button type="primary" onClick={() => handleRunWorkflow()}>重试当前步骤</Button>}
        />
      );
    }

    switch (project.current_step) {
      case 'tender_detail_extract':
        if (!tenderFile) {
          return (
            <div style={{ padding: '0 40px' }}>
              <Card style={{ border: '2px dashed #d9d9d9', borderRadius: 12 }}>
                <Dragger
                  name="file"
                  multiple={false}
                  action="/api/files/upload"
                  headers={{ 'x-company-id': currentCompanyId || '' }}
                  data={{ source_module: 'tender' }}
                  onChange={handleFileUpload}
                  showUploadList={false}
                >
                  <p className="ant-upload-drag-icon"><InboxOutlined style={{ color: '#1890ff' }} /></p>
                  <p className="ant-upload-text">点击或将招标文件拖拽到此处上传</p>
                  <p className="ant-upload-hint">支持 PDF, DOCX 格式，上传后系统将启动标书数字化解析（OCR/VLM）。</p>
                </Dragger>
              </Card>
            </div>
          );
        }

        // 当已存在招标文件时，无论是解析完成还是回退回来的状态，都显示统一的解析完毕视图，
        // 避免出现两个容易混淆的提示状态。
        return (
          <div style={{ padding: '0 40px' }}>
            <Card style={{ borderRadius: 12 }}>
              <Result
                status="success"
                icon={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
                title="标书数字化解析完毕"
                subTitle={
                  <span>
                    已上传文件：<strong>{tenderFile.file_name}</strong>
                    <br />全文内容已成功转化为数字化 Markdown 并存储在本地。
                    <br />现在可以开始进行 AI 招标规则结构化提取。
                  </span>
                }
              />
            </Card>
          </div>
        );

      case 'rule_parse': {
        const hasData = (activeStepData || editedRules) && (
          ((editedRules || activeStepData)?.eligibility?.length > 0 || (editedRules || activeStepData)?.scoring?.length > 0) ||
          ((editedRules || activeStepData)?.normalized_rules?.length > 0)
        );

        if (!hasData) {
          const isSuccess = project?.current_step_status === 'success';
          return (
            <div style={{ padding: '0 40px' }}>
              <Card style={{ borderRadius: 12 }}>
                <Result
                  icon={
                    isSuccess ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> :
                      project.current_step_status === 'failed' ? <CloseCircleOutlined style={{ color: '#ff4d4f' }} /> :
                        <FileSearchOutlined style={{ color: '#1890ff' }} />
                  }
                  title={isSuccess ? "招标规则解析完成" : project.current_step_status === 'failed' ? "提取失败" : "招标规则正在深度提取..."}
                  subTitle={
                    isSuccess
                      ? (project.current_stage_message || "未在文档中解析到明确的投标资格或评分硬指标。建议检查原文或尝试重新提取。")
                      : (project.current_step_status === 'failed' ? (project.last_error_message || '提取过程中发生了未知错误。') :
                        project.current_step_status === 'running' ? (project.current_stage_message || project.last_error_message || '正在由 AI 专家级提取硬指标与评分点...') :
                          '系统即将自动执行专家模式提取，请稍候...')
                  }
                  extra={
                    project.current_step_status === 'running' ? null :
                      isSuccess ? (
                        <Space>
                          <Button
                            type="default"
                            size="large"
                            onClick={() => {
                              Modal.confirm({
                                title: '重新提取确认',
                                content: '当前已有解析记录，确定要覆盖现有记录并重新分析吗？',
                                onOk: () => handleRunWorkflow(true)
                              });
                            }}
                            icon={<SyncOutlined />}
                          >
                            重新解析
                          </Button>
                        </Space>
                      ) : (
                        <Space>
                          <Button
                            type="primary"
                            size="large"
                            loading={running}
                            onClick={() => handleRunWorkflow(true)}
                            icon={<SyncOutlined />}
                          >
                            {project.current_step_status === 'failed' ? '重新解析' : '开始解析'}
                          </Button>
                        </Space>
                      )
                  }
                />
              </Card>
            </div>
          );
        }

        const renderCompactSections = (dataList: any[], sourceType: string) => (
          <div style={{ padding: '0 8px 12px 12px' }}>
            {dataList.map((cat: any, idx: number) => (
              <div key={idx} style={{ marginBottom: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', marginBottom: 4, justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <div style={{ width: 4, height: 14, backgroundColor: '#3b82f6', marginRight: 8, borderRadius: 2 }} />
                    <span style={{ position: 'relative', paddingRight: 16 }}>
                      <Text strong style={{ fontSize: 15, color: '#1e293b' }}>{cat.category}</Text>
                      {cat.count > 0 && (
                        <span style={{ position: 'absolute', top: -2, right: -4, fontSize: 10, background: '#f1f5f9', color: '#64748b', padding: '0 5px', borderRadius: '10px', border: '1px solid #e2e8f0', lineHeight: '14px' }}>
                          {cat.count}
                        </span>
                      )}
                    </span>
                  </div>
                  <Space>
                    <Tooltip title="删除整个分类">
                      <Button
                        type="text"
                        size="small"
                        danger
                        icon={<DeleteOutlined style={{ fontSize: 12 }} />}
                        onClick={() => handleDeleteCategory(sourceType, cat.category)}
                      />
                    </Tooltip>
                  </Space>
                </div>
                <ul style={{ paddingLeft: 24, margin: 0, listStyleType: 'circle' }}>
                  {cat.items?.map((item: any, i: number) => {
                    const text = typeof item === 'string' ? item : (item.requirement_text || item.normalized_text || item.raw_text || '');
                    const isWarning = item.need_human_review || item.conflict_detected;
                    return (
                      <li key={i} style={{ padding: '4px 0', color: isWarning ? '#b45309' : '#475569', fontSize: 13, lineHeight: '20px' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                          <Space size={4} style={{ flex: 1 }}>
                            <Text
                              editable={{
                                onChange: (val) => handleTextChange(item.id, sourceType, cat.category, text, val),
                                tooltip: '点击编辑规则文本',
                                icon: <EditOutlined style={{ fontSize: 12, color: '#94a3b8' }} />
                              }}
                              style={{ color: isWarning ? '#b45309' : '#475569' }}
                            >
                              {text}
                            </Text>
                            {isWarning && <Tooltip title="需人工复核该项"><WarningOutlined style={{ color: '#d97706', fontSize: 12 }} /></Tooltip>}
                            {item.merge_from?.length > 0 && <Tooltip title={`合并了 ${item.merge_from.length} 条相似项`}><BranchesOutlined style={{ color: '#94a3b8', fontSize: 12 }} /></Tooltip>}
                          </Space>
                          <Button
                            type="text"
                            size="small"
                            danger
                            icon={<DeleteOutlined style={{ fontSize: 11 }} />}
                            onClick={() => handleDeleteRule(item.id, sourceType, cat.category, text)}
                            style={{ marginLeft: 8 }}
                          />
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
        );

        const eligibilityData = getGroupedData('eligibility');
        const scoringData = getGroupedData('scoring');

        return (
          <div style={{ padding: '0 24px', position: 'relative' }}>
            {/* Sticky Floating Modification Alert (Left Side) */}
            {isDirty && (
              <div style={{
                position: 'fixed',
                top: 280,
                left: 220,
                width: 240,
                zIndex: 1001,
                backgroundColor: 'rgba(255, 255, 255, 0.95)',
                backdropFilter: 'blur(12px)',
                borderRadius: 20,
                padding: '24px 20px',
                boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
                border: '1px solid #fbbf24',
                animation: 'slideInLeft 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
                display: 'flex',
                flexDirection: 'column',
                gap: 16
              }}>
                <div style={{ display: 'flex', alignItems: 'center' }}>
                  <div style={{ backgroundColor: '#fff7ed', color: '#ea580c', padding: '8px', borderRadius: '12px', marginRight: 12, boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.05)' }}>
                    <WarningOutlined style={{ fontSize: 20 }} />
                  </div>
                  <Text strong style={{ fontSize: 17, color: '#1e293b', letterSpacing: '-0.02em' }}>规则已修改</Text>
                </div>

                <Text type="secondary" style={{ fontSize: 13, lineHeight: '1.6', color: '#64748b' }}>
                  检测到您对提取规则进行了人工调整。请及时同步，以免刷新后丢失。
                </Text>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 4 }}>
                  <Button
                    type="primary"
                    icon={<SyncOutlined />}
                    onClick={handleSaveRules}
                    loading={running}
                    style={{
                      borderRadius: 12,
                      height: 44,
                      background: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)',
                      border: 'none',
                      fontWeight: 600,
                      boxShadow: '0 4px 12px rgba(217, 119, 6, 0.3)'
                    }}
                  >
                    同步保存修改
                  </Button>
                  <Button
                    onClick={() => { fetchProjectDetail(); setIsDirty(false); }}
                    style={{
                      borderRadius: 12,
                      height: 42,
                      border: '1px solid #e2e8f0',
                      color: '#64748b',
                      fontWeight: 500
                    }}
                  >
                    重置并放弃
                  </Button>
                </div>
              </div>
            )}


            <style>
              {`
                    @keyframes slideInLeft {
                        from { opacity: 0; transform: translateX(-30px); }
                        to { opacity: 1; transform: translateX(0); }
                    }
                `}
            </style>

            <Row gutter={16}>
              {/* 第一大块：资格要求 */}
              <Col span={12}>
                <Collapse
                  defaultActiveKey={['1']}
                  expandIconPosition="end"
                  style={{ backgroundColor: 'transparent', border: 'none' }}
                >
                  <Collapse.Panel
                    header={<Text strong style={{ color: '#1e40af', fontSize: 16 }}><SafetyOutlined style={{ marginRight: 8 }} />第一大块：资格要求（硬指标）</Text>}
                    key="1"
                    style={{ backgroundColor: '#eff6ff', border: '1px solid #dbeafe', borderRadius: 8, overflow: 'hidden' }}
                  >
                    <div style={{ backgroundColor: '#ffffff', borderTop: '1px solid #dbeafe', paddingTop: 12, minHeight: '400px' }}>
                      {eligibilityData.length > 0 ? renderCompactSections(eligibilityData, 'eligibility') : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未发现资格要求项" />}
                    </div>
                  </Collapse.Panel>
                </Collapse>
              </Col>

              {/* 第二大块：评分细则 */}
              <Col span={12}>
                <Collapse
                  defaultActiveKey={['2']}
                  expandIconPosition="end"
                  style={{ backgroundColor: 'transparent', border: 'none' }}
                >
                  <Collapse.Panel
                    header={<Text strong style={{ color: '#166534', fontSize: 16 }}><SolutionOutlined style={{ marginRight: 8 }} />第二大块：评分细则与得分点</Text>}
                    key="2"
                    style={{ backgroundColor: '#f0fdf4', border: '1px solid #dcfce7', borderRadius: 8, overflow: 'hidden' }}
                  >
                    <div style={{ backgroundColor: '#ffffff', borderTop: '1px solid #dcfce7', paddingTop: 12, minHeight: '400px' }}>
                      {scoringData.length > 0 ? renderCompactSections(scoringData, 'scoring') : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未发现评分细则项" />}
                    </div>
                  </Collapse.Panel>
                </Collapse>
              </Col>
            </Row>
          </div>
        );
      }
      case 'company_adaptation': {
        const hasData = activeStepData?.results?.length > 0;

        if (!hasData) {
          const isSuccess = project?.current_step_status === 'success';
          return (
            <div style={{ padding: '0 40px' }}>
              <Card style={{ borderRadius: 12 }}>
                <Result
                  icon={
                    isSuccess ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> :
                      project.current_step_status === 'failed' ? <CloseCircleOutlined style={{ color: '#ff4d4f' }} /> :
                        <FileSearchOutlined style={{ color: '#1890ff' }} />
                  }
                  title={isSuccess ? "企业合规对标完成" : project.current_step_status === 'failed' ? "对标失败" : "企业库比对正在进行中..."}
                  subTitle={
                    isSuccess
                      ? (project.current_stage_message || "指标适配已完成，系统未发现任何有效指标准则。")
                      : (project.current_step_status === 'failed' ? (project.last_error_message || '对标过程中发生了未知错误。') :
                        project.current_step_status === 'running' ? (project.current_stage_message || project.last_error_message || '正在由 AI 专家模式对企业库进行深度扫描比对...') :
                          '系统即将自动执行企业资质与业绩适配分析，请稍候...')
                  }
                  extra={
                    project.current_step_status === 'running' ? null :
                      isSuccess ? (
                        <Space>
                          <Button
                            type="default"
                            size="large"
                            onClick={() => {
                              Modal.confirm({
                                title: '重新对标确认',
                                content: '确定要重新进行企业库适配比对吗？',
                                onOk: () => handleRunWorkflow(true)
                              });
                            }}
                            icon={<SyncOutlined />}
                          >
                            重新对标
                          </Button>
                        </Space>
                      ) : (
                        <Space>
                          <Button
                            type="primary"
                            size="large"
                            loading={running}
                            onClick={() => handleRunWorkflow(true)}
                            icon={<SyncOutlined />}
                          >
                            {project.current_step_status === 'failed' ? '重新对标' : '开始对标'}
                          </Button>
                        </Space>
                      )
                  }
                />
              </Card>
            </div>
          );
        }

        const results = activeStepData?.results || [];
        const summary = activeStepData?.summary || {};
        const ignoredRules = results.filter((r: any) => r.status === 'ignored');

        const eligibilityAdapted = getAdaptationGroupedData('eligibility', results);
        const scoringAdapted = getAdaptationGroupedData('scoring', results);

        return (
          <div style={{ padding: '0 24px' }}>


            <Row gutter={16}>
              <Col span={12}>
                <Collapse
                  defaultActiveKey={['1']}
                  expandIconPosition="end"
                  style={{ backgroundColor: 'transparent', border: 'none' }}
                >
                  <Collapse.Panel
                    header={<Text strong style={{ color: '#1e40af', fontSize: 16 }}><SafetyOutlined style={{ marginRight: 8 }} />第一大块：资格要求（对标详情）</Text>}
                    key="1"
                    style={{ backgroundColor: '#eff6ff', border: '1px solid #dbeafe', borderRadius: 12, overflow: 'hidden' }}
                  >
                    <div style={{ backgroundColor: '#ffffff', borderTop: '1px solid #dbeafe', paddingTop: 12, minHeight: '500px' }}>
                      {eligibilityAdapted.length > 0 ? renderAdaptationSections(eligibilityAdapted) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未发现对标结果" />}
                    </div>
                  </Collapse.Panel>
                </Collapse>
              </Col>

              <Col span={12}>
                <Collapse
                  defaultActiveKey={['2']}
                  expandIconPosition="end"
                  style={{ backgroundColor: 'transparent', border: 'none' }}
                >
                  <Collapse.Panel
                    header={<Text strong style={{ color: '#166534', fontSize: 16 }}><SolutionOutlined style={{ marginRight: 8 }} />第二大块：评分细则与得分点（对标详情）</Text>}
                    key="2"
                    style={{ backgroundColor: '#f0fdf4', border: '1px solid #dcfce7', borderRadius: 12, overflow: 'hidden' }}
                  >
                    <div style={{ backgroundColor: '#ffffff', borderTop: '1px solid #dcfce7', paddingTop: 12, minHeight: '500px' }}>
                      {scoringAdapted.length > 0 ? renderAdaptationSections(scoringAdapted) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未发现对标结果" />}
                    </div>
                  </Collapse.Panel>
                </Collapse>
              </Col>
            </Row>


          </div>
        );
      }


      case 'resource_combination':
        return (
          <ResourceCombinationPanel
            project_id={id || ''}
            sourceData={project.latestCompanyAdaptation}
            savedBindings={project.latestResourceCombination?.bindings}
            onReadyToNext={(payload) => setResourceBindings(payload)}
            onOpenGenericRules={() => setIsGenericRulesModalVisible(true)}
            onRefresh={() => fetchProjectDetail(true)}
          />
        );

      case 'user_confirmation':

      case 'chapter_generation':
        return (
          <CommerceChapterGenerationPanel
            project_id={id || ''}
            onReadyToNext={() => handleConfirm()}
            onRefresh={() => fetchProjectDetail(true)}
            hideFloatingPanel={curStepKey === 'chapter_generation'}
            onValidationChange={(isValid, count) => {
              setIsChapterGenerationValid(isValid);
              setMissingChapterCount(count);
            }}
          />
        );

      case 'output_finalize':
        return (
          <div style={{ padding: '0 24px', textAlign: 'center' }}>
            <Result
              status="success"
              title="商务标制作完成"
              subTitle="所有章节已生成，附件已装配，风险审查已通过。"
              extra={[
                <Button type="primary" key="down" icon={<DownloadOutlined />} size="large">下载完整项目包 (.zip)</Button>,
                <Button key="back" onClick={() => navigate('/bid-projects')}>返回项目列表</Button>
              ]}
            />
          </div>
        );

      default:
        return (
          <div style={{ padding: 100, textAlign: 'center' }}>
            <Empty description="该阶段数据详情开发中..." />
            {project.current_step_status === 'success' && <Button onClick={() => handleRunWorkflow()}>推进到下一步</Button>}
          </div>
        );
    }
  };

  const curStepKey =
    project.current_step && STEP_DETAILS[project.current_step]
      ? project.current_step
      : 'tender_detail_extract';

  let globalIgnoredRulesCount = 0;
  let globalIgnoredRules: any[] = [];
  const results = (project as any)?.latestCompanyAdaptation?.results || [];
  globalIgnoredRules = results.filter((r: any) => r.status === 'ignored');
  globalIgnoredRulesCount = globalIgnoredRules.length;
  const curStepInfo = STEP_DETAILS[curStepKey];
  const stepsCurrent = currentStepIndex >= 0 ? currentStepIndex : 0;

  return (
    <div style={{ background: '#f8fafc', minHeight: 'calc(100vh - 150px)', margin: '-24px', padding: '24px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
        <Space align="start">
          <Button icon={<LeftOutlined />} onClick={() => navigate('/bid-projects')} type="text" />
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <Title level={4} style={{ margin: 0 }}>{project.project_name}</Title>
              {tenderFile?.parse_status === 'completed' && (
                <Button
                  type="link"
                  size="small"
                  icon={<FileTextOutlined style={{ color: '#52c41a' }} />}
                  onClick={handlePreviewMarkdown}
                  style={{ padding: 0, height: 'auto', color: '#52c41a' }}
                >
                  已解析
                </Button>
              )}
              <Tag color="blue">商务标</Tag>
              {project.project_status === 'completed' ? (
                <Badge status="success" text="已完成" />
              ) : project.current_step_status === 'running' ? (
                <Badge status="processing" text="处理中" />
              ) : null}
            </div>
            <Text type="secondary" style={{ fontSize: 13 }}>
              项目 ID: {project.id} | 创建时间: {new Date(project.created_at).toLocaleDateString()}
              {' | '}
              业主：{project.owner_name || '--'}
              {tenderFile && (
                <>
                  {' | 招标文件：'}
                  <Text ellipsis={{ tooltip: tenderFile.file_name }} style={{ maxWidth: 420, verticalAlign: 'bottom' }}>
                    {tenderFile.file_name}
                  </Text>
                </>
              )}
            </Text>
          </div>
        </Space>
        <Space wrap>


          {(project.current_step === 'rule_parse' && !activeStepData && !tenderFile) ? (
            <Upload
              name="file"
              action="/api/files/upload"
              headers={{ 'x-company-id': currentCompanyId || '' }}
              data={{ source_module: 'tender' }}
              onChange={handleFileUpload}
              showUploadList={false}
            >
              <Button icon={<CloudUploadOutlined />}>上传招标文件</Button>
            </Upload>
          ) : null}
        </Space>
      </div>

      <Row gutter={[24, 24]}>
        <Col xs={24} lg={4}>
          <Card styles={{ body: { padding: '24px 16px' } }} variant="borderless" style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <Steps
              orientation={screens.lg ? "vertical" : "horizontal"}
              current={stepsCurrent}
              size="small"
              items={STEP_ORDER.map((key, idx) => {
                const detail = STEP_DETAILS[key];
                const isFinished = stepsCurrent > idx;
                const isActive = stepsCurrent === idx;
                const isFailed = isActive && (project.current_step_status === 'failed' || project.current_step_status === 'warning');

                return {
                  title: (
                    <span style={{
                      color: isFailed ? '#ff4d4f' : (isFinished || isActive ? '#1890ff' : 'inherit'),
                      fontWeight: isActive ? 600 : 400,
                      fontSize: 14
                    }}>
                      {detail.label}
                    </span>
                  ),
                  status: isFinished ? 'finish' : (isActive ? (isFailed ? 'error' : 'process') : 'wait'),
                  icon: detail.icon
                };
              })}
            />
          </Card>
        </Col>

        <Col xs={24} lg={20}>
          <Card
            styles={{ body: { minHeight: 480, padding: 32 } }}
            variant="borderless"
            style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}
            title={
              <Space>
                <div style={{ background: '#e6f4ff', padding: 8, borderRadius: 8, display: 'flex' }}>
                  {curStepInfo.icon}
                </div>
                <Text strong style={{ fontSize: 18 }}>{curStepInfo.label}</Text>
              </Space>
            }
            extra={
              <Space>
                {curStepKey === 'company_adaptation' && project.current_step_status === 'success' && (
                  <Button onClick={handleReRun} icon={<SyncOutlined />} style={{ color: '#64748b', borderColor: '#d9d9d9' }}>
                    重新对标分析
                  </Button>
                )}

                {currentStepIndex > 0 && (
                  <Button icon={<RollbackOutlined />} onClick={handleGoBack}>
                    回退上一步
                  </Button>
                )}
                {curStepKey === 'rule_parse' && !isDirty && project.current_step_status === 'success' && (
                  <Button onClick={handleReRun} icon={<SyncOutlined />} style={{ color: '#64748b', borderColor: '#d9d9d9' }}>
                    重新提取规则
                  </Button>
                )}
                {(project.current_step_status === 'success' ||
                  (project.current_step_status === 'waiting' && curStepKey === 'tender_detail_extract' && !!tenderFile) ||
                  (project.current_step_status === 'waiting' && curStepKey === 'rule_parse' && !!project.latestRuleParse) ||
                  (project.current_step_status === 'waiting' && curStepKey === 'company_adaptation' && !!(project as any)?.latestCompanyAdaptation) ||
                  (project.current_step_status === 'waiting' && curStepKey === 'resource_combination') ||
                  (project.current_step_status === 'waiting' && curStepKey === 'user_confirmation'))
                  && project.project_status !== 'completed' && (
                    <Tooltip title={!isChapterGenerationValid && curStepKey === 'user_confirmation' ? `检测到还有 ${missingChapterCount} 个章节尚未确认装配资料，请先完成处理以防止废标风险。` : ''}>
                      <span style={!isChapterGenerationValid && curStepKey === 'user_confirmation' ? { cursor: 'not-allowed' } : {}}>
                        <Button 
                          type="primary" 
                          ghost 
                          loading={running} 
                          onClick={handleConfirm} 
                          icon={<CheckCircleOutlined />}
                          disabled={!isChapterGenerationValid && curStepKey === 'user_confirmation'}
                          style={!isChapterGenerationValid && curStepKey === 'user_confirmation' ? { pointerEvents: 'none' } : {}}
                        >
                          确认并执行下一步
                        </Button>
                      </span>
                    </Tooltip>
                  )}
              </Space>
            }
          >
            {renderStepContent()}
          </Card>
        </Col>
      </Row>

      {globalIgnoredRulesCount > 0 && currentStepIndex >= 3 && (
        <div id="generic-commitments-float-btn" style={{ position: 'fixed', right: 40, top: '40%', zIndex: 1000, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <Badge count={globalIgnoredRulesCount} overflowCount={999} color="#f43f5e" offset={[-5, 5]} size="medium" style={{ zIndex: 1001 }}>
            <Button
              type="primary"
              shape="circle"
              style={{
                background: '#6366f1',
                boxShadow: '0 8px 24px rgba(99, 102, 241, 0.4)',
                width: 64,
                height: 64,
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                border: 'none'
              }}
              icon={<SafetyCertificateOutlined style={{ fontSize: 28 }} />}
              onClick={() => setIsGenericRulesModalVisible(true)}
            />
          </Badge>
          <div style={{ marginTop: 8, fontSize: 12, fontWeight: 600, color: '#475569', background: 'rgba(255,255,255,0.8)', padding: '2px 8px', borderRadius: 4, backdropFilter: 'blur(4px)' }}>
            特别关注
          </div>
        </div>
      )}

      <Modal
        title={
          <Space>
            <SafetyCertificateOutlined style={{ color: '#6366f1', fontSize: 20 }} />
            <span style={{ fontSize: 16 }}>通用及特别关注条款库</span>
          </Space>
        }
        open={isGenericRulesModalVisible}
        onCancel={() => setIsGenericRulesModalVisible(false)}
        footer={null}
        width={700}
        styles={{ body: { maxHeight: '60vh', overflowY: 'auto', padding: '16px 24px' } }}
      >

        <ul style={{ paddingLeft: 0, margin: 0, listStyleType: 'none' }}>
          {globalIgnoredRules.map((rule: any, idx: number) => {
            const title = rule.requirement_text || rule.requirement || rule.text || '条款项';
            return (
              <li key={idx} style={{ marginBottom: 16, color: '#475569', fontSize: 13 }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <Text style={{ color: '#334155', fontWeight: 500, display: 'block', wordBreak: 'break-word', lineHeight: '20px' }}>
                    <span style={{ color: '#6366f1', marginRight: 6, fontWeight: 700 }}>{idx + 1}.</span>
                    {title}
                  </Text>
                  <Space>
                    <Tag color="purple" style={{ fontSize: 11, border: 'none', background: '#f3e8ff' }}>{rule.category_group || '合规与信用'}</Tag>
                    <Text type="secondary" style={{ fontSize: 11 }}>由大模型收纳</Text>
                  </Space>
                </div>
              </li>
            )
          })}
        </ul>
      </Modal>

    </div>
  );
};

export default BidProjectWorkbench;
