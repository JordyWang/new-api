package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	Name               string `json:"name"`
	URL                string `json:"url"`
	Enabled            *bool  `json:"enabled"`
	MaxChannelAccounts *int   `json:"max_channel_accounts"`
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
	Name                    string `json:"name"`
	ChannelId               *int   `json:"channel_id"`
	AgentId                 int    `json:"agent_id"`
	ProxyId                 int    `json:"proxy_id"`
	FingerprintId           int    `json:"fingerprint_id"`
	AutoGenerateFingerprint bool   `json:"auto_generate_fingerprint"`
	RuntimeKey              string `json:"runtime_key"`
	Persistent              *bool  `json:"persistent"`
	Enabled                 *bool  `json:"enabled"`
}

type startCodexBrowserOAuthRequest struct {
	ProfileId           int    `json:"profile_id"`
	ChannelId           int    `json:"channel_id"`
	AutoGenerateProfile bool   `json:"auto_generate_profile"`
	ProfileName         string `json:"profile_name"`
	AgentId             int    `json:"agent_id"`
	ProxyId             int    `json:"proxy_id"`
	RuntimeKey          string `json:"runtime_key"`
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
	Id                  int    `json:"id"`
	Name                string `json:"name"`
	Scheme              string `json:"scheme"`
	URLMasked           string `json:"url_masked"`
	HasCredentials      bool   `json:"has_credentials"`
	Enabled             bool   `json:"enabled"`
	MaxChannelAccounts  int    `json:"max_channel_accounts"`
	ChannelAccountCount int64  `json:"channel_account_count"`
	ProfileCount        int64  `json:"profile_count"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
}

type browserProfileResponse struct {
	model.BrowserProfile
	ChannelName              string   `json:"channel_name"`
	AgentName                string   `json:"agent_name"`
	AgentOnline              bool     `json:"agent_online"`
	AgentRuntimes            []string `json:"agent_runtimes"`
	ProxyName                string   `json:"proxy_name"`
	ProxyMaxChannelAccounts  int      `json:"proxy_max_channel_accounts"`
	ProxyChannelAccountCount int64    `json:"proxy_channel_account_count"`
	ProxyProfileCount        int64    `json:"proxy_profile_count"`
	ProxyAtCapacity          bool     `json:"proxy_at_capacity"`
	FingerprintName          string   `json:"fingerprint_name"`
	ActiveLaunchId           string   `json:"active_launch_id"`
	ActiveLaunchStatus       string   `json:"active_launch_status"`
}

type browserProfileChannelResponse struct {
	Id               int    `json:"id"`
	Name             string `json:"name"`
	BrowserProxyId   int    `json:"browser_proxy_id"`
	BrowserProfileId *int   `json:"browser_profile_id"`
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
	proxyIds := make([]int, 0, len(proxies))
	for index := range proxies {
		proxyIds = append(proxyIds, proxies[index].Id)
	}
	channelCounts, err := model.CountChannelsByBrowserProxies(proxyIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profileCounts, err := model.CountBrowserProfilesByProxies(proxyIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]browserProxyResponse, 0, len(proxies))
	for index := range proxies {
		response, err := browserProxyToResponse(&proxies[index], channelCounts[proxies[index].Id], profileCounts[proxies[index].Id])
		if err != nil {
			common.ApiError(c, err)
			return
		}
		data = append(data, response)
	}
	sort.SliceStable(data, func(left int, right int) bool {
		if data[left].ProfileCount != data[right].ProfileCount {
			return data[left].ProfileCount < data[right].ProfileCount
		}
		if data[left].ChannelAccountCount != data[right].ChannelAccountCount {
			return data[left].ChannelAccountCount < data[right].ChannelAccountCount
		}
		return data[left].Id < data[right].Id
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func ListAvailableBrowserProxies(c *gin.Context) {
	ListBrowserProxies(c)
}

func ListAvailableBrowserRuntimes(c *gin.Context) {
	options, err := service.ListReadyBrowserRuntimeOptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": options})
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
	maxChannelAccounts := model.DefaultBrowserProxyChannelAccounts
	if request.MaxChannelAccounts != nil {
		maxChannelAccounts = *request.MaxChannelAccounts
	}
	if maxChannelAccounts < 0 || maxChannelAccounts > model.MaxBrowserProxyChannelAccounts {
		common.ApiErrorMsg(c, fmt.Sprintf("managed proxy channel account limit must be between 0 and %d", model.MaxBrowserProxyChannelAccounts))
		return
	}
	proxy := &model.BrowserProxy{
		Name:               name,
		URLCiphertext:      ciphertext,
		Scheme:             scheme,
		Enabled:            boolOrDefault(request.Enabled, true),
		MaxChannelAccounts: maxChannelAccounts,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := model.CreateBrowserProxy(proxy); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetBrowserProxyURLCache()
	response, err := browserProxyToResponse(proxy, 0, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_proxy.create", map[string]interface{}{"id": proxy.Id, "name": proxy.Name, "scheme": proxy.Scheme, "max_channel_accounts": proxy.MaxChannelAccounts})
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
	if request.MaxChannelAccounts != nil {
		if *request.MaxChannelAccounts < 0 || *request.MaxChannelAccounts > model.MaxBrowserProxyChannelAccounts {
			common.ApiErrorMsg(c, fmt.Sprintf("managed proxy channel account limit must be between 0 and %d", model.MaxBrowserProxyChannelAccounts))
			return
		}
		proxy.MaxChannelAccounts = *request.MaxChannelAccounts
	}
	proxy.UpdatedAt = time.Now().Unix()
	if err := model.UpdateBrowserProxy(proxy, updateURL); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetBrowserProxyURLCache()
	service.ResetProxyClientCache()
	channelCount, err := model.CountChannelsByBrowserProxy(proxy.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profileCount, err := model.CountBrowserProfilesByProxy(proxy.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := browserProxyToResponse(proxy, channelCount, profileCount)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_proxy.update", map[string]interface{}{"id": proxy.Id, "name": proxy.Name, "scheme": proxy.Scheme, "url_rotated": updateURL, "max_channel_accounts": proxy.MaxChannelAccounts})
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
	profiles, err := browserProfileResponses(false, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profiles})
}

func ListAvailableBrowserProfiles(c *gin.Context) {
	channelId := 0
	if raw := strings.TrimSpace(c.Query("channel_id")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 0 {
			common.ApiErrorMsg(c, "channel ID is invalid")
			return
		}
		channelId = parsed
	}
	profiles, err := browserProfileResponses(true, channelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": profiles})
}

func ListBrowserProfileChannels(c *gin.Context) {
	channels, err := model.GetChannelsByType(0, -1, true, constant.ChannelTypeCodex)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profiles, err := model.ListBrowserProfiles()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	boundProfiles := make(map[int]int, len(profiles))
	for index := range profiles {
		if profiles[index].ChannelId != nil {
			boundProfiles[*profiles[index].ChannelId] = profiles[index].Id
		}
	}
	data := make([]browserProfileChannelResponse, 0, len(channels))
	for _, channel := range channels {
		profileId := boundProfiles[channel.Id]
		var profileIdPointer *int
		if profileId > 0 {
			profileIdPointer = &profileId
		}
		data = append(data, browserProfileChannelResponse{
			Id:               channel.Id,
			Name:             channel.Name,
			BrowserProxyId:   channel.GetSetting().BrowserProxyId,
			BrowserProfileId: profileIdPointer,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
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
	now := time.Now().Unix()
	if request.AutoGenerateFingerprint {
		profile, _, err = service.CreateGeneratedBrowserProfile(
			profile.Name,
			profile.ChannelId,
			profile.AgentId,
			profile.ProxyId,
			profile.RuntimeKey,
			profile.Persistent,
			profile.Enabled,
			now,
		)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		dataKey, generateErr := common.GenerateRandomCharsKey(40)
		if generateErr != nil {
			common.ApiError(c, generateErr)
			return
		}
		profile.DataKey = dataKey
		profile.CreatedAt = now
		profile.UpdatedAt = now
		if err := model.CreateBrowserProfile(profile); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	recordManageAudit(c, "browser_profile.create", map[string]interface{}{"id": profile.Id, "name": profile.Name, "auto_generated_fingerprint": request.AutoGenerateFingerprint})
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
	if request.AutoGenerateFingerprint {
		common.ApiErrorMsg(c, "automatic fingerprint generation is only available when creating a browser profile")
		return
	}
	existing, err := model.GetBrowserProfileById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	activeLaunch, err := model.HasActiveBrowserLaunchForProfile(id, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activeLaunch {
		common.ApiErrorMsg(c, "stop the active browser before updating this profile")
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
	activeLaunch, err := model.HasActiveBrowserLaunchForProfile(id, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activeLaunch {
		common.ApiErrorMsg(c, "stop the active browser before resetting this profile")
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
	activeLaunch, err := model.HasActiveBrowserLaunchForProfile(id, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activeLaunch {
		common.ApiErrorMsg(c, "stop the active browser before deleting this profile")
		return
	}
	if err := model.DeleteBrowserProfile(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func StartBrowserProfileLaunch(c *gin.Context) {
	id, ok := browserResourceId(c)
	if !ok {
		return
	}
	launch, err := service.StartBrowserProfileLaunch(id)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "browser_profile.launch", map[string]interface{}{"profile_id": id, "launch_id": launch.Id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": launch})
}

func GetBrowserProfileLaunch(c *gin.Context) {
	launch, err := service.GetBrowserProfileLaunch(c.Param("launch_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": launch})
}

func CancelBrowserProfileLaunch(c *gin.Context) {
	launchId := c.Param("launch_id")
	if err := service.CancelBrowserProfileLaunch(launchId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "browser_profile.stop", map[string]interface{}{"launch_id": launchId})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func StartCodexBrowserOAuth(c *gin.Context) {
	var request startCodexBrowserOAuthRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.ChannelId < 0 {
		common.ApiErrorMsg(c, "channel ID is invalid")
		return
	}
	var flow *service.CodexBrowserOAuthFlowView
	var err error
	if request.AutoGenerateProfile {
		if request.ChannelId != 0 {
			common.ApiErrorMsg(c, "automatic browser profile generation is only available when creating a channel")
			return
		}
		profileName := strings.TrimSpace(request.ProfileName)
		if len(profileName) > 128 {
			common.ApiErrorMsg(c, "browser profile name must not exceed 128 characters")
			return
		}
		if request.AgentId <= 0 || request.ProxyId <= 0 || !browserRuntimeKeyPattern.MatchString(strings.TrimSpace(request.RuntimeKey)) {
			common.ApiErrorMsg(c, "browser agent, runtime, and managed proxy are required for automatic profile generation")
			return
		}
		flow, err = service.StartCodexBrowserOAuthWithGeneratedProfile(
			c.GetInt("id"),
			profileName,
			request.AgentId,
			request.ProxyId,
			strings.TrimSpace(request.RuntimeKey),
		)
	} else {
		if request.ProfileId <= 0 {
			common.ApiErrorMsg(c, "browser profile is required")
			return
		}
		flow, err = service.StartCodexBrowserOAuthFlow(c.GetInt("id"), request.ChannelId, request.ProfileId)
	}
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "codex_oauth.start", map[string]interface{}{"flow_id": flow.Id, "profile_id": flow.ProfileId, "channel_id": flow.ChannelId, "auto_generated_profile": request.AutoGenerateProfile})
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

func BrowserAgentClaimBrowserLaunch(c *gin.Context) {
	var request browserAgentFlowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	claim, err := service.ClaimBrowserProfileLaunch(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": claim})
}

func BrowserAgentMarkBrowserLaunchRunning(c *gin.Context) {
	var request browserAgentFlowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if err := service.MarkBrowserProfileLaunchRunning(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId, c.Param("launch_id")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentCompleteBrowserLaunch(c *gin.Context) {
	var request browserAgentFlowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if err := service.CompleteBrowserProfileLaunch(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId, c.Param("launch_id")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentFailBrowserLaunch(c *gin.Context) {
	var request browserAgentFailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validBrowserAgentInstanceId(request.InstanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	if err := service.FailBrowserProfileLaunch(c.GetInt(middleware.BrowserAgentIdContextKey), request.InstanceId, c.Param("launch_id"), request.Message); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func BrowserAgentGetBrowserLaunchStatus(c *gin.Context) {
	instanceId := strings.TrimSpace(c.Query("instance_id"))
	if !validBrowserAgentInstanceId(instanceId) {
		common.ApiErrorMsg(c, "browser agent instance ID is invalid")
		return
	}
	status, err := service.GetAgentBrowserProfileLaunchStatus(c.GetInt(middleware.BrowserAgentIdContextKey), instanceId, c.Param("launch_id"))
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

func browserProxyToResponse(proxy *model.BrowserProxy, channelAccountCount int64, profileCount int64) (browserProxyResponse, error) {
	proxyURL, err := service.DecryptBrowserProxyURL(proxy)
	if err != nil {
		return browserProxyResponse{}, err
	}
	parsed, _ := url.Parse(proxyURL)
	return browserProxyResponse{
		Id:                  proxy.Id,
		Name:                proxy.Name,
		Scheme:              proxy.Scheme,
		URLMasked:           service.MaskBrowserProxyURL(proxyURL),
		HasCredentials:      parsed != nil && parsed.User != nil,
		Enabled:             proxy.Enabled,
		MaxChannelAccounts:  proxy.MaxChannelAccounts,
		ChannelAccountCount: channelAccountCount,
		ProfileCount:        profileCount,
		CreatedAt:           proxy.CreatedAt,
		UpdatedAt:           proxy.UpdatedAt,
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
	corePayload := payload
	fingerprintValue, hasFingerprint := payload["fingerprint"]
	if hasFingerprint {
		fingerprint, ok := fingerprintValue.(map[string]any)
		if !ok {
			return "", errors.New("fingerprint payload fingerprint field must be a JSON object")
		}
		corePayload = fingerprint
	}
	for _, key := range []string{"accept_language", "accept_languages", "country_code", "geo_overlay", "languages", "locale", "timezone", "timezone_id"} {
		delete(corePayload, key)
	}
	if navigator, ok := corePayload["navigator"].(map[string]any); ok {
		delete(navigator, "language")
		delete(navigator, "languages")
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
	if request.AutoGenerateFingerprint {
		if existing != nil {
			return nil, errors.New("automatic fingerprint generation is only available when creating a browser profile")
		}
		if err := model.ValidateBrowserAgentProxyReferences(request.AgentId, request.ProxyId); err != nil {
			return nil, err
		}
	} else {
		if err := model.ValidateBrowserModelReferences(request.AgentId, request.ProxyId, request.FingerprintId); err != nil {
			return nil, err
		}
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
		ChannelId:     request.ChannelId,
		AgentId:       request.AgentId,
		ProxyId:       request.ProxyId,
		FingerprintId: request.FingerprintId,
		RuntimeKey:    strings.TrimSpace(request.RuntimeKey),
		Persistent:    persistent,
		Enabled:       enabled,
	}, nil
}

func browserProfileResponses(availableOnly bool, channelId int) ([]browserProfileResponse, error) {
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
	channels, err := model.GetChannelsByType(0, -1, true, constant.ChannelTypeCodex)
	if err != nil {
		return nil, err
	}
	launches, err := model.ListActiveBrowserLaunches(time.Now().Unix())
	if err != nil {
		return nil, err
	}

	agentMap := make(map[int]model.BrowserAgent, len(agents))
	for index := range agents {
		agentMap[agents[index].Id] = agents[index]
	}
	proxyMap := make(map[int]model.BrowserProxy, len(proxies))
	proxyIds := make([]int, 0, len(proxies))
	for index := range proxies {
		proxyMap[proxies[index].Id] = proxies[index]
		proxyIds = append(proxyIds, proxies[index].Id)
	}
	proxyChannelCounts, err := model.CountChannelsByBrowserProxies(proxyIds)
	if err != nil {
		return nil, err
	}
	proxyProfileCounts, err := model.CountBrowserProfilesByProxies(proxyIds)
	if err != nil {
		return nil, err
	}
	fingerprintMap := make(map[int]model.BrowserFingerprint, len(fingerprints))
	for index := range fingerprints {
		fingerprintMap[fingerprints[index].Id] = fingerprints[index]
	}
	channelMap := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		channelMap[channel.Id] = channel
	}
	launchMap := make(map[int]model.BrowserLaunch, len(launches))
	for index := range launches {
		launchMap[launches[index].ProfileId] = launches[index]
	}
	currentChannelProxyId := 0
	if channel := channelMap[channelId]; channel != nil {
		currentChannelProxyId = channel.GetSetting().BrowserProxyId
	}

	now := time.Now().Unix()
	responses := make([]browserProfileResponse, 0, len(profiles))
	for index := range profiles {
		profile := profiles[index]
		if availableOnly {
			if channelId == 0 && profile.ChannelId != nil {
				continue
			}
			if channelId > 0 && profile.ChannelId != nil && *profile.ChannelId != channelId {
				continue
			}
		}
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
		response := browserProfileResponse{
			BrowserProfile:           profile,
			AgentName:                agent.Name,
			AgentOnline:              agent.Enabled && agent.LastSeenAt >= now-45,
			AgentRuntimes:            runtimes,
			ProxyName:                proxy.Name,
			ProxyMaxChannelAccounts:  proxy.MaxChannelAccounts,
			ProxyChannelAccountCount: proxyChannelCounts[proxy.Id],
			ProxyProfileCount:        proxyProfileCounts[proxy.Id],
			ProxyAtCapacity:          proxy.MaxChannelAccounts > 0 && proxyChannelCounts[proxy.Id] >= int64(proxy.MaxChannelAccounts) && currentChannelProxyId != proxy.Id,
			FingerprintName:          fingerprint.Name,
		}
		if profile.ChannelId != nil {
			if channel := channelMap[*profile.ChannelId]; channel != nil {
				response.ChannelName = channel.Name
			}
		}
		if launch, found := launchMap[profile.Id]; found {
			response.ActiveLaunchId = launch.Id
			response.ActiveLaunchStatus = launch.Status
		}
		responses = append(responses, response)
	}
	sort.SliceStable(responses, func(left int, right int) bool {
		if responses[left].ProxyProfileCount != responses[right].ProxyProfileCount {
			return responses[left].ProxyProfileCount < responses[right].ProxyProfileCount
		}
		if responses[left].ProxyChannelAccountCount != responses[right].ProxyChannelAccountCount {
			return responses[left].ProxyChannelAccountCount < responses[right].ProxyChannelAccountCount
		}
		return responses[left].Id < responses[right].Id
	})
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
