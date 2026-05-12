package service

import (
	"backend_go/internal/model"
	"strings"

	"github.com/jmoiron/sqlx"
	"log"
)

type FileReviewService struct {
	db *sqlx.DB
}

func NewFileReviewService(db *sqlx.DB) *FileReviewService {
	return &FileReviewService{db: db}
}

func (s *FileReviewService) GetAuditDetail(id string, companyID string) (*model.AuditDetail, error) {
	var detail model.AuditDetail
	query := `
		SELECT 
            a.id, 
            COALESCE(f.file_name, '') as file_name, 
            COALESCE(f.mime_type, '') as mime_type, 
            COALESCE(f.stored_path, '') as stored_path, 
            a.object_type, 
            a.audit_status,
            COALESCE(a.ocr_text, '') as ocr_text,
            COALESCE(a.ai_clean_text, '') as ai_clean_text,
            a.extracted_data_json as extracted_data,
            a.file_id as file_id,
            a.reviewer_id,
            a.reviewer_name,
            a.archive_target_type,
            a.archive_target_id
		FROM audit_item a
		LEFT JOIN file_asset f ON a.file_id = f.id
		WHERE a.id = ? AND (a.company_id = ? OR f.company_id = ?)
	`
	err := s.db.Get(&detail, query, id, companyID, companyID)
	if err != nil {
		log.Printf("[FileReviewService] GetAuditDetail error: %v", err)
		return nil, err
	}
	// 统一把模型常用的「详细信息」「内容」等键并入 content；人员档案再跑固定 6 类目。与库不一致则回写，修复历史数据。
	updated := CoerceExtractionJSONArray(detail.ExtractedData)
	if strings.EqualFold(strings.TrimSpace(detail.ObjectType), "person") {
		updated = NormalizePersonExtractedDataJSON(updated)
	}
	if updated != detail.ExtractedData {
		if _, uerr := s.db.Exec(`UPDATE audit_item SET extracted_data_json = ? WHERE id = ?`, updated, id); uerr != nil {
			log.Printf("[FileReviewService] persist coerced extraction failed: %v", uerr)
		} else {
			log.Printf("[FileReviewService] persisted coerced extraction for audit %s", id)
		}
		detail.ExtractedData = updated
	}
	return &detail, nil
}

func (s *FileReviewService) UpdateAuditStatus(id string, status string) error {
	_, err := s.db.Exec("UPDATE audit_item SET audit_status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}
