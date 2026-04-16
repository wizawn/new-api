package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	defaultTeamAutomationTimeoutSeconds = 30
	TeamAutomationModeRemoteProxy       = "remote_proxy"
	TeamAutomationModeLocalPlaywright   = "local_playwright"
	defaultTeamAutomationMode           = TeamAutomationModeRemoteProxy
	defaultLocalRunnerBaseURL           = "http://127.0.0.1:38210"
)

type TeamAutomationConfig struct {
	Enabled        bool   `json:"enabled"`
	Mode           string `json:"mode"`
	BaseURL        string `json:"base_url"`
	Username       string `json:"username"`
	Password       string `json:"-"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	VerifySSL      bool   `json:"verify_ssl"`
}

type TeamAutomationClient struct {
	config TeamAutomationConfig
	client *http.Client
}

type TeamAutomationDownload struct {
	StatusCode         int
	ContentType        string
	ContentDisposition string
	Body               []byte
}

func LoadTeamAutomationConfig() TeamAutomationConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	timeoutSeconds, err := strconv.Atoi(strings.TrimSpace(common.OptionMap["TeamAutomationTimeoutSeconds"]))
	if err != nil || timeoutSeconds <= 0 {
		timeoutSeconds = defaultTeamAutomationTimeoutSeconds
	}

	verifySSL := strings.TrimSpace(common.OptionMap["TeamAutomationVerifySSLEnabled"]) != "false"

	mode := normalizeTeamAutomationMode(common.OptionMap["TeamAutomationMode"])

	return TeamAutomationConfig{
		Enabled:        strings.TrimSpace(common.OptionMap["TeamAutomationEnabled"]) == "true",
		Mode:           mode,
		BaseURL:        strings.TrimRight(strings.TrimSpace(common.OptionMap["TeamAutomationBaseURL"]), "/"),
		Username:       strings.TrimSpace(common.OptionMap["TeamAutomationUsername"]),
		Password:       strings.TrimSpace(common.OptionMap["TeamAutomationPasswordSecret"]),
		TimeoutSeconds: timeoutSeconds,
		VerifySSL:      verifySSL,
	}
}

func (c TeamAutomationConfig) Sanitized() map[string]any {
	effectiveBaseURL := c.EffectiveBaseURL()
	return map[string]any{
		"enabled":           c.Enabled,
		"mode":              c.Mode,
		"base_url":          c.BaseURL,
		"resolved_base_url": effectiveBaseURL,
		"username":          c.Username,
		"configured":        effectiveBaseURL != "",
		"auth_configured":   c.Username != "" && c.Password != "",
		"timeout_seconds":   c.TimeoutSeconds,
		"verify_ssl":        c.VerifySSL,
	}
}

func (c TeamAutomationConfig) EffectiveBaseURL() string {
	mode := normalizeTeamAutomationMode(c.Mode)
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if mode == TeamAutomationModeLocalPlaywright && baseURL == "" {
		return defaultLocalRunnerBaseURL
	}
	return baseURL
}

func (c TeamAutomationConfig) Validate() error {
	if !c.Enabled {
		return fmt.Errorf("team automation is disabled")
	}
	mode := normalizeTeamAutomationMode(c.Mode)
	if mode != TeamAutomationModeRemoteProxy && mode != TeamAutomationModeLocalPlaywright {
		return fmt.Errorf("team automation mode is invalid")
	}
	if strings.TrimSpace(c.EffectiveBaseURL()) == "" {
		return fmt.Errorf("team automation base url is not configured")
	}
	parsedURL, err := url.ParseRequestURI(c.EffectiveBaseURL())
	if err != nil {
		return fmt.Errorf("invalid team automation base url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("team automation base url must use http or https")
	}
	if c.TimeoutSeconds < 5 || c.TimeoutSeconds > 300 {
		return fmt.Errorf("team automation timeout seconds must be between 5 and 300")
	}
	return nil
}

func NewTeamAutomationClient(config TeamAutomationConfig) (*TeamAutomationClient, error) {
	config.Mode = normalizeTeamAutomationMode(config.Mode)
	config.BaseURL = config.EffectiveBaseURL()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	transport := &http.Transport{}
	if !config.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &TeamAutomationClient{
		config: config,
		client: &http.Client{
			Timeout:   time.Duration(config.TimeoutSeconds) * time.Second,
			Transport: transport,
		},
	}, nil
}

func normalizeTeamAutomationMode(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case TeamAutomationModeRemoteProxy, TeamAutomationModeLocalPlaywright:
		return normalized
	default:
		return defaultTeamAutomationMode
	}
}

func NewTeamAutomationClientFromOptions() (*TeamAutomationClient, error) {
	return NewTeamAutomationClient(LoadTeamAutomationConfig())
}

func (c *TeamAutomationClient) Do(ctx context.Context, method string, path string, query url.Values, body any) (any, int, error) {
	if c == nil {
		return nil, 0, fmt.Errorf("team automation client is nil")
	}

	targetURL := strings.TrimRight(c.config.BaseURL, "/") + path
	if len(query) > 0 {
		targetURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.config.Username != "" || c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	payload, err := parseTeamAutomationJSON(raw)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return payload, resp.StatusCode, errors.New(extractTeamAutomationErrorMessage(resp.Status, payload))
	}

	return payload, resp.StatusCode, nil
}

func (c *TeamAutomationClient) Download(ctx context.Context, path string, query url.Values) (*TeamAutomationDownload, error) {
	if c == nil {
		return nil, fmt.Errorf("team automation client is nil")
	}

	targetURL := strings.TrimRight(c.config.BaseURL, "/") + path
	if len(query) > 0 {
		targetURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if c.config.Username != "" || c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		message := resp.Status
		var payload any
		if parsed, err := parseTeamAutomationJSON(body); err == nil {
			payload = parsed
			message = extractTeamAutomationErrorMessage(resp.Status, payload)
		}
		return nil, errors.New(message)
	}

	return &TeamAutomationDownload{
		StatusCode:         resp.StatusCode,
		ContentType:        resp.Header.Get("Content-Type"),
		ContentDisposition: resp.Header.Get("Content-Disposition"),
		Body:               body,
	}, nil
}

func (c *TeamAutomationClient) Upload(ctx context.Context, path string, query url.Values, fieldName string, fileName string, data []byte) (any, int, error) {
	if c == nil {
		return nil, 0, fmt.Errorf("team automation client is nil")
	}
	if strings.TrimSpace(fieldName) == "" {
		fieldName = "file"
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "upload.bin"
	}

	targetURL := strings.TrimRight(c.config.BaseURL, "/") + path
	if len(query) > 0 {
		targetURL += "?" + query.Encode()
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, 0, err
	}
	if _, err = part.Write(data); err != nil {
		return nil, 0, err
	}
	if err = writer.Close(); err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, &body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.config.Username != "" || c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	payload, err := parseTeamAutomationJSON(raw)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return payload, resp.StatusCode, errors.New(extractTeamAutomationErrorMessage(resp.Status, payload))
	}

	return payload, resp.StatusCode, nil
}

func parseTeamAutomationJSON(raw []byte) (any, error) {
	var payload any = map[string]any{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return payload, nil
	}
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("team automation returned non-json response: %s", string(raw))
	}
	return payload, nil
}

func extractTeamAutomationErrorMessage(status string, payload any) string {
	message := status
	if payloadMap, ok := payload.(map[string]any); ok {
		if detail := strings.TrimSpace(fmt.Sprintf("%v", payloadMap["detail"])); detail != "" && detail != "<nil>" {
			message = detail
		} else if payloadMessage := strings.TrimSpace(fmt.Sprintf("%v", payloadMap["message"])); payloadMessage != "" && payloadMessage != "<nil>" {
			message = payloadMessage
		}
	}
	return message
}
