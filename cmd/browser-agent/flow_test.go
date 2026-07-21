package main

import (
	"net"
	"net/http"
	"net/url"
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

func TestManagedBrowserEnforcesProxyLocaleAndTimezone(t *testing.T) {
	claim := &browseragentapi.CodexOAuthClaim{
		AuthorizeURL: "https://auth.openai.com/oauth/authorize?state=test",
		Fingerprint: browseragentapi.CodexOAuthFingerprint{
			UserAgent:   "managed-agent",
			Locale:      "en-US",
			Timezone:    "America/Los_Angeles",
			ViewportW:   1280,
			ViewportH:   800,
			LaunchArgs:  `[]`,
			Environment: `{"FINGERPRINT_PROFILE":"managed"}`,
		},
	}

	preflightURL := "http://127.0.0.1:1455/browser/preflight?token=test"
	args, err := buildBrowserLaunchArgs("/profiles/test", "/profiles/test/fingerprint.json", "http://127.0.0.1:3000", preflightURL, claim)
	require.NoError(t, err)
	assert.Contains(t, args, "--proxy-server=http://127.0.0.1:3000")
	assert.Contains(t, args, "--proxy-bypass-list=<-loopback>;localhost;127.0.0.1;[::1]")
	assert.Contains(t, args, "--lang=en-US")
	assert.Contains(t, args, "--force-time-zone-for-testing=America/Los_Angeles")
	assert.Contains(t, args, "--force-webrtc-ip-handling-policy=disable_non_proxied_udp")
	assert.Contains(t, args, "--disable-quic")
	assert.Equal(t, preflightURL, args[len(args)-1])
	assert.NotContains(t, args, claim.AuthorizeURL)

	t.Setenv("TZ", "UTC")
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("HTTP_PROXY", "http://unmanaged.example:8080")
	environment, err := buildBrowserEnvironment("/profiles/test", "/profiles/test/fingerprint.json", "http://127.0.0.1:3000", preflightURL, claim)
	require.NoError(t, err)
	environmentValues := make(map[string]string, len(environment))
	for _, item := range environment {
		key, value, found := strings.Cut(item, "=")
		if found {
			environmentValues[strings.ToUpper(key)] = value
		}
	}
	assert.Equal(t, "America/Los_Angeles", environmentValues["TZ"])
	assert.Equal(t, "en_US.UTF-8", environmentValues["LANG"])
	assert.Equal(t, "en-US", environmentValues["LANGUAGE"])
	assert.Equal(t, "http://127.0.0.1:3000", environmentValues["HTTP_PROXY"])
	assert.Equal(t, "http://127.0.0.1:3000", environmentValues["HTTPS_PROXY"])
	assert.Equal(t, "localhost,127.0.0.1,::1", environmentValues["NO_PROXY"])
}

func TestBrowserFingerprintPreflightVerifiesReportedValues(t *testing.T) {
	fingerprint := browseragentapi.CodexOAuthFingerprint{
		Locale:   "en-US",
		Timezone: "America/Los_Angeles",
	}

	require.NoError(t, validateBrowserReportedFingerprint(browserFingerprintReport{
		Locale:    "en-US",
		Languages: []string{"en-US", "en"},
		Timezone:  "America/Los_Angeles",
	}, fingerprint))

	for _, report := range []browserFingerprintReport{
		{Locale: "fr-FR", Languages: []string{"fr-FR"}, Timezone: "America/Los_Angeles"},
		{Locale: "en-US", Languages: []string{"en-US"}, Timezone: "UTC"},
		{Locale: "", Languages: nil, Timezone: "America/Los_Angeles"},
	} {
		assert.Error(t, validateBrowserReportedFingerprint(report, fingerprint))
	}
}

func TestOAuthCallbackRequiresSuccessfulBrowserFingerprintPreflight(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	callback := serveOAuthCallbackServer(
		listener,
		"test-state",
		"https://auth.openai.com/oauth/authorize?state=test-state",
		browseragentapi.CodexOAuthFingerprint{Locale: "en-US", Timezone: "America/Los_Angeles"},
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
