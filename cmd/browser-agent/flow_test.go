package main

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRejectsServerControlledNetworkAndProfileOverrides(t *testing.T) {
	for _, argument := range []string{
		"--proxy-server=http://other-proxy.example:8080",
		"--proxy-pac-url=https://example.com/proxy.pac",
		"--proxy-bypass-list=*",
		"--user-data-dir=/tmp/unmanaged",
		"--user-agent=unmanaged",
		"--lang=fr-FR",
		"--window-size=1,1",
		"--force-time-zone-for-testing=UTC",
		"--force-webrtc-ip-handling-policy=default",
		"--enable-quic",
		"--remote-debugging-port=9222",
	} {
		assert.True(t, forbiddenBrowserArgument(argument), argument)
	}
	assert.False(t, forbiddenBrowserArgument("--fingerprint-config={fingerprint_file}"))
}

func TestAgentRejectsDangerousBrowserEnvironmentVariables(t *testing.T) {
	for _, key := range []string{
		"LD_PRELOAD", "PATH", "HOME", "DYLD_INSERT_LIBRARIES", "NODE_OPTIONS",
		"TZ", "LANG", "LANGUAGE", "LC_ALL", "LC_TIME", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
	} {
		assert.True(t, forbiddenBrowserEnvironmentKey(key), key)
	}
	assert.False(t, forbiddenBrowserEnvironmentKey("FINGERPRINT_PROFILE"))
}

func TestManagedBrowserUsesProxyGeoOverlay(t *testing.T) {
	claim := &browseragentapi.CodexOAuthClaim{
		AuthorizeURL: "https://auth.openai.com/oauth/authorize?state=test",
		Fingerprint: browseragentapi.CodexOAuthFingerprint{
			UserAgent:   "managed-agent",
			ViewportW:   1280,
			ViewportH:   800,
			LaunchArgs:  `[]`,
			Environment: `{"FINGERPRINT_PROFILE":"managed"}`,
		},
	}
	geo := &proxyGeoIdentity{
		CountryCode:    "SG",
		Locale:         "en-SG",
		Timezone:       "Asia/Singapore",
		Languages:      []string{"en-SG", "en", "cmn"},
		AcceptLanguage: "en-SG,en,cmn",
	}

	preflightURL := "http://127.0.0.1:1455/browser/preflight?token=test"
	args, err := buildBrowserLaunchArgs("/profiles/test", "/profiles/test/fingerprint.json", "http://127.0.0.1:3000", preflightURL, claim, geo)
	require.NoError(t, err)
	assert.Contains(t, args, "--proxy-server=http://127.0.0.1:3000")
	assert.Contains(t, args, "--proxy-bypass-list=<-loopback>;localhost;127.0.0.1;[::1]")
	assert.Contains(t, args, "--lang=en-SG")
	assert.Contains(t, args, "--force-time-zone-for-testing=Asia/Singapore")
	assert.Contains(t, args, "--force-webrtc-ip-handling-policy=disable_non_proxied_udp")
	assert.Contains(t, args, "--disable-quic")
	assert.Equal(t, preflightURL, args[len(args)-1])
	assert.NotContains(t, args, claim.AuthorizeURL)

	t.Setenv("TZ", "UTC")
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("HTTP_PROXY", "http://unmanaged.example:8080")
	environment, err := buildBrowserEnvironment("/profiles/test", "/profiles/test/fingerprint.json", "http://127.0.0.1:3000", preflightURL, claim, geo)
	require.NoError(t, err)
	environmentValues := make(map[string]string, len(environment))
	for _, item := range environment {
		key, value, found := strings.Cut(item, "=")
		if found {
			environmentValues[strings.ToUpper(key)] = value
		}
	}
	assert.Equal(t, "Asia/Singapore", environmentValues["TZ"])
	assert.Equal(t, "en_SG.UTF-8", environmentValues["LANG"])
	assert.Equal(t, "en-SG", environmentValues["LANGUAGE"])
	assert.Equal(t, "http://127.0.0.1:3000", environmentValues["HTTP_PROXY"])
	assert.Equal(t, "http://127.0.0.1:3000", environmentValues["HTTPS_PROXY"])
	assert.Equal(t, "localhost,127.0.0.1,::1", environmentValues["NO_PROXY"])
}

