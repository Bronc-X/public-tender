# AI工作台

## 1. 项目概览

该项目是一个围绕**投标资料管理、商务标/技术标制作、文件解析与知识沉淀**构建的业务平台，整体采用**前后端分离**架构：

- **前端**：`frontend`，基于 React + TypeScript + Vite + Ant Design
- **后端**：`backend_go`，基于 Go + Gin + SQLite
- **文档区**：`docs`，存放技术方案、数据库设计和专项实施文档
- **运行辅助层**：根目录下有若干静态服务/代理服务程序与日志文件

从入口代码和目录结构看，这个项目的核心目标包括：

1. 多公司维度下的投标资料管理
2. 文件上传、OCR/解析、审核、归档
3. 商务标项目管理
4. 技术标项目生成与校验
5. 历史项目知识抽取与知识库沉淀
6. Prompt、AI、OCR、存储等系统配置管理

---

## 2. 项目根目录结构

```text
bid_data_management/
├── backend_go/                 # Go 后端主工程
├── frontend/                   # React 前端主工程
├── docs/                       # 项目方案与设计文档
├── tmp/                        # 临时目录
├── production_server.go        # 生产态静态资源 + API 反向代理服务
├── static_server.go            # 静态资源服务（带 SPA fallback）
├── server.js                   # Node 版静态资源 + API 代理服务
├── test.js                     # 测试脚本
├── *.log                       # 运行日志
├── cto_plan.txt                # 方案类文本
├── db_changes.txt              # 数据库变更说明
└── *.docx                      # 业务/实施方案文档
```

### 根目录各部分作用说明

| 路径 | 作用 | 是否核心 |
|---|---|---|
| `backend_go` | 业务后端、数据库访问、AI/OCR/文件处理、API 路由 | 是 |
| `frontend` | 管理后台/工作台界面 | 是 |
| `docs` | 技术方案、数据库草案、任务清单 | 重要辅助 |
| `production_server.go` / `static_server.go` / `server.js` | 前端静态资源托管与 API 转发 | 运维辅助 |
| `*.log` | 运行日志 | 非源码 |
| `tmp` | 临时文件 | 非核心 |

---

## 3. 后端代码结构（`backend_go`）

### 3.1 目录结构

```text
backend_go/
├── cmd/                        # 程序入口
│   └── server/
│       └── main.go             # 后端启动入口
├── internal/
│   ├── config/                 # 内部配置（当前目录较轻）
│   ├── db/                     # SQLite 初始化、Schema Patch
│   ├── handler/                # HTTP 接口处理层
│   ├── middleware/             # 中间件
│   ├── model/                  # 数据模型
│   ├── repository/             # 仓储层预留/轻量目录
│   ├── router/                 # 路由层预留/轻量目录
│   └── service/                # 业务服务层（核心）
├── migrations/                 # 迁移相关资源
├── pkg/
│   └── common/                 # 公共工具
├── data/                       # SQLite 数据文件、上传数据等
├── go.mod                      # Go 依赖定义
└── go.sum                      # Go 依赖锁定
```

### 3.2 后端分层说明

#### 1）入口层：`cmd/server/main.go`
负责完成整个后端系统的装配：

- 初始化 SQLite 数据库
- 启动时执行 Schema Patch
- 构建系统设置、Prompt、AI Client、文件处理、知识抽取、技术标生成等服务
- 创建各业务 Handler
- 注册 `/api` 路由
- 挂载中间件
- 配置未命中路由的代理逻辑

这是整个后端的**总装配入口**。

#### 2）数据库层：`internal/db`
负责数据库初始化与结构演进。

核心文件：

- `internal/db/db.go`
  - 初始化 SQLite 连接
  - 强制单连接，降低 SQLite 锁冲突概率
  - 返回 `Unsafe()` DB，兼容表结构演进后 `SELECT *` 的扫描问题
- `internal/db/schema_patch.go`
  - 启动时自动补齐表、索引、字段
  - 包含知识抽取、行业骨架、技术标步骤四强映射、覆盖率校验等表结构补丁

