package service

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"fmt"
	"log"
	"strings"
	"sync"
)

type FileArchiveService struct {
	db              *sqlx.DB
	personArchiveMu sync.Mutex
}

type ExtractionItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	SourcePage string  `json:"source_page"`
}

func NewFileArchiveService(db *sqlx.DB) *FileArchiveService {
	return &FileArchiveService{db: db}
}

// ArchiveToLibrary distributes audited data to the respective business tables
func (s *FileArchiveService) ArchiveToLibrary(companyID string, libraryType string, data string, fileID string) (string, error) {
	log.Printf("[FileArchiveService] Archiving to %s (company %s, file %s)", libraryType, companyID, fileID)
	
	var items []ExtractionItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		log.Printf("[FileArchiveService] Failed to unmarshal items: %v", err)
	}

	switch libraryType {
	case "person":
		return s.archiveToPerson(companyID, items, fileID)
	case "qualification":
		return s.archiveToQualification(companyID, items, fileID)
	case "performance":
		return s.archiveToPerformance(companyID, items, fileID)
	case "laborcontract":
		return s.archiveToKnowledge(companyID, "laborcontract", items, fileID)
	default:
		return s.archiveToKnowledge(companyID, libraryType, items, fileID)
	}
}

func (s *FileArchiveService) findValue(items []ExtractionItem, keywords ...string) string {
	for _, item := range items {
		for _, kw := range keywords {
			if strings.Contains(item.Title, kw) || strings.Contains(item.Content, kw) {
				// If the title matches exactly or contains the keyword, this might be our value
				// In simple extraction, title is the label and content is the value
				if strings.Contains(item.Title, kw) {
					return item.Content
				}
			}
		}
	}
	return ""
}

// findValuePreferTitles 按 titles 的先后顺序匹配条目 title（优先「类别」再「职称」等）。
func (s *FileArchiveService) findValuePreferTitles(items []ExtractionItem, titles ...string) string {
	for _, want := range titles {
		for _, item := range items {
			if strings.Contains(item.Title, want) {
				return item.Content
			}
		}
	}
	return ""
}

// findValuesByTitleKeywords returns all non-empty contents whose title matches any keyword.
func (s *FileArchiveService) findValuesByTitleKeywords(items []ExtractionItem, keywords ...string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		content := strings.TrimSpace(item.Content)
		if title == "" || content == "" {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(title, kw) {
				if !seen[content] {
					seen[content] = true
					out = append(out, content)
				}
				break
			}
		}
	}
	return out
}

