package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AutomationTeamStatusActive   = "active"
	AutomationTeamStatusFull     = "full"
	AutomationTeamStatusExpired  = "expired"
	AutomationTeamStatusError    = "error"
	AutomationTeamStatusBanned   = "banned"
	AutomationTeamStatusDisabled = "disabled"
)

// AutomationTeam stores Team asset state for team automation.
type AutomationTeam struct {
	Id                    int            `json:"id"`
	Email                 string         `json:"email" gorm:"size:255;not null;uniqueIndex:uk_automation_team_email_delete_at,priority:1"`
	AccessToken           string         `json:"access_token,omitempty" gorm:"type:text"`
	RefreshToken          string         `json:"refresh_token,omitempty" gorm:"type:text"`
	SessionToken          string         `json:"session_token,omitempty" gorm:"type:text"`
	ClientID              string         `json:"client_id,omitempty" gorm:"size:255;index"`
	AccountID             string         `json:"account_id,omitempty" gorm:"size:255;index"`
	TeamName              string         `json:"team_name,omitempty" gorm:"size:255"`
	PlanType              string         `json:"plan_type,omitempty" gorm:"size:64"`
	SubscriptionPlan      string         `json:"subscription_plan,omitempty" gorm:"size:128"`
	ExpiresAt             int64          `json:"expires_at" gorm:"bigint"`
	CurrentMembers        int            `json:"current_members" gorm:"not null;default:0"`
	PendingInvites        int            `json:"pending_invites" gorm:"not null;default:0"`
	MaxMembers            int            `json:"max_members" gorm:"not null;default:6"`
	Status                string         `json:"status" gorm:"size:32;not null;default:'active';index"`
	AccountRole           string         `json:"account_role,omitempty" gorm:"size:64"`
	DeviceCodeAuthEnabled bool           `json:"device_code_auth_enabled" gorm:"not null;default:false"`
	ErrorCount            int            `json:"error_count" gorm:"not null;default:0"`
	LastSyncTime          int64          `json:"last_sync_time" gorm:"bigint"`
	Metadata              string         `json:"metadata,omitempty" gorm:"type:text"`
	CreatedTime           int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime           int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_automation_team_email_delete_at,priority:2"`
}

func (AutomationTeam) TableName() string {
	return "automation_teams"
}

func normalizeAutomationTeamStatus(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case AutomationTeamStatusActive,
		AutomationTeamStatusFull,
		AutomationTeamStatusExpired,
		AutomationTeamStatusError,
		AutomationTeamStatusBanned,
		AutomationTeamStatusDisabled:
		return status
	default:
		return AutomationTeamStatusActive
	}
}

func (t *AutomationTeam) Normalize() {
	t.Email = strings.TrimSpace(strings.ToLower(t.Email))
	t.AccessToken = strings.TrimSpace(t.AccessToken)
	t.RefreshToken = strings.TrimSpace(t.RefreshToken)
	t.SessionToken = strings.TrimSpace(t.SessionToken)
	t.ClientID = strings.TrimSpace(t.ClientID)
	t.AccountID = strings.TrimSpace(t.AccountID)
	t.TeamName = strings.TrimSpace(t.TeamName)
	t.PlanType = strings.TrimSpace(strings.ToLower(t.PlanType))
	t.SubscriptionPlan = strings.TrimSpace(t.SubscriptionPlan)
	t.Status = normalizeAutomationTeamStatus(t.Status)
	t.AccountRole = strings.TrimSpace(strings.ToLower(t.AccountRole))
	t.Metadata = strings.TrimSpace(t.Metadata)
	if t.MaxMembers <= 0 {
		t.MaxMembers = 6
	}
	if t.CurrentMembers < 0 {
		t.CurrentMembers = 0
	}
	if t.PendingInvites < 0 {
		t.PendingInvites = 0
	}
	if t.ErrorCount < 0 {
		t.ErrorCount = 0
	}
}

func (t *AutomationTeam) ValidateForSave() error {
	if t.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	if t.AccessToken == "" && t.SessionToken == "" && (t.RefreshToken == "" || t.ClientID == "") {
		return fmt.Errorf("至少提供 access_token，或 session_token，或 refresh_token + client_id")
	}
	if t.MaxMembers <= 0 {
		return fmt.Errorf("最大成员数必须大于 0")
	}
	return nil
}

