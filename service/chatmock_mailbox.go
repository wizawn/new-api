package service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	neturl "net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	imap "github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

const (
	defaultChatMockMailPullLimit    = 30
	defaultChatMockMailNetworkSec   = 20
	maxChatMockMailSnippetLength    = 3000
	defaultChatMockMailboxPollFloor = 30
	chatMockMailboxMaxBackoffSec    = 15 * 60
	chatMockMailboxEventThrottleSec = 15 * 60
	chatMockMailboxProviderIMAP     = "imap"
	chatMockMailboxProviderWizawn   = "wizawn_http"
)

var (
	chatMockMailboxPollOnce sync.Once
	chatMockMailboxRetryMu  sync.Mutex
	chatMockMailboxRetryMap = map[int]*chatMockMailboxRetryState{}
	chatMockCodePatterns    = []*regexp.Regexp{
		regexp.MustCompile(`(?i)verification\s*code[^0-9]{0,20}([0-9]{4,8})`),
		regexp.MustCompile(`(?i)code[^0-9]{0,20}([0-9]{4,8})`),
		regexp.MustCompile(`验证码[^0-9]{0,10}([0-9]{4,8})`),
		regexp.MustCompile(`\b([0-9]{6})\b`),
	}
	chatMockInviteLinkPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	htmlTagPattern            = regexp.MustCompile(`<[^>]+>`)
	spacePattern              = regexp.MustCompile(`\s+`)
)

type chatMockMailAnalysis struct {
	EventType         string
	Classification    string
	Confidence        float64
	Code              string
	InviteLink        string
	RelatedIdentifier string
	ParsedPayload     map[string]any
}

type chatMockMailboxRetryState struct {
	FailCount   int
	NextRetryAt int64
	LastEventAt int64
}

type chatMockMailboxProviderConfig struct {
	ProviderType string
	APIBaseURL   string
	JWT          string
	Address      string
}

type chatMockWizawnMailRow struct {
	ID        string
	Subject   string
	Sender    string
	Recipient string
	Snippet   string
	Timestamp int64
	Raw       map[string]any
}

func normalizeChatMockMailboxProvider(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "wizawn", "wizawn_http", "mailbox_wizawn", "tempmail_wizawn", "tempmail":
		return chatMockMailboxProviderWizawn
	default:
		return chatMockMailboxProviderIMAP
	}
}

func normalizeWizawnAPIBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		base = "https://mailbox.wizawn.com"
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	parsed, err := neturl.Parse(base)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "https://mailbox.wizawn.com"
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func parseChatMockMailboxProviderConfig(mailbox *model.ChatMockMailbox) chatMockMailboxProviderConfig {
	config := chatMockMailboxProviderConfig{
		ProviderType: chatMockMailboxProviderIMAP,
		APIBaseURL:   "",
		JWT:          "",
		Address:      "",
	}
	if mailbox == nil {
		return config
	}
	metadata := map[string]any{}
	if err := common.UnmarshalJsonStr(strings.TrimSpace(mailbox.Metadata), &metadata); err == nil && metadata != nil {
		provider := strings.TrimSpace(toRunOptionString(metadata, "provider"))
		if provider == "" {
			provider = strings.TrimSpace(toRunOptionString(metadata, "provider_type"))
		}
		if provider != "" {
			config.ProviderType = normalizeChatMockMailboxProvider(provider)
		}

		apiBase := strings.TrimSpace(toRunOptionString(metadata, "api_base"))
		if apiBase == "" {
			apiBase = strings.TrimSpace(toRunOptionString(metadata, "base_url"))
		}
		if apiBase != "" {
			config.APIBaseURL = normalizeWizawnAPIBaseURL(apiBase)
		}

		jwt := strings.TrimSpace(toRunOptionString(metadata, "jwt"))
		if jwt == "" {
			jwt = strings.TrimSpace(toRunOptionString(metadata, "access_token"))
		}
		if jwt == "" {
			jwt = strings.TrimSpace(toRunOptionString(metadata, "token"))
		}
		config.JWT = jwt

		address := strings.TrimSpace(strings.ToLower(toRunOptionString(metadata, "address")))
		if address == "" {
			address = strings.TrimSpace(strings.ToLower(toRunOptionString(metadata, "email")))
		}
		config.Address = address
	}
	if config.ProviderType == chatMockMailboxProviderWizawn {
		config.APIBaseURL = normalizeWizawnAPIBaseURL(config.APIBaseURL)
		if strings.TrimSpace(config.JWT) == "" {
			config.JWT = strings.TrimSpace(mailbox.Password)
		}
		if strings.TrimSpace(config.Address) == "" {
			config.Address = strings.TrimSpace(strings.ToLower(mailbox.Email))
		}
	}
	return config
}

func hashChatMockMailboxMessageUID(rawID string) uint32 {
	text := strings.TrimSpace(rawID)
	if text == "" {
		return 1
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(text))
	value := hasher.Sum32()
	if value == 0 {
		return 1
	}
	return value
}

func ValidateChatMockMailboxConnectivity(mailbox *model.ChatMockMailbox, timeoutSec int) error {
	if mailbox == nil {
		return fmt.Errorf("邮箱配置不存在")
	}
	if timeoutSec <= 0 {
		timeoutSec = defaultChatMockMailNetworkSec
	}
	providerConfig := parseChatMockMailboxProviderConfig(mailbox)
	if providerConfig.ProviderType == chatMockMailboxProviderWizawn {
		return validateWizawnMailboxConnectivity(mailbox, providerConfig, timeoutSec)
	}
	if err := validateImapConnectivity(mailbox, timeoutSec); err != nil {
		return fmt.Errorf("IMAP 连接失败: %w", err)
	}
	// SMTP is optional for current mailbox workflow (IMAP polling).
	// Only validate SMTP when a host is explicitly configured.
	if strings.TrimSpace(mailbox.SmtpHost) == "" {
		return nil
	}
	if err := validateSmtpConnectivity(mailbox, timeoutSec); err != nil {
		return fmt.Errorf("SMTP 连接失败: %w", err)
	}
	return nil
}

