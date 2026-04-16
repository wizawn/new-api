package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/playwright-community/playwright-go"
	"gorm.io/gorm"
)

type AutomationRunRequest struct {
	Module        string         `json:"module"`
	AccountStatus string         `json:"account_status"`
	Search        string         `json:"search"`
	RedeemSite    string         `json:"redeem_site"`
	RunOptions    map[string]any `json:"run_options"`
}

type automationRunHandle struct {
	cancel context.CancelFunc
}

type cloudflareAuthAttempt struct {
	Mode string
}

type tempMailCreateResult struct {
	Address   string
	JWT       string
	AddressID int
	Password  string
}

var (
	automationRunMu      sync.Mutex
	automationRunHandles = map[string]*automationRunHandle{}
	automationURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)
	playwrightInstallMu  sync.Mutex
	cloudflareZoneCache  sync.Map
)

func isAutomationRunning(executionID string) bool {
	automationRunMu.Lock()
	defer automationRunMu.Unlock()
	_, ok := automationRunHandles[strings.TrimSpace(executionID)]
	return ok
}

func registerAutomationRun(executionID string, cancel context.CancelFunc) {
	automationRunMu.Lock()
	defer automationRunMu.Unlock()
	automationRunHandles[strings.TrimSpace(executionID)] = &automationRunHandle{cancel: cancel}
}

func unregisterAutomationRun(executionID string) {
	automationRunMu.Lock()
	defer automationRunMu.Unlock()
	delete(automationRunHandles, strings.TrimSpace(executionID))
}

func StopLocalAutomationRun(executionID string) error {
	automationRunMu.Lock()
	handle, ok := automationRunHandles[strings.TrimSpace(executionID)]
	automationRunMu.Unlock()
	if !ok || handle == nil || handle.cancel == nil {
		execution, err := model.GetAutomationExecutionByExecutionID(executionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("execution not found")
			}
			return err
		}
		if execution.Status == constant.AutomationExecutionStatusCompleted || execution.Status == constant.AutomationExecutionStatusFailed || execution.Status == constant.AutomationExecutionStatusStopped {
			return nil
		}
		return fmt.Errorf("execution is not running")
	}
	handle.cancel()
	return nil
}

func randomExecutionID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("auto-%s-%04d", now.Format("20060102T150405"), rand.Intn(10000))
}

func appendAutomationLog(executionID string, accountID int, level string, step string, message string, payload any) {
	entry := &model.AutomationExecutionLog{
		ExecutionID: strings.TrimSpace(executionID),
		AccountID:   accountID,
		Level:       strings.TrimSpace(strings.ToLower(level)),
		Step:        strings.TrimSpace(strings.ToLower(step)),
		Message:     strings.TrimSpace(message),
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	if payload != nil {
		if data, err := common.Marshal(payload); err == nil {
			entry.Payload = string(data)
		}
	}
	if err := entry.Insert(); err != nil {
		common.SysError("failed to insert automation log: " + err.Error())
	}
}

func updateAutomationExecution(execution *model.AutomationExecution, updates map[string]any) {
	if execution == nil {
		return
	}
	if len(updates) == 0 {
		return
	}
	updates["updated_at_ts"] = common.GetTimestamp()
	if err := model.DB.Model(&model.AutomationExecution{}).Where("execution_id = ?", execution.ExecutionID).Updates(updates).Error; err != nil {
		common.SysError("failed to update automation execution: " + err.Error())
	}
	if status, ok := updates["status"].(string); ok {
		execution.Status = status
	}
	if progress, ok := updates["progress"].(int); ok {
		execution.Progress = progress
	}
	if successCount, ok := updates["success_count"].(int); ok {
		execution.SuccessCount = successCount
	}
	if failedCount, ok := updates["failed_count"].(int); ok {
		execution.FailedCount = failedCount
	}
}

func updateAutomationAccountStatus(accountID int, status string, failureReason string) {
	updates := map[string]any{
		"status":         strings.TrimSpace(strings.ToLower(status)),
		"failure_reason": strings.TrimSpace(failureReason),
		"updated_time":   common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", accountID).Updates(updates).Error; err != nil {
		common.SysError("failed to update automation account status: " + err.Error())
	}
}

func mergeExecutionSummary(summary map[string]any, key string, value any) map[string]any {
	if summary == nil {
		summary = map[string]any{}
	}
	summary[strings.TrimSpace(key)] = value
	return summary
}

func toRunOptionString(runOptions map[string]any, key string) string {
	if runOptions == nil {
		return ""
	}
	raw, ok := runOptions[strings.TrimSpace(key)]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		if data, err := common.Marshal(v); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func toRunOptionBool(runOptions map[string]any, key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func toRunOptionInt(runOptions map[string]any, key string, fallback int) int {
	value := strings.TrimSpace(toRunOptionString(runOptions, key))
	if value == "" {
		return fallback
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}

func cloneRunOptions(runOptions map[string]any) map[string]any {
	if len(runOptions) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(runOptions))
	for key, value := range runOptions {
		cloned[key] = value
	}
	return cloned
}

func normalizeProxyRetryCount(count int) int {
	if count < 0 {
		return 0
	}
	if count > 5 {
		return 5
	}
	return count
}

func buildRunOptionsForProxyAttempt(runOptions map[string]any, attempt int) map[string]any {
	cloned := cloneRunOptions(runOptions)
	if attempt < 0 {
		attempt = 0
	}
	cloned["_proxy_retry_attempt"] = attempt
	cloned["_proxy_candidate_offset"] = attempt
	if _, ok := cloned["_proxy_random_base_index"]; !ok {
		cloned["_proxy_random_base_index"] = rand.Intn(1_000_000)
	}
	if attempt > 0 {
		cloned["_proxy_force_switch"] = true
	} else {
		delete(cloned, "_proxy_force_switch")
	}
	return cloned
}

func hasConfiguredProxyCandidates(runOptions map[string]any, settings AutomationSettings) bool {
	if toRunOptionBool(runOptions, "no_proxy", false) || toRunOptionBool(runOptions, "disable_proxy", false) {
		return false
	}
	if len(splitProxyPool(strings.TrimSpace(toRunOptionString(runOptions, "proxy_pool")))) > 0 {
		return true
	}
	if len(splitProxyPool(strings.TrimSpace(settings.RegisterProxyPool))) > 0 {
		return true
	}
	return strings.TrimSpace(resolveConfiguredProxyGateway(runOptions, settings)) != ""
}

func isRetriableProxyError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	patterns := []string{
		"cloudflare",
		"add-phone",
		"切换代理",
		"代理节点",
		"proxy switch failed",
		"proxyconnect",
		"tunnel connection failed",
		"context deadline exceeded",
		"deadline exceeded",
		"timeout",
		"timed out",
		"connection reset by peer",
		"connection refused",
		"tls handshake timeout",
		"net::err",
		"captcha",
		"too many requests",
		" 429",
	}
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func shouldRetryAutomationModuleWithNextProxy(ctx context.Context, err error, runOptions map[string]any, settings AutomationSettings) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if !hasConfiguredProxyCandidates(runOptions, settings) {
		return false
	}
	return isRetriableProxyError(err)
}

func modulePipeline(module string, runOptions map[string]any) []string {
	module = strings.TrimSpace(strings.ToLower(module))
	allModules := []string{
		constant.AutomationModuleCloudflareEmailGenerate,
		constant.AutomationModuleTeamRedeem,
		constant.AutomationModuleGmailInviteWatch,
		constant.AutomationModuleBrowserLoginCapture,
		constant.AutomationModuleImportTarget,
	}
	if module == "" {
		return allModules
	}

	shouldContinue := false
	switch strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "pipeline_mode"))) {
	case "forward", "continue", "from_module":
		shouldContinue = true
	}
	if toRunOptionBool(runOptions, "continue_from_module", false) || toRunOptionBool(runOptions, "continue_pipeline", false) {
		shouldContinue = true
	}
	if !shouldContinue {
		return []string{module}
	}

	for i, moduleID := range allModules {
		if moduleID == module {
			return allModules[i:]
		}
	}
	return []string{module}
}

