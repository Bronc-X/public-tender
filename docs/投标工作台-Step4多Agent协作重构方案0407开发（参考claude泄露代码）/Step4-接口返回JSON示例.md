# Step4 接口返回 JSON 示例

项目：投标工作台 / 技术标第 4 步多 Agent 协作

项目路径：
`/Users/raoyi/.openclaw/workspace/hudi/bid_data_management`

本文给出 Step4 关键接口的建议返回结构，便于前后端联调。

---

## 1. 启动 Step4 编排

### 接口
`POST /api/tech-bid/projects/:id/outline/run`

### 响应示例
```json
{
  "success": true,
  "message": "Step4 编排任务已启动",
  "data": {
    "run_id": 10021,
    "project_id": 501,
    "status": "requirements_extracting",
    "gate_result": null,
    "current_agent": "requirement_agent",
    "started_at": "2026-04-07T09:50:00+08:00"
  }
}
```

---

## 2. 查询运行状态

### 接口
`GET /api/tech-bid/projects/:id/outline/run-status`

### 响应示例
```json
{
  "success": true,
  "data": {
    "run_id": 10021,
    "project_id": 501,
    "status": "coverage_auditing",
    "gate_result": null,
    "current_stage": "coverage_auditing",
    "current_agent": "coverage_auditor_agent",
    "progress": 80,
    "started_at": "2026-04-07T09:50:00+08:00",
    "last_error": null,
    "stages": [
      {
        "stage": "requirements_extracting",
        "agent": "requirement_agent",
        "status": "done",
        "duration_ms": 4200
      },
      {
        "stage": "facts_mapping",
        "agent": "fact_agent",
        "status": "done",
        "duration_ms": 5100
      },
      {
        "stage": "outline_planning",
        "agent": "outline_planner_agent",
        "status": "done",
        "duration_ms": 6200
      },
      {
        "stage": "coverage_auditing",
        "agent": "coverage_auditor_agent",
        "status": "running",
        "duration_ms": 1300
      }
    ]
  }
}
```

---

## 3. 查询 requirement 清单

### 接口
`GET /api/tech-bid/projects/:id/outline/requirements`

### 响应示例
```json
{
  "success": true,
  "data": {
    "run_id": 10021,
    "requirements": [
      {
        "id": "req_001",
        "requirement_id": "R001",
        "requirement_type": "technical_requirement",
        "source_text": "施工组织设计应包含施工部署、资源配置、工期安排等内容。",
        "source_location": "招标文件 第三章 3.2.1",
        "priority": "high",
        "must_be_explicit": 1,
        "expected_response_level": "chapter",
        "domain": "construction_plan",
        "response_tier": "strong",
        "summary": "需明确设置施工部署、资源配置、工期安排相关章节"
      }
    ]
  }
}
```

---

## 4. 查询 fact mapping

### 接口
`GET /api/tech-bid/projects/:id/outline/mappings`

### 响应示例
```json
{
  "success": true,
  "data": {
    "run_id": 10021,
    "mappings": [
      {
        "id": "map_001",
        "fact_id": "fact_1001",
        "fact_type": "project_experience",
        "fact_name": "XX 水利枢纽施工组织经验",
        "target_level": "chapter",
        "target_path": ["施工组织总体部署", "施工资源配置"],
        "required": true,
        "priority": "high",
        "mapping_reason": "该业绩可直接支撑施工组织与资源配置目录设置",
        "mapping_source": "企业业绩库",
        "source_chapter": "类似项目经验",
        "page_number": 12,
        "line_number": 8,
        "source_location": "业绩材料 第12页"
      }
    ]
  }
}
```

---

## 5. 查询 coverage 结果

### 接口
`GET /api/tech-bid/projects/:id/outline/coverage`

### 响应示例
```json
{
  "success": true,
  "data": {
    "coverage": {
      "id": "cov_001",
      "outline_version": 3,
      "fact_total": 48,
      "fact_mapped": 44,
      "coverage_rate": 0.9167,
      "missing_fact_ids": ["fact_1012", "fact_1033"],
      "weak_fact_ids": ["fact_1009"],
      "duplicate_node_hints": ["施工部署 与 施工总体安排 语义重复"],
      "result": "REVISE",
      "summary": "覆盖率较高，但仍有少量事实未映射"
    }
  }
}
```

---

## 6. 查询 full response 结果

### 接口
`GET /api/tech-bid/projects/:id/outline/full-response`

