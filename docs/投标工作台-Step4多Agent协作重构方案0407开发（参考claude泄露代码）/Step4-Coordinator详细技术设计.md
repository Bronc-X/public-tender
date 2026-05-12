# Step4 Coordinator 详细技术设计

## 1. 目标

本文是《投标工作台 Step4 多 Agent 协作重构方案》的落地版，重点描述：

- Coordinator 如何编排 Step4 的多个专职 Agent
- 后端接口如何设计
- 数据表如何落库
- 状态机如何推进
- 前端如何展示执行过程
- 如何保证可审计、可追踪、可回滚

项目代码位置：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

---

## 2. 设计原则

### 2.1 业务优先，不造通用平台
Step4 的目标不是做通用 Agent 平台，而是把“目录生成”做稳、做准、做可审计。

### 2.2 生成与审计分离
任何“生成目录”的能力，都必须与“审计目录”的能力分离。

### 2.3 中间工件可落库
所有关键结果必须结构化落库，不能只停留在 prompt 返回文本里。

### 2.4 局部上下文最小化
每个 Agent 只读自己任务需要的信息，避免上下文污染。

### 2.5 人工审批不可省
对投标业务来说，人工审批是最终安全阀。

---

## 3. 总体服务拆分

建议在后端新增一个 Step4 编排服务，作为本阶段的统一入口。

### 3.1 主要服务模块

#### A. Step4 Coordinator Service
职责：
- 接收前端触发请求
- 创建本轮执行任务
- 按顺序调度各子 Agent
- 汇总结果并决策 PASS / REVISE / BLOCK
- 生成结构修正方案

#### B. Requirement Extraction Service
职责：
- 从招标文件中提取 requirement
- 标记必须显式响应项
- 提取评分点与风险条款

#### C. Fact Retrieval Service
职责：
- 从企业知识库 / 事实库中检索相关事实
- 为 requirement 提供支撑证据

#### D. Outline Planning Service
职责：
- 生成候选目录树
- 输出章节层级、节点顺序、章节目标

#### E. Coverage Audit Service
职责：
- 检查 requirement 覆盖情况
- 计算 coverage / full-response
- 标识遗漏项和弱响应项

#### F. Conflict Audit Service
职责：
- 检查重复、冲突、层级错位、空壳标题
- 输出风险清单

#### G. Structure Plan Service
职责：
- 将审计结果转成结构修正动作
- 为人工审批提供明确建议

---

## 4. Step4 执行流程

### 4.1 触发方式

前端在用户点击“生成目录”或“重新编排”时，通过接口触发：

`POST /api/tech-bid/projects/:id/outline/run`

### 4.2 执行步骤

#### Step 1：创建执行上下文
Coordinator 创建一条运行记录：
- project_id
- outline_version
- trigger_source
- operator_id
- current_status = `requirements_extracting`

#### Step 2：Requirement Agent
- 输入：招标文件、评分点、项目基础信息
- 输出：`requirement_rows`
- 写库：requirements 表

#### Step 3：Fact Agent
- 输入：requirement_rows、企业知识库索引
- 输出：`fact_candidates`、`fact_mappings`
- 写库：fact_candidates / fact_mappings 表

#### Step 4：Outline Planner Agent
- 输入：requirement_rows + fact_mappings
- 输出：候选目录树
- 写库：outline_versions / outline_nodes 表

#### Step 5：Coverage Auditor Agent
- 输入：outline_nodes + requirement_rows
- 输出：coverage / full response 结果
- 写库：coverage_audits / full_response_audits 表

#### Step 6：Conflict Auditor Agent
- 输入：outline_nodes + mapping 关系
- 输出：冲突清单、重复提示、风险提示
- 写库：conflict_audits 表

#### Step 7：Coordinator 裁决
- 如果审计全部通过 → `PASS`
- 如果可修复 → `REVISE`
- 如果存在硬性风险 → `BLOCK`

#### Step 8：StructurePlan 生成
- 当结果为 REVISE 时，生成修正动作清单
- 写库：structure_plans 表

#### Step 9：人工审批
- 前端展示结果
- 人工通过 / 驳回 / override
- 写库：approval_logs 表

---

## 5. 建议的数据表设计

下面是建议的最小可用表结构。

### 5.1 step4_runs
记录每次 Step4 的完整执行。

字段建议：
- id
- project_id
- outline_version
- trigger_source
- operator_id
- status
- gate_result
- started_at
- finished_at
- error_message
- created_at

作用：
- 追踪一次完整编排的生命周期
- 支持重放和排错

---

### 5.2 step4_requirements
记录 requirement 真相层。

字段建议：
- id
- run_id
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
- risk_level
- summary
- created_at

---

### 5.3 step4_fact_candidates
记录检索到的事实候选。

字段建议：
- id
- run_id
- project_id
- fact_id
- fact_type
- fact_name
- source_library
- source_location
- confidence_score
- snippet
- created_at

---

### 5.4 step4_fact_mappings
记录 requirement 和 fact 的映射关系。

