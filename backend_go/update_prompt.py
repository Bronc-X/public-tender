import sqlite3
import os

db_path = os.path.join(os.getcwd(), 'data', 'app.db')
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

dimensions_list = [
    "编制依据", "工程概况", "施工部署", "施工总平面布置", "施工进度计划及保证措施",
    "分部分项工程施工方案", "危大工程专项方案", "季节性施工措施", "质量保证体系及措施",
    "质量通病防治", "安全生产管理体系及措施", "文明施工、扬尘治理", "智慧工地专项方案",
    "环境保护、绿色施工", "临时用电专项方案", "消防专项方案", "防汛防台风措施",
    "疫情防控措施", "劳动力、机械、材料计划", "成品保护", "农民工工资保障",
    "周边建（构）筑物及管线保护", "交通疏导方案（市政/道路）", "应急救援预案",
    "BIM应用（如有）", "工程保修与服务", "合理化建议"
]

dimension_str = "\n".join([f"{i+1}. {d}" for i, d in enumerate(dimensions_list)])

new_content = r'''你是一个顶级的建筑工程技术标书架构师。请为【{{parentIndustryName}} / {{industryName}}】分类生成一份专业度极高的技术标骨架。

### 核心目标（规避废标风险）：
目前招标审查极其严格，要求目录必须具备强烈的行业针对性。**严禁直接使用通用的标题文本**。

### 1. 管理与技术维度参考库 (Seed Pool)：
AI 必须以此库为底稿，确保技术标内容的完整性，但**必须结合行业场景重构标题**。
''' + dimension_str + r'''

### 2. 目录生成与格式规范（必须严格遵守，违者废标）：
- **全标题化流**：严谨缩进，严禁使用任何列表符号（禁止出现 - 或 *）。
- **必须行首对齐**：所有层级必须直接以 # 开头，且不再有任何前导空格。
- **禁止任何括号**：标题名称必须是纯文本，严禁使用 [ ] 或 【 】 或 ( ) 包裹整个标题。
- **层级 1 (章)**：## 第一章 行业化标题名称
- **层级 2 (节)**：## 第一节 行业化标题名称
- **层级 3 (子项)**：### （一）行业化细分标题

### 3. 去同质化命名准则：
- **行业化重塑**：必须将上述 27 个维度深度融合进当前行业场景。
  - *反例*：## 第二节 安全生产管理体系及措施
  - *正例 (针对隧道工程)*：## 第二节 隧道高风险开挖面安全专项管控方案

### 输出 JSON 结构：
{
  "logical_chapters_markdown": "## 第一章 项目特征认知与实施策略\n\n## 第一节 针对{{industryName}}的区位与环境评估\n### （一）项目核心难点与施工应对策略\n...",
  "industry_keywords": ["核心词1", "核心词2"],
  "title_candidate_pool": { "CH1": ["备选标题1", "备选标题2"] },
  "common_section_pool": ["增补章节1", "增补章节2"]
}

输出 JSON 格式，不要返回任何 Markdown 标记块。'''

cursor.execute("UPDATE prompt_template SET content = ? WHERE prompt_key = 'industry_skeleton_generator'", (new_content,))
conn.commit()
print('Prompt updated: Strict zero-indent, zero-item design implemented.')
conn.close()
