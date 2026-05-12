package service

import (
	"strings"
)

// IndustryDictionary holds domain-specific keywords for an industry.
type IndustryDictionary struct {
	Name     string              `json:"name"`
	Keywords map[string][]string `json:"keywords"` // category → keywords
}

// GetIndustryDictionaries returns the first batch of 4 industry dictionaries.
func GetIndustryDictionaries() []IndustryDictionary {
	return []IndustryDictionary{
		{
			Name: "能源燃气",
			Keywords: map[string][]string{
				"equipment": {
					"管道", "阀门", "调压站", "调压器", "调压箱",
					"LNG", "CNG", "燃气表", "PE管", "钢管",
					"球阀", "蝶阀", "法兰", "三通", "弯头",
					"储气罐", "气化器", "加臭装置", "流量计",
					"防爆电气", "燃气报警器", "切断阀",
				},
				"process": {
					"带气作业", "穿越", "定向钻", "焊接", "热熔连接",
					"电熔连接", "管道试压", "气密性试验", "通球试验",
					"阴极保护", "防腐", "补口", "开挖", "回填",
					"管道清洗", "管道置换", "碰口", "封堵",
				},
				"standard": {
					"GB50028", "CJJ63", "CJJ/T", "GB50251", "GB50369",
					"TSG", "城镇燃气", "天然气", "液化石油气",
				},
				"risk": {
					"泄漏", "爆炸", "腐蚀", "第三方破坏", "管道老化",
					"超压", "带压操作", "有限空间", "窒息",
				},
			},
		},
		{
			Name: "市政",
			Keywords: map[string][]string{
				"equipment": {
					"排水管", "给水管", "雨水管", "污水管", "桥梁",
					"管廊", "路灯", "信号灯", "窨井", "检查井",
					"泵站", "水表", "消火栓", "交通标志", "护栏",
					"隔离墩", "电缆沟", "变压器", "配电箱",
				},
				"process": {
					"沥青摊铺", "管道开挖", "顶管", "非开挖",
					"路基处理", "路面铺设", "雨污分流",
					"绿化移植", "管线迁改", "交通导改", "围挡施工",
					"混凝土浇筑", "压路", "碾压", "切割",
				},
				"standard": {
					"CJJ1", "CJJ/T", "CJJ2", "GB50268", "GB50141",
					"GB50838", "市政道路", "城市道路", "市政管网",
				},
				"risk": {
					"塌陷", "渗漏", "沉降", "管线冲突", "交通中断",
					"地下管线", "噪音扰民", "扬尘", "管道爆裂",
				},
			},
		},
		{
			Name: "水利",
			Keywords: map[string][]string{
				"equipment": {
					"大坝", "水库", "泵站", "闸门", "渡槽",
					"水电站", "涵洞", "堤防", "溢洪道", "消力池",
					"引水隧洞", "压力管道", "启闭机", "清污机",
					"量水堰", "水位计", "流速仪",
				},
				"process": {
					"灌浆", "防渗", "清淤", "截流", "导流",
					"碾压混凝土", "帷幕灌浆", "固结灌浆", "锚固",
					"抛石护岸", "生态护坡", "河道治理", "疏浚",
					"围堰", "基坑降水", "灌溉", "排涝",
				},
				"standard": {
					"SL", "DL", "SL274", "SL288", "GB50201",
					"水利水电", "水利枢纽", "灌区", "河道",
				},
				"risk": {
					"溃坝", "渗流", "冲刷", "滑坡", "洪水",
					"管涌", "坝基渗漏", "边坡失稳", "淤积",
				},
			},
		},
		{
			Name: "房建",
			Keywords: map[string][]string{
				"equipment": {
					"基坑", "桩基", "剪力墙", "幕墙", "电梯",
					"钢结构", "框架结构", "砌体", "模板", "脚手架",
					"塔吊", "施工电梯", "混凝土泵", "钢筋",
					"防水卷材", "保温板", "铝合金门窗",
				},
				"process": {
					"精装修", "消防安装", "暖通安装", "给排水安装",
					"电气安装", "土方开挖", "基坑支护", "降水",
					"钢筋绑扎", "混凝土浇筑", "砌墙", "抹灰",
					"防水施工", "保温施工", "吊装", "焊接",
				},
				"standard": {
					"GB50300", "JGJ", "GB50204", "GB50207", "GB50210",
					"GB50303", "建筑工程", "住宅工程", "房屋建筑",
				},
				"risk": {
					"坍塌", "高处坠落", "触电", "物体打击",
					"机械伤害", "深基坑", "高支模", "大体积混凝土",
					"临边防护", "脚手架倒塌",
				},
			},
		},
	}
}

