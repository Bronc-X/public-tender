# 给 Cursor 的执行任务清单：项目画像抽取链路改造（P0 / P1 / P2）

> 项目路径：`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`
> 
> 后端主代码：`backend_go/`
> 前端主代码：`frontend/`
> 
> 本任务目标：解决 `http://127.0.0.1:5173/tech-bid-projects/:id` 页面第二步“项目画像”抽取不全的问题，把当前“单次抽取”升级为“固定 schema + 证据链 + 补漏审计 + 字段级 merge + 前端复核”的工程化方案。

---

# 一、先读这些目录和文件（不要跳过）

## 后端优先排查文件
请先阅读并梳理以下文件的职责和调用链：

- `backend_go/internal/handler/tech_bid_project.go`
- `backend_go/internal/handler/knowledge_extract_handler.go`
- `backend_go/internal/service/knowledge_extract_service.go`
- `backend_go/internal/service/step4_domain.go`
- `backend_go/internal/service/step4_persist.go`
- `backend_go/internal/service/docmind_parse_service.go`
- `backend_go/internal/service/tender_digitization.go`
- `backend_go/internal/service/extraction_json_coerce.go`
- `backend_go/internal/model/step4.go`
- `backend_go/internal/handler/prompt_handler.go`
- `backend_go/internal/model/prompt.go`

## 前端优先排查文件
- `frontend/src/pages/TechBidProjectWorkbench.tsx`
- `frontend/src/api/techBidStep4.ts`
- `frontend/src/components/KnowledgeExtractWizard.tsx`
- `frontend/src/components/Step4MappingCoveragePanel.tsx`
- 与“项目画像”展示直接相关的 step4/画像组件

## 先输出一份分析结果
先不要急着大改。先输出一份简短分析，说明：
1. 当前“项目画像”抽取入口函数在哪
2. prompt 在哪里定义/拼装
3. 文档解析在哪里做
4. chunk 是在哪里切分的
5. merge 是在哪里做的
6. 最终前端读取的是哪个接口字段
7. 目前漏项最可能发生在哪一层

把这份分析写到：
`/Users/raoyi/Desktop/项目画像链路现状分析.md`

---

# 二、P0 任务（必须先完成）

## P0-1：建立最小可观测链路

### 目标
让我们能追溯“某个字段为什么没出来”。

### 要求
在后端抽取链路中增加中间结果留痕，至少保存：
- 原始解析文本
- chunk 列表
- 每个 chunk 的抽取结果
- merge 前结果
- merge 后结果
- 返回前端的最终 payload

### 实现建议
优先在这些位置加：
- `knowledge_extract_service.go`
- `step4_persist.go`
- `tender_digitization.go`
- 如有现成 step4 持久化结构，则扩展它；否则新增调试存储字段或 JSON 文件快照

### 输出要求
至少支持通过 project_id / file_id 查到一轮抽取的中间产物。

---

## P0-2：把项目画像输出改为固定 schema（最小版）

### 目标
禁止模型自由发挥字段，避免“这次有、下次没”。

### 最小 schema（第一版必须实现）
```json
{
  "project_base_info": {
    "project_name": {},
    "project_type": {},
    "project_location": {},
    "contract_scope_summary": {},
    "total_duration": {},
    "quality_target": {},
    "safety_target": {}
  },
  "construction_core_requirements": {
    "construction_scope": {},
    "material_equipment_rules": {},
    "procurement_boundary": {},
    "owner_supplied_items": [],
    "contractor_supplied_items": [],
    "quality_acceptance_rules": {},
    "safety_civilization_rules": {},
    "schedule_constraints": {}
  },
  "bidder_requirements": {
    "qualification_requirements": {},
    "personnel_requirements": {},
    "performance_requirements": {},
    "certificate_requirements": {}
  },
  "evaluation_and_performance_rules": {
    "scoring_items": [],
    "bonus_rules": [],
    "deduction_rules": [],
    "disqualification_rules": [],
    "mandatory_response_items": []
  },
  "extraction_gaps": [],
  "uncertain_items": [],
  "requires_manual_review": []
}
```

### 规则
- 所有字段必须输出
- 没命中也要输出空对象/空数组
- 不允许新增随意命名字段
- 关键字段不允许只放在长摘要里

### 实现建议
- 如果 prompt 模板在 `prompt_handler.go` / `prompt_service.go` / prompt 表中，请同步修改模板
- 如果解析结果还会经过 `extraction_json_coerce.go`，请同步加强 schema 纠偏逻辑
- 如果 `internal/model/step4.go` 已有画像结构，优先扩展而不是另起一套

