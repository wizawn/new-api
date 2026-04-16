package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// ChatMockMailbox 通用邮箱中心配置（IMAP/SMTP 协议）。
// Password 字段存储邮箱授权码或登录密码，仅供 IMAP/SMTP 连接使用。
type ChatMockMailbox struct {
	Id           int    `json:"id"`
	Name         string `json:"name" gorm:"size:128;not null"`
	Email        string `json:"email" gorm:"size:255;not null;uniqueIndex:uk_chatmock_mailbox_email_delete_at,priority:1"`
	Username     string `json:"username" gorm:"size:255;not null"`
	Password     string `json:"password,omitempty" gorm:"type:text;not null"`
	ImapHost     string `json:"imap_host" gorm:"size:255;not null"`
	ImapPort     int    `json:"imap_port" gorm:"not null;default:993"`
	ImapSecurity string `json:"imap_security" gorm:"size:32;not null;default:'ssl'"`
	// SmtpHost/SmtpPort/SmtpSecurity are optional.
	// Current mailbox workflow only requires IMAP for code polling.
	SmtpHost           string         `json:"smtp_host" gorm:"size:255;not null"`
	SmtpPort           int            `json:"smtp_port" gorm:"not null;default:465"`
	SmtpSecurity       string         `json:"smtp_security" gorm:"size:32;not null;default:'ssl'"`
	Status             string         `json:"status" gorm:"size:32;not null;default:'enabled';index"`
	PollIntervalSecond int            `json:"poll_interval_second" gorm:"not null;default:60"`
	LastUID            uint32         `json:"last_uid" gorm:"not null;default:0"`
	LastCheckedTime    int64          `json:"last_checked_time" gorm:"bigint;not null;default:0"`
	LastError          string         `json:"last_error,omitempty" gorm:"type:text"`
	Metadata           string         `json:"metadata,omitempty" gorm:"type:text"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime        int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_chatmock_mailbox_email_delete_at,priority:2"`
}

func (ChatMockMailbox) TableName() string {
	return "chatmock_mailboxes"
}

func normalizeSecurityMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "ssl", "tls":
		return "ssl"
	case "starttls":
		return "starttls"
	case "none", "plain":
		return "none"
	default:
		return "ssl"
	}
}

func normalizeChatMockMailboxProviderType(metadata string) string {
	payload := map[string]any{}
	if err := common.UnmarshalJsonStr(strings.TrimSpace(metadata), &payload); err != nil || payload == nil {
		return ""
	}
	provider := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", payload["provider"])))
	if provider == "" {
		provider = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", payload["provider_type"])))
	}
	switch provider {
	case "wizawn", "wizawn_http", "mailbox_wizawn", "tempmail_wizawn", "tempmail":
		return "wizawn_http"
	default:
		return provider
	}
}

func (m *ChatMockMailbox) UsesProviderMailbox() bool {
	if m == nil {
		return false
	}
	switch normalizeChatMockMailboxProviderType(m.Metadata) {
	case "wizawn_http":
		return true
	default:
		return false
	}
}

func (m *ChatMockMailbox) Normalize() {
	m.Name = strings.TrimSpace(m.Name)
	m.Email = strings.TrimSpace(strings.ToLower(m.Email))
	m.Username = strings.TrimSpace(m.Username)
	m.Password = strings.TrimSpace(m.Password)
	m.ImapHost = strings.TrimSpace(m.ImapHost)
	m.SmtpHost = strings.TrimSpace(m.SmtpHost)
	m.ImapSecurity = normalizeSecurityMode(m.ImapSecurity)
	m.SmtpSecurity = normalizeSecurityMode(m.SmtpSecurity)
	m.Status = strings.TrimSpace(strings.ToLower(m.Status))
	if m.Status == "" {
		m.Status = constant.ChatMockMailboxStatusEnabled
	}
	if m.Status != constant.ChatMockMailboxStatusEnabled && m.Status != constant.ChatMockMailboxStatusDisabled {
		m.Status = constant.ChatMockMailboxStatusDisabled
	}
	if m.ImapPort <= 0 {
		m.ImapPort = 993
	}
	if m.SmtpPort <= 0 {
		m.SmtpPort = 465
	}
	if m.PollIntervalSecond <= 0 {
		m.PollIntervalSecond = 60
	}
	if m.UsesProviderMailbox() && m.Username == "" {
		m.Username = m.Email
	}
}

func (m *ChatMockMailbox) ValidateForSave() error {
	if m.Name == "" {
		return fmt.Errorf("邮箱名称不能为空")
	}
	if m.Email == "" {
		return fmt.Errorf("邮箱地址不能为空")
	}
	if m.Username == "" {
		return fmt.Errorf("邮箱用户名不能为空")
	}
	if m.Password == "" {
		return fmt.Errorf("邮箱密码/授权码不能为空")
	}
	if !m.UsesProviderMailbox() && m.ImapHost == "" {
		return fmt.Errorf("IMAP 主机不能为空")
	}
	return nil
}

func (m *ChatMockMailbox) Insert() error {
	m.Normalize()
	if err := m.ValidateForSave(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	m.CreatedTime = now
	m.UpdatedTime = now
	return DB.Create(m).Error
}

func (m *ChatMockMailbox) Update() error {
	m.Normalize()
	if m.Id == 0 {
		return fmt.Errorf("缺少邮箱 ID")
	}
	if err := m.ValidateForSave(); err != nil {
		return err
	}
	m.UpdatedTime = common.GetTimestamp()
	return DB.Model(&ChatMockMailbox{}).Where("id = ?", m.Id).
		Select(
			"name", "email", "username", "password", "imap_host", "imap_port", "imap_security",
			"smtp_host", "smtp_port", "smtp_security", "status", "poll_interval_second",
			"metadata", "updated_time",
		).Updates(m).Error
}

func (m *ChatMockMailbox) Delete() error {
	return DB.Delete(m).Error
}

func GetChatMockMailboxByID(id int) (*ChatMockMailbox, error) {
	var mailbox ChatMockMailbox
	err := DB.Where("id = ?", id).First(&mailbox).Error
	if err != nil {
		return nil, err
	}
	return &mailbox, nil
}

func GetChatMockMailboxByEmail(email string) (*ChatMockMailbox, error) {
	var mailbox ChatMockMailbox
	err := DB.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).First(&mailbox).Error
	if err != nil {
		return nil, err
	}
	return &mailbox, nil
}

func ListChatMockMailboxes(keyword, status string, offset int, limit int) ([]*ChatMockMailbox, int64, error) {
	db := DB.Model(&ChatMockMailbox{})
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		db = db.Where("name LIKE ? OR email LIKE ? OR imap_host LIKE ? OR smtp_host LIKE ?", like, like, like, like)
	}
	if status != "" {
		db = db.Where("status = ?", strings.TrimSpace(strings.ToLower(status)))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*ChatMockMailbox
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func ListEnabledChatMockMailboxes() ([]*ChatMockMailbox, error) {
	var list []*ChatMockMailbox
	err := DB.Where("status = ?", constant.ChatMockMailboxStatusEnabled).Find(&list).Error
	return list, err
}

func (m *ChatMockMailbox) Masked() *ChatMockMailbox {
	clone := *m
	if clone.Password != "" {
		clone.Password = "***"
	}
	return &clone
}
