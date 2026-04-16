package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ChatMockMailEvent 记录邮箱拉取事件、验证码提取事件和异常日志。
type ChatMockMailEvent struct {
	Id          int            `json:"id"`
	MailboxID   int            `json:"mailbox_id" gorm:"not null;index:idx_chatmock_mailbox_uid,priority:1"`
	MessageUID  uint32         `json:"message_uid" gorm:"not null;default:0;index:idx_chatmock_mailbox_uid,priority:2"`
	EventType   string         `json:"event_type" gorm:"size:64;not null;index"`
	Classification   string         `json:"classification,omitempty" gorm:"size:64;index"`
	Subject     string         `json:"subject,omitempty" gorm:"type:text"`
	Sender      string         `json:"sender,omitempty" gorm:"size:255"`
	RelatedIdentifier string         `json:"related_identifier,omitempty" gorm:"size:255;index"`
	Confidence       float64        `json:"confidence,omitempty" gorm:"not null;default:0"`
	Code        string         `json:"code,omitempty" gorm:"size:64;index"`
	ParsedPayload    string         `json:"parsed_payload,omitempty" gorm:"type:text"`
	RawSnippet  string         `json:"raw_snippet,omitempty" gorm:"type:text"`
	Status      string         `json:"status" gorm:"size:32;not null;default:'ok'"`
	ReceivedAt  int64          `json:"received_at" gorm:"bigint;not null;default:0"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ChatMockMailEvent) TableName() string {
	return "chatmock_mail_events"
}

func (e *ChatMockMailEvent) Insert() error {
	now := common.GetTimestamp()
	if e.CreatedTime == 0 {
		e.CreatedTime = now
	}
	e.UpdatedTime = now
	if e.ReceivedAt == 0 {
		e.ReceivedAt = now
	}
	return DB.Create(e).Error
}

func ListChatMockMailEvents(mailboxID int, eventType string, classification string, keyword string, offset, limit int) ([]*ChatMockMailEvent, int64, error) {
	db := DB.Model(&ChatMockMailEvent{})
	if mailboxID > 0 {
		db = db.Where("mailbox_id = ?", mailboxID)
	}
	if eventType != "" {
		db = db.Where("event_type = ?", eventType)
	}
	if classification != "" {
		db = db.Where("classification = ?", classification)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("subject LIKE ? OR sender LIKE ? OR raw_snippet LIKE ? OR related_identifier LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var events []*ChatMockMailEvent
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func GetChatMockMailEventByID(id int) (*ChatMockMailEvent, error) {
	var event ChatMockMailEvent
	err := DB.Where("id = ?", id).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (e *ChatMockMailEvent) UpdateParsedFields() error {
	if e == nil || e.Id <= 0 {
		return nil
	}
	e.UpdatedTime = common.GetTimestamp()
	return DB.Model(&ChatMockMailEvent{}).
		Where("id = ?", e.Id).
		Updates(map[string]any{
			"event_type":          e.EventType,
			"classification":      e.Classification,
			"related_identifier":  e.RelatedIdentifier,
			"confidence":          e.Confidence,
			"code":                e.Code,
			"parsed_payload":      e.ParsedPayload,
			"raw_snippet":         e.RawSnippet,
			"status":              e.Status,
			"updated_time":        e.UpdatedTime,
		}).Error
}

func GetLatestVerificationCodeEvent(mailboxID int) (*ChatMockMailEvent, error) {
	var event ChatMockMailEvent
	err := DB.Where("mailbox_id = ? AND code <> ''", mailboxID).
		Order("id DESC").
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func ExistsChatMockMailEvent(mailboxID int, uid uint32, eventType string) (bool, error) {
	var count int64
	err := DB.Model(&ChatMockMailEvent{}).
		Where("mailbox_id = ? AND message_uid = ? AND event_type = ?", mailboxID, uid, eventType).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
