# Step4 服务调用时序图

本文描述投标工作台技术标第 4 步“目录生成”在多 Agent 架构下的服务调用顺序。

项目代码位置：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

---

## 1. 总体目标

目标是在一次 Step4 执行中，按顺序完成：

1. 招标要求抽取
2. 企业事实检索
3. 候选目录生成
4. 覆盖率审计
5. 冲突审计
6. 结构修正建议生成
7. 人工审批

---

## 2. 服务调用总览

```text
前端工作台
   ↓
Step4 Coordinator Service
   ├── Requirement Extraction Service
   ├── Fact Retrieval Service
   ├── Outline Planning Service
   ├── Coverage Audit Service
   ├── Conflict Audit Service
   └── Structure Plan Service
   ↓
前端审批面板
   ↓
人工确认
```

---

## 3. 标准执行时序

### 3.1 触发阶段

```text
用户点击“生成目录”
  ↓
前端调用 /outline/run
  ↓
后端创建 step4_run 记录
  ↓
Coordinator 初始化运行上下文
```

### 3.2 Requirement 抽取阶段

```text
Coordinator
  ↓
Requirement Extraction Service
  ↓
读取招标文件 / 评分标准 / 技术要求
  ↓
生成 requirement_rows
  ↓
写入 step4_requirements
```

输出内容：
- 必须显式响应条款
- 高优先级条款
- 风险条款
- 期望响应层级
- requirement summary

### 3.3 Fact 检索阶段

```text
Coordinator
  ↓
Fact Retrieval Service
  ↓
读取 requirement_rows
  ↓
查询企业事实库 / 知识库
  ↓
生成 fact_candidates
  ↓
生成 fact_mappings
  ↓
写入 step4_fact_candidates / step4_fact_mappings
```

输出内容：
- 事实名称
- 事实类型
- 来源位置
- 支撑强度
- requirement 到 fact 的映射原因

### 3.4 目录规划阶段

```text
Coordinator
  ↓
Outline Planning Service
  ↓
读取 requirement_rows + fact_mappings
  ↓
生成候选目录版本
  ↓
生成目录树节点
  ↓
写入 step4_outline_versions / step4_outline_nodes
```

输出内容：
- 目录版本号
- 每个节点的层级与顺序
- 节点说明
- 节点响应目标
- 关联 requirement / fact

### 3.5 覆盖率审计阶段

```text
Coordinator
  ↓
Coverage Audit Service
  ↓
读取 requirement_rows + outline_nodes
  ↓
计算 coverage_rate
  ↓
计算 full_response_rate
  ↓
生成 missing / weak / only-tagged 列表
  ↓
写入 step4_coverage_audits / step4_full_response_audits
```

输出内容：
- 覆盖率
- 完全响应率
- 缺失项
- 弱响应项
- 仅挂标签项
- 高优先级缺失项
- 必须显式响应缺失项

### 3.6 冲突审计阶段

```text
Coordinator
  ↓
Conflict Audit Service
  ↓
读取 outline_nodes + fact_mappings
  ↓
检查重复节点
  ↓
检查层级冲突
  ↓
检查语义冲突
  ↓
生成冲突清单
  ↓
写入 step4_conflict_audits
```

输出内容：
- 重复节点提示
- 冲突类型
- 风险说明
- 是否 BLOCK

### 3.7 裁决阶段

```text
Coordinator
  ↓
汇总 coverage + full response + conflict 结果
  ↓
判断 PASS / REVISE / BLOCK
  ↓
若 REVISE → 生成 structure_plan
  ↓
若 BLOCK → 记录阻断原因
  ↓
写入 step4_runs.status / gate_result
```

### 3.8 人工审批阶段

```text
前端展示 structure_plan
  ↓
人工查看目录、审计结果、风险提示
  ↓
人工点击 approve / reject / override
  ↓
写入 step4_approval_logs
  ↓
更新 structure_plan.status
  ↓
更新 step4_runs.status
```

---

## 4. 详细时序图

