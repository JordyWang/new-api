package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationInviteCodeTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RegistrationInviteCode{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RegistrationInviteCode{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RegistrationInviteCode{}).Error)
	})
}

func TestReserveRegistrationInviteCodeAppliesProvisioningAndLimit(t *testing.T) {
	setupRegistrationInviteCodeTest(t)

	saved, err := SaveRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "invite-123",
		Group:            "vip",
		InitialQuota:     1234,
		MaxRegistrations: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "vip", saved.Group)

	var reserved *RegistrationInviteCode
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reserved, err = ReserveRegistrationInviteCode(tx, " invite-123 ")
		return err
	}))
	require.NotNil(t, reserved)
	assert.Equal(t, 1234, reserved.InitialQuota)
	assert.Equal(t, "vip", reserved.Group)
	assert.Equal(t, 1, reserved.RegisteredCount)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := ReserveRegistrationInviteCode(tx, "invite-123")
		return err
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRegistrationInviteCodeExhausted)
}

func TestReserveRegistrationInviteCodeRollsBackUsageOnUserTransactionFailure(t *testing.T) {
	setupRegistrationInviteCodeTest(t)
	require.NoError(t, DB.Create(&RegistrationInviteCode{
		ID:               RegistrationInviteCodeID,
		Code:             "rollback-code",
		Group:            "default",
		InitialQuota:     10,
		MaxRegistrations: 1,
	}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := ReserveRegistrationInviteCode(tx, "rollback-code")
		if err != nil {
			return err
		}
		return errors.New("simulate user creation failure")
	})
	require.EqualError(t, err, "simulate user creation failure")

	var config RegistrationInviteCode
	require.NoError(t, DB.First(&config, RegistrationInviteCodeID).Error)
	assert.Zero(t, config.RegisteredCount)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := ReserveRegistrationInviteCode(tx, "rollback-code")
		return err
	}))
}

func TestSaveRegistrationInviteCodeStartsNewCampaignWhenCodeChanges(t *testing.T) {
	setupRegistrationInviteCodeTest(t)

	_, err := SaveRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "old-code",
		InitialQuota:     1,
		MaxRegistrations: 10,
	})
	require.NoError(t, err)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := ReserveRegistrationInviteCode(tx, "old-code")
		return err
	}))

	saved, err := SaveRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "old-code",
		Group:            "premium",
		InitialQuota:     2,
		MaxRegistrations: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, saved.RegisteredCount)
	assert.Equal(t, "premium", saved.Group)

	saved, err = SaveRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "new-code",
		Group:            "default",
		InitialQuota:     3,
		MaxRegistrations: 2,
	})
	require.NoError(t, err)
	assert.Zero(t, saved.RegisteredCount)
}