func (s *FileArchiveService) archiveToPerson(companyID string, items []ExtractionItem, fileID string) (string, error) {
	s.personArchiveMu.Lock()
	defer s.personArchiveMu.Unlock()

	var personGroups [][]ExtractionItem
	var currentGroup []ExtractionItem
	
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if (strings.Contains(title, "姓名") || strings.Contains(title, "持证人") || strings.Contains(title, "人员")) && len(currentGroup) > 0 {
			personGroups = append(personGroups, currentGroup)
			currentGroup = []ExtractionItem{}
		}
		currentGroup = append(currentGroup, item)
	}
	if len(currentGroup) > 0 {
		personGroups = append(personGroups, currentGroup)
	}

	var lastPersonID string
	for _, group := range personGroups {
		name := s.findValuePreferTitles(group, "姓名", "持证人", "人员")
		if name == "" {
			continue
		}
		
		idCard := s.findValuePreferTitles(group, "身份证号", "证件号", "身份证")
		qualType := s.findValuePreferTitles(group, "资格类型", "类别", "资格")
		certType := s.findValuePreferTitles(group, "证书类别", "职务", "职称")
		specialty := s.findValuePreferTitles(group, "专业", "注册专业", "执业专业")
		certNo := s.findValuePreferTitles(group, "证书编号", "证号", "编号")
		regNo := s.findValuePreferTitles(group, "注册编号", "执业印章号")
		regDate := s.findValuePreferTitles(group, "注册时间", "注册日期")
		expiryDate := s.findValuePreferTitles(group, "有效期截止", "有效期", "到期日")
		issuer := s.findValuePreferTitles(group, "颁发单位", "发证机关", "发证机构")
		
		finalQualType := qualType
		finalCertType := certType
		finalQualLevel := ""

		if strings.Contains(name, "安全生产许可证") || strings.Contains(qualType, "安全生产许可证") || strings.Contains(certType, "安全生产许可证") {
			finalQualType = "安全生产许可证"
			finalCertType = "安全生产许可证"
			finalQualLevel = "安全生产许可证"
		} else if strings.Contains(name, "化工装置拆除施工企业安全服务能力等级证书") || strings.Contains(qualType, "化工装置拆除施工企业安全服务能力等级证书") {
			prefix := "化工装置拆除施工企业安全服务能力等级证书"
			levelSuffix := ""
			if certType != "" && certType != prefix {
				levelSuffix = certType
			} else if qualType != prefix {
				levelSuffix = qualType
			}
			finalQualType = prefix
			finalCertType = prefix
			if levelSuffix != "" {
				finalQualLevel = prefix + "-" + levelSuffix
			} else {
				finalQualLevel = prefix
			}
		}

		var existingPersonID string
		if idCard != "" {
			_ = s.db.Get(&existingPersonID, "SELECT id FROM person WHERE company_id = ? AND (id_card_no = ? OR name = ?) LIMIT 1", companyID, idCard, name)
		} else {
			_ = s.db.Get(&existingPersonID, "SELECT id FROM person WHERE company_id = ? AND name = ? LIMIT 1", companyID, name)
		}

		var personID string
		if existingPersonID != "" {
			personID = existingPersonID
			_, err := s.db.Exec(`
				UPDATE person 
				SET 
					id_card_no = COALESCE(NULLIF(id_card_no, ''), ?),
					role_type = COALESCE(NULLIF(role_type, ''), ?),
					specialty = COALESCE(NULLIF(specialty, ''), ?),
					registration_no = COALESCE(NULLIF(registration_no, ''), ?),
					reg_date = COALESCE(NULLIF(reg_date, ''), ?),
					expiry_date = COALESCE(NULLIF(expiry_date, ''), ?),
					issuing_authority = COALESCE(NULLIF(issuing_authority, ''), ?)
				WHERE id = ?
			`, idCard, finalQualType, specialty, regNo, regDate, expiryDate, issuer, personID)
			s.db.Exec("UPDATE person SET role_type = ? WHERE id = ? AND (role_type IS NULL OR role_type = '')", finalQualType, personID)
			if err != nil {
				log.Printf("[FileArchiveService] Person Merge UPDATE failed: %v", err)
			}
		} else {
			var maxSuffix int
			s.db.Get(&maxSuffix, "SELECT COALESCE(MAX(CAST(SUBSTR(id, LENGTH(?)+3) AS INTEGER)), 0) FROM person WHERE company_id = ?", companyID, companyID)
			personID = fmt.Sprintf("%s-P%d", companyID, maxSuffix+1)
			
			_, err := s.db.Exec(`
				INSERT INTO person (id, company_id, name, id_card_no, role_type, specialty, registration_no, reg_date, expiry_date, issuing_authority, bid_usable_status, on_job_status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'usable', 'active', CURRENT_TIMESTAMP)
			`, personID, companyID, name, idCard, finalQualType, specialty, regNo, regDate, expiryDate, issuer)
			if err != nil {
				log.Printf("[FileArchiveService] Person INSERT failed: %v", err)
				continue
			}
		}
		lastPersonID = personID

		if fileID != "" {
			certName := finalCertType
			if certName == "" { certName = finalQualType }
			if certName == "" { certName = "人证" }
			
			var existingCertID string
			if certNo != "" {
				_ = s.db.Get(&existingCertID, "SELECT id FROM qualification WHERE owner_type = 'person' AND owner_id = ? AND certificate_no = ? LIMIT 1", personID, certNo)
			} else {
				_ = s.db.Get(&existingCertID, "SELECT id FROM qualification WHERE owner_type = 'person' AND owner_id = ? AND qualification_name = ? LIMIT 1", personID, certName)
			}

			if existingCertID != "" {
				s.db.Exec("UPDATE qualification SET file_asset_id = ?, valid_to = COALESCE(NULLIF(valid_to, ''), ?), specialty = COALESCE(NULLIF(specialty, ''), ?) WHERE id = ?", fileID, expiryDate, specialty, existingCertID)
			} else {
				certID := uuid.New().String()
				s.db.Exec(`
					INSERT INTO qualification (
						id, company_id, qualification_name, qualification_type, qualification_level, specialty,
						certificate_no, registration_no, issuing_authority, valid_to,
						file_asset_id, owner_type, owner_id, created_at
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'person', ?, CURRENT_TIMESTAMP)
				`, certID, companyID, certName, finalQualType, finalQualLevel, specialty, certNo, regNo, issuer, expiryDate, fileID, personID)
			}
		}
	}
	return lastPersonID, nil
}

func (s *FileArchiveService) archiveToQualification(companyID string, items []ExtractionItem, fileID string) (string, error) {
	var currentItems []ExtractionItem
	seen := make(map[string]bool)
	var firstCreatedID string

	saveRow := func(row []ExtractionItem) {
		name := s.findValuePreferTitles(row, "证书名称", "资质名称", "证照名称", "名称")
		number := s.findValuePreferTitles(row, "证书编号", "编号", "号码", "统一社会信用代码", "社会信用代码", "NO")
		levels := s.findValuesByTitleKeywords(row, "资质等级/类别", "资质等级", "等级", "类别")
		validFrom := s.findValue(row, "颁发/起始日期", "起始日期", "颁发日期", "发证日期", "生效日期")
		validTo := s.findValue(row, "有效期至", "有效期", "到期", "截止日期")
		issuer := s.findValue(row, "发证机关", "颁发机关", "颁发单位", "发证单位")
		if name == "" && len(levels) == 0 { return }
		if len(levels) == 0 { levels = []string{""} }

		for _, level := range levels {
			finalName := name
			if finalName == "" { finalName = "资质证书" }
			finalLevel := level
			if strings.Contains(finalName, "安全生产许可证") || strings.Contains(level, "安全生产许可证") {
				finalLevel = "安全生产许可证"
				if !strings.Contains(finalName, "安全生产许可证") { finalName = "安全生产许可证" }
			}

			id := uuid.New().String()
			if firstCreatedID == "" { firstCreatedID = id }
			s.db.Exec(`
				INSERT INTO qualification (
					id, company_id, qualification_name, certificate_no,
					qualification_level, qualification_type, valid_from, valid_to, issuing_authority,
					owner_type, owner_id, file_asset_id, created_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'company', ?, ?, CURRENT_TIMESTAMP)
			`, id, companyID, finalName, number, finalLevel, finalLevel, validFrom, validTo, issuer, companyID, fileID)
		}
	}

	for _, item := range items {
		if seen[item.Title] && (item.Title == "名称" || item.Title == "资质" || item.Title == "证书名称") {
			saveRow(currentItems)
			currentItems = nil
			seen = make(map[string]bool)
		}
		currentItems = append(currentItems, item)
		seen[item.Title] = true
	}
	saveRow(currentItems)
	return firstCreatedID, nil
}

