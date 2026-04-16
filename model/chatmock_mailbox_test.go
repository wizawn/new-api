package model

import "testing"

func TestChatMockMailboxValidateForSave_AllowsEmptySMTPHost(t *testing.T) {
	mailbox := &ChatMockMailbox{
		Name:     "imap-only",
		Email:    "imap-only@example.com",
		Username: "imap-only@example.com",
		Password: "app-pass",
		ImapHost: "imap.example.com",
		SmtpHost: "",
	}
	if err := mailbox.ValidateForSave(); err != nil {
		t.Fatalf("expected smtp_host optional, got error: %v", err)
	}
}

func TestChatMockMailboxValidateForSave_RequiresIMAPHost(t *testing.T) {
	mailbox := &ChatMockMailbox{
		Name:     "invalid",
		Email:    "invalid@example.com",
		Username: "invalid@example.com",
		Password: "app-pass",
		ImapHost: "",
	}
	if err := mailbox.ValidateForSave(); err == nil {
		t.Fatalf("expected error when imap_host is empty")
	}
}

func TestChatMockMailboxValidateForSave_AllowsWizawnProviderWithoutIMAP(t *testing.T) {
	mailbox := &ChatMockMailbox{
		Name:     "wizawn-provider",
		Email:    "temp@caowo.de",
		Username: "",
		Password: "jwt-token",
		ImapHost: "",
		Metadata: `{"provider":"wizawn_http","base_url":"https://mailbox.wizawn.com","domains":["wizawn.com","caowo.de"]}`,
	}
	mailbox.Normalize()
	if mailbox.Username != mailbox.Email {
		t.Fatalf("expected username fallback to email, got %q", mailbox.Username)
	}
	if err := mailbox.ValidateForSave(); err != nil {
		t.Fatalf("expected wizawn provider mailbox to skip imap validation, got error: %v", err)
	}
}