func StartLocalAutomationRun(req AutomationRunRequest, settings AutomationSettings) (*model.AutomationExecution, error) {
	if normalizeAutomationMode(settings.Mode) == constant.AutomationModeDisabled {
		return nil, fmt.Errorf("automation is disabled")
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	if err := settings.EnsureArtifactRoot(); err != nil {
		return nil, err
	}

	accounts, err := model.ListAutomationAccountsByFilter(req.AccountStatus, req.Search)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("没有匹配的自动化账号")
	}

	runOptions := req.RunOptions
	if runOptions == nil {
		runOptions = map[string]any{}
	}
	if strings.TrimSpace(req.RedeemSite) != "" {
		runOptions["redeem_site"] = strings.TrimSpace(req.RedeemSite)
	}
	rawOptions, _ := common.Marshal(runOptions)
	execution := &model.AutomationExecution{
		ExecutionID:   randomExecutionID(),
		Module:        strings.TrimSpace(strings.ToLower(req.Module)),
		Mode:          constant.AutomationModeLocal,
		Status:        constant.AutomationExecutionStatusQueued,
		Progress:      0,
		TotalAccounts: len(accounts),
		RunOptions:    string(rawOptions),
	}
	if err := execution.Insert(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	registerAutomationRun(execution.ExecutionID, cancel)

	go func() {
		defer unregisterAutomationRun(execution.ExecutionID)
		executeLocalAutomationRun(ctx, execution, accounts, runOptions, settings)
	}()

	return execution, nil
}

func executeLocalAutomationRun(ctx context.Context, execution *model.AutomationExecution, accounts []*model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) {
	if execution == nil {
		return
	}
	startedAt := common.GetTimestamp()
	updateAutomationExecution(execution, map[string]any{
		"status":     constant.AutomationExecutionStatusRunning,
		"started_at": startedAt,
	})
	appendAutomationLog(execution.ExecutionID, 0, "info", "run", "automation execution started", map[string]any{
		"module":         execution.Module,
		"total_accounts": len(accounts),
	})

	pipeline := modulePipeline(execution.Module, runOptions)
	successCount := 0
	failedCount := 0
	summary := map[string]any{}

	for index, account := range accounts {
		if account == nil {
			continue
		}
		if ctx.Err() != nil {
			appendAutomationLog(execution.ExecutionID, account.Id, "warning", "run", "execution canceled", nil)
			updateAutomationExecution(execution, map[string]any{
				"status":      constant.AutomationExecutionStatusStopped,
				"finished_at": common.GetTimestamp(),
				"progress":    int(float64(index) / float64(len(accounts)) * 100),
			})
			return
		}

		accountFailed := false
		for _, moduleID := range pipeline {
			if ctx.Err() != nil {
				accountFailed = true
				break
			}
			err := runAutomationModule(ctx, execution.ExecutionID, account, moduleID, runOptions, settings)
			if err != nil {
				accountFailed = true
				failedCount++
				updateAutomationAccountStatus(account.Id, constant.AutomationAccountStatusManualReview, err.Error())
				appendAutomationLog(execution.ExecutionID, account.Id, "error", moduleID, err.Error(), nil)
				summary = mergeExecutionSummary(summary, fmt.Sprintf("account_%d", account.Id), map[string]any{
					"status": "failed",
					"module": moduleID,
					"error":  err.Error(),
				})
				break
			}
		}

		if !accountFailed {
			successCount++
			summary = mergeExecutionSummary(summary, fmt.Sprintf("account_%d", account.Id), map[string]any{
				"status": "success",
				"module": pipeline[len(pipeline)-1],
			})
		}

		progress := int(float64(index+1) / float64(len(accounts)) * 100)
		if progress > 100 {
			progress = 100
		}
		updateAutomationExecution(execution, map[string]any{
			"success_count": successCount,
			"failed_count":  failedCount,
			"progress":      progress,
		})
	}

	finalStatus := constant.AutomationExecutionStatusCompleted
	if ctx.Err() != nil {
		finalStatus = constant.AutomationExecutionStatusStopped
	} else if successCount == 0 {
		finalStatus = constant.AutomationExecutionStatusFailed
	}
	summaryBytes, _ := common.Marshal(summary)
	updateAutomationExecution(execution, map[string]any{
		"status":         finalStatus,
		"finished_at":    common.GetTimestamp(),
		"progress":       100,
		"success_count":  successCount,
		"failed_count":   failedCount,
		"result_summary": string(summaryBytes),
	})
	appendAutomationLog(execution.ExecutionID, 0, "info", "run", "automation execution finished", map[string]any{
		"status":        finalStatus,
		"success_count": successCount,
		"failed_count":  failedCount,
	})
}

func runAutomationModule(ctx context.Context, executionID string, account *model.AutomationAccount, moduleID string, runOptions map[string]any, settings AutomationSettings) error {
	moduleID = strings.TrimSpace(strings.ToLower(moduleID))
	accountID := 0
	if account != nil {
		accountID = account.Id
	}
	appendAutomationLog(executionID, accountID, "info", moduleID, "module started", nil)
	maxRetry := normalizeProxyRetryCount(toRunOptionInt(runOptions, "proxy_retry_count", 1))
	baseRunOptions := buildRunOptionsForProxyAttempt(runOptions, 0)
	var lastErr error
	for attempt := 0; attempt <= maxRetry; attempt++ {
		attemptRunOptions := buildRunOptionsForProxyAttempt(baseRunOptions, attempt)
		err := runAutomationModuleOnce(ctx, executionID, account, moduleID, attemptRunOptions, settings)
		if err == nil {
			appendAutomationLog(executionID, accountID, "info", moduleID, "module completed", map[string]any{
				"attempt": attempt + 1,
			})
			return nil
		}
		lastErr = err
		if attempt >= maxRetry || !shouldRetryAutomationModuleWithNextProxy(ctx, err, attemptRunOptions, settings) {
			appendAutomationLog(executionID, accountID, "error", moduleID, "module failed", map[string]any{
				"attempt": attempt + 1,
				"error":   err.Error(),
			})
			return err
		}
		appendAutomationLog(executionID, accountID, "warning", moduleID, "proxy retry scheduled", map[string]any{
			"attempt":        attempt + 1,
			"next_attempt":   attempt + 2,
			"max_attempts":   maxRetry + 1,
			"retry_reason":   err.Error(),
			"rotate_mode":    strings.TrimSpace(toRunOptionString(runOptions, "proxy_rotate_mode")),
			"proxy_gateway":  strings.TrimSpace(resolveConfiguredProxyGateway(attemptRunOptions, settings)),
			"has_proxy_pool": len(splitProxyPool(strings.TrimSpace(toRunOptionString(attemptRunOptions, "proxy_pool")))) > 0 || len(splitProxyPool(strings.TrimSpace(settings.RegisterProxyPool))) > 0,
		})
	}
	return lastErr
}

func runAutomationModuleOnce(ctx context.Context, executionID string, account *model.AutomationAccount, moduleID string, runOptions map[string]any, settings AutomationSettings) error {
	switch moduleID {
	case constant.AutomationModuleCloudflareEmailGenerate:
		return runModuleCloudflareEmailGenerate(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleAutoRegister:
		return runModuleAutoRegister(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleTeamRedeem:
		return runModuleTeamRedeem(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleGmailInviteWatch:
		return runModuleGmailInviteWatch(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleBrowserLoginCapture:
		return runModuleBrowserLoginCapture(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleSessionConvert:
		return runModuleSessionConvert(ctx, executionID, account, runOptions, settings)
	case constant.AutomationModuleImportTarget:
		return runModuleImportTarget(ctx, executionID, account, runOptions, settings)
	default:
		return fmt.Errorf("unsupported module: %s", moduleID)
	}
}

func buildCloudflareAuthAttempts(settings AutomationSettings) []cloudflareAuthAttempt {
	token := strings.TrimSpace(settings.CloudflareAPIToken)
	email := strings.TrimSpace(settings.CloudflareAPIEmail)
	if token == "" {
		return []cloudflareAuthAttempt{}
	}
	attempts := []cloudflareAuthAttempt{
		{Mode: "api_token"},
	}
	if email != "" {
		attempts = append(attempts, cloudflareAuthAttempt{Mode: "global_key"})
	}
	return attempts
}

func applyCloudflareAuthHeaders(req *http.Request, settings AutomationSettings, attempt cloudflareAuthAttempt) {
	token := strings.TrimSpace(settings.CloudflareAPIToken)
	email := strings.TrimSpace(settings.CloudflareAPIEmail)
	switch attempt.Mode {
	case "global_key":
		if token != "" && email != "" {
			req.Header.Set("X-Auth-Key", token)
			req.Header.Set("X-Auth-Email", email)
		}
	default:
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
}

func extractCloudflareErrorMessage(payload map[string]any, fallback string) string {
	if payload == nil {
		return fallback
	}
	if msg := strings.TrimSpace(stringFromAny(payload["message"])); msg != "" {
		return msg
	}
	if errText := strings.TrimSpace(stringFromAny(payload["error"])); errText != "" {
		return errText
	}
	if errorsRaw, exists := payload["errors"]; exists {
		if data, err := common.Marshal(errorsRaw); err == nil {
			text := strings.TrimSpace(string(data))
			if text != "" && text != "null" {
				return text
			}
		}
	}
	return fallback
}

func cloudflareAPIRequest(ctx context.Context, settings AutomationSettings, method string, targetURL string, payload any) (map[string]any, error) {
	if strings.TrimSpace(settings.CloudflareAPIToken) == "" {
		return nil, fmt.Errorf("cloudflare token 配置缺失")
	}
	attempts := buildCloudflareAuthAttempts(settings)
	if len(attempts) == 0 {
		return nil, fmt.Errorf("cloudflare 认证配置缺失")
	}

	var bodyData []byte
	if payload != nil {
		raw, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyData = raw
	}

	client := &http.Client{Timeout: 30 * time.Second}
	lastErr := ""
	for _, attempt := range attempts {
		var body io.Reader
		if len(bodyData) > 0 {
			body = bytes.NewReader(bodyData)
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if len(bodyData) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		applyCloudflareAuthHeaders(req, settings, attempt)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err.Error()
			continue
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr.Error()
			continue
		}

		result := map[string]any{}
		if len(raw) > 0 {
			if err := common.Unmarshal(raw, &result); err != nil {
				lastErr = fmt.Sprintf("cloudflare response parse failed: %v", err)
				continue
			}
		}

		if resp.StatusCode >= 400 {
			message := extractCloudflareErrorMessage(result, strings.TrimSpace(string(raw)))
			if message == "" {
				message = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			lastErr = fmt.Sprintf("%s (mode=%s, status=%d)", message, attempt.Mode, resp.StatusCode)
			continue
		}
		if success, ok := result["success"].(bool); ok && !success {
			lastErr = fmt.Sprintf("%s (mode=%s)", extractCloudflareErrorMessage(result, "cloudflare request failed"), attempt.Mode)
			continue
		}
		return result, nil
	}

	if strings.TrimSpace(lastErr) == "" {
		lastErr = "unknown error"
	}
	return nil, errors.New(lastErr)
}

func buildCloudflareZoneLookupCandidates(domain string) []string {
	domain = strings.Trim(strings.TrimSpace(strings.ToLower(domain)), ".")
	if domain == "" {
		return []string{}
	}
	seen := map[string]bool{}
	appendCandidate := func(items []string, value string) []string {
		value = strings.Trim(strings.TrimSpace(strings.ToLower(value)), ".")
		if value == "" || seen[value] {
			return items
		}
		seen[value] = true
		return append(items, value)
	}

	candidates := []string{}
	candidates = appendCandidate(candidates, domain)
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts)-1; i++ {
		candidate := strings.Join(parts[i:], ".")
		candidates = appendCandidate(candidates, candidate)
	}
	return candidates
}

func resolveCloudflareZoneIDFromResult(result any, targetDomain string) string {
	rows, ok := result.([]any)
	if !ok || len(rows) == 0 {
		return ""
	}
	targetDomain = strings.Trim(strings.TrimSpace(strings.ToLower(targetDomain)), ".")
	bestID := ""
	bestNameLen := 0
	firstID := ""
	for _, row := range rows {
		item, ok := row.(map[string]any)
		if !ok || item == nil {
			continue
		}
		zoneID := strings.TrimSpace(stringFromAny(item["id"]))
		zoneName := strings.Trim(strings.TrimSpace(strings.ToLower(stringFromAny(item["name"]))), ".")
		if zoneID == "" {
			continue
		}
		if firstID == "" {
			firstID = zoneID
		}
		if zoneName == targetDomain {
			return zoneID
		}
		if zoneName != "" && strings.HasSuffix(targetDomain, "."+zoneName) {
			if len(zoneName) > bestNameLen {
				bestID = zoneID
				bestNameLen = len(zoneName)
			}
		}
	}
	if bestID != "" {
		return bestID
	}
	return firstID
}

func resolveCloudflareZoneID(ctx context.Context, settings AutomationSettings) (string, error) {
	if zoneID := strings.TrimSpace(settings.CloudflareZoneID); zoneID != "" {
		return zoneID, nil
	}
	domain := strings.Trim(strings.TrimSpace(strings.ToLower(settings.CloudflareDomain)), ".")
	if domain == "" {
		return "", fmt.Errorf("cloudflare domain 配置缺失（且未提供 zone id）")
	}
	if cached, ok := cloudflareZoneCache.Load(domain); ok {
		cachedZone := strings.TrimSpace(stringFromAny(cached))
		if cachedZone != "" {
			return cachedZone, nil
		}
	}

	candidates := buildCloudflareZoneLookupCandidates(domain)
	if len(candidates) == 0 {
		return "", fmt.Errorf("cloudflare domain 无效: %s", domain)
	}

	lastErr := ""
	for _, candidate := range candidates {
		lookupURL := "https://api.cloudflare.com/client/v4/zones?name=" + url.QueryEscape(candidate) + "&status=active&page=1&per_page=50"
		response, err := cloudflareAPIRequest(ctx, settings, http.MethodGet, lookupURL, nil)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		zoneID := resolveCloudflareZoneIDFromResult(response["result"], candidate)
		if zoneID != "" {
			cloudflareZoneCache.Store(domain, zoneID)
			return zoneID, nil
		}
		lastErr = "no matching zone returned"
	}

	if lastErr == "" {
		lastErr = "unknown error"
	}
	return "", fmt.Errorf("cloudflare zone 自动识别失败 (domain=%s): %s", domain, lastErr)
}

func cloudflareRequest(ctx context.Context, settings AutomationSettings, method string, path string, payload any) (map[string]any, error) {
	if strings.TrimSpace(settings.CloudflareAPIToken) == "" {
		return nil, fmt.Errorf("cloudflare token 配置缺失")
	}
	zoneID, err := resolveCloudflareZoneID(ctx, settings)
	if err != nil {
		return nil, err
	}
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		normalizedPath = "/"
	}
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	targetURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s%s", zoneID, normalizedPath)
	return cloudflareAPIRequest(ctx, settings, method, targetURL, payload)
}

func normalizeAutomationEmailGenerateProvider(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "":
		return ""
	case "cloudflare":
		return "cloudflare"
	case "tempmail", "wizawn", "wizawn_http", "tempmail_wizawn", "mailbox_wizawn":
		return "tempmail_wizawn"
	default:
		return value
	}
}

func selectAutomationEmailGenerateProvider(runOptions map[string]any, settings AutomationSettings) string {
	value := normalizeAutomationEmailGenerateProvider(toRunOptionString(runOptions, "provider"))
	if value != "" && value != "cloudflare" {
		return value
	}
	if value == "cloudflare" {
		return value
	}
	if strings.TrimSpace(settings.CloudflareAPIToken) != "" {
		return "cloudflare"
	}
	if strings.TrimSpace(settings.TempMailBaseURL) != "" && strings.TrimSpace(settings.TempMailAdminAuth) != "" {
		return "tempmail_wizawn"
	}
	return "cloudflare"
}

func sanitizeAutomationMailboxLocalPart(raw string, fallback string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		value = strings.TrimSpace(strings.ToLower(fallback))
	}
	if at := strings.Index(value, "@"); at >= 0 {
		value = value[:at]
	}
	if value == "" {
		value = "auto"
	}
	var builder strings.Builder
	lastDash := false
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
			lastDash = false
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastDash = false
		case ch == '.' || ch == '_' || ch == '-':
			builder.WriteRune(ch)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	value = strings.Trim(builder.String(), "._-")
	if value == "" {
		value = "auto"
	}
	if len(value) > 48 {
		value = strings.Trim(value[:48], "._-")
	}
	if value == "" {
		value = "auto"
	}
	return value
}

func buildAutomationAccountMetadata(account *model.AutomationAccount, patch map[string]any) string {
	existing := map[string]any{}
	if account != nil && strings.TrimSpace(account.Metadata) != "" {
		if err := common.UnmarshalJsonStr(strings.TrimSpace(account.Metadata), &existing); err != nil {
			existing = map[string]any{"raw": strings.TrimSpace(account.Metadata)}
		}
	}
	for key, value := range patch {
		existing[strings.TrimSpace(key)] = value
	}
	raw, err := common.Marshal(existing)
	if err != nil {
		if account != nil {
			return strings.TrimSpace(account.Metadata)
		}
		return ""
	}
	return string(raw)
}

func tempMailAdminRequest(ctx context.Context, settings AutomationSettings, method string, path string, payload any) (map[string]any, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(settings.TempMailBaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("tempmail base url 配置缺失")
	}
	adminAuth := strings.TrimSpace(settings.TempMailAdminAuth)
	if adminAuth == "" {
		return nil, fmt.Errorf("tempmail admin auth 配置缺失")
	}
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		normalizedPath = "/"
	}
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	var body io.Reader
	if payload != nil {
		raw, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+normalizedPath, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-admin-auth", adminAuth)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	timeoutSec := settings.WorkerTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tempmail admin API %s %s 失败 %d: %s", method, normalizedPath, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	payloadMap := map[string]any{}
	if len(raw) == 0 {
		return payloadMap, nil
	}
	var decoded any
	if err := common.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("tempmail admin API 响应解析失败: %w", err)
	}
	switch typed := decoded.(type) {
	case map[string]any:
		payloadMap = typed
	default:
		payloadMap = map[string]any{"data": decoded}
	}
	if success, ok := payloadMap["success"].(bool); ok && !success {
		errMsg := strings.TrimSpace(stringFromAny(payloadMap["message"]))
		if errMsg == "" {
			errMsg = strings.TrimSpace(stringFromAny(payloadMap["error"]))
		}
		if errMsg == "" {
			errMsg = "success=false"
		}
		return nil, fmt.Errorf("tempmail admin API %s %s 返回失败: %s", method, normalizedPath, errMsg)
	}
	return payloadMap, nil
}

func createTempMailAddress(ctx context.Context, settings AutomationSettings, name string, domain string, enableRandomSubdomain bool) (*tempMailCreateResult, error) {
	payload := map[string]any{
		"name":                  strings.TrimSpace(name),
		"enablePrefix":          false,
		"enableRandomSubdomain": enableRandomSubdomain,
	}
	if strings.TrimSpace(domain) != "" {
		payload["domain"] = strings.TrimSpace(strings.ToLower(domain))
	}
	response, err := tempMailAdminRequest(ctx, settings, http.MethodPost, "/admin/new_address", payload)
	if err != nil {
		return nil, err
	}
	result := &tempMailCreateResult{
		Address:   strings.TrimSpace(strings.ToLower(stringFromAny(response["address"]))),
		JWT:       strings.TrimSpace(stringFromAny(response["jwt"])),
		AddressID: toRunOptionInt(response, "address_id", 0),
		Password:  strings.TrimSpace(stringFromAny(response["password"])),
	}
	if result.Address == "" {
		return nil, fmt.Errorf("tempmail worker 未返回 address")
	}
	if result.JWT == "" {
		return nil, fmt.Errorf("tempmail worker 未返回 jwt")
	}
	return result, nil
}

func ensureTempMailMailboxRecord(account *model.AutomationAccount, address *tempMailCreateResult, settings AutomationSettings, runOptions map[string]any) (*model.ChatMockMailbox, error) {
	if account == nil || address == nil {
		return nil, fmt.Errorf("缺少账号或邮箱地址")
	}
	pollInterval := toRunOptionInt(runOptions, "mailbox_poll_interval_second", 60)
	if pollInterval <= 0 {
		pollInterval = 60
	}
	domain := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "domain")))
	if domain == "" {
		domain = strings.TrimSpace(strings.ToLower(settings.TempMailDomain))
	}

	metadata := map[string]any{
		"provider": "wizawn_http",
		"base_url": strings.TrimRight(strings.TrimSpace(settings.TempMailBaseURL), "/"),
		"address":  address.Address,
		"email":    address.Address,
	}
	if domain != "" {
		metadata["default_domain"] = domain
		metadata["domains"] = []string{domain}
	}
	metadataRaw, err := common.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	localPart := sanitizeAutomationMailboxLocalPart(strings.Split(address.Address, "@")[0], fmt.Sprintf("auto-%d", account.Id))
	name := fmt.Sprintf("auto-%d-%s", account.Id, localPart)
	mailbox := &model.ChatMockMailbox{
		Name:               name,
		Email:              address.Address,
		Username:           address.Address,
		Password:           address.JWT,
		ImapHost:           "",
		ImapPort:           993,
		ImapSecurity:       "ssl",
		SmtpHost:           "",
		SmtpPort:           465,
		SmtpSecurity:       "ssl",
		Status:             constant.ChatMockMailboxStatusEnabled,
		PollIntervalSecond: pollInterval,
		Metadata:           string(metadataRaw),
	}
	if existed, err := model.GetChatMockMailboxByEmail(address.Address); err == nil && existed != nil {
		existed.Name = mailbox.Name
		existed.Username = mailbox.Username
		existed.Password = mailbox.Password
		existed.ImapHost = mailbox.ImapHost
		existed.ImapPort = mailbox.ImapPort
		existed.ImapSecurity = mailbox.ImapSecurity
		existed.SmtpHost = mailbox.SmtpHost
		existed.SmtpPort = mailbox.SmtpPort
		existed.SmtpSecurity = mailbox.SmtpSecurity
		existed.Status = mailbox.Status
		existed.PollIntervalSecond = mailbox.PollIntervalSecond
		existed.Metadata = mailbox.Metadata
		if updateErr := existed.Update(); updateErr != nil {
			return nil, updateErr
		}
		return existed, nil
	}
	if err := mailbox.Insert(); err != nil {
		return nil, err
	}
	return mailbox, nil
}

