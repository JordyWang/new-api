package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedBrowserFingerprintIsStableAndRegionIndependent(t *testing.T) {
	now := time.Now().Unix()
	first, err := NewGeneratedBrowserFingerprint("channel one", "stable-profile-key", now)
	require.NoError(t, err)
	second, err := NewGeneratedBrowserFingerprint("channel one", "stable-profile-key", now)
	require.NoError(t, err)
	different, err := NewGeneratedBrowserFingerprint("channel one", "different-profile-key", now)
	require.NoError(t, err)

	assert.Equal(t, first.Payload, second.Payload)
	assert.NotEqual(t, first.Payload, different.Payload)
	assert.Equal(t, generatedFingerprintUserAgent, first.UserAgent)
	assert.Equal(t, `[]`, first.LaunchArgs)

	var payload map[string]any
	require.NoError(t, common.UnmarshalJsonStr(first.Payload, &payload))
	for _, key := range []string{"country_code", "languages", "locale", "timezone", "timezone_id"} {
		assert.NotContains(t, payload, key)
	}
	fingerprint, ok := payload["fingerprint"].(map[string]any)
	require.True(t, ok)
	navigator, ok := fingerprint["navigator"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, navigator, "language")
	assert.NotContains(t, navigator, "languages")
}

func TestGeneratedBrowserFingerprintMatchesFormalChromium150Contract(t *testing.T) {
	fingerprint, err := NewGeneratedBrowserFingerprint("formal profile", "formal-profile-key", time.Now().Unix())
	require.NoError(t, err)

	var payload struct {
		ProfileId   string `json:"profile_id"`
		Fingerprint struct {
			UserAgent   string `json:"user_agent"`
			ClientHints struct {
				Platform        string   `json:"platform"`
				PlatformVersion string   `json:"platform_version"`
				Architecture    string   `json:"architecture"`
				Bitness         string   `json:"bitness"`
				FullVersion     string   `json:"full_version"`
				FormFactors     []string `json:"form_factors"`
				Mobile          bool     `json:"mobile"`
				Wow64           bool     `json:"wow64"`
			} `json:"client_hints"`
			Navigator struct {
				AppVersion string `json:"app_version"`
				Platform   string `json:"platform"`
				Vendor     string `json:"vendor"`
				Product    string `json:"product"`
			} `json:"navigator"`
			Screen struct {
				Width             int `json:"width"`
				Height            int `json:"height"`
				AvailableWidth    int `json:"avail_width"`
				AvailableHeight   int `json:"avail_height"`
				DeviceScaleFactor int `json:"device_scale_factor"`
			} `json:"screen"`
			Hardware struct {
				DeviceMemory        int `json:"device_memory"`
				HardwareConcurrency int `json:"hardware_concurrency"`
			} `json:"hardware"`
			Fonts struct {
				Families []string `json:"families"`
			} `json:"fonts"`
			WebGL struct {
				Vendor           string `json:"vendor"`
				Renderer         string `json:"renderer"`
				UnmaskedVendor   string `json:"unmasked_vendor"`
				UnmaskedRenderer string `json:"unmasked_renderer"`
			} `json:"webgl"`
			WebGPU struct {
				Vendor       string `json:"vendor"`
				Architecture string `json:"architecture"`
				Device       string `json:"device"`
				Description  string `json:"description"`
				Driver       string `json:"driver"`
			} `json:"webgpu"`
			WebRTC struct {
				IPHandlingPolicy string `json:"ip_handling_policy"`
			} `json:"webrtc"`
		} `json:"fingerprint"`
	}
	require.NoError(t, common.UnmarshalJsonStr(fingerprint.Payload, &payload))

	assert.Contains(t, payload.ProfileId, generatedFingerprintTemplate)
	assert.Equal(t, generatedFingerprintUserAgent, payload.Fingerprint.UserAgent)
	assert.Equal(t, "Linux", payload.Fingerprint.ClientHints.Platform)
	assert.Empty(t, payload.Fingerprint.ClientHints.PlatformVersion)
	assert.Equal(t, "x86", payload.Fingerprint.ClientHints.Architecture)
	assert.Equal(t, "64", payload.Fingerprint.ClientHints.Bitness)
	assert.Equal(t, "150.0.7871.46", payload.Fingerprint.ClientHints.FullVersion)
	assert.Equal(t, []string{"Desktop"}, payload.Fingerprint.ClientHints.FormFactors)
	assert.False(t, payload.Fingerprint.ClientHints.Mobile)
	assert.False(t, payload.Fingerprint.ClientHints.Wow64)
	assert.Equal(t, strings.TrimPrefix(generatedFingerprintUserAgent, "Mozilla/"), payload.Fingerprint.Navigator.AppVersion)
	assert.Equal(t, "Linux x86_64", payload.Fingerprint.Navigator.Platform)
	assert.Equal(t, "Google Inc.", payload.Fingerprint.Navigator.Vendor)
	assert.Equal(t, "Gecko", payload.Fingerprint.Navigator.Product)
	assert.Equal(t, 1920, payload.Fingerprint.Screen.Width)
	assert.Equal(t, 1080, payload.Fingerprint.Screen.Height)
	assert.Equal(t, 1920, payload.Fingerprint.Screen.AvailableWidth)
	assert.Equal(t, 1040, payload.Fingerprint.Screen.AvailableHeight)
	assert.Equal(t, 1, payload.Fingerprint.Screen.DeviceScaleFactor)
	assert.Equal(t, 1920, fingerprint.ViewportW)
	assert.Equal(t, 1080, fingerprint.ViewportH)
	assert.Equal(t, 8, payload.Fingerprint.Hardware.DeviceMemory)
	assert.Equal(t, 8, payload.Fingerprint.Hardware.HardwareConcurrency)
	assert.Equal(t, []string{
		"Arial", "Courier New", "Times New Roman", "Noto Sans", "Noto Serif", "Noto Color Emoji",
	}, payload.Fingerprint.Fonts.Families)
	assert.Equal(t, "WebKit", payload.Fingerprint.WebGL.Vendor)
	assert.Equal(t, "WebKit WebGL", payload.Fingerprint.WebGL.Renderer)
	assert.Equal(t, "Google Inc. (Intel)", payload.Fingerprint.WebGL.UnmaskedVendor)
	assert.Equal(t, "ANGLE (Intel, Mesa Intel(R) UHD Graphics 630, OpenGL 4.6)", payload.Fingerprint.WebGL.UnmaskedRenderer)
	assert.Equal(t, "intel", payload.Fingerprint.WebGPU.Vendor)
	assert.Equal(t, "gen-9", payload.Fingerprint.WebGPU.Architecture)
	assert.Equal(t, "0x3e92", payload.Fingerprint.WebGPU.Device)
	assert.Equal(t, "Intel(R) UHD Graphics 630", payload.Fingerprint.WebGPU.Description)
	assert.Equal(t, "Mesa Intel(R) UHD Graphics 630", payload.Fingerprint.WebGPU.Driver)
	assert.Equal(t, "disable_non_proxied_udp", payload.Fingerprint.WebRTC.IPHandlingPolicy)
}

