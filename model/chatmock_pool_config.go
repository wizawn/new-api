package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"gorm.io/gorm"
)

// ChatMockPoolConfig 存储账号池调度配置。
type ChatMockPoolConfig struct {
	Id                       int    `json:"id"`
	EnableWeighted           bool   `json:"enable_weighted" gorm:"not null;default:true"`
	DefaultCooldownSecond    int    `json:"default_cooldown_second" gorm:"not null;default:300"`
	RequestTimeoutSecond     int    `json:"request_timeout_second" gorm:"not null;default:60"`
	MaxRetry                 int    `json:"max_retry" gorm:"not null;default:3"`
	HealthCheckInterval      int    `json:"health_check_interval" gorm:"not null;default:300"`
	EntitlementCheckInterval int    `json:"entitlement_check_interval" gorm:"not null;default:3600"`
	DefaultInstructions      string `json:"default_instructions" gorm:"type:text"`
	CreatedTime              int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime              int64  `json:"updated_time" gorm:"bigint"`
}

func (ChatMockPoolConfig) TableName() string {
	return "chatmock_pool_configs"
}

func GetOrInitChatMockPoolConfig() (*ChatMockPoolConfig, error) {
	var cfg ChatMockPoolConfig
	err := DB.Where("id = ?", 1).First(&cfg).Error
	if err == nil {
		cfg.Normalize()
		return &cfg, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	cfg = ChatMockPoolConfig{
		Id:                       1,
		EnableWeighted:           true,
		DefaultCooldownSecond:    setting.ChatMockDefaultCooldownSeconds,
		RequestTimeoutSecond:     setting.ChatMockDefaultRequestTimeoutSec,
		MaxRetry:                 3,
		HealthCheckInterval:      setting.ChatMockDefaultHealthIntervalSec,
		EntitlementCheckInterval: setting.ChatMockEntitlementIntervalSec,
		DefaultInstructions:      setting.ChatMockDefaultInstructions,
		CreatedTime:              common.GetTimestamp(),
		UpdatedTime:              common.GetTimestamp(),
	}
	cfg.Normalize()
	if createErr := DB.Create(&cfg).Error; createErr != nil {
		return nil, createErr
	}
	return &cfg, nil
}

func (c *ChatMockPoolConfig) Normalize() {
	if c.DefaultCooldownSecond <= 0 {
		c.DefaultCooldownSecond = setting.ChatMockDefaultCooldownSeconds
	}
	if c.RequestTimeoutSecond <= 0 {
		c.RequestTimeoutSecond = setting.ChatMockDefaultRequestTimeoutSec
	}
	if c.MaxRetry <= 0 {
		c.MaxRetry = 3
	}
	if c.HealthCheckInterval <= 0 {
		c.HealthCheckInterval = setting.ChatMockDefaultHealthIntervalSec
	}
	if c.EntitlementCheckInterval < 0 {
		c.EntitlementCheckInterval = setting.ChatMockEntitlementIntervalSec
	}
	c.DefaultInstructions = strings.TrimSpace(c.DefaultInstructions)
	if c.DefaultInstructions == "" {
		c.DefaultInstructions = setting.ChatMockDefaultInstructions
	}
}

func (c *ChatMockPoolConfig) Save() error {
	c.Normalize()
	c.Id = 1
	c.UpdatedTime = common.GetTimestamp()
	if c.CreatedTime == 0 {
		c.CreatedTime = c.UpdatedTime
	}
	return DB.Model(&ChatMockPoolConfig{}).Where("id = ?", 1).
		Select(
			"enable_weighted",
			"default_cooldown_second",
			"request_timeout_second",
			"max_retry",
			"health_check_interval",
			"entitlement_check_interval",
			"default_instructions",
			"updated_time",
		).Updates(c).Error
}
