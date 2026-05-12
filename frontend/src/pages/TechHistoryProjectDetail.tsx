import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
    Typography,
    Button,
    Card,
    List,
    Space,
    Spin,
    Alert,
    Empty,
    ConfigProvider,
    message,
    Tag,
    Modal,
} from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { ArrowLeftOutlined, FilePdfOutlined, FileWordOutlined, ThunderboltOutlined } from '@ant-design/icons';
import axios from 'axios';
import { useCompany } from '../context/CompanyContext';
import { findTechHistoryProject, type TechHistoryProjectFile } from '../lib/techHistoryLibrary';

const { Title, Text } = Typography;

/** 与后端 file_asset.id 一致即可。为了兼容 mock 数据和各种可能的后端 ID，放宽校验。
 * 只要不是 f1, f2 这种样式的 mock ID，且长度超过 2 位，就认为是真实 ID。 */
function isLikelyFileAssetId(id: string): boolean {
    const trimmed = id.trim();
    if (!trimmed) return false;
    // 排除 mock 数据格式 f1, f2, f3...
    if (/^f\d+$/.test(trimmed)) return false;
    return trimmed.length > 2;
}

/** 稳定空数组引用，避免 `project?.files ?? []` 每帧新引用触发 effect 死循环 */
const EMPTY_FILE_LIST: TechHistoryProjectFile[] = [];

/** 旧版后端未识别 markdownContent 时会把整份 Layout JSON 落库，用于提示用户重新解析 */
function looksLikeDocMindJsonDump(s: string): boolean {
    const t = s.trim();
    const jsonLike = t.startsWith('{') || t.startsWith('[');
    if (!jsonLike) return false;
    return t.includes('"Layouts"') || t.includes('"layouts"') || t.includes('"markdownContent"');
}

// 移除昂贵的 MarkdownContent 和 MarkdownStyles 组件，仅保留基础展示逻辑