func TestStartCodexBrowserOAuthWithGeneratedProfilePersistsOneFingerprint(t *testing.T) {
	database, agent, _, proxy, _ := useBrowserLaunchServiceTestDatabase(t)

	flow, err := StartCodexBrowserOAuthWithGeneratedProfile(
		42,
		"generated channel profile",
		agent.Id,
		proxy.Id,
		"chromium",
	)

	require.NoError(t, err)
	assert.Positive(t, flow.ProfileId)
	assert.Equal(t, proxy.Id, flow.ProxyId)
	profile, err := model.GetBrowserProfileById(flow.ProfileId)
	require.NoError(t, err)
	assert.Nil(t, profile.ChannelId)
	assert.True(t, profile.Persistent)
	assert.NotZero(t, profile.FingerprintId)

	fingerprint, err := model.GetBrowserFingerprintById(profile.FingerprintId)
	require.NoError(t, err)
	assert.Equal(t, generatedFingerprintUserAgent, fingerprint.UserAgent)
	assert.Contains(t, fingerprint.Name, "generated channel profile")

	var profileCount int64
	require.NoError(t, database.Model(&model.BrowserProfile{}).Count(&profileCount).Error)
	assert.EqualValues(t, 2, profileCount)
	var fingerprintCount int64
	require.NoError(t, database.Model(&model.BrowserFingerprint{}).Count(&fingerprintCount).Error)
	assert.EqualValues(t, 2, fingerprintCount)
}

