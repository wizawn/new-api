package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ChatMockAccountSnapshot struct {
	Id           int            `json:"id"`
	AccountID    int            `json:"account_id" gorm:"not null;index"`
	SnapshotType string         `json:"snapshot_type" gorm:"size:64;not null;index"`
	Status       string         `json:"status" gorm:"size:32;not null;default:'ok';index"`
	Summary      string         `json:"summary" gorm:"type:text"`
	Payload      string         `json:"payload" gorm:"type:text"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ChatMockAccountSnapshot) TableName() string {
	return "chatmock_account_snapshots"
}

func (s *ChatMockAccountSnapshot) Normalize() {
	s.SnapshotType = strings.TrimSpace(strings.ToLower(s.SnapshotType))
	s.Status = strings.TrimSpace(strings.ToLower(s.Status))
	if s.Status == "" {
		s.Status = "ok"
	}
	s.Summary = strings.TrimSpace(s.Summary)
	s.Payload = strings.TrimSpace(s.Payload)
}

func (s *ChatMockAccountSnapshot) Insert() error {
	if s == nil {
		return nil
	}
	s.Normalize()
	now := common.GetTimestamp()
	if s.CreatedTime == 0 {
		s.CreatedTime = now
	}
	s.UpdatedTime = now
	return DB.Create(s).Error
}

func ListChatMockAccountSnapshots(accountID int, snapshotType string, offset int, limit int) ([]*ChatMockAccountSnapshot, int64, error) {
	return ListChatMockAccountSnapshotsWithFilter(accountID, snapshotType, "", "", offset, limit)
}

func ListChatMockAccountSnapshotsWithFilter(
	accountID int,
	snapshotType string,
	status string,
	keyword string,
	offset int,
	limit int,
) ([]*ChatMockAccountSnapshot, int64, error) {
	db := DB.Model(&ChatMockAccountSnapshot{})
	if accountID > 0 {
		db = db.Where("account_id = ?", accountID)
	}
	if strings.TrimSpace(snapshotType) != "" {
		db = db.Where("snapshot_type = ?", strings.TrimSpace(strings.ToLower(snapshotType)))
	}
	if strings.TrimSpace(status) != "" {
		db = db.Where("status = ?", strings.TrimSpace(strings.ToLower(status)))
	}
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		db = db.Where("summary LIKE ? OR payload LIKE ?", like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*ChatMockAccountSnapshot
	query := db.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
