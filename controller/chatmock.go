package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type chatMockAccountUpsertRequest struct {
	Name            *string `json:"name"`
	Account         *string `json:"account"`
	AccountID       *string `json:"account_id"`
	ApiKey          *string `json:"api_key"`
	AccessToken     *string `json:"access_token"`
	IdToken         *string `json:"id_token"`
	RefreshToken    *string `json:"refresh_token"`
	Expired         *string `json:"expired"`
	BaseURL         *string `json:"base_url"`
	ProxyURL        *string `json:"proxy_url"`
	Status          *string `json:"status"`
	Weight          *int    `json:"weight"`
	Priority        *int    `json:"priority"`
	MaxConcurrency  *int    `json:"max_concurrency"`
	RPM             *int    `json:"rpm"`
	DailyTokenLimit *int64  `json:"daily_token_limit"`
	SupportedModels any     `json:"supported_models"`
	Metadata        *string `json:"metadata"`
	Email           *string `json:"email"`
	Type            *string `json:"type"`
}

type chatMockPoolConfigUpdateRequest struct {
	EnableWeighted           *bool   `json:"enable_weighted"`
	DefaultCooldownSecond    *int    `json:"default_cooldown_second"`
	RequestTimeoutSecond     *int    `json:"request_timeout_second"`
	MaxRetry                 *int    `json:"max_retry"`
	HealthCheckInterval      *int    `json:"health_check_interval"`
	EntitlementCheckInterval *int    `json:"entitlement_check_interval"`
	DefaultInstructions      *string `json:"default_instructions"`
}

type chatMockBatchUpsertRequest struct {
	Items  []chatMockAccountUpsertRequest `json:"items"`
	Upsert *bool                          `json:"upsert"`
}

type chatMockMailboxBatchUpsertRequest struct {
	Items  []chatMockMailboxUpsertRequest `json:"items"`
	Upsert *bool                          `json:"upsert"`
}

type chatMockMailboxUpsertRequest struct {
	Name               *string `json:"name"`
	Email              *string `json:"email"`
	Username           *string `json:"username"`
	Password           *string `json:"password"`
	ImapHost           *string `json:"imap_host"`
	ImapPort           *int    `json:"imap_port"`
	ImapSecurity       *string `json:"imap_security"`
	SmtpHost           *string `json:"smtp_host"`
	SmtpPort           *int    `json:"smtp_port"`
	SmtpSecurity       *string `json:"smtp_security"`
	Status             *string `json:"status"`
	PollIntervalSecond *int    `json:"poll_interval_second"`
	Metadata           *string `json:"metadata"`
}

type chatMockRouteRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type chatMockRouteDiagnoseRequest struct {
	Model       string `json:"model"`
	ExcludedIDs []int  `json:"excluded_ids"`
}

type chatMockStatusUpdateRequest struct {
	Status string `json:"status"`
}

type chatMockAccountBatchFilter struct {
	Keyword     string `json:"keyword"`
	Status      string `json:"status"`
	Tier        string `json:"tier"`
	QuotaStatus string `json:"quota_status"`
}

type chatMockAccountBatchDeleteRequest struct {
	IDs     []int                       `json:"ids"`
	Filters *chatMockAccountBatchFilter `json:"filters"`
	All     bool                        `json:"all"`
}

type chatMockAccountBatchStatusRequest struct {
	IDs          []int                       `json:"ids"`
	Filters      *chatMockAccountBatchFilter `json:"filters"`
	All          bool                        `json:"all"`
	TargetStatus string                      `json:"target_status"`
}

type chatMockPullRequest struct {
	Limit      int `json:"limit"`
	TimeoutSec int `json:"timeout_sec"`
}

type chatMockBatchSummary struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

type chatMockBatchRefreshResult struct {
	Total         int     `json:"total"`
	Success       int     `json:"success"`
	Failed        int     `json:"failed"`
	Refreshed     int     `json:"refreshed"`
	Concurrency   int     `json:"concurrency"`
	RetryAttempts int     `json:"retry_attempts"`
	Details       []gin.H `json:"details"`
}

type chatMockBatchInspectResult struct {
	Total         int                  `json:"total"`
	Health        chatMockBatchSummary `json:"health"`
	Entitlement   chatMockBatchSummary `json:"entitlement"`
	Concurrency   int                  `json:"concurrency"`
	RetryAttempts int                  `json:"retry_attempts"`
	Details       []gin.H              `json:"details"`
}

const (
	chatMockPoolChannelName = "ChatMockPool"
	chatMockPoolChannelKey  = "chatmock_pool"
)

func parseChatMockBatchConcurrency(c *gin.Context, fallback int) int {
	if fallback <= 0 {
		fallback = 6
	}
	raw := strings.TrimSpace(c.DefaultQuery("concurrency", ""))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < 1 {
		return 1
	}
	if value > 32 {
		return 32
	}
	return value
}

func parseChatMockRetryAttempts(c *gin.Context, fallback int) int {
	if fallback <= 0 {
		fallback = 2
	}
	raw := strings.TrimSpace(c.DefaultQuery("retry_attempts", ""))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < 1 {
		return 1
	}
	if value > 5 {
		return 5
	}
	return value
}

func shouldRetryChatMockError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	transientKeywords := []string{
		"timeout",
		"timed out",
		"temporary",
		"temporarily",
		"try again",
		"system busy",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"i/o timeout",
		"tls handshake",
		"eof",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"too many requests",
		"rate limit",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
	}
	for _, keyword := range transientKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func normalizeUniquePositiveIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	result := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Ints(result)
	return result
}

func normalizeChatMockBatchFilter(filters *chatMockAccountBatchFilter) *chatMockAccountBatchFilter {
	if filters == nil {
		return nil
	}
	normalized := &chatMockAccountBatchFilter{
		Keyword:     strings.TrimSpace(filters.Keyword),
		Status:      strings.TrimSpace(strings.ToLower(filters.Status)),
		Tier:        strings.TrimSpace(strings.ToLower(filters.Tier)),
		QuotaStatus: strings.TrimSpace(strings.ToLower(filters.QuotaStatus)),
	}
	if normalized.Keyword == "" &&
		normalized.Status == "" &&
		normalized.Tier == "" &&
		normalized.QuotaStatus == "" {
		return nil
	}
	return normalized
}

func resolveChatMockAccountBatchIDs(ids []int, filters *chatMockAccountBatchFilter, includeAll bool) ([]int, error) {
	normalizedIDs := normalizeUniquePositiveIDs(ids)
	if len(normalizedIDs) > 0 {
		return normalizedIDs, nil
	}
	normalizedFilters := normalizeChatMockBatchFilter(filters)
	if normalizedFilters == nil && !includeAll {
		return nil, nil
	}
	if normalizedFilters == nil {
		normalizedFilters = &chatMockAccountBatchFilter{}
	}
	items, _, err := model.ListChatMockAccounts(
		normalizedFilters.Keyword,
		normalizedFilters.Status,
		normalizedFilters.Tier,
		normalizedFilters.QuotaStatus,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(items))
	for _, item := range items {
		if item == nil || item.Id <= 0 {
			continue
		}
		result = append(result, item.Id)
	}
	return normalizeUniquePositiveIDs(result), nil
}

func normalizeChatMockAccountStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case constant.ChatMockAccountStatusEnabled,
		constant.ChatMockAccountStatusDisabled,
		constant.ChatMockAccountStatusCooling,
		constant.ChatMockAccountStatusAbnormal,
		constant.ChatMockAccountStatusQuotaLow,
		constant.ChatMockAccountStatusQuotaExhausted,
		constant.ChatMockAccountStatusAuthInvalid,
		constant.ChatMockAccountStatusNetworkIssue,
		constant.ChatMockAccountStatusManualReview:
		return status, true
	default:
		return "", false
	}
}

func runChatMockWithRetry(attempts int, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if i >= attempts-1 || !shouldRetryChatMockError(err) {
			break
		}
		time.Sleep(time.Duration(i+1) * 300 * time.Millisecond)
	}
	return lastErr
}

func getChatMockRequestTimeoutSecond() int {
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	return timeoutSec
}

func performChatMockBatchRefresh(concurrency int, retryAttempts int) (*chatMockBatchRefreshResult, error) {
	accounts, err := model.ListEnabledChatMockAccounts()
	if err != nil {
		return nil, err
	}
	timeoutSec := getChatMockRequestTimeoutSecond()
	result := &chatMockBatchRefreshResult{
		Concurrency:   concurrency,
		RetryAttempts: retryAttempts,
		Details:       make([]gin.H, 0, len(accounts)),
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, account := range accounts {
		if account == nil {
			continue
		}
		result.Total++
		wg.Add(1)
		go func(ac *model.ChatMockAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				<-sem
			}()
			refreshed := false
			refreshErr := runChatMockWithRetry(retryAttempts, func() error {
				done, err := service.RefreshChatMockAccountAccessToken(ac, true, timeoutSec)
				if done {
					refreshed = true
				}
				return err
			})
			mu.Lock()
			defer mu.Unlock()
			if refreshErr != nil {
				result.Details = append(result.Details, gin.H{
					"id":      ac.Id,
					"account": ac.Account,
					"message": refreshErr.Error(),
				})
				return
			}
			result.Success++
			if refreshed {
				result.Refreshed++
			}
		}(account)
	}
	wg.Wait()
	result.Failed = len(result.Details)
	return result, nil
}

