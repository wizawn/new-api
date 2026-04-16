package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type AutomationSettings struct {
	Mode                 string `json:"mode"`
	WorkerBaseURL        string `json:"worker_base_url"`
	WorkerUsername       string `json:"worker_username"`
	WorkerPassword       string `json:"-"`
	WorkerTimeoutSeconds int    `json:"worker_timeout_seconds"`
	WorkerVerifySSL      bool   `json:"worker_verify_ssl"`
	ArtifactRoot         string `json:"artifact_root"`
	PlaywrightHeadless   bool   `json:"playwright_headless"`
	PlaywrightBrowser    string `json:"playwright_browser"`
	RegisterEntryURL     string `json:"register_entry_url"`
	RegisterProxyPool    string `json:"register_proxy_pool"`
	RegisterProxyGateway string `json:"register_proxy_gateway"`
	RegisterUserAgent    string `json:"register_user_agent"`
	CloudflareAPIToken   string `json:"-"`
	CloudflareAPIEmail   string `json:"cloudflare_api_email"`
	CloudflareZoneID     string `json:"cloudflare_zone_id"`
	CloudflareDomain     string `json:"cloudflare_domain"`
	TempMailBaseURL      string `json:"tempmail_base_url"`
	TempMailAdminAuth    string `json:"-"`
	TempMailDomain       string `json:"tempmail_domain"`
	DefaultMailboxID     int    `json:"default_mailbox_id"`
	CPAEndpoint          string `json:"cpa_endpoint"`
	CPAToken             string `json:"-"`
}

