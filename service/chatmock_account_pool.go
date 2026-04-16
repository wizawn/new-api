package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

type chatMockRuntimeState struct {
	InFlight    int
	WindowStart int64
	RequestCnt  int
}

type ChatMockRuntimeSnapshot struct {
	AccountID   int   `json:"account_id"`
	InFlight    int   `json:"in_flight"`
	WindowStart int64 `json:"window_start"`
	RequestCnt  int   `json:"request_cnt"`
}

type ChatMockRouteDiagnoseItem struct {
	Account                   *model.ChatMockAccount  `json:"account"`
	Decision                  string                  `json:"decision"`
	Reasons                   []string                `json:"reasons,omitempty"`
	Notes                     []string                `json:"notes,omitempty"`
	Suggestions               []string                `json:"suggestions,omitempty"`
	RouteScore                int64                   `json:"route_score"`
	EffectiveWeight           int64                   `json:"effective_weight"`
	SelectionRank             int                     `json:"selection_rank,omitempty"`
	Runtime                   ChatMockRuntimeSnapshot `json:"runtime"`
	WeeklyRemainingPercent    *float64                `json:"weekly_remaining_percent,omitempty"`
	FiveHourRemainingPercent  *float64                `json:"five_hour_remaining_percent,omitempty"`
	CodeReviewRemainingPct    *float64                `json:"code_review_remaining_percent,omitempty"`
	CooldownRemainingSec      int64                   `json:"cooldown_remaining_sec"`
	DailyTokenUsedToday       int64                   `json:"daily_token_used_today"`
	DailyTokenLimit           int64                   `json:"daily_token_limit"`
	EntitlementQuotaExhausted bool                    `json:"entitlement_quota_exhausted"`
}

type ChatMockRouteDiagnoseResult struct {
	Model               string                      `json:"model"`
	EnableWeighted      bool                        `json:"enable_weighted"`
	SelectionPolicy     string                      `json:"selection_policy"`
	SelectionNote       string                      `json:"selection_note,omitempty"`
	Timestamp           int64                       `json:"timestamp"`
	TotalCount          int                         `json:"total_count"`
	EligibleCount       int                         `json:"eligible_count"`
	BlockedCount        int                         `json:"blocked_count"`
	SelectedAccountID   int                         `json:"selected_account_id,omitempty"`
	SelectedAccountName string                      `json:"selected_account_name,omitempty"`
	Items               []ChatMockRouteDiagnoseItem `json:"items"`
}

type ChatMockEntitlementResult struct {
	Tier                   string `json:"tier"`
	Valid                  bool   `json:"valid"`
	Usage5hUsed            int64  `json:"usage_5h_used"`
	Usage5hLimit           int64  `json:"usage_5h_limit"`
	Usage5hResetAt         int64  `json:"usage_5h_reset_at"`
	UsageWeeklyUsed        int64  `json:"usage_weekly_used"`
	UsageWeeklyLimit       int64  `json:"usage_weekly_limit"`
	UsageWeeklyResetAt     int64  `json:"usage_weekly_reset_at"`
	UsageCodeReviewUsed    int64  `json:"usage_code_review_used"`
	UsageCodeReviewLimit   int64  `json:"usage_code_review_limit"`
	UsageCodeReviewResetAt int64  `json:"usage_code_review_reset_at"`
	CheckedAt              int64  `json:"checked_at"`
	Source                 string `json:"source"`
	ErrorMsg               string `json:"error_msg"`
	Raw                    string `json:"raw,omitempty"`
}

var (
	chatMockRuntimeMu       sync.Mutex
	chatMockRuntimeByAcct   = make(map[int]*chatMockRuntimeState)
	chatMockHealthOnce      sync.Once
	chatMockEntitlementOnce sync.Once
	chatMockRefreshMu       sync.Mutex
	chatMockRefreshByAcct   = make(map[int]*sync.Mutex)
	chatMock5hColumnOnce    sync.Once
	chatMock5hUsedColumn    = "usage5h_used"
	chatMock5hLimitColumn   = "usage5h_limit"
	chatMock5hResetColumn   = "usage5h_reset_at"
	chatMockRetryAfterRegex = regexp.MustCompile(`(?i)retry(?:\s+after|\s+in)?\s*[:=]?\s*(\d{1,5})\s*(?:s|sec|second|seconds)?`)
)

const (
	chatMockCodexOriginator   = "codex_cli_rs"
	chatMockCodexVersion      = "0.101.0"
	chatMockCodexUserAgent    = "codex_cli_rs/0.101.0"
	chatMockCodexBaseURL      = "https://chatgpt.com"
	chatMockCodexResponsePath = "/backend-api/codex/responses"
	chatMockTransientAttempts = 2
	chatMockAffinityNeutral   = int64(100)
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func validateChatMockUpstreamURL(urlStr string) error {
	fetchSetting := system_setting.GetFetchSetting()
	return common.ValidateURLWithFetchSetting(
		urlStr,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	)
}

func GetChatMockPoolConfig() (*model.ChatMockPoolConfig, error) {
	return model.GetOrInitChatMockPoolConfig()
}

func RouteChatMockChatCompletion(ctx context.Context, rawBody []byte, modelName string, incomingHeader http.Header) (*http.Response, *model.ChatMockAccount, error) {
	if !setting.IsChatMockEnabled() {
		return nil, nil, fmt.Errorf("CHATMOCK_ENABLED=false，账号池未启用")
	}

	cfg, err := GetChatMockPoolConfig()
	if err != nil {
		return nil, nil, err
	}

	excluded := make(map[int]struct{})
	attempts := cfg.MaxRetry
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		selected, reserveErr := selectAccountForRequest(modelName, excluded, cfg)
		if reserveErr != nil {
			lastErr = reserveErr
			break
		}
		routeStart := time.Now()
		routeStrategy := "priority_weight_order"
		if cfg.EnableWeighted {
			routeStrategy = "weighted_random_with_quota_bias"
		}
		routeScore := computeChatMockSelectionWeight(selected, modelName)
		requestID := strings.TrimSpace(
			pickFirstNonEmptyHeaders(
				incomingHeader,
				"X-Request-Id",
				"X-Request-ID",
				"X-Amzn-Trace-Id",
				"X-B3-Traceid",
			),
		)
		if requestID == "" {
			requestID = common.GetUUID()
		}

		response, reqErr := forwardToUpstreamAccount(ctx, selected, rawBody, incomingHeader, cfg.RequestTimeoutSecond)
		if reqErr != nil {
			releaseRuntimeSlot(selected.Id)
			RecordChatMockAccountFailure(selected.Id, reqErr.Error(), cfg.DefaultCooldownSecond)
			RecordChatMockRouteLog(ChatMockRouteLogInput{
				AccountID:       selected.Id,
				AccountName:     selected.Name,
				Model:           modelName,
				Strategy:        routeStrategy,
				RouteScore:      routeScore,
				SelectionReason: "request_error",
				LatencyMS:       time.Since(routeStart).Milliseconds(),
				Success:         false,
				FailoverReason:  reqErr.Error(),
				TokenUsage:      0,
				RequestID:       requestID,
			})
			excluded[selected.Id] = struct{}{}
			lastErr = reqErr
			continue
		}

		if shouldFailoverByStatus(response.StatusCode) {
			failBody, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
			_ = response.Body.Close()
			failReason := describeChatMockStatusFailure(response.StatusCode)
			failPreview := strings.TrimSpace(string(failBody))
			if failPreview != "" {
				failPreview = truncateChatMockRaw(failPreview, 180)
			}
			releaseRuntimeSlot(selected.Id)
			recordMsg := fmt.Sprintf("upstream status=%d (%s)", response.StatusCode, failReason)
			if failPreview != "" {
				recordMsg += ": " + failPreview
			}
			retryAfterSec := extractRetryAfterSeconds(response.Header.Get("Retry-After"), failPreview)
			cooldownSec := computeChatMockCooldownSeconds(response.StatusCode, cfg.DefaultCooldownSecond, retryAfterSec)
			RecordChatMockAccountFailure(selected.Id, recordMsg, cooldownSec)
			RecordChatMockRouteLog(ChatMockRouteLogInput{
				AccountID:       selected.Id,
				AccountName:     selected.Name,
				Model:           modelName,
				Strategy:        routeStrategy,
				RouteScore:      routeScore,
				SelectionReason: "status_failover",
				LatencyMS:       time.Since(routeStart).Milliseconds(),
				Success:         false,
				FailoverReason:  recordMsg,
				TokenUsage:      0,
				RequestID:       requestID,
			})
			excluded[selected.Id] = struct{}{}
			if failPreview != "" {
				lastErr = fmt.Errorf("账号 %s 上游返回 %d（%s）：%s", selected.Name, response.StatusCode, failReason, failPreview)
			} else {
				lastErr = fmt.Errorf("账号 %s 上游返回 %d（%s）", selected.Name, response.StatusCode, failReason)
			}
			continue
		}

		if buffered, shouldFailover, failReason := inspectChatMockLogicalErrorPayload(response); buffered != nil {
			response = buffered
			if shouldFailover {
				releaseRuntimeSlot(selected.Id)
				excluded[selected.Id] = struct{}{}
				if isChatMockModelRestrictionReason(failReason) {
					lastErr = fmt.Errorf("账号 %s 不支持模型 %s：%s", selected.Name, strings.TrimSpace(modelName), failReason)
					continue
				}
				recordMsg := "upstream logical error: " + failReason
				RecordChatMockAccountFailure(selected.Id, recordMsg, deriveChatMockLogicalCooldown(failReason, cfg.DefaultCooldownSecond))
				RecordChatMockRouteLog(ChatMockRouteLogInput{
					AccountID:       selected.Id,
					AccountName:     selected.Name,
					Model:           modelName,
					Strategy:        routeStrategy,
					RouteScore:      routeScore,
					SelectionReason: "logical_failover",
					LatencyMS:       time.Since(routeStart).Milliseconds(),
					Success:         false,
					FailoverReason:  recordMsg,
					TokenUsage:      0,
					RequestID:       requestID,
				})
				lastErr = fmt.Errorf("账号 %s 上游返回业务错误：%s", selected.Name, failReason)
				continue
			}
		}

		RecordChatMockRouteLog(ChatMockRouteLogInput{
			AccountID:       selected.Id,
			AccountName:     selected.Name,
			Model:           modelName,
			Strategy:        routeStrategy,
			RouteScore:      routeScore,
			SelectionReason: "selected",
			LatencyMS:       time.Since(routeStart).Milliseconds(),
			Success:         true,
			FailoverReason:  "",
			TokenUsage:      0,
			RequestID:       requestID,
		})

		return response, selected, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("账号池中没有可用账号")
	}
	return nil, nil, lastErr
}

func shouldFailoverByStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return status >= 500
	}
}

func describeChatMockStatusFailure(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "鉴权失败（token 无效或已过期）"
	case http.StatusForbidden:
		return "权限不足（账号不可用或无调用权限）"
	case http.StatusTooManyRequests:
		return "触发上游限流"
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return "上游响应超时"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "上游服务异常"
	default:
		if status >= 500 {
			return "上游服务异常"
		}
		return "请求失败"
	}
}

func shouldRetryChatMockTransientStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func shouldRetryChatMockTransportErr(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	markers := []string{
		"timed out",
		"timeout",
		"deadline exceeded",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"tls handshake timeout",
		"i/o timeout",
		"http2: client connection lost",
		"server closed idle connection",
		"remote end closed connection",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func chatMockTransientRetryDelay(attempt int, retryAfterSec int) time.Duration {
	if retryAfterSec > 0 {
		delay := time.Duration(retryAfterSec) * time.Second
		if delay > 15*time.Second {
			delay = 15 * time.Second
		}
		if delay < 250*time.Millisecond {
			delay = 250 * time.Millisecond
		}
		return delay
	}
	base := 250 * time.Millisecond
	if attempt > 0 {
		base = base * time.Duration(1<<attempt)
	}
	if base > 3*time.Second {
		base = 3 * time.Second
	}
	jitter := time.Duration(rand.Intn(150)) * time.Millisecond
	return base + jitter
}

func parseRetryAfterHeader(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	if sec, err := strconv.Atoi(value); err == nil {
		if sec > 0 {
			return sec
		}
		return 0
	}
	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	remaining := int(time.Until(retryAt).Seconds())
	if remaining <= 0 {
		return 0
	}
	return remaining
}

func parseRetryAfterFromText(raw string) int {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0
	}
	match := chatMockRetryAfterRegex.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(match[1]))
	if err != nil || seconds <= 0 {
		return 0
	}
	return seconds
}

