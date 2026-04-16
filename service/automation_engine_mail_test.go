package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestNormalizeAutomationMailClassification(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "ALL", want: ""},
		{in: "any", want: ""},
		{in: constant.ChatMockMailEventTypeInviteLink, want: constant.ChatMockMailEventTypeInviteLink},
		{in: "system_notice", want: constant.ChatMockMailEventTypeSystemNotice},
		{in: "custom_value", want: "custom_value"},
	}
	for _, tc := range cases {
		got := normalizeAutomationMailClassification(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeAutomationMailClassification(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractInviteLinkFromAutomationMailEvent(t *testing.T) {
	eventWithRaw := &model.ChatMockMailEvent{
		RawSnippet: "click here https://helloteam.store/invite/abc123 to join",
	}
	if got := extractInviteLinkFromAutomationMailEvent(eventWithRaw); got != "https://helloteam.store/invite/abc123" {
		t.Fatalf("unexpected invite link from raw snippet: %q", got)
	}

	payloadRaw, err := common.Marshal(map[string]any{
		"invite_link": "https://teamxz.store/invite/xyz789",
	})
	if err != nil {
		t.Fatalf("marshal parsed payload failed: %v", err)
	}
	eventWithPayload := &model.ChatMockMailEvent{
		ParsedPayload: string(payloadRaw),
	}
	if got := extractInviteLinkFromAutomationMailEvent(eventWithPayload); got != "https://teamxz.store/invite/xyz789" {
		t.Fatalf("unexpected invite link from parsed payload: %q", got)
	}

	eventWithoutLink := &model.ChatMockMailEvent{
		RawSnippet:    "no link here",
		ParsedPayload: `{"verification_code":"123456"}`,
	}
	if got := extractInviteLinkFromAutomationMailEvent(eventWithoutLink); got != "" {
		t.Fatalf("expected empty invite link, got %q", got)
	}
}

func TestCreateTempMailAddressParsesWorkerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/new_address" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("x-admin-auth"); got != "admin-secret" {
			t.Fatalf("unexpected x-admin-auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"address":"demo@caowo.de","jwt":"jwt-token","address_id":12,"password":"generated-pass"}`))
	}))
	defer server.Close()

	result, err := createTempMailAddress(context.Background(), AutomationSettings{
		TempMailBaseURL:      server.URL,
		TempMailAdminAuth:    "admin-secret",
		WorkerTimeoutSeconds: 10,
	}, "demo", "caowo.de", false)
	if err != nil {
		t.Fatalf("createTempMailAddress returned error: %v", err)
	}
	if result.Address != "demo@caowo.de" {
		t.Fatalf("unexpected address: %q", result.Address)
	}
	if result.JWT != "jwt-token" {
		t.Fatalf("unexpected jwt: %q", result.JWT)
	}
	if result.AddressID != 12 {
		t.Fatalf("unexpected address id: %d", result.AddressID)
	}
}
