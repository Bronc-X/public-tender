package service

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"backend_go/internal/model"
)

// SkeletonCandidate 骨架候选结果
type SkeletonCandidate struct {
	SkeletonID     string           `json:"skeleton_id"`
	IndustryName   string           `json:"industry_name"`
	MatchScore     float64          `json:"match_score"` // 总分 0-100
	ScoreBreakdown ScoreBreakdown   `json:"score_breakdown"`
	MatchReasons   []string         `json:"match_reasons"`
	Confidence     string           `json:"confidence"` // high, medium, low
	Recommended    bool             `json:"recommended"`
	SkeletonData   *SkeletonSummary `json:"skeleton_data,omitempty"`
}

// ScoreBreakdown 四维度评分明细
type ScoreBreakdown struct {
	KeywordScore        float64 `json:"keyword_score"`         // 关键词匹配 30%
	StructureScore      float64 `json:"structure_score"`       // 结构相似度 30%
	SpecialChapterScore float64 `json:"special_chapter_score"` // 特殊章节匹配 25%
	HistoryScore        float64 `json:"history_score"`         // 历史相似度 15%
}

// SkeletonSummary 骨架摘要信息（用于前端展示）
type SkeletonSummary struct {
	IndustryName string   `json:"industry_name"`
	ChapterCount int      `json:"chapter_count"`
	Chapters     []string `json:"chapters"`
	Keywords     []string `json:"keywords"`
}

// SkeletonScoreWeights 评分权重配置
var SkeletonScoreWeights = struct {
	Keyword   float64
	Structure float64
	Special   float64
	History   float64
}{
	Keyword:   0.30,
	Structure: 0.30,
	Special:   0.25,
	History:   0.15,
}

// SkeletonScorer 骨架评分器
type SkeletonScorer struct {
	db         interface{} // *sqlx.DB - 暂时用 interface{}
	historySvc interface{} // 历史相似度服务
}

// NewSkeletonScorer 创建骨架评分器
func NewSkeletonScorer() *SkeletonScorer {
	return &SkeletonScorer{}
}

// ProjectProfile 项目画像（用于骨架匹配）
type ProjectProfile struct {
	ProjectType  string   // 项目类型：水利、房建、公路、市政等
	Profession   string   `json:"profession"`
	Keywords     []string `json:"keywords"`      // 从招标文件中提取的关键词
	ChapterCount int      `json:"chapter_count"` // 招标文件章节数
	HasSpecials  []string `json:"has_specials"`  // 特殊章节标识
}

// ScoreSkeletons 对候选骨架列表进行评分排序
func (s *SkeletonScorer) ScoreSkeletons(skeletons []model.IndustrySkeletonDB, profile ProjectProfile) []SkeletonCandidate {
	candidates := make([]SkeletonCandidate, 0, len(skeletons))

	for _, sk := range skeletons {
		candidate := s.scoreSingleSkeleton(sk, profile)
		candidates = append(candidates, candidate)
	}

	// 按总分排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].MatchScore > candidates[j].MatchScore
	})

	// 标记推荐项（最高分）
	if len(candidates) > 0 {
		candidates[0].Recommended = true
	}

	return candidates
}

// scoreSingleSkeleton 对单个骨架评分
func (s *SkeletonScorer) scoreSingleSkeleton(sk model.IndustrySkeletonDB, profile ProjectProfile) SkeletonCandidate {
	breakdown := ScoreBreakdown{}

	// 1. 关键词匹配评分 (30%)
	breakdown.KeywordScore = s.scoreKeywordMatch(sk, profile)

	// 2. 结构相似度评分 (30%)
	breakdown.StructureScore = s.scoreStructureMatch(sk, profile)

	// 3. 特殊章节匹配评分 (25%)
	breakdown.SpecialChapterScore = s.scoreSpecialChapterMatch(sk, profile)

	// 4. 历史相似度评分 (15%)
	breakdown.HistoryScore = s.scoreHistoryMatch(sk, profile)

	// 计算总分
	totalScore := breakdown.KeywordScore*SkeletonScoreWeights.Keyword +
		breakdown.StructureScore*SkeletonScoreWeights.Structure +
		breakdown.SpecialChapterScore*SkeletonScoreWeights.Special +
		breakdown.HistoryScore*SkeletonScoreWeights.History

	// 生成匹配理由
	reasons := s.generateMatchReasons(sk, profile, breakdown)

	// 确定置信度
	confidence := s.determineConfidence(breakdown)

	return SkeletonCandidate{
		SkeletonID:     sk.ID,
		IndustryName:   sk.IndustryName,
		MatchScore:     math.Round(totalScore*100) / 100,
		ScoreBreakdown: breakdown,
		MatchReasons:   reasons,
		Confidence:     confidence,
		SkeletonData:   s.extractSkeletonSummary(sk),
	}
}