func runModuleTempMailGenerate(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	if strings.TrimSpace(settings.TempMailBaseURL) == "" {
		return fmt.Errorf("缺少 tempmail base url")
	}
	if strings.TrimSpace(settings.TempMailAdminAuth) == "" {
		return fmt.Errorf("缺少 tempmail admin auth")
	}

	baseAlias := strings.TrimSpace(strings.ToLower(account.CloudflareAlias))
	if baseAlias == "" {
		baseAlias = strings.TrimSpace(strings.ToLower(account.Name))
	}
	if baseAlias == "" {
		baseAlias = strings.TrimSpace(strings.ToLower(account.Email))
	}
	if baseAlias == "" {
		baseAlias = fmt.Sprintf("auto-%d", account.Id)
	}
	baseAlias = sanitizeAutomationMailboxLocalPart(baseAlias, fmt.Sprintf("auto-%d", account.Id))

	strategy := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "alias_strategy")))
	domain := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "domain")))
	if domain == "" {
		domain = strings.TrimSpace(strings.ToLower(settings.TempMailDomain))
	}
	enableRandomSubdomain := toRunOptionBool(runOptions, "enable_random_subdomain", false)

	candidates := []string{baseAlias}
	if strategy == "append" {
		candidates = []string{
			fmt.Sprintf("%s-%d", baseAlias, time.Now().Unix()%10000),
			fmt.Sprintf("%s-%04d", baseAlias, rand.Intn(10000)),
		}
	} else {
		candidates = append(candidates,
			fmt.Sprintf("%s-%d", baseAlias, time.Now().Unix()%10000),
			fmt.Sprintf("%s-%04d", baseAlias, rand.Intn(10000)),
		)
	}

	var generated *tempMailCreateResult
	var lastErr error
	for index, candidate := range candidates {
		result, err := createTempMailAddress(ctx, settings, candidate, domain, enableRandomSubdomain)
		if err == nil {
			generated = result
			baseAlias = candidate
			break
		}
		lastErr = err
		appendAutomationLog(executionID, account.Id, "warning", constant.AutomationModuleCloudflareEmailGenerate, "tempmail address create failed", map[string]any{
			"candidate": candidate,
			"attempt":   index + 1,
			"error":     err.Error(),
		})
		if strategy == "skip" {
			break
		}
	}
	if generated == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("tempmail 地址生成失败")
		}
		return lastErr
	}

	mailbox, err := ensureTempMailMailboxRecord(account, generated, settings, runOptions)
	if err != nil {
		return err
	}

	metadataRaw := buildAutomationAccountMetadata(account, map[string]any{
		"email_provider":            "tempmail_wizawn",
		"tempmail_address":          generated.Address,
		"tempmail_address_id":       generated.AddressID,
		"tempmail_base_url":         strings.TrimRight(strings.TrimSpace(settings.TempMailBaseURL), "/"),
		"tempmail_domain":           domain,
		"tempmail_random_subdomain": enableRandomSubdomain,
	})
	updates := map[string]any{
		"email":          generated.Address,
		"mailbox_id":     mailbox.Id,
		"status":         constant.AutomationAccountStatusGenerated,
		"failure_reason": "",
		"metadata":       metadataRaw,
		"updated_time":   common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	account.Email = generated.Address
	account.MailboxId = mailbox.Id
	account.Status = constant.AutomationAccountStatusGenerated
	account.Metadata = metadataRaw
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleCloudflareEmailGenerate, "tempmail address generated", map[string]any{
		"address":    generated.Address,
		"address_id": generated.AddressID,
		"mailbox_id": mailbox.Id,
		"domain":     domain,
		"alias":      baseAlias,
	})
	return nil
}

func runModuleCloudflareEmailGenerate(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	switch selectAutomationEmailGenerateProvider(runOptions, settings) {
	case "tempmail_wizawn":
		return runModuleTempMailGenerate(ctx, executionID, account, runOptions, settings)
	case "cloudflare":
	default:
		return fmt.Errorf("unsupported email generate provider: %s", toRunOptionString(runOptions, "provider"))
	}

	domain := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "domain")))
	if domain == "" {
		domain = strings.TrimSpace(strings.ToLower(account.CloudflareDomain))
	}
	if domain == "" {
		domain = strings.TrimSpace(strings.ToLower(settings.CloudflareDomain))
	}
	if domain == "" {
		return fmt.Errorf("缺少 cloudflare domain")
	}
	alias := strings.TrimSpace(strings.ToLower(account.CloudflareAlias))
	if alias == "" {
		alias = strings.Split(strings.TrimSpace(strings.ToLower(account.Email)), "@")[0]
	}
	if alias == "" {
		alias = fmt.Sprintf("auto-%d", account.Id)
	}
	strategy := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "alias_strategy")))
	if strategy == "append" {
		alias = fmt.Sprintf("%s-%d", alias, time.Now().Unix()%10000)
	}
	targetEmail := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "forward_target")))
	if targetEmail == "" {
		targetEmail = strings.TrimSpace(strings.ToLower(account.ForwardTarget))
	}
	if targetEmail == "" {
		targetEmail = strings.TrimSpace(strings.ToLower(account.Email))
	}
	if targetEmail == "" {
		return fmt.Errorf("缺少转发目标邮箱")
	}
	aliasEmail := alias + "@" + domain

	_, err := cloudflareRequest(ctx, settings, http.MethodPost, "/email/routing/addresses", map[string]any{
		"email": targetEmail,
	})
	if err != nil {
		if strategy == "skip" && strings.Contains(strings.ToLower(err.Error()), "already") {
			appendAutomationLog(executionID, account.Id, "warning", constant.AutomationModuleCloudflareEmailGenerate, "cloudflare address already exists", map[string]any{"target": targetEmail})
		} else {
			return err
		}
	}

	_, err = cloudflareRequest(ctx, settings, http.MethodPost, "/email/routing/rules", map[string]any{
		"actions": []map[string]any{{
			"type":  "forward",
			"value": []string{targetEmail},
		}},
		"enabled": true,
		"matchers": []map[string]any{{
			"field": "to",
			"type":  "literal",
			"value": aliasEmail,
		}},
		"name":     fmt.Sprintf("auto-%s", alias),
		"priority": 0,
	})
	if err != nil {
		if strategy == "skip" && strings.Contains(strings.ToLower(err.Error()), "already") {
			appendAutomationLog(executionID, account.Id, "warning", constant.AutomationModuleCloudflareEmailGenerate, "cloudflare rule already exists", map[string]any{"alias": aliasEmail})
		} else {
			return err
		}
	}

	updates := map[string]any{
		"cloudflare_domain": domain,
		"cloudflare_alias":  alias,
		"forward_target":    targetEmail,
		"email":             aliasEmail,
		"status":            constant.AutomationAccountStatusGenerated,
		"failure_reason":    "",
		"updated_time":      common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	account.Email = aliasEmail
	account.CloudflareDomain = domain
	account.CloudflareAlias = alias
	account.ForwardTarget = targetEmail
	return nil
}

type automationProxyCandidate struct {
	Original  string
	Server    string
	Username  string
	Password  string
	Mode      string
	NodeName  string
	VMessNode *automationVMessNode
	Cleanup   func()
}

func splitProxyPool(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		for _, part := range strings.Split(line, ",") {
			value := strings.TrimSpace(part)
			if value == "" || strings.HasPrefix(value, "#") {
				continue
			}
			items = append(items, value)
		}
	}
	return items
}

func parseProxyCandidate(raw string) (*automationProxyCandidate, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, fmt.Errorf("empty proxy")
	}
	if strings.HasPrefix(strings.ToLower(value), "vmess://") {
		node, err := decodeVMessNode(value)
		if err != nil {
			return nil, err
		}
		return &automationProxyCandidate{
			Original:  value,
			Mode:      "vmess",
			NodeName:  node.displayName(),
			VMessNode: node,
		}, nil
	}
	if !strings.Contains(value, "://") {
		value = "socks5://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	switch scheme {
	case "socks5", "socks5h", "http", "https":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("proxy host is empty")
	}
	candidate := &automationProxyCandidate{
		Original: raw,
		Server:   fmt.Sprintf("%s://%s", scheme, parsed.Host),
		Mode:     scheme,
	}
	if parsed.User != nil {
		candidate.Username = parsed.User.Username()
		password, _ := parsed.User.Password()
		candidate.Password = password
	}
	return candidate, nil
}

func buildProxyCandidates(runOptions map[string]any, settings AutomationSettings) ([]*automationProxyCandidate, error) {
	if toRunOptionBool(runOptions, "no_proxy", false) || toRunOptionBool(runOptions, "disable_proxy", false) {
		return []*automationProxyCandidate{}, nil
	}
	pool := strings.TrimSpace(toRunOptionString(runOptions, "proxy_pool"))
	if pool == "" {
		pool = strings.TrimSpace(settings.RegisterProxyPool)
	}
	rawItems := splitProxyPool(pool)
	if len(rawItems) == 0 {
		return []*automationProxyCandidate{}, nil
	}
	candidates := make([]*automationProxyCandidate, 0, len(rawItems))
	invalidItems := make([]string, 0)
	for _, raw := range rawItems {
		item, err := parseProxyCandidate(raw)
		if err != nil {
			invalidItems = append(invalidItems, err.Error())
			continue
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 && len(invalidItems) > 0 {
		message := invalidItems[0]
		if len(invalidItems) > 1 {
			message = fmt.Sprintf("%s; 另有 %d 条代理配置无效", message, len(invalidItems)-1)
		}
		return nil, fmt.Errorf("proxy pool contains no valid entries: %s", message)
	}
	return candidates, nil
}

func resolveConfiguredProxyGateway(runOptions map[string]any, settings AutomationSettings) string {
	gateway := strings.TrimSpace(toRunOptionString(runOptions, "proxy_gateway"))
	if gateway == "" {
		gateway = strings.TrimSpace(settings.RegisterProxyGateway)
	}
	return gateway
}

func resolveDirectProxyCandidate(runOptions map[string]any, settings AutomationSettings) (*automationProxyCandidate, error) {
	gateway := resolveConfiguredProxyGateway(runOptions, settings)
	if gateway == "" {
		return nil, nil
	}
	return parseProxyCandidate(gateway)
}

func pickProxyCandidate(candidates []*automationProxyCandidate, runOptions map[string]any, account *model.AutomationAccount) *automationProxyCandidate {
	if len(candidates) == 0 {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "proxy_rotate_mode")))
	offset := toRunOptionInt(runOptions, "_proxy_candidate_offset", 0)
	if offset < 0 {
		offset = 0
	}
	if mode == "random" {
		base := toRunOptionInt(runOptions, "_proxy_random_base_index", -1)
		if base < 0 {
			base = rand.Intn(len(candidates))
			if runOptions != nil {
				runOptions["_proxy_random_base_index"] = base
			}
		}
		return candidates[(base+offset)%len(candidates)]
	}
	base := 0
	if account != nil && account.Id > 0 {
		base = account.Id
	}
	return candidates[(base+offset)%len(candidates)]
}

func switchProxyNode(ctx context.Context, switchURL string, token string, nodeName string) error {
	switchURL = strings.TrimSpace(switchURL)
	if switchURL == "" {
		return nil
	}
	payload := map[string]any{}
	if strings.TrimSpace(nodeName) != "" {
		payload["name"] = strings.TrimSpace(nodeName)
	} else {
		payload["action"] = "next"
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, switchURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("proxy switch failed %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func resolveLaunchProxy(ctx context.Context, candidate *automationProxyCandidate, runOptions map[string]any, settings AutomationSettings) (*automationProxyCandidate, error) {
	switchURL := strings.TrimSpace(toRunOptionString(runOptions, "proxy_switch_url"))
	switchToken := strings.TrimSpace(toRunOptionString(runOptions, "proxy_switch_token"))
	forceSwitch := toRunOptionBool(runOptions, "_proxy_force_switch", false) || toRunOptionInt(runOptions, "_proxy_retry_attempt", 0) > 0
	if candidate == nil {
		return nil, nil
	}
	if candidate.Mode != "vmess" {
		gateway := resolveConfiguredProxyGateway(runOptions, settings)
		if forceSwitch && switchURL != "" && gateway != "" && strings.EqualFold(strings.TrimSpace(candidate.Original), strings.TrimSpace(gateway)) {
			if err := switchProxyNode(ctx, switchURL, switchToken, candidate.NodeName); err != nil {
				return nil, err
			}
		}
		return candidate, nil
	}
	vmessErr := error(nil)
	if launchProxy, err := startVMessProxyRuntime(ctx, candidate); err == nil {
		return launchProxy, nil
	} else {
		vmessErr = err
	}
	gateway := resolveConfiguredProxyGateway(runOptions, settings)
	if gateway == "" {
		if vmessErr != nil {
			return nil, fmt.Errorf("vmess proxy convert failed: %w", vmessErr)
		}
		return nil, fmt.Errorf("vmess proxy requires local xray runtime or proxy_gateway")
	}
	if switchURL != "" {
		if err := switchProxyNode(ctx, switchURL, switchToken, candidate.NodeName); err != nil {
			return nil, err
		}
	}
	return parseProxyCandidate(gateway)
}

func cloneAutomationHTTPClient(client *http.Client, timeout time.Duration) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	cloned := *client
	if timeout > 0 {
		cloned.Timeout = timeout
	}
	return &cloned
}

func openAutomationHTTPClientWithOptions(
	ctx context.Context,
	settings AutomationSettings,
	runOptions map[string]any,
	account *model.AutomationAccount,
	executionID string,
	module string,
	timeout time.Duration,
) (*http.Client, func(), map[string]any, error) {
	sessionMeta := map[string]any{}
	proxyCleanup := func() {}

	candidates, buildErr := buildProxyCandidates(runOptions, settings)
	if buildErr != nil {
		return nil, proxyCleanup, sessionMeta, buildErr
	}
	selected := pickProxyCandidate(candidates, runOptions, account)
	if selected == nil {
		selected, buildErr = resolveDirectProxyCandidate(runOptions, settings)
		if buildErr != nil {
			return nil, proxyCleanup, sessionMeta, buildErr
		}
	}
	launchProxy, resolveErr := resolveLaunchProxy(ctx, selected, runOptions, settings)
	if resolveErr != nil {
		return nil, proxyCleanup, sessionMeta, resolveErr
	}
	if launchProxy == nil {
		return cloneAutomationHTTPClient(GetHttpClient(), timeout), proxyCleanup, sessionMeta, nil
	}
	if launchProxy.Cleanup != nil {
		proxyCleanup = launchProxy.Cleanup
	}
	baseClient, err := NewProxyHttpClient(launchProxy.Server)
	if err != nil {
		proxyCleanup()
		return nil, func() {}, sessionMeta, err
	}
	sessionMeta["proxy_mode"] = launchProxy.Mode
	sessionMeta["proxy_value"] = launchProxy.Server
	sessionMeta["proxy_node"] = launchProxy.NodeName
	if strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" {
		accountID := 0
		if account != nil {
			accountID = account.Id
		}
		appendAutomationLog(executionID, accountID, "info", module, "http proxy selected", map[string]any{
			"mode":  launchProxy.Mode,
			"proxy": launchProxy.Server,
			"node":  launchProxy.NodeName,
		})
	}
	return cloneAutomationHTTPClient(baseClient, timeout), proxyCleanup, sessionMeta, nil
}

func parseStringMapFromOption(raw string) map[string]string {
	result := map[string]string{}
	text := strings.TrimSpace(raw)
	if text == "" {
		return result
	}
	parsed := map[string]any{}
	if err := common.UnmarshalJsonStr(text, &parsed); err != nil {
		return result
	}
	for key, value := range parsed {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		v := strings.TrimSpace(fmt.Sprintf("%v", value))
		if v == "" || v == "<nil>" {
			continue
		}
		result[k] = v
	}
	return result
}

func isOpenAIAuthHost(host string) bool {
	normalized := strings.TrimSpace(strings.ToLower(host))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, ":") {
		if parsedHost, _, err := net.SplitHostPort(normalized); err == nil {
			normalized = parsedHost
		}
	}
	switch normalized {
	case "auth.openai.com", "auth0.openai.com", "accounts.openai.com":
		return true
	default:
		return false
	}
}

func isChatGPTHost(host string) bool {
	normalized := strings.TrimSpace(strings.ToLower(host))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, ":") {
		if parsedHost, _, err := net.SplitHostPort(normalized); err == nil {
			normalized = parsedHost
		}
	}
	switch normalized {
	case "chatgpt.com", "www.chatgpt.com":
		return true
	default:
		return false
	}
}

