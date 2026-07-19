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
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const RegistrationInviteCodeID uint = 1

var (
	ErrRegistrationInviteCodeRequired      = errors.New("registration invitation code is required")
	ErrRegistrationInviteCodeNotConfigured = errors.New("registration invitation code is not configured")
	ErrRegistrationInviteCodeInvalid       = errors.New("registration invitation code is invalid")
	ErrRegistrationInviteCodeExpired       = errors.New("registration invitation code has expired")
	ErrRegistrationInviteCodeExhausted     = errors.New("registration invitation code registration limit reached")
	ErrRegistrationInviteCodeConfig        = errors.New("registration invitation code configuration is invalid")
)

// RegistrationInviteCode is the singleton invitation-code policy used for
// public registration. ID is fixed to RegistrationInviteCodeID so updating
// the policy cannot accidentally create multiple active policies.
type RegistrationInviteCode struct {
	ID               uint   `json:"id" gorm:"primaryKey"`
	Code             string `json:"code" gorm:"type:varchar(128);not null"`
	Group            string `json:"group" gorm:"type:varchar(64);not null"`
	ExpiredTime      int64  `json:"expired_time" gorm:"type:bigint;not null"`
	InitialQuota     int    `json:"initial_quota" gorm:"type:int;not null"`
	MaxRegistrations int    `json:"max_registrations" gorm:"type:int;not null"`
	RegisteredCount  int    `json:"registered_count" gorm:"type:int;not null"`
}

func (config *RegistrationInviteCode) normalize() {
	config.Code = strings.TrimSpace(config.Code)
	config.Group = strings.TrimSpace(config.Group)
	if config.Group == "" {
		config.Group = "default"
	}
}

func validateRegistrationInviteCodeFields(config *RegistrationInviteCode) error {
	if config == nil {
		return fmt.Errorf("%w: missing configuration", ErrRegistrationInviteCodeConfig)
	}
	config.normalize()
	if len([]rune(config.Code)) > 128 {
		return fmt.Errorf("%w: code must be at most 128 characters", ErrRegistrationInviteCodeConfig)
	}
	if len([]rune(config.Group)) > 64 {
		return fmt.Errorf("%w: group must be at most 64 characters", ErrRegistrationInviteCodeConfig)
	}
	if config.ExpiredTime < 0 {
		return fmt.Errorf("%w: expiration time cannot be negative", ErrRegistrationInviteCodeConfig)
	}
	if config.InitialQuota < 0 || config.InitialQuota > common.MaxQuota {
		return fmt.Errorf("%w: initial quota is out of range", ErrRegistrationInviteCodeConfig)
	}
	if config.MaxRegistrations < 0 || config.MaxRegistrations > common.MaxQuota {
		return fmt.Errorf("%w: maximum registrations is out of range", ErrRegistrationInviteCodeConfig)
	}
	if config.RegisteredCount < 0 || config.RegisteredCount > common.MaxQuota {
		return fmt.Errorf("%w: registered count is out of range", ErrRegistrationInviteCodeConfig)
	}
	if config.MaxRegistrations > 0 && config.RegisteredCount > config.MaxRegistrations {
		return fmt.Errorf("%w: registered count exceeds maximum registrations", ErrRegistrationInviteCodeConfig)
	}
	return nil
}

// ValidateRegistrationInviteCodeConfig validates administrator-controlled
// values before they are persisted. A blank code deliberately disables public
// registration because the registration endpoint always requires a code.
func ValidateRegistrationInviteCodeConfig(config *RegistrationInviteCode) error {
	if err := validateRegistrationInviteCodeFields(config); err != nil {
		return err
	}
	if config.ExpiredTime != 0 && config.ExpiredTime < common.GetTimestamp() {
		return fmt.Errorf("%w: expiration time cannot be earlier than now", ErrRegistrationInviteCodeConfig)
	}
	return nil
}

func GetRegistrationInviteCode() (*RegistrationInviteCode, error) {
	config := &RegistrationInviteCode{
		ID:    RegistrationInviteCodeID,
		Group: "default",
	}
	if err := DB.First(config, RegistrationInviteCodeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return config, nil
		}
		return nil, err
	}
	return config, nil
}

// SaveRegistrationInviteCode updates the singleton policy. The usage counter
// is server-owned: changing the code starts a new campaign, while editing the
// other fields preserves the current usage count.
func SaveRegistrationInviteCode(config *RegistrationInviteCode) (*RegistrationInviteCode, error) {
	if err := ValidateRegistrationInviteCodeConfig(config); err != nil {
		return nil, err
	}

	var saved RegistrationInviteCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current RegistrationInviteCode
		err := lockForUpdate(tx).First(&current, RegistrationInviteCodeID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.ID = RegistrationInviteCodeID
			config.RegisteredCount = 0
			if err := tx.Create(config).Error; err != nil {
				return err
			}
			saved = *config
			return nil
		}
		if err != nil {
			return err
		}

		config.ID = RegistrationInviteCodeID
		if config.Code == current.Code {
			config.RegisteredCount = current.RegisteredCount
		} else {
			config.RegisteredCount = 0
		}
		if err := ValidateRegistrationInviteCodeConfig(config); err != nil {
			return err
		}
		if err := tx.Model(&RegistrationInviteCode{}).
			Where("id = ?", RegistrationInviteCodeID).
			Select("code", "group", "expired_time", "initial_quota", "max_registrations", "registered_count").
			Updates(config).Error; err != nil {
			return err
		}
		saved = *config
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func registrationInviteCodeStateError(config *RegistrationInviteCode, code string, now int64) error {
	if config.Code == "" {
		return ErrRegistrationInviteCodeNotConfigured
	}
	if config.Code != code {
		return ErrRegistrationInviteCodeInvalid
	}
	if config.ExpiredTime != 0 && config.ExpiredTime < now {
		return ErrRegistrationInviteCodeExpired
	}
	if config.MaxRegistrations > 0 && config.RegisteredCount >= config.MaxRegistrations {
		return ErrRegistrationInviteCodeExhausted
	}
	return fmt.Errorf("%w: concurrent update", ErrRegistrationInviteCodeConfig)
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
	if err := tx.First(&current, RegistrationInviteCodeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRegistrationInviteCodeNotConfigured
		}
		return nil, err
	}
	if err := validateRegistrationInviteCodeFields(&current); err != nil {
		return nil, err
	}

	result := tx.Model(&RegistrationInviteCode{}).
		Where("id = ? AND code = ? AND (expired_time = 0 OR expired_time >= ?) AND (max_registrations = 0 OR registered_count < max_registrations)",
			RegistrationInviteCodeID, code, now).
		UpdateColumn("registered_count", gorm.Expr("registered_count + ?", 1))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := lockForUpdate(tx).First(&current, RegistrationInviteCodeID).Error; err != nil {
			return nil, err
		}
		return nil, registrationInviteCodeStateError(&current, code, now)
	}

	if err := lockForUpdate(tx).First(&current, RegistrationInviteCodeID).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
