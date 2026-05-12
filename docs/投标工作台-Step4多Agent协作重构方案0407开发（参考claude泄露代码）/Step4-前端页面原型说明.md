# Step4 前端页面原型说明

项目：投标工作台 / 技术标第 4 步多 Agent 协作

本文描述 Step4 前端页面的推荐布局与交互逻辑，便于前端工程师快速落地。

项目路径：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

---

## 1. 页面定位

页面承载的是“目录生成 + 多 Agent 证据展示 + 人工审批”。

因此页面目标不是只显示最终目录，而是同时展示：

1. 当前目录结果
2. 目录生成过程
3. 证据与审计结果
4. 结构调整建议
5. 人工审批入口

---

## 2. 页面整体布局建议

建议页面拆成 5 个主区域：

### A. 顶部状态栏
显示：
- 当前步骤：目录生成（Step4）
- 当前运行状态
- Gate 结果
- 当前版本号
- 最近一次运行时间
- 运行按钮 / 重跑按钮

### B. 左侧：Agent 执行时间线
显示整个 Step4 执行流程：
- Requirement Agent
- Fact Agent
- Outline Planner Agent
- Coverage Auditor Agent
- Conflict Auditor Agent
- Coordinator

每个节点显示：
- 状态（未开始 / 运行中 / 完成 / 失败）
- 耗时
- 输出摘要

### C. 中间：目录树主视图
核心区域，展示当前候选目录版本。

支持：
- 树形展开/折叠
- 版本切换
- 节点高亮
- 节点风险标记
- 节点证据联动

### D. 右侧：证据与审计面板
根据选中的目录节点，展示：
- 关联 requirement
- 支撑 fact
- coverage 提示
- conflict 提示
- 规划理由

### E. 底部：结构方案与人工审批
展示：
- 结构调整建议
- 高风险缺失项
- approve / reject / override

---

## 3. 具体模块说明

## 3.1 顶部状态栏

### 推荐字段
- 项目名称
- Step4 状态
- 当前 Agent
- Gate 结果
- 目录版本号
- 当前覆盖率
- 当前完全响应率

### 推荐按钮
- `启动生成`
- `重新生成`
- `重跑审计`
- `导出结果`

### 视觉建议
- PASS 用绿色
- REVISE 用橙色
- BLOCK 用红色
- running 用蓝色

---

## 3.2 Agent 执行时间线

### 展示形式
建议采用垂直 Timeline。

### 每个时间线节点展示
- Agent 名称
- 阶段名称
- 状态 icon
- 起止时间
- 耗时
- 输出摘要

### 示例
- Requirement Agent：抽取 62 条 requirement
- Fact Agent：匹配 48 条事实，建立 44 条映射
- Outline Planner：生成 3 个候选目录版本
- Coverage Auditor：coverage_rate = 91.67%
- Conflict Auditor：发现 1 个重复节点
- Coordinator：结果 REVISE，已生成结构修正方案

---

## 3.3 目录树主视图

这是页面的核心。

### 功能要求
- 显示目录层级
- 展示节点顺序
- 节点支持展开/折叠
- 支持切换不同 outline version
- 点击节点后联动右侧证据面板

### 每个节点建议展示的信息
- 节点标题
- 节点层级
- 风险标签
- requirement 数量
- fact 支撑数量
- 是否存在弱响应

### 节点标签建议
- `强响应`
- `弱响应`
- `缺少支撑`
- `重复风险`
- `人工修正`

---

## 3.4 版本切换区

建议在目录树上方增加版本切换条。

### 展示内容
- Version 1 / 2 / 3
- 每个版本的：
  - coverage_rate
  - full_response_rate
  - conflict_count
  - 推荐状态

### 支持操作
- 查看版本详情
- 设为当前版本
- 对比两个版本

---

## 3.5 右侧证据面板

点击某个目录节点时，这里展示它的依据。

### 分 4 个 Tab

