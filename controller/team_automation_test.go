package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

type teamAutomationAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setTeamAutomationOptions(t *testing.T, values map[string]string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	common.OptionMapRWMutex.Lock()
	previous := make(map[string]string, len(common.OptionMap))
	for k, v := range common.OptionMap {
		previous[k] = v
	}
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for k, v := range values {
		common.OptionMap[k] = v
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func newTeamAutomationTestContext(method, target string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, body)
	return ctx, recorder
}

func decodeTeamAutomationAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) teamAutomationAPIResponse {
	t.Helper()

	var response teamAutomationAPIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return response
}

func TestGetTeamAutomationSettingsMasksSecret(t *testing.T) {
	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          "https://team-auto.example.com",
		"TeamAutomationUsername":         "demo",
		"TeamAutomationPasswordSecret":   "secret",
		"TeamAutomationTimeoutSeconds":   "45",
		"TeamAutomationVerifySSLEnabled": "false",
		"AutomationTempmailBaseURL":      "https://mailbox.wizawn.com",
		"AutomationTempmailAdminAuth":    "admin-secret",
		"AutomationTempmailDomain":       "caowo.de",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/settings", nil)
	GetTeamAutomationSettings(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if bytes.Contains(response.Data, []byte("secret")) {
		t.Fatalf("settings response leaked password secret: %s", string(response.Data))
	}
	if bytes.Contains(response.Data, []byte("admin-secret")) {
		t.Fatalf("settings response leaked tempmail admin auth: %s", string(response.Data))
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode settings payload: %v", err)
	}
	if payload["auth_configured"] != true {
		t.Fatalf("expected auth_configured=true, got %#v", payload["auth_configured"])
	}
	if payload["verify_ssl"] != false {
		t.Fatalf("expected verify_ssl=false, got %#v", payload["verify_ssl"])
	}
	if payload["tempmail_configured"] != true {
		t.Fatalf("expected tempmail_configured=true, got %#v", payload["tempmail_configured"])
	}
	if payload["tempmail_domain"] != "caowo.de" {
		t.Fatalf("expected tempmail_domain=caowo.de, got %#v", payload["tempmail_domain"])
	}
}

func TestExportTeamAutomationAccountsCSVReturnsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts/export/csv" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("demo:secret"))
		if got := r.Header.Get("Authorization"); got != expectedAuth {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=accounts.csv")
		_, _ = w.Write([]byte("email,status\nfoo@example.com,completed\n"))
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          server.URL,
		"TeamAutomationUsername":         "demo",
		"TeamAutomationPasswordSecret":   "secret",
		"TeamAutomationTimeoutSeconds":   "10",
		"TeamAutomationVerifySSLEnabled": "true",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/accounts/export/csv", nil)
	ExportTeamAutomationAccountsCSV(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("expected content disposition header")
	}
	if body := recorder.Body.String(); body != "email,status\nfoo@example.com,completed\n" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestExportTeamAutomationAccountsCSVAppliesLocalFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts/export/csv" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=accounts.csv")
		_, _ = w.Write([]byte("email,redeem_code,status\nfoo@example.com,foo-code,completed\nbar@example.com,bar-code,failed\n"))
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          server.URL,
		"TeamAutomationTimeoutSeconds":   "10",
		"TeamAutomationVerifySSLEnabled": "true",
	})

	ctx, recorder := newTeamAutomationTestContext(
		http.MethodGet,
		"/api/team-automation/accounts/export/csv?status=completed&search=foo",
		nil,
	)
	ExportTeamAutomationAccountsCSV(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	expected := "email,redeem_code,status\nfoo@example.com,foo-code,completed\n"
	if body := recorder.Body.String(); body != expected {
		t.Fatalf("unexpected filtered body: %q", body)
	}
}

