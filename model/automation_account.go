package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// AutomationAccount stores local Team Automation account state.
type AutomationAccount struct {
	Id                  int            `json:"id"`
	Name                string         `json:"name" gorm:"size:128;not null"`
	Email               string         `json:"email" gorm:"size:255;not null;uniqueIndex:uk_automation_account_email_delete_at,priority:1"`
	Password            string         `json:"password,omitempty" gorm:"type:text"`
	RedeemCode          string         `json:"redeem_code,omitempty" gorm:"size:255"`
	RedeemType          string         `json:"redeem_type,omitempty" gorm:"size:64"`
	Status              string         `json:"status" gorm:"size:64;not null;default:'pending';index"`
	FailureReason       string         `json:"failure_reason,omitempty" gorm:"type:text"`
	CloudflareDomain    string         `json:"cloudflare_domain,omitempty" gorm:"size:255"`
	CloudflareAlias     string         `json:"cloudflare_alias,omitempty" gorm:"size:255"`
	ForwardTarget       string         `json:"forward_target,omitempty" gorm:"size:255"`
	RedeemSite          string         `json:"redeem_site,omitempty" gorm:"size:255"`
	InviteLink          string         `json:"invite_link,omitempty" gorm:"type:text"`
	InviteCode          string         `json:"invite_code,omitempty" gorm:"size:128"`
	MailboxId           int            `json:"mailbox_id" gorm:"not null;default:0;index"`
	SessionArtifactPath string         `json:"session_artifact_path,omitempty" gorm:"type:text"`
	AuthArtifactPath    string         `json:"auth_artifact_path,omitempty" gorm:"type:text"`
	ImportedTarget      string         `json:"imported_target,omitempty" gorm:"size:64"`
	ImportedReference   string         `json:"imported_reference,omitempty" gorm:"size:255"`
	Metadata            string         `json:"metadata,omitempty" gorm:"type:text"`
	CreatedTime         int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt           gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_automation_account_email_delete_at,priority:2"`
}

func (AutomationAccount) TableName() string {
	return "automation_accounts"
}

func normalizeAutomationAccountStatus(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case constant.AutomationAccountStatusPending,
		constant.AutomationAccountStatusGenerated,
		constant.AutomationAccountStatusWaitingInvite,
		constant.AutomationAccountStatusInviteReceived,
		constant.AutomationAccountStatusJoinedTeam,
		constant.AutomationAccountStatusLoginCaptured,
		constant.AutomationAccountStatusConverted,
		constant.AutomationAccountStatusImported,
		constant.AutomationAccountStatusCompleted,
		constant.AutomationAccountStatusManualReview,
		constant.AutomationAccountStatusFailed,
		constant.AutomationAccountStatusDisabled:
		return status
	default:
		return constant.AutomationAccountStatusPending
	}
}

func (a *AutomationAccount) Normalize() {
	a.Name = strings.TrimSpace(a.Name)
	a.Email = strings.TrimSpace(strings.ToLower(a.Email))
	a.Password = strings.TrimSpace(a.Password)
	a.RedeemCode = strings.TrimSpace(a.RedeemCode)
	a.RedeemType = strings.TrimSpace(a.RedeemType)
	a.Status = normalizeAutomationAccountStatus(a.Status)
	a.FailureReason = strings.TrimSpace(a.FailureReason)
	a.CloudflareDomain = strings.TrimSpace(strings.ToLower(a.CloudflareDomain))
	a.CloudflareAlias = strings.TrimSpace(strings.ToLower(a.CloudflareAlias))
	a.ForwardTarget = strings.TrimSpace(strings.ToLower(a.ForwardTarget))
	a.RedeemSite = strings.TrimSpace(strings.ToLower(a.RedeemSite))
	a.InviteLink = strings.TrimSpace(a.InviteLink)
	a.InviteCode = strings.TrimSpace(a.InviteCode)
	a.SessionArtifactPath = strings.TrimSpace(a.SessionArtifactPath)
	a.AuthArtifactPath = strings.TrimSpace(a.AuthArtifactPath)
	a.ImportedTarget = strings.TrimSpace(strings.ToLower(a.ImportedTarget))
	a.ImportedReference = strings.TrimSpace(a.ImportedReference)
	a.Metadata = strings.TrimSpace(a.Metadata)
	if a.Name == "" && a.Email != "" {
		a.Name = a.Email
	}
}

