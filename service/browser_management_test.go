package service

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateBrowserProxyURLAcceptsSupportedPublicProxies(t *testing.T) {
	tests := []struct {
		name       string
		proxyURL   string
		expectURL  string
		expectType string
	}{
		{name: "http credentials", proxyURL: "http://user:pass@203.0.113.10:8080", expectURL: "http://user:pass@203.0.113.10:8080", expectType: "http"},
		{name: "https domain", proxyURL: "HTTPS://proxy.example.com:443", expectURL: "https://proxy.example.com:443", expectType: "https"},
		{name: "socks sticky query", proxyURL: "socks5h://user:session@198.51.100.8:1080?region=us", expectURL: "socks5h://user:session@198.51.100.8:1080?region=us", expectType: "socks5h"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalized, scheme, err := ValidateBrowserProxyURL(test.proxyURL)
			require.NoError(t, err)
			assert.Equal(t, test.expectURL, normalized)
			assert.Equal(t, test.expectType, scheme)
		})
	}
}

func TestValidateBrowserProxyURLRejectsPrivateAndUnsupportedTargets(t *testing.T) {
	for _, test := range []struct {
		name     string
		proxyURL string
	}{
		{name: "loopback", proxyURL: "http://127.0.0.1:8080"},
		{name: "private address", proxyURL: "http://10.0.0.1:8080"},
		{name: "localhost", proxyURL: "http://localhost:8080"},
		{name: "multicast", proxyURL: "http://224.0.0.1:8080"},
		{name: "unsupported scheme", proxyURL: "ftp://203.0.113.10:21"},
		{name: "path", proxyURL: "http://203.0.113.10:8080/path"},
		{name: "oversized URL", proxyURL: "http://" + strings.Repeat("a", 4096)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := ValidateBrowserProxyURL(test.proxyURL)
			assert.Error(t, err)
		})
	}
}

func TestMaskBrowserProxyURLRemovesCredentialsAndQueryValues(t *testing.T) {
	masked := MaskBrowserProxyURL("http://sticky-user:secret@proxy.example.com:8080?session=abc&region=us")
	assert.Equal(t, "http://%2A%2A%2A@proxy.example.com:8080?region=%2A%2A%2A&session=%2A%2A%2A", masked)
	assert.NotContains(t, masked, "secret")
	assert.NotContains(t, masked, "sticky-user")
	assert.NotContains(t, masked, "abc")
}

func TestEncryptBrowserProxyURLRequiresPersistentServerSecret(t *testing.T) {
	originalCryptoSecret := common.CryptoSecret
	t.Cleanup(func() {
		common.CryptoSecret = originalCryptoSecret
	})

	t.Setenv("CRYPTO_SECRET", "")
	t.Setenv("SESSION_SECRET", "")
	common.CryptoSecret = "ephemeral-process-secret"
	_, err := EncryptBrowserProxyURL("https://proxy.example.com:443")
	assert.Error(t, err)

	t.Setenv("CRYPTO_SECRET", "persistent-test-secret")
	common.CryptoSecret = os.Getenv("CRYPTO_SECRET")
	encrypted, err := EncryptBrowserProxyURL("https://proxy.example.com:443")
	require.NoError(t, err)
	assert.NotContains(t, encrypted, "proxy.example.com")
}

func TestBrowserAgentTokenDigestDoesNotDependOnProcessCryptoSecret(t *testing.T) {
	originalCryptoSecret := common.CryptoSecret
	t.Cleanup(func() {
		common.CryptoSecret = originalCryptoSecret
	})

	common.CryptoSecret = "first-secret"
	first := BrowserAgentTokenDigest("nba_example-token")
	common.CryptoSecret = "second-secret"
	second := BrowserAgentTokenDigest("nba_example-token")

	assert.Equal(t, first, second)
	assert.Len(t, first, 64)
}

func TestResolveChannelProxyURLRejectsManagedCredentialProxyBypass(t *testing.T) {
	keyBytes, err := common.Marshal(CodexOAuthKey{
		AccessToken:    "access-token",
		RefreshToken:   "refresh-token",
		ManagedProxyID: 42,
	})
	require.NoError(t, err)

	for _, setting := range []dto.ChannelSettings{
		{Proxy: "https://fallback-proxy.example.com:443"},
		{BrowserProxyId: 41, Proxy: "https://fallback-proxy.example.com:443"},
	} {
		settingBytes, err := common.Marshal(setting)
		require.NoError(t, err)
		settingJSON := string(settingBytes)
		channel := &model.Channel{
			Type:    constant.ChannelTypeCodex,
			Key:     string(keyBytes),
			Setting: &settingJSON,
		}

		_, err = ResolveChannelProxyURL(channel)
		require.Error(t, err)
		assert.ErrorContains(t, err, "managed browser proxy 42")
	}
}

func TestResolveChannelProxyURLUsesManagedProxyForAnyChannel(t *testing.T) {
	originalDB := model.DB
	originalCryptoSecret := common.CryptoSecret
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	t.Setenv("CRYPTO_SECRET", "managed-proxy-test-secret")
	common.CryptoSecret = "managed-proxy-test-secret"
	ResetBrowserProxyURLCache()
	t.Cleanup(func() {
		model.DB = originalDB
		common.CryptoSecret = originalCryptoSecret
		ResetBrowserProxyURLCache()
	})
	require.NoError(t, database.AutoMigrate(&model.BrowserProxy{}))

	proxyURL := "https://user:password@proxy.example.com:443"
	ciphertext, err := EncryptBrowserProxyURL(proxyURL)
	require.NoError(t, err)
	require.NoError(t, database.Create(&model.BrowserProxy{
		Id:            19,
		Name:          "shared proxy",
		URLCiphertext: ciphertext,
		Scheme:        "https",
		Enabled:       true,
	}).Error)
	settingBytes, err := common.Marshal(dto.ChannelSettings{BrowserProxyId: 19})
	require.NoError(t, err)
	setting := string(settingBytes)
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Setting: &setting,
	}

	resolved, err := ResolveChannelProxyURL(channel)
	require.NoError(t, err)
	assert.Equal(t, proxyURL, resolved)
}

func TestParseCodexOAuthKeyPreservesManagedProxyBinding(t *testing.T) {
	key, err := parseCodexOAuthKey(`{"access_token":"token","managed_proxy_id":42}`)
	require.NoError(t, err)
	assert.Equal(t, 42, key.ManagedProxyID)
}
