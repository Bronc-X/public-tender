import sqlite3
import json
import uuid
import os

DB_PATH = "/Users/raoyi/.openclaw/workspace/hudi/bid_data_management/backend_go/data/app.db"

def get_general_chapters():
    return [
        {"id": "CH1", "name": "施工组织总体策划", "description": "项目理解与总体分析、管理目标与承诺、施工组织总体思路", "is_mandatory": True, "unit_pool": ["项目理解与总体分析", "管理目标与承诺", "施工组织总体思路"]},
        {"id": "CH2", "name": "施工部署与资源清单", "description": "施工现场平面布置、劳动力配置计划、主要物资与机械设备计划", "is_mandatory": True, "unit_pool": ["施工现场平面布置", "劳动力配置计划", "主要物资与机械设备计划"]},
        {"id": "CH3", "name": "分部分项工程主要施工方案", "description": "土石方工程专项方案、主体结构工艺流程、关键部位专项施工方案", "is_mandatory": True, "unit_pool": ["土石方工程专项方案", "主体结构工艺流程", "关键部位专项施工方案"]},
        {"id": "CH4", "name": "质量目标与保证体系", "description": "质量管理体系及制度", "is_mandatory": True, "unit_pool": ["质量管理体系及制度", "质量通病预防与处理"]},
        {"id": "CH5", "name": "安全管理与文明施工规程", "description": "安全生产管理体系、施工现场文明工地建设", "is_mandatory": True, "unit_pool": ["安全生产管理体系", "施工现场文明工地建设"]},
        {"id": "CH6", "name": "工期保障与应急处置预案", "description": "施工总进度计划保证、防汛防台及应急处置", "is_mandatory": True, "unit_pool": ["施工总进度计划保证", "应急演练与突发事件处置"]}
    ]

def get_water_chapters():
    return [
        {"id": "CH1", "name": "施工组织总体思路与规划", "is_mandatory": True, "unit_pool": ["编制说明", "工程概况", "施工组织总体策划"]},
        {"id": "CH2", "name": "施工部署与资源配置", "is_mandatory": True, "unit_pool": ["施工现场平面设计", "主要管理人员配置", "劳动力计划"]},
        {"id": "CH3", "name": "主要施工方案与工艺技术", "is_mandatory": True, "unit_pool": ["导流与围堰工程", "土石方工程", "砌石与混凝土施工", "堤防与河道治理", "险工加固专项"]},
        {"id": "CH4", "name": "关键工序与专项工艺控制", "is_mandatory": False, "unit_pool": ["雨季施工专项", "冬期施工专项", "软基处理技术"]},
        {"id": "CH5", "name": "质量控制保障与验收管理", "is_mandatory": True, "unit_pool": ["质量管理体系及措施", "关键工序质量验收"]},
        {"id": "CH6", "name": "安全文明施工与环保水保", "is_mandatory": True, "unit_pool": ["安全生产管理体系及措施", "文明施工及扬尘治理", "水土保持与环境保护"]},
        {"id": "CH7", "name": "工期保障与风险防控应对", "is_mandatory": True, "unit_pool": ["进度计划与保障措施", "防汛应急预案"]}
    ]

def get_building_chapters():
    return [
        {"id": "CH1", "name": "项目概况与管理部署", "is_mandatory": True, "unit_pool": ["工程简述", "管理目标", "施工组织总体安排"]},
        {"id": "CH2", "name": "施工平面布置与资源配置", "is_mandatory": True, "unit_pool": ["施工总平面图布置", "资源投入计划"]},
        {"id": "CH3", "name": "主要施工方案与工艺流程", "is_mandatory": True, "unit_pool": ["主体结构工程", "建筑防水工程", "二次结构与装饰装修"]},
        {"id": "CH4", "name": "质量重点监控与成品保护", "is_mandatory": True, "unit_pool": ["质量管理节点控制", "样板先行制度"]},
        {"id": "CH5", "name": "安全文明施工与绿色施工", "is_mandatory": True, "unit_pool": ["安全生产保障措施", "文明施工管理体系"]},
        {"id": "CH6", "name": "总分包配合与季节性施工", "is_mandatory": True, "unit_pool": ["总分包协调管理", "季节性施工措施"]}
    ]

def run_injection():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    # Get Parents
    cursor.execute("SELECT id, industry_name FROM tech_bid_industry_skeletons WHERE parent_id IS NULL OR parent_id = ''")
    parents = cursor.fetchall()
    print(f"Found {len(parents)} top-level categories.")

    # 1. Global "通用工程"
    global_id = str(uuid.uuid4())
    cursor.execute("SELECT id FROM tech_bid_industry_skeletons WHERE industry_name = '通用工程' LIMIT 1")
    if not cursor.fetchone():
        print("Inserting Global '通用工程'...")
        cursor.execute("""
            INSERT INTO tech_bid_industry_skeletons (id, industry_name, logical_chapters_json, generation_status)
            VALUES (?, ?, ?, 'done')
        """, (global_id, "通用工程", json.dumps(get_general_chapters(), ensure_ascii=False)))

    # 2. Per-industry templates
    for p_id, p_name in parents:
        sub_name = f"通用{p_name}工程模版"
        cursor.execute("SELECT id FROM tech_bid_industry_skeletons WHERE industry_name = ? AND parent_id = ?", (sub_name, p_id))
        if cursor.fetchone():
            print(f"Skip existing: {sub_name}")
            continue

        print(f"Injecting: {sub_name}")
        chapters = get_general_chapters()
        if "水利" in p_name: chapters = get_water_chapters()
        elif "建筑" in p_name or "房建" in p_name: chapters = get_building_chapters()
        
        cursor.execute("""
            INSERT INTO tech_bid_industry_skeletons (id, industry_name, parent_id, logical_chapters_json, generation_status)
            VALUES (?, ?, ?, ?, 'done')
        """, (str(uuid.uuid4()), sub_name, p_id, json.dumps(chapters, ensure_ascii=False)))

    conn.commit()
    conn.close()
    print("Injection completed.")

if __name__ == "__main__":
    run_injection()
