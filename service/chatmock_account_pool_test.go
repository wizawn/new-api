package service

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestParseCodexUsagePayloadIncludesCodeReviewWindow(t *testing.T) {
	payload := map[string]any{
		"plan_type": "free",
		"rate_limit": map[string]any{
			"primary_window": map[string]any{
				"limit_window_seconds": int64(5 * 60 * 60),
				"reset_at":             int64(1777000000),
				"used_percent":         40.0,
			},
			"secondary_window": map[string]any{
				"limit_window_seconds": int64(7 * 24 * 60 * 60),
				"reset_at":             int64(1777600000),
				"used_percent":         15.0,
			},
		},
		"code_review_rate_limit": map[string]any{
			"primary_window": map[string]any{
				"limit_window_seconds": int64(7 * 24 * 60 * 60),
				"reset_at":             int64(1777654321),
				"used_percent":         25.0,
			},
		},
	}

	metrics, tier, ok := parseCodexUsagePayload(payload)
	if !ok {
		t.Fatalf("parseCodexUsagePayload returned ok=false")
	}
	if tier != "free" {
		t.Fatalf("unexpected tier: %s", tier)
	}
	if !metrics.HasUsage5hUsed || metrics.Usage5hUsed != 40 {
		t.Fatalf("unexpected 5h usage: has=%v value=%d", metrics.HasUsage5hUsed, metrics.Usage5hUsed)
	}
	if !metrics.HasWeeklyUsed || metrics.UsageWeeklyUsed != 15 {
		t.Fatalf("unexpected weekly usage: has=%v value=%d", metrics.HasWeeklyUsed, metrics.UsageWeeklyUsed)
	}
	if !metrics.HasCodeReviewUsed || metrics.UsageCodeReviewUsed != 25 {
		t.Fatalf(
			"unexpected code-review usage: has=%v value=%d",
			metrics.HasCodeReviewUsed,
			metrics.UsageCodeReviewUsed,
		)
	}
	if !metrics.HasCodeReviewLimit || metrics.UsageCodeReviewLimit != 100 {
		t.Fatalf(
			"unexpected code-review limit: has=%v value=%d",
			metrics.HasCodeReviewLimit,
			metrics.UsageCodeReviewLimit,
		)
	}
	if !metrics.HasCodeReviewReset || metrics.UsageCodeReviewResetAt != 1777654321 {
		t.Fatalf(
			"unexpected code-review reset: has=%v value=%d",
			metrics.HasCodeReviewReset,
			metrics.UsageCodeReviewResetAt,
		)
	}
}

func TestParseCodexUsagePayloadSkips5hWhenOnlyWeeklyWindow(t *testing.T) {
	payload := map[string]any{
		"plan_type": "free",
		"rate_limit": map[string]any{
			"primary_window": map[string]any{
				"limit_window_seconds": int64(7 * 24 * 60 * 60),
				"reset_at":             int64(1777600000),
				"used_percent":         20.0,
			},
		},
	}

	metrics, _, ok := parseCodexUsagePayload(payload)
	if !ok {
		t.Fatalf("parseCodexUsagePayload returned ok=false")
	}
	if metrics.HasUsage5hUsed {
		t.Fatalf("5h usage should be absent when payload only contains weekly window")
	}
	if !metrics.HasWeeklyUsed || metrics.UsageWeeklyUsed != 20 {
		t.Fatalf("unexpected weekly usage: has=%v value=%d", metrics.HasWeeklyUsed, metrics.UsageWeeklyUsed)
	}
}

