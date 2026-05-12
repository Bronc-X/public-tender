-- Migration: Add Format Template Boundary Extraction Prompt
-- Category: 技术标 AI 编排 (ID: 100)

UPDATE prompt_template 
SET content = content || '
# 特殊提取增强指令：格式模板页码边界提取
本招标文档阅读过程中，请务必同步寻找明确标有《投标文件格式》、《投标文件组成》、《附件表单》等章节。
如果发现此类提供标准商务表单或技术表单空壳模板的地方，请在输出的最外层 JSON 结构中新增一个键 `format_template_boundary` 对象，格式必须严格如下:
"format_template_boundary": {
  "detected": true,
  "start_page": 对应的自然起始页码整数,
  "end_page": 对应的自然结束页码整数,
  "source_text": "此处是支持该边界判断的简单原文大纲名字",
  "notes": "填写为何判断是模板，例如：标题为‘第三章 投标文件格式’"
}
如果没有发现明确的批量格式模板部分，请同样输出该对象，但必须将 `detected` 设为 false，以表明已确认检查过。'
WHERE prompt_key IN ('tender_profile', 'tender_rule_extraction', 'commerce_rule_extraction');
