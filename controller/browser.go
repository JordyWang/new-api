package controller

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

var browserRuntimeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type browserAgentRequest struct {
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled"`
}

type browserAgentHeartbeatRequest struct {
	InstanceId string         `json:"instance_id"`
	Version    string         `json:"version"`
	Platform   string         `json:"platform"`
	Arch       string         `json:"arch"`
	Runtimes   []string       `json:"runtimes"`
	Metadata   map[string]any `json:"metadata"`
}

type browserAgentFlowRequest struct {
	InstanceId string `json:"instance_id"`
}

type browserAgentCompleteRequest struct {
	InstanceId   string `json:"instance_id"`
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type browserAgentFailRequest struct {
	InstanceId string `json:"instance_id"`
	Message    string `json:"message"`
}

type browserProxyRequest struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled *bool  `json:"enabled"`
}

type browserFingerprintRequest struct {
	Name        string `json:"name"`
	UserAgent   string `json:"user_agent"`
	ViewportW   int    `json:"viewport_width"`
	ViewportH   int    `json:"viewport_height"`
	Payload     string `json:"payload"`
	LaunchArgs  string `json:"launch_args"`
	Environment string `json:"environment"`
	Enabled     *bool  `json:"enabled"`
}

type browserProfileRequest struct {
	Name          string `json:"name"`
	AgentId       int    `json:"agent_id"`
	ProxyId       int    `json:"proxy_id"`
	FingerprintId int    `json:"fingerprint_id"`
	RuntimeKey    string `json:"runtime_key"`
	Persistent    *bool  `json:"persistent"`
	Enabled       *bool  `json:"enabled"`
}

type startCodexBrowserOAuthRequest struct {
	ProfileId int `json:"profile_id"`
	ChannelId int `json:"channel_id"`
}

type browserAgentResponse struct {
	Id         int            `json:"id"`
	Name       string         `json:"name"`
	Enabled    bool           `json:"enabled"`
	Status     string         `json:"status"`
	Online     bool           `json:"online"`
	LastSeenAt int64          `json:"last_seen_at"`
	Version    string         `json:"version"`
	Platform   string         `json:"platform"`
	Arch       string         `json:"arch"`
	Runtimes   []string       `json:"runtimes"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
}

type browserProxyResponse struct {
	Id             int    `json:"id"`
	Name           string `json:"name"`
	Scheme         string `json:"scheme"`
	URLMasked      string `json:"url_masked"`
	HasCredentials bool   `json:"has_credentials"`
	Enabled        bool   `json:"enabled"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type browserProfileResponse struct {
	model.BrowserProfile
	AgentName       string   `json:"agent_name"`
	AgentOnline     bool     `json:"agent_online"`
	AgentRuntimes   []string `json:"agent_runtimes"`
	ProxyName       string   `json:"proxy_name"`
	FingerprintName string   `json:"fingerprint_name"`
}

func ListBrowserAgents(c *gin.Context) {
	agents, err := model.ListBrowserAgents()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]browserAgentResponse, 0, len(agents))
	for index := range agents {
		data = append(data, browserAgentToResponse(&agents[index]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func CreateBrowserAgent(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request browserAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 128 {
		common.ApiErrorMsg(c, "browser agent name is required and must not exceed 128 characters")
		return
	}
	tokenBody, err := common.GenerateRandomCharsKey(64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := "nba_" + tokenBody
	now := time.Now().Unix()
	agent := &model.BrowserAgent{
		Name:        name,
		TokenDigest: service.BrowserAgentTokenDigest(token),
		Enabled:     boolOrDefault(request.Enabled, true),
		Status:      model.BrowserAgentStatusOffline,
		Runtimes:    "[]",
		Metadata:    "{}",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.CreateBrowserAgent(agent); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_agent.create", map[string]interface{}{"id": agent.Id, "name": agent.Name})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"agent": browserAgentToResponse(agent),
			"token": token,
		},
	})
}

func UpdateBrowserAgent(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	var request browserAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	agent, err := model.GetBrowserAgentById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(request.Name) != "" {
		agent.Name = strings.TrimSpace(request.Name)
	}
	if len(agent.Name) > 128 {
		common.ApiErrorMsg(c, "browser agent name must not exceed 128 characters")
		return
	}
	if request.Enabled != nil {
		agent.Enabled = *request.Enabled
	}
	agent.UpdatedAt = time.Now().Unix()
	if err := model.UpdateBrowserAgent(agent); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_agent.update", map[string]interface{}{"id": agent.Id, "name": agent.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": browserAgentToResponse(agent)})
}