这说明该项目采用了**运行时增量补丁**而不是完全依赖独立迁移框架。

#### 3）接口层：`internal/handler`
每个业务域通常对应一个 handler 文件，对外暴露 HTTP API。

已识别的主要 handler 包括：

- `company.go`：公司管理
- `person.go`：人员库
- `qualification.go`：资质库
- `performance.go`：业绩库
- `honor.go`：荣誉库
- `file.go`：文件中心、上传、下载、解析、路由
- `audit.go`：审核中心
- `issue.go`：异常中心
- `export.go`：导出中心
- `bid_project.go`：商务标项目
- `tech_bid_project.go`：技术标项目主流程
- `chapter.go`：技术标章节计划与内容生成
- `risk_review.go`：技术标风险复核
- `knowledge_library.go`：技术知识库
- `knowledge_extract_handler.go`：历史项目知识抽取
- `prompt_handler.go`：Prompt 配置中心
- `settings.go`：系统设置
- `storage.go`：本地存储/目录打开
- `dashboard.go`：首页统计
- `cache_stats.go`：缓存统计
- `import.go`：导入与 OCR 任务
- `auth.go`：登录入口/鉴权辅助
- `shared_tender.go`：共享标书候选与绑定处理

#### 4）服务层：`internal/service`
这是后端最核心的部分，承载主要业务逻辑。

重点服务文件如下：

| 文件 | 作用 |
|---|---|
| `ai_client.go` | AI 接口客户端封装 |
| `settings.go` | 系统配置读取，如 AI/OCR 配置 |
| `prompt_service.go` | Prompt 模板读取、版本管理 |
| `file_service.go` | 文件资产管理 |
| `file_archive_service.go` | 文件归档与入库 |
| `file_task_service.go` | 文件任务/OCR/解析调度 |
| `file_review_service.go` | 文件审核相关处理 |
| `docmind_parse_service.go` | 文档解析服务，对接阿里云 DocMind |
| `pdf_ocr_bridge.go` | PDF OCR 桥接处理 |
| `knowledge_extract_service.go` | 从历史项目中抽取知识、生成任务、提交入库 |
| `industry_skeleton.go` | 行业骨架管理 |
| `outline_elastic_engine.go` | 目录弹性生成引擎 |
| `outline_strong_mapping.go` | Step4 facts→目录强映射逻辑 |
| `requirement_full_response.go` | 完全响应率校验 |
| `tender_conflict_audit.go` | 技术标冲突审计 |
| `full_response_gate.go` | 步骤门禁/准入校验 |
| `industry_hard_rules.go` | 行业硬规则约束 |
| `similarity.go` | 相似度分析 |
| `tender_digitization.go` | 技术标数字化/事实抽取/章节生成主服务 |
| `cache_logger.go` | 缓存指标与观测 |

#### 5）中间件层：`internal/middleware`
从启动入口可确认当前主要中间件包括：

- `CORS()`：跨域处理
- `CompanyID()`：公司维度上下文注入
- `AuthRequired()`：鉴权保护
- `NoRouteProxy()`：未命中路由时转发给其他后端/代理目标

这表明项目在请求链路里明确引入了**多公司上下文**与**代理兼容模式**。

---

## 4. 后端核心业务模块

### 4.1 基础资料库模块
对应公司与基础投标资料管理：

- 公司管理
- 人员库
- 资质库
- 业绩库
- 荣誉库

这部分是项目的**基础数据底座**，为后续商务标/技术标生成提供企业素材。

### 4.2 文件中心模块
负责文件资产全生命周期：

- 文件上传
- OCR / 文档解析
- 文件详情查看
- 文件二进制下载
- 页面级预览
- 文件路由到不同业务模块
- 文件删除

前后端 API 中都明确体现出该模块是整个平台的重要入口。

### 4.3 审核与异常模块
围绕解析后的结构化结果进行人工确认：

- 审核列表
- 审核详情
- 审核确认与入库
- 审核忽略
- 异常中心问题处理

这说明项目不是“纯自动化抽取”，而是**AI + 人工复核**的业务闭环。

