# 第四步「目录生成」前后端流程改造方案

> 适用范围：仅优化技术标生成流程中的**第四步：目录生成**。  
> 不改变 Step1-3 的画像/事实抽取逻辑，不改变后续正文填充的总体业务边界。

---

## 1. 背景与目标

### 1.1 当前问题
现有第四步在直生模式下，由 `tech_bid_outline_direct_generation` 一次性生成完整三级目录。虽然效率高，但存在以下问题：

- 一级章、二级节、三级小节一把生成，结构容易跑偏
- 用户无法在“主骨架”阶段及时干预
- 目录生成后再返工，成本高
- 容易出现标题重复、商务内容污染技术标、层级串乱等问题

### 1.2 改造目标
将第四步目录生成拆成**两段式流程**：

1. **先生成一级章目录**
2. **用户确认 / 手动修改一级章**
3. **在确认后的一级章基础上，再生成二级节和三级小节**

这样可以达到：

- 先稳住结构，再补充血肉
- 让用户把控目录主框架
- 降低一次性生成三级目录的幻觉风险
- 更符合高质量技术标编制习惯

### 1.3 改造范围
本次只改第四步，具体包括：

- 前端目录生成交互
- 后端目录生成编排逻辑
- 新增两个 prompt key
- 目录生成状态流转
- 目录结果持久化与展示

---

## 2. 现状梳理

### 2.1 当前后端流程
从 `backend_go/internal/service/step4_coordinator.go` 看，当前直生模式会直接调用：

- `GenerateOutlineDirectly(...)` 生成完整目录
- 然后做目录编号清洗 `NormalizeOutlineNames(...)`
- 再做必出项校验、覆盖率审计、优化与回滚

也就是说，现在是：

`招标文件 + 事实 + 画像 + 路线 → 一次性生成完整三级目录`

### 2.2 当前前端展示
从 `frontend/src/components/OutlineGenerationPanel.tsx` 和 `frontend/src/pages/TechBidProjectWorkbench.tsx` 看，当前目录生成结果直接进入展示和后续编辑流程，用户介入点较晚。

---

## 3. 推荐改造方案

## 3.1 总体思路
把第四步拆成两个子阶段：

### 阶段 A：一级章生成
目标：只输出一级章，形成“目录主骨架”。

### 阶段 B：一级章展开
目标：在用户确认后的一级章基础上，补全每章下的二级节和三级小节。

---

## 4. 前端流程设计

## 4.1 第一步：生成一级章
用户点击“目录生成”后，先进入一级章生成流程。

前端行为：
- 显示“正在生成一级章”状态
- 生成完成后，仅展示一级章列表
- 每个一级章支持编辑、增删、调整顺序
- 用户确认后，才能进入下一步

### UI建议
在 `OutlineGenerationPanel` 中增加一个一级章确认卡片：

- 一级章列表
- 编辑按钮
- 删除按钮（可选）
- 上移 / 下移按钮（可选）
- “确认一级章，生成二级节和三级小节”主按钮

## 4.2 第二步：展开二级节和三级小节
当用户确认一级章后，前端触发第二次生成：

- 将“确认后的一级章”传给后端
- 后端按一级章逐章展开
- 前端展示每章对应的二级节 / 三级小节树形结构
- 用户可继续微调，但一级章保持锁定

## 4.3 推荐的页面状态
建议前端增加以下状态：

- `outline_chapter_generating`：一级章生成中
- `outline_chapter_ready_for_approval`：一级章待确认
- `outline_expanding`：二级节/三级小节生成中
- `outline_ready`：完整目录完成

---

## 5. 后端流程设计

## 5.1 新增两个 prompt key
建议将原 `tech_bid_outline_direct_generation` 拆为两个配置项：

### Prompt 1：一级章生成
- key：`tech_bid_outline_chapter_generation`
- 输入：招标文件全文 + 评分标准 + 行业骨架 + 技术/商务约束
- 输出：仅一级章 JSON 数组

### Prompt 2：基于一级章生成二级节和三级小节
- key：`tech_bid_outline_units_subsections_generation`
- 输入：招标文件全文 + 用户确认后的一级章 + 行业骨架 + 技术要求
- 输出：完整的章节树结构

## 5.2 后端状态机建议
建议在 Step4 中增加两个阶段性的编排状态：

1. `outline_chapter_generating`
2. `outline_chapter_pending_approval`
3. `outline_structure_expanding`
4. `outline_ready`

如果用户修改一级章后再确认，则重新进入 `outline_structure_expanding`。

## 5.3 数据持久化建议
建议保留“一级章草稿”和“确认后的一级章”两个概念。

可选实现方式：

### 方案 A：复用现有表，增加 JSON 字段
- 在目录版本或章节计划表中保存一级章草稿
- 用户确认后，写入 confirmed_chapters_json

### 方案 B：新增确认表
- `tech_bid_outline_chapter_drafts`
- `tech_bid_outline_chapter_confirmations`

