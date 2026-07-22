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

export type BrowserAgent = {
  id: number
  name: string
  enabled: boolean
  status: 'online' | 'offline'
  online: boolean
  last_seen_at: number
  version: string
  platform: string
  arch: string
  runtimes: string[]
  metadata: Record<string, unknown>
  created_at: number
  updated_at: number
}

export type BrowserProxy = {
  id: number
  name: string
  scheme: 'http' | 'https' | 'socks5' | 'socks5h'
  url_masked: string
  has_credentials: boolean
  enabled: boolean
  max_channel_accounts: number
  channel_account_count: number
  profile_count: number
  created_at: number
  updated_at: number
}

export type BrowserFingerprint = {
  id: number
  name: string
  user_agent: string
  viewport_width: number
  viewport_height: number
  payload: string
  launch_args: string
  environment: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export type BrowserProfile = {
  id: number
  name: string
  channel_id: number | null
  channel_name: string
  agent_id: number
  proxy_id: number
  fingerprint_id: number
  runtime_key: string
  persistent: boolean
  enabled: boolean
  created_at: number
  updated_at: number
  agent_name: string
  agent_online: boolean
  agent_runtimes: string[]
  proxy_name: string
  proxy_max_channel_accounts: number
  proxy_channel_account_count: number
  proxy_profile_count: number
  proxy_at_capacity: boolean
  fingerprint_name: string
  active_launch_id: string
  active_launch_status: BrowserLaunchStatus | ''
}

export type BrowserProfileChannel = {
  id: number
  name: string
  browser_proxy_id: number
  browser_profile_id: number | null
}

export type BrowserLaunchStatus =
  | 'pending'
  | 'claimed'
  | 'running'
  | 'completed'
  | 'failed'
  | 'canceled'
  | 'expired'

export type BrowserLaunch = {
  id: string
  status: BrowserLaunchStatus
  profile_id: number
  profile_name: string
  channel_id: number
  channel_name: string
  agent_id: number
  agent_name: string
  agent_online: boolean
  expires_at: number
  created_at: number
  running_at: number
  completed_at: number
  error_message: string
}

export type BrowserAgentInput = {
  name: string
  enabled: boolean
}

export type BrowserProxyInput = {
  name: string
  url: string
  enabled: boolean
  max_channel_accounts: number
}

export type BrowserFingerprintInput = {
  name: string
  user_agent: string
  viewport_width: number
  viewport_height: number
  payload: string
  launch_args: string
  environment: string
  enabled: boolean
}

export type BrowserProfileInput = {
  name: string
  channel_id: number | null
  agent_id: number
  proxy_id: number
  fingerprint_id: number
  auto_generate_fingerprint: boolean
  runtime_key: string
  persistent: boolean
  enabled: boolean
}

export type BrowserAgentTokenResult = {
  token: string
  agent?: BrowserAgent
}
