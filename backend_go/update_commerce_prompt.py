import sqlite3
import os

db_path = os.path.join(os.getcwd(), 'data', 'app.db')
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

new_content = r'''你是一个资深的工程商务标分析专家。请根据以下文本，提取出商务规则，严格排除施工组织设计、施工方案、安全文明等纯技术层面内容。

如果在标书中发现了对供应商的奇葩限制、新颖规则，但无法归类到资质或人员中，请务必全部塞入 other_mandatory_requirements 数组中，禁止遗漏任何约束条件！

请必须提取以下结构并以 JSON 返回（如果在文本中找不到，请赋值空字符串或空数组，不要虚构）：
{
  "bidder_requirements": {
    "qualification_requirements": "企业资质要求描述（如建筑工程总承包一级等）",
    "personnel_requirements": ["各类人员要求（建造师、安全员等）和他们的资格证书说明"],
    "performance_requirements": ["企业或项目经理类似工程业绩要求（需要具体到年份、容量、规模、造价或数量要求。如果原文中写有“详见综合评分明细表”或类似指引，请你必须在文中找出并提取出对应评分表内的具体业绩考核指标！千万不能仅回复“详见明细表”！）"],
    "financial_requirements": ["财务状况要求（如要求提供近几年审计报告、营业额、利润要求、资不抵债将被否决等）"],
    "credit_requirements": ["信誉要求（如没有处于被责令停业状态、未被列入严重违法失信企业名单或失信被执行人等）"],
    "other_mandatory_requirements": ["非技术、非资质类的其他奇葩要求、本地化限制等"]
  },
  "evaluation_and_performance_rules": {
    "scoring_items": ["商务、报价以及人员的评分条款明细（强烈要求：排除现场施工方案的打分）"],
    "disqualification_rules": ["任何会导致废标或否决投标的约定条款"]
  }
}

如果文本中包含列表数据，将其转化为数组输出。切记：严格返回标准JSON格式，禁止包含解释性后记。
'''

cursor.execute("UPDATE prompt_template SET content = ? WHERE prompt_key = 'commerce_rule_extraction'", (new_content,))
conn.commit()
print('commerce_rule_extraction updated in app.db')
conn.close()
