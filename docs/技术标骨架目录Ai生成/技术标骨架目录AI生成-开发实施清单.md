# 技术标骨架目录 AI 生成 - 开发实施清单

## 1. 文档目标

本文是《技术标骨架目录AI生成完整实施方案》的工程落地版，面向前端工程师、后端工程师、测试工程师和提示词优化人员，直接用于排期和开工。

项目路径：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

目标功能：
在技术标骨架目录页面，对二级分类新增 **AI 生成** 能力，一次性生成以下 4 个字段，并支持人工审核后保存：

1. 章节与结构配置（Markdown 格式）
2. 行业关键词池
3. 差异化标题池
4. 通用可选子节池（JSON Array）

---

## 2. 第一阶段交付目标

本阶段交付完成后，应满足：

- 可以在二级分类上点击“生成”
- 系统可以调用豆包生成 4 个字段草案
- 用户可以预览与修改
- 用户确认后可保存到骨架表
- 全流程有报错提示和格式校验

---

## 3. 角色分工建议

## 3.1 前端工程师
负责：
- 生成按钮
- 生成中 loading
- 预览弹窗
- 草案回填
- 表单保存
- 错误提示

## 3.2 后端工程师
负责：
- 新增 generate 接口
- 新增骨架草案生成 service
- 豆包调用接入
- JSON 解析与校验
- 返回标准化草案结果

## 3.3 测试工程师
负责：
- 生成成功场景
- 模型报错场景
- JSON 错误场景
- 用户修改后保存场景
- 边界输入测试

## 3.4 提示词优化负责人
负责：
- 通用 prompt 设计
- 行业增强 prompt 设计
- 失败样本分析
- 输出质量迭代优化

---

## 4. 前端实施清单

## 4.1 页面改造位置
文件：
`frontend/src/components/SystemSettingsSkeletonTab.tsx`

---

## 4.2 前端任务拆解

### 任务 F1：在二级分类管理列增加“生成”按钮
要求：
- 仅在二级分类模式下显示
- 与编辑、删除并列
- 点击后调用生成接口

验收标准：
- 二级分类每行可见“生成”按钮
- 一级分类不显示生成按钮

---

### 任务 F2：增加生成中状态
要求：
- 点击后按钮进入 loading
- 禁止重复点击
- 当前行独立 loading，不影响其他行

验收标准：
- 用户可清楚感知正在生成
- 重复点击不会重复发请求

---

### 任务 F3：新增 AI 生成预览弹窗
建议新增组件：
`frontend/src/components/SkeletonGeneratePreviewModal.tsx`

弹窗内容：
- 章节与结构配置（大文本框）
- 行业关键词池（JSON 文本框）
- 差异化标题池（JSON 文本框）
- 通用可选子节池（JSON 文本框）

能力要求：
- 支持编辑
- 支持复制
- 支持局部微调
- 支持关闭不保存

验收标准：
- 生成结果能完整展示
- 用户可直接编辑 4 个字段

---

### 任务 F4：生成结果回填表单
要求：
- 用户点击“采纳并回填”后
- 生成结果回填到 `skeletonForm`
- 用户再走现有保存逻辑

验收标准：
- 回填后用户能继续使用已有保存功能

---

### 任务 F5：错误提示
要求：
- 模型调用失败时提示
- JSON 解析失败时提示
- 网络错误时提示
- 后端校验失败时提示

验收标准：
- 用户知道失败原因，不会误以为卡死

---

## 4.3 建议前端状态字段

建议新增状态：
- `generateLoadingId`
- `previewVisible`
- `generatedDraft`
- `generateError`

---

## 5. 后端实施清单

## 5.1 新增接口

### 任务 B1：新增 generate handler
建议接口：
`POST /api/settings/industry-skeletons/:id/generate`

请求：
```json
{
  "overwrite": false,
  "mode": "draft"
}
```

返回：
- 4 个字段草案
- prompt version
- raw output（可选）

验收标准：
- 接口可用
- 可根据分类 id 正确生成内容

---

### 任务 B2：新增生成 service
建议文件：
`backend_go/internal/service/industry_skeleton_generator.go`

建议方法：
- `GenerateSkeletonDraft(ctx, categoryID string, overwrite bool) (*SkeletonDraft, error)`
- `BuildPrompt(...)`
- `ValidateDraft(...)`
- `NormalizeDraft(...)`

验收标准：
- service 可被 handler 调用
- 逻辑清晰，可单测

---

### 任务 B3：读取分类上下文
需要读取：
- 当前二级分类名称
- parent_id
- 一级分类名称
- 当前已存在字段内容

验收标准：
- prompt 输入上下文正确
- 不会出现一级二级分类混淆

---

### 任务 B4：接入 AIClient 调用豆包
利用现有：
`backend_go/internal/service/ai_client.go`

要求：
- 支持指定 model / endpoint / apiKey
- 调用 chat/completions
- 超时可控
- 错误信息透明返回

验收标准：
- 能稳定完成一次生成

---

### 任务 B5：解析模型 JSON 输出
要求：
- 解析出：
  - `logical_chapters_markdown`
  - `industry_keywords`
  - `title_candidate_pool`
  - `common_section_pool`
