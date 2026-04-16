package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

type AutomationExecution struct {
	Id            int            `json:"id"`
	ExecutionID   string         `json:"execution_id" gorm:"size:64;not null;uniqueIndex"`
	Module        string         `json:"module" gorm:"size:64;not null;default:'';index"`
	Mode          string         `json:"mode" gorm:"size:32;not null;default:'local'"`
	Status        string         `json:"status" gorm:"size:32;not null;default:'queued';index"`
	Progress      int            `json:"progress" gorm:"not null;default:0"`
	TotalAccounts int            `json:"total_accounts" gorm:"not null;default:0"`
	SuccessCount  int            `json:"success_count" gorm:"not null;default:0"`
	FailedCount   int            `json:"failed_count" gorm:"not null;default:0"`
	RunOptions    string         `json:"run_options,omitempty" gorm:"type:text"`
	ResultSummary string         `json:"result_summary,omitempty" gorm:"type:text"`
	ErrorMessage  string         `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAtTs   int64          `json:"created_at" gorm:"bigint"`
	UpdatedAtTs   int64          `json:"updated_at" gorm:"bigint"`
	StartedAt     int64          `json:"started_at" gorm:"bigint"`
	FinishedAt    int64          `json:"finished_at" gorm:"bigint"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

type AutomationExecutionListOptions struct {
	Status           string
	Module           string
	Search           string
	OnlyProxyRetried bool
}

func (AutomationExecution) TableName() string {
	return "automation_executions"
}

func normalizeAutomationExecutionStatus(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case constant.AutomationExecutionStatusQueued,
		constant.AutomationExecutionStatusRunning,
		constant.AutomationExecutionStatusStopping,
		constant.AutomationExecutionStatusStopped,
		constant.AutomationExecutionStatusCompleted,
		constant.AutomationExecutionStatusFailed:
		return status
	default:
		return constant.AutomationExecutionStatusQueued
	}
}

func (e *AutomationExecution) Normalize() {
	e.ExecutionID = strings.TrimSpace(e.ExecutionID)
	e.Module = strings.TrimSpace(strings.ToLower(e.Module))
	e.Mode = strings.TrimSpace(strings.ToLower(e.Mode))
	if e.Mode == "" {
		e.Mode = constant.AutomationModeLocal
	}
	e.Status = normalizeAutomationExecutionStatus(e.Status)
	e.ErrorMessage = strings.TrimSpace(e.ErrorMessage)
	if e.Progress < 0 {
		e.Progress = 0
	}
	if e.Progress > 100 {
		e.Progress = 100
	}
}

func (e *AutomationExecution) ValidateForSave() error {
	if e.ExecutionID == "" {
		return fmt.Errorf("执行 ID 不能为空")
	}
	if len(e.ExecutionID) > 64 {
		return fmt.Errorf("执行 ID 过长")
	}
	return nil
}

func (e *AutomationExecution) Insert() error {
	e.Normalize()
	if err := e.ValidateForSave(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	e.CreatedAtTs = now
	e.UpdatedAtTs = now
	if e.Status == constant.AutomationExecutionStatusRunning && e.StartedAt == 0 {
		e.StartedAt = now
	}
	return DB.Create(e).Error
}

func (e *AutomationExecution) Update() error {
	e.Normalize()
	e.UpdatedAtTs = common.GetTimestamp()
	return DB.Model(&AutomationExecution{}).Where("execution_id = ?", e.ExecutionID).
		Select(
			"module", "mode", "status", "progress", "total_accounts", "success_count", "failed_count",
			"run_options", "result_summary", "error_message", "updated_at_ts", "started_at", "finished_at",
		).
		Updates(e).Error
}

func GetAutomationExecutionByExecutionID(executionID string) (*AutomationExecution, error) {
	var execution AutomationExecution
	if err := DB.Where("execution_id = ?", strings.TrimSpace(executionID)).First(&execution).Error; err != nil {
		return nil, err
	}
	return &execution, nil
}

func normalizeAutomationExecutionFilterStatus(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case constant.AutomationExecutionStatusQueued,
		constant.AutomationExecutionStatusRunning,
		constant.AutomationExecutionStatusStopping,
		constant.AutomationExecutionStatusStopped,
		constant.AutomationExecutionStatusCompleted,
		constant.AutomationExecutionStatusFailed:
		return status
	default:
		return ""
	}
}

func applyAutomationExecutionListOptions(db *gorm.DB, filter AutomationExecutionListOptions) *gorm.DB {
	if db == nil {
		db = DB.Model(&AutomationExecution{})
	}
	if status := normalizeAutomationExecutionFilterStatus(filter.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if module := strings.TrimSpace(strings.ToLower(filter.Module)); module != "" {
		db = db.Where("module = ?", module)
	}
	if keyword := strings.TrimSpace(filter.Search); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			DB.Where("execution_id LIKE ?", like).
				Or("module LIKE ?", like).
				Or("error_message LIKE ?", like).
				Or("result_summary LIKE ?", like),
		)
	}
	if filter.OnlyProxyRetried {
		subQuery := DB.Model(&AutomationExecutionLog{}).
			Select("1").
			Where("automation_execution_logs.execution_id = automation_executions.execution_id").
			Where("message = ?", "proxy retry scheduled")
		db = db.Where("EXISTS (?)", subQuery)
	}
	return db
}

func normalizeAutomationExecutionIDs(executionIDs []string) []string {
	result := make([]string, 0, len(executionIDs))
	seen := make(map[string]struct{}, len(executionIDs))
	for _, executionID := range executionIDs {
		value := strings.TrimSpace(executionID)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func ListAutomationExecutions(offset int, limit int, options ...AutomationExecutionListOptions) ([]*AutomationExecution, int64, error) {
	filter := AutomationExecutionListOptions{}
	if len(options) > 0 {
		filter = options[0]
	}
	db := applyAutomationExecutionListOptions(DB.Model(&AutomationExecution{}), filter)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := db.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	var list []*AutomationExecution
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func ListAutomationExecutionIDs(filter AutomationExecutionListOptions, limit int) ([]string, error) {
	db := applyAutomationExecutionListOptions(DB.Model(&AutomationExecution{}), filter).
		Order("id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}
	rows := make([]string, 0)
	if err := db.Pluck("execution_id", &rows).Error; err != nil {
		return nil, err
	}
	return normalizeAutomationExecutionIDs(rows), nil
}

func ListAutomationExecutionsByExecutionIDs(executionIDs []string) ([]*AutomationExecution, error) {
	normalizedIDs := normalizeAutomationExecutionIDs(executionIDs)
	if len(normalizedIDs) == 0 {
		return []*AutomationExecution{}, nil
	}
	var list []*AutomationExecution
	if err := DB.Where("execution_id IN ?", normalizedIDs).
		Order("id DESC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
