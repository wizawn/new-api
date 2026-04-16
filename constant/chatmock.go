package constant

const (
	ChatMockAccountStatusEnabled        = "enabled"
	ChatMockAccountStatusDisabled       = "disabled"
	ChatMockAccountStatusCooling        = "cooling"
	ChatMockAccountStatusAbnormal       = "abnormal"
	ChatMockAccountStatusQuotaLow       = "quota_low"
	ChatMockAccountStatusQuotaExhausted = "quota_exhausted"
	ChatMockAccountStatusAuthInvalid    = "auth_invalid"
	ChatMockAccountStatusNetworkIssue   = "network_issue"
	ChatMockAccountStatusManualReview   = "manual_review"
)

const (
	ChatMockMailboxStatusEnabled  = "enabled"
	ChatMockMailboxStatusDisabled = "disabled"
)

const (
	ChatMockMailEventTypeVerificationCode = "verification_code"
	ChatMockMailEventTypeInviteLink       = "invite_link"
	ChatMockMailEventTypeAuthAlert        = "auth_alert"
	ChatMockMailEventTypeBillingAlert     = "billing_alert"
	ChatMockMailEventTypeSystemNotice     = "system_notice"
	ChatMockMailEventTypeSystem           = "system"
)