func TestGeneratedFingerprintSurvivesProfileDataReset(t *testing.T) {
	_, agent, _, proxy, _ := useBrowserLaunchServiceTestDatabase(t)
	now := time.Now().Unix()
	profile, fingerprint, err := CreateGeneratedBrowserProfile(
		"stable browser identity",
		nil,
		agent.Id,
		proxy.Id,
		"chromium",
		true,
		true,
		now,
	)
	require.NoError(t, err)
	originalDataKey := profile.DataKey
	originalPayload := fingerprint.Payload

	require.NoError(t, model.RotateBrowserProfileDataKey(profile.Id, "rotated-browser-data-key", now+1))
	storedProfile, err := model.GetBrowserProfileById(profile.Id)
	require.NoError(t, err)
	storedFingerprint, err := model.GetBrowserFingerprintById(storedProfile.FingerprintId)
	require.NoError(t, err)

	assert.NotEqual(t, originalDataKey, storedProfile.DataKey)
	assert.Equal(t, "rotated-browser-data-key", storedProfile.DataKey)
	assert.Equal(t, fingerprint.Id, storedProfile.FingerprintId)
	assert.Equal(t, originalPayload, storedFingerprint.Payload)
}

func TestReadyBrowserRuntimesPreferAgentsWithFewerProfiles(t *testing.T) {
	database, existingAgent, _, _, _ := useBrowserLaunchServiceTestDatabase(t)
	lessUsedAgent := &model.BrowserAgent{
		Name:        "less used agent",
		TokenDigest: "less-used-agent-token",
		Enabled:     true,
		LastSeenAt:  time.Now().Unix(),
		Runtimes:    `["zeta","alpha"]`,
		Metadata:    existingAgent.Metadata,
	}
	require.NoError(t, database.Create(lessUsedAgent).Error)

	options, err := ListReadyBrowserRuntimeOptions()

	require.NoError(t, err)
	require.Len(t, options, 3)
	assert.Equal(t, lessUsedAgent.Id, options[0].AgentId)
	assert.Equal(t, "alpha", options[0].RuntimeKey)
	assert.Zero(t, options[0].ProfileCount)
	assert.Equal(t, lessUsedAgent.Id, options[1].AgentId)
	assert.Equal(t, "zeta", options[1].RuntimeKey)
	assert.Equal(t, existingAgent.Id, options[2].AgentId)
	assert.EqualValues(t, 1, options[2].ProfileCount)
}

func TestGeneratedOAuthProfileIsRemovedWhenAgentIsNotReady(t *testing.T) {
	database, agent, _, proxy, _ := useBrowserLaunchServiceTestDatabase(t)
	require.NoError(t, database.Model(&model.BrowserAgent{}).Where("id = ?", agent.Id).Update("last_seen_at", 0).Error)

	_, err := StartCodexBrowserOAuthWithGeneratedProfile(
		42,
		"failed generated profile",
		agent.Id,
		proxy.Id,
		"chromium",
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "offline")
	var profileCount int64
	require.NoError(t, database.Model(&model.BrowserProfile{}).Count(&profileCount).Error)
	assert.EqualValues(t, 1, profileCount)
	var fingerprintCount int64
	require.NoError(t, database.Model(&model.BrowserFingerprint{}).Count(&fingerprintCount).Error)
	assert.EqualValues(t, 1, fingerprintCount)
}
