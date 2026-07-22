package service

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"gorm.io/gorm"
)

const (
	codexBrowserOAuthFlowLifetime = 10 * time.Minute
	codexBrowserAgentLease        = 30 * time.Second
	browserAgentOnlineWindow      = 45 * time.Second
)

type CodexBrowserOAuthClaim = browseragentapi.CodexOAuthClaim
type CodexBrowserOAuthProfile = browseragentapi.CodexOAuthProfile
type CodexBrowserOAuthProxy = browseragentapi.CodexOAuthProxy
type CodexBrowserOAuthFingerprint = browseragentapi.CodexOAuthFingerprint

type CodexBrowserOAuthFlowView struct {
	Id                  string `json:"id"`
	Status              string `json:"status"`
	ChannelId           int    `json:"channel_id"`
	ProfileId           int    `json:"profile_id"`
	ProfileName         string `json:"profile_name"`
	AgentId             int    `json:"agent_id"`
	AgentName           string `json:"agent_name"`
	AgentOnline         bool   `json:"agent_online"`
	ProxyId             int    `json:"proxy_id"`
	AccountId           string `json:"account_id"`
	Email               string `json:"email"`
	PlanType            string `json:"plan_type"`
	CredentialExpiresAt int64  `json:"credential_expires_at"`
	ExpiresAt           int64  `json:"expires_at"`
	CreatedAt           int64  `json:"created_at"`
	CompletedAt         int64  `json:"completed_at"`
	Bound               bool   `json:"bound"`
	ErrorMessage        string `json:"error_message"`
}

