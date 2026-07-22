package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"
)

const (
	generatedFingerprintTemplate  = "formal-150-linux-x86_64-20260718-v1"
	generatedFingerprintUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.7871.46 Safari/537.36"
)

type BrowserRuntimeOption struct {
	AgentId      int    `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	RuntimeKey   string `json:"runtime_key"`
	ProfileCount int64  `json:"profile_count"`
}

func ListReadyBrowserRuntimeOptions() ([]BrowserRuntimeOption, error) {
	agents, err := model.ListBrowserAgents()
	if err != nil {
		return nil, err
	}
	agentIds := make([]int, 0, len(agents))
	for index := range agents {
		agentIds = append(agentIds, agents[index].Id)
	}
	profileCounts, err := model.CountBrowserProfilesByAgents(agentIds)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	options := make([]BrowserRuntimeOption, 0)
	for index := range agents {
		agent := agents[index]
		if !agent.Enabled || agent.LastSeenAt < now-int64(browserAgentOnlineWindow/time.Second) {
			continue
		}
		if !browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityStrictProxyGeoV1) ||
			!browserAgentHasCapability(agent.Metadata, browseragentapi.CapabilityProxyGeoOverlayV1) {
			continue
		}
		var runtimes []string
		if err := common.UnmarshalJsonStr(agent.Runtimes, &runtimes); err != nil {
			continue
		}
		sort.Strings(runtimes)
		previous := ""
		for _, runtimeKey := range runtimes {
			runtimeKey = strings.TrimSpace(runtimeKey)
			if runtimeKey == "" || runtimeKey == previous {
				continue
			}
			previous = runtimeKey
			options = append(options, BrowserRuntimeOption{
				AgentId:      agent.Id,
				AgentName:    agent.Name,
				RuntimeKey:   runtimeKey,
				ProfileCount: profileCounts[agent.Id],
			})
		}
	}
	sort.SliceStable(options, func(left int, right int) bool {
		if options[left].ProfileCount != options[right].ProfileCount {
			return options[left].ProfileCount < options[right].ProfileCount
		}
		if options[left].AgentId != options[right].AgentId {
			return options[left].AgentId < options[right].AgentId
		}
		return options[left].RuntimeKey < options[right].RuntimeKey
	})
	return options, nil
}

func NewGeneratedBrowserFingerprint(profileName string, dataKey string, now int64) (*model.BrowserFingerprint, error) {
	if strings.TrimSpace(dataKey) == "" {
		return nil, errors.New("browser profile data key is required for fingerprint generation")
	}
	profileSeed := common.GenerateHMACWithKey([]byte(dataKey), generatedFingerprintTemplate)
	canvasSeed := common.GenerateHMACWithKey([]byte(dataKey), "canvas")
	webglSeed := common.GenerateHMACWithKey([]byte(dataKey), "webgl")
	audioSeed := common.GenerateHMACWithKey([]byte(dataKey), "audio")
	mediaSeed := common.GenerateHMACWithKey([]byte(dataKey), "media")
	profileCode := profileSeed[:16]

	payload := map[string]any{
		"profile_id": generatedFingerprintTemplate + "-" + profileCode,
		"fingerprint": map[string]any{
			"audio": map[string]any{"noise_seed": audioSeed},
			"fonts": map[string]any{"families": []string{
				"Arial", "Courier New", "Times New Roman", "Noto Sans", "Noto Serif", "Noto Color Emoji",
			}},
			"webgl": map[string]any{
				"vendor":            "WebKit",
				"renderer":          "WebKit WebGL",
				"extensions":        []string{"EXT_color_buffer_float", "EXT_texture_filter_anisotropic", "OES_texture_float", "OES_texture_float_linear", "WEBGL_debug_renderer_info", "WEBGL_lose_context"},
				"noise_seed":        webglSeed,
				"unmasked_vendor":   "Google Inc. (Intel)",
				"unmasked_renderer": "ANGLE (Intel, Mesa Intel(R) UHD Graphics 630, OpenGL 4.6)",
			},
			"canvas": map[string]any{"noise_seed": canvasSeed},
			"screen": map[string]any{
				"width": 1920, "height": 1080, "avail_top": 0, "avail_left": 0,
				"avail_width": 1920, "avail_height": 1040, "color_depth": 24,
				"pixel_depth": 24, "device_scale_factor": 1,
			},
			"webgpu": map[string]any{
				"device": "0x3e92", "driver": "Mesa Intel(R) UHD Graphics 630",
				"vendor": "intel", "description": "Intel(R) UHD Graphics 630", "architecture": "gen-9",
			},
			"webrtc": map[string]any{"ip_handling_policy": "disable_non_proxied_udp"},
			"battery": map[string]any{
				"level": 1, "charging": true, "charging_time": 0, "discharging_time": 36000,
			},
			"plugins":  map[string]any{"pdf_viewer_enabled": true},
			"storage":  map[string]any{"quota": 21474836480, "usage": 104857600, "persisted": false},
			"hardware": map[string]any{"device_memory": 8, "hardware_concurrency": 8},
			"navigator": map[string]any{
				"online": true, "vendor": "Google Inc.", "product": "Gecko", "app_name": "Netscape",
				"platform": "Linux x86_64", "vendor_sub": "", "app_version": strings.TrimPrefix(generatedFingerprintUserAgent, "Mozilla/"),
				"product_sub": "20030107", "do_not_track": "unspecified", "app_code_name": "Mozilla",
				"cookie_enabled": true, "max_touch_points": 0,
			},
			"automation": map[string]any{"webdriver": false},
			"user_agent": generatedFingerprintUserAgent,
			"client_hints": map[string]any{
				"wow64": false, "mobile": false, "bitness": "64", "platform": "Linux", "architecture": "x86",
				"form_factors": []string{"Desktop"}, "full_version": "150.0.7871.46", "platform_version": "",
				"brand_version_list":      []map[string]string{{"brand": "Chromium", "version": "150"}},
				"brand_full_version_list": []map[string]string{{"brand": "Chromium", "version": "150.0.7871.46"}},
			},
			"media_devices": []map[string]string{
				{"kind": "audioinput", "label": "Built-in Microphone", "group_id": mediaSeed + "-audio", "device_id": mediaSeed + "-audio-input"},
				{"kind": "videoinput", "label": "HD Camera", "group_id": mediaSeed + "-video", "device_id": mediaSeed + "-video-input"},
				{"kind": "audiooutput", "label": "Built-in Speakers", "group_id": mediaSeed + "-audio", "device_id": mediaSeed + "-audio-output"},
			},
			"network_information": map[string]any{
				"rtt": 50, "type": "wifi", "downlink": 10, "save_data": false, "downlink_max": 100, "effective_type": "4g",
			},
		},
	}
	encodedPayload, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	fingerprintName := "Auto · " + strings.TrimSpace(profileName)
	if fingerprintName == "Auto · " {
		fingerprintName = "Auto · Browser profile"
	}
	for len(fingerprintName) > 128 {
		runes := []rune(fingerprintName)
		fingerprintName = string(runes[:len(runes)-1])
	}
	return &model.BrowserFingerprint{
		Name:        fingerprintName,
		UserAgent:   generatedFingerprintUserAgent,
		ViewportW:   1920,
		ViewportH:   1080,
		Payload:     string(encodedPayload),
		LaunchArgs:  `[]`,
		Environment: `{}`,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func CreateGeneratedBrowserProfile(name string, channelId *int, agentId int, proxyId int, runtimeKey string, persistent bool, enabled bool, now int64) (*model.BrowserProfile, *model.BrowserFingerprint, error) {
	if err := model.ValidateBrowserAgentProxyReferences(agentId, proxyId); err != nil {
		return nil, nil, err
	}
	agent, err := model.GetBrowserAgentById(agentId)
	if err != nil {
		return nil, nil, err
	}
	var runtimes []string
	if err := common.UnmarshalJsonStr(agent.Runtimes, &runtimes); err != nil {
		return nil, nil, errors.New("browser agent runtime advertisement is invalid")
	}
	runtimeKey = strings.TrimSpace(runtimeKey)
	runtimeAvailable := false
	for _, advertisedRuntime := range runtimes {
		if advertisedRuntime == runtimeKey {
			runtimeAvailable = true
			break
		}
	}
	if !runtimeAvailable {
		return nil, nil, fmt.Errorf("browser runtime %q is not available on the selected agent", runtimeKey)
	}

	dataKey, err := common.GenerateRandomCharsKey(40)
	if err != nil {
		return nil, nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Automatic browser profile"
	}
	for len(name) > 128 {
		runes := []rune(name)
		name = string(runes[:len(runes)-1])
	}
	fingerprint, err := NewGeneratedBrowserFingerprint(name, dataKey, now)
	if err != nil {
		return nil, nil, err
	}
	profile := &model.BrowserProfile{
		Name:          name,
		ChannelId:     channelId,
		AgentId:       agentId,
		ProxyId:       proxyId,
		FingerprintId: 0,
		RuntimeKey:    runtimeKey,
		DataKey:       dataKey,
		Persistent:    persistent,
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := model.CreateBrowserProfileWithFingerprint(profile, fingerprint); err != nil {
		return nil, nil, err
	}
	return profile, fingerprint, nil
}

func StartCodexBrowserOAuthWithGeneratedProfile(userId int, profileName string, agentId int, proxyId int, runtimeKey string) (*CodexBrowserOAuthFlowView, error) {
	now := time.Now().Unix()
	profile, fingerprint, err := CreateGeneratedBrowserProfile(profileName, nil, agentId, proxyId, runtimeKey, true, true, now)
	if err != nil {
		return nil, err
	}
	flow, err := StartCodexBrowserOAuthFlow(userId, 0, profile.Id)
	if err == nil {
		return flow, nil
	}
	_ = model.DeleteBrowserProfile(profile.Id)
	_ = model.DeleteBrowserFingerprint(fingerprint.Id)
	return nil, err
}