func extractRetryAfterSeconds(retryAfterHeader string, message string) int {
	if sec := parseRetryAfterHeader(retryAfterHeader); sec > 0 {
		return sec
	}
	return parseRetryAfterFromText(message)
}

func computeChatMockCooldownSeconds(status int, defaultCooldown int, retryAfterSec int) int {
	if defaultCooldown <= 0 {
		defaultCooldown = setting.ChatMockDefaultCooldownSeconds
	}
	clamp := func(v int, minValue int, maxValue int) int {
		if v < minValue {
			return minValue
		}
		if v > maxValue {
			return maxValue
		}
		return v
	}
	if retryAfterSec > 0 {
		switch status {
		case http.StatusTooManyRequests:
			return clamp(retryAfterSec, 30, 1800)
		case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return clamp(retryAfterSec, 5, 300)
		default:
			return clamp(retryAfterSec, 10, 900)
		}
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if defaultCooldown < 900 {
			return 900
		}
		return defaultCooldown
	case http.StatusTooManyRequests:
		return clamp(defaultCooldown, 120, 900)
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return clamp(defaultCooldown, 30, 180)
	default:
		return defaultCooldown
	}
}

func deriveChatMockLogicalCooldown(reason string, defaultCooldown int) int {
	retryAfterSec := parseRetryAfterFromText(reason)
	if retryAfterSec > 0 {
		return computeChatMockCooldownSeconds(http.StatusTooManyRequests, defaultCooldown, retryAfterSec)
	}
	if isChatMockTransientReason(reason) {
		return computeChatMockCooldownSeconds(http.StatusServiceUnavailable, defaultCooldown, 0)
	}
	return defaultCooldown
}

func isChatMockTransientReason(reason string) bool {
	text := strings.ToLower(strings.TrimSpace(reason))
	if text == "" {
		return false
	}
	markers := []string{
		"request failed",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"connection reset",
		"remote end closed connection",
		"operation not permitted",
		"system busy",
		"upstream busy",
		"overloaded",
		"稍后重试",
		"服务不可用",
		"网关超时",
		"请求超时",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func copyRelayHeaders(dst http.Header, src http.Header) {
	allowed := map[string]struct{}{
		"Content-Type":    {},
		"Accept":          {},
		"OpenAI-Beta":     {},
		"Openai-Beta":     {},
		"X-Request-Id":    {},
		"User-Agent":      {},
		"Cache-Control":   {},
		"Accept-Encoding": {},
	}
	for k, v := range src {
		if len(v) == 0 {
			continue
		}
		if _, ok := allowed[k]; !ok {
			continue
		}
		dst.Set(k, v[0])
	}
}

func forwardToUpstreamAccount(ctx context.Context, account *model.ChatMockAccount, rawBody []byte, incomingHeader http.Header, timeoutSec int) (*http.Response, error) {
	client, clientErr := buildChatMockHTTPClient(account.ProxyURL, timeoutSec)
	if clientErr != nil {
		return nil, fmt.Errorf("初始化代理失败: %w", clientErr)
	}

	if isLikelyChatMockCodexAccount(account) {
		if _, refreshErr := RefreshChatMockAccountAccessToken(account, false, timeoutSec); refreshErr != nil {
			common.SysError(fmt.Sprintf("ChatMock 预刷新 access token 失败 account=%d: %v", account.Id, refreshErr))
		}

		resp, err := forwardWithTransientRetry(func() (*http.Response, error) {
			return forwardToChatMockCodexResponses(ctx, client, account, rawBody, incomingHeader)
		})
		if err != nil {
			return nil, err
		}
		if resp != nil && shouldRetryByAuthStatus(resp.StatusCode) && canRefreshChatMockAccessToken(account) {
			refreshed, refreshErr := RefreshChatMockAccountAccessToken(account, true, timeoutSec)
			if refreshErr == nil && refreshed {
				_ = resp.Body.Close()
				return forwardWithTransientRetry(func() (*http.Response, error) {
					return forwardToChatMockCodexResponses(ctx, client, account, rawBody, incomingHeader)
				})
			}
		}
		return resp, nil
	}

	return forwardWithTransientRetry(func() (*http.Response, error) {
		return forwardToChatMockOpenAI(ctx, client, account, rawBody, incomingHeader)
	})
}

func forwardWithTransientRetry(doRequest func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < chatMockTransientAttempts; attempt++ {
		resp, err := doRequest()
		if err != nil {
			lastErr = err
			if attempt+1 >= chatMockTransientAttempts || !shouldRetryChatMockTransportErr(err) {
				return nil, err
			}
			time.Sleep(chatMockTransientRetryDelay(attempt, 0))
			continue
		}
		if resp == nil {
			return nil, fmt.Errorf("upstream response is nil")
		}
		if shouldRetryChatMockTransientStatus(resp.StatusCode) && attempt+1 < chatMockTransientAttempts {
			retryAfterSec := extractRetryAfterSeconds(resp.Header.Get("Retry-After"), "")
			_ = resp.Body.Close()
			time.Sleep(chatMockTransientRetryDelay(attempt, retryAfterSec))
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("transient retry attempts exhausted")
	}
	return nil, lastErr
}

func forwardToChatMockOpenAI(ctx context.Context, client *http.Client, account *model.ChatMockAccount, rawBody []byte, incomingHeader http.Header) (*http.Response, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("账号 %d BaseURL 为空", account.Id)
	}
	requestURL := baseURL + "/v1/chat/completions"
	if err := validateChatMockUpstreamURL(requestURL); err != nil {
		return nil, fmt.Errorf("上游地址被 SSRF 防护阻止: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	copyRelayHeaders(req.Header, incomingHeader)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.ApiKey))
	req.Header.Set("X-ChatMock-Account", account.Account)

	return client.Do(req)
}

func forwardToChatMockCodexResponses(ctx context.Context, client *http.Client, account *model.ChatMockAccount, rawBody []byte, incomingHeader http.Header) (*http.Response, error) {
	var chatReq dto.GeneralOpenAIRequest
	if err := common.Unmarshal(rawBody, &chatReq); err != nil {
		return nil, fmt.Errorf("解析 chat/completions 请求失败: %w", err)
	}
	chatReq.Model = normalizeChatMockCodexModelName(chatReq.Model)
	if strings.TrimSpace(chatReq.Model) == "" {
		return nil, fmt.Errorf("model 不能为空")
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(&chatReq)
	if err != nil {
		return nil, fmt.Errorf("转换到 Codex responses 请求失败: %w", err)
	}
	if isEmptyJSONRawMessage(responsesReq.Instructions) {
		if instructionRaw, marshalErr := common.Marshal(getChatMockDefaultInstructions()); marshalErr == nil {
			responsesReq.Instructions = instructionRaw
		}
	}
	storeRaw, _ := common.Marshal(false)
	responsesReq.Store = storeRaw
	// Codex /backend-api/codex/responses currently requires stream=true.
	// Downstream stream/non-stream compatibility is handled in controller layer.
	streamEnabled := true
	responsesReq.Stream = &streamEnabled

	responsesPayload, err := common.Marshal(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("序列化 Codex requests 失败: %w", err)
	}

	requestURL := resolveChatMockCodexResponsesURL(account.BaseURL)
	if err := validateChatMockUpstreamURL(requestURL); err != nil {
		return nil, fmt.Errorf("上游地址被 SSRF 防护阻止: %w", err)
	}

	buildCodexRequest := func(payload []byte) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
		if reqErr != nil {
			return nil, reqErr
		}
		copyRelayHeaders(req.Header, incomingHeader)
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.ApiKey))
		if strings.TrimSpace(account.Account) != "" {
			req.Header.Set("ChatGPT-Account-Id", strings.TrimSpace(account.Account))
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Originator", pickFirstNonEmptyHeader(incomingHeader, "Originator", chatMockCodexOriginator))
		req.Header.Set("Version", pickFirstNonEmptyHeader(incomingHeader, "Version", chatMockCodexVersion))
		req.Header.Set("Session_id", pickFirstNonEmptyHeader(incomingHeader, "Session_id", common.GetUUID()))
		req.Header.Set("User-Agent", pickFirstNonEmptyHeader(incomingHeader, "User-Agent", chatMockCodexUserAgent))
		return req, nil
	}

	req, err := buildCodexRequest(responsesPayload)
	if err != nil {
		return nil, err
	}
	upstreamResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Codex 上游失败: %w", err)
	}
	if upstreamResp.StatusCode == http.StatusBadRequest || upstreamResp.StatusCode == http.StatusUnprocessableEntity {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(upstreamResp.Body, 2<<20))
		_ = upstreamResp.Body.Close()
		if readErr != nil {
			return buildBufferedChatMockResponse(upstreamResp.StatusCode, upstreamResp.Header, nil), nil
		}
		if shouldRetryWithoutMaxOutputTokens(upstreamResp.StatusCode, bodyBytes) {
			trimmedPayload, trimErr := stripMaxOutputTokensField(responsesPayload)
			if trimErr == nil {
				retryReq, reqErr := buildCodexRequest(trimmedPayload)
				if reqErr == nil {
					retryResp, retryErr := client.Do(retryReq)
					if retryErr == nil {
						return retryResp, nil
					}
					common.SysError(fmt.Sprintf("ChatMock Codex 去除 max_output_tokens 重试失败 account=%d: %v", account.Id, retryErr))
				}
			}
		}
		return buildBufferedChatMockResponse(upstreamResp.StatusCode, upstreamResp.Header, bodyBytes), nil
	}
	return upstreamResp, nil
}

func normalizeChatMockCodexModelName(modelName string) string {
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return model
	}
	if strings.HasSuffix(model, "-openai-compact") {
		model = strings.TrimSuffix(model, "-openai-compact")
	}
	model = trimChatMockRouteModelDateSuffix(model)
	model = mapChatMockCodexModelAlias(model)
	return strings.TrimSpace(model)
}

func mapChatMockCodexModelAlias(modelName string) string {
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return model
	}
	// Some ChatGPT-backed Codex accounts do not expose the spark variants.
	// Fallback to the nearest codex sibling to keep routing stable.
	if strings.HasSuffix(model, "-codex-spark") {
		return strings.TrimSuffix(model, "-spark")
	}
	if strings.Contains(model, "-codex-spark-") {
		return strings.Replace(model, "-codex-spark-", "-codex-", 1)
	}
	return model
}

func trimChatMockRouteModelDateSuffix(modelName string) string {
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
	if !isChatMockAllDigits(year) || !isChatMockAllDigits(month) || !isChatMockAllDigits(day) {
		return modelName
	}
	return strings.Join(parts[:len(parts)-3], "-")
}

func isChatMockAllDigits(value string) bool {
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

func shouldRetryWithoutMaxOutputTokens(status int, body []byte) bool {
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(string(body)))
	if text == "" {
		return false
	}
	if !strings.Contains(text, "max_output_tokens") {
		return false
	}
	return strings.Contains(text, "unsupported parameter") ||
		strings.Contains(text, "unknown parameter") ||
		strings.Contains(text, "not allowed")
}

func stripMaxOutputTokensField(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return payload, nil
	}
	var decoded map[string]any
	if err := common.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	delete(decoded, "max_output_tokens")
	return common.Marshal(decoded)
}

func isEmptyJSONRawMessage(raw []byte) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "\"\""
}