type CodexBrowserOAuthCompletion struct {
	IDToken      string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func StartCodexBrowserOAuthFlow(userId int, channelId int, profileId int) (*CodexBrowserOAuthFlowView, error) {
	now := time.Now().Unix()
	profile, agent, _, _, err := loadReadyBrowserProfile(profileId, now)
	if err != nil {
		return nil, err
	}

	additionalProxyAccounts := int64(1)
	if channelId > 0 {
		channel, err := model.GetChannelById(channelId, false)
		if err != nil {
			return nil, err
		}
		if channel.Type != constant.ChannelTypeCodex {
			return nil, errors.New("channel type is not Codex")
		}
		boundProfile, err := model.GetBrowserProfileByChannelId(channelId)
		if err == nil && boundProfile.Id != profile.Id {
			return nil, model.ErrBrowserChannelBound
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if profile.ChannelId != nil && *profile.ChannelId != channelId {
			return nil, model.ErrBrowserProfileBound
		}
		channelProxyId := channel.GetSetting().BrowserProxyId
		if channelProxyId > 0 && channelProxyId != profile.ProxyId {
			return nil, errors.New("browser profile and channel must use the same managed proxy")
		}
		if channelProxyId == profile.ProxyId {
			additionalProxyAccounts = 0
		}
	} else if profile.ChannelId != nil {
		return nil, errors.New("browser profile is already bound; select its channel instead of creating a new one")
	}
	if additionalProxyAccounts > 0 {
		if err := model.CheckBrowserProxyChannelCapacity(profile.ProxyId, additionalProxyAccounts); err != nil {
			return nil, err
		}
	}

	authorization, err := CreateCodexOAuthAuthorizationFlow()
	if err != nil {
		return nil, err
	}
	flowId := common.GetUUID()
	stateCiphertext, err := common.EncryptSecret(authorization.State, codexOAuthFlowSecretContext(flowId, "state"))
	if err != nil {
		return nil, err
	}
	verifierCiphertext, err := common.EncryptSecret(authorization.Verifier, codexOAuthFlowSecretContext(flowId, "verifier"))
	if err != nil {
		return nil, err
	}

	flow := &model.CodexOAuthFlow{
		Id:                 flowId,
		UserId:             userId,
		ChannelId:          channelId,
		ProfileId:          profile.Id,
		AgentId:            profile.AgentId,
		ProxyId:            profile.ProxyId,
		FingerprintId:      profile.FingerprintId,
		Status:             model.CodexOAuthFlowStatusPending,
		StateDigest:        common.GenerateHMAC("codex-oauth-state:" + authorization.State),
		StateCiphertext:    stateCiphertext,
		VerifierCiphertext: verifierCiphertext,
		AuthorizeURL:       authorization.AuthorizeURL,
		ExpiresAt:          now + int64(codexBrowserOAuthFlowLifetime/time.Second),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := model.CreateExclusiveCodexOAuthFlow(flow); err != nil {
		return nil, err
	}
	return codexBrowserOAuthFlowView(flow, profile, agent, now), nil
}

func GetCodexBrowserOAuthFlow(userId int, isRoot bool, flowId string) (*CodexBrowserOAuthFlowView, error) {
	now := time.Now().Unix()
	flow, err := model.GetCodexOAuthFlowForUser(flowId, userId, isRoot, now)
	if err != nil {
		return nil, err
	}
	profile, err := model.GetBrowserProfileById(flow.ProfileId)
	if err != nil {
		return nil, err
	}
	agent, err := model.GetBrowserAgentById(flow.AgentId)
	if err != nil {
		return nil, err
	}
	return codexBrowserOAuthFlowView(flow, profile, agent, now), nil
}

func CancelCodexBrowserOAuthFlow(userId int, isRoot bool, flowId string) error {
	return model.CancelCodexOAuthFlow(flowId, userId, isRoot, time.Now().Unix())
}

func ClaimCodexBrowserOAuthFlow(agentId int, instanceId string) (*CodexBrowserOAuthClaim, error) {
	now := time.Now().Unix()
	flow, err := model.ClaimCodexOAuthFlow(
		agentId,
		instanceId,
		now,
		now+int64(codexBrowserAgentLease/time.Second),
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	agent, err := model.GetBrowserAgentById(agentId)
	if err != nil {
		_ = model.FailCodexOAuthFlow(agentId, instanceId, flow.Id, "failed to verify browser agent capabilities", now)
		return nil, err
	}
	if !browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityStrictProxyGeoV1) {
		err := errors.New("browser agent does not enforce strict proxy geography")
		_ = model.FailCodexOAuthFlow(agentId, instanceId, flow.Id, err.Error(), now)
		return nil, err
	}
	if !browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityProxyGeoOverlayV1) {
		err := errors.New("browser agent does not derive locale and timezone from proxy GeoIP")
		_ = model.FailCodexOAuthFlow(agentId, instanceId, flow.Id, err.Error(), now)
		return nil, err
	}
	claim, err := buildCodexBrowserOAuthClaim(flow)
	if err != nil {
		_ = model.FailCodexOAuthFlow(agentId, instanceId, flow.Id, "failed to load managed browser configuration", now)
		return nil, err
	}
	return claim, nil
}

func buildCodexBrowserOAuthClaim(flow *model.CodexOAuthFlow) (*CodexBrowserOAuthClaim, error) {
	profile, err := model.GetBrowserProfileById(flow.ProfileId)
	if err != nil {
		return nil, err
	}
	proxy, err := model.GetBrowserProxyById(flow.ProxyId)
	if err != nil {
		return nil, err
	}
	fingerprint, err := model.GetBrowserFingerprintById(flow.FingerprintId)
	if err != nil {
		return nil, err
	}
	state, err := common.DecryptSecret(flow.StateCiphertext, codexOAuthFlowSecretContext(flow.Id, "state"))
	if err != nil {
		return nil, err
	}
	verifier, err := common.DecryptSecret(flow.VerifierCiphertext, codexOAuthFlowSecretContext(flow.Id, "verifier"))
	if err != nil {
		return nil, err
	}
	proxyURL, err := DecryptBrowserProxyURL(proxy)
	if err != nil {
		return nil, err
	}

	return &CodexBrowserOAuthClaim{
		FlowId:       flow.Id,
		AuthorizeURL: flow.AuthorizeURL,
		State:        state,
		Verifier:     verifier,
		ExpiresAt:    flow.ExpiresAt,
		Profile: CodexBrowserOAuthProfile{
			Id:         profile.Id,
			Name:       profile.Name,
			RuntimeKey: profile.RuntimeKey,
			DataKey:    profile.DataKey,
			Persistent: profile.Persistent,
		},
		Proxy: CodexBrowserOAuthProxy{
			Id:  proxy.Id,
			URL: proxyURL,
		},
		Fingerprint: CodexBrowserOAuthFingerprint{
			Id:          fingerprint.Id,
			Name:        fingerprint.Name,
			UserAgent:   fingerprint.UserAgent,
			ViewportW:   fingerprint.ViewportW,
			ViewportH:   fingerprint.ViewportH,
			Payload:     fingerprint.Payload,
			LaunchArgs:  fingerprint.LaunchArgs,
			Environment: fingerprint.Environment,
		},
	}, nil
}

func MarkCodexBrowserOAuthFlowRunning(agentId int, instanceId string, flowId string) error {
	now := time.Now().Unix()
	return model.MarkCodexOAuthFlowRunning(
		agentId,
		instanceId,
		flowId,
		now,
		now+int64(codexBrowserAgentLease/time.Second),
	)
}

func CompleteCodexBrowserOAuthFlow(agentId int, instanceId string, flowId string, completion CodexBrowserOAuthCompletion) error {
	if strings.TrimSpace(completion.AccessToken) == "" || strings.TrimSpace(completion.RefreshToken) == "" {
		return errors.New("OAuth token response is incomplete")
	}
	if completion.ExpiresIn <= 0 || completion.ExpiresIn > int((30*24*time.Hour)/time.Second) {
		return errors.New("OAuth token expiry is invalid")
	}

	flow, err := model.GetCodexOAuthFlowById(flowId)
	if err != nil {
		return err
	}
	if flow.AgentId != agentId {
		return model.ErrCodexOAuthFlowInstance
	}

	accountId, ok := ExtractCodexAccountIDFromJWT(completion.AccessToken)
	if !ok && strings.TrimSpace(completion.IDToken) != "" {
		accountId, ok = ExtractCodexAccountIDFromJWT(completion.IDToken)
	}
	if !ok {
		return errors.New("failed to extract account_id from OAuth token")
	}

	emailToken := completion.IDToken
	if strings.TrimSpace(emailToken) == "" {
		emailToken = completion.AccessToken
	}
	email, _ := ExtractEmailFromJWT(emailToken)
	planType, _ := ExtractCodexPlanTypeFromJWT(completion.AccessToken)
	if planType == "" && strings.TrimSpace(completion.IDToken) != "" {
		planType, _ = ExtractCodexPlanTypeFromJWT(completion.IDToken)
	}

	now := time.Now()
	credentialExpiresAt := now.Add(time.Duration(completion.ExpiresIn) * time.Second)
	key := CodexOAuthKey{
		IDToken:        strings.TrimSpace(completion.IDToken),
		AccessToken:    strings.TrimSpace(completion.AccessToken),
		RefreshToken:   strings.TrimSpace(completion.RefreshToken),
		AccountID:      accountId,
		LastRefresh:    now.Format(time.RFC3339),
		Email:          email,
		PlanType:       planType,
		Type:           "codex",
		Expired:        credentialExpiresAt.Format(time.RFC3339),
		ManagedProxyID: flow.ProxyId,
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		return err
	}
	credentialsCiphertext, err := common.EncryptSecret(string(encoded), codexOAuthFlowSecretContext(flow.Id, "credentials"))
	if err != nil {
		return err
	}

	err = model.CompleteCodexOAuthFlow(agentId, instanceId, flowId, model.CodexOAuthFlowCompletion{
		CredentialsCiphertext: credentialsCiphertext,
		ChannelKey:            string(encoded),
		AccountId:             accountId,
		Email:                 email,
		PlanType:              planType,
		CredentialExpiresAt:   credentialExpiresAt.Unix(),
		CompletedAt:           now.Unix(),
	})
	if err != nil {
		return err
	}
	if flow.ChannelId > 0 {
		model.InitChannelCache()
		ResetProxyClientCache()
	}
	return nil
}

func FailCodexBrowserOAuthFlow(agentId int, instanceId string, flowId string, message string) error {
	trimmedMessage := strings.TrimSpace(message)
	messageRunes := []rune(trimmedMessage)
	if len(messageRunes) > 500 {
		trimmedMessage = string(messageRunes[:500])
	}
	if trimmedMessage == "" {
		trimmedMessage = "browser OAuth flow failed"
	}
	return model.FailCodexOAuthFlow(agentId, instanceId, flowId, trimmedMessage, time.Now().Unix())
}

func GetAgentCodexBrowserOAuthFlowStatus(agentId int, instanceId string, flowId string) (string, error) {
	flow, err := model.GetCodexOAuthFlowById(flowId)
	if err != nil {
		return "", err
	}
	if flow.AgentId != agentId || flow.AgentInstanceId != instanceId {
		return "", model.ErrCodexOAuthFlowInstance
	}
	return flow.Status, nil
}

func PrepareCompletedCodexBrowserOAuthFlow(userId int, flowId string) (string, int, error) {
	flow, err := model.GetCodexOAuthFlowForUser(flowId, userId, false, time.Now().Unix())
	if err != nil {
		return "", 0, err
	}
	if flow.Status != model.CodexOAuthFlowStatusCompleted || flow.ConsumedAt != 0 || strings.TrimSpace(flow.CredentialsCiphertext) == "" {
		return "", 0, model.ErrCodexOAuthFlowState
	}
	credentials, err := common.DecryptSecret(flow.CredentialsCiphertext, codexOAuthFlowSecretContext(flow.Id, "credentials"))
	if err != nil {
		return "", 0, err
	}
	return credentials, flow.ProxyId, nil
}

func ApplyManagedBrowserProxyToChannel(channel *model.Channel, proxyId int) error {
	if channel == nil {
		return errors.New("channel is nil")
	}
	setting := channel.GetSetting()
	setting.BrowserProxyId = proxyId
	setting.Proxy = ""
	encoded, err := common.Marshal(setting)
	if err != nil {
		return err
	}
	value := string(encoded)
	channel.Setting = &value
	return nil
}

func codexBrowserOAuthFlowView(flow *model.CodexOAuthFlow, profile *model.BrowserProfile, agent *model.BrowserAgent, now int64) *CodexBrowserOAuthFlowView {
	return &CodexBrowserOAuthFlowView{
		Id:                  flow.Id,
		Status:              flow.Status,
		ChannelId:           flow.ChannelId,
		ProfileId:           flow.ProfileId,
		ProfileName:         profile.Name,
		AgentId:             flow.AgentId,
		AgentName:           agent.Name,
		AgentOnline:         agent.Enabled && agent.LastSeenAt >= now-int64(browserAgentOnlineWindow/time.Second),
		ProxyId:             flow.ProxyId,
		AccountId:           flow.AccountId,
		Email:               flow.Email,
		PlanType:            flow.PlanType,
		CredentialExpiresAt: flow.CredentialExpiresAt,
		ExpiresAt:           flow.ExpiresAt,
		CreatedAt:           flow.CreatedAt,
		CompletedAt:         flow.CompletedAt,
		Bound:               flow.ConsumedAt > 0,
		ErrorMessage:        flow.ErrorMessage,
	}
}

func codexOAuthFlowSecretContext(flowId string, field string) string {
	return "codex-oauth-flow:" + flowId + ":" + field
}

func browserAgentHasCapability(metadataJSON string, capability string) bool {
	var metadata struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := common.UnmarshalJsonStr(metadataJSON, &metadata); err != nil {
		return false
	}
	for _, advertised := range metadata.Capabilities {
		if advertised == capability {
			return true
		}
	}
	return false
}