func TestParseCodexUsagePayloadDerivesUsedPercentFromLimitReached(t *testing.T) {
	payload := map[string]any{
		"plan_type": "free",
		"rate_limit": map[string]any{
			"allowed":       false,
			"limit_reached": true,
			"primary_window": map[string]any{
				"limit_window_seconds": int64(5 * 60 * 60),
				"reset_at":             int64(1777000000),
			},
		},
	}

	metrics, _, ok := parseCodexUsagePayload(payload)
	if !ok {
		t.Fatalf("parseCodexUsagePayload returned ok=false")
	}
	if !metrics.HasUsage5hUsed || metrics.Usage5hUsed != 100 {
		t.Fatalf("unexpected 5h usage: has=%v value=%d", metrics.HasUsage5hUsed, metrics.Usage5hUsed)
	}
}

func TestExtractChatMockUsageMetricsIncludesCodeReviewPaths(t *testing.T) {
	payload := map[string]any{
		"usage": map[string]any{
			"code_review_rate_limit": map[string]any{
				"primary_window": map[string]any{
					"used_percent": 31,
					"reset_at":     int64(1777999999),
				},
				"quota_limit": 100,
			},
		},
	}

	metrics := extractChatMockUsageMetrics(payload)
	if !metrics.HasCodeReviewUsed || metrics.UsageCodeReviewUsed != 31 {
		t.Fatalf(
			"unexpected code-review usage: has=%v value=%d",
			metrics.HasCodeReviewUsed,
			metrics.UsageCodeReviewUsed,
		)
	}
	if !metrics.HasCodeReviewLimit || metrics.UsageCodeReviewLimit != 100 {
		t.Fatalf(
			"unexpected code-review limit: has=%v value=%d",
			metrics.HasCodeReviewLimit,
			metrics.UsageCodeReviewLimit,
		)
	}
	if !metrics.HasCodeReviewReset || metrics.UsageCodeReviewResetAt != 1777999999 {
		t.Fatalf(
			"unexpected code-review reset: has=%v value=%d",
			metrics.HasCodeReviewReset,
			metrics.UsageCodeReviewResetAt,
		)
	}
}

func TestExtractRetryAfterSeconds(t *testing.T) {
	if got := extractRetryAfterSeconds("120", ""); got != 120 {
		t.Fatalf("extractRetryAfterSeconds header seconds = %d, want 120", got)
	}
	if got := extractRetryAfterSeconds("", "temporarily unavailable, retry in 45s"); got != 45 {
		t.Fatalf("extractRetryAfterSeconds text marker = %d, want 45", got)
	}
	if got := extractRetryAfterSeconds("", "no retry hint"); got != 0 {
		t.Fatalf("extractRetryAfterSeconds no hint = %d, want 0", got)
	}
}

func TestComputeChatMockCooldownSeconds(t *testing.T) {
	if got := computeChatMockCooldownSeconds(http.StatusTooManyRequests, 300, 45); got != 45 {
		t.Fatalf("429 cooldown with retry-after = %d, want 45", got)
	}
	if got := computeChatMockCooldownSeconds(http.StatusServiceUnavailable, 300, 0); got != 180 {
		t.Fatalf("503 cooldown fallback = %d, want 180", got)
	}
	if got := computeChatMockCooldownSeconds(http.StatusForbidden, 300, 0); got != 900 {
		t.Fatalf("403 cooldown fallback = %d, want 900", got)
	}
}

func TestShouldRetryChatMockTransportErr(t *testing.T) {
	if !shouldRetryChatMockTransportErr(assertErr("request timeout")) {
		t.Fatalf("timeout should be retryable")
	}
	if !shouldRetryChatMockTransportErr(assertErr("connection reset by peer")) {
		t.Fatalf("connection reset should be retryable")
	}
	if shouldRetryChatMockTransportErr(assertErr("invalid api key")) {
		t.Fatalf("invalid api key should not be retryable")
	}
}

func TestExtractChatMockAccountFailoverReasonWithQuotaMarkers(t *testing.T) {
	reason, shouldFailover := extractChatMockAccountFailoverReason(map[string]any{
		"message": "quota_exhausted, retry in 60s",
	})
	if !shouldFailover {
		t.Fatalf("expected failover=true for quota_exhausted")
	}
	if reason == "" {
		t.Fatalf("expected non-empty reason")
	}
	if got := deriveChatMockLogicalCooldown(reason, 300); got != 60 {
		t.Fatalf("deriveChatMockLogicalCooldown = %d, want 60", got)
	}
}