func buildBufferedChatMockResponse(statusCode int, src http.Header, body []byte) *http.Response {
	cloned := make(http.Header, len(src))
	for key, values := range src {
		for _, value := range values {
			cloned.Add(key, value)
		}
	}
	if cloned.Get("Content-Type") == "" {
		cloned.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode:    statusCode,
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:        cloned,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func inspectChatMockLogicalErrorPayload(resp *http.Response) (*http.Response, bool, string) {
	if resp == nil {
		return nil, false, ""
	}
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusBadRequest &&
		resp.StatusCode != http.StatusUnprocessableEntity {
		return resp, false, ""
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "text/event-stream") {
		return resp, false, ""
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	buffered := buildBufferedChatMockResponse(resp.StatusCode, resp.Header, body)
	if readErr != nil {
		return buffered, false, ""
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return buffered, false, ""
	}
	if !strings.Contains(contentType, "json") && trimmed[0] != '{' {
		return buffered, false, ""
	}

	payload := map[string]any{}
	if err := common.Unmarshal(trimmed, &payload); err != nil {
		return buffered, false, ""
	}

	reason, shouldFailover := extractChatMockAccountFailoverReason(payload)
	return buffered, shouldFailover, reason
}

func extractChatMockAccountFailoverReason(payload map[string]any) (string, bool) {
	if payload == nil {
		return "", false
	}
	fragments := make([]string, 0, 8)

	appendFragment := func(value any) {
		text := strings.TrimSpace(extractChatMockErrorText(value))
		if text == "" {
			return
		}
		fragments = append(fragments, text)
	}

	if rawError, ok := payload["error"]; ok && rawError != nil {
		appendFragment(rawError)
	}
	for _, key := range []string{"message", "detail", "error_message", "errorMsg", "reason"} {
		if value, ok := payload[key]; ok && value != nil {
			appendFragment(value)
		}
	}
	if successRaw, ok := payload["success"]; ok {
		if success, parsed := parseChatMockBool(successRaw); parsed && !success {
			fragments = append(fragments, "success=false")
		}
	}
	if statusRaw, ok := payload["status"]; ok {
		statusText := strings.ToLower(strings.TrimSpace(extractChatMockErrorText(statusRaw)))
		if statusText != "" && statusText != "ok" && statusText != "success" {
			fragments = append(fragments, "status="+statusText)
		}
	}
	if len(fragments) == 0 {
		return "", false
	}

	merged := strings.TrimSpace(strings.Join(fragments, " | "))
	if merged == "" {
		return "", false
	}
	lower := strings.ToLower(merged)
	failoverMarkers := []string{
		"unauthorized",
		"forbidden",
		"invalid api key",
		"invalid_api_key",
		"invalid token",
		"token expired",
		"session expired",
		"deactivated",
		"rate limit",
		"too many requests",
		"throttle",
		"insufficient_quota",
		"quota exceeded",
		"quota exhausted",
		"quota_exhausted",
		"resource exhausted",
		"limit reached",
		"model_not_found",
		"unsupported model",
		"model is not supported",
		"does not support model",
		"does not have access to model",
		"do not have access to model",
		"not supported when using codex with a chatgpt account",
		"system busy",
		"upstream busy",
		"overloaded",
		"service unavailable",
		"temporarily unavailable",
		"gateway timeout",
		"bad gateway",
		"request failed",
		"timed out",
		"connection reset",
		"稍后重试",
		"服务不可用",
		"网关超时",
		"请求超时",
	}
	for _, marker := range failoverMarkers {
		if strings.Contains(lower, marker) {
			return truncateChatMockRaw(merged, 240), true
		}
	}
	return truncateChatMockRaw(merged, 240), false
}

func isChatMockModelRestrictionReason(reason string) bool {
	text := strings.ToLower(strings.TrimSpace(reason))
	if text == "" {
		return false
	}
	markers := []string{
		"model_not_found",
		"unsupported model",
		"model is not supported",
		"does not support model",
		"does not have access to model",
		"do not have access to model",
		"not supported when using codex with a chatgpt account",
		"model access",
		"模型不支持",
		"模型不可用",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func parseChatMockBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "ok", "success":
			return true, true
		case "false", "0", "no", "fail", "failed", "error":
			return false, true
		default:
			return false, false
		}
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	default:
		return false, false
	}
}

func extractChatMockErrorText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		parts := make([]string, 0, 8)
		for _, key := range []string{"message", "detail", "reason", "code", "type", "status", "error"} {
			raw, ok := typed[key]
			if !ok || raw == nil {
				continue
			}
			text := strings.TrimSpace(extractChatMockErrorText(raw))
			if text == "" {
				continue
			}
			if key == "code" || key == "type" || key == "status" {
				text = key + "=" + text
			}
			parts = append(parts, text)
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, ", ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(extractChatMockErrorText(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func pickFirstNonEmptyHeader(header http.Header, key string, fallback string) string {
	if header == nil {
		return fallback
	}
	value := strings.TrimSpace(header.Get(key))
	if value == "" {
		return fallback
	}
	return value
}

func pickFirstNonEmptyHeaders(header http.Header, keys ...string) string {
	if header == nil {
		return ""
	}
	for _, key := range keys {
		value := strings.TrimSpace(header.Get(strings.TrimSpace(key)))
		if value != "" {
			return value
		}
	}
	return ""
}

func isLikelyChatMockCodexAccount(account *model.ChatMockAccount) bool {
	if account == nil {
		return false
	}
	baseURL := strings.ToLower(strings.TrimSpace(account.BaseURL))
	if strings.Contains(baseURL, "chatgpt.com") || strings.Contains(baseURL, "chat.openai.com") ||
		strings.Contains(baseURL, "/backend-api") || strings.Contains(baseURL, "/codex") {
		return true
	}

	token := strings.TrimSpace(account.ApiKey)
	if token == "" {
		return false
	}
	lowerToken := strings.ToLower(token)
	if strings.HasPrefix(lowerToken, "sk-") {
		return false
	}
	if strings.Count(token, ".") >= 2 && (strings.HasPrefix(token, "eyJ") || strings.HasPrefix(token, "eyj")) {
		return true
	}
	return strings.Contains(token, "auth_") || strings.Contains(token, "sess_")
}

func IsLikelyChatMockCodexAccount(account *model.ChatMockAccount) bool {
	return isLikelyChatMockCodexAccount(account)
}

func resolveChatMockCodexResponsesURL(baseURL string) string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalized == "" {
		normalized = chatMockCodexBaseURL
	}
	if strings.HasSuffix(normalized, "/backend-api") {
		return normalized + "/codex/responses"
	}
	if strings.Contains(normalized, "/backend-api/codex/responses") {
		return normalized
	}
	if strings.Contains(normalized, "/backend-api/codex") {
		return strings.TrimSuffix(normalized, "/") + "/responses"
	}
	return normalized + chatMockCodexResponsePath
}

func shouldRetryByAuthStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func canRefreshChatMockAccessToken(account *model.ChatMockAccount) bool {
	if account == nil {
		return false
	}
	if !isLikelyChatMockCodexAccount(account) {
		return false
	}
	return strings.TrimSpace(account.RefreshToken) != ""
}

func getChatMockRefreshLock(accountID int) *sync.Mutex {
	chatMockRefreshMu.Lock()
	defer chatMockRefreshMu.Unlock()
	lock, ok := chatMockRefreshByAcct[accountID]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	chatMockRefreshByAcct[accountID] = lock
	return lock
}

func extractJWTExpiryUnix(token string) int64 {
	claims, ok := decodeJWTClaims(strings.TrimSpace(token))
	if !ok || claims == nil {
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

func shouldRefreshChatMockAccessToken(account *model.ChatMockAccount, force bool) bool {
	if account == nil || !canRefreshChatMockAccessToken(account) {
		return false
	}
	if force {
		return true
	}
	now := common.GetTimestamp()
	exp := account.TokenExpiredAt
	if exp <= 0 {
		exp = extractJWTExpiryUnix(account.ApiKey)
		if exp > 0 {
			account.TokenExpiredAt = exp
		}
	}
	if exp <= 0 {
		return false
	}
	// Refresh 10 minutes before expiry to reduce runtime auth failures.
	return exp-now <= 10*60
}

func RefreshChatMockAccountAccessToken(account *model.ChatMockAccount, force bool, timeoutSec int) (bool, error) {
	if account == nil {
		return false, fmt.Errorf("账号不存在")
	}
	if !canRefreshChatMockAccessToken(account) {
		return false, nil
	}
	if !shouldRefreshChatMockAccessToken(account, force) {
		return false, nil
	}

	lock := getChatMockRefreshLock(account.Id)
	lock.Lock()
	defer lock.Unlock()

	// Re-check after acquiring lock to avoid duplicate refreshes.
	if !shouldRefreshChatMockAccessToken(account, force) {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxInt(timeoutSec, 10))*time.Second)
	defer cancel()

	result, err := RefreshCodexOAuthTokenWithProxy(ctx, strings.TrimSpace(account.RefreshToken), strings.TrimSpace(account.ProxyURL))
	if err != nil {
		RecordChatMockAccountSnapshot(
			account.Id,
			"token_refresh",
			"error",
			"oauth refresh failed",
			map[string]any{
				"force":       force,
				"timeout_sec": timeoutSec,
				"error":       err.Error(),
			},
		)
		return false, err
	}

	updatedToken := strings.TrimSpace(result.AccessToken)
	updatedRefresh := strings.TrimSpace(result.RefreshToken)
	if updatedToken == "" || updatedRefresh == "" {
		err = fmt.Errorf("oauth refresh 响应缺少 access_token/refresh_token")
		RecordChatMockAccountSnapshot(
			account.Id,
			"token_refresh",
			"error",
			"oauth refresh payload invalid",
			map[string]any{
				"force": force,
				"error": err.Error(),
			},
		)
		return false, err
	}
	now := common.GetTimestamp()
	expiredAt := result.ExpiresAt.Unix()
	if expiredAt <= 0 {
		expiredAt = extractJWTExpiryUnix(updatedToken)
	}

	updateValues := map[string]any{
		"api_key":               updatedToken,
		"refresh_token":         updatedRefresh,
		"token_expired_at":      expiredAt,
		"token_last_refresh_at": now,
		"updated_time":          now,
	}

	if accountID, ok := ExtractCodexAccountIDFromJWT(updatedToken); ok && strings.TrimSpace(accountID) != "" {
		updateValues["account"] = strings.TrimSpace(accountID)
		account.Account = strings.TrimSpace(accountID)
	}
	if accountEmail, ok := ExtractEmailFromJWT(updatedToken); ok && strings.TrimSpace(accountEmail) != "" && strings.TrimSpace(account.Name) == "" {
		updateValues["name"] = strings.TrimSpace(strings.ToLower(accountEmail))
		account.Name = strings.TrimSpace(strings.ToLower(accountEmail))
	}

	if err = model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", account.Id).Updates(updateValues).Error; err != nil {
		RecordChatMockAccountSnapshot(
			account.Id,
			"token_refresh",
			"error",
			"persist refresh token failed",
			map[string]any{
				"force": force,
				"error": err.Error(),
			},
		)
		return false, err
	}

	account.ApiKey = updatedToken
	account.RefreshToken = updatedRefresh
	account.TokenExpiredAt = expiredAt
	account.TokenLastRefreshAt = now
	RecordChatMockAccountSnapshot(
		account.Id,
		"token_refresh",
		"ok",
		"oauth refresh succeeded",
		map[string]any{
			"force":            force,
			"token_expired_at": expiredAt,
			"refresh_at":       now,
			"account":          account.Account,
		},
	)
	return true, nil
}

func getChatMockDefaultInstructions() string {
	cfg, err := GetChatMockPoolConfig()
	if err == nil && cfg != nil {
		if text := strings.TrimSpace(cfg.DefaultInstructions); text != "" {
			return text
		}
	}
	return strings.TrimSpace(setting.ChatMockDefaultInstructions)
}

func chatMockRemainingPercentFloat(used int64, limit int64) *float64 {
	percent := chatMockRemainingPercent(used, limit)
	if percent < 0 {
		return nil
	}
	value := float64(percent)
	return &value
}

func normalizeChatMockRuntimeRequestCount(now int64, runtime ChatMockRuntimeSnapshot) (int, int64) {
	if runtime.WindowStart <= 0 {
		return 0, 0
	}
	elapsed := now - runtime.WindowStart
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= 60 {
		return 0, elapsed
	}
	return runtime.RequestCnt, elapsed
}

func appendUniqueChatMockMessage(target *[]string, message string) {
	if target == nil {
		return
	}
	text := strings.TrimSpace(message)
	if text == "" {
		return
	}
	for _, item := range *target {
		if item == text {
			return
		}
	}
	*target = append(*target, text)
}

func isLikelyChatMockCodexModelName(modelName string) bool {
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return false
	}
	if strings.Contains(model, "codex") || strings.Contains(model, "computer-use") {
		return true
	}
	return strings.HasPrefix(model, "gpt-5")
}

func chatMockTokenExpiryUnix(account *model.ChatMockAccount) int64 {
	if account == nil {
		return 0
	}
	if account.TokenExpiredAt > 0 {
		return account.TokenExpiredAt
	}
	if exp := extractJWTExpiryUnix(account.ApiKey); exp > 0 {
		account.TokenExpiredAt = exp
		return exp
	}
	return 0
}

func isChatMockCodexTokenExpiredNoRefresh(account *model.ChatMockAccount, now int64) bool {
	if account == nil || !isLikelyChatMockCodexAccount(account) {
		return false
	}
	if strings.TrimSpace(account.RefreshToken) != "" {
		return false
	}
	exp := chatMockTokenExpiryUnix(account)
	return exp > 0 && exp <= now
}

