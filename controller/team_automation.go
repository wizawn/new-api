package controller

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const teamAutomationImportMaxBytes = 2 * 1024 * 1024

const (
	teamAutomationCustomRedeemSiteValue = "__custom__"
	teamAutomationSettingsTabPath       = "/console/setting?tab=team-automation"

	teamAutomationErrorDisabled             = "TEAM_AUTOMATION_DISABLED"
	teamAutomationErrorBaseURLMissing       = "TEAM_AUTOMATION_BASE_URL_MISSING"
	teamAutomationErrorConnectorUnavailable = "TEAM_AUTOMATION_CONNECTOR_UNAVAILABLE"
)

type automationSettingsRequest struct {
	Mode                 *string `json:"mode"`
	WorkerBaseURL        *string `json:"worker_base_url"`
	WorkerUsername       *string `json:"worker_username"`
	WorkerPassword       *string `json:"worker_password"`
	WorkerTimeoutSeconds *int    `json:"worker_timeout_seconds"`
	WorkerVerifySSL      *bool   `json:"worker_verify_ssl"`
	ArtifactRoot         *string `json:"artifact_root"`
	PlaywrightHeadless   *bool   `json:"playwright_headless"`
	PlaywrightBrowser    *string `json:"playwright_browser"`
	RegisterEntryURL     *string `json:"register_entry_url"`
	RegisterProxyPool    *string `json:"register_proxy_pool"`
	RegisterProxyGateway *string `json:"register_proxy_gateway"`
	RegisterUserAgent    *string `json:"register_user_agent"`
	CloudflareAPIToken   *string `json:"cloudflare_api_token"`
	CloudflareAPIEmail   *string `json:"cloudflare_api_email"`
	CloudflareZoneID     *string `json:"cloudflare_zone_id"`
	CloudflareDomain     *string `json:"cloudflare_domain"`
	TempMailBaseURL      *string `json:"tempmail_base_url"`
	TempMailAdminAuth    *string `json:"tempmail_admin_auth"`
	TempMailDomain       *string `json:"tempmail_domain"`
	DefaultMailboxID     *int    `json:"default_mailbox_id"`
	CPAEndpoint          *string `json:"cpa_endpoint"`
	CPAToken             *string `json:"cpa_token"`
	TestType             *string `json:"test_type"`
}

type automationAccountUpsertRequest struct {
	Name                *string `json:"name"`
	Email               *string `json:"email"`
	Password            *string `json:"password"`
	RedeemCode          *string `json:"redeem_code"`
	RedeemType          *string `json:"redeem_type"`
	Status              *string `json:"status"`
	FailureReason       *string `json:"failure_reason"`
	CloudflareDomain    *string `json:"cloudflare_domain"`
	CloudflareAlias     *string `json:"cloudflare_alias"`
	ForwardTarget       *string `json:"forward_target"`
	RedeemSite          *string `json:"redeem_site"`
	InviteLink          *string `json:"invite_link"`
	InviteCode          *string `json:"invite_code"`
	MailboxID           *int    `json:"mailbox_id"`
	SessionArtifactPath *string `json:"session_artifact_path"`
	AuthArtifactPath    *string `json:"auth_artifact_path"`
	ImportedTarget      *string `json:"imported_target"`
	ImportedReference   *string `json:"imported_reference"`
	Metadata            *string `json:"metadata"`
}

type automationTeamUpsertRequest struct {
	Email                 *string `json:"email"`
	AccessToken           *string `json:"access_token"`
	RefreshToken          *string `json:"refresh_token"`
	SessionToken          *string `json:"session_token"`
	ClientID              *string `json:"client_id"`
	AccountID             *string `json:"account_id"`
	TeamName              *string `json:"team_name"`
	PlanType              *string `json:"plan_type"`
	SubscriptionPlan      *string `json:"subscription_plan"`
	ExpiresAt             *int64  `json:"expires_at"`
	CurrentMembers        *int    `json:"current_members"`
	PendingInvites        *int    `json:"pending_invites"`
	MaxMembers            *int    `json:"max_members"`
	Status                *string `json:"status"`
	AccountRole           *string `json:"account_role"`
	DeviceCodeAuthEnabled *bool   `json:"device_code_auth_enabled"`
	ErrorCount            *int    `json:"error_count"`
	LastSyncTime          *int64  `json:"last_sync_time"`
	Metadata              *string `json:"metadata"`
}

type automationTeamAutoSelectRequest struct {
	ExcludeTeamIDs []int `json:"exclude_team_ids"`
}

type automationBatchDeleteRequest struct {
	IDs        []int    `json:"ids"`
	Identities []string `json:"identities"`
	Emails     []string `json:"emails"`
	Status     string   `json:"status"`
	Search     string   `json:"search"`
	All        bool     `json:"all"`
}

type automationAccountBatchUpdateRequest struct {
	IDs          []int    `json:"ids"`
	Identities   []string `json:"identities"`
	Emails       []string `json:"emails"`
	FilterStatus string   `json:"filter_status"`
	Search       string   `json:"search"`
	All          bool     `json:"all"`
	Status       *string  `json:"status"`
	RedeemSite   *string  `json:"redeem_site"`
	Metadata     *string  `json:"metadata"`
}

type automationTeamBatchUpdateRequest struct {
	IDs          []int    `json:"ids"`
	Identities   []string `json:"identities"`
	Emails       []string `json:"emails"`
	FilterStatus string   `json:"filter_status"`
	Search       string   `json:"search"`
	All          bool     `json:"all"`
	Status       *string  `json:"status"`
	Metadata     *string  `json:"metadata"`
}

type automationRunBatchDeleteRequest struct {
	ExecutionIDs        []string `json:"execution_ids"`
	Status              string   `json:"status"`
	Module              string   `json:"module"`
	Search              string   `json:"search"`
	OnlyFailed          bool     `json:"only_failed"`
	OnlyProxyRetried    bool     `json:"only_proxy_retried"`
	DeleteArtifactFiles bool     `json:"delete_artifact_files"`
}

type automationImportRecord struct {
	Name                string
	Email               string
	Password            string
	RedeemCode          string
	RedeemType          string
	Status              string
	FailureReason       string
	CloudflareDomain    string
	CloudflareAlias     string
	ForwardTarget       string
	RedeemSite          string
	InviteLink          string
	InviteCode          string
	MailboxID           int
	SessionArtifactPath string
	AuthArtifactPath    string
	ImportedTarget      string
	ImportedReference   string
	Metadata            string
}

type automationRegisteredImportOptions struct {
	StartFrom          string
	DefaultRedeemSite  string
	DefaultMailboxID   int
	ContinueFromModule bool
	AutoRun            bool
	RunOptions         map[string]any
}

type automationRedeemCodeImportItem struct {
	ID         int
	Email      string
	Name       string
	Password   string
	RedeemCode string
	RedeemType string
	RedeemSite string
	Status     string
	Metadata   string
}

func normalizeAutomationModuleID(module string) string {
	module = strings.TrimSpace(strings.ToLower(module))
	switch module {
	case "", "all":
		return ""
	case "1", "module1", "module_1", "cloudflare":
		return constant.AutomationModuleCloudflareEmailGenerate
	case "2", "module2", "module_2", "redeem":
		return constant.AutomationModuleTeamRedeem
	case "3", "module3", "module_3", "gmail":
		return constant.AutomationModuleGmailInviteWatch
	case "4", "module4", "module_4", "browser", "browser_login":
		return constant.AutomationModuleBrowserLoginCapture
	case "6", "module6", "module_6", "import":
		return constant.AutomationModuleImportTarget
	case constant.AutomationModuleCloudflareEmailGenerate,
		constant.AutomationModuleTeamRedeem,
		constant.AutomationModuleGmailInviteWatch,
		constant.AutomationModuleBrowserLoginCapture,
		constant.AutomationModuleImportTarget:
		return module
	default:
		return module
	}
}

func normalizeAutomationStartFrom(raw string) string {
	module := normalizeAutomationModuleID(raw)
	switch module {
	case "",
		constant.AutomationModuleCloudflareEmailGenerate,
		constant.AutomationModuleTeamRedeem,
		constant.AutomationModuleGmailInviteWatch,
		constant.AutomationModuleBrowserLoginCapture,
		constant.AutomationModuleImportTarget:
		return module
	default:
		return ""
	}
}

func defaultAutomationStatusByStartFrom(startFrom string) string {
	switch normalizeAutomationStartFrom(startFrom) {
	case constant.AutomationModuleCloudflareEmailGenerate, "":
		return constant.AutomationAccountStatusPending
	case constant.AutomationModuleTeamRedeem:
		return constant.AutomationAccountStatusGenerated
	case constant.AutomationModuleGmailInviteWatch:
		return constant.AutomationAccountStatusWaitingInvite
	case constant.AutomationModuleBrowserLoginCapture:
		return constant.AutomationAccountStatusInviteReceived
	case constant.AutomationModuleImportTarget:
		return constant.AutomationAccountStatusConverted
	default:
		return constant.AutomationAccountStatusPending
	}
}

func normalizeAutomationImportStatus(raw string, startFrom string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	switch status {
	case constant.AutomationAccountStatusPending,
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
		constant.AutomationAccountStatusDisabled:
		return status
	default:
		return defaultAutomationStatusByStartFrom(startFrom)
	}
}

func parseBoolValue(raw string, fallback bool) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func automationAliasValues(row map[string]any, keys ...string) string {
	if len(row) == 0 {
		return ""
	}
	for _, key := range keys {
		normalized := strings.TrimSpace(strings.ToLower(key))
		if normalized == "" {
			continue
		}
		value, ok := row[normalized]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		default:
			stringified := strings.TrimSpace(fmt.Sprintf("%v", typed))
			if stringified != "" && stringified != "<nil>" {
				return stringified
			}
		}
	}
	return ""
}

func mapImportRecordFromAliases(values map[string]any) automationImportRecord {
	record := automationImportRecord{
		Name:                automationAliasValues(values, "name", "nickname", "display_name"),
		Email:               strings.ToLower(strings.TrimSpace(automationAliasValues(values, "email", "account", "username", "mail", "login_email"))),
		Password:            automationAliasValues(values, "password", "pass", "pwd", "login_password"),
		RedeemCode:          automationAliasValues(values, "redeem_code", "code", "team_code", "redeemtoken", "invite_code"),
		RedeemType:          automationAliasValues(values, "redeem_type", "code_type", "plan_type"),
		Status:              automationAliasValues(values, "status", "account_status"),
		FailureReason:       automationAliasValues(values, "failure_reason", "error", "reason"),
		CloudflareDomain:    automationAliasValues(values, "cloudflare_domain", "domain"),
		CloudflareAlias:     automationAliasValues(values, "cloudflare_alias", "alias"),
		ForwardTarget:       automationAliasValues(values, "forward_target", "forward_email", "target_email"),
		RedeemSite:          automationAliasValues(values, "redeem_site", "site", "redeem_domain", "redeem_url"),
		InviteLink:          automationAliasValues(values, "invite_link", "invitation_link"),
		InviteCode:          automationAliasValues(values, "invite_code", "invitation_code"),
		SessionArtifactPath: automationAliasValues(values, "session_artifact_path", "session_backup_path", "storage_state_path"),
		AuthArtifactPath:    automationAliasValues(values, "auth_artifact_path", "auth_file_path"),
		ImportedTarget:      automationAliasValues(values, "imported_target", "target"),
		ImportedReference:   automationAliasValues(values, "imported_reference", "reference", "channel_name"),
		Metadata:            automationAliasValues(values, "metadata", "meta", "extra"),
	}

	mailboxRaw := automationAliasValues(values, "mailbox_id", "mailbox", "mailboxid")
	if mailboxRaw != "" {
		if mailboxID, err := strconv.Atoi(strings.TrimSpace(mailboxRaw)); err == nil {
			record.MailboxID = mailboxID
		}
	}
	if record.Metadata == "" {
		if metaRaw, ok := values["metadata"]; ok {
			if data, err := common.Marshal(metaRaw); err == nil {
				record.Metadata = strings.TrimSpace(string(data))
			}
		} else if metaRaw, ok := values["meta"]; ok {
			if data, err := common.Marshal(metaRaw); err == nil {
				record.Metadata = strings.TrimSpace(string(data))
			}
		}
	}
	return record
}

func mergeAutomationImportMetadata(raw string, importBatch string) string {
	raw = strings.TrimSpace(raw)
	importBatch = strings.TrimSpace(importBatch)
	if importBatch == "" {
		return raw
	}

	parsed := map[string]any{}
	if raw != "" {
		if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
			parsed = map[string]any{"raw": raw}
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	parsed["import_batch"] = importBatch
	parsed["import_source"] = "registered"
	data, err := common.Marshal(parsed)
	if err != nil {
		return raw
	}
	return string(data)
}

func mergeAutomationBatchMetadata(current string, incoming string) string {
	current = strings.TrimSpace(current)
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return current
	}
	if current == "" {
		return incoming
	}
	if current == incoming {
		return current
	}

	currentJSON := map[string]any{}
	incomingJSON := map[string]any{}
	currentOK := common.UnmarshalJsonStr(current, &currentJSON) == nil
	incomingOK := common.UnmarshalJsonStr(incoming, &incomingJSON) == nil
	if currentOK && incomingOK {
		if currentJSON == nil {
			currentJSON = map[string]any{}
		}
		for key, value := range incomingJSON {
			currentJSON[key] = value
		}
		if data, err := common.Marshal(currentJSON); err == nil {
			return string(data)
		}
	}

	lines := strings.Split(strings.ReplaceAll(current, "\r\n", "\n"), "\n")
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	appendLines := strings.Split(strings.ReplaceAll(incoming, "\r\n", "\n"), "\n")
	for _, line := range appendLines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		lines = append(lines, value)
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value != "" {
			result = append(result, value)
		}
	}
	return strings.Join(result, "\n")
}