func buildAutoRegisterTarget(runOptions map[string]any, settings AutomationSettings) (entryURL string, targetURL string, signupPath string, officialEntry bool, forcedReason string, err error) {
	officialEntry = toRunOptionBool(runOptions, "official_entry", true)
	preferAuthEntry := toRunOptionBool(runOptions, "prefer_auth_entry", false)
	entryURL = strings.TrimSpace(toRunOptionString(runOptions, "entry_url"))
	if entryURL == "" {
		entryURL = strings.TrimSpace(settings.RegisterEntryURL)
	}
	if entryURL == "" {
		if officialEntry && preferAuthEntry {
			entryURL = "https://auth.openai.com"
		} else {
			entryURL = "https://chatgpt.com"
		}
	}
	if !strings.HasPrefix(strings.ToLower(entryURL), "http://") && !strings.HasPrefix(strings.ToLower(entryURL), "https://") {
		entryURL = "https://" + entryURL
	}
	signupPath = strings.TrimSpace(toRunOptionString(runOptions, "signup_path"))
	if signupPath == "" {
		if officialEntry && preferAuthEntry {
			signupPath = "/log-in-or-create-account"
		} else if officialEntry {
			signupPath = "/auth/login"
		} else {
			signupPath = "/auth/signup"
		}
	}

	entryParsed, parseErr := url.Parse(entryURL)
	if parseErr != nil || strings.TrimSpace(entryParsed.Hostname()) == "" {
		return "", "", "", officialEntry, "", fmt.Errorf("entry_url 无效: %s", entryURL)
	}
	entryHost := strings.TrimSpace(strings.ToLower(entryParsed.Hostname()))

	signupAbsolute := strings.HasPrefix(strings.ToLower(signupPath), "http://") || strings.HasPrefix(strings.ToLower(signupPath), "https://")
	if officialEntry {
		if preferAuthEntry {
			if entryHost != "auth.openai.com" {
				forcedReason = fmt.Sprintf("official_entry=true + prefer_auth_entry=true，入口已从 %s 强制切换为 auth.openai.com", entryHost)
			}
			entryURL = "https://auth.openai.com"
			entryHost = "auth.openai.com"
			if signupAbsolute {
				forcedReason = strings.TrimSpace(forcedReason + "; signup_path 绝对地址已回退为 /log-in-or-create-account")
			}
			signupPath = "/log-in-or-create-account"
		} else {
			if !isChatGPTHost(entryHost) {
				forcedReason = fmt.Sprintf("official_entry=true，入口已从 %s 强制切换为 chatgpt.com（非 OAuth 官网入口）", entryHost)
			}
			entryURL = "https://chatgpt.com"
			entryHost = "chatgpt.com"
			if signupAbsolute {
				signupParsed, signupParseErr := url.Parse(signupPath)
				if signupParseErr != nil || !isChatGPTHost(signupParsed.Hostname()) {
					forcedReason = strings.TrimSpace(forcedReason + "; signup_path 绝对地址已回退为 /auth/login（非 OAuth）")
					signupPath = "/auth/login"
				}
			}
			if !signupAbsolute {
				signupPath = "/auth/login"
			}
		}
	} else {
		if signupAbsolute {
			signupParsed, signupParseErr := url.Parse(signupPath)
			if signupParseErr == nil && isOpenAIAuthHost(signupParsed.Hostname()) {
				signupPath = "/auth/signup"
				if forcedReason == "" {
					forcedReason = "检测到 signup_path 为 auth.openai.com OAuth 地址，已回退为 /auth/signup"
				}
			}
		}
	}

	baseURL := strings.TrimRight(entryURL, "/")
	if strings.HasPrefix(strings.ToLower(signupPath), "http://") || strings.HasPrefix(strings.ToLower(signupPath), "https://") {
		targetURL = signupPath
	} else if signupPath != "" {
		if !strings.HasPrefix(signupPath, "/") {
			signupPath = "/" + signupPath
		}
		targetURL = baseURL + signupPath
	} else {
		targetURL = baseURL
	}
	return entryURL, targetURL, signupPath, officialEntry, strings.TrimSpace(forcedReason), nil
}

func detectAddPhoneChallenge(page playwright.Page) (bool, string) {
	if page == nil {
		return false, ""
	}
	currentURL := strings.ToLower(strings.TrimSpace(page.URL()))
	if strings.Contains(currentURL, "add-phone") || strings.Contains(currentURL, "phone") {
		return true, "url contains add-phone/phone"
	}
	bodyText, _ := page.InnerText("body")
	pageText := strings.ToLower(strings.TrimSpace(bodyText))
	keywords := []string{
		"add phone",
		"phone number",
		"verify your phone",
		"enter your phone",
		"手机号",
		"手机号码",
		"验证手机",
	}
	for _, keyword := range keywords {
		if strings.Contains(pageText, keyword) {
			return true, keyword
		}
	}
	selectors := []string{
		"input[type='tel']",
		"input[name*='phone']",
		"input[id*='phone']",
	}
	for _, selector := range selectors {
		node, err := page.QuerySelector(selector)
		if err == nil && node != nil {
			return true, selector
		}
	}
	return false, ""
}

func extractContinueURLFromPageURL(rawURL string) string {
	candidate := strings.TrimSpace(rawURL)
	if candidate == "" {
		return ""
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	candidates := []string{
		strings.TrimSpace(parsed.Query().Get("continue_url")),
		strings.TrimSpace(parsed.Query().Get("continue")),
	}
	for _, item := range candidates {
		if item == "" {
			continue
		}
		decoded, decodeErr := url.QueryUnescape(item)
		if decodeErr == nil && strings.TrimSpace(decoded) != "" {
			item = strings.TrimSpace(decoded)
		}
		next, parseErr := url.Parse(item)
		if parseErr != nil {
			continue
		}
		if !next.IsAbs() {
			next = parsed.ResolveReference(next)
		}
		if next == nil {
			continue
		}
		if next.Scheme == "http" || next.Scheme == "https" {
			return next.String()
		}
	}
	return ""
}

func tryResolveAddPhoneChallenge(page playwright.Page, humanLike bool) (bool, string) {
	if page == nil {
		return false, "page is nil"
	}
	if hit, _ := detectAddPhoneChallenge(page); !hit {
		return true, ""
	}

	skipSelectors := []string{
		"button:has-text('Skip')",
		"button:has-text('Not now')",
		"button:has-text('Later')",
		"button:has-text('Continue without')",
		"a:has-text('Skip')",
		"a:has-text('Not now')",
		"a:has-text('Later')",
		"a:has-text('Continue without')",
		"button:has-text('暂不')",
		"button:has-text('稍后')",
		"button:has-text('跳过')",
		"a:has-text('暂不')",
		"a:has-text('稍后')",
		"a:has-text('跳过')",
	}
	triedContinueURLs := map[string]struct{}{}
	deadline := time.Now().Add(20 * time.Second)
	lastReason := "页面停留在 add-phone，且未发现可跳过入口"

	for time.Now().Before(deadline) {
		if hit, reason := detectAddPhoneChallenge(page); !hit {
			return true, ""
		} else if strings.TrimSpace(reason) != "" {
			lastReason = strings.TrimSpace(reason)
		}

		if err := clickWithSelectorFallback(page, "", skipSelectors); err == nil {
			if humanLike {
				humanPause(page, 500, 1400)
			} else {
				page.WaitForTimeout(1000)
			}
			continue
		}

		continueURL := extractContinueURLFromPageURL(page.URL())
		if continueURL != "" {
			if _, exists := triedContinueURLs[continueURL]; !exists {
				triedContinueURLs[continueURL] = struct{}{}
				if _, err := page.Goto(continueURL, playwright.PageGotoOptions{
					Timeout:   playwright.Float(30000),
					WaitUntil: playwright.WaitUntilStateDomcontentloaded,
				}); err == nil {
					if humanLike {
						humanPause(page, 600, 1500)
					} else {
						page.WaitForTimeout(1000)
					}
					continue
				}
			}
		}

		page.WaitForTimeout(500)
	}
	return false, lastReason
}

func detectCloudflareChallenge(page playwright.Page) (bool, string) {
	if page == nil {
		return false, ""
	}
	currentURL := strings.ToLower(strings.TrimSpace(page.URL()))
	if strings.Contains(currentURL, "/cdn-cgi/challenge") ||
		strings.Contains(currentURL, "challenge-platform") ||
		strings.Contains(currentURL, "__cf_chl_") ||
		strings.Contains(currentURL, "cf_chl") {
		return true, "url contains cloudflare challenge path"
	}
	title, _ := page.Title()
	titleText := strings.ToLower(strings.TrimSpace(title))
	titleKeywords := []string{
		"just a moment",
		"attention required",
		"cloudflare",
	}
	for _, keyword := range titleKeywords {
		if strings.Contains(titleText, keyword) {
			return true, "page title contains " + keyword
		}
	}
	bodyText, _ := page.InnerText("body")
	pageText := strings.ToLower(strings.TrimSpace(bodyText))
	keywords := []string{
		"enable javascript and cookies to continue",
		"enable javascript and cookies",
		"verification successful. waiting for",
		"checking your browser before accessing",
		"checking if the site connection is secure",
		"verify you are human",
		"just a moment",
		"attention required",
		"please unblock challenges.cloudflare.com",
		"cf-mitigated",
		"ray id",
	}
	for _, keyword := range keywords {
		if strings.Contains(pageText, keyword) {
			return true, keyword
		}
	}
	selectors := []string{
		"#challenge-error-text",
		"script[src*='challenge-platform']",
		"iframe[src*='challenge-platform']",
	}
	for _, selector := range selectors {
		node, err := page.QuerySelector(selector)
		if err == nil && node != nil {
			return true, "selector " + selector
		}
	}
	content, _ := page.Content()
	contentText := strings.ToLower(content)
	contentKeywords := []string{
		"cdn-cgi/challenge-platform",
		"_cf_chl_opt",
		"__cf_chl_rt_tk",
		"challenges.cloudflare.com",
	}
	for _, keyword := range contentKeywords {
		if strings.Contains(contentText, keyword) {
			return true, "html contains " + keyword
		}
	}
	return false, ""
}

func waitCloudflareChallengeResolved(page playwright.Page, maxWaitSec int, humanLike bool) (bool, string) {
	if page == nil {
		return false, "page is nil"
	}
	if maxWaitSec < 0 {
		maxWaitSec = 0
	}
	blocked, reason := detectCloudflareChallenge(page)
	if !blocked {
		return true, ""
	}
	lastReason := strings.TrimSpace(reason)
	if maxWaitSec == 0 {
		return false, lastReason
	}

	deadline := time.Now().Add(time.Duration(maxWaitSec) * time.Second)
	for time.Now().Before(deadline) {
		if humanLike {
			humanPause(page, 900, 2200)
		} else {
			page.WaitForTimeout(1200)
		}
		blocked, reason = detectCloudflareChallenge(page)
		if !blocked {
			return true, ""
		}
		if strings.TrimSpace(reason) != "" {
			lastReason = strings.TrimSpace(reason)
		}
	}
	if lastReason == "" {
		lastReason = "challenge still active after wait window"
	}
	return false, lastReason
}

func waitForAutoRegisterOutcome(page playwright.Page, humanLike bool, timeoutSec int, customSuccessSelector string) error {
	if page == nil {
		return fmt.Errorf("page is nil")
	}
	if timeoutSec < 30 {
		timeoutSec = 30
	}
	if timeoutSec > 900 {
		timeoutSec = 900
	}

	successSelectors := []string{
		"#prompt-textarea",
		"[data-testid='composer']",
		"nav button[data-testid='profile-button']",
		"[data-testid='conversation-turn-0']",
		"[data-testid='model-switcher-dropdown']",
	}
	if strings.TrimSpace(customSuccessSelector) != "" {
		successSelectors = append([]string{strings.TrimSpace(customSuccessSelector)}, successSelectors...)
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if blocked, reason := detectCloudflareChallenge(page); blocked {
			return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
		}
		if hitPhone, reason := detectAddPhoneChallenge(page); hitPhone {
			resolved, resolveReason := tryResolveAddPhoneChallenge(page, humanLike)
			if !resolved {
				detail := strings.TrimSpace(resolveReason)
				if detail == "" {
					detail = strings.TrimSpace(reason)
				}
				return fmt.Errorf("注册流程命中 add-phone 验证（%s），请改用新代理节点重试", detail)
			}
			continue
		}
		if pageHasAnySelector(page, successSelectors) {
			return nil
		}
		page.WaitForTimeout(800)
	}
	return fmt.Errorf("注册流程超时：未进入可识别的成功页面")
}

func pageHasAnySelector(page playwright.Page, selectors []string) bool {
	if page == nil {
		return false
	}
	for _, selector := range selectors {
		sel := strings.TrimSpace(selector)
		if sel == "" {
			continue
		}
		node, err := page.QuerySelector(sel)
		if err == nil && node != nil {
			return true
		}
	}
	return false
}

func runModuleAutoRegister(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	email := strings.ToLower(strings.TrimSpace(account.Email))
	if email == "" {
		return fmt.Errorf("缺少注册邮箱")
	}
	password := strings.TrimSpace(account.Password)
	if password == "" {
		password = strings.TrimSpace(toRunOptionString(runOptions, "password"))
	}
	if password == "" {
		local := strings.TrimSpace(strings.Split(email, "@")[0])
		if local != "" {
			password = local
		}
	}
	if password == "" {
		return fmt.Errorf("缺少注册密码")
	}

	entryURL, targetURL, signupPath, officialEntry, forcedReason, err := buildAutoRegisterTarget(runOptions, settings)
	if err != nil {
		return err
	}
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleAutoRegister, "register target prepared", map[string]any{
		"entry_url":      entryURL,
		"target_url":     targetURL,
		"signup_path":    signupPath,
		"official_entry": officialEntry,
		"forced_reason":  forcedReason,
	})

	headless := toRunOptionBool(runOptions, "headless", false)
	humanLike := toRunOptionBool(runOptions, "human_like", true)
	cfWaitSec := toRunOptionInt(runOptions, "cloudflare_wait_seconds", 45)
	if cfWaitSec < 0 {
		cfWaitSec = 0
	}
	if cfWaitSec > 180 {
		cfWaitSec = 180
	}
	timeoutSec := toRunOptionInt(runOptions, "timeout_seconds", 180)
	if timeoutSec < 30 {
		timeoutSec = 30
	}
	if timeoutSec > 900 {
		timeoutSec = 900
	}

	pw, browser, page, closeFn, sessionMeta, err := openPlaywrightPageWithOptions(
		ctx,
		settings,
		headless,
		runOptions,
		account,
		executionID,
		constant.AutomationModuleAutoRegister,
	)
	if err != nil {
		return err
	}
	defer closeFn()
	_ = pw
	_ = browser

	page.SetDefaultTimeout(float64(timeoutSec * 1000))
	page.SetDefaultNavigationTimeout(float64(timeoutSec * 1000))
	navOpts := playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(timeoutSec * 1000)),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	if _, err = page.Goto(entryURL, navOpts); err != nil {
		return err
	}
	if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
		return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
	}

	loginSelectors := []string{
		"button:has-text('Log in')",
		"a:has-text('Log in')",
		"button:has-text('Login')",
		"a:has-text('Login')",
		"button:has-text('登录')",
		"a:has-text('登录')",
	}
	signupSelectors := []string{
		"a[href*='/auth/signup']",
		"button:has-text('Sign up')",
		"a:has-text('Sign up')",
		"button:has-text('Create account')",
		"a:has-text('Create account')",
		"button:has-text('注册')",
		"a:has-text('注册')",
	}
	emailSelectors := []string{
		"input[type='email']",
		"input[autocomplete='email']",
		"input[autocomplete='username']",
		"input[name='email']",
		"input[name='username']",
		"input[id*='email']",
		"input[placeholder*='email' i]",
	}
	authAction := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "auth_action")))
	if authAction != "login" && authAction != "signup" {
		authAction = "signup"
	}
	preferSignup := authAction != "login"
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleAutoRegister, "register auth action selected", map[string]any{
		"auth_action": authAction,
		"human_like":  humanLike,
	})

	if officialEntry {
		if humanLike {
			humanPause(page, 400, 1200)
		}
		_ = clickWithSelectorFallback(page, "", []string{
			"button:has-text('Reject non-essential')",
			"button:has-text('Accept all')",
			"button:has-text('Manage Cookies')",
			"button:has-text('Accept')",
		})
		if humanLike {
			humanPause(page, 700, 1800)
		} else {
			page.WaitForTimeout(600)
		}
		if preferSignup {
			_ = clickWithSelectorFallback(page, "", signupSelectors)
		} else {
			_ = clickWithSelectorFallback(page, "", []string{
				"a[href*='/auth/login']",
				"a[href*='chatgpt.com/auth/login']",
				loginSelectors[0],
				loginSelectors[1],
				loginSelectors[2],
				loginSelectors[3],
				loginSelectors[4],
				loginSelectors[5],
			})
		}
		if humanLike {
			humanPause(page, 1800, 4200)
		} else {
			page.WaitForTimeout(3500)
		}
		// chatgpt.com 有时先落地营销页，需要二次点击入口按钮进入真实认证页。
		if !pageHasAnySelector(page, emailSelectors) {
			if _, err = page.Goto(targetURL, navOpts); err != nil {
				return err
			}
			if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
				return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
			}
			if humanLike {
				humanPause(page, 900, 2400)
			} else {
				page.WaitForTimeout(2000)
			}
			if preferSignup {
				_ = clickWithSelectorFallback(page, "", signupSelectors)
			} else {
				_ = clickWithSelectorFallback(page, "", loginSelectors)
			}
			if humanLike {
				humanPause(page, 1800, 4200)
			} else {
				page.WaitForTimeout(3500)
			}
			if preferSignup && !pageHasAnySelector(page, emailSelectors) {
				signupURL := strings.TrimRight(entryURL, "/") + "/auth/signup"
				if strings.Contains(strings.ToLower(entryURL), "auth.openai.com") {
					signupURL = "https://auth.openai.com/log-in-or-create-account"
				}
				if strings.TrimSpace(targetURL) != signupURL {
					if _, err = page.Goto(signupURL, navOpts); err != nil {
						return err
					}
					if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
						return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
					}
					if humanLike {
						humanPause(page, 900, 2600)
					} else {
						page.WaitForTimeout(1800)
					}
				}
			}
		}
	} else {
		if _, err = page.Goto(targetURL, navOpts); err != nil {
			return err
		}
		if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
			return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
		}
		if preferSignup && !pageHasAnySelector(page, emailSelectors) {
			_ = clickWithSelectorFallback(page, "", signupSelectors)
			if humanLike {
				humanPause(page, 900, 2400)
			} else {
				page.WaitForTimeout(2000)
			}
		}
	}

	if !pageHasAnySelector(page, emailSelectors) {
		_ = clickWithSelectorFallback(page, "", []string{
			"button:has-text('Continue with email')",
			"button:has-text('Use email')",
			"button:has-text('Email')",
			"button:has-text('继续使用邮箱')",
			"button:has-text('使用邮箱')",
		})
		if humanLike {
			humanPause(page, 600, 1600)
		} else {
			page.WaitForTimeout(1000)
		}
	}

	if !pageHasAnySelector(page, emailSelectors) {
		for i := 0; i < 20; i++ {
			if pageHasAnySelector(page, emailSelectors) {
				break
			}
			page.WaitForTimeout(1000)
		}
	}
	if !pageHasAnySelector(page, emailSelectors) {
		if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
			return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
		}
		if hitPhone, reason := detectAddPhoneChallenge(page); hitPhone {
			resolved, resolveReason := tryResolveAddPhoneChallenge(page, humanLike)
			if !resolved {
				detail := strings.TrimSpace(resolveReason)
				if detail == "" {
					detail = strings.TrimSpace(reason)
				}
				return fmt.Errorf("注册流程命中 add-phone 验证（%s），请改用新代理节点重试", detail)
			}
		}
	}
	fillEmail := fillWithSelectorFallback
	if humanLike {
		fillEmail = fillWithSelectorTypingFallback
		humanPause(page, 350, 1200)
	}
	if err := fillEmail(page, toRunOptionString(runOptions, "email_selector"), email, emailSelectors); err != nil {
		if ok, reason := waitCloudflareChallengeResolved(page, cfWaitSec, humanLike); !ok {
			return fmt.Errorf("注册流程命中 Cloudflare 挑战（%s），请切换代理或降低自动化频率", reason)
		}
		return err
	}

	if humanLike {
		humanPause(page, 400, 1100)
	}
	_ = clickWithSelectorFallback(page, toRunOptionString(runOptions, "continue_selector"), []string{
		"button[type='submit']",
		"button:has-text('Continue')",
		"button:has-text('继续')",
		"button:has-text('Next')",
	})

	// 认证页有时在下一屏展示密码框，这里给一次等待缓冲。
	if humanLike {
		humanPause(page, 1000, 2400)
	} else {
		page.WaitForTimeout(1200)
	}

	fillPassword := fillWithSelectorFallback
	if humanLike {
		fillPassword = fillWithSelectorTypingFallback
		humanPause(page, 300, 1000)
	}
	if err := fillPassword(page, toRunOptionString(runOptions, "password_selector"), password, []string{
		"input[type='password']",
	}); err != nil {
		return err
	}

	if humanLike {
		humanPause(page, 350, 1100)
	}
	if err := clickWithSelectorFallback(page, toRunOptionString(runOptions, "submit_selector"), []string{
		"button[type='submit']",
		"button:has-text('Sign up')",
		"button:has-text('Create account')",
		"button:has-text('注册')",
	}); err != nil {
		return err
	}

	successSelector := strings.TrimSpace(toRunOptionString(runOptions, "success_selector"))
	if err := waitForAutoRegisterOutcome(page, humanLike, timeoutSec, successSelector); err != nil {
		return err
	}

	pageContext := page.Context()
	dir, err := ensureExecutionAccountDir(settings, executionID, account.Id)
	if err != nil {
		return err
	}
	statePath := filepath.Join(dir, "register-storage-state.json")
	if _, err := pageContext.StorageState(statePath); err != nil {
		return err
	}
	cookies, err := pageContext.Cookies()
	if err != nil {
		return err
	}
	cookiesRaw, err := common.Marshal(cookies)
	if err != nil {
		return err
	}
	cookiePath := filepath.Join(dir, "register-cookies.json")
	if err := os.WriteFile(cookiePath, cookiesRaw, 0o600); err != nil {
		return err
	}
	recordAutomationArtifact(executionID, account.Id, constant.AutomationModuleAutoRegister, "register_storage_state", statePath, map[string]any{
		"entry_url":   entryURL,
		"target_url":  targetURL,
		"proxy_mode":  sessionMeta["proxy_mode"],
		"proxy_value": sessionMeta["proxy_value"],
		"proxy_node":  sessionMeta["proxy_node"],
	})
	recordAutomationArtifact(executionID, account.Id, constant.AutomationModuleAutoRegister, "register_cookies", cookiePath, nil)

	registerSource := "chatgpt.com"
	if parsedEntryURL, parseErr := url.Parse(entryURL); parseErr == nil && strings.TrimSpace(parsedEntryURL.Hostname()) != "" {
		registerSource = strings.ToLower(strings.TrimSpace(parsedEntryURL.Hostname()))
	}
	metadata := map[string]any{
		"register_source": registerSource,
		"register_entry":  entryURL,
		"register_path":   signupPath,
		"official_entry":  officialEntry,
		"register_proxy":  sessionMeta["proxy_value"],
		"proxy_node":      sessionMeta["proxy_node"],
	}
	if forcedReason != "" {
		metadata["register_forced_reason"] = forcedReason
	}
	rawMetadata := strings.TrimSpace(account.Metadata)
	existing := map[string]any{}
	if rawMetadata != "" {
		if err := common.UnmarshalJsonStr(rawMetadata, &existing); err != nil {
			existing = map[string]any{"raw": rawMetadata}
		}
	}
	for key, value := range metadata {
		existing[key] = value
	}
	metadataRaw, _ := common.Marshal(existing)

	updates := map[string]any{
		"status":                constant.AutomationAccountStatusGenerated,
		"failure_reason":        "",
		"password":              password,
		"session_artifact_path": statePath,
		"metadata":              string(metadataRaw),
		"updated_time":          common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	account.Password = password
	account.SessionArtifactPath = statePath
	account.Status = constant.AutomationAccountStatusGenerated
	account.Metadata = string(metadataRaw)
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleAutoRegister, "auto register completed", map[string]any{
		"entry_url":  entryURL,
		"target_url": targetURL,
		"proxy":      sessionMeta["proxy_value"],
	})
	return nil
}

