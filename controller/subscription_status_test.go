package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSubscriptionStatusTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.User{},
		&model.UserSubscription{},
		&model.SubscriptionOrder{},
		&model.Log{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func disablePaymentComplianceForSubscriptionTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
}

func newSubscriptionStatusContext(t *testing.T, planID int, enabled bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := common.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/subscription/admin/plans/"+strconv.Itoa(planID), bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(planID)}}
	return ctx, recorder
}

func TestAdminUpdateSubscriptionPlanStatusAllowsDisableWithoutCompliance(t *testing.T) {
	db := openSubscriptionStatusTestDB(t)
	plan := &model.SubscriptionPlan{Id: 1, Title: "Pro", Enabled: true, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, db.Create(plan).Error)
	disablePaymentComplianceForSubscriptionTest(t)

	ctx, recorder := newSubscriptionStatusContext(t, plan.Id, false)
	AdminUpdateSubscriptionPlanStatus(ctx)

	var updated model.SubscriptionPlan
	require.NoError(t, db.First(&updated, plan.Id).Error)
	assert.False(t, updated.Enabled)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
}

func TestAdminUpdateSubscriptionPlanStatusAllowsEnableWithoutCompliance(t *testing.T) {
	db := openSubscriptionStatusTestDB(t)
	plan := &model.SubscriptionPlan{Id: 1, Title: "Pro", Enabled: false, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)
	disablePaymentComplianceForSubscriptionTest(t)

	ctx, recorder := newSubscriptionStatusContext(t, plan.Id, true)
	AdminUpdateSubscriptionPlanStatus(ctx)

	var unchanged model.SubscriptionPlan
	require.NoError(t, db.First(&unchanged, plan.Id).Error)
	assert.True(t, unchanged.Enabled)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
}

func TestGetSubscriptionPlansReturnsEnabledPlansWithoutCompliance(t *testing.T) {
	db := openSubscriptionStatusTestDB(t)
	enabledPlan := &model.SubscriptionPlan{Id: 101, Title: "Visible", Enabled: true, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	disabledPlan := &model.SubscriptionPlan{Id: 102, Title: "Hidden", Enabled: true, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, db.Create(enabledPlan).Error)
	require.NoError(t, db.Create(disabledPlan).Error)
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("id = ?", disabledPlan.Id).Update("enabled", false).Error)
	disablePaymentComplianceForSubscriptionTest(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/plans", nil)
	GetSubscriptionPlans(ctx)

	var response struct {
		Success bool                  `json:"success"`
		Data    []SubscriptionPlanDTO `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, enabledPlan.Id, response.Data[0].Plan.Id)
}

func TestAdminCreateSubscriptionPlanAllowsWithoutCompliance(t *testing.T) {
	db := openSubscriptionStatusTestDB(t)
	disablePaymentComplianceForSubscriptionTest(t)

	payload := AdminUpsertSubscriptionPlanRequest{Plan: model.SubscriptionPlan{
		Title:         "Internal",
		Enabled:       true,
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
	}}
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/admin/plans", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	AdminCreateSubscriptionPlan(ctx)

	var count int64
	require.NoError(t, db.Model(&model.SubscriptionPlan{}).Where("title = ?", payload.Plan.Title).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
}

func TestSubscriptionBalancePayAllowsWithoutCompliance(t *testing.T) {
	db := openSubscriptionStatusTestDB(t)
	user := &model.User{Id: 201, Username: "balance-user", Password: "password", Group: "default"}
	plan := &model.SubscriptionPlan{Id: 202, Title: "Free Internal", Enabled: true, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(plan).Error)
	disablePaymentComplianceForSubscriptionTest(t)

	payload, err := common.Marshal(SubscriptionBalancePayRequest{PlanId: plan.Id})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/balance/pay", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", user.Id)
	SubscriptionRequestBalancePay(ctx)

	var count int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
}

func TestExternalSubscriptionPaymentGatewaysStillRequireCompliance(t *testing.T) {
	disablePaymentComplianceForSubscriptionTest(t)
	handlers := map[string]gin.HandlerFunc{
		"stripe":         SubscriptionRequestStripePay,
		"creem":          SubscriptionRequestCreemPay,
		"epay":           SubscriptionRequestEpay,
		"waffo-pancake": SubscriptionRequestWaffoPancakePay,
	}

	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/"+name+"/pay", nil)
			handler(ctx)

			var response struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.False(t, response.Success)
		})
	}
}