func PullChatMockMailboxByID(mailboxID int, limit int, timeoutSec int) ([]*model.ChatMockMailEvent, error) {
	mailbox, err := model.GetChatMockMailboxByID(mailboxID)
	if err != nil {
		return nil, err
	}
	return PullChatMockMailbox(mailbox, limit, timeoutSec)
}

func PullChatMockMailbox(mailbox *model.ChatMockMailbox, limit int, timeoutSec int) ([]*model.ChatMockMailEvent, error) {
	if !setting.IsChatMockMailboxEnabled() {
		return nil, fmt.Errorf("CHATMOCK_MAILBOX_ENABLED=false，邮箱中心未启用")
	}
	if mailbox == nil {
		return nil, fmt.Errorf("邮箱配置不存在")
	}
	if limit <= 0 {
		limit = defaultChatMockMailPullLimit
	}
	if timeoutSec <= 0 {
		timeoutSec = defaultChatMockMailNetworkSec
	}
	providerConfig := parseChatMockMailboxProviderConfig(mailbox)
	if providerConfig.ProviderType == chatMockMailboxProviderWizawn {
		events, err := pullWizawnMailbox(mailbox, providerConfig, limit, timeoutSec)
		if err != nil {
			handleChatMockMailboxPullError(mailbox, err)
			return nil, err
		}
		resetChatMockMailboxRetryState(mailbox.Id)
		return events, nil
	}

	client, err := dialAndLoginIMAP(mailbox, timeoutSec)
	if err != nil {
		handleChatMockMailboxPullError(mailbox, err)
		return nil, err
	}
	defer func() {
		_ = client.Logout()
	}()

	if _, err = client.Select("INBOX", false); err != nil {
		handleChatMockMailboxPullError(mailbox, err)
		return nil, err
	}

	uids, err := searchMailboxUIDs(client, mailbox.LastUID)
	if err != nil {
		handleChatMockMailboxPullError(mailbox, err)
		return nil, err
	}
	if len(uids) == 0 {
		resetChatMockMailboxRetryState(mailbox.Id)
		touchChatMockMailbox(mailbox.Id, mailbox.LastUID, "")
		return []*model.ChatMockMailEvent{}, nil
	}

	sort.Slice(uids, func(i, j int) bool {
		return uids[i] < uids[j]
	})
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	seqSet := new(imap.SeqSet)
	for _, uid := range uids {
		seqSet.AddNum(uid)
	}
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, len(uids))
	fetchDone := make(chan error, 1)
	go func() {
		fetchDone <- client.UidFetch(seqSet, items, messages)
	}()

	events := make([]*model.ChatMockMailEvent, 0, len(uids))
	maxUID := mailbox.LastUID
	for msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Uid > maxUID {
			maxUID = msg.Uid
		}
		event, parseErr := buildChatMockMailEvent(mailbox.Id, msg, section)
		if parseErr != nil {
			common.SysError(fmt.Sprintf("解析邮箱消息失败 mailbox=%d uid=%d: %v", mailbox.Id, msg.Uid, parseErr))
			continue
		}
		exists, existsErr := model.ExistsChatMockMailEvent(mailbox.Id, msg.Uid, event.EventType)
		if existsErr != nil {
			common.SysError(fmt.Sprintf("查询邮箱事件失败 mailbox=%d uid=%d: %v", mailbox.Id, msg.Uid, existsErr))
			continue
		}
		if exists {
			continue
		}
		if insertErr := event.Insert(); insertErr != nil {
			common.SysError(fmt.Sprintf("写入邮箱事件失败 mailbox=%d uid=%d: %v", mailbox.Id, msg.Uid, insertErr))
			continue
		}
		events = append(events, event)
	}

	if fetchErr := <-fetchDone; fetchErr != nil {
		handleChatMockMailboxPullError(mailbox, fetchErr)
		return events, fetchErr
	}

	resetChatMockMailboxRetryState(mailbox.Id)
	touchChatMockMailbox(mailbox.Id, maxUID, "")
	return events, nil
}

func StartChatMockMailboxPollingTask() {
	if !setting.IsChatMockMailboxEnabled() {
		return
	}
	chatMockMailboxPollOnce.Do(func() {
		go func() {
			for {
				mailboxes, err := model.ListEnabledChatMockMailboxes()
				if err != nil {
					common.SysError("拉取 ChatMock 邮箱列表失败: " + err.Error())
					time.Sleep(60 * time.Second)
					continue
				}
				now := common.GetTimestamp()
				minSleep := int64(300)
				if len(mailboxes) == 0 {
					time.Sleep(60 * time.Second)
					continue
				}
				for _, mailbox := range mailboxes {
					if mailbox == nil {
						continue
					}
					interval := int64(mailbox.PollIntervalSecond)
					if interval < defaultChatMockMailboxPollFloor {
						interval = defaultChatMockMailboxPollFloor
					}
					nextAt := mailbox.LastCheckedTime + interval
					if retryAt := getChatMockMailboxNextRetryAt(mailbox.Id); retryAt > nextAt {
						nextAt = retryAt
					}
					if mailbox.LastCheckedTime == 0 || now >= nextAt {
						if _, pullErr := PullChatMockMailbox(mailbox, defaultChatMockMailPullLimit, defaultChatMockMailNetworkSec); pullErr != nil {
							if !isTransientChatMockMailboxError(pullErr) {
								common.SysError(fmt.Sprintf("ChatMock 邮箱轮询失败 mailbox=%d: %v", mailbox.Id, pullErr))
							}
						}
						nextAt = now + interval
						if retryAt := getChatMockMailboxNextRetryAt(mailbox.Id); retryAt > nextAt {
							nextAt = retryAt
						}
					}
					if delta := nextAt - now; delta > 0 && delta < minSleep {
						minSleep = delta
					}
				}
				if minSleep < int64(defaultChatMockMailboxPollFloor) {
					minSleep = int64(defaultChatMockMailboxPollFloor)
				}
				time.Sleep(time.Duration(minSleep) * time.Second)
			}
		}()
	})
}