func runModuleTeamRedeem(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	redeemSite := strings.TrimSpace(toRunOptionString(runOptions, "redeem_site"))
	if redeemSite == "" {
		redeemSite = strings.TrimSpace(account.RedeemSite)
	}
	if redeemSite == "" {
		return fmt.Errorf("缺少兑换站点")
	}
	if !strings.HasPrefix(strings.ToLower(redeemSite), "http://") && !strings.HasPrefix(strings.ToLower(redeemSite), "https://") {
		redeemSite = "https://" + redeemSite
	}
	redeemMode := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "redeem_mode")))
	if redeemMode == "" {
		redeemMode = "browser"
	}

	if redeemMode == "http" {
		payload := map[string]any{
			"email": account.Email,
			"code":  account.RedeemCode,
		}
		data, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		redeemPath := strings.TrimSpace(toRunOptionString(runOptions, "redeem_path"))
		if redeemPath == "" {
			redeemPath = "/api/redeem"
		}
		targetURL := strings.TrimRight(redeemSite, "/") + redeemPath
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		client, cleanupProxy, sessionMeta, err := openAutomationHTTPClientWithOptions(
			ctx,
			settings,
			runOptions,
			account,
			executionID,
			constant.AutomationModuleTeamRedeem,
			30*time.Second,
		)
		if err != nil {
			return err
		}
		defer cleanupProxy()
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if resp.StatusCode >= 400 {
			return fmt.Errorf("兑换接口返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleTeamRedeem, "http redeem success", map[string]any{
			"response":    strings.TrimSpace(string(raw)),
			"proxy_mode":  sessionMeta["proxy_mode"],
			"proxy_value": sessionMeta["proxy_value"],
			"proxy_node":  sessionMeta["proxy_node"],
			"target_url":  targetURL,
		})
	} else {
		if err := runPlaywrightRedeemFlow(ctx, executionID, account, redeemSite, runOptions, settings); err != nil {
			return err
		}
	}

	updateAutomationAccountStatus(account.Id, constant.AutomationAccountStatusWaitingInvite, "")
	_ = model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Update("redeem_site", strings.ToLower(strings.TrimSpace(redeemSite))).Error
	account.RedeemSite = strings.ToLower(strings.TrimSpace(redeemSite))
	return nil
}

func runModuleGmailInviteWatch(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	mailboxID := toRunOptionInt(runOptions, "mailbox_id", 0)
	if mailboxID <= 0 {
		mailboxID = account.MailboxId
	}
	if mailboxID <= 0 {
		mailboxID = settings.DefaultMailboxID
	}
	if mailboxID <= 0 {
		return fmt.Errorf("未配置 mailbox_id")
	}

	classificationFilter := normalizeAutomationMailClassification(toRunOptionString(runOptions, "classification"))
	senderKeyword := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "sender_keyword")))
	subjectKeyword := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "subject_keyword")))
	keyword := strings.ToLower(strings.TrimSpace(toRunOptionString(runOptions, "keyword")))
	requireInviteLink := toRunOptionBool(
		runOptions,
		"require_invite_link",
		classificationFilter == constant.ChatMockMailEventTypeInviteLink,
	)
	maxEventAgeSeconds := toRunOptionInt(runOptions, "max_event_age_seconds", 1200)
	if maxEventAgeSeconds < 60 {
		maxEventAgeSeconds = 60
	}
	timeoutSec := toRunOptionInt(runOptions, "wait_timeout_seconds", 120)
	if timeoutSec < 10 {
		timeoutSec = 10
	}
	pollSec := toRunOptionInt(runOptions, "poll_interval_seconds", 15)
	if pollSec < 5 {
		pollSec = 5
	}

	watchStart := common.GetTimestamp()
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("任务已停止")
		}
		events, err := PullChatMockMailboxByID(mailboxID, 50, 25)
		if err != nil {
			appendAutomationLog(executionID, account.Id, "warning", constant.AutomationModuleGmailInviteWatch, "pull mailbox failed", map[string]any{"error": err.Error()})
		} else {
			candidates := events
			if len(candidates) == 0 {
				if rows, _, listErr := model.ListChatMockMailEvents(mailboxID, "", classificationFilter, keyword, 0, 50); listErr == nil {
					candidates = rows
				} else {
					appendAutomationLog(
						executionID,
						account.Id,
						"warning",
						constant.AutomationModuleGmailInviteWatch,
						"list mailbox events failed",
						map[string]any{"error": listErr.Error()},
					)
				}
			}

			for _, event := range candidates {
				if event == nil {
					continue
				}
				if event.ReceivedAt > 0 && event.ReceivedAt < watchStart-int64(maxEventAgeSeconds) {
					continue
				}

				eventType := strings.ToLower(strings.TrimSpace(event.EventType))
				eventClass := strings.ToLower(strings.TrimSpace(event.Classification))
				if classificationFilter != "" && classificationFilter != eventType && classificationFilter != eventClass {
					continue
				}

				sender := strings.ToLower(strings.TrimSpace(event.Sender))
				subject := strings.ToLower(strings.TrimSpace(event.Subject))
				if senderKeyword != "" && !strings.Contains(sender, senderKeyword) {
					continue
				}
				if subjectKeyword != "" && !strings.Contains(subject, subjectKeyword) {
					continue
				}

				parsedPayloadText := strings.ToLower(strings.TrimSpace(event.ParsedPayload))
				rawSnippetText := strings.ToLower(strings.TrimSpace(event.RawSnippet))
				if keyword != "" &&
					!strings.Contains(sender, keyword) &&
					!strings.Contains(subject, keyword) &&
					!strings.Contains(rawSnippetText, keyword) &&
					!strings.Contains(parsedPayloadText, keyword) {
					continue
				}

				inviteLink := extractInviteLinkFromAutomationMailEvent(event)
				if requireInviteLink && inviteLink == "" {
					continue
				}

				inviteCode := strings.TrimSpace(event.Code)
				if inviteCode == "" {
					inviteCode = ExtractChatMockVerificationCode(event.RawSnippet)
				}
				updates := map[string]any{
					"invite_code":    inviteCode,
					"invite_link":    strings.TrimSpace(inviteLink),
					"failure_reason": "",
					"status":         constant.AutomationAccountStatusInviteReceived,
					"updated_time":   common.GetTimestamp(),
					"mailbox_id":     mailboxID,
				}
				if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
					return err
				}
				account.InviteCode = inviteCode
				account.InviteLink = strings.TrimSpace(inviteLink)
				account.MailboxId = mailboxID
				appendAutomationLog(
					executionID,
					account.Id,
					"info",
					constant.AutomationModuleGmailInviteWatch,
					"invite message detected",
					map[string]any{
						"subject":        event.Subject,
						"sender":         event.Sender,
						"classification": event.Classification,
						"event_type":     event.EventType,
						"invite_link":    inviteLink,
						"invite_code":    inviteCode,
					},
				)
				return nil
			}
		}
		if time.Now().After(deadline) {
			updateAutomationAccountStatus(account.Id, constant.AutomationAccountStatusWaitingInvite, "邀请邮件超时")
			return fmt.Errorf("等待邀请邮件超时")
		}
		timer := time.NewTimer(time.Duration(pollSec) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("任务已停止")
		case <-timer.C:
		}
	}
}

