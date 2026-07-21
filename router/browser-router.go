package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

func registerBrowserRoutes(apiRouter *gin.RouterGroup) {
	managementRoute := apiRouter.Group("/browser")
	managementRoute.Use(middleware.RootAuth())
	{
		managementRoute.GET("/agents", controller.ListBrowserAgents)
		managementRoute.POST("/agents", controller.CreateBrowserAgent)
		managementRoute.PUT("/agents/:id", controller.UpdateBrowserAgent)
		managementRoute.POST("/agents/:id/rotate-token", controller.RotateBrowserAgentToken)
		managementRoute.DELETE("/agents/:id", controller.DeleteBrowserAgent)

		managementRoute.GET("/proxies", controller.ListBrowserProxies)
		managementRoute.POST("/proxies", controller.CreateBrowserProxy)
		managementRoute.PUT("/proxies/:id", controller.UpdateBrowserProxy)
		managementRoute.DELETE("/proxies/:id", controller.DeleteBrowserProxy)

		managementRoute.GET("/fingerprints", controller.ListBrowserFingerprints)
		managementRoute.POST("/fingerprints", controller.CreateBrowserFingerprint)
		managementRoute.PUT("/fingerprints/:id", controller.UpdateBrowserFingerprint)
		managementRoute.DELETE("/fingerprints/:id", controller.DeleteBrowserFingerprint)

		managementRoute.GET("/profiles", controller.ListBrowserProfiles)
		managementRoute.POST("/profiles", controller.CreateBrowserProfile)
		managementRoute.PUT("/profiles/:id", controller.UpdateBrowserProfile)
		managementRoute.POST("/profiles/:id/reset", controller.ResetBrowserProfile)
		managementRoute.DELETE("/profiles/:id", controller.DeleteBrowserProfile)
	}

	channelOAuthRoute := apiRouter.Group("/browser-oauth")
	channelOAuthRoute.Use(middleware.AdminAuth(), middleware.RequirePermission(authz.ChannelSensitiveWrite))
	{
		channelOAuthRoute.GET("/profiles", controller.ListAvailableBrowserProfiles)
		channelOAuthRoute.POST("/codex", controller.StartCodexBrowserOAuth)
		channelOAuthRoute.GET("/codex/:flow_id", controller.GetCodexBrowserOAuth)
		channelOAuthRoute.POST("/codex/:flow_id/cancel", controller.CancelCodexBrowserOAuth)
	}

	agentRoute := apiRouter.Group("/browser-agent")
	agentRoute.Use(middleware.BrowserAgentAuth())
	{
		agentRoute.POST("/heartbeat", controller.BrowserAgentHeartbeat)
		agentRoute.POST("/codex/claim", controller.BrowserAgentClaimCodexOAuth)
		agentRoute.POST("/codex/:flow_id/running", controller.BrowserAgentMarkCodexOAuthRunning)
		agentRoute.GET("/codex/:flow_id/status", controller.BrowserAgentGetCodexOAuthStatus)
		agentRoute.POST("/codex/:flow_id/complete", controller.BrowserAgentCompleteCodexOAuth)
		agentRoute.POST("/codex/:flow_id/fail", controller.BrowserAgentFailCodexOAuth)
	}
}