### 4.4 商务标项目模块
商务标部分以项目为中心，能力包括：

- 创建/查询/更新/删除项目
- 启动工作流
- 获取项目动作清单
- 获取输出结果

### 4.5 技术标项目模块
这是当前项目最复杂、最有特色的模块。

从 API 和服务命名看，技术标模块包括：

1. 技术标项目管理
2. 技术标文件绑定
3. 路线选择
4. 目录生成
5. 目录事实映射
6. 目录覆盖率检查
7. 招标要求完整响应率检查
8. 冲突审计
9. 章节内容生成
10. 人工确认、回退、强制解锁、覆盖准入
11. 结构计划审批/驳回

这是一个典型的**多步骤 AI 工作流系统**，不是单点生成工具。

### 4.6 知识库与知识抽取模块
项目支持从历史技术标项目中抽取知识并沉淀：

- 历史项目列表
- 历史项目文件列表
- Prompt 模板选择
- 创建抽取任务
- 获取抽取任务状态
- 提交抽取结果入库
- 技术知识项 CRUD

这说明系统具备**经验复用与知识资产化**能力。

### 4.7 Prompt / 设置 / 存储模块
系统还具备较强的平台化特征：

- Prompt 分类与模板管理
- OCR 设置
- AI 设置
- 行业骨架设置
- 缓存统计
- 本地存储信息查看
- 打开导入目录/数据库目录

这部分说明项目不只是业务界面，还包含一定的**AI 运维后台**能力。

---

## 5. 前端代码结构（`frontend`）

### 5.1 目录结构

```text
frontend/
├── src/
│   ├── api/                    # 后端 API 封装
│   ├── assets/                 # 静态资源
│   ├── components/             # 复用组件
│   ├── constants/              # 常量定义
│   ├── context/                # 全局上下文
│   ├── layout/                 # 页面总体布局
│   ├── lib/                    # 前端逻辑工具
│   ├── pages/                  # 页面级模块
│   ├── store/                  # 状态管理预留/轻量目录
│   ├── utils/                  # 工具函数
│   ├── App.tsx                 # 路由主入口
│   └── main.tsx                # React 启动入口
├── package.json                # 前端依赖定义
├── vite.config.ts              # Vite 配置与代理
├── tsconfig*.json              # TS 配置
└── eslint.config.js            # ESLint 配置
```

### 5.2 前端入口关系

#### 1）`src/main.tsx`
负责创建 React 根节点并渲染 `App`。

#### 2）`src/App.tsx`
负责：

- 包装 `CompanyProvider`
- 启动 `BrowserRouter`
- 定义主路由
- 组织页面层级
- 支持审核详情抽屉叠层路由

从这里可以看出前端页面基本完整覆盖了：

- 首页工作台
- 文件中心
- 资料库
- 商务标制作
- 技术标制作
- 技术知识库/标书库
- 异常中心
- 系统设置

#### 3）`src/layout/MainLayout.tsx`
负责整个后台的骨架布局：

- 左侧菜单
- 顶部折叠/展开
- 当前公司选择器
- 内容区 `Outlet`

左侧菜单直接体现了产品一级导航：

- 首页工作台
- 文件库
- 资料库
- 标书制作
- 异常中心
- 系统设置

#### 4）`src/context/CompanyContext.tsx`
这是前端非常关键的全局上下文：

- 读取公司列表 `/api/companies`
- 管理当前公司 ID
- 持久化到 `localStorage`
- 通过 Axios 全局拦截器自动注入 `X-Company-Id`

这意味着前端的绝大多数请求都是**强依赖当前公司上下文**的。

---

## 6. 前端主要页面模块说明

`src/pages` 下页面较多，整体上与后端业务域高度对应。

### 6.1 首页与工作台
- `Dashboard.tsx`：首页汇总与概览

