package model

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

// ChatMockAccount 存储 ChatMock 账号池账号。
// 仅用于 chatmock_ 前缀数据表，与 NewAPI 原生表完全隔离。
type ChatMockAccount struct {
	Id                     int            `json:"id"`
	Name                   string         `json:"name" gorm:"size:128;not null"`
	Account                string         `json:"account" gorm:"size:255;not null;uniqueIndex:uk_chatmock_account_delete_at,priority:1"`
	ApiKey                 string         `json:"api_key,omitempty" gorm:"type:text;not null"`
	IdToken                string         `json:"id_token,omitempty" gorm:"type:text"`
	RefreshToken           string         `json:"refresh_token,omitempty" gorm:"type:text"`
	TokenExpiredAt         int64          `json:"token_expired_at" gorm:"bigint;not null;default:0"`
	TokenLastRefreshAt     int64          `json:"token_last_refresh_at" gorm:"bigint;not null;default:0"`
	BaseURL                string         `json:"base_url" gorm:"size:255;not null;default:'https://api.openai.com'"`
	ProxyURL               string         `json:"proxy_url,omitempty" gorm:"size:255"`
	Status                 string         `json:"status" gorm:"size:32;not null;default:'enabled';index"`
	Weight                 int            `json:"weight" gorm:"not null;default:1"`
	Priority               int            `json:"priority" gorm:"not null;default:100"`
	MaxConcurrency         int            `json:"max_concurrency" gorm:"not null;default:2"`
	RPM                    int            `json:"rpm" gorm:"not null;default:60"`
	DailyTokenLimit        int64          `json:"daily_token_limit" gorm:"not null;default:0"`
	DailyTokenUsed         int64          `json:"daily_token_used" gorm:"not null;default:0"`
	DailyTokenDate         string         `json:"daily_token_date" gorm:"size:8;not null;default:''"`
	CooldownUntil          int64          `json:"cooldown_until" gorm:"bigint;not null;default:0;index"`
	LastHealthAt           int64          `json:"last_health_at" gorm:"bigint;not null;default:0"`
	LastError              string         `json:"last_error,omitempty" gorm:"type:text"`
	EntitlementTier        string         `json:"entitlement_tier" gorm:"size:32;not null;default:'unknown'"`
	EntitlementValid       bool           `json:"entitlement_valid" gorm:"not null;default:false"`
	Usage5hUsed            int64          `json:"usage_5h_used" gorm:"bigint;not null;default:0"`
	Usage5hLimit           int64          `json:"usage_5h_limit" gorm:"bigint;not null;default:0"`
	Usage5hResetAt         int64          `json:"usage_5h_reset_at" gorm:"bigint;not null;default:0"`
	UsageWeeklyUsed        int64          `json:"usage_weekly_used" gorm:"bigint;not null;default:0"`
	UsageWeeklyLimit       int64          `json:"usage_weekly_limit" gorm:"bigint;not null;default:0"`
	UsageWeeklyResetAt     int64          `json:"usage_weekly_reset_at" gorm:"bigint;not null;default:0"`
	UsageCodeReviewUsed    int64          `json:"usage_code_review_used" gorm:"bigint;not null;default:0"`
	UsageCodeReviewLimit   int64          `json:"usage_code_review_limit" gorm:"bigint;not null;default:0"`
	UsageCodeReviewResetAt int64          `json:"usage_code_review_reset_at" gorm:"bigint;not null;default:0"`
	EntitlementCheckedAt   int64          `json:"entitlement_checked_at" gorm:"bigint;not null;default:0"`
	EntitlementError       string         `json:"entitlement_error,omitempty" gorm:"type:text"`
	EntitlementRaw         string         `json:"entitlement_raw,omitempty" gorm:"type:text"`
	QuotaStatus            string         `json:"quota_status" gorm:"-"`
	WeeklyRemainingPct     *float64       `json:"weekly_remaining_percent,omitempty" gorm:"-"`
	RouteScore             int64          `json:"route_score" gorm:"-"`
	SupportedModels        string         `json:"supported_models" gorm:"type:text"`
	Metadata               string         `json:"metadata,omitempty" gorm:"type:text"`
	CreatedTime            int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime            int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt              gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_chatmock_account_delete_at,priority:2"`
}

