package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type browserProxyChannelFixture struct {
	Id      int `gorm:"primaryKey"`
	Setting *string
}

func (browserProxyChannelFixture) TableName() string {
	return "channels"
}

func TestCountChannelsByBrowserProxyUsesPortableSettingsParsing(t *testing.T) {
	originalDB := DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = database
	t.Cleanup(func() {
		DB = originalDB
	})
	require.NoError(t, database.AutoMigrate(&browserProxyChannelFixture{}))

	managedSeven := browserProxySettingJSON(t, 7)
	managedEight := browserProxySettingJSON(t, 8)
	plainSetting := browserProxySettingJSON(t, 0)
	require.NoError(t, database.Create(&[]browserProxyChannelFixture{
		{Id: 1, Setting: &managedSeven},
		{Id: 2, Setting: &managedEight},
		{Id: 3, Setting: &managedSeven},
		{Id: 4, Setting: &plainSetting},
		{Id: 5, Setting: nil},
	}).Error)

	count, err := CountChannelsByBrowserProxy(7)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	count, err = CountChannelsByBrowserProxy(8)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = CountChannelsByBrowserProxy(9)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestCountChannelsByBrowserProxyFailsClosedOnInvalidSettings(t *testing.T) {
	originalDB := DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = database
	t.Cleanup(func() {
		DB = originalDB
	})
	require.NoError(t, database.AutoMigrate(&browserProxyChannelFixture{}))

	invalid := "{"
	require.NoError(t, database.Create(&browserProxyChannelFixture{Id: 1, Setting: &invalid}).Error)

	_, err = CountChannelsByBrowserProxy(7)
	assert.Error(t, err)
}

func TestCancelCodexOAuthFlowDiscardsUnconsumedCompletedCredential(t *testing.T) {
	originalDB := DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = database
	t.Cleanup(func() {
		DB = originalDB
	})
	require.NoError(t, database.AutoMigrate(&CodexOAuthFlow{}))

	flow := &CodexOAuthFlow{
		Id:                    "flow-discard",
		UserId:                42,
		ProfileId:             1,
		AgentId:               2,
		ProxyId:               3,
		FingerprintId:         4,
		Status:                CodexOAuthFlowStatusCompleted,
		StateDigest:           "digest",
		StateCiphertext:       "state",
		VerifierCiphertext:    "verifier",
		CredentialsCiphertext: "credential",
		AuthorizeURL:          "https://auth.openai.com/oauth/authorize",
		ExpiresAt:             200,
		CreatedAt:             100,
		UpdatedAt:             100,
	}
	require.NoError(t, database.Create(flow).Error)

	err = CancelCodexOAuthFlow(flow.Id, flow.UserId, false, 150)
	require.NoError(t, err)

	var stored CodexOAuthFlow
	require.NoError(t, database.First(&stored, "id = ?", flow.Id).Error)
	assert.Equal(t, CodexOAuthFlowStatusCanceled, stored.Status)
	assert.Empty(t, stored.CredentialsCiphertext)
	assert.Empty(t, stored.StateCiphertext)
	assert.Empty(t, stored.VerifierCiphertext)
}

func TestBrowserProfileBindingSynchronizesChannelProxy(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	channel := &Channel{Name: "bound Codex", Type: constant.ChannelTypeCodex, Key: `{}`, Group: "default"}
	require.NoError(t, database.Create(channel).Error)

	profile := &BrowserProfile{Name: "primary", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-primary"}
	require.NoError(t, CreateBrowserProfile(profile))

	var stored Channel
	require.NoError(t, database.First(&stored, "id = ?", channel.Id).Error)
	assert.Equal(t, 7, stored.GetSetting().BrowserProxyId)
	assert.Empty(t, stored.GetSetting().Proxy)
}

func TestBrowserProfileBindingRejectsProxyMismatch(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	setting := browserProxySettingJSON(t, 8)
	channel := &Channel{Name: "bound Codex", Type: constant.ChannelTypeCodex, Key: `{}`, Group: "default", Setting: &setting}
	require.NoError(t, database.Create(channel).Error)

	profile := &BrowserProfile{Name: "mismatch", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-mismatch"}
	err := CreateBrowserProfile(profile)

	require.Error(t, err)
	assert.ErrorContains(t, err, "channel uses managed proxy 8")
}

func TestBrowserProfileBindingEnforcesOneProfilePerChannel(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	channel := &Channel{Name: "exclusive Codex", Type: constant.ChannelTypeCodex, Key: `{}`, Group: "default"}
	require.NoError(t, database.Create(channel).Error)
	first := &BrowserProfile{Name: "first", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-first"}
	require.NoError(t, CreateBrowserProfile(first))
	second := &BrowserProfile{Name: "second", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-second"}

	err := CreateBrowserProfile(second)

	assert.ErrorIs(t, err, ErrBrowserChannelBound)
}

func TestCompletedOAuthChannelCreationBindsProfile(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	profile := &BrowserProfile{Name: "new channel profile", AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-new-channel"}
	require.NoError(t, database.Create(profile).Error)
	flow := &CodexOAuthFlow{
		Id:                    "flow-create-channel",
		UserId:                42,
		ProfileId:             profile.Id,
		AgentId:               1,
		ProxyId:               profile.ProxyId,
		FingerprintId:         profile.FingerprintId,
		Status:                CodexOAuthFlowStatusCompleted,
		StateDigest:           "digest",
		StateCiphertext:       "state",
		VerifierCiphertext:    "verifier",
		CredentialsCiphertext: "encrypted-credential",
		AuthorizeURL:          "https://auth.openai.com/oauth/authorize",
		ExpiresAt:             200,
		CreatedAt:             100,
		CompletedAt:           120,
	}
	require.NoError(t, database.Create(flow).Error)
	channels := []Channel{{
		Name:   "created Codex",
		Type:   constant.ChannelTypeCodex,
		Key:    `{"access_token":"token","managed_proxy_id":7}`,
		Status: common.ChannelStatusEnabled,
		Models: "gpt-5",
		Group:  "default",
	}}

	require.NoError(t, InsertChannelsWithCodexOAuthFlow(channels, flow.Id, flow.UserId, 150))

	storedProfile, err := GetBrowserProfileById(profile.Id)
	require.NoError(t, err)
	require.NotNil(t, storedProfile.ChannelId)
	storedChannel, err := GetChannelById(*storedProfile.ChannelId, true)
	require.NoError(t, err)
	assert.Equal(t, "created Codex", storedChannel.Name)
	assert.Equal(t, profile.ProxyId, storedChannel.GetSetting().BrowserProxyId)
	storedFlow, err := GetCodexOAuthFlowById(flow.Id)
	require.NoError(t, err)
	assert.Equal(t, storedChannel.Id, storedFlow.ChannelId)
	assert.EqualValues(t, 150, storedFlow.ConsumedAt)
}

func TestBrowserLaunchLifecycle(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	require.NoError(t, database.Create(&BrowserAgent{Id: 3, Name: "desktop", TokenDigest: "digest"}).Error)
	launch := &BrowserLaunch{
		Id: "launch-lifecycle", ProfileId: 1, ChannelId: 2, AgentId: 3, ProxyId: 4, FingerprintId: 5,
		Status: browseragentapi.FlowStatusPending, StartURL: "https://chatgpt.com/", ExpiresAt: 1000, CreatedAt: 100, UpdatedAt: 100,
	}
	require.NoError(t, CreateExclusiveBrowserLaunch(launch))

	claimed, err := ClaimBrowserLaunch(launch.AgentId, "instance-a", 110, 140)
	require.NoError(t, err)
	assert.Equal(t, browseragentapi.FlowStatusClaimed, claimed.Status)
	assert.Equal(t, "instance-a", claimed.AgentInstanceId)
	require.NoError(t, MarkBrowserLaunchRunning(launch.AgentId, "instance-a", launch.Id, 120, 150))
	require.NoError(t, CompleteBrowserLaunch(launch.AgentId, "instance-a", launch.Id, 130))

	stored, err := GetBrowserLaunchById(launch.Id, 130)
	require.NoError(t, err)
	assert.Equal(t, browseragentapi.FlowStatusCompleted, stored.Status)
	assert.EqualValues(t, 120, stored.RunningAt)
	assert.EqualValues(t, 130, stored.CompletedAt)
}

func TestBrowserLaunchAndOAuthAreAgentExclusive(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	require.NoError(t, database.Create(&BrowserAgent{Id: 3, Name: "desktop", TokenDigest: "digest"}).Error)
	launch := &BrowserLaunch{
		Id: "launch-active", ProfileId: 1, ChannelId: 2, AgentId: 3, ProxyId: 4, FingerprintId: 5,
		Status: browseragentapi.FlowStatusPending, StartURL: "https://chatgpt.com/", ExpiresAt: 1000, CreatedAt: 100,
	}
	require.NoError(t, CreateExclusiveBrowserLaunch(launch))
	flow := &CodexOAuthFlow{
		Id: "flow-blocked", UserId: 1, ProfileId: 1, AgentId: 3, ProxyId: 4, FingerprintId: 5,
		Status: CodexOAuthFlowStatusPending, StateDigest: "digest", StateCiphertext: "state", VerifierCiphertext: "verifier",
		AuthorizeURL: "https://auth.openai.com/oauth/authorize", ExpiresAt: 1000, CreatedAt: 110,
	}
	assert.ErrorIs(t, CreateExclusiveCodexOAuthFlow(flow), ErrBrowserAgentBusy)
	require.NoError(t, CancelBrowserLaunch(launch.Id, 120))
	require.NoError(t, CreateExclusiveCodexOAuthFlow(flow))
	secondLaunch := &BrowserLaunch{
		Id: "launch-blocked", ProfileId: 1, ChannelId: 2, AgentId: 3, ProxyId: 4, FingerprintId: 5,
		Status: browseragentapi.FlowStatusPending, StartURL: "https://chatgpt.com/", ExpiresAt: 1000, CreatedAt: 130,
	}
	assert.ErrorIs(t, CreateExclusiveBrowserLaunch(secondLaunch), ErrBrowserAgentBusy)
}

func TestBoundBrowserProfilePreventsChannelDeletion(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	channel := &Channel{Name: "bound Codex", Type: constant.ChannelTypeCodex, Key: `{}`, Group: "default"}
	require.NoError(t, database.Create(channel).Error)
	profile := &BrowserProfile{Name: "primary", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-delete-guard"}
	require.NoError(t, CreateBrowserProfile(profile))

	err := channel.Delete()

	assert.True(t, errors.Is(err, ErrBrowserChannelBound))
	var count int64
	require.NoError(t, database.Model(&Channel{}).Where("id = ?", channel.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestBoundBrowserProfilePreventsChannelProxyMismatchOnUpdate(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	channel := &Channel{Name: "bound Codex", Type: constant.ChannelTypeCodex, Key: `{"managed_proxy_id":7}`, Group: "default"}
	require.NoError(t, database.Create(channel).Error)
	profile := &BrowserProfile{Name: "primary", ChannelId: &channel.Id, AgentId: 1, ProxyId: 7, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-update-guard"}
	require.NoError(t, CreateBrowserProfile(profile))
	mismatch := browserProxySettingJSON(t, 8)
	channel.Setting = &mismatch

	err := channel.Update()

	require.Error(t, err)
	assert.ErrorContains(t, err, "must use browser profile managed proxy 7")
	stored, getErr := GetChannelById(channel.Id, true)
	require.NoError(t, getErr)
	assert.Equal(t, 7, stored.GetSetting().BrowserProxyId)
}

func TestBrowserProxyChannelLimitRejectsSecondChannel(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 17, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	first := &Channel{Name: "first account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}
	second := &Channel{Name: "second account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}

	require.NoError(t, first.Insert())
	err := second.Insert()

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	var count int64
	require.NoError(t, database.Model(&Channel{}).Where("name IN ?", []string{first.Name, second.Name}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestNormalizeBrowserProxyChannelLimitsBackfillsExistingNull(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	require.NoError(t, database.Exec("UPDATE browser_proxies SET max_channel_accounts = NULL WHERE id = ?", 7).Error)

	require.NoError(t, normalizeBrowserProxyChannelLimits())

	var nullCount int64
	require.NoError(t, database.Model(&BrowserProxy{}).Where("id = ? AND max_channel_accounts IS NULL", 7).Count(&nullCount).Error)
	assert.Zero(t, nullCount)
	stored, err := GetBrowserProxyById(7)
	require.NoError(t, err)
	assert.Equal(t, DefaultBrowserProxyChannelAccounts, stored.MaxChannelAccounts)
}

func TestCountBrowserProfilesByProxiesReportsBoundEnvironments(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	require.NoError(t, database.Create(&[]BrowserProfile{
		{Name: "proxy seven environment one", AgentId: 1, ProxyId: 7, FingerprintId: 1, RuntimeKey: "chromium", DataKey: "proxy-seven-one"},
		{Name: "proxy seven environment two", AgentId: 1, ProxyId: 7, FingerprintId: 1, RuntimeKey: "chromium", DataKey: "proxy-seven-two"},
		{Name: "proxy eight environment", AgentId: 1, ProxyId: 8, FingerprintId: 1, RuntimeKey: "chromium", DataKey: "proxy-eight-one"},
	}).Error)

	counts, err := CountBrowserProfilesByProxies([]int{8, 7, 9, 7})

	require.NoError(t, err)
	assert.EqualValues(t, 2, counts[7])
	assert.EqualValues(t, 1, counts[8])
	assert.Zero(t, counts[9])
}

func TestBrowserProxyChannelLimitRejectsBatchAtomically(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 18, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	channels := []Channel{
		{Name: "batch account one", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting},
		{Name: "batch account two", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting},
	}

	err := BatchInsertChannels(channels)

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	var count int64
	require.NoError(t, database.Model(&Channel{}).Where("name LIKE ?", "batch account%").Count(&count).Error)
	assert.Zero(t, count)
}

func TestBrowserProxyChannelLimitRejectsRebindButAllowsCurrentChannelUpdate(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	sourceProxy := createBrowserProxyFixture(t, database, 19, 0)
	targetProxy := createBrowserProxyFixture(t, database, 20, 1)
	sourceSetting := browserProxySettingJSON(t, sourceProxy.Id)
	targetSetting := browserProxySettingJSON(t, targetProxy.Id)
	occupied := &Channel{Name: "occupied account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &targetSetting}
	candidate := &Channel{Name: "candidate account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &sourceSetting}
	require.NoError(t, occupied.Insert())
	require.NoError(t, candidate.Insert())

	candidate.Setting = &targetSetting
	err := candidate.Update()

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	storedCandidate, getErr := GetChannelById(candidate.Id, true)
	require.NoError(t, getErr)
	assert.Equal(t, sourceProxy.Id, storedCandidate.GetSetting().BrowserProxyId)

	occupied.Name = "occupied account renamed"
	require.NoError(t, occupied.Update())
	storedOccupied, getErr := GetChannelById(occupied.Id, true)
	require.NoError(t, getErr)
	assert.Equal(t, "occupied account renamed", storedOccupied.Name)
}

func TestBrowserProxyChannelLimitCannotDropBelowUsage(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 21, 0)
	setting := browserProxySettingJSON(t, proxy.Id)
	require.NoError(t, (&Channel{Name: "existing account one", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}).Insert())
	require.NoError(t, (&Channel{Name: "existing account two", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}).Insert())

	proxy.MaxChannelAccounts = 1
	err := UpdateBrowserProxy(proxy, false)

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	stored, getErr := GetBrowserProxyById(proxy.Id)
	require.NoError(t, getErr)
	assert.Zero(t, stored.MaxChannelAccounts)
}

func TestBrowserProxyChannelLimitAppliesWhenProfileBindsChannel(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 22, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	require.NoError(t, (&Channel{Name: "occupied profile account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}).Insert())
	channel := &Channel{Name: "unbound profile account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default"}
	require.NoError(t, database.Create(channel).Error)
	profile := &BrowserProfile{Name: "capacity profile", ChannelId: &channel.Id, AgentId: 1, ProxyId: proxy.Id, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-capacity"}

	err := CreateBrowserProfile(profile)

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	var profileCount int64
	require.NoError(t, database.Model(&BrowserProfile{}).Where("data_key = ?", profile.DataKey).Count(&profileCount).Error)
	assert.Zero(t, profileCount)
	storedChannel, getErr := GetChannelById(channel.Id, true)
	require.NoError(t, getErr)
	assert.Zero(t, storedChannel.GetSetting().BrowserProxyId)
}

func TestBrowserProxyChannelLimitPreservesUnconsumedOAuthCredential(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 23, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	require.NoError(t, (&Channel{Name: "occupied OAuth account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}).Insert())
	profile := &BrowserProfile{Name: "new OAuth account", AgentId: 1, ProxyId: proxy.Id, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-oauth-capacity"}
	require.NoError(t, database.Create(profile).Error)
	flow := &CodexOAuthFlow{
		Id: "flow-capacity", UserId: 42, ProfileId: profile.Id, AgentId: 1, ProxyId: proxy.Id, FingerprintId: 2,
		Status: CodexOAuthFlowStatusCompleted, StateDigest: "digest", StateCiphertext: "state", VerifierCiphertext: "verifier",
		CredentialsCiphertext: "encrypted-credential", AuthorizeURL: "https://auth.openai.com/oauth/authorize", ExpiresAt: 200, CreatedAt: 100,
	}
	require.NoError(t, database.Create(flow).Error)
	channels := []Channel{{Name: "rejected OAuth account", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default"}}

	err := InsertChannelsWithCodexOAuthFlow(channels, flow.Id, flow.UserId, 150)

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	storedFlow, getErr := GetCodexOAuthFlowById(flow.Id)
	require.NoError(t, getErr)
	assert.Zero(t, storedFlow.ConsumedAt)
	assert.Equal(t, "encrypted-credential", storedFlow.CredentialsCiphertext)
	var channelCount int64
	require.NoError(t, database.Model(&Channel{}).Where("name = ?", channels[0].Name).Count(&channelCount).Error)
	assert.Zero(t, channelCount)
}

func TestBrowserProxyChannelLimitAllowsOAuthRefreshForCurrentChannel(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 24, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	channel := &Channel{Name: "OAuth refresh account", Type: constant.ChannelTypeCodex, Key: `{"old":"credential"}`, Models: "gpt-5", Group: "default", Setting: &setting}
	require.NoError(t, channel.Insert())
	profile := &BrowserProfile{Name: "OAuth refresh profile", ChannelId: &channel.Id, AgentId: 5, ProxyId: proxy.Id, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-oauth-refresh"}
	require.NoError(t, CreateBrowserProfile(profile))
	flow := &CodexOAuthFlow{
		Id: "flow-refresh-capacity", UserId: 42, ChannelId: channel.Id, ProfileId: profile.Id, AgentId: profile.AgentId, ProxyId: proxy.Id, FingerprintId: 2,
		Status: CodexOAuthFlowStatusRunning, AgentInstanceId: "instance-refresh", StateDigest: "digest", StateCiphertext: "state", VerifierCiphertext: "verifier",
		AuthorizeURL: "https://auth.openai.com/oauth/authorize", ExpiresAt: 300, CreatedAt: 100,
	}
	require.NoError(t, database.Create(flow).Error)

	err := CompleteCodexOAuthFlow(profile.AgentId, flow.AgentInstanceId, flow.Id, CodexOAuthFlowCompletion{
		ChannelKey: "new-credential", CompletedAt: 150,
	})

	require.NoError(t, err)
	storedChannel, getErr := GetChannelById(channel.Id, true)
	require.NoError(t, getErr)
	assert.Equal(t, "new-credential", storedChannel.Key)
	assert.Equal(t, proxy.Id, storedChannel.GetSetting().BrowserProxyId)
}

func TestBrowserProxyChannelLimitRecheckedWhenOAuthBindsExistingChannel(t *testing.T) {
	database := useBrowserModelTestDatabase(t)
	proxy := createBrowserProxyFixture(t, database, 25, 1)
	setting := browserProxySettingJSON(t, proxy.Id)
	require.NoError(t, (&Channel{Name: "occupied OAuth refresh proxy", Type: constant.ChannelTypeCodex, Key: `{}`, Models: "gpt-5", Group: "default", Setting: &setting}).Insert())
	candidate := &Channel{Name: "OAuth bind candidate", Type: constant.ChannelTypeCodex, Key: `old-credential`, Models: "gpt-5", Group: "default"}
	require.NoError(t, database.Create(candidate).Error)
	profile := &BrowserProfile{Name: "OAuth bind profile", AgentId: 6, ProxyId: proxy.Id, FingerprintId: 2, RuntimeKey: "chromium", DataKey: "profile-oauth-bind"}
	require.NoError(t, database.Create(profile).Error)
	flow := &CodexOAuthFlow{
		Id: "flow-bind-capacity", UserId: 42, ChannelId: candidate.Id, ProfileId: profile.Id, AgentId: profile.AgentId, ProxyId: proxy.Id, FingerprintId: 2,
		Status: CodexOAuthFlowStatusRunning, AgentInstanceId: "instance-bind", StateDigest: "digest", StateCiphertext: "state", VerifierCiphertext: "verifier",
		AuthorizeURL: "https://auth.openai.com/oauth/authorize", ExpiresAt: 300, CreatedAt: 100,
	}
	require.NoError(t, database.Create(flow).Error)

	err := CompleteCodexOAuthFlow(profile.AgentId, flow.AgentInstanceId, flow.Id, CodexOAuthFlowCompletion{
		ChannelKey: "new-credential", CompletedAt: 150,
	})

	assert.ErrorIs(t, err, ErrBrowserProxyChannelLimit)
	storedChannel, getErr := GetChannelById(candidate.Id, true)
	require.NoError(t, getErr)
	assert.Equal(t, "old-credential", storedChannel.Key)
	assert.Zero(t, storedChannel.GetSetting().BrowserProxyId)
	storedFlow, getErr := GetCodexOAuthFlowById(flow.Id)
	require.NoError(t, getErr)
	assert.Equal(t, CodexOAuthFlowStatusRunning, storedFlow.Status)
}

func useBrowserModelTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = database
	t.Cleanup(func() {
		DB = originalDB
	})
	require.NoError(t, database.AutoMigrate(
		&BrowserAgent{},
		&BrowserProxy{},
		&BrowserFingerprint{},
		&BrowserProfile{},
		&BrowserLaunch{},
		&CodexOAuthFlow{},
		&Channel{},
		&Ability{},
	))
	require.NoError(t, database.Create(&[]BrowserProxy{
		{Id: 7, Name: "fixture proxy 7", URLCiphertext: "encrypted-7", Scheme: "socks5h", Enabled: true},
		{Id: 8, Name: "fixture proxy 8", URLCiphertext: "encrypted-8", Scheme: "socks5h", Enabled: true},
	}).Error)
	return database
}

func createBrowserProxyFixture(t *testing.T, database *gorm.DB, id int, maxChannelAccounts int) *BrowserProxy {
	t.Helper()
	proxy := &BrowserProxy{
		Id:                 id,
		Name:               "capacity proxy",
		URLCiphertext:      "encrypted-capacity",
		Scheme:             "socks5h",
		Enabled:            true,
		MaxChannelAccounts: maxChannelAccounts,
	}
	require.NoError(t, database.Create(proxy).Error)
	return proxy
}

func browserProxySettingJSON(t *testing.T, proxyId int) string {
	t.Helper()
	encoded, err := common.Marshal(dto.ChannelSettings{BrowserProxyId: proxyId})
	require.NoError(t, err)
	return string(encoded)
}
