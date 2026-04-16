package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ChatMockRouteLog struct {
	Id              int            `json:"id"`
	AccountID       int            `json:"account_id" gorm:"not null;index"`
	AccountName     string         `json:"account_name" gorm:"size:255"`
	Model           string         `json:"model" gorm:"size:255;index"`
	Strategy        string         `json:"strategy" gorm:"size:64;index"`
	RouteScore      int64          `json:"route_score" gorm:"not null;default:0"`
	SelectionReason string         `json:"selection_reason" gorm:"type:text"`
	LatencyMS       int64          `json:"latency_ms" gorm:"not null;default:0"`
	Success         bool           `json:"success" gorm:"not null;default:false;index"`
	FailoverReason  string         `json:"failover_reason" gorm:"type:text"`
	TokenUsage      int64          `json:"token_usage" gorm:"not null;default:0"`
	RequestID       string         `json:"request_id" gorm:"size:128;index"`
	CreatedTime     int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime     int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ChatMockRouteLog) TableName() string {
	return "chatmock_route_logs"
}

func (l *ChatMockRouteLog) Normalize() {
	l.AccountName = strings.TrimSpace(l.AccountName)
	l.Model = strings.TrimSpace(l.Model)
	l.Strategy = strings.TrimSpace(l.Strategy)
	l.SelectionReason = strings.TrimSpace(l.SelectionReason)
	l.FailoverReason = strings.TrimSpace(l.FailoverReason)
	l.RequestID = strings.TrimSpace(l.RequestID)
}

func (l *ChatMockRouteLog) Insert() error {
	if l == nil {
		return nil
	}
	l.Normalize()
	now := common.GetTimestamp()
	if l.CreatedTime == 0 {
		l.CreatedTime = now
	}
	l.UpdatedTime = now
	return DB.Create(l).Error
}

func ListChatMockRouteLogs(accountID int, modelName string, success string, offset int, limit int) ([]*ChatMockRouteLog, int64, error) {
	return ListChatMockRouteLogsWithFilter(accountID, modelName, success, "", "", offset, limit)
}

func ListChatMockRouteLogsWithFilter(
	accountID int,
	modelName string,
	success string,
	strategy string,
	keyword string,
	offset int,
	limit int,
) ([]*ChatMockRouteLog, int64, error) {
	db := DB.Model(&ChatMockRouteLog{})
	if accountID > 0 {
		db = db.Where("account_id = ?", accountID)
	}
	if strings.TrimSpace(modelName) != "" {
		db = db.Where("model = ?", strings.TrimSpace(modelName))
	}
	if strings.TrimSpace(strategy) != "" {
		db = db.Where("strategy = ?", strings.TrimSpace(strategy))
	}
	if strings.TrimSpace(keyword) != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		db = db.Where(
			"selection_reason LIKE ? OR failover_reason LIKE ? OR account_name LIKE ? OR request_id LIKE ?",
			like,
			like,
			like,
			like,
		)
	}
	switch strings.ToLower(strings.TrimSpace(success)) {
	case "true", "1", "yes":
		db = db.Where("success = ?", true)
	case "false", "0", "no":
		db = db.Where("success = ?", false)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*ChatMockRouteLog
	query := db.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