func normalizeAutomationMailClassification(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "", "all", "any":
		return ""
	case constant.ChatMockMailEventTypeVerificationCode,
		constant.ChatMockMailEventTypeSystem,
		constant.ChatMockMailEventTypeInviteLink,
		constant.ChatMockMailEventTypeAuthAlert,
		constant.ChatMockMailEventTypeBillingAlert,
		constant.ChatMockMailEventTypeSystemNotice:
		return value
	default:
		return value
	}
}

func extractInviteLinkFromAutomationMailEvent(event *model.ChatMockMailEvent) string {
	if event == nil {
		return ""
	}
	if link := extractFirstURL(event.RawSnippet); link != "" {
		return link
	}
	parsedPayload := map[string]any{}
	if common.UnmarshalJsonStr(strings.TrimSpace(event.ParsedPayload), &parsedPayload) != nil {
		return ""
	}
	for _, key := range []string{"invite_link", "invite_url", "link", "url"} {
		value := strings.TrimSpace(toRunOptionString(parsedPayload, key))
		if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
			return value
		}
	}
	return ""
}

func runPlaywrightRedeemFlow(ctx context.Context, executionID string, account *model.AutomationAccount, redeemSite string, runOptions map[string]any, settings AutomationSettings) error {
	pw, browser, page, closeFn, sessionMeta, err := openPlaywrightPageWithOptions(
		ctx,
		settings,
		toRunOptionBool(runOptions, "headless", settings.PlaywrightHeadless),
		runOptions,
		account,
		executionID,
		constant.AutomationModuleTeamRedeem,
	)
	if err != nil {
		return err
	}
	defer closeFn()
	_ = pw
	_ = browser

	if _, err := page.Goto(redeemSite); err != nil {
		return err
	}
	if err := fillWithSelectorFallback(page, toRunOptionString(runOptions, "code_selector"), account.RedeemCode, []string{"input[name='code']", "input[name='redeemCode']", "input[type='text']"}); err != nil {
		return err
	}
	if strings.TrimSpace(account.Email) != "" {
		_ = fillWithSelectorFallback(page, toRunOptionString(runOptions, "email_selector"), account.Email, []string{"input[type='email']", "input[name='email']"})
	}
	if err := clickWithSelectorFallback(page, toRunOptionString(runOptions, "submit_selector"), []string{"button[type='submit']", "button"}); err != nil {
		return err
	}
	if selector := strings.TrimSpace(toRunOptionString(runOptions, "success_selector")); selector != "" {
		if _, err := page.WaitForSelector(selector); err != nil {
			return err
		}
	} else {
		page.WaitForTimeout(5000)
	}
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleTeamRedeem, "browser redeem flow completed", map[string]any{
		"site":        redeemSite,
		"proxy_mode":  sessionMeta["proxy_mode"],
		"proxy_value": sessionMeta["proxy_value"],
		"proxy_node":  sessionMeta["proxy_node"],
	})
	return nil
}

func openPlaywrightPage(settings AutomationSettings, headless bool) (*playwright.Playwright, playwright.Browser, playwright.Page, func(), error) {
	pw, browser, page, closeFn, _, err := openPlaywrightPageWithOptions(
		context.Background(),
		settings,
		headless,
		nil,
		nil,
		"",
		"",
	)
	return pw, browser, page, closeFn, err
}

func openPlaywrightPageWithOptions(
	ctx context.Context,
	settings AutomationSettings,
	headless bool,
	runOptions map[string]any,
	account *model.AutomationAccount,
	executionID string,
	module string,
) (*playwright.Playwright, playwright.Browser, playwright.Page, func(), map[string]any, error) {
	sessionMeta := map[string]any{
		"proxy_mode":  "",
		"proxy_value": "",
		"proxy_node":  "",
	}
	browserName := strings.TrimSpace(strings.ToLower(settings.PlaywrightBrowser))
	if browserName == "" {
		browserName = "chromium"
	}
	accountID := 0
	if account != nil {
		accountID = account.Id
	}
	pw, err := playwright.Run()
	if err != nil {
		errText := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(errText, "please install the driver") {
			if strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" {
				appendAutomationLog(executionID, accountID, "info", module, "playwright driver missing, attempting auto install", map[string]any{
					"browser": browserName,
				})
			}
			if installErr := ensurePlaywrightInstalled(browserName); installErr != nil {
				return nil, nil, nil, func() {}, sessionMeta, fmt.Errorf("playwright install failed: %w", installErr)
			}
			if strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" {
				appendAutomationLog(executionID, accountID, "info", module, "playwright auto install completed", map[string]any{
					"browser": browserName,
				})
			}
			pw, err = playwright.Run()
		}
	}
	if err != nil {
		return nil, nil, nil, func() {}, sessionMeta, fmt.Errorf("playwright run failed: %w", err)
	}
	launchOpts := playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(headless)}
	if browserName == "chromium" {
		launchOpts.Args = append(launchOpts.Args,
			"--enable-webgl",
			"--use-gl=swiftshader",
			"--use-angle=swiftshader-webgl",
			"--ignore-gpu-blocklist",
			"--enable-accelerated-2d-canvas",
		)
	}
	proxyCleanup := func() {}

	if runOptions != nil {
		candidates, buildErr := buildProxyCandidates(runOptions, settings)
		if buildErr != nil {
			_ = pw.Stop()
			return nil, nil, nil, func() {}, sessionMeta, buildErr
		}
		selected := pickProxyCandidate(candidates, runOptions, account)
		if selected == nil {
			selected, buildErr = resolveDirectProxyCandidate(runOptions, settings)
			if buildErr != nil {
				_ = pw.Stop()
				return nil, nil, nil, func() {}, sessionMeta, buildErr
			}
		}
		launchProxy, resolveErr := resolveLaunchProxy(ctx, selected, runOptions, settings)
		if resolveErr != nil {
			_ = pw.Stop()
			return nil, nil, nil, func() {}, sessionMeta, resolveErr
		}
		if launchProxy != nil {
			if launchProxy.Cleanup != nil {
				proxyCleanup = launchProxy.Cleanup
			}
			launchOpts.Proxy = &playwright.Proxy{
				Server: launchProxy.Server,
			}
			if strings.TrimSpace(launchProxy.Username) != "" {
				launchOpts.Proxy.Username = playwright.String(launchProxy.Username)
			}
			if strings.TrimSpace(launchProxy.Password) != "" {
				launchOpts.Proxy.Password = playwright.String(launchProxy.Password)
			}
			sessionMeta["proxy_mode"] = launchProxy.Mode
			sessionMeta["proxy_value"] = launchProxy.Server
			sessionMeta["proxy_node"] = launchProxy.NodeName
			if strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" {
				accountID := 0
				if account != nil {
					accountID = account.Id
				}
				appendAutomationLog(executionID, accountID, "info", module, "proxy selected", map[string]any{
					"mode":  launchProxy.Mode,
					"proxy": launchProxy.Server,
					"node":  launchProxy.NodeName,
				})
			}
		}
	}

	var browser playwright.Browser
	switch browserName {
	case "firefox":
		browser, err = pw.Firefox.Launch(launchOpts)
	case "webkit":
		browser, err = pw.WebKit.Launch(launchOpts)
	default:
		browser, err = pw.Chromium.Launch(launchOpts)
	}
	if err != nil {
		_ = pw.Stop()
		return nil, nil, nil, func() {}, sessionMeta, fmt.Errorf("playwright launch failed: %w", err)
	}

	moduleName := strings.TrimSpace(strings.ToLower(module))
	isAutoRegister := moduleName == constant.AutomationModuleAutoRegister
	contextOpts := playwright.BrowserNewContextOptions{
		JavaScriptEnabled: playwright.Bool(true),
	}
	strictDefaultHeaders := false
	if isAutoRegister {
		strictDefaultHeaders = true
		requestedStrict := toRunOptionBool(runOptions, "strict_default_headers", true)
		requestedUA := strings.TrimSpace(toRunOptionString(runOptions, "user_agent"))
		requestedHeaders := parseStringMapFromOption(toRunOptionString(runOptions, "extra_headers_json"))
		if strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" && (!requestedStrict || requestedUA != "" || len(requestedHeaders) > 0) {
			appendAutomationLog(executionID, accountID, "info", module, "strict default headers enforced for auto register", map[string]any{
				"requested_strict_default_headers": requestedStrict,
				"ignored_user_agent":               requestedUA != "",
				"ignored_extra_headers":            len(requestedHeaders),
			})
		}
	} else {
		strictDefaultHeaders = toRunOptionBool(runOptions, "strict_default_headers", false)
	}
	if !strictDefaultHeaders {
		userAgent := strings.TrimSpace(toRunOptionString(runOptions, "user_agent"))
		if userAgent == "" {
			userAgent = strings.TrimSpace(settings.RegisterUserAgent)
		}
		if userAgent != "" {
			contextOpts.UserAgent = playwright.String(userAgent)
		}
		extraHeaders := parseStringMapFromOption(toRunOptionString(runOptions, "extra_headers_json"))
		if len(extraHeaders) > 0 {
			contextOpts.ExtraHttpHeaders = extraHeaders
		}
	}
	browserContext, err := browser.NewContext(contextOpts)
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, nil, func() {}, sessionMeta, err
	}
	page, err := browserContext.NewPage()
	if err != nil {
		_ = browserContext.Close()
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, nil, func() {}, sessionMeta, err
	}
	if isAutoRegister && strings.TrimSpace(executionID) != "" && strings.TrimSpace(module) != "" {
		appendAutomationLog(executionID, accountID, "info", module, "browser capability snapshot", browserCapabilitySnapshot(page))
	}
	closeFn := func() {
		_ = browserContext.Close()
		_ = browser.Close()
		_ = pw.Stop()
		proxyCleanup()
	}
	return pw, browser, page, closeFn, sessionMeta, nil
}

func browserCapabilitySnapshot(page playwright.Page) map[string]any {
	base := map[string]any{
		"javascript_enabled": true,
	}
	if page == nil {
		base["snapshot_error"] = "nil page"
		return base
	}
	result, err := page.Evaluate(`() => {
		const canvas = document.createElement('canvas');
		let webglEnabled = false;
		let webgl2Enabled = false;
		let canvas2dEnabled = false;
		try {
			canvas2dEnabled = !!canvas.getContext('2d');
		} catch (_err) {}
		try {
			webglEnabled = !!(canvas.getContext('webgl') || canvas.getContext('experimental-webgl'));
		} catch (_err) {}
		try {
			webgl2Enabled = !!canvas.getContext('webgl2');
		} catch (_err) {}
		return {
			cookies_enabled: !!navigator.cookieEnabled,
			canvas2d_enabled: canvas2dEnabled,
			webgl_enabled: webglEnabled,
			webgl2_enabled: webgl2Enabled
		};
	}`)
	if err != nil {
		base["snapshot_error"] = err.Error()
		return base
	}
	if data, ok := result.(map[string]interface{}); ok {
		for k, v := range data {
			base[k] = v
		}
		return base
	}
	base["snapshot_raw"] = fmt.Sprintf("%v", result)
	return base
}

func ensurePlaywrightInstalled(browserName string) error {
	playwrightInstallMu.Lock()
	defer playwrightInstallMu.Unlock()

	target := strings.TrimSpace(strings.ToLower(browserName))
	switch target {
	case "chromium", "firefox", "webkit":
	default:
		target = "chromium"
	}

	options := &playwright.RunOptions{
		Browsers: []string{target},
		Verbose:  true,
	}
	return playwright.Install(options)
}

func splitSelectors(primary string, fallbacks []string) []string {
	seen := map[string]struct{}{}
	selectors := make([]string, 0, len(fallbacks)+2)
	add := func(value string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		selectors = append(selectors, v)
	}
	for _, part := range strings.Split(primary, ",") {
		add(part)
	}
	for _, item := range fallbacks {
		add(item)
	}
	return selectors
}