### 推荐
优先采用**最小改造**：
- 先复用现有目录版本/章节计划表
- 增加一个 `confirmed_chapters_json` 字段或同等存储位

这样改动小、风险低、回滚方便。

---

## 6. 建议的接口改造

## 6.1 一级章生成接口
建议新增或拆分为：

- `POST /api/tech-bid/projects/:id/outline/chapter-only`

返回：
```json
{
  "success": true,
  "data": {
    "run_id": 123,
    "chapters": [
      { "name": "第一章 施工组织设计" }
    ]
  }
}
```

## 6.2 一级章确认接口
建议新增：

- `POST /api/tech-bid/projects/:id/outline/confirm-chapters`

请求体：
```json
{
  "chapters": [
    { "name": "第一章 施工组织设计" },
    { "name": "第二章 质量、安全与HSE管理" }
  ]
}
```

## 6.3 生成二级节和三级小节接口
建议新增：

- `POST /api/tech-bid/projects/:id/outline/expand-chapters`

请求体：
```json
{
  "confirmed_chapters": [
    { "name": "第一章 施工组织设计" },
    { "name": "第二章 质量、安全与HSE管理" }
  ]
}
```

返回：完整目录树 JSON。

---

## 7. 前端改造点

### 7.1 `OutlineGenerationPanel.tsx`
需要改造为两段式展示：

- 一级章结果区
- 一级章编辑区
- 一级章确认按钮
- 二级/三级生成结果区

### 7.2 `TechBidProjectWorkbench.tsx`
需要新增状态流转控制：

- 发起一级章生成
- 接收一级章结果
- 用户确认后发起展开请求
- 展示最终完整目录

### 7.3 `techBidStep4.ts`
建议新增对应 API 封装：

- `generateOutlineChapters(...)`
- `confirmOutlineChapters(...)`
- `expandOutlineChapters(...)`

---

## 8. 后端改造点

### 8.1 `step4_coordinator.go`
建议把原直生流程拆分为：

- `GenerateOutlineChapters(...)`
- `GenerateOutlineUnitsAndSubsections(...)`

编排逻辑从“一次性直出完整目录”改为“两阶段串联”。

### 8.2 `tech_bid_project.go`
建议增加或调整以下 handler：

- 一级章生成
- 一级章确认
- 结构展开

### 8.3 Prompt 配置读取
建议后台系统设置中增加两个 prompt 配置项：

- `tech_bid_outline_chapter_generation`
- `tech_bid_outline_units_subsections_generation`

原 `tech_bid_outline_direct_generation` 可以：
- 保留作为旧版本兼容
- 或标记为废弃

---

## 9. 推荐状态流转

建议第四步的状态流转如下：

1. `outline_chapter_generating`
2. `outline_chapter_pending_approval`
3. `outline_structure_expanding`
4. `outline_ready`

如果用户修改一级章：

- 重新进入 `outline_chapter_pending_approval`
- 或直接进入 `outline_structure_expanding` 的重算流程

---

## 10. 推荐的最小可行版本（MVP）

如果要尽量少改代码，建议先做这个版本：

### 前端
- 一级章先展示
- 用户点“确认一级章”
- 再触发第二次生成

### 后端
- 新增两个 prompt key
- 增加两个 API
- 保留当前目录审计、覆盖率审计、编号清洗逻辑

### 数据层
- 不大改表结构
- 先用现有 JSON 存储位临时承载确认结果

这样可以快速上线验证。

---

## 11. 风险与对策

### 风险 1：一级章确认后，二级节展开跑偏
**对策：** 二级展开 prompt 强制引用 confirmed_chapters，禁止新增一级章。

### 风险 2：编号重复或层级错乱
**对策：** 保留现有 `NormalizeOutlineNames(...)`，并增加二级/三级层级校验。

### 风险 3：商务内容污染技术标
**对策：** 在两个 prompt 中继续强化商务隔离约束；后端再做一次目录内容分类校验。

### 风险 4：前端状态过多导致复杂
**对策：** 用明确的阶段状态机管理，不要用散乱布尔值控制。

---

## 12. 验收标准

改造完成后，应满足：

1. 一级章可单独生成并可人工确认
2. 一级章确认后，才允许生成二级节和三级小节
3. 二级节和三级小节必须挂在确认后的一级章下
4. 不得新增未确认的一级章
5. 不得出现明显的编号重复、层级串乱
6. 技术标目录不应被商务内容污染
7. 整体流程只影响第四步，其他步骤保持不变

---

## 13. 结论

本次改造的核心，不是简单“换两个 prompt”，而是把第四步从：

> 一次性生成完整三级目录

升级为：

> **先定骨架，再填血肉**

这是更稳、更符合人工审核习惯、也更适合高质量技术标生成的流程。

---

## 14. 建议下一步

建议下一步按以下顺序实施：

1. 先改 prompt 配置
2. 再改后端编排接口
3. 再改前端交互流程
4. 最后补状态校验与目录清洗
