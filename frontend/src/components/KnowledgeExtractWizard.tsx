import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Modal,
  Steps,
  Button,
  Table,
  Input,
  Radio,
  Tree,
  Card,
  Space,
  Typography,
  message,
  Spin,
  Alert,
} from 'antd';
import type { DataNode } from 'antd/es/tree';
import axios from 'axios';
import { readTechHistoryProjects, type TechHistoryProject, type TechHistoryProjectFile } from '../lib/techHistoryLibrary';

const { Text, Paragraph } = Typography;
const { TextArea } = Input;

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export interface MergedHistoryProject {
  id: string;
  name: string;
  origin: 'tech_bid' | 'local_library';
  project_type?: string;
  tender_code?: string;
  file_count: number;
  created_at?: string;
  localProject?: TechHistoryProject;
}

interface HistoryProjectApiRow {
  id: string;
  name: string;
  origin: string;
  project_type?: string;
  tender_code?: string;
  file_count: number;
  created_at?: string;
}

interface ProjectFileRow {
  id: string;
  file_asset_id: string;
  file_name: string;
  file_role?: string;
  parse_ready: boolean;
  markdown_ready: boolean;
  content_updated_at?: string;
  file_size?: number;
  recommended?: boolean;
}

interface PromptTpl {
  id: number;
  prompt_key: string;
  prompt_name: string;
  version: number;
  content: string;
}

/** 与后端 ListPromptTemplatesForType 优先级一致：当前类型 default → 通用兜底 → 第一条 */
function pickSingleExtractPrompt(list: PromptTpl[], knowledgeType: string): PromptTpl | null {
  if (!list.length) return null;
  const preferredKey = `knowledge_extract_${knowledgeType}_default`;
  const fallbackKey = 'knowledge_extract_default';
  return (
    list.find((t) => t.prompt_key === preferredKey) ||
    list.find((t) => t.prompt_key === fallbackKey) ||
    list[0]
  );
}

interface ExtractResultRow {
  id: string;
  title?: string;
  content_json: string;
  source_section?: string;
  selected_flag: boolean;
}