func buildChatMockMailEvent(mailboxID int, msg *imap.Message, section *imap.BodySectionName) (*model.ChatMockMailEvent, error) {
	bodyReader := msg.GetBody(section)
	if bodyReader == nil {
		return nil, fmt.Errorf("消息体为空")
	}
	rawBody, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, err
	}
	text := sanitizeUTF8(extractMailText(rawBody))
	subject := ""
	sender := ""
	receivedAt := int64(0)
	if msg.Envelope != nil {
		subject = sanitizeUTF8(strings.TrimSpace(msg.Envelope.Subject))
		if msg.Envelope.Date.Unix() > 0 {
			receivedAt = msg.Envelope.Date.Unix()
		}
		sender = sanitizeUTF8(buildMailAddress(msg.Envelope.From))
	}
	analysis := analyzeChatMockMailContent(subject, sender, text)
	parsedPayload := ""
	if payloadBytes, marshalErr := common.Marshal(analysis.ParsedPayload); marshalErr == nil {
		parsedPayload = string(payloadBytes)
	}
	if receivedAt <= 0 {
		receivedAt = common.GetTimestamp()
	}

	event := &model.ChatMockMailEvent{
		MailboxID:         mailboxID,
		MessageUID:        msg.Uid,
		EventType:         analysis.EventType,
		Classification:    analysis.Classification,
		Subject:           subject,
		Sender:            sender,
		RelatedIdentifier: analysis.RelatedIdentifier,
		Confidence:        analysis.Confidence,
		Code:              analysis.Code,
		ParsedPayload:     parsedPayload,
		RawSnippet:        truncateText(text, maxChatMockMailSnippetLength),
		Status:            "ok",
		ReceivedAt:        receivedAt,
	}
	return event, nil
}

func ReparseChatMockMailEvent(event *model.ChatMockMailEvent) error {
	if event == nil {
		return fmt.Errorf("mail event is nil")
	}
	analysis := analyzeChatMockMailContent(
		strings.TrimSpace(event.Subject),
		strings.TrimSpace(event.Sender),
		strings.TrimSpace(event.RawSnippet),
	)
	payloadBytes, err := common.Marshal(analysis.ParsedPayload)
	if err != nil {
		return err
	}
	event.EventType = analysis.EventType
	event.Classification = analysis.Classification
	event.RelatedIdentifier = analysis.RelatedIdentifier
	event.Confidence = analysis.Confidence
	event.Code = analysis.Code
	event.ParsedPayload = string(payloadBytes)
	return event.UpdateParsedFields()
}

func analyzeChatMockMailContent(subject string, sender string, text string) chatMockMailAnalysis {
	subject = strings.TrimSpace(subject)
	sender = strings.TrimSpace(sender)
	text = strings.TrimSpace(text)
	combined := strings.ToLower(strings.TrimSpace(subject + "\n" + sender + "\n" + text))

	analysis := chatMockMailAnalysis{
		EventType:      constant.ChatMockMailEventTypeSystem,
		Classification: constant.ChatMockMailEventTypeSystem,
		Confidence:     0.45,
		ParsedPayload: map[string]any{
			"subject": subject,
			"sender":  sender,
		},
	}

	if code := ExtractChatMockVerificationCode(text); code != "" {
		analysis.EventType = constant.ChatMockMailEventTypeVerificationCode
		analysis.Classification = constant.ChatMockMailEventTypeVerificationCode
		analysis.Code = code
		analysis.Confidence = 0.92
		analysis.ParsedPayload["verification_code"] = code
	}

	if link := extractChatMockInviteLink(subject + "\n" + text); link != "" {
		analysis.EventType = constant.ChatMockMailEventTypeInviteLink
		analysis.Classification = constant.ChatMockMailEventTypeInviteLink
		analysis.InviteLink = link
		analysis.Confidence = 0.96
		analysis.ParsedPayload["invite_link"] = link
	}

	if isChatMockAuthAlert(combined) {
		if analysis.EventType == constant.ChatMockMailEventTypeSystem {
			analysis.EventType = constant.ChatMockMailEventTypeAuthAlert
			analysis.Classification = constant.ChatMockMailEventTypeAuthAlert
			analysis.Confidence = 0.82
		}
		analysis.ParsedPayload["auth_alert"] = true
	}

	if isChatMockBillingAlert(combined) {
		if analysis.EventType == constant.ChatMockMailEventTypeSystem {
			analysis.EventType = constant.ChatMockMailEventTypeBillingAlert
			analysis.Classification = constant.ChatMockMailEventTypeBillingAlert
			analysis.Confidence = 0.8
		}
		analysis.ParsedPayload["billing_alert"] = true
	}

	if analysis.EventType == constant.ChatMockMailEventTypeSystem {
		analysis.EventType = constant.ChatMockMailEventTypeSystemNotice
		analysis.Classification = constant.ChatMockMailEventTypeSystemNotice
		analysis.Confidence = 0.6
	}

	related := extractChatMockRelatedIdentifier(sender, analysis.InviteLink)
	if related != "" {
		analysis.RelatedIdentifier = related
		analysis.ParsedPayload["related_identifier"] = related
	}
	return analysis
}