### 6.2 资料库模块
- `PersonLibrary.tsx` / `PersonDetail.tsx` / `PersonEdit.tsx`
- `QualificationLibrary.tsx` / `QualificationDetail.tsx` / `QualificationEdit.tsx`
- `PerformanceLibrary.tsx` / `PerformanceCreate.tsx` / `PerformanceDetail.tsx` / `PerformanceEdit.tsx`
- `HonorLibrary.tsx` / `HonorDetail.tsx` / `HonorEdit.tsx`

### 6.3 文件与审核模块
- `FileCenter.tsx`：文件中心主页面
- `AuditDetail.tsx`：审核详情页
- `AuditDetailDrawerRoute.tsx`：抽屉式审核详情路由
- `IssueCenter.tsx`：异常中心
- `ExportCenter.tsx`：导出中心

### 6.4 标书制作模块
- `BidProjectList.tsx`：商务标项目列表
- `BidProjectWorkbench.tsx`：商务标工作台
- `TechBidProjectList.tsx`：技术标项目列表
- `TechBidProjectWorkbench.tsx`：技术标工作台
- `TechBidPlaceholder.tsx`：技术标占位/过渡页面

### 6.5 技术知识与历史标书模块
- `TechHistoryLibrary.tsx`：历史标书库
- `TechHistoryProjectDetail.tsx`：历史项目详情
- `TechKnowledgeHub.tsx`：技术知识中心
- `TechKnowledgeLibrary.tsx`：相关知识展示页

### 6.6 系统设置模块
- `SystemSettings.tsx` / `Settings.tsx`

> 备注：`pages` 目录中同时存在一部分早期页面或兼容性页面，如 `Audits.tsx`、`Imports.tsx`、`Issues.tsx`、`Library.tsx`、`Placeholders.tsx` 等，当前主路由真正使用的是 `App.tsx` 中注册的页面集合。

---

## 7. 前端 API 层说明

当前明确识别到的 API 文件有：

### 7.1 `src/api/fileCenter.ts`
封装文件中心相关接口：

- 文件列表
- 文件详情
- 文件删除
- 文件上传
- 审核列表
- 审核详情
- 审核确认/忽略
- OCR 任务状态
- 手动启动 OCR 任务

该文件体现了**文件中心 = 上传 + 解析 + 审核 + 入库**的完整流程。

### 7.2 `src/api/techBidStep4.ts`
封装技术标 Step4 相关接口：

- facts→目录映射获取
- 覆盖率结果获取
- 招标要求总表获取
- 完全响应率检查获取
- 冲突审计获取
- Step4 Gate 人工覆盖
- 结构计划查询、审批、驳回

这是理解技术标模块最关键的前端 API 文件之一，说明该项目已经把**技术标目录生成与审计流程产品化**。

---

## 8. 主要依赖关系

## 8.1 前端依赖

`frontend/package.json` 中核心依赖如下：

- `react` / `react-dom`：前端框架
- `react-router-dom`：路由系统
- `axios`：HTTP 请求
- `antd` / `@ant-design/icons`：后台 UI 组件体系
- `@tanstack/react-query`：服务端状态/异步数据管理
- `zustand`：轻量状态管理
- `dayjs`：时间处理
- `react-markdown` + `remark-gfm`：Markdown 渲染
- `framer-motion`：动画
- `lucide-react`：图标
- `tailwind-merge`：样式合并

开发依赖包括：

- `vite`
- `typescript`
- `eslint`
- `tailwindcss`
- `postcss`
- `autoprefixer`

### 8.2 后端依赖

`backend_go/go.mod` 中核心依赖如下：

- `github.com/gin-gonic/gin`：Web 框架
- `github.com/jmoiron/sqlx`：数据库访问增强
- `modernc.org/sqlite`：SQLite 驱动
- `github.com/google/uuid`：ID 生成
- `github.com/go-playground/validator/v10`：参数校验
- `github.com/xuri/excelize/v2`：Excel 处理
- `github.com/alibabacloud-go/docmind-api-20220711`：阿里云文档解析能力
- `github.com/alibabacloud-go/darabonba-openapi/v2`、`tea`：阿里云 SDK 支撑

### 8.3 外部能力依赖

从代码命名可判断项目依赖以下外部能力：