- 做类型检查
- 做非空检查

验收标准：
- 返回结构稳定
- 无效 JSON 可识别

---

### 任务 B6：后端校验与标准化
规则建议：
- `logical_chapters_markdown` 至少有 3 个 `##`
- `industry_keywords` 为数组且不为空
- `title_candidate_pool` 为对象
- `common_section_pool` 为数组
- 所有 JSON 返回前统一格式化

验收标准：
- 前端回填数据时不需要二次大改

---

## 6. 提示词实施清单

## 6.1 通用 Prompt 初版

### 任务 P1：编写通用 System Prompt
目标：
- 定义模型是“技术标目录架构专家”
- 强调只输出骨架模板，不输出正文
- 强调必须输出 JSON

验收标准：
- 模型能稳定按 JSON 输出

---

### 任务 P2：编写通用 User Prompt 模板
包含：
- 一级分类
- 二级分类
- 当前已有内容
- 输出结构规范
- 字段生成要求

验收标准：
- 支持任意行业二级分类生成

---

### 任务 P3：输出结构约束
必须包含：
```json
{
  "logical_chapters_markdown": "...",
  "industry_keywords": [],
  "title_candidate_pool": {},
  "common_section_pool": []
}
```

验收标准：
- 模型输出结构统一

---

## 6.2 提示词迭代方向

### 第二阶段可加
- 水利工程增强 prompt
- 房建工程增强 prompt
- 市政工程增强 prompt
- 公路工程增强 prompt

目标：
让不同一级行业的大类下，生成更像行业专家，而不是一套通用模板。

---

## 7. 测试实施清单

## 7.1 成功场景

### 任务 T1：二级分类正常生成
验证：
- 按钮可点
- 返回 4 个字段
- 预览正常
- 保存正常

### 任务 T2：用户修改后保存
验证：
- 生成结果可编辑
- 修改后可保存
- 数据入库正确

---

## 7.2 异常场景

### 任务 T3：模型超时/报错
验证：
- 前端有错误提示
- 页面不卡死
- 可重新点击生成

### 任务 T4：模型输出非 JSON
验证：
- 后端识别失败
- 前端收到明确报错

### 任务 T5：字段缺失
验证：
- 后端拦截并提示哪个字段缺失

---

## 7.3 边界场景

### 任务 T6：已有内容时重复生成
验证：
- overwrite=false 时不直接覆盖
- 用户可决定是否采纳

### 任务 T7：行业名称过短/过泛
验证：
- 模型仍能生成基础结果
- 或返回提示建议人工补充

---

## 8. 建议接口设计稿

## 8.1 请求

### `POST /api/settings/industry-skeletons/:id/generate`

请求体：
```json
{
  "overwrite": false,
  "mode": "draft"
}
```

---

## 8.2 响应

```json
{
  "success": true,
  "data": {
    "id": "9f3c...",
    "industry_name": "河道治理工程",
    "parent_industry_name": "水利工程",
    "logical_chapters_markdown": "## 第一章 ...",
    "industry_keywords": ["河道治理", "护岸", "导流围堰"],
    "title_candidate_pool": {
      "CH1": ["施工组织总体思路", "总体施工部署"]
    },
    "common_section_pool": ["编制依据", "工程概况", "施工重点难点分析"],
    "prompt_version": "skeleton_gen_v1"
  }
}
```

---

## 9. 推荐开发顺序

建议按下面顺序做：

1. 后端接口打通
2. AI 调用打通
3. JSON 解析和校验
4. 前端加按钮和 loading
5. 前端加预览弹窗
6. 回填表单
7. 保存入库
8. 联调测试
9. 调 prompt

---

## 10. 关键风险与决策

### 风险 1：输出质量不稳定
决策：
- 用强约束 JSON prompt
- 加后端校验
- 失败后允许重试

### 风险 2：章节写成正文提纲
决策：
- prompt 里明确“不是正文，只是骨架模板”

### 风险 3：生成过于雷同
决策：
- 差异化标题池作为必生成字段

### 风险 4：人工审核成本仍然高
决策：
- 预览弹窗内直接可编辑
- 不需要再切换页面二次录入

---

## 11. 验收标准

### 功能验收
- 二级分类可点击 AI 生成
- 可返回 4 个字段草案
- 可人工修改并保存

### 技术验收
- 接口稳定
- JSON 解析稳定
- 错误可解释

### 业务验收
- 至少 3 个不同行业二级分类可生成可用初稿
- 人工只需少量修改即可入库

---

## 12. 建议的下一步输出

如果要继续推进，建议后续再补两份文档：

1. **豆包 Prompt 精修版（按行业增强）**
2. **前后端接口草稿代码清单**

这样工程师和提示词负责人就能直接并行开工。

---

## 13. 一句话结论

这项功能最适合做成：

**“二级分类一键生成骨架草案 + 人工审核确认保存”**

它能在不破坏现有骨架体系的前提下，大幅提升技术标骨架库建设效率，并为后续 Step4 目录生成提供更完整、稳定的底座。 