const TechHistoryProjectDetailInner: React.FC = () => {
    const { projectId } = useParams<{ projectId: string }>();
    const navigate = useNavigate();
    const { currentCompanyId } = useCompany();
    const decodedId = projectId ? decodeURIComponent(projectId) : '';

    const project = useMemo(() => (decodedId ? findTechHistoryProject(decodedId) : undefined), [decodedId]);

    const files = useMemo(() => {
        if (!project) return EMPTY_FILE_LIST;
        if (Array.isArray(project.files)) return project.files;
        return EMPTY_FILE_LIST;
    }, [project]);

    const [selectedId, setSelectedId] = useState<string | null>(null);
    const selected = files.find((f) => f.id === selectedId) ?? null;

    useEffect(() => {
        if (files.length > 0 && !selectedId) {
            setSelectedId(files[0].id);
        }
    }, [files, selectedId]);

    const headers = useMemo(() => ({ 'X-Company-Id': currentCompanyId || '' }), [currentCompanyId]);

    const [parsedMd, setParsedMd] = useState<string>('');
    const [parsedLoading, setParsedLoading] = useState(false);
    const [parseRunning, setParseRunning] = useState(false);
    const [normalizeRunning, setNormalizeRunning] = useState(false);


    /** PDF 预览加载与错误状态 */
    const [pdfPreviewLoading, setPdfPreviewLoading] = useState(false);
    const [pdfPreviewError, setPdfPreviewError] = useState<string | null>(null);

    // 检查文件是否存在且可访问
    useEffect(() => {
        if (!selected || selected.type !== 'pdf' || !isLikelyFileAssetId(selected.id)) {
            setPdfPreviewLoading(false);
            setPdfPreviewError(null);
            return;
        }

        const ac = new AbortController();
        setPdfPreviewLoading(true);
        setPdfPreviewError(null);

        (async () => {
            try {
                // 仅执行 HEAD 请求或快速 GET 检查文件状态，不下载大 Blob，避免内存占用与加载过慢
                const res = await fetch(`/api/file-binary/${selected.id.trim()}`, {
                    method: 'GET',
                    headers: { 'X-Company-Id': currentCompanyId || '' },
                    signal: ac.signal,
                });

                if (!res.ok) {
                    let msg = `加载失败（${res.status}）`;
                    try {
                        const j = await res.json();
                        if (j?.error) msg = j.error;
                    } catch { /* ignore */ }
                    setPdfPreviewError(msg);
                } else {
                    const ct = res.headers.get('Content-Type') || '';
                    if (ct.includes('application/json')) {
                        setPdfPreviewError('服务器返回了非 PDF 数据');
                    }
                }
            } catch (e: unknown) {
                const err = e as { name?: string; message?: string };
                if (err.name !== 'AbortError') {
                    setPdfPreviewError(err.message || '网络错误');
                }
            } finally {
                setPdfPreviewLoading(false);
            }
        })();

        return () => ac.abort();
    }, [selected, currentCompanyId]);

    const fetchParsed = useCallback(async () => {
        if (!selectedId) {
            setParsedMd('');
            return;
        }
        const sel = files.find((f) => f.id === selectedId) ?? null;
        if (!sel || !isLikelyFileAssetId(sel.id)) {
            setParsedMd('');
            return;
        }
        setParsedLoading(true);
        try {
            const res = await axios.get<string>(`/api/file-parsed/${sel.id}`, {
                headers: { 'X-Company-Id': currentCompanyId || '' },
                responseType: 'text',
                transformResponse: [(d) => d],
            });
            setParsedMd(typeof res.data === 'string' ? res.data : String(res.data ?? ''));
        } catch {
            setParsedMd('');
        } finally {
            setParsedLoading(false);
        }
    }, [selectedId, files, currentCompanyId]);

    useEffect(() => {
        void fetchParsed();
    }, [fetchParsed, currentCompanyId]);

    const runAliyunParse = async () => {
        if (!selected || !isLikelyFileAssetId(selected.id)) {
            return;
        }
        setParseRunning(true);
        try {
            await axios.post(
                `/api/files/${selected.id}/aliyun-doc-parse`,
                {},
                { headers, timeout: 360000 }
            );
            message.success('阿里云文档解析完成');
            await fetchParsed();
        } catch (e: unknown) {
            const err = e as { response?: { data?: { error?: string } } };
            const msg = err.response?.data?.error || '解析失败';
            message.error(msg);
        } finally {
            setParseRunning(false);
        }
    };

    const normalizeStoredDocMindMarkdown = async () => {
        if (!selected || !isLikelyFileAssetId(selected.id)) {
            return;
        }
        setNormalizeRunning(true);
        try {
            const res = await axios.post<{ updated?: boolean }>(
                `/api/file-normalize/${selected.id}`,
                {},
                { headers }
            );
            if (res.data?.updated === false) {
                message.info('内容已是可读 Markdown，无需转换');
            } else {
                message.success('已将 JSON 转换为 Markdown');
            }
            await fetchParsed();
        } catch (e: unknown) {
            const err = e as { response?: { status?: number; data?: { error?: string } } };
            if (err.response?.status === 502) {
                message.error('转换失败：后端接口未就绪，请重启后端服务后重试');
            } else {
                message.error(err.response?.data?.error || '转换失败');
            }
        } finally {
            setNormalizeRunning(false);
        }
    };

    if (!project) {
        return (
            <div style={{ padding: 24 }}>
                <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/tech-library/history')}>
                    返回标书库
                </Button>
                <Empty description="未找到该项目" style={{ marginTop: 48 }} />
            </div>
        );
    }

    const canUseBackend = Boolean(selected && isLikelyFileAssetId(selected.id));
    const hasParsedContent = Boolean(parsedMd.trim());
    /** 解析进行中仅保留内容区等待态，底部条收起，避免重复 */
    const showParseFooter = canUseBackend && !parsedLoading && !parseRunning;

    return (
        <div style={{ padding: '0 0 24px 0' }}>
            <Space style={{ marginBottom: 16 }} align="center">
                <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/tech-library/history')}>
                    返回
                </Button>
                <Title level={4} style={{ margin: 0 }}>
                    {project.project_name}
                </Title>
                {project.winning_date ? (
                    <Text type="secondary">中标/完成：{project.winning_date}</Text>
                ) : null}
            </Space>

            <div
                style={{
                    display: 'flex',
                    gap: 16,
                    minHeight: 'calc(100vh - 200px)',
                    alignItems: 'stretch',
                }}
            >
                <Card size="small" title="标书文件" styles={{ body: { padding: 0 } }} style={{ width: 260, flexShrink: 0 }}>
                    {files.length === 0 ? (
                        <div style={{ padding: 24 }}>
                            <Text type="secondary">暂无文件，请在标书库列表中上传</Text>
                        </div>
                    ) : (
                        <List
                            size="small"
                            dataSource={files}
                            renderItem={(item: TechHistoryProjectFile) => (
                                <List.Item
                                    style={{
                                        cursor: 'pointer',
                                        background: item.id === selectedId ? '#e6f7ff' : undefined,
                                        paddingLeft: 12,
                                    }}
                                    onClick={() => setSelectedId(item.id)}
                                >
                                    <Space>
                                        {item.type === 'pdf' ? (
                                            <FilePdfOutlined style={{ color: '#ff4d4f' }} />
                                        ) : (
                                            <FileWordOutlined style={{ color: '#1890ff' }} />
                                        )}
                                        <span style={{ wordBreak: 'break-all' }}>{item.name}</span>
                                    </Space>
                                </List.Item>
                            )}
                        />
                    )}
                </Card>

                <Card
                    size="small"
                    title="原文预览"
                    styles={{ body: { height: '100%', minHeight: 360, padding: 8 } }}
                    style={{ flex: 1.2 }}
                >
                    {!selected ? (
                        <Empty description="请选择左侧文件" />
                    ) : !canUseBackend ? (
                        <Alert
                            type="info"
                            showIcon
                            message="演示占位文件无法预览"
                            description="该条目为示例数据，无后端文件。请在标书库中上传真实 PDF/Word，上传后即可在此预览并调用阿里云解析。"
                        />
                    ) : selected.type === 'pdf' ? (
                        <div style={{ width: '100%', height: '100%', minHeight: 520, position: 'relative', display: 'flex', flexDirection: 'column' }}>
                            {pdfPreviewLoading ? (
                                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 400 }}>
                                    <Spin tip="正在加载 PDF…" />
                                </div>
                            ) : pdfPreviewError ? (
                                <Alert
                                    type="error"
                                    showIcon
                                    message="无法预览 PDF"
                                    description={
                                        <div>
                                            <p>{pdfPreviewError}</p>
                                            <Button type="primary" size="small" href={`/api/file-binary/${selected.id}`} target="_blank">下载原文查看</Button>
                                        </div>
                                    }
                                />
                            ) : (
                                <>
                                    <iframe
                                        title="pdf-preview"
                                        src={`/api/file-binary/${selected.id.trim()}#view=FitH`}
                                        style={{ width: '100%', flex: 1, minHeight: 500, border: '1px solid #f0f0f0', borderRadius: 8 }}
                                    />
                                    <div style={{ marginTop: 8, textAlign: 'right' }}>
                                        <Button type="link" size="small" href={`/api/file-binary/${selected.id.trim()}`} target="_blank">
                                            在新窗口打开或下载原文
                                        </Button>
                                    </div>
                                </>
                            )}
                        </div>
                    ) : (
                        <div style={{ padding: 16 }}>
                            <Text>Word 文档无法在浏览器内直接排版预览，请下载后本地查看。</Text>
                            <div style={{ marginTop: 16 }}>
                                <Button
                                    type="primary"
                                    href={`/api/file-binary/${selected.id.trim()}`}
                                    target="_blank"
                                    rel="noreferrer"
                                >
                                    下载 {selected.name}
                                </Button>
                            </div>
                        </div>
                    )}
                </Card>

                <Card
                    size="small"
                    title="AI 解析状态"
                    styles={{
                        body: {
                            display: 'flex',
                            flexDirection: 'column',
                            minHeight: 360,
                            padding: 0,
                        },
                    }}
                    style={{ flex: 0.8 }}
                >
                    <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: 16 }}>
                        {!selected ? (
                            <Empty description="请选择左侧文件" />
                        ) : !canUseBackend ? (
                            <Alert
                                type="warning"
                                showIcon
                                message="需使用实际上传的文件（带有效文件 ID）才能拉取或生成解析结果"
                            />
                        ) : parsedLoading ? (
                            <div style={{ textAlign: 'center', padding: 48 }}>
                                <Spin />
                                <div style={{ marginTop: 12 }}>
                                    <Text type="secondary">正在加载已有解析内容…</Text>
                                </div>
                            </div>
                        ) : parseRunning ? (
                            <div style={{ textAlign: 'center', padding: '48px 24px' }}>
                                <Spin size="large" />
                                <Title level={5} style={{ marginTop: 24, marginBottom: 8 }}>
                                    正在解析文档
                                </Title>
                                <Text type="secondary" style={{ display: 'block', maxWidth: 320, margin: '0 auto' }}>
                                    已调用本站阿里云文档智能解析，大文件可能需要数分钟，请勿关闭页面。
                                </Text>
                            </div>
                        ) : !hasParsedContent ? (
                            <Empty
                                description="暂无解析内容"
                                image={Empty.PRESENTED_IMAGE_SIMPLE}
                            >
                                <Text type="secondary" style={{ display: 'block', maxWidth: 360, margin: '0 auto' }}>
                                    请在下方点击「立即开始解析」调用阿里云文档智能（需先在系统设置中配置文档解析密钥）；若文件曾走本地 OCR 入库，加载完成后也会显示在此。
                                </Text>
                            </Empty>
                        ) : (
                            <div style={{ textAlign: 'center', padding: '60px 24px' }}>
                                <div style={{ fontSize: 48, color: '#52c41a', marginBottom: 24 }}>
                                    <ThunderboltOutlined />
                                </div>
                                <Title level={4}>AI 知识提取已就绪</Title>
                                <div style={{ marginTop: 16 }}>
                                    <Tag color="success">状态：已完成解析</Tag>
                                    <Tag color="cyan">引擎：阿里云文档智能</Tag>
                                </div>
                                <div style={{ marginTop: 32 }}>
                                    <Text type="secondary" style={{ display: 'block', marginBottom: 24 }}>
                                        该标书内容已被 AI 深度理解，并录入到企业技术标知识库中。
                                        您可以直接在“技术标制作”中使用 AI 助手调用此文件作为素材。
                                    </Text>
                                    <Button
                                        onClick={() => {
                                            Modal.info({
                                                title: `文本内容预览 - ${selected?.name}`,
                                                width: 1000,
                                                maskClosable: true,
                                                content: (
                                                    <div style={{ maxHeight: '60vh', overflowY: 'auto', padding: '16px', background: '#f8fafc', borderRadius: 8, fontFamily: 'monospace' }}>
                                                        {looksLikeDocMindJsonDump(parsedMd) ? (
                                                            <Alert
                                                                type="warning"
                                                                showIcon
                                                                style={{ marginBottom: 16, fontFamily: 'inherit' }}
                                                                message="当前为阿里云返回的结构化 JSON（历史入库），不是排版后的 Markdown 正文"
                                                                description="你可以点「将已存内容转为 Markdown（本地）」快速修复（不调用阿里云），或点「重新解析」走完整云端解析。"
                                                            />
                                                        ) : null}
                                                        <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{parsedMd}</pre>
                                                    </div>
                                                )
                                            })
                                        }}
                                    >
                                        快速预览原文字符
                                    </Button>
                                    {canUseBackend && hasParsedContent ? (
                                        <Button loading={normalizeRunning} onClick={() => void normalizeStoredDocMindMarkdown()}>
                                            将已存内容转为 Markdown（本地）
                                        </Button>
                                    ) : null}
                                </div>
                            </div>
                        )}
                    </div>

                    {showParseFooter ? (
                        <div
                            style={{
                                flexShrink: 0,
                                borderTop: '1px solid #f0f0f0',
                                padding: '16px 20px',
                                background: '#fafafa',
                                textAlign: 'center',
                            }}
                        >
                            {!hasParsedContent ? (
                                <Button
                                    type="primary"
                                    size="large"
                                    icon={<ThunderboltOutlined />}
                                    onClick={() => void runAliyunParse()}
                                    style={{ minWidth: 200, height: 44 }}
                                >
                                    立即开始解析
                                </Button>
                            ) : (
                                <Button icon={<ThunderboltOutlined />} onClick={() => void runAliyunParse()}>
                                    重新解析
                                </Button>
                            )}
                        </div>
                    ) : null}
                </Card>
            </div>
        </div>
    );
};

const TechHistoryProjectDetail: React.FC = () => (
    <ConfigProvider locale={zhCN}>
        <TechHistoryProjectDetailInner />
    </ConfigProvider>
);

export default TechHistoryProjectDetail;