func performChatMockBatchInspect(concurrency int, retryAttempts int) (*chatMockBatchInspectResult, error) {
	accounts, err := model.ListEnabledChatMockAccounts()
	if err != nil {
		return nil, err
	}
	timeoutSec := getChatMockRequestTimeoutSecond()
	result := &chatMockBatchInspectResult{
		Concurrency:   concurrency,
		RetryAttempts: retryAttempts,
		Details:       make([]gin.H, 0, len(accounts)),
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, account := range accounts {
		if account == nil {
			continue
		}
		result.Total++
		wg.Add(1)
		go func(ac *model.ChatMockAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				<-sem
			}()
			item := gin.H{
				"id":      ac.Id,
				"account": ac.Account,
				"name":    ac.Name,
			}
			healthErr := runChatMockWithRetry(retryAttempts, func() error {
				return service.HealthCheckChatMockAccount(ac, timeoutSec)
			})
			if healthErr != nil {
				mu.Lock()
				result.Health.Failed++
				item["health"] = "failed"
				item["health_error"] = healthErr.Error()
				result.Details = append(result.Details, item)
				mu.Unlock()
				return
			}

			entitlementErr := runChatMockWithRetry(retryAttempts, func() error {
				_, err := service.RefreshChatMockAccountEntitlement(ac, timeoutSec)
				return err
			})
			mu.Lock()
			defer mu.Unlock()
			result.Health.Success++
			item["health"] = "ok"
			if entitlementErr != nil {
				result.Entitlement.Failed++
				item["entitlement"] = "failed"
				item["entitlement_error"] = entitlementErr.Error()
				result.Details = append(result.Details, item)
				return
			}
			result.Entitlement.Success++
			item["entitlement"] = "ok"
			result.Details = append(result.Details, item)
		}(account)
	}
	wg.Wait()
	return result, nil
}

func collectChatMockPoolChannelModels() ([]string, error) {
	return service.CollectChatMockPoolChannelModels()
}

func isChatMockExperimentalModelName(modelName string) bool {
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return false
	}
	return strings.Contains(model, "-spark")
}

func syncChatMockPoolChannel() (*model.Channel, bool, []string, error) {
	return service.SyncChatMockPoolChannel()
}

func findChatMockPoolChannel() (*model.Channel, error) {
	return service.FindChatMockPoolChannel()
}

func splitAndDedupeChatMockModels(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func buildChatMockModelSet(models []string) map[string]string {
	set := make(map[string]string, len(models))
	for _, modelName := range models {
		trimmed := strings.TrimSpace(modelName)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, exists := set[lower]; !exists {
			set[lower] = trimmed
		}
	}
	return set
}

func diffChatMockModels(current []string, recommended []string) ([]string, []string) {
	currentSet := buildChatMockModelSet(current)
	recommendedSet := buildChatMockModelSet(recommended)

	missing := make([]string, 0)
	for key, value := range recommendedSet {
		if _, exists := currentSet[key]; exists {
			continue
		}
		missing = append(missing, value)
	}
	extra := make([]string, 0)
	for key, value := range currentSet {
		if _, exists := recommendedSet[key]; exists {
			continue
		}
		extra = append(extra, value)
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

func chatMockChannelStatusText(status int) string {
	switch status {
	case common.ChannelStatusEnabled:
		return "enabled"
	case common.ChannelStatusManuallyDisabled:
		return "disabled"
	case common.ChannelStatusAutoDisabled:
		return "auto_disabled"
	default:
		return "unknown"
	}
}

func tryAutoSyncChatMockPoolChannel() {
	if !setting.IsChatMockEnabled() {
		return
	}
	if _, _, _, err := syncChatMockPoolChannel(); err != nil {
		common.SysError("ChatMock 账号池自动同步渠道失败: " + err.Error())
	}
}

func RouteChatMockAccount(c *gin.Context) {
	if !setting.IsChatMockEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "CHATMOCK_ENABLED=false，账号池未启用",
		})
		return
	}

	maxMB := constant.MaxRequestBodyMB
	if maxMB <= 0 {
		maxMB = 128
	}
	maxBytes := int64(maxMB) << 20
	rawBody, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "读取请求体失败: " + err.Error(),
		})
		return
	}
	if int64(len(rawBody)) > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success": false,
			"message": "请求体过大",
		})
		return
	}
	if len(rawBody) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体不能为空",
		})
		return
	}

	request := chatMockRouteRequest{}
	if err = common.Unmarshal(rawBody, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体不是有效的 JSON",
		})
		return
	}
	openAIRequest := dto.GeneralOpenAIRequest{}
	if err = common.Unmarshal(rawBody, &openAIRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体不是有效的 JSON",
		})
		return
	}

	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(openAIRequest.Model)
	}
	shouldStream := request.Stream
	if openAIRequest.Stream != nil {
		shouldStream = *openAIRequest.Stream
	}
	includeUsage := false
	if openAIRequest.StreamOptions != nil {
		includeUsage = openAIRequest.StreamOptions.IncludeUsage
	}

	upstreamResp, account, routeErr := service.RouteChatMockChatCompletion(c.Request.Context(), rawBody, modelName, c.Request.Header)
	if routeErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": routeErr.Error(),
		})
		return
	}
	if account == nil || upstreamResp == nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "账号池路由结果无效",
		})
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(upstreamResp.Header.Get("Content-Type")))
	isStream := strings.Contains(contentType, "text/event-stream")
	isCodexAccount := service.IsLikelyChatMockCodexAccount(account)

	// Codex compatibility path:
	// - upstream is forced to responses stream=true
	// - stream request => convert responses SSE to chat.completion.chunk SSE
	// - non-stream request => aggregate to response.completed and convert to chat/completions JSON
	//
	// Some codex-compatible upstreams incorrectly return text/plain for SSE payloads.
	// For codex accounts we rely on protocol behavior instead of Content-Type.
	if isCodexAccount && upstreamResp.StatusCode == http.StatusOK {
		if shouldStream {
			info := &relaycommon.RelayInfo{
				IsStream:           true,
				RelayFormat:        types.RelayFormatOpenAI,
				ShouldIncludeUsage: includeUsage,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: modelName,
				},
			}
			usage, streamErr := openaichannel.OaiResponsesToChatStreamHandler(c, info, upstreamResp)
			if streamErr != nil {
				service.RecordChatMockAccountFailure(account.Id, "Codex 流式兼容转换失败: "+streamErr.Error(), setting.ChatMockDefaultCooldownSeconds)
				return
			}
			tokenUsed := int64(0)
			if usage != nil && usage.TotalTokens > 0 {
				tokenUsed = int64(usage.TotalTokens)
			}
			service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
			return
		}

		payload, tokenUsed, convertErr := service.ConvertChatMockCodexSSEToChatCompletion(upstreamResp.Body, "chatmock-"+common.GetUUID())
		_ = upstreamResp.Body.Close()
		if convertErr != nil {
			service.RecordChatMockAccountFailure(account.Id, "Codex 非流式兼容转换失败: "+convertErr.Error(), setting.ChatMockDefaultCooldownSeconds)
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"message": convertErr.Error(),
			})
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(payload)
		service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
		return
	}

	defer func() {
		_ = upstreamResp.Body.Close()
	}()

	copyChatMockUpstreamHeaders(c.Writer.Header(), upstreamResp.Header)
	c.Status(upstreamResp.StatusCode)

	if isStream {
		if _, err = io.Copy(c.Writer, upstreamResp.Body); err != nil {
			service.RecordChatMockAccountFailure(account.Id, "流式响应回写失败: "+err.Error(), setting.ChatMockDefaultCooldownSeconds)
			return
		}
		service.RecordChatMockAccountSuccess(account.Id, 0)
		return
	}

	payload, readErr := io.ReadAll(upstreamResp.Body)
	if readErr != nil {
		service.RecordChatMockAccountFailure(account.Id, "读取上游响应失败: "+readErr.Error(), setting.ChatMockDefaultCooldownSeconds)
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "读取上游响应失败: " + readErr.Error(),
		})
		return
	}
	tokenUsed := service.ExtractTokenUsageFromOpenAIResponse(payload)
	service.RecordChatMockAccountSuccess(account.Id, tokenUsed)
	_, _ = c.Writer.Write(payload)
}

func AdminListChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	tier := strings.TrimSpace(strings.ToLower(c.Query("tier")))
	quotaStatus := strings.TrimSpace(strings.ToLower(c.Query("quota_status")))
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListChatMockAccounts(
		keyword,
		status,
		tier,
		quotaStatus,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	masked := make([]*model.ChatMockAccount, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		maskedItem := item.Masked()
		maskedItem.RouteScore = service.EstimateChatMockRouteScore(item)
		masked = append(masked, maskedItem)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(masked)
	common.ApiSuccess(c, pageInfo)
}

func AdminSummaryChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	tier := strings.TrimSpace(strings.ToLower(c.Query("tier")))
	quotaStatus := strings.TrimSpace(strings.ToLower(c.Query("quota_status")))
	items, err := model.ListChatMockAccountsForSummary(keyword, status, tier, quotaStatus)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	entitlementInterval := int64(0)
	if cfg, cfgErr := service.GetChatMockPoolConfig(); cfgErr == nil && cfg != nil && cfg.EntitlementCheckInterval > 0 {
		entitlementInterval = int64(cfg.EntitlementCheckInterval)
	}

	statusCounts := map[string]int{}
	planCounts := map[string]int{}
	accountStatusCounts := map[string]int{}
	signalCounts := map[string]int{
		"auth":       0,
		"quota":      0,
		"rate_limit": 0,
		"network":    0,
		"unknown":    0,
	}
	exhaustedNames := make([]string, 0)
	lowNames := make([]string, 0)
	errorNames := make([]string, 0)
	authIssueNames := make([]string, 0)
	quotaIssueNames := make([]string, 0)
	rateLimitedNames := make([]string, 0)
	networkIssueNames := make([]string, 0)
	freeEquivalent7D := 0.0
	plusEquivalent7D := 0.0
	staleEntitlementAccounts := 0
	resetPassedAccounts := 0
	tokenExpiring24hAccounts := 0

	for _, item := range items {
		if item == nil {
			continue
		}
		quotaState := item.DeriveQuotaStatus()
		statusCounts[quotaState]++
		accountStatus := strings.TrimSpace(strings.ToLower(item.Status))
		if accountStatus == "" {
			accountStatus = "unknown"
		}
		accountStatusCounts[accountStatus]++
		if quotaState == "exhausted" {
			exhaustedNames = append(exhaustedNames, item.Name)
		}
		if quotaState == "low" {
			lowNames = append(lowNames, item.Name)
		}
		if quotaState == "error" || quotaState == "unknown" {
			errorNames = append(errorNames, item.Name)
		}
		signal := classifyChatMockIssueSignal(item, quotaState)
		if signal != "" {
			signalCounts[signal]++
			switch signal {
			case "auth":
				authIssueNames = append(authIssueNames, item.Name)
			case "quota":
				quotaIssueNames = append(quotaIssueNames, item.Name)
			case "rate_limit":
				rateLimitedNames = append(rateLimitedNames, item.Name)
			case "network":
				networkIssueNames = append(networkIssueNames, item.Name)
			}
		}

		plan := strings.TrimSpace(strings.ToLower(item.EntitlementTier))
		if plan == "" {
			plan = "unknown"
		}
		planCounts[plan]++

		remaining := item.WeeklyRemainingPercent()
		if remaining == nil {
			continue
		}
		switch plan {
		case "free":
			freeEquivalent7D += *remaining
		case "plus":
			plusEquivalent7D += *remaining
		}
		if isChatMockEntitlementDataStale(item, now, entitlementInterval) {
			staleEntitlementAccounts++
		}
		if isChatMockQuotaWindowCounterStale(item, now) {
			resetPassedAccounts++
		}
		if isChatMockTokenExpiringSoon(item, now, 24*3600) {
			tokenExpiring24hAccounts++
		}
	}

	signalSuggestions := make([]string, 0, 4)
	if signalCounts["auth"] > 0 {
		signalSuggestions = append(signalSuggestions, "检测到鉴权异常账号，建议先执行“刷新授权”，并补充 refresh_token 以支持自动续期")
	}
	if signalCounts["quota"] > 0 {
		signalSuggestions = append(signalSuggestions, "检测到额度耗尽账号，建议优先使用 Smart Switch 排序并清理已耗尽账号")
	}
	if signalCounts["rate_limit"] > 0 {
		signalSuggestions = append(signalSuggestions, "检测到限流账号，建议降低并发/RPM，或增加账号数量分摊请求")
	}
	if signalCounts["network"] > 0 {
		signalSuggestions = append(signalSuggestions, "检测到网络异常账号，建议检查 proxy_url、上游连通性与 TLS 配置")
	}

	common.ApiSuccess(c, gin.H{
		"accounts":                    len(items),
		"status_counts":               statusCounts,
		"plan_counts":                 planCounts,
		"account_status_counts":       accountStatusCounts,
		"exhausted_accounts":          statusCounts["exhausted"],
		"low_accounts":                statusCounts["low"],
		"error_accounts":              statusCounts["error"] + statusCounts["unknown"],
		"exhausted_names":             exhaustedNames,
		"low_names":                   lowNames,
		"error_names":                 errorNames,
		"signal_counts":               signalCounts,
		"auth_issue_accounts":         signalCounts["auth"],
		"quota_issue_accounts":        signalCounts["quota"],
		"rate_limited_accounts":       signalCounts["rate_limit"],
		"network_issue_accounts":      signalCounts["network"],
		"auth_issue_names":            authIssueNames,
		"quota_issue_names":           quotaIssueNames,
		"rate_limited_names":          rateLimitedNames,
		"network_issue_names":         networkIssueNames,
		"signal_suggestions":          signalSuggestions,
		"free_equivalent_7d":          freeEquivalent7D,
		"plus_equivalent_7d":          plusEquivalent7D,
		"stale_entitlement_accounts":  staleEntitlementAccounts,
		"reset_passed_accounts":       resetPassedAccounts,
		"token_expiring_24h_accounts": tokenExpiring24hAccounts,
	})
}

func classifyChatMockIssueSignal(account *model.ChatMockAccount, quotaState string) string {
	if quotaState == "exhausted" {
		return "quota"
	}
	if account == nil {
		if quotaState == "error" || quotaState == "unknown" {
			return "unknown"
		}
		return ""
	}
	signalText := strings.ToLower(strings.TrimSpace(account.LastError + " " + account.EntitlementError))
	if signalText == "" {
		if quotaState == "error" || quotaState == "unknown" {
			return "unknown"
		}
		return ""
	}
	if chatMockSignalContainsAny(signalText, []string{
		"status=401", "status=403", "unauthorized", "invalid token", "invalid_token",
		"token expired", "jwt expired", "session expired", "forbidden", "deactivated",
	}) {
		return "auth"
	}
	if chatMockSignalContainsAny(signalText, []string{
		"insufficient_quota", "quota exceeded", "quota exhausted", "usage_limit_reached",
		"limit_reached", "额度耗尽", "额度不足", "quota_exceeded",
	}) {
		return "quota"
	}
	if chatMockSignalContainsAny(signalText, []string{
		"status=429", "rate limit", "too many requests", "请求次数过多",
	}) {
		return "rate_limit"
	}
	if chatMockSignalContainsAny(signalText, []string{
		"timeout", "connection reset", "connection refused", "network is unreachable",
		"tls handshake", "i/o timeout", "dial tcp", "bad gateway", "service unavailable",
		"gateway timeout", "temporary", "temporarily",
	}) {
		return "network"
	}
	if quotaState == "error" || quotaState == "unknown" {
		return "unknown"
	}
	return ""
}

func chatMockSignalContainsAny(source string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(source, keyword) {
			return true
		}
	}
	return false
}

func isChatMockEntitlementDataStale(account *model.ChatMockAccount, now int64, intervalSec int64) bool {
	if account == nil {
		return false
	}
	checkedAt := account.EntitlementCheckedAt
	if checkedAt <= 0 {
		return true
	}
	if intervalSec <= 0 {
		return false
	}
	return now-checkedAt > intervalSec*2
}

func isChatMockQuotaWindowCounterStale(account *model.ChatMockAccount, now int64) bool {
	if account == nil {
		return false
	}
	if account.Usage5hLimit > 0 && account.Usage5hUsed >= account.Usage5hLimit && account.Usage5hResetAt > 0 && now >= account.Usage5hResetAt {
		return true
	}
	if account.UsageWeeklyLimit > 0 && account.UsageWeeklyUsed >= account.UsageWeeklyLimit && account.UsageWeeklyResetAt > 0 && now >= account.UsageWeeklyResetAt {
		return true
	}
	if account.UsageCodeReviewLimit > 0 && account.UsageCodeReviewUsed >= account.UsageCodeReviewLimit && account.UsageCodeReviewResetAt > 0 && now >= account.UsageCodeReviewResetAt {
		return true
	}
	return false
}

func isChatMockTokenExpiringSoon(account *model.ChatMockAccount, now int64, withinSec int64) bool {
	if account == nil {
		return false
	}
	if account.TokenExpiredAt <= 0 {
		return false
	}
	if withinSec <= 0 {
		withinSec = 24 * 3600
	}
	return account.TokenExpiredAt > now && account.TokenExpiredAt-now <= withinSec
}

func AdminCreateChatMockAccount(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	request := chatMockAccountUpsertRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	account := &model.ChatMockAccount{}
	fillChatMockAccountFromRequest(account, request)

	if !hasNonEmptyString(request.Name) || !hasNonEmptyString(request.Account) || !hasNonEmptyString(request.ApiKey) || !hasNonEmptyString(request.BaseURL) {
		common.ApiErrorMsg(c, "name/account/api_key/base_url 为必填项")
		return
	}
	models, err := parseSupportedModels(request.SupportedModels)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = account.SetSupportedModels(models); err != nil {
		common.ApiError(c, err)
		return
	}
	if err = account.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	tryAutoSyncChatMockPoolChannel()
	common.ApiSuccess(c, account.Masked())
}

func AdminBatchCreateChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	rawBody, err := io.ReadAll(io.LimitReader(c.Request.Body, 4<<20))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(rawBody) == 0 {
		common.ApiErrorMsg(c, "请求体不能为空")
		return
	}

	batchReq := chatMockBatchUpsertRequest{}
	items := make([]chatMockAccountUpsertRequest, 0, 32)
	if err = common.Unmarshal(rawBody, &batchReq); err == nil && len(batchReq.Items) > 0 {
		items = batchReq.Items
	}
	if len(items) == 0 {
		directItems := make([]chatMockAccountUpsertRequest, 0, 32)
		if err = common.Unmarshal(rawBody, &directItems); err != nil {
			common.ApiErrorMsg(c, "批量导入格式错误，需传数组或 {items:[...]}")
			return
		}
		items = directItems
	}
	if len(items) == 0 {
		common.ApiErrorMsg(c, "批量导入列表为空")
		return
	}
	if len(items) > 1000 {
		common.ApiErrorMsg(c, "单次批量导入最多 1000 条")
		return
	}

	upsert := true
	if batchReq.Upsert != nil {
		upsert = *batchReq.Upsert
	}

	successCount := 0
	failed := make([]gin.H, 0)
	for idx, request := range items {
		account := &model.ChatMockAccount{}
		fillChatMockAccountFromRequest(account, request)
		if strings.TrimSpace(account.Name) == "" ||
			strings.TrimSpace(account.Account) == "" ||
			strings.TrimSpace(account.ApiKey) == "" ||
			strings.TrimSpace(account.BaseURL) == "" {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"account": account.Account,
				"message": "name/account/api_key/base_url 为必填项",
			})
			continue
		}

		models, parseErr := parseSupportedModels(request.SupportedModels)
		if parseErr != nil {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"account": account.Account,
				"message": parseErr.Error(),
			})
			continue
		}
		if setErr := account.SetSupportedModels(models); setErr != nil {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"account": account.Account,
				"message": setErr.Error(),
			})
			continue
		}

		if upsert {
			existed, findErr := model.GetChatMockAccountByIdentity(account.Account)
			if findErr == nil && existed != nil {
				fillChatMockAccountFromRequest(existed, request)
				if setErr := existed.SetSupportedModels(models); setErr != nil {
					failed = append(failed, gin.H{
						"index":   idx + 1,
						"account": account.Account,
						"message": setErr.Error(),
					})
					continue
				}
				if updateErr := existed.Update(); updateErr != nil {
					failed = append(failed, gin.H{
						"index":   idx + 1,
						"account": account.Account,
						"message": updateErr.Error(),
					})
					continue
				}
				successCount++
				continue
			}
			if findErr != nil && findErr != gorm.ErrRecordNotFound {
				failed = append(failed, gin.H{
					"index":   idx + 1,
					"account": account.Account,
					"message": findErr.Error(),
				})
				continue
			}
		}

		if insertErr := account.Insert(); insertErr != nil {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"account": account.Account,
				"message": insertErr.Error(),
			})
			continue
		}
		successCount++
	}
	if successCount > 0 {
		tryAutoSyncChatMockPoolChannel()
	}

	common.ApiSuccess(c, gin.H{
		"total":   len(items),
		"success": successCount,
		"failed":  len(failed),
		"details": failed,
		"upsert":  upsert,
	})
}

func AdminUpdateChatMockAccount(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	account, err := model.GetChatMockAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request := chatMockAccountUpsertRequest{}
	if err = c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	fillChatMockAccountFromRequest(account, request)
	models, err := parseSupportedModels(request.SupportedModels)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if request.SupportedModels != nil {
		if err = account.SetSupportedModels(models); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err = account.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	tryAutoSyncChatMockPoolChannel()
	common.ApiSuccess(c, account.Masked())
}

func AdminDeleteChatMockAccount(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	account, err := model.GetChatMockAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = account.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	tryAutoSyncChatMockPoolChannel()
	common.ApiSuccess(c, gin.H{"id": id})
}

func AdminBatchDeleteChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	req := chatMockAccountBatchDeleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	targetIDs, err := resolveChatMockAccountBatchIDs(req.IDs, req.Filters, req.All)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(targetIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":         0,
			"matched_count":           0,
			"deleted_count":           0,
			"skipped_missing_count":   0,
			"deleted_snapshot_count":  0,
			"deleted_route_log_count": 0,
		})
		return
	}

	existingIDs := make([]int, 0, len(targetIDs))
	if err = model.DB.Model(&model.ChatMockAccount{}).
		Where("id IN ?", targetIDs).
		Pluck("id", &existingIDs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	existingIDs = normalizeUniquePositiveIDs(existingIDs)
	if len(existingIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":         len(targetIDs),
			"matched_count":           0,
			"deleted_count":           0,
			"skipped_missing_count":   len(targetIDs),
			"deleted_snapshot_count":  0,
			"deleted_route_log_count": 0,
		})
		return
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	snapshotResult := tx.Where("account_id IN ?", existingIDs).Delete(&model.ChatMockAccountSnapshot{})
	if snapshotResult.Error != nil {
		tx.Rollback()
		common.ApiError(c, snapshotResult.Error)
		return
	}
	routeLogResult := tx.Where("account_id IN ?", existingIDs).Delete(&model.ChatMockRouteLog{})
	if routeLogResult.Error != nil {
		tx.Rollback()
		common.ApiError(c, routeLogResult.Error)
		return
	}
	accountResult := tx.Where("id IN ?", existingIDs).Delete(&model.ChatMockAccount{})
	if accountResult.Error != nil {
		tx.Rollback()
		common.ApiError(c, accountResult.Error)
		return
	}
	if err = tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if accountResult.RowsAffected > 0 {
		tryAutoSyncChatMockPoolChannel()
	}
	common.ApiSuccess(c, gin.H{
		"requested_count":         len(targetIDs),
		"matched_count":           len(existingIDs),
		"deleted_count":           int(accountResult.RowsAffected),
		"skipped_missing_count":   len(targetIDs) - len(existingIDs),
		"deleted_snapshot_count":  int(snapshotResult.RowsAffected),
		"deleted_route_log_count": int(routeLogResult.RowsAffected),
	})
}

func AdminSetChatMockAccountStatus(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	request := chatMockStatusUpdateRequest{}
	if err = c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	status, ok := normalizeChatMockAccountStatus(request.Status)
	if !ok {
		common.ApiErrorMsg(c, "status 仅支持 enabled/disabled/cooling/abnormal/quota_low/quota_exhausted/auth_invalid/network_issue/manual_review")
		return
	}
	if updateErr := model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", id).Updates(map[string]any{
		"status":       status,
		"updated_time": common.GetTimestamp(),
	}).Error; updateErr != nil {
		common.ApiError(c, updateErr)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": status})
}

func AdminBatchSetChatMockAccountStatus(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	req := chatMockAccountBatchStatusRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	status, ok := normalizeChatMockAccountStatus(req.TargetStatus)
	if !ok {
		common.ApiErrorMsg(c, "target_status 仅支持 enabled/disabled/cooling/abnormal/quota_low/quota_exhausted/auth_invalid/network_issue/manual_review")
		return
	}
	targetIDs, err := resolveChatMockAccountBatchIDs(req.IDs, req.Filters, req.All)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(targetIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":       0,
			"matched_count":         0,
			"updated_count":         0,
			"skipped_missing_count": 0,
			"status":                status,
		})
		return
	}

	existingIDs := make([]int, 0, len(targetIDs))
	if err = model.DB.Model(&model.ChatMockAccount{}).
		Where("id IN ?", targetIDs).
		Pluck("id", &existingIDs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	existingIDs = normalizeUniquePositiveIDs(existingIDs)
	if len(existingIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":       len(targetIDs),
			"matched_count":         0,
			"updated_count":         0,
			"skipped_missing_count": len(targetIDs),
			"status":                status,
		})
		return
	}
	updateResult := model.DB.Model(&model.ChatMockAccount{}).
		Where("id IN ?", existingIDs).
		Updates(map[string]any{
			"status":       status,
			"updated_time": common.GetTimestamp(),
		})
	if updateResult.Error != nil {
		common.ApiError(c, updateResult.Error)
		return
	}
	common.ApiSuccess(c, gin.H{
		"requested_count":       len(targetIDs),
		"matched_count":         len(existingIDs),
		"updated_count":         int(updateResult.RowsAffected),
		"skipped_missing_count": len(targetIDs) - len(existingIDs),
		"status":                status,
	})
}

func AdminHealthCheckChatMockAccount(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	account, err := model.GetChatMockAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	if err = service.HealthCheckChatMockAccount(account, timeoutSec); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": "ok"})
}

func AdminEntitlementCheckChatMockAccount(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	account, err := model.GetChatMockAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	result, refreshErr := service.RefreshChatMockAccountEntitlement(account, timeoutSec)
	if refreshErr != nil {
		common.ApiError(c, refreshErr)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":     id,
		"result": result,
	})
}

func AdminRefreshChatMockAccountToken(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "账号 ID 无效")
		return
	}
	account, err := model.GetChatMockAccountByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	refreshed, refreshErr := service.RefreshChatMockAccountAccessToken(account, true, timeoutSec)
	if refreshErr != nil {
		common.ApiError(c, refreshErr)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":        id,
		"refreshed": refreshed,
	})
}

func AdminBatchHealthCheckChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	accounts, err := model.ListEnabledChatMockAccounts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	concurrency := parseChatMockBatchConcurrency(c, 6)
	retryAttempts := parseChatMockRetryAttempts(c, 2)

	successCount := 0
	failed := make([]gin.H, 0, len(accounts))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0

	for _, account := range accounts {
		if account == nil {
			continue
		}
		total++
		wg.Add(1)
		go func(ac *model.ChatMockAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			checkErr := runChatMockWithRetry(retryAttempts, func() error {
				return service.HealthCheckChatMockAccount(ac, timeoutSec)
			})
			mu.Lock()
			defer mu.Unlock()
			if checkErr != nil {
				failed = append(failed, gin.H{
					"id":      ac.Id,
					"account": ac.Account,
					"message": checkErr.Error(),
				})
				return
			}
			successCount++
		}(account)
	}
	wg.Wait()

	common.ApiSuccess(c, gin.H{
		"total":          total,
		"success":        successCount,
		"failed":         len(failed),
		"concurrency":    concurrency,
		"retry_attempts": retryAttempts,
		"details":        failed,
	})
}

func AdminBatchRefreshChatMockAccountTokens(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	concurrency := parseChatMockBatchConcurrency(c, 6)
	retryAttempts := parseChatMockRetryAttempts(c, 2)
	result, err := performChatMockBatchRefresh(concurrency, retryAttempts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminBatchEntitlementCheckChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	accounts, err := model.ListEnabledChatMockAccounts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, cfgErr := service.GetChatMockPoolConfig()
	timeoutSec := setting.ChatMockDefaultRequestTimeoutSec
	if cfgErr == nil && cfg != nil && cfg.RequestTimeoutSecond > 0 {
		timeoutSec = cfg.RequestTimeoutSecond
	}
	concurrency := parseChatMockBatchConcurrency(c, 6)
	retryAttempts := parseChatMockRetryAttempts(c, 2)

	successCount := 0
	failed := make([]gin.H, 0, len(accounts))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0

	for _, account := range accounts {
		if account == nil {
			continue
		}
		total++
		wg.Add(1)
		go func(ac *model.ChatMockAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				<-sem
			}()
			refreshErr := runChatMockWithRetry(retryAttempts, func() error {
				_, err := service.RefreshChatMockAccountEntitlement(ac, timeoutSec)
				return err
			})
			mu.Lock()
			defer mu.Unlock()
			if refreshErr != nil {
				failed = append(failed, gin.H{
					"id":      ac.Id,
					"account": ac.Account,
					"message": refreshErr.Error(),
				})
				return
			}
			successCount++
		}(account)
	}
	wg.Wait()

	common.ApiSuccess(c, gin.H{
		"total":          total,
		"success":        successCount,
		"failed":         len(failed),
		"concurrency":    concurrency,
		"retry_attempts": retryAttempts,
		"details":        failed,
	})
}

func AdminInspectChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	concurrency := parseChatMockBatchConcurrency(c, 6)
	retryAttempts := parseChatMockRetryAttempts(c, 2)
	result, err := performChatMockBatchInspect(concurrency, retryAttempts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminStabilizeChatMockPool(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	concurrency := parseChatMockBatchConcurrency(c, 6)
	retryAttempts := parseChatMockRetryAttempts(c, 2)

	refreshResult, err := performChatMockBatchRefresh(concurrency, retryAttempts)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	inspectResult, err := performChatMockBatchInspect(concurrency, retryAttempts)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channel, created, models, err := syncChatMockPoolChannel()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	hasFailure := refreshResult.Failed > 0 ||
		inspectResult.Health.Failed > 0 ||
		inspectResult.Entitlement.Failed > 0

	common.ApiSuccess(c, gin.H{
		"refresh": refreshResult,
		"inspect": inspectResult,
		"channel": gin.H{
			"created":      created,
			"channel_id":   channel.Id,
			"channel_name": channel.Name,
			"channel_key":  channel.Key,
			"models_count": len(models),
			"models":       models,
		},
		"has_failure":    hasFailure,
		"concurrency":    concurrency,
		"retry_attempts": retryAttempts,
	})
}

func AdminGetChatMockPoolConfig(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	cfg, err := service.GetChatMockPoolConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func AdminUpdateChatMockPoolConfig(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	cfg, err := service.GetChatMockPoolConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request := chatMockPoolConfigUpdateRequest{}
	if err = c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.EnableWeighted != nil {
		cfg.EnableWeighted = *request.EnableWeighted
	}
	if request.DefaultCooldownSecond != nil {
		cfg.DefaultCooldownSecond = *request.DefaultCooldownSecond
	}
	if request.RequestTimeoutSecond != nil {
		cfg.RequestTimeoutSecond = *request.RequestTimeoutSecond
	}
	if request.MaxRetry != nil {
		cfg.MaxRetry = *request.MaxRetry
	}
	if request.HealthCheckInterval != nil {
		cfg.HealthCheckInterval = *request.HealthCheckInterval
	}
	if request.EntitlementCheckInterval != nil {
		cfg.EntitlementCheckInterval = *request.EntitlementCheckInterval
	}
	if request.DefaultInstructions != nil {
		cfg.DefaultInstructions = *request.DefaultInstructions
	}
	if err = cfg.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func AdminGetChatMockPoolRuntime(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	pageInfo := common.GetPageQuery(c)
	accounts, total, err := model.ListChatMockAccounts("", "", "", "", pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	runtime := service.GetChatMockRuntimeSnapshot()
	items := make([]gin.H, 0, len(accounts))
	now := common.GetTimestamp()
	for _, account := range accounts {
		if account == nil {
			continue
		}
		masked := account.Masked()
		masked.RouteScore = service.EstimateChatMockRouteScore(account)
		state := runtime[account.Id]
		cooldownRemainingSec := int64(0)
		if account.CooldownUntil > now {
			cooldownRemainingSec = account.CooldownUntil - now
		}
		items = append(items, gin.H{
			"account": masked,
			"runtime": state,
			"diagnostics": gin.H{
				"quota_status":             masked.QuotaStatus,
				"weekly_remaining_percent": masked.WeeklyRemainingPct,
				"route_score":              masked.RouteScore,
				"cooldown_remaining_sec":   cooldownRemainingSec,
				"is_codex_account":         service.IsLikelyChatMockCodexAccount(account),
				"token_refresh_available":  strings.TrimSpace(account.RefreshToken) != "",
				"last_error":               strings.TrimSpace(account.LastError),
			},
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListChatMockRouteLogs(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	accountID, _ := strconv.Atoi(strings.TrimSpace(c.Query("account_id")))
	modelName := strings.TrimSpace(c.Query("model"))
	success := strings.TrimSpace(c.Query("success"))
	strategy := strings.TrimSpace(c.Query("strategy"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListChatMockRouteLogsWithFilter(
		accountID,
		modelName,
		success,
		strategy,
		keyword,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListChatMockAccountSnapshots(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	accountID, _ := strconv.Atoi(strings.TrimSpace(c.Query("account_id")))
	snapshotType := strings.TrimSpace(c.Query("snapshot_type"))
	status := strings.TrimSpace(c.Query("status"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListChatMockAccountSnapshotsWithFilter(
		accountID,
		snapshotType,
		status,
		keyword,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetChatMockPoolChannelStatus(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}

	recommendedModels, err := collectChatMockPoolChannelModels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(recommendedModels) == 0 {
		recommendedModels = []string{"gpt-5.2", "gpt-5.3-codex"}
	}
	recommendedModels = splitAndDedupeChatMockModels(strings.Join(recommendedModels, ","))

	channel, findErr := findChatMockPoolChannel()
	if findErr == gorm.ErrRecordNotFound {
		common.ApiSuccess(c, gin.H{
			"exists":                   false,
			"enabled":                  false,
			"need_sync":                true,
			"recommended_models_count": len(recommendedModels),
			"recommended_models":       recommendedModels,
			"message":                  "未检测到 ChatMockPool 渠道，请先执行“一键接入 NewAPI 渠道”",
		})
		return
	}
	if findErr != nil {
		common.ApiError(c, findErr)
		return
	}

	currentModels := splitAndDedupeChatMockModels(channel.Models)
	missingModels, extraModels := diffChatMockModels(currentModels, recommendedModels)
	enabled := channel.Status == common.ChannelStatusEnabled
	needSync := !enabled ||
		len(missingModels) > 0 ||
		channel.Type != constant.ChannelTypeChatMock ||
		strings.TrimSpace(channel.Key) != chatMockPoolChannelKey

	message := "渠道已接入并处于启用状态"
	if !enabled {
		message = "渠道已存在但未启用，请先启用后再转发请求"
	} else if len(missingModels) > 0 {
		message = "渠道模型列表不完整，建议执行同步"
	} else if len(extraModels) > 0 {
		message = "渠道包含额外模型，已保留；如需严格对齐可执行同步"
	}

	common.ApiSuccess(c, gin.H{
		"exists":                   true,
		"enabled":                  enabled,
		"need_sync":                needSync,
		"message":                  message,
		"channel_id":               channel.Id,
		"channel_name":             channel.Name,
		"channel_key":              channel.Key,
		"channel_type":             channel.Type,
		"channel_status":           channel.Status,
		"channel_status_text":      chatMockChannelStatusText(channel.Status),
		"group":                    channel.Group,
		"models_count":             len(currentModels),
		"models":                   currentModels,
		"recommended_models_count": len(recommendedModels),
		"recommended_models":       recommendedModels,
		"missing_models":           missingModels,
		"extra_models":             extraModels,
	})
}

func AdminSyncChatMockPoolChannel(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	channel, created, models, err := syncChatMockPoolChannel()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"created":      created,
		"channel_id":   channel.Id,
		"channel_name": channel.Name,
		"channel_key":  channel.Key,
		"models_count": len(models),
		"models":       models,
	})
}

func AdminDiagnoseChatMockRoute(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	request := chatMockRouteDiagnoseRequest{}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(c.Query("model"))
	}
	excluded := make(map[int]struct{}, len(request.ExcludedIDs))
	for _, id := range request.ExcludedIDs {
		if id > 0 {
			excluded[id] = struct{}{}
		}
	}

	result, err := service.DiagnoseChatMockRoute(modelName, excluded)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminListChatMockMailboxes(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListChatMockMailboxes(keyword, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	masked := make([]*model.ChatMockMailbox, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		masked = append(masked, item.Masked())
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(masked)
	common.ApiSuccess(c, pageInfo)
}

func AdminExportChatMockAccounts(c *gin.Context) {
	if !ensureChatMockEnabled(c) {
		return
	}
	includeSensitive, approvalMessage := resolveChatMockSensitiveExportOption(c)
	if approvalMessage != "" {
		common.ApiErrorMsg(c, approvalMessage)
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	tier := strings.TrimSpace(strings.ToLower(c.Query("tier")))
	quotaStatus := strings.TrimSpace(strings.ToLower(c.Query("quota_status")))

	db := model.DB.Model(&model.ChatMockAccount{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR account LIKE ? OR base_url LIKE ?", like, like, like)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if tier != "" {
		db = db.Where("LOWER(entitlement_tier) = ?", tier)
	}
	switch quotaStatus {
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

	items := make([]*model.ChatMockAccount, 0, 64)
	if err := db.Order("priority ASC, weight DESC, id DESC").Find(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	filePrefix := "chatmock-accounts-" + strconv.FormatInt(common.GetTimestamp(), 10)
	switch format {
	case "csv":
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			exportItem := sanitizeChatMockAccountForExport(item, includeSensitive)
			rows = append(rows, []string{
				strconv.Itoa(exportItem.Id),
				exportItem.Name,
				exportItem.Account,
				exportItem.ApiKey,
				exportItem.IdToken,
				exportItem.RefreshToken,
				exportItem.BaseURL,
				exportItem.ProxyURL,
				exportItem.Status,
				strconv.Itoa(exportItem.Weight),
				strconv.Itoa(exportItem.Priority),
				strconv.Itoa(exportItem.MaxConcurrency),
				strconv.Itoa(exportItem.RPM),
				strconv.FormatInt(exportItem.DailyTokenLimit, 10),
				strconv.FormatInt(exportItem.DailyTokenUsed, 10),
				exportItem.DailyTokenDate,
				exportItem.EntitlementTier,
				strconv.FormatBool(exportItem.EntitlementValid),
				strconv.FormatInt(exportItem.Usage5hUsed, 10),
				strconv.FormatInt(exportItem.Usage5hLimit, 10),
				strconv.FormatInt(exportItem.Usage5hResetAt, 10),
				strconv.FormatInt(exportItem.UsageWeeklyUsed, 10),
				strconv.FormatInt(exportItem.UsageWeeklyLimit, 10),
				strconv.FormatInt(exportItem.UsageWeeklyResetAt, 10),
				strconv.FormatInt(exportItem.UsageCodeReviewUsed, 10),
				strconv.FormatInt(exportItem.UsageCodeReviewLimit, 10),
				strconv.FormatInt(exportItem.UsageCodeReviewResetAt, 10),
				strconv.FormatInt(exportItem.TokenExpiredAt, 10),
				strconv.FormatInt(exportItem.TokenLastRefreshAt, 10),
				exportItem.LastError,
				exportItem.Metadata,
				exportItem.SupportedModels,
				strconv.FormatInt(exportItem.CreatedTime, 10),
				strconv.FormatInt(exportItem.UpdatedTime, 10),
			})
		}
		payload, err := encodeChatMockCSV(
			[]string{
				"id",
				"name",
				"account",
				"api_key",
				"id_token",
				"refresh_token",
				"base_url",
				"proxy_url",
				"status",
				"weight",
				"priority",
				"max_concurrency",
				"rpm",
				"daily_token_limit",
				"daily_token_used",
				"daily_token_date",
				"entitlement_tier",
				"entitlement_valid",
				"usage_5h_used",
				"usage_5h_limit",
				"usage_5h_reset_at",
				"usage_weekly_used",
				"usage_weekly_limit",
				"usage_weekly_reset_at",
				"usage_code_review_used",
				"usage_code_review_limit",
				"usage_code_review_reset_at",
				"token_expired_at",
				"token_last_refresh_at",
				"last_error",
				"metadata",
				"supported_models",
				"created_time",
				"updated_time",
			},
			rows,
		)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+filePrefix+".csv\"")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", payload)
		return
	case "json":
		exportItems := make([]*model.ChatMockAccount, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			exportItems = append(exportItems, sanitizeChatMockAccountForExport(item, includeSensitive))
		}
		payload, err := common.Marshal(gin.H{
			"total":       len(exportItems),
			"exported_at": common.GetTimestamp(),
			"sensitive":   includeSensitive,
			"items":       exportItems,
		})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+filePrefix+".json\"")
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
		return
	default:
		common.ApiErrorMsg(c, "format 仅支持 json/csv")
	}
}

func AdminExportChatMockMailboxes(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	includeSensitive, approvalMessage := resolveChatMockSensitiveExportOption(c)
	if approvalMessage != "" {
		common.ApiErrorMsg(c, approvalMessage)
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))

	db := model.DB.Model(&model.ChatMockMailbox{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR email LIKE ? OR username LIKE ?", like, like, like)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}

	items := make([]*model.ChatMockMailbox, 0, 64)
	if err := db.Order("id DESC").Find(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	filePrefix := "chatmock-mailboxes-" + strconv.FormatInt(common.GetTimestamp(), 10)
	switch format {
	case "csv":
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			exportItem := sanitizeChatMockMailboxForExport(item, includeSensitive)
			rows = append(rows, []string{
				strconv.Itoa(exportItem.Id),
				exportItem.Name,
				exportItem.Email,
				exportItem.Username,
				exportItem.Password,
				exportItem.ImapHost,
				strconv.Itoa(exportItem.ImapPort),
				exportItem.ImapSecurity,
				exportItem.SmtpHost,
				strconv.Itoa(exportItem.SmtpPort),
				exportItem.SmtpSecurity,
				exportItem.Status,
				strconv.Itoa(exportItem.PollIntervalSecond),
				strconv.FormatInt(exportItem.LastCheckedTime, 10),
				exportItem.LastError,
				exportItem.Metadata,
				strconv.FormatInt(exportItem.CreatedTime, 10),
				strconv.FormatInt(exportItem.UpdatedTime, 10),
			})
		}
		payload, err := encodeChatMockCSV(
			[]string{
				"id",
				"name",
				"email",
				"username",
				"password",
				"imap_host",
				"imap_port",
				"imap_security",
				"smtp_host",
				"smtp_port",
				"smtp_security",
				"status",
				"poll_interval_second",
				"last_checked_time",
				"last_error",
				"metadata",
				"created_time",
				"updated_time",
			},
			rows,
		)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+filePrefix+".csv\"")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", payload)
		return
	case "json":
		exportItems := make([]*model.ChatMockMailbox, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			exportItems = append(exportItems, sanitizeChatMockMailboxForExport(item, includeSensitive))
		}
		payload, err := common.Marshal(gin.H{
			"total":       len(exportItems),
			"exported_at": common.GetTimestamp(),
			"sensitive":   includeSensitive,
			"items":       exportItems,
		})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\""+filePrefix+".json\"")
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
		return
	default:
		common.ApiErrorMsg(c, "format 仅支持 json/csv")
	}
}

func AdminCreateChatMockMailbox(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	request := chatMockMailboxUpsertRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	mailbox := &model.ChatMockMailbox{}
	fillChatMockMailboxFromRequest(mailbox, request)
	mailbox.Normalize()
	if strings.TrimSpace(mailbox.Name) == "" ||
		strings.TrimSpace(mailbox.Email) == "" ||
		strings.TrimSpace(mailbox.Username) == "" ||
		strings.TrimSpace(mailbox.Password) == "" ||
		(!mailbox.UsesProviderMailbox() && strings.TrimSpace(mailbox.ImapHost) == "") {
		if mailbox.UsesProviderMailbox() {
			common.ApiErrorMsg(c, "name/email/username/password 为必填项（provider 邮箱可不填 imap_host）")
			return
		}
		common.ApiErrorMsg(c, "name/email/username/password/imap_host 为必填项（smtp_host 可选）")
		return
	}
	if err := mailbox.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, mailbox.Masked())
}

func AdminBatchCreateChatMockMailboxes(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	rawBody, err := io.ReadAll(io.LimitReader(c.Request.Body, 4<<20))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(rawBody) == 0 {
		common.ApiErrorMsg(c, "请求体不能为空")
		return
	}

	batchReq := chatMockMailboxBatchUpsertRequest{}
	items := make([]chatMockMailboxUpsertRequest, 0, 32)
	if err = common.Unmarshal(rawBody, &batchReq); err == nil && len(batchReq.Items) > 0 {
		items = batchReq.Items
	}
	if len(items) == 0 {
		directItems := make([]chatMockMailboxUpsertRequest, 0, 32)
		if err = common.Unmarshal(rawBody, &directItems); err != nil {
			common.ApiErrorMsg(c, "批量导入格式错误，需传数组或 {items:[...]}")
			return
		}
		items = directItems
	}
	if len(items) == 0 {
		common.ApiErrorMsg(c, "批量导入列表为空")
		return
	}
	if len(items) > 1000 {
		common.ApiErrorMsg(c, "单次批量导入最多 1000 条")
		return
	}

	upsert := true
	if batchReq.Upsert != nil {
		upsert = *batchReq.Upsert
	}

	successCount := 0
	failed := make([]gin.H, 0)
	for idx, request := range items {
		mailbox := &model.ChatMockMailbox{}
		fillChatMockMailboxFromRequest(mailbox, request)
		mailbox.Normalize()
		if strings.TrimSpace(mailbox.Name) == "" ||
			strings.TrimSpace(mailbox.Email) == "" ||
			strings.TrimSpace(mailbox.Username) == "" ||
			(!mailbox.UsesProviderMailbox() && strings.TrimSpace(mailbox.ImapHost) == "") {
			failed = append(failed, gin.H{
				"index": idx + 1,
				"email": mailbox.Email,
				"message": func() string {
					if mailbox.UsesProviderMailbox() {
						return "name/email/username 为必填项（provider 邮箱可不填 imap_host）"
					}
					return "name/email/username/imap_host 为必填项（smtp_host 可选）"
				}(),
			})
			continue
		}

		if upsert {
			existed, findErr := model.GetChatMockMailboxByEmail(mailbox.Email)
			if findErr == nil && existed != nil {
				fillChatMockMailboxFromRequest(existed, request)
				if updateErr := existed.Update(); updateErr != nil {
					failed = append(failed, gin.H{
						"index":   idx + 1,
						"email":   mailbox.Email,
						"message": updateErr.Error(),
					})
					continue
				}
				successCount++
				continue
			}
			if findErr != nil && findErr != gorm.ErrRecordNotFound {
				failed = append(failed, gin.H{
					"index":   idx + 1,
					"email":   mailbox.Email,
					"message": findErr.Error(),
				})
				continue
			}
		}

		if strings.TrimSpace(mailbox.Password) == "" {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"email":   mailbox.Email,
				"message": "新建邮箱必须提供 password",
			})
			continue
		}
		if insertErr := mailbox.Insert(); insertErr != nil {
			failed = append(failed, gin.H{
				"index":   idx + 1,
				"email":   mailbox.Email,
				"message": insertErr.Error(),
			})
			continue
		}
		successCount++
	}

	common.ApiSuccess(c, gin.H{
		"total":   len(items),
		"success": successCount,
		"failed":  len(failed),
		"details": failed,
		"upsert":  upsert,
	})
}

func AdminUpdateChatMockMailbox(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮箱 ID 无效")
		return
	}
	mailbox, err := model.GetChatMockMailboxByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	request := chatMockMailboxUpsertRequest{}
	if err = c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	fillChatMockMailboxFromRequest(mailbox, request)
	if err = mailbox.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, mailbox.Masked())
}

func AdminDeleteChatMockMailbox(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮箱 ID 无效")
		return
	}
	mailbox, err := model.GetChatMockMailboxByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = mailbox.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

func AdminTestChatMockMailbox(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮箱 ID 无效")
		return
	}
	mailbox, err := model.GetChatMockMailboxByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	timeoutSec := defaultIfNotPositive(c.Query("timeout_sec"), 20)
	if err = service.ValidateChatMockMailboxConnectivity(mailbox, timeoutSec); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": "ok"})
}

func AdminBatchTestChatMockMailboxes(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	timeoutSec := defaultIfNotPositive(c.Query("timeout_sec"), 20)
	mailboxes, err := model.ListEnabledChatMockMailboxes()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	successCount := 0
	failed := make([]gin.H, 0)
	now := common.GetTimestamp()
	for _, mailbox := range mailboxes {
		if mailbox == nil {
			continue
		}
		checkErr := service.ValidateChatMockMailboxConnectivity(mailbox, timeoutSec)
		updateValues := map[string]any{
			"last_checked_time": now,
			"updated_time":      now,
		}
		if checkErr != nil {
			updateValues["last_error"] = checkErr.Error()
			failed = append(failed, gin.H{
				"id":      mailbox.Id,
				"email":   mailbox.Email,
				"message": checkErr.Error(),
			})
			_ = model.DB.Model(&model.ChatMockMailbox{}).Where("id = ?", mailbox.Id).Updates(updateValues).Error
			continue
		}
		updateValues["last_error"] = ""
		_ = model.DB.Model(&model.ChatMockMailbox{}).Where("id = ?", mailbox.Id).Updates(updateValues).Error
		successCount++
	}

	common.ApiSuccess(c, gin.H{
		"total":   len(mailboxes),
		"success": successCount,
		"failed":  len(failed),
		"details": failed,
	})
}

func AdminPullChatMockMailbox(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮箱 ID 无效")
		return
	}
	request := chatMockPullRequest{}
	_ = c.ShouldBindJSON(&request)
	if request.Limit <= 0 {
		request.Limit = defaultIfNotPositive(c.Query("limit"), 30)
	}
	if request.TimeoutSec <= 0 {
		request.TimeoutSec = defaultIfNotPositive(c.Query("timeout_sec"), 20)
	}
	events, pullErr := service.PullChatMockMailboxByID(id, request.Limit, request.TimeoutSec)
	if pullErr != nil {
		common.ApiError(c, pullErr)
		return
	}
	common.ApiSuccess(c, events)
}

func AdminListChatMockMailEvents(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	mailboxID, _ := strconv.Atoi(c.Query("mailbox_id"))
	eventType := strings.TrimSpace(strings.ToLower(c.Query("event_type")))
	classification := strings.TrimSpace(strings.ToLower(c.Query("classification")))
	keyword := strings.TrimSpace(c.Query("keyword"))
	pageInfo := common.GetPageQuery(c)
	events, total, err := model.ListChatMockMailEvents(
		mailboxID,
		eventType,
		classification,
		keyword,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(events)
	common.ApiSuccess(c, pageInfo)
}

func AdminReparseChatMockMailEvent(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮件事件 ID 无效")
		return
	}
	event, err := model.GetChatMockMailEventByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = service.ReparseChatMockMailEvent(event); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, event)
}

func AdminGetChatMockLatestCode(c *gin.Context) {
	if !ensureChatMockMailboxEnabled(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "邮箱 ID 无效")
		return
	}
	event, err := model.GetLatestVerificationCodeEvent(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, event)
}

func fillChatMockAccountFromRequest(account *model.ChatMockAccount, request chatMockAccountUpsertRequest) {
	if account == nil {
		return
	}
	hasExplicitBaseURL := false
	if request.Name != nil {
		account.Name = strings.TrimSpace(*request.Name)
	}
	if request.Account != nil {
		account.Account = strings.TrimSpace(*request.Account)
	}
	if strings.TrimSpace(account.Account) == "" && request.AccountID != nil {
		account.Account = strings.TrimSpace(*request.AccountID)
	}
	if request.ApiKey != nil {
		value := strings.TrimSpace(*request.ApiKey)
		if value != "" {
			account.ApiKey = value
		}
	}
	if strings.TrimSpace(account.ApiKey) == "" && request.AccessToken != nil {
		value := strings.TrimSpace(*request.AccessToken)
		if value != "" {
			account.ApiKey = value
		}
	}
	if request.IdToken != nil {
		value := strings.TrimSpace(*request.IdToken)
		if value != "" {
			account.IdToken = value
		}
	}
	if request.RefreshToken != nil {
		value := strings.TrimSpace(*request.RefreshToken)
		if value != "" {
			account.RefreshToken = value
		}
	}
	if request.BaseURL != nil {
		baseURL := strings.TrimSpace(*request.BaseURL)
		account.BaseURL = baseURL
		hasExplicitBaseURL = baseURL != ""
	}
	token := strings.TrimSpace(account.ApiKey)
	idToken := ""
	if request.IdToken != nil {
		idToken = strings.TrimSpace(*request.IdToken)
	}
	derivedEmail, derivedAccountID, derivedTier := deriveChatMockCodexIdentity(token, idToken)
	if strings.TrimSpace(account.Account) == "" && derivedAccountID != "" {
		account.Account = derivedAccountID
	}
	typeValue := ""
	if request.Type != nil {
		typeValue = strings.ToLower(strings.TrimSpace(*request.Type))
	}
	isLikelyCodex := typeValue == "codex" || (token != "" && !strings.HasPrefix(strings.ToLower(token), "sk-"))
	if isLikelyCodex && !hasExplicitBaseURL {
		currentBaseURL := strings.ToLower(strings.TrimSpace(account.BaseURL))
		if currentBaseURL == "" ||
			currentBaseURL == "https://api.openai.com" ||
			currentBaseURL == "http://api.openai.com" ||
			strings.HasPrefix(currentBaseURL, "https://api.openai.com/v1") ||
			strings.HasPrefix(currentBaseURL, "http://api.openai.com/v1") {
			account.BaseURL = "https://chatgpt.com"
		}
	} else if strings.TrimSpace(account.BaseURL) == "" && !isLikelyCodex {
		account.BaseURL = "https://api.openai.com"
	}
	if strings.TrimSpace(account.Name) == "" && request.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*request.Email))
		if email != "" {
			account.Name = email
		}
	}
	if strings.TrimSpace(account.Name) == "" && derivedEmail != "" {
		account.Name = derivedEmail
	}
	if derivedTier != "" {
		currentTier := strings.ToLower(strings.TrimSpace(account.EntitlementTier))
		if currentTier == "" || currentTier == "unknown" {
			account.EntitlementTier = derivedTier
		}
	}
	if exp := parseJWTExpiryUnix(account.ApiKey); exp > 0 {
		account.TokenExpiredAt = exp
	}
	if account.TokenExpiredAt <= 0 && request.Expired != nil {
		if parsed := parseRFC3339Timestamp(*request.Expired); parsed > 0 {
			account.TokenExpiredAt = parsed
		}
	}
	if request.ProxyURL != nil {
		account.ProxyURL = strings.TrimSpace(*request.ProxyURL)
	}
	if request.Status != nil {
		account.Status = strings.TrimSpace(strings.ToLower(*request.Status))
	}
	if request.Weight != nil {
		account.Weight = *request.Weight
	}
	if request.Priority != nil {
		account.Priority = *request.Priority
	}
	if request.MaxConcurrency != nil {
		account.MaxConcurrency = *request.MaxConcurrency
	}
	if request.RPM != nil {
		account.RPM = *request.RPM
	}
	if request.DailyTokenLimit != nil {
		account.DailyTokenLimit = *request.DailyTokenLimit
	}
	if request.Metadata != nil {
		account.Metadata = strings.TrimSpace(*request.Metadata)
	}
}

func deriveChatMockCodexIdentity(accessToken string, idToken string) (email string, accountID string, planType string) {
	merge := func(claims map[string]any) {
		if claims == nil {
			return
		}

		authMap := castAnyMap(claims["https://api.openai.com/auth"])
		profileMap := castAnyMap(claims["https://api.openai.com/profile"])

		if email == "" {
			email = strings.ToLower(strings.TrimSpace(firstNonEmptyString(
				readStringValue(claims, "email"),
				readStringValue(profileMap, "email"),
			)))
		}
		if accountID == "" {
			accountID = strings.TrimSpace(firstNonEmptyString(
				readStringValue(claims, "account_id"),
				readStringValue(claims, "chatgpt_account_id"),
				readStringValue(authMap, "chatgpt_account_id"),
			))
		}
		if planType == "" {
			planType = normalizeChatMockTierValue(firstNonEmptyString(
				readStringValue(claims, "chatgpt_plan_type"),
				readStringValue(authMap, "chatgpt_plan_type"),
			))
		}
	}

	merge(decodeChatMockJWTClaims(idToken))
	merge(decodeChatMockJWTClaims(accessToken))
	return email, accountID, planType
}

func decodeChatMockJWTClaims(token string) map[string]any {
	raw := strings.TrimSpace(token)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return nil
	}

	payload := strings.TrimSpace(parts[1])
	if payload == "" {
		return nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		padded := payload
		if mod := len(padded) % 4; mod != 0 {
			padded += strings.Repeat("=", 4-mod)
		}
		decoded, err = base64.URLEncoding.DecodeString(padded)
		if err != nil {
			return nil
		}
	}

	claims := make(map[string]any)
	if err = common.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

func parseJWTExpiryUnix(token string) int64 {
	claims := decodeChatMockJWTClaims(token)
	if claims == nil {
		return 0
	}
	expRaw, ok := claims["exp"]
	if !ok || expRaw == nil {
		return 0
	}
	switch typed := expRaw.(type) {
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	}
	return 0
}

func parseRFC3339Timestamp(raw string) int64 {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0
	}
	tm, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return 0
	}
	return tm.Unix()
}

func castAnyMap(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	if typed, ok := raw.(map[string]any); ok {
		return typed
	}
	return nil
}

func readStringValue(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeChatMockTierValue(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "team"):
		return "team"
	case strings.Contains(value, "plus"):
		return "plus"
	case strings.Contains(value, "pro"):
		return "pro"
	case strings.Contains(value, "free"):
		return "free"
	default:
		return ""
	}
}

