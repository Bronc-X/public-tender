import React, { useState, useEffect, useCallback, useMemo, lazy, Suspense } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
    Typography, Button, Steps, Card, Space, Tag,
    Empty, Spin, message, Upload, List, Modal,
    Alert, Result, Row, Col, Statistic, Progress, Badge, Input, Select, Drawer, Form, Radio, Checkbox, Tabs, Tooltip, Timeline, Grid
} from 'antd';
import {
    LeftOutlined,
    FileTextOutlined, ProjectOutlined, SafetyCertificateOutlined,
    RocketOutlined, BuildOutlined,
    InboxOutlined, SearchOutlined,
    DatabaseOutlined, ToolOutlined, FireOutlined, WarningOutlined, TeamOutlined, BulbOutlined,
    RollbackOutlined as AntdRollbackOutlined, CloudSyncOutlined, PlusCircleOutlined,
} from '@ant-design/icons';
import axios from 'axios';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useCompany } from '../context/CompanyContext';
import {
    fetchStep4Mappings,
    fetchStep4Coverage,
    fetchStep4Requirements,
    fetchStep4FullResponse,
    fetchStep4ConflictAudit,
    fetchStructurePlan,
    approveStructurePlan,
    rejectStructurePlan,
    fetchOutlineRunStatus,
    fetchOutlineAgentRuns,
    fetchOutlineRunHistory,
    fetchStep4ApprovalLogs,
    fetchStep4FactCandidates,
    fetchOutlineVersions,
    fetchOutlineVersionDetail,
    selectOutlineVersion,
    postStep4GateOverride,
    type Step4FactMapping,
    type Step4RunStatusPayload,
    type Step4AgentRunRow,
    type Step4RunRow,
    type Step4ApprovalLogRow,
    type Step4FactCandidate,
    type Step4Coverage,
    type Step4RequirementRow,
    type Step4FullResponse,
    type Step4ConflictAudit,
    type StructurePlan,
    type OutlineVersionRow,
    regenerateOutline,
    type SkeletonCandidate,
    fetchSkeletonCandidates,
    confirmSkeleton,
} from '../api/techBidStep4';

// P1-2: 代码分割 - 懒加载重型组件，减少首屏 bundle 体积
const StructurePlanReviewPanel = lazy(() =>
    import('../components/StructurePlanReviewPanel').then(m => ({ default: m.StructurePlanReviewPanel }))
);
const ProjectProfilePanel = lazy(() =>
    import('../components/ProjectProfilePanel').then(m => ({ default: m.default }))
);
const OutlineVerificationPanel = lazy(() =>
    import('../components/OutlineVerificationPanel').then(m => ({ default: m.default }))
);
const ContentGenerationPanel = lazy(() =>
    import('../components/ContentGenerationPanel').then(m => ({ default: m.default }))
);
const RiskReviewPanel = lazy(() =>
    import('../components/RiskReviewPanel').then(m => ({ default: m.default }))
);
const OutlineGenerationPanel = lazy(() =>
    import('../components/OutlineGenerationPanel').then(m => ({ default: m.default }))
);

// 懒加载组件的加载占位符
const ComponentLoadingPlaceholder: React.FC<{ height?: number }> = ({ height = 400 }) => (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: height, background: '#fafafa', borderRadius: 8 }}>
        <Spin size="large" description="加载组件中..." />
    </div>
);

const { Title, Text, Paragraph } = Typography;
const { Dragger } = Upload;
const { TextArea } = Input;
const { Option } = Select;

const TECH_STEP_DETAILS: Record<string, { label: string, description: string, icon: React.ReactNode }> = {
    tender_parse: { label: '招标解析', description: '解析招标文件并提取技术要求与评分点', icon: <SearchOutlined /> },
    project_profile: { label: '项目画像', description: '提炼项目特点、施工难点与企业优势匹配', icon: <ProjectOutlined /> },
    route_planning: { label: '路线规划', description: '确定整体写作策略、亮点分布与编制路线', icon: <BuildOutlined /> },
    outline_generation: { label: '目录生成', description: 'AI 基于招标文件直接生成三级目录，漏项校验兜底', icon: <DatabaseOutlined /> },
    outline_verification: { label: '终审核查', description: '专家级 AI 终审，排除废标与丢分风险', icon: <SafetyCertificateOutlined /> },
    content_generation: { label: '正文生成', icon: <FileTextOutlined />, description: '按小节逐步生成高保真标书内容' },
    risk_review: { label: '风控合规', description: '整标合规性复检与潜在风险扫描', icon: <WarningOutlined /> },
    output_finalize: { label: '导出定稿', icon: <RocketOutlined />, description: '最终格式排版并导出 Word/PDF' }
};

const STEP_ORDER = ['tender_parse', 'project_profile', 'route_planning', 'outline_generation', 'outline_verification', 'content_generation', 'risk_review', 'output_finalize'];

