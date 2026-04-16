package service

import (
	"strings"
	"testing"
)

func TestBuildProxyCandidatesSkipsInvalidEntries(t *testing.T) {
	settings := AutomationSettings{
		RegisterProxyPool: "bad://proxy\nsocks5://127.0.0.1:7890",
	}
	candidates, err := buildProxyCandidates(nil, settings)
	if err != nil {
		t.Fatalf("buildProxyCandidates returned unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 valid candidate, got %d", len(candidates))
	}
	if candidates[0].Mode != "socks5" {
		t.Fatalf("unexpected candidate mode: %q", candidates[0].Mode)
	}
}

func TestBuildProxyCandidatesRejectsAllInvalidEntries(t *testing.T) {
	settings := AutomationSettings{
		RegisterProxyPool: "bad://proxy\nalso-bad://proxy",
	}
	_, err := buildProxyCandidates(nil, settings)
	if err == nil {
		t.Fatal("expected error when all proxy entries are invalid")
	}
	if got := err.Error(); !strings.Contains(got, "proxy pool contains no valid entries") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildAutomationProxyPoolSummary(t *testing.T) {
	summary := BuildAutomationProxyPoolSummary(sampleVMessNode+"\nhttp://127.0.0.1:8080\nbad://proxy", "socks5://127.0.0.1:7890")
	if total := summary["total_entries"].(int); total != 3 {
		t.Fatalf("unexpected total_entries: %d", total)
	}
	if valid := summary["valid_entries"].(int); valid != 2 {
		t.Fatalf("unexpected valid_entries: %d", valid)
	}
	if invalid := summary["invalid_entries"].(int); invalid != 1 {
		t.Fatalf("unexpected invalid_entries: %d", invalid)
	}
	if vmess := summary["vmess_entries"].(int); vmess != 1 {
		t.Fatalf("unexpected vmess_entries: %d", vmess)
	}
	if gatewayValid := summary["gateway_valid"].(bool); !gatewayValid {
		t.Fatal("expected gateway_valid to be true")
	}
	items, ok := summary["items"].([]map[string]any)
	if !ok || len(items) != 3 {
		t.Fatalf("unexpected items payload: %#v", summary["items"])
	}
	if raw := strings.TrimSpace(items[2]["raw"].(string)); raw != "bad://proxy" {
		t.Fatalf("unexpected raw proxy entry: %q", raw)
	}
}

func TestResolveDirectProxyCandidateFromGateway(t *testing.T) {
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
	if candidate.Mode != "socks5" {
		t.Fatalf("unexpected gateway mode: %q", candidate.Mode)
	}
	if candidate.Server != "socks5://127.0.0.1:7890" {
		t.Fatalf("unexpected gateway server: %q", candidate.Server)
	}
}