func (t *AutomationTeam) Insert() error {
	t.Normalize()
	if err := t.ValidateForSave(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	t.CreatedTime = now
	t.UpdatedTime = now
	return DB.Create(t).Error
}

func (t *AutomationTeam) Update() error {
	if t.Id <= 0 {
		return fmt.Errorf("缺少 Team ID")
	}
	t.Normalize()
	if err := t.ValidateForSave(); err != nil {
		return err
	}
	t.UpdatedTime = common.GetTimestamp()
	return DB.Model(&AutomationTeam{}).Where("id = ?", t.Id).
		Select(
			"email", "access_token", "refresh_token", "session_token", "client_id", "account_id",
			"team_name", "plan_type", "subscription_plan", "expires_at", "current_members", "pending_invites",
			"max_members", "status", "account_role", "device_code_auth_enabled", "error_count",
			"last_sync_time", "metadata", "updated_time",
		).
		Updates(t).Error
}

func (t *AutomationTeam) Delete() error {
	return DB.Delete(t).Error
}

func (t *AutomationTeam) OccupiedSlots() int {
	occupied := t.CurrentMembers + t.PendingInvites
	if occupied < 0 {
		return 0
	}
	return occupied
}

func (t *AutomationTeam) RemainingSlots() int {
	remaining := t.MaxMembers - t.OccupiedSlots()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func GetAutomationTeamByID(id int) (*AutomationTeam, error) {
	var team AutomationTeam
	if err := DB.Where("id = ?", id).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func GetAutomationTeamByEmail(email string) (*AutomationTeam, error) {
	var team AutomationTeam
	if err := DB.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func GetAutomationTeamByAccountID(accountID string) (*AutomationTeam, error) {
	value := strings.TrimSpace(accountID)
	if value == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var team AutomationTeam
	if err := DB.Where("account_id = ?", value).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func GetAutomationTeamByClientID(clientID string) (*AutomationTeam, error) {
	value := strings.TrimSpace(clientID)
	if value == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var team AutomationTeam
	if err := DB.Where("client_id = ?", value).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func ListAutomationTeams(keyword string, status string, onlyAvailable bool, offset int, limit int) ([]*AutomationTeam, int64, error) {
	db := DB.Model(&AutomationTeam{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("email LIKE ? OR team_name LIKE ? OR account_id LIKE ? OR client_id LIKE ? OR metadata LIKE ?", like, like, like, like, like)
	}
	if status = strings.TrimSpace(strings.ToLower(status)); status != "" {
		db = db.Where("status = ?", status)
	}
	if onlyAvailable {
		db = db.Where("status = ?", AutomationTeamStatusActive).
			Where("(COALESCE(max_members, 0) - (COALESCE(current_members, 0) + COALESCE(pending_invites, 0))) > 0")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := db.Order("status = 'active' DESC").Order("(COALESCE(current_members, 0) + COALESCE(pending_invites, 0)) ASC").Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	var list []*AutomationTeam
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func GetAutomationTeamStats() (map[string]any, error) {
	stats := map[string]any{
		"total":                 int64(0),
		"active":                int64(0),
		"available":             int64(0),
		"total_remaining_slots": int64(0),
		"total_pending_invites": int64(0),
		"by_status":             map[string]int64{},
		"updated_at":            common.GetTimestamp(),
	}
	type bucket struct {
		Name  string
		Count int64
	}
	var total int64
	if err := DB.Model(&AutomationTeam{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total
	var statusRows []bucket
	if err := DB.Model(&AutomationTeam{}).Select("status as name, count(*) as count").Group("status").Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	statusMap := map[string]int64{}
	for _, row := range statusRows {
		key := normalizeAutomationTeamStatus(row.Name)
		statusMap[key] = row.Count
	}
	stats["by_status"] = statusMap
	stats["active"] = statusMap[AutomationTeamStatusActive]

	type aggRow struct {
		Available int64
		Remaining int64
		Pending   int64
	}
	var agg aggRow
	if err := DB.Model(&AutomationTeam{}).
		Select(`
			SUM(CASE WHEN status = ? AND (COALESCE(max_members, 0) - (COALESCE(current_members, 0) + COALESCE(pending_invites, 0))) > 0 THEN 1 ELSE 0 END) as available,
			SUM(CASE WHEN status = ? THEN CASE WHEN (COALESCE(max_members, 0) - (COALESCE(current_members, 0) + COALESCE(pending_invites, 0))) > 0 THEN (COALESCE(max_members, 0) - (COALESCE(current_members, 0) + COALESCE(pending_invites, 0))) ELSE 0 END ELSE 0 END) as remaining,
			SUM(COALESCE(pending_invites, 0)) as pending
		`, AutomationTeamStatusActive, AutomationTeamStatusActive).
		Scan(&agg).Error; err != nil {
		return nil, err
	}
	stats["available"] = agg.Available
	stats["total_remaining_slots"] = agg.Remaining
	stats["total_pending_invites"] = agg.Pending
	return stats, nil
}
