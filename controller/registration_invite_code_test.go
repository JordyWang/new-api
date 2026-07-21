package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type registrationInviteCodeAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupRegistrationInviteCodeControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousQuotaForNewUser := common.QuotaForNewUser
	previousGenerateDefaultToken := constant.GenerateDefaultToken

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 999
	constant.GenerateDefaultToken = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.RegistrationInviteCode{},
		&model.Log{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		common.QuotaForNewUser = previousQuotaForNewUser
		constant.GenerateDefaultToken = previousGenerateDefaultToken
	})

	return db
}

func callRegisterWithInviteCode(t *testing.T, payload map[string]any) registrationInviteCodeAPIResponse {
	t.Helper()

	body, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Register(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response registrationInviteCodeAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestRegisterRequiresRegistrationInviteCode(t *testing.T) {
	db := setupRegistrationInviteCodeControllerTestDB(t)
	created, err := model.CreateRegistrationInviteCode(&model.RegistrationInviteCode{
		Code:             "required-code",
		Group:            "default",
		MaxRegistrations: 10,
	})
	require.NoError(t, err)

	response := callRegisterWithInviteCode(t, map[string]any{
		"username": "missing-invite",
		"password": "password123",
	})

	assert.False(t, response.Success)
	assert.NotEmpty(t, response.Message)
	var userCount int64
	require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
	assert.Zero(t, userCount)
	config, err := model.GetRegistrationInviteCodeByID(created.ID)
	require.NoError(t, err)
	assert.Zero(t, config.RegisteredCount)
}

func TestRegisterAppliesRegistrationInviteCodeProvisioning(t *testing.T) {
	db := setupRegistrationInviteCodeControllerTestDB(t)
	created, err := model.CreateRegistrationInviteCode(&model.RegistrationInviteCode{
		Code:             "vip-code",
		Group:            "vip",
		InitialQuota:     1234,
		MaxRegistrations: 2,
	})
	require.NoError(t, err)

	response := callRegisterWithInviteCode(t, map[string]any{
		"username":    "provisioned-user",
		"password":    "password123",
		"invite_code": " vip-code ",
	})

	assert.True(t, response.Success)
	var user model.User
	require.NoError(t, db.Where("username = ?", "provisioned-user").First(&user).Error)
	assert.Equal(t, "vip", user.Group)
	assert.Equal(t, 1234, user.Quota)
	config, err := model.GetRegistrationInviteCodeByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, config.RegisteredCount)
}

func TestRegisterRejectsExhaustedRegistrationInviteCode(t *testing.T) {
	db := setupRegistrationInviteCodeControllerTestDB(t)
	created, err := model.CreateRegistrationInviteCode(&model.RegistrationInviteCode{
		Code:             "single-use-code",
		Group:            "default",
		InitialQuota:     10,
		MaxRegistrations: 1,
	})
	require.NoError(t, err)

	firstResponse := callRegisterWithInviteCode(t, map[string]any{
		"username":    "first-user",
		"password":    "password123",
		"invite_code": "single-use-code",
	})
	require.True(t, firstResponse.Success)

	secondResponse := callRegisterWithInviteCode(t, map[string]any{
		"username":    "second-user",
		"password":    "password123",
		"invite_code": "single-use-code",
	})

	assert.False(t, secondResponse.Success)
	assert.NotEmpty(t, secondResponse.Message)
	var userCount int64
	require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
	assert.EqualValues(t, 1, userCount)
	config, err := model.GetRegistrationInviteCodeByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, config.RegisteredCount)
}

func TestRegistrationInviteCodeManagementPagination(t *testing.T) {
	setupRegistrationInviteCodeControllerTestDB(t)
	for _, payload := range []map[string]any{
		{
			"code":              "default-code",
			"group":             "default",
			"initial_quota":     10,
			"max_registrations": 5,
		},
		{
			"code":              "vip-code",
			"group":             "vip",
			"initial_quota":     20,
			"max_registrations": 10,
		},
	} {
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/registration/invite-codes", bytes.NewReader(body))
		CreateRegistrationInviteCode(ctx)
		require.Equal(t, http.StatusOK, recorder.Code)
		var response registrationInviteCodeAPIResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		require.True(t, response.Success, response.Message)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/registration/invite-codes?p=1&page_size=1&keyword=vip",
		nil,
	)
	ListRegistrationInviteCodes(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int                            `json:"page"`
			PageSize int                            `json:"page_size"`
			Total    int                            `json:"total"`
			Items    []model.RegistrationInviteCode `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Page)
	assert.Equal(t, 1, response.Data.PageSize)
	assert.Equal(t, 1, response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "vip-code", response.Data.Items[0].Code)
}
