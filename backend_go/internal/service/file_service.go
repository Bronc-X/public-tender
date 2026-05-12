package service

import (
	"backend_go/internal/model"
	"github.com/jmoiron/sqlx"
	"log"
)

type FileService struct {
	db *sqlx.DB
}

func NewFileService(db *sqlx.DB) *FileService {
	return &FileService{db: db}
}

func (s *FileService) ListFiles(companyID string) ([]model.FileAsset, error) {
	var files = make([]model.FileAsset, 0)
	query := `
		SELECT 
			f.id, f.file_name, f.ext, f.mime_type, f.file_size, f.sha256, 
			f.source_path, f.stored_path, f.source_type, f.import_batch_id, 
			f.scan_status, f.parse_status, f.archive_status, f.company_id, 
			f.created_at, f.updated_at,
			f.source_module, f.source_project_id, f.last_task_id, f.last_error_message,
			f.scan_status as status,
			f.archive_target_type, f.archive_target_id,
			a.id as audit_id,
			a.object_type as object_type,
			COALESCE(c.plain_text, '') as plain_text, 
			COALESCE(c.markdown_text, '') as markdown_text 
		FROM file_asset f
		LEFT JOIN file_content c ON f.id = c.file_asset_id
		LEFT JOIN audit_item a ON f.id = a.file_id
		WHERE f.company_id = ? 
		  AND (f.source_module IS NULL OR f.source_module NOT IN ('tech_library', 'bid_project'))
		ORDER BY f.created_at DESC 
		LIMIT 200
	`
	err := s.db.Select(&files, query, companyID)
	if err != nil {
		log.Printf("[FileService] ListFiles error: %v", err)
		return nil, err
	}
	return files, nil
}

func (s *FileService) GetFileDetail(id string) (*model.FileAsset, error) {
	var file model.FileAsset
	query := `
		SELECT 
			f.id, f.file_name, f.ext, f.mime_type, f.file_size, f.sha256, 
			f.source_path, f.stored_path, f.source_type, f.import_batch_id, 
			f.scan_status, f.parse_status, f.archive_status, f.company_id, 
			f.created_at, f.updated_at,
			f.source_module, f.source_project_id, f.last_task_id, f.last_error_message,
			f.scan_status as status,
			f.archive_target_type, f.archive_target_id,
			a.id as audit_id,
			a.object_type as object_type,
			COALESCE(c.plain_text, '') as plain_text, 
			COALESCE(c.markdown_text, '') as markdown_text 
		FROM file_asset f
		LEFT JOIN file_content c ON f.id = c.file_asset_id
		LEFT JOIN audit_item a ON f.id = a.file_id
		WHERE f.id = ?
	`
	err := s.db.Get(&file, query, id)
	if err != nil {
		log.Printf("[FileService] GetFileDetail error: %v", err)
		return nil, err
	}
	return &file, nil
}

func (s *FileService) UpdateFileStatus(id string, status string) error {
	_, err := s.db.Exec("UPDATE file_asset SET scan_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id)
	return err
}

func (s *FileService) CreateFileAsset(file *model.FileAsset) error {
	query := `
		INSERT INTO file_asset (
			id, file_name, ext, mime_type, file_size, sha256, stored_path, 
			company_id, scan_status, source_module, source_project_id
		) VALUES (
			:id, :file_name, :ext, :mime_type, :file_size, :sha256, :stored_path, 
			:company_id, :scan_status, :source_module, :source_project_id
		)
	`
	_, err := s.db.NamedExec(query, file)
	return err
}
