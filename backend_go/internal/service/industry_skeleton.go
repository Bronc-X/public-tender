package service

import (
	"backend_go/internal/model"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// IndustrySkeleton defines the logical chapters and common section pools for a specific industry.
type IndustrySkeleton struct {
	IndustryName       string
	LogicalChapters    []LogicalChapter
	CommonSectionPool  []string
	IndustryKeywords   []string
	TitleCandidatePool map[string][]string // Map chapter/section categories to synonym candidates for differentiation
}

type LogicalChapter struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	IsMandatory    bool                `json:"is_mandatory"`
	UnitPool       []string            `json:"unit_pool"`       // Level 2 sections
	SubsectionPool map[string][]string `json:"subsection_pool"` // Level 3 subsections (mapped from L2 name)
	// Elasticity Controls
	IsCoreChapter      bool     `json:"is_core_chapter"`
	CanReorder         bool     `json:"can_reorder"`
	CanSplit           bool     `json:"can_split"`
	CanMerge           bool     `json:"can_merge"`
	CanInsertBefore    bool     `json:"can_insert_before"`
	CanInsertAfter     bool     `json:"can_insert_after"`
	PriorityRange      []string `json:"priority_range"`
	FactTypePreference []string `json:"fact_type_preference"`
}

// ResolveIndustryLabel maps project_type / profession 文案到骨架表 industry_name。
func ResolveIndustryLabel(projectType, profession string) string {
	combined := strings.ToLower(strings.TrimSpace(projectType) + " " + strings.TrimSpace(profession))
	if combined == "" {
		return ""
	}
	// 增加对通用名词的映射，对齐数据库中的冗长命名
	if strings.Contains(combined, "水利") || strings.Contains(combined, "河道") || strings.Contains(combined, "堤防") ||
		strings.Contains(combined, "坝垛") || strings.Contains(combined, "水务") || strings.Contains(combined, "水工") ||
		strings.Contains(combined, "航道") {
		return "水利"
	}
	if strings.Contains(combined, "房建") || strings.Contains(combined, "房屋建筑") || strings.Contains(combined, "住宅") ||
		strings.Contains(combined, "公建") || strings.Contains(combined, "建筑工程") {
		return "房建"
	}
	if strings.Contains(combined, "公路") || strings.Contains(combined, "路基") || strings.Contains(combined, "路面") ||
		strings.Contains(combined, "高速") || strings.Contains(combined, "省道") {
		return "公路"
	}
	if strings.Contains(combined, "市政") || strings.Contains(combined, "管网") || strings.Contains(combined, "顶管") ||
		strings.Contains(combined, "城镇") || strings.Contains(combined, "给排水") {
		return "市政"
	}
	if strings.Contains(combined, "铁路") || strings.Contains(combined, "轨道") || strings.Contains(combined, "站房") {
		return "铁路"
	}
	if strings.Contains(combined, "电力") || strings.Contains(combined, "输变电") || strings.Contains(combined, "电网") {
		return "电力"
	}
	return ""
}

// LoadIndustrySkeletonDB returns the raw database row for structural planning.
func (s *TenderDigitizationService) LoadIndustrySkeletonDB(projectType, profession string) (*model.IndustrySkeletonDB, error) {
	kw := ResolveIndustryLabel(derefSkeletonStr(projectType), derefSkeletonStr(profession))
	var dbRow model.IndustrySkeletonDB

	// 1. 尝试关键词匹配（最强匹配）
	if kw != "" {
		// 模糊搜索：包含该行业关键字（如“房建”），且属于工程模版
		query := "SELECT * FROM tech_bid_industry_skeletons WHERE (industry_name LIKE ? OR industry_keywords_json LIKE ?) AND parent_id IS NOT NULL LIMIT 1"
		err := s.db.Get(&dbRow, query, "%"+kw+"%", "%"+kw+"%")
		if err == nil {
			return &dbRow, nil
		}
		// 再次尝试，不带 parent_id 限制
		err = s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE (industry_name LIKE ? OR industry_keywords_json LIKE ?) LIMIT 1", "%"+kw+"%", "%"+kw+"%")
		if err == nil {
			return &dbRow, nil
		}
	}

	// 2. 尝试 projectType 全文模糊匹配
	industry := derefSkeletonStr(projectType)
	if industry != "" {
		err := s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE LOWER(industry_name) LIKE ? AND parent_id IS NOT NULL LIMIT 1", "%"+strings.ToLower(industry)+"%")
		if err != nil {
			err = s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE LOWER(industry_name) LIKE ? LIMIT 1", "%"+strings.ToLower(industry)+"%")
		}
		if err == nil {
			return &dbRow, nil
		}
	}

	// 3. 回退到通用工程
	err := s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE industry_name = '通用工程' LIMIT 1")
	if err == nil {
		return &dbRow, nil
	}

	return nil, fmt.Errorf("no skeleton found")
}