function parseMarkdownSections(md: string): { key: string; title: string; level: number; startLine: number; endLine: number }[] {
  const lines = md.split('\n');
  const headers: { key: string; title: string; level: number; line: number }[] = [];
  let hIdx = 0;
  lines.forEach((line, i) => {
    const m = /^(#{1,6})\s+(.+)$/.exec(line);
    if (m) {
      headers.push({
        key: `h-${hIdx++}`,
        title: m[2].trim(),
        level: m[1].length,
        line: i,
      });
    }
  });
  const sections: { key: string; title: string; level: number; startLine: number; endLine: number }[] = [];
  for (let i = 0; i < headers.length; i++) {
    const startLine = headers[i].line;
    const endLine = i + 1 < headers.length ? headers[i + 1].line : lines.length;
    sections.push({
      key: headers[i].key,
      title: headers[i].title,
      level: headers[i].level,
      startLine,
      endLine,
    });
  }
  if (sections.length === 0 && md.trim()) {
    return [{ key: 'full', title: '全文', level: 0, startLine: 0, endLine: lines.length }];
  }
  return sections;
}

function buildMarkdownFromSections(
  lines: string[],
  keys: Set<string>,
  sections: ReturnType<typeof parseMarkdownSections>
): string {
  if (keys.size === 0 || sections.length === 0) return lines.join('\n');
  const chosen = sections.filter((s) => keys.has(s.key));
  if (chosen.length === 0) return lines.join('\n');
  return chosen
    .map((s) => lines.slice(s.startLine, s.endLine).join('\n').trim())
    .filter(Boolean)
    .join('\n\n');
}

export interface ExtractStartedPayload {
  taskId: string;
  displayName: string;
  /** 与当前知识库分类一致（method / equipment / 自定义 id 等），用于仅在对应库列表展示进行中任务 */
  knowledgeType: string;
}

const KnowledgeExtractWizard: React.FC<{
  open: boolean;
  onClose: () => void;
  knowledgeType: string;
  currentCompanyId: string;
  onCommitted?: (committedTaskId?: string) => void;
  /** 从列表「确认入库」进入时传入，直接打开第 5 步并加载结果 */
  resumeTaskId?: string | null;
  /** 点击「开始提炼」且任务已异步启动后回调，用于关闭向导并在列表展示进行中状态 */
  onExtractStarted?: (payload: ExtractStartedPayload) => void;
}> = ({ open, onClose, knowledgeType, currentCompanyId, resumeTaskId, onExtractStarted }) => {
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(false);
  const [projectSearch, setProjectSearch] = useState('');
  const [mergedProjects, setMergedProjects] = useState<MergedHistoryProject[]>([]);
  const [selectedProject, setSelectedProject] = useState<MergedHistoryProject | null>(null);
  const [files, setFiles] = useState<ProjectFileRow[]>([]);
  const [selectedFile, setSelectedFile] = useState<ProjectFileRow | null>(null);
  const [markdown, setMarkdown] = useState('');
  const [scope, setScope] = useState<'full' | 'sections'>('full');
  const [sectionKeys, setSectionKeys] = useState<Set<string>>(new Set());
  const [mdSearch, setMdSearch] = useState('');
  /** 当前知识库类型对应的唯一提炼提示词（展示 + 可编辑草稿） */
  const [extractPromptTitle, setExtractPromptTitle] = useState('');
  const [extractPromptText, setExtractPromptText] = useState('');
  const [extractPromptTemplateId, setExtractPromptTemplateId] = useState<number | undefined>();
  const [extractPromptEditing, setExtractPromptEditing] = useState(false);
  const [extractPromptSnapshot, setExtractPromptSnapshot] = useState('');
  const [taskId, setTaskId] = useState<string | null>(null);

  const mdLines = useMemo(() => markdown.split('\n'), [markdown]);
  const sections = useMemo(() => parseMarkdownSections(markdown), [markdown]);

  const reset = () => {
    setStep(0);
    setProjectSearch('');
    setMergedProjects([]);
    setSelectedProject(null);
    setFiles([]);
    setSelectedFile(null);
    setMarkdown('');
    setScope('full');
    setSectionKeys(new Set());
    setMdSearch('');
    setExtractPromptTitle('');
    setExtractPromptText('');
    setExtractPromptTemplateId(undefined);
    setExtractPromptEditing(false);
    setExtractPromptSnapshot('');
    setTaskId(null);
  };

  const loadTaskForResume = useCallback(
    async (tid: string) => {
      if (!currentCompanyId) return;
      setLoading(true);
      try {
        const res = await axios.get<{ meta: { status?: string }; results: ExtractResultRow[] }>(`/api/knowledge-extract/tasks/${tid}`, {
          headers: { 'X-Company-Id': currentCompanyId },
        });
        const st = res.data.meta?.status;
        if (st !== 'completed') {
          message.warning(st === 'failed' ? '该提炼任务已失败，请重新提炼' : '提炼尚未完成或已取消');
          onClose();
          return;
        }
        setTaskId(tid);
        // Step 4 removed for automation
        onClose();
      } catch (e: unknown) {
        const err = e as { response?: { data?: { error?: string } } };
        message.error(err.response?.data?.error || '加载提炼结果失败');
        onClose();
      } finally {
        setLoading(false);
      }
    },
    [currentCompanyId, onClose]
  );

  useEffect(() => {
    if (!open) {
      reset();
      return;
    }
    if (resumeTaskId) {
      void loadTaskForResume(resumeTaskId);
      return;
    }
    setStep(0);
  }, [open, resumeTaskId, loadTaskForResume]);

  const loadProjects = useCallback(async () => {
    if (!currentCompanyId) return;
    setLoading(true);
    const localRaw = readTechHistoryProjects();
    const local: MergedHistoryProject[] = localRaw.map((p) => ({
      id: p.id,
      name: p.project_name,
      origin: 'local_library' as const,
      file_count: p.file_count ?? p.files?.length ?? 0,
      created_at: p.winning_date,
      localProject: p,
    }));

    let tech: MergedHistoryProject[] = [];
    try {
      const res = await axios.get<HistoryProjectApiRow[] | unknown>('/api/knowledge-extract/history-projects', {
        params: { q: projectSearch || undefined },
        headers: { 'X-Company-Id': currentCompanyId },
      });
      const raw = res.data;
      const rows: HistoryProjectApiRow[] = Array.isArray(raw) ? raw : [];
      tech = rows.map((r) => ({
        id: r.id,
        name: r.name,
        origin: 'tech_bid' as const,
        project_type: r.project_type,
        tender_code: r.tender_code,
        file_count: r.file_count,
        created_at: r.created_at,
      }));
    } catch (e) {
      // 后端不可用时仍展示本地标书库；不在此处弹 message，避免打开向导即出现干扰提示
      console.error('[KnowledgeExtractWizard] history-projects', e);
    }

    setMergedProjects([...tech, ...local]);
    setLoading(false);
  }, [currentCompanyId, projectSearch]);

  useEffect(() => {
    if (open && step === 0) {
      loadProjects();
    }
  }, [open, step, loadProjects]);

  const loadFilesForProject = async (p: MergedHistoryProject) => {
    setLoading(true);
    setFiles([]);
    setSelectedFile(null);
    try {
      if (p.origin === 'tech_bid') {
        const res = await axios.get<ProjectFileRow[] | unknown>(
          `/api/knowledge-extract/projects/tech_bid/${encodeURIComponent(p.id)}/files`,
          {
            headers: { 'X-Company-Id': currentCompanyId },
          }
        );
        const raw = res.data;
        const rows: ProjectFileRow[] = Array.isArray(raw) ? raw : [];
        setFiles(
          rows.map((row) => ({
            ...row,
            id: row.id || row.file_asset_id,
          }))
        );
      } else {
        const rawFiles: TechHistoryProjectFile[] = p.localProject?.files || [];
        const body = {
          client_project_id: p.id,
          files: rawFiles.map((f) => ({ id: f.id, name: f.name, role: f.role })),
        };
        const res = await axios.post<{ files?: ProjectFileRow[] }>('/api/knowledge-extract/resolve-local-files', body, {
          headers: { 'X-Company-Id': currentCompanyId },
        });
        const list = res.data?.files;
        const rows: ProjectFileRow[] = Array.isArray(list) ? list : [];
        setFiles(
          rows.map((row) => ({
            ...row,
            id: row.id || row.file_asset_id,
          }))
        );
        if (!rows.length && rawFiles.some((f) => !UUID_RE.test(f.id))) {
          message.warning('本地标书库中部分文件未关联真实上传 ID，仅显示已成功入库且可解析的文件。');
        }
      }
    } catch (e) {
      console.error('[KnowledgeExtractWizard] loadFilesForProject', e);
    } finally {
      setLoading(false);
    }
  };

  const loadMarkdown = async (fileAssetId: string) => {
    setLoading(true);
    try {
      const res = await axios.get(`/api/file-parsed/${fileAssetId}`, {
        headers: { 'X-Company-Id': currentCompanyId },
        responseType: 'text',
        transformResponse: [(d) => (typeof d === 'string' ? d : String(d ?? ''))],
      });
      const text = typeof res.data === 'string' ? res.data : String(res.data ?? '');
      setMarkdown(text || '');
      // 默认不选中任何章节，由用户手动选择
      setSectionKeys(new Set());
    } catch (e) {
      console.error('[KnowledgeExtractWizard] file-parsed', e);
      setMarkdown('');
      setSectionKeys(new Set());
    } finally {
      setLoading(false);
    }
  };

  const loadDefaultPrompt = useCallback(async () => {
    if (!currentCompanyId) return;
    try {
      const res = await axios.get<PromptTpl[] | unknown>('/api/knowledge-extract/prompt-templates', {
        params: { knowledge_type: knowledgeType },
        headers: { 'X-Company-Id': currentCompanyId },
      });
      const list: PromptTpl[] = Array.isArray(res.data) ? res.data : [];
      const picked = pickSingleExtractPrompt(list, knowledgeType);
      if (picked) {
        setExtractPromptTitle(`${picked.prompt_name} (v${picked.version})`);
        setExtractPromptText(picked.content || '');
        setExtractPromptTemplateId(picked.id);
      } else {
        setExtractPromptTitle('未配置提炼提示词');
        setExtractPromptText('');
        setExtractPromptTemplateId(undefined);
      }
      setExtractPromptEditing(false);
      setExtractPromptSnapshot('');
    } catch (e) {
      console.error('[KnowledgeExtractWizard] prompt-templates', e);
    }
  }, [currentCompanyId, knowledgeType]);

  useEffect(() => {
    if (open && step === 3) {
      void loadDefaultPrompt();
    }
  }, [open, step, loadDefaultPrompt]);

  const treeData: DataNode[] = useMemo(() => {
    return sections.map((s) => ({
      title: `${'#'.repeat(s.level)} ${s.title}`,
      key: s.key,
    }));
  }, [sections]);

  const markdownForExtract = useMemo(() => {
    if (scope === 'full') return markdown;
    return buildMarkdownFromSections(mdLines, sectionKeys, sections);
  }, [markdown, mdLines, scope, sectionKeys, sections]);

  const runExtract = async () => {
    if (!selectedProject || !selectedFile?.file_asset_id) {
      message.error('请选择项目与文件');
      return;
    }
    const md = markdownForExtract.trim();
    if (!md) {
      message.error('提炼内容为空，请检查全文或章节选择');
      return;
    }
    const displayName = `${selectedProject.name} · AI提炼`;
    setLoading(true);
    try {
      const payload: Record<string, unknown> = {
        knowledge_type: knowledgeType,
        source_origin: selectedProject.origin,
        source_project_id: selectedProject.id,
        source_project_name: selectedProject.name,
        source_file_id: selectedFile.file_asset_id,
        extract_scope: scope,
        selected_sections_json:
          scope === 'sections' ? JSON.stringify(Array.from(sectionKeys)) : JSON.stringify([]),
        markdown_content: md,
      };
      const pt = extractPromptText.trim();
      if (pt) {
        if (!pt.includes('markdown_content')) {
          message.warning('提示词中建议包含 {{markdown_content}} 占位符，否则模型可能无法收到正文');
        }
        payload.prompt_override = pt;
      } else if (extractPromptTemplateId != null) {
        payload.prompt_template_id = extractPromptTemplateId;
      }
      const res = await axios.post<{
        task_id?: string;
        status?: string;
        results?: ExtractResultRow[];
      }>('/api/knowledge-extract/tasks', payload, {
        headers: { 'X-Company-Id': currentCompanyId },
        validateStatus: (s) => s === 200 || s === 201 || s === 202,
      });
      const tid = res.data.task_id;
      if (!tid) {
        message.error('未返回任务编号');
        return;
      }

      // 旧版同步接口：同一次响应里带 results，留在向导内确认入库
      const syncResults = res.data.results;
      if (Array.isArray(syncResults) && syncResults.length > 0) {
        setTaskId(tid);
        // Step 4 confirmation removed for automation
        onClose();
        message.success('提炼完成，请在列表中查看');
        return;
      }

      // 异步：立即关弹窗，由列表展示进度（不依赖 onExtractStarted 是否存在）
      onExtractStarted?.({ taskId: tid, displayName, knowledgeType });
      message.success('已开始后台提炼，请在列表中查看进度');
      onClose();
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } };
      message.error(err.response?.data?.error || '提炼失败');
    } finally {
      setLoading(false);
    }
  };

  const cancelTask = async () => {
    if (!taskId) return;
    try {
      await axios.post(`/api/knowledge-extract/tasks/${taskId}/cancel`, {}, { headers: { 'X-Company-Id': currentCompanyId } });
    } catch {
      /* ignore */
    }
  };

  const handleClose = () => {
    if (taskId && step < 4) {
      void cancelTask();
    }
    onClose();
  };

  const previewMd = useMemo(() => {
    if (!mdSearch.trim()) return markdown.slice(0, 12000);
    const i = markdown.toLowerCase().indexOf(mdSearch.toLowerCase());
    if (i < 0) return markdown.slice(0, 8000);
    const start = Math.max(0, i - 200);
    return markdown.slice(start, start + 8000);
  }, [markdown, mdSearch]);

  return (
    <Modal
      title="AI 从历史项目提炼"
      open={open}
      onCancel={handleClose}
      width={960}
      footer={null}
      destroyOnClose
    >
      {resumeTaskId && loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin size="large" tip="加载提炼结果…" />
        </div>
      ) : (
        <>
      <Steps
        current={step}
        items={[
          { title: '选择项目' },
          { title: '选择文件' },
          { title: '范围与预览' },
          { title: '提炼提示词' },
        ]}
        style={{ marginBottom: 24 }}
      />

      {step === 0 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Input.Search
            placeholder="按项目名称、编号、类型搜索"
            onSearch={() => loadProjects()}
            onChange={(e) => setProjectSearch(e.target.value)}
            enterButton
          />
          <Spin spinning={loading}>
            <Table
              rowKey={(r) => `${r.origin}:${r.id}`}
              size="small"
              dataSource={mergedProjects}
              pagination={{ pageSize: 8 }}
              columns={[
                { title: '项目名称', dataIndex: 'name', key: 'name' },
                {
                  title: '来源',
                  key: 'origin',
                  width: 100,
                  render: (_: unknown, r: MergedHistoryProject) =>
                    r.origin === 'tech_bid' ? <Text type="success">技术标项目</Text> : <Text type="warning">标书库(本地)</Text>,
                },
                { title: '类型', dataIndex: 'project_type', key: 'project_type', width: 120, render: (t: string) => t || '—' },
                { title: '文件数', dataIndex: 'file_count', key: 'file_count', width: 80 },
              ]}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedProject ? [`${selectedProject.origin}:${selectedProject.id}`] : [],
                onChange: (_keys, rows) => {
                  const r = rows[0] as MergedHistoryProject | undefined;
                  setSelectedProject(r || null);
                },
              }}
              onRow={(record) => ({
                onClick: () => setSelectedProject(record),
              })}
            />
          </Spin>
          <Space style={{ justifyContent: 'flex-end', width: '100%' }}>
            <Button onClick={handleClose}>取消</Button>
            <Button
              type="primary"
              disabled={!selectedProject}
              onClick={() => {
                setStep(1);
                if (selectedProject) void loadFilesForProject(selectedProject);
              }}
            >
              下一步
            </Button>
          </Space>
        </Space>
      )}

      {step === 1 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert
            type="info"
            showIcon
            message="仅「已解析」（含 Markdown 或纯文本）的文件可选；未解析文件不可选。"
          />
          <Spin spinning={loading}>
            <Table
              rowKey="id"
              size="small"
              dataSource={files}
              pagination={false}
              columns={[
                { title: '文件名称', dataIndex: 'file_name', key: 'file_name' },
                { title: '角色', dataIndex: 'file_role', key: 'file_role', width: 120, render: (t: string) => t || '—' },
                {
                  title: '解析状态',
                  key: 'st',
                  width: 120,
                  render: (_: unknown, r: ProjectFileRow) =>
                    r.parse_ready ? (
                      <Text type="success">已解析</Text>
                    ) : (
                      <Text type="secondary">未解析</Text>
                    ),
                },
                {
                  title: '推荐',
                  key: 'rec',
                  width: 70,
                  render: (_: unknown, r: ProjectFileRow) => (r.recommended ? <Text type="success">是</Text> : '—'),
                },
              ]}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedFile ? [selectedFile.id] : [],
                getCheckboxProps: (r: ProjectFileRow) => ({
                  disabled: !r.parse_ready || !r.file_asset_id,
                }),
                onChange: (_keys, rows) => {
                  const r = rows[0] as ProjectFileRow | undefined;
                  setSelectedFile(r || null);
                },
              }}
              onRow={(record) => ({
                onClick: () => {
                  if (record.parse_ready && record.file_asset_id) setSelectedFile(record);
                },
              })}
            />
          </Spin>
          <Space style={{ justifyContent: 'space-between', width: '100%' }}>
            <Button onClick={() => setStep(0)}>上一步</Button>
            <Space>
              <Button onClick={handleClose}>取消</Button>
              <Button
                type="primary"
                disabled={!selectedFile?.file_asset_id || !selectedFile.parse_ready}
                onClick={() => {
                  const fid = selectedFile?.file_asset_id;
                  if (!fid) return;
                  setStep(2);
                  void loadMarkdown(fid);
                }}
              >
                下一步
              </Button>
            </Space>
          </Space>
        </Space>
      )}

      {step === 2 && (
        <div style={{ display: 'flex', gap: 16, minHeight: 360 }}>
          <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Text strong>Markdown 预览</Text>
            <Input placeholder="关键词定位" value={mdSearch} onChange={(e) => setMdSearch(e.target.value)} allowClear />
            <pre
              style={{
                flex: 1,
                overflow: 'auto',
                background: '#f5f5f5',
                padding: 12,
                fontSize: 12,
                borderRadius: 8,
                maxHeight: 420,
                whiteSpace: 'pre-wrap',
              }}
            >
              {previewMd}
            </pre>
          </div>
          <div style={{ width: 280 }}>
            <Text strong>提炼范围</Text>
            <Radio.Group
              style={{ display: 'block', marginTop: 8 }}
              value={scope}
              onChange={(e) => setScope(e.target.value)}
            >
              <Radio value="full">全文提炼</Radio>
              <Radio value="sections">指定章节</Radio>
            </Radio.Group>
            {scope === 'sections' && (
              <Tree
                style={{ marginTop: 12, maxHeight: 320, overflow: 'auto' }}
                checkable
                treeData={treeData}
                checkedKeys={Array.from(sectionKeys)}
                onCheck={(keys) => {
                  const k = keys as React.Key[];
                  setSectionKeys(new Set(k.map(String)));
                }}
                onSelect={(_, info) => {
                  const key = String(info.node.key);
                  const newKeys = new Set(sectionKeys);
                  if (newKeys.has(key)) {
                    newKeys.delete(key);
                  } else {
                    newKeys.add(key);
                  }
                  setSectionKeys(newKeys);
                }}
              />
            )}
            <Paragraph type="secondary" style={{ marginTop: 12, fontSize: 12 }}>
              将送入模型的内容约 {markdownForExtract.length} 字符
            </Paragraph>
          </div>
        </div>
      )}

      {step === 2 && (
        <Space style={{ justifyContent: 'space-between', width: '100%', marginTop: 16 }}>
          <Button onClick={() => setStep(1)}>上一步</Button>
          <Space>
            <Button onClick={handleClose}>取消</Button>
            <Button type="primary" onClick={() => setStep(3)}>
              下一步
            </Button>
          </Space>
        </Space>
      )}

      {step === 3 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert
            type="info"
            showIcon
            message="以下为本次提炼使用的提示词（与系统设置中的模板同步）。可直接查看，点击「修改」编辑后「保存」，再开始提炼。"
          />
          <Card
            size="small"
            title={extractPromptTitle || '提炼提示词'}
            extra={
              <Space size="small">
                {!extractPromptEditing ? (
                  <Button
                    type="link"
                    size="small"
                    onClick={() => {
                      setExtractPromptSnapshot(extractPromptText);
                      setExtractPromptEditing(true);
                    }}
                  >
                    修改
                  </Button>
                ) : (
                  <>
                    <Button
                      size="small"
                      type="primary"
                      onClick={() => {
                        setExtractPromptEditing(false);
                        message.success('已保存本次编辑（仅作用于本次提炼，写入系统设置请在「提示词配置」中操作）');
                      }}
                    >
                      保存
                    </Button>
                    <Button
                      size="small"
                      onClick={() => {
                        setExtractPromptText(extractPromptSnapshot);
                        setExtractPromptEditing(false);
                      }}
                    >
                      取消
                    </Button>
                  </>
                )}
              </Space>
            }
          >
            <TextArea
              readOnly={!extractPromptEditing}
              value={extractPromptText}
              onChange={(e) => setExtractPromptText(e.target.value)}
              rows={14}
              placeholder="未加载到提示词模板，请先在系统设置 → 提示词配置中维护 knowledge_extract_*_default"
              style={
                extractPromptEditing
                  ? { fontFamily: 'inherit', fontSize: 13 }
                  : { fontFamily: 'inherit', fontSize: 13, background: '#fafafa', color: 'rgba(0,0,0,0.88)', cursor: 'default' }
              }
            />
            <Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0, fontSize: 12 }}>
              正文中需包含占位符 <Text code>{'{{markdown_content}}'}</Text>，系统会将上一步选定的 Markdown 注入该位置。
            </Paragraph>
          </Card>
          <Space style={{ justifyContent: 'space-between', width: '100%' }}>
            <Button onClick={() => setStep(2)}>上一步</Button>
            <Space>
              <Button onClick={handleClose}>取消</Button>
              <Button
                type="primary"
                loading={loading}
                disabled={!extractPromptText.trim() && extractPromptTemplateId == null}
                onClick={() => void runExtract()}
              >
                开始提炼
              </Button>
            </Space>
          </Space>
        </Space>
      )}

      {/* Step 4 confirmation removed for automation */}
        </>
      )}
    </Modal>
  );
};

export default KnowledgeExtractWizard;
