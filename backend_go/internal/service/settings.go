package service

import (
	"log"

	"github.com/jmoiron/sqlx"
)

type SettingsService struct {
	db *sqlx.DB
}

func NewSettingsService(db *sqlx.DB) *SettingsService {
	return &SettingsService{db: db}
}

func (s *SettingsService) GetAIKey() string {
	var key string
	err := s.db.Get(&key, "SELECT value FROM system_settings WHERE key = 'ai_api_key'")
	if err != nil {
		log.Printf("[Settings] Error fetching AI key: %v", err)
		return ""
	}
	return key
}

func (s *SettingsService) GetSetting(key string) string {
	var value string
	err := s.db.Get(&value, "SELECT value FROM system_settings WHERE key = ?", key)
	if err != nil {
		return ""
	}
	return value
}
func (s *SettingsService) GetDoubaoConfig() (string, string, string) {
	key := s.GetSetting("doubao_api_key")
	endpoint := s.GetSetting("doubao_endpoint")
	model := s.GetSetting("doubao_model_id")
	return key, endpoint, model
}
