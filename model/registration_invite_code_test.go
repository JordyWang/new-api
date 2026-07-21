package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestReserveRegistrationInviteCodeSelectsMatchingCodeAndEnforcesLimit(t *testing.T) {
	setupRegistrationInviteCodeTest(t)

	limited, err := CreateRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "invite-123",
		Group:            "vip",
		InitialQuota:     1234,
		MaxRegistrations: 1,
	})
	require.NoError(t, err)
	_, err = CreateRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "invite-456",
		Group:            "premium",
		InitialQuota:     5678,
		MaxRegistrations: 2,
	})
	require.NoError(t, err)

	var reserved *RegistrationInviteCode
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		reserved, reserveErr = ReserveRegistrationInviteCode(tx, " invite-123 ")
		return reserveErr
	}))
	require.NotNil(t, reserved)
	assert.Equal(t, limited.ID, reserved.ID)
	assert.Equal(t, 1234, reserved.InitialQuota)
	assert.Equal(t, "vip", reserved.Group)
	assert.Equal(t, 1, reserved.RegisteredCount)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, reserveErr := ReserveRegistrationInviteCode(tx, "invite-123")
		return reserveErr
	})
	require.ErrorIs(t, err, ErrRegistrationInviteCodeExhausted)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		second, reserveErr := ReserveRegistrationInviteCode(tx, "invite-456")
		if reserveErr == nil {
			assert.Equal(t, "premium", second.Group)
		}
		return reserveErr
	}))
}

func TestReserveRegistrationInviteCodeRejectsDisabledCode(t *testing.T) {
	setupRegistrationInviteCodeTest(t)

	created, err := CreateRegistrationInviteCode(&RegistrationInviteCode{Code: "disabled-code"})
	require.NoError(t, err)
	_, err = UpdateRegistrationInviteCodeStatus(created.ID, common.RedemptionCodeStatusDisabled)
	require.NoError(t, err)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, reserveErr := ReserveRegistrationInviteCode(tx, "disabled-code")
		return reserveErr
	})
	require.ErrorIs(t, err, ErrRegistrationInviteCodeInvalid)
}

func TestReserveRegistrationInviteCodeRollsBackUsageOnUserTransactionFailure(t *testing.T) {
	setupRegistrationInviteCodeTest(t)
	created, err := CreateRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "rollback-code",
		Group:            "default",
		InitialQuota:     10,
		MaxRegistrations: 1,
	})
	require.NoError(t, err)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, reserveErr := ReserveRegistrationInviteCode(tx, "rollback-code")
		if reserveErr != nil {
			return reserveErr
		}
		return errors.New("simulate user creation failure")
	})
	require.EqualError(t, err, "simulate user creation failure")

	config, err := GetRegistrationInviteCodeByID(created.ID)
	require.NoError(t, err)
	assert.Zero(t, config.RegisteredCount)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, reserveErr := ReserveRegistrationInviteCode(tx, "rollback-code")
		return reserveErr
	}))
}

func TestRegistrationInviteCodeCRUDAndPagination(t *testing.T) {
	setupRegistrationInviteCodeTest(t)

	first, err := CreateRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "first-code",
		Group:            "default",
		InitialQuota:     10,
		MaxRegistrations: 5,
	})
	require.NoError(t, err)
	second, err := CreateRegistrationInviteCode(&RegistrationInviteCode{
		Code:             "second-code",
		Group:            "vip",
		InitialQuota:     20,
		MaxRegistrations: 10,
	})
	require.NoError(t, err)

	codes, total, err := GetRegistrationInviteCodes("", "", 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, codes, 1)
	assert.Equal(t, second.ID, codes[0].ID)

	second.Code = "updated-code"
	second.Group = "premium"
	second.InitialQuota = 30
	updated, err := UpdateRegistrationInviteCode(second)
	require.NoError(t, err)
	assert.Equal(t, "updated-code", updated.Code)
	assert.Equal(t, "premium", updated.Group)
	assert.Equal(t, 30, updated.InitialQuota)

	_, err = CreateRegistrationInviteCode(&RegistrationInviteCode{Code: "updated-code"})
	require.ErrorIs(t, err, ErrRegistrationInviteCodeDuplicate)

	_, err = UpdateRegistrationInviteCodeStatus(first.ID, common.RedemptionCodeStatusDisabled)
	require.NoError(t, err)
	disabled, total, err := GetRegistrationInviteCodes("", "disabled", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, disabled, 1)
	assert.Equal(t, first.ID, disabled[0].ID)

	require.NoError(t, DeleteRegistrationInviteCode(first.ID))
	_, err = GetRegistrationInviteCodeByID(first.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestMigrateRegistrationInviteCodesPreservesConfiguredLegacyCode(t *testing.T) {
	setupRegistrationInviteCodeTest(t)
	require.NoError(t, DB.Create(&RegistrationInviteCode{
		Code:  "legacy-code",
		Group: "legacy",
	}).Error)
	require.NoError(t, DB.Create(&RegistrationInviteCode{
		Code:  "",
		Group: "default",
	}).Error)

	require.NoError(t, migrateRegistrationInviteCodes())

	var codes []RegistrationInviteCode
	require.NoError(t, DB.Order("id ASC").Find(&codes).Error)
	require.Len(t, codes, 1)
	assert.Equal(t, "legacy-code", codes[0].Code)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, codes[0].Status)
	assert.Positive(t, codes[0].CreatedTime)
	assert.Positive(t, codes[0].UpdatedTime)
}