func fillWithSelectorFallback(page playwright.Page, primary string, value string, fallbacks []string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("缺少填写值")
	}
	selectors := splitSelectors(primary, fallbacks)
	for _, selector := range selectors {
		if err := page.Fill(selector, value, playwright.PageFillOptions{
			Timeout: playwright.Float(4000),
		}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("未找到可填写输入框")
}

func fillWithSelectorTypingFallback(page playwright.Page, primary string, value string, fallbacks []string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("缺少填写值")
	}
	selectors := splitSelectors(primary, fallbacks)
	for _, selector := range selectors {
		if err := page.Click(selector, playwright.PageClickOptions{
			Timeout: playwright.Float(4000),
		}); err != nil {
			continue
		}
		_ = page.Fill(selector, "", playwright.PageFillOptions{
			Timeout: playwright.Float(2000),
		})
		delay := float64(70 + rand.Intn(91))
		if err := page.Type(selector, value, playwright.PageTypeOptions{
			Delay: playwright.Float(delay),
		}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("未找到可填写输入框")
}

func humanPause(page playwright.Page, minMs int, maxMs int) {
	if page == nil {
		return
	}
	if minMs < 0 {
		minMs = 0
	}
	if minMs > 0 && minMs < 1000 {
		minMs = 1000
	}
	if maxMs > 0 && maxMs < 1000 {
		maxMs = 1000
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	waitMs := minMs
	if maxMs > minMs {
		waitMs += rand.Intn(maxMs - minMs + 1)
	}
	page.WaitForTimeout(float64(waitMs))
}

func clickWithSelectorFallback(page playwright.Page, primary string, fallbacks []string) error {
	selectors := splitSelectors(primary, fallbacks)
	for _, selector := range selectors {
		if err := page.Click(selector, playwright.PageClickOptions{
			Timeout: playwright.Float(2500),
		}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("未找到可点击按钮")
}

func ensureExecutionAccountDir(settings AutomationSettings, executionID string, accountID int) (string, error) {
	root := strings.TrimSpace(settings.ArtifactRoot)
	if root == "" {
		return "", fmt.Errorf("artifact root is empty")
	}
	dir := filepath.Join(root, strings.TrimSpace(executionID), fmt.Sprintf("account-%d", accountID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

func recordAutomationArtifact(executionID string, accountID int, module string, artifactType string, path string, meta any) {
	entry := &model.AutomationArtifact{
		ExecutionID:  executionID,
		AccountID:    accountID,
		Module:       module,
		ArtifactType: artifactType,
		Path:         path,
	}
	if data, err := os.ReadFile(path); err == nil {
		sum := sha256.Sum256(data)
		entry.Checksum = fmt.Sprintf("%x", sum)
	}
	if meta != nil {
		if raw, err := common.Marshal(meta); err == nil {
			entry.Meta = string(raw)
		}
	}
	if err := entry.Insert(); err != nil {
		common.SysError("failed to insert automation artifact: " + err.Error())
	}
}

func runModuleBrowserLoginCapture(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	loginURL := strings.TrimSpace(toRunOptionString(runOptions, "login_url"))
	if loginURL == "" {
		return fmt.Errorf("缺少 login_url")
	}
	password := strings.TrimSpace(account.Password)
	if password == "" {
		password = strings.TrimSpace(toRunOptionString(runOptions, "password"))
	}
	if password == "" {
		return fmt.Errorf("缺少登录密码")
	}
	headless := toRunOptionBool(runOptions, "headless", settings.PlaywrightHeadless)
	timeoutSec := toRunOptionInt(runOptions, "timeout_seconds", 90)
	if timeoutSec < 10 {
		timeoutSec = 10
	}
	pw, browser, page, closeFn, sessionMeta, err := openPlaywrightPageWithOptions(
		ctx,
		settings,
		headless,
		runOptions,
		account,
		executionID,
		constant.AutomationModuleBrowserLoginCapture,
	)
	if err != nil {
		return err
	}
	defer closeFn()
	_ = pw
	_ = browser

	if _, err := page.Goto(loginURL); err != nil {
		return err
	}
	if err := fillWithSelectorFallback(page, toRunOptionString(runOptions, "username_selector"), account.Email, []string{"input[type='email']", "input[name='email']", "input[name='username']"}); err != nil {
		return err
	}
	if err := fillWithSelectorFallback(page, toRunOptionString(runOptions, "password_selector"), password, []string{"input[type='password']"}); err != nil {
		return err
	}
	if err := clickWithSelectorFallback(page, toRunOptionString(runOptions, "submit_selector"), []string{"button[type='submit']", "button"}); err != nil {
		return err
	}
	if selector := strings.TrimSpace(toRunOptionString(runOptions, "success_selector")); selector != "" {
		if _, err := page.WaitForSelector(selector); err != nil {
			return err
		}
	} else {
		page.WaitForTimeout(float64(timeoutSec * 1000))
	}

	pageContext := page.Context()
	dir, err := ensureExecutionAccountDir(settings, executionID, account.Id)
	if err != nil {
		return err
	}
	statePath := filepath.Join(dir, "storage-state.json")
	if _, err := pageContext.StorageState(statePath); err != nil {
		return err
	}
	cookies, err := pageContext.Cookies()
	if err != nil {
		return err
	}
	cookiesRaw, err := common.Marshal(cookies)
	if err != nil {
		return err
	}
	cookiePath := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(cookiePath, cookiesRaw, 0o600); err != nil {
		return err
	}
	recordAutomationArtifact(executionID, account.Id, constant.AutomationModuleBrowserLoginCapture, "storage_state", statePath, map[string]any{
		"login_url":   loginURL,
		"proxy_mode":  sessionMeta["proxy_mode"],
		"proxy_value": sessionMeta["proxy_value"],
		"proxy_node":  sessionMeta["proxy_node"],
	})
	recordAutomationArtifact(executionID, account.Id, constant.AutomationModuleBrowserLoginCapture, "cookies", cookiePath, nil)
	updates := map[string]any{
		"session_artifact_path": statePath,
		"status":                constant.AutomationAccountStatusLoginCaptured,
		"failure_reason":        "",
		"updated_time":          common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	account.SessionArtifactPath = statePath
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleBrowserLoginCapture, "browser login capture completed", map[string]any{
		"login_url":   loginURL,
		"proxy_mode":  sessionMeta["proxy_mode"],
		"proxy_value": sessionMeta["proxy_value"],
		"proxy_node":  sessionMeta["proxy_node"],
	})
	return nil
}

func runModuleSessionConvert(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	sourcePath := strings.TrimSpace(toRunOptionString(runOptions, "source_path"))
	if sourcePath == "" {
		sourcePath = strings.TrimSpace(account.SessionArtifactPath)
	}
	if sourcePath == "" {
		return fmt.Errorf("缺少 session 文件路径")
	}
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	parsed := map[string]any{}
	if err := common.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("session 文件解析失败: %w", err)
	}
	cookiesRaw, ok := parsed["cookies"]
	if !ok {
		return fmt.Errorf("session 文件缺少 cookies")
	}
	cookieCount := 0
	switch typed := cookiesRaw.(type) {
	case []any:
		cookieCount = len(typed)
	}
	minCookies := toRunOptionInt(runOptions, "validate_cookie_min", 1)
	if cookieCount < minCookies {
		return fmt.Errorf("cookie 数量不足，期望至少 %d，实际 %d", minCookies, cookieCount)
	}
	dir, err := ensureExecutionAccountDir(settings, executionID, account.Id)
	if err != nil {
		return err
	}
	authPath := filepath.Join(dir, "auth-artifact.json")
	converted := map[string]any{
		"email":                 account.Email,
		"source":                "playwright_storage_state",
		"session_artifact_path": sourcePath,
		"cookies":               cookiesRaw,
		"generated_at":          common.GetTimestamp(),
		"artifact_version":      "v1",
	}
	convertedRaw, err := common.Marshal(converted)
	if err != nil {
		return err
	}
	if err := os.WriteFile(authPath, convertedRaw, 0o600); err != nil {
		return err
	}
	recordAutomationArtifact(executionID, account.Id, constant.AutomationModuleSessionConvert, "auth_artifact", authPath, map[string]any{"cookie_count": cookieCount})
	updates := map[string]any{
		"auth_artifact_path": authPath,
		"status":             constant.AutomationAccountStatusConverted,
		"failure_reason":     "",
		"updated_time":       common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	account.AuthArtifactPath = authPath
	return nil
}

func resolveAccountImportAPIKey(account *model.AutomationAccount, runOptions map[string]any) string {
	apiKey := strings.TrimSpace(toRunOptionString(runOptions, "api_key"))
	if apiKey != "" {
		return apiKey
	}
	if strings.TrimSpace(account.Metadata) == "" {
		return ""
	}
	metadata := map[string]any{}
	if err := common.UnmarshalJsonStr(account.Metadata, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(toRunOptionString(metadata, "api_key"))
}

func importAccountToNewAPI(account *model.AutomationAccount, runOptions map[string]any) (string, error) {
	apiKey := resolveAccountImportAPIKey(account, runOptions)
	if apiKey == "" {
		return "", fmt.Errorf("缺少导入 API Key（可在 run_options.api_key 或账号 metadata.api_key 提供）")
	}
	baseURL := "https://api.openai.com"
	if strings.TrimSpace(account.AuthArtifactPath) != "" {
		baseURL = "https://chatgpt.com"
	}
	existing, err := model.GetChatMockAccountByIdentity(account.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if existing == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newAccount := &model.ChatMockAccount{
			Name:     strings.TrimSpace(account.Name),
			Account:  strings.TrimSpace(account.Email),
			ApiKey:   apiKey,
			BaseURL:  baseURL,
			Status:   constant.ChatMockAccountStatusEnabled,
			Weight:   1,
			Priority: 100,
			Metadata: account.Metadata,
		}
		if err := newAccount.Insert(); err != nil {
			return "", err
		}
		return fmt.Sprintf("chatmock_account:%d", newAccount.Id), nil
	}
	existing.ApiKey = apiKey
	existing.Name = strings.TrimSpace(account.Name)
	existing.BaseURL = baseURL
	existing.Status = constant.ChatMockAccountStatusEnabled
	existing.Metadata = account.Metadata
	if err := existing.Update(); err != nil {
		return "", err
	}
	return fmt.Sprintf("chatmock_account:%d", existing.Id), nil
}

func shouldUseCLIProxyCPAImport(endpoint string, runOptions map[string]any) (use bool, forced bool) {
	mode := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "cpa_mode")))
	switch mode {
	case "legacy", "custom":
		return false, true
	case "cliproxy", "management", "auth_file", "auth-files":
		return true, true
	}
	path := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "cpa_path")))
	if strings.Contains(path, "auth-files") || strings.Contains(path, "/v0/management") {
		return true, false
	}
	return strings.Contains(strings.ToLower(endpoint), "/v0/management"), false
}

func sanitizeCPAAuthFileName(raw string) string {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "-"))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.', r == '@':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	name := strings.Trim(builder.String(), "-.")
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		name += ".json"
	}
	return name
}

func resolveAccountCPAProvider(account *model.AutomationAccount, runOptions map[string]any) string {
	provider := strings.TrimSpace(toRunOptionString(runOptions, "cpa_provider"))
	if provider != "" {
		return provider
	}
	if strings.TrimSpace(account.Metadata) == "" {
		return ""
	}
	metadata := map[string]any{}
	if err := common.UnmarshalJsonStr(account.Metadata, &metadata); err != nil {
		return ""
	}
	for _, key := range []string{"cpa_provider", "provider", "type", "account_type"} {
		value := strings.TrimSpace(toRunOptionString(metadata, key))
		if value != "" {
			return value
		}
	}
	return ""
}

func buildCPAAuthFilePayload(account *model.AutomationAccount, runOptions map[string]any) ([]byte, string, error) {
	payload := map[string]any{}
	authPath := strings.TrimSpace(toRunOptionString(runOptions, "cpa_auth_file_path"))
	if authPath == "" {
		authPath = strings.TrimSpace(account.AuthArtifactPath)
	}
	if authPath != "" {
		raw, err := os.ReadFile(authPath)
		if err != nil {
			return nil, "", fmt.Errorf("读取 auth 文件失败: %w", err)
		}
		if err := common.Unmarshal(raw, &payload); err != nil {
			return nil, "", fmt.Errorf("auth 文件不是有效 JSON: %w", err)
		}
	}
	if len(payload) == 0 {
		payload = map[string]any{
			"source":                "newapi_team_automation",
			"generated_at":          common.GetTimestamp(),
			"session_artifact_path": strings.TrimSpace(account.SessionArtifactPath),
			"auth_artifact_path":    strings.TrimSpace(account.AuthArtifactPath),
		}
	}
	if strings.TrimSpace(toRunOptionString(payload, "email")) == "" && strings.TrimSpace(account.Email) != "" {
		payload["email"] = strings.TrimSpace(account.Email)
	}
	if strings.TrimSpace(toRunOptionString(payload, "name")) == "" && strings.TrimSpace(account.Name) != "" {
		payload["name"] = strings.TrimSpace(account.Name)
	}
	if strings.TrimSpace(toRunOptionString(payload, "type")) == "" {
		if provider := resolveAccountCPAProvider(account, runOptions); provider != "" {
			payload["type"] = provider
		}
	}

	fileName := strings.TrimSpace(toRunOptionString(runOptions, "cpa_file_name"))
	if fileName == "" {
		email := strings.TrimSpace(toRunOptionString(payload, "email"))
		if email == "" {
			email = strings.TrimSpace(account.Email)
		}
		if email == "" {
			email = fmt.Sprintf("account-%d", account.Id)
		}
		prefix := strings.TrimSpace(toRunOptionString(payload, "type"))
		if prefix != "" {
			fileName = fmt.Sprintf("%s-%s.json", prefix, email)
		} else {
			fileName = email + ".json"
		}
	}
	fileName = sanitizeCPAAuthFileName(fileName)
	if fileName == "" {
		fileName = fmt.Sprintf("account-%d.json", account.Id)
	}

	data, err := common.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return data, fileName, nil
}