func TestImportTeamAutomationAccountsCSVUploadsMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/accounts/import/csv" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse media type: %v", err)
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
			t.Fatalf("unexpected form field: %s", part.FormName())
		}
		if part.FileName() != "accounts.csv" {
			t.Fatalf("unexpected filename: %s", part.FileName())
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("failed to read multipart payload: %v", err)
		}
		if string(content) != "email,password,redeem_code\nfoo@example.com,secret,code\n" {
			t.Fatalf("unexpected content: %q", string(content))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"成功导入 1 个账号"}`))
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          server.URL,
		"TeamAutomationTimeoutSeconds":   "10",
		"TeamAutomationVerifySSLEnabled": "true",
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "accounts.csv")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = part.Write([]byte("email,password,redeem_code\nfoo@example.com,secret,code\n")); err != nil {
		t.Fatalf("failed to write multipart body: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/accounts/import/csv", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ImportTeamAutomationAccountsCSV(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload map[string]string
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode import payload: %v", err)
	}
	if payload["message"] != "成功导入 1 个账号" {
		t.Fatalf("unexpected import payload: %#v", payload)
	}
}

func TestFilterTeamAutomationAccountsCSVMissingStatusColumn(t *testing.T) {
	_, err := filterTeamAutomationAccountsCSV(
		[]byte("email,redeem_code\nfoo@example.com,foo-code\n"),
		"completed",
		"",
	)
	if err == nil || err.Error() != "team automation export csv missing status column" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTeamAutomationCapabilitiesDefaultsToLocalMode(t *testing.T) {
	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "false",
		"TeamAutomationMode":             "",
		"TeamAutomationBaseURL":          "",
		"AutomationMode":                 "",
		"AutomationArtifactRoot":         "/tmp/newapi-team-automation-test",
		"AutomationWorkerBaseURL":        "",
		"AutomationWorkerTimeoutSeconds": "30",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/capabilities", nil)
	ctx.Set("role", common.RoleRootUser)
	GetTeamAutomationCapabilities(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", payload["mode"])); got != "local" {
		t.Fatalf("expected mode=local, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", payload["enabled"])); got != "true" {
		t.Fatalf("expected enabled=true, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", payload["error_code"])); got != "" && got != "<nil>" {
		t.Fatalf("expected no error_code in local default mode, got %q", got)
	}
}

func TestImportTeamAutomationAccountsCSVRequiresFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream should not be called when file is missing")
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          server.URL,
		"TeamAutomationTimeoutSeconds":   "10",
		"TeamAutomationVerifySSLEnabled": "true",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodPost, "/api/team-automation/accounts/import/csv", nil)
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	ImportTeamAutomationAccountsCSV(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if response.Success {
		t.Fatal("expected failure response when file is missing")
	}
	if response.Message != "file is required" {
		t.Fatalf("unexpected error message: %s", response.Message)
	}
}

func TestGetTeamAutomationCapabilitiesReturnsLocalDefaultWhenDisabled(t *testing.T) {
	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled": "false",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/capabilities", nil)
	GetTeamAutomationCapabilities(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["source"] != "local" {
		t.Fatalf("unexpected source: %#v", payload["source"])
	}
	if payload["mode"] != "local" {
		t.Fatalf("unexpected mode: %#v", payload["mode"])
	}
	if payload["enabled"] != true {
		t.Fatalf("expected enabled=true for local default mode, got %#v", payload["enabled"])
	}
	if payload["error_code"] != nil {
		t.Fatalf("did not expect error_code in local default mode, got %#v", payload["error_code"])
	}
	modules, ok := payload["modules"].([]any)
	if !ok || len(modules) == 0 {
		t.Fatalf("expected non-empty modules, got %#v", payload["modules"])
	}
	redeemSites, ok := payload["redeem_sites"].([]any)
	if !ok || len(redeemSites) == 0 {
		t.Fatalf("expected non-empty redeem sites, got %#v", payload["redeem_sites"])
	}
	foundCustom := false
	for _, raw := range redeemSites {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["value"] == "__custom__" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom redeem site option in fallback payload: %#v", redeemSites)
	}
}

func TestGetTeamAutomationCapabilitiesUsesRemotePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if r.URL.Path != "/api/capabilities" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"modules": [{"label":"全部模块","value":""},{"label":"模块 2：Team 兑换","value":"2"}],
			"redeem_sites": ["chatgpt.com"],
			"supports": {"run_options": false, "redeem_site": true},
			"message": "remote-ok"
		}`))
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"AutomationMode":                   "remote",
		"AutomationWorkerBaseURL":          server.URL,
		"AutomationWorkerUsername":         "demo",
		"AutomationWorkerPasswordSecret":   "secret",
		"AutomationWorkerTimeoutSeconds":   "10",
		"AutomationWorkerVerifySSLEnabled": "true",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/capabilities", nil)
	GetTeamAutomationCapabilities(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["source"] != "remote" {
		t.Fatalf("unexpected source: %#v", payload["source"])
	}
	if payload["remote_path"] != "/api/capabilities" {
		t.Fatalf("unexpected remote_path: %#v", payload["remote_path"])
	}
	if payload["message"] != "remote-ok" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
	if payload["remote_status"] != float64(http.StatusOK) {
		t.Fatalf("unexpected remote_status: %#v", payload["remote_status"])
	}

	supports, ok := payload["supports"].(map[string]any)
	if !ok {
		t.Fatalf("expected supports map, got %#v", payload["supports"])
	}
	if supports["run_options"] != false {
		t.Fatalf("expected run_options=false, got %#v", supports["run_options"])
	}

	redeemSites, ok := payload["redeem_sites"].([]any)
	if !ok || len(redeemSites) == 0 {
		t.Fatalf("expected redeem sites, got %#v", payload["redeem_sites"])
	}
	siteValueSet := map[string]bool{}
	foundCustom := false
	for _, raw := range redeemSites {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		value := strings.TrimSpace(fmt.Sprintf("%v", item["value"]))
		if value != "" {
			siteValueSet[strings.ToLower(value)] = true
		}
		if value == "__custom__" {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom redeem site option in remote payload: %#v", redeemSites)
	}
	if !siteValueSet["helloteam.store"] {
		t.Fatalf("expected helloteam.store option in merged redeem_sites: %#v", redeemSites)
	}
	if !siteValueSet["teamxz.store"] {
		t.Fatalf("expected teamxz.store option in merged redeem_sites: %#v", redeemSites)
	}
}

func TestGetTeamAutomationCapabilitiesLocalModeUsesResolvedBaseURL(t *testing.T) {
	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled": "true",
		"TeamAutomationMode":    "local_playwright",
		"TeamAutomationBaseURL": "",
	})

	ctx, recorder := newTeamAutomationTestContext(http.MethodGet, "/api/team-automation/capabilities", nil)
	GetTeamAutomationCapabilities(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["mode"] != "local" {
		t.Fatalf("unexpected mode: %#v", payload["mode"])
	}
	if payload["configured"] != true {
		t.Fatalf("expected configured=true in local mode fallback, got %#v", payload["configured"])
	}
	if payload["error_code"] != nil {
		t.Fatalf("did not expect error_code in local mode, got %#v", payload["error_code"])
	}
}