func isChatMockCodexTokenNearExpiry(account *model.ChatMockAccount, now int64, leadSec int64) bool {
	if account == nil || !isLikelyChatMockCodexAccount(account) {
		return false
	}
	exp := chatMockTokenExpiryUnix(account)
	if exp <= 0 {
		return false
	}
	if leadSec <= 0 {
		leadSec = 10 * 60
	}
	return exp-now <= leadSec
}

func computeChatMockModelAffinityFactor(account *model.ChatMockAccount, modelName string) int64 {
	if account == nil {
		return chatMockAffinityNeutral
	}
	model := strings.TrimSpace(modelName)
	if model == "" {
		return chatMockAffinityNeutral
	}
	modelLikelyCodex := isLikelyChatMockCodexModelName(model)
	accountLikelyCodex := isLikelyChatMockCodexAccount(account)
	switch {
	case modelLikelyCodex && accountLikelyCodex:
		return 145
	case modelLikelyCodex && !accountLikelyCodex:
		return 80
	case !modelLikelyCodex && accountLikelyCodex:
		return 92
	case !modelLikelyCodex && !accountLikelyCodex:
		return 110
	default:
		return chatMockAffinityNeutral
	}
}

func buildChatMockRouteDiagnoseItem(
	account *model.ChatMockAccount,
	modelName string,
	excluded map[int]struct{},
	runtime ChatMockRuntimeSnapshot,
	now int64,
	today string,
) ChatMockRouteDiagnoseItem {
	masked := account.Masked()
	masked.RouteScore = computeChatMockSelectionWeight(account, modelName)
	item := ChatMockRouteDiagnoseItem{
		Account:                   masked,
		Decision:                  "blocked",
		RouteScore:                masked.RouteScore,
		Runtime:                   runtime,
		CooldownRemainingSec:      0,
		DailyTokenUsedToday:       account.DailyTokenUsed,
		DailyTokenLimit:           account.DailyTokenLimit,
		EntitlementQuotaExhausted: isChatMockAccountEntitlementQuotaExhausted(account),
	}

	if weekly := account.WeeklyRemainingPercent(); weekly != nil {
		value := *weekly
		item.WeeklyRemainingPercent = &value
	}
	item.FiveHourRemainingPercent = chatMockRemainingPercentFloat(account.Usage5hUsed, account.Usage5hLimit)
	item.CodeReviewRemainingPct = chatMockRemainingPercentFloat(account.UsageCodeReviewUsed, account.UsageCodeReviewLimit)

	if _, skipped := excluded[account.Id]; skipped {
		item.Reasons = append(item.Reasons, "本轮重试已排除该账号")
	}

	status := strings.TrimSpace(strings.ToLower(account.Status))
	if status != constant.ChatMockAccountStatusEnabled {
		if status == "" {
			status = "unknown"
		}
		item.Reasons = append(item.Reasons, "账号状态不为 enabled（当前="+status+"）")
		appendUniqueChatMockMessage(&item.Suggestions, "将账号状态调整为 enabled 后再参与路由")
	}

	if account.CooldownUntil > now {
		item.CooldownRemainingSec = account.CooldownUntil - now
		item.Reasons = append(item.Reasons, fmt.Sprintf("账号处于冷却中（剩余 %ds）", item.CooldownRemainingSec))
		appendUniqueChatMockMessage(&item.Suggestions, "等待冷却结束，或手动清空 cooldown_until 后再重试")
	}

	if isChatMockCodexTokenExpiredNoRefresh(account, now) {
		expiredAt := chatMockTokenExpiryUnix(account)
		if expiredAt > 0 {
			item.Reasons = append(item.Reasons, fmt.Sprintf("Codex access token 已过期（%s）且缺少 refresh_token", time.Unix(expiredAt, 0).UTC().Format("2006-01-02 15:04:05 UTC")))
		} else {
			item.Reasons = append(item.Reasons, "Codex access token 可能已失效且缺少 refresh_token")
		}
		appendUniqueChatMockMessage(&item.Suggestions, "重新导入有效 access_token，并补充 refresh_token 以支持自动续期")
	}
	if isChatMockCodexTokenNearExpiry(account, now, 10*60) && canRefreshChatMockAccessToken(account) {
		expiredAt := chatMockTokenExpiryUnix(account)
		if expiredAt > now {
			item.Notes = append(item.Notes, fmt.Sprintf("Codex access token 将在 %s 过期，建议提前刷新", time.Unix(expiredAt, 0).UTC().Format("2006-01-02 15:04:05 UTC")))
			appendUniqueChatMockMessage(&item.Suggestions, "执行“刷新授权”避免请求高峰期触发鉴权失败")
		}
	}

	if !account.SupportsModel(modelName) {
		if strings.TrimSpace(modelName) == "" {
			item.Reasons = append(item.Reasons, "请求未提供 model，无法校验账号模型支持")
		} else {
			item.Reasons = append(item.Reasons, "不支持模型 "+modelName)
		}
		appendUniqueChatMockMessage(&item.Suggestions, "将该模型加入 supported_models，或切换到支持该模型的账号")
	}

	dailyTokenUsed := account.DailyTokenUsed
	if strings.TrimSpace(today) != "" && strings.TrimSpace(account.DailyTokenDate) != "" && account.DailyTokenDate != today {
		dailyTokenUsed = 0
		item.Notes = append(item.Notes, "日额度计数已跨 UTC 日期，下一次请求会自动归零")
	}
	item.DailyTokenUsedToday = dailyTokenUsed
	if account.DailyTokenLimit > 0 && dailyTokenUsed >= account.DailyTokenLimit {
		item.Reasons = append(item.Reasons, fmt.Sprintf("已触发日额度上限（%d/%d）", dailyTokenUsed, account.DailyTokenLimit))
		appendUniqueChatMockMessage(&item.Suggestions, "提高 daily_token_limit，或等待下一 UTC 日自动清零")
	}

	if item.EntitlementQuotaExhausted {
		item.Reasons = append(item.Reasons, "权益额度窗口已耗尽（5h/weekly/code-review）")
		appendUniqueChatMockMessage(&item.Suggestions, "先执行权益检查并切换到有剩余额度的账号")
	}

	effectiveReqCnt, elapsed := normalizeChatMockRuntimeRequestCount(now, runtime)
	if runtime.WindowStart > 0 && elapsed >= 60 {
		item.Notes = append(item.Notes, "RPM 计数窗口已过期，将在下一次请求自动归零")
	}
	if account.MaxConcurrency > 0 && runtime.InFlight >= account.MaxConcurrency {
		item.Reasons = append(item.Reasons, fmt.Sprintf("并发已满（%d/%d）", runtime.InFlight, account.MaxConcurrency))
		appendUniqueChatMockMessage(&item.Suggestions, "提高 max_concurrency 或增加账号副本分担并发")
	}
	if account.RPM > 0 && effectiveReqCnt >= account.RPM {
		item.Reasons = append(item.Reasons, fmt.Sprintf("RPM 达到上限（%d/%d）", effectiveReqCnt, account.RPM))
		appendUniqueChatMockMessage(&item.Suggestions, "提高 RPM 上限，或增加账号数量分摊请求速率")
	}

	affinityFactor := computeChatMockModelAffinityFactor(account, modelName)
	if strings.TrimSpace(modelName) != "" && affinityFactor != chatMockAffinityNeutral {
		if affinityFactor > chatMockAffinityNeutral {
			item.Notes = append(item.Notes, fmt.Sprintf("模型亲和系数 x%.2f：当前账号与 %s 更匹配", float64(affinityFactor)/100, modelName))
		} else {
			item.Notes = append(item.Notes, fmt.Sprintf("模型亲和系数 x%.2f：当前账号与 %s 匹配度较低", float64(affinityFactor)/100, modelName))
		}
	}

	if len(item.Reasons) == 0 {
		item.Decision = "eligible"
	}
	return item
}

func DiagnoseChatMockRoute(modelName string, excluded map[int]struct{}) (*ChatMockRouteDiagnoseResult, error) {
	if !setting.IsChatMockEnabled() {
		return nil, fmt.Errorf("CHATMOCK_ENABLED=false，账号池未启用")
	}
	if excluded == nil {
		excluded = map[int]struct{}{}
	}

	cfg, err := GetChatMockPoolConfig()
	if err != nil {
		return nil, err
	}
	accounts, _, err := model.ListChatMockAccounts("", "", "", "", 0, 0)
	if err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	today := time.Now().UTC().Format("20060102")
	runtimeSnapshot := GetChatMockRuntimeSnapshot()
	items := make([]ChatMockRouteDiagnoseItem, 0, len(accounts))
	eligibleAccounts := make([]*model.ChatMockAccount, 0, len(accounts))
	eligibleIndexByID := make(map[int]int, len(accounts))

	for _, account := range accounts {
		if account == nil {
			continue
		}
		runtime := runtimeSnapshot[account.Id]
		item := buildChatMockRouteDiagnoseItem(account, modelName, excluded, runtime, now, today)
		if item.Decision == "eligible" {
			eligibleIndexByID[account.Id] = len(items)
			eligibleAccounts = append(eligibleAccounts, account)
		}
		items = append(items, item)
	}

	result := &ChatMockRouteDiagnoseResult{
		Model:          strings.TrimSpace(modelName),
		EnableWeighted: cfg.EnableWeighted,
		SelectionPolicy: func() string {
			if cfg.EnableWeighted {
				return "weighted_random_with_quota_bias"
			}
			return "priority_weight_order"
		}(),
		Timestamp:     now,
		TotalCount:    len(items),
		EligibleCount: len(eligibleAccounts),
		BlockedCount:  len(items) - len(eligibleAccounts),
	}

	if len(eligibleAccounts) == 0 {
		result.SelectionNote = "当前没有可路由账号，请检查 blocked 原因并修复"
	} else {
		sortChatMockAccountsForSelection(eligibleAccounts, modelName)
		for rank, account := range eligibleAccounts {
			idx, ok := eligibleIndexByID[account.Id]
			if !ok {
				continue
			}
			items[idx].SelectionRank = rank + 1
			items[idx].EffectiveWeight = computeChatMockSelectionWeight(account, modelName)
		}

		selected := pickAccountByPolicy(eligibleAccounts, cfg.EnableWeighted, modelName)
		if selected != nil {
			if idx, ok := eligibleIndexByID[selected.Id]; ok {
				items[idx].Decision = "selected"
			}
			result.SelectedAccountID = selected.Id
			result.SelectedAccountName = selected.Name
		}
		if cfg.EnableWeighted {
			result.SelectionNote = "加权模式包含额度与模型亲和系数；effective_weight 越高、selection_rank 越靠前，被选中概率越高"
		} else {
			result.SelectionNote = "当前未启用加权模式，会固定选择排序第 1 的账号（排序已考虑额度与模型亲和系数）"
		}
	}

	decisionRank := func(decision string) int {
		switch decision {
		case "selected":
			return 0
		case "eligible":
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		leftRank := decisionRank(left.Decision)
		rightRank := decisionRank(right.Decision)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.SelectionRank > 0 && right.SelectionRank > 0 && left.SelectionRank != right.SelectionRank {
			return left.SelectionRank < right.SelectionRank
		}

		leftPriority := 0
		rightPriority := 0
		leftWeight := 0
		rightWeight := 0
		leftID := 0
		rightID := 0
		if left.Account != nil {
			leftPriority = left.Account.Priority
			leftWeight = left.Account.Weight
			leftID = left.Account.Id
		}
		if right.Account != nil {
			rightPriority = right.Account.Priority
			rightWeight = right.Account.Weight
			rightID = right.Account.Id
		}
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if leftWeight != rightWeight {
			return leftWeight > rightWeight
		}
		return leftID < rightID
	})
	result.Items = items
	return result, nil
}