字段建议：
- id
- run_id
- project_id
- requirement_id
- fact_id
- target_level
- target_path
- mapping_reason
- mapping_source
- support_strength
- created_at

---

### 5.5 step4_outline_versions
记录候选目录版本。

字段建议：
- id
- run_id
- project_id
- version_no
- version_source
- status
- rationale
- created_by
- created_at

---

### 5.6 step4_outline_nodes
记录目录树节点。

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
- node_summary
- created_at

---

### 5.7 step4_coverage_audits
记录覆盖率结果。

字段建议：
- id
- run_id
- project_id
- outline_version
- requirement_total
- requirement_mapped
- coverage_rate
- missing_requirement_ids
- weak_requirement_ids
- result
- summary
- created_at

---

### 5.8 step4_full_response_audits
记录完全响应率。

字段建议：
- id
- run_id
- project_id
- outline_version
- requirement_total
- requirement_fully_responded
- requirement_weakly_responded
- requirement_only_tagged
- full_response_rate
- weak_response_rate
- response_quality_score
- missing_requirement_ids
- mandatory_missing_ids
- mandatory_insufficient_ids
- hard_rule_warnings
- result
- summary
- created_at

---

### 5.9 step4_conflict_audits
记录冲突审计。

字段建议：
- id
- run_id
- project_id
- has_block
- conflicts_json
- duplicate_node_hints
- summary
- created_at

---

### 5.10 step4_structure_plans
记录结构修正方案。

字段建议：
- id
- run_id
- project_id
- outline_version
- adjustments_json
- rationale
- status
- approved_by
- approved_at
- rejected_reason
- created_at

---

### 5.11 step4_approval_logs
记录审批行为。

字段建议：
- id
- run_id
- project_id
- stage
- action
- operator_id
- reason
- snapshot_version
- created_at

---

## 6. 状态机设计

### 6.1 主状态
建议采用以下状态：

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

### 6.2 状态流转

```text
idle
  ↓
requirements_extracting
  ↓
facts_mapping
  ↓
outline_planning
  ↓
coverage_auditing
  ↓
conflict_auditing
  ↓
Coordinator 决策
  ├── PASS → waiting_for_approval → approved
  ├── REVISE → revising → outline_planning
  └── BLOCK → blocked
```

### 6.3 Gate 结果
- `PASS`
- `REVISE`
- `BLOCK`

### 6.4 审批结果
- `pending`
- `approved`
- `rejected`
- `overridden`

---

## 7. Coordinator 详细职责

Coordinator 是整个 Step4 的中枢，不直接产出目录内容，而是调度和裁决。

### 7.1 输入
- 项目基础信息
- 招标文件
- 企业知识库索引
- 目录生成策略
- 历史运行记录

### 7.2 输出
- requirement_rows
- fact_mappings
- outline_versions
- coverage_audits
- conflict_audits
- structure_plans
- gate_result

### 7.3 决策规则

#### PASS 条件
- 必须显式响应项覆盖率达到阈值
- 无硬性冲突
- 无 BLOCK 级风险
- 关键评分点均有响应

#### REVISE 条件
- 存在可修复遗漏
- 存在弱响应项
- 存在可通过结构调整修正的冲突

#### BLOCK 条件
- 必须响应条款缺失
- 存在严重冲突或硬规则违规
- 出现废标风险或目录结构明显失真

---

## 8. Agent 输入输出协议

### 8.1 Requirement Agent

#### 输入
- 招标文件文本
- 评分标准
- 技术要求章节

#### 输出
```json
{
  "requirements": [
    {
      "requirement_id": "R001",
      "source_text": "...",
      "priority": "high",
      "must_be_explicit": true,
      "expected_response_level": "chapter",
      "domain": "construction_method",
      "risk_level": "high"
    }
  ]
}
```

### 8.2 Fact Agent

#### 输入
- requirement 列表
- 企业事实库

#### 输出
```json
{
  "fact_candidates": [
    {
      "fact_id": "F123",
      "fact_type": "project_experience",
      "confidence_score": 0.92,
      "snippet": "..."
    }
  ],
  "mappings": [
    {
      "requirement_id": "R001",
      "fact_id": "F123",
      "support_strength": "strong",
      "mapping_reason": "该业绩可直接支撑施工组织要求"
    }
  ]
}
```

### 8.3 Outline Planner Agent

#### 输入
- requirement_rows
- fact_mappings
- 行业 skeleton 规则

#### 输出
```json
{
  "versions": [
    {
      "version_no": 1,
      "rationale": "...",
      "nodes": [
        {
          "node_name": "施工组织总体部署",
          "node_level": 1,
          "linked_requirement_ids": ["R001", "R002"]
        }
      ]
    }
  ]
}
```

### 8.4 Coverage Auditor Agent

#### 输入
- requirement_rows
- outline_nodes

#### 输出
```json
{
  "coverage_rate": 0.92,
  "missing_requirement_ids": ["R009"],
  "weak_requirement_ids": ["R004", "R008"],
  "result": "REVISE"
}
```

### 8.5 Conflict Auditor Agent