func (ChatMockAccount) TableName() string {
	return "chatmock_accounts"
}

func (a *ChatMockAccount) Normalize() {
	a.Name = strings.TrimSpace(a.Name)
	a.Account = strings.TrimSpace(a.Account)
	a.ApiKey = strings.TrimSpace(a.ApiKey)
	a.IdToken = strings.TrimSpace(a.IdToken)
	a.RefreshToken = strings.TrimSpace(a.RefreshToken)
	a.BaseURL = strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	a.ProxyURL = strings.TrimSpace(a.ProxyURL)
	a.Status = strings.TrimSpace(strings.ToLower(a.Status))
	if a.Status == "" {
		a.Status = constant.ChatMockAccountStatusEnabled
	}
	if a.Status != constant.ChatMockAccountStatusEnabled &&
		a.Status != constant.ChatMockAccountStatusDisabled &&
		a.Status != constant.ChatMockAccountStatusCooling &&
		a.Status != constant.ChatMockAccountStatusAbnormal &&
		a.Status != constant.ChatMockAccountStatusQuotaLow &&
		a.Status != constant.ChatMockAccountStatusQuotaExhausted &&
		a.Status != constant.ChatMockAccountStatusAuthInvalid &&
		a.Status != constant.ChatMockAccountStatusNetworkIssue &&
		a.Status != constant.ChatMockAccountStatusManualReview {
		a.Status = constant.ChatMockAccountStatusDisabled
	}
	if a.Weight <= 0 {
		a.Weight = 1
	}
	if a.Priority <= 0 {
		a.Priority = 100
	}
	if a.MaxConcurrency < 0 {
		a.MaxConcurrency = 0
	}
	if a.RPM < 0 {
		a.RPM = 0
	}
	if a.DailyTokenLimit < 0 {
		a.DailyTokenLimit = 0
	}
	if a.DailyTokenUsed < 0 {
		a.DailyTokenUsed = 0
	}
}

func (a *ChatMockAccount) ValidateForSave() error {
	if a.Name == "" {
		return fmt.Errorf("账号名称不能为空")
	}
	if a.Account == "" {
		return fmt.Errorf("账号标识不能为空")
	}
	if a.ApiKey == "" {
		return fmt.Errorf("API Key 不能为空")
	}
	if a.BaseURL == "" {
		return fmt.Errorf("Base URL 不能为空")
	}
	return nil
}