func selectAccountForRequest(modelName string, excluded map[int]struct{}, cfg *model.ChatMockPoolConfig) (*model.ChatMockAccount, error) {
	accounts, err := model.ListEnabledChatMockAccounts()
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("账号池为空")
	}

	today := time.Now().UTC().Format("20060102")
	now := common.GetTimestamp()
	blockedByExcluded := 0
	blockedByCooldown := 0
	blockedByTokenExpired := 0
	blockedByUnsupportedModel := 0
	blockedByDailyLimit := 0
	blockedByEntitlement := 0
	candidates := make([]*model.ChatMockAccount, 0, len(accounts))
	for _, account := range accounts {
		if _, skip := excluded[account.Id]; skip {
			blockedByExcluded++
			continue
		}
		if account.CooldownUntil > 0 && account.CooldownUntil > now {
			blockedByCooldown++
			continue
		}
		if isChatMockCodexTokenExpiredNoRefresh(account, now) {
			blockedByTokenExpired++
			continue
		}
		if !account.SupportsModel(modelName) {
			blockedByUnsupportedModel++
			continue
		}
		if account.DailyTokenDate != today {
			_ = model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", account.Id).Updates(map[string]any{
				"daily_token_date": today,
				"daily_token_used": 0,
				"updated_time":     now,
			}).Error
			account.DailyTokenDate = today
			account.DailyTokenUsed = 0
		}
		if account.DailyTokenLimit > 0 && account.DailyTokenUsed >= account.DailyTokenLimit {
			blockedByDailyLimit++
			continue
		}
		// 权益窗口（5h/weekly）用量已达上限时，跳过当前账号，避免继续路由到已耗尽账号。
		if isChatMockAccountEntitlementQuotaExhausted(account) {
			blockedByEntitlement++
			continue
		}
		candidates = append(candidates, account)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf(
			"没有满足模型与限额条件的账号(model=%s, excluded=%d, cooling=%d, token_expired=%d, unsupported_model=%d, daily_limit=%d, quota_exhausted=%d)",
			strings.TrimSpace(modelName),
			blockedByExcluded,
			blockedByCooldown,
			blockedByTokenExpired,
			blockedByUnsupportedModel,
			blockedByDailyLimit,
			blockedByEntitlement,
		)
	}

	chatMockRuntimeMu.Lock()
	defer chatMockRuntimeMu.Unlock()

	blockedByConcurrency := 0
	blockedByRPM := 0
	available := make([]*model.ChatMockAccount, 0, len(candidates))
	for _, account := range candidates {
		state := getOrCreateRuntimeLocked(account.Id)
		if state.WindowStart == 0 || now-state.WindowStart >= 60 {
			state.WindowStart = now
			state.RequestCnt = 0
		}
		if account.MaxConcurrency > 0 && state.InFlight >= account.MaxConcurrency {
			blockedByConcurrency++
			continue
		}
		if account.RPM > 0 && state.RequestCnt >= account.RPM {
			blockedByRPM++
			continue
		}
		available = append(available, account)
	}
	if len(available) == 0 {
		return nil, fmt.Errorf(
			"账号池达到并发或 RPM 限制(model=%s, concurrency_limited=%d, rpm_limited=%d)",
			strings.TrimSpace(modelName),
			blockedByConcurrency,
			blockedByRPM,
		)
	}

	sortChatMockAccountsForSelection(available, modelName)
	selected := pickAccountByPolicy(available, cfg.EnableWeighted, modelName)
	state := getOrCreateRuntimeLocked(selected.Id)
	if state.WindowStart == 0 || now-state.WindowStart >= 60 {
		state.WindowStart = now
		state.RequestCnt = 0
	}
	state.InFlight++
	state.RequestCnt++
	return selected, nil
}

func pickAccountByPolicy(accounts []*model.ChatMockAccount, weighted bool, modelName string) *model.ChatMockAccount {
	if len(accounts) == 1 {
		return accounts[0]
	}
	if !weighted {
		return accounts[0]
	}
	totalWeight := int64(0)
	effectiveWeights := make([]int64, len(accounts))
	for i, acc := range accounts {
		effective := computeChatMockSelectionWeight(acc, modelName)
		if effective <= 0 {
			effective = 1
		}
		effectiveWeights[i] = effective
		totalWeight += effective
	}
	if totalWeight <= 0 {
		return accounts[rand.Intn(len(accounts))]
	}
	r := rand.Int63n(totalWeight)
	for idx, acc := range accounts {
		weight := effectiveWeights[idx]
		if weight <= 0 {
			weight = 1
		}
		r -= weight
		if r < 0 {
			return acc
		}
	}
	return accounts[len(accounts)-1]
}

func sortChatMockAccountsForSelection(accounts []*model.ChatMockAccount, modelName string) {
	if len(accounts) <= 1 {
		return
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		left := accounts[i]
		right := accounts[j]
		leftAffinity := computeChatMockModelAffinityFactor(left, modelName)
		rightAffinity := computeChatMockModelAffinityFactor(right, modelName)
		if leftAffinity != rightAffinity {
			return leftAffinity > rightAffinity
		}
		// 参考 codex-tools 的账号排序口径：先比较 weekly 剩余，再比较 5h 剩余。
		// 对于未知窗口（上游未返回）使用 -1，明确排在已知窗口之后。
		leftWeekly := chatMockRemainingPercent(left.UsageWeeklyUsed, left.UsageWeeklyLimit)
		rightWeekly := chatMockRemainingPercent(right.UsageWeeklyUsed, right.UsageWeeklyLimit)
		if leftWeekly != rightWeekly {
			return leftWeekly > rightWeekly
		}

		leftFiveHour := chatMockRemainingPercent(left.Usage5hUsed, left.Usage5hLimit)
		rightFiveHour := chatMockRemainingPercent(right.Usage5hUsed, right.Usage5hLimit)
		if leftFiveHour != rightFiveHour {
			return leftFiveHour > rightFiveHour
		}

		if left.EntitlementValid != right.EntitlementValid {
			return left.EntitlementValid && !right.EntitlementValid
		}

		leftTierRank := chatMockTierPriority(left.EntitlementTier)
		rightTierRank := chatMockTierPriority(right.EntitlementTier)
		if leftTierRank != rightTierRank {
			return leftTierRank > rightTierRank
		}

		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		if left.Weight != right.Weight {
			return left.Weight > right.Weight
		}
		return left.Id < right.Id
	})
}

func computeChatMockQuotaSelectionFactor(account *model.ChatMockAccount) int64 {
	if account == nil {
		return 1
	}
	weeklyRemain := chatMockRemainingPercent(account.UsageWeeklyUsed, account.UsageWeeklyLimit)
	fiveHourRemain := chatMockRemainingPercent(account.Usage5hUsed, account.Usage5hLimit)

	// 未知窗口（-1）降级为 0 分，避免把“未知额度”误判成高可用。
	if weeklyRemain < 0 {
		weeklyRemain = 0
	}
	if fiveHourRemain < 0 {
		fiveHourRemain = 0
	}

	// 以 weekly 为主、5h 为辅，保持与 codex-tools 一致的主次口径。
	factor := int64(1 + weeklyRemain*100 + fiveHourRemain*10)
	factor += int64(chatMockTierPriority(account.EntitlementTier) * 100)
	if account.EntitlementValid {
		factor += 1000
	} else {
		factor = factor * 60 / 100
	}
	if factor <= 0 {
		return 1
	}
	return factor
}

func computeChatMockSelectionWeight(account *model.ChatMockAccount, modelName string) int64 {
	if account == nil {
		return 1
	}
	weight := account.Weight
	if weight <= 0 {
		weight = 1
	}
	quotaFactor := computeChatMockQuotaSelectionFactor(account)
	if quotaFactor <= 0 {
		quotaFactor = 1
	}
	affinityFactor := computeChatMockModelAffinityFactor(account, modelName)
	if affinityFactor <= 0 {
		affinityFactor = chatMockAffinityNeutral
	}
	effective := int64(weight) * quotaFactor
	effective = effective * affinityFactor / chatMockAffinityNeutral
	if effective <= 0 {
		return 1
	}
	return effective
}

// EstimateChatMockRouteScore returns the same weighted score logic used by runtime selection.
// This lets the admin UI transparently display why an account is preferred.
func EstimateChatMockRouteScore(account *model.ChatMockAccount) int64 {
	if account == nil {
		return 0
	}
	return computeChatMockSelectionWeight(account, "")
}

func chatMockRemainingPercent(used int64, limit int64) int {
	if limit <= 0 {
		// 未知窗口，不参与额度优先排序。
		return -1
	}
	if used <= 0 {
		return 100
	}
	if used >= limit {
		return 0
	}
	return int((limit - used) * 100 / limit)
}

func chatMockTierPriority(rawTier string) int {
	switch strings.ToLower(strings.TrimSpace(rawTier)) {
	case "team":
		return 4
	case "pro":
		return 3
	case "plus":
		return 2
	case "free":
		return 1
	default:
		return 0
	}
}

func getOrCreateRuntimeLocked(accountID int) *chatMockRuntimeState {
	state, ok := chatMockRuntimeByAcct[accountID]
	if ok {
		return state
	}
	state = &chatMockRuntimeState{}
	chatMockRuntimeByAcct[accountID] = state
	return state
}

func releaseRuntimeSlot(accountID int) {
	chatMockRuntimeMu.Lock()
	defer chatMockRuntimeMu.Unlock()
	state := getOrCreateRuntimeLocked(accountID)
	if state.InFlight > 0 {
		state.InFlight--
	}
}

func RecordChatMockAccountSuccess(accountID int, tokenUsed int64) {
	releaseRuntimeSlot(accountID)
	now := common.GetTimestamp()
	updates := map[string]any{
		"last_health_at": now,
		"last_error":     "",
		"cooldown_until": 0,
		"updated_time":   now,
		"status":         constant.ChatMockAccountStatusEnabled,
	}
	if tokenUsed > 0 {
		updates["daily_token_used"] = gorm.Expr("daily_token_used + ?", tokenUsed)
	}
	if err := model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", accountID).Updates(updates).Error; err != nil {
		common.SysError("更新 ChatMock 账号成功状态失败: " + err.Error())
	}
	RecordChatMockAccountSnapshot(
		accountID,
		"route_result",
		"ok",
		"route success",
		map[string]any{
			"token_used": tokenUsed,
			"at":         now,
		},
	)
}

func AddChatMockAccountTokenUsage(accountID int, tokenUsed int64) {
	if accountID <= 0 || tokenUsed <= 0 {
		return
	}
	now := common.GetTimestamp()
	if err := model.DB.Model(&model.ChatMockAccount{}).
		Where("id = ?", accountID).
		Updates(map[string]any{
			"daily_token_used": gorm.Expr("daily_token_used + ?", tokenUsed),
			"updated_time":     now,
		}).Error; err != nil {
		common.SysError("更新 ChatMock 账号 token 使用量失败: " + err.Error())
	}
}

func RecordChatMockAccountFailure(accountID int, errMsg string, cooldownSec int) {
	releaseRuntimeSlot(accountID)
	now := common.GetTimestamp()
	if cooldownSec <= 0 {
		cooldownSec = setting.ChatMockDefaultCooldownSeconds
	}
	nextStatus := classifyChatMockFailureStatus(errMsg)
	if isChatMockTransientReason(errMsg) {
		nextStatus = constant.ChatMockAccountStatusCooling
	}
	updates := map[string]any{
		"last_error":     strings.TrimSpace(errMsg),
		"cooldown_until": now + int64(cooldownSec),
		"updated_time":   now,
		"status":         nextStatus,
	}
	if err := model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", accountID).Updates(updates).Error; err != nil {
		common.SysError("更新 ChatMock 账号失败状态失败: " + err.Error())
	}
	RecordChatMockAccountSnapshot(
		accountID,
		"route_result",
		"error",
		truncateChatMockRaw(strings.TrimSpace(errMsg), 300),
		map[string]any{
			"status":         nextStatus,
			"cooldown_until": now + int64(cooldownSec),
			"cooldown_sec":   cooldownSec,
			"at":             now,
		},
	)
}

func classifyChatMockFailureStatus(errMsg string) string {
	text := strings.ToLower(strings.TrimSpace(errMsg))
	if text == "" {
		return constant.ChatMockAccountStatusAbnormal
	}
	authMarkers := []string{
		"status=401",
		"status=403",
		"unauthorized",
		"invalid token",
		"invalid_token",
		"jwt expired",
		"token expired",
		"session expired",
		"forbidden",
	}
	for _, marker := range authMarkers {
		if strings.Contains(text, marker) {
			return constant.ChatMockAccountStatusAuthInvalid
		}
	}

	quotaMarkers := []string{
		"quota",
		"insufficient_quota",
		"limit reached",
		"usage_limit_reached",
		"rate limit",
		"too many requests",
		"429",
		"额度",
	}
	for _, marker := range quotaMarkers {
		if strings.Contains(text, marker) {
			if strings.Contains(text, "low") || strings.Contains(text, "不足") {
				return constant.ChatMockAccountStatusQuotaLow
			}
			return constant.ChatMockAccountStatusQuotaExhausted
		}
	}

	networkMarkers := []string{
		"timeout",
		"timed out",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"temporarily unavailable",
		"tls",
		"eof",
	}
	for _, marker := range networkMarkers {
		if strings.Contains(text, marker) {
			return constant.ChatMockAccountStatusNetworkIssue
		}
	}

	if strings.Contains(text, "manual") || strings.Contains(text, "review") || strings.Contains(text, "需要人工") {
		return constant.ChatMockAccountStatusManualReview
	}
	return constant.ChatMockAccountStatusAbnormal
}