func parseAutomationImportCSVRecords(raw []byte) ([]automationImportRecord, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return []automationImportRecord{}, nil
	}
	header := rows[0]
	indexMap := map[string]int{}
	for idx, col := range header {
		indexMap[strings.TrimSpace(strings.ToLower(col))] = idx
	}

	readRow := func(row []string, aliases ...string) string {
		for _, alias := range aliases {
			key := strings.TrimSpace(strings.ToLower(alias))
			idx, ok := indexMap[key]
			if !ok || idx >= len(row) {
				continue
			}
			value := strings.TrimSpace(row[idx])
			if value != "" {
				return value
			}
		}
		return ""
	}

	emailAliasFound := false
	for _, alias := range []string{"email", "account", "username", "mail", "login_email"} {
		if _, ok := indexMap[alias]; ok {
			emailAliasFound = true
			break
		}
	}
	if !emailAliasFound {
		return nil, errors.New("csv missing required column: email/account/username")
	}

	records := make([]automationImportRecord, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		fields := map[string]any{
			"name":                  readRow(row, "name", "nickname", "display_name"),
			"email":                 strings.ToLower(strings.TrimSpace(readRow(row, "email", "account", "username", "mail", "login_email"))),
			"password":              readRow(row, "password", "pass", "pwd", "login_password"),
			"redeem_code":           readRow(row, "redeem_code", "code", "team_code", "redeemtoken", "invite_code"),
			"redeem_type":           readRow(row, "redeem_type", "code_type", "plan_type"),
			"status":                readRow(row, "status", "account_status"),
			"failure_reason":        readRow(row, "failure_reason", "error", "reason"),
			"cloudflare_domain":     readRow(row, "cloudflare_domain", "domain"),
			"cloudflare_alias":      readRow(row, "cloudflare_alias", "alias"),
			"forward_target":        readRow(row, "forward_target", "forward_email", "target_email"),
			"redeem_site":           readRow(row, "redeem_site", "site", "redeem_domain", "redeem_url"),
			"invite_link":           readRow(row, "invite_link", "invitation_link"),
			"invite_code":           readRow(row, "invite_code", "invitation_code"),
			"mailbox_id":            readRow(row, "mailbox_id", "mailbox", "mailboxid"),
			"session_artifact_path": readRow(row, "session_artifact_path", "session_backup_path", "storage_state_path"),
			"auth_artifact_path":    readRow(row, "auth_artifact_path", "auth_file_path"),
			"imported_target":       readRow(row, "imported_target", "target"),
			"imported_reference":    readRow(row, "imported_reference", "reference", "channel_name"),
			"metadata":              readRow(row, "metadata", "meta", "extra"),
		}
		records = append(records, mapImportRecordFromAliases(fields))
	}
	return records, nil
}

func parseAutomationImportJSONRecords(raw any) ([]automationImportRecord, error) {
	if raw == nil {
		return []automationImportRecord{}, nil
	}
	items := make([]any, 0)
	switch typed := raw.(type) {
	case string:
		return parseAutomationImportTextRecords(typed), nil
	case []any:
		items = typed
	case map[string]any:
		if records, ok := typed["accounts"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		if records, ok := typed["items"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		if records, ok := typed["data"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		if records, ok := typed["text"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		if records, ok := typed["lines"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		if records, ok := typed["payload"]; ok {
			return parseAutomationImportJSONRecords(records)
		}
		items = []any{typed}
	default:
		return nil, errors.New("import payload must be object/array/string")
	}

	records := make([]automationImportRecord, 0, len(items))
	for _, item := range items {
		recordMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalized := map[string]any{}
		for key, value := range recordMap {
			normalized[strings.TrimSpace(strings.ToLower(key))] = value
		}
		records = append(records, mapImportRecordFromAliases(normalized))
	}
	return records, nil
}

func parseAutomationImportTextRecords(raw string) []automationImportRecord {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	records := make([]automationImportRecord, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == '\t' || r == '|'
		})
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) == 0 {
			continue
		}

		first := strings.ToLower(parts[0])
		if first == "email" || first == "account" || first == "username" {
			continue
		}

		values := map[string]any{}
		switch {
		case strings.Contains(parts[0], "@"):
			values["email"] = parts[0]
			if len(parts) > 1 {
				values["password"] = parts[1]
			}
			if len(parts) > 2 {
				values["redeem_code"] = parts[2]
			}
			if len(parts) > 3 {
				values["redeem_site"] = parts[3]
			}
		case len(parts) > 1 && strings.Contains(parts[1], "@"):
			values["name"] = parts[0]
			values["email"] = parts[1]
			if len(parts) > 2 {
				values["password"] = parts[2]
			}
			if len(parts) > 3 {
				values["redeem_code"] = parts[3]
			}
			if len(parts) > 4 {
				values["redeem_site"] = parts[4]
			}
		default:
			// 保持兜底：格式不标准时仍返回记录，让后续统一校验并给出错误行
			values["email"] = parts[0]
			if len(parts) > 1 {
				values["password"] = parts[1]
			}
			if len(parts) > 2 {
				values["redeem_code"] = parts[2]
			}
			if len(parts) > 3 {
				values["redeem_site"] = parts[3]
			}
		}
		records = append(records, mapImportRecordFromAliases(values))
	}
	return records
}

func mapRedeemCodeImportItem(values map[string]any) automationRedeemCodeImportItem {
	item := automationRedeemCodeImportItem{
		Email:      strings.ToLower(strings.TrimSpace(automationAliasValues(values, "email", "account", "username", "mail", "login_email"))),
		Name:       strings.TrimSpace(automationAliasValues(values, "name", "nickname", "display_name")),
		Password:   strings.TrimSpace(automationAliasValues(values, "password", "pass", "pwd", "login_password")),
		RedeemCode: strings.TrimSpace(automationAliasValues(values, "redeem_code", "code", "team_code", "redeemtoken", "invite_code")),
		RedeemType: strings.TrimSpace(automationAliasValues(values, "redeem_type", "code_type", "plan_type")),
		RedeemSite: normalizeTeamAutomationRedeemSite(strings.TrimSpace(automationAliasValues(values, "redeem_site", "site", "redeem_domain", "redeem_url"))),
		Status:     strings.TrimSpace(strings.ToLower(automationAliasValues(values, "status", "account_status"))),
		Metadata:   strings.TrimSpace(automationAliasValues(values, "metadata", "meta", "extra")),
	}
	if idRaw := strings.TrimSpace(automationAliasValues(values, "id", "account_id")); idRaw != "" {
		if idValue, err := strconv.Atoi(idRaw); err == nil {
			item.ID = idValue
		}
	}
	return item
}

func parseRedeemCodeImportItemsFromText(raw string) []automationRedeemCodeImportItem {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	result := make([]automationRedeemCodeImportItem, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == '\t' || r == '|'
		})
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) == 0 {
			continue
		}
		item := automationRedeemCodeImportItem{}
		// 支持: email,code[,site] / id,code[,site] / id,email,code[,site]
		if strings.Contains(parts[0], "@") {
			item.Email = strings.ToLower(parts[0])
			if len(parts) > 1 {
				item.RedeemCode = parts[1]
			}
			if len(parts) > 2 {
				item.RedeemSite = normalizeTeamAutomationRedeemSite(parts[2])
			}
		} else if id, err := strconv.Atoi(parts[0]); err == nil && id > 0 {
			item.ID = id
			if len(parts) > 1 && strings.Contains(parts[1], "@") {
				item.Email = strings.ToLower(parts[1])
				if len(parts) > 2 {
					item.RedeemCode = parts[2]
				}
				if len(parts) > 3 {
					item.RedeemSite = normalizeTeamAutomationRedeemSite(parts[3])
				}
			} else {
				if len(parts) > 1 {
					item.RedeemCode = parts[1]
				}
				if len(parts) > 2 {
					item.RedeemSite = normalizeTeamAutomationRedeemSite(parts[2])
				}
			}
		} else {
			// 没有 email/id，按 code 处理，后续会给出失败原因
			item.RedeemCode = parts[0]
			if len(parts) > 1 {
				item.RedeemSite = normalizeTeamAutomationRedeemSite(parts[1])
			}
		}
		result = append(result, item)
	}
	return result
}

func parseRedeemCodeImportItems(raw any) ([]automationRedeemCodeImportItem, error) {
	if raw == nil {
		return []automationRedeemCodeImportItem{}, nil
	}
	switch typed := raw.(type) {
	case string:
		return parseRedeemCodeImportItemsFromText(typed), nil
	case []any:
		items := make([]automationRedeemCodeImportItem, 0, len(typed))
		for _, item := range typed {
			switch row := item.(type) {
			case string:
				items = append(items, parseRedeemCodeImportItemsFromText(row)...)
			case map[string]any:
				normalized := map[string]any{}
				for key, value := range row {
					normalized[strings.TrimSpace(strings.ToLower(key))] = value
				}
				items = append(items, mapRedeemCodeImportItem(normalized))
			}
		}
		return items, nil
	case map[string]any:
		if lines, ok := typed["lines"]; ok {
			return parseRedeemCodeImportItems(lines)
		}
		if text, ok := typed["text"]; ok {
			return parseRedeemCodeImportItems(text)
		}
		if items, ok := typed["items"]; ok {
			return parseRedeemCodeImportItems(items)
		}
		if accounts, ok := typed["accounts"]; ok {
			return parseRedeemCodeImportItems(accounts)
		}
		normalized := map[string]any{}
		for key, value := range typed {
			normalized[strings.TrimSpace(strings.ToLower(key))] = value
		}
		return []automationRedeemCodeImportItem{mapRedeemCodeImportItem(normalized)}, nil
	default:
		return nil, errors.New("redeem code import payload must be object/array/string")
	}
}

func upsertAutomationRedeemCodes(items []automationRedeemCodeImportItem, options automationRegisteredImportOptions) (map[string]any, error) {
	importBatch := fmt.Sprintf("redeem-%d", time.Now().UTC().UnixNano())
	created := 0
	updated := 0
	failed := 0
	createdIDs := make([]int, 0)
	updatedIDs := make([]int, 0)
	errorsList := make([]string, 0)

	for idx, item := range items {
		code := strings.TrimSpace(item.RedeemCode)
		if code == "" {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: redeem_code is empty", idx+1))
			continue
		}
		site := normalizeTeamAutomationRedeemSite(item.RedeemSite)
		if site == "" {
			site = options.DefaultRedeemSite
		}
		status := normalizeAutomationImportStatus(item.Status, options.StartFrom)
		email := strings.ToLower(strings.TrimSpace(item.Email))

		var account *model.AutomationAccount
		var err error
		if item.ID > 0 {
			account, err = model.GetAutomationAccountByID(item.ID)
		} else if email != "" {
			account, err = model.GetAutomationAccountByEmail(email)
		} else {
			err = gorm.ErrRecordNotFound
		}

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+1, err))
			continue
		}

		metadata := strings.TrimSpace(item.Metadata)
		if metadata == "" && account != nil {
			metadata = account.Metadata
		}
		metadata = mergeAutomationImportMetadata(metadata, importBatch)

		if errors.Is(err, gorm.ErrRecordNotFound) || account == nil {
			if email == "" {
				failed++
				errorsList = append(errorsList, fmt.Sprintf("line %d: email/id is required for new account", idx+1))
				continue
			}
			newAccount := &model.AutomationAccount{
				Name:          strings.TrimSpace(item.Name),
				Email:         email,
				Password:      strings.TrimSpace(item.Password),
				RedeemCode:    code,
				RedeemType:    strings.TrimSpace(item.RedeemType),
				RedeemSite:    site,
				Status:        status,
				FailureReason: "",
				Metadata:      metadata,
			}
			if newAccount.Name == "" {
				newAccount.Name = email
			}
			if insertErr := newAccount.Insert(); insertErr != nil {
				failed++
				errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+1, insertErr))
				continue
			}
			created++
			createdIDs = append(createdIDs, newAccount.Id)
			continue
		}

		account.RedeemCode = code
		if strings.TrimSpace(item.RedeemType) != "" {
			account.RedeemType = strings.TrimSpace(item.RedeemType)
		}
		if site != "" {
			account.RedeemSite = site
		}
		if strings.TrimSpace(item.Name) != "" {
			account.Name = strings.TrimSpace(item.Name)
		}
		if strings.TrimSpace(item.Password) != "" {
			account.Password = strings.TrimSpace(item.Password)
		}
		if email != "" {
			account.Email = email
		}
		account.Status = status
		account.FailureReason = ""
		account.Metadata = metadata
		if updateErr := account.Update(); updateErr != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+1, updateErr))
			continue
		}
		updated++
		updatedIDs = append(updatedIDs, account.Id)
	}

	sort.Strings(errorsList)
	if len(errorsList) > 50 {
		errorsList = errorsList[:50]
	}
	return map[string]any{
		"created":      created,
		"updated":      updated,
		"failed":       failed,
		"errors":       errorsList,
		"import_batch": importBatch,
		"start_from":   options.StartFrom,
		"status_set":   defaultAutomationStatusByStartFrom(options.StartFrom),
		"created_ids":  createdIDs,
		"updated_ids":  updatedIDs,
	}, nil
}

func getContextRole(c *gin.Context) int {
	roleAny, ok := c.Get("role")
	if !ok {
		return 0
	}
	role, ok := roleAny.(int)
	if !ok {
		return 0
	}
	return role
}

func canConfigureAutomation(c *gin.Context) bool {
	return getContextRole(c) >= common.RoleRootUser
}

func decodeJSONBody(c *gin.Context, dest any) error {
	if c.Request == nil || c.Request.Body == nil {
		return nil
	}
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	return common.Unmarshal(payload, dest)
}

func decodeJSONAnyBody(c *gin.Context) (any, error) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, nil
	}
	var body any
	if err := common.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func listAutomationModuleOptions() []map[string]string {
	defs := service.GetAutomationModuleDefinitions()
	options := make([]map[string]string, 0, len(defs)+1)
	options = append(options, map[string]string{
		"label": "全部模块",
		"value": "",
	})
	for _, module := range defs {
		options = append(options, map[string]string{
			"label": module.Name,
			"value": module.ID,
		})
	}
	return options
}

func listAutomationRedeemSites() []map[string]string {
	return []map[string]string{
		{"label": "自动", "value": "auto"},
		{"label": "helloteam.store", "value": "helloteam.store"},
		{"label": "teamxz.store", "value": "teamxz.store"},
		{"label": "chatgpt.com", "value": "chatgpt.com"},
		{"label": "chat.openai.com", "value": "chat.openai.com"},
		{"label": "api.caowo.de", "value": "api.caowo.de"},
		{"label": "自定义站点", "value": teamAutomationCustomRedeemSiteValue},
	}
}