// LoadIndustrySkeleton 先按画像关键词精确命中行业骨架，否则回退到 GetSkeletonByIndustry 的模糊匹配。
func (s *TenderDigitizationService) LoadIndustrySkeleton(projectType, profession string) *IndustrySkeleton {
	dbRow, err := s.LoadIndustrySkeletonDB(projectType, profession)
	if err == nil {
		return s.mapDBToSkeleton(*dbRow)
	}
	return generalEngineeringSkeleton()
}

func derefSkeletonStr(s string) string {
	return strings.TrimSpace(s)
}

// GetSkeletonByIndustry returns the pre-defined skeleton for a given industry from DB.
func (s *TenderDigitizationService) GetSkeletonByIndustry(industry string) *IndustrySkeleton {
	industry = strings.ToLower(industry)

	var dbRow model.IndustrySkeletonDB
	// 优先查询二级分类
	err := s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE LOWER(industry_name) LIKE ? AND parent_id IS NOT NULL LIMIT 1", "%"+industry+"%")
	if err != nil {
		// 再次尝试一级分类
		err = s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE LOWER(industry_name) LIKE ? LIMIT 1", "%"+industry+"%")
	}

	if err != nil {
		// Try to find a general one if specific not found
		err = s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE industry_name = '通用工程' LIMIT 1")
		if err != nil {
			log.Printf("[Skeleton] No skeleton found in DB for industry '%s', using hardcoded fallback", industry)
			return generalEngineeringSkeleton()
		}
	}

	return s.mapDBToSkeleton(dbRow)
}

// LoadSkeletonByID loads a specific skeleton by its ID (used for user-confirmed skeleton selection)
func (s *TenderDigitizationService) LoadSkeletonByID(skeletonID string) (*model.IndustrySkeletonDB, error) {
	var dbRow model.IndustrySkeletonDB
	err := s.db.Get(&dbRow, "SELECT * FROM tech_bid_industry_skeletons WHERE id = ?", skeletonID)
	if err != nil {
		log.Printf("[Skeleton] LoadSkeletonByID failed for ID %s: %v", skeletonID, err)
		return nil, err
	}
	return &dbRow, nil
}

func (s *TenderDigitizationService) mapDBToSkeleton(dbRow model.IndustrySkeletonDB) *IndustrySkeleton {
	var chapters []LogicalChapter
	json.Unmarshal([]byte(dbRow.LogicalChaptersJSON), &chapters)

	var commonPool []string
	if dbRow.CommonSectionPoolJSON != nil {
		json.Unmarshal([]byte(*dbRow.CommonSectionPoolJSON), &commonPool)
	}

	var keywords []string
	if dbRow.IndustryKeywordsJSON != nil {
		json.Unmarshal([]byte(*dbRow.IndustryKeywordsJSON), &keywords)
	}

	var titlePool map[string][]string
	if dbRow.TitleCandidatePoolJSON != nil {
		json.Unmarshal([]byte(*dbRow.TitleCandidatePoolJSON), &titlePool)
	}

	return &IndustrySkeleton{
		IndustryName:       dbRow.IndustryName,
		LogicalChapters:    chapters,
		CommonSectionPool:  commonPool,
		IndustryKeywords:   keywords,
		TitleCandidatePool: titlePool,
	}
}