func ExtractTokenUsageFromOpenAIResponse(body []byte) int64 {
	if len(body) == 0 {
		return 0
	}
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return 0
	}
	usageRaw, ok := payload["usage"].(map[string]any)
	if !ok {
		return 0
	}
	totalRaw, ok := usageRaw["total_tokens"]
	if !ok {
		return 0
	}
	switch v := totalRaw.(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return int64(v)
	case int64:
		if v < 0 {
			return 0
		}
		return v
	case int:
		if v < 0 {
			return 0
		}
		return int64(v)
	default:
		return 0
	}
}

func HealthCheckChatMockAccount(account *model.ChatMockAccount, timeoutSec int) error {
	if account == nil {
		return fmt.Errorf("账号不存在")
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("BaseURL 为空")
	}

	client, err := buildChatMockHTTPClient(account.ProxyURL, timeoutSec)
	if err != nil {
		return err
	}
	if _, refreshErr := RefreshChatMockAccountAccessToken(account, false, timeoutSec); refreshErr != nil {
		common.SysError(fmt.Sprintf("ChatMock 健康检查预刷新失败 account=%d: %v", account.Id, refreshErr))
	}

	checkURLs := []string{baseURL + "/v1/models"}
	if isLikelyChatMockCodexAccount(account) {
		checkURLs = append(checkURLs, resolveChatMockUsageURLs(baseURL)...)
	}

	var lastErr error
	tried := 0
	for _, requestURL := range dedupeChatMockURLs(checkURLs) {
		if requestURL == "" {
			continue
		}
		tried++
		reqErr := runChatMockHealthCheckRequest(client, account, requestURL, timeoutSec)
		if reqErr == nil {
			RecordChatMockAccountSuccess(account.Id, 0)
			RecordChatMockAccountSnapshot(
				account.Id,
				"health_check",
				"ok",
				"health check passed",
				map[string]any{
					"url":         requestURL,
					"timeout_sec": timeoutSec,
				},
			)
			return nil
		}
		lastErr = reqErr
	}

	if lastErr == nil {
		if tried == 0 {
			lastErr = fmt.Errorf("health check no endpoint")
		} else {
			lastErr = fmt.Errorf("health check failed")
		}
	}
	RecordChatMockAccountFailure(account.Id, lastErr.Error(), setting.ChatMockDefaultCooldownSeconds)
	RecordChatMockAccountSnapshot(
		account.Id,
		"health_check",
		"error",
		"health check failed",
		map[string]any{
			"timeout_sec": timeoutSec,
			"error":       lastErr.Error(),
		},
	)
	return lastErr
}

func runChatMockHealthCheckRequest(client *http.Client, account *model.ChatMockAccount, requestURL string, timeoutSec int) error {
	if err := validateChatMockUpstreamURL(requestURL); err != nil {
		return err
	}
	refreshTried := false
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxInt(timeoutSec, 10))*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.ApiKey))
		if strings.TrimSpace(account.Account) != "" {
			req.Header.Set("ChatGPT-Account-Id", strings.TrimSpace(account.Account))
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		cancel()
		if err != nil {
			return err
		}
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			return nil
		}
		if !refreshTried && shouldRetryByAuthStatus(resp.StatusCode) && canRefreshChatMockAccessToken(account) {
			refreshed, refreshErr := RefreshChatMockAccountAccessToken(account, true, timeoutSec)
			if refreshErr == nil && refreshed {
				refreshTried = true
				continue
			}
		}
		reason := describeChatMockStatusFailure(resp.StatusCode)
		preview := strings.TrimSpace(string(bodyBytes))
		if preview != "" {
			preview = truncateChatMockRaw(preview, 160)
			return fmt.Errorf("health check status=%d (%s) url=%s body=%s", resp.StatusCode, reason, requestURL, preview)
		}
		return fmt.Errorf("health check status=%d (%s) url=%s", resp.StatusCode, reason, requestURL)
	}
}

func dedupeChatMockURLs(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	output := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		url := strings.TrimSpace(item)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		output = append(output, url)
	}
	return output
}

type chatMockUsageMetrics struct {
	Usage5hUsed            int64
	HasUsage5hUsed         bool
	Usage5hLimit           int64
	HasUsage5hLimit        bool
	Usage5hResetAt         int64
	HasUsage5hReset        bool
	UsageWeeklyUsed        int64
	HasWeeklyUsed          bool
	UsageWeeklyLimit       int64
	HasWeeklyLimit         bool
	UsageWeeklyResetAt     int64
	HasWeeklyReset         bool
	UsageCodeReviewUsed    int64
	HasCodeReviewUsed      bool
	UsageCodeReviewLimit   int64
	HasCodeReviewLimit     bool
	UsageCodeReviewResetAt int64
	HasCodeReviewReset     bool
}

func (m *chatMockUsageMetrics) Merge(other chatMockUsageMetrics) {
	if !m.HasUsage5hUsed && other.HasUsage5hUsed {
		m.Usage5hUsed = other.Usage5hUsed
		m.HasUsage5hUsed = true
	}
	if !m.HasUsage5hLimit && other.HasUsage5hLimit {
		m.Usage5hLimit = other.Usage5hLimit
		m.HasUsage5hLimit = true
	}
	if !m.HasUsage5hReset && other.HasUsage5hReset {
		m.Usage5hResetAt = other.Usage5hResetAt
		m.HasUsage5hReset = true
	}
	if !m.HasWeeklyUsed && other.HasWeeklyUsed {
		m.UsageWeeklyUsed = other.UsageWeeklyUsed
		m.HasWeeklyUsed = true
	}
	if !m.HasWeeklyLimit && other.HasWeeklyLimit {
		m.UsageWeeklyLimit = other.UsageWeeklyLimit
		m.HasWeeklyLimit = true
	}
	if !m.HasWeeklyReset && other.HasWeeklyReset {
		m.UsageWeeklyResetAt = other.UsageWeeklyResetAt
		m.HasWeeklyReset = true
	}
	if !m.HasCodeReviewUsed && other.HasCodeReviewUsed {
		m.UsageCodeReviewUsed = other.UsageCodeReviewUsed
		m.HasCodeReviewUsed = true
	}
	if !m.HasCodeReviewLimit && other.HasCodeReviewLimit {
		m.UsageCodeReviewLimit = other.UsageCodeReviewLimit
		m.HasCodeReviewLimit = true
	}
	if !m.HasCodeReviewReset && other.HasCodeReviewReset {
		m.UsageCodeReviewResetAt = other.UsageCodeReviewResetAt
		m.HasCodeReviewReset = true
	}
}

func (m chatMockUsageMetrics) HasAny() bool {
	return m.HasUsage5hUsed || m.HasUsage5hLimit || m.HasUsage5hReset ||
		m.HasWeeklyUsed || m.HasWeeklyLimit || m.HasWeeklyReset ||
		m.HasCodeReviewUsed || m.HasCodeReviewLimit || m.HasCodeReviewReset
}

func (m chatMockUsageMetrics) HasAll() bool {
	return m.HasUsage5hUsed && m.HasUsage5hLimit && m.HasWeeklyUsed && m.HasWeeklyLimit
}

type chatMockUsagePayload struct {
	PlanType             string                             `json:"plan_type"`
	RateLimit            *chatMockRateLimitDetails          `json:"rate_limit"`
	CodeReviewRateLimit  *chatMockRateLimitDetails          `json:"code_review_rate_limit"`
	AdditionalRateLimits []chatMockAdditionalRateLimitEntry `json:"additional_rate_limits"`
}

type chatMockAdditionalRateLimitEntry struct {
	RateLimit *chatMockRateLimitDetails `json:"rate_limit"`
}

type chatMockRateLimitDetails struct {
	PrimaryWindow   *chatMockUsageWindowRaw `json:"primary_window"`
	SecondaryWindow *chatMockUsageWindowRaw `json:"secondary_window"`
	Allowed         *bool                   `json:"allowed"`
	LimitReached    *bool                   `json:"limit_reached"`
}

type chatMockUsageWindowRaw struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds int64    `json:"limit_window_seconds"`
	ResetAt            int64    `json:"reset_at"`
}

func RefreshChatMockAccountEntitlement(account *model.ChatMockAccount, timeoutSec int) (*ChatMockEntitlementResult, error) {
	if account == nil {
		return nil, fmt.Errorf("账号不存在")
	}

	result := &ChatMockEntitlementResult{
		Tier:      "unknown",
		CheckedAt: common.GetTimestamp(),
	}

	baseURL := strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
	if baseURL == "" {
		result.ErrorMsg = "BaseURL 为空"
		_ = persistChatMockEntitlementResult(account.Id, result)
		RecordChatMockAccountSnapshot(account.Id, "entitlement_check", "error", result.ErrorMsg, map[string]any{
			"timeout_sec": timeoutSec,
		})
		return result, fmt.Errorf("%s", result.ErrorMsg)
	}

	client, clientErr := buildChatMockHTTPClient(account.ProxyURL, timeoutSec)
	if clientErr != nil {
		result.ErrorMsg = clientErr.Error()
		_ = persistChatMockEntitlementResult(account.Id, result)
		RecordChatMockAccountSnapshot(account.Id, "entitlement_check", "error", result.ErrorMsg, map[string]any{
			"timeout_sec": timeoutSec,
		})
		return result, clientErr
	}
	if _, refreshErr := RefreshChatMockAccountAccessToken(account, false, timeoutSec); refreshErr != nil {
		common.SysError(fmt.Sprintf("ChatMock 权益检查预刷新失败 account=%d: %v", account.Id, refreshErr))
	}

	var (
		usagePayload any
		usageRaw     string
		usageSource  string
		lastUsageErr error
	)
	for _, endpoint := range resolveChatMockUsageURLs(baseURL) {
		payload, raw, reqErr := requestChatMockJSONPayloadWithAuthRetry(client, account, endpoint, timeoutSec)
		if reqErr != nil {
			lastUsageErr = reqErr
			continue
		}
		usagePayload = payload
		usageRaw = raw
		usageSource = endpoint
		break
	}

	metrics := chatMockUsageMetrics{}
	if usagePayload != nil {
		parsedMetrics, parsedTier, parsedOk := parseCodexUsagePayload(usagePayload)
		if parsedOk {
			metrics.Merge(parsedMetrics)
			if parsedTier != "" {
				result.Tier = parsedTier
			}
		}
		metrics.Merge(extractChatMockUsageMetrics(usagePayload))
		result.Valid = true
		result.Source = usageSource
	}

	var (
		subscriptionPayload any
		subscriptionSource  string
		subscriptionRaw     string
		lastSubErr          error
	)
	if usagePayload == nil || result.Tier == "unknown" || !metrics.HasAny() {
		subscriptionEndpoints := []string{
			baseURL + "/dashboard/billing/subscription",
			baseURL + "/v1/dashboard/billing/subscription",
			baseURL + "/v1/me",
		}
		for _, endpoint := range subscriptionEndpoints {
			payload, raw, reqErr := requestChatMockJSONPayloadWithAuthRetry(client, account, endpoint, timeoutSec)
			if reqErr != nil {
				lastSubErr = reqErr
				continue
			}
			subscriptionPayload = payload
			subscriptionRaw = raw
			subscriptionSource = endpoint
			break
		}
		if subscriptionPayload != nil {
			if result.Source == "" {
				result.Source = subscriptionSource
			}
			if tier := extractChatMockTier(subscriptionPayload); tier != "" {
				result.Tier = tier
			}
			if valid, ok := extractChatMockValidity(subscriptionPayload); ok {
				result.Valid = valid
			} else if !result.Valid {
				result.Valid = true
			}
			metrics.Merge(extractChatMockUsageMetrics(subscriptionPayload))
		}
	}

	if usagePayload == nil && subscriptionPayload == nil {
		failErr := lastUsageErr
		if failErr == nil {
			failErr = lastSubErr
		}
		if failErr == nil {
			failErr = fmt.Errorf("未获得权益信息响应")
		}
		result.ErrorMsg = failErr.Error()
		_ = persistChatMockEntitlementResult(account.Id, result)
		RecordChatMockAccountSnapshot(account.Id, "entitlement_check", "error", result.ErrorMsg, map[string]any{
			"timeout_sec": timeoutSec,
		})
		return result, failErr
	}

	result.Usage5hUsed = metrics.Usage5hUsed
	result.Usage5hLimit = metrics.Usage5hLimit
	result.Usage5hResetAt = metrics.Usage5hResetAt
	result.UsageWeeklyUsed = metrics.UsageWeeklyUsed
	result.UsageWeeklyLimit = metrics.UsageWeeklyLimit
	result.UsageWeeklyResetAt = metrics.UsageWeeklyResetAt
	result.UsageCodeReviewUsed = metrics.UsageCodeReviewUsed
	result.UsageCodeReviewLimit = metrics.UsageCodeReviewLimit
	result.UsageCodeReviewResetAt = metrics.UsageCodeReviewResetAt

	if result.Tier == "" {
		result.Tier = "unknown"
	}
	if usagePayload != nil && !result.Valid {
		result.ErrorMsg = appendChatMockErrorMsg(result.ErrorMsg, "上游返回账号状态无效")
	}
	if !metrics.HasAny() {
		if lastUsageErr != nil && usagePayload == nil {
			result.ErrorMsg = appendChatMockErrorMsg(result.ErrorMsg, "未获取到 5h/weekly/code-review 使用量字段: "+lastUsageErr.Error())
		} else {
			result.ErrorMsg = appendChatMockErrorMsg(result.ErrorMsg, "上游响应未包含 5h/weekly/code-review 使用量字段")
		}
	}

	rawBundle := map[string]any{
		"usage_source":        usageSource,
		"usage":               usagePayload,
		"subscription_source": subscriptionSource,
		"subscription":        subscriptionPayload,
	}
	if rawBytes, err := common.Marshal(rawBundle); err == nil {
		result.Raw = truncateChatMockRaw(string(rawBytes), 8192)
	} else {
		result.Raw = truncateChatMockRaw(subscriptionRaw+"\n"+usageRaw, 8192)
	}

	if err := persistChatMockEntitlementResult(account.Id, result); err != nil {
		RecordChatMockAccountSnapshot(account.Id, "entitlement_check", "error", err.Error(), map[string]any{
			"timeout_sec": timeoutSec,
		})
		return result, err
	}
	snapshotStatus := "ok"
	if strings.TrimSpace(result.ErrorMsg) != "" {
		snapshotStatus = "warn"
	}
	RecordChatMockAccountSnapshot(
		account.Id,
		"entitlement_check",
		snapshotStatus,
		"entitlement check finished",
		map[string]any{
			"tier":                    result.Tier,
			"valid":                   result.Valid,
			"usage_5h_used":           result.Usage5hUsed,
			"usage_5h_limit":          result.Usage5hLimit,
			"usage_weekly_used":       result.UsageWeeklyUsed,
			"usage_weekly_limit":      result.UsageWeeklyLimit,
			"usage_code_review_used":  result.UsageCodeReviewUsed,
			"usage_code_review_limit": result.UsageCodeReviewLimit,
			"checked_at":              result.CheckedAt,
			"source":                  result.Source,
			"error":                   strings.TrimSpace(result.ErrorMsg),
		},
	)
	return result, nil
}

