/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type registrationInviteCodeUpdateRequest struct {
	Code             string `json:"code"`
	Group            string `json:"group"`
	ExpiredTime      int64  `json:"expired_time"`
	InitialQuota     int    `json:"initial_quota"`
	MaxRegistrations int    `json:"max_registrations"`
}

func registrationInviteCodeErrorKey(err error) string {
	switch {
	case errors.Is(err, model.ErrRegistrationInviteCodeRequired):
		return i18n.MsgUserRegistrationInviteRequired
	case errors.Is(err, model.ErrRegistrationInviteCodeNotConfigured):
		return i18n.MsgUserRegistrationInviteNotConfigured
	case errors.Is(err, model.ErrRegistrationInviteCodeInvalid):
		return i18n.MsgUserRegistrationInviteInvalid
	case errors.Is(err, model.ErrRegistrationInviteCodeExpired):
		return i18n.MsgUserRegistrationInviteExpired
	case errors.Is(err, model.ErrRegistrationInviteCodeExhausted):
		return i18n.MsgUserRegistrationInviteExhausted
	default:
		return ""
	}
}

func writeRegistrationInviteCodeError(c *gin.Context, err error) bool {
	if key := registrationInviteCodeErrorKey(err); key != "" {
		common.ApiErrorI18n(c, key)
		return true
	}
	return false
}

func GetRegistrationInviteCode(c *gin.Context) {
	config, err := model.GetRegistrationInviteCode()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func UpdateRegistrationInviteCode(c *gin.Context) {
	var request registrationInviteCodeUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	config := &model.RegistrationInviteCode{
		Code:             request.Code,
		Group:            request.Group,
		ExpiredTime:      request.ExpiredTime,
		InitialQuota:     request.InitialQuota,
		MaxRegistrations: request.MaxRegistrations,
	}
	updated, err := model.SaveRegistrationInviteCode(config)
	if err != nil {
		if errors.Is(err, model.ErrRegistrationInviteCodeConfig) {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		common.ApiError(c, err)
		return
	}

	recordManageAudit(c, "registration_invite_code.update", map[string]interface{}{
		"group":             updated.Group,
		"expired_time":      updated.ExpiredTime,
		"initial_quota":     updated.InitialQuota,
		"max_registrations": updated.MaxRegistrations,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updated,
	})
}