#### Tab 1：Requirement
显示：
- requirement 列表
- 来源位置
- 优先级
- 是否必须显式响应

#### Tab 2：Fact
显示：
- 支撑事实
- 来源知识库
- 事实片段
- 支撑强度

#### Tab 3：Audit
显示：
- coverage 状态
- weak / missing 提示
- 冲突提示
- hard rule warning

#### Tab 4：Rationale
显示：
- 为什么生成这个节点
- 为什么放在该层级
- 为什么需要拆分 / 合并 / 插入

---

## 3.6 高风险缺失项面板

这是很重要的业务模块，建议固定展示。

### 重点显示
- mandatory missing ids
- high priority missing ids
- hard rule warnings
- only tagged requirement ids

### 视觉建议
- 红色：必须立即修正
- 橙色：建议修正
- 灰色：提示信息

---

## 3.7 结构方案审批面板

### 内容
- 当前 structure plan 状态
- 调整动作列表
- 调整原因
- 风险等级
- 审批意见输入框

### 动作类型展示
- keep
- move
- split
- merge
- promote
- insert

### 操作按钮
- `批准方案`
- `驳回方案`
- `人工覆盖通过`

### 驳回时必须填写
- 驳回原因
- 期望的修正方向

### override 时必须填写
- 人工覆盖原因

---

## 4. 页面交互流程

### 场景 1：正常生成
1. 用户点击“启动生成”
2. 顶部状态切换为 running
3. 左侧时间线逐步推进
4. 中间目录树出现候选版本
5. 右侧出现审计证据
6. 最终展示 PASS / REVISE / BLOCK

### 场景 2：结果为 REVISE
1. 页面突出显示缺失项与弱响应项
2. 结构方案面板展示修正动作
3. 用户查看后点击 approve / reject

### 场景 3：结果为 BLOCK
1. 页面红色提示阻断原因
2. 隐藏“直接下一步”入口
3. 只允许重跑 / 修正 / 人工 override

---

## 5. 推荐组件拆分

建议将页面拆成以下组件：

- `Step4HeaderBar`
- `Step4AgentTimeline`
- `Step4OutlineVersionSwitcher`
- `Step4OutlineTree`
- `Step4EvidencePanel`
- `Step4RiskPanel`
- `Step4StructurePlanPanel`
- `Step4ApprovalActions`

这样更利于维护和多人并行开发。

---

## 6. 推荐前端状态管理

建议维护以下状态：

- `runStatus`
- `currentVersion`
- `outlineVersions`
- `selectedNode`
- `requirements`
- `mappings`
- `coverage`
- `fullResponse`
- `conflictAudit`
- `structurePlan`
- `agentRuns`

---

## 7. 推荐接口绑定关系

- `run-status` → 顶部状态栏 + 时间线
- `requirements` → Requirement Tab
- `mappings` → Fact Tab
- `coverage` / `full-response` → Audit Tab
- `conflict-audit` → Audit Tab + 风险面板
- `structure-plan` → 审批面板
- `agent-runs` → 时间线详情

---

## 8. 一页版原型思路

```text
┌──────────────── 顶部状态栏 ────────────────┐
│ 项目名 | Step4状态 | 当前Agent | Gate结果 | 按钮 │
└──────────────────────────────────────────┘

┌────────────┬──────────────────────┬──────────────────────┐
│ Agent时间线 │     目录树主视图       │     证据/审计面板      │
│            │  版本切换 + 树结构      │ Requirement / Fact    │
│            │                        │ Audit / Rationale     │
└────────────┴──────────────────────┴──────────────────────┘

┌──────────────── 底部结构方案与审批区 ────────────────┐
│ 缺失项 | 风险项 | 结构修正动作 | approve/reject/override │
└───────────────────────────────────────────────────┘
```

---

## 9. 结论

前端页面的重点不是“把目录显示出来”，而是：

**把目录生成过程、证据链、审计结果、人工决策完整地呈现出来。**

只有这样，Step4 才不是黑箱，业务方才敢真正使用。 