func StartChatMockEntitlementCheckTask() {
	if !setting.IsChatMockEnabled() {
		return
	}
	chatMockEntitlementOnce.Do(func() {
		go func() {
			for {
				cfg, err := GetChatMockPoolConfig()
				if err != nil {
					common.SysError("读取 ChatMock 权益检查配置失败: " + err.Error())
					time.Sleep(60 * time.Second)
					continue
				}
				interval := cfg.EntitlementCheckInterval
				if interval <= 0 {
					time.Sleep(60 * time.Second)
					continue
				}
				interval = maxInt(interval, 60)

				accounts := make([]*model.ChatMockAccount, 0, 32)
				listErr := model.DB.Order("priority ASC, id ASC").Find(&accounts).Error
				if listErr != nil {
					common.SysError("拉取 ChatMock 账号失败: " + listErr.Error())
				} else {
					timeoutSec := maxInt(cfg.RequestTimeoutSecond, 10)
					for _, account := range accounts {
						if _, refreshErr := RefreshChatMockAccountEntitlement(account, timeoutSec); refreshErr != nil {
							common.SysError(fmt.Sprintf("ChatMock 权益检查失败 account=%d: %v", account.Id, refreshErr))
						}
					}
				}
				time.Sleep(time.Duration(interval) * time.Second)
			}
		}()
	})
}

func resolveChatMock5hColumns() (string, string, string) {
	chatMock5hColumnOnce.Do(func() {
		if model.DB == nil {
			return
		}
		if model.DB.Migrator().HasColumn(&model.ChatMockAccount{}, "usage_5h_used") {
			chatMock5hUsedColumn = "usage_5h_used"
			chatMock5hLimitColumn = "usage_5h_limit"
			chatMock5hResetColumn = "usage_5h_reset_at"
			return
		}
		chatMock5hUsedColumn = "usage5h_used"
		chatMock5hLimitColumn = "usage5h_limit"
		chatMock5hResetColumn = "usage5h_reset_at"
	})
	return chatMock5hUsedColumn, chatMock5hLimitColumn, chatMock5hResetColumn
}

func persistChatMockEntitlementResult(accountID int, result *ChatMockEntitlementResult) error {
	if accountID <= 0 || result == nil {
		return nil
	}
	usage5hUsedColumn, usage5hLimitColumn, usage5hResetColumn := resolveChatMock5hColumns()
	updates := map[string]any{
		"entitlement_tier":           strings.TrimSpace(strings.ToLower(result.Tier)),
		"entitlement_valid":          result.Valid,
		"usage_weekly_used":          result.UsageWeeklyUsed,
		"usage_weekly_limit":         result.UsageWeeklyLimit,
		"usage_weekly_reset_at":      result.UsageWeeklyResetAt,
		"usage_code_review_used":     result.UsageCodeReviewUsed,
		"usage_code_review_limit":    result.UsageCodeReviewLimit,
		"usage_code_review_reset_at": result.UsageCodeReviewResetAt,
		"entitlement_checked_at":     result.CheckedAt,
		"entitlement_error":          strings.TrimSpace(result.ErrorMsg),
		"entitlement_raw":            truncateChatMockRaw(result.Raw, 8192),
		"updated_time":               common.GetTimestamp(),
	}
	updates[usage5hUsedColumn] = result.Usage5hUsed
	updates[usage5hLimitColumn] = result.Usage5hLimit
	updates[usage5hResetColumn] = result.Usage5hResetAt
	if updates["entitlement_tier"] == "" {
		updates["entitlement_tier"] = "unknown"
	}
	return model.DB.Model(&model.ChatMockAccount{}).Where("id = ?", accountID).Updates(updates).Error
}

func buildChatMockHTTPClient(proxyURL string, timeoutSec int) (*http.Client, error) {
	client := &http.Client{Timeout: time.Duration(maxInt(timeoutSec, 10)) * time.Second}
	if strings.TrimSpace(proxyURL) == "" {
		return client, nil
	}
	proxyClient, err := NewProxyHttpClient(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	proxyClient.Timeout = time.Duration(maxInt(timeoutSec, 10)) * time.Second
	return proxyClient, nil
}

func likelyChatMockAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	return strings.Contains(text, "status=401") ||
		strings.Contains(text, "status=403") ||
		strings.Contains(text, "unauthorized") ||
		strings.Contains(text, "invalid token") ||
		strings.Contains(text, "invalid_token") ||
		strings.Contains(text, "token expired") ||
		strings.Contains(text, "jwt expired") ||
		strings.Contains(text, "session expired") ||
		strings.Contains(text, "deactivated")
}

func requestChatMockJSONPayloadWithAuthRetry(client *http.Client, account *model.ChatMockAccount, requestURL string, timeoutSec int) (any, string, error) {
	if account == nil {
		return nil, "", fmt.Errorf("账号不存在")
	}
	payload, raw, err := requestChatMockJSONPayload(client, account.ApiKey, account.Account, requestURL, timeoutSec)
	if err == nil {
		return payload, raw, nil
	}
	if !canRefreshChatMockAccessToken(account) || !likelyChatMockAuthFailure(err) {
		return payload, raw, err
	}
	refreshed, refreshErr := RefreshChatMockAccountAccessToken(account, true, timeoutSec)
	if refreshErr != nil || !refreshed {
		return payload, raw, err
	}
	return requestChatMockJSONPayload(client, account.ApiKey, account.Account, requestURL, timeoutSec)
}

func requestChatMockJSONPayload(client *http.Client, apiKey string, accountID string, requestURL string, timeoutSec int) (any, string, error) {
	if err := validateChatMockUpstreamURL(requestURL); err != nil {
		return nil, "", fmt.Errorf("上游地址被 SSRF 防护阻止: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxInt(timeoutSec, 10))*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	if strings.TrimSpace(accountID) != "" {
		req.Header.Set("ChatGPT-Account-Id", strings.TrimSpace(accountID))
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return nil, "", readErr
	}
	rawBody := string(body)
	if resp.StatusCode >= 400 {
		return nil, rawBody, fmt.Errorf("status=%d body=%s", resp.StatusCode, truncateChatMockRaw(rawBody, 256))
	}
	if strings.TrimSpace(rawBody) == "" {
		return nil, rawBody, fmt.Errorf("empty response body")
	}

	var payload any
	if err = common.Unmarshal(body, &payload); err != nil {
		return nil, rawBody, fmt.Errorf("response is not valid json: %w", err)
	}
	return payload, rawBody, nil
}

func resolveChatMockUsageURLs(baseURL string) []string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalized == "" {
		return nil
	}

	const backendPrefix = "/backend-api"
	const whamUsagePath = "/wham/usage"
	const codexUsagePath = "/api/codex/usage"

	candidates := make([]string, 0, 8)
	if strings.HasSuffix(normalized, backendPrefix) {
		origin := strings.TrimSuffix(normalized, backendPrefix)
		candidates = append(candidates,
			normalized+whamUsagePath,
			origin+backendPrefix+whamUsagePath,
			origin+codexUsagePath,
		)
	} else {
		candidates = append(candidates,
			normalized+backendPrefix+whamUsagePath,
			normalized+whamUsagePath,
			normalized+codexUsagePath,
		)
	}

	// 固定兜底地址
	candidates = append(candidates,
		"https://chatgpt.com/backend-api/wham/usage",
		"https://chatgpt.com/api/codex/usage",
		"https://chat.openai.com/backend-api/wham/usage",
		"https://chat.openai.com/api/codex/usage",
	)

	deduped := make([]string, 0, len(candidates))
	for _, url := range candidates {
		exists := false
		for _, current := range deduped {
			if current == url {
				exists = true
				break
			}
		}
		if !exists {
			deduped = append(deduped, url)
		}
	}
	return deduped
}

func parseCodexUsagePayload(payload any) (chatMockUsageMetrics, string, bool) {
	metrics := chatMockUsageMetrics{}
	if payload == nil {
		return metrics, "", false
	}
	raw, err := common.Marshal(payload)
	if err != nil {
		return metrics, "", false
	}

	parsed := chatMockUsagePayload{}
	if err = common.Unmarshal(raw, &parsed); err != nil {
		return metrics, "", false
	}

	windows := make([]chatMockUsageWindowRaw, 0, 8)
	if parsed.RateLimit != nil {
		if parsed.RateLimit.PrimaryWindow != nil {
			windows = append(windows, *parsed.RateLimit.PrimaryWindow)
		}
		if parsed.RateLimit.SecondaryWindow != nil {
			windows = append(windows, *parsed.RateLimit.SecondaryWindow)
		}
	}
	if parsed.CodeReviewRateLimit != nil {
		// codex usage payload may contain dedicated code_review_rate_limit window.
		if window := parsed.CodeReviewRateLimit.PrimaryWindow; window != nil {
			if used, ok := deriveChatMockWindowUsedPercent(window, parsed.CodeReviewRateLimit); ok {
				metrics.UsageCodeReviewUsed = used
				metrics.HasCodeReviewUsed = true
			}
			metrics.UsageCodeReviewLimit = 100
			metrics.HasCodeReviewLimit = true
			metrics.UsageCodeReviewResetAt = normalizeUnixTimestamp(window.ResetAt)
			metrics.HasCodeReviewReset = metrics.UsageCodeReviewResetAt > 0
		}
	}
	for _, item := range parsed.AdditionalRateLimits {
		if item.RateLimit == nil {
			continue
		}
		if item.RateLimit.PrimaryWindow != nil {
			windows = append(windows, *item.RateLimit.PrimaryWindow)
		}
		if item.RateLimit.SecondaryWindow != nil {
			windows = append(windows, *item.RateLimit.SecondaryWindow)
		}
	}

	if len(windows) > 0 {
		window5h, windowWeekly := pickChatMockUsageWindows(windows)
		if window5h != nil {
			if used, ok := deriveChatMockWindowUsedPercent(window5h, parsed.RateLimit); ok {
				metrics.Usage5hUsed = used
				metrics.HasUsage5hUsed = true
			}
			metrics.Usage5hLimit = 100
			metrics.HasUsage5hLimit = true
			metrics.Usage5hResetAt = normalizeUnixTimestamp(window5h.ResetAt)
			metrics.HasUsage5hReset = metrics.Usage5hResetAt > 0
		}
		if windowWeekly != nil {
			if used, ok := deriveChatMockWindowUsedPercent(windowWeekly, parsed.RateLimit); ok {
				metrics.UsageWeeklyUsed = used
				metrics.HasWeeklyUsed = true
			}
			metrics.UsageWeeklyLimit = 100
			metrics.HasWeeklyLimit = true
			metrics.UsageWeeklyResetAt = normalizeUnixTimestamp(windowWeekly.ResetAt)
			metrics.HasWeeklyReset = metrics.UsageWeeklyResetAt > 0
		}
	}

	return metrics, classifyChatMockTier(parsed.PlanType), true
}

