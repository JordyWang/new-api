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

export const registrationInviteCodeSchema = z.object({
  id: z.number(),
  code: z.string(),
  group: z.string(),
  status: z.number(),
  expired_time: z.number(),
  initial_quota: z.number(),
  max_registrations: z.number(),
  registered_count: z.number(),
  created_time: z.number(),
  updated_time: z.number(),
})

export type RegistrationInviteCode = z.infer<
  typeof registrationInviteCodeSchema
>

export type RegistrationInviteCodePayload = {
  code: string
  group: string
  expired_time: number
  initial_quota: number
  max_registrations: number
}

export type RegistrationInviteCodePage = {
  page: number
  page_size: number
  total: number
  items: RegistrationInviteCode[]
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type RegistrationInviteCodesDialog = 'create' | 'update' | 'delete'
