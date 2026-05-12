# 技术标骨架目录 AI 生成完整实施方案

## 1. 背景与目标

当前投标工作台在「技术标骨架目录」配置页面中，已经具备按行业分类维护目录骨架的能力，但手工录入以下字段效率较低：

1. 章节与结构配置（Markdown 格式）
2. 行业关键词池
3. 差异化标题池
4. 通用可选子节池（JSON Array）

为提升骨架库建设效率，建议在二级分类管理中新增“生成”能力，通过对接豆包大模型，为指定行业分类自动生成上述 4 个字段，人工审核后再落库。

项目代码位置：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

页面入口：
`http://127.0.0.1:5173/settings?tab=skeleton&categoryId=4419cbee-125f-4448-b3dc-0fbae5029db8`

---

## 2. 现状判断

从当前代码看，已经具备实现本功能的基础：

### 前端已有能力
- `frontend/src/components/SystemSettingsSkeletonTab.tsx`
- 已有字段：
  - `logical_chapters_json`
  - `common_section_pool_json`
  - `industry_keywords_json`
  - `title_candidate_pool_json`
- 已有编辑、保存、删除能力
- 已有“一级大类 / 二级分类”的钻取管理结构

### 后端已有能力
- `backend_go/internal/handler/settings.go`
  - `ListIndustrySkeletons`
  - `UpdateIndustrySkeleton`
  - `DeleteIndustrySkeleton`
- `backend_go/internal/service/ai_client.go`
  - 已封装 LLM 调用能力
- `backend_go/internal/service/industry_skeleton.go`
  - 已支持行业骨架加载与回退

### 结论
**不需要重构整个骨架体系，只需要新增一个“AI 生成草案”的服务链路。**

---

## 3. 推荐业务流程

### 3.1 推荐交互方式
建议采用：**编辑弹窗内生成**

流程如下：
1. 用户进入某个一级分类下的二级分类列表
2. 点击二级分类名称或右侧管理按钮
3. 打开编辑弹窗
4. 点击弹窗内的“AI生成”按钮
5. 后端调用豆包大模型生成 4 个字段草案
6. 前端展示生成结果预览
7. 用户手工微调
8. 点击保存
9. 写回 `tech_bid_industry_skeletons`

### 3.2 为什么不建议直接自动落库
- AI 输出需要人工审核
- 骨架库属于基础配置，错误会放大后续 Step4 结果偏差
- 必须保留“预览 + 确认”步骤

---

## 4. 功能范围定义

### 4.1 本期要做
- 单个二级分类一键生成
- 一次生成 4 个字段
- 支持人工审核与编辑
- 支持保存入库
- 支持生成中 loading 状态
- 支持失败重试

### 4.2 本期不做
- 批量生成所有行业分类
- 自动覆盖已有模板
- 多模型投票生成
- 自动评分排序多个版本
- 自动发布到正式骨架库

---

## 5. 后端实施方案

## 5.1 新增接口

建议新增接口：

### `POST /api/settings/industry-skeletons/:id/generate`

用途：
- 根据当前骨架分类信息调用豆包大模型
- 生成骨架草案
- 不直接入库

### 请求体建议
```json
{
  "overwrite": false,
  "mode": "draft"
}
```

### 响应体建议
```json
{
  "success": true,
  "data": {
    "id": "category-id",
    "industry_name": "水利工程-河道治理",
    "logical_chapters_markdown": "## 第一章 ...",
    "industry_keywords": ["河道治理", "护岸", "堤防"],
    "title_candidate_pool": {
      "CH1": ["施工组织总体思路", "总体施工部署"]
    },
    "common_section_pool": ["编制依据", "工程概况", "施工重点难点"],
    "raw_model_output": "...",
    "prompt_version": "skeleton_gen_v1"
  }
}
```

---

## 5.2 新增服务

建议新增服务文件：

### `backend_go/internal/service/industry_skeleton_generator.go`

职责：
1. 读取目标骨架分类信息
2. 组装豆包 prompt
3. 调用 AIClient
4. 解析模型输出
5. 做结构校验
6. 返回前端可直接回填的数据

### 核心方法建议
- `GenerateSkeletonDraft(...)`
- `BuildSkeletonGenerationPrompt(...)`
- `ValidateSkeletonDraft(...)`
- `NormalizeSkeletonDraft(...)`

---

## 5.3 新增 Handler

建议在 `settings.go` 中增加：

### `GenerateIndustrySkeletonDraft(c *gin.Context)`

行为：
1. 根据 `:id` 查骨架记录
2. 组装 prompt
3. 调用 `AIClient`
4. 解析 JSON
5. 返回草案结果

---

## 5.4 接口参数建议

### 请求参数
- `overwrite`: 是否允许覆盖已有值
- `mode`: `draft` / `refine`

### 输入上下文建议
应至少传给模型：
- 一级分类名称
- 二级分类名称
- 是否已有父级骨架
- 当前已有字段（如有）
- 目标行业说明（如果存在）

---