func TestStartTeamAutomationRunNormalizesRedeemSite(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode proxied payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"execution_id":"run-1","status":"queued"}`))
	}))
	defer server.Close()

	setTeamAutomationOptions(t, map[string]string{
		"TeamAutomationEnabled":          "true",
		"TeamAutomationBaseURL":          server.URL,
		"TeamAutomationUsername":         "demo",
		"TeamAutomationPasswordSecret":   "secret",
		"TeamAutomationTimeoutSeconds":   "10",
		"TeamAutomationVerifySSLEnabled": "true",
	})

	ctx, recorder := newTeamAutomationTestContext(
		http.MethodPost,
		"/api/team-automation/run",
		strings.NewReader(`{"module":"2","redeem_site":"https://helloteam.store/path/","run_options":{"max_concurrency":2}}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	StartTeamAutomationRun(ctx)

	response := decodeTeamAutomationAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	if captured == nil {
		t.Fatal("expected proxied payload")
	}
	if captured["redeem_site"] != "helloteam.store/path" {
		t.Fatalf("unexpected normalized redeem_site: %#v", captured["redeem_site"])
	}

	runOptions, ok := captured["run_options"].(map[string]any)
	if !ok {
		t.Fatalf("expected run_options object, got %#v", captured["run_options"])
	}
	if runOptions["redeem_site"] != "helloteam.store/path" {
		t.Fatalf("expected run_options.redeem_site synced, got %#v", runOptions["redeem_site"])
	}
	if runOptions["site_strategy"] != "helloteam" {
		t.Fatalf("expected helloteam site strategy preset, got %#v", runOptions["site_strategy"])
	}
}