- AI 大模型能力
- OCR / 文档解析能力
- 本地文件系统
- SQLite 本地数据库
- 可能兼容外部 Node/Java 服务

其中一个明显特征是：

- Vite 开发代理指向 `http://localhost:8081`
- 后端 `NoRouteProxy` 默认会把未命中的请求转发到 `http://localhost:8889`
- `performance` 路由里存在对 Java 健康检查的兼容逻辑

说明项目处在**多后端并存/迁移兼容阶段**。

---

## 9. 核心依赖关系与调用链

### 9.1 前端到后端

调用链如下：

```text
页面（pages）
  -> API 封装层（src/api）
  -> Axios 请求
  -> CompanyContext 自动注入 X-Company-Id
  -> Vite 代理 / 生产代理
  -> Go 后端 /api 与 /files
```

### 9.2 后端内部依赖链

后端主链路如下：

```text
main.go
  -> InitDB / ApplySchemaPatches
  -> SettingsService / PromptService / CacheMetricsService
  -> AIClient
  -> ElasticOutlineEngine
  -> TenderDigitizationService
  -> FileService / FileTaskService / FileReviewService / KnowledgeExtractService
  -> 各业务 Handler
  -> Gin Route 注册
```

也就是说，系统是典型的：

```text
数据库层 -> 服务层 -> 接口层 -> 路由层
```

### 9.3 技术标主流程依赖

从文件命名与路由可以推导技术标主流程大致为：

```text
技术标项目
  -> 绑定招标文件
  -> 文档解析 / OCR
  -> facts 抽取
  -> 行业骨架加载
  -> 目录生成
  -> facts 强映射
  -> 覆盖率检查
  -> 完全响应率检查
  -> 冲突审计
  -> 结构计划审批
  -> 章节内容生成
  -> 输出导出
```

这是项目最核心、最有业务价值的一条主链路。

### 9.4 文件处理主流程依赖

```text
文件上传
  -> 文件资产记录
  -> OCR/DocMind 解析
  -> 提取结构化信息
  -> 进入审核台
  -> 人工确认/忽略
  -> 归档到资料库或项目模块
```

---

## 10. 关键配置与功能文件说明

### 10.1 后端关键文件

| 文件 | 说明 |
|---|---|
| `backend_go/cmd/server/main.go` | 后端启动入口，装配所有服务与 API |
| `backend_go/internal/db/db.go` | SQLite 初始化，单连接与 Unsafe 模式 |
| `backend_go/internal/db/schema_patch.go` | 启动时自动补齐数据库结构 |
| `backend_go/internal/service/tender_digitization.go` | 技术标数字化与核心 AI 工作流服务 |
| `backend_go/internal/service/knowledge_extract_service.go` | 历史项目知识抽取与提交 |
| `backend_go/internal/service/docmind_parse_service.go` | 文档解析服务 |
| `backend_go/internal/service/prompt_service.go` | Prompt 管理 |
| `backend_go/internal/handler/tech_bid_project.go` | 技术标项目 API 主处理器 |
| `backend_go/internal/handler/file.go` | 文件中心 API 主处理器 |
| `backend_go/internal/handler/settings.go` | 系统配置 API |

### 10.2 前端关键文件

| 文件 | 说明 |
|---|---|
| `frontend/src/main.tsx` | React 应用启动入口 |
| `frontend/src/App.tsx` | 主路由定义与全局 Provider 包装 |
| `frontend/src/layout/MainLayout.tsx` | 后台整体布局与导航 |
| `frontend/src/context/CompanyContext.tsx` | 多公司上下文与 Axios 请求头注入 |
| `frontend/src/pages/FileCenter.tsx` | 文件中心主页面 |
| `frontend/src/pages/TechBidProjectWorkbench.tsx` | 技术标工作台核心页面 |
| `frontend/src/pages/BidProjectWorkbench.tsx` | 商务标工作台 |
| `frontend/src/pages/TechKnowledgeHub.tsx` | 技术知识中心 |
| `frontend/src/api/fileCenter.ts` | 文件中心接口层 |
| `frontend/src/api/techBidStep4.ts` | 技术标 Step4 接口层 |
| `frontend/vite.config.ts` | 开发代理配置 |
| `frontend/package.json` | 前端依赖与脚本定义 |

