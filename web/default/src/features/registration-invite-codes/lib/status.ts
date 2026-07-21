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
import type { RegistrationInviteCode } from '../types'

export const INVITE_CODE_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export type InviteCodeDisplayStatus =
  | 'enabled'
  | 'disabled'
  | 'expired'
  | 'exhausted'

export function getInviteCodeDisplayStatus(
  code: RegistrationInviteCode
): InviteCodeDisplayStatus {
  if (code.status === INVITE_CODE_STATUS.DISABLED) return 'disabled'
  if (code.expired_time > 0 && code.expired_time < Date.now() / 1000) {
    return 'expired'
  }
  if (
    code.max_registrations > 0 &&
    code.registered_count >= code.max_registrations
  ) {
    return 'exhausted'
  }
  return 'enabled'
}
