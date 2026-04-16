package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type AutomationArtifact struct {
	Id           int    `json:"id"`
	ExecutionID  string `json:"execution_id" gorm:"size:64;not null;index"`
	AccountID    int    `json:"account_id" gorm:"not null;default:0;index"`
	Module       string `json:"module" gorm:"size:64;not null;default:''"`
	ArtifactType string `json:"artifact_type" gorm:"size:64;not null;default:''"`
	Path         string `json:"path" gorm:"type:text"`
	Checksum     string `json:"checksum,omitempty" gorm:"size:128"`
	Meta         string `json:"meta,omitempty" gorm:"type:text"`
	CreatedAtTs  int64  `json:"created_at" gorm:"bigint;index"`
}

func (AutomationArtifact) TableName() string {
	return "automation_artifacts"
}

func (a *AutomationArtifact) Normalize() {
	a.ExecutionID = strings.TrimSpace(a.ExecutionID)
	a.Module = strings.TrimSpace(strings.ToLower(a.Module))
	a.ArtifactType = strings.TrimSpace(strings.ToLower(a.ArtifactType))
	a.Path = strings.TrimSpace(a.Path)
	a.Checksum = strings.TrimSpace(strings.ToLower(a.Checksum))
	a.Meta = strings.TrimSpace(a.Meta)
}

func (a *AutomationArtifact) Insert() error {
	a.Normalize()
	a.CreatedAtTs = common.GetTimestamp()
	return DB.Create(a).Error
}

func ListAutomationArtifactsByExecution(executionID string) ([]*AutomationArtifact, error) {
	var list []*AutomationArtifact
	err := DB.Where("execution_id = ?", strings.TrimSpace(executionID)).Order("id ASC").Find(&list).Error
	return list, err
}