func (a *ChatMockAccount) Insert() error {
	a.Normalize()
	if err := a.ValidateForSave(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	a.CreatedTime = now
	a.UpdatedTime = now
	if a.DailyTokenDate == "" {
		a.DailyTokenDate = time.Now().UTC().Format("20060102")
	}
	return DB.Create(a).Error
}

func (a *ChatMockAccount) Update() error {
	a.Normalize()
	if a.Id == 0 {
		return fmt.Errorf("缺少账号 ID")
	}
	if err := a.ValidateForSave(); err != nil {
		return err
	}
	a.UpdatedTime = common.GetTimestamp()
	return DB.Model(&ChatMockAccount{}).Where("id = ?", a.Id).
		Select(
			"name", "account", "api_key", "id_token", "refresh_token",
			"token_expired_at", "token_last_refresh_at",
			"base_url", "proxy_url", "status",
			"weight", "priority", "max_concurrency", "rpm", "daily_token_limit",
			"supported_models", "metadata", "updated_time",
		).Updates(a).Error
}

func (a *ChatMockAccount) Delete() error {
	return DB.Delete(a).Error
}

func GetChatMockAccountByID(id int) (*ChatMockAccount, error) {
	var account ChatMockAccount
	err := DB.Where("id = ?", id).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func GetChatMockAccountByIdentity(account string) (*ChatMockAccount, error) {
	var data ChatMockAccount
	err := DB.Where("account = ?", strings.TrimSpace(account)).First(&data).Error
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func ListChatMockAccounts(keyword string, status string, tier string, quotaStatus string, offset int, limit int) ([]*ChatMockAccount, int64, error) {
	db := applyChatMockAccountFilters(DB.Model(&ChatMockAccount{}), keyword, status, tier, quotaStatus)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*ChatMockAccount
	query := db.Order("priority ASC, weight DESC, id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	for _, item := range list {
		if item == nil {
			continue
		}
		item.FillDerivedQuotaFields()
	}
	return list, total, nil
}

func ListChatMockAccountsForSummary(keyword string, status string, tier string, quotaStatus string) ([]*ChatMockAccount, error) {
	db := applyChatMockAccountFilters(DB.Model(&ChatMockAccount{}), keyword, status, tier, quotaStatus)
	var list []*ChatMockAccount
	if err := db.Order("priority ASC, weight DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		if item == nil {
			continue
		}
		item.FillDerivedQuotaFields()
	}
	return list, nil
}

func applyChatMockAccountFilters(db *gorm.DB, keyword string, status string, tier string, quotaStatus string) *gorm.DB {
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		db = db.Where("name LIKE ? OR account LIKE ? OR base_url LIKE ?", like, like, like)
	}
	if status != "" {
		db = db.Where("status = ?", strings.TrimSpace(strings.ToLower(status)))
	}
	if tier != "" {
		db = db.Where("LOWER(entitlement_tier) = ?", strings.TrimSpace(strings.ToLower(tier)))
	}
	switch strings.TrimSpace(strings.ToLower(quotaStatus)) {
	case "":
		// no-op
	case "error":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) <> ''")
	case "unknown":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit <= 0")
	case "exhausted":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit > 0 AND usage_weekly_used >= usage_weekly_limit")
	case "low":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit > 0 AND usage_weekly_used < usage_weekly_limit AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) <= 30")
	case "medium":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit > 0 AND usage_weekly_used < usage_weekly_limit AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) > 30 AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) <= 70")
	case "high":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit > 0 AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) > 70 AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) < 100")
	case "full":
		db = db.Where("TRIM(COALESCE(entitlement_error, '')) = '' AND usage_weekly_limit > 0 AND ((usage_weekly_limit - usage_weekly_used) * 100.0 / usage_weekly_limit) >= 100")
	}
	return db
}

func ListEnabledChatMockAccounts() ([]*ChatMockAccount, error) {
	var list []*ChatMockAccount
	err := DB.Where("status = ?", constant.ChatMockAccountStatusEnabled).
		Order("priority ASC, weight DESC, id ASC").
		Find(&list).Error
	return list, err
}

func (a *ChatMockAccount) GetSupportedModels() []string {
	if strings.TrimSpace(a.SupportedModels) == "" {
		return nil
	}
	var models []string
	if err := common.UnmarshalJsonStr(a.SupportedModels, &models); err != nil {
		return nil
	}
	for i := range models {
		models[i] = strings.TrimSpace(models[i])
	}
	return models
}

func (a *ChatMockAccount) SetSupportedModels(models []string) error {
	cleaned := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		cleaned = append(cleaned, name)
	}
	if len(cleaned) == 0 {
		a.SupportedModels = "[]"
		return nil
	}
	b, err := common.Marshal(cleaned)
	if err != nil {
		return err
	}
	a.SupportedModels = string(b)
	return nil
}

func (a *ChatMockAccount) SupportsModel(modelName string) bool {
	modelName = normalizeChatMockModelName(modelName)
	if modelName == "" {
		return true
	}
	models := a.GetSupportedModels()
	if len(models) == 0 {
		return true
	}
	candidates := expandChatMockModelCandidates(modelName)
	for _, raw := range models {
		spec := normalizeChatMockModelName(raw)
		if spec == "" {
			continue
		}
		if spec == "*" {
			return true
		}
		for _, candidate := range candidates {
			if matchChatMockModelPattern(spec, candidate) {
				return true
			}
		}
	}
	return false
}