type AutomationModuleField struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Type        string            `json:"type"`
	Required    bool              `json:"required"`
	Placeholder string            `json:"placeholder,omitempty"`
	Description string            `json:"description,omitempty"`
	Default     any               `json:"default,omitempty"`
	Options     []map[string]any  `json:"options,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}

type AutomationModuleDefinition struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	Fields            []AutomationModuleField `json:"fields"`
	SupportedStatuses []string                `json:"supported_statuses"`
	BulkSupported     bool                    `json:"bulk_supported"`
}

func parseBoolOption(raw string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseIntOption(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func normalizeAutomationMode(mode string) string {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	switch normalized {
	case constant.AutomationModeLocal, constant.AutomationModeRemote, constant.AutomationModeDisabled:
		return normalized
	default:
		return constant.AutomationModeLocal
	}
}

func resolveAutomationModeFromLegacy() string {
	enabled := parseBoolOption(common.OptionMap["TeamAutomationEnabled"], false)
	legacyMode := strings.TrimSpace(strings.ToLower(common.OptionMap["TeamAutomationMode"]))
	if !enabled && legacyMode == "" {
		return constant.AutomationModeLocal
	}
	if !enabled {
		return constant.AutomationModeDisabled
	}
	switch legacyMode {
	case "remote_proxy":
		return constant.AutomationModeRemote
	case "local_playwright":
		return constant.AutomationModeLocal
	case "disabled":
		return constant.AutomationModeDisabled
	default:
		return constant.AutomationModeRemote
	}
}

func LoadAutomationSettings() AutomationSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	mode := normalizeAutomationMode(common.OptionMap["AutomationMode"])
	if strings.TrimSpace(common.OptionMap["AutomationMode"]) == "" {
		mode = resolveAutomationModeFromLegacy()
	}

	settings := AutomationSettings{
		Mode:                 mode,
		WorkerBaseURL:        strings.TrimRight(strings.TrimSpace(common.OptionMap["AutomationWorkerBaseURL"]), "/"),
		WorkerUsername:       strings.TrimSpace(common.OptionMap["AutomationWorkerUsername"]),
		WorkerPassword:       strings.TrimSpace(common.OptionMap["AutomationWorkerPasswordSecret"]),
		WorkerTimeoutSeconds: parseIntOption(common.OptionMap["AutomationWorkerTimeoutSeconds"], 30),
		WorkerVerifySSL:      parseBoolOption(common.OptionMap["AutomationWorkerVerifySSLEnabled"], true),
		ArtifactRoot:         strings.TrimSpace(common.OptionMap["AutomationArtifactRoot"]),
		PlaywrightHeadless:   parseBoolOption(common.OptionMap["AutomationPlaywrightHeadless"], true),
		PlaywrightBrowser:    strings.TrimSpace(strings.ToLower(common.OptionMap["AutomationPlaywrightBrowser"])),
		RegisterEntryURL:     strings.TrimSpace(common.OptionMap["AutomationRegisterEntryURL"]),
		RegisterProxyPool:    strings.TrimSpace(common.OptionMap["AutomationRegisterProxyPool"]),
		RegisterProxyGateway: strings.TrimSpace(common.OptionMap["AutomationRegisterProxyGateway"]),
		RegisterUserAgent:    strings.TrimSpace(common.OptionMap["AutomationRegisterUserAgent"]),
		CloudflareAPIToken:   strings.TrimSpace(common.OptionMap["AutomationCloudflareApiToken"]),
		CloudflareAPIEmail:   strings.TrimSpace(common.OptionMap["AutomationCloudflareApiEmail"]),
		CloudflareZoneID:     strings.TrimSpace(common.OptionMap["AutomationCloudflareZoneID"]),
		CloudflareDomain:     strings.TrimSpace(strings.ToLower(common.OptionMap["AutomationCloudflareDomain"])),
		TempMailBaseURL:      strings.TrimRight(strings.TrimSpace(common.OptionMap["AutomationTempmailBaseURL"]), "/"),
		TempMailAdminAuth:    strings.TrimSpace(common.OptionMap["AutomationTempmailAdminAuth"]),
		TempMailDomain:       strings.TrimSpace(strings.ToLower(common.OptionMap["AutomationTempmailDomain"])),
		DefaultMailboxID:     parseIntOption(common.OptionMap["AutomationDefaultMailboxID"], 0),
		CPAEndpoint:          strings.TrimRight(strings.TrimSpace(common.OptionMap["AutomationCPAEndpoint"]), "/"),
		CPAToken:             strings.TrimSpace(common.OptionMap["AutomationCPAToken"]),
	}

	if settings.WorkerBaseURL == "" {
		settings.WorkerBaseURL = strings.TrimRight(strings.TrimSpace(common.OptionMap["TeamAutomationBaseURL"]), "/")
	}
	if settings.WorkerUsername == "" {
		settings.WorkerUsername = strings.TrimSpace(common.OptionMap["TeamAutomationUsername"])
	}
	if settings.WorkerPassword == "" {
		settings.WorkerPassword = strings.TrimSpace(common.OptionMap["TeamAutomationPasswordSecret"])
	}
	if settings.WorkerTimeoutSeconds <= 0 {
		settings.WorkerTimeoutSeconds = parseIntOption(common.OptionMap["TeamAutomationTimeoutSeconds"], 30)
	}
	if strings.TrimSpace(common.OptionMap["AutomationWorkerVerifySSLEnabled"]) == "" {
		settings.WorkerVerifySSL = parseBoolOption(common.OptionMap["TeamAutomationVerifySSLEnabled"], true)
	}
	if settings.ArtifactRoot == "" {
		settings.ArtifactRoot = "/data/automation-artifacts"
	}
	if settings.PlaywrightBrowser == "" {
		settings.PlaywrightBrowser = "chromium"
	}
	if strings.TrimSpace(settings.RegisterEntryURL) == "" {
		settings.RegisterEntryURL = "https://chatgpt.com"
	}
	if settings.WorkerTimeoutSeconds < 5 || settings.WorkerTimeoutSeconds > 300 {
		settings.WorkerTimeoutSeconds = 30
	}
	return settings
}

func (s AutomationSettings) ToRemoteTeamAutomationConfig() TeamAutomationConfig {
	return TeamAutomationConfig{
		Enabled:        s.Mode != constant.AutomationModeDisabled,
		Mode:           TeamAutomationModeRemoteProxy,
		BaseURL:        s.WorkerBaseURL,
		Username:       s.WorkerUsername,
		Password:       s.WorkerPassword,
		TimeoutSeconds: s.WorkerTimeoutSeconds,
		VerifySSL:      s.WorkerVerifySSL,
	}
}

func (s AutomationSettings) Sanitized() map[string]any {
	configured := false
	reachable := false
	enabled := s.Mode != constant.AutomationModeDisabled
	warning := ""

	switch s.Mode {
	case constant.AutomationModeLocal:
		configured = strings.TrimSpace(s.ArtifactRoot) != ""
		reachable = configured
	case constant.AutomationModeRemote:
		configured = strings.TrimSpace(s.WorkerBaseURL) != ""
		reachable = configured
	default:
		configured = false
		reachable = false
	}

	if enabled && !configured {
		warning = "automation mode is enabled but not fully configured"
	}

	return map[string]any{
		"mode":                        s.Mode,
		"enabled":                     enabled,
		"configured":                  configured,
		"reachable":                   reachable,
		"warning":                     warning,
		"worker_base_url":             s.WorkerBaseURL,
		"worker_username":             s.WorkerUsername,
		"worker_timeout_seconds":      s.WorkerTimeoutSeconds,
		"worker_verify_ssl":           s.WorkerVerifySSL,
		"artifact_root":               s.ArtifactRoot,
		"playwright_headless":         s.PlaywrightHeadless,
		"playwright_browser":          s.PlaywrightBrowser,
		"register_entry_url":          s.RegisterEntryURL,
		"register_proxy_pool":         s.RegisterProxyPool,
		"register_proxy_gateway":      s.RegisterProxyGateway,
		"register_user_agent":         s.RegisterUserAgent,
		"cloudflare_api_email":        s.CloudflareAPIEmail,
		"cloudflare_zone_id":          s.CloudflareZoneID,
		"cloudflare_domain":           s.CloudflareDomain,
		"cloudflare_configured":       s.CloudflareAPIToken != "" && (s.CloudflareZoneID != "" || s.CloudflareDomain != ""),
		"tempmail_base_url":           s.TempMailBaseURL,
		"tempmail_domain":             s.TempMailDomain,
		"tempmail_configured":         s.TempMailBaseURL != "" && s.TempMailAdminAuth != "",
		"default_mailbox_id":          s.DefaultMailboxID,
		"cpa_endpoint":                s.CPAEndpoint,
		"cpa_configured":              s.CPAEndpoint != "" && s.CPAToken != "",
		"worker_auth_configured":      s.WorkerUsername != "" && s.WorkerPassword != "",
		"cloudflare_token_configured": s.CloudflareAPIToken != "",
	}
}

func (s AutomationSettings) Validate() error {
	s.Mode = normalizeAutomationMode(s.Mode)
	if s.WorkerTimeoutSeconds < 5 || s.WorkerTimeoutSeconds > 300 {
		return fmt.Errorf("automation worker timeout seconds must be between 5 and 300")
	}
	s.ArtifactRoot = strings.TrimSpace(s.ArtifactRoot)
	if s.Mode == constant.AutomationModeLocal {
		if s.ArtifactRoot == "" {
			return fmt.Errorf("artifact root is required in local mode")
		}
	}
	if s.Mode == constant.AutomationModeRemote && strings.TrimSpace(s.WorkerBaseURL) == "" {
		return fmt.Errorf("worker base url is required in remote mode")
	}
	return nil
}

func (s AutomationSettings) EnsureArtifactRoot() error {
	if normalizeAutomationMode(s.Mode) != constant.AutomationModeLocal {
		return nil
	}
	root := strings.TrimSpace(s.ArtifactRoot)
	if root == "" {
		return fmt.Errorf("artifact root is empty")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return err
	}
	probe := filepath.Join(root, ".automation-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return err
	}
	_ = os.Remove(probe)
	return nil
}

func GetAutomationModuleDefinitions() []AutomationModuleDefinition {
	statuses := []string{
		constant.AutomationAccountStatusPending,
		constant.AutomationAccountStatusGenerated,
		constant.AutomationAccountStatusWaitingInvite,
		constant.AutomationAccountStatusInviteReceived,
		constant.AutomationAccountStatusJoinedTeam,
		constant.AutomationAccountStatusLoginCaptured,
		constant.AutomationAccountStatusConverted,
		constant.AutomationAccountStatusImported,
		constant.AutomationAccountStatusCompleted,
		constant.AutomationAccountStatusManualReview,
		constant.AutomationAccountStatusFailed,
	}

	proxyControlFields := []AutomationModuleField{
		{Key: "proxy_rotate_mode", Label: "代理轮换策略", Type: "select", Required: false, Default: "stable", Options: []map[string]any{{"label": "稳定映射", "value": "stable"}, {"label": "随机选择", "value": "random"}}, Description: "stable 会尽量让同一账号稳定命中同一节点；random 每次随机选节点。"},
		{Key: "proxy_retry_count", Label: "代理失败重试次数", Type: "number", Required: false, Default: 1, Description: "命中 Cloudflare、add-phone、超时或代理切换失败时，自动换下一个节点重试；建议 0-2。"},
		{Key: "proxy_pool", Label: "运行期代理池覆盖", Type: "textarea", Required: false, Placeholder: "每行一个代理，支持 socks5/http/https/vmess://...", Description: "留空则使用设置页中的 Register Proxy Pool；适合临时覆盖本次运行的代理池。"},
		{Key: "proxy_gateway", Label: "外部代理网关", Type: "text", Required: false, Placeholder: "socks5://cpa-host:7890", Description: "适合 CPA 自主管理节点池场景；业务侧始终连接固定网关，由 CPA 在内部切换真实出口。"},
		{Key: "proxy_switch_url", Label: "外部换节点接口", Type: "text", Required: false, Placeholder: "http://cpa-host:9000/api/proxy/switch", Description: "当选中 vmess 节点且由外部控制器接管时，先调用此接口切换节点，再走 proxy_gateway。"},
		{Key: "proxy_switch_token", Label: "换节点接口 Token", Type: "password", Required: false, Placeholder: "可选 Bearer Token"},
		{Key: "disable_proxy", Label: "禁用代理", Type: "boolean", Required: false, Default: false, Description: "开启后本次模块执行将直接使用本地网络，不读取代理池。"},
	}

	return []AutomationModuleDefinition{
		{
			ID:          constant.AutomationModuleCloudflareEmailGenerate,
			Name:        "邮箱生成（Cloudflare / TempMail）",
			Description: "支持 Cloudflare Email Routing 或 TempMail / Wizawn worker 生成账号邮箱，并自动绑定到后续自动化流程。",
			Fields: []AutomationModuleField{
				{Key: "provider", Label: "生成方式", Type: "select", Required: true, Default: "cloudflare", Options: []map[string]any{{"label": "Cloudflare Email Routing", "value": "cloudflare"}, {"label": "TempMail / Wizawn", "value": "tempmail_wizawn"}}},
				{Key: "domain", Label: "邮箱域名", Type: "text", Required: false, Placeholder: "Cloudflare 读取 AutomationCloudflareDomain；TempMail 读取 AutomationTempmailDomain"},
				{Key: "alias_strategy", Label: "别名策略", Type: "select", Required: true, Default: "skip", Options: []map[string]any{{"label": "跳过已存在", "value": "skip"}, {"label": "覆盖同名", "value": "overwrite"}, {"label": "追加后缀", "value": "append"}}},
				{Key: "forward_target", Label: "转发目标邮箱", Type: "text", Required: false, Placeholder: "默认使用账号邮箱"},
				{Key: "enable_random_subdomain", Label: "随机子域名", Type: "boolean", Required: false, Default: false, Description: "仅 TempMail 模式生效，需 worker 允许 RANDOM_SUBDOMAIN_DOMAINS"},
				{Key: "mailbox_poll_interval_second", Label: "邮箱轮询间隔(秒)", Type: "number", Required: false, Default: 60, Description: "仅 TempMail 模式生效，用于自动创建的邮箱中心记录"},
			},
			SupportedStatuses: statuses,
			BulkSupported:     true,
		},
		{
			ID:          constant.AutomationModuleTeamRedeem,
			Name:        "Team 兑换",
			Description: "执行兑换码提交流程，支持 HTTP 直连或浏览器自动化。",
			Fields: append([]AutomationModuleField{
				{Key: "redeem_site", Label: "兑换站点", Type: "text", Required: true, Placeholder: "https://helloteam.store"},
				{Key: "redeem_mode", Label: "执行方式", Type: "select", Required: true, Default: "browser", Options: []map[string]any{{"label": "浏览器自动化", "value": "browser"}, {"label": "HTTP 提交", "value": "http"}}},
				{Key: "code_selector", Label: "兑换码选择器", Type: "text", Required: false, Placeholder: "input[name='code']"},
				{Key: "email_selector", Label: "邮箱选择器", Type: "text", Required: false, Placeholder: "input[type='email']"},
				{Key: "submit_selector", Label: "提交按钮选择器", Type: "text", Required: false, Placeholder: "button[type='submit']"},
				{Key: "success_selector", Label: "成功态选择器", Type: "text", Required: false, Placeholder: ".success"},
			}, proxyControlFields...),
			SupportedStatuses: statuses,
			BulkSupported:     true,
		},
		{
			ID:          constant.AutomationModuleGmailInviteWatch,
			Name:        "Gmail 邀请监听",
			Description: "通过 IMAP 拉取邮件并提取邀请链接/验证码。",
			Fields: []AutomationModuleField{
				{Key: "mailbox_id", Label: "邮箱中心 ID", Type: "number", Required: false, Placeholder: "默认读取 AutomationDefaultMailboxID"},
				{Key: "classification", Label: "事件分类", Type: "select", Required: false, Default: constant.ChatMockMailEventTypeInviteLink, Options: []map[string]any{
					{"label": "任意事件", "value": ""},
					{"label": "invite_link", "value": constant.ChatMockMailEventTypeInviteLink},
					{"label": "verification_code", "value": constant.ChatMockMailEventTypeVerificationCode},
					{"label": "auth_alert", "value": constant.ChatMockMailEventTypeAuthAlert},
					{"label": "billing_alert", "value": constant.ChatMockMailEventTypeBillingAlert},
					{"label": "system_notice", "value": constant.ChatMockMailEventTypeSystemNotice},
				}},
				{Key: "sender_keyword", Label: "发件人关键字", Type: "text", Required: false, Placeholder: "team / openai"},
				{Key: "subject_keyword", Label: "主题关键字", Type: "text", Required: false, Placeholder: "invite / 邀请"},
				{Key: "keyword", Label: "内容关键字", Type: "text", Required: false, Placeholder: "同时匹配主题/发件人/正文摘要"},
				{Key: "require_invite_link", Label: "必须提取邀请链接", Type: "boolean", Required: false, Default: true},
				{Key: "max_event_age_seconds", Label: "允许事件最大时效(秒)", Type: "number", Required: false, Default: 1200},
				{Key: "wait_timeout_seconds", Label: "等待超时(秒)", Type: "number", Required: false, Default: 120},
				{Key: "poll_interval_seconds", Label: "轮询间隔(秒)", Type: "number", Required: false, Default: 15},
			},
			SupportedStatuses: statuses,
			BulkSupported:     true,
		},
		{
			ID:          constant.AutomationModuleBrowserLoginCapture,
			Name:        "浏览器登录 / 会话抓取",
			Description: "本机 Playwright 执行登录并保存 storageState/cookies。",
			Fields: append([]AutomationModuleField{
				{Key: "login_url", Label: "登录地址", Type: "text", Required: true, Placeholder: "https://chatgpt.com/auth/login"},
				{Key: "username_selector", Label: "账号输入框选择器", Type: "text", Required: false, Placeholder: "input[type='email']"},
				{Key: "password_selector", Label: "密码输入框选择器", Type: "text", Required: false, Placeholder: "input[type='password']"},
				{Key: "submit_selector", Label: "提交按钮选择器", Type: "text", Required: false, Placeholder: "button[type='submit']"},
				{Key: "success_selector", Label: "成功态选择器", Type: "text", Required: false, Placeholder: "[data-testid='profile-button']"},
				{Key: "headless", Label: "无头模式", Type: "boolean", Required: false, Default: true},
				{Key: "timeout_seconds", Label: "超时(秒)", Type: "number", Required: false, Default: 90},
			}, proxyControlFields...),
			SupportedStatuses: statuses,
			BulkSupported:     true,
		},
		{
			ID:          constant.AutomationModuleImportTarget,
			Name:        "导入 NewAPI / CPA",
			Description: "将转换产物导入当前 NewAPI 账号池或外部 CPA 服务。",
			Fields: []AutomationModuleField{
				{Key: "target", Label: "导入目标", Type: "select", Required: true, Default: "newapi", Options: []map[string]any{{"label": "仅 NewAPI", "value": "newapi"}, {"label": "仅 CPA", "value": "cpa"}, {"label": "NewAPI + CPA", "value": "both"}}},
				{Key: "newapi_channel_group", Label: "NewAPI 组", Type: "text", Required: false, Placeholder: "default"},
				{Key: "cpa_path", Label: "CPA 导入路径", Type: "text", Required: false, Placeholder: "CLIProxyAPI: /v0/management/auth-files；传统导入器: /api/accounts/import"},
				{Key: "api_key", Label: "导入 API Key(可选)", Type: "text", Required: false, Placeholder: "缺省时尝试从 metadata/api_key 读取"},
			},
			SupportedStatuses: statuses,
			BulkSupported:     true,
		},
	}
}

func TestAutomationConnectivity(ctx context.Context, settings AutomationSettings, testType string) (map[string]any, error) {
	settings.Mode = normalizeAutomationMode(settings.Mode)
	typeName := strings.TrimSpace(strings.ToLower(testType))
	if typeName == "" {
		typeName = "engine"
	}
	if ctx == nil {
		ctx = context.Background()
	}

	switch typeName {
	case "engine", "worker", "local_worker", "remote_worker":
		result, err := TestAutomationSettings(settings)
		if err != nil {
			return nil, err
		}
		result["test_type"] = typeName
		return result, nil
	case "cloudflare":
		if strings.TrimSpace(settings.CloudflareAPIToken) == "" {
			return nil, fmt.Errorf("cloudflare token 配置缺失")
		}
		if strings.TrimSpace(settings.CloudflareZoneID) == "" && strings.TrimSpace(settings.CloudflareDomain) == "" {
			return nil, fmt.Errorf("cloudflare zone/domain 配置缺失")
		}
		response, err := cloudflareRequest(ctx, settings, http.MethodGet, "/email/routing/rules?per_page=1", nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"test_type": "cloudflare",
			"ok":        true,
			"response":  response,
		}, nil
	case "gmail", "mailbox", "imap":
		if settings.DefaultMailboxID <= 0 {
			return nil, fmt.Errorf("default_mailbox_id 未配置")
		}
		mailbox, err := model.GetChatMockMailboxByID(settings.DefaultMailboxID)
		if err != nil {
			return nil, err
		}
		timeout := settings.WorkerTimeoutSeconds
		if timeout <= 0 {
			timeout = 30
		}
		if err := ValidateChatMockMailboxConnectivity(mailbox, timeout); err != nil {
			return nil, err
		}
		return map[string]any{
			"test_type":         "gmail",
			"ok":                true,
			"mailbox_id":        mailbox.Id,
			"mailbox_name":      mailbox.Name,
			"imap_host":         mailbox.ImapHost,
			"smtp_host_present": strings.TrimSpace(mailbox.SmtpHost) != "",
		}, nil
	case "cpa", "importer":
		return testCPAConnectivity(ctx, settings)
	case "newapi":
		if model.DB == nil {
			return nil, fmt.Errorf("database is not initialized")
		}
		sqlDB, err := model.DB.DB()
		if err != nil {
			return nil, err
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			return nil, err
		}
		return map[string]any{
			"test_type": "newapi",
			"ok":        true,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported test_type: %s", typeName)
	}
}

func buildCPAProbeCandidates(endpoint string) []string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return nil
	}
	add := func(items []string, value string) []string {
		value = strings.TrimSpace(value)
		if value == "" {
			return items
		}
		for _, existing := range items {
			if existing == value {
				return items
			}
		}
		return append(items, value)
	}

	items := []string{}
	path := ""
	if parsed, err := url.Parse(endpoint); err == nil {
		path = strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	}

	if strings.Contains(path, "/v0/management") {
		items = add(items, endpoint+"/config")
		items = add(items, endpoint+"/api-keys")
		items = add(items, endpoint+"/auth-files")
		return items
	}

	items = add(items, endpoint+"/health")
	items = add(items, endpoint+"/v0/management/config")
	items = add(items, endpoint+"/v0/management/api-keys")
	items = add(items, endpoint+"/v0/management/auth-files")
	return items
}

func testCPAConnectivity(ctx context.Context, settings AutomationSettings) (map[string]any, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(settings.CPAEndpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("AutomationCPAEndpoint 未配置")
	}

	attempts := make([]map[string]any, 0, 4)
	client := &http.Client{Timeout: 15 * time.Second}
	token := strings.TrimSpace(settings.CPAToken)
	candidates := buildCPAProbeCandidates(endpoint)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("无法构造 CPA 探测地址")
	}

	lastErr := ""
	for _, probeURL := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			lastErr = err.Error()
			attempts = append(attempts, map[string]any{
				"url":   probeURL,
				"ok":    false,
				"error": err.Error(),
			})
			continue
		}
		req.Header.Set("Accept", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Management-Key", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err.Error()
			attempts = append(attempts, map[string]any{
				"url":   probeURL,
				"ok":    false,
				"error": err.Error(),
			})
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		trimmedBody := strings.TrimSpace(string(body))

		if resp.StatusCode < 400 {
			mode := "generic"
			if strings.Contains(strings.ToLower(probeURL), "/v0/management/") {
				mode = "cliproxy_management"
			}
			attempts = append(attempts, map[string]any{
				"url":         probeURL,
				"ok":          true,
				"status_code": resp.StatusCode,
			})
			return map[string]any{
				"test_type":     "cpa",
				"ok":            true,
				"endpoint":      endpoint,
				"matched_probe": probeURL,
				"mode":          mode,
				"status_code":   resp.StatusCode,
				"response":      trimmedBody,
				"attempts":      attempts,
			}, nil
		}

		lastErr = fmt.Sprintf("%d: %s", resp.StatusCode, trimmedBody)
		attempts = append(attempts, map[string]any{
			"url":         probeURL,
			"ok":          false,
			"status_code": resp.StatusCode,
			"response":    trimmedBody,
		})
	}

	if lastErr == "" {
		lastErr = "unknown error"
	}
	return nil, fmt.Errorf("CPA 连通性测试失败（%d 个探测端点均不可用）：%s", len(attempts), lastErr)
}