func (a *AutomationAccount) ValidateForSave() error {
	if a.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	if a.Name == "" {
		return fmt.Errorf("账号名称不能为空")
	}
	return nil
}

func (a *AutomationAccount) Insert() error {
	a.Normalize()
	if err := a.ValidateForSave(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	a.CreatedTime = now
	a.UpdatedTime = now
	return DB.Create(a).Error
}

func (a *AutomationAccount) Update() error {
	if a.Id <= 0 {
		return fmt.Errorf("缺少账号 ID")
	}
	a.Normalize()
	if err := a.ValidateForSave(); err != nil {
		return err
	}
	a.UpdatedTime = common.GetTimestamp()
	return DB.Model(&AutomationAccount{}).Where("id = ?", a.Id).
		Select(
			"name", "email", "password", "redeem_code", "redeem_type", "status", "failure_reason",
			"cloudflare_domain", "cloudflare_alias", "forward_target", "redeem_site", "invite_link", "invite_code",
			"mailbox_id", "session_artifact_path", "auth_artifact_path", "imported_target", "imported_reference",
			"metadata", "updated_time",
		).
		Updates(a).Error
}

func (a *AutomationAccount) Delete() error {
	return DB.Delete(a).Error
}

func GetAutomationAccountByID(id int) (*AutomationAccount, error) {
	var account AutomationAccount
	if err := DB.Where("id = ?", id).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func GetAutomationAccountByEmail(email string) (*AutomationAccount, error) {
	var account AutomationAccount
	if err := DB.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func ListAutomationAccounts(keyword string, status string, offset int, limit int) ([]*AutomationAccount, int64, error) {
	db := DB.Model(&AutomationAccount{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR email LIKE ? OR redeem_code LIKE ? OR metadata LIKE ?", like, like, like, like)
	}
	if status = strings.TrimSpace(strings.ToLower(status)); status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := db.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	var list []*AutomationAccount
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func ListAutomationAccountsByFilter(status string, keyword string) ([]*AutomationAccount, error) {
	db := DB.Model(&AutomationAccount{})
	if status = strings.TrimSpace(strings.ToLower(status)); status != "" {
		db = db.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR email LIKE ? OR redeem_code LIKE ? OR metadata LIKE ?", like, like, like, like)
	}
	var list []*AutomationAccount
	if err := db.Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func GetAutomationAccountStats() (map[string]any, error) {
	stats := map[string]any{
		"total":      int64(0),
		"by_status":  map[string]int64{},
		"by_redeem":  map[string]int64{},
		"by_site":    map[string]int64{},
		"updated_at": common.GetTimestamp(),
	}
	var total int64
	if err := DB.Model(&AutomationAccount{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type bucket struct {
		Name  string
		Count int64
	}
	var statusRows []bucket
	if err := DB.Model(&AutomationAccount{}).Select("status as name, count(*) as count").Group("status").Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	statusMap := map[string]int64{}
	for _, row := range statusRows {
		key := strings.TrimSpace(strings.ToLower(row.Name))
		if key == "" {
			key = constant.AutomationAccountStatusPending
		}
		statusMap[key] = row.Count
	}
	stats["by_status"] = statusMap

	var redeemRows []bucket
	if err := DB.Model(&AutomationAccount{}).Select("redeem_type as name, count(*) as count").Group("redeem_type").Scan(&redeemRows).Error; err != nil {
		return nil, err
	}
	redeemMap := map[string]int64{}
	for _, row := range redeemRows {
		key := strings.TrimSpace(strings.ToLower(row.Name))
		if key == "" {
			key = "unknown"
		}
		redeemMap[key] = row.Count
	}
	stats["by_redeem"] = redeemMap

	var siteRows []bucket
	if err := DB.Model(&AutomationAccount{}).Select("redeem_site as name, count(*) as count").Group("redeem_site").Scan(&siteRows).Error; err != nil {
		return nil, err
	}
	siteMap := map[string]int64{}
	for _, row := range siteRows {
		key := strings.TrimSpace(strings.ToLower(row.Name))
		if key == "" {
			key = "default"
		}
		siteMap[key] = row.Count
	}
	stats["by_site"] = siteMap

	return stats, nil
}