## 6. 前端实施方案

## 6.1 入口位置
当前页面：
`frontend/src/components/SystemSettingsSkeletonTab.tsx`

### 在二级分类管理列新增按钮：
- `生成`
- `编辑`
- `删除`

或者将 `生成` 放入编辑弹窗顶部，推荐两者同时支持：
- 列表页按钮：快速生成
- 编辑弹窗按钮：精修再生成

---

## 6.2 前端交互设计

### 建议新增 UI 组件
1. **生成按钮**
   - loading 状态
   - 防重复提交

2. **AI 生成预览弹窗**
   - 左侧显示四个字段的预览
   - 右侧可编辑
   - 支持“全部采纳 / 局部修改”

3. **字段回填逻辑**
   - 生成结果回填到 `skeletonForm`
   - 用户再保存

4. **错误提示**
   - 模型调用失败
   - JSON 解析失败
   - 字段校验失败

---

## 6.3 推荐前端改造点

### `SystemSettingsSkeletonTab.tsx`
新增：
- `handleGenerate(record)`
- `generateLoadingId`
- `generatedDraft` 状态
- `previewModalVisible`

### 交互流程
```text
点击生成
  ↓
显示 loading
  ↓
调用后端 generate 接口
  ↓
弹出预览弹窗
  ↓
用户编辑
  ↓
保存
```

---

## 7. 数据结构设计

本功能不新增数据库表，直接使用现有：

### `tech_bid_industry_skeletons`
字段：
- `industry_name`
- `parent_id`
- `logical_chapters_json`
- `common_section_pool_json`
- `industry_keywords_json`
- `title_candidate_pool_json`

### 说明
- 生成的是草案，最终仍由原保存接口写入
- 这样可以减少改造面

---

## 8. 提示词设计原则

这是本功能的核心。

### 8.1 总原则
- 结构化输出
- 强约束 JSON
- 避免自由发挥
- 避免输出正文
- 输出必须适合投标骨架复用

### 8.2 模型任务定义
模型不是在写“技术标正文”，而是在写：

> 某个行业分类的技术标目录骨架模板

即：
- 章标题怎么定
- 章与节如何组织
- 有哪些常见关键词
- 同一章有哪些差异化标题候选
- 哪些小节是常见但可选的

---

## 9. 提示词详细方案

下面给出建议的提示词结构，可直接作为豆包调用参考。

### 9.1 System Prompt

```text
你是中国建筑工程领域的技术标目录架构专家，擅长为不同细分行业设计可复用、可审计、可扩展的“技术标骨架目录模板”。

你的任务不是生成正文，而是根据给定的行业分类，输出一套可作为技术标目录底座的骨架配置。

你必须严格遵守以下要求：
1. 输出必须专业、稳健、贴合工程投标场景
2. 不允许输出与行业无关的章节
3. 不允许输出空泛营销语言
4. 不允许输出正文内容，只能输出目录骨架与配置资源
5. 输出必须严格符合指定 JSON 格式
6. 章节数量建议控制在 6~10 个主章
7. 章节结构要能支撑后续三级目录扩展
8. 要体现行业特征，而不是通用工程模板
9. 关键词池要覆盖行业核心术语、工艺术语、专项术语、风险术语、场景术语
10. 差异化标题池要体现同一章节的不同命名方式，避免模板雷同
11. 通用可选子节池要选取“常见但非必选”的内容
12. 如果信息不足，应基于行业常识补全，但必须保持工程投标语境一致

你需要输出的结果必须是纯 JSON，不要输出 Markdown 代码块，不要输出解释性文字。
```

### 9.2 User Prompt 模板

```text
请为以下行业分类生成技术标骨架目录模板：

一级分类：{{parentIndustryName}}
二级分类：{{industryName}}

已有信息（如有）：
{{existingSkeletonJsonOrNull}}

生成目标：
1. logical_chapters_markdown
2. industry_keywords
3. title_candidate_pool
4. common_section_pool

生成要求：
- logical_chapters_markdown 必须使用 Markdown 格式
- 每个主章节用 "##" 开头
- 必要章节后面可加 "(必选)"
- 可选结构属性使用花括号，例如：{core, split, fix}
- 章节应体现该行业的典型投标逻辑
- 必须适合作为后续三级目录展开的骨架
- 行业关键词池必须是数组，数量建议 30~80 个
- 差异化标题池必须按章节编码组织，格式为对象
- 通用可选子节池必须是 JSON Array，数量建议 10~30 个
- 输出中不要包含正文段落，不要写解释说明
- 不要输出 Markdown 代码块
- 不要输出除 JSON 之外的任何内容

输出 JSON 结构如下：
{
  "logical_chapters_markdown": "...",
  "industry_keywords": ["..."],
  "title_candidate_pool": {
    "CH1": ["...", "..."],
    "CH2": ["...", "..."]
  },
  "common_section_pool": ["...", "..."]
}
```

---

## 10. 字段生成规则