func TestNormalizeTeamAutomationRunPayloadRejectsInvalidRunOptions(t *testing.T) {
	_, err := normalizeTeamAutomationRunPayload(map[string]any{
		"run_options": "oops",
	})
	if err == nil || err.Error() != "team automation run_options must be a JSON object" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeTeamAutomationRunPayloadInfersTeamXZStrategy(t *testing.T) {
	normalized, err := normalizeTeamAutomationRunPayload(map[string]any{
		"redeem_site": "https://teamxz.store",
		"run_options": map[string]any{
			"max_concurrency": 3,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload, ok := normalized.(map[string]any)
	if !ok {
		t.Fatalf("expected normalized map payload, got %#v", normalized)
	}
	if payload["redeem_site"] != "teamxz.store" {
		t.Fatalf("unexpected normalized redeem_site: %#v", payload["redeem_site"])
	}
	runOptions, ok := payload["run_options"].(map[string]any)
	if !ok {
		t.Fatalf("expected run_options object, got %#v", payload["run_options"])
	}
	if runOptions["site_strategy"] != "teamxz" {
		t.Fatalf("expected teamxz site strategy preset, got %#v", runOptions["site_strategy"])
	}
}

func TestParseAutomationImportCSVRecordsSupportsAliasColumns(t *testing.T) {
	csvBody := "account,pass,code,site,mailbox,metadata\nfoo@example.com,secret,redeem-1,helloteam.store,12,plugin_import\n"
	records, err := parseAutomationImportCSVRecords([]byte(csvBody))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	record := records[0]
	if record.Email != "foo@example.com" {
		t.Fatalf("unexpected email: %s", record.Email)
	}
	if record.Password != "secret" {
		t.Fatalf("unexpected password: %s", record.Password)
	}
	if record.RedeemCode != "redeem-1" {
		t.Fatalf("unexpected redeem code: %s", record.RedeemCode)
	}
	if record.RedeemSite != "helloteam.store" {
		t.Fatalf("unexpected redeem site: %s", record.RedeemSite)
	}
	if record.MailboxID != 12 {
		t.Fatalf("unexpected mailbox id: %d", record.MailboxID)
	}
}

func TestParseAutomationImportJSONRecordsSupportsTextInput(t *testing.T) {
	records, err := parseAutomationImportJSONRecords(map[string]any{
		"text": "foo@example.com,secret,code-1\nbar@example.com,pass-2,code-2,helloteam.store",
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Email != "foo@example.com" || records[0].Password != "secret" || records[0].RedeemCode != "code-1" {
		t.Fatalf("unexpected first record: %#v", records[0])
	}
	if records[1].RedeemSite != "helloteam.store" {
		t.Fatalf("unexpected second record redeem site: %#v", records[1])
	}
}

func TestNormalizeAutomationStartFromExcludesDeprecatedModules(t *testing.T) {
	if value := normalizeAutomationStartFrom("module7"); value != "" {
		t.Fatalf("expected module7 to be disabled, got %q", value)
	}
	if value := normalizeAutomationStartFrom("module5"); value != "" {
		t.Fatalf("expected module5 to be disabled, got %q", value)
	}
}

func TestNormalizeAutomationImportStatusUsesStartFromDefault(t *testing.T) {
	status := normalizeAutomationImportStatus("", constant.AutomationModuleTeamRedeem)
	if status != constant.AutomationAccountStatusGenerated {
		t.Fatalf("expected generated, got %s", status)
	}
	status = normalizeAutomationImportStatus("invalid_status", constant.AutomationModuleImportTarget)
	if status != constant.AutomationAccountStatusConverted {
		t.Fatalf("expected converted, got %s", status)
	}
	status = normalizeAutomationImportStatus("completed", constant.AutomationModuleTeamRedeem)
	if status != constant.AutomationAccountStatusCompleted {
		t.Fatalf("expected completed, got %s", status)
	}
}
