package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type automationVMessNode struct {
	Version  string `json:"v"`
	Name     string `json:"ps"`
	Address  string `json:"add"`
	Port     string `json:"port"`
	ID       string `json:"id"`
	AlterID  string `json:"aid"`
	Security string `json:"scy"`
	Network  string `json:"net"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	TLS      string `json:"tls"`
	SNI      string `json:"sni"`
	ALPN     string `json:"alpn"`
}

func decodeVMessNode(raw string) (*automationVMessNode, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(value), "vmess://") {
		return nil, fmt.Errorf("not a vmess uri")
	}
	encoded := strings.TrimSpace(value[len("vmess://"):])
	if encoded == "" {
		return nil, fmt.Errorf("empty vmess payload")
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		payload, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(encoded)
	}
	if err != nil {
		payload, err = base64.RawURLEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("decode vmess payload failed: %w", err)
	}
	node := &automationVMessNode{}
	if err := common.Unmarshal(payload, node); err != nil {
		return nil, fmt.Errorf("parse vmess payload failed: %w", err)
	}
	node.Name = strings.TrimSpace(node.Name)
	node.Address = strings.TrimSpace(node.Address)
	node.Port = strings.TrimSpace(node.Port)
	node.ID = strings.TrimSpace(node.ID)
	node.AlterID = strings.TrimSpace(node.AlterID)
	node.Security = strings.TrimSpace(strings.ToLower(node.Security))
	node.Network = strings.TrimSpace(strings.ToLower(node.Network))
	node.Type = strings.TrimSpace(strings.ToLower(node.Type))
	node.Host = strings.TrimSpace(node.Host)
	node.Path = strings.TrimSpace(node.Path)
	node.TLS = strings.TrimSpace(strings.ToLower(node.TLS))
	node.SNI = strings.TrimSpace(node.SNI)
	node.ALPN = strings.TrimSpace(node.ALPN)
	if node.Address == "" {
		return nil, fmt.Errorf("vmess address is empty")
	}
	if node.Port == "" {
		return nil, fmt.Errorf("vmess port is empty")
	}
	if node.ID == "" {
		return nil, fmt.Errorf("vmess id is empty")
	}
	if node.Security == "" {
		node.Security = "auto"
	}
	if node.Network == "" {
		node.Network = "tcp"
	}
	if node.Path == "" {
		node.Path = "/"
	}
	return node, nil
}

func (n *automationVMessNode) displayName() string {
	if n == nil {
		return "vmess"
	}
	if strings.TrimSpace(n.Name) != "" {
		return strings.TrimSpace(n.Name)
	}
	return fmt.Sprintf("%s:%s", strings.TrimSpace(n.Address), strings.TrimSpace(n.Port))
}

func findAvailableTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr == nil {
		return 0, fmt.Errorf("resolve tcp addr failed")
	}
	return addr.Port, nil
}

func buildVMessXrayConfig(node *automationVMessNode, socksPort int) map[string]any {
	user := map[string]any{
		"id":       node.ID,
		"security": node.Security,
	}
	if alterID, err := strconv.Atoi(strings.TrimSpace(node.AlterID)); err == nil && alterID >= 0 {
		user["alterId"] = alterID
	}
	outbound := map[string]any{
		"tag":      "proxy",
		"protocol": "vmess",
		"settings": map[string]any{
			"vnext": []map[string]any{
				{
					"address": node.Address,
					"port":    common.String2Int(node.Port),
					"users":   []map[string]any{user},
				},
			},
		},
	}
	streamSettings := map[string]any{
		"network": node.Network,
	}
	if node.Network == "ws" {
		headers := map[string]any{}
		if node.Host != "" {
			headers["Host"] = node.Host
		}
		wsSettings := map[string]any{
			"path": node.Path,
		}
		if len(headers) > 0 {
			wsSettings["headers"] = headers
		}
		streamSettings["wsSettings"] = wsSettings
	}
	if node.Network == "tcp" && node.Host != "" {
		streamSettings["tcpSettings"] = map[string]any{
			"header": map[string]any{
				"type": "http",
				"request": map[string]any{
					"path": []string{node.Path},
					"headers": map[string]any{
						"Host": []string{node.Host},
					},
				},
			},
		}
	}
	if node.TLS == "tls" || node.TLS == "reality" {
		streamSettings["security"] = "tls"
		tlsSettings := map[string]any{}
		if node.SNI != "" {
			tlsSettings["serverName"] = node.SNI
		} else if node.Host != "" {
			tlsSettings["serverName"] = node.Host
		}
		streamSettings["tlsSettings"] = tlsSettings
	}
	outbound["streamSettings"] = streamSettings
	return map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []map[string]any{
			{
				"tag":      "socks-in",
				"listen":   "127.0.0.1",
				"port":     socksPort,
				"protocol": "socks",
				"settings": map[string]any{
					"auth": "noauth",
					"udp":  true,
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls"},
				},
			},
		},
		"outbounds": []map[string]any{
			outbound,
			{
				"tag":      "direct",
				"protocol": "freedom",
			},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []map[string]any{
				{
					"type":        "field",
					"inboundTag":  []string{"socks-in"},
					"outboundTag": "proxy",
				},
			},
		},
	}
}

func startVMessProxyRuntime(ctx context.Context, candidate *automationProxyCandidate) (*automationProxyCandidate, error) {
	if candidate == nil || candidate.VMessNode == nil {
		return nil, fmt.Errorf("vmess node is nil")
	}
	xrayPath, err := exec.LookPath("xray")
	if err != nil {
		return nil, fmt.Errorf("xray not found in PATH")
	}
	socksPort, err := findAvailableTCPPort()
	if err != nil {
		return nil, err
	}
	workDir, err := os.MkdirTemp("", "automation-vmess-xray-*")
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(workDir, "config.json")
	configData, err := common.Marshal(buildVMessXrayConfig(candidate.VMessNode, socksPort))
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	cmd := exec.CommandContext(ctx, xrayPath, "run", "-c", configPath)
	cmd.Dir = workDir
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	stopFn := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		_ = os.RemoveAll(workDir)
	}
	targetAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	ready := false
	for i := 0; i < 20; i++ {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
		conn, dialErr := net.DialTimeout("tcp", targetAddr, 300*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		stopFn()
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = "xray startup timeout"
		}
		return nil, fmt.Errorf("vmess runtime start failed: %s", errMsg)
	}
	return &automationProxyCandidate{
		Original: candidate.Original,
		Server:   "socks5://127.0.0.1:" + strconv.Itoa(socksPort),
		Mode:     "vmess_xray",
		NodeName: candidate.NodeName,
		Cleanup:  stopFn,
	}, nil
}