func normalizeChatMockModelName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func expandChatMockModelCandidates(modelName string) []string {
	normalized := normalizeChatMockModelName(modelName)
	if normalized == "" {
		return nil
	}
	seen := map[string]struct{}{}
	queue := []string{}
	appendCandidate := func(value string) {
		v := normalizeChatMockModelName(value)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		queue = append(queue, v)
	}

	appendCandidate(normalized)
	for idx := 0; idx < len(queue); idx++ {
		current := queue[idx]
		if strings.HasSuffix(current, "-openai-compact") {
			appendCandidate(strings.TrimSuffix(current, "-openai-compact"))
		}
		trimmedDate := trimChatMockModelDateSuffix(current)
		if trimmedDate != current {
			appendCandidate(trimmedDate)
		}
	}
	return queue
}

func trimChatMockModelDateSuffix(modelName string) string {
	parts := strings.Split(modelName, "-")
	if len(parts) < 4 {
		return modelName
	}
	year := parts[len(parts)-3]
	month := parts[len(parts)-2]
	day := parts[len(parts)-1]
	if len(year) != 4 || len(month) != 2 || len(day) != 2 {
		return modelName
	}
	if !isAllDigits(year) || !isAllDigits(month) || !isAllDigits(day) {
		return modelName
	}
	return strings.Join(parts[:len(parts)-3], "-")
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// matchChatMockModelPattern performs a lightweight glob match where '*' matches
// zero or more characters.
func matchChatMockModelPattern(pattern string, model string) bool {
	pattern = normalizeChatMockModelName(pattern)
	model = normalizeChatMockModelName(model)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	pi, si := 0, 0
	starIdx := -1
	matchIdx := 0
	for si < len(model) {
		if pi < len(pattern) && pattern[pi] == model[si] {
			pi++
			si++
			continue
		}
		if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
			continue
		}
		if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
			continue
		}
		return false
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

func (a *ChatMockAccount) Masked() *ChatMockAccount {
	clone := *a
	if clone.ApiKey != "" {
		if len(clone.ApiKey) <= 10 {
			clone.ApiKey = "***"
		} else {
			clone.ApiKey = clone.ApiKey[:6] + "***" + clone.ApiKey[len(clone.ApiKey)-4:]
		}
	}
	if clone.IdToken != "" {
		clone.IdToken = "***"
	}
	if clone.RefreshToken != "" {
		clone.RefreshToken = "***"
	}
	clone.FillDerivedQuotaFields()
	return &clone
}

func (a *ChatMockAccount) WeeklyRemainingPercent() *float64 {
	if a == nil {
		return nil
	}
	return remainingPercentWithReset(
		a.UsageWeeklyUsed,
		a.UsageWeeklyLimit,
		a.UsageWeeklyResetAt,
		common.GetTimestamp(),
	)
}

func remainingPercentWithReset(used int64, limit int64, resetAt int64, now int64) *float64 {
	if limit <= 0 {
		return nil
	}
	if resetAt > 0 && now >= resetAt {
		full := 100.0
		return &full
	}
	remaining := float64(limit-used) * 100 / float64(limit)
	if math.IsNaN(remaining) || math.IsInf(remaining, 0) {
		return nil
	}
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}
	rounded := math.Round(remaining*100) / 100
	return &rounded
}

func (a *ChatMockAccount) DeriveQuotaStatus() string {
	if a == nil {
		return "unknown"
	}
	if strings.TrimSpace(a.EntitlementError) != "" {
		return "error"
	}
	remaining := a.WeeklyRemainingPercent()
	if remaining == nil {
		return "unknown"
	}
	if *remaining <= 0 {
		return "exhausted"
	}
	if *remaining <= 30 {
		return "low"
	}
	if *remaining <= 70 {
		return "medium"
	}
	if *remaining < 100 {
		return "high"
	}
	return "full"
}

func (a *ChatMockAccount) FillDerivedQuotaFields() {
	if a == nil {
		return
	}
	a.WeeklyRemainingPct = a.WeeklyRemainingPercent()
	a.QuotaStatus = a.DeriveQuotaStatus()
}
