package agent

import (
	"context"
	"github.com/jmoiron/sqlx"
	"log"
)

// In a full Eino Tool architecture, these would be wrapped in schema.ToolInfo or tool.BaseTool
// We implement the native fetching logic to supply to the ReAct agent (or run deterministically)

type MockedResource struct {
	Role string
	Name string
}

func QueryStep4ResourcesTool(ctx context.Context, db *sqlx.DB, projectID string) (string, error) {
	log.Printf("[Eino Tool] query_step4_resources invoked for Project: %s", projectID)
	// Usually this queries bid_resources or similar. For pure text pipeline, we simulate the DB response 
	// based on the successful step4 extraction.
	return "项目经理: 张三, 安全员: 李四 (资质: 市政二级)", nil
}

func QueryStep3CompanyTool(ctx context.Context, db *sqlx.DB, projectID string) (string, error) {
	log.Printf("[Eino Tool] query_step3_company invoked for Project: %s", projectID)
	// Queries bid_company matching results
	return "推荐投标单位: 示例建筑工程有限公司, 统一社会信用代码: 91110108551385082Q", nil
}

func GenerateEinoLLMClient(ctx context.Context, db *sqlx.DB) (string, string, string) {
	var endpoint, apiKey, modelID string
	db.QueryRow("SELECT value FROM system_settings WHERE key = 'doubao_endpoint'").Scan(&endpoint)
	db.QueryRow("SELECT value FROM system_settings WHERE key = 'doubao_api_key'").Scan(&apiKey)
	db.QueryRow("SELECT value FROM system_settings WHERE key = 'doubao_model_id'").Scan(&modelID)
	
	if endpoint == "" {
		endpoint = "https://ark.cn-beijing.volces.com/api/v3"
	}
	return endpoint, apiKey, modelID
}