func buildAutomationSettingsFromRequest(req automationSettingsRequest) service.AutomationSettings {
	settings := service.LoadAutomationSettings()
	if req.Mode != nil {
		settings.Mode = strings.TrimSpace(strings.ToLower(*req.Mode))
	}
	if req.WorkerBaseURL != nil {
		settings.WorkerBaseURL = strings.TrimRight(strings.TrimSpace(*req.WorkerBaseURL), "/")
	}
	if req.WorkerUsername != nil {
		settings.WorkerUsername = strings.TrimSpace(*req.WorkerUsername)
	}
	if req.WorkerPassword != nil {
		settings.WorkerPassword = strings.TrimSpace(*req.WorkerPassword)
	}
	if req.WorkerTimeoutSeconds != nil {
		settings.WorkerTimeoutSeconds = *req.WorkerTimeoutSeconds
	}
	if req.WorkerVerifySSL != nil {
		settings.WorkerVerifySSL = *req.WorkerVerifySSL
	}
	if req.ArtifactRoot != nil {
		settings.ArtifactRoot = strings.TrimSpace(*req.ArtifactRoot)
	}
	if req.PlaywrightHeadless != nil {
		settings.PlaywrightHeadless = *req.PlaywrightHeadless
	}
	if req.PlaywrightBrowser != nil {
		settings.PlaywrightBrowser = strings.TrimSpace(strings.ToLower(*req.PlaywrightBrowser))
	}
	if req.RegisterEntryURL != nil {
		settings.RegisterEntryURL = strings.TrimSpace(*req.RegisterEntryURL)
	}
	if req.RegisterProxyPool != nil {
		settings.RegisterProxyPool = strings.TrimSpace(*req.RegisterProxyPool)
	}
	if req.RegisterProxyGateway != nil {
		settings.RegisterProxyGateway = strings.TrimSpace(*req.RegisterProxyGateway)
	}
	if req.RegisterUserAgent != nil {
		settings.RegisterUserAgent = strings.TrimSpace(*req.RegisterUserAgent)
	}
	if req.CloudflareAPIToken != nil {
		settings.CloudflareAPIToken = strings.TrimSpace(*req.CloudflareAPIToken)
	}
	if req.CloudflareAPIEmail != nil {
		settings.CloudflareAPIEmail = strings.TrimSpace(strings.ToLower(*req.CloudflareAPIEmail))
	}
	if req.CloudflareZoneID != nil {
		settings.CloudflareZoneID = strings.TrimSpace(*req.CloudflareZoneID)
	}
	if req.CloudflareDomain != nil {
		settings.CloudflareDomain = strings.TrimSpace(strings.ToLower(*req.CloudflareDomain))
	}
	if req.TempMailBaseURL != nil {
		settings.TempMailBaseURL = strings.TrimRight(strings.TrimSpace(*req.TempMailBaseURL), "/")
	}
	if req.TempMailAdminAuth != nil {
		settings.TempMailAdminAuth = strings.TrimSpace(*req.TempMailAdminAuth)
	}
	if req.TempMailDomain != nil {
		settings.TempMailDomain = strings.TrimSpace(strings.ToLower(*req.TempMailDomain))
	}
	if req.DefaultMailboxID != nil {
		settings.DefaultMailboxID = *req.DefaultMailboxID
	}
	if req.CPAEndpoint != nil {
		settings.CPAEndpoint = strings.TrimRight(strings.TrimSpace(*req.CPAEndpoint), "/")
	}
	if req.CPAToken != nil {
		settings.CPAToken = strings.TrimSpace(*req.CPAToken)
	}
	return settings
}

func buildAutomationSettingsResponse(settings service.AutomationSettings, canConfigure bool) map[string]any {
	result := settings.Sanitized()
	legacy := service.LoadTeamAutomationConfig()
	result["proxy_pool_summary"] = service.BuildAutomationProxyPoolSummary(settings.RegisterProxyPool, settings.RegisterProxyGateway)
	result["can_configure"] = canConfigure
	result["legacy_team_automation_enabled"] = common.OptionMap["TeamAutomationEnabled"] == "true"
	result["legacy_team_automation_mode"] = strings.TrimSpace(common.OptionMap["TeamAutomationMode"])
	result["base_url"] = settings.WorkerBaseURL
	result["resolved_base_url"] = legacy.EffectiveBaseURL()
	result["timeout_seconds"] = settings.WorkerTimeoutSeconds
	result["verify_ssl"] = settings.WorkerVerifySSL
	result["auth_configured"] = strings.TrimSpace(settings.WorkerUsername) != "" && strings.TrimSpace(settings.WorkerPassword) != ""
	return result
}

func normalizeAutomationExecutionIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func isAutomationExecutionDeletableStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case constant.AutomationExecutionStatusQueued,
		constant.AutomationExecutionStatusRunning,
		constant.AutomationExecutionStatusStopping:
		return false
	default:
		return true
	}
}

func resolveAutomationRunDeleteExecutionIDs(req automationRunBatchDeleteRequest) ([]string, error) {
	executionIDs := normalizeAutomationExecutionIDs(req.ExecutionIDs)
	if len(executionIDs) > 0 {
		return executionIDs, nil
	}
	filter := model.AutomationExecutionListOptions{
		Status:           strings.TrimSpace(strings.ToLower(req.Status)),
		Module:           normalizeAutomationModuleID(req.Module),
		Search:           strings.TrimSpace(req.Search),
		OnlyProxyRetried: req.OnlyProxyRetried,
	}
	if req.OnlyFailed {
		filter.Status = constant.AutomationExecutionStatusFailed
	}
	return model.ListAutomationExecutionIDs(filter, 0)
}

func deleteAutomationArtifactFiles(root string, paths []string) (int, []string) {
	artifactRoot := strings.TrimSpace(root)
	if artifactRoot == "" {
		return 0, nil
	}
	artifactRootAbs, err := filepath.Abs(artifactRoot)
	if err != nil {
		return 0, []string{fmt.Sprintf("artifact root invalid: %v", err)}
	}
	deleted := 0
	failures := make([]string, 0)
	seen := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		normalized := strings.TrimSpace(rawPath)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		targetAbs, err := filepath.Abs(normalized)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", normalized, err))
			continue
		}
		rel, err := filepath.Rel(artifactRootAbs, targetAbs)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			failures = append(failures, fmt.Sprintf("%s: outside artifact root", normalized))
			continue
		}
		if err := os.Remove(targetAbs); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			failures = append(failures, fmt.Sprintf("%s: %v", normalized, err))
			continue
		}
		deleted++
	}
	sort.Strings(failures)
	if len(failures) > 20 {
		failures = failures[:20]
	}
	return deleted, failures
}

func buildRemoteClientBySettings(settings service.AutomationSettings) (*service.TeamAutomationClient, error) {
	config := settings.ToRemoteTeamAutomationConfig()
	config.Enabled = true
	return service.NewTeamAutomationClient(config)
}

func withRemoteReadFallback(c *gin.Context, settings service.AutomationSettings, fallback any) bool {
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiSuccess(c, gin.H{
			"mode":       constant.AutomationModeDisabled,
			"enabled":    false,
			"configured": false,
			"data":       fallback,
		})
		return true
	}
	if mode != constant.AutomationModeRemote {
		return false
	}
	if strings.TrimSpace(settings.WorkerBaseURL) == "" {
		common.ApiSuccess(c, gin.H{
			"mode":       constant.AutomationModeRemote,
			"enabled":    true,
			"configured": false,
			"warning":    "remote worker base url is not configured",
			"data":       fallback,
		})
		return true
	}
	return false
}

func proxyRemoteTeamAutomation(c *gin.Context, method string, path string, query url.Values, body any) {
	settings := service.LoadAutomationSettings()
	client, err := buildRemoteClientBySettings(settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload, _, err := client.Do(c.Request.Context(), method, path, query, body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func formatAutomationTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func toAutomationAccountPayload(account *model.AutomationAccount) map[string]any {
	if account == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                    account.Id,
		"name":                  account.Name,
		"email":                 account.Email,
		"redeem_code":           account.RedeemCode,
		"redeem_type":           account.RedeemType,
		"status":                account.Status,
		"failure_reason":        account.FailureReason,
		"cloudflare_domain":     account.CloudflareDomain,
		"cloudflare_alias":      account.CloudflareAlias,
		"forward_target":        account.ForwardTarget,
		"redeem_site":           account.RedeemSite,
		"invite_link":           account.InviteLink,
		"invite_code":           account.InviteCode,
		"mailbox_id":            account.MailboxId,
		"session_artifact_path": account.SessionArtifactPath,
		"auth_artifact_path":    account.AuthArtifactPath,
		"imported_target":       account.ImportedTarget,
		"imported_reference":    account.ImportedReference,
		"metadata":              account.Metadata,
		"created_time":          account.CreatedTime,
		"updated_time":          account.UpdatedTime,
		"last_update_time":      formatAutomationTime(account.UpdatedTime),
		"site_used":             account.RedeemSite,
		"session_backup_path":   account.SessionArtifactPath,
		"auth_file_path":        account.AuthArtifactPath,
		"newapi_channel_name":   account.ImportedReference,
	}
}

func toAutomationTeamPayload(team *model.AutomationTeam) map[string]any {
	if team == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                       team.Id,
		"email":                    team.Email,
		"account_id":               team.AccountID,
		"team_name":                team.TeamName,
		"plan_type":                team.PlanType,
		"subscription_plan":        team.SubscriptionPlan,
		"expires_at":               team.ExpiresAt,
		"expires_at_text":          formatAutomationTime(team.ExpiresAt),
		"current_members":          team.CurrentMembers,
		"pending_invites":          team.PendingInvites,
		"max_members":              team.MaxMembers,
		"occupied_slots":           team.OccupiedSlots(),
		"remaining_slots":          team.RemainingSlots(),
		"status":                   team.Status,
		"account_role":             team.AccountRole,
		"device_code_auth_enabled": team.DeviceCodeAuthEnabled,
		"error_count":              team.ErrorCount,
		"last_sync_time":           team.LastSyncTime,
		"last_sync_time_text":      formatAutomationTime(team.LastSyncTime),
		"metadata":                 team.Metadata,
		"created_time":             team.CreatedTime,
		"updated_time":             team.UpdatedTime,
		"last_update_time":         formatAutomationTime(team.UpdatedTime),
		"has_access_token":         strings.TrimSpace(team.AccessToken) != "",
		"has_refresh_token":        strings.TrimSpace(team.RefreshToken) != "",
		"has_session_token":        strings.TrimSpace(team.SessionToken) != "",
		"client_id":                team.ClientID,
	}
}

func loadAutomationAccountByIdentity(identity string) (*model.AutomationAccount, error) {
	trimmed := strings.TrimSpace(identity)
	if trimmed == "" {
		return nil, fmt.Errorf("identity is required")
	}
	if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
		return model.GetAutomationAccountByID(id)
	}
	return model.GetAutomationAccountByEmail(trimmed)
}

func loadAutomationTeamByIdentity(identity string) (*model.AutomationTeam, error) {
	trimmed := strings.TrimSpace(identity)
	if trimmed == "" {
		return nil, fmt.Errorf("identity is required")
	}
	if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
		return model.GetAutomationTeamByID(id)
	}
	return model.GetAutomationTeamByEmail(trimmed)
}

func collectAutomationBatchIdentities(req automationBatchDeleteRequest) []string {
	items := make([]string, 0, len(req.IDs)+len(req.Identities)+len(req.Emails))
	for _, id := range req.IDs {
		if id > 0 {
			items = append(items, strconv.Itoa(id))
		}
	}
	items = append(items, req.Identities...)
	items = append(items, req.Emails...)
	dedup := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := dedup[value]; ok {
			continue
		}
		dedup[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func resolveAutomationAccountBatchDeleteIdentities(req automationBatchDeleteRequest) ([]string, error) {
	identities := collectAutomationBatchIdentities(req)
	if len(identities) > 0 {
		return identities, nil
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	search := strings.TrimSpace(req.Search)
	if !req.All && status == "" && search == "" {
		return nil, nil
	}
	accounts, err := model.ListAutomationAccountsByFilter(status, search)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account == nil || account.Id <= 0 {
			continue
		}
		result = append(result, strconv.Itoa(account.Id))
	}
	return result, nil
}

func resolveAutomationTeamBatchDeleteIdentities(req automationBatchDeleteRequest) ([]string, error) {
	identities := collectAutomationBatchIdentities(req)
	if len(identities) > 0 {
		return identities, nil
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	search := strings.TrimSpace(req.Search)
	if !req.All && status == "" && search == "" {
		return nil, nil
	}
	teams, _, err := model.ListAutomationTeams(search, status, false, 0, 0)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(teams))
	for _, team := range teams {
		if team == nil || team.Id <= 0 {
			continue
		}
		result = append(result, strconv.Itoa(team.Id))
	}
	return result, nil
}

func collectAutomationBatchIdentitiesFromUpdate(ids []int, identities []string, emails []string) []string {
	return collectAutomationBatchIdentities(automationBatchDeleteRequest{
		IDs:        ids,
		Identities: identities,
		Emails:     emails,
	})
}

func resolveAutomationAccountBatchUpdateIdentities(req automationAccountBatchUpdateRequest) ([]string, error) {
	identities := collectAutomationBatchIdentitiesFromUpdate(req.IDs, req.Identities, req.Emails)
	if len(identities) > 0 {
		return identities, nil
	}
	status := strings.TrimSpace(strings.ToLower(req.FilterStatus))
	search := strings.TrimSpace(req.Search)
	if !req.All && status == "" && search == "" {
		return nil, nil
	}
	accounts, err := model.ListAutomationAccountsByFilter(status, search)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account == nil || account.Id <= 0 {
			continue
		}
		result = append(result, strconv.Itoa(account.Id))
	}
	return result, nil
}

func resolveAutomationTeamBatchUpdateIdentities(req automationTeamBatchUpdateRequest) ([]string, error) {
	identities := collectAutomationBatchIdentitiesFromUpdate(req.IDs, req.Identities, req.Emails)
	if len(identities) > 0 {
		return identities, nil
	}
	status := strings.TrimSpace(strings.ToLower(req.FilterStatus))
	search := strings.TrimSpace(req.Search)
	if !req.All && status == "" && search == "" {
		return nil, nil
	}
	teams, _, err := model.ListAutomationTeams(search, status, false, 0, 0)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(teams))
	for _, team := range teams {
		if team == nil || team.Id <= 0 {
			continue
		}
		result = append(result, strconv.Itoa(team.Id))
	}
	return result, nil
}

