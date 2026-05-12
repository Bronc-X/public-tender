# Step4 第一阶段开发任务拆解清单

项目：投标工作台 / 技术标第 4 步多 Agent 协作

目标：在第一阶段完成 Step4 的最小可用闭环，让系统具备：
- Coordinator 编排
- 中间工件落库
- 审计结果展示
- 结构方案审批

项目路径：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

---

## 一、第一阶段目标

第一阶段不追求“最强”，只追求“能稳定跑通闭环”。

完成后应达到：
1. 能启动一次 Step4 编排
2. 能落 requirement / mapping / audit / structure plan
3. 能前端查看运行进度与结果
4. 能进行人工审批
5. 能输出 PASS / REVISE / BLOCK

---

## 二、后端任务拆解

## 2.1 数据层

### 任务 1：新增 Step4 表结构
交付：
- `step4_runs`
- `step4_requirements`
- `step4_fact_candidates`
- `step4_fact_mappings`
- `step4_outline_versions`
- `step4_outline_nodes`
- `step4_coverage_audits`
- `step4_full_response_audits`
- `step4_conflict_audits`
- `step4_structure_plans`
- `step4_approval_logs`
- `step4_agent_runs`

验收标准：
- 本地数据库可成功建表
- CRUD 正常

### 任务 2：补 ORM / Model 定义
交付：
- 对应 Go model struct
- 基础 repository 方法

验收标准：
- 可完成增删改查

---

## 2.2 Service 层

### 任务 3：新增 Step4CoordinatorService
职责：
- 创建 run
- 调度各子阶段 service
- 更新状态
- 汇总 gate 结果

验收标准：
- 能从入口接口完整跑通一次

### 任务 4：梳理 Requirement Extraction Service
职责：
- 抽 requirement
- 标记高优先级 / must_be_explicit

验收标准：
- requirement 可结构化落库

### 任务 5：梳理 Fact Retrieval / Mapping Service
职责：
- 生成 fact_candidates
- 建立 requirement 到 fact 映射

验收标准：
- mapping 数据可查询

### 任务 6：梳理 Outline Planning Service
职责：
- 输出目录版本与目录树节点

验收标准：
- 可生成至少一个 outline_version
- 节点树可落库

### 任务 7：梳理 Coverage Audit Service
职责：
- 计算 coverage_rate / full_response_rate
- 输出 missing / weak / only_tagged

验收标准：
- 能给出结果和 summary

### 任务 8：梳理 Conflict Audit Service
职责：
- 输出 conflict 清单
- 判断 has_block

验收标准：
- 能发现至少重复 / 冲突 / 层级问题

### 任务 9：新增 StructurePlanService
职责：
- 根据 audit 结果生成调整动作

验收标准：
- 输出 keep/move/split/merge/promote/insert 动作

---

## 2.3 Handler / API 层

### 任务 10：新增或统一 Step4 接口
第一阶段至少实现：
- `POST /outline/run`
- `GET /outline/run-status`
- `GET /outline/requirements`
- `GET /outline/mappings`
- `GET /outline/coverage`
- `GET /outline/full-response`
- `GET /outline/conflict-audit`
- `GET /structure-plan`
- `POST /structure-plan/approve`
- `POST /structure-plan/reject`
- `POST /outline/step4-gate/override`
- `GET /outline/agent-runs`

验收标准：
- 接口返回统一 JSON 结构
- 前端可直接接入

---

## 三、前端任务拆解

## 3.1 页面与组件

### 任务 11：新增 Step4HeaderBar
显示：
- 当前状态
- Gate 结果
- 当前 Agent
- 覆盖率
- 响应率

### 任务 12：新增 Step4AgentTimeline
显示：
- 各 Agent 执行状态
- 耗时
- 输出摘要

### 任务 13：增强 Step4 目录树视图
功能：
- 目录树展示
- 节点高亮
- 版本切换

### 任务 14：新增 Step4EvidencePanel
显示：
- requirement
- fact
- audit
- rationale

### 任务 15：新增 Step4StructurePlanPanel
显示：
- 调整动作
- 风险提示
- 审批按钮

验收标准：
- 用户可在一个页面看完整个 Step4 过程

---

## 3.2 数据接入

### 任务 16：接入 run-status
驱动：
- 顶部状态
- 时间线
- 进度条

### 任务 17：接入 requirements / mappings / coverage / full-response / conflict-audit
驱动：
- 右侧证据面板
- 风险面板

### 任务 18：接入 structure-plan + 审批动作
驱动：
- approve / reject / override

验收标准：
- Step4 页面能完整展示证据链与审批链

---

## 四、测试任务拆解

### 任务 19：后端接口联调测试
验证：
- 每个接口返回结构正确
- 状态推进正确
- 异常时返回可解释错误

### 任务 20：Step4 全链路测试
测试场景：
1. 正常通过（PASS）
2. 可修正（REVISE）
3. 阻断（BLOCK）
4. 人工 override
5. 人工 reject

### 任务 21：前端交互测试
验证：
- 页面刷新后状态不丢失
- 时间线展示正确
- 节点联动正确
- 审批动作可回显

---

## 五、建议角色分工

### 后端工程师
负责：
- 表结构
- model/repository
- coordinator/service
- API

### 前端工程师
负责：
- 页面布局
- 时间线
- 目录树
- 证据面板
- 审批面板

### 测试工程师
负责：
- 状态流转测试
- Gate 结果测试
- 边界输入测试
- 人工审批测试

### CTO / 技术负责人
负责：
- 审查状态机
- 审查接口边界
- 审查是否真正实现“生成与审计分离”

---

## 六、阶段验收标准

第一阶段完成后，至少满足以下条件：

### 功能标准
- 可以发起 Step4 编排
- 可以看到 Agent 执行过程
- 可以查看 requirement / mapping / audit / structure plan
- 可以执行 approve / reject / override

### 架构标准
- Coordinator 已建立
- 中间工件已落库
- 状态机清晰
- 接口结构统一

### 业务标准
- 至少能稳定区分 PASS / REVISE / BLOCK
- 人工能理解目录为什么这样编
- 人工能明确看到缺失项和风险项

---

## 七、推荐开发顺序

建议按以下顺序做：

1. 建表
2. model / repository
3. coordinator service
4. requirement / mapping / outline / audit service 串起来
5. API 输出统一化
6. 前端状态栏 + 时间线
7. 前端目录树 + 证据面板
8. 前端结构方案审批
9. 联调测试
10. 验收

---

## 八、第一阶段不做什么

为了控制范围，第一阶段建议先不做：
- 多版本复杂对比算法
- 自动多轮自修复
- 通用 Agent 平台
- 复杂权限系统
- 太深的 prompt 策略优化

先把闭环打通，再迭代增强。

---

## 九、一句话总结

第一阶段最重要的不是“把 AI 做得多聪明”，而是：

**把 Step4 做成一个可运行、可追踪、可审批的多 Agent 最小闭环。**

只要这个闭环跑通，后面优化质量、成本、规则、策略都会容易很多。