func extractChatMockInviteLink(source string) string {
	matches := chatMockInviteLinkPattern.FindAllString(strings.TrimSpace(source), -1)
	for _, raw := range matches {
		link := strings.TrimSpace(strings.Trim(raw, ".,;)]}\"'"))
		if link == "" {
			continue
		}
		parsed, err := neturl.Parse(link)
		if err != nil || parsed.Host == "" {
			continue
		}
		lower := strings.ToLower(link)
		if strings.Contains(lower, "invite") ||
			strings.Contains(lower, "invitation") ||
			strings.Contains(lower, "join") ||
			strings.Contains(lower, "redeem") ||
			strings.Contains(lower, "team") {
			return link
		}
	}
	return ""
}

func extractChatMockRelatedIdentifier(sender string, inviteLink string) string {
	if sender != "" {
		if addr, err := mail.ParseAddress(sender); err == nil {
			return strings.ToLower(strings.TrimSpace(addr.Address))
		}
		if at := strings.LastIndex(sender, "@"); at > 0 {
			text := strings.ToLower(strings.TrimSpace(sender))
			if len(text) > at+1 {
				return text
			}
		}
	}
	if inviteLink != "" {
		parsed, err := neturl.Parse(inviteLink)
		if err == nil && parsed.Host != "" {
			return strings.ToLower(strings.TrimSpace(parsed.Host))
		}
	}
	return ""
}

func isChatMockAuthAlert(text string) bool {
	keywords := []string{
		"new login",
		"new sign-in",
		"security alert",
		"suspicious",
		"verify your identity",
		"登录提醒",
		"异常登录",
		"安全提醒",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func isChatMockBillingAlert(text string) bool {
	keywords := []string{
		"billing",
		"invoice",
		"payment failed",
		"payment method",
		"subscription",
		"账单",
		"扣费",
		"付款失败",
		"订阅",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func searchMailboxUIDs(client *imapclient.Client, lastUID uint32) ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	if lastUID > 0 {
		uidSet := new(imap.SeqSet)
		uidSet.AddRange(lastUID+1, ^uint32(0))
		criteria.Uid = uidSet
	}
	uids, err := client.UidSearch(criteria)
	if err != nil {
		return nil, err
	}
	return uids, nil
}

func dialAndLoginIMAP(mailbox *model.ChatMockMailbox, timeoutSec int) (*imapclient.Client, error) {
	addr := net.JoinHostPort(mailbox.ImapHost, strconv.Itoa(mailbox.ImapPort))
	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}

	var (
		client *imapclient.Client
		err    error
	)
	switch strings.ToLower(strings.TrimSpace(mailbox.ImapSecurity)) {
	case "ssl", "tls", "":
		client, err = imapclient.DialTLS(addr, &tls.Config{ServerName: mailbox.ImapHost, MinVersion: tls.VersionTLS12})
	case "starttls":
		client, err = imapclient.DialWithDialer(dialer, addr)
		if err == nil {
			err = client.StartTLS(&tls.Config{ServerName: mailbox.ImapHost, MinVersion: tls.VersionTLS12})
		}
	case "none":
		client, err = imapclient.DialWithDialer(dialer, addr)
	default:
		client, err = imapclient.DialTLS(addr, &tls.Config{ServerName: mailbox.ImapHost, MinVersion: tls.VersionTLS12})
	}
	if err != nil {
		return nil, err
	}
	if loginErr := client.Login(mailbox.Username, mailbox.Password); loginErr != nil {
		_ = client.Logout()
		return nil, loginErr
	}
	return client, nil
}

func validateImapConnectivity(mailbox *model.ChatMockMailbox, timeoutSec int) error {
	client, err := dialAndLoginIMAP(mailbox, timeoutSec)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Logout()
	}()
	_, err = client.Select("INBOX", true)
	return err
}

func validateSmtpConnectivity(mailbox *model.ChatMockMailbox, timeoutSec int) error {
	if strings.TrimSpace(mailbox.SmtpHost) == "" {
		return nil
	}
	addr := net.JoinHostPort(mailbox.SmtpHost, strconv.Itoa(mailbox.SmtpPort))
	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}

	var conn net.Conn
	var err error
	smtpMode := strings.ToLower(strings.TrimSpace(mailbox.SmtpSecurity))
	if smtpMode == "" {
		smtpMode = "ssl"
	}

	switch smtpMode {
	case "ssl", "tls":
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: mailbox.SmtpHost, MinVersion: tls.VersionTLS12})
	default:
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	client, err := smtp.NewClient(conn, mailbox.SmtpHost)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Quit()
	}()

	if smtpMode == "starttls" {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("SMTP 服务器不支持 STARTTLS")
		}
		if err = client.StartTLS(&tls.Config{ServerName: mailbox.SmtpHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	ok, _ := client.Extension("AUTH")
	if !ok {
		return nil
	}
	auth := smtp.PlainAuth("", mailbox.Username, mailbox.Password, mailbox.SmtpHost)
	return client.Auth(auth)
}

func validateWizawnMailboxConnectivity(mailbox *model.ChatMockMailbox, config chatMockMailboxProviderConfig, timeoutSec int) error {
	if mailbox == nil {
		return fmt.Errorf("邮箱配置不存在")
	}
	if strings.TrimSpace(config.JWT) == "" {
		return fmt.Errorf("Wizawn 邮箱 JWT 未配置（metadata.jwt / metadata.access_token）")
	}
	_, err := wizawnAPIRequest(config, http.MethodGet, "/api/settings", nil, timeoutSec)
	if err != nil {
		return fmt.Errorf("Wizawn API 连接失败: %w", err)
	}
	return nil
}

func wizawnAPIRequest(config chatMockMailboxProviderConfig, method string, path string, payload any, timeoutSec int) (map[string]any, error) {
	if timeoutSec <= 0 {
		timeoutSec = defaultChatMockMailNetworkSec
	}
	if strings.TrimSpace(config.APIBaseURL) == "" {
		config.APIBaseURL = "https://mailbox.wizawn.com"
	}
	targetURL := strings.TrimRight(config.APIBaseURL, "/")
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		normalizedPath = "/"
	}
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}
	targetURL += normalizedPath

	var body io.Reader
	if payload != nil {
		raw, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(config.JWT) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.JWT))
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
		return nil, fmt.Errorf("Wizawn API %s %s 失败 %d: %s", method, normalizedPath, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	payloadMap := map[string]any{}
	if len(raw) == 0 {
		return payloadMap, nil
	}
	var decoded any
	if err = common.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("Wizawn API 响应解析失败: %w", err)
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
		return nil, fmt.Errorf("Wizawn API %s %s 返回失败: %s", method, normalizedPath, errMsg)
	}
	return payloadMap, nil
}