func parseAutomationAccountStatusForBatchUpdate(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case constant.AutomationAccountStatusPending,
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
		constant.AutomationAccountStatusDisabled:
		return value, nil
	default:
		return "", fmt.Errorf("invalid account status: %s", raw)
	}
}

func parseAutomationTeamStatusForBatchUpdate(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case model.AutomationTeamStatusActive,
		model.AutomationTeamStatusFull,
		model.AutomationTeamStatusExpired,
		model.AutomationTeamStatusError,
		model.AutomationTeamStatusBanned,
		model.AutomationTeamStatusDisabled:
		return value, nil
	default:
		return "", fmt.Errorf("invalid team status: %s", raw)
	}
}

func applyAutomationAccountUpdate(target *model.AutomationAccount, req automationAccountUpsertRequest, forCreate bool) {
	if req.Name != nil {
		target.Name = strings.TrimSpace(*req.Name)
	}
	if req.Email != nil {
		target.Email = strings.TrimSpace(strings.ToLower(*req.Email))
	}
	if req.Password != nil {
		value := strings.TrimSpace(*req.Password)
		if value != "" || forCreate {
			target.Password = value
		}
	}
	if req.RedeemCode != nil {
		target.RedeemCode = strings.TrimSpace(*req.RedeemCode)
	}
	if req.RedeemType != nil {
		target.RedeemType = strings.TrimSpace(*req.RedeemType)
	}
	if req.Status != nil {
		target.Status = strings.TrimSpace(strings.ToLower(*req.Status))
	}
	if req.FailureReason != nil {
		target.FailureReason = strings.TrimSpace(*req.FailureReason)
	}
	if req.CloudflareDomain != nil {
		target.CloudflareDomain = strings.TrimSpace(strings.ToLower(*req.CloudflareDomain))
	}
	if req.CloudflareAlias != nil {
		target.CloudflareAlias = strings.TrimSpace(strings.ToLower(*req.CloudflareAlias))
	}
	if req.ForwardTarget != nil {
		target.ForwardTarget = strings.TrimSpace(strings.ToLower(*req.ForwardTarget))
	}
	if req.RedeemSite != nil {
		target.RedeemSite = strings.TrimSpace(*req.RedeemSite)
	}
	if req.InviteLink != nil {
		target.InviteLink = strings.TrimSpace(*req.InviteLink)
	}
	if req.InviteCode != nil {
		target.InviteCode = strings.TrimSpace(*req.InviteCode)
	}
	if req.MailboxID != nil {
		target.MailboxId = *req.MailboxID
	}
	if req.SessionArtifactPath != nil {
		target.SessionArtifactPath = strings.TrimSpace(*req.SessionArtifactPath)
	}
	if req.AuthArtifactPath != nil {
		target.AuthArtifactPath = strings.TrimSpace(*req.AuthArtifactPath)
	}
	if req.ImportedTarget != nil {
		target.ImportedTarget = strings.TrimSpace(*req.ImportedTarget)
	}
	if req.ImportedReference != nil {
		target.ImportedReference = strings.TrimSpace(*req.ImportedReference)
	}
	if req.Metadata != nil {
		target.Metadata = strings.TrimSpace(*req.Metadata)
	}
	if target.Status == "" {
		target.Status = constant.AutomationAccountStatusPending
	}
}

func applyAutomationTeamUpdate(target *model.AutomationTeam, req automationTeamUpsertRequest, forCreate bool) {
	if req.Email != nil {
		target.Email = strings.TrimSpace(strings.ToLower(*req.Email))
	}
	if req.AccessToken != nil {
		value := strings.TrimSpace(*req.AccessToken)
		if value != "" || forCreate {
			target.AccessToken = value
		}
	}
	if req.RefreshToken != nil {
		value := strings.TrimSpace(*req.RefreshToken)
		if value != "" || forCreate {
			target.RefreshToken = value
		}
	}
	if req.SessionToken != nil {
		value := strings.TrimSpace(*req.SessionToken)
		if value != "" || forCreate {
			target.SessionToken = value
		}
	}
	if req.ClientID != nil {
		value := strings.TrimSpace(*req.ClientID)
		if value != "" || forCreate {
			target.ClientID = value
		}
	}
	if req.AccountID != nil {
		target.AccountID = strings.TrimSpace(*req.AccountID)
	}
	if req.TeamName != nil {
		target.TeamName = strings.TrimSpace(*req.TeamName)
	}
	if req.PlanType != nil {
		target.PlanType = strings.TrimSpace(strings.ToLower(*req.PlanType))
	}
	if req.SubscriptionPlan != nil {
		target.SubscriptionPlan = strings.TrimSpace(*req.SubscriptionPlan)
	}
	if req.ExpiresAt != nil {
		target.ExpiresAt = *req.ExpiresAt
	}
	if req.CurrentMembers != nil {
		target.CurrentMembers = *req.CurrentMembers
	}
	if req.PendingInvites != nil {
		target.PendingInvites = *req.PendingInvites
	}
	if req.MaxMembers != nil {
		target.MaxMembers = *req.MaxMembers
	}
	if req.Status != nil {
		target.Status = strings.TrimSpace(strings.ToLower(*req.Status))
	}
	if req.AccountRole != nil {
		target.AccountRole = strings.TrimSpace(strings.ToLower(*req.AccountRole))
	}
	if req.DeviceCodeAuthEnabled != nil {
		target.DeviceCodeAuthEnabled = *req.DeviceCodeAuthEnabled
	}
	if req.ErrorCount != nil {
		target.ErrorCount = *req.ErrorCount
	}
	if req.LastSyncTime != nil {
		target.LastSyncTime = *req.LastSyncTime
	}
	if req.Metadata != nil {
		target.Metadata = strings.TrimSpace(*req.Metadata)
	}
	service.FillAutomationTeamClaims(target)
	if target.Status == "" {
		target.Status = model.AutomationTeamStatusActive
	}
}

func parsePagination(c *gin.Context, defaultPage int, defaultPageSize int, maxPageSize int) (int, int) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("page_size")))
	if page <= 0 {
		page = defaultPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func anyToAutomationTeamInt(raw any) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		value, _ := strconv.Atoi(strings.TrimSpace(typed))
		return value
	default:
		value, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprintf("%v", typed)))
		return value
	}
}

func anyToAutomationTeamInt64(raw any) int64 {
	switch typed := raw.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		value, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return value
	default:
		value, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprintf("%v", typed)), 10, 64)
		return value
	}
}

func anyToAutomationTeamBool(raw any) bool {
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		return parseBoolValue(typed, false)
	default:
		return parseBoolValue(fmt.Sprintf("%v", typed), false)
	}
}

func mapAutomationTeamImportRecordFromAliases(values map[string]any) service.AutomationTeamImportRecord {
	return service.AutomationTeamImportRecord{
		Email:                 strings.TrimSpace(strings.ToLower(automationAliasValues(values, "email", "mail", "username"))),
		AccessToken:           automationAliasValues(values, "access_token", "token", "at"),
		RefreshToken:          automationAliasValues(values, "refresh_token", "rt"),
		SessionToken:          automationAliasValues(values, "session_token", "st"),
		ClientID:              automationAliasValues(values, "client_id"),
		AccountID:             automationAliasValues(values, "account_id"),
		TeamName:              automationAliasValues(values, "team_name", "name"),
		PlanType:              automationAliasValues(values, "plan_type"),
		SubscriptionPlan:      automationAliasValues(values, "subscription_plan", "subscription"),
		ExpiresAt:             anyToAutomationTeamInt64(automationAliasValues(values, "expires_at", "expiry", "expire_at")),
		CurrentMembers:        anyToAutomationTeamInt(automationAliasValues(values, "current_members", "members")),
		PendingInvites:        anyToAutomationTeamInt(automationAliasValues(values, "pending_invites")),
		MaxMembers:            anyToAutomationTeamInt(automationAliasValues(values, "max_members", "seats")),
		Status:                automationAliasValues(values, "status"),
		AccountRole:           automationAliasValues(values, "account_role", "role"),
		DeviceCodeAuthEnabled: anyToAutomationTeamBool(automationAliasValues(values, "device_code_auth_enabled", "device_code_auth")),
		ErrorCount:            anyToAutomationTeamInt(automationAliasValues(values, "error_count")),
		LastSyncTime:          anyToAutomationTeamInt64(automationAliasValues(values, "last_sync_time", "last_sync")),
		Metadata:              automationAliasValues(values, "metadata", "note"),
	}
}

func parseAutomationTeamImportTextContent(raw string) []service.AutomationTeamImportRecord {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	records := make([]service.AutomationTeamImportRecord, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var parts []string
		switch {
		case strings.Contains(line, "----"):
			parts = strings.Split(line, "----")
		case strings.Contains(line, "\t"):
			parts = strings.Split(line, "\t")
		case strings.Contains(line, "|"):
			parts = strings.Split(line, "|")
		default:
			parts = strings.Split(line, ",")
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		record := service.AutomationTeamImportRecord{}
		if len(parts) > 0 {
			record.Email = strings.TrimSpace(strings.ToLower(parts[0]))
		}
		if len(parts) > 1 {
			record.AccessToken = parts[1]
		}
		if len(parts) > 2 {
			record.RefreshToken = parts[2]
		}
		if len(parts) > 3 {
			record.SessionToken = parts[3]
		}
		if len(parts) > 4 {
			record.ClientID = parts[4]
		}
		if len(parts) > 5 {
			record.AccountID = parts[5]
		}
		if len(parts) > 6 {
			record.TeamName = parts[6]
		}
		if len(parts) > 7 {
			record.CurrentMembers = anyToAutomationTeamInt(parts[7])
		}
		if len(parts) > 8 {
			record.PendingInvites = anyToAutomationTeamInt(parts[8])
		}
		if len(parts) > 9 {
			record.MaxMembers = anyToAutomationTeamInt(parts[9])
		}
		if len(parts) > 10 {
			record.Status = parts[10]
		}
		records = append(records, record)
	}
	return records
}

func parseAutomationTeamImportPayload(raw any) ([]service.AutomationTeamImportRecord, error) {
	switch typed := raw.(type) {
	case nil:
		return nil, nil
	case string:
		return parseAutomationTeamImportTextContent(typed), nil
	case []any:
		records := make([]service.AutomationTeamImportRecord, 0, len(typed))
		for _, item := range typed {
			nested, err := parseAutomationTeamImportPayload(item)
			if err != nil {
				return nil, err
			}
			records = append(records, nested...)
		}
		return records, nil
	case map[string]any:
		if content := strings.TrimSpace(fmt.Sprintf("%v", typed["content"])); content != "" && content != "<nil>" {
			return parseAutomationTeamImportTextContent(content), nil
		}
		for _, key := range []string{"items", "records", "teams", "data"} {
			if nested, ok := typed[key]; ok {
				return parseAutomationTeamImportPayload(nested)
			}
		}
		return []service.AutomationTeamImportRecord{mapAutomationTeamImportRecordFromAliases(typed)}, nil
	default:
		return nil, fmt.Errorf("invalid team import payload")
	}
}

func normalizeTeamAutomationRedeemSite(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == teamAutomationCustomRedeemSiteValue {
		return ""
	}
	trimmed = strings.Trim(trimmed, `"'`)
	if trimmed == "" {
		return ""
	}
	switch strings.ToLower(trimmed) {
	case "<nil>", "nil", "null", "undefined", "auto":
		return ""
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			host := strings.ToLower(strings.TrimSpace(parsed.Host))
			path := strings.TrimSpace(parsed.Path)
			path = strings.TrimRight(path, "/")
			if path == "/" {
				path = ""
			}
			if host != "" {
				return host + path
			}
		}
		trimmed = strings.TrimPrefix(trimmed, "https://")
		trimmed = strings.TrimPrefix(trimmed, "http://")
	}

	if queryIdx := strings.IndexAny(trimmed, "?#"); queryIdx >= 0 {
		trimmed = trimmed[:queryIdx]
	}
	trimmed = strings.TrimRight(trimmed, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	host := strings.ToLower(strings.TrimSpace(parts[0]))
	if host == "" {
		return ""
	}
	if len(parts) == 1 {
		return host
	}
	path := "/" + strings.TrimLeft(parts[1], "/")
	path = strings.TrimRight(path, "/")
	if path == "/" {
		path = ""
	}
	return host + path
}

func ensureTeamAutomationSiteStrategyPreset(runOptions map[string]any, redeemSite string) {
	if len(runOptions) == 0 || redeemSite == "" {
		return
	}
	if existing, ok := runOptions["site_strategy"]; ok && existing != nil {
		existingValue := strings.TrimSpace(fmt.Sprintf("%v", existing))
		switch strings.ToLower(existingValue) {
		case "", "<nil>", "nil", "null", "undefined":
		default:
			return
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(redeemSite))
	switch {
	case strings.Contains(normalized, "helloteam.store"):
		runOptions["site_strategy"] = "helloteam"
	case strings.Contains(normalized, "teamxz.store"):
		runOptions["site_strategy"] = "teamxz"
	}
}

func normalizeTeamAutomationRunPayload(raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("team automation run payload must be a JSON object")
	}

	redeemSite := normalizeTeamAutomationRedeemSite(fmt.Sprintf("%v", payload["redeem_site"]))
	if alias := normalizeTeamAutomationRedeemSite(fmt.Sprintf("%v", payload["site"])); redeemSite == "" && alias != "" {
		redeemSite = alias
	}

	var runOptions map[string]any
	if rawRunOptions, exists := payload["run_options"]; exists && rawRunOptions != nil {
		parsedRunOptions, ok := rawRunOptions.(map[string]any)
		if !ok {
			return nil, errors.New("team automation run_options must be a JSON object")
		}
		runOptions = parsedRunOptions
		if runSite := normalizeTeamAutomationRedeemSite(fmt.Sprintf("%v", runOptions["redeem_site"])); runSite != "" {
			redeemSite = runSite
		}
		if runSiteAlias := normalizeTeamAutomationRedeemSite(fmt.Sprintf("%v", runOptions["site"])); redeemSite == "" && runSiteAlias != "" {
			redeemSite = runSiteAlias
		}
	}

	if redeemSite != "" {
		payload["redeem_site"] = redeemSite
	} else {
		delete(payload, "redeem_site")
	}

	if runOptions != nil {
		if redeemSite != "" {
			runOptions["redeem_site"] = redeemSite
		}
		ensureTeamAutomationSiteStrategyPreset(runOptions, redeemSite)
	}
	return payload, nil
}

func filterTeamAutomationAccountsCSV(body []byte, statusFilter string, searchFilter string) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	reader := csv.NewReader(bytes.NewReader(body))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return body, nil
	}

	header := rows[0]
	indexMap := map[string]int{}
	for idx, name := range header {
		indexMap[strings.TrimSpace(name)] = idx
	}

	statusIndex, hasStatus := indexMap["status"]
	emailIndex, hasEmail := indexMap["email"]
	redeemCodeIndex, hasRedeemCode := indexMap["redeem_code"]

	if statusFilter != "" && !hasStatus {
		return nil, errors.New("team automation export csv missing status column")
	}
	if searchFilter != "" && (!hasEmail || !hasRedeemCode) {
		return nil, errors.New("team automation export csv missing email or redeem_code column")
	}

	lowerSearch := strings.ToLower(searchFilter)
	filteredRows := make([][]string, 0, len(rows))
	filteredRows = append(filteredRows, header)

	for _, row := range rows[1:] {
		if statusFilter != "" {
			if statusIndex >= len(row) || strings.TrimSpace(row[statusIndex]) != statusFilter {
				continue
			}
		}
		if lowerSearch != "" {
			emailValue := ""
			redeemCodeValue := ""
			if emailIndex < len(row) {
				emailValue = strings.ToLower(strings.TrimSpace(row[emailIndex]))
			}
			if redeemCodeIndex < len(row) {
				redeemCodeValue = strings.ToLower(strings.TrimSpace(row[redeemCodeIndex]))
			}
			if !strings.Contains(emailValue, lowerSearch) && !strings.Contains(redeemCodeValue, lowerSearch) {
				continue
			}
		}
		filteredRows = append(filteredRows, row)
	}

	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.WriteAll(filteredRows); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func decodeAutomationRunRequest(c *gin.Context) (service.AutomationRunRequest, map[string]any, error) {
	bodyAny, err := decodeJSONAnyBody(c)
	if err != nil {
		return service.AutomationRunRequest{}, nil, err
	}
	bodyAny, err = normalizeTeamAutomationRunPayload(bodyAny)
	if err != nil {
		return service.AutomationRunRequest{}, nil, err
	}
	payload := map[string]any{}
	if bodyAny != nil {
		typed, ok := bodyAny.(map[string]any)
		if !ok {
			return service.AutomationRunRequest{}, nil, fmt.Errorf("run payload must be object")
		}
		payload = typed
	}
	if rawModule, ok := payload["module"]; ok {
		payload["module"] = normalizeAutomationModuleID(strings.TrimSpace(fmt.Sprintf("%v", rawModule)))
	}
	if rawStartFrom, ok := payload["start_from"]; ok && strings.TrimSpace(fmt.Sprintf("%v", payload["module"])) == "" {
		payload["module"] = normalizeAutomationStartFrom(strings.TrimSpace(fmt.Sprintf("%v", rawStartFrom)))
	}
	if rawModule, ok := payload["module_id"]; ok && strings.TrimSpace(fmt.Sprintf("%v", payload["module"])) == "" {
		payload["module"] = normalizeAutomationModuleID(strings.TrimSpace(fmt.Sprintf("%v", rawModule)))
	}
	if rawSite, ok := payload["site"]; ok {
		if strings.TrimSpace(fmt.Sprintf("%v", payload["redeem_site"])) == "" {
			payload["redeem_site"] = strings.TrimSpace(fmt.Sprintf("%v", rawSite))
		}
	}
	if runRaw, ok := payload["run_options"]; ok {
		switch typed := runRaw.(type) {
		case string:
			runMap := map[string]any{}
			if strings.TrimSpace(typed) != "" {
				if err := common.UnmarshalJsonStr(typed, &runMap); err != nil {
					return service.AutomationRunRequest{}, nil, fmt.Errorf("run_options must be object")
				}
			}
			payload["run_options"] = runMap
		case map[string]any:
			// keep as-is
		default:
			return service.AutomationRunRequest{}, nil, fmt.Errorf("run_options must be object")
		}
	}
	if runMap, ok := payload["run_options"].(map[string]any); ok {
		if rawContinue, exists := payload["continue_from_module"]; exists {
			if _, has := runMap["continue_from_module"]; !has {
				switch typed := rawContinue.(type) {
				case bool:
					runMap["continue_from_module"] = typed
				case string:
					runMap["continue_from_module"] = parseBoolValue(typed, false)
				default:
					runMap["continue_from_module"] = parseBoolValue(fmt.Sprintf("%v", typed), false)
				}
			}
		}
	}
	req, err := service.ParseRemoteRunPayload(payload)
	if err != nil {
		return service.AutomationRunRequest{}, nil, err
	}
	return req, payload, nil
}