```text
User
  ↓
Frontend Workbench
  ↓ POST /outline/run
Backend API
  ↓
Step4 Coordinator
  ↓
Requirement Agent
  ↓ write requirements
Step4 Coordinator
  ↓
Fact Agent
  ↓ write fact candidates / mappings
Step4 Coordinator
  ↓
Outline Planner Agent
  ↓ write outline versions / nodes
Step4 Coordinator
  ↓
Coverage Auditor Agent
  ↓ write coverage audits / full response audits
Step4 Coordinator
  ↓
Conflict Auditor Agent
  ↓ write conflict audits
Step4 Coordinator
  ↓
Decision Engine
  ↓
- PASS → waiting_for_approval
- REVISE → structure_plan
- BLOCK → blocked
  ↓
Frontend Workbench
  ↓
Human Review
  ↓
approve / reject / override
  ↓
Backend API
  ↓ write approval log
Finalize Step4
```

---

## 5. 各阶段的输入输出边界

### 5.1 Requirement Agent
输入：
- 招标文件
- 评分标准
- 技术要求

输出：
- requirement_rows

不做：
- 目录生成
- 审计判断

### 5.2 Fact Agent
输入：
- requirement_rows
- 企业知识库

输出：
- fact_candidates
- fact_mappings

不做：
- 目录改写
- 审批决策

### 5.3 Outline Planner Agent
输入：
- requirement_rows
- fact_mappings
- 目录规则

输出：
- outline_versions
- outline_nodes

不做：
- 覆盖率裁决
- 冲突最终判定

### 5.4 Coverage Auditor Agent
输入：
- requirement_rows
- outline_nodes

输出：
- coverage_audit
- full_response_audit

不做：
- 结构修正
- 人工审批替代

### 5.5 Conflict Auditor Agent
输入：
- outline_nodes
- fact_mappings

输出：
- conflict_audit

不做：
- 目录修改
- 直接通过

### 5.6 Coordinator
输入：
- 所有阶段输出

输出：
- gate_result
- structure_plan
- run status

---

## 6. 推荐的错误处理策略

### 6.1 Requirement 抽取失败
处理：
- 标记 run 为 failed
- 记录错误摘要
- 前端提示“招标要求抽取失败，请检查输入文件”

### 6.2 Fact 检索无结果
处理：
- 允许继续生成目录，但标记弱支撑
- 提示人工补充事实库

### 6.3 Outline 规划失败
处理：
- 回退到上一个可用版本
- 若无版本则 BLOCK

### 6.4 覆盖率不足
处理：
- 自动进入 REVISE
- 生成修正建议

### 6.5 冲突过多
处理：
- 进入 BLOCK 或 REVISE
- 视冲突严重程度决定是否允许人工 override

### 6.6 审批超时
处理：
- 保留 waiting_for_approval 状态
- 支持后续继续审批

---

## 7. 推荐的日志字段

每次运行建议记录：
- run_id
- project_id
- operator_id
- started_at
- finished_at
- current_stage
- current_agent
- gate_result
- error_message
- retry_count
- outline_version

这些字段可帮助后续排错和质量复盘。

---

## 8. 适合前端展示的时序信息

前端可按下面方式展示：

- 当前阶段
- 当前 Agent
- 已完成阶段数 / 总阶段数
- 每阶段耗时
- 阶段结果摘要
- 是否可继续
- 是否需要人工审批

---

## 9. 最佳实践

### 9.1 支持单阶段重跑
不要每次都重跑整条链路。

### 9.2 支持结果快照
每个阶段输出都要有快照版本，方便比较。

### 9.3 支持人工插入修正
人工可以改 structure plan，但修改必须记录原因。

### 9.4 支持只读查看历史运行
历史 run 应可追踪、对比和回看。

---

## 10. 结论

这套时序图的核心目的，是把 Step4 从“一个目录生成接口”升级为“可追踪、可审计、可回滚的多 Agent 编排流程”。

它不是为了复杂而复杂，而是为了让投标目录生成更稳定、更可解释、更适合人工审核。