func extractWizawnMailRows(payload map[string]any) []map[string]any {
	if payload == nil {
		return []map[string]any{}
	}
	candidates := []any{
		payload["results"],
		payload["mails"],
		payload["items"],
		payload["list"],
		payload["records"],
	}
	if dataMap, ok := payload["data"].(map[string]any); ok && dataMap != nil {
		candidates = append(candidates,
			dataMap["results"],
			dataMap["mails"],
			dataMap["items"],
			dataMap["list"],
			dataMap["records"],
		)
	}
	for _, candidate := range candidates {
		rows, ok := candidate.([]any)
		if !ok || len(rows) == 0 {
			continue
		}
		result := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if rowMap, ok := item.(map[string]any); ok && rowMap != nil {
				result = append(result, rowMap)
			}
		}
		return result
	}
	return []map[string]any{}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func normalizeMailboxTimestamp(value any) int64 {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		if typed > 1_000_000_000_000 {
			return typed / 1000
		}
		return typed
	case int:
		ts := int64(typed)
		if ts > 1_000_000_000_000 {
			return ts / 1000
		}
		return ts
	case float64:
		ts := int64(typed)
		if ts > 1_000_000_000_000 {
			return ts / 1000
		}
		return ts
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0
		}
		if number, err := strconv.ParseInt(text, 10, 64); err == nil {
			if number > 1_000_000_000_000 {
				return number / 1000
			}
			return number
		}
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			return parsed.Unix()
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", text); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}

func normalizeWizawnMailRow(row map[string]any, index int) *chatMockWizawnMailRow {
	if row == nil {
		return nil
	}
	id := stringFromAny(row["id"])
	if id == "" {
		id = stringFromAny(row["mail_id"])
	}
	if id == "" {
		id = stringFromAny(row["message_id"])
	}
	if id == "" {
		id = stringFromAny(row["mailId"])
	}
	if id == "" {
		id = stringFromAny(row["messageId"])
	}
	if id == "" {
		id = stringFromAny(row["uid"])
	}
	if id == "" {
		id = fmt.Sprintf("wizawn-%d", index)
	}

	subject := stringFromAny(row["subject"])
	if subject == "" {
		subject = stringFromAny(row["title"])
	}
	sender := stringFromAny(row["from"])
	if sender == "" {
		sender = stringFromAny(row["sender"])
	}
	if sender == "" {
		sender = stringFromAny(row["from_address"])
	}
	recipient := stringFromAny(row["to"])
	if recipient == "" {
		recipient = stringFromAny(row["receiver"])
	}
	if recipient == "" {
		recipient = stringFromAny(row["address"])
	}
	snippet := stringFromAny(row["snippet"])
	if snippet == "" {
		snippet = stringFromAny(row["preview"])
	}
	if snippet == "" {
		snippet = stringFromAny(row["summary"])
	}
	if snippet == "" {
		snippet = stringFromAny(row["text"])
	}

	timestamp := normalizeMailboxTimestamp(row["created_at"])
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["createdAt"])
	}
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["received_at"])
	}
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["receivedAt"])
	}
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["updated_at"])
	}
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["updatedAt"])
	}
	if timestamp == 0 {
		timestamp = normalizeMailboxTimestamp(row["timestamp"])
	}
	if timestamp == 0 {
		timestamp = common.GetTimestamp()
	}

	return &chatMockWizawnMailRow{
		ID:        strings.TrimSpace(id),
		Subject:   sanitizeUTF8(subject),
		Sender:    sanitizeUTF8(sender),
		Recipient: sanitizeUTF8(recipient),
		Snippet:   sanitizeUTF8(snippet),
		Timestamp: timestamp,
		Raw:       row,
	}
}

