package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type AutomationExecutionLog struct {
	Id          int    `json:"id"`
	ExecutionID string `json:"execution_id" gorm:"size:64;not null;index"`
	AccountID   int    `json:"account_id" gorm:"not null;default:0;index"`
	Level       string `json:"level" gorm:"size:16;not null;default:'info'"`
	Step        string `json:"step" gorm:"size:64;not null;default:''"`
	Message     string `json:"message" gorm:"type:text"`
	Payload     string `json:"payload,omitempty" gorm:"type:text"`
	CreatedAtTs int64  `json:"created_at" gorm:"bigint;index"`
}

func (AutomationExecutionLog) TableName() string {
	return "automation_execution_logs"
}

func (l *AutomationExecutionLog) Normalize() {
	l.ExecutionID = strings.TrimSpace(l.ExecutionID)
	l.Level = strings.TrimSpace(strings.ToLower(l.Level))
	if l.Level == "" {
		l.Level = "info"
	}
	l.Step = strings.TrimSpace(strings.ToLower(l.Step))
	l.Message = strings.TrimSpace(l.Message)
}

func (l *AutomationExecutionLog) Insert() error {
	l.Normalize()
	l.CreatedAtTs = common.GetTimestamp()
	return DB.Create(l).Error
}

func ListAutomationExecutionLogs(executionID string, limit int, offset int) ([]*AutomationExecutionLog, int64, error) {
	db := DB.Model(&AutomationExecutionLog{}).Where("execution_id = ?", strings.TrimSpace(executionID))
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := db.Order("id ASC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	var list []*AutomationExecutionLog
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func CountAutomationExecutionLogsByMessage(executionIDs []string, message string) (map[string]int64, error) {
	normalizedIDs := make([]string, 0, len(executionIDs))
	for _, executionID := range executionIDs {
		value := strings.TrimSpace(executionID)
		if value == "" {
			continue
		}
		normalizedIDs = append(normalizedIDs, value)
	}
	result := map[string]int64{}
	if len(normalizedIDs) == 0 {
		return result, nil
	}
	rows := []struct {
		ExecutionID string
		Count       int64
	}{}
	if err := DB.Model(&AutomationExecutionLog{}).
		Select("execution_id, COUNT(*) AS count").
		Where("execution_id IN ? AND message = ?", normalizedIDs, strings.TrimSpace(message)).
		Group("execution_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[strings.TrimSpace(row.ExecutionID)] = row.Count
	}
	return result, nil
}
