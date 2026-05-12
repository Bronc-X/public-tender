import axios from 'axios';

/**
 * 文件全生命周期状态
 */
export type FileStatus =
  | 'uploaded'      // 已上传，待分析
  | 'analyzing'     // 智能预分析中
  | 'queued'        // OCR/解析排队中
  | 'running'       // 正在处理
  | 'waiting_review' // 处理完成，待人工审核
  | 'pending'        // 待人工核对
  | 'reviewing'      // 正在审核
  | 'approved'      // 审核通过
  | 'archived'      // 已入库
  | 'failed'        // 处理失败
  | 'ignored';      // 已忽略

/**
 * 结构化提取项
 */
export interface ExtractionItem {
  id: string;
  title: string;
  summary?: string;
  content: string;
  confidence: number;
  source_page: string;
}

/**
 * 文件资产定义
 */
export interface FileAsset {
  id: string;
  file_name: string;
  ext?: string;
  mime_type?: string;
  file_size?: number;
  sha256?: string;
  source_path?: string;
  stored_path?: string;
  source_type?: string;
  source_module?: string;
  source_project_id?: string;
  scan_status?: string; // 兼容旧版
  status?: FileStatus;   // V2 统一状态
  archive_status?: string;
  last_task_id?: string;
  last_error_message?: string;
  audit_id?: string;
  /** 审核项推荐入库类型，与审核台下拉一致 */
  object_type?: string;
  archive_target_type?: string;
  archive_target_id?: string;
  created_at?: string;
  updated_at?: string;
  plain_text?: string;
  markdown_text?: string;
}

/**
 * 审核项定义
 */
export interface AuditItem {
  id: string;
  file_id: string;
  file_name?: string;
  mime_type?: string;
  /** 用于审核页在 mime 不可靠时判断 PDF（与后端 AuditDetail 一致） */
  stored_path?: string;
  object_type: string;
  audit_status: 'pending' | 'processing' | 'confirmed' | 'rejected' | 'ignored';
  confidence_score: number;
  risk_level: 'low' | 'medium' | 'high';
  extracted_data?: string; // JSON string
  ocr_text?: string;
  ai_clean_text?: string;
  reviewer_id?: string;
  reviewer_name?: string;
  archive_target_type?: string;
  archive_target_id?: string;
  created_at: string;
}

/**
 * 文件中心 API SDK
 */
const fileCenterApi = {
  // --- 文件库管理 ---

  /** 获取文件列表 */
  listFiles: async () => {
    const resp = await axios.get<FileAsset[]>('/api/files');
    return resp.data;
  },

  /** 获取文件详情 */
  getFileDetail: async (id: string) => {
    const resp = await axios.get<FileAsset>(`/api/files/${id}`);
    return resp.data;
  },

  /** 删除文件资产 */
  deleteFile: async (id: string) => {
    const resp = await axios.delete(`/api/files/${id}`);
    return resp.data;
  },

  /** 上传文件 */
  uploadFile: async (file: File, options?: { source_module?: string, source_project_id?: string }) => {
    const formData = new FormData();
    formData.append('file', file);
    if (options?.source_module) formData.append('source_module', options.source_module);
    if (options?.source_project_id) formData.append('source_project_id', options.source_project_id);

    const resp = await axios.post('/api/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    return resp.data;
  },

  // --- 审核台管理 ---

  /** 获取待审核列表 */
  listAudits: async () => {
    const resp = await axios.get<AuditItem[]>('/api/audits');
    return resp.data;
  },

  /** 获取审核详情 */
  getAuditDetail: async (id: string) => {
    const resp = await axios.get<AuditItem>(`/api/audits/${id}`);
    return resp.data;
  },

  /** 确认审核并入库 */
  confirmAudit: async (id: string, payload: {
    file_id: string,
    extracted_items: any[],
    object_type?: string,
    confirmed_text?: string
  }) => {
    const resp = await axios.post(`/api/audits/${id}/confirm`, payload);
    return resp.data;
  },

  /** 忽略审核记录 */
  ignoreAudit: async (id: string) => {
    const resp = await axios.post(`/api/audits/${id}/ignore`);
    return resp.data;
  },

  // --- 任务管理 ---

  /** 获取 OCR/解析任务状态 */
  getTaskStatus: async (taskId: string) => {
    const resp = await axios.get(`/api/imports/tasks/${taskId}`);
    return resp.data;
  },

  /** 手动启动 OCR 识别任务 */
  startOcrTask: async (fileId: string, mode: string = 'accurate') => {
    const resp = await axios.post('/api/tech-bid/import/tasks', {
      source_file_id: fileId,
      ocr_mode: mode
    });
    return resp.data;
  }
};

export default fileCenterApi;
