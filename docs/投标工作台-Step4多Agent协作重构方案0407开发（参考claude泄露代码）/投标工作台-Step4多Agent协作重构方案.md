# 投标工作台 Step4 多 Agent 协作重构方案

## 一、文档目标

本文面向 **投标工作台** 项目，聚焦技术标制作第 4 步“目录生成”，给出一套可落地的 **多 Agent 协作重构方案**。

项目代码位置：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

目标不是做一个通用 Agent 平台，而是在现有系统基础上，把 Step4 从“单次生成目录”升级为：

**要求解析 → 事实映射 → 候选目录生成 → 覆盖校验 → 冲突审计 → 结构修正 → 人工审批**

从而提升目录质量、可解释性、可审计性和后续正文生成质量。

---

## 二、为什么要重构 Step4

当前 Step4 已经具备较好的基础，前端接口层已体现出以下核心工件：

- `Step4RequirementRow`：招标要求真相层
- `Step4FactMapping`：事实映射
- `Step4Coverage`：覆盖率校验
- `Step4FullResponse`：完全响应率校验
- `Step4ConflictAudit`：冲突审计
- `StructurePlan`：结构调整方案

这说明当前系统已经具备“多工件 + 多阶段校验”的雏形。

但若目录生成仍主要依赖单次 AI 生成，则会有以下问题：

1. **目录稳定性不足**：同一项目多次生成结果波动大
2. **可解释性不足**：人工无法快速理解“为什么这么编”
3. **漏项风险高**：必须响应条款容易被遗漏或弱响应
4. **难以扩展**：后续正文生成难以利用目录的结构化信息
5. **难以多人协作优化**：规则、策略、审计耦合在一起

因此，Step4 最合适的演进方向是：

**从“生成一个目录”升级为“编排多个专职 Agent 共同产出可审计目录”**。

---

## 三、可借鉴的核心思想

可参考社区高热开源 Agent 项目的工程思想，但只借鉴其 **架构逻辑**，不直接照搬源码实现。

最值得学习的不是外壳，而是以下 5 个核心思想：

### 1. Coordinator / Subagent 调度
把复杂任务拆成多个角色并行或串行协作，再由一个协调器统一裁决。

### 2. 按职责分上下文
不同 Agent 只读取自己需要的上下文，而不是把所有资料一次性塞给单个模型。

### 3. 中间工件化
Agent 产出的不是直接“最终答案”，而是可审计、可追踪、可复用的结构化中间结果。

### 4. 审计闭环
生成后必须经过覆盖率、强响应率、冲突、禁写项、风险项等审计。

### 5. 权限与职责边界
生成者不能兼任审核者；读招标要求、读企业事实、改目录、做审计的能力边界要明确。

---

## 四、重构后的 Step4 总体架构

建议将 Step4 设计成一个 **专用多 Agent 编排器**，而不是通用 Agent 平台。

### 4.1 核心角色

建议拆分为 6 个角色：

#### 1）Requirement Agent（招标要求解析代理）
职责：
- 从招标文件中提取必须响应条款
- 提取评分点与关键响应点
- 标注高风险条款、废标项、必须显式响应项
- 生成结构化 requirement 清单

输出：
- `requirement_rows`
- 风险等级
- 必须显式响应标记
- 期望响应层级

#### 2）Fact Agent（企业事实检索代理）
职责：
- 从企业知识库中检索可支撑事实
- 识别可用于技术标目录的资质、业绩、设备、工艺、组织、制度、类似项目经验等
- 生成 requirement 到 fact 的候选映射

输出：
- `fact_candidates`
- `fact_mappings`
- 事实支持强度评分

#### 3）Outline Planner Agent（目录规划代理）
职责：
- 基于 requirement + fact 生成候选目录树
- 兼顾招标逻辑、评分逻辑、行业习惯、项目特征
- 生成多个候选结构版本（可选）

输出：
- `candidate_outline_versions`
- 章节层级
- 每章响应目标说明

#### 4）Coverage Auditor Agent（覆盖率审计代理）
职责：
- 检查每个 requirement 是否被目录显式覆盖
- 检查是否仅挂标签但无实质响应
- 标识缺失项、弱响应项、空壳标题

输出：
- `coverage_audit`
- `full_response_audit`
- `missing_requirement_ids`
- `weak_requirement_ids`

#### 5）Conflict Auditor Agent（冲突审计代理）
职责：
- 检查目录中的重复章节、语义冲突、层级错位
- 检查同一要求是否被多个章节稀释
- 检查是否存在高风险编排问题

输出：
- `conflict_audit`
- `duplicate_node_hints`
- `conflict_list`
- `risk_summary`

