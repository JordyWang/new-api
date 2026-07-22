package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const BrowserAgentIdContextKey = "browser_agent_id"

func BrowserAgentAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if len(authorization) < 8 || !strings.EqualFold(authorization[:7], "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "browser agent token is required"})
			c.Abort()
			return
		}
		token := strings.TrimSpace(authorization[7:])
		if len(token) < 32 || len(token) > 256 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "browser agent token is invalid"})
			c.Abort()
			return
		}

		agent, err := model.GetBrowserAgentByTokenDigest(service.BrowserAgentTokenDigest(token))
		if err != nil || agent == nil || !agent.Enabled {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "browser agent token is invalid"})
			c.Abort()
			return
		}
		c.Set(BrowserAgentIdContextKey, agent.Id)
		c.Next()
	}
}
