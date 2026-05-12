import sqlite3
import json
import uuid

DB_PATH = "/Users/raoyi/.openclaw/workspace/hudi/bid_data_management/backend_go/data/app.db"

def get_general_chapters():
    return [
        {"id": "CH1", "name": "施工组织总体策划", "description": "项目理解与总体分析、管理目标与承诺、施工组织总体思路", "is_mandatory": True, "unit_pool": ["项目理解与总体分析", "管理目标与承诺", "施工组织总体思路"]},
        {"id": "CH2", "name": "施工部署与资源清单", "description": "施工现场平面布置、劳动力配置计划", "is_mandatory": True, "unit_pool": ["施工现场平面布置", "劳动力配置计划", "主要物资与机械设备计划"]},
        {"id": "CH3", "name": "主要施工方案与工艺方法", "description": "针对本项目特性的核心施工技术方案", "is_mandatory": True, "unit_pool": ["核心工艺流程", "关键部位施工技术", "专项难点控制措施"]},
        {"id": "CH4", "name": "质量目标与保证体系", "is_mandatory": True, "unit_pool": ["质量管理体系及制度", "质量验收标准"]},
        {"id": "CH5", "name": "安全与文明施工保证措施", "is_mandatory": True, "unit_pool": ["安全生产管理体系", "环境保护与文明施工"]},
        {"id": "CH6", "name": "进度保障与应急预案", "is_mandatory": True, "unit_pool": ["施工进度计划", "突发事件应急预案"]}
    ]

def inject_subs():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    # 1. 查找顶级“通用工程”ID
    cursor.execute("SELECT id FROM tech_bid_industry_skeletons WHERE industry_name = '通用工程' AND (parent_id IS NULL OR parent_id = '') LIMIT 1")
    row = cursor.fetchone()
    if not row:
        print("Error: Global '通用工程' not found. Please run the previous injection script first.")
        return
    parent_id = row[0]
    print(f"Parent Group '通用工程' found with ID: {parent_id}")

    sub_names = [
        "通用设备安装与调试工作",
        "零星房屋维修与零星修缮工程",
        "通用技术改造与设施升级",
        "纯劳务/临时机械租赁分包",
        "日常综合管护与园林环卫服务",
        "临时设施搭建与配套辅助工程",
        "通用物资采购及配套安装"
    ]

    chapters_json = json.dumps(get_general_chapters(), ensure_ascii=False)

    for name in sub_names:
        cursor.execute("SELECT id FROM tech_bid_industry_skeletons WHERE industry_name = ? AND parent_id = ?", (name, parent_id))
        if cursor.fetchone():
            print(f"Skip existing: {name}")
            continue

        print(f"Injecting: {name}")
        cursor.execute("""
            INSERT INTO tech_bid_industry_skeletons (id, industry_name, parent_id, logical_chapters_json, generation_status)
            VALUES (?, ?, ?, ?, 'done')
        """, (str(uuid.uuid4()), name, parent_id, chapters_json))

    conn.commit()
    conn.close()
    print("Sub-injection completed.")

if __name__ == "__main__":
    inject_subs()
