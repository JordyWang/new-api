package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"gorm.io/gorm"
)

const (
	standaloneBrowserLaunchLifetime = 12 * time.Hour
	standaloneBrowserStartURL       = "https://chatgpt.com/"
)

type BrowserLaunchView struct {
	Id           string `json:"id"`
	Status       string `json:"status"`
	ProfileId    int    `json:"profile_id"`
	ProfileName  string `json:"profile_name"`
	ChannelId    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	AgentId      int    `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	AgentOnline  bool   `json:"agent_online"`
	ExpiresAt    int64  `json:"expires_at"`
	CreatedAt    int64  `json:"created_at"`
	RunningAt    int64  `json:"running_at"`
	CompletedAt  int64  `json:"completed_at"`
	ErrorMessage string `json:"error_message"`
}

func loadReadyBrowserProfile(profileId int, now int64) (*model.BrowserProfile, *model.BrowserAgent, *model.BrowserProxy, *model.BrowserFingerprint, error) {
	profile, err := model.GetBrowserProfileById(profileId)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !profile.Enabled {
		return nil, nil, nil, nil, errors.New("browser profile is disabled")
	}
	agent, err := model.GetBrowserAgentById(profile.AgentId)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !agent.Enabled {
		return nil, nil, nil, nil, errors.New("browser agent is disabled")
	}
	if !browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityStrictProxyGeoV1) {
		return nil, nil, nil, nil, errors.New("browser agent does not enforce strict proxy geography; upgrade and restart the agent")
	}
	if !browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityProxyGeoOverlayV1) {
		return nil, nil, nil, nil, errors.New("browser agent does not derive locale and timezone from proxy GeoIP; upgrade and restart the agent")
	}
	if agent.LastSeenAt < now-int64(browserAgentOnlineWindow/time.Second) {
		return nil, nil, nil, nil, errors.New("browser agent is offline")
	}
	var runtimes []string
	if err := common.UnmarshalJsonStr(agent.Runtimes, &runtimes); err != nil {
		return nil, nil, nil, nil, errors.New("browser agent runtime advertisement is invalid")
	}
	runtimeAvailable := false
	for _, runtimeKey := range runtimes {
		if runtimeKey == profile.RuntimeKey {
			runtimeAvailable = true
			break
		}
	}
	if !runtimeAvailable {
		return nil, nil, nil, nil, fmt.Errorf("browser runtime %q is not available on the selected agent", profile.RuntimeKey)
	}
	proxy, err := model.GetBrowserProxyById(profile.ProxyId)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !proxy.Enabled {
		return nil, nil, nil, nil, errors.New("browser proxy is disabled")
	}
	fingerprint, err := model.GetBrowserFingerprintById(profile.FingerprintId)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !fingerprint.Enabled {
		return nil, nil, nil, nil, errors.New("browser fingerprint is disabled")
	}
	return profile, agent, proxy, fingerprint, nil
}

func StartBrowserProfileLaunch(profileId int) (*BrowserLaunchView, error) {
	now := time.Now().Unix()
	profile, agent, _, _, err := loadReadyBrowserProfile(profileId, now)
	if err != nil {
		return nil, err
	}
	var channel *model.Channel
	channelId := 0
	if profile.ChannelId != nil && *profile.ChannelId > 0 {
		channel, err = model.GetChannelById(*profile.ChannelId, false)
		if err != nil {
			return nil, err
		}
		if channel.Type != constant.ChannelTypeCodex {
			return nil, errors.New("browser profile channel is not Codex")
		}
		if channel.GetSetting().BrowserProxyId != profile.ProxyId {
			return nil, errors.New("browser profile and channel must use the same managed proxy")
		}
		channelId = channel.Id
	}

	launch := &model.BrowserLaunch{
		Id:            common.GetUUID(),
		ProfileId:     profile.Id,
		ChannelId:     channelId,
		AgentId:       profile.AgentId,
		ProxyId:       profile.ProxyId,
		FingerprintId: profile.FingerprintId,
		Status:        browseragentapi.FlowStatusPending,
		StartURL:      standaloneBrowserStartURL,
		ExpiresAt:     now + int64(standaloneBrowserLaunchLifetime/time.Second),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := model.CreateExclusiveBrowserLaunch(launch); err != nil {
		return nil, err
	}
	return browserLaunchView(launch, profile, channel, agent, now), nil
}

func GetBrowserProfileLaunch(launchId string) (*BrowserLaunchView, error) {
	now := time.Now().Unix()
	launch, err := model.GetBrowserLaunchById(strings.TrimSpace(launchId), now)
	if err != nil {
		return nil, err
	}
	profile, err := model.GetBrowserProfileById(launch.ProfileId)
	if err != nil {
		return nil, err
	}
	var channel *model.Channel
	if launch.ChannelId > 0 {
		channel, err = model.GetChannelById(launch.ChannelId, false)
		if err != nil {
			return nil, err
		}
	}
	agent, err := model.GetBrowserAgentById(launch.AgentId)
	if err != nil {
		return nil, err
	}
	return browserLaunchView(launch, profile, channel, agent, now), nil
}

func CancelBrowserProfileLaunch(launchId string) error {
	return model.CancelBrowserLaunch(strings.TrimSpace(launchId), time.Now().Unix())
}

func ClaimBrowserProfileLaunch(agentId int, instanceId string) (*browseragentapi.BrowserLaunchClaim, error) {
	now := time.Now().Unix()
	launch, err := model.ClaimBrowserLaunch(agentId, instanceId, now, now+int64(codexBrowserAgentLease/time.Second))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	claim, err := buildBrowserLaunchClaim(launch)
	if err != nil {
		_ = model.FailBrowserLaunch(agentId, instanceId, launch.Id, "failed to load managed browser configuration", now)
		return nil, err
	}
	return claim, nil
}

func buildBrowserLaunchClaim(launch *model.BrowserLaunch) (*browseragentapi.BrowserLaunchClaim, error) {
	profile, err := model.GetBrowserProfileById(launch.ProfileId)
	if err != nil {
		return nil, err
	}
	proxy, err := model.GetBrowserProxyById(launch.ProxyId)
	if err != nil {
		return nil, err
	}
	fingerprint, err := model.GetBrowserFingerprintById(launch.FingerprintId)
	if err != nil {
		return nil, err
	}
	proxyURL, err := DecryptBrowserProxyURL(proxy)
	if err != nil {
		return nil, err
	}
	return &browseragentapi.BrowserLaunchClaim{
		LaunchId:  launch.Id,
		StartURL:  launch.StartURL,
		ExpiresAt: launch.ExpiresAt,
		Profile: browseragentapi.CodexOAuthProfile{
			Id:         profile.Id,
			Name:       profile.Name,
			RuntimeKey: profile.RuntimeKey,
			DataKey:    profile.DataKey,
			Persistent: profile.Persistent,
		},
		Proxy: browseragentapi.CodexOAuthProxy{
			Id:  proxy.Id,
			URL: proxyURL,
		},
		Fingerprint: browseragentapi.CodexOAuthFingerprint{
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

func MarkBrowserProfileLaunchRunning(agentId int, instanceId string, launchId string) error {
	now := time.Now().Unix()
	return model.MarkBrowserLaunchRunning(agentId, instanceId, launchId, now, now+int64(codexBrowserAgentLease/time.Second))
}

func CompleteBrowserProfileLaunch(agentId int, instanceId string, launchId string) error {
	return model.CompleteBrowserLaunch(agentId, instanceId, launchId, time.Now().Unix())
}

func FailBrowserProfileLaunch(agentId int, instanceId string, launchId string, message string) error {
	trimmedMessage := strings.TrimSpace(message)
	messageRunes := []rune(trimmedMessage)
	if len(messageRunes) > 500 {
		trimmedMessage = string(messageRunes[:500])
	}
	if trimmedMessage == "" {
		trimmedMessage = "browser launch failed"
	}
	return model.FailBrowserLaunch(agentId, instanceId, launchId, trimmedMessage, time.Now().Unix())
}

func GetAgentBrowserProfileLaunchStatus(agentId int, instanceId string, launchId string) (string, error) {
	launch, err := model.GetBrowserLaunchById(launchId, time.Now().Unix())
	if err != nil {
		return "", err
	}
	if launch.AgentId != agentId || launch.AgentInstanceId != instanceId {
		return "", model.ErrBrowserLaunchInstance
	}
	return launch.Status, nil
}

func browserLaunchView(launch *model.BrowserLaunch, profile *model.BrowserProfile, channel *model.Channel, agent *model.BrowserAgent, now int64) *BrowserLaunchView {
	view := &BrowserLaunchView{
		Id:           launch.Id,
		Status:       launch.Status,
		ProfileId:    launch.ProfileId,
		ProfileName:  profile.Name,
		ChannelId:    launch.ChannelId,
		AgentId:      launch.AgentId,
		AgentName:    agent.Name,
		AgentOnline:  agent.Enabled && agent.LastSeenAt >= now-int64(browserAgentOnlineWindow/time.Second),
		ExpiresAt:    launch.ExpiresAt,
		CreatedAt:    launch.CreatedAt,
		RunningAt:    launch.RunningAt,
		CompletedAt:  launch.CompletedAt,
		ErrorMessage: launch.ErrorMessage,
	}
	if channel != nil {
		view.ChannelName = channel.Name
	}
	return view
}
