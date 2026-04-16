package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestClassifyChatMockFailureStatus(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "auth invalid",
			message: "upstream status=401 unauthorized",
			want:    constant.ChatMockAccountStatusAuthInvalid,
		},
		{
			name:    "quota low",
			message: "quota low, please recharge",
			want:    constant.ChatMockAccountStatusQuotaLow,
		},
		{
			name:    "quota exhausted",
			message: "insufficient_quota: limit reached",
			want:    constant.ChatMockAccountStatusQuotaExhausted,
		},
		{
			name:    "network issue",
			message: "gateway timeout while forwarding request",
			want:    constant.ChatMockAccountStatusNetworkIssue,
		},
		{
			name:    "manual review",
			message: "manual review required by upstream policy",
			want:    constant.ChatMockAccountStatusManualReview,
		},
		{
			name:    "abnormal fallback",
			message: "unexpected malformed response",
			want:    constant.ChatMockAccountStatusAbnormal,
		},
	}

	for _, tt := range cases {
		got := classifyChatMockFailureStatus(tt.message)
		if got != tt.want {
			t.Fatalf("%s: got %s want %s", tt.name, got, tt.want)
		}
	}
}