---

## P0-3：所有关键字段加证据结构

### 标准结构
对象字段统一成：
```json
{
  "value": "",
  "source_text": [],
  "source_location": [],
  "confidence": 0,
  "missing": false,
  "notes": ""
}
```

数组项统一成：
```json
{
  "name": "",
  "value": "",
  "source_text": [],
  "source_location": [],
  "confidence": 0,
  "missing": false
}
```

### 要求
至少这几类字段必须带证据：
- procurement_boundary
- owner_supplied_items
- contractor_supplied_items
- total_duration / schedule_constraints
- qualification_requirements
- personnel_requirements
- scoring_items
- disqualification_rules

---

## P0-4：加入缺失/不确定/人工复核区域

### 必须实现
```json
{
  "extraction_gaps": [],
  "uncertain_items": [],
  "requires_manual_review": []
}
```

### 语义要求
- extraction_gaps：应该有但没找到
- uncertain_items：找到相关文本但无法稳定判断
- requires_manual_review：高风险、表格/附录疑似遗漏、或证据冲突

---

## P0-5：先做四类专项补漏（主抽取后第二轮）

### 只做以下四类，不要一次铺太大
1. 采购边界
2. 工期节点
3. 评分与废标
4. 资格与人员

### 要求
主抽取结束后，再补一轮专项 prompt：
- 不重新做全量画像
- 只检查这四类有没有漏
- 把补漏结果并入主结果

### 重点字段
- procurement_boundary
- owner_supplied_items
- contractor_supplied_items
- total_duration
- schedule_constraints
- scoring_items
- disqualification_rules
- mandatory_response_items
- qualification_requirements
- personnel_requirements
- performance_requirements

---

## P0-6：前端显示字段状态

### 在第二步“项目画像”页增加字段状态标签
至少支持：
- 已提取
- 缺失
- 低置信度
- 待人工确认
- 多候选冲突

### 前端建议排查位置
- `frontend/src/pages/TechBidProjectWorkbench.tsx`
- `frontend/src/components/KnowledgeExtractWizard.tsx`
- `frontend/src/components/Step4MappingCoveragePanel.tsx`
- `frontend/src/api/techBidStep4.ts`

### 要求
前端不是只展示 value，还要能展示：
- 是否 missing
- 是否低 confidence
- 是否进入 requires_manual_review
- 是否有 source_text/source_location

---

# 三、P1 任务（P0 完成后再做）

## P1-1：重写 merge 为字段级 merge

### 禁止继续使用的逻辑
- 后值直接覆盖前值
- 长字符串覆盖短字符串
- 只保留最后一个 chunk 的结果

### 新规则
- 对象字段：递归 merge
- 字符串字段：保留更完整、更具体版本；互补则拼接去重
- 数组字段：按语义去重；同名项合并证据
- 证据字段：append 合并，不允许覆盖
- 高风险字段：保留 `value + candidates + evidence_list`

### 重点排查实现位置
优先查：
- `knowledge_extract_service.go`
- `step4_domain.go`
- `step4_persist.go`
- `extraction_json_coerce.go`

### 输出要求
新增单元测试，验证：
- chunk A 提到甲供设备
- chunk B 提到采购责任
- merge 后两者都保留，不丢信息

---

## P1-2：把 chunk 分成正文 / 表格 / 附录 / 清单

### 目标
不要把所有 chunk 当同一种内容处理。

### 要求
在文档预处理或抽取前，为每个 chunk 打标签：
- narrative（正文）
- table（表格）
- appendix（附录）
- list（清单）

### 建议排查位置
- `docmind_parse_service.go`
- `tender_digitization.go`
- `pdf_ocr_bridge.go`

### 对应处理
- narrative → 综合抽取
- table → 表格专项抽取
- appendix → 约束补充抽取
- list → 设备/评分/资格专项抽取

---

## P1-3：增加关键词反查审计

### 目标
如果原文出现高价值关键词，但结果字段为空，系统要报警。

### 第一批关键词
#### 采购边界
甲供、乙供、招标人采购、中标人采购、自行采购、供货范围、责任界面

#### 工期
工期、节点、里程碑、完工、交付、延误、奖罚

#### 质量
验收、检验、试验、规范、标准、合格率

#### 安全
安全、文明施工、环保、危大工程、专项方案

