package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestModulePipelineSingleAndForward(t *testing.T) {
	single := modulePipeline(constant.AutomationModuleTeamRedeem, map[string]any{})
	if len(single) != 1 || single[0] != constant.AutomationModuleTeamRedeem {
		t.Fatalf("expected single redeem module, got %#v", single)
	}

	forward := modulePipeline(constant.AutomationModuleTeamRedeem, map[string]any{
		"continue_from_module": true,
	})
	expected := []string{
		constant.AutomationModuleTeamRedeem,
		constant.AutomationModuleGmailInviteWatch,
		constant.AutomationModuleBrowserLoginCapture,
		constant.AutomationModuleImportTarget,
	}
	if len(forward) != len(expected) {
		t.Fatalf("unexpected pipeline length: %#v", forward)
	}
	for idx := range expected {
		if forward[idx] != expected[idx] {
			t.Fatalf("unexpected pipeline at %d: got %s want %s", idx, forward[idx], expected[idx])
		}
	}

	full := modulePipeline(constant.AutomationModuleCloudflareEmailGenerate, map[string]any{
		"continue_from_module": true,
	})
	expectedFull := []string{
		constant.AutomationModuleCloudflareEmailGenerate,
		constant.AutomationModuleTeamRedeem,
		constant.AutomationModuleGmailInviteWatch,
		constant.AutomationModuleBrowserLoginCapture,
		constant.AutomationModuleImportTarget,
	}
	if len(full) != len(expectedFull) {
		t.Fatalf("unexpected full pipeline length: %#v", full)
	}
	for idx := range expectedFull {
		if full[idx] != expectedFull[idx] {
			t.Fatalf("unexpected full pipeline at %d: got %s want %s", idx, full[idx], expectedFull[idx])
		}
	}
}

func TestPickProxyCandidateAppliesRetryOffset(t *testing.T) {
	candidates := []*automationProxyCandidate{
		{Server: "socks5://127.0.0.1:7001"},
		{Server: "socks5://127.0.0.1:7002"},
		{Server: "socks5://127.0.0.1:7003"},
	}
	account := &model.AutomationAccount{Id: 1}

	selected := pickProxyCandidate(candidates, map[string]any{
		"proxy_rotate_mode": "stable",
	}, account)
	if selected == nil || selected.Server != "socks5://127.0.0.1:7002" {
		t.Fatalf("unexpected initial candidate: %#v", selected)
	}

	retrySelected := pickProxyCandidate(candidates, map[string]any{
		"proxy_rotate_mode":       "stable",
		"_proxy_candidate_offset": 1,
	}, account)
	if retrySelected == nil || retrySelected.Server != "socks5://127.0.0.1:7003" {
		t.Fatalf("unexpected retry candidate: %#v", retrySelected)
	}
}

func TestShouldRetryAutomationModuleWithNextProxy(t *testing.T) {
	settings := AutomationSettings{
		RegisterProxyGateway: "socks5://127.0.0.1:7890",
	}
	if !shouldRetryAutomationModuleWithNextProxy(context.Background(), newPipelineTestErr("注册流程命中 Cloudflare 挑战（waiting room），请切换代理或降低自动化频率"), map[string]any{
		"proxy_retry_count": 1,
	}, settings) {
		t.Fatal("expected cloudflare challenge to trigger proxy retry")
	}
	if shouldRetryAutomationModuleWithNextProxy(context.Background(), newPipelineTestErr("unsupported module"), map[string]any{
		"proxy_retry_count": 1,
	}, settings) {
		t.Fatal("did not expect unsupported module to trigger proxy retry")
	}
	if shouldRetryAutomationModuleWithNextProxy(context.Background(), newPipelineTestErr("context canceled"), map[string]any{
		"proxy_retry_count": 1,
		"disable_proxy":     true,
	}, settings) {
		t.Fatal("did not expect disabled proxy run to retry")
	}
}

