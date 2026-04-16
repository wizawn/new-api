package service

import (
	"context"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

const sampleVMessNode = "vmess://ew0KICAidiI6ICIyIiwNCiAgInBzIjogIuaXpeacrC3kvJjljJYiLA0KICAiYWRkIjogInBsYW5iLm1vamNuLmNvbSIsDQogICJwb3J0IjogIjE2NjE3IiwNCiAgImlkIjogIjMxYWNiZmIyLTU3NDAtNDVkNS1iZTJjLTA3OWY1MGUyMTU4ZiIsDQogICJhaWQiOiAiMCIsDQogICJzY3kiOiAiYXV0byIsDQogICJuZXQiOiAid3MiLA0KICAidHlwZSI6ICJub25lIiwNCiAgImhvc3QiOiAiMWI3OWFhYWMzNDZmMDlmOTFjM2IzMGE4NDc2YWYyMDUyLm1vYmdzbGIudGJjYWNoZS5jb20iLA0KICAicGF0aCI6ICIvIiwNCiAgInRscyI6ICIiLA0KICAic25pIjogIiIsDQogICJhbHBuIjogIiINCn0="

func TestDecodeVMessNode(t *testing.T) {
	node, err := decodeVMessNode(sampleVMessNode)
	if err != nil {
		t.Fatalf("decodeVMessNode failed: %v", err)
	}
	if node.Address != "planb.mojcn.com" {
		t.Fatalf("unexpected address: %q", node.Address)
	}
	if node.Port != "16617" {
		t.Fatalf("unexpected port: %q", node.Port)
	}
	if node.Network != "ws" {
		t.Fatalf("unexpected network: %q", node.Network)
	}
	if node.Path != "/" {
		t.Fatalf("unexpected path: %q", node.Path)
	}
	if node.displayName() == "" {
		t.Fatal("displayName should not be empty")
	}
}

func TestParseProxyCandidateVMess(t *testing.T) {
	candidate, err := parseProxyCandidate(sampleVMessNode)
	if err != nil {
		t.Fatalf("parseProxyCandidate failed: %v", err)
	}
	if candidate.Mode != "vmess" {
		t.Fatalf("unexpected mode: %q", candidate.Mode)
	}
	if candidate.VMessNode == nil {
		t.Fatal("expected parsed vmess node")
	}
}

func TestStartVMessProxyRuntime(t *testing.T) {
	if _, err := exec.LookPath("xray"); err != nil {
		t.Skip("xray not found in PATH")
	}
	candidate, err := parseProxyCandidate(sampleVMessNode)
	if err != nil {
		t.Fatalf("parseProxyCandidate failed: %v", err)
	}
	launchProxy, err := startVMessProxyRuntime(context.Background(), candidate)
	if err != nil {
		t.Fatalf("startVMessProxyRuntime failed: %v", err)
	}
	defer launchProxy.Cleanup()
	if launchProxy.Mode != "vmess_xray" {
		t.Fatalf("unexpected launch mode: %q", launchProxy.Mode)
	}
	if !strings.HasPrefix(launchProxy.Server, "socks5://127.0.0.1:") {
		t.Fatalf("unexpected launch server: %q", launchProxy.Server)
	}
	if _, err := url.Parse(launchProxy.Server); err != nil {
		t.Fatalf("invalid proxy server url: %v", err)
	}
}