func TestShouldRetryWithoutMaxOutputTokens(t *testing.T) {
	body := []byte(`{"error":{"message":"Unsupported parameter: max_output_tokens"}}`)
	if !shouldRetryWithoutMaxOutputTokens(http.StatusBadRequest, body) {
		t.Fatalf("expected retry when max_output_tokens is rejected")
	}
	if shouldRetryWithoutMaxOutputTokens(http.StatusBadRequest, []byte(`{"error":{"message":"invalid api key"}}`)) {
		t.Fatalf("unexpected retry for non max_output_tokens errors")
	}
}

func TestStripMaxOutputTokensField(t *testing.T) {
	original := []byte(`{"model":"gpt-5.3-codex","max_output_tokens":128,"stream":true}`)
	trimmed, err := stripMaxOutputTokensField(original)
	if err != nil {
		t.Fatalf("stripMaxOutputTokensField failed: %v", err)
	}
	if bytes.Contains(trimmed, []byte(`"max_output_tokens"`)) {
		t.Fatalf("max_output_tokens should be removed, got: %s", string(trimmed))
	}
}

func TestBuildChatMockRouteDiagnoseItemEligible(t *testing.T) {
	account := &model.ChatMockAccount{
		Id:             11,
		Name:           "eligible-account",
		Account:        "acct-11",
		ApiKey:         "sk-eligible",
		BaseURL:        "https://api.openai.com",
		Status:         constant.ChatMockAccountStatusEnabled,
		Weight:         2,
		Priority:       1,
		MaxConcurrency: 2,
		RPM:            60,
		SupportedModels: `[
			"gpt-5.3-codex"
		]`,
		DailyTokenLimit:      1000,
		DailyTokenUsed:       120,
		DailyTokenDate:       "20260412",
		Usage5hUsed:          20,
		Usage5hLimit:         100,
		UsageWeeklyUsed:      15,
		UsageWeeklyLimit:     100,
		UsageCodeReviewUsed:  10,
		UsageCodeReviewLimit: 100,
	}
	runtime := ChatMockRuntimeSnapshot{
		AccountID:   account.Id,
		InFlight:    1,
		WindowStart: 1712900000,
		RequestCnt:  10,
	}
	item := buildChatMockRouteDiagnoseItem(
		account,
		"gpt-5.3-codex",
		map[int]struct{}{},
		runtime,
		1712900020,
		"20260412",
	)
	if item.Decision != "eligible" {
		t.Fatalf("expected eligible decision, got %s reasons=%v", item.Decision, item.Reasons)
	}
	if len(item.Reasons) != 0 {
		t.Fatalf("expected no blocking reasons, got %v", item.Reasons)
	}
}

func TestBuildChatMockRouteDiagnoseItemBlocked(t *testing.T) {
	account := &model.ChatMockAccount{
		Id:              22,
		Name:            "blocked-account",
		Account:         "acct-22",
		ApiKey:          "sk-blocked",
		BaseURL:         "https://api.openai.com",
		Status:          constant.ChatMockAccountStatusDisabled,
		MaxConcurrency:  1,
		RPM:             5,
		SupportedModels: `["gpt-5.2"]`,
		DailyTokenLimit: 100,
		DailyTokenUsed:  100,
		DailyTokenDate:  "20260412",
	}
	runtime := ChatMockRuntimeSnapshot{
		AccountID:   account.Id,
		InFlight:    1,
		WindowStart: 1712900000,
		RequestCnt:  5,
	}
	item := buildChatMockRouteDiagnoseItem(
		account,
		"gpt-5.3-codex",
		map[int]struct{}{22: {}},
		runtime,
		1712900010,
		"20260412",
	)
	if item.Decision != "blocked" {
		t.Fatalf("expected blocked decision, got %s", item.Decision)
	}
	expectedMarkers := []string{
		"本轮重试已排除该账号",
		"账号状态不为 enabled",
		"不支持模型",
		"已触发日额度上限",
		"并发已满",
		"RPM 达到上限",
	}
	for _, marker := range expectedMarkers {
		found := false
		for _, reason := range item.Reasons {
			if strings.Contains(reason, marker) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected reason containing %q, got %v", marker, item.Reasons)
		}
	}
}