#### 输入
- outline_nodes
- fact_mappings

#### 输出
```json
{
  "has_block": false,
  "conflicts": [
    {
      "conflict_type": "duplicate_node",
      "field_name": "node_name",
      "description": "两个节点表达重复"
    }
  ]
}
```

---

## 9. 前端改造建议

当前 `TechBidProjectWorkbench.tsx` 已经有 Step4 的入口，建议继续增强以下组件。

### 9.1 Agent 执行时间线
展示每个 Agent 的运行顺序和状态：
- Requirement Agent
- Fact Agent
- Outline Planner Agent
- Coverage Auditor Agent
- Conflict Auditor Agent
- Coordinator

每个节点展示：
- 状态
- 耗时
- 输入摘要
- 输出摘要
- 错误信息

### 9.2 候选目录版本对比
支持版本切换和差异查看：
- 目录节点差异
- coverage 差异
- full response 差异
- 冲突差异

### 9.3 可解释性面板
点击某个目录节点时展示：
- 该节点对应的 requirement
- 该节点支撑 fact
- 节点规划理由
- 风险提示

### 9.4 审批面板
展示：
- gate 结果
- 硬性风险
- 可修复建议
- override 输入框
- 审批按钮

### 9.5 交互建议
- 支持单独重跑某个 Agent
- 支持人工修改结构计划
- 支持先预览后提交审批
- 支持导出审计结果

---

## 10. 后端接口建议

### 10.1 启动编排
`POST /api/tech-bid/projects/:id/outline/run`

请求体建议：
```json
{
  "mode": "auto",
  "force_rebuild": false,
  "operator_id": "u123"
}
```

### 10.2 查看运行状态
`GET /api/tech-bid/projects/:id/outline/run-status`

返回：
- 当前状态
- 当前 Agent
- 最近错误
- 运行耗时

### 10.3 查看 requirement
`GET /api/tech-bid/projects/:id/outline/requirements`

### 10.4 查看映射
`GET /api/tech-bid/projects/:id/outline/mappings`

### 10.5 查看候选目录版本
`GET /api/tech-bid/projects/:id/outline/versions`

### 10.6 查看单个版本
`GET /api/tech-bid/projects/:id/outline/versions/:versionId`

### 10.7 选择推荐版本
`POST /api/tech-bid/projects/:id/outline/versions/:versionId/select`

### 10.8 查看 coverage
`GET /api/tech-bid/projects/:id/outline/coverage`

### 10.9 查看 full response
`GET /api/tech-bid/projects/:id/outline/full-response`

### 10.10 查看冲突审计
`GET /api/tech-bid/projects/:id/outline/conflict-audit`

### 10.11 查看结构方案
`GET /api/tech-bid/projects/:id/structure-plan`

### 10.12 审批结构方案
`POST /api/tech-bid/projects/:id/structure-plan/approve`

### 10.13 驳回结构方案
`POST /api/tech-bid/projects/:id/structure-plan/reject`

### 10.14 Gate Override
`POST /api/tech-bid/projects/:id/outline/step4-gate/override`

### 10.15 查看 Agent 运行日志
`GET /api/tech-bid/projects/:id/outline/agent-runs`
`GET /api/tech-bid/projects/:id/outline/agent-runs/:runId`

---

## 11. 推荐实现顺序

### 第 1 步：统一数据结构
先把 requirement / mapping / audit / structure plan 的结构标准化。

### 第 2 步：加 Coordinator 层
把 Step4 的各个逻辑入口统一由 Coordinator 调度。

### 第 3 步：补运行日志
没有运行日志，排错和审计会很痛苦。

### 第 4 步：增强前端工作台
让用户看见“过程”，而不是只看结果。

### 第 5 步：支持候选版本
版本化后，目录质量会更稳，也更适合比较。

### 第 6 步：增强局部上下文和规则包
不同工程类型可以逐步沉淀不同策略。

---

## 12. 风险控制

### 12.1 不要让单个 Agent 既生成又审核
否则审计失去意义。

### 12.2 不要把所有资料一次性塞给模型
会导致上下文污染和 token 浪费。

### 12.3 不要省略人工审批
投标场景中人工最终把关必须保留。

### 12.4 不要把接口做得太自由
建议接口尽量结构化，不给前端传过于自由的非结构化指令。

### 12.5 不要让规则分散在 UI 和后端两边
核心规则尽量收敛到后端，前端负责展示和交互。

---

## 13. 一页版结论

如果只保留一句话，那就是：

**Step4 应从“单次目录生成”演进为“Coordinator 编排的多 Agent 审计式目录编制系统”。**

这样做带来的直接收益是：
- 目录更稳
- 漏项更少
- 风险更低
- 解释性更强
- 更适合人工审批
- 更利于后续正文生成

---

## 14. 下一步建议

建议下一步继续补两份内容：

1. **Step4 服务调用时序图**
2. **Step4 数据库建表 SQL 草案**

如果你要，我可以继续直接补：

- `Step4 服务调用时序图.md`
- `Step4 数据库建表草案.sql`
