package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestAutomationModuleDefinitionsIncludeGmailClassificationField(t *testing.T) {
	defs := GetAutomationModuleDefinitions()
	var gmailDef *AutomationModuleDefinition
	for i := range defs {
		if defs[i].ID == constant.AutomationModuleGmailInviteWatch {
			gmailDef = &defs[i]
			break
		}
	}
	if gmailDef == nil {
		t.Fatalf("gmail module definition not found")
	}

	fieldKeys := make(map[string]struct{}, len(gmailDef.Fields))
	for _, field := range gmailDef.Fields {
		fieldKeys[field.Key] = struct{}{}
	}
	requiredKeys := []string{
		"classification",
		"keyword",
		"require_invite_link",
		"max_event_age_seconds",
	}
	for _, key := range requiredKeys {
		if _, ok := fieldKeys[key]; !ok {
			t.Fatalf("gmail module missing field %q", key)
		}
	}
}

func TestAutomationModuleDefinitionsExcludeDeprecatedModules(t *testing.T) {
	defs := GetAutomationModuleDefinitions()
	for _, def := range defs {
		if def.ID == constant.AutomationModuleAutoRegister {
			t.Fatalf("auto register module should not be exposed")
		}
		if def.ID == constant.AutomationModuleSessionConvert {
			t.Fatalf("session convert module should not be exposed")
		}
	}
}

func TestAutomationModuleDefinitionsIncludeEmailGenerateProvider(t *testing.T) {
	defs := GetAutomationModuleDefinitions()
	var emailDef *AutomationModuleDefinition
	for i := range defs {
		if defs[i].ID == constant.AutomationModuleCloudflareEmailGenerate {
			emailDef = &defs[i]
			break
		}
	}
	if emailDef == nil {
		t.Fatalf("email generate module definition not found")
	}

	fieldKeys := make(map[string]struct{}, len(emailDef.Fields))
	for _, field := range emailDef.Fields {
		fieldKeys[field.Key] = struct{}{}
	}
	requiredKeys := []string{
		"provider",
		"domain",
		"enable_random_subdomain",
		"mailbox_poll_interval_second",
	}
	for _, key := range requiredKeys {
		if _, ok := fieldKeys[key]; !ok {
			t.Fatalf("email generate module missing field %q", key)
		}
	}
}

func TestAutomationModuleDefinitionsIncludeProxyControlFieldsForBrowserModules(t *testing.T) {
	defs := GetAutomationModuleDefinitions()
	targetModules := map[string][]string{
		constant.AutomationModuleTeamRedeem: {
			"proxy_rotate_mode",
			"proxy_retry_count",
			"proxy_pool",
			"proxy_gateway",
			"proxy_switch_url",
			"proxy_switch_token",
			"disable_proxy",
		},
		constant.AutomationModuleBrowserLoginCapture: {
			"proxy_rotate_mode",
			"proxy_retry_count",
			"proxy_pool",
			"proxy_gateway",
			"proxy_switch_url",
			"proxy_switch_token",
			"disable_proxy",
		},
	}

	for moduleID, requiredKeys := range targetModules {
		var moduleDef *AutomationModuleDefinition
		for i := range defs {
			if defs[i].ID == moduleID {
				moduleDef = &defs[i]
				break
			}
		}
		if moduleDef == nil {
			t.Fatalf("module definition not found: %s", moduleID)
		}

		fieldKeys := make(map[string]struct{}, len(moduleDef.Fields))
		for _, field := range moduleDef.Fields {
			fieldKeys[field.Key] = struct{}{}
		}
		for _, key := range requiredKeys {
			if _, ok := fieldKeys[key]; !ok {
				t.Fatalf("module %s missing proxy control field %q", moduleID, key)
			}
		}
	}
}
