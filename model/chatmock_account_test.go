package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestChatMockAccountSupportsModel_WithWildcardAndCompatSuffix(t *testing.T) {
	account := &ChatMockAccount{}
	if err := account.SetSupportedModels([]string{
		"gpt-5*",
		"gpt-5.2",
		"claude-opus-4-6",
	}); err != nil {
		t.Fatalf("SetSupportedModels failed: %v", err)
	}

	cases := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5.3-codex", want: true},
		{model: "gpt-5.2-2025-12-11", want: true},
		{model: "gpt-5.2-openai-compact", want: true},
		{model: "claude-opus-4-6", want: true},
		{model: "claude-sonnet-4-6", want: false},
	}

	for _, tt := range cases {
		got := account.SupportsModel(tt.model)
		if got != tt.want {
			t.Fatalf("SupportsModel(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestRemainingPercentWithReset(t *testing.T) {
	now := int64(1_700_000_000)

	fullAfterReset := remainingPercentWithReset(100, 100, now-10, now)
	if fullAfterReset == nil || *fullAfterReset != 100 {
		t.Fatalf("remainingPercentWithReset should return 100 after reset, got %#v", fullAfterReset)
	}

	normal := remainingPercentWithReset(80, 100, now+600, now)
	if normal == nil || *normal != 20 {
		t.Fatalf("remainingPercentWithReset normal expected 20, got %#v", normal)
	}
}

func TestWeeklyRemainingPercent_UsesResetAt(t *testing.T) {
	now := common.GetTimestamp()
	account := &ChatMockAccount{
		UsageWeeklyUsed:    100,
		UsageWeeklyLimit:   100,
		UsageWeeklyResetAt: now - 1,
	}
	remaining := account.WeeklyRemainingPercent()
	if remaining == nil || *remaining != 100 {
		t.Fatalf("WeeklyRemainingPercent should be 100 after reset, got %#v", remaining)
	}
}
