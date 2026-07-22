package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateChannelAcceptsManagedProxyForAnyChannel(t *testing.T) {
	originalDB := model.DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = database
	t.Cleanup(func() {
		model.DB = originalDB
	})
	require.NoError(t, database.AutoMigrate(&model.BrowserProxy{}))
	require.NoError(t, database.Create(&model.BrowserProxy{
		Id:            7,
		Name:          "shared proxy",
		URLCiphertext: "encrypted",
		Scheme:        "https",
		Enabled:       true,
	}).Error)

	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	channel.SetSetting(dto.ChannelSettings{
		BrowserProxyId: 7,
		Proxy:          "socks5://legacy.example.com:1080",
	})

	require.NoError(t, validateChannel(channel, false))
	setting := channel.GetSetting()
	assert.Equal(t, 7, setting.BrowserProxyId)
	assert.Empty(t, setting.Proxy)
}
