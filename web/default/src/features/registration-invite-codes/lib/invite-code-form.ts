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
import { z } from 'zod'

import type {
  RegistrationInviteCode,
  RegistrationInviteCodePayload,
} from '../types'

const MAX_INT32 = 2_147_483_647

export const registrationInviteCodeFormSchema = z.object({
  code: z.string().trim().min(1).max(128),
  group: z.string().trim().min(1).max(64),
  expiredTime: z.date().optional(),
  initialQuota: z.number().int().min(0).max(MAX_INT32),
  maxRegistrations: z.number().int().min(0).max(MAX_INT32),
})

export type RegistrationInviteCodeFormValues = z.infer<
  typeof registrationInviteCodeFormSchema
>

export const REGISTRATION_INVITE_CODE_FORM_DEFAULTS: RegistrationInviteCodeFormValues =
  {
    code: '',
    group: 'default',
    expiredTime: undefined,
    initialQuota: 0,
    maxRegistrations: 0,
  }

export function inviteCodeToFormValues(
  code: RegistrationInviteCode
): RegistrationInviteCodeFormValues {
  return {
    code: code.code,
    group: code.group,
    expiredTime:
      code.expired_time > 0 ? new Date(code.expired_time * 1000) : undefined,
    initialQuota: code.initial_quota,
    maxRegistrations: code.max_registrations,
  }
}

export function inviteCodeFormToPayload(
  values: RegistrationInviteCodeFormValues
): RegistrationInviteCodePayload {
  return {
    code: values.code.trim(),
    group: values.group.trim(),
    expired_time: values.expiredTime
      ? Math.floor(values.expiredTime.getTime() / 1000)
      : 0,
    initial_quota: values.initialQuota,
    max_registrations: values.maxRegistrations,
  }
}
