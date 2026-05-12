package service

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
)

// FullResponseGateConfig 完全响应硬门槛数值（可由 system_settings 与按专业覆盖）
type FullResponseGateConfig struct {
	BlockFullRateMax       float64 `json:"block_full_rate_max"`       // 低于此完全响应率 → BLOCK（默认 75）
	PassFullRateMin        float64 `json:"pass_full_rate_min"`        // 放行线（默认 90）
	OnlyTaggedRatioBlock   float64 `json:"only_tagged_ratio_block"`   // 仅挂标签占比超过 → BLOCK（默认 35）
	ShellHintsBlockCount     int     `json:"shell_hints_block_count"`   // 空壳提示条数 ≥ → BLOCK（默认 10）
	ShellHintsReviseCount    int     `json:"shell_hints_revise_count"`  // 空壳提示条数 > → REVISE（默认 4）
	MinQualityScore          float64 `json:"min_quality_score"`         // 质量分放行线（默认 85）
	ShellPenaltyPerHint      float64 `json:"shell_penalty_per_hint"`    // 每条空壳扣分（默认 2.5）
	ShellPenaltyMax          float64 `json:"shell_penalty_max"`         // 空壳扣分上限（默认 22）
}

// gateConfigFile 顶层 JSON：可选 default + 按专业名子串匹配 by_profession
type gateConfigFile struct {
	Default        *FullResponseGateConfig            `json:"default"`
	ByProfession   map[string]FullResponseGateConfig `json:"by_profession"`
	ByProjectType  map[string]FullResponseGateConfig `json:"by_project_type"`
}

// DefaultFullResponseGateConfig 与当前硬编码行为一致的默认值
func DefaultFullResponseGateConfig() FullResponseGateConfig {
	return FullResponseGateConfig{
		BlockFullRateMax:      75,
		PassFullRateMin:       90,
		OnlyTaggedRatioBlock:  35,
		ShellHintsBlockCount:  10,
		ShellHintsReviseCount: 4,
		MinQualityScore:       85,
		ShellPenaltyPerHint:   2.5,
		ShellPenaltyMax:       22,
	}
}

// LoadFullResponseGateConfig 从 system_settings 合并；按 profession / project_type 选覆盖
func LoadFullResponseGateConfig(db *sqlx.DB, profession, projectType string) FullResponseGateConfig {
	base := DefaultFullResponseGateConfig()
	if db == nil {
		return base
	}
	var raw string
	if err := db.Get(&raw, `SELECT value FROM system_settings WHERE key = 'tech_bid_full_response_gate_config'`); err != nil || strings.TrimSpace(raw) == "" {
		return applyProfessionHints(base, profession, projectType)
	}
	var f gateConfigFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		log.Printf("[GateConfig] invalid JSON tech_bid_full_response_gate_config: %v", err)
		return applyProfessionHints(base, profession, projectType)
	}
	if f.Default != nil {
		base = mergeGateConfig(base, *f.Default)
	}
	pt := strings.TrimSpace(projectType)
	if pt != "" && f.ByProjectType != nil {
		for key, ov := range f.ByProjectType {
			if strings.EqualFold(key, pt) || strings.Contains(strings.ToLower(pt), strings.ToLower(key)) {
				base = mergeGateConfig(base, ov)
				break
			}
		}
	}
	pr := strings.TrimSpace(profession)
	if pr != "" && f.ByProfession != nil {
		for key, ov := range f.ByProfession {
			k := strings.TrimSpace(key)
			if k == "" {
				continue
			}
			if strings.Contains(strings.ToLower(pr), strings.ToLower(k)) {
				base = mergeGateConfig(base, ov)
				break
			}
		}
	}
	return base
}

func mergeGateConfig(base FullResponseGateConfig, ov FullResponseGateConfig) FullResponseGateConfig {
	if ov.BlockFullRateMax > 0 {
		base.BlockFullRateMax = ov.BlockFullRateMax
	}
	if ov.PassFullRateMin > 0 {
		base.PassFullRateMin = ov.PassFullRateMin
	}
	if ov.OnlyTaggedRatioBlock > 0 {
		base.OnlyTaggedRatioBlock = ov.OnlyTaggedRatioBlock
	}
	if ov.ShellHintsBlockCount > 0 {
		base.ShellHintsBlockCount = ov.ShellHintsBlockCount
	}
	if ov.ShellHintsReviseCount > 0 {
		base.ShellHintsReviseCount = ov.ShellHintsReviseCount
	}
	if ov.MinQualityScore > 0 {
		base.MinQualityScore = ov.MinQualityScore
	}
	if ov.ShellPenaltyPerHint > 0 {
		base.ShellPenaltyPerHint = ov.ShellPenaltyPerHint
	}
	if ov.ShellPenaltyMax > 0 {
		base.ShellPenaltyMax = ov.ShellPenaltyMax
	}
	return base
}

// 无 DB 配置时的内置「水利/市政」略放宽示例（仍可通过 JSON 覆盖）
func applyProfessionHints(base FullResponseGateConfig, profession, projectType string) FullResponseGateConfig {
	p := strings.ToLower(profession + " " + projectType)
	if strings.Contains(p, "水利") || strings.Contains(p, "水电") || strings.Contains(p, "河道") {
		ov := FullResponseGateConfig{
			BlockFullRateMax:     72,
			PassFullRateMin:      88,
			OnlyTaggedRatioBlock: 38,
			MinQualityScore:      82,
		}
		return mergeGateConfig(base, ov)
	}
	if strings.Contains(p, "市政") || strings.Contains(p, "公路") {
		ov := FullResponseGateConfig{
			BlockFullRateMax:     74,
			PassFullRateMin:      89,
			OnlyTaggedRatioBlock: 36,
		}
		return mergeGateConfig(base, ov)
	}
	return base
}
