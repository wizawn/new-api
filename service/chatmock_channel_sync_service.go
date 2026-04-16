package service

import (
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	ChatMockPoolChannelKey  = "chatmock_pool"
	ChatMockPoolChannelName = "ChatMockPool"
)

var chatMockPoolChannelSyncMu sync.Mutex

var chatMockPoolBaseModels = []string{
	"gpt-5", "gpt-5-codex", "gpt-5-codex-mini",
	"gpt-5.1", "gpt-5.1-codex", "gpt-5.1-codex-max", "gpt-5.1-codex-mini",
	"gpt-5.2", "gpt-5.2-codex", "gpt-5.3-codex", "gpt-5.3-codex-spark",
	"gpt-5.4",
}

func chatMockPoolBaselineModels() []string {
	seen := make(map[string]struct{}, len(chatMockPoolBaseModels)*2)
	models := make([]string, 0, len(chatMockPoolBaseModels)*2)
	appendModel := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		models = append(models, trimmed)
	}
	for _, modelName := range chatMockPoolBaseModels {
		appendModel(modelName)
		appendModel(ratio_setting.WithCompactModelSuffix(modelName))
	}
	return models
}

func CollectChatMockPoolChannelModels() ([]string, error) {
	accounts := make([]*model.ChatMockAccount, 0, 64)
	if err := model.DB.Order("id ASC").Find(&accounts).Error; err != nil {
		return nil, err
	}

	modelSet := make(map[string]struct{}, 64)
	explicitModelSet := make(map[string]struct{}, 32)
	appendModel := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		modelSet[trimmed] = struct{}{}
	}

	for _, account := range accounts {
		if account == nil {
			continue
		}
		for _, modelName := range account.GetSupportedModels() {
			normalized := strings.ToLower(strings.TrimSpace(modelName))
			if normalized == "" {
				continue
			}
			explicitModelSet[normalized] = struct{}{}
			appendModel(modelName)
		}
	}

	for _, baseline := range chatMockPoolBaselineModels() {
		lower := strings.ToLower(strings.TrimSpace(baseline))
		if isChatMockPoolExperimentalModelName(lower) {
			if _, ok := explicitModelSet[lower]; !ok {
				continue
			}
		}
		appendModel(baseline)
	}

	models := make([]string, 0, len(modelSet))
	for modelName := range modelSet {
		models = append(models, modelName)
	}
	sort.Strings(models)
	return models, nil
}

func isChatMockPoolExperimentalModelName(modelName string) bool {
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return false
	}
	return strings.Contains(model, "-spark")
}

func FindChatMockPoolChannel() (*model.Channel, error) {
	channel := &model.Channel{}
	err := model.DB.
		Where("type = ? AND `key` = ?", constant.ChannelTypeChatMock, ChatMockPoolChannelKey).
		Order("id ASC").
		First(channel).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil, gorm.ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func SyncChatMockPoolChannel() (*model.Channel, bool, []string, error) {
	chatMockPoolChannelSyncMu.Lock()
	defer chatMockPoolChannelSyncMu.Unlock()

	models, err := CollectChatMockPoolChannelModels()
	if err != nil {
		return nil, false, nil, err
	}
	if len(models) == 0 {
		models = []string{"gpt-5.2", "gpt-5.3-codex"}
	}
	modelsText := strings.Join(models, ",")

	channel, findErr := FindChatMockPoolChannel()
	if findErr != nil && findErr != gorm.ErrRecordNotFound {
		return nil, false, nil, findErr
	}

	weight := uint(0)
	priority := int64(10)
	autoBan := 1
	settingText := "{}"
	modelMapping := ""

	if findErr == gorm.ErrRecordNotFound {
		channel = &model.Channel{
			Type:         constant.ChannelTypeChatMock,
			Key:          ChatMockPoolChannelKey,
			Status:       common.ChannelStatusEnabled,
			Name:         ChatMockPoolChannelName,
			Weight:       &weight,
			CreatedTime:  common.GetTimestamp(),
			Models:       modelsText,
			Group:        "default",
			ModelMapping: &modelMapping,
			Priority:     &priority,
			AutoBan:      &autoBan,
			Setting:      &settingText,
		}
		if err = channel.Insert(); err != nil {
			return nil, false, nil, err
		}
		model.InitChannelCache()
		return channel, true, models, nil
	}

	channel.Type = constant.ChannelTypeChatMock
	channel.Name = ChatMockPoolChannelName
	channel.Status = common.ChannelStatusEnabled
	channel.Models = modelsText
	if strings.TrimSpace(channel.Key) == "" {
		channel.Key = ChatMockPoolChannelKey
	}
	if strings.TrimSpace(channel.Group) == "" {
		channel.Group = "default"
	}
	if channel.Weight == nil {
		channel.Weight = &weight
	}
	if channel.Priority == nil {
		channel.Priority = &priority
	}
	if channel.AutoBan == nil {
		channel.AutoBan = &autoBan
	}
	if channel.Setting == nil || strings.TrimSpace(*channel.Setting) == "" {
		channel.Setting = &settingText
	}
	if channel.ModelMapping == nil {
		channel.ModelMapping = &modelMapping
	}
	if err = channel.Update(); err != nil {
		return nil, false, nil, err
	}
	model.InitChannelCache()
	return channel, false, models, nil
}
