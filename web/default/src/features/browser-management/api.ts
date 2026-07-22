/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { t } from 'i18next'

import { api, type ApiRequestConfig } from '@/lib/api'

import type {
  BrowserAgent,
  BrowserAgentInput,
  BrowserAgentTokenResult,
  BrowserFingerprint,
  BrowserFingerprintInput,
  BrowserProfile,
  BrowserProfileChannel,
  BrowserProfileInput,
  BrowserLaunch,
  BrowserProxy,
  BrowserProxyInput,
} from './types'

type ApiEnvelope<T = undefined> = {
  success: boolean
  message?: string
  data?: T
}

const mutationConfig: ApiRequestConfig = {
  skipBusinessError: true,
  skipErrorHandler: true,
}

export const browserManagementQueryKeys = {
  all: ['browser-management'] as const,
  agents: () => [...browserManagementQueryKeys.all, 'agents'] as const,
  proxies: () => [...browserManagementQueryKeys.all, 'proxies'] as const,
  fingerprints: () =>
    [...browserManagementQueryKeys.all, 'fingerprints'] as const,
  profiles: () => [...browserManagementQueryKeys.all, 'profiles'] as const,
  profileChannels: () =>
    [...browserManagementQueryKeys.all, 'profile-channels'] as const,
}

function unwrap<T>(response: ApiEnvelope<T>, fallbackMessage: string): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || fallbackMessage)
  }
  return response.data
}

function assertSuccess(response: ApiEnvelope, fallbackMessage: string): void {
  if (!response.success) {
    throw new Error(response.message || fallbackMessage)
  }
}

export async function listBrowserAgents(): Promise<BrowserAgent[]> {
  const response = await api.get<ApiEnvelope<BrowserAgent[]>>(
    '/api/browser/agents'
  )
  return unwrap(response.data, t('Failed to load browser agents'))
}

export async function createBrowserAgent(
  input: BrowserAgentInput
): Promise<BrowserAgentTokenResult> {
  const response = await api.post<ApiEnvelope<BrowserAgentTokenResult>>(
    '/api/browser/agents',
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to create browser agent'))
}

export async function updateBrowserAgent(
  id: number,
  input: BrowserAgentInput
): Promise<BrowserAgent> {
  const response = await api.put<ApiEnvelope<BrowserAgent>>(
    `/api/browser/agents/${id}`,
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to update browser agent'))
}

export async function rotateBrowserAgentToken(
  id: number
): Promise<BrowserAgentTokenResult> {
  const response = await api.post<ApiEnvelope<BrowserAgentTokenResult>>(
    `/api/browser/agents/${id}/rotate-token`,
    undefined,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to rotate browser agent token'))
}

export async function deleteBrowserAgent(id: number): Promise<void> {
  const response = await api.delete<ApiEnvelope>(
    `/api/browser/agents/${id}`,
    mutationConfig
  )
  assertSuccess(response.data, t('Failed to delete browser agent'))
}

export async function listBrowserProxies(): Promise<BrowserProxy[]> {
  const response = await api.get<ApiEnvelope<BrowserProxy[]>>(
    '/api/browser/proxies'
  )
  return unwrap(response.data, t('Failed to load managed proxies'))
}

export async function createBrowserProxy(
  input: BrowserProxyInput
): Promise<BrowserProxy> {
  const response = await api.post<ApiEnvelope<BrowserProxy>>(
    '/api/browser/proxies',
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to create managed proxy'))
}

export async function updateBrowserProxy(
  id: number,
  input: BrowserProxyInput
): Promise<BrowserProxy> {
  const response = await api.put<ApiEnvelope<BrowserProxy>>(
    `/api/browser/proxies/${id}`,
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to update managed proxy'))
}

export async function deleteBrowserProxy(id: number): Promise<void> {
  const response = await api.delete<ApiEnvelope>(
    `/api/browser/proxies/${id}`,
    mutationConfig
  )
  assertSuccess(response.data, t('Failed to delete managed proxy'))
}

export async function listBrowserFingerprints(): Promise<BrowserFingerprint[]> {
  const response = await api.get<ApiEnvelope<BrowserFingerprint[]>>(
    '/api/browser/fingerprints'
  )
  return unwrap(response.data, t('Failed to load browser fingerprints'))
}

export async function createBrowserFingerprint(
  input: BrowserFingerprintInput
): Promise<BrowserFingerprint> {
  const response = await api.post<ApiEnvelope<BrowserFingerprint>>(
    '/api/browser/fingerprints',
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to create browser fingerprint'))
}

export async function updateBrowserFingerprint(
  id: number,
  input: BrowserFingerprintInput
): Promise<BrowserFingerprint> {
  const response = await api.put<ApiEnvelope<BrowserFingerprint>>(
    `/api/browser/fingerprints/${id}`,
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to update browser fingerprint'))
}

export async function deleteBrowserFingerprint(id: number): Promise<void> {
  const response = await api.delete<ApiEnvelope>(
    `/api/browser/fingerprints/${id}`,
    mutationConfig
  )
  assertSuccess(response.data, t('Failed to delete browser fingerprint'))
}

export async function listBrowserProfiles(): Promise<BrowserProfile[]> {
  const response = await api.get<ApiEnvelope<BrowserProfile[]>>(
    '/api/browser/profiles'
  )
  return unwrap(response.data, t('Failed to load browser profiles'))
}

export async function listBrowserProfileChannels(): Promise<
  BrowserProfileChannel[]
> {
  const response = await api.get<ApiEnvelope<BrowserProfileChannel[]>>(
    '/api/browser/profile-channels'
  )
  return unwrap(response.data, t('Failed to load Codex channels'))
}

export async function launchBrowserProfile(id: number): Promise<BrowserLaunch> {
  const response = await api.post<ApiEnvelope<BrowserLaunch>>(
    `/api/browser/profiles/${id}/launch`,
    undefined,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to open browser profile'))
}

export async function cancelBrowserLaunch(launchId: string): Promise<void> {
  const response = await api.post<ApiEnvelope>(
    `/api/browser/launches/${encodeURIComponent(launchId)}/cancel`,
    undefined,
    mutationConfig
  )
  assertSuccess(response.data, t('Failed to stop browser profile'))
}

export async function createBrowserProfile(
  input: BrowserProfileInput
): Promise<BrowserProfile> {
  const response = await api.post<ApiEnvelope<BrowserProfile>>(
    '/api/browser/profiles',
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to create browser profile'))
}

export async function updateBrowserProfile(
  id: number,
  input: BrowserProfileInput
): Promise<BrowserProfile> {
  const response = await api.put<ApiEnvelope<BrowserProfile>>(
    `/api/browser/profiles/${id}`,
    input,
    mutationConfig
  )
  return unwrap(response.data, t('Failed to update browser profile'))
}

export async function resetBrowserProfile(id: number): Promise<void> {
  const response = await api.post<ApiEnvelope>(
    `/api/browser/profiles/${id}/reset`,
    undefined,
    mutationConfig
  )
  if (!response.data.success) {
    throw new Error(
      response.data.message || t('Failed to reset browser profile')
    )
  }
}

export async function deleteBrowserProfile(id: number): Promise<void> {
  const response = await api.delete<ApiEnvelope>(
    `/api/browser/profiles/${id}`,
    mutationConfig
  )
  assertSuccess(response.data, t('Failed to delete browser profile'))
}
