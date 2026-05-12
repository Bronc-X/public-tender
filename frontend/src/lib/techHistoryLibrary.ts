/** 标书库（历史项目）本地存储与类型，供列表页与详情页共用 */

export const TECH_HISTORY_STORAGE_KEY = 'bid_tech_history_library_projects_v1';

export interface TechHistoryProjectFile {
    id: string;
    name: string;
    type: 'pdf' | 'doc';
    size: string;
    upload_date: string;
    role: string;
}

export interface TechHistoryProject {
    id: string;
    project_name: string;
    winning_date: string;
    file_count: number;
    status: string;
    tags: string[];
    files?: TechHistoryProjectFile[];
}

const mockData: TechHistoryProject[] = [
    {
        id: '1',
        project_name: '2023年某大型体育场馆建设项目',
        winning_date: '2023-05-12',
        file_count: 3,
        status: 'Archived',
        tags: ['场馆', '大跨度'],
        files: [
            {
                id: 'f1',
                name: '施工组织设计_技术实施方案.pdf',
                type: 'pdf',
                size: '12.5MB',
                upload_date: '2023-05-15',
                role: '主标书',
            },
            {
                id: 'f2',
                name: '大跨度钢结构专项方案.pdf',
                type: 'pdf',
                size: '5.2MB',
                upload_date: '2023-05-16',
                role: '专项方案',
            },
            {
                id: 'f3',
                name: '施工机械配置月计划.docx',
                type: 'doc',
                size: '1.1MB',
                upload_date: '2023-05-16',
                role: '附件',
            },
        ],
    },
    {
        id: '2',
        project_name: '某市中心商办综合体钢结构工程',
        winning_date: '2022-11-20',
        file_count: 2,
        status: 'Archived',
        tags: ['商办', '钢结构'],
    },
    {
        id: '3',
        project_name: '深中通道三标段沉管隧道施工',
        winning_date: '2023-01-05',
        file_count: 5,
        status: 'Completed',
        tags: ['市政', '隧道'],
    },
];

export function readTechHistoryProjects(): TechHistoryProject[] {
    try {
        const raw = localStorage.getItem(TECH_HISTORY_STORAGE_KEY);
        if (!raw) return mockData;
        const parsed = JSON.parse(raw) as unknown;
        if (!Array.isArray(parsed) || parsed.length === 0) return mockData;
        const first = parsed[0] as Record<string, unknown>;
        if (first && typeof first.id === 'string' && typeof first.project_name === 'string') {
            return parsed as TechHistoryProject[];
        }
    } catch {
        /* ignore */
    }
    return mockData;
}

export function findTechHistoryProject(projectId: string): TechHistoryProject | undefined {
    return readTechHistoryProjects().find((p) => String(p.id) === String(projectId));
}
