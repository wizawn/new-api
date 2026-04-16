package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetChatMockRouter(router *gin.Engine) {
	chatMockAdmin := router.Group("/api/admin/chatmock")
	chatMockAdmin.Use(middleware.RouteTag("api"))
	chatMockAdmin.Use(middleware.AdminAuth())
	{
		chatMockAdmin.GET("/accounts", controller.AdminListChatMockAccounts)
		chatMockAdmin.GET("/accounts/summary", controller.AdminSummaryChatMockAccounts)
		chatMockAdmin.POST("/accounts", controller.AdminCreateChatMockAccount)
		chatMockAdmin.POST("/accounts/batch", controller.AdminBatchCreateChatMockAccounts)
		chatMockAdmin.POST("/accounts/batch-delete", controller.AdminBatchDeleteChatMockAccounts)
		chatMockAdmin.POST("/accounts/batch-status", controller.AdminBatchSetChatMockAccountStatus)
		chatMockAdmin.PUT("/accounts/:id", controller.AdminUpdateChatMockAccount)
		chatMockAdmin.DELETE("/accounts/:id", controller.AdminDeleteChatMockAccount)
		chatMockAdmin.POST("/accounts/:id/status", controller.AdminSetChatMockAccountStatus)
		chatMockAdmin.POST("/accounts/:id/health-check", controller.AdminHealthCheckChatMockAccount)
		chatMockAdmin.POST("/accounts/:id/entitlement-check", controller.AdminEntitlementCheckChatMockAccount)
		chatMockAdmin.POST("/accounts/:id/refresh-token", controller.AdminRefreshChatMockAccountToken)
		chatMockAdmin.POST("/accounts/health-check-all", controller.AdminBatchHealthCheckChatMockAccounts)
		chatMockAdmin.POST("/accounts/entitlement-check-all", controller.AdminBatchEntitlementCheckChatMockAccounts)
		chatMockAdmin.POST("/accounts/refresh-token-all", controller.AdminBatchRefreshChatMockAccountTokens)
		chatMockAdmin.POST("/accounts/inspect-all", controller.AdminInspectChatMockAccounts)
		chatMockAdmin.POST("/accounts/stabilize", controller.AdminStabilizeChatMockPool)
		chatMockAdmin.GET("/accounts/export", controller.AdminExportChatMockAccounts)

		chatMockAdmin.GET("/pool-config", controller.AdminGetChatMockPoolConfig)
		chatMockAdmin.PUT("/pool-config", controller.AdminUpdateChatMockPoolConfig)
		chatMockAdmin.GET("/runtime", controller.AdminGetChatMockPoolRuntime)
		chatMockAdmin.GET("/route-logs", controller.AdminListChatMockRouteLogs)
		chatMockAdmin.GET("/account-snapshots", controller.AdminListChatMockAccountSnapshots)
		chatMockAdmin.GET("/channel-status", controller.AdminGetChatMockPoolChannelStatus)
		chatMockAdmin.POST("/sync-channel", controller.AdminSyncChatMockPoolChannel)
		chatMockAdmin.POST("/accounts/route-diagnose", controller.AdminDiagnoseChatMockRoute)

		chatMockAdmin.GET("/mailboxes", controller.AdminListChatMockMailboxes)
		chatMockAdmin.POST("/mailboxes", controller.AdminCreateChatMockMailbox)
		chatMockAdmin.POST("/mailboxes/batch", controller.AdminBatchCreateChatMockMailboxes)
		chatMockAdmin.PUT("/mailboxes/:id", controller.AdminUpdateChatMockMailbox)
		chatMockAdmin.DELETE("/mailboxes/:id", controller.AdminDeleteChatMockMailbox)
		chatMockAdmin.POST("/mailboxes/:id/test", controller.AdminTestChatMockMailbox)
		chatMockAdmin.POST("/mailboxes/test-all", controller.AdminBatchTestChatMockMailboxes)
		chatMockAdmin.POST("/mailboxes/:id/pull", controller.AdminPullChatMockMailbox)
		chatMockAdmin.GET("/mailboxes/:id/latest-code", controller.AdminGetChatMockLatestCode)
		chatMockAdmin.GET("/mailboxes/export", controller.AdminExportChatMockMailboxes)

		chatMockAdmin.GET("/mail-events", controller.AdminListChatMockMailEvents)
		chatMockAdmin.POST("/mail-events/:id/reparse", controller.AdminReparseChatMockMailEvent)
	}

	chatMockPublic := router.Group("/chatmock")
	chatMockPublic.Use(middleware.RouteTag("relay"))
	chatMockPublic.Use(middleware.SystemPerformanceCheck())
	{
		chatMockPublic.POST("/account/route", controller.RouteChatMockAccount)
	}
}