#### 6）Coordinator（协调器）
职责：
- 负责编排整个 Step4 流程
- 控制串并行顺序
- 汇总各 Agent 结果
- 生成结构修正方案 `StructurePlan`
- 决策当前结果是 `PASS / REVISE / BLOCK`
- 推动人工审批

输出：
- 最终推荐目录版本
- 结构调整建议
- Step4 Gate 结果
- 审批状态

---

## 五、推荐的执行流程

### 阶段 A：要求真相层建立
1. Coordinator 拉起 Requirement Agent
2. 解析招标文件、评分表、专用条款、技术规范
3. 生成统一 requirement 清单
4. 标注：
   - 优先级
   - 是否必须显式响应
   - 所属领域
   - 预期响应层级
   - 风险等级

### 阶段 B：企业事实检索与映射
1. Coordinator 拉起 Fact Agent
2. 从企业知识库检索事实
3. 生成 requirement → fact 映射
4. 记录每条映射的原因、来源、位置

### 阶段 C：候选目录树生成
1. Coordinator 拉起 Outline Planner Agent
2. 输入 requirement 清单和 fact 映射
3. 输出 1~3 个候选目录版本
4. 为每个目录节点附带：
   - 响应哪些 requirement
   - 支撑 facts 是什么
   - 为什么放在当前层级

### 阶段 D：审计与 Gate
1. 拉起 Coverage Auditor Agent
2. 拉起 Conflict Auditor Agent
3. 分别生成：
   - 覆盖率结果
   - 完全响应率结果
   - 冲突清单
   - 高风险漏项
4. Coordinator 根据审计结果判断：
   - `PASS`：进入人工审批
   - `REVISE`：自动生成结构修正方案
   - `BLOCK`：阻止进入下一步

### 阶段 E：结构修正与人工审批
1. Coordinator 生成 `StructurePlan`
2. 前端展示调整动作：
   - keep
   - move
   - split
   - merge
   - promote
   - insert
3. 人工可：
   - 通过
   - 驳回
   - 填原因
4. 通过后，固化目录版本，供正文生成使用

---

## 六、为什么这样做有价值

### 1. 目录质量更稳
将目录生成拆成多个可控环节后，不再依赖单轮模型灵感，稳定性更高。

### 2. 可解释性更强
人工可以看到：
- 该目录来自哪些招标要求
- 哪些企业事实支撑该目录
- 为什么放在这个章节层级
- 哪些条款尚未被充分响应

### 3. 更容易发现漏项和废标风险
典型问题可被更早识别：
- 必须显式响应条款未覆盖
- 高优先级要求仅弱响应
- 重复目录稀释重点
- 标题好看但不对应评分点

### 4. 为正文生成铺路
目录一旦结构化，后续章节生成就能精准知道：
- 本章响应什么
- 本章可用哪些事实
- 本章禁止偏离哪些边界
- 本章应采用什么强响应措辞

### 5. 更利于后续持续优化
后续可以分别优化：
- requirement 抽取策略
- fact 检索策略
- 目录规划策略
- 审计规则
- 人工审批界面

不会牵一发而动全身。

---

## 七、与当前项目代码的对应关系

### 7.1 已有可复用部分

从当前代码结构判断，以下部分可直接复用或扩展：

#### 前端
- `frontend/src/api/techBidStep4.ts`
- `frontend/src/pages/TechBidProjectWorkbench.tsx`
- `frontend/src/components/Step4MappingCoveragePanel.tsx`
- `frontend/src/components/StructurePlanReviewPanel.tsx`

#### 后端
- `backend_go/internal/handler/tech_bid_project.go`
- `backend_go/internal/handler/bid_project.go`
- `backend_go/internal/service/outline_elastic_engine.go`
- `backend_go/internal/service/outline_strong_mapping.go`
- `backend_go/internal/service/requirement_full_response.go`
- `backend_go/internal/service/tender_conflict_audit.go`
- `backend_go/internal/service/industry_skeleton.go`
- `backend_go/internal/service/full_response_gate.go`

### 7.2 当前已有的正确方向
当前项目已经体现出以下正确方向，应保留并强化：

- requirement 真相层
- fact mapping
- coverage/full-response 审计
- conflict audit
- structure plan
- gate override
- 审批流程

也就是说，不需要推倒重来，而是应当：

**在现有 Step4 基础上，补齐 Coordinator 编排能力和 Agent 角色边界。**

---

## 八、建议的数据模型扩展

当前结构可继续标准化，建议新增或强化以下表 / 实体概念。

### 8.1 requirement_rows
字段建议：
- id
- project_id
- requirement_id
- requirement_type
- source_text
- source_location
- priority
- must_be_explicit
- expected_response_level
- domain
- response_tier
- summary
- risk_level
- created_at

