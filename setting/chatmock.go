package setting

import "github.com/QuantumNous/new-api/common"

var (
	ChatMockEnabled                  bool
	ChatMockMailboxEnabled           bool
	ChatMockDefaultCooldownSeconds   int
	ChatMockDefaultRequestTimeoutSec int
	ChatMockDefaultHealthIntervalSec int
	ChatMockEntitlementIntervalSec   int
	ChatMockDefaultInstructions      string
)

func init() {
	ReloadChatMockFromEnv()
}

func ReloadChatMockFromEnv() {
	ChatMockEnabled = common.GetEnvOrDefaultBool("CHATMOCK_ENABLED", false)
	ChatMockMailboxEnabled = common.GetEnvOrDefaultBool("CHATMOCK_MAILBOX_ENABLED", false)
	ChatMockDefaultCooldownSeconds = common.GetEnvOrDefault("CHATMOCK_DEFAULT_COOLDOWN_SECONDS", 300)
	ChatMockDefaultRequestTimeoutSec = common.GetEnvOrDefault("CHATMOCK_REQUEST_TIMEOUT_SECONDS", 60)
	ChatMockDefaultHealthIntervalSec = common.GetEnvOrDefault("CHATMOCK_HEALTH_INTERVAL_SECONDS", 300)
	ChatMockEntitlementIntervalSec = common.GetEnvOrDefault("CHATMOCK_ENTITLEMENT_INTERVAL_SECONDS", 3600)
	ChatMockDefaultInstructions = common.GetEnvOrDefaultString("CHATMOCK_DEFAULT_INSTRUCTIONS", "You are a helpful assistant.")
}

func IsChatMockEnabled() bool {
	return ChatMockEnabled
}

func IsChatMockMailboxEnabled() bool {
	return ChatMockEnabled && ChatMockMailboxEnabled
}
