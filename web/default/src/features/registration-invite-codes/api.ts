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
import { api } from '@/lib/api'

import type {
  ApiResponse,
  RegistrationInviteCode,
  RegistrationInviteCodePage,
  RegistrationInviteCodePayload,
} from './types'

export async function getRegistrationInviteCodes(params: {
  page: number
  pageSize: number
  keyword?: string
  status?: string
}): Promise<ApiResponse<RegistrationInviteCodePage>> {
  const response = await api.get('/api/registration/invite-codes', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword,
      status: params.status,
    },
  })
  return response.data
}

export async function getRegistrationInviteCode(
  id: number
): Promise<ApiResponse<RegistrationInviteCode>> {
  const response = await api.get(`/api/registration/invite-codes/${id}`)
  return response.data
}

export async function createRegistrationInviteCode(
  payload: RegistrationInviteCodePayload
): Promise<ApiResponse<RegistrationInviteCode>> {
  const response = await api.post('/api/registration/invite-codes', payload)
  return response.data
}

export async function updateRegistrationInviteCode(
  id: number,
  payload: RegistrationInviteCodePayload
): Promise<ApiResponse<RegistrationInviteCode>> {
  const response = await api.put(
    `/api/registration/invite-codes/${id}`,
    payload
  )
  return response.data
}

export async function updateRegistrationInviteCodeStatus(
  id: number,
  status: number
): Promise<ApiResponse<RegistrationInviteCode>> {
  const response = await api.patch(
    `/api/registration/invite-codes/${id}/status`,
    { status }
  )
  return response.data
}

export async function deleteRegistrationInviteCode(
  id: number
): Promise<ApiResponse> {
  const response = await api.delete(`/api/registration/invite-codes/${id}`)
  return response.data
}
