import React, { useCallback, useEffect, useState } from 'react';
import { Menu, Row, Col, Card, Space, Typography, Button, Modal, Input, Popconfirm, Divider } from 'antd';
import {
    BookOutlined,
    CarOutlined,
    SafetyOutlined,
    PieChartOutlined,
    LockOutlined,
    TrophyOutlined,
    TeamOutlined,
    SolutionOutlined,
    ReadOutlined,
    PlusOutlined,
    DeleteOutlined,
    FolderOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams, Navigate } from 'react-router-dom';
import TechKnowledgeLibrary from './TechKnowledgeLibrary';
import { useCompany } from '../context/CompanyContext';
import type { MenuProps } from 'antd';

const { Title, Text } = Typography;

/** 内置知识库类型（与路由 segment 一致，不可删除） */
export const KNOWLEDGE_SECTION_TYPES = [
    'method',
    'equipment',
    'system',
    'performance',
    'risks',
    'regions',
    'subcontractors',
    'costs',
] as const;

export type KnowledgeSectionType = (typeof KNOWLEDGE_SECTION_TYPES)[number];

/** 用户新增的、与工法库等平级的自定义分类 */
export interface CustomKnowledgeCategory {
    id: string;
    label: string;
}

const SECTION_MENU: { type: KnowledgeSectionType; label: string; icon: React.ReactNode }[] = [
    { type: 'method', label: '工法库', icon: <BookOutlined /> },
    { type: 'equipment', label: '设备库', icon: <CarOutlined /> },
    { type: 'system', label: '制度与规范库', icon: <SafetyOutlined /> },
    { type: 'performance', label: '业绩库（技术）', icon: <PieChartOutlined /> },
    { type: 'risks', label: '风控库', icon: <LockOutlined /> },
    { type: 'regions', label: '企业优势库', icon: <TrophyOutlined /> },
    { type: 'subcontractors', label: '分包库', icon: <TeamOutlined /> },
    { type: 'costs', label: '风险应对措施库', icon: <SolutionOutlined /> },
];

function peerCategoriesStorageKey(companyId: string) {
    return `bid_knowledge_peer_categories_v1_${companyId}`;
}

function isBuiltinType(t: string): t is KnowledgeSectionType {
    return (KNOWLEDGE_SECTION_TYPES as readonly string[]).includes(t);
}

