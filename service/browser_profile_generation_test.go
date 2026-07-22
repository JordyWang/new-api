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
	assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36", first.UserAgent)
	assert.NotContains(t, first.UserAgent, "Linux")
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

func TestGeneratedBrowserFingerprintMatchesManagedChromium150Contract(t *testing.T) {
	type brandVersion struct {
		Brand   string `json:"brand"`
		Version string `json:"version"`
	}
	testCases := []struct {
		name              string
		dataKey           string
		profilePlatform   string
		userAgent         string
		clientPlatform    string
		clientPlatformVer string
		navigatorPlatform string
	}{
		{
			name:              "Windows 11",
			dataKey:           "windows-profile-key",
			profilePlatform:   "windows-x86_64",
			userAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
			clientPlatform:    "Windows",
			clientPlatformVer: "19.0.0",
			navigatorPlatform: "Win32",
		},
		{
			name:              "macOS",
			dataKey:           "macos-profile-key",
			profilePlatform:   "macos-x86_64",
			userAgent:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
			clientPlatform:    "macOS",
			clientPlatformVer: "15.7.6",
			navigatorPlatform: "MacIntel",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fingerprint, err := NewGeneratedBrowserFingerprint("managed profile", testCase.dataKey, time.Now().Unix())
			require.NoError(t, err)

			var payload struct {
				ProfileId   string `json:"profile_id"`
				Fingerprint struct {
					UserAgent   string `json:"user_agent"`
					ClientHints struct {
						Platform             string         `json:"platform"`
						PlatformVersion      string         `json:"platform_version"`
						Architecture         string         `json:"architecture"`
						Bitness              string         `json:"bitness"`
						FullVersion          string         `json:"full_version"`
						FormFactors          []string       `json:"form_factors"`
						Mobile               bool           `json:"mobile"`
						Wow64                bool           `json:"wow64"`
						BrandVersionList     []brandVersion `json:"brand_version_list"`
						BrandFullVersionList []brandVersion `json:"brand_full_version_list"`
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
					WebRTC struct {
						IPHandlingPolicy string `json:"ip_handling_policy"`
					} `json:"webrtc"`
				} `json:"fingerprint"`
			}
			require.NoError(t, common.UnmarshalJsonStr(fingerprint.Payload, &payload))

			assert.Regexp(t, `^formal-150-desktop-20260722-v4-`+testCase.profilePlatform+`-[0-9a-f]{16}$`, payload.ProfileId)
			assert.Equal(t, testCase.userAgent, fingerprint.UserAgent)
			assert.Equal(t, testCase.userAgent, payload.Fingerprint.UserAgent)
			assert.NotContains(t, payload.Fingerprint.UserAgent, "Linux")
			assert.Equal(t, testCase.clientPlatform, payload.Fingerprint.ClientHints.Platform)
			assert.Equal(t, testCase.clientPlatformVer, payload.Fingerprint.ClientHints.PlatformVersion)
			assert.Equal(t, "x86", payload.Fingerprint.ClientHints.Architecture)
			assert.Equal(t, "64", payload.Fingerprint.ClientHints.Bitness)
			assert.Equal(t, "150.0.7871.46", payload.Fingerprint.ClientHints.FullVersion)
			assert.Equal(t, []string{"Desktop"}, payload.Fingerprint.ClientHints.FormFactors)
			assert.False(t, payload.Fingerprint.ClientHints.Mobile)
			assert.False(t, payload.Fingerprint.ClientHints.Wow64)
			assert.Equal(t, []brandVersion{
				{Brand: "Not;A=Brand", Version: "8"},
				{Brand: "Chromium", Version: "150"},
			}, payload.Fingerprint.ClientHints.BrandVersionList)
			assert.Equal(t, []brandVersion{
				{Brand: "Not;A=Brand", Version: "8.0.0.0"},
				{Brand: "Chromium", Version: "150.0.7871.46"},
			}, payload.Fingerprint.ClientHints.BrandFullVersionList)
			assert.Equal(t, strings.TrimPrefix(testCase.userAgent, "Mozilla/"), payload.Fingerprint.Navigator.AppVersion)
			assert.Equal(t, testCase.navigatorPlatform, payload.Fingerprint.Navigator.Platform)
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
			assert.Equal(t, "disable_non_proxied_udp", payload.Fingerprint.WebRTC.IPHandlingPolicy)

			var untypedPayload map[string]any
			require.NoError(t, common.UnmarshalJsonStr(fingerprint.Payload, &untypedPayload))
			fingerprintPayload, ok := untypedPayload["fingerprint"].(map[string]any)
			require.True(t, ok)
			for _, browserNativeField := range []string{
				"audio", "battery", "canvas", "fonts", "media_devices", "network_information", "storage", "webgpu",
			} {
				assert.NotContains(t, fingerprintPayload, browserNativeField)
			}
			webgl, ok := fingerprintPayload["webgl"].(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, webgl["noise_seed"])
			for _, nativeGPUField := range []string{"extensions", "renderer", "unmasked_renderer", "unmasked_vendor", "vendor"} {
				assert.NotContains(t, webgl, nativeGPUField)
			}
			navigator, ok := fingerprintPayload["navigator"].(map[string]any)
			require.True(t, ok)
			for _, dynamicNavigatorField := range []string{"cookie_enabled", "do_not_track", "online"} {
				assert.NotContains(t, navigator, dynamicNavigatorField)
			}
		})
	}
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
	assert.NotEmpty(t, fingerprint.UserAgent)
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
