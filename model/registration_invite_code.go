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
package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrRegistrationInviteCodeRequired      = errors.New("registration invitation code is required")
	ErrRegistrationInviteCodeNotConfigured = errors.New("registration invitation code is not configured")
	ErrRegistrationInviteCodeInvalid       = errors.New("registration invitation code is invalid")
	ErrRegistrationInviteCodeExpired       = errors.New("registration invitation code has expired")
	ErrRegistrationInviteCodeExhausted     = errors.New("registration invitation code registration limit reached")
	ErrRegistrationInviteCodeConfig        = errors.New("registration invitation code configuration is invalid")
	ErrRegistrationInviteCodeDuplicate     = errors.New("registration invitation code already exists")
)

type RegistrationInviteCode struct {
	ID               uint   `json:"id" gorm:"primaryKey"`
	Code             string `json:"code" gorm:"type:varchar(128);not null;uniqueIndex"`
	Group            string `json:"group" gorm:"type:varchar(64);not null"`
	Status           int    `json:"status" gorm:"type:int"`
	ExpiredTime      int64  `json:"expired_time" gorm:"type:bigint;not null"`
	InitialQuota     int    `json:"initial_quota" gorm:"type:int;not null"`
	MaxRegistrations int    `json:"max_registrations" gorm:"type:int;not null"`
	RegisteredCount  int    `json:"registered_count" gorm:"type:int;not null"`
	CreatedTime      int64  `json:"created_time" gorm:"type:bigint"`
	UpdatedTime      int64  `json:"updated_time" gorm:"type:bigint"`
}

func (code *RegistrationInviteCode) normalize() {
	code.Code = strings.TrimSpace(code.Code)
	code.Group = strings.TrimSpace(code.Group)
	if code.Group == "" {
		code.Group = "default"
	}
}

func validateRegistrationInviteCodeFields(code *RegistrationInviteCode) error {
	if code == nil {
		return fmt.Errorf("%w: missing configuration", ErrRegistrationInviteCodeConfig)
	}
	code.normalize()
	if code.Code == "" {
		return fmt.Errorf("%w: code is required", ErrRegistrationInviteCodeConfig)
	}
	if len([]rune(code.Code)) > 128 {
		return fmt.Errorf("%w: code must be at most 128 characters", ErrRegistrationInviteCodeConfig)
	}
	if len([]rune(code.Group)) > 64 {
		return fmt.Errorf("%w: group must be at most 64 characters", ErrRegistrationInviteCodeConfig)
	}
	if code.Status != common.RedemptionCodeStatusEnabled && code.Status != common.RedemptionCodeStatusDisabled {
		return fmt.Errorf("%w: unsupported status", ErrRegistrationInviteCodeConfig)
	}
	if code.ExpiredTime < 0 {
		return fmt.Errorf("%w: expiration time cannot be negative", ErrRegistrationInviteCodeConfig)
	}
	if code.InitialQuota < 0 || code.InitialQuota > common.MaxQuota {
		return fmt.Errorf("%w: initial quota is out of range", ErrRegistrationInviteCodeConfig)
	}
	if code.MaxRegistrations < 0 || code.MaxRegistrations > common.MaxQuota {
		return fmt.Errorf("%w: maximum registrations is out of range", ErrRegistrationInviteCodeConfig)
	}
	if code.RegisteredCount < 0 || code.RegisteredCount > common.MaxQuota {
		return fmt.Errorf("%w: registered count is out of range", ErrRegistrationInviteCodeConfig)
	}
	if code.MaxRegistrations > 0 && code.RegisteredCount > code.MaxRegistrations {
		return fmt.Errorf("%w: registered count exceeds maximum registrations", ErrRegistrationInviteCodeConfig)
	}
	return nil
}

func ValidateRegistrationInviteCodeConfig(code *RegistrationInviteCode) error {
	if code != nil && code.Status == 0 {
		code.Status = common.RedemptionCodeStatusEnabled
	}
	if err := validateRegistrationInviteCodeFields(code); err != nil {
		return err
	}
	if code.ExpiredTime != 0 && code.ExpiredTime < common.GetTimestamp() {
		return fmt.Errorf("%w: expiration time cannot be earlier than now", ErrRegistrationInviteCodeConfig)
	}
	return nil
}

