package controller

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestFillChatMockAccountFromRequestDerivesCodexIdentity(t *testing.T) {
	accessToken := buildTestJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc-from-access",
			"chatgpt_plan_type":  "free",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "from-access@example.com",
		},
	})
	idToken := buildTestJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc-from-id",
			"chatgpt_plan_type":  "plus",
		},
		"email": "from-id@example.com",
	})

	req := chatMockAccountUpsertRequest{
		AccessToken: &accessToken,
		IdToken:     &idToken,
		Type:        ptrString("codex"),
	}
	account := &model.ChatMockAccount{}
	fillChatMockAccountFromRequest(account, req)

	if account.ApiKey != accessToken {
		t.Fatalf("api_key not assigned from access_token")
	}
	if account.Account != "acc-from-id" {
		t.Fatalf("account mismatch, got %q", account.Account)
	}
	if account.Name != "from-id@example.com" {
		t.Fatalf("name mismatch, got %q", account.Name)
	}
	if account.EntitlementTier != "plus" {
		t.Fatalf("tier mismatch, got %q", account.EntitlementTier)
	}
	if account.BaseURL != "https://chatgpt.com" {
		t.Fatalf("base_url mismatch, got %q", account.BaseURL)
	}
}

func TestFillChatMockAccountFromRequestDoesNotOverrideKnownTier(t *testing.T) {
	accessToken := buildTestJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc-1",
			"chatgpt_plan_type":  "free",
		},
		"email": "test@example.com",
	})

	account := &model.ChatMockAccount{
		EntitlementTier: "team",
	}
	req := chatMockAccountUpsertRequest{
		AccessToken: &accessToken,
		Type:        ptrString("codex"),
	}
	fillChatMockAccountFromRequest(account, req)

	if account.EntitlementTier != "team" {
		t.Fatalf("expected existing tier to stay unchanged, got %q", account.EntitlementTier)
	}
}

func buildTestJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	headerBytes, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(headerBytes) +
		"." +
		base64.RawURLEncoding.EncodeToString(payloadBytes) +
		".signature"
}

func ptrString(value string) *string {
	return &value
}

func TestIsChatMockExperimentalModelName(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5.3-codex-spark", want: true},
		{model: "gpt-5.3-codex", want: false},
		{model: "gpt-5.4-openai-compact", want: false},
	}
	for _, tt := range cases {
		got := isChatMockExperimentalModelName(tt.model)
		if got != tt.want {
			t.Fatalf("isChatMockExperimentalModelName(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}