func collectMailboxTextFragments(value any, output *[]string, depth int) {
	if output == nil || depth > 4 || len(*output) > 120 {
		return
	}
	switch typed := value.(type) {
	case nil:
		return
	case string:
		text := strings.TrimSpace(typed)
		if text != "" {
			*output = append(*output, text)
		}
		return
	case []any:
		max := len(typed)
		if max > 20 {
			max = 20
		}
		for i := 0; i < max; i++ {
			collectMailboxTextFragments(typed[i], output, depth+1)
			if len(*output) > 120 {
				return
			}
		}
		return
	case map[string]any:
		preferredKeys := []string{
			"subject", "title", "from", "sender", "to", "receiver", "address", "snippet", "preview",
			"text", "plain", "html", "body", "content", "raw", "source",
		}
		for _, key := range preferredKeys {
			if child, exists := typed[key]; exists {
				collectMailboxTextFragments(child, output, depth+1)
				if len(*output) > 120 {
					return
				}
			}
		}
		for key, child := range typed {
			skip := false
			for _, preferred := range preferredKeys {
				if key == preferred {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			collectMailboxTextFragments(child, output, depth+1)
			if len(*output) > 120 {
				return
			}
		}
	}
}

func buildWizawnMailEvent(mailbox *model.ChatMockMailbox, providerConfig chatMockMailboxProviderConfig, row *chatMockWizawnMailRow, detail map[string]any) *model.ChatMockMailEvent {
	if mailbox == nil || row == nil {
		return nil
	}
	fragments := make([]string, 0, 16)
	collectMailboxTextFragments(row.Raw, &fragments, 0)
	collectMailboxTextFragments(detail, &fragments, 0)
	if row.Snippet != "" {
		fragments = append(fragments, row.Snippet)
	}
	text := strings.TrimSpace(strings.Join(fragments, "\n"))
	analysis := analyzeChatMockMailContent(row.Subject, row.Sender, text)
	payload := analysis.ParsedPayload
	if payload == nil {
		payload = map[string]any{}
	}
	payload["provider"] = chatMockMailboxProviderWizawn
	payload["mail_id"] = row.ID
	payload["recipient"] = row.Recipient
	if strings.TrimSpace(providerConfig.Address) != "" {
		payload["mailbox_address"] = providerConfig.Address
	}
	parsedPayload := ""
	if data, err := common.Marshal(payload); err == nil {
		parsedPayload = string(data)
	}
	receivedAt := row.Timestamp
	if receivedAt <= 0 {
		receivedAt = common.GetTimestamp()
	}
	return &model.ChatMockMailEvent{
		MailboxID:         mailbox.Id,
		MessageUID:        hashChatMockMailboxMessageUID(row.ID),
		EventType:         analysis.EventType,
		Classification:    analysis.Classification,
		Subject:           row.Subject,
		Sender:            row.Sender,
		RelatedIdentifier: analysis.RelatedIdentifier,
		Confidence:        analysis.Confidence,
		Code:              analysis.Code,
		ParsedPayload:     parsedPayload,
		RawSnippet:        truncateText(normalizeSnippet(text), maxChatMockMailSnippetLength),
		Status:            "ok",
		ReceivedAt:        receivedAt,
	}
}

func pullWizawnMailbox(mailbox *model.ChatMockMailbox, providerConfig chatMockMailboxProviderConfig, limit int, timeoutSec int) ([]*model.ChatMockMailEvent, error) {
	if mailbox == nil {
		return nil, fmt.Errorf("邮箱配置不存在")
	}
	if strings.TrimSpace(providerConfig.JWT) == "" {
		return nil, fmt.Errorf("Wizawn 邮箱 JWT 未配置（metadata.jwt / metadata.access_token）")
	}
	if limit <= 0 {
		limit = defaultChatMockMailPullLimit
	}

	listPayload, err := wizawnAPIRequest(
		providerConfig,
		http.MethodGet,
		fmt.Sprintf("/api/mails?limit=%d&offset=0", limit),
		nil,
		timeoutSec,
	)
	if err != nil {
		return nil, err
	}
	rows := extractWizawnMailRows(listPayload)
	if len(rows) == 0 {
		touchChatMockMailbox(mailbox.Id, mailbox.LastUID, "")
		return []*model.ChatMockMailEvent{}, nil
	}

	normalizedRows := make([]*chatMockWizawnMailRow, 0, len(rows))
	for index, row := range rows {
		item := normalizeWizawnMailRow(row, index)
		if item != nil {
			normalizedRows = append(normalizedRows, item)
		}
	}
	if len(normalizedRows) == 0 {
		touchChatMockMailbox(mailbox.Id, mailbox.LastUID, "")
		return []*model.ChatMockMailEvent{}, nil
	}
	sort.Slice(normalizedRows, func(i, j int) bool {
		return normalizedRows[i].Timestamp < normalizedRows[j].Timestamp
	})
	if len(normalizedRows) > limit {
		normalizedRows = normalizedRows[len(normalizedRows)-limit:]
	}

	events := make([]*model.ChatMockMailEvent, 0, len(normalizedRows))
	maxUID := mailbox.LastUID
	for _, row := range normalizedRows {
		if row == nil || strings.TrimSpace(row.ID) == "" {
			continue
		}
		uid := hashChatMockMailboxMessageUID(row.ID)
		if uid > maxUID {
			maxUID = uid
		}
		detailPayload, detailErr := wizawnAPIRequest(
			providerConfig,
			http.MethodGet,
			"/api/mail/"+neturl.PathEscape(row.ID),
			nil,
			timeoutSec,
		)
		if detailErr != nil {
			common.SysError(fmt.Sprintf("Wizawn 邮件详情读取失败 mailbox=%d mail_id=%s: %v", mailbox.Id, row.ID, detailErr))
			detailPayload = nil
		}
		event := buildWizawnMailEvent(mailbox, providerConfig, row, detailPayload)
		if event == nil {
			continue
		}
		exists, existsErr := model.ExistsChatMockMailEvent(mailbox.Id, event.MessageUID, event.EventType)
		if existsErr != nil {
			common.SysError(fmt.Sprintf("查询 Wizawn 邮件事件失败 mailbox=%d uid=%d: %v", mailbox.Id, event.MessageUID, existsErr))
			continue
		}
		if exists {
			continue
		}
		if insertErr := event.Insert(); insertErr != nil {
			common.SysError(fmt.Sprintf("写入 Wizawn 邮件事件失败 mailbox=%d uid=%d: %v", mailbox.Id, event.MessageUID, insertErr))
			continue
		}
		events = append(events, event)
	}
	touchChatMockMailbox(mailbox.Id, maxUID, "")
	return events, nil
}

func buildMailAddress(addrs []*imap.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	first := addrs[0]
	if first == nil {
		return ""
	}
	if first.MailboxName == "" || first.HostName == "" {
		return strings.TrimSpace(first.PersonalName)
	}
	if first.PersonalName == "" {
		return strings.TrimSpace(first.MailboxName + "@" + first.HostName)
	}
	return strings.TrimSpace(first.PersonalName + " <" + first.MailboxName + "@" + first.HostName + ">")
}

func ExtractChatMockVerificationCode(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, pattern := range chatMockCodePatterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

func extractMailText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return normalizeSnippet(string(raw))
	}
	bodyBytes, err := io.ReadAll(message.Body)
	if err != nil {
		return normalizeSnippet(string(raw))
	}
	contentType := message.Header.Get("Content-Type")
	transferEncoding := message.Header.Get("Content-Transfer-Encoding")
	decodedBody := decodeTransferEncoding(transferEncoding, bodyBytes)
	text := extractTextByContentType(contentType, decodedBody)
	if strings.TrimSpace(text) == "" {
		text = string(decodedBody)
	}
	return normalizeSnippet(text)
}

func extractTextByContentType(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return normalizeSnippet(string(body))
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return normalizeSnippet(string(body))
		}
		mr := multipart.NewReader(bytes.NewReader(body), boundary)
		parts := make([]string, 0, 4)
		for {
			part, partErr := mr.NextPart()
			if partErr == io.EOF {
				break
			}
			if partErr != nil {
				break
			}
			partBody, _ := io.ReadAll(part)
			decoded := decodeTransferEncoding(part.Header.Get("Content-Transfer-Encoding"), partBody)
			partContentType := part.Header.Get("Content-Type")
			partMediaType, _, _ := mime.ParseMediaType(partContentType)
			partMediaType = strings.ToLower(strings.TrimSpace(partMediaType))
			if strings.HasPrefix(partMediaType, "text/plain") {
				parts = append(parts, string(decoded))
				continue
			}
			if strings.HasPrefix(partMediaType, "text/html") {
				parts = append(parts, stripHTML(string(decoded)))
				continue
			}
			if strings.HasPrefix(partMediaType, "multipart/") {
				parts = append(parts, extractTextByContentType(partContentType, decoded))
			}
		}
		return strings.Join(parts, "\n")
	}
	if strings.HasPrefix(mediaType, "text/html") {
		return stripHTML(string(body))
	}
	return string(body)
}

