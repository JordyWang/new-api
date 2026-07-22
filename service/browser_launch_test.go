package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClaimBrowserProfileLaunchReturnsManagedConfiguration(t *testing.T) {
	database, agent, profile, proxy, fingerprint := useBrowserLaunchServiceTestDatabase(t)
	now := time.Now().Unix()
	launch := &model.BrowserLaunch{
		Id:            "launch-service-claim",
		ProfileId:     profile.Id,
		ChannelId:     99,
		AgentId:       agent.Id,
		ProxyId:       proxy.Id,
		FingerprintId: fingerprint.Id,
		Status:        browseragentapi.FlowStatusPending,
		StartURL:      standaloneBrowserStartURL,
		ExpiresAt:     now + 600,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, database.Create(launch).Error)

	claim, err := ClaimBrowserProfileLaunch(agent.Id, "agent-instance")
	require.NoError(t, err)
	require.NotNil(t, claim)
	assert.Equal(t, launch.Id, claim.LaunchId)
	assert.Equal(t, standaloneBrowserStartURL, claim.StartURL)
	assert.Equal(t, profile.DataKey, claim.Profile.DataKey)
	assert.Equal(t, profile.RuntimeKey, claim.Profile.RuntimeKey)
	assert.Equal(t, "http://user:password@203.0.113.10:8080", claim.Proxy.URL)
	assert.Equal(t, fingerprint.Payload, claim.Fingerprint.Payload)
	assert.Equal(t, fingerprint.LaunchArgs, claim.Fingerprint.LaunchArgs)

	stored, err := model.GetBrowserLaunchById(launch.Id, now)
	require.NoError(t, err)
	assert.Equal(t, browseragentapi.FlowStatusClaimed, stored.Status)
	assert.Equal(t, "agent-instance", stored.AgentInstanceId)
}

func TestStartBrowserProfileLaunchRequiresChannelBinding(t *testing.T) {
	_, _, profile, _, _ := useBrowserLaunchServiceTestDatabase(t)

	_, err := StartBrowserProfileLaunch(profile.Id)

	require.Error(t, err)
	assert.ErrorContains(t, err, "must be bound to a channel")
}

func useBrowserLaunchServiceTestDatabase(t *testing.T) (*gorm.DB, *model.BrowserAgent, *model.BrowserProfile, *model.BrowserProxy, *model.BrowserFingerprint) {
	t.Helper()
	originalDB := model.DB
	originalCryptoSecret := common.CryptoSecret
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	t.Setenv("CRYPTO_SECRET", "browser-launch-service-test-secret")
	common.CryptoSecret = "browser-launch-service-test-secret"
	ResetBrowserProxyURLCache()
	t.Cleanup(func() {
		model.DB = originalDB
		common.CryptoSecret = originalCryptoSecret
		ResetBrowserProxyURLCache()
	})
	require.NoError(t, database.AutoMigrate(
		&model.BrowserAgent{},
		&model.BrowserProxy{},
		&model.BrowserFingerprint{},
		&model.BrowserProfile{},
		&model.BrowserLaunch{},
		&model.CodexOAuthFlow{},
		&model.Channel{},
	))
	now := time.Now().Unix()
	metadata, err := common.Marshal(map[string]any{"capabilities": []string{
		browseragentapi.CapabilityStrictProxyGeoV1,
		browseragentapi.CapabilityProxyGeoOverlayV1,
	}})
	require.NoError(t, err)
	agent := &model.BrowserAgent{
		Name: "desktop", TokenDigest: "digest", Enabled: true, LastSeenAt: now,
		Runtimes: `["chromium"]`, Metadata: string(metadata),
	}
	require.NoError(t, database.Create(agent).Error)
	proxyCiphertext, err := EncryptBrowserProxyURL("http://user:password@203.0.113.10:8080")
	require.NoError(t, err)
	proxy := &model.BrowserProxy{Name: "managed", URLCiphertext: proxyCiphertext, Scheme: "http", Enabled: true}
	require.NoError(t, database.Create(proxy).Error)
	fingerprint := &model.BrowserFingerprint{
		Name: "fixed", Payload: `{"fingerprint":{"canvas":{"noise_seed":"fixed"}}}`,
		LaunchArgs: `[]`, Environment: `{}`, Enabled: true,
	}
	require.NoError(t, database.Create(fingerprint).Error)
	profile := &model.BrowserProfile{
		Name: "primary", AgentId: agent.Id, ProxyId: proxy.Id, FingerprintId: fingerprint.Id,
		RuntimeKey: "chromium", DataKey: "profile-service-test", Persistent: true, Enabled: true,
	}
	require.NoError(t, database.Create(profile).Error)
	return database, agent, profile, proxy, fingerprint
}