func TestStandaloneBrowserClaimValidation(t *testing.T) {
	runtimes := map[string]string{"chromium": "/opt/chromium"}
	valid := validStandaloneBrowserClaim()
	require.NoError(t, validateBrowserLaunchClaim(valid, runtimes))

	tests := []struct {
		name  string
		claim browseragentapi.BrowserLaunchClaim
	}{
		{name: "different host", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "https://example.com/", ExpiresAt: valid.ExpiresAt, Profile: valid.Profile, Proxy: valid.Proxy}},
		{name: "host suffix confusion", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "https://chatgpt.com.example.com/", ExpiresAt: valid.ExpiresAt, Profile: valid.Profile, Proxy: valid.Proxy}},
		{name: "insecure scheme", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "http://chatgpt.com/", ExpiresAt: valid.ExpiresAt, Profile: valid.Profile, Proxy: valid.Proxy}},
		{name: "query injection", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "https://chatgpt.com/?next=https://example.com", ExpiresAt: valid.ExpiresAt, Profile: valid.Profile, Proxy: valid.Proxy}},
		{name: "expired", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "https://chatgpt.com/", ExpiresAt: time.Now().Unix() - 1, Profile: valid.Profile, Proxy: valid.Proxy}},
		{name: "invalid profile key", claim: browseragentapi.BrowserLaunchClaim{LaunchId: "launch-valid", StartURL: "https://chatgpt.com/", ExpiresAt: valid.ExpiresAt, Profile: browseragentapi.CodexOAuthProfile{RuntimeKey: "chromium", DataKey: "../escape"}, Proxy: valid.Proxy}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, validateBrowserLaunchClaim(&test.claim, runtimes))
		})
	}
}

func TestStandaloneBrowserUsesManagedProfileProxyFingerprintAndPreflight(t *testing.T) {
	launch := validStandaloneBrowserClaim()
	claim := &browseragentapi.CodexOAuthClaim{
		FlowId:      launch.LaunchId,
		State:       launch.LaunchId,
		ExpiresAt:   launch.ExpiresAt,
		Profile:     launch.Profile,
		Proxy:       launch.Proxy,
		Fingerprint: launch.Fingerprint,
	}
	geo := &proxyGeoIdentity{
		CountryCode: "US", Locale: "en-US", Timezone: "America/Los_Angeles",
		Languages: []string{"en-US", "en"}, AcceptLanguage: "en-US,en",
	}
	preflightURL := "http://127.0.0.1:1455/browser/preflight?token=" + launch.LaunchId

	args, err := buildBrowserLaunchArgs(
		"/profiles/"+launch.Profile.DataKey,
		"/profiles/"+launch.Profile.DataKey+"/fingerprint.json",
		"http://127.0.0.1:3000",
		preflightURL,
		claim,
		geo,
	)
	require.NoError(t, err)
	assert.Contains(t, args, "--user-data-dir=/profiles/"+launch.Profile.DataKey)
	assert.Contains(t, args, "--proxy-server=http://127.0.0.1:3000")
	assert.Contains(t, args, "--user-agent=standalone-agent")
	assert.Equal(t, preflightURL, args[len(args)-1])
	assert.NotContains(t, args, launch.StartURL)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	callback := serveOAuthCallbackServer(listener, launch.LaunchId, launch.StartURL, geo)
	t.Cleanup(callback.Close)
	request, err := http.NewRequest(
		http.MethodPost,
		"http://"+listener.Addr().String()+"/browser/preflight?token="+launch.LaunchId,
		strings.NewReader(`{"locale":"en-US","languages":["en-US","en"],"timezone":"America/Los_Angeles"}`),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	var payload map[string]string
	require.NoError(t, common.DecodeJson(response.Body, &payload))
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, launch.StartURL, payload["authorize_url"])
}

func TestFingerprintFileCombinesFixedCoreWithProxyGeoOverlay(t *testing.T) {
	geo := &proxyGeoIdentity{
		CountryCode:    "SG",
		Locale:         "en-SG",
		Timezone:       "Asia/Singapore",
		Languages:      []string{"en-SG", "en", "cmn"},
		AcceptLanguage: "en-SG,en,cmn",
	}
	path, err := writeFingerprintPayload(t.TempDir(), `{
		"profile_id":"fixed-core-v1",
		"geo_overlay":{"country_code":"US"},
		"fingerprint":{
			"accept_language":"en-US,en",
			"timezone":"America/Los_Angeles",
			"canvas":{"noise_seed":"fixed-core"}
		}
	}`, geo)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var rendered struct {
		ProfileId   string `json:"profile_id"`
		Fingerprint struct {
			AcceptLanguage string            `json:"accept_language"`
			Timezone       string            `json:"timezone"`
			Canvas         map[string]string `json:"canvas"`
		} `json:"fingerprint"`
		GeoOverlay struct {
			CountryCode string   `json:"country_code"`
			Locale      string   `json:"locale"`
			Timezone    string   `json:"timezone"`
			Languages   []string `json:"languages"`
		} `json:"geo_overlay"`
	}
	require.NoError(t, common.Unmarshal(data, &rendered))
	assert.Equal(t, "fixed-core-v1", rendered.ProfileId)
	assert.Equal(t, "fixed-core", rendered.Fingerprint.Canvas["noise_seed"])
	assert.Equal(t, "en-SG,en,cmn", rendered.Fingerprint.AcceptLanguage)
	assert.Equal(t, "Asia/Singapore", rendered.Fingerprint.Timezone)
	assert.Equal(t, "SG", rendered.GeoOverlay.CountryCode)
	assert.Equal(t, "en-SG", rendered.GeoOverlay.Locale)
	assert.Equal(t, "Asia/Singapore", rendered.GeoOverlay.Timezone)
	assert.Equal(t, []string{"en-SG", "en", "cmn"}, rendered.GeoOverlay.Languages)
}