func decodeTransferEncoding(encoding string, data []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		decoded, err := io.ReadAll(base64NewDecoder(data))
		if err != nil {
			return data
		}
		return decoded
	case "quoted-printable":
		reader := quotedprintable.NewReader(bytes.NewReader(data))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return data
		}
		return decoded
	default:
		return data
	}
}

func base64NewDecoder(data []byte) io.Reader {
	cleaned := strings.ReplaceAll(string(data), "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.TrimSpace(cleaned)
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(cleaned))
}

func stripHTML(value string) string {
	return htmlTagPattern.ReplaceAllString(value, " ")
}

func normalizeSnippet(text string) string {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "\u00a0", " ")
	cleaned = spacePattern.ReplaceAllString(cleaned, " ")
	return sanitizeUTF8(strings.TrimSpace(cleaned))
}

func truncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen])
}

func sanitizeUTF8(text string) string {
	return strings.ToValidUTF8(text, "")
}

func handleChatMockMailboxPullError(mailbox *model.ChatMockMailbox, err error) {
	if mailbox == nil {
		return
	}
	if isTransientChatMockMailboxError(err) {
		retryAfterSec := markChatMockMailboxTransientError(mailbox, err)
		if retryAfterSec > 0 {
			common.SysLog(fmt.Sprintf("ChatMock 邮箱瞬态异常 mailbox=%d，下次重试约 %d 秒后: %v", mailbox.Id, retryAfterSec, err))
		}
		return
	}
	resetChatMockMailboxRetryState(mailbox.Id)
	markChatMockMailboxError(mailbox.Id, err)
}

