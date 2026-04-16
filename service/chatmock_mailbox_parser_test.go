package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestAnalyzeChatMockMailContentInviteLink(t *testing.T) {
	analysis := analyzeChatMockMailContent(
		"Welcome to Team",
		"no-reply@helloteam.store",
		"Click https://helloteam.store/invite/abc123 to accept invitation.",
	)
	if analysis.EventType != constant.ChatMockMailEventTypeInviteLink {
		t.Fatalf("expected invite_link event, got %s", analysis.EventType)
	}
	if analysis.InviteLink == "" {
		t.Fatalf("expected invite link extracted")
	}
	if analysis.Classification != constant.ChatMockMailEventTypeInviteLink {
		t.Fatalf("expected invite classification, got %s", analysis.Classification)
	}
}

func TestAnalyzeChatMockMailContentVerificationCode(t *testing.T) {
	analysis := analyzeChatMockMailContent(
		"Verification Code",
		"security@openai.com",
		"Your verification code is 123456.",
	)
	if analysis.EventType != constant.ChatMockMailEventTypeVerificationCode {
		t.Fatalf("expected verification_code event, got %s", analysis.EventType)
	}
	if analysis.Code != "123456" {
		t.Fatalf("expected code 123456, got %s", analysis.Code)
	}
}

func TestAnalyzeChatMockMailContentAuthAlert(t *testing.T) {
	analysis := analyzeChatMockMailContent(
		"New login alert",
		"security@openai.com",
		"We detected a new sign in from a browser device.",
	)
	if analysis.EventType != constant.ChatMockMailEventTypeAuthAlert {
		t.Fatalf("expected auth_alert event, got %s", analysis.EventType)
	}
}

func TestAnalyzeChatMockMailContentBillingAlert(t *testing.T) {
	analysis := analyzeChatMockMailContent(
		"Payment failed",
		"billing@openai.com",
		"Your billing payment failed and card was declined.",
	)
	if analysis.EventType != constant.ChatMockMailEventTypeBillingAlert {
		t.Fatalf("expected billing_alert event, got %s", analysis.EventType)
	}
}