### 10.3 根目录辅助运行文件

| 文件 | 说明 |
|---|---|
| `production_server.go` | 把前端 `dist` 作为静态站点对外提供，并把 `/api`、`/files` 转发到 8081 |
| `static_server.go` | 静态资源服务，支持 SPA 路由 fallback |
| `server.js` | Node 版静态服务器 + API 代理 |

这说明项目在部署方式上可能存在多种历史实现或调试方式并行。

---

## 11. `docs` 目录说明

`docs` 中目前能看到的文档主要围绕**技术标 Step4 目录强映射**展开，例如：

- `第四步目录强映射实施方案_含facts映射示例.md`
- `第四步目录强映射_后端函数拆分建议.md`
- `第四步目录强映射_antigravity开发任务清单.md`
- `第四步目录强映射_数据库表设计草案.md`

这些文档与后端中以下能力高度对应：

- `outline_strong_mapping.go`
- `requirement_full_response.go`
- `tender_conflict_audit.go`
- `schema_patch.go` 中的 Step4 强映射相关表结构
- 前端 `src/api/techBidStep4.ts`

因此可以判断：

> `docs` 目录并不是普通文档区，而是与当前技术标核心功能直接对应的专项设计资料库。

---

## 12. 当前项目的整体判断

### 12.1 架构定位
这是一个典型的：

- **前端工作台** + **Go 业务后端** + **SQLite 本地数据存储** + **AI/OCR 能力集成**

的业务平台。

### 12.2 业务定位
从模块设计看，项目核心定位不是简单的文件系统，而是：

> 面向投标场景的“资料库 + 文件解析 + 审核归档 + 商务标/技术标生成 + 知识复用”一体化工作台。

### 12.3 技术亮点
当前最有辨识度的核心能力有三块：

1. **多公司上下文管理**
2. **文件解析 + 审核 + 归档闭环**
3. **技术标 Step4 强映射/覆盖率/响应率/冲突审计的 AI 工作流**

### 12.4 当前代码形态特点
项目呈现出以下代码特征：

- 主体源码集中，目录层次清晰
- 前后端业务域命名基本一致，便于联调
- 后端保留了一定兼容层，可能仍在系统迁移或收敛阶段
- 根目录日志和辅助服务文件较多，运行痕迹明显
- 技术标流程是当前最复杂、最值得重点维护的核心模块

---

## 13. 建议阅读顺序

如果要快速接手该项目，建议按以下顺序阅读：

### 第一层：先看入口与整体装配
1. `backend_go/cmd/server/main.go`
2. `frontend/src/App.tsx`
3. `frontend/src/layout/MainLayout.tsx`
4. `frontend/src/context/CompanyContext.tsx`

### 第二层：再看最关键业务链路
5. `backend_go/internal/service/tender_digitization.go`
6. `backend_go/internal/service/outline_strong_mapping.go`
7. `backend_go/internal/service/requirement_full_response.go`
8. `backend_go/internal/service/tender_conflict_audit.go`
9. `frontend/src/api/techBidStep4.ts`
10. `frontend/src/pages/TechBidProjectWorkbench.tsx`

### 第三层：补文件与知识抽取链路
11. `backend_go/internal/handler/file.go`
12. `backend_go/internal/service/file_task_service.go`
13. `backend_go/internal/service/docmind_parse_service.go`
14. `backend_go/internal/service/knowledge_extract_service.go`
15. `frontend/src/api/fileCenter.ts`
16. `frontend/src/pages/FileCenter.tsx`

---

## 14. 一句话总结

这个项目本质上是一个面向投标业务的 **AI 工作台**：以多公司资料库为底座，以文件解析和人工审核为中枢，以商务标/技术标项目为业务主线，并通过知识抽取、Prompt 管理和结构化校验机制，把 AI 生成流程做成可控、可追踪、可复核的产品系统。