## 10.1 logical_chapters_markdown
要求：
- Markdown 格式
- 使用 `##` 表示主章节
- 每章下面可用 `- ` 列出可选子项
- 章节要体现行业特征

示例：
```md
## 第一章 施工组织总体思路与规划 (必选) {core, fix}
- 编制说明
- 工程概况
- 施工组织总体策划

## 第二章 施工部署与资源配置 (必选) {core, split}
- 现场布置
- 人员配置
- 机械材料计划
```

### 10.2 industry_keywords
要求：
- 数组
- 覆盖：
  - 行业名词
  - 工艺名词
  - 专项名词
  - 风险名词
  - 场景名词

示例：
```json
[
  "河道治理",
  "堤防加固",
  "护岸施工",
  "导流围堰",
  "土石方工程"
]
```

### 10.3 title_candidate_pool
要求：
- 对象
- 以章节 ID 为 key
- 每个章节对应 2~5 个可替换标题
- 目的是降低不同项目间目录雷同

示例：
```json
{
  "CH1": ["施工组织总体思路与规划", "总体施工部署与实施规划"],
  "CH2": ["施工部署与资源配置", "施工组织与资源保障" ]
}
```

### 10.4 common_section_pool
要求：
- JSON Array
- 是“常见但非每个项目都必须出现”的内容
- 用于后续目录弹性扩展

示例：
```json
[
  "编制依据",
  "工程概况",
  "施工重点难点分析",
  "质量创优目标",
  "安全文明施工措施"
]
```

---

## 11. 质量控制方案

## 11.1 后端校验
AI 返回后，后端应校验：
- JSON 是否可解析
- 是否包含 4 个字段
- 字段类型是否正确
- `logical_chapters_markdown` 是否至少包含 3 个 `##`
- `industry_keywords` 是否为空
- `common_section_pool` 是否为空
- `title_candidate_pool` 是否至少覆盖 3 个章节

## 11.2 业务规则校验
- 章节数量建议 6~10 个
- 标题不能过于泛化
- 关键词不能全是通用词
- 不允许输出明显不属于工程投标的内容

## 11.3 前端校验
- 生成结果必须可预览
- 用户可直接修改
- 提交前再做一次 JSON 结构检查

---

## 12. 技术实现建议

## 12.1 推荐代码路径

### 前端
- `frontend/src/components/SystemSettingsSkeletonTab.tsx`
- 可新增辅助组件：
  - `SkeletonGeneratePreviewModal.tsx`

### 后端
- `backend_go/internal/handler/settings.go`
- `backend_go/internal/service/industry_skeleton_generator.go`
- `backend_go/internal/service/ai_client.go`

---

## 12.2 推荐实现顺序
1. 后端新增生成草案接口
2. 打通豆包调用
3. 实现 JSON 解析与校验
4. 前端加生成按钮
5. 前端加预览弹窗
6. 人工确认后保存
7. 再做提示词调优

---

## 13. 风险与对策

### 风险 1：模型输出不稳定
对策：强制 JSON、增加校验、失败重试。

### 风险 2：生成内容过于空泛
对策：在 prompt 中强调行业名词、工艺词、投标语境。

### 风险 3：章节模板雷同
对策：启用差异化标题池，并要求模型给出多个命名候选。

### 风险 4：字段不一致
对策：后端做标准化清洗，再回填到表单。

### 风险 5：误生成正文
对策：prompt 中明确“只输出目录骨架，不输出正文”。

---

## 14. 可直接落地的用户体验

推荐最终用户体验如下：

1. 进入某二级分类
2. 点击“生成”
3. 等待 3~20 秒
4. 弹出 AI 生成结果
5. 人工快速检查
6. 修改后保存

这样可以把原来 30~60 分钟的手工配置，压缩到几分钟。

---

## 15. CTO 结论

这个功能是值得做的，并且适合先做成“草案生成 + 人工审核”的模式。

### 核心判断：
- **业务上必要**：骨架库建设效率会明显提升
- **技术上可控**：现有数据结构和 AIClient 都能支撑
- **风险可控**：人工审核兜底
- **收益明显**：Step4 后续目录生成质量会更稳定

### 最终建议：
**优先落地“二级分类一键生成 4 字段草案”功能。**

---

## 16. 附：推荐的接口返回示例

```json
{
  "success": true,
  "data": {
    "industry_name": "水利工程-河道治理",
    "logical_chapters_markdown": "## 第一章 ...",
    "industry_keywords": ["河道治理", "护岸", "堤防"],
    "title_candidate_pool": {
      "CH1": ["施工组织总体思路", "总体施工部署"]
    },
    "common_section_pool": ["编制依据", "工程概况"],
    "raw_model_output": "...",
    "prompt_version": "skeleton_gen_v1"
  }
}
```

---

## 17. 下一步建议

如果你认可这份方案，下一步可以继续补：
1. 前端按钮与弹窗交互原型
2. 后端接口草稿代码
3. 豆包 prompt 的精修版（按水利/房建/市政/公路分行业增强）