func pickChatMockUsageWindows(windows []chatMockUsageWindowRaw) (*chatMockUsageWindowRaw, *chatMockUsageWindowRaw) {
	if len(windows) == 0 {
		return nil, nil
	}
	const (
		target5h        = int64(5 * 60 * 60)
		targetWeekly    = int64(7 * 24 * 60 * 60)
		tolerance5h     = int64(2 * 60 * 60)
		toleranceWeekly = int64(24 * 60 * 60)
	)

	pickByTarget := func(target, tolerance int64) *chatMockUsageWindowRaw {
		bestIndex := -1
		bestDiff := int64(0)
		for i := range windows {
			seconds := normalizeChatMockWindowSeconds(windows[i].LimitWindowSeconds)
			if seconds <= 0 {
				continue
			}
			diff := absInt64(seconds - target)
			if diff > tolerance {
				continue
			}
			if bestIndex == -1 || diff < bestDiff {
				bestIndex = i
				bestDiff = diff
			}
		}
		if bestIndex < 0 {
			return nil
		}
		return &windows[bestIndex]
	}

	window5h := pickByTarget(target5h, tolerance5h)
	windowWeekly := pickByTarget(targetWeekly, toleranceWeekly)

	// Fallback heuristics: infer by duration buckets when exact targets are unavailable.
	if window5h == nil || windowWeekly == nil {
		shortestIdx := -1
		longestIdx := -1
		var shortestSec int64
		var longestSec int64
		for i := range windows {
			seconds := normalizeChatMockWindowSeconds(windows[i].LimitWindowSeconds)
			if seconds <= 0 {
				continue
			}
			if shortestIdx == -1 || seconds < shortestSec {
				shortestIdx = i
				shortestSec = seconds
			}
			if longestIdx == -1 || seconds > longestSec {
				longestIdx = i
				longestSec = seconds
			}
		}

		if window5h == nil && shortestIdx >= 0 && shortestSec > 0 && shortestSec <= 12*60*60 {
			window5h = &windows[shortestIdx]
		}
		if windowWeekly == nil && longestIdx >= 0 && longestSec >= 24*60*60 {
			windowWeekly = &windows[longestIdx]
		}
	}

	return window5h, windowWeekly
}

func normalizeChatMockWindowSeconds(raw int64) int64 {
	if raw <= 0 {
		return 0
	}
	// Some upstream payloads may return milliseconds.
	if raw > 1_000_000_000 {
		return raw / 1000
	}
	return raw
}

func deriveChatMockWindowUsedPercent(window *chatMockUsageWindowRaw, detail *chatMockRateLimitDetails) (int64, bool) {
	if window == nil {
		return 0, false
	}
	if window.UsedPercent != nil {
		return normalizeUsedPercent(*window.UsedPercent), true
	}
	if detail != nil {
		if detail.LimitReached != nil && *detail.LimitReached {
			return 100, true
		}
		if detail.Allowed != nil && !*detail.Allowed {
			return 100, true
		}
	}
	return 0, false
}

func normalizeUsedPercent(value float64) int64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int64(value + 0.5)
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func extractChatMockTier(payload any) string {
	tier := ""
	walkAnyPayload(payload, "", func(path string, value any) {
		if tier != "" {
			return
		}
		lowerPath := strings.ToLower(path)
		strValue, ok := value.(string)
		if !ok {
			return
		}
		if !keyContainsAny(lowerPath, []string{"tier", "plan", "entitlement", "membership", "subscription"}) {
			return
		}
		tier = classifyChatMockTier(strValue)
	})
	if tier != "" {
		return tier
	}
	walkAnyPayload(payload, "", func(path string, value any) {
		if tier != "" {
			return
		}
		strValue, ok := value.(string)
		if !ok {
			return
		}
		tier = classifyChatMockTier(strValue)
	})
	return tier
}

func classifyChatMockTier(raw string) string {
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

func extractChatMockValidity(payload any) (bool, bool) {
	found := false
	valid := false
	walkAnyPayload(payload, "", func(path string, value any) {
		if found {
			return
		}
		lowerPath := strings.ToLower(path)
		switch typed := value.(type) {
		case bool:
			if keyContainsAny(lowerPath, []string{"valid", "active", "enabled"}) {
				valid = typed
				found = true
			}
		case string:
			if !keyContainsAny(lowerPath, []string{"status", "state", "valid", "active", "enabled", "subscription"}) {
				return
			}
			s := strings.ToLower(strings.TrimSpace(typed))
			switch {
			case strings.Contains(s, "active"), strings.Contains(s, "valid"), strings.Contains(s, "enabled"), s == "ok":
				valid = true
				found = true
			case strings.Contains(s, "expired"), strings.Contains(s, "invalid"), strings.Contains(s, "inactive"), strings.Contains(s, "disabled"), strings.Contains(s, "suspend"):
				valid = false
				found = true
			}
		}
	})
	return valid, found
}

func extractChatMockUsageMetrics(payload any) chatMockUsageMetrics {
	metrics := chatMockUsageMetrics{}
	walkAnyPayload(payload, "", func(path string, value any) {
		if path == "" {
			return
		}
		number, ok := int64FromAny(value)
		if !ok {
			return
		}
		lowerPath := strings.ToLower(path)
		leafKey := usageMetricLeafKey(lowerPath)

		is5hPath := keyContainsAny(lowerPath, []string{"5h", "five_hour", "5hour"})
		isWeeklyPath := keyContainsAny(lowerPath, []string{"weekly", "week"})
		isCodeReviewPath := keyContainsAny(lowerPath, []string{"code_review", "codereview", "code-review"})
		isUsage := keyContainsAny(leafKey, []string{"used_percent", "used", "consumed", "spent"}) || leafKey == "usage"
		isLimit := keyContainsAny(leafKey, []string{"limit", "quota", "cap", "max"})
		isReset := keyContainsAny(leafKey, []string{"reset", "reset_at", "resetat"})

		if is5hPath {
			if isUsage && !metrics.HasUsage5hUsed {
				metrics.Usage5hUsed = number
				metrics.HasUsage5hUsed = true
			}
			if isLimit && !metrics.HasUsage5hLimit {
				metrics.Usage5hLimit = number
				metrics.HasUsage5hLimit = true
			}
			if isReset && !metrics.HasUsage5hReset {
				metrics.Usage5hResetAt = normalizeUnixTimestamp(number)
				metrics.HasUsage5hReset = true
			}
		}
		if isWeeklyPath {
			if isUsage && !metrics.HasWeeklyUsed {
				metrics.UsageWeeklyUsed = number
				metrics.HasWeeklyUsed = true
			}
			if isLimit && !metrics.HasWeeklyLimit {
				metrics.UsageWeeklyLimit = number
				metrics.HasWeeklyLimit = true
			}
			if isReset && !metrics.HasWeeklyReset {
				metrics.UsageWeeklyResetAt = normalizeUnixTimestamp(number)
				metrics.HasWeeklyReset = true
			}
		}
		if isCodeReviewPath {
			if isUsage && !metrics.HasCodeReviewUsed {
				metrics.UsageCodeReviewUsed = number
				metrics.HasCodeReviewUsed = true
			}
			if isLimit && !metrics.HasCodeReviewLimit {
				metrics.UsageCodeReviewLimit = number
				metrics.HasCodeReviewLimit = true
			}
			if isReset && !metrics.HasCodeReviewReset {
				metrics.UsageCodeReviewResetAt = normalizeUnixTimestamp(number)
				metrics.HasCodeReviewReset = true
			}
		}
	})
	return metrics
}

func usageMetricLeafKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	segments := strings.Split(path, ".")
	leaf := segments[len(segments)-1]
	if idx := strings.Index(leaf, "["); idx >= 0 {
		leaf = leaf[:idx]
	}
	return strings.TrimSpace(leaf)
}

func appendChatMockErrorMsg(original string, extra string) string {
	original = strings.TrimSpace(original)
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return original
	}
	if original == "" {
		return extra
	}
	return original + "; " + extra
}

func walkAnyPayload(value any, path string, fn func(path string, value any)) {
	fn(path, value)

	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := key
			if path != "" {
				next = path + "." + key
			}
			walkAnyPayload(child, next, fn)
		}
	case []any:
		for idx, child := range typed {
			next := fmt.Sprintf("[%d]", idx)
			if path != "" {
				next = fmt.Sprintf("%s[%d]", path, idx)
			}
			walkAnyPayload(child, next, fn)
		}
	}
}

func int64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return int64(f), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func normalizeUnixTimestamp(raw int64) int64 {
	if raw <= 0 {
		return 0
	}
	// Some upstreams return milliseconds.
	if raw > 1_000_000_000_000 {
		return raw / 1000
	}
	return raw
}

func keyContainsAny(raw string, keys []string) bool {
	lower := strings.ToLower(raw)
	for _, key := range keys {
		if strings.Contains(lower, strings.ToLower(key)) {
			return true
		}
	}
	return false
}

func truncateChatMockRaw(raw string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(raw) <= max {
		return raw
	}
	return raw[:max]
}

func StartChatMockAccountHealthCheckTask() {
	if !setting.IsChatMockEnabled() {
		return
	}
	chatMockHealthOnce.Do(func() {
		go func() {
			for {
				cfg, err := GetChatMockPoolConfig()
				if err != nil {
					common.SysError("读取 ChatMock 配置失败: " + err.Error())
					time.Sleep(60 * time.Second)
					continue
				}
				interval := maxInt(cfg.HealthCheckInterval, 30)
				accounts, listErr := model.ListEnabledChatMockAccounts()
				if listErr != nil {
					common.SysError("拉取 ChatMock 账号失败: " + listErr.Error())
				} else {
					for _, account := range accounts {
						if err := HealthCheckChatMockAccount(account, maxInt(cfg.RequestTimeoutSecond, 10)); err != nil {
							common.SysError(fmt.Sprintf("ChatMock 健康检查失败 account=%d: %v", account.Id, err))
						}
					}
				}
				time.Sleep(time.Duration(interval) * time.Second)
			}
		}()
	})
}

func GetChatMockRuntimeSnapshot() map[int]ChatMockRuntimeSnapshot {
	chatMockRuntimeMu.Lock()
	defer chatMockRuntimeMu.Unlock()
	snapshot := make(map[int]ChatMockRuntimeSnapshot, len(chatMockRuntimeByAcct))
	for accountID, state := range chatMockRuntimeByAcct {
		if state == nil {
			continue
		}
		snapshot[accountID] = ChatMockRuntimeSnapshot{
			AccountID:   accountID,
			InFlight:    state.InFlight,
			WindowStart: state.WindowStart,
			RequestCnt:  state.RequestCnt,
		}
	}
	return snapshot
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func isChatMockAccountEntitlementQuotaExhausted(account *model.ChatMockAccount) bool {
	if account == nil {
		return false
	}
	now := common.GetTimestamp()
	if isChatMockQuotaWindowExhausted(account.Usage5hUsed, account.Usage5hLimit, account.Usage5hResetAt, now) {
		return true
	}
	if isChatMockQuotaWindowExhausted(
		account.UsageWeeklyUsed,
		account.UsageWeeklyLimit,
		account.UsageWeeklyResetAt,
		now,
	) {
		return true
	}
	if isChatMockQuotaWindowExhausted(
		account.UsageCodeReviewUsed,
		account.UsageCodeReviewLimit,
		account.UsageCodeReviewResetAt,
		now,
	) {
		return true
	}
	return false
}

func isChatMockQuotaWindowExhausted(used int64, limit int64, resetAt int64, now int64) bool {
	if limit <= 0 {
		return false
	}
	// If reset time has passed, treat current counters as stale and allow routing.
	if resetAt > 0 && now >= resetAt {
		return false
	}
	return used >= limit
}
