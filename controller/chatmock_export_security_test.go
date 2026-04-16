package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestResolveChatMockSensitiveExportOption(t *testing.T) {
	gin.SetMode(gin.TestMode)

	buildCtx := func(query string, role int, userID int) *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest("GET", "/api/admin/chatmock/accounts/export?"+query, nil)
		ctx.Request = req
		ctx.Set("role", role)
		ctx.Set("id", userID)
		return ctx
	}

	noSensitiveCtx := buildCtx("", common.RoleAdminUser, 10)
	include, msg := resolveChatMockSensitiveExportOption(noSensitiveCtx)
	if include {
		t.Fatalf("expected include=false when no sensitive flag")
	}
	if msg != "" {
		t.Fatalf("expected empty message when no sensitive flag, got %q", msg)
	}

	adminCtx := buildCtx("include_sensitive=true", common.RoleAdminUser, 11)
	include, msg = resolveChatMockSensitiveExportOption(adminCtx)
	if include {
		t.Fatalf("expected include=false for non-root sensitive export")
	}
	if !strings.Contains(msg, "root") {
		t.Fatalf("expected root restriction message, got %q", msg)
	}

	rootNoConfirmCtx := buildCtx("include_sensitive=true", common.RoleRootUser, 12)
	include, msg = resolveChatMockSensitiveExportOption(rootNoConfirmCtx)
	if include {
		t.Fatalf("expected include=false without confirm flag")
	}
	if !strings.Contains(msg, "confirm_sensitive_export") {
		t.Fatalf("expected confirm flag hint, got %q", msg)
	}

	rootConfirmCtx := buildCtx("include_sensitive=true&confirm_sensitive_export=true", common.RoleRootUser, 13)
	include, msg = resolveChatMockSensitiveExportOption(rootConfirmCtx)
	if !include {
		t.Fatalf("expected include=true for root with confirm flag")
	}
	if msg != "" {
		t.Fatalf("expected empty message for approved export, got %q", msg)
	}
}

func TestSanitizeChatMockExportDefaultMasking(t *testing.T) {
	account := &model.ChatMockAccount{
		ApiKey:       "sk-secret-123456",
		IdToken:      "id-token-raw",
		RefreshToken: "refresh-token-raw",
	}
	maskedAccount := sanitizeChatMockAccountForExport(account, false)
	if maskedAccount == nil {
		t.Fatalf("masked account should not be nil")
	}
	if maskedAccount.ApiKey == account.ApiKey {
		t.Fatalf("api_key should be masked by default")
	}
	if maskedAccount.IdToken == account.IdToken {
		t.Fatalf("id_token should be masked by default")
	}
	if maskedAccount.RefreshToken == account.RefreshToken {
		t.Fatalf("refresh_token should be masked by default")
	}

	mailbox := &model.ChatMockMailbox{
		Password: "mailbox-secret",
	}
	maskedMailbox := sanitizeChatMockMailboxForExport(mailbox, false)
	if maskedMailbox == nil {
		t.Fatalf("masked mailbox should not be nil")
	}
	if maskedMailbox.Password == mailbox.Password {
		t.Fatalf("mailbox password should be masked by default")
	}

	rawAccount := sanitizeChatMockAccountForExport(account, true)
	if rawAccount.ApiKey != account.ApiKey {
		t.Fatalf("api_key should be kept when includeSensitive=true")
	}
	rawMailbox := sanitizeChatMockMailboxForExport(mailbox, true)
	if rawMailbox.Password != mailbox.Password {
		t.Fatalf("mailbox password should be kept when includeSensitive=true")
	}
}