func listLocalExecutionSummary() map[string]any {
	type row struct {
		Status string
		Count  int64
	}
	rows := make([]row, 0)
	_ = model.DB.Model(&model.AutomationExecution{}).
		Select("status as status, count(*) as count").
		Group("status").
		Scan(&rows).Error
	statusMap := map[string]int64{}
	for _, item := range rows {
		status := strings.TrimSpace(strings.ToLower(item.Status))
		if status == "" {
			status = constant.AutomationExecutionStatusQueued
		}
		statusMap[status] = item.Count
	}
	var runningCount int64
	_ = model.DB.Model(&model.AutomationExecution{}).
		Where("status IN ?", []string{constant.AutomationExecutionStatusRunning, constant.AutomationExecutionStatusStopping}).
		Count(&runningCount).Error
	return map[string]any{
		"by_status": statusMap,
		"running":   runningCount,
	}
}

func getAutomationArtifactUsage(root string) map[string]any {
	root = strings.TrimSpace(root)
	usage := map[string]any{
		"artifact_root": root,
		"files":         0,
		"size_bytes":    int64(0),
		"exists":        false,
	}
	if root == "" {
		return usage
	}
	if _, err := os.Stat(root); err != nil {
		return usage
	}
	usage["exists"] = true
	files := 0
	totalSize := int64(0)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		files++
		totalSize += info.Size()
		return nil
	})
	usage["files"] = files
	usage["size_bytes"] = totalSize
	return usage
}

func GetTeamAutomationSettings(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	common.ApiSuccess(c, buildAutomationSettingsResponse(settings, canConfigureAutomation(c)))
}

func SaveTeamAutomationSettings(c *gin.Context) {
	if !canConfigureAutomation(c) {
		common.ApiErrorMsg(c, "no permission to configure automation")
		return
	}
	var req automationSettingsRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	settings := buildAutomationSettingsFromRequest(req)
	if err := service.SaveAutomationSettings(settings); err != nil {
		common.ApiError(c, err)
		return
	}
	saved := service.LoadAutomationSettings()
	common.ApiSuccess(c, buildAutomationSettingsResponse(saved, true))
}

func TestTeamAutomationConnection(c *gin.Context) {
	var req automationSettingsRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	settings := buildAutomationSettingsFromRequest(req)
	testType := "engine"
	if req.TestType != nil {
		testType = strings.TrimSpace(strings.ToLower(*req.TestType))
	}
	result, err := service.TestAutomationConnectivity(c.Request.Context(), settings, testType)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetTeamAutomationMeta(c *gin.Context) {
	meta := service.BuildAutomationMeta(canConfigureAutomation(c))
	common.ApiSuccess(c, meta)
}

func GetTeamAutomationModules(c *gin.Context) {
	modules := service.GetAutomationModuleDefinitions()
	common.ApiSuccess(c, gin.H{
		"items": modules,
		"total": len(modules),
	})
}

func GetTeamAutomationCapabilities(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	meta := service.BuildAutomationMeta(canConfigureAutomation(c))
	mode := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", meta["mode"])))
	enabled := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", meta["enabled"]))) == "true"
	configured := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", meta["configured"]))) == "true"
	reachable := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", meta["reachable"]))) == "true"
	warning := strings.TrimSpace(fmt.Sprintf("%v", meta["warning"]))

	supports := map[string]bool{
		"module":           true,
		"account_filter":   true,
		"redeem_site":      true,
		"run_options":      true,
		"team_pool":        true,
		"team_import":      true,
		"team_auto_select": true,
	}
	if capabilityMap, ok := meta["capabilities"].(map[string]any); ok {
		for key, value := range capabilityMap {
			lowerKey := strings.TrimSpace(strings.ToLower(key))
			if lowerKey == "" {
				continue
			}
			supports[lowerKey] = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value))) == "true"
		}
	}

	remoteStatus := 0
	if value := strings.TrimSpace(fmt.Sprintf("%v", meta["remote_status"])); value != "" && value != "<nil>" {
		parsed, _ := strconv.Atoi(value)
		remoteStatus = parsed
	}

	result := map[string]any{
		"source":            "local",
		"enabled":           enabled,
		"mode":              mode,
		"configured":        configured,
		"reachable":         reachable,
		"warning":           warning,
		"base_url":          settings.WorkerBaseURL,
		"resolved_base_url": settings.WorkerBaseURL,
		"modules":           listAutomationModuleOptions(),
		"redeem_sites":      listAutomationRedeemSites(),
		"supports":          supports,
		"remote_status":     remoteStatus,
	}

	if mode == constant.AutomationModeDisabled || !enabled {
		if strings.TrimSpace(fmt.Sprintf("%v", result["warning"])) == "" {
			result["warning"] = "team automation connector disabled"
		}
		result["error_code"] = teamAutomationErrorDisabled
		result["action"] = teamAutomationSettingsTabPath
		common.ApiSuccess(c, result)
		return
	}

	if mode != constant.AutomationModeRemote {
		common.ApiSuccess(c, result)
		return
	}

	if !configured || strings.TrimSpace(settings.WorkerBaseURL) == "" {
		result["warning"] = "team automation base url not configured"
		result["error_code"] = teamAutomationErrorBaseURLMissing
		result["action"] = teamAutomationSettingsTabPath
		common.ApiSuccess(c, result)
		return
	}

	client, err := buildRemoteClientBySettings(settings)
	if err != nil {
		result["warning"] = err.Error()
		result["error_code"] = teamAutomationErrorConnectorUnavailable
		result["action"] = teamAutomationSettingsTabPath
		common.ApiSuccess(c, result)
		return
	}

	payload, statusCode, err := client.Do(c.Request.Context(), http.MethodGet, "/api/capabilities", nil, nil)
	if err != nil {
		result["warning"] = err.Error()
		result["error_code"] = teamAutomationErrorConnectorUnavailable
		result["action"] = teamAutomationSettingsTabPath
		result["remote_status"] = statusCode
		result["reachable"] = false
		common.ApiSuccess(c, result)
		return
	}

	result["source"] = "remote"
	result["reachable"] = true
	result["remote_status"] = statusCode
	result["remote_path"] = "/api/capabilities"
	parsed, ok := payload.(map[string]any)
	if !ok {
		common.ApiSuccess(c, result)
		return
	}
	if nested, hasData := parsed["data"].(map[string]any); hasData && len(nested) > 0 {
		parsed = nested
	}

	if message := strings.TrimSpace(fmt.Sprintf("%v", parsed["message"])); message != "" && message != "<nil>" {
		result["message"] = message
	}

	if supports, ok := result["supports"].(map[string]bool); ok {
		if remoteSupports, exists := parsed["supports"].(map[string]any); exists {
			for key, value := range remoteSupports {
				supports[strings.TrimSpace(strings.ToLower(key))] = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value))) == "true"
			}
		}
	}

	if rawModules, exists := parsed["modules"]; exists {
		if rows, ok := rawModules.([]any); ok {
			modules := make([]map[string]string, 0, len(rows))
			for _, row := range rows {
				switch typed := row.(type) {
				case string:
					v := strings.TrimSpace(typed)
					if v != "" {
						modules = append(modules, map[string]string{"label": v, "value": v})
					}
				case map[string]any:
					label := strings.TrimSpace(fmt.Sprintf("%v", typed["label"]))
					value := strings.TrimSpace(fmt.Sprintf("%v", typed["value"]))
					if label == "" {
						label = strings.TrimSpace(fmt.Sprintf("%v", typed["name"]))
					}
					if value == "" {
						value = strings.TrimSpace(fmt.Sprintf("%v", typed["id"]))
					}
					if label == "" {
						label = value
					}
					if value == "" {
						value = label
					}
					if label != "" && value != "" {
						modules = append(modules, map[string]string{"label": label, "value": value})
					}
				}
			}
			if len(modules) > 0 {
				result["modules"] = modules
			}
		}
	}

	if rawSites, exists := parsed["redeem_sites"]; exists {
		if rows, ok := rawSites.([]any); ok {
			sites := make([]map[string]string, 0, len(rows)+1)
			hasCustom := false
			hasHelloTeam := false
			hasTeamXZ := false
			for _, row := range rows {
				switch typed := row.(type) {
				case string:
					v := normalizeTeamAutomationRedeemSite(typed)
					if v != "" {
						sites = append(sites, map[string]string{"label": v, "value": v})
						lowerV := strings.ToLower(v)
						if strings.Contains(lowerV, "helloteam.store") {
							hasHelloTeam = true
						}
						if strings.Contains(lowerV, "teamxz.store") {
							hasTeamXZ = true
						}
					}
				case map[string]any:
					label := strings.TrimSpace(fmt.Sprintf("%v", typed["label"]))
					value := normalizeTeamAutomationRedeemSite(fmt.Sprintf("%v", typed["value"]))
					if label == "" {
						label = strings.TrimSpace(fmt.Sprintf("%v", typed["name"]))
					}
					if value == "" {
						value = normalizeTeamAutomationRedeemSite(label)
					}
					if value == teamAutomationCustomRedeemSiteValue {
						hasCustom = true
					}
					lowerValue := strings.ToLower(value)
					if strings.Contains(lowerValue, "helloteam.store") {
						hasHelloTeam = true
					}
					if strings.Contains(lowerValue, "teamxz.store") {
						hasTeamXZ = true
					}
					if label != "" && value != "" {
						sites = append(sites, map[string]string{"label": label, "value": value})
					}
				}
			}
			if !hasHelloTeam {
				sites = append(sites, map[string]string{"label": "helloteam.store", "value": "helloteam.store"})
			}
			if !hasTeamXZ {
				sites = append(sites, map[string]string{"label": "teamxz.store", "value": "teamxz.store"})
			}
			if !hasCustom {
				sites = append(sites, map[string]string{"label": "自定义站点", "value": teamAutomationCustomRedeemSiteValue})
			}
			if len(sites) > 0 {
				result["redeem_sites"] = sites
			}
		}
	}
	common.ApiSuccess(c, result)
}

