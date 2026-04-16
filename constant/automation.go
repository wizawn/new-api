package constant

const (
	AutomationModeDisabled = "disabled"
	AutomationModeLocal    = "local"
	AutomationModeRemote   = "remote"
)

const (
	AutomationModuleCloudflareEmailGenerate = "cloudflare_email_generate"
	AutomationModuleAutoRegister            = "auto_register"
	AutomationModuleTeamRedeem              = "team_redeem"
	AutomationModuleGmailInviteWatch        = "gmail_invite_watch"
	AutomationModuleBrowserLoginCapture     = "browser_login_capture"
	AutomationModuleSessionConvert          = "session_convert"
	AutomationModuleImportTarget            = "import_target"
)

const (
	AutomationExecutionStatusQueued    = "queued"
	AutomationExecutionStatusRunning   = "running"
	AutomationExecutionStatusStopping  = "stopping"
	AutomationExecutionStatusStopped   = "stopped"
	AutomationExecutionStatusCompleted = "completed"
	AutomationExecutionStatusFailed    = "failed"
)

const (
	AutomationAccountStatusPending        = "pending"
	AutomationAccountStatusGenerated      = "generated"
	AutomationAccountStatusWaitingInvite  = "waiting_for_invite"
	AutomationAccountStatusInviteReceived = "invite_received"
	AutomationAccountStatusJoinedTeam     = "joined_team"
	AutomationAccountStatusLoginCaptured  = "login_captured"
	AutomationAccountStatusConverted      = "converted"
	AutomationAccountStatusImported       = "imported"
	AutomationAccountStatusCompleted      = "completed"
	AutomationAccountStatusManualReview   = "manual_review"
	AutomationAccountStatusFailed         = "failed"
	AutomationAccountStatusDisabled       = "disabled"
)