func fillChatMockMailboxFromRequest(mailbox *model.ChatMockMailbox, request chatMockMailboxUpsertRequest) {
	if mailbox == nil {
		return
	}
	if request.Name != nil {
		mailbox.Name = strings.TrimSpace(*request.Name)
	}
	if request.Email != nil {
		mailbox.Email = strings.TrimSpace(strings.ToLower(*request.Email))
	}
	if request.Username != nil {
		mailbox.Username = strings.TrimSpace(*request.Username)
	}
	if request.Password != nil {
		value := strings.TrimSpace(*request.Password)
		if value != "" {
			mailbox.Password = value
		}
	}
	if request.ImapHost != nil {
		mailbox.ImapHost = strings.TrimSpace(*request.ImapHost)
	}
	if request.ImapPort != nil {
		mailbox.ImapPort = *request.ImapPort
	}
	if request.ImapSecurity != nil {
		mailbox.ImapSecurity = strings.TrimSpace(*request.ImapSecurity)
	}
	if request.SmtpHost != nil {
		mailbox.SmtpHost = strings.TrimSpace(*request.SmtpHost)
	}
	if request.SmtpPort != nil {
		mailbox.SmtpPort = *request.SmtpPort
	}
	if request.SmtpSecurity != nil {
		mailbox.SmtpSecurity = strings.TrimSpace(*request.SmtpSecurity)
	}
	if request.Status != nil {
		mailbox.Status = strings.TrimSpace(strings.ToLower(*request.Status))
	}
	if request.PollIntervalSecond != nil {
		mailbox.PollIntervalSecond = *request.PollIntervalSecond
	}
	if request.Metadata != nil {
		mailbox.Metadata = strings.TrimSpace(*request.Metadata)
	}
}