func TestResolveLaunchProxyTriggersGatewaySwitchForRetry(t *testing.T) {
	var gotMethod string
	var gotToken string
	var gotPayload map[string]any
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotToken = r.Header.Get("Authorization")
		defer r.Body.Close()
		decodeErr = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	settings := AutomationSettings{
		RegisterProxyGateway: "socks5://127.0.0.1:7890",
	}
	candidate, err := resolveDirectProxyCandidate(nil, settings)
	if err != nil {
		t.Fatalf("resolveDirectProxyCandidate returned unexpected error: %v", err)
	}
	if candidate == nil {
		t.Fatal("expected gateway candidate")
	}

	launchProxy, err := resolveLaunchProxy(context.Background(), candidate, map[string]any{
		"proxy_switch_url":    server.URL,
		"proxy_switch_token":  "abc123",
		"_proxy_force_switch": true,
	}, settings)
	if err != nil {
		t.Fatalf("resolveLaunchProxy returned unexpected error: %v", err)
	}
	if launchProxy == nil || launchProxy.Server != "socks5://127.0.0.1:7890" {
		t.Fatalf("unexpected launch proxy: %#v", launchProxy)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if decodeErr != nil {
		t.Fatalf("decode switch payload: %v", decodeErr)
	}
	if gotToken != "Bearer abc123" {
		t.Fatalf("unexpected authorization header: %q", gotToken)
	}
	if action := toRunOptionString(gotPayload, "action"); action != "next" {
		t.Fatalf("unexpected switch payload: %#v", gotPayload)
	}
}

func TestSummarizeExecutionFailureText(t *testing.T) {
	raw := `{
		"account_12":{"status":"failed","module":"team_redeem","error":"proxy timeout"},
		"account_8":{"status":"success","module":"team_redeem"},
		"account_15":{"status":"failed","module":"browser_login_capture","error":"cloudflare challenge"}
	}`
	text := summarizeExecutionFailureText(raw, "")
	if text == "" {
		t.Fatal("expected failure summary text")
	}
	if !strings.Contains(text, "账号#12[team_redeem] proxy timeout") {
		t.Fatalf("unexpected failure summary: %q", text)
	}
	if !strings.Contains(text, "账号#15[browser_login_capture] cloudflare challenge") {
		t.Fatalf("unexpected failure summary: %q", text)
	}
}

func TestBuildExecutionHistoryAppliesFilters(t *testing.T) {
	oldDB := model.DB
	defer func() {
		model.DB = oldDB
	}()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AutomationExecution{}, &model.AutomationExecutionLog{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	model.DB = db

	executions := []*model.AutomationExecution{
		{
			ExecutionID:   "exec-failed",
			Module:        constant.AutomationModuleTeamRedeem,
			Status:        constant.AutomationExecutionStatusFailed,
			ErrorMessage:  "proxy timeout",
			ResultSummary: `{"account_1":{"status":"failed","module":"team_redeem","error":"proxy timeout"}}`,
		},
		{
			ExecutionID:  "exec-completed",
			Module:       constant.AutomationModuleBrowserLoginCapture,
			Status:       constant.AutomationExecutionStatusCompleted,
			ErrorMessage: "",
		},
	}
	for _, execution := range executions {
		if err := execution.Insert(); err != nil {
			t.Fatalf("insert execution: %v", err)
		}
	}
	if err := (&model.AutomationExecutionLog{
		ExecutionID: "exec-failed",
		Message:     "proxy retry scheduled",
	}).Insert(); err != nil {
		t.Fatalf("insert execution log: %v", err)
	}

	failedHistory, err := BuildExecutionHistory(1, 20, AutomationExecutionHistoryFilter{
		OnlyFailed: true,
	})
	if err != nil {
		t.Fatalf("BuildExecutionHistory failed filter returned error: %v", err)
	}
	failedItems := failedHistory["items"].([]map[string]any)
	if len(failedItems) != 1 || failedItems[0]["execution_id"] != "exec-failed" {
		t.Fatalf("unexpected failed filter result: %#v", failedItems)
	}

	retriedHistory, err := BuildExecutionHistory(1, 20, AutomationExecutionHistoryFilter{
		OnlyProxyRetried: true,
	})
	if err != nil {
		t.Fatalf("BuildExecutionHistory retried filter returned error: %v", err)
	}
	retriedItems := retriedHistory["items"].([]map[string]any)
	if len(retriedItems) != 1 || retriedItems[0]["execution_id"] != "exec-failed" {
		t.Fatalf("unexpected proxy retried result: %#v", retriedItems)
	}

	searchHistory, err := BuildExecutionHistory(1, 20, AutomationExecutionHistoryFilter{
		Search: "browser_login",
	})
	if err != nil {
		t.Fatalf("BuildExecutionHistory search filter returned error: %v", err)
	}
	searchItems := searchHistory["items"].([]map[string]any)
	if len(searchItems) != 1 || searchItems[0]["execution_id"] != "exec-completed" {
		t.Fatalf("unexpected search result: %#v", searchItems)
	}
}

func newPipelineTestErr(message string) error {
	return &pipelineTestErr{message: message}
}

type pipelineTestErr struct {
	message string
}

func (e *pipelineTestErr) Error() string {
	return e.message
}
