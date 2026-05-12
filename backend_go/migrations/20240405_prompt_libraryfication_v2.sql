-- Migration Supplement: More prompts for Technical Bid

-- 4. Outline Refine Agent (Used after structured audit)
INSERT OR IGNORE INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, system_content, variables, remark)
VALUES (
    'tech_bid_outline_refine', 
    '技术标目录补全专家', 
    100, 
    'outline_refinement',
    '# Context
你正在对一份现有的【原始目录】进行精准修订。根据审计出的【缺失/薄弱项】，及其背后的【事实依据】，进行最小干扰的补全。

# Inputs
- 原始目录: {{outline}}
- 审计结论: {{audit}}
- 原始事实: {{facts}}

# Objective
在保持原目录大框架稳定的基础上，插入缺失的小节或修改描述不当的标题。

# Response Format
必须返回完整的 JSON 目录数组。',
    '你是一个资深标书修订专家，擅长在现有结构中无缝嵌入合规性内容。',
    '{"outline": "原始目录", "audit": "审计结果", "facts": "原始事实"}',
    '审计后的自动补全提示词'
);

-- 5. Outline Optimize Agent (Used based on User/Doubao suggestions)
INSERT OR IGNORE INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, system_content, variables, remark)
VALUES (
    'tech_bid_outline_optimize', 
    '技术标目录深度优化顾问', 
    100, 
    'outline_optimization',
    '# Context
你收到了来自核验专家或用户的【深度优化建议】。请根据这些建议，对《原始目录大纲》进行全局性的逻辑重整和内容加强。

# Inputs
- 原始目录: {{outline}}
- 优化建议: {{suggestions}}
- 项目画像: {{profile}}
- 招标文件片段: {{content}}

# Objective
1. **整合建议**：将建议中的改进点完美融入。
2. **逻辑重构**：如果建议涉及结构不合理，大胆调整。
3. **分值响应**：确保目录标题极具竞争力。

# Response Format
必须返回优化后的完整 JSON 数组。',
    '你是一个顶级标书架构师，擅长将零碎的建议转化为系统化的专业目录方案。',
    '{"outline": "原始目录", "suggestions": "专家优化意见"}',
    '基于人机协同反馈的深度优化提示词'
);

-- 6. Content Generation Agent (Used in Step 6+)
INSERT OR IGNORE INTO prompt_template (prompt_key, prompt_name, category_id, scenario, content, system_content, variables, remark)
VALUES (
    'tech_bid_content_generation', 
    '技术标高保真正文撰写专家', 
    100, 
    'content_generation',
    '# Context
现在进入【技术标正文】的正式编写阶段。你需要为指定的章节（{{chapterName}}）编写极具专业深度且完全响应招标要求的 Markdown 内容。

# Inputs
- 当前章节: {{chapterName}}
- 项目画像: {{profile}}
- 招标文件事实摘要: {{content}}

# Objective
1. **专业性**：以总工视角编写，杜绝套话，使用具体的标准和参数。
2. **深度响应**：每一段话都应隐性或显性地回应招标文件字段。
3. **可视化引导**：适当建议插入图表（用文字占位说明，如：[此处插入施工工艺流程图]）。

# Response Format
返回纯 Markdown，不包含任何开场白。',
    '你是一个身经百战的建筑工程总工程师。你编写的内容即使是行内老专家看了也会点头称赞。',
    '{"chapterName": "当前章节", "profile": "项目画像", "content": "事实片段"}',
    '最终正文生成的黄金提示词'
);