### 8.2 fact_candidates
字段建议：
- id
- project_id
- fact_id
- fact_type
- fact_name
- source_library
- source_location
- confidence_score
- snippet
- created_at

### 8.3 fact_mappings
字段建议：
- id
- project_id
- requirement_id
- fact_id
- target_level
- target_path
- mapping_reason
- mapping_source
- support_strength
- created_at

### 8.4 outline_versions
字段建议：
- id
- project_id
- version_no
- version_source（agent/manual/revised）
- status（draft/recommended/approved/rejected）
- rationale
- created_by
- created_at

### 8.5 outline_nodes
字段建议：
- id
- outline_version_id
- parent_id
- node_name
- node_level
- node_order
- response_goal
- linked_requirement_ids
- linked_fact_ids
- created_at

### 8.6 coverage_audits
字段建议：
- id
- project_id
- outline_version
- fact_total
- fact_mapped
- coverage_rate
- missing_fact_ids
- weak_fact_ids
- result
- summary
- created_at

### 8.7 full_response_audits
字段建议：
- id
- project_id
- outline_version
- requirement_total
- requirement_fully_responded
- requirement_weakly_responded
- requirement_only_tagged
- full_response_rate
- weak_response_rate
- hard_rule_warnings
- result
- summary
- created_at

### 8.8 conflict_audits
字段建议：
- id
- project_id
- outline_version
- has_block
- conflicts_json
- summary
- created_at

### 8.9 structure_plans
字段建议：
- id
- project_id
- outline_version
- adjustments_json
- rationale
- status
- approved_by
- approved_at
- created_at

### 8.10 approval_logs
字段建议：
- id
- project_id
- stage
- action
- operator_id
- reason
- snapshot_version
- created_at

---

## 九、推荐的状态机设计

建议将 Step4 状态拆得更清晰。

### 9.1 Step4 主状态
- `idle`
- `requirements_extracting`
- `facts_mapping`
- `outline_planning`
- `coverage_auditing`
- `conflict_auditing`
- `revising`
- `waiting_for_approval`
- `approved`
- `blocked`
- `failed`

### 9.2 Gate 结果
- `PASS`
- `REVISE`
- `BLOCK`

### 9.3 人工审批结果
- `pending`
- `approved`
- `rejected`
- `overridden`

这样前端工作台展示会更清晰，日志与排错也更容易。

---

## 十、接口设计建议

以下为建议新增 / 统一的接口方向。

### 10.1 编排入口
- `POST /api/tech-bid/projects/:id/outline/run`
  - 启动 Step4 协调流程

### 10.2 requirement 结果
- `GET /api/tech-bid/projects/:id/outline/requirements`

### 10.3 fact mapping 结果
- `GET /api/tech-bid/projects/:id/outline/mappings`

### 10.4 候选目录版本
- `GET /api/tech-bid/projects/:id/outline/versions`
- `GET /api/tech-bid/projects/:id/outline/versions/:versionId`
- `POST /api/tech-bid/projects/:id/outline/versions/:versionId/select`

### 10.5 coverage/full-response 审计
- `GET /api/tech-bid/projects/:id/outline/coverage`
- `GET /api/tech-bid/projects/:id/outline/full-response`

### 10.6 conflict 审计
- `GET /api/tech-bid/projects/:id/outline/conflict-audit`

### 10.7 结构修正方案
- `GET /api/tech-bid/projects/:id/structure-plan`
- `POST /api/tech-bid/projects/:id/structure-plan/approve`
- `POST /api/tech-bid/projects/:id/structure-plan/reject`

### 10.8 Gate Override
- `POST /api/tech-bid/projects/:id/outline/step4-gate/override`

### 10.9 Agent 执行日志（建议新增）
- `GET /api/tech-bid/projects/:id/outline/agent-runs`
- `GET /api/tech-bid/projects/:id/outline/agent-runs/:runId`

这个日志接口非常重要，可用于问题追踪和人工复核。

---

## 十一、前端工作台改造建议

当前 `TechBidProjectWorkbench.tsx` 已具备 Step4 展示能力，建议继续增强为“多 Agent 工作台”。

### 11.1 新增板块

#### 1）Agent 执行时间线
展示：
- Requirement Agent
- Fact Agent
- Outline Planner Agent
- Coverage Auditor Agent
- Conflict Auditor Agent
- Coordinator

每个节点显示：
- 执行状态
- 耗时
- 输入摘要
- 输出摘要
- 失败原因

#### 2）候选目录版本对比
支持对比多个候选目录版本：
- 目录树结构差异
- coverage 差异
- full-response 差异
- conflict 数量差异