### 响应示例
```json
{
  "success": true,
  "data": {
    "full_response": {
      "id": "fr_001",
      "outline_version": 3,
      "requirement_total": 62,
      "requirement_mapped": 58,
      "requirement_fully_responded": 52,
      "requirement_weakly_responded": 6,
      "requirement_only_tagged": 4,
      "full_response_rate": 0.8387,
      "weak_response_rate": 0.0968,
      "response_quality_score": 0.8510,
      "missing_requirement_ids": ["R021", "R034"],
      "weak_requirement_ids": ["R006", "R009"],
      "only_tagged_requirement_ids": ["R015", "R018"],
      "shell_title_hints": ["质量管理措施 标题存在但支撑不足"],
      "high_priority_missing_ids": ["R021"],
      "mandatory_missing_ids": ["R034"],
      "mandatory_insufficient_ids": ["R015"],
      "hard_rule_warnings": ["存在必须显式响应项未形成独立目录表达"],
      "result": "REVISE",
      "summary": "当前目录尚未完全响应关键条款，需要修正"
    }
  }
}
```

---

## 7. 查询冲突审计

### 接口
`GET /api/tech-bid/projects/:id/outline/conflict-audit`

### 响应示例
```json
{
  "success": true,
  "data": {
    "audit": {
      "id": "ca_001",
      "project_id": "501",
      "has_block": false,
      "conflicts": [
        {
          "conflict_id": "c001",
          "conflict_type": "duplicate_node",
          "field_name": "node_name",
          "source_a": "施工总体部署",
          "source_b": "施工部署安排",
          "description": "两个节点语义高度重复，建议合并",
          "severity": "medium"
        }
      ],
      "summary": "存在重复节点，但不构成阻断"
    }
  }
}
```

---

## 8. 查询结构修正方案

### 接口
`GET /api/tech-bid/projects/:id/structure-plan`

### 响应示例
```json
{
  "success": true,
  "data": {
    "plan": {
      "id": "sp_001",
      "adjustments": [
        {
          "action": "merge",
          "target_name": "施工总体部署 / 施工部署安排",
          "reason": "语义重复，合并后表达更清晰",
          "priority": 1
        },
        {
          "action": "insert",
          "target_name": "质量管理与保证措施",
          "new_index": 6,
          "reason": "用于承接高优先级 requirement R021",
          "priority": 1
        }
      ],
      "profile": {
        "gate_result": "REVISE",
        "risk_level": "medium"
      },
      "rationale": "当前目录可修正后通过，建议按调整动作优化",
      "status": "pending",
      "created_at": "2026-04-07T09:52:00+08:00"
    }
  }
}
```

---

## 9. 审批结构方案

### 接口
`POST /api/tech-bid/projects/:id/structure-plan/approve`

### 响应示例
```json
{
  "success": true,
  "message": "结构方案已通过",
  "data": {
    "plan_id": "sp_001",
    "status": "approved",
    "approved_at": "2026-04-07T09:55:00+08:00"
  }
}
```

---

## 10. 驳回结构方案

### 接口
`POST /api/tech-bid/projects/:id/structure-plan/reject`

### 请求示例
```json
{
  "reason": "质量管理章节不应后置，应提前到施工组织部分"
}
```

### 响应示例
```json
{
  "success": true,
  "message": "结构方案已驳回",
  "data": {
    "plan_id": "sp_001",
    "status": "rejected"
  }
}
```

---

## 11. Gate Override

### 接口
`POST /api/tech-bid/projects/:id/outline/step4-gate/override`

### 请求示例
```json
{
  "reason": "缺失项已人工确认可在正文生成阶段补齐",
  "operator_id": "u10086"
}
```

### 响应示例
```json
{
  "success": true,
  "message": "Step4 Gate 已人工覆盖",
  "data": {
    "project_id": 501,
    "gate_result": "PASS",
    "override": true
  }
}
```

---

## 12. 查询 Agent 运行日志

### 接口
`GET /api/tech-bid/projects/:id/outline/agent-runs`

### 响应示例
```json
{
  "success": true,
  "data": {
    "runs": [
      {
        "agent_name": "requirement_agent",
        "stage": "requirements_extracting",
        "status": "done",
        "input_summary": "招标文件 + 评分表",
        "output_summary": "抽取 62 条 requirement",
        "started_at": "2026-04-07T09:50:00+08:00",
        "finished_at": "2026-04-07T09:50:04+08:00"
      },
      {
        "agent_name": "coverage_auditor_agent",
        "stage": "coverage_auditing",
        "status": "done",
        "input_summary": "目录版本 v3 + requirement 清单",
        "output_summary": "coverage_rate=0.9167",
        "started_at": "2026-04-07T09:50:14+08:00",
        "finished_at": "2026-04-07T09:50:16+08:00"
      }
    ]
  }
}
```

---

## 13. 建议说明

建议后端统一返回格式：
- `success`
- `message`
- `data`

建议前端按下面方式消费：
- `run-status` 驱动时间线与进度条
- `requirements/mappings/coverage/full-response/conflict-audit` 驱动 Step4 证据面板
- `structure-plan` 驱动人工审批面板
- `agent-runs` 驱动过程可视化