func GetTeamAutomationHealth(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/health", c.Request.URL.Query(), nil)
		return
	}
	common.ApiSuccess(c, gin.H{
		"status":        "ok",
		"mode":          settings.Mode,
		"artifact_root": settings.ArtifactRoot,
		"timestamp":     common.GetTimestamp(),
	})
}

func GetTeamAutomationStats(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{"account": map[string]any{}, "team": map[string]any{}, "execution": map[string]any{}}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/stats", c.Request.URL.Query(), nil)
		return
	}
	accountStats, err := model.GetAutomationAccountStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	teamStats, err := model.GetAutomationTeamStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	history, _ := service.BuildExecutionHistory(1, 20, service.AutomationExecutionHistoryFilter{})
	common.ApiSuccess(c, gin.H{
		"mode":      settings.Mode,
		"account":   accountStats,
		"team":      teamStats,
		"execution": listLocalExecutionSummary(),
		"history":   history,
	})
}

func GetTeamAutomationCacheStats(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/cache/stats", c.Request.URL.Query(), nil)
		return
	}
	common.ApiSuccess(c, getAutomationArtifactUsage(settings.ArtifactRoot))
}

func GetTeamAutomationAccountStats(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{"total": 0, "by_status": map[string]int64{}}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/accounts/stats", c.Request.URL.Query(), nil)
		return
	}
	stats, err := model.GetAutomationAccountStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetTeamAutomationAccounts(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	fallback := gin.H{"items": []any{}, "accounts": []any{}, "total": 0}
	if withRemoteReadFallback(c, settings, fallback) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/accounts", c.Request.URL.Query(), nil)
		return
	}
	page, pageSize := parsePagination(c, 1, 20, 200)
	keyword := strings.TrimSpace(c.Query("search"))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	offset := (page - 1) * pageSize
	list, total, err := model.ListAutomationAccounts(keyword, status, offset, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, toAutomationAccountPayload(item))
	}
	common.ApiSuccess(c, gin.H{
		"items":     rows,
		"accounts":  rows,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func CreateTeamAutomationAccount(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/accounts", c.Request.URL.Query(), body)
		return
	}
	var req automationAccountUpsertRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	account := &model.AutomationAccount{}
	applyAutomationAccountUpdate(account, req, true)
	if account.Email == "" {
		common.ApiErrorMsg(c, "email is required")
		return
	}
	if account.Name == "" {
		account.Name = account.Email
	}
	if _, err := model.GetAutomationAccountByEmail(account.Email); err == nil {
		common.ApiErrorMsg(c, "account email already exists")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	if err := account.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationAccountPayload(account))
}

func GetTeamAutomationAccount(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	identity := strings.TrimSpace(c.Param("identity"))
	if identity == "" {
		identity = strings.TrimSpace(c.Param("email"))
	}
	if withRemoteReadFallback(c, settings, map[string]any{}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/accounts/"+url.PathEscape(identity), c.Request.URL.Query(), nil)
		return
	}
	account, err := loadAutomationAccountByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationAccountPayload(account))
}

func UpdateTeamAutomationAccount(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	identity := strings.TrimSpace(c.Param("identity"))
	if identity == "" {
		identity = strings.TrimSpace(c.Param("email"))
	}
	if identity == "" {
		common.ApiErrorMsg(c, "identity is required")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPut, "/api/accounts/"+url.PathEscape(identity), c.Request.URL.Query(), body)
		return
	}
	var req automationAccountUpsertRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := loadAutomationAccountByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	originalEmail := account.Email
	applyAutomationAccountUpdate(account, req, false)
	if account.Email == "" {
		common.ApiErrorMsg(c, "email is required")
		return
	}
	if account.Name == "" {
		account.Name = account.Email
	}
	if account.Email != originalEmail {
		if _, err := model.GetAutomationAccountByEmail(account.Email); err == nil {
			common.ApiErrorMsg(c, "account email already exists")
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
			return
		}
	}
	if err := account.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationAccountPayload(account))
}

func DeleteTeamAutomationAccount(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	identity := strings.TrimSpace(c.Param("identity"))
	if identity == "" {
		identity = strings.TrimSpace(c.Param("email"))
	}
	if identity == "" {
		common.ApiErrorMsg(c, "identity is required")
		return
	}
	if mode == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodDelete, "/api/accounts/"+url.PathEscape(identity), c.Request.URL.Query(), nil)
		return
	}
	account, err := loadAutomationAccountByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := account.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": account.Id, "email": account.Email, "deleted": true})
}

func BatchDeleteTeamAutomationAccounts(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/accounts/batch-delete", c.Request.URL.Query(), body)
		return
	}
	var req automationBatchDeleteRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	identities, err := resolveAutomationAccountBatchDeleteIdentities(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(identities) == 0 {
		common.ApiErrorMsg(c, "no account identities provided")
		return
	}
	deleted := make([]map[string]any, 0, len(identities))
	failed := make([]string, 0)
	for _, identity := range identities {
		account, err := loadAutomationAccountByIdentity(identity)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		if err := account.Delete(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		deleted = append(deleted, gin.H{
			"id":    account.Id,
			"email": account.Email,
		})
	}
	if len(deleted) == 0 && len(failed) > 0 {
		common.ApiErrorMsg(c, strings.Join(failed, "; "))
		return
	}
	common.ApiSuccess(c, gin.H{
		"deleted":       deleted,
		"deleted_count": len(deleted),
		"failed":        failed,
		"failed_count":  len(failed),
	})
}

func BatchUpdateTeamAutomationAccounts(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/accounts/batch-update", c.Request.URL.Query(), body)
		return
	}
	var req automationAccountBatchUpdateRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	identities, err := resolveAutomationAccountBatchUpdateIdentities(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(identities) == 0 {
		common.ApiErrorMsg(c, "no account identities provided")
		return
	}
	updates := map[string]any{}
	if req.Status != nil {
		status, err := parseAutomationAccountStatusForBatchUpdate(*req.Status)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		updates["status"] = status
	}
	if req.RedeemSite != nil {
		updates["redeem_site"] = normalizeTeamAutomationRedeemSite(*req.RedeemSite)
	}
	if req.Metadata != nil {
		updates["metadata"] = strings.TrimSpace(*req.Metadata)
	}
	if len(updates) == 0 {
		common.ApiErrorMsg(c, "no update fields provided")
		return
	}
	updated := make([]map[string]any, 0, len(identities))
	failed := make([]string, 0)
	for _, identity := range identities {
		account, err := loadAutomationAccountByIdentity(identity)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		if status, ok := updates["status"].(string); ok {
			account.Status = status
		}
		if redeemSite, ok := updates["redeem_site"].(string); ok {
			account.RedeemSite = redeemSite
		}
		if metadata, ok := updates["metadata"].(string); ok && metadata != "" {
			account.Metadata = mergeAutomationBatchMetadata(account.Metadata, metadata)
		}
		if err := account.Update(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		updated = append(updated, gin.H{
			"id":          account.Id,
			"email":       account.Email,
			"status":      account.Status,
			"redeem_site": account.RedeemSite,
			"metadata":    account.Metadata,
		})
	}
	if len(updated) == 0 && len(failed) > 0 {
		common.ApiErrorMsg(c, strings.Join(failed, "; "))
		return
	}
	common.ApiSuccess(c, gin.H{
		"updated":       updated,
		"updated_count": len(updated),
		"failed":        failed,
		"failed_count":  len(failed),
	})
}

func GetTeamAutomationTeamStats(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{"total": 0, "by_status": map[string]int64{}}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/teams/stats", c.Request.URL.Query(), nil)
		return
	}
	stats, err := model.GetAutomationTeamStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetTeamAutomationTeams(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	fallback := gin.H{"items": []any{}, "teams": []any{}, "total": 0}
	if withRemoteReadFallback(c, settings, fallback) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/teams", c.Request.URL.Query(), nil)
		return
	}
	page, pageSize := parsePagination(c, 1, 20, 200)
	keyword := strings.TrimSpace(c.Query("search"))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	onlyAvailable := parseBoolValue(c.Query("only_available"), false)
	offset := (page - 1) * pageSize
	list, total, err := model.ListAutomationTeams(keyword, status, onlyAvailable, offset, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		rows = append(rows, toAutomationTeamPayload(item))
	}
	common.ApiSuccess(c, gin.H{
		"items":          rows,
		"teams":          rows,
		"total":          total,
		"page":           page,
		"page_size":      pageSize,
		"only_available": onlyAvailable,
	})
}

func GetTeamAutomationTeam(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	identity := strings.TrimSpace(c.Param("identity"))
	if withRemoteReadFallback(c, settings, map[string]any{}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/teams/"+url.PathEscape(identity), c.Request.URL.Query(), nil)
		return
	}
	team, err := loadAutomationTeamByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationTeamPayload(team))
}

func CreateTeamAutomationTeam(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/teams", c.Request.URL.Query(), body)
		return
	}
	var req automationTeamUpsertRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	team := &model.AutomationTeam{}
	applyAutomationTeamUpdate(team, req, true)
	if team.Email == "" {
		common.ApiErrorMsg(c, "email is required")
		return
	}
	if _, err := model.GetAutomationTeamByEmail(team.Email); err == nil {
		common.ApiErrorMsg(c, "team email already exists")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	if err := team.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationTeamPayload(team))
}

func UpdateTeamAutomationTeam(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	identity := strings.TrimSpace(c.Param("identity"))
	if identity == "" {
		common.ApiErrorMsg(c, "identity is required")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPut, "/api/teams/"+url.PathEscape(identity), c.Request.URL.Query(), body)
		return
	}
	var req automationTeamUpsertRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	team, err := loadAutomationTeamByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	originalEmail := team.Email
	applyAutomationTeamUpdate(team, req, false)
	if team.Email == "" {
		common.ApiErrorMsg(c, "email is required")
		return
	}
	if team.Email != originalEmail {
		if _, err := model.GetAutomationTeamByEmail(team.Email); err == nil {
			common.ApiErrorMsg(c, "team email already exists")
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
			return
		}
	}
	if err := team.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAutomationTeamPayload(team))
}

func DeleteTeamAutomationTeam(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	identity := strings.TrimSpace(c.Param("identity"))
	if identity == "" {
		common.ApiErrorMsg(c, "identity is required")
		return
	}
	if mode == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodDelete, "/api/teams/"+url.PathEscape(identity), c.Request.URL.Query(), nil)
		return
	}
	team, err := loadAutomationTeamByIdentity(identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := team.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": team.Id, "email": team.Email, "deleted": true})
}