// scoreKeywordMatch 关键词匹配评分 (满分100，占比30%)
func (s *SkeletonScorer) scoreKeywordMatch(sk model.IndustrySkeletonDB, profile ProjectProfile) float64 {
	if sk.IndustryKeywordsJSON == nil {
		// 回退：使用 industry_name 进行简单匹配
		name := strings.ToLower(sk.IndustryName)
		keywords := []string{
			"水利", "房建", "公路", "市政", "铁路", "电力", "通用",
		}
		for i, kw := range keywords {
			if strings.Contains(name, kw) {
				return 60.0 + float64(40-i*5) // 越靠前匹配度越高
			}
		}
		return 30.0 // 默认低分
	}

	var industryKeywords []string
	if err := json.Unmarshal([]byte(*sk.IndustryKeywordsJSON), &industryKeywords); err != nil {
		return 30.0
	}

	if len(industryKeywords) == 0 || len(profile.Keywords) == 0 {
		return 40.0
	}

	// 计算关键词重叠率
	matchCount := 0
	for _, profileKw := range profile.Keywords {
		profileKwLower := strings.ToLower(profileKw)
		for _, indKw := range industryKeywords {
			if strings.Contains(strings.ToLower(indKw), profileKwLower) ||
				strings.Contains(profileKwLower, strings.ToLower(indKw)) {
				matchCount++
				break
			}
		}
	}

	// Jaccard 相似度
	union := len(industryKeywords) + len(profile.Keywords) - matchCount
	if union == 0 {
		return 50.0
	}

	jaccard := float64(matchCount) / float64(union)
	return jaccard * 100
}

// scoreStructureMatch 结构相似度评分 (满分100，占比30%)
func (s *SkeletonScorer) scoreStructureMatch(sk model.IndustrySkeletonDB, profile ProjectProfile) float64 {
	var chapters []model.LogicalChapter
	if err := json.Unmarshal([]byte(sk.LogicalChaptersJSON), &chapters); err != nil {
		return 40.0
	}

	// 基础分：根据章节数量估算
	chapterCountScore := 50.0
	if len(chapters) >= 5 && len(chapters) <= 8 {
		chapterCountScore = 80.0 // 合理区间
	} else if len(chapters) > 8 {
		chapterCountScore = 70.0
	} else if len(chapters) > 3 {
		chapterCountScore = 60.0
	}

	// 项目章节数相似度
	profileChapterScore := 30.0
	if profile.ChapterCount > 0 {
		ratio := float64(len(chapters)) / float64(profile.ChapterCount)
		if ratio >= 0.7 && ratio <= 1.3 {
			profileChapterScore = 30.0
		} else if ratio >= 0.5 && ratio <= 1.5 {
			profileChapterScore = 20.0
		} else {
			profileChapterScore = 10.0
		}
	}

	return chapterCountScore + profileChapterScore
}