func GetRegistrationInviteCodes(keyword, status string, startIdx, num int) ([]*RegistrationInviteCode, int64, error) {
	query := DB.Model(&RegistrationInviteCode{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR code LIKE ? OR "+commonGroupCol+" LIKE ?", id, "%"+keyword+"%", "%"+keyword+"%")
		} else {
			query = query.Where("code LIKE ? OR "+commonGroupCol+" LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	now := common.GetTimestamp()
	switch status {
	case "enabled":
		query = query.Where("status = ? AND (expired_time = 0 OR expired_time >= ?) AND (max_registrations = 0 OR registered_count < max_registrations)", common.RedemptionCodeStatusEnabled, now)
	case "disabled":
		query = query.Where("status = ?", common.RedemptionCodeStatusDisabled)
	case "expired":
		query = query.Where("status = ? AND expired_time != 0 AND expired_time < ?", common.RedemptionCodeStatusEnabled, now)
	case "exhausted":
		query = query.Where("status = ? AND max_registrations > 0 AND registered_count >= max_registrations", common.RedemptionCodeStatusEnabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var codes []*RegistrationInviteCode
	if err := query.Order("id DESC").Limit(num).Offset(startIdx).Find(&codes).Error; err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func GetRegistrationInviteCodeByID(id uint) (*RegistrationInviteCode, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: invalid id", ErrRegistrationInviteCodeConfig)
	}
	var code RegistrationInviteCode
	if err := DB.First(&code, id).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

func CreateRegistrationInviteCode(code *RegistrationInviteCode) (*RegistrationInviteCode, error) {
	if code == nil {
		return nil, fmt.Errorf("%w: missing configuration", ErrRegistrationInviteCodeConfig)
	}
	code.ID = 0
	code.RegisteredCount = 0
	code.Status = common.RedemptionCodeStatusEnabled
	if err := ValidateRegistrationInviteCodeConfig(code); err != nil {
		return nil, err
	}
	var count int64
	if err := DB.Model(&RegistrationInviteCode{}).Where("code = ?", code.Code).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrRegistrationInviteCodeDuplicate
	}
	now := common.GetTimestamp()
	code.CreatedTime = now
	code.UpdatedTime = now
	if err := DB.Create(code).Error; err != nil {
		return nil, err
	}
	return code, nil
}

func UpdateRegistrationInviteCode(code *RegistrationInviteCode) (*RegistrationInviteCode, error) {
	if code == nil || code.ID == 0 {
		return nil, fmt.Errorf("%w: invalid id", ErrRegistrationInviteCodeConfig)
	}
	code.Status = common.RedemptionCodeStatusEnabled
	if err := ValidateRegistrationInviteCodeConfig(code); err != nil {
		return nil, err
	}

	var saved RegistrationInviteCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&saved, code.ID).Error; err != nil {
			return err
		}
		var duplicateCount int64
		if err := tx.Model(&RegistrationInviteCode{}).
			Where("code = ? AND id != ?", code.Code, code.ID).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return ErrRegistrationInviteCodeDuplicate
		}
		registeredCount := saved.RegisteredCount
		if saved.Code != code.Code {
			registeredCount = 0
		}
		validated := *code
		validated.Status = saved.Status
		validated.RegisteredCount = registeredCount
		if err := ValidateRegistrationInviteCodeConfig(&validated); err != nil {
			return err
		}
		updates := map[string]interface{}{
			"code":              code.Code,
			"group":             code.Group,
			"expired_time":      code.ExpiredTime,
			"initial_quota":     code.InitialQuota,
			"max_registrations": code.MaxRegistrations,
			"registered_count":  registeredCount,
			"updated_time":      common.GetTimestamp(),
		}
		if err := tx.Model(&RegistrationInviteCode{}).Where("id = ?", code.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&saved, code.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func UpdateRegistrationInviteCodeStatus(id uint, status int) (*RegistrationInviteCode, error) {
	if id == 0 || (status != common.RedemptionCodeStatusEnabled && status != common.RedemptionCodeStatusDisabled) {
		return nil, fmt.Errorf("%w: invalid status update", ErrRegistrationInviteCodeConfig)
	}
	result := DB.Model(&RegistrationInviteCode{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"updated_time": common.GetTimestamp(),
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return GetRegistrationInviteCodeByID(id)
}

func DeleteRegistrationInviteCode(id uint) error {
	if id == 0 {
		return fmt.Errorf("%w: invalid id", ErrRegistrationInviteCodeConfig)
	}
	result := DB.Delete(&RegistrationInviteCode{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func registrationInviteCodeStateError(code *RegistrationInviteCode, now int64) error {
	if code.Status != common.RedemptionCodeStatusEnabled {
		return ErrRegistrationInviteCodeInvalid
	}
	if code.ExpiredTime != 0 && code.ExpiredTime < now {
		return ErrRegistrationInviteCodeExpired
	}
	if code.MaxRegistrations > 0 && code.RegisteredCount >= code.MaxRegistrations {
		return ErrRegistrationInviteCodeExhausted
	}
	return nil
}

// ReserveRegistrationInviteCode validates a code and increments its usage
// count in the same transaction as user creation. The conditional UPDATE is
// intentionally used in addition to lockForUpdate: SQLite skips FOR UPDATE,
// while the compare-and-swap predicate still prevents an over-limit commit.
func ReserveRegistrationInviteCode(tx *gorm.DB, code string) (*RegistrationInviteCode, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrRegistrationInviteCodeRequired
	}

	now := common.GetTimestamp()
	var current RegistrationInviteCode
	if err := tx.Where("code = ?", code).First(&current).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var count int64
		if countErr := tx.Model(&RegistrationInviteCode{}).Count(&count).Error; countErr != nil {
			return nil, countErr
		}
		if count == 0 {
			return nil, ErrRegistrationInviteCodeNotConfigured
		}
		return nil, ErrRegistrationInviteCodeInvalid
	}
	if err := validateRegistrationInviteCodeFields(&current); err != nil {
		return nil, err
	}
	if err := registrationInviteCodeStateError(&current, now); err != nil {
		return nil, err
	}

	result := tx.Model(&RegistrationInviteCode{}).
		Where("id = ? AND code = ? AND status = ? AND (expired_time = 0 OR expired_time >= ?) AND (max_registrations = 0 OR registered_count < max_registrations)",
			current.ID, code, common.RedemptionCodeStatusEnabled, now).
		UpdateColumns(map[string]interface{}{
			"registered_count": gorm.Expr("registered_count + ?", 1),
			"updated_time":     now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := lockForUpdate(tx).First(&current, current.ID).Error; err != nil {
			return nil, err
		}
		if err := registrationInviteCodeStateError(&current, now); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: concurrent update", ErrRegistrationInviteCodeConfig)
	}

	if err := lockForUpdate(tx).First(&current, current.ID).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