func BatchDeleteTeamAutomationTeams(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/teams/batch-delete", c.Request.URL.Query(), body)
		return
	}
	var req automationBatchDeleteRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	identities, err := resolveAutomationTeamBatchDeleteIdentities(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(identities) == 0 {
		common.ApiErrorMsg(c, "no team identities provided")
		return
	}
	deleted := make([]map[string]any, 0, len(identities))
	failed := make([]string, 0)
	for _, identity := range identities {
		team, err := loadAutomationTeamByIdentity(identity)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		if err := team.Delete(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		deleted = append(deleted, gin.H{
			"id":    team.Id,
			"email": team.Email,
		})
	}
	if len(deleted) == 0 && len(failed) > 0 {
		common.ApiErrorMsg(c, strings.Join(failed, "; "))
		return
	}
	common.ApiSuccess(c, gin.H{
		"deleted":       deleted,
		"deleted_count": len(deleted),
		"failed":        failed,
		"failed_count":  len(failed),
	})
}

func BatchUpdateTeamAutomationTeams(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/teams/batch-update", c.Request.URL.Query(), body)
		return
	}
	var req automationTeamBatchUpdateRequest
	if err := decodeJSONBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	identities, err := resolveAutomationTeamBatchUpdateIdentities(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(identities) == 0 {
		common.ApiErrorMsg(c, "no team identities provided")
		return
	}
	if req.Status == nil && req.Metadata == nil {
		common.ApiErrorMsg(c, "no update fields provided")
		return
	}
	status := ""
	if req.Status != nil {
		parsedStatus, err := parseAutomationTeamStatusForBatchUpdate(*req.Status)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		status = parsedStatus
	}
	metadata := ""
	if req.Metadata != nil {
		metadata = strings.TrimSpace(*req.Metadata)
	}
	updated := make([]map[string]any, 0, len(identities))
	failed := make([]string, 0)
	for _, identity := range identities {
		team, err := loadAutomationTeamByIdentity(identity)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		if status != "" {
			team.Status = status
		}
		if metadata != "" {
			team.Metadata = mergeAutomationBatchMetadata(team.Metadata, metadata)
		}
		if err := team.Update(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", identity, err))
			continue
		}
		updated = append(updated, gin.H{
			"id":       team.Id,
			"email":    team.Email,
			"status":   team.Status,
			"metadata": team.Metadata,
		})
	}
	if len(updated) == 0 && len(failed) > 0 {
		common.ApiErrorMsg(c, strings.Join(failed, "; "))
		return
	}
	common.ApiSuccess(c, gin.H{
		"updated":       updated,
		"updated_count": len(updated),
		"failed":        failed,
		"failed_count":  len(failed),
	})
}

func ImportTeamAutomationTeams(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/teams/import", c.Request.URL.Query(), body)
		return
	}
	body, err := decodeJSONAnyBody(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	records, err := parseAutomationTeamImportPayload(body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(records) == 0 {
		common.ApiErrorMsg(c, "no import records provided")
		return
	}
	result, err := service.ImportAutomationTeams(records)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AutoSelectTeamAutomationTeam(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, map[string]any{"selected": nil, "candidates": []any{}}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/teams/auto-select", c.Request.URL.Query(), body)
		return
	}
	req := automationTeamAutoSelectRequest{}
	_ = decodeJSONBody(c, &req)
	selected, candidates, err := service.SelectAutomationTeam(req.ExcludeTeamIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows := make([]map[string]any, 0, len(candidates))
	for _, item := range candidates {
		rows = append(rows, toAutomationTeamPayload(item))
	}
	result := gin.H{
		"selected":   nil,
		"candidates": rows,
		"total":      len(rows),
	}
	if selected != nil {
		result["selected"] = toAutomationTeamPayload(selected)
	}
	common.ApiSuccess(c, result)
}

func ExportTeamAutomationAccountsCSV(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, "") {
		common.ApiErrorMsg(c, "automation remote mode is not configured")
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		client, err := buildRemoteClientBySettings(settings)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		download, err := client.Download(c.Request.Context(), "/api/accounts/export/csv", c.Request.URL.Query())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		contentType := strings.TrimSpace(download.ContentType)
		if contentType == "" {
			contentType = "text/csv; charset=utf-8"
		}
		if disposition := strings.TrimSpace(download.ContentDisposition); disposition != "" {
			c.Header("Content-Disposition", disposition)
		}
		filtered := download.Body
		statusFilter := strings.TrimSpace(strings.ToLower(c.Query("status")))
		searchFilter := strings.TrimSpace(c.Query("search"))
		if statusFilter != "" || searchFilter != "" {
			filteredCSV, err := filterTeamAutomationAccountsCSV(download.Body, statusFilter, searchFilter)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			filtered = filteredCSV
		}
		c.Data(http.StatusOK, contentType, filtered)
		return
	}

	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	keyword := strings.TrimSpace(c.Query("search"))
	accounts, err := model.ListAutomationAccountsByFilter(status, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	header := []string{
		"id", "name", "email", "redeem_code", "redeem_type", "status", "failure_reason",
		"cloudflare_domain", "cloudflare_alias", "forward_target", "redeem_site", "invite_link", "invite_code",
		"mailbox_id", "session_artifact_path", "auth_artifact_path", "imported_target", "imported_reference",
		"metadata", "updated_time", "created_time",
	}
	if err := writer.Write(header); err != nil {
		common.ApiError(c, err)
		return
	}
	for _, item := range accounts {
		if item == nil {
			continue
		}
		row := []string{
			strconv.Itoa(item.Id),
			item.Name,
			item.Email,
			item.RedeemCode,
			item.RedeemType,
			item.Status,
			item.FailureReason,
			item.CloudflareDomain,
			item.CloudflareAlias,
			item.ForwardTarget,
			item.RedeemSite,
			item.InviteLink,
			item.InviteCode,
			strconv.Itoa(item.MailboxId),
			item.SessionArtifactPath,
			item.AuthArtifactPath,
			item.ImportedTarget,
			item.ImportedReference,
			item.Metadata,
			strconv.FormatInt(item.UpdatedTime, 10),
			strconv.FormatInt(item.CreatedTime, 10),
		}
		if err := writer.Write(row); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=automation_accounts.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

func ExportTeamAutomationTeamsCSV(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	if withRemoteReadFallback(c, settings, "") {
		common.ApiErrorMsg(c, "automation remote mode is not configured")
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		client, err := buildRemoteClientBySettings(settings)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		download, err := client.Download(c.Request.Context(), "/api/teams/export/csv", c.Request.URL.Query())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		contentType := strings.TrimSpace(download.ContentType)
		if contentType == "" {
			contentType = "text/csv; charset=utf-8"
		}
		if disposition := strings.TrimSpace(download.ContentDisposition); disposition != "" {
			c.Header("Content-Disposition", disposition)
		}
		c.Data(http.StatusOK, contentType, download.Body)
		return
	}

	pageStatus := strings.TrimSpace(strings.ToLower(c.Query("status")))
	keyword := strings.TrimSpace(c.Query("search"))
	teams, _, err := model.ListAutomationTeams(keyword, pageStatus, false, 0, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	header := []string{
		"id", "email", "account_id", "client_id", "team_name", "plan_type", "subscription_plan",
		"expires_at", "current_members", "pending_invites", "max_members", "occupied_slots", "remaining_slots",
		"status", "account_role", "device_code_auth_enabled", "error_count", "last_sync_time",
		"metadata", "updated_time", "created_time",
	}
	if err := writer.Write(header); err != nil {
		common.ApiError(c, err)
		return
	}
	for _, item := range teams {
		if item == nil {
			continue
		}
		row := []string{
			strconv.Itoa(item.Id),
			item.Email,
			item.AccountID,
			item.ClientID,
			item.TeamName,
			item.PlanType,
			item.SubscriptionPlan,
			strconv.FormatInt(item.ExpiresAt, 10),
			strconv.Itoa(item.CurrentMembers),
			strconv.Itoa(item.PendingInvites),
			strconv.Itoa(item.MaxMembers),
			strconv.Itoa(item.OccupiedSlots()),
			strconv.Itoa(item.RemainingSlots()),
			item.Status,
			item.AccountRole,
			strconv.FormatBool(item.DeviceCodeAuthEnabled),
			strconv.Itoa(item.ErrorCount),
			strconv.FormatInt(item.LastSyncTime, 10),
			item.Metadata,
			strconv.FormatInt(item.UpdatedTime, 10),
			strconv.FormatInt(item.CreatedTime, 10),
		}
		if err := writer.Write(row); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=automation_teams.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

func parseRegisteredImportOptionsFromContext(c *gin.Context, payload map[string]any, defaultStartFrom string) automationRegisteredImportOptions {
	options := automationRegisteredImportOptions{
		StartFrom:          normalizeAutomationStartFrom(defaultStartFrom),
		ContinueFromModule: false,
		AutoRun:            false,
		RunOptions:         map[string]any{},
	}
	if options.StartFrom == "" {
		options.StartFrom = constant.AutomationModuleTeamRedeem
	}

	if queryStart := normalizeAutomationStartFrom(strings.TrimSpace(c.Query("start_from"))); queryStart != "" {
		options.StartFrom = queryStart
	}
	if payload != nil {
		if startFrom := normalizeAutomationStartFrom(strings.TrimSpace(fmt.Sprintf("%v", payload["start_from"]))); startFrom != "" {
			options.StartFrom = startFrom
		}
	}

	defaultSite := normalizeTeamAutomationRedeemSite(strings.TrimSpace(c.Query("default_redeem_site")))
	if defaultSite == "" {
		defaultSite = normalizeTeamAutomationRedeemSite(strings.TrimSpace(c.Query("redeem_site")))
	}
	if payload != nil {
		if defaultSite == "" {
			defaultSite = normalizeTeamAutomationRedeemSite(strings.TrimSpace(fmt.Sprintf("%v", payload["default_redeem_site"])))
		}
		if defaultSite == "" {
			defaultSite = normalizeTeamAutomationRedeemSite(strings.TrimSpace(fmt.Sprintf("%v", payload["redeem_site"])))
		}
	}
	options.DefaultRedeemSite = defaultSite

	defaultMailboxID := 0
	if mailboxRaw := strings.TrimSpace(c.Query("default_mailbox_id")); mailboxRaw != "" {
		if mailboxID, err := strconv.Atoi(mailboxRaw); err == nil {
			defaultMailboxID = mailboxID
		}
	}
	if payload != nil && defaultMailboxID <= 0 {
		if mailboxID, err := strconv.Atoi(strings.TrimSpace(fmt.Sprintf("%v", payload["default_mailbox_id"]))); err == nil {
			defaultMailboxID = mailboxID
		}
	}
	options.DefaultMailboxID = defaultMailboxID

	options.ContinueFromModule = parseBoolValue(strings.TrimSpace(c.Query("continue_from_module")), false)
	options.AutoRun = parseBoolValue(strings.TrimSpace(c.Query("auto_run")), false)

	if payload != nil {
		if value := strings.TrimSpace(fmt.Sprintf("%v", payload["continue_from_module"])); value != "" && value != "<nil>" {
			options.ContinueFromModule = parseBoolValue(value, options.ContinueFromModule)
		}
		if value := strings.TrimSpace(fmt.Sprintf("%v", payload["auto_run"])); value != "" && value != "<nil>" {
			options.AutoRun = parseBoolValue(value, options.AutoRun)
		}
		if runRaw, ok := payload["run_options"]; ok {
			if runMap, ok := runRaw.(map[string]any); ok {
				options.RunOptions = runMap
			}
		}
	}
	if options.RunOptions == nil {
		options.RunOptions = map[string]any{}
	}
	return options
}

func upsertAutomationImportRecords(records []automationImportRecord, options automationRegisteredImportOptions) (map[string]any, error) {
	importBatch := fmt.Sprintf("registered-%d", time.Now().UTC().UnixNano())
	created := 0
	updated := 0
	failed := 0
	createdIDs := make([]int, 0)
	updatedIDs := make([]int, 0)
	errorsList := make([]string, 0)

	for idx, record := range records {
		email := strings.TrimSpace(strings.ToLower(record.Email))
		if email == "" {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: email is empty", idx+2))
			continue
		}

		effectiveSite := normalizeTeamAutomationRedeemSite(record.RedeemSite)
		if effectiveSite == "" {
			effectiveSite = options.DefaultRedeemSite
		}
		status := normalizeAutomationImportStatus(record.Status, options.StartFrom)
		mailboxID := record.MailboxID
		if mailboxID <= 0 {
			mailboxID = options.DefaultMailboxID
		}

		sourceMetadata := strings.TrimSpace(record.Metadata)
		upsertMetadata := mergeAutomationImportMetadata(sourceMetadata, importBatch)
		upsert := model.AutomationAccount{
			Name:                strings.TrimSpace(record.Name),
			Email:               email,
			Password:            strings.TrimSpace(record.Password),
			RedeemCode:          strings.TrimSpace(record.RedeemCode),
			RedeemType:          strings.TrimSpace(record.RedeemType),
			Status:              status,
			FailureReason:       strings.TrimSpace(record.FailureReason),
			CloudflareDomain:    strings.TrimSpace(record.CloudflareDomain),
			CloudflareAlias:     strings.TrimSpace(record.CloudflareAlias),
			ForwardTarget:       strings.TrimSpace(record.ForwardTarget),
			RedeemSite:          effectiveSite,
			InviteLink:          strings.TrimSpace(record.InviteLink),
			InviteCode:          strings.TrimSpace(record.InviteCode),
			MailboxId:           mailboxID,
			SessionArtifactPath: strings.TrimSpace(record.SessionArtifactPath),
			AuthArtifactPath:    strings.TrimSpace(record.AuthArtifactPath),
			ImportedTarget:      strings.TrimSpace(record.ImportedTarget),
			ImportedReference:   strings.TrimSpace(record.ImportedReference),
			Metadata:            upsertMetadata,
		}
		if upsert.Name == "" {
			upsert.Name = email
		}

		existing, getErr := model.GetAutomationAccountByEmail(email)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				if err := upsert.Insert(); err != nil {
					failed++
					errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+2, err))
					continue
				}
				created++
				createdIDs = append(createdIDs, upsert.Id)
				continue
			}
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+2, getErr))
			continue
		}

		if upsert.Name != "" {
			existing.Name = upsert.Name
		}
		if upsert.Password != "" {
			existing.Password = upsert.Password
		}
		if upsert.RedeemCode != "" {
			existing.RedeemCode = upsert.RedeemCode
		}
		if upsert.RedeemType != "" {
			existing.RedeemType = upsert.RedeemType
		}
		existing.Status = status
		if upsert.FailureReason != "" {
			existing.FailureReason = upsert.FailureReason
		} else {
			existing.FailureReason = ""
		}
		if upsert.CloudflareDomain != "" {
			existing.CloudflareDomain = upsert.CloudflareDomain
		}
		if upsert.CloudflareAlias != "" {
			existing.CloudflareAlias = upsert.CloudflareAlias
		}
		if upsert.ForwardTarget != "" {
			existing.ForwardTarget = upsert.ForwardTarget
		}
		if upsert.RedeemSite != "" {
			existing.RedeemSite = upsert.RedeemSite
		}
		if upsert.InviteLink != "" {
			existing.InviteLink = upsert.InviteLink
		}
		if upsert.InviteCode != "" {
			existing.InviteCode = upsert.InviteCode
		}
		if upsert.MailboxId > 0 {
			existing.MailboxId = upsert.MailboxId
		}
		if upsert.SessionArtifactPath != "" {
			existing.SessionArtifactPath = upsert.SessionArtifactPath
		}
		if upsert.AuthArtifactPath != "" {
			existing.AuthArtifactPath = upsert.AuthArtifactPath
		}
		if upsert.ImportedTarget != "" {
			existing.ImportedTarget = upsert.ImportedTarget
		}
		if upsert.ImportedReference != "" {
			existing.ImportedReference = upsert.ImportedReference
		}
		if upsert.Metadata != "" {
			existing.Metadata = mergeAutomationImportMetadata(upsert.Metadata, importBatch)
		} else {
			existing.Metadata = mergeAutomationImportMetadata(existing.Metadata, importBatch)
		}

		if err := existing.Update(); err != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", idx+2, err))
			continue
		}
		updated++
		updatedIDs = append(updatedIDs, existing.Id)
	}

	sort.Strings(errorsList)
	if len(errorsList) > 50 {
		errorsList = errorsList[:50]
	}
	result := map[string]any{
		"created":      created,
		"updated":      updated,
		"failed":       failed,
		"errors":       errorsList,
		"import_batch": importBatch,
		"start_from":   options.StartFrom,
		"status_set":   defaultAutomationStatusByStartFrom(options.StartFrom),
		"created_ids":  createdIDs,
		"updated_ids":  updatedIDs,
	}
	return result, nil
}

func maybeStartAutomationRunForImport(mode string, options automationRegisteredImportOptions, importResult map[string]any) {
	if !options.AutoRun {
		return
	}
	if strings.TrimSpace(strings.ToLower(mode)) != constant.AutomationModeLocal {
		importResult["auto_run"] = map[string]any{
			"queued": false,
			"reason": "auto_run only supported in local mode",
		}
		return
	}
	importBatch := strings.TrimSpace(fmt.Sprintf("%v", importResult["import_batch"]))
	if importBatch == "" {
		importResult["auto_run"] = map[string]any{
			"queued": false,
			"reason": "import batch tag missing",
		}
		return
	}
	settings := service.LoadAutomationSettings()
	runOptions := map[string]any{}
	for key, value := range options.RunOptions {
		runOptions[key] = value
	}
	if options.ContinueFromModule {
		runOptions["continue_from_module"] = true
	}
	req := service.AutomationRunRequest{
		Module:        options.StartFrom,
		AccountStatus: normalizeAutomationImportStatus("", options.StartFrom),
		Search:        importBatch,
		RunOptions:    runOptions,
	}
	execution, err := service.StartLocalAutomationRun(req, settings)
	if err != nil {
		importResult["auto_run"] = map[string]any{
			"queued": false,
			"error":  err.Error(),
		}
		return
	}
	importResult["auto_run"] = map[string]any{
		"queued":       true,
		"execution_id": execution.ExecutionID,
		"module":       req.Module,
		"search":       req.Search,
	}
}

func ImportTeamAutomationAccountsCSV(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "file is required")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer src.Close()

	raw, err := io.ReadAll(io.LimitReader(src, teamAutomationImportMaxBytes+1))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(raw) > teamAutomationImportMaxBytes {
		common.ApiErrorMsg(c, "csv file too large")
		return
	}

	if mode == constant.AutomationModeRemote {
		client, err := buildRemoteClientBySettings(settings)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		name := strings.TrimSpace(fileHeader.Filename)
		if name == "" {
			name = "accounts.csv"
		}
		payload, _, err := client.Upload(c.Request.Context(), "/api/accounts/import/csv", c.Request.URL.Query(), "file", name, raw)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, payload)
		return
	}

	records, err := parseAutomationImportCSVRecords(raw)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	options := parseRegisteredImportOptionsFromContext(c, nil, constant.AutomationModuleCloudflareEmailGenerate)
	result, err := upsertAutomationImportRecords(records, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maybeStartAutomationRunForImport(mode, options, result)
	common.ApiSuccess(c, result)
}

