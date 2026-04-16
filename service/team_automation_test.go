package service

import (
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTeamAutomationClientDoSupportsJSONArrayAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run/history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("demo:secret"))
		if got := r.Header.Get("Authorization"); got != expectedAuth {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"execution_id":"abc123","status":"completed"}]`))
	}))
	defer server.Close()

	client, err := NewTeamAutomationClient(TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		Username:       "demo",
		Password:       "secret",
		TimeoutSeconds: 5,
		VerifySSL:      true,
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	payload, statusCode, err := client.Do(context.Background(), http.MethodGet, "/api/run/history", nil, nil)
	if err != nil {
		t.Fatalf("unexpected do error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", statusCode)
	}

	items, ok := payload.([]any)
	if !ok {
		t.Fatalf("payload is not array: %#v", payload)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected payload length: %d", len(items))
	}
}

func TestTeamAutomationClientDoReturnsErrorMessageFromPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"bad config"}`))
	}))
	defer server.Close()

	client, err := NewTeamAutomationClient(TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		TimeoutSeconds: 5,
		VerifySSL:      true,
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	_, statusCode, err := client.Do(context.Background(), http.MethodGet, "/api/health", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if statusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	if err.Error() != "bad config" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestTeamAutomationConfigSanitizedDoesNotExposePassword(t *testing.T) {
	config := TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        "https://team-auto.example.com",
		Username:       "demo",
		Password:       "secret",
		TimeoutSeconds: 30,
		VerifySSL:      true,
	}

	sanitized := config.Sanitized()
	if _, ok := sanitized["password"]; ok {
		t.Fatalf("sanitized config should not include password: %#v", sanitized)
	}
	if sanitized["auth_configured"] != true {
		t.Fatalf("expected auth_configured=true, got %#v", sanitized["auth_configured"])
	}
}

func TestTeamAutomationConfigValidateRequiresBaseURLWhenEnabled(t *testing.T) {
	err := (TeamAutomationConfig{Enabled: true}).Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "team automation base url is not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamAutomationConfigValidateRequiresHTTPOrHTTPS(t *testing.T) {
	err := (TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        "ftp://team-auto.example.com",
		TimeoutSeconds: 30,
		VerifySSL:      true,
	}).Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "team automation base url must use http or https" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamAutomationConfigValidateRequiresTimeoutInRange(t *testing.T) {
	err := (TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        "https://team-auto.example.com",
		TimeoutSeconds: 301,
		VerifySSL:      true,
	}).Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "team automation timeout seconds must be between 5 and 300" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamAutomationConfigValidateUsesDefaultBaseURLInLocalMode(t *testing.T) {
	err := (TeamAutomationConfig{
		Enabled:        true,
		Mode:           TeamAutomationModeLocalPlaywright,
		BaseURL:        "",
		TimeoutSeconds: 30,
		VerifySSL:      true,
	}).Validate()
	if err != nil {
		t.Fatalf("expected local mode config to be valid without explicit base url, got: %v", err)
	}
}

func TestTeamAutomationConfigSanitizedIncludesResolvedBaseURL(t *testing.T) {
	config := TeamAutomationConfig{
		Enabled:        true,
		Mode:           TeamAutomationModeLocalPlaywright,
		BaseURL:        "",
		TimeoutSeconds: 30,
		VerifySSL:      true,
	}
	sanitized := config.Sanitized()
	if sanitized["resolved_base_url"] != defaultLocalRunnerBaseURL {
		t.Fatalf("unexpected resolved_base_url: %#v", sanitized["resolved_base_url"])
	}
	if sanitized["mode"] != TeamAutomationModeLocalPlaywright {
		t.Fatalf("unexpected mode: %#v", sanitized["mode"])
	}
}

func TestTeamAutomationClientDownloadReturnsCSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts/export/csv" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=accounts.csv")
		_, _ = w.Write([]byte("email,status\nfoo@example.com,completed\n"))
	}))
	defer server.Close()

	client, err := NewTeamAutomationClient(TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		TimeoutSeconds: 5,
		VerifySSL:      true,
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	download, err := client.Download(context.Background(), "/api/accounts/export/csv", nil)
	if err != nil {
		t.Fatalf("unexpected download error: %v", err)
	}
	if download.ContentDisposition == "" {
		t.Fatal("expected content disposition")
	}
	if len(download.Body) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestTeamAutomationClientUploadMultipartReturnsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/accounts/import/csv" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("unexpected parse media type error: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("unexpected media type: %s", mediaType)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("expected multipart body: %v", err)
		}
		if part.FormName() != "file" {
			t.Fatalf("unexpected form name: %s", part.FormName())
		}
		if part.FileName() != "accounts.csv" {
			t.Fatalf("unexpected file name: %s", part.FileName())
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if string(content) != "email,password,redeem_code\nfoo@example.com,secret,code\n" {
			t.Fatalf("unexpected multipart content: %s", string(content))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"成功导入 1 个账号"}`))
	}))
	defer server.Close()

	client, err := NewTeamAutomationClient(TeamAutomationConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		TimeoutSeconds: 5,
		VerifySSL:      true,
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	payload, statusCode, err := client.Upload(
		context.Background(),
		"/api/accounts/import/csv",
		nil,
		"file",
		"accounts.csv",
		[]byte("email,password,redeem_code\nfoo@example.com,secret,code\n"),
	)
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %#v", payload)
	}
	if payloadMap["message"] != "成功导入 1 个账号" {
		t.Fatalf("unexpected payload: %#v", payloadMap)
	}
}