func TestFingerprintFileRejectsNullPayload(t *testing.T) {
	geo := &proxyGeoIdentity{
		CountryCode:    "SG",
		Locale:         "en-SG",
		Timezone:       "Asia/Singapore",
		Languages:      []string{"en-SG", "en"},
		AcceptLanguage: "en-SG,en",
	}

	_, err := writeFingerprintPayload(t.TempDir(), `null`, geo)

	assert.ErrorContains(t, err, "JSON object")
}

func TestBrowserGeoPreflightVerifiesReportedValues(t *testing.T) {
	geo := &proxyGeoIdentity{Locale: "en-US", Timezone: "America/Los_Angeles"}

	require.NoError(t, validateBrowserReportedGeo(browserGeoReport{
		Locale:    "en-US",
		Languages: []string{"en-US", "en"},
		Timezone:  "America/Los_Angeles",
	}, geo))

	for _, report := range []browserGeoReport{
		{Locale: "fr-FR", Languages: []string{"fr-FR"}, Timezone: "America/Los_Angeles"},
		{Locale: "en-US", Languages: []string{"en-US"}, Timezone: "UTC"},
		{Locale: "", Languages: nil, Timezone: "America/Los_Angeles"},
	} {
		assert.Error(t, validateBrowserReportedGeo(report, geo))
	}
}

func TestOAuthCallbackRequiresSuccessfulBrowserGeoPreflight(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	callback := serveOAuthCallbackServer(
		listener,
		"test-state",
		"https://auth.openai.com/oauth/authorize?state=test-state",
		&proxyGeoIdentity{Locale: "en-US", Timezone: "America/Los_Angeles"},
	)
	t.Cleanup(callback.Close)
	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := "http://" + listener.Addr().String()

	response, err := client.Get(baseURL + "/auth/callback?state=test-state&code=before-preflight")
	require.NoError(t, err)
	_ = response.Body.Close()
	assert.Equal(t, http.StatusConflict, response.StatusCode)

	preflightRequest, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/browser/preflight?token=test-state",
		strings.NewReader(`{"locale":"en-US","languages":["en-US","en"],"timezone":"America/Los_Angeles"}`),
	)
	require.NoError(t, err)
	preflightRequest.Header.Set("Content-Type", "application/json")
	response, err = client.Do(preflightRequest)
	require.NoError(t, err)
	var preflightResponse map[string]string
	require.NoError(t, common.DecodeJson(response.Body, &preflightResponse))
	_ = response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "https://auth.openai.com/oauth/authorize?state=test-state", preflightResponse["authorize_url"])
	assert.True(t, callback.preflightVerified.Load())
	require.NoError(t, <-callback.preflight)

	callbackResponse := make(chan *http.Response, 1)
	callbackError := make(chan error, 1)
	go func() {
		response, err := client.Get(baseURL + "/auth/callback?state=test-state&code=verified-code")
		if err != nil {
			callbackError <- err
			return
		}
		callbackResponse <- response
	}()
	result := <-callback.result
	assert.Equal(t, "verified-code", result.Code)
	callback.complete(nil)
	select {
	case err := <-callbackError:
		require.NoError(t, err)
	case response := <-callbackResponse:
		_ = response.Body.Close()
		assert.Equal(t, http.StatusOK, response.StatusCode)
	}
}

func TestProxyEndpointAddsSchemeDefaultPort(t *testing.T) {
	httpsProxy, _ := url.Parse("https://proxy.example.com")
	socksProxy, _ := url.Parse("socks5://proxy.example.com")
	explicitProxy, _ := url.Parse("http://proxy.example.com:3128")

	assert.Equal(t, "proxy.example.com:443", proxyEndpoint(httpsProxy))
	assert.Equal(t, "proxy.example.com:1080", proxyEndpoint(socksProxy))
	assert.Equal(t, "proxy.example.com:3128", proxyEndpoint(explicitProxy))
}

func validStandaloneBrowserClaim() *browseragentapi.BrowserLaunchClaim {
	return &browseragentapi.BrowserLaunchClaim{
		LaunchId:  "launch-valid",
		StartURL:  "https://chatgpt.com/",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		Profile: browseragentapi.CodexOAuthProfile{
			Id: 1, Name: "primary", RuntimeKey: "chromium", DataKey: "profile-valid", Persistent: true,
		},
		Proxy: browseragentapi.CodexOAuthProxy{Id: 2, URL: "http://203.0.113.10:8080"},
		Fingerprint: browseragentapi.CodexOAuthFingerprint{
			Id: 3, Name: "fixed", UserAgent: "standalone-agent", ViewportW: 1280, ViewportH: 800,
			Payload: `{"fingerprint":{"canvas":{"noise_seed":"fixed"}}}`, LaunchArgs: `[]`, Environment: `{}`,
		},
	}
}