func ImportRegisteredTeamAutomationAccounts(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}

	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if body == nil {
			body = map[string]any{}
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/accounts/import/registered", c.Request.URL.Query(), body)
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	var (
		records []automationImportRecord
		options automationRegisteredImportOptions
		err     error
	)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		fileHeader, fileErr := c.FormFile("file")
		if fileErr != nil {
			common.ApiErrorMsg(c, "file is required")
			return
		}
		src, openErr := fileHeader.Open()
		if openErr != nil {
			common.ApiError(c, openErr)
			return
		}
		defer src.Close()
		raw, readErr := io.ReadAll(io.LimitReader(src, teamAutomationImportMaxBytes+1))
		if readErr != nil {
			common.ApiError(c, readErr)
			return
		}
		if len(raw) > teamAutomationImportMaxBytes {
			common.ApiErrorMsg(c, "import file too large")
			return
		}
		records, err = parseAutomationImportCSVRecords(raw)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		options = parseRegisteredImportOptionsFromContext(c, nil, constant.AutomationModuleTeamRedeem)
	} else {
		body, decodeErr := decodeJSONAnyBody(c)
		if decodeErr != nil {
			common.ApiError(c, decodeErr)
			return
		}
		payloadMap := map[string]any{}
		if typed, ok := body.(map[string]any); ok {
			payloadMap = typed
		}
		options = parseRegisteredImportOptionsFromContext(c, payloadMap, constant.AutomationModuleTeamRedeem)
		records, err = parseAutomationImportJSONRecords(body)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	result, err := upsertAutomationImportRecords(records, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	created := int64(0)
	updated := int64(0)
	if value, ok := result["created"].(int); ok {
		created = int64(value)
	}
	if value, ok := result["updated"].(int); ok {
		updated = int64(value)
	}
	result["run_hint"] = map[string]any{
		"module":               options.StartFrom,
		"account_status":       defaultAutomationStatusByStartFrom(options.StartFrom),
		"search":               result["import_batch"],
		"continue_from_module": options.ContinueFromModule,
		"can_start":            created+updated > 0,
	}
	maybeStartAutomationRunForImport(mode, options, result)
	common.ApiSuccess(c, result)
}

func ImportTeamAutomationRedeemCodes(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}

	if mode == constant.AutomationModeRemote {
		body, err := decodeJSONAnyBody(c)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if body == nil {
			body = map[string]any{}
		}
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/accounts/import/redeem-codes", c.Request.URL.Query(), body)
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	var (
		items   []automationRedeemCodeImportItem
		options automationRegisteredImportOptions
		err     error
	)

	if strings.HasPrefix(contentType, "multipart/form-data") {
		fileHeader, fileErr := c.FormFile("file")
		if fileErr != nil {
			common.ApiErrorMsg(c, "file is required")
			return
		}
		src, openErr := fileHeader.Open()
		if openErr != nil {
			common.ApiError(c, openErr)
			return
		}
		defer src.Close()
		raw, readErr := io.ReadAll(io.LimitReader(src, teamAutomationImportMaxBytes+1))
		if readErr != nil {
			common.ApiError(c, readErr)
			return
		}
		if len(raw) > teamAutomationImportMaxBytes {
			common.ApiErrorMsg(c, "import file too large")
			return
		}
		items = parseRedeemCodeImportItemsFromText(string(raw))
		options = parseRegisteredImportOptionsFromContext(c, nil, constant.AutomationModuleTeamRedeem)
	} else {
		bodyAny, decodeErr := decodeJSONAnyBody(c)
		if decodeErr != nil {
			common.ApiError(c, decodeErr)
			return
		}
		payloadMap := map[string]any{}
		if typed, ok := bodyAny.(map[string]any); ok {
			payloadMap = typed
		}
		items, err = parseRedeemCodeImportItems(bodyAny)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		options = parseRegisteredImportOptionsFromContext(c, payloadMap, constant.AutomationModuleTeamRedeem)
	}

	if len(items) == 0 {
		common.ApiErrorMsg(c, "没有可导入的兑换码数据")
		return
	}
	result, err := upsertAutomationRedeemCodes(items, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	created := int64(0)
	updated := int64(0)
	if value, ok := result["created"].(int); ok {
		created = int64(value)
	}
	if value, ok := result["updated"].(int); ok {
		updated = int64(value)
	}
	result["run_hint"] = map[string]any{
		"module":               options.StartFrom,
		"account_status":       defaultAutomationStatusByStartFrom(options.StartFrom),
		"search":               result["import_batch"],
		"continue_from_module": options.ContinueFromModule,
		"can_start":            created+updated > 0,
	}
	maybeStartAutomationRunForImport(mode, options, result)
	common.ApiSuccess(c, result)
}

func StartTeamAutomationRun(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	req, rawPayload, err := decodeAutomationRunRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if mode == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/run", c.Request.URL.Query(), rawPayload)
		return
	}
	execution, err := service.StartLocalAutomationRun(req, settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail := service.BuildExecutionDetail(execution)
	common.ApiSuccess(c, detail)
}

func GetTeamAutomationRunHistory(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	fallback := gin.H{"items": []any{}, "total": 0, "page": 1, "page_size": 20}
	if withRemoteReadFallback(c, settings, fallback) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/run/history", c.Request.URL.Query(), nil)
		return
	}
	page, pageSize := parsePagination(c, 1, 20, 200)
	filter := service.AutomationExecutionHistoryFilter{
		Status:           strings.TrimSpace(strings.ToLower(c.Query("status"))),
		Module:           normalizeAutomationModuleID(c.Query("module")),
		Search:           strings.TrimSpace(c.Query("search")),
		OnlyFailed:       parseBoolValue(c.Query("only_failed"), false),
		OnlyProxyRetried: parseBoolValue(c.Query("only_proxy_retried"), false),
	}
	payload, err := service.BuildExecutionHistory(page, pageSize, filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func GetTeamAutomationRun(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		common.ApiErrorMsg(c, "execution id is required")
		return
	}
	if withRemoteReadFallback(c, settings, map[string]any{}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/run/"+url.PathEscape(executionID), c.Request.URL.Query(), nil)
		return
	}
	execution, err := model.GetAutomationExecutionByExecutionID(executionID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail := service.BuildExecutionDetail(execution)
	if summaryRaw, ok := detail["result_summary"].(string); ok {
		detail["result_summary"] = service.ParseExecutionResultSummary(summaryRaw)
		detail["result_summary_raw"] = summaryRaw
	}
	common.ApiSuccess(c, detail)
}

func GetTeamAutomationRunLogs(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		common.ApiErrorMsg(c, "execution id is required")
		return
	}
	if withRemoteReadFallback(c, settings, map[string]any{"items": []any{}, "total": 0}) {
		return
	}
	if strings.TrimSpace(strings.ToLower(settings.Mode)) == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodGet, "/api/run/"+url.PathEscape(executionID)+"/logs", c.Request.URL.Query(), nil)
		return
	}
	page, pageSize := parsePagination(c, 1, 200, 2000)
	payload, err := service.BuildExecutionLogs(executionID, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func BatchDeleteTeamAutomationRuns(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	var req automationRunBatchDeleteRequest
	rawPayload, err := decodeJSONAnyBody(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if rawPayload != nil {
		bodyBytes, marshalErr := common.Marshal(rawPayload)
		if marshalErr != nil {
			common.ApiError(c, marshalErr)
			return
		}
		if unmarshalErr := common.Unmarshal(bodyBytes, &req); unmarshalErr != nil {
			common.ApiError(c, unmarshalErr)
			return
		}
	}
	if mode == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/run/batch-delete", c.Request.URL.Query(), rawPayload)
		return
	}
	executionIDs, err := resolveAutomationRunDeleteExecutionIDs(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(executionIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":             0,
			"matched_count":               0,
			"deleted_count":               0,
			"skipped_active_count":        0,
			"skipped_missing_count":       0,
			"deleted_artifact_file_count": 0,
		})
		return
	}

	executions, err := model.ListAutomationExecutionsByExecutionIDs(executionIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	executionMap := make(map[string]*model.AutomationExecution, len(executions))
	for _, execution := range executions {
		if execution == nil {
			continue
		}
		executionMap[strings.TrimSpace(execution.ExecutionID)] = execution
	}

	missingExecutionIDs := make([]string, 0)
	activeExecutionIDs := make([]string, 0)
	deletableExecutionIDs := make([]string, 0)
	for _, executionID := range executionIDs {
		execution := executionMap[executionID]
		if execution == nil {
			missingExecutionIDs = append(missingExecutionIDs, executionID)
			continue
		}
		if !isAutomationExecutionDeletableStatus(execution.Status) {
			activeExecutionIDs = append(activeExecutionIDs, executionID)
			continue
		}
		deletableExecutionIDs = append(deletableExecutionIDs, executionID)
	}
	sort.Strings(missingExecutionIDs)
	sort.Strings(activeExecutionIDs)
	if len(deletableExecutionIDs) == 0 {
		common.ApiSuccess(c, gin.H{
			"requested_count":               len(executionIDs),
			"matched_count":                 len(executionIDs) - len(missingExecutionIDs),
			"deleted_count":                 0,
			"deleted_execution_ids":         []string{},
			"skipped_active_count":          len(activeExecutionIDs),
			"skipped_active_execution_ids":  activeExecutionIDs,
			"skipped_missing_count":         len(missingExecutionIDs),
			"skipped_missing_execution_ids": missingExecutionIDs,
			"deleted_artifact_file_count":   0,
		})
		return
	}

	artifacts := make([]*model.AutomationArtifact, 0)
	if err := model.DB.Where("execution_id IN ?", deletableExecutionIDs).Find(&artifacts).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	artifactPaths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		artifactPaths = append(artifactPaths, artifact.Path)
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	if err := tx.Where("execution_id IN ?", deletableExecutionIDs).Delete(&model.AutomationExecutionLog{}).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}
	if err := tx.Where("execution_id IN ?", deletableExecutionIDs).Delete(&model.AutomationArtifact{}).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}
	if err := tx.Where("execution_id IN ?", deletableExecutionIDs).Delete(&model.AutomationExecution{}).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}

	deletedArtifactFileCount := 0
	artifactDeleteErrors := []string{}
	if req.DeleteArtifactFiles {
		deletedArtifactFileCount, artifactDeleteErrors = deleteAutomationArtifactFiles(settings.ArtifactRoot, artifactPaths)
	}

	common.ApiSuccess(c, gin.H{
		"requested_count":                 len(executionIDs),
		"matched_count":                   len(executionIDs) - len(missingExecutionIDs),
		"deleted_count":                   len(deletableExecutionIDs),
		"deleted_execution_ids":           deletableExecutionIDs,
		"skipped_active_count":            len(activeExecutionIDs),
		"skipped_active_execution_ids":    activeExecutionIDs,
		"skipped_missing_count":           len(missingExecutionIDs),
		"skipped_missing_execution_ids":   missingExecutionIDs,
		"deleted_artifact_file_count":     deletedArtifactFileCount,
		"artifact_delete_error_count":     len(artifactDeleteErrors),
		"artifact_delete_errors":          artifactDeleteErrors,
		"delete_artifact_files_requested": req.DeleteArtifactFiles,
	})
}

func StopTeamAutomationRun(c *gin.Context) {
	settings := service.LoadAutomationSettings()
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		common.ApiErrorMsg(c, "execution id is required")
		return
	}
	mode := strings.TrimSpace(strings.ToLower(settings.Mode))
	if mode == constant.AutomationModeDisabled {
		common.ApiErrorMsg(c, "automation is disabled")
		return
	}
	if mode == constant.AutomationModeRemote {
		proxyRemoteTeamAutomation(c, http.MethodPost, "/api/run/"+url.PathEscape(executionID)+"/stop", c.Request.URL.Query(), nil)
		return
	}
	_ = model.DB.Model(&model.AutomationExecution{}).
		Where("execution_id = ? AND status = ?", executionID, constant.AutomationExecutionStatusRunning).
		Updates(map[string]any{
			"status":        constant.AutomationExecutionStatusStopping,
			"updated_at_ts": common.GetTimestamp(),
		}).Error
	if err := service.StopLocalAutomationRun(executionID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"execution_id": executionID,
		"status":       constant.AutomationExecutionStatusStopping,
		"stopped":      true,
	})
}
