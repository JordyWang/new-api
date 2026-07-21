package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

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

func browserProxySettingJSON(t *testing.T, proxyId int) string {
	t.Helper()
	encoded, err := common.Marshal(dto.ChannelSettings{BrowserProxyId: proxyId})
	require.NoError(t, err)
	return string(encoded)
}
