package service

import (
	"os/exec"
	"strconv"
	"strings"
)

func automationXrayBinary() string {
	path, err := exec.LookPath("xray")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(path)
}

func proxyDisplayLabel(candidate *automationProxyCandidate, index int) string {
	if candidate == nil {
		return "entry_" + strconv.Itoa(index)
	}
	switch candidate.Mode {
	case "vmess", "vmess_xray":
		if strings.TrimSpace(candidate.NodeName) != "" {
			return strings.TrimSpace(candidate.NodeName)
		}
		return "vmess"
	default:
		if strings.TrimSpace(candidate.Server) != "" {
			return strings.TrimSpace(candidate.Server)
		}
		return "entry"
	}
}

func BuildAutomationProxyPoolSummary(pool string, gateway string) map[string]any {
	items := splitProxyPool(pool)
	xrayPath := automationXrayBinary()
	summary := map[string]any{
		"total_entries":      len(items),
		"valid_entries":      0,
		"invalid_entries":    0,
		"vmess_entries":      0,
		"native_entries":     0,
		"xray_available":     xrayPath != "",
		"xray_path":          xrayPath,
		"gateway_configured": strings.TrimSpace(gateway) != "",
		"gateway_valid":      false,
		"gateway_error":      "",
		"items":              []map[string]any{},
	}

	rowItems := make([]map[string]any, 0, len(items))
	for idx, raw := range items {
		row := map[string]any{
			"index": idx + 1,
			"valid": false,
			"raw":   strings.TrimSpace(raw),
		}
		candidate, err := parseProxyCandidate(raw)
		if err != nil {
			row["error"] = err.Error()
			summary["invalid_entries"] = summary["invalid_entries"].(int) + 1
			rowItems = append(rowItems, row)
			continue
		}
		row["valid"] = true
		row["mode"] = candidate.Mode
		row["label"] = proxyDisplayLabel(candidate, idx+1)
		if candidate.Mode == "vmess" {
			row["requires_xray"] = true
			summary["vmess_entries"] = summary["vmess_entries"].(int) + 1
		} else {
			summary["native_entries"] = summary["native_entries"].(int) + 1
		}
		summary["valid_entries"] = summary["valid_entries"].(int) + 1
		rowItems = append(rowItems, row)
	}
	summary["items"] = rowItems

	gateway = strings.TrimSpace(gateway)
	if gateway != "" {
		candidate, err := parseProxyCandidate(gateway)
		if err != nil {
			summary["gateway_error"] = err.Error()
		} else {
			summary["gateway_valid"] = true
			summary["gateway_mode"] = candidate.Mode
			summary["gateway_label"] = proxyDisplayLabel(candidate, 0)
		}
	}

	vmessEntries := summary["vmess_entries"].(int)
	xrayAvailable := summary["xray_available"].(bool)
	gatewayValid := summary["gateway_valid"].(bool)
	summary["vmess_ready"] = vmessEntries == 0 || xrayAvailable || gatewayValid
	if vmessEntries > 0 && !xrayAvailable && !gatewayValid {
		summary["warning"] = "vmess 节点存在，但当前既没有 xray 也没有可用 proxy_gateway"
	}
	return summary
}