func (s *FileArchiveService) archiveToPerformance(companyID string, items []ExtractionItem, fileID string) (string, error) {
	var currentItems []ExtractionItem
	seen := make(map[string]bool)

	saveRow := func(row []ExtractionItem) {
		projectName := s.findValue(row, "项目名称", "工程名称", "业绩")
		amount := s.findValue(row, "金额", "大小", "造价")
		manager := s.findValue(row, "项目经理", "负责人", "经理", "建造师")
		date := s.findValue(row, "竣工日期", "完成日期", "日期", "竣工")
		period := s.findValue(row, "工期", "日历天", "施工工期", "建设周期")
		if projectName == "" { return }
		id := uuid.New().String()
		s.db.Exec(`
			INSERT INTO project_performance (
				id, company_id, project_name, bid_amount_value, 
				project_manager_name, completion_date, construction_period, 
				file_asset_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, id, companyID, projectName, amount, manager, date, period, fileID)
	}

	for _, item := range items {
		if seen[item.Title] && (item.Title == "项目名称" || item.Title == "业绩名称") {
			saveRow(currentItems)
			currentItems = nil
			seen = make(map[string]bool)
		}
		currentItems = append(currentItems, item)
		seen[item.Title] = true
	}
	saveRow(currentItems)
	
	var lastPid string
	_ = s.db.Get(&lastPid, "SELECT id FROM project_performance WHERE file_asset_id = ? ORDER BY created_at DESC LIMIT 1", fileID)
	return lastPid, nil
}

// UpdatePerformanceFromExtraction updates an existing performance record using AI extracted fields.
// It maps: 项目名称 -> project_name, 发包方/业主 -> owner_org, 项目经理 -> project_manager_name,
// 技术负责人 -> technical_leader_name, 工期 -> construction_period, 项目概况 -> scale_desc.
func (s *FileArchiveService) UpdatePerformanceFromExtraction(companyID, performanceID, data string) error {
	log.Printf("[FileArchiveService] Updating performance %s from extraction", performanceID)
	
	var items []ExtractionItem
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		return err
	}

	projectName := s.findValue(items, "项目名称", "工程名称")
	ownerOrg := s.findValue(items, "发包方", "业主", "建设单位", "甲方")
	pmName := s.findValue(items, "项目经理", "建造师", "负责人")
	techLeader := s.findValue(items, "技术负责人", "项目总工", "总工程师")
	period := s.findValue(items, "工期", "建设周期", "施工工期")
	summary := s.findValue(items, "项目概况", "项目概况", "建设内容", "规模", "指标")

	// Create update map to only update non-empty fields
	updates := make(map[string]interface{})
	if projectName != "" { updates["project_name"] = projectName }
	if ownerOrg != "" { updates["owner_org"] = ownerOrg }
	if pmName != "" { updates["project_manager_name"] = pmName }
	if techLeader != "" { updates["technical_leader_name"] = techLeader }
	if period != "" { updates["construction_period"] = period }
	if summary != "" { updates["scale_desc"] = summary }

	if len(updates) == 0 {
		return nil
	}

	query := "UPDATE project_performance SET "
	args := []interface{}{}
	i := 0
	for k, v := range updates {
		if i > 0 { query += ", " }
		query += fmt.Sprintf("%s = ?", k)
		args = append(args, v)
		i++
	}
	query += " WHERE id = ? AND company_id = ?"
	args = append(args, performanceID, companyID)

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *FileArchiveService) archiveToKnowledge(companyID string, itemType string, items []ExtractionItem, fileID string) (string, error) {
	var firstID string
	for _, item := range items {
		id := uuid.New().String()
		if firstID == "" { firstID = id }
		_, err := s.db.Exec(`
			INSERT INTO tech_bid_knowledge_items (id, company_id, item_type, item_name, item_content, file_asset_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, id, companyID, itemType, item.Title, item.Content, fileID)
		if err != nil {
			log.Printf("[FileArchiveService] Knowledge insert failed: %v", err)
		}
	}
	return "knowledge:" + itemType + ":" + firstID, nil 
}