func RotateBrowserAgentToken(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	if _, err := model.GetBrowserAgentById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	tokenBody, err := common.GenerateRandomCharsKey(64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := "nba_" + tokenBody
	if err := model.RotateBrowserAgentToken(id, service.BrowserAgentTokenDigest(token), time.Now().Unix()); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_agent.rotate_token", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"token": token}})
}

func DeleteBrowserAgent(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	count, err := model.CountBrowserProfilesByAgent(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "delete or reassign browser profiles before deleting this agent")
		return
	}
	if err := model.DeleteBrowserAgent(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_agent.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func ListBrowserProxies(c *gin.Context) {
	proxies, err := model.ListBrowserProxies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]browserProxyResponse, 0, len(proxies))
	for index := range proxies {
		response, err := browserProxyToResponse(&proxies[index])
		if err != nil {
			common.ApiError(c, err)
			return
		}
		data = append(data, response)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func CreateBrowserProxy(c *gin.Context) {
	var request browserProxyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 128 {
		common.ApiErrorMsg(c, "browser proxy name is required and must not exceed 128 characters")
		return
	}
	proxyURL, scheme, err := service.ValidateBrowserProxyURL(request.URL)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	ciphertext, err := service.EncryptBrowserProxyURL(proxyURL)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := time.Now().Unix()
	proxy := &model.BrowserProxy{
		Name:          name,
		URLCiphertext: ciphertext,
		Scheme:        scheme,
		Enabled:       boolOrDefault(request.Enabled, true),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := model.CreateBrowserProxy(proxy); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetBrowserProxyURLCache()
	response, err := browserProxyToResponse(proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_proxy.create", map[string]interface{}{"id": proxy.Id, "name": proxy.Name, "scheme": proxy.Scheme})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

func UpdateBrowserProxy(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	var request browserProxyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	proxy, err := model.GetBrowserProxyById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(request.Name) != "" {
		proxy.Name = strings.TrimSpace(request.Name)
	}
	if len(proxy.Name) > 128 {
		common.ApiErrorMsg(c, "browser proxy name must not exceed 128 characters")
		return
	}
	updateURL := strings.TrimSpace(request.URL) != ""
	if updateURL {
		proxyURL, scheme, err := service.ValidateBrowserProxyURL(request.URL)
		if err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		proxy.URLCiphertext, err = service.EncryptBrowserProxyURL(proxyURL)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		proxy.Scheme = scheme
	}
	if request.Enabled != nil {
		proxy.Enabled = *request.Enabled
	}
	proxy.UpdatedAt = time.Now().Unix()
	if err := model.UpdateBrowserProxy(proxy, updateURL); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetBrowserProxyURLCache()
	service.ResetProxyClientCache()
	response, err := browserProxyToResponse(proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_proxy.update", map[string]interface{}{"id": proxy.Id, "name": proxy.Name, "scheme": proxy.Scheme, "url_rotated": updateURL})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

func DeleteBrowserProxy(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	count, err := model.CountBrowserProfilesByProxy(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "delete or reassign browser profiles before deleting this proxy")
		return
	}
	channelCount, err := model.CountChannelsByBrowserProxy(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channelCount > 0 {
		common.ApiErrorMsg(c, "remove this managed proxy from channels before deleting it")
		return
	}
	if err := model.DeleteBrowserProxy(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetBrowserProxyURLCache()
	service.ResetProxyClientCache()
	recordManageAudit(c, "browser_proxy.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func ListBrowserFingerprints(c *gin.Context) {
	fingerprints, err := model.ListBrowserFingerprints()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": fingerprints})
}

func CreateBrowserFingerprint(c *gin.Context) {
	var request browserFingerprintRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	fingerprint, err := normalizeBrowserFingerprintRequest(request, nil)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	now := time.Now().Unix()
	fingerprint.CreatedAt = now
	fingerprint.UpdatedAt = now
	if err := model.CreateBrowserFingerprint(fingerprint); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_fingerprint.create", map[string]interface{}{"id": fingerprint.Id, "name": fingerprint.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": fingerprint})
}

func UpdateBrowserFingerprint(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	var request browserFingerprintRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetBrowserFingerprintById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fingerprint, err := normalizeBrowserFingerprintRequest(request, existing)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	fingerprint.Id = id
	fingerprint.UpdatedAt = time.Now().Unix()
	if err := model.UpdateBrowserFingerprint(fingerprint); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_fingerprint.update", map[string]interface{}{"id": fingerprint.Id, "name": fingerprint.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": fingerprint})
}

func DeleteBrowserFingerprint(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	count, err := model.CountBrowserProfilesByFingerprint(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "delete or reassign browser profiles before deleting this fingerprint")
		return
	}
	if err := model.DeleteBrowserFingerprint(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_fingerprint.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func ListBrowserProfiles(c *gin.Context) {
	profiles, err := browserProfileResponses(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profiles})
}

func ListAvailableBrowserProfiles(c *gin.Context) {
	profiles, err := browserProfileResponses(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profiles})
}

func CreateBrowserProfile(c *gin.Context) {
	var request browserProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	profile, err := normalizeBrowserProfileRequest(request, nil)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	dataKey, err := common.GenerateRandomCharsKey(40)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := time.Now().Unix()
	profile.DataKey = dataKey
	profile.CreatedAt = now
	profile.UpdatedAt = now
	if err := model.CreateBrowserProfile(profile); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.create", map[string]interface{}{"id": profile.Id, "name": profile.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profile})
}

func UpdateBrowserProfile(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	var request browserProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetBrowserProfileById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profile, err := normalizeBrowserProfileRequest(request, existing)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	profile.Id = id
	profile.DataKey = existing.DataKey
	profile.UpdatedAt = time.Now().Unix()
	if err := model.UpdateBrowserProfile(profile); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.update", map[string]interface{}{"id": profile.Id, "name": profile.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profile})
}

func ResetBrowserProfile(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	if _, err := model.GetBrowserProfileById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	active, err := model.HasUnfinishedCodexOAuthFlowForProfile(id, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if active {
		common.ApiErrorMsg(c, "cancel or finish the active OAuth flow before resetting this profile")
		return
	}
	dataKey, err := common.GenerateRandomCharsKey(40)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.RotateBrowserProfileDataKey(id, dataKey, time.Now().Unix()); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.reset", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteBrowserProfile(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	active, err := model.HasUnfinishedCodexOAuthFlowForProfile(id, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if active {
		common.ApiErrorMsg(c, "cancel or finish the active OAuth flow before deleting this profile")
		return
	}
	if err := model.DeleteBrowserProfile(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func StartCodexBrowserOAuth(c *gin.Context) {
	var request startCodexBrowserOAuthRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.ProfileId <= 0 {
		common.ApiErrorMsg(c, "browser profile is required")
		return
	}
	if request.ChannelId < 0 {
		common.ApiErrorMsg(c, "channel ID is invalid")
		return
	}
	flow, err := service.StartCodexBrowserOAuthFlow(c.GetInt("id"), request.ChannelId, request.ProfileId)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "codex_oauth.start", map[string]interface{}{"flow_id": flow.Id, "profile_id": flow.ProfileId, "channel_id": flow.ChannelId})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": flow})
}

func GetCodexBrowserOAuth(c *gin.Context) {
	flow, err := service.GetCodexBrowserOAuthFlow(c.GetInt("id"), c.GetInt("role") == common.RoleRootUser, c.Param("flow_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": flow})
}

func CancelCodexBrowserOAuth(c *gin.Context) {
	flowId := c.Param("flow_id")
	if err := service.CancelCodexBrowserOAuthFlow(c.GetInt("id"), c.GetInt("role") == common.RoleRootUser, flowId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "codex_oauth.cancel", map[string]interface{}{"flow_id": flowId})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentHeartbeat(c *gin.Context) {
	var request browserAgentHeartbeatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	version := strings.TrimSpace(request.Version)
	platform := strings.TrimSpace(request.Platform)
	arch := strings.TrimSpace(request.Arch)
	if len(version) > 64 || len(platform) > 32 || len(arch) > 32 {
		common.ApiErrorMsg(c, "browser agent version or platform metadata is too long")
		return
	}
	runtimes, err := normalizeBrowserRuntimeKeys(request.Runtimes)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	runtimeBytes, err := common.Marshal(runtimes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	metadataBytes, err := common.Marshal(request.Metadata)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(metadataBytes) > 16*1024 {
		common.ApiErrorMsg(c, "browser agent metadata is too large")
		return
	}
	now := time.Now().Unix()
	if err := model.TouchBrowserAgent(
		c.GetInt(middleware.BrowserAgentIdContextKey),
		version,
		platform,
		arch,
		string(runtimeBytes),
		string(metadataBytes),
		now,
	); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"server_time": now}})
}

func BrowserAgentClaimCodexOAuth(c *gin.Context) {
	var request browserAgentFlowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	claim, err := service.ClaimCodexBrowserOAuthFlow(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": claim})
}

func BrowserAgentMarkCodexOAuthRunning(c *gin.Context) {
	var request browserAgentFlowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if err := service.MarkCodexBrowserOAuthFlowRunning(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId, c.Param("flow_id")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentCompleteCodexOAuth(c *gin.Context) {
	var request browserAgentCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if len(request.IDToken) > 64*1024 || len(request.AccessToken) > 64*1024 || len(request.RefreshToken) > 64*1024 {
		common.ApiErrorMsg(c, "OAuth token payload is too large")
		return
	}
	if err := service.CompleteCodexBrowserOAuthFlow(
		c.GetInt(middleware.BrowserAgentIdContextKey),
		request.InstanceId,
		c.Param("flow_id"),
		service.CodexBrowserOAuthCompletion{
			IDToken:      request.IDToken,
			AccessToken:  request.AccessToken,
			RefreshToken: request.RefreshToken,
			ExpiresIn:    request.ExpiresIn,
		},
	); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentFailCodexOAuth(c *gin.Context) {
	var request browserAgentFailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if err := service.FailCodexBrowserOAuthFlow(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId, c.Param("flow_id"), request.Message); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentGetCodexOAuthStatus(c *gin.Context) {
	instanceId := strings.TrimSpace(c.Query("instance_id"))
	if !validBrowserAgentInstanceId(instanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	status, err := service.GetAgentCodexBrowserOAuthFlowStatus(c.GetInt(middleware.BrowserAgentIdContextKey), instanceId, c.Param("flow_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"status": status}})
}

func browserAgentToResponse(agent *model.BrowserAgent) browserAgentResponse {
	var runtimes []string
	_ = common.UnmarshalJsonStr(agent.Runtimes, &runtimes)
	var metadata map[string]any
	_ = common.UnmarshalJsonStr(agent.Metadata, &metadata)
	if metadata == nil {
		metadata = make(map[string]any)
	}
	online := agent.Enabled && agent.LastSeenAt >= time.Now().Unix()-45
	status := model.BrowserAgentStatusOffline
	if online {
		status = model.BrowserAgentStatusOnline
	}
	return browserAgentResponse{
		Id:         agent.Id,
		Name:       agent.Name,
		Enabled:    agent.Enabled,
		Status:     status,
		Online:     online,
		LastSeenAt: agent.LastSeenAt,
		Version:    agent.Version,
		Platform:   agent.Platform,
		Arch:       agent.Arch,
		Runtimes:   runtimes,
		Metadata:   metadata,
		CreatedAt:  agent.CreatedAt,
		UpdatedAt:  agent.UpdatedAt,
	}
}

func browserProxyToResponse(proxy *model.BrowserProxy) (browserProxyResponse, error) {
	proxyURL, err := service.DecryptBrowserProxyURL(proxy)
	if err != nil {
		return browserProxyResponse{}, err
	}
	parsed, _ := url.Parse(proxyURL)
	return browserProxyResponse{
		Id:             proxy.Id,
		Name:           proxy.Name,
		Scheme:         proxy.Scheme,
		URLMasked:      service.MaskBrowserProxyURL(proxyURL),
		HasCredentials: parsed != nil && parsed.User != nil,
		Enabled:        proxy.Enabled,
		CreatedAt:      proxy.CreatedAt,
		UpdatedAt:      proxy.UpdatedAt,
	}, nil
}

func normalizeBrowserFingerprintRequest(request browserFingerprintRequest, existing *model.BrowserFingerprint) (*model.BrowserFingerprint, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" && existing != nil {
		name = existing.Name
	}
	if name == "" || len(name) > 128 {
		return nil, errors.New("browser fingerprint name is required and must not exceed 128 characters")
	}
	userAgent := strings.TrimSpace(request.UserAgent)
	if len(userAgent) > 4096 || strings.ContainsRune(userAgent, '\x00') {
		return nil, errors.New("browser user agent is invalid")
	}
	viewportW := request.ViewportW
	if viewportW == 0 {
		viewportW = 1280
	}
	viewportH := request.ViewportH
	if viewportH == 0 {
		viewportH = 800
	}
	if viewportW < 320 || viewportW > 7680 || viewportH < 240 || viewportH > 4320 {
		return nil, errors.New("browser viewport is outside the supported range")
	}

	payload, err := normalizeCoreFingerprintPayload(request.Payload)
	if err != nil {
		return nil, err
	}
	launchArgs, err := normalizeStringArray(request.LaunchArgs, "fingerprint launch arguments")
	if err != nil {
		return nil, err
	}
	environment, err := normalizeStringMap(request.Environment, "fingerprint environment")
	if err != nil {
		return nil, err
	}

	enabled := true
	if existing != nil {
		enabled = existing.Enabled
	}
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	return &model.BrowserFingerprint{
		Name:        name,
		UserAgent:   userAgent,
		ViewportW:   viewportW,
		ViewportH:   viewportH,
		Payload:     payload,
		LaunchArgs:  launchArgs,
		Environment: environment,
		Enabled:     enabled,
	}, nil
}

func normalizeCoreFingerprintPayload(raw string) (string, error) {
	normalized, err := normalizeJSONObject(raw, "fingerprint payload")
	if err != nil {
		return "", err
	}
	var payload map[string]any
	if err := common.UnmarshalJsonStr(normalized, &payload); err != nil {
		return "", err
	}
	for _, key := range []string{"accept_language", "accept_languages", "country_code", "geo_overlay", "languages", "locale", "timezone", "timezone_id"} {
		delete(payload, key)
	}
	fingerprintValue, hasFingerprint := payload["fingerprint"]
	if hasFingerprint {
		fingerprint, ok := fingerprintValue.(map[string]any)
		if !ok {
			return "", errors.New("fingerprint payload fingerprint field must be a JSON object")
		}
		for _, key := range []string{"accept_language", "accept_languages", "country_code", "geo_overlay", "languages", "locale", "timezone", "timezone_id"} {
			delete(fingerprint, key)
		}
		if navigator, ok := fingerprint["navigator"].(map[string]any); ok {
			delete(navigator, "language")
			delete(navigator, "languages")
		}
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizeBrowserProfileRequest(request browserProfileRequest, existing *model.BrowserProfile) (*model.BrowserProfile, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" && existing != nil {
		name = existing.Name
	}
	if name == "" || len(name) > 128 {
		return nil, errors.New("browser profile name is required and must not exceed 128 characters")
	}
	if !browserRuntimeKeyPattern.MatchString(strings.TrimSpace(request.RuntimeKey)) {
		return nil, errors.New("browser runtime key is invalid")
	}
	if err := model.ValidateBrowserModelReferences(request.AgentId, request.ProxyId, request.FingerprintId); err != nil {
		return nil, err
	}
	persistent := true
	enabled := true
	if existing != nil {
		persistent = existing.Persistent
		enabled = existing.Enabled
	}
	if request.Persistent != nil {
		persistent = *request.Persistent
	}
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	return &model.BrowserProfile{
		Name:          name,
		AgentId:       request.AgentId,
		ProxyId:       request.ProxyId,
		FingerprintId: request.FingerprintId,
		RuntimeKey:    strings.TrimSpace(request.RuntimeKey),
		Persistent:    persistent,
		Enabled:       enabled,
	}, nil
}

func browserProfileResponses(availableOnly bool) ([]browserProfileResponse, error) {
	profiles, err := model.ListBrowserProfiles()
	if err != nil {
		return nil, err
	}
	agents, err := model.ListBrowserAgents()
	if err != nil {
		return nil, err
	}
	proxies, err := model.ListBrowserProxies()
	if err != nil {
		return nil, err
	}
	fingerprints, err := model.ListBrowserFingerprints()
	if err != nil {
		return nil, err
	}

	agentMap := make(map[int]model.BrowserAgent, len(agents))
	for index := range agents {
		agentMap[agents[index].Id] = agents[index]
	}
	proxyMap := make(map[int]model.BrowserProxy, len(proxies))
	for index := range proxies {
		proxyMap[proxies[index].Id] = proxies[index]
	}
	fingerprintMap := make(map[int]model.BrowserFingerprint, len(fingerprints))
	for index := range fingerprints {
		fingerprintMap[fingerprints[index].Id] = fingerprints[index]
	}

	now := time.Now().Unix()
	responses := make([]browserProfileResponse, 0, len(profiles))
	for index := range profiles {
		profile := profiles[index]
		agent, agentFound := agentMap[profile.AgentId]
		proxy, proxyFound := proxyMap[profile.ProxyId]
		fingerprint, fingerprintFound := fingerprintMap[profile.FingerprintId]
		if availableOnly && (!profile.Enabled || !agentFound || !agent.Enabled || agent.LastSeenAt < now-45 || !proxyFound || !proxy.Enabled || !fingerprintFound || !fingerprint.Enabled) {
			continue
		}
		var runtimes []string
		_ = common.UnmarshalJsonStr(agent.Runtimes, &runtimes)
		if availableOnly {
			runtimeAvailable := false
			for _, runtimeKey := range runtimes {
				if runtimeKey == profile.RuntimeKey {
					runtimeAvailable = true
					break
				}
			}
			if !runtimeAvailable {
				continue
			}
		}
		responses = append(responses, browserProfileResponse{
			BrowserProfile:  profile,
			AgentName:       agent.Name,
			AgentOnline:     agent.Enabled && agent.LastSeenAt >= now-45,
			AgentRuntimes:   runtimes,
			ProxyName:       proxy.Name,
			FingerprintName: fingerprint.Name,
		})
	}
	return responses, nil
}

func normalizeJSONObject(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}", nil
	}
	var value map[string]any
	if err := common.UnmarshalJsonStr(trimmed, &value); err != nil || value == nil {
		return "", errors.New(label + " must be a JSON object")
	}
	encoded, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	if len(encoded) > 64*1024 {
		return "", errors.New(label + " is too large")
	}
	return string(encoded), nil
}

func normalizeStringArray(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "[]", nil
	}
	var values []string
	if err := common.UnmarshalJsonStr(trimmed, &values); err != nil {
		return "", errors.New(label + " must be a JSON string array")
	}
	if len(values) > 64 {
		return "", errors.New(label + " must contain at most 64 entries")
	}
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
		if values[index] == "" || len(values[index]) > 2048 || strings.ContainsRune(values[index], '\x00') {
			return "", errors.New(label + " contains an invalid entry")
		}
	}
	encoded, err := common.Marshal(values)
	if err != nil {
		return "", err
	}
	if len(encoded) > 64*1024 {
		return "", errors.New(label + " is too large")
	}
	return string(encoded), nil
}

func normalizeStringMap(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}", nil
	}
	var values map[string]string
	if err := common.UnmarshalJsonStr(trimmed, &values); err != nil || values == nil {
		return "", errors.New(label + " must be a JSON string map")
	}
	if len(values) > 64 {
		return "", errors.New(label + " must contain at most 64 entries")
	}
	for key, value := range values {
		if !browserRuntimeKeyPattern.MatchString(key) || len(value) > 4096 || strings.ContainsRune(value, '\x00') {
			return "", errors.New(label + " contains an invalid entry")
		}
	}
	encoded, err := common.Marshal(values)
	if err != nil {
		return "", err
	}
	if len(encoded) > 64*1024 {
		return "", errors.New(label + " is too large")
	}
	return string(encoded), nil
}

func normalizeBrowserRuntimeKeys(values []string) ([]string, error) {
	if len(values) > 64 {
		return nil, errors.New("browser agent must advertise at most 64 runtimes")
	}
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !browserRuntimeKeyPattern.MatchString(value) {
			return nil, errors.New("browser runtime key is invalid")
		}
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validBrowserAgentInstanceId(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed) <= 64 && browserRuntimeKeyPattern.MatchString(trimmed)
}

func browserResourceId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid resource ID")
		return 0, false
	}
	return id, true
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