#### 3）可解释性面板
点击某目录节点时显示：
- 来源 requirement
- 支撑 facts
- 规划原因
- 风险提示

#### 4）高风险缺失项面板
突出显示：
- mandatory missing
- high priority missing
- only tagged but not responded
- hard rule warnings

### 11.2 交互策略建议
- 不要只展示结果，要展示“过程证据”
- 审批前必须能看见审计摘要
- override 时必须填写理由
- 支持刷新单个审计模块，而不是整个 Step4 全重跑

---

## 十二、上下文与权限边界设计

这是多 Agent 架构能否稳定的关键。

### 12.1 每个 Agent 的最小上下文

#### Requirement Agent 只读
- 招标文件
- 评分标准
- 专用条款
- 技术规范

不应读取：
- 全部企业事实库
- 历史目录版本
- 正文内容

#### Fact Agent 只读
- requirement 清单
- 企业知识库
- 结构化事实库

不应直接修改目录。

#### Outline Planner Agent 只读
- requirement 清单
- fact mappings
- 行业模板或 skeleton 规则

可以生成候选目录，但不负责最终放行。

#### Auditor Agents 只读
- 候选目录
- requirement / fact mapping
- 风险规则

不能直接改目录，只能提出审计意见。

#### Coordinator
- 有权读取全部中间工件
- 有权下达下一阶段任务
- 有权生成修正方案
- 无权绕过人工审批直接强制最终通过（除系统明确允许 override）

---

## 十三、实施路线图

建议分三期推进。

### 第一期：补齐专用编排层（低风险、最高收益）
目标：
- 不改动整体产品架构
- 在现有 Step4 上引入 Coordinator
- 统一各审计模块的输入输出

重点动作：
1. 梳理现有 service 的调用链
2. 增加 Step4 coordinator service
3. 统一 requirement / mapping / audit / structure plan 数据结构
4. 前端新增 Agent 时间线和结果面板

### 第二期：引入候选版本机制 + 对比机制
目标：
- 支持 2~3 个候选目录版本
- 支持版本比较与人工选优

重点动作：
1. 增加 outline_versions / outline_nodes
2. 增加对比 UI
3. 增加“选择推荐版本”能力

### 第三期：进一步强化局部上下文与策略优化
目标：
- 提升质量与成本效率
- 减少 token 浪费
- 增强不同类型项目的适配能力

重点动作：
1. 引入领域规则包（按工程类型）
2. Agent 局部上下文切片
3. 增加运行日志与质量回溯
4. 让正文生成直接消费目录结构化结果

---

## 十四、注意事项与边界

### 1. 不要做成通用 Agent 平台
当前最重要的是服务业务，不是造大而全平台。

### 2. 不要一开始就塞太多 Agent
建议先从 4~6 个角色做最小闭环。

### 3. 生成和审计必须分离
否则会出现“自己生成、自己打高分”的假闭环。

### 4. 一定要保留人工审批
投标业务天然高风险，人工审批不能省。

### 5. 审计结果要能追溯
后续若客户、业务、运营质疑某个目录，必须能快速回溯其依据。

---

## 十五、对当前项目的最终建议

### 建议结论
**建议你们明确启动 Step4 多 Agent 协作重构。**

但方式应为：

**以现有 Step4 为基础，做“专用编排增强”，而不是重写整套系统。**

### 最优策略
1. 保留现有 Step4 数据结构与页面框架
2. 在后端新增 Coordinator 编排层
3. 明确 Requirement / Fact / Planner / Auditor / Coordinator 五类职责边界
4. 强化版本化、审计化、可解释化
5. 把 Step4 结果作为 Step5 正文生成的标准输入

---

## 十六、一句话总结

你们的技术标第 4 步，已经具备走向“工业级多 Agent 协作目录编制系统”的基础。

下一步最值得做的，不是简单继续调 prompt，而是把它升级为：

**有协调器、有角色分工、有中间工件、有审计闭环、有人工审批的专用多 Agent 编排系统。**

这样做的好处是：
- 目录更稳
- 漏项更少
- 风险更低
- 可解释性更强
- 更方便人工审核
- 更利于后续正文自动生成

---

## 十七、建议下一步落地动作

建议按以下顺序推进：

1. 先梳理当前 Step4 后端 service 调用链
2. 识别可直接复用的 requirement / mapping / audit 服务
3. 设计 Step4 Coordinator service
4. 增加 outline version 数据模型
5. 前端增加 Agent 执行时间线 + 版本对比 + 解释面板
6. 完成一期闭环后，再优化策略与性能

如果需要，下一份文档可以继续输出：

**《Step4 Coordinator 详细技术设计（接口 + 表结构 + 状态机 + 服务编排图）》**