#### 评分废标
评分、加分、扣分、否决、废标、不得分、必须响应

### 输出要求
若关键词命中但字段仍为空：
- 自动写入 `requires_manual_review`
- 前端红色提示“可能遗漏”

---

## P1-4：拆出专项画像层

### 目标
不要只返回一个巨大的 profile_json。

### 至少拆出这些视图
- 总画像
- 材料设备专项画像
- 工期专项画像
- 质量专项画像
- 安全文明专项画像
- 评分专项画像
- 风险专项画像

### 说明
后续大纲生成、正文生成、风控审核，要能按专项消费，而不是全量硬读。

---

## P1-5：前端增加人工补录/确认入口

### 目标
允许用户修正 AI 未抽全或抽错的高风险字段。

### 需求
- 支持编辑关键字段
- 支持标记“人工已确认”
- 下游模块优先使用人工确认值

---

# 四、P2 任务（增强项）

## P2-1：规则引擎补强

### 目标
把明确模式交给规则，不全靠模型。

### 第一批规则
- 识别工期天数、节点日期
- 识别资质等级、证书名称
- 识别评分分值、权重、加减分
- 识别甲供/乙供/招标人采购等采购边界词
- 识别岗位名称和人数

### 输出方式
规则命中的结果不要直接替换模型值，而是作为：
- 候选值
- 审计补充
- 冲突提示

---

## P2-2：建立最小回归测试集

### 目标
保证改造后不倒退。

### 要求
先准备 10 份真实或近真实样本，覆盖：
- 正文型文档
- 表格密集型文档
- 含附录/清单文档
- 含采购边界/评分/工期/资格要求文档

### 每份文档手工标注这些字段
- procurement_boundary
- owner_supplied_items
- contractor_supplied_items
- total_duration
- scoring_items
- disqualification_rules
- qualification_requirements
- personnel_requirements

### 测试输出指标
- 高风险字段召回率
- 证据可追溯率
- 漏项预警命中率
- 人工修正率

---

## P2-3：保存 chunk 级原始抽取结果和 merge 日志

### 目标
方便排查、回归、人工复盘。

### 要求
保存：
- 每个 chunk 的原始抽取 JSON
- 每个 chunk 的 evidence
- merge 前后 diff

---

## P2-4：行业词典增强

### 第一批行业
- 能源燃气
- 市政
- 水利
- 房建

### 词典用途
- 分块打标签
- 关键词审计
- prompt 行业提示
- 低置信度字段兜底

---

# 五、实施顺序（必须遵守）

## 第一轮提交
只做：P0

提交内容必须包括：
1. 代码改动
2. 一份简要说明文档
3. 至少 3 份样本文档的前后对比
4. 明确列出仍未解决的问题

## 第二轮提交
在 P0 稳定后，再做：P1

## 第三轮提交
最后做：P2

不要一上来把 P0/P1/P2 混在一起大改。

---

# 六、交付要求

## 代码之外，必须补这些文档
请输出到桌面：

1. `项目画像链路现状分析.md`
2. `项目画像改造实施记录.md`
3. `项目画像P0验收报告.md`
4. （P1 完成后）`项目画像P1验收报告.md`

---

# 七、验收口径

## P0 验收通过标准
- 固定 schema 稳定输出
- 高风险字段有证据链
- 页面能看到缺失/低置信度/待人工确认
- 采购边界、工期、评分废标、资格人员这四类字段明显改善
- 能追溯某字段从原文到前端的完整链路

## P1 验收通过标准
- 长文档分块后不再因为 merge 丢失信息
- 表格/附录中的高价值内容能进入结构字段
- 关键词命中但字段为空时会自动预警
- 支持人工补录并参与下游消费

## P2 验收通过标准
- 有最小回归测试集
- 有规则补强能力
- 有 chunk 级原始结果与 merge 日志

---

# 八、注意事项

1. 不要只改 prompt，必须连 schema、merge、前端状态一起做
2. 不要一口气追求“所有字段全覆盖”，先把高风险字段打稳
3. 不要把证据链做成可选项，证据链是必须项
4. 不要只做后端，前端必须能展示缺失、低置信度、人工确认状态
5. 不要偷懒用“长摘要”替代结构化字段

---

# 九、最终一句话任务定义

请把当前“项目画像”功能，从“偶尔抽一些内容出来的黑盒 prompt”，改造成“固定 schema、可追溯、可补漏、可人工复核”的稳定抽取系统。