func TestBuildChatMockRouteDiagnoseItemExpiredCodexTokenWithoutRefresh(t *testing.T) {
	account := &model.ChatMockAccount{
		Id:              33,
		Name:            "expired-codex",
		Account:         "acct-33",
		ApiKey:          "eyJhbGciOiJub25lIn0.eyJleHAiOjE3MTI5MDAwMDB9.signature",
		BaseURL:         "https://chatgpt.com",
		Status:          constant.ChatMockAccountStatusEnabled,
		SupportedModels: `["gpt-5.3-codex"]`,
		TokenExpiredAt:  1712900000,
	}
	item := buildChatMockRouteDiagnoseItem(
		account,
		"gpt-5.3-codex",
		map[int]struct{}{},
		ChatMockRuntimeSnapshot{},
		1712900600,
		"20260412",
	)
	if item.Decision != "blocked" {
		t.Fatalf("expected blocked decision, got %s", item.Decision)
	}
	reasonMatched := false
	for _, reason := range item.Reasons {
		if strings.Contains(reason, "缺少 refresh_token") {
			reasonMatched = true
			break
		}
	}
	if !reasonMatched {
		t.Fatalf("expected reason mentioning missing refresh_token, got %v", item.Reasons)
	}
	suggestionMatched := false
	for _, suggestion := range item.Suggestions {
		if strings.Contains(suggestion, "重新导入有效 access_token") {
			suggestionMatched = true
			break
		}
	}
	if !suggestionMatched {
		t.Fatalf("expected actionable suggestion for expired token, got %v", item.Suggestions)
	}
}

func TestComputeChatMockModelAffinityFactor(t *testing.T) {
	codexAccount := &model.ChatMockAccount{
		Id:      41,
		ApiKey:  "eyJ.codex.token",
		BaseURL: "https://chatgpt.com",
	}
	openAIAccount := &model.ChatMockAccount{
		Id:      42,
		ApiKey:  "sk-openai-token",
		BaseURL: "https://api.openai.com",
	}

	codexModelCodexAffinity := computeChatMockModelAffinityFactor(codexAccount, "gpt-5.3-codex")
	codexModelOpenAIAffinity := computeChatMockModelAffinityFactor(openAIAccount, "gpt-5.3-codex")
	if codexModelCodexAffinity <= codexModelOpenAIAffinity {
		t.Fatalf(
			"expected codex account to have higher affinity for codex model, got codex=%d openai=%d",
			codexModelCodexAffinity,
			codexModelOpenAIAffinity,
		)
	}

	chatModelCodexAffinity := computeChatMockModelAffinityFactor(codexAccount, "gpt-4o")
	chatModelOpenAIAffinity := computeChatMockModelAffinityFactor(openAIAccount, "gpt-4o")
	if chatModelCodexAffinity >= chatModelOpenAIAffinity {
		t.Fatalf(
			"expected openai account to have higher affinity for gpt-4o, got codex=%d openai=%d",
			chatModelCodexAffinity,
			chatModelOpenAIAffinity,
		)
	}
}

