import React, { useState, useRef, useCallback, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Table, Button, Space, Typography, Card, Tag, Modal, Form, Input, Select, Upload, message, Divider, DatePicker, ConfigProvider, App } from 'antd';
import type { UploadChangeParam } from 'antd/es/upload/interface';
import { useCompany } from '../context/CompanyContext';
import {
    TECH_HISTORY_STORAGE_KEY,
    readTechHistoryProjects,
    type TechHistoryProject,
    type TechHistoryProjectFile,
} from '../lib/techHistoryLibrary';
import zhCN from 'antd/locale/zh_CN';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import {
    HistoryOutlined,
    PlusOutlined,
    FilePdfOutlined,
    FileWordOutlined,
    CloudUploadOutlined,
    SearchOutlined,
    DeleteOutlined,
    EditOutlined,
    FileTextOutlined,
    InboxOutlined
} from '@ant-design/icons';

dayjs.locale('zh-cn');

const { Title, Text } = Typography;

/** 录入 / 修改共用表单项（字段定义唯一） */
const HistoryProjectFormFields: React.FC = () => (
    <>
        <Form.Item label="项目名称" name="project_name" rules={[{ required: true, message: '请输入项目名称' }]}>
            <Input placeholder="项目完整名称" />
        </Form.Item>
        <Form.Item label="行业标签" name="tags">
            <Select mode="tags" placeholder="输入后回车添加标签" />
        </Form.Item>
        <Form.Item label="中标/完成日期" name="winning_date">
            <DatePicker
                picker="month"
                style={{ width: '100%' }}
                format="YYYY-MM"
                placeholder="点击选择年份、月份（选填）"
                allowClear
            />
        </Form.Item>
    </>
);

function formatWinningMonth(v: unknown): string {
    if (v == null) return '';
    if (dayjs.isDayjs(v)) return v.format('YYYY-MM');
    if (typeof v === 'string') return v.trim();
    return '';
}

/** 操作列「删除」：淡黑，非警示红 */
const DELETE_ACTION_COLOR = '#595959';