const TechBidProjectWorkbench: React.FC = () => {
    const { id } = useParams();
    const navigate = useNavigate();
    const { currentCompanyId } = useCompany();
    const screens = Grid.useBreakpoint();
    const [project, setProject] = useState<any>(null); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [loading, setLoading] = useState(true);
    const [chapters, setChapters] = useState<any[]>([]); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [editingChapter, setEditingChapter] = useState<any>(null); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [editorContent, setEditorContent] = useState('');
    const [isEditorVisible, setIsEditorVisible] = useState(false);
    const [risks, setRisks] = useState<any[]>([]); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [auditLoading, setAuditLoading] = useState(false);
    const [exportingWord, setExportingWord] = useState(false);
    const [isGenerateModalVisible, setIsGenerateModalVisible] = useState(false);
    const [generatingChapter, setGeneratingChapter] = useState<any>(null);
    const [batchGeneration, setBatchGeneration] = useState({
        running: false,
        total: 0,
        completed: 0,
        failed: 0,
        currentName: '',
    });
    const [isResourceDrawerVisible, setIsResourceDrawerVisible] = useState(false);
    const [selectedResource, setSelectedResource] = useState<any>(null);
    const [genConfig, setGenConfig] = useState<any>({
        wordCount: 1000,
        style: 'professional',
        references: ['equipment', 'method', 'risk', 'person'],
        tone: 'balanced',
        depth: 'detailed'
    });
    const [selectingRoute, setSelectingRoute] = useState(false);
    const [verifying, setVerifying] = useState(false);
    const [isVerificationEditVisible, setIsVerificationEditVisible] = useState(false);
    const [editedVerificationSuggestions, setEditedVerificationSuggestions] = useState('');
    const [isConfigModalVisible, setIsConfigModalVisible] = useState(false);
    const [isFactsDrawerVisible, setIsFactsDrawerVisible] = useState(false);
    const [doubaoApiKey, setDoubaoApiKey] = useState('');
    const [doubaoEndpoint, setDoubaoEndpoint] = useState('');
    const [doubaoModelId, setDoubaoModelId] = useState('');
    const [step4Mappings, setStep4Mappings] = useState<Step4FactMapping[]>([]);
    const [step4Coverage, setStep4Coverage] = useState<Step4Coverage | null>(null);
    const [step4Requirements, setStep4Requirements] = useState<Step4RequirementRow[]>([]);
    const [step4FullResponse, setStep4FullResponse] = useState<Step4FullResponse | null>(null);
    const [step4ConflictAudit, setStep4ConflictAudit] = useState<Step4ConflictAudit | null>(null);
    const [step4ArtifactsLoading, setStep4ArtifactsLoading] = useState(false);
    const [step4DrawerOpen, setStep4DrawerOpen] = useState(false);
    const [step4HighlightFactId, setStep4HighlightFactId] = useState<string | null>(null);
    const [structurePlan, setStructurePlan] = useState<StructurePlan | null>(null);
    const [approvingPlan, setApprovingPlan] = useState(false);
    const [step4ArtifactsWarning, setStep4ArtifactsWarning] = useState<string | null>(null);
    const [step4RunStatus, setStep4RunStatus] = useState<Step4RunStatusPayload | null>(null);
    const [step4AgentRuns, setStep4AgentRuns] = useState<Step4AgentRunRow[]>([]);
    const [step4RunHistory, setStep4RunHistory] = useState<Step4RunRow[]>([]);
    const [step4ApprovalLogs, setStep4ApprovalLogs] = useState<Step4ApprovalLogRow[]>([]);
    const [step4FactCandidates, setStep4FactCandidates] = useState<Step4FactCandidate[]>([]);
    const [outlineVersions, setOutlineVersions] = useState<OutlineVersionRow[]>([]);
    const [selectedOutlineVerId, setSelectedOutlineVerId] = useState<string | null>(null);
    const [selectedOutlineVersionDetail, setSelectedOutlineVersionDetail] = useState<any>(null); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [recommendedOutlineVersionDetail, setRecommendedOutlineVersionDetail] = useState<any>(null); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [selectingOutlineVer, setSelectingOutlineVer] = useState(false);

    // 骨架选择状态
    const [skeletonModalVisible, setSkeletonModalVisible] = useState(false);
    const [skeletonCandidates, setSkeletonCandidates] = useState<SkeletonCandidate[]>([]);
    const [selectedSkeletonId, setSelectedSkeletonId] = useState<string | null>(null);
    const [skeletonSearchText, setSkeletonSearchText] = useState('');
    const [skeletonLoading, setSkeletonLoading] = useState(false);
    const [skeletonConfirming, setSkeletonConfirming] = useState(false);

    // Shared traceability helper to open the audit drawer and highlight a fact
    useEffect(() => {
        const rec = outlineVersions.find(v => v.status === 'recommended');
        setSelectedOutlineVerId(rec?.id ?? outlineVersions[0]?.id ?? null);
    }, [outlineVersions]);

    useEffect(() => {
        if (!selectedOutlineVerId || !id || !currentCompanyId) {
            setSelectedOutlineVersionDetail(null);
            return;
        }
        let cancelled = false;
        const tick = async () => {
            try {
                const detail = await fetchOutlineVersionDetail(id, currentCompanyId, selectedOutlineVerId);
                if (!cancelled) setSelectedOutlineVersionDetail(detail);
            } catch {
                if (!cancelled) setSelectedOutlineVersionDetail(null);
            }
        };
        void tick();
        return () => {
            cancelled = true;
        };
    }, [selectedOutlineVerId, id, currentCompanyId]);

    useEffect(() => {
        const recommended = outlineVersions.find((v) => v.status === 'recommended')?.id ?? null;
        if (!recommended || !id || !currentCompanyId) {
            setRecommendedOutlineVersionDetail(null);
            return;
        }
        let cancelled = false;
        const tick = async () => {
            try {
                const detail = await fetchOutlineVersionDetail(id, currentCompanyId, recommended);
                if (!cancelled) setRecommendedOutlineVersionDetail(detail);
            } catch {
                if (!cancelled) setRecommendedOutlineVersionDetail(null);
            }
        };
        void tick();
        return () => {
            cancelled = true;
        };
    }, [outlineVersions, id, currentCompanyId]);

    const handleOpenAuditFact = (node: unknown) => {
        if (!node || typeof node !== 'object') return;
        const chapterNode = node as { id?: string; chapter_name?: string };
        const chapterName = chapterNode.chapter_name || '';
        setStep4DrawerOpen(true);
        const cleanNodeTitle = chapterName.replace(/^[0-9一二三四五六七八九十()（）①②③④⑤\s.\u4e00-\u9fa5]+[章节节部个项、.\s]*/, '').trim();

        const matched = step4Mappings.find(m =>
            (m.target_node_id && m.target_node_id === chapterNode.id) ||
            (m.target_path?.some(p => {
                const cleanP = p.replace(/^[0-9一二三四五六七八九十()（）①②③④⑤\s.\u4e00-\u9fa5]+[章节节部个项、.\s]*/, '').trim();
                return cleanP === cleanNodeTitle || cleanP.includes(cleanNodeTitle) || cleanNodeTitle.includes(cleanP) || p.includes(chapterName);
            }))
        );

        if (matched) {
            setStep4HighlightFactId(matched.fact_id);
        } else {
            const simpleMatch = step4Mappings.find(m => m.target_path?.some(p => p.includes(chapterName)));
            setStep4HighlightFactId(simpleMatch ? simpleMatch.fact_id : 'MAPPING_NOT_FOUND');
        }
    };

    const fetchStep4Artifacts = useCallback(async () => {
        if (!currentCompanyId || !id) return;
        setStep4ArtifactsLoading(true);
        try {
            const [m, c, req, fr, ca, sp, ov, agentRuns, runHistory, approvalLogs, factCandidates] = await Promise.all([
                fetchStep4Mappings(id, currentCompanyId),
                fetchStep4Coverage(id, currentCompanyId),
                fetchStep4Requirements(id, currentCompanyId),
                fetchStep4FullResponse(id, currentCompanyId),
                fetchStep4ConflictAudit(id, currentCompanyId),
                fetchStructurePlan(id, currentCompanyId),
                fetchOutlineVersions(id, currentCompanyId).catch(() => [] as OutlineVersionRow[]),
                fetchOutlineAgentRuns(id, currentCompanyId).catch(() => [] as Step4AgentRunRow[]),
                fetchOutlineRunHistory(id, currentCompanyId).catch(() => [] as Step4RunRow[]),
                fetchStep4ApprovalLogs(id, currentCompanyId).catch(() => [] as Step4ApprovalLogRow[]),
                fetchStep4FactCandidates(id, currentCompanyId).catch(() => [] as Step4FactCandidate[]),
            ]);
            setStep4Mappings(m);
            setStep4Coverage(c);
            setStep4Requirements(req);
            setStep4FullResponse(fr);
            setStep4ConflictAudit(ca);
            setStructurePlan(sp);
            setOutlineVersions(ov);
            setStep4AgentRuns(agentRuns);
            setStep4RunHistory(runHistory);
            setStep4ApprovalLogs(approvalLogs);
            setStep4FactCandidates(factCandidates);
            setStep4ArtifactsWarning(null);
        } catch (e) {
            console.error('Step4 artifacts fetch failed', e);
            setStep4ArtifactsWarning('Step4 数据拉取失败，当前展示可能已过期，建议刷新或重新触发目录生成。');
        } finally {
            setStep4ArtifactsLoading(false);
        }
    }, [id, currentCompanyId]);

    const fetchProject = useCallback(async (silent = false) => {
        if (!currentCompanyId || !id) return;
        if (!silent) setLoading(true);
        try {
            const res = await axios.get(`/api/tech-bid/projects/${id}`, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            setProject(res.data);
            const step = res.data?.current_step;
            const embedded = res.data?.chapterPlans;
            if (step === 'outline_generation' || step === 'content_generation') {
                try {
                    const chRes = await axios.get(`/api/tech-bid/chapters/project/${id}`, {
                        headers: { 'X-Company-Id': currentCompanyId }
                    });
                    if (Array.isArray(chRes.data) && chRes.data.length > 0) {
                        setChapters(chRes.data);
                    } else if (Array.isArray(embedded) && embedded.length > 0) {
                        setChapters(embedded);
                    } else {
                        setChapters([]);
                    }
                } catch (chErr) {
                    console.error('Fetch chapters error', chErr);
                    if (Array.isArray(embedded) && embedded.length > 0) {
                        setChapters(embedded);
                    } else {
                        setChapters([]);
                    }
                }
            } else {
                setChapters([]);
            }
            return res.data;
        } catch (err) {
            console.error(err);
            if (!silent) message.error('获取项目详情失败');
            return null;
        } finally {
            if (!silent) setLoading(false);
        }
    }, [id, currentCompanyId]);

    const fetchChapters = useCallback(async () => {
        if (!currentCompanyId || !id) return;
        try {
            const res = await axios.get(`/api/tech-bid/chapters/project/${id}`, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            setChapters((prev) => {
                if (!Array.isArray(res.data)) return prev;
                if (res.data.length === 0 && prev.length > 0) return prev;
                return res.data;
            });
        } catch (err) {
            console.error('Fetch chapters error', err);
        }
    }, [id, currentCompanyId]);

    const fetchRisks = useCallback(async () => {
        if (!currentCompanyId || !id) return;
        try {
            const res = await axios.get(`/api/tech-bid/risk/projects/${id}`);
            setRisks(Array.isArray(res.data) ? res.data : []);
        } catch (err) {
            console.error('Fetch risks error', err);
            setRisks([]);
        }
    }, [id, currentCompanyId]);

    useEffect(() => {
        void fetchProject();
    }, [currentCompanyId, id, fetchProject]);

    useEffect(() => {
        if (!currentCompanyId || !id) return;
        if (project?.current_step !== 'risk_review') return;
        void fetchRisks();
    }, [project?.current_step, currentCompanyId, id, fetchRisks]);

    // Polling while running or when step4 data may still be materializing
    useEffect(() => {
        let timer: any;
        if (
            project?.current_step_status === 'running'
            || project?.current_step_status === 'waiting_for_approval'
            || project?.current_step === 'outline_generation'
        ) {
            timer = setInterval(() => {
                void fetchProject(true);
                void fetchChapters();
                void fetchStep4Artifacts();
            }, 2000);
        }
        return () => timer && clearInterval(timer);
    }, [project?.current_step_status, project?.current_step, fetchProject, fetchChapters, fetchStep4Artifacts]);

    // Step4 Coordinator：轮询 run-status，驱动 Agent 时间线
    useEffect(() => {
        if (!id || !currentCompanyId || !project) return;
        const running =
            project.current_step === 'outline_generation' && project.current_step_status === 'running';
        if (!running) {
            setStep4RunStatus(null);
            return;
        }
        let cancelled = false;
        const tick = async () => {
            try {
                const r = await fetchOutlineRunStatus(id, currentCompanyId);
                if (!cancelled) setStep4RunStatus(r);
            } catch {
                /* ignore */
            }
        };
        void tick();
        const iv = setInterval(() => void tick(), 3000);
        return () => {
            cancelled = true;
            clearInterval(iv);
        };
    }, [id, currentCompanyId, project?.current_step, project?.current_step_status]);

    useEffect(() => {
        if (!id || !currentCompanyId || !project) return;
        if (project.current_step !== 'outline_generation' && project.current_step !== 'outline_verification') return;
        void fetchStep4Artifacts();
    }, [id, currentCompanyId, project?.current_step, project?.step4_status, fetchStep4Artifacts]);

    // Step4 强映射表 + 结构化覆盖率（目录生成 / 终审核查阶段展示）
    useEffect(() => {
        if (!project?.current_step || !id || !currentCompanyId) return;
        if (project.current_step !== 'outline_generation' && project.current_step !== 'outline_verification') {
            return;
        }
        void fetchStep4Artifacts();
    }, [
        id,
        currentCompanyId,
        project?.current_step,
        project?.step4_status,
        fetchStep4Artifacts,
    ]);

    const displayChapters = useMemo(() => {
        const step = project?.current_step;
        const useEmbedded = step === 'outline_generation' || step === 'content_generation';
        const embedded =
            useEmbedded && Array.isArray(project?.chapterPlans) ? project.chapterPlans : [];
        if (chapters.length > 0) return chapters;
        if (embedded.length > 0) return embedded;
        return [];
    }, [chapters, project?.chapterPlans, project?.current_step]);

    const handleRunStep = async () => {
        try {
            await axios.post(`/api/tech-bid/projects/${id}/run`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            message.success('已启动 AI 处理任务');
            fetchProject(true);
        } catch (err: unknown) {
            message.error('启动失败');
        }
    };

    const handleConfirm = async () => {
        try {
            await axios.post(`/api/tech-bid/projects/${id}/confirm`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            message.success('确认成功，进入下一环节');
            fetchProject();
        } catch (err) {
            message.error('操作失败');
        }
    };

    const handleExportWord = async () => {
        if (!id || !currentCompanyId) return;
        setExportingWord(true);
        try {
            const res = await axios.post(`/api/tech-bid/projects/${id}/step6/export`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            const downloadUrl = res.data?.download_url || `/api/tech-bid/projects/${id}/step6/download`;
            const link = document.createElement('a');
            link.href = downloadUrl;
            link.download = '';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            message.success('定稿 Word 已生成，正在下载');
            fetchProject(true);
        } catch {
            message.error('导出 Word 失败');
        } finally {
            setExportingWord(false);
        }
    };

    const getContentChapters = useCallback((source: any[]) => { // eslint-disable-line @typescript-eslint/no-explicit-any
        return source.filter((item: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            const hasChildren = source.some((child: any) => child.parent_id === item.id); // eslint-disable-line @typescript-eslint/no-explicit-any
            return item.node_level === 'subsection' || (!item.parent_id && !hasChildren);
        });
    }, []);

    const handleBatchGenerateContent = useCallback(async () => {
        if (!id || !currentCompanyId || batchGeneration.running) return;
        const contentChapters = getContentChapters(displayChapters);
        const pendingChapters = contentChapters.filter((chapter: any) => chapter.generation_status !== 'completed' && !chapter.content_md); // eslint-disable-line @typescript-eslint/no-explicit-any
        if (pendingChapters.length === 0) {
            message.success('所有小节正文已生成');
            return;
        }

        let completed = 0;
        let failed = 0;
        setBatchGeneration({
            running: true,
            total: pendingChapters.length,
            completed: 0,
            failed: 0,
            currentName: pendingChapters[0]?.chapter_name || '',
        });

        for (const chapter of pendingChapters) {
            setBatchGeneration(prev => ({ ...prev, currentName: chapter.chapter_name || '未命名小节' }));
            setChapters(prev => prev.map(item => item.id === chapter.id ? { ...item, generation_status: 'generating' } : item));
            try {
                await axios.post(`/api/tech-bid/chapters/${chapter.id}/generate`, {}, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                completed += 1;
                setBatchGeneration(prev => ({ ...prev, completed }));
                await fetchChapters();
            } catch (err) {
                failed += 1;
                setBatchGeneration(prev => ({ ...prev, failed }));
                setChapters(prev => prev.map(item => item.id === chapter.id ? { ...item, generation_status: 'error' } : item));
            }
        }

        await fetchChapters();
        await fetchProject(true);
        setBatchGeneration(prev => ({ ...prev, running: false, currentName: '' }));
        if (failed > 0) {
            message.warning(`全书排队生成完成：成功 ${completed} 节，失败 ${failed} 节`);
        } else {
            message.success(`全书排队生成完成：共生成 ${completed} 节`);
        }
    }, [id, currentCompanyId, batchGeneration.running, displayChapters, getContentChapters, fetchChapters, fetchProject]);

    const handleGoBack = async () => {
        try {
            await axios.post(`/api/tech-bid/projects/${id}/goback`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            fetchProject();
        } catch (err) {
            message.error('回退失败');
        }
    };

    const handleApproveStructurePlan = useCallback(async () => {
        if (!currentCompanyId || !id) return;
        setApprovingPlan(true);
        try {
            await approveStructurePlan(id, currentCompanyId);
            message.success('已批准结构计划，后台正在继续生成目录');
            await fetchProject(true);
            await fetchStep4Artifacts();
        } catch (e) {
            console.error(e);
            message.error('批准结构计划失败');
        } finally {
            setApprovingPlan(false);
        }
    }, [currentCompanyId, id, fetchProject, fetchStep4Artifacts]);

    const handleRejectStructurePlan = useCallback(async (reason: string) => {
        if (!currentCompanyId || !id) return;
        setApprovingPlan(true);
        try {
            await rejectStructurePlan(id, currentCompanyId, reason);
            message.success('已拒绝结构计划，请重新触发结构规划');
            await fetchProject(true);
            await fetchStep4Artifacts();
        } catch (e) {
            console.error(e);
            message.error('拒绝结构计划失败');
        } finally {
            setApprovingPlan(false);
        }
    }, [currentCompanyId, id, fetchProject, fetchStep4Artifacts]);

    const handleFileUpload = async (info: unknown) => {
        if (!info || typeof info !== 'object') return;
        const uploadInfo = info as { file?: { status?: string; response?: { id?: string } } };
        if (uploadInfo.file?.status === 'done' && uploadInfo.file?.response?.id) {
            const fileAssetId = uploadInfo.file.response.id;
            try {
                await axios.post(`/api/tech-bid/projects/${id}/files`, {
                    file_asset_id: fileAssetId,
                    file_role: 'tender'
                }, {
                    headers: { 'X-Company-Id': currentCompanyId }
                });
                message.success('文件上传并关联成功');
                const updatedProject = await fetchProject();
                
                // UX Optimization: If we just uploaded a tender file in Step 1, auto-start the extraction.
                if (updatedProject?.current_step === 'tender_parse' && updatedProject?.current_step_status !== 'running') {
                    handleRunStep();
                }
            } catch (err) {
                message.error('关联文件失败');
            }
        }
    };

    const handleSelectRoute = async (route: unknown) => {
        if (!route || typeof route !== 'object') return;
        const selectedRoute = route as { id: string; chapters: string[] };
        console.log('handleSelectRoute triggered for route:', selectedRoute.id);
        const hideLoading = message.loading('AI 正在深度解析招标文件并生成详细大纲，请耐心等待（耗时约 30-60 秒）...', 0);
        setSelectingRoute(true);
        try {
            console.log('Sending POST request to /api/tech-bid/projects/' + id + '/routes/select');
            await axios.post(`/api/tech-bid/projects/${id}/routes/select`, {
                routeId: selectedRoute.id,
                chapters: selectedRoute.chapters
            }, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            hideLoading();
            message.success('路线选择成功，已启动目录构建任务');
            await fetchProject();
            await fetchChapters();
            console.log('Project and chapters refreshed successfully');
        } catch (err: unknown) {
            hideLoading();
            console.error('Select route failed:', err);
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const errMsg = (err as any).response?.data?.error || (err as { message?: string }).message || '未知错误';
            message.error('选择路线失败: ' + errMsg);
        } finally {
            setSelectingRoute(false);
        }
    };

    // 骨架选择相关：先打开骨架选择 Modal，用户确认后再生成目录
    // （handleRegenerateOutline 被骨架选择流程替代，不再直接调用）

    // 打开骨架选择 Modal
    const handleOpenSkeletonModal = async () => {
        if (!currentCompanyId || !id) return;
        setSkeletonModalVisible(true);
        setSkeletonLoading(true);
        try {
            const result = await fetchSkeletonCandidates(id, currentCompanyId);
            setSkeletonCandidates(result.candidates);
            // 默认选中推荐的骨架
            if (result.recommended) {
                setSelectedSkeletonId(result.recommended.skeleton_id);
            } else if (result.candidates.length > 0) {
                setSelectedSkeletonId(result.candidates[0].skeleton_id);
            }
        } catch (err) {
            console.error('Fetch skeleton candidates failed:', err);
            message.error('获取骨架候选失败');
        } finally {
            setSkeletonLoading(false);
        }
    };

    // 确认骨架选择并生成目录
    const handleConfirmSkeletonAndGenerate = async () => {
        if (!currentCompanyId || !id || !selectedSkeletonId) return;

        setSkeletonConfirming(true);
        try {
            // 1. 确认骨架选择
            await confirmSkeleton(id, currentCompanyId, selectedSkeletonId);
            setSkeletonModalVisible(false);
            message.success('骨架已确认，正在生成目录...');

            // 2. 启动目录生成
            const hideLoading = message.loading('AI 正在基于选择的骨架生成目录，请耐心等待...', 0);
            try {
                await regenerateOutline(id, currentCompanyId);
                hideLoading();
                message.success('已启动目录生成任务');
                await fetchProject(true);
                await fetchChapters();
            } catch (err) {
                hideLoading();
                throw err;
            }
        } catch (err: unknown) {
            console.error('Skeleton confirm and generate failed:', err);
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const errMsg = (err as any).response?.data?.error || (err as { message?: string }).message || '未知错误';
            message.error('操作失败: ' + errMsg);
        } finally {
            setSkeletonConfirming(false);
        }
    };

    const handleRunVerification = async () => {
        setVerifying(true);
        try {
            await axios.post(`/api/tech-bid/projects/${id}/outline/verify`);
            message.success('已启动豆包 AI 目录审计');
            fetchProject(true);
        } catch (err) {
            message.error('启动核验失败');
        } finally {
            setVerifying(false);
        }
    };

    const handleOptimizeOutline = async () => {
        setVerifying(true);
        try {
            await axios.post(`/api/tech-bid/projects/${id}/outline/optimize`, {
                suggestions: editedVerificationSuggestions || project?.verification_result
            });
            message.success('目录已根据建议完成优化');
            setIsVerificationEditVisible(false);
            fetchProject();
            fetchChapters();
        } catch (err) {
            message.error('优化失败');
        } finally {
            setVerifying(false);
        }
    };

    const handleUpdateChapterName = async (chapterId: string, newName: string) => {
        try {
            await axios.patch(`/api/tech-bid/chapters/${chapterId}`, {
                chapter_name: newName
            });
            message.success('章节名称已更新');
            await fetchChapters();
        } catch (err) {
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const errMsg = (err as any).response?.data?.error || (err as { message?: string }).message || '更新失败';
            message.error('更新章节名称失败: ' + errMsg);
        }
    };

    const handleGenerateOutlineChapters = async () => {
        if (!id || !currentCompanyId) return;
        try {
            await axios.post(`/api/tech-bid/projects/${id}/outline/chapters/generate`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            message.success('已启动 AI 骨架生成');
            fetchProject(true);
        } catch (err) {
            message.error('启动失败');
        }
    };

    const handleConfirmOutlineChapters = async (chapters: string[]) => {
        if (!id || !currentCompanyId) return;
        try {
            await axios.post(`/api/tech-bid/projects/${id}/outline/chapters/confirm`, { chapters }, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            message.success('骨架确认成功');
            fetchProject();
        } catch (err) {
            message.error('操作失败');
        }
    };

    const handleExpandOutlineStructure = async () => {
        if (!id || !currentCompanyId) return;
        try {
            await axios.post(`/api/tech-bid/projects/${id}/outline/expand`, {}, {
                headers: { 'X-Company-Id': currentCompanyId }
            });
            message.success('已启动全文目录逻辑展开');
            fetchProject(true);
        } catch (err) {
            message.error('启动失败');
        }
    };

    const handleForceUnlock = async () => {
        let reason = '';
        Modal.confirm({
            title: '确认强制解锁核验阻断？',
            content: (
                <div style={{ marginTop: 16 }}>
                    <Alert message="强制解锁风险提示" description="此操作将绕过 AI 审计的 BLOCK 状态，通常仅在专家确认 AI 判定有误时使用。操作将被记录在案。" type="warning" showIcon style={{ marginBottom: 12 }} />
                    <TextArea
                        rows={4}
                        placeholder="请输入解锁理由/专家评审结论..."
                        onChange={(e) => { reason = e.target.value; }}
                    />
                </div>
            ),
            width: 500,
            onOk: async () => {
                if (!reason.trim()) {
                    message.warning('请填写解锁理由');
                    return Promise.reject();
                }
                try {
                    await postStep4GateOverride(id!, currentCompanyId!, {
                        reason,
                        operator_id: 'admin_expert_01'
                    });
                    message.success('已记录 Step4 门槛人工放行');
                    await fetchProject(true);
                    await fetchStep4Artifacts();
                } catch (err) {
                    message.error('解锁失败');
                }
            }
        });
    };

    const handleStartManualAudit = () => {
        setEditedVerificationSuggestions(project?.verification_result || '【人工核验意见模板】\n1. 建议补充：\n2. 建议修改：\n3. 核验结论：');
        setIsVerificationEditVisible(true);
    };

    const handleSaveConfig = async () => {
        try {
            await axios.post('/api/settings', {
                settings: [
                    { key: 'doubao_api_key', value: doubaoApiKey },
                    { key: 'doubao_endpoint', value: doubaoEndpoint },
                    { key: 'doubao_model_id', value: doubaoModelId }
                ]
            });
            message.success('豆包 AI 配置已保存');
            setIsConfigModalVisible(false);
        } catch (err) {
            message.error('保存失败');
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
                                .markdown-preview th, .markdown-preview td { border: 1px solid #e2e8f0; padding: 12px; text-align: left; }
                                .markdown-preview th { background: #f8fafc; font-weight: 600; }
                                .markdown-preview p { margin: 16px 0; }
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
            message.error('获取解析内容失败');
        }
    };

    if (loading) return <div style={{ textAlign: 'center', padding: '100px' }}><Spin size="large" description="正在加载工作台..." /></div>;
    if (!project) return <Empty description="未找到项目" />;

    // Normalize current_step: fallback to tender_parse if empty or unknown
    const currentStep = (project.current_step && TECH_STEP_DETAILS[project.current_step])
        ? project.current_step
        : 'tender_parse';
    if (project.current_step !== currentStep) {
        project.current_step = currentStep;
    }

    const currentIndex = STEP_ORDER.indexOf(currentStep);
    const tenderFile = project?.tenderFiles?.find((f: any) => f.file_role === 'tender') || project?.tenderFiles?.[0]; // eslint-disable-line @typescript-eslint/no-explicit-any

    const renderStep4Progress = () => {
        const s4 = project?.step4_status || 'idle';
        let percent = 0;
        let title = 'AI 专家正在处理中...';
        let subText = '任务初始化';
        if (s4 === 'facts_extracting') { percent = 15; title = 'AI 专家正在提取核验事实...'; subText = '与招标文件知识图谱比对，提取核心评分项...'; }
        if (s4 === 'facts_ready') { percent = 30; title = 'AI 专家正在分析事实覆盖...'; subText = '事实库就绪，正在准备目录直生...'; }
        if (s4 === 'generating_outline') { percent = 50; title = 'AI 专家正在构建目录体系...'; subText = '基于招标文件直接生成三级目录，漏项校验兜底...'; }
        if (s4 === 'outline_generating_direct') { percent = 50; title = 'AI 专家正在直生目录...'; subText = '模型直接分析招标文件，生成专业三级目录...'; }
        if (s4 === 'outline_ready') { percent = 65; title = 'AI 专家正在准备目录审计...'; subText = '初步目录已生成，正在执行行业语义审计...'; }
        if (s4 === 'auditing_outline') { percent = 80; title = 'AI 专家正在执行目录审计...'; subText = '审计执行中：检查评分项覆盖率与行业规范响应情况...'; }
        if (s4 === 'refining_outline') { percent = 90; title = 'AI 专家正在修补目录结构...'; subText = '审计结果已出，正在执行定点原子级修复 (Patching)...'; }
        if (s4 === 'audit_ready' || s4 === 'refine_ready' || s4 === 'optimized_ready') { percent = 100; title = 'AI 专家已完成目录构建'; subText = '目录体系构建完成，通过行业合规审计。'; }
        if (s4 === 'failed') { percent = 100; title = '目录构建失败'; subText = '处理失败，请查看错误信息并重试。'; }

        const detailText = project.current_step_status === 'running'
            ? subText
            : (project.last_error_message || subText || '深度处理中，预计还需 30-60 秒');

        const apiProg = step4RunStatus?.progress;
        const circlePercent =
            typeof apiProg === 'number' && apiProg > 0 ? Math.min(99, apiProg) : percent;
        type HistoryRunItem = { stage?: string; agent?: string; agent_name?: string; status?: string; duration_ms?: number };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const historyRuns: HistoryRunItem[] = (step4RunStatus?.stages?.length ? step4RunStatus.stages : step4AgentRuns) as any[];

        return (
            <div style={{ padding: '60px 0', textAlign: 'center' }}>
                <Progress type="circle" percent={circlePercent} status="active" strokeColor={{ '0%': '#108ee9', '100%': '#87d068' }} />
                <Title level={4} style={{ marginTop: 24 }}>{title}</Title>
                <div style={{ marginBottom: 16 }}><Badge status="processing" text={subText} /></div>
                <Text type="secondary">{detailText}</Text>
                {step4RunStatus?.current_agent && (
                    <div style={{ marginTop: 8 }}>
                        <Text type="secondary">当前 Agent：{step4RunStatus.current_agent}</Text>
                    </div>
                )}
                {historyRuns.length > 0 && (
                    <div style={{ maxWidth: 560, margin: '28px auto 0', textAlign: 'left' }}>
                        <Text strong>Agent 执行时间线</Text>
                        <Timeline style={{ marginTop: 16 }}>
                            {historyRuns.map((s, idx) => (
                                <Timeline.Item
                                    key={s.stage ? `${s.stage}-${idx}` : `${s.agent_name || 'agent'}-${idx}`}
                                    color={
                                        s.status === 'done' || s.status === 'completed' ? 'green'
                                            : s.status === 'failed' ? 'red'
                                                : s.status === 'running' ? 'blue' : 'gray'
                                    }
                                >
                                    <Text>{s.agent || s.agent_name}</Text>
                                    <Text type="secondary"> · {s.stage}</Text>
                                    <div style={{ fontSize: 12, color: '#94a3b8' }}>
                                        {s.status}
                                        {typeof s.duration_ms === 'number' ? ` · ${s.duration_ms} ms` : ''}
                                    </div>
                                </Timeline.Item>
                            ))}
                        </Timeline>
                    </div>
                )}
            </div>
        );
    };
    if (loading || !project) {
        return (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: 400, background: '#fff', borderRadius: 16 }}>
                <Spin size="large" description="AI 数字化引擎启动中..." />
                <div style={{ marginTop: 24, textAlign: 'center', color: '#64748b' }}>
                    <Title level={5}>加载项目工作台中</Title>
                    <Text type="secondary">正在拉取技术标项目状态机与事实库数据...</Text>
                </div>
            </div>
        );
    }

    const renderStepContent = () => {
        if (project.current_step_status === 'waiting_for_approval') {
            if (project.current_step === 'outline_generation' && structurePlan) {
                return (
                    <Suspense fallback={<ComponentLoadingPlaceholder />}>
                        <StructurePlanReviewPanel
                            plan={structurePlan}
                            onApprove={handleApproveStructurePlan}
                            onReject={handleRejectStructurePlan}
                            loading={approvingPlan}
                        />
                    </Suspense>
                );
            }
            if (project.current_step === 'outline_generation' && !structurePlan) {
                return (
                    <Result
                        status="warning"
                        title="结构计划状态异常"
                        subTitle={step4ArtifactsWarning || '项目显示待审批，但当前没有可用的结构计划，请刷新或重新触发目录生成。'}
                        extra={[
                            <Button key="refresh" type="primary" onClick={() => void fetchProject(true)}>
                                刷新状态
                            </Button>,
                        ]}
                    />
                );
            }
        }

        if (project.current_step_status === 'running') {
            if (project.current_step === 'outline_generation') return renderStep4Progress();
            return (
                <div style={{ padding: '60px 0', textAlign: 'center' }}>
                    <Progress type="circle" percent={project.current_progress || 45} status="active" strokeColor={{ '0%': '#108ee9', '100%': '#87d068' }} />
                    <Title level={4} style={{ marginTop: 24 }}>AI 专家正在处理中...</Title>
                    <div style={{ marginBottom: 16 }}><Badge status="processing" text={`当前处理进度: ${project.current_progress || 45}%`} /></div>
                    <Text type="secondary">{project.last_error_message || '深度处理中，预计还需 1-2 分钟'}</Text>
                </div>
            );
        }

        if (project.current_step_status === 'failed') {
            return (
                <Result
                    status="error"
                    title="当前步骤执行失败"
                    subTitle={project.last_error_message || '系统在处理过程中遇到错误，请查看后台日志并重新执行。'}
                    extra={[
                        <Button key="refresh" type="primary" onClick={() => fetchProject()}>
                            刷新状态
                        </Button>,
                        <Button key="back" onClick={handleGoBack}>
                            回退并重选路线
                        </Button>
                    ]}
                />
            );
        }

        switch (project.current_step) {
            case 'tender_parse': {
                if (!tenderFile) {
                    return (
                        <div style={{ padding: '20px' }}>
                            <Alert
                                message="技术标制作第一步：上传招标文件"
                                description="请上传本项目的招标文件（PDF/Word），系统将自动进行深度数字化还原与技术要求提取。"
                                type="info"
                                showIcon
                                style={{ marginBottom: 24 }}
                            />
                            <Dragger
                                action="/api/files/upload"
                                headers={{ 'X-Company-Id': currentCompanyId || '' }}
                                data={{ source_module: 'tender' }}
                                onChange={handleFileUpload}
                                multiple={false}
                                showUploadList={false}
                            >
                                <p className="ant-upload-drag-icon"><InboxOutlined /></p>
                                <p className="ant-upload-text">点击或拖拽招标文件进行解析</p>
                                <p className="ant-upload-hint">支持 PDF/DOCX，建议单文件大小不超过 50MB</p>
                            </Dragger>
                        </div>
                    );
                }
                return (
                    <Result
                        status={project.current_step_status === 'success' ? "success" : "info"}
                        title={project.current_step_status === 'success' ? "招标文件深度提取已完成" : "招标文件预处理完成"}
                        subTitle={
                            <span>
                                当前文件：<Text strong>{tenderFile.file_name}</Text>，
                                {project.current_step_status === 'success'
                                    ? 'AI 专家已完成技术要点数字化提取。'
                                    : '已完成 OCR 识别与结构化存储。等待进一步提取技术指标。'}
                            </span>
                        }
                        extra={[
                            project.current_step_status === 'success' ? (
                                <Button type="primary" key="confirm" size="large" onClick={handleConfirm}>
                                    确认解析结果并开始项目画像
                                </Button>
                            ) : (
                                <Button type="primary" key="run" size="large" onClick={handleRunStep}>
                                    开始提取技术要求
                                </Button>
                            ),
                            <Upload key="upload" action="/api/files/upload" headers={{ 'X-Company-Id': currentCompanyId || '' }} data={{ source_module: 'tender' }} onChange={handleFileUpload} showUploadList={false}>
                                <Button size="large">重新上传文件</Button>
                            </Upload>
                        ]}
                    />
                );
            }
            case 'project_profile': {
                return (
                    <Suspense fallback={<ComponentLoadingPlaceholder />}>
                        <ProjectProfilePanel
                            project={project}
                            onConfirm={handleConfirm}
                            onRefresh={() => fetchProject()}
                        />
                    </Suspense>
                );
            }
            case 'route_planning': {
                return (
                    <div style={{ padding: '20px' }}>
                        <Title level={4}>施工组织路线规划 (编制策略)</Title>
                        <Paragraph>基于项目特征，AI 推荐了以下 3 种施工路线方案。确定路线后，系统将为您生成完整的三层目录树。</Paragraph>

                        {project.current_step_status === 'success' ? (
                            <Result
                                status="success"
                                title="路线规划已确定"
                                subTitle="系统已根据所选路线规划了编制策略。点击下一步进入目录树生成。"
                                extra={[
                                    <Button type="primary" size="large" onClick={handleConfirm}>进入目录生成</Button>,
                                    <Button key="retry" onClick={() => fetchProject()}>重选路线</Button>
                                ]}
                            />
                        ) : (
                            <Row gutter={[24, 24]}>
                                {[
                                    { id: 'std_conv', name: '常规施工组织路线', desc: '基于企业标准模板，强调工期可控与施工安全。', tags: ['稳健', '标准'], chapters: ['编制依据', '工程概况', '施工部署', '质量与安全'] },
                                    { id: 'green_std', name: '绿色低碳示范路线', desc: '增加节能减排与扬尘控制专项章节。', tags: ['政策导向', '高标准'], chapters: ['编制依据', '资源节约与利用', '环保施工'] },
                                    { id: 'smart_site', name: '智慧工地数字化路线', desc: '强调全流程数字化审计与智能预警。', tags: ['前沿', 'BIM'], chapters: ['编制依据', '数字化模型应用', '自动化监测'] }
                                ].map(route => (
                                    <Col span={8} key={route.id}>
                                        <Card
                                            hoverable
                                            title={route.name}
                                            actions={[
                                                <Button
                                                    type="link"
                                                    key="sel"
                                                    loading={selectingRoute}
                                                    onClick={(e) => {
                                                        e.stopPropagation();
                                                        console.log('Button clicked:', route.name);
                                                        handleSelectRoute(route);
                                                    }}
                                                >
                                                    选择此路线
                                                </Button>
                                            ]}
                                        >
                                            {route.tags.map(t => <Tag color="blue" key={t}>{t}</Tag>)}
                                            <div style={{ marginTop: 12, fontSize: 13, color: '#64748b' }}>{route.desc}</div>
                                            <div style={{ marginTop: 12 }}>
                                                {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                                                {route.chapters.map((c: any) => <Tag key={c}>{c}</Tag>)}
                                            </div>
                                        </Card>
                                    </Col>
                                ))}
                            </Row>
                        )}
                    </div>
                );
            }
            case 'outline_generation': {
                return (
                    <Suspense fallback={<ComponentLoadingPlaceholder />}>
                        <OutlineGenerationPanel
                            project={project}
                            displayChapters={displayChapters}
                            structurePlan={structurePlan}
                            step4ArtifactsWarning={step4ArtifactsWarning}
                            onOpenAuditFact={handleOpenAuditFact}
                            outlineVersions={outlineVersions}
                            selectedOutlineVerId={selectedOutlineVerId}
                            onSelectOutlineVerId={setSelectedOutlineVerId}
                            selectingOutlineVer={selectingOutlineVer}
                            onSwitchOutlineVersion={async () => {
                                if (!selectedOutlineVerId || !id || !currentCompanyId) return;
                                setSelectingOutlineVer(true);
                                try {
                                    await selectOutlineVersion(id, currentCompanyId, selectedOutlineVerId);
                                    message.success('已切换版本并同步章节计划');
                                    await fetchChapters();
                                    await fetchProject(true);
                                    await fetchStep4Artifacts();
                                } catch {
                                    message.error('切换失败');
                                } finally {
                                    setSelectingOutlineVer(false);
                                }
                            }}
                            step4ArtifactsLoading={step4ArtifactsLoading}
                            step4Mappings={step4Mappings}
                            step4Coverage={step4Coverage}
                            step4Requirements={step4Requirements}
                            step4FullResponse={step4FullResponse}
                            step4ConflictAudit={step4ConflictAudit}
                            step4FactCandidates={step4FactCandidates}
                            selectedOutlineVersionDetail={selectedOutlineVersionDetail}
                            recommendedOutlineVersionDetail={recommendedOutlineVersionDetail}
                            step4DrawerOpen={step4DrawerOpen}
                            onOpenDrawer={() => setStep4DrawerOpen(true)}
                            onDrawerClose={() => setStep4DrawerOpen(false)}
                            step4HighlightFactId={step4HighlightFactId}
                            step4AgentRuns={step4AgentRuns}
                            step4ApprovalLogs={step4ApprovalLogs}
                            step4RunHistory={step4RunHistory}
                            onUpdateChapterName={handleUpdateChapterName}
                            // 两步走支持
                            onGenerateChapters={handleGenerateOutlineChapters}
                            onConfirmChapters={handleConfirmOutlineChapters}
                            onExpandStructure={handleExpandOutlineStructure}
                        />
                    </Suspense>
                );
            }
            case 'outline_verification': {
                return (
                    <OutlineVerificationPanel
                        project={project}
                        verifying={verifying}
                        onConfirm={handleConfirm}
                        onRunVerification={handleRunVerification}
                        onStartManualAudit={handleStartManualAudit}
                        onForceUnlock={handleForceUnlock}
                        onOpenFactsDrawer={() => setIsFactsDrawerVisible(true)}
                        step4ArtifactsLoading={step4ArtifactsLoading}
                        step4Mappings={step4Mappings}
                        step4Coverage={step4Coverage}
                        step4Requirements={step4Requirements}
                        step4FullResponse={step4FullResponse}
                        step4ConflictAudit={step4ConflictAudit}
                        step4FactCandidates={step4FactCandidates}
                        outlineVersions={outlineVersions}
                        selectedOutlineVerId={selectedOutlineVerId}
                        selectedOutlineVersionDetail={selectedOutlineVersionDetail}
                        recommendedOutlineVersionDetail={recommendedOutlineVersionDetail}
                        step4DrawerOpen={step4DrawerOpen}
                        onOpenDrawer={() => setStep4DrawerOpen(true)}
                        onDrawerClose={() => setStep4DrawerOpen(false)}
                        step4HighlightFactId={step4HighlightFactId}
                    />
                );
            }
            case 'content_generation': {
                return (
                    <ContentGenerationPanel
                        displayChapters={displayChapters}
                        editingChapter={editingChapter}
                        onSelectChapter={(ch) => setEditingChapter(ch)}
                        onGenerateChapter={(ch) => { setGeneratingChapter(ch); setIsGenerateModalVisible(true); }}
                        onEditChapter={(md) => { setEditorContent(md); setIsEditorVisible(true); }}
                        onConfirm={handleConfirm}
                        onBatchGenerate={handleBatchGenerateContent}
                        batchGeneration={batchGeneration}
                    />
                );
            }
            case 'risk_review': {
                return (
                    <Suspense fallback={<ComponentLoadingPlaceholder />}>
                        <RiskReviewPanel
                            projectId={id!}
                            risks={risks}
                            auditLoading={auditLoading}
                            onAuditLoadingChange={setAuditLoading}
                            onRefreshRisks={fetchRisks}
                            onConfirm={handleConfirm}
                        />
                    </Suspense>
                );
            }
            case 'output_finalize': {
                return (
                    <div style={{ textAlign: 'center', padding: '48px 0' }}>
                        <Result
                            status="success"
                            title="所有技术标环节已完成！"
                            subTitle="项目画像、章节生成及安全合规性检查均已通过。现在可以导出定稿版本。"
                            extra={[
                                <Button type="primary" key="word" icon={<FileTextOutlined />} size="large" loading={exportingWord} onClick={handleExportWord}>导出 Word 格式 (标准版)</Button>,
                                <Button key="pdf" icon={<CloudSyncOutlined />} size="large" disabled>同步存证并导出 PDF</Button>
                            ]}
                        />
                    </div>
                );
            }
            default:
                return (
                    <Result
                        icon={<RocketOutlined style={{ color: '#1890ff' }} />}
                        title="功能开发中"
                        subTitle={`当前步骤 [${project.current_step}] 的 UI 正在按照 CTO 方案加速实现中。`}
                        extra={<Button type="primary" onClick={handleConfirm}>直接跳过 (模拟执行)</Button>}
                    />
                );
        }
    };

    return (
        <div style={{ background: '#f8fafc', minHeight: 'calc(100vh - 150px)', margin: '-24px', padding: '24px' }}>
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
                <Space>
                    <Button icon={<LeftOutlined />} onClick={() => navigate('/tech-bid-projects')} type="text" />
                    <div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
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
                            <Tag color="blue">{project.profession || '通用施工'}</Tag>
                            {project.current_step_status === 'running' && (
                                <Badge status="processing" text="处理中" />
                            )}
                        </div>
                        <Text type="secondary" style={{ fontSize: 13 }}>项目 ID: {project.id} | 创建时间: {new Date(project.created_at).toLocaleDateString()}</Text>
                    </div>
                </Space>
                <Space>
                    {currentIndex > 0 && <Button icon={<AntdRollbackOutlined />} onClick={handleGoBack}>回退上一步</Button>}
                    {project && (
                        project.current_step_status === 'success' ||
                        (project.current_step === 'tender_parse' && (tenderFile || project.tender_file_id)) ||
                        (project.current_step === 'project_profile' && project.profile) ||
                        (project.current_step === 'route_planning' && (project.chapterPlans?.length || 0) > 0) ||
                        (project.current_step === 'outline_generation' && (displayChapters?.length || 0) > 0) ||
                        (project.current_step === 'outline_verification' && project.verification_result) ||
                        (project.current_step === 'content_generation' && (displayChapters?.some((c: any) => c.generation_status === "completed")))
                    ) && (
                            <Button type="primary" onClick={handleConfirm}>
                                确认并执行下一步
                            </Button>
                        )}
                </Space>
            </div>

            <Row gutter={[24, 24]}>
                {/* Left Sidebar Steps */}
                <Col xs={24} lg={3}>
                    <Card styles={{ body: { padding: '24px 8px' } }} variant="borderless" style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
                        <Steps
                            orientation={screens.lg ? "vertical" : "horizontal"}
                            current={currentIndex}
                            size="small"
                            items={STEP_ORDER.map((key, idx) => {
                                const isFinished = currentIndex > idx;
                                const isActive = currentIndex === idx;
                                const isFailed = isActive && (project.current_step_status === 'failed' || project.current_step_status === 'warning');

                                return {
                                    title: (
                                        <Tooltip title={TECH_STEP_DETAILS[key].description} placement="right">
                                            <span style={{
                                                color: isFailed ? '#ff4d4f' : (isFinished || isActive ? '#1890ff' : 'inherit'),
                                                fontWeight: isActive ? 600 : 400,
                                                fontSize: 13,
                                                cursor: 'help'
                                            }}>
                                                {TECH_STEP_DETAILS[key].label}
                                            </span>
                                        </Tooltip>
                                    ),
                                    status: isFinished ? 'finish' : (isActive ? (isFailed ? 'error' : 'process') : 'wait'),
                                    icon: TECH_STEP_DETAILS[key].icon
                                };
                            })}
                        />
                    </Card>
                </Col>

                {/* Main Content Area */}
                <Col xs={24} lg={21}>
                    <Card
                        styles={{ body: { minHeight: 480, padding: 32 } }}
                        variant="borderless"
                        style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}
                        title={
                            <Space align="center">
                                <div style={{ background: '#e6f4ff', padding: 8, borderRadius: 8, display: 'flex' }}>
                                    {TECH_STEP_DETAILS[project.current_step].icon}
                                </div>
                                <Text strong style={{ fontSize: 18 }}>{TECH_STEP_DETAILS[project.current_step].label}</Text>
                            </Space>
                        }
                        extra={
                            project.current_step === 'outline_generation' && (
                                <Space>
                                    <Button icon={<PlusCircleOutlined />} onClick={handleOpenSkeletonModal} size="small" type="primary">
                                        选择骨架并生成
                                    </Button>
                                    <Button icon={<DatabaseOutlined />} onClick={() => setIsFactsDrawerVisible(true)}>查看原文核验库</Button>
                                </Space>
                            )
                        }
                    >
                        {renderStepContent()}
                    </Card>

                    {/* 骨架选择确认 Modal */}
                    <Modal
                        title={<Space><BulbOutlined /> 选择行业骨架（Human-in-the-loop）</Space>}
                        open={skeletonModalVisible}
                        onCancel={() => {
                            setSkeletonModalVisible(false);
                            setSkeletonSearchText('');
                        }}
                        footer={
                            <Space>
                                <Button onClick={() => {
                                    setSkeletonModalVisible(false);
                                    setSkeletonSearchText('');
                                }}>取消</Button>
                                <Button
                                    type="primary"
                                    loading={skeletonConfirming}
                                    disabled={!selectedSkeletonId}
                                    onClick={handleConfirmSkeletonAndGenerate}
                                >
                                    确认骨架并生成目录
                                </Button>
                            </Space>
                        }
                        width={800}
                        style={{ maxHeight: '90vh' }}
                        styles={{ body: { maxHeight: 'calc(90vh - 200px)', overflow: 'auto' } }}
                    >
                        <Alert
                            type="info"
                            showIcon
                            message="骨架选择说明"
                            description="AI 基于招标文件特征推荐最匹配的骨架分类。您可以查看各骨架的评分明细，确认或调整选择后再生成目录。"
                            style={{ marginBottom: 16 }}
                        />

                        {/* 搜索框 */}
                        <Input.Search
                            placeholder="搜索二级分类行业骨架名称..."
                            value={skeletonSearchText}
                            onChange={(e) => setSkeletonSearchText(e.target.value)}
                            style={{ marginBottom: 16 }}
                            allowClear
                        />

                        {skeletonLoading ? (
                            <div style={{ textAlign: 'center', padding: 40 }}>
                                <Spin description="正在获取骨架候选..." />
                            </div>
                        ) : (
                            <List
                                dataSource={skeletonCandidates.filter(candidate =>
                                    skeletonSearchText.trim() === '' ||
                                    candidate.industry_name.toLowerCase().includes(skeletonSearchText.toLowerCase())
                                )}
                                renderItem={(candidate) => (
                                    <List.Item
                                        key={candidate.skeleton_id}
                                        onClick={() => setSelectedSkeletonId(candidate.skeleton_id)}
                                        style={{
                                            cursor: 'pointer',
                                            background: selectedSkeletonId === candidate.skeleton_id ? '#e6f7ff' : 'transparent',
                                            border: selectedSkeletonId === candidate.skeleton_id ? '2px solid #1890ff' : '1px solid #f0f0f0',
                                            borderRadius: 8,
                                            marginBottom: 8,
                                            padding: 16,
                                        }}
                                    >
                                        <div style={{ width: '100%' }}>
                                            <Space style={{ marginBottom: 8 }}>
                                                <Radio checked={selectedSkeletonId === candidate.skeleton_id} />
                                                <Text strong style={{ fontSize: 16 }}>{candidate.industry_name}</Text>
                                                {candidate.recommended && <Tag color="green">推荐</Tag>}
                                                <Tag color={candidate.confidence === 'high' ? 'blue' : candidate.confidence === 'medium' ? 'orange' : 'default'}>
                                                    置信度: {candidate.confidence === 'high' ? '高' : candidate.confidence === 'medium' ? '中' : '低'}
                                                </Tag>
                                                <Text type="secondary">总分: {candidate.match_score.toFixed(1)}</Text>
                                            </Space>

                                            {/* 评分明细 */}
                                            <Row gutter={16} style={{ marginBottom: 8 }}>
                                                <Col span={6}>
                                                    <Text type="secondary" style={{ fontSize: 12 }}>关键词匹配</Text>
                                                    <div>
                                                        <Progress percent={candidate.score_breakdown.keyword_score} size="small" strokeColor="#1890ff" />
                                                        <Text type="secondary" style={{ fontSize: 11 }}>30%</Text>
                                                    </div>
                                                </Col>
                                                <Col span={6}>
                                                    <Text type="secondary" style={{ fontSize: 12 }}>结构相似度</Text>
                                                    <div>
                                                        <Progress percent={candidate.score_breakdown.structure_score} size="small" strokeColor="#52c41a" />
                                                        <Text type="secondary" style={{ fontSize: 11 }}>30%</Text>
                                                    </div>
                                                </Col>
                                                <Col span={6}>
                                                    <Text type="secondary" style={{ fontSize: 12 }}>特殊章节匹配</Text>
                                                    <div>
                                                        <Progress percent={candidate.score_breakdown.special_chapter_score} size="small" strokeColor="#faad14" />
                                                        <Text type="secondary" style={{ fontSize: 11 }}>25%</Text>
                                                    </div>
                                                </Col>
                                                <Col span={6}>
                                                    <Text type="secondary" style={{ fontSize: 12 }}>历史相似度</Text>
                                                    <div>
                                                        <Progress percent={candidate.score_breakdown.history_score} size="small" strokeColor="#f5222d" />
                                                        <Text type="secondary" style={{ fontSize: 11 }}>15%</Text>
                                                    </div>
                                                </Col>
                                            </Row>

                                            {/* 匹配理由 */}
                                            <Space wrap>
                                                {candidate.match_reasons.map((reason, idx) => (
                                                    <Tag key={idx} color="purple">{reason}</Tag>
                                                ))}
                                            </Space>
                                        </div>
                                    </List.Item>
                                )}
                            />
                        )}
                    </Modal>
                </Col>
            </Row>

            {project.current_step === 'content_generation' && (
                <Row gutter={24} style={{ marginTop: 24 }}>
                    <Col span={24}>
                        <Card title={<Space><DatabaseOutlined /> 技术标资源库 (CTO 推荐资源)</Space>} variant="borderless" style={{ borderRadius: 12 }}>
                            <Row gutter={16}>
                                {[
                                    { title: '设备库', count: 12, icon: <ToolOutlined style={{ color: '#1890ff' }} />, desc: '特种作业吊装、焊接自动化设备' },
                                    { title: '工法库', count: 8, icon: <FireOutlined style={{ color: '#faad14' }} />, desc: '冬季大体积混凝土、抗冻外加剂工法' },
                                    { title: '风险库', count: 5, icon: <WarningOutlined style={{ color: '#ff4d4f' }} />, desc: '历史类似项目的渗漏风险与地质复杂响应' },
                                    { title: '人员库', count: 24, icon: <TeamOutlined style={{ color: '#52c41a' }} />, desc: '项目经理李某某(壹级建造师)、总工唐某' }
                                ].map(lib => (
                                    <Col span={6} key={lib.title}>
                                        <Card
                                            hoverable
                                            styles={{ body: { padding: 16 } }}
                                            onClick={() => {
                                                setSelectedResource(lib);
                                                setIsResourceDrawerVisible(true);
                                            }}
                                        >
                                            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 8 }}>
                                                {lib.icon}
                                                <Text strong style={{ marginLeft: 8 }}>{lib.title}</Text>
                                            </div>
                                            <Statistic value={lib.count} suffix="条匹配" valueStyle={{ fontSize: 20 }} />
                                            <div style={{ fontSize: 12, color: '#94a3b8', marginTop: 8 }}>{lib.desc}</div>
                                            <div style={{ marginTop: 12, textAlign: 'right' }}>
                                                <Button type="link" size="small" style={{ fontSize: 12, padding: 0 }}>点击查看详情</Button>
                                            </div>
                                        </Card>
                                    </Col>
                                ))}
                            </Row>
                        </Card>
                    </Col>
                </Row>
            )}

            <Modal
                title={`AI 配置生成: ${generatingChapter?.chapter_name}`}
                open={isGenerateModalVisible}
                onCancel={() => setIsGenerateModalVisible(false)}
                onOk={async () => {
                    setIsGenerateModalVisible(false);
                    // 模拟生成
                    message.loading('AI 正在编写正文...', 0);
                    try {
                        await axios.post(`/api/tech-bid/chapters/${generatingChapter.id}/generate`);
                        message.destroy();
                        message.success('正文生成成功');
                        fetchChapters();
                    } catch (err) {
                        message.destroy();
                        message.error('生成失败');
                    }
                }}
                okText="开始编写"
                width={600}
            >
                <Alert
                    message="知识引擎正在加载"
                    description="系统将自动从公司设备库、工法库提取上下文，并结合招标文件要求生成符合专业标准的正文。"
                    type="info"
                    showIcon
                    style={{ marginBottom: 20 }}
                />
                <Form layout="vertical">
                    <Form.Item label="写作倾向控制">
                        <Radio.Group defaultValue="balanced" onChange={(e) => setGenConfig({ ...genConfig, tone: e.target.value })}>
                            <Radio value="technical">硬核技术型</Radio>
                            <Radio value="balanced">专业均衡型</Radio>
                            <Radio value="concise">简练清晰型</Radio>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item label="深度等级">
                        <Select defaultValue="detailed" style={{ width: '100%' }} onChange={(val) => setGenConfig({ ...genConfig, depth: val })}>
                            <Option value="detailed">精编写 (包含施工细节与工艺流程)</Option>
                            <Option value="outline">大纲式 (仅包含要点与核心指标)</Option>
                        </Select>
                    </Form.Item>
                    <Form.Item label="是否包含图表建议">
                        <Checkbox defaultChecked>在适当位置插入插图参考建议</Checkbox>
                    </Form.Item>
                </Form>
            </Modal>

            <Modal
                title={`编辑正文: ${editingChapter?.chapter_name}`}
                open={isEditorVisible}
                onCancel={() => setIsEditorVisible(false)}
                onOk={async () => {
                    try {
                        await axios.put(`/api/tech-bid/chapters/${editingChapter.id}/content`, { content_md: editorContent });
                        message.success('保存成功');
                        setIsEditorVisible(false);
                        fetchChapters();
                    } catch (err) {
                        message.error('保存失败');
                    }
                }}
                width={1000}
                style={{ top: 20 }}
                okText="保存并关闭"
            >
                <Row gutter={24}>
                    <Col span={12}>
                        <Paragraph strong>Markdown 编辑区</Paragraph>
                        <TextArea
                            rows={24}
                            value={editorContent}
                            onChange={(e) => setEditorContent(e.target.value)}
                            style={{ fontFamily: 'monospace', fontSize: 13 }}
                            placeholder="请输入 Markdown 格式的内容..."
                        />
                    </Col>
                    <Col span={12}>
                        <Paragraph strong>预览效果</Paragraph>
                        <div style={{
                            height: 524,
                            overflow: 'auto',
                            padding: 20,
                            border: '1px solid #f0f0f0',
                            borderRadius: 8,
                            background: '#fafafa'
                        }}>
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{editorContent}</ReactMarkdown>
                        </div>
                    </Col>
                </Row>
            </Modal>

            <Drawer
                title={<Space>{selectedResource?.icon} {selectedResource?.title} 详细清单</Space>}
                placement="right"
                size={500}
                onClose={() => setIsResourceDrawerVisible(false)}
                open={isResourceDrawerVisible}
                extra={
                    <Space>
                        <Button onClick={() => setIsResourceDrawerVisible(false)}>关闭</Button>
                        <Button type="primary">确认启用这些上下文</Button>
                    </Space>
                }
            >
                <Alert
                    message="AI 检索反馈"
                    description={`系统已自动从“${selectedResource?.title}”中检索出与本项目工程特征（${project.project_profile_id || '建筑/工业/市政'}）高度相关的核心项。您可以勾选其中的条目作为 AI 生成章节内容的显式参考背景。`}
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                />
                <List
                    itemLayout="vertical"
                    dataSource={
                        selectedResource?.title === '设备库' ? [
                            { name: '中联重科 ZAT4000V 全地面起重机', desc: '400吨位，适用于本项目的主体钢结构吊装。', tag: '高匹配' },
                            { name: '自动多点焊接工作站 (12轴)', desc: '支持现场复杂节点结构的高精度自动焊接。', tag: '推荐' },
                            { name: '智能塔吊防碰撞系统 (V3.0)', desc: '针对本项目狭窄场地的安全监控需求。', tag: '安全必选' },
                            { name: '模块化集装箱式搅拌站', desc: '配套快速部署，适合项目初期地基处理。', tag: '匹配' }
                        ] : selectedResource?.title === '工法库' ? [
                            { name: '超厚大体积混凝土单次浇注技术', desc: '针对主楼基础底板的温控与抗裂优化方案。', tag: '核心' },
                            { name: '高强钢结构预应力张拉集成工法', desc: '提升长跨度结构的整体稳定性。', tag: '技术标加分项' },
                            { name: '装配式预制预留高精度锚栓系统', desc: '缩短工期并提高螺栓孔位的一致性。', tag: '推荐' }
                        ] : selectedResource?.title === '风险库' ? [
                            { name: '深基坑作业过程中的流砂/管涌风险', desc: '本项目周边地勘显示水位较高，需重点防范。', tag: '关键预案' },
                            { name: '冬雨季施工期间的质量保障风险', desc: '主要针对混凝土凝结与钢构防腐。', tag: '气候风险' },
                            { name: '临近既有地铁线路的微振动控制', desc: '本项目距离3号线仅 50 米，技术规格要求极高。', tag: '硬性红线' }
                        ] : [
                            { name: '李某某 (壹级注册建造师)', desc: '15年行业经验，曾主导类似 3 个超高层项目。', tag: '项目经理' },
                            { name: '张工 (高级工程师 - 钢结构)', desc: '精通 BIM 深度应用，擅长吊装逻辑优化。', tag: '技术总工' },
                            { name: '王师傅 (全国技术能手 - 特种焊接)', desc: '持证高级技师，解决关键节点工艺难题。', tag: '核心骨干' }
                        ]
                    }
                    renderItem={item => (
                        <List.Item
                            actions={[<Button type="link" size="small">作为参考引用</Button>, <Button type="link" size="small" danger>排除此项</Button>]}
                        >
                            <List.Item.Meta
                                title={<span>{item.name} <Tag color="blue" style={{ marginLeft: 8 }}>{item.tag}</Tag></span>}
                                description={item.desc}
                            />
                        </List.Item>
                    )}
                />
            </Drawer>

            <Drawer
                title={<Space><DatabaseOutlined /> 招标文件核心核验事实库 (AI 提取)</Space>}
                size={650}
                onClose={() => setIsFactsDrawerVisible(false)}
                open={isFactsDrawerVisible}
                extra={
                    <Space>
                        <Button onClick={() => setIsFactsDrawerVisible(false)}>关闭</Button>
                    </Space>
                }
            >
                {project?.facts && project.facts.length > 0 ? (
                    <div style={{ padding: '0 8px' }}>
                        <Alert
                            message="事实库说明"
                            description="以下是由 AI 抽取的高价值核验事实，作为目录生成和审计的原始依据。涵盖评分标准、强制性技术要求及项目难点。"
                            type="info"
                            showIcon
                            style={{ marginBottom: 24 }}
                        />

                        <Tabs defaultActiveKey="score">
                            <Tabs.TabPane tab="关键评分项" key="score">
                                <List
                                    itemLayout="vertical"
                                    dataSource={project.facts.filter((f: any) => f.fact_type === 'score_item')}
                                    renderItem={(f: any) => (
                                        <List.Item key={f.id}>
                                            <Space style={{ marginBottom: 8 }}>
                                                <Tag color="gold">得分点</Tag>
                                                <Text strong>{f.fact_name}</Text>
                                                {f.score_value && <Badge count={`${f.score_value}分`} style={{ backgroundColor: '#52c41a' }} />}
                                            </Space>
                                            <div style={{ background: '#fffbe6', padding: '12px 16px', borderRadius: 8, fontSize: 13, border: '1px solid #ffe58f' }}>
                                                {f.fact_content}
                                            </div>
                                            {f.source_text && (
                                                <div style={{ marginTop: 8, fontSize: 12, color: '#8c8c8c' }}>
                                                    <Text type="secondary">原文依据: </Text>
                                                    <Text type="secondary" italic>“{f.source_text}”</Text>
                                                </div>
                                            )}
                                        </List.Item>
                                    )}
                                />
                            </Tabs.TabPane>
                            <Tabs.TabPane tab="强制技术规范" key="spec">
                                <List
                                    dataSource={project.facts.filter((f: any) => f.fact_type === 'mandatory_spec')}
                                    renderItem={(f: any) => (
                                        <List.Item key={f.id}>
                                            <Card size="small" style={{ width: '100%', borderLeft: '4px solid #ff4d4f' }}>
                                                <Space style={{ marginBottom: 4 }}>
                                                    <WarningOutlined style={{ color: '#ff4d4f' }} />
                                                    <Text strong>{f.fact_name}</Text>
                                                </Space>
                                                <div style={{ fontSize: 13, color: '#595959' }}>{f.fact_content}</div>
                                            </Card>
                                        </List.Item>
                                    )}
                                />
                            </Tabs.TabPane>
                            <Tabs.TabPane tab="项目难点特性" key="char">
                                <List
                                    dataSource={project.facts.filter((f: any) => f.fact_type === 'project_characteristic')}
                                    renderItem={(f: any) => (
                                        <List.Item key={f.id}>
                                            <Card size="small" style={{ width: '100%', borderLeft: '4px solid #1890ff' }}>
                                                <Space style={{ marginBottom: 4 }}>
                                                    <BulbOutlined style={{ color: '#1890ff' }} />
                                                    <Text strong>{f.fact_name}</Text>
                                                </Space>
                                                <div style={{ fontSize: 13, color: '#595959' }}>{f.fact_content}</div>
                                            </Card>
                                        </List.Item>
                                    )}
                                />
                            </Tabs.TabPane>
                        </Tabs>
                    </div>
                ) : (
                    <Empty description="暂无抽取到的核验事实" />
                )}
            </Drawer>

            <Modal
                title="编辑核验建议"
                open={isVerificationEditVisible}
                onOk={handleOptimizeOutline}
                onCancel={() => setIsVerificationEditVisible(false)}
                confirmLoading={verifying}
                width={800}
                okText="保存并优化目录"
            >
                <Paragraph type="secondary">您可以根据实际情况修正 AI 的建议。点击确定后，AI 将根据此建议重新调整目录结构。</Paragraph>
                <TextArea
                    rows={15}
                    value={editedVerificationSuggestions}
                    onChange={(e) => setEditedVerificationSuggestions(e.target.value)}
                    placeholder="输入核验建议..."
                />
            </Modal>

            <Modal
                title="配置豆包 AI 大模型"
                open={isConfigModalVisible}
                onOk={handleSaveConfig}
                onCancel={() => setIsConfigModalVisible(false)}
                width={600}
            >
                <Alert
                    message="配置说明"
                    description="“检查核验”环节默认使用豆包审计引擎提供第二视角建议。请在下方填入火山引擎豆包大模型的 API 信息。"
                    type="warning"
                    showIcon
                    style={{ marginBottom: 20 }}
                />
                <div style={{ marginBottom: 16 }}>
                    <Text strong>API Key</Text>
                    <Input.Password
                        placeholder="sk-..."
                        value={doubaoApiKey}
                        onChange={(e) => setDoubaoApiKey(e.target.value)}
                        style={{ marginTop: 8 }}
                    />
                </div>
                <div style={{ marginBottom: 16 }}>
                    <Text strong>API Endpoint (HTTP 地址)</Text>
                    <Input
                        placeholder="http://..."
                        value={doubaoEndpoint}
                        onChange={(e) => setDoubaoEndpoint(e.target.value)}
                        style={{ marginTop: 8 }}
                    />
                </div>
                <div style={{ marginBottom: 16 }}>
                    <Text strong>Model ID (推理接入点 ID)</Text>
                    <Input
                        placeholder="ep-..."
                        value={doubaoModelId}
                        onChange={(e) => setDoubaoModelId(e.target.value)}
                        style={{ marginTop: 8 }}
                    />
                </div>
            </Modal>

            <style>{`
                .ant-steps-item-title { line-height: 1.4 !important; margin-bottom: 4px; }
                .profile-card .ant-list-item { padding: 12px 0; border-bottom: 1px dashed #f0f0f0; }
                .profile-card .ant-list-item:last-child { border-bottom: none; }
            `}</style>
        </div>
    );
};


export default TechBidProjectWorkbench;