function newCustomCategoryId(): string {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        return `kcat_${crypto.randomUUID().replace(/-/g, '')}`;
    }
    return `kcat_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function loadPeerCategories(companyId: string): CustomKnowledgeCategory[] {
    if (!companyId) return [];
    try {
        const raw = localStorage.getItem(peerCategoriesStorageKey(companyId));
        if (!raw) return [];
        const parsed = JSON.parse(raw) as CustomKnowledgeCategory[];
        return Array.isArray(parsed) ? parsed : [];
    } catch {
        return [];
    }
}

const TechKnowledgeHub: React.FC = () => {
    const { type } = useParams<{ type: string }>();
    const navigate = useNavigate();
    const { currentCompanyId } = useCompany();
    const [customCategories, setCustomCategories] = useState<CustomKnowledgeCategory[]>(() =>
        loadPeerCategories(typeof window !== 'undefined' ? localStorage.getItem('current_company_id') || 'c1' : '')
    );
    const [addOpen, setAddOpen] = useState(false);
    const [newLabel, setNewLabel] = useState('');

    useEffect(() => {
        setCustomCategories(loadPeerCategories(currentCompanyId));
    }, [currentCompanyId]);

    const persistCustom = useCallback(
        (rows: CustomKnowledgeCategory[]) => {
            if (!currentCompanyId) return;
            localStorage.setItem(peerCategoriesStorageKey(currentCompanyId), JSON.stringify(rows));
        },
        [currentCompanyId]
    );

    const isValidRouteType = (t: string | undefined): boolean =>
        !!t && (isBuiltinType(t) || customCategories.some((c) => c.id === t));

    const handleAddCategory = () => {
        const label = newLabel.trim();
        if (!label || !currentCompanyId) return;
        const id = newCustomCategoryId();
        setCustomCategories((prev) => {
            const next = [...prev, { id, label }];
            persistCustom(next);
            return next;
        });
        setNewLabel('');
        setAddOpen(false);
        navigate(`/tech-library/knowledge/${id}`);
    };

    const handleDeleteCategory = (id: string) => {
        setCustomCategories((prev) => {
            const next = prev.filter((c) => c.id !== id);
            persistCustom(next);
            return next;
        });
        if (type === id) {
            navigate('/tech-library/knowledge/method', { replace: true });
        }
    };

    const menuItems: MenuProps['items'] = [
        ...SECTION_MENU.map((s) => ({
            key: s.type,
            icon: s.icon,
            label: s.label,
        })),
        ...customCategories.map((c) => ({
            key: c.id,
            icon: <FolderOutlined />,
            label: (
                <div
                    style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 8,
                        width: '100%',
                    }}
                >
                    <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>{c.label}</span>
                    <Popconfirm
                        title="删除该分类？"
                        description="仅从左侧菜单移除；该分类下已保存的条目不会自动删除。"
                        onConfirm={() => handleDeleteCategory(c.id)}
                        okText="删除"
                        cancelText="取消"
                    >
                        <Button
                            type="text"
                            size="small"
                            icon={<DeleteOutlined />}
                            style={{ color: 'rgba(0,0,0,0.45)' }}
                            onClick={(e) => e.stopPropagation()}
                        />
                    </Popconfirm>
                </div>
            ),
        })),
    ];

    if (!type || !isValidRouteType(type)) {
        return <Navigate to="/tech-library/knowledge/method" replace />;
    }

    const customMeta = customCategories.find((c) => c.id === type);
    const displayOverride =
        !isBuiltinType(type) && customMeta
            ? {
                  title: customMeta.label,
                  icon: <FolderOutlined />,
                  itemLabel: '条目名称',
                  subTitle: '自定义知识库分类，条目仍按公司维度存储',
              }
            : undefined;

    return (
        <div style={{ background: '#f8fafc', minHeight: 'calc(100vh - 150px)', margin: '-24px', padding: '24px' }}>
            <Row gutter={24}>
                <Col span={6}>
                    <Card
                        styles={{ body: { padding: '16px 8px' } }}
                        bordered={false}
                        style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}
                    >
                        <Menu
                            mode="inline"
                            selectedKeys={[type]}
                            style={{ borderInlineEnd: 0 }}
                            items={menuItems}
                            onClick={({ key }) => navigate(`/tech-library/knowledge/${key}`)}
                        />
                        <Divider style={{ margin: '12px 0' }} />
                        <Button type="dashed" block icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
                            新增分类
                        </Button>
                    </Card>
                </Col>
                <Col span={18}>
                    <Card
                        styles={{ body: { minHeight: 480, padding: 32 } }}
                        bordered={false}
                        style={{ borderRadius: 12, boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}
                    >
                        <TechKnowledgeLibrary type={type} displayOverride={displayOverride} />
                    </Card>
                </Col>
            </Row>

            <Modal
                title="新增知识库分类"
                open={addOpen}
                onOk={handleAddCategory}
                onCancel={() => {
                    setAddOpen(false);
                    setNewLabel('');
                }}
                okText="确定"
                cancelText="取消"
                destroyOnClose
            >
                <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                    将与「工法库」「设备库」等并列出现在左侧菜单中。
                </Text>
                <Input
                    placeholder="分类名称，例如：专利库、工法汇编"
                    value={newLabel}
                    onChange={(e) => setNewLabel(e.target.value)}
                    onPressEnter={handleAddCategory}
                    maxLength={32}
                    showCount
                />
            </Modal>
        </div>
    );
};

export default TechKnowledgeHub;