// InitDefaultSkeletons migrations hardcoded data to DB if empty
func InitDefaultSkeletons(db *sqlx.DB) {
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM tech_bid_industry_skeletons")
	if err != nil || count > 0 {
		return
	}

	log.Println("[Skeleton] Migrating hardcoded skeletons to database...")
	defaults := []*IndustrySkeleton{
		waterConservancySkeleton(),
		buildingConstructionSkeleton(),
		highwayEngineeringSkeleton(),
		municipalEngineeringSkeleton(),
		generalEngineeringSkeleton(),
	}

	for _, sk := range defaults {
		chaptersJSON, _ := json.Marshal(sk.LogicalChapters)
		commonPoolJSON, _ := json.Marshal(sk.CommonSectionPool)
		keywordsJSON, _ := json.Marshal(sk.IndustryKeywords)
		titlePoolJSON, _ := json.Marshal(sk.TitleCandidatePool)

		cp := string(commonPoolJSON)
		kw := string(keywordsJSON)
		tp := string(titlePoolJSON)

		_, err := db.Exec(`INSERT INTO tech_bid_industry_skeletons 
			(id, industry_name, logical_chapters_json, common_section_pool_json, industry_keywords_json, title_candidate_pool_json)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), sk.IndustryName, string(chaptersJSON), &cp, &kw, &tp)
		if err != nil {
			log.Printf("[Skeleton] Failed to migrate %s: %v", sk.IndustryName, err)
		}
	}
}

func waterConservancySkeleton() *IndustrySkeleton {
	return &IndustrySkeleton{
		IndustryName: "水利工程",
		LogicalChapters: []LogicalChapter{
			{ID: "CH1", Name: "施工组织总体思路与规划", Description: "整体架构、编制依据、管理目标等", IsMandatory: true, UnitPool: []string{"编制说明", "工程概况", "施工组织总体策划"}},
			{ID: "CH2", Name: "施工部署与资源配置", Description: "现场布置、人员清单、机械配置、材料计划", IsMandatory: true, UnitPool: []string{"施工现场平面设计", "主要管理人员配置", "劳动力计划", "施工机械与物资准备"}},
			{ID: "CH3", Name: "主要施工方案与工艺技术", Description: "河道、堤防、控导、险工等关键方案", IsMandatory: true, UnitPool: []string{"导流与围堰工程", "土石方工程", "砌石与混凝土施工", "堤防与河道治理", "险工加固专项"}},
			{ID: "CH4", Name: "关键工序与专项工艺控制", Description: "冬雨季施工、围堰排水、特殊地质处理", IsMandatory: false, UnitPool: []string{"雨季施工专项", "冬期施工专项", "软基处理技术", "智慧工地系统应用"}},
			{ID: "CH5", Name: "质量控制保障与验收管理", Description: "质量目标、体系、验收节点、关键环节控制", IsMandatory: true, UnitPool: []string{"质量管理体系及措施", "关键工序质量验收", "材料检测与试验计划"}},
			{ID: "CH6", Name: "安全文明施工与环保水保", Description: "安全生产、文明工地、河道弃渣保护、水土保持", IsMandatory: true, UnitPool: []string{"安全生产管理体系及措施", "文明施工及扬尘治理", "水土保持与环境保护"}},
			{ID: "CH7", Name: "工期保障与风险防控应对", Description: "进度计划、资源保障、防汛应急、突发事件处理", IsMandatory: true, UnitPool: []string{"进度计划与保障措施", "防汛应急预案", "突发公共卫生事件应对"}},
		},
		CommonSectionPool: []string{"控导工程", "坝垛施工", "护坡工程", "土工格栅铺设", "河道疏浚", "闸门安装"},
		IndustryKeywords:  []string{"堤防", "控导", "坝垛", "险工", "河道", "水生生态", "围堰", "围垦"},
		TitleCandidatePool: map[string][]string{
			"CH1": {"施工组织设计总体框架", "总体施工组织策划与概况", "施工方案总体说明"},
			"CH3": {"施工技术方案与核心工艺", "主要分部工程施工方法", "核心分项工程技术方案"},
			"CH5": {"质量管理评价与验收机制", "质量保证体系及各阶段控制", "工程质量创优策划方案"},
		},
	}
}

func buildingConstructionSkeleton() *IndustrySkeleton {
	return &IndustrySkeleton{
		IndustryName: "房建工程",
		LogicalChapters: []LogicalChapter{
			{ID: "CH1", Name: "项目概况与管理部署", Description: "编制依据、现场概况、项目管理目标", IsMandatory: true, UnitPool: []string{"工程简述", "管理目标", "施工组织总体安排"}},
			{ID: "CH2", Name: "施工平面布置与资源配置", Description: "塔吊布置、临水临电、人员与物资保障", IsMandatory: true, UnitPool: []string{"施工总平面图布置", "资源投入计划", "周转材料配备"}},
			{ID: "CH3", Name: "主要施工方案与工艺流程", Description: "地基、主体结构、外墙、防水、二次结构", IsMandatory: true, UnitPool: []string{"深基坑支护及降水", "主体结构工程", "二次结构与装饰装修", "建筑防水工程"}},
			{ID: "CH4", Name: "质量重点监控与成品保护", Description: "结构实测实量、样板引路、防渗漏防治", IsMandatory: true, UnitPool: []string{"质量管理节点控制", "样板先行制度", "成品保护专项方案"}},
			{ID: "CH5", Name: "安全文明施工与绿色施工", Description: "智慧工地、消防安全、外墙防坠落、降尘降噪", IsMandatory: true, UnitPool: []string{"安全生产保障措施", "文明施工管理体系", "环境保护与绿色建筑"}},
			{ID: "CH6", Name: "工程总承包与专业分包配合", Description: "多专业交叉施工、机电预留、电梯安装配合", IsMandatory: false, UnitPool: []string{"总分包协调管理", "安装工程配合措施"}},
			{ID: "CH7", Name: "季节性施工与应急保障", Description: "雨季、暑期、冬季、极端天气响应", IsMandatory: true, UnitPool: []string{"季节性施工措施", "应急预案及防汛防台"}},
		},
		TitleCandidatePool: map[string][]string{
			"CH3": {"主要施工方法与核心技术方案", "各分部分项工程施工方案", "施工工艺控制点与流程规划"},
		},
	}
}

func highwayEngineeringSkeleton() *IndustrySkeleton {
	return &IndustrySkeleton{
		IndustryName: "公路工程",
		LogicalChapters: []LogicalChapter{
			{ID: "CH1", Name: "总体施工组织布置与规划", IsMandatory: true, UnitPool: []string{"编制依据", "总体施工安排"}},
			{ID: "CH2", Name: "主要工程施工方案与方法", IsMandatory: true, UnitPool: []string{"路基工程", "路面底层施工", "桥涵构造物", "隧道施工方案"}},
			{ID: "CH3", Name: "工期进度保证体系与措施", IsMandatory: true, UnitPool: []string{"年度计划安排", "进度延误追赶措施"}},
			{ID: "CH4", Name: "质量保证体系与交竣工验收", IsMandatory: true, UnitPool: []string{"原材料控制", "主要试验检测计划"}},
			{ID: "CH5", Name: "安全文明、环保水保与保通方案", IsMandatory: true, UnitPool: []string{"临时场站建设", "既有线交通保通", "水土保持专项"}},
		},
	}
}

func municipalEngineeringSkeleton() *IndustrySkeleton {
	return &IndustrySkeleton{
		IndustryName: "市政工程",
		LogicalChapters: []LogicalChapter{
			{ID: "CH1", Name: "项目总体认识与部署", IsMandatory: true},
			{ID: "CH2", Name: "道路、管网、桥梁主要工艺方案", IsMandatory: true, UnitPool: []string{"土路床整理", "顶管专项工艺", "高架支架施工"}},
			{ID: "CH3", Name: "地上地下公用设施保护专项", IsMandatory: true, UnitPool: []string{"地下管线探测与保护", "交通导流方案"}},
			{ID: "CH4", Name: "质量、安全及文明施工规范", IsMandatory: true},
		},
	}
}

func generalEngineeringSkeleton() *IndustrySkeleton {
	return &IndustrySkeleton{
		IndustryName: "通用工程",
		LogicalChapters: []LogicalChapter{
			{ID: "CH1", Name: "施工组织总体策划", IsMandatory: true, UnitPool: []string{"项目理解与总体分析", "管理目标与承诺", "施工组织总体思路"}},
			{ID: "CH2", Name: "施工部署与资源清单", IsMandatory: true, UnitPool: []string{"施工现场平面布置", "劳动力配置计划", "主要物资与机械设备计划"}},
			{ID: "CH3", Name: "分部分项工程主要施工方案", IsMandatory: true, UnitPool: []string{"土石方工程专项方案", "主体结构工艺流程", "关键部位专项施工方案"}},
			{ID: "CH4", Name: "质量目标与保证体系", IsMandatory: true, UnitPool: []string{"质量管理体系及制度", "质量通病预防与处理", "成品保护专项措施"}},
			{ID: "CH5", Name: "安全管理与文明施工规程", IsMandatory: true, UnitPool: []string{"安全生产管理体系", "施工现场文明工地建设", "环境保护与扬尘控制"}},
			{ID: "CH6", Name: "工期保障与应急处置预案", IsMandatory: true, UnitPool: []string{"施工总进度计划保证", "防汛防台及极端天气应对", "应急演练与突发事件处置"}},
		},
	}
}