// DetectIndustry scans text and returns the most likely industry name.
// Returns empty string if no industry reaches the minimum threshold.
func DetectIndustry(text string) string {
	if text == "" {
		return ""
	}

	const minThreshold = 5

	dictionaries := GetIndustryDictionaries()
	bestIndustry := ""
	bestCount := 0

	// Use a substring of text if very long (first 50000 chars)
	searchText := text
	if len(searchText) > 50000 {
		searchText = searchText[:50000]
	}

	for _, dict := range dictionaries {
		count := 0
		seen := make(map[string]bool)
		for _, keywords := range dict.Keywords {
			for _, kw := range keywords {
				if seen[kw] {
					continue
				}
				if strings.Contains(searchText, kw) {
					seen[kw] = true
					count++
				}
			}
		}
		if count > bestCount {
			bestCount = count
			bestIndustry = dict.Name
		}
	}

	if bestCount >= minThreshold {
		return bestIndustry
	}
	return ""
}

// GetIndustryAuditGroups returns additional keyword audit groups for a detected industry.
// These extend the base getKeywordAuditGroups with industry-specific checks.
func GetIndustryAuditGroups(industry string) []keywordAuditGroup {
	if industry == "" {
		return nil
	}

	var groups []keywordAuditGroup

	switch industry {
	case "能源燃气":
		groups = append(groups, keywordAuditGroup{
			Name:     "燃气专项",
			Keywords: []string{"带气作业", "阴极保护", "防腐", "PE管", "气密性", "管道试压", "燃气报警"},
			Fields: []keywordAuditFieldCheck{
				{"专项作业要求", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.SpecialOperations.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.SpecialOperations.Value)
				}},
				{"施工技术规范", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.TechnicalSpecifications.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.TechnicalSpecifications.Value)
				}},
			},
		})
	case "市政":
		groups = append(groups, keywordAuditGroup{
			Name:     "市政专项",
			Keywords: []string{"交通导改", "管线迁改", "雨污分流", "非开挖", "顶管", "围挡", "沥青摊铺"},
			Fields: []keywordAuditFieldCheck{
				{"现场管理要求", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.SiteManagement.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.SiteManagement.Value)
				}},
				{"施工技术规范", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.TechnicalSpecifications.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.TechnicalSpecifications.Value)
				}},
			},
		})
	case "水利":
		groups = append(groups, keywordAuditGroup{
			Name:     "水利专项",
			Keywords: []string{"灌浆", "防渗", "围堰", "截流", "导流", "碾压混凝土", "帷幕灌浆"},
			Fields: []keywordAuditFieldCheck{
				{"专项作业要求", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.SpecialOperations.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.SpecialOperations.Value)
				}},
				{"施工技术规范", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.TechnicalSpecifications.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.TechnicalSpecifications.Value)
				}},
			},
		})
	case "房建":
		groups = append(groups, keywordAuditGroup{
			Name:     "房建专项",
			Keywords: []string{"基坑支护", "高支模", "深基坑", "大体积混凝土", "脚手架", "临边防护", "吊装"},
			Fields: []keywordAuditFieldCheck{
				{"专项作业要求", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.SpecialOperations.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.SpecialOperations.Value)
				}},
				{"现场管理要求", func(p *ProjectProfileResult) bool {
					return p.ConstructionCoreRequirements.SiteManagement.Missing || isEmptyProjectProfileValue(p.ConstructionCoreRequirements.SiteManagement.Value)
				}},
			},
		})
	}

	return groups
}

// GetIndustryPromptHint returns an industry-specific context string to prepend to extraction prompts.
func GetIndustryPromptHint(industry string) string {
	switch industry {
	case "能源燃气":
		return "本项目属于能源燃气行业。请重点关注：管道材质(PE/钢管)、带气作业要求、阴极保护、气密性试验、防腐标准(GB50028/CJJ63)、燃气安全规范等专业要求。"
	case "市政":
		return "本项目属于市政工程行业。请重点关注：交通导改方案、管线迁改、雨污分流、路基路面标准(CJJ1)、非开挖/顶管工艺、围挡施工、市政管网标准等专业要求。"
	case "水利":
		return "本项目属于水利水电行业。请重点关注：灌浆防渗工艺、围堰/截流/导流方案、水利标准(SL/DL)、度汛要求、生态护坡、河道治理等专业要求。"
	case "房建":
		return "本项目属于房屋建筑行业。请重点关注：基坑支护方案、高支模/深基坑专项、大体积混凝土要求、建筑标准(GB50300/JGJ)、精装修标准、消防/暖通/电气安装要求。"
	default:
		return ""
	}
}
