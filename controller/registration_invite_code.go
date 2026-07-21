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
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type registrationInviteCodeRequest struct {
	Code             string `json:"code"`
	Group            string `json:"group"`
	ExpiredTime      int64  `json:"expired_time"`
	InitialQuota     int    `json:"initial_quota"`
	MaxRegistrations int    `json:"max_registrations"`
}

type registrationInviteCodeStatusRequest struct {
	Status int `json:"status"`
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

func parseRegistrationInviteCodeID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	return uint(id), true
}

func ListRegistrationInviteCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	codes, total, err := model.GetRegistrationInviteCodes(
		c.Query("keyword"),
		c.Query("status"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(codes)
	common.ApiSuccess(c, pageInfo)
}

func GetRegistrationInviteCode(c *gin.Context) {
	id, ok := parseRegistrationInviteCodeID(c)
	if !ok {
		return
	}
	code, err := model.GetRegistrationInviteCodeByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, code)
}

func CreateRegistrationInviteCode(c *gin.Context) {
	var request registrationInviteCodeRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	created, err := model.CreateRegistrationInviteCode(&model.RegistrationInviteCode{
		Code:             request.Code,
		Group:            request.Group,
		ExpiredTime:      request.ExpiredTime,
		InitialQuota:     request.InitialQuota,
		MaxRegistrations: request.MaxRegistrations,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_invite_code.create", map[string]interface{}{
		"id":                created.ID,
		"group":             created.Group,
		"expired_time":      created.ExpiredTime,
		"initial_quota":     created.InitialQuota,
		"max_registrations": created.MaxRegistrations,
	})
	common.ApiSuccess(c, created)
}

func UpdateRegistrationInviteCode(c *gin.Context) {
	id, ok := parseRegistrationInviteCodeID(c)
	if !ok {
		return
	}
	var request registrationInviteCodeRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	updated, err := model.UpdateRegistrationInviteCode(&model.RegistrationInviteCode{
		ID:               id,
		Code:             request.Code,
		Group:            request.Group,
		ExpiredTime:      request.ExpiredTime,
		InitialQuota:     request.InitialQuota,
		MaxRegistrations: request.MaxRegistrations,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_invite_code.update", map[string]interface{}{
		"id":                updated.ID,
		"group":             updated.Group,
		"expired_time":      updated.ExpiredTime,
		"initial_quota":     updated.InitialQuota,
		"max_registrations": updated.MaxRegistrations,
	})
	common.ApiSuccess(c, updated)
}

func UpdateRegistrationInviteCodeStatus(c *gin.Context) {
	id, ok := parseRegistrationInviteCodeID(c)
	if !ok {
		return
	}
	var request registrationInviteCodeStatusRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	updated, err := model.UpdateRegistrationInviteCodeStatus(id, request.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_invite_code.status", map[string]interface{}{
		"id":     updated.ID,
		"status": updated.Status,
	})
	common.ApiSuccess(c, updated)
}

func DeleteRegistrationInviteCode(c *gin.Context) {
	id, ok := parseRegistrationInviteCodeID(c)
	if !ok {
		return
	}
	if err := model.DeleteRegistrationInviteCode(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration_invite_code.delete", map[string]interface{}{
		"id": id,
	})
	common.ApiSuccess(c, nil)
}