func isTransientChatMockMailboxError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}

	// Authentication failures should be treated as hard failures.
	hardMarkers := []string{
		"authentication failed",
		"auth failed",
		"invalid credentials",
		"invalid password",
		"login failed",
		"authorization failed",
		"no such user",
		"unknown user",
		"username or password",
	}
	for _, marker := range hardMarkers {
		if strings.Contains(text, marker) {
			return false
		}
	}

	transientMarkers := []string{
		"system busy",
		"try again later",
		"temporarily unavailable",
		"temporary failure",
		"too many",
		"rate limit",
		"throttle",
		"server unavailable",
		"service unavailable",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"timeout",
		"eof",
	}
	for _, marker := range transientMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func getChatMockMailboxRetryStateLocked(mailboxID int) *chatMockMailboxRetryState {
	state, ok := chatMockMailboxRetryMap[mailboxID]
	if ok {
		return state
	}
	state = &chatMockMailboxRetryState{}
	chatMockMailboxRetryMap[mailboxID] = state
	return state
}

func resetChatMockMailboxRetryState(mailboxID int) {
	if mailboxID <= 0 {
		return
	}
	chatMockMailboxRetryMu.Lock()
	delete(chatMockMailboxRetryMap, mailboxID)
	chatMockMailboxRetryMu.Unlock()
}

func getChatMockMailboxNextRetryAt(mailboxID int) int64 {
	if mailboxID <= 0 {
		return 0
	}
	chatMockMailboxRetryMu.Lock()
	defer chatMockMailboxRetryMu.Unlock()
	state, ok := chatMockMailboxRetryMap[mailboxID]
	if !ok || state == nil || state.NextRetryAt <= 0 {
		return 0
	}
	return state.NextRetryAt
}

func markChatMockMailboxTransientError(mailbox *model.ChatMockMailbox, err error) int64 {
	if mailbox == nil || mailbox.Id <= 0 {
		return 0
	}
	now := common.GetTimestamp()
	interval := int64(mailbox.PollIntervalSecond)
	if interval < defaultChatMockMailboxPollFloor {
		interval = defaultChatMockMailboxPollFloor
	}

	chatMockMailboxRetryMu.Lock()
	state := getChatMockMailboxRetryStateLocked(mailbox.Id)
	state.FailCount++
	if state.FailCount < 1 {
		state.FailCount = 1
	}
	shift := state.FailCount - 1
	if shift > 4 {
		shift = 4
	}
	retryAfter := interval << shift
	if retryAfter < defaultChatMockMailboxPollFloor {
		retryAfter = defaultChatMockMailboxPollFloor
	}
	if retryAfter > chatMockMailboxMaxBackoffSec {
		retryAfter = chatMockMailboxMaxBackoffSec
	}
	state.NextRetryAt = now + retryAfter
	shouldInsertEvent := state.LastEventAt == 0 || now-state.LastEventAt >= chatMockMailboxEventThrottleSec
	if shouldInsertEvent {
		state.LastEventAt = now
	}
	chatMockMailboxRetryMu.Unlock()

	errText := ""
	if err != nil {
		errText = truncateText(strings.TrimSpace(err.Error()), 1024)
	}
	if errText == "" {
		errText = "transient mailbox error"
	}
	lastErr := fmt.Sprintf("transient: %s (retry in %ds)", errText, retryAfter)
	if updateErr := model.DB.Model(&model.ChatMockMailbox{}).Where("id = ?", mailbox.Id).Updates(map[string]any{
		"last_checked_time": now,
		"last_error":        truncateText(lastErr, 1024),
		"updated_time":      now,
	}).Error; updateErr != nil {
		common.SysError("更新 ChatMock 邮箱瞬态错误状态失败: " + updateErr.Error())
	}

	if shouldInsertEvent {
		event := &model.ChatMockMailEvent{
			MailboxID:  mailbox.Id,
			MessageUID: 0,
			EventType:  constant.ChatMockMailEventTypeSystem,
			Subject:    "",
			Sender:     "",
			Code:       "",
			RawSnippet: truncateText(lastErr, 1024),
			Status:     "warn",
			ReceivedAt: now,
		}
		if insertErr := event.Insert(); insertErr != nil {
			common.SysError("写入 ChatMock 邮箱瞬态异常事件失败: " + insertErr.Error())
		}
	}
	return retryAfter
}

func markChatMockMailboxError(mailboxID int, err error) {
	if mailboxID <= 0 {
		return
	}
	errMsg := ""
	if err != nil {
		errMsg = truncateText(strings.TrimSpace(err.Error()), 1024)
	}
	now := common.GetTimestamp()
	if updateErr := model.DB.Model(&model.ChatMockMailbox{}).Where("id = ?", mailboxID).Updates(map[string]any{
		"last_checked_time": now,
		"last_error":        errMsg,
		"updated_time":      now,
	}).Error; updateErr != nil {
		common.SysError("更新 ChatMock 邮箱错误状态失败: " + updateErr.Error())
	}

	event := &model.ChatMockMailEvent{
		MailboxID:  mailboxID,
		MessageUID: 0,
		EventType:  constant.ChatMockMailEventTypeSystem,
		Subject:    "",
		Sender:     "",
		Code:       "",
		RawSnippet: errMsg,
		Status:     "error",
		ReceivedAt: now,
	}
	if insertErr := event.Insert(); insertErr != nil {
		common.SysError("写入 ChatMock 邮箱异常事件失败: " + insertErr.Error())
	}
}

func touchChatMockMailbox(mailboxID int, lastUID uint32, lastError string) {
	now := common.GetTimestamp()
	updates := map[string]any{
		"last_checked_time": now,
		"last_error":        truncateText(strings.TrimSpace(lastError), 1024),
		"updated_time":      now,
	}
	if lastUID > 0 {
		updates["last_uid"] = lastUID
	}
	if err := model.DB.Model(&model.ChatMockMailbox{}).Where("id = ?", mailboxID).Updates(updates).Error; err != nil {
		common.SysError("更新 ChatMock 邮箱状态失败: " + err.Error())
	}
}