func parseSupportedModels(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	switch value := raw.(type) {
	case []string:
		return value, nil
	case []any:
		models := make([]string, 0, len(value))
		for _, item := range value {
			modelName, ok := item.(string)
			if !ok {
				continue
			}
			models = append(models, strings.TrimSpace(modelName))
		}
		return models, nil
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return nil, nil
		}
		if strings.HasPrefix(text, "[") {
			var models []string
			if err := common.Unmarshal([]byte(text), &models); err != nil {
				return nil, err
			}
			return models, nil
		}
		parts := strings.Split(text, ",")
		models := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			models = append(models, trimmed)
		}
		return models, nil
	default:
		return nil, nil
	}
}

func hasNonEmptyString(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

func defaultIfNotPositive(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func copyChatMockUpstreamHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if len(values) == 0 {
			continue
		}
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		switch lowerKey {
		case "transfer-encoding", "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "upgrade":
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func encodeChatMockCSV(headers []string, rows [][]string) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	writer := csv.NewWriter(buffer)
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseChatMockBoolQuery(c *gin.Context, keys ...string) bool {
	for _, key := range keys {
		raw := strings.TrimSpace(strings.ToLower(c.Query(key)))
		switch raw {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func getChatMockRequestRole(c *gin.Context) int {
	roleAny, ok := c.Get("role")
	if !ok {
		return 0
	}
	role, ok := roleAny.(int)
	if !ok {
		return 0
	}
	return role
}

func getChatMockRequestUserID(c *gin.Context) int {
	value, ok := c.Get("id")
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func resolveChatMockSensitiveExportOption(c *gin.Context) (bool, string) {
	includeSensitive := parseChatMockBoolQuery(
		c,
		"include_sensitive",
		"export_sensitive",
		"with_sensitive",
	)
	if !includeSensitive {
		return false, ""
	}
	if getChatMockRequestRole(c) < common.RoleRootUser {
		return false, "仅 root 可导出敏感字段"
	}
	if !parseChatMockBoolQuery(c, "confirm_sensitive_export", "confirm") {
		return false, "导出敏感字段必须携带 confirm_sensitive_export=true"
	}
	common.SysLog(
		fmt.Sprintf(
			"[audit] sensitive-export approved module=chatmock user_id=%d path=%s query=%s",
			getChatMockRequestUserID(c),
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
		),
	)
	return true, ""
}

func sanitizeChatMockAccountForExport(item *model.ChatMockAccount, includeSensitive bool) *model.ChatMockAccount {
	if item == nil {
		return nil
	}
	if includeSensitive {
		clone := *item
		clone.FillDerivedQuotaFields()
		return &clone
	}
	return item.Masked()
}

func sanitizeChatMockMailboxForExport(item *model.ChatMockMailbox, includeSensitive bool) *model.ChatMockMailbox {
	if item == nil {
		return nil
	}
	if includeSensitive {
		clone := *item
		return &clone
	}
	return item.Masked()
}

func ensureChatMockEnabled(c *gin.Context) bool {
	if setting.IsChatMockEnabled() {
		return true
	}
	common.ApiErrorMsg(c, "CHATMOCK_ENABLED=false，账号池未启用")
	return false
}

func ensureChatMockMailboxEnabled(c *gin.Context) bool {
	if setting.IsChatMockMailboxEnabled() {
		return true
	}
	common.ApiErrorMsg(c, "CHATMOCK_MAILBOX_ENABLED=false，邮箱中心未启用")
	return false
}