// scoreSpecialChapterMatch 特殊章节匹配评分 (满分100，占比25%)
func (s *SkeletonScorer) scoreSpecialChapterMatch(sk model.IndustrySkeletonDB, profile ProjectProfile) float64 {
	if len(profile.HasSpecials) == 0 {
		return 50.0 // 无特殊章节要求，默认中等
	}

	var chapters []model.LogicalChapter
	if err := json.Unmarshal([]byte(sk.LogicalChaptersJSON), &chapters); err != nil {
		return 30.0
	}

	matchCount := 0
	specialKeywords := map[string][]string{
		"隧道": {"隧道", "掘进", "盾构"},
		"桥梁": {"桥梁", "桥涵", "上部结构", "下部结构"},
		"基坑": {"基坑", "支护", "降水", "开挖"},
		"管网": {"管网", "管道", "顶管", "不开挖"},
		"电气": {"电气", "机电", "配电", "照明"},
		"绿化": {"绿化", "景观", "种植", "养护"},
		"防洪": {"防洪", "防汛", "度汛"},
		"航道": {"航道", "疏浚", "码头"},
	}

	for _, special := range profile.HasSpecials {
		specialLower := strings.ToLower(special)
		found := false
		for _, ch := range chapters {
			chLower := strings.ToLower(ch.Name)
			if strings.Contains(chLower, specialLower) {
				found = true
				break
			}
			// 检查同义词
			if syns, ok := specialKeywords[specialLower]; ok {
				for _, syn := range syns {
					if strings.Contains(chLower, syn) {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if found {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(profile.HasSpecials)) * 100
}

// scoreHistoryMatch 历史相似度评分 (满分100，占比15%)
func (s *SkeletonScorer) scoreHistoryMatch(sk model.IndustrySkeletonDB, profile ProjectProfile) float64 {
	// 简化实现：基于项目类型与骨架名称的匹配
	name := strings.ToLower(sk.IndustryName)

	// 行业类型映射
	typeMatches := map[string][]string{
		"水利": {"水利", "水务", "河道", "堤防", "大坝"},
		"房建": {"房建", "房屋", "建筑", "住宅", "公建"},
		"公路": {"公路", "道路", "路基", "路面", "高速"},
		"市政": {"市政", "管网", "给排水", "城镇"},
		"铁路": {"铁路", "轨道", "站场"},
		"电力": {"电力", "电网", "输变电"},
	}

	for projType, keywords := range typeMatches {
		if strings.ToLower(profile.ProjectType) == strings.ToLower(projType) {
			for _, kw := range keywords {
				if strings.Contains(name, kw) {
					return 90.0 // 直接匹配
				}
			}
		}
	}

	// Profession 匹配
	if profile.Profession != "" {
		profLower := strings.ToLower(profile.Profession)
		if strings.Contains(name, profLower) {
			return 80.0
		}
	}

	return 40.0 // 无明确匹配
}

// generateMatchReasons 生成匹配理由
func (s *SkeletonScorer) generateMatchReasons(sk model.IndustrySkeletonDB, profile ProjectProfile, breakdown ScoreBreakdown) []string {
	reasons := []string{}

	// 关键词匹配理由
	if breakdown.KeywordScore >= 70 {
		reasons = append(reasons, "行业关键词高度匹配")
	} else if breakdown.KeywordScore >= 50 {
		reasons = append(reasons, "行业关键词部分匹配")
	}

	// 结构匹配理由
	if breakdown.StructureScore >= 80 {
		reasons = append(reasons, "章节结构高度相似")
	} else if breakdown.StructureScore >= 60 {
		reasons = append(reasons, "章节数量与招标文件接近")
	}

	// 特殊章节理由
	if breakdown.SpecialChapterScore >= 80 {
		reasons = append(reasons, "涵盖所有特殊施工要求")
	} else if breakdown.SpecialChapterScore >= 50 && len(profile.HasSpecials) > 0 {
		reasons = append(reasons, "包含部分特殊章节")
	}

	// 历史匹配理由
	if breakdown.HistoryScore >= 80 {
		reasons = append(reasons, "项目类型与骨架行业一致")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "通用工程模板，可适配各类项目")
	}

	return reasons
}

// determineConfidence 确定置信度
func (s *SkeletonScorer) determineConfidence(breakdown ScoreBreakdown) string {
	avg := (breakdown.KeywordScore + breakdown.StructureScore + breakdown.SpecialChapterScore + breakdown.HistoryScore) / 4

	if avg >= 75 {
		return "high"
	} else if avg >= 55 {
		return "medium"
	}
	return "low"
}

// extractSkeletonSummary 提取骨架摘要
func (s *SkeletonScorer) extractSkeletonSummary(sk model.IndustrySkeletonDB) *SkeletonSummary {
	var chapters []model.LogicalChapter
	json.Unmarshal([]byte(sk.LogicalChaptersJSON), &chapters)

	chapterNames := make([]string, 0, len(chapters))
	for _, ch := range chapters {
		chapterNames = append(chapterNames, ch.Name)
	}

	var keywords []string
	if sk.IndustryKeywordsJSON != nil {
		json.Unmarshal([]byte(*sk.IndustryKeywordsJSON), &keywords)
	}

	return &SkeletonSummary{
		IndustryName: sk.IndustryName,
		ChapterCount: len(chapters),
		Chapters:     chapterNames,
		Keywords:     keywords,
	}
}

// GetAllSkeletonsForScoring 获取所有可用骨架用于评分
func (s *SkeletonScorer) GetAllSkeletonsForScoring(_ interface{}) ([]model.IndustrySkeletonDB, error) {
	// 简化实现：返回硬编码的默认骨架列表
	// 实际使用时应该从数据库查询（由 handler 层负责）
	return s.GetHardcodedSkeletons(), nil
}

// GetHardcodedSkeletons 获取硬编码的骨架列表（用于评分）
func (s *SkeletonScorer) GetHardcodedSkeletons() []model.IndustrySkeletonDB {
	return []model.IndustrySkeletonDB{
		{
			ID:                   "sk-water",
			IndustryName:         "水利工程",
			LogicalChaptersJSON:  `[]`, // 将在查询时填充
			IndustryKeywordsJSON: strPtr(`["堤防","控导","坝垛","险工","河道","水生生态","围堰","围垦","防洪","防汛"]`),
		},
		{
			ID:                   "sk-building",
			IndustryName:         "房建工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["房建","房屋建筑","住宅","公建","建筑工程","基坑","主体结构","装饰装修"]`),
		},
		{
			ID:                   "sk-highway",
			IndustryName:         "公路工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["公路","道路","路基","路面","高速","省道","桥涵","隧道"]`),
		},
		{
			ID:                   "sk-municipal",
			IndustryName:         "市政工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["市政","管网","给排水","城镇","道路","桥梁","顶管"]`),
		},
		{
			ID:                   "sk-railway",
			IndustryName:         "铁路工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["铁路","轨道","站场","隧道","桥梁"]`),
		},
		{
			ID:                   "sk-power",
			IndustryName:         "电力工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["电力","输变电","电网","配电","电气"]`),
		},
		{
			ID:                   "sk-general",
			IndustryName:         "通用工程",
			LogicalChaptersJSON:  `[]`,
			IndustryKeywordsJSON: strPtr(`["施工","组织","质量","安全","工期"]`),
		},
	}
}

func strPtr(s string) *string {
	return &s
}