func TestComputeChatMockSelectionWeightIncludesAffinity(t *testing.T) {
	codexAccount := &model.ChatMockAccount{
		Id:               51,
		ApiKey:           "eyJ.codex.token",
		BaseURL:          "https://chatgpt.com",
		Weight:           1,
		EntitlementTier:  "plus",
		EntitlementValid: true,
		UsageWeeklyUsed:  10,
		UsageWeeklyLimit: 100,
		Usage5hUsed:      10,
		Usage5hLimit:     100,
	}
	openAIAccount := &model.ChatMockAccount{
		Id:               52,
		ApiKey:           "sk-openai-token",
		BaseURL:          "https://api.openai.com",
		Weight:           1,
		EntitlementTier:  "plus",
		EntitlementValid: true,
		UsageWeeklyUsed:  10,
		UsageWeeklyLimit: 100,
		Usage5hUsed:      10,
		Usage5hLimit:     100,
	}

	codexWeight := computeChatMockSelectionWeight(codexAccount, "gpt-5.3-codex")
	openAIWeight := computeChatMockSelectionWeight(openAIAccount, "gpt-5.3-codex")
	if codexWeight <= openAIWeight {
		t.Fatalf(
			"expected codex account weight > openai account weight for codex model, got codex=%d openai=%d",
			codexWeight,
			openAIWeight,
		)
	}
}

func TestNormalizeChatMockCodexModelName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "gpt-5.2-openai-compact", want: "gpt-5.2"},
		{in: "gpt-5.2-2025-12-11", want: "gpt-5.2"},
		{in: "gpt-5.3-codex", want: "gpt-5.3-codex"},
		{in: "gpt-5.3-codex-spark", want: "gpt-5.3-codex"},
		{in: " gpt-5.3-codex-openai-compact ", want: "gpt-5.3-codex"},
		{in: " gpt-5.3-codex-spark-openai-compact ", want: "gpt-5.3-codex"},
	}
	for _, tt := range cases {
		got := normalizeChatMockCodexModelName(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeChatMockCodexModelName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsChatMockQuotaWindowExhausted_WithResetPassed(t *testing.T) {
	now := int64(1_700_000_000)
	if isChatMockQuotaWindowExhausted(100, 100, now-30, now) {
		t.Fatalf("window should not be exhausted after reset time passed")
	}
	if !isChatMockQuotaWindowExhausted(100, 100, now+30, now) {
		t.Fatalf("window should be exhausted before reset time")
	}
}

func TestIsChatMockAccountEntitlementQuotaExhausted_WithResetAwareWindows(t *testing.T) {
	now := common.GetTimestamp()
	account := &model.ChatMockAccount{
		UsageWeeklyUsed:    100,
		UsageWeeklyLimit:   100,
		UsageWeeklyResetAt: now - 60,
	}
	if isChatMockAccountEntitlementQuotaExhausted(account) {
		t.Fatalf("account should not be exhausted when weekly reset already passed")
	}

	account.UsageWeeklyResetAt = now + 600
	if !isChatMockAccountEntitlementQuotaExhausted(account) {
		t.Fatalf("account should remain exhausted before weekly reset time")
	}
}

func TestIsChatMockModelRestrictionReason(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "model_not_found", want: true},
		{text: "unsupported model", want: true},
		{text: "The 'gpt-5.3-codex-spark' model is not supported when using Codex with a ChatGPT account.", want: true},
		{text: "quota_exhausted retry in 60s", want: false},
	}
	for _, tt := range cases {
		got := isChatMockModelRestrictionReason(tt.text)
		if got != tt.want {
			t.Fatalf("isChatMockModelRestrictionReason(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestInspectChatMockLogicalErrorPayloadBadRequestModelRestriction(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"detail":"The 'gpt-5.3-codex-spark' model is not supported when using Codex with a ChatGPT account."}`,
		)),
	}
	buffered, shouldFailover, reason := inspectChatMockLogicalErrorPayload(resp)
	if buffered == nil {
		t.Fatalf("expected buffered response")
	}
	if !shouldFailover {
		t.Fatalf("expected shouldFailover for unsupported model")
	}
	if !strings.Contains(strings.ToLower(reason), "not supported") {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}

func assertErr(message string) error {
	return errString(message)
}
