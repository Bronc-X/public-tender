# 第四步目录生成流程改造代码清单

> 目标：把技术标目录生成从“直接生成完整三级目录”改成“先生成一级章 → 用户确认 → 再生成二级节/三级小节”。

---

## 一、改造目标

### 当前问题
现有第四步目录生成链路在直生模式下，一次性输出完整三级目录，存在：
- 一级章无法提前确认
- 目录结构容易跑偏
- 商务内容容易污染技术标
- 标题重复、层级串乱风险高

### 改造目标
把第四步改成两段式：
1. 先生成一级章目录
2. 用户确认一级章
3. 基于确认后的一级章，再生成二级节和三级小节

---

## 二、前端改造清单

### 1. `frontend/src/components/OutlineGenerationPanel.tsx`
#### 需要新增的能力
- 展示“一级章生成结果”
- 支持一级章编辑
- 支持一级章确认按钮
- 确认后触发二级节/三级小节生成
- 增加两段式状态展示

#### 建议新增状态
- `outline_chapter_generating`
- `outline_chapter_pending_approval`
- `outline_structure_expanding`
- `outline_ready`

#### 建议新增交互
- 一级章列表可编辑
- 一级章顺序可调整
- 确认后锁定一级章
- 二级/三级生成完成后展示完整树

---

### 2. `frontend/src/pages/TechBidProjectWorkbench.tsx`
#### 需要改造的内容
- 接入新的“一级章生成”接口
- 接入新的“确认一级章”接口
- 接入新的“展开二级/三级”接口
- 维护第四步的分阶段状态流转

#### 建议控制流程
1. 用户点击“生成目录”
2. 前端先请求一级章
3. 显示一级章待确认
4. 用户确认或修改
5. 再请求完整目录展开
6. 展示最终目录结果

---

### 3. `frontend/src/api/techBidStep4.ts`
#### 需要新增 API 封装
- `generateOutlineChapters(projectId, companyId)`
- `confirmOutlineChapters(projectId, companyId, chapters)`
- `expandOutlineChapters(projectId, companyId, confirmedChapters)`

#### 说明
当前已有的目录相关 API 不要直接废弃，建议保留旧接口用于兼容，但前端新流程改用拆分后的接口。

---

## 三、后端改造清单

### 1. `backend_go/internal/service/step4_coordinator.go`
#### 需要拆分的逻辑
当前 `GenerateOutlineDirectly(...)` 一次性生成完整三级目录，应拆成两步：

- `GenerateOutlineChapters(...)`
- `GenerateOutlineUnitsAndSubsections(...)`

#### 建议编排逻辑
1. 读取招标文件、画像、事实、路线
2. 调用一级章 prompt 生成一级章
3. 保存一级章草稿
4. 等用户确认
5. 再调用二级/三级 prompt 展开
6. 做编号清洗、校验、覆盖率审计

#### 需要保留的现有能力
- `NormalizeOutlineNames(...)`
- 必出项校验
- 覆盖率审计
- full response 校验
- 再优化/回滚机制

---

### 2. `backend_go/internal/handler/tech_bid_project.go`
#### 需要新增或拆分的接口
建议新增：
- `POST /outline/chapter-only`
- `POST /outline/confirm-chapters`
- `POST /outline/expand-chapters`

#### 需要调整的现有逻辑
- `PostOutlineRegenerate` 不再直接走完整三级目录生成
- 先走一级章生成，再等待确认，再展开

#### 建议保留
- 当前 run/status/history/versions 等查询接口
- 目录审计和版本查看能力

---

### 3. `backend_go/internal/model/step4.go`
#### 建议补充字段/结构
如果现有模型不够表达两段式流程，建议补充：
- 一级章草稿 JSON
- 确认后的一级章 JSON
- 一级章确认状态
- 展开状态

#### 原则
优先最小改动，不要大拆表；如已有 JSON 字段可复用，先复用。

---

### 4. `backend_go/internal/service/step4_store.go`
#### 建议支持的持久化内容
- 一级章草稿保存
- 一级章确认结果保存
- 二级/三级展开结果保存
- 目录版本记录

---

## 四、Prompt 改造清单

### 1. 新增 `tech_bid_outline_chapter_generation`
用途：只生成一级章。

### 2. 新增 `tech_bid_outline_units_subsections_generation`
用途：基于确认后的一级章，生成二级节和三级小节。

### 3. 旧 prompt 兼容策略
原 `tech_bid_outline_direct_generation` 可以：
- 保留为旧版本兜底
- 或逐步废弃

建议先保留，等新链路稳定后再下线。

---

## 五、状态机改造清单

### 建议新增状态
- `outline_chapter_generating`
- `outline_chapter_pending_approval`
- `outline_structure_expanding`
- `outline_ready`

### 状态流转建议
1. 生成一级章中
2. 一级章待确认
3. 用户确认后开始展开
4. 完整目录生成完成

### 需要注意
- 一级章确认前，不能进入二级/三级生成
- 一级章确认后，不允许后续 prompt 擅自新增一级章

---

## 六、目录质量校验清单

### 1. 目录编号清洗
保留并加强：
- 重复编号去重
- 层级错乱修正
- 标题格式标准化

### 2. 商务污染检测
检查目录中是否出现以下不应混入技术标的内容：
- 报价策略
- 结算方式
- 审减率
- 计价软件
- 付款方式
- 开标唱标规则

如果招标文件明确要求商务响应，则只能进入单独商务章。

### 3. 必出项检查
一级章阶段就要对评分表必出项做校验；展开后再做完整目录校验。

---

## 七、推荐实施顺序

### 第一阶段：Prompt 拆分
- 新增两个 prompt key
- 先不动流程，仅把 prompt 能力拆开

### 第二阶段：后端编排
- 改造 step4 coordinator
- 增加一级章确认和展开逻辑

### 第三阶段：前端交互
- 修改目录生成页
- 做一级章确认 UI

### 第四阶段：质量加固
- 编号清洗
- 商务污染检测
- 必出项补章校验

---

## 八、验收标准

改造完成后，应满足：
1. 可以单独生成一级章
2. 一级章可被用户修改并确认
3. 确认后再展开二级节和三级小节
4. 不得擅自新增一级章
5. 目录编号不重复、不串层
6. 技术标不被商务内容污染
7. 其他步骤不受影响

---

## 九、备注

本次改造的核心不是“再优化一次 prompt”，而是把第四步变成一个**先定骨架、再填血肉**的可控流程。