func buildCLIProxyCPATargetURL(endpoint string, runOptions map[string]any) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	path := strings.TrimSpace(toRunOptionString(runOptions, "cpa_path"))
	if path == "" {
		if strings.Contains(strings.ToLower(endpoint), "/v0/management") {
			path = "/auth-files"
		} else {
			path = "/v0/management/auth-files"
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.Contains(strings.ToLower(endpoint), "/v0/management") && strings.HasPrefix(strings.ToLower(path), "/v0/management/") {
		path = strings.TrimPrefix(path, "/v0/management")
	}
	return endpoint + path
}

func importAccountToCPACliproxy(ctx context.Context, settings AutomationSettings, account *model.AutomationAccount, runOptions map[string]any) (string, error) {
	targetURL := buildCLIProxyCPATargetURL(settings.CPAEndpoint, runOptions)
	data, fileName, err := buildCPAAuthFilePayload(account, runOptions)
	if err != nil {
		return "", err
	}
	separator := "?"
	if strings.Contains(targetURL, "?") {
		separator = "&"
	}
	targetURL = targetURL + separator + "name=" + url.QueryEscape(fileName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	token := strings.TrimSpace(settings.CPAToken)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Management-Key", token)
	}
	client, cleanupProxy, _, err := openAutomationHTTPClientWithOptions(
		ctx,
		settings,
		runOptions,
		account,
		"",
		constant.AutomationModuleImportTarget,
		30*time.Second,
	)
	if err != nil {
		return "", err
	}
	defer cleanupProxy()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("CPA(auth-files) 导入失败 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return "cpa_auth_file:" + fileName, nil
}

func importAccountToCPALegacy(ctx context.Context, settings AutomationSettings, account *model.AutomationAccount, runOptions map[string]any) (string, error) {
	path := strings.TrimSpace(toRunOptionString(runOptions, "cpa_path"))
	if path == "" {
		path = "/api/accounts/import"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	targetURL := strings.TrimRight(settings.CPAEndpoint, "/") + path
	payload := map[string]any{
		"email":                 account.Email,
		"name":                  account.Name,
		"status":                account.Status,
		"redeem_code":           account.RedeemCode,
		"session_artifact_path": account.SessionArtifactPath,
		"auth_artifact_path":    account.AuthArtifactPath,
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(settings.CPAToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(settings.CPAToken))
	}
	client, cleanupProxy, _, err := openAutomationHTTPClientWithOptions(
		ctx,
		settings,
		runOptions,
		account,
		"",
		constant.AutomationModuleImportTarget,
		30*time.Second,
	)
	if err != nil {
		return "", err
	}
	defer cleanupProxy()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("CPA 导入失败 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	payloadMap := map[string]any{}
	if len(raw) > 0 {
		_ = common.Unmarshal(raw, &payloadMap)
	}
	if id := strings.TrimSpace(toRunOptionString(payloadMap, "id")); id != "" {
		return "cpa:" + id, nil
	}
	return "cpa:ok", nil
}

func importAccountToCPA(ctx context.Context, settings AutomationSettings, account *model.AutomationAccount, runOptions map[string]any) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(settings.CPAEndpoint), "/")
	if endpoint == "" {
		return "", fmt.Errorf("未配置 AutomationCPAEndpoint")
	}
	settings.CPAEndpoint = endpoint
	useCliproxy, forced := shouldUseCLIProxyCPAImport(endpoint, runOptions)
	if useCliproxy {
		ref, err := importAccountToCPACliproxy(ctx, settings, account, runOptions)
		if err == nil || forced {
			return ref, err
		}
	}
	return importAccountToCPALegacy(ctx, settings, account, runOptions)
}

func runModuleImportTarget(ctx context.Context, executionID string, account *model.AutomationAccount, runOptions map[string]any, settings AutomationSettings) error {
	target := strings.TrimSpace(strings.ToLower(toRunOptionString(runOptions, "target")))
	if target == "" {
		target = "newapi"
	}
	refs := []string{}
	if target == "newapi" || target == "both" {
		ref, err := importAccountToNewAPI(account, runOptions)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		channel, _, models, syncErr := SyncChatMockPoolChannel()
		if syncErr != nil {
			return fmt.Errorf("导入后同步 ChatMockPool 渠道失败: %w", syncErr)
		}
		refs = append(refs, fmt.Sprintf("chatmock_channel:%d", channel.Id))
		appendAutomationLog(
			executionID,
			account.Id,
			"info",
			constant.AutomationModuleImportTarget,
			"chatmock pool channel synced",
			map[string]any{
				"channel_id":   channel.Id,
				"channel_name": channel.Name,
				"models_count": len(models),
			},
		)
	}
	if target == "cpa" || target == "both" {
		ref, err := importAccountToCPA(ctx, settings, account, runOptions)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return fmt.Errorf("未识别导入目标: %s", target)
	}
	updates := map[string]any{
		"imported_target":    target,
		"imported_reference": strings.Join(refs, ","),
		"status":             constant.AutomationAccountStatusImported,
		"failure_reason":     "",
		"updated_time":       common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.AutomationAccount{}).Where("id = ?", account.Id).Updates(updates).Error; err != nil {
		return err
	}
	appendAutomationLog(executionID, account.Id, "info", constant.AutomationModuleImportTarget, "import completed", map[string]any{"refs": refs})
	return nil
}

func extractFirstURL(text string) string {
	match := automationURLPattern.FindString(strings.TrimSpace(text))
	return strings.TrimSpace(match)
}

func BuildAutomationMeta(canConfigure bool) map[string]any {
	settings := LoadAutomationSettings()
	base := settings.Sanitized()
	delete(base, "register_proxy_pool")
	delete(base, "worker_password")
	delete(base, "cloudflare_api_token")
	delete(base, "cpa_token")
	mode := normalizeAutomationMode(settings.Mode)
	base["mode"] = mode
	base["enabled"] = mode != constant.AutomationModeDisabled
	base["can_configure"] = canConfigure
	base["version"] = common.Version
	base["modules"] = GetAutomationModuleDefinitions()
	base["worker_status"] = map[string]any{
		"mode":        mode,
		"running":     mode == constant.AutomationModeLocal,
		"local_ready": mode == constant.AutomationModeLocal,
	}
	base["proxy_pool_summary"] = BuildAutomationProxyPoolSummary(settings.RegisterProxyPool, settings.RegisterProxyGateway)
	base["capabilities"] = map[string]any{
		"module":          true,
		"account_filter":  true,
		"redeem_site":     true,
		"run_options":     true,
		"local_execution": mode == constant.AutomationModeLocal,
		"remote_proxy":    mode == constant.AutomationModeRemote,
	}
	base["source"] = "local"

	if mode == constant.AutomationModeRemote {
		client, err := NewTeamAutomationClient(settings.ToRemoteTeamAutomationConfig())
		if err != nil {
			base["reachable"] = false
			base["warning"] = err.Error()
			return base
		}
		_, statusCode, err := client.Do(context.Background(), http.MethodGet, "/api/health", nil, nil)
		if err != nil {
			base["reachable"] = false
			base["warning"] = err.Error()
			base["remote_status"] = statusCode
			return base
		}
		base["reachable"] = true
		base["source"] = "remote"
		base["remote_status"] = statusCode
	}
	return base
}

func TestAutomationSettings(settings AutomationSettings) (map[string]any, error) {
	settings.Mode = normalizeAutomationMode(settings.Mode)
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	result := map[string]any{
		"mode": settings.Mode,
	}
	if settings.Mode == constant.AutomationModeLocal {
		if err := settings.EnsureArtifactRoot(); err != nil {
			return nil, err
		}
		result["artifact_root"] = settings.ArtifactRoot
		result["local_ready"] = true
		result["proxy_pool_summary"] = BuildAutomationProxyPoolSummary(settings.RegisterProxyPool, settings.RegisterProxyGateway)
		return result, nil
	}
	if settings.Mode == constant.AutomationModeRemote {
		client, err := NewTeamAutomationClient(settings.ToRemoteTeamAutomationConfig())
		if err != nil {
			return nil, err
		}
		health, statusCode, err := client.Do(context.Background(), http.MethodGet, "/api/health", nil, nil)
		if err != nil {
			return nil, err
		}
		result["remote_status"] = statusCode
		result["health"] = health
		result["proxy_pool_summary"] = BuildAutomationProxyPoolSummary(settings.RegisterProxyPool, settings.RegisterProxyGateway)
		return result, nil
	}
	result["disabled"] = true
	result["proxy_pool_summary"] = BuildAutomationProxyPoolSummary(settings.RegisterProxyPool, settings.RegisterProxyGateway)
	return result, nil
}

func DecodeRunOptions(raw string) map[string]any {
	payload := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return payload
	}
	if err := common.UnmarshalJsonStr(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func BuildExecutionDetail(execution *model.AutomationExecution) map[string]any {
	if execution == nil {
		return map[string]any{}
	}
	runOptions := DecodeRunOptions(execution.RunOptions)
	artifacts, _ := model.ListAutomationArtifactsByExecution(execution.ExecutionID)
	artifactRows := make([]map[string]any, 0, len(artifacts))
	for _, item := range artifacts {
		if item == nil {
			continue
		}
		artifactRows = append(artifactRows, map[string]any{
			"id":            item.Id,
			"account_id":    item.AccountID,
			"module":        item.Module,
			"artifact_type": item.ArtifactType,
			"path":          item.Path,
			"checksum":      item.Checksum,
			"meta":          item.Meta,
			"created_at":    item.CreatedAtTs,
		})
	}
	return map[string]any{
		"id":             execution.Id,
		"execution_id":   execution.ExecutionID,
		"module":         execution.Module,
		"mode":           execution.Mode,
		"status":         execution.Status,
		"progress":       execution.Progress,
		"total_accounts": execution.TotalAccounts,
		"success_count":  execution.SuccessCount,
		"failed_count":   execution.FailedCount,
		"run_options":    runOptions,
		"result_summary": execution.ResultSummary,
		"error_message":  execution.ErrorMessage,
		"created_at":     execution.CreatedAtTs,
		"updated_at":     execution.UpdatedAtTs,
		"started_at":     execution.StartedAt,
		"finished_at":    execution.FinishedAt,
		"artifacts":      artifactRows,
		"running":        isAutomationRunning(execution.ExecutionID),
	}
}

type AutomationExecutionHistoryFilter struct {
	Status           string
	Module           string
	Search           string
	OnlyFailed       bool
	OnlyProxyRetried bool
}

func BuildExecutionHistory(page int, pageSize int, filter AutomationExecutionHistoryFilter) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	filter.Status = strings.TrimSpace(strings.ToLower(filter.Status))
	filter.Module = strings.TrimSpace(strings.ToLower(filter.Module))
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.OnlyFailed {
		filter.Status = constant.AutomationExecutionStatusFailed
	}
	offset := (page - 1) * pageSize
	list, total, err := model.ListAutomationExecutions(offset, pageSize, model.AutomationExecutionListOptions{
		Status:           filter.Status,
		Module:           filter.Module,
		Search:           filter.Search,
		OnlyProxyRetried: filter.OnlyProxyRetried,
	})
	if err != nil {
		return nil, err
	}
	executionIDs := make([]string, 0, len(list))
	for _, item := range list {
		if item == nil {
			continue
		}
		executionIDs = append(executionIDs, item.ExecutionID)
	}
	proxyRetryCounts, err := model.CountAutomationExecutionLogsByMessage(executionIDs, "proxy retry scheduled")
	if err != nil {
		return nil, err
	}
	history := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if item == nil {
			continue
		}
		history = append(history, map[string]any{
			"execution_id":      item.ExecutionID,
			"module":            item.Module,
			"mode":              item.Mode,
			"status":            item.Status,
			"progress":          item.Progress,
			"total_accounts":    item.TotalAccounts,
			"success_count":     item.SuccessCount,
			"failed_count":      item.FailedCount,
			"error_message":     item.ErrorMessage,
			"failed_summary":    summarizeExecutionFailureText(item.ResultSummary, item.ErrorMessage),
			"proxy_retry_count": proxyRetryCounts[strings.TrimSpace(item.ExecutionID)],
			"started_at":        item.StartedAt,
			"finished_at":       item.FinishedAt,
			"created_at":        item.CreatedAtTs,
		})
	}
	return map[string]any{
		"items":              history,
		"total":              total,
		"page":               page,
		"page_size":          pageSize,
		"status":             filter.Status,
		"module":             filter.Module,
		"search":             filter.Search,
		"only_failed":        filter.OnlyFailed,
		"only_proxy_retried": filter.OnlyProxyRetried,
	}, nil
}

func BuildExecutionLogs(executionID string, page int, pageSize int) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize
	rows, total, err := model.ListAutomationExecutionLogs(executionID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	logs := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		logs = append(logs, map[string]any{
			"id":           row.Id,
			"execution_id": row.ExecutionID,
			"account_id":   row.AccountID,
			"level":        row.Level,
			"step":         row.Step,
			"message":      row.Message,
			"payload":      row.Payload,
			"created_at":   row.CreatedAtTs,
		})
	}
	return map[string]any{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, nil
}

func ParseExecutionResultSummary(raw string) map[string]any {
	parsed := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return parsed
	}
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return map[string]any{"raw": raw}
	}
	return parsed
}

func summarizeExecutionFailureText(raw string, fallback string) string {
	summary := ParseExecutionResultSummary(raw)
	if len(summary) == 0 {
		return strings.TrimSpace(fallback)
	}
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, 2)
	totalFailed := 0
	for _, key := range keys {
		payload, ok := summary[key].(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprintf("%v", payload["status"])) != "failed" {
			continue
		}
		totalFailed++
		if len(parts) >= 2 {
			continue
		}
		module := strings.TrimSpace(fmt.Sprintf("%v", payload["module"]))
		errText := strings.TrimSpace(fmt.Sprintf("%v", payload["error"]))
		label := strings.TrimPrefix(key, "account_")
		if label == "" {
			label = key
		}
		text := "账号#" + label
		if module != "" {
			text += "[" + module + "]"
		}
		if errText != "" {
			text += " " + errText
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(fallback)
	}
	text := strings.Join(parts, "; ")
	if totalFailed > len(parts) {
		text += fmt.Sprintf(" 等%d项", totalFailed-len(parts))
	}
	return text
}

func SaveAutomationSettings(settings AutomationSettings) error {
	settings.Mode = normalizeAutomationMode(settings.Mode)
	if settings.ArtifactRoot == "" {
		settings.ArtifactRoot = "/data/automation-artifacts"
	}
	if settings.PlaywrightBrowser == "" {
		settings.PlaywrightBrowser = "chromium"
	}
	if strings.TrimSpace(settings.RegisterEntryURL) == "" {
		settings.RegisterEntryURL = "https://chatgpt.com"
	}
	if settings.WorkerTimeoutSeconds <= 0 {
		settings.WorkerTimeoutSeconds = 30
	}
	if err := settings.Validate(); err != nil {
		return err
	}

	updates := map[string]string{
		"AutomationMode":                   settings.Mode,
		"AutomationWorkerBaseURL":          strings.TrimRight(strings.TrimSpace(settings.WorkerBaseURL), "/"),
		"AutomationWorkerUsername":         strings.TrimSpace(settings.WorkerUsername),
		"AutomationWorkerPasswordSecret":   strings.TrimSpace(settings.WorkerPassword),
		"AutomationWorkerTimeoutSeconds":   strconv.Itoa(settings.WorkerTimeoutSeconds),
		"AutomationWorkerVerifySSLEnabled": strconv.FormatBool(settings.WorkerVerifySSL),
		"AutomationArtifactRoot":           strings.TrimSpace(settings.ArtifactRoot),
		"AutomationPlaywrightHeadless":     strconv.FormatBool(settings.PlaywrightHeadless),
		"AutomationPlaywrightBrowser":      strings.TrimSpace(strings.ToLower(settings.PlaywrightBrowser)),
		"AutomationRegisterEntryURL":       strings.TrimSpace(settings.RegisterEntryURL),
		"AutomationRegisterProxyPool":      strings.TrimSpace(settings.RegisterProxyPool),
		"AutomationRegisterProxyGateway":   strings.TrimSpace(settings.RegisterProxyGateway),
		"AutomationRegisterUserAgent":      strings.TrimSpace(settings.RegisterUserAgent),
		"AutomationCloudflareApiToken":     strings.TrimSpace(settings.CloudflareAPIToken),
		"AutomationCloudflareApiEmail":     strings.TrimSpace(settings.CloudflareAPIEmail),
		"AutomationCloudflareZoneID":       strings.TrimSpace(settings.CloudflareZoneID),
		"AutomationCloudflareDomain":       strings.TrimSpace(strings.ToLower(settings.CloudflareDomain)),
		"AutomationTempmailBaseURL":        strings.TrimRight(strings.TrimSpace(settings.TempMailBaseURL), "/"),
		"AutomationTempmailAdminAuth":      strings.TrimSpace(settings.TempMailAdminAuth),
		"AutomationTempmailDomain":         strings.TrimSpace(strings.ToLower(settings.TempMailDomain)),
		"AutomationDefaultMailboxID":       strconv.Itoa(settings.DefaultMailboxID),
		"AutomationCPAEndpoint":            strings.TrimRight(strings.TrimSpace(settings.CPAEndpoint), "/"),
		"AutomationCPAToken":               strings.TrimSpace(settings.CPAToken),
	}

	legacyEnabled := "false"
	legacyMode := "disabled"
	if settings.Mode == constant.AutomationModeLocal {
		legacyEnabled = "true"
		legacyMode = "local_playwright"
	}
	if settings.Mode == constant.AutomationModeRemote {
		legacyEnabled = "true"
		legacyMode = "remote_proxy"
	}
	updates["TeamAutomationEnabled"] = legacyEnabled
	updates["TeamAutomationMode"] = legacyMode
	updates["TeamAutomationBaseURL"] = strings.TrimRight(strings.TrimSpace(settings.WorkerBaseURL), "/")
	updates["TeamAutomationUsername"] = strings.TrimSpace(settings.WorkerUsername)
	updates["TeamAutomationPasswordSecret"] = strings.TrimSpace(settings.WorkerPassword)
	updates["TeamAutomationTimeoutSeconds"] = strconv.Itoa(settings.WorkerTimeoutSeconds)
	updates["TeamAutomationVerifySSLEnabled"] = strconv.FormatBool(settings.WorkerVerifySSL)

	for key, value := range updates {
		if err := model.UpdateOption(key, value); err != nil {
			return err
		}
	}
	return nil
}

func ParseRemoteRunPayload(raw any) (AutomationRunRequest, error) {
	payloadMap, ok := raw.(map[string]any)
	if !ok {
		return AutomationRunRequest{}, fmt.Errorf("run payload must be object")
	}
	req := AutomationRunRequest{
		Module:        strings.TrimSpace(strings.ToLower(toRunOptionString(payloadMap, "module"))),
		AccountStatus: strings.TrimSpace(strings.ToLower(toRunOptionString(payloadMap, "account_status"))),
		Search:        strings.TrimSpace(toRunOptionString(payloadMap, "search")),
		RedeemSite:    strings.TrimSpace(toRunOptionString(payloadMap, "redeem_site")),
		RunOptions:    map[string]any{},
	}
	if runRaw, exists := payloadMap["run_options"]; exists {
		if runMap, ok := runRaw.(map[string]any); ok {
			req.RunOptions = runMap
		} else {
			return req, fmt.Errorf("run_options must be object")
		}
	}
	if req.RunOptions == nil {
		req.RunOptions = map[string]any{}
	}
	if req.Module == "" {
		startFrom := strings.TrimSpace(strings.ToLower(toRunOptionString(payloadMap, "start_from")))
		if startFrom != "" {
			req.Module = startFrom
		}
	}
	if continueRaw, exists := payloadMap["continue_from_module"]; exists {
		if _, has := req.RunOptions["continue_from_module"]; !has {
			switch typed := continueRaw.(type) {
			case bool:
				req.RunOptions["continue_from_module"] = typed
			case string:
				req.RunOptions["continue_from_module"] = strings.ToLower(strings.TrimSpace(typed)) == "true"
			}
		}
	}
	if req.Module != "" {
		known := map[string]struct{}{}
		for _, module := range GetAutomationModuleDefinitions() {
			known[module.ID] = struct{}{}
		}
		if _, ok := known[req.Module]; !ok {
			return req, fmt.Errorf("unsupported module: %s", req.Module)
		}
	}
	return req, nil
}

func encodeQueryFromRequest(values url.Values, req AutomationRunRequest) url.Values {
	query := url.Values{}
	for key, item := range values {
		if len(item) == 0 {
			continue
		}
		query[key] = item
	}
	if req.AccountStatus != "" {
		query.Set("account_status", req.AccountStatus)
	}
	if req.Search != "" {
		query.Set("search", req.Search)
	}
	return query
}