function formatFileSize(bytes: number | undefined): string {
    if (bytes == null || Number.isNaN(bytes)) return '—';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function fileTypeFromName(name: string): 'pdf' | 'doc' {
    const n = name.toLowerCase();
    if (n.endsWith('.pdf')) return 'pdf';
    return 'doc';
}

const TechHistoryLibraryInner: React.FC = () => {
    const { modal } = App.useApp();
    const { currentCompanyId } = useCompany();
    const [projects, setProjects] = useState<TechHistoryProject[]>(() => readTechHistoryProjects());

    const setProjectsPersist = useCallback((updater: React.SetStateAction<TechHistoryProject[]>) => {
        setProjects((prev) => {
            const next =
                typeof updater === 'function'
                    ? (updater as (p: TechHistoryProject[]) => TechHistoryProject[])(prev)
                    : updater;
            try {
                localStorage.setItem(TECH_HISTORY_STORAGE_KEY, JSON.stringify(next));
            } catch {
                /* quota / private mode */
            }
            return next;
        });
    }, []);
    const [historyModalOpen, setHistoryModalOpen] = useState(false);
    const [historyModalMode, setHistoryModalMode] = useState<'create' | 'edit'>('create');
    const [isUploadModalVisible, setIsUploadModalVisible] = useState(false);
    const [currentProject, setCurrentProject] = useState<TechHistoryProject | null>(null);
    const currentProjectRef = useRef<TechHistoryProject | null>(null);
    useEffect(() => {
        currentProjectRef.current = currentProject;
    }, [currentProject]);
    const [editingProject, setEditingProject] = useState<TechHistoryProject | null>(null);
    const [projectForm] = Form.useForm();
    /** 受控弹窗删除；ref 避免 Modal.onOk 读到过期的 deleteTarget 状态 */
    const [deleteTarget, setDeleteTarget] = useState<TechHistoryProject | null>(null);
    const deleteTargetRef = useRef<TechHistoryProject | null>(null);

    const handleOpenUpload = (project: TechHistoryProject) => {
        setCurrentProject(project);
        setIsUploadModalVisible(true);
    };

    const handleTechHistoryUploadChange = useCallback(
        (info: UploadChangeParam) => {
            const { file } = info;
            if (file.status === 'done') {
                const res = file.response as { id?: string; file_name?: string } | undefined;
                const pid = currentProjectRef.current?.id;
                if (!pid || !res?.id) {
                    message.warning('上传成功但未关联到当前项目');
                    return;
                }
                const fileName = res.file_name || file.name || '未命名';
                const newFile: TechHistoryProjectFile = {
                    id: res.id,
                    name: fileName,
                    type: fileTypeFromName(fileName),
                    size: formatFileSize(file.size),
                    upload_date: dayjs().format('YYYY-MM-DD'),
                    role: '附件',
                };
                setProjectsPersist((prev) =>
                    prev.map((p) => {
                        if (p.id !== pid) return p;
                        const nextFiles = [...(p.files || []), newFile];
                        return { ...p, files: nextFiles, file_count: nextFiles.length };
                    })
                );
                setCurrentProject((prev) => {
                    if (!prev || prev.id !== pid) return prev;
                    const nextFiles = [...(prev.files || []), newFile];
                    return { ...prev, files: nextFiles, file_count: nextFiles.length };
                });
                message.success(`${fileName} 已上传`);
            } else if (file.status === 'error') {
                message.error('上传失败，请确认后端已启动且可访问 /api/files/upload');
            }
        },
        [setProjectsPersist]
    );

    const handleOpenCreate = () => {
        setHistoryModalMode('create');
        setEditingProject(null);
        projectForm.resetFields();
        projectForm.setFieldsValue({ tags: [] });
        setHistoryModalOpen(true);
    };

    const handleOpenEdit = (record: TechHistoryProject) => {
        setHistoryModalMode('edit');
        setEditingProject(record);
        projectForm.setFieldsValue({
            project_name: record.project_name,
            winning_date: record.winning_date ? dayjs(record.winning_date) : undefined,
            tags: record.tags,
        });
        setHistoryModalOpen(true);
    };

    const closeHistoryModal = () => {
        setHistoryModalOpen(false);
        setEditingProject(null);
        projectForm.resetFields();
    };

    const handleHistoryModalOk = () =>
        projectForm.validateFields().then((values) => {
            const winning_date = formatWinningMonth(values.winning_date);
            if (historyModalMode === 'edit' && editingProject) {
                setProjectsPersist((prev) =>
                    prev.map((p) =>
                        p.id === editingProject.id
                            ? {
                                  ...p,
                                  project_name: values.project_name,
                                  winning_date,
                                  tags: values.tags ?? [],
                                  status: p.status,
                              }
                            : p
                    )
                );
                message.success('项目信息已更新');
            } else {
                const newId = `new-${Date.now()}`;
                setProjectsPersist((prev) => [
                    ...prev,
                    {
                        id: newId,
                        project_name: values.project_name,
                        winning_date: winning_date || '',
                        file_count: 0,
                        status: 'Archived',
                        tags: values.tags ?? [],
                        files: [],
                    },
                ]);
                message.success('历史项目录入成功');
            }
            closeHistoryModal();
        });

    const handleDeleteProject = (record: TechHistoryProject) => {
        deleteTargetRef.current = record;
        setDeleteTarget(record);
    };

    const handleConfirmDeleteProject = () => {
        const target = deleteTargetRef.current;
        if (!target) return;
        const id = String(target.id);
        deleteTargetRef.current = null;

        setProjectsPersist((prev) => prev.filter((p) => String(p.id) !== id));

        const cp = currentProjectRef.current;
        if (cp && String(cp.id) === id) {
            setIsUploadModalVisible(false);
        }
        setCurrentProject((prev) => (prev && String(prev.id) === id ? null : prev));

        message.success('已删除');
        setDeleteTarget(null);
    };

    const handleDeleteProjectFile = (projectId: string, fileId: string) => {
        if (!projectId) return;
        modal.confirm({
            title: '移除文件',
            content: '确定从该项目中移除此文件吗？',
            okText: '移除',
            okButtonProps: { danger: true },
            cancelText: '取消',
            onOk: () => {
                setProjectsPersist((prev) =>
                    prev.map((p) => {
                        if (p.id !== projectId) return p;
                        const nextFiles = (p.files || []).filter((f) => f.id !== fileId);
                        return {
                            ...p,
                            files: nextFiles,
                            file_count: nextFiles.length,
                        };
                    })
                );
                setCurrentProject((prev) => {
                    if (!prev || prev.id !== projectId) return prev;
                    const nextFiles = (prev.files || []).filter((f) => f.id !== fileId);
                    return { ...prev, files: nextFiles, file_count: nextFiles.length };
                });
                message.success('已移除文件');
            },
        });
    };

    const columns = [
        {
            title: '项目名称',
            dataIndex: 'project_name',
            key: 'project_name',
            render: (_: string, record: TechHistoryProject) => (
                <Link to={`/tech-library/history/${encodeURIComponent(String(record.id))}`}>{record.project_name}</Link>
            ),
        },
        {
            title: '中标/完成日期',
            dataIndex: 'winning_date',
            key: 'winning_date',
            width: 150,
        },
        {
            title: '包含文件',
            dataIndex: 'file_count',
            key: 'file_count',
            width: 120,
            render: (count: number) => <Tag color="blue">{count} 个文件</Tag>
        },
        {
            title: '行业标签',
            dataIndex: 'tags',
            key: 'tags',
            render: (tags: string[]) => (
                <>
                    {tags.map(tag => <Tag key={tag}>{tag}</Tag>)}
                </>
            )
        },
        {
            title: '操作',
            key: 'action',
            width: 260,
            render: (_: unknown, record: TechHistoryProject) => (
                <Space size="middle">
                    <Button type="link" icon={<CloudUploadOutlined />} onClick={() => handleOpenUpload(record)}>上传标书</Button>
                    <Button type="link" icon={<EditOutlined />} onClick={() => handleOpenEdit(record)}>
                        修改
                    </Button>
                    <Button
                        type="link"
                        icon={<DeleteOutlined />}
                        onClick={() => handleDeleteProject(record)}
                        style={{ color: DELETE_ACTION_COLOR }}
                    >
                        删除
                    </Button>
                </Space>
            )
        }
    ];

    return (
        <div style={{ padding: '0 0 24px 0' }}>

            <Card bordered={false} styles={{ body: { padding: 0 } }} style={{ boxShadow: '0 2px 8px rgba(0,0,0,0.05)', borderRadius: '12px', overflow: 'hidden' }}>
                <div style={{ padding: '16px 24px', borderBottom: '1px solid #f0f0f0' }}>
                    <Input
                        prefix={<SearchOutlined style={{ color: '#bfbfbf' }} />}
                        placeholder="搜索项目名称、年份或标签..."
                        style={{ width: 400 }}
                    />
                </div>
                <Table
                    columns={columns}
                    dataSource={projects}
                    rowKey="id"
                    pagination={{ pageSize: 10 }}
                />
            </Card>

            <Modal
                title="删除历史项目"
                open={deleteTarget != null}
                onCancel={() => {
                    deleteTargetRef.current = null;
                    setDeleteTarget(null);
                }}
                onOk={() => Promise.resolve(handleConfirmDeleteProject())}
                okText="删除"
                okButtonProps={{ danger: true }}
                cancelText="取消"
            >
                <Text>
                    确定删除「{deleteTarget?.project_name}」吗？此操作不可恢复。
                </Text>
            </Modal>

            <Modal
                title={historyModalMode === 'create' ? '录入历史项目' : '修改历史项目'}
                open={historyModalOpen}
                onCancel={closeHistoryModal}
                onOk={handleHistoryModalOk}
                okText={historyModalMode === 'create' ? '确定' : '保存'}
                width={560}
                destroyOnClose
            >
                <Form form={projectForm} layout="vertical" preserve={false}>
                    <HistoryProjectFormFields />
                </Form>
            </Modal>

            {/* Upload Files Modal */}
            <Modal
                title={
                    <Space>
                        <CloudUploadOutlined style={{ color: '#1890ff' }} />
                        <span>管理项目技术标文件：{currentProject?.project_name}</span>
                    </Space>
                }
                open={isUploadModalVisible}
                onCancel={() => setIsUploadModalVisible(false)}
                footer={null}
                width={800}
                destroyOnClose
                styles={{ body: { paddingBottom: 32 } }}
            >
                <div style={{ marginBottom: 24 }}>
                    <Text type="secondary">上传技术标的主文件（施工组织设计、技术方案）及相关附件。AI 将基于这些文件进行知识提取。</Text>
                </div>

                <div
                    style={{
                        display: 'grid',
                        gridTemplateColumns: '1fr 300px',
                        gap: 24,
                        alignItems: 'start',
                        paddingBottom: 20,
                    }}
                >
                    <div>
                        <Divider style={{ marginTop: 0 }}>已上传文件 ({currentProject?.files?.length || 0})</Divider>
                        <div style={{ maxHeight: 400, overflowY: 'auto', paddingRight: 8 }}>
                            {currentProject?.files && currentProject.files.length > 0 ? (
                                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                                    {currentProject.files.map(file => (
                                        <Card size="small" key={file.id} style={{ borderRadius: 8 }}>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                                <Space size={12}>
                                                    <div style={{ width: 40, height: 40, borderRadius: 8, backgroundColor: file.type === 'pdf' ? '#fff1f0' : '#e6f7ff', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                                                        {file.type === 'pdf' ? <FilePdfOutlined style={{ color: '#ff4d4f', fontSize: 20 }} /> : <FileWordOutlined style={{ color: '#1890ff', fontSize: 20 }} />}
                                                    </div>
                                                    <div>
                                                        <Text strong style={{ display: 'block' }}>{file.name}</Text>
                                                        <Space split={<Divider type="vertical" />} style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)' }}>
                                                            <span>{file.size}</span>
                                                            <span>{file.upload_date}</span>
                                                            <Tag color={file.role === '主标书' ? 'blue' : 'default'}>{file.role}</Tag>
                                                        </Space>
                                                    </div>
                                                </Space>
                                                <Button
                                                    type="text"
                                                    icon={<DeleteOutlined />}
                                                    onClick={() =>
                                                        currentProject &&
                                                        handleDeleteProjectFile(currentProject.id, file.id)
                                                    }
                                                    style={{ color: DELETE_ACTION_COLOR }}
                                                    aria-label="移除文件"
                                                />
                                            </div>
                                        </Card>
                                    ))}
                                </div>
                            ) : (
                                <div style={{ padding: '40px 0', textAlign: 'center', backgroundColor: '#fafafa', borderRadius: 8 }}>
                                    <FileTextOutlined style={{ fontSize: 40, color: '#d9d9d9', marginBottom: 16 }} />
                                    <p style={{ color: '#8c8c8c' }}>暂无技术标文件，请从右侧上传</p>
                                </div>
                            )}
                        </div>
                    </div>

                    <div>
                        <Divider style={{ marginTop: 0 }}>上传新文件</Divider>
                        <Upload.Dragger
                            name="file"
                            action="/api/files/upload"
                            headers={{ 'X-Company-Id': currentCompanyId || '' }}
                            data={{ 
                                source_module: 'tech_library',
                                source_project_id: currentProject?.id 
                            }}
                            multiple
                            showUploadList
                            accept=".pdf,.doc,.docx"
                            style={{ padding: '20px 0' }}
                            onChange={handleTechHistoryUploadChange}
                        >
                            <p className="ant-upload-drag-icon">
                                <InboxOutlined style={{ color: '#1890ff' }} />
                            </p>
                            <p className="ant-upload-text" style={{ fontSize: 13 }}>点击或拖拽文件</p>
                            <p className="ant-upload-hint" style={{ fontSize: 12 }}>支持 PDF/Word</p>
                        </Upload.Dragger>
                    </div>
                </div>

                {(currentProject?.files?.length ?? 0) > 0 && (
                    <div
                        style={{
                            marginTop: 40,
                            paddingTop: 24,
                            paddingBottom: 8,
                            borderTop: '1px solid #f0f0f0',
                            display: 'flex',
                            justifyContent: 'center',
                            gap: 12,
                        }}
                    >
                        <Button onClick={() => setIsUploadModalVisible(false)}>关闭</Button>
                        <Button
                            type="primary"
                            onClick={() => {
                                message.success('已保存');
                                setIsUploadModalVisible(false);
                            }}
                        >
                            保存更改
                        </Button>
                    </div>
                )}
            </Modal>
        </div>
    );
};

const TechHistoryLibrary: React.FC = () => (
    <ConfigProvider locale={zhCN}>
        <App>
            <TechHistoryLibraryInner />
        </App>
    </ConfigProvider>
);

export default TechHistoryLibrary;
