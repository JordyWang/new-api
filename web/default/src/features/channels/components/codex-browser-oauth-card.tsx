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
import { useMutation, useQuery } from '@tanstack/react-query'
import {
  Bot,
  CheckCircle2,
  CircleAlert,
  ExternalLink,
  Loader2,
  LogIn,
  RefreshCw,
  ShieldCheck,
  XCircle,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  cancelCodexBrowserOAuth,
  getBrowserOAuthProfiles,
  getBrowserOAuthRuntimes,
  getCodexBrowserOAuth,
  getManagedProxies,
  startCodexBrowserOAuth,
} from '../api'
import type {
  BrowserOAuthProfile,
  BrowserRuntimeOption,
  CodexBrowserOAuthFlow,
  CodexBrowserOAuthFlowStatus,
  ManagedProxy,
} from '../types'

const ACTIVE_FLOW_STATUSES = new Set<CodexBrowserOAuthFlowStatus>([
  'pending',
  'claimed',
  'running',
])
const EMPTY_PROFILES: BrowserOAuthProfile[] = []
const EMPTY_RUNTIMES: BrowserRuntimeOption[] = []
const EMPTY_PROXIES: ManagedProxy[] = []
const AUTOMATIC_PROFILE_VALUE = 'automatic'

type CodexBrowserOAuthCardProps = {
  open: boolean
  channelId: number | null
  isEditing: boolean
  disabled: boolean
  flowId: string
  channelName: string
  managedProxyId: number
  onManagedProxyChange: (proxyId: number) => void
  onCompleted: (flow: CodexBrowserOAuthFlow) => void
  onFlowReset: () => void
}

function requireData<T>(
  response: { success: boolean; message?: string; data?: T },
  fallback: string
): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || fallback)
  }
  return response.data
}

function formatTime(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

export function CodexBrowserOAuthCard(props: CodexBrowserOAuthCardProps) {
  const { t } = useTranslation()
  const onCompleted = props.onCompleted
  const onFlowReset = props.onFlowReset
  const onManagedProxyChange = props.onManagedProxyChange
  const isEditing = props.isEditing
  const [autoGenerateProfile, setAutoGenerateProfile] = useState(
    !props.isEditing
  )
  const [selectedProfileId, setSelectedProfileId] = useState(0)
  const [selectedRuntimeValue, setSelectedRuntimeValue] = useState('')
  const [selectedProxyId, setSelectedProxyId] = useState(0)
  const [activeFlowId, setActiveFlowId] = useState<string | null>(
    props.flowId || null
  )
  const completedFlowRef = useRef<string | null>(null)
  const wasOpenRef = useRef(props.open)

  const profilesQuery = useQuery({
    queryKey: ['browser-oauth', 'profiles', props.channelId ?? 0],
    queryFn: async () => {
      const response = await getBrowserOAuthProfiles(props.channelId)
      return requireData(response, t('Failed to load browser profiles'))
    },
    enabled: props.open && !props.disabled,
    staleTime: 15_000,
  })
  const profiles = profilesQuery.data ?? EMPTY_PROFILES
  const selectableProfiles = useMemo(
    () => profiles.filter((profile) => !profile.proxy_at_capacity),
    [profiles]
  )

  const runtimesQuery = useQuery({
    queryKey: ['browser-oauth', 'runtimes'],
    queryFn: async () => {
      const response = await getBrowserOAuthRuntimes()
      return requireData(response, t('Failed to load browser runtimes'))
    },
    enabled: props.open && !props.disabled && !props.isEditing,
    staleTime: 15_000,
  })
  const runtimeOptions = runtimesQuery.data ?? EMPTY_RUNTIMES

  const proxiesQuery = useQuery({
    queryKey: ['browser-oauth', 'proxies'],
    queryFn: async () => {
      const response = await getManagedProxies()
      return requireData(response, t('Failed to load managed proxies'))
    },
    enabled: props.open && !props.disabled && !props.isEditing,
    staleTime: 15_000,
  })
  const managedProxies = proxiesQuery.data ?? EMPTY_PROXIES
  const availableProxies = useMemo(
    () =>
      managedProxies.filter(
        (proxy) =>
          proxy.enabled &&
          (proxy.max_channel_accounts === 0 ||
            proxy.channel_account_count < proxy.max_channel_accounts)
      ),
    [managedProxies]
  )
  const selectedRuntime = runtimeOptions.find(
    (option) =>
      `${option.agent_id}:${option.runtime_key}` === selectedRuntimeValue
  )

  useEffect(() => {
    if (autoGenerateProfile) return
    if (!selectableProfiles.length) {
      setSelectedProfileId(0)
      return
    }
    if (
      !selectableProfiles.some((profile) => profile.id === selectedProfileId)
    ) {
      setSelectedProfileId(selectableProfiles[0].id)
    }
  }, [autoGenerateProfile, selectableProfiles, selectedProfileId])

  useEffect(() => {
    if (!autoGenerateProfile || props.isEditing) return
    if (
      !runtimeOptions.some(
        (option) =>
          `${option.agent_id}:${option.runtime_key}` === selectedRuntimeValue
      )
    ) {
      const first = runtimeOptions[0]
      setSelectedRuntimeValue(
        first ? `${first.agent_id}:${first.runtime_key}` : ''
      )
    }
  }, [
    autoGenerateProfile,
    props.isEditing,
    runtimeOptions,
    selectedRuntimeValue,
  ])

  useEffect(() => {
    if (!autoGenerateProfile || props.isEditing) return
    if (availableProxies.some((proxy) => proxy.id === selectedProxyId)) return
    const preferred = availableProxies.find(
      (proxy) => proxy.id === props.managedProxyId
    )
    setSelectedProxyId(preferred?.id ?? availableProxies[0]?.id ?? 0)
  }, [
    autoGenerateProfile,
    availableProxies,
    props.isEditing,
    props.managedProxyId,
    selectedProxyId,
  ])

  useEffect(() => {
    if (
      autoGenerateProfile &&
      selectedProxyId > 0 &&
      selectedProxyId !== props.managedProxyId
    ) {
      onManagedProxyChange(selectedProxyId)
    }
  }, [
    autoGenerateProfile,
    props.managedProxyId,
    onManagedProxyChange,
    selectedProxyId,
  ])

  useEffect(() => {
    if (props.flowId && !activeFlowId) setActiveFlowId(props.flowId)
  }, [activeFlowId, props.flowId])

  const flowQuery = useQuery({
    queryKey: ['browser-oauth', 'codex', activeFlowId],
    queryFn: async () => {
      const response = await getCodexBrowserOAuth(activeFlowId ?? '')
      return requireData(response, t('Failed to load OAuth flow'))
    },
    enabled: props.open && Boolean(activeFlowId),
    refetchInterval: (query) => {
      const currentFlow = query.state.data
      if (currentFlow && ACTIVE_FLOW_STATUSES.has(currentFlow.status)) {
        return 2_000
      }
      if (
        currentFlow?.status === 'completed' &&
        !currentFlow.bound &&
        currentFlow.expires_at > Date.now() / 1000
      ) {
        return 5_000
      }
      return false
    },
    retry: false,
  })
  const flow = flowQuery.data

  useEffect(() => {
    const wasOpen = wasOpenRef.current
    wasOpenRef.current = props.open
    if (props.open || !wasOpen) return

    const shouldCancel =
      activeFlowId &&
      (!flow ||
        ACTIVE_FLOW_STATUSES.has(flow.status) ||
        (flow.status === 'completed' && !flow.bound))
    if (shouldCancel) {
      void cancelCodexBrowserOAuth(activeFlowId).catch(() => undefined)
    }
    setActiveFlowId(null)
    setSelectedProfileId(0)
    setAutoGenerateProfile(!props.isEditing)
    setSelectedRuntimeValue('')
    setSelectedProxyId(0)
    completedFlowRef.current = null
  }, [activeFlowId, flow, props.isEditing, props.open])

  useEffect(() => {
    if (flow?.status !== 'completed' || completedFlowRef.current === flow.id) {
      return
    }
    completedFlowRef.current = flow.id
    onCompleted(flow)
    toast.success(
      isEditing
        ? t('Codex OAuth credential updated')
        : t('Codex OAuth login completed')
    )
  }, [flow, isEditing, onCompleted, t])

  useEffect(() => {
    if (
      flow &&
      props.flowId === flow.id &&
      (flow.status === 'failed' ||
        flow.status === 'canceled' ||
        flow.status === 'expired')
    ) {
      onFlowReset()
    }
  }, [flow, onFlowReset, props.flowId])

  const startMutation = useMutation({
    mutationFn: async () => {
      if (flow?.status === 'completed' && !flow.bound) {
        const discardResponse = await cancelCodexBrowserOAuth(flow.id)
        if (!discardResponse.success) {
          throw new Error(
            discardResponse.message || t('Failed to cancel OAuth flow')
          )
        }
      }
      if (autoGenerateProfile && !selectedRuntime) {
        throw new Error(t('Select a browser runtime'))
      }
      const response = await startCodexBrowserOAuth({
        profile_id: autoGenerateProfile ? 0 : selectedProfileId,
        channel_id: props.channelId ?? 0,
        auto_generate_profile: autoGenerateProfile,
        profile_name: `${props.channelName.trim() || t('Codex channel')} · ${t('Browser profile')}`,
        agent_id: autoGenerateProfile ? selectedRuntime?.agent_id : undefined,
        proxy_id: autoGenerateProfile ? selectedProxyId : undefined,
        runtime_key: autoGenerateProfile
          ? selectedRuntime?.runtime_key
          : undefined,
      })
      return requireData(response, t('Failed to start Codex OAuth'))
    },
    onSuccess: (nextFlow) => {
      completedFlowRef.current = null
      props.onFlowReset()
      if (autoGenerateProfile) {
        setAutoGenerateProfile(false)
        setSelectedProfileId(nextFlow.profile_id)
        void profilesQuery.refetch()
        void proxiesQuery.refetch()
      }
      setActiveFlowId(nextFlow.id)
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start Codex OAuth')
      )
    },
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      if (!activeFlowId) return
      const response = await cancelCodexBrowserOAuth(activeFlowId)
      if (!response.success) {
        throw new Error(response.message || t('Failed to cancel OAuth flow'))
      }
    },
    onSuccess: () => {
      flowQuery.refetch()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to cancel OAuth flow')
      )
    },
  })

  const profileItems = profiles.map((profile) => {
    const capacity =
      profile.proxy_max_channel_accounts > 0
        ? `${profile.proxy_channel_account_count}/${profile.proxy_max_channel_accounts}`
        : `${profile.proxy_channel_account_count}/${t('Unlimited')}`
    return {
      value: String(profile.id),
      label: `${profile.name} · ${profile.agent_name} · ${profile.proxy_name} · ${t('Profiles')}: ${profile.proxy_profile_count} · ${t('Channel accounts')}: ${capacity}`,
      disabled: profile.proxy_at_capacity,
    }
  })
  if (
    selectedProfileId > 0 &&
    !profileItems.some((item) => item.value === String(selectedProfileId))
  ) {
    profileItems.push({
      value: String(selectedProfileId),
      label: flow?.profile_name || t('Generated browser profile'),
      disabled: false,
    })
  }
  if (!props.isEditing) {
    profileItems.unshift({
      value: AUTOMATIC_PROFILE_VALUE,
      label: t('Automatically create a fixed Profile and fingerprint'),
      disabled: false,
    })
  }
  const runtimeItems = runtimeOptions.map((option) => ({
    value: `${option.agent_id}:${option.runtime_key}`,
    label: `${option.agent_name} · ${option.runtime_key} · ${t('Profiles')}: ${option.profile_count}`,
  }))
  const proxyItems = availableProxies.map((proxy) => {
    const capacity =
      proxy.max_channel_accounts > 0
        ? `${proxy.channel_account_count}/${proxy.max_channel_accounts}`
        : `${proxy.channel_account_count}/${t('Unlimited')}`
    return {
      value: String(proxy.id),
      label: `${proxy.name} · ${t('Profiles')}: ${proxy.profile_count} · ${t('Channel accounts')}: ${capacity}`,
    }
  })
  let profileSelectionValue = ''
  if (autoGenerateProfile) {
    profileSelectionValue = AUTOMATIC_PROFILE_VALUE
  } else if (selectedProfileId) {
    profileSelectionValue = String(selectedProfileId)
  }
  let automaticOptionsError = ''
  if (runtimesQuery.error instanceof Error) {
    automaticOptionsError = runtimesQuery.error.message
  } else if (proxiesQuery.error instanceof Error) {
    automaticOptionsError = proxiesQuery.error.message
  }
  const isFlowActive = Boolean(flow && ACTIVE_FLOW_STATUSES.has(flow.status))
  const status = flow?.status
  let statusLabel = t('Ready to start')
  let statusIcon = <ShieldCheck />
  let statusVariant: 'secondary' | 'outline' | 'destructive' = 'outline'
  if (status === 'pending') {
    statusLabel = t('Waiting for browser agent')
    statusIcon = <Loader2 className='animate-spin' />
    statusVariant = 'secondary'
  } else if (status === 'claimed') {
    statusLabel = t('Browser agent claimed the flow')
    statusIcon = <Bot />
    statusVariant = 'secondary'
  } else if (status === 'running') {
    statusLabel = t('Browser opened — complete login there')
    statusIcon = <ExternalLink />
    statusVariant = 'secondary'
  } else if (status === 'completed') {
    statusLabel = props.isEditing
      ? t('Credential saved to channel')
      : t('Credential ready for channel creation')
    statusIcon = <CheckCircle2 />
    statusVariant = 'secondary'
  } else if (status === 'failed') {
    statusLabel = t('OAuth flow failed')
    statusIcon = <XCircle />
    statusVariant = 'destructive'
  } else if (status === 'canceled') {
    statusLabel = t('OAuth flow canceled')
    statusIcon = <CircleAlert />
  } else if (status === 'expired') {
    statusLabel = t('OAuth flow expired')
    statusIcon = <CircleAlert />
  }

  let startIcon = <LogIn data-icon='inline-start' />
  if (startMutation.isPending) {
    startIcon = <Loader2 data-icon='inline-start' className='animate-spin' />
  } else if (status) {
    startIcon = <RefreshCw data-icon='inline-start' />
  }

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('Managed browser OAuth')}</CardTitle>
        <CardDescription>
          {t(
            'Launch the selected profile on its assigned agent. OAuth tokens are exchanged through the same proxy and never returned to this page.'
          )}
        </CardDescription>
        <CardAction>
          <Badge variant={statusVariant}>
            {statusIcon}
            {statusLabel}
          </Badge>
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <div className='flex flex-col gap-2'>
          <span className='text-sm font-medium'>{t('Browser profile')}</span>
          <Select
            items={profileItems}
            value={profileSelectionValue}
            onValueChange={(value) => {
              if (value === AUTOMATIC_PROFILE_VALUE) {
                setAutoGenerateProfile(true)
                return
              }
              setAutoGenerateProfile(false)
              setSelectedProfileId(Number(value))
            }}
            disabled={props.disabled || isFlowActive || startMutation.isPending}
          >
            <SelectTrigger>
              <SelectValue>
                {(value) =>
                  profileItems.find((item) => item.value === value)?.label ||
                  t('Select a browser profile')
                }
              </SelectValue>
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {profileItems.map((item) => (
                  <SelectItem
                    key={item.value}
                    value={item.value}
                    disabled={item.disabled}
                  >
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {autoGenerateProfile && !props.isEditing && (
            <div className='grid gap-3 rounded-lg border p-3 sm:grid-cols-2'>
              <div className='flex min-w-0 flex-col gap-2'>
                <span className='text-sm font-medium'>
                  {t('Browser runtime')}
                </span>
                <Select
                  items={runtimeItems}
                  value={selectedRuntimeValue}
                  onValueChange={(value) =>
                    setSelectedRuntimeValue(value ?? '')
                  }
                  disabled={runtimesQuery.isPending || isFlowActive}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {runtimeItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
              <div className='flex min-w-0 flex-col gap-2'>
                <span className='text-sm font-medium'>
                  {t('Managed proxy')}
                </span>
                <Select
                  items={proxyItems}
                  value={selectedProxyId ? String(selectedProxyId) : ''}
                  onValueChange={(value) => setSelectedProxyId(Number(value))}
                  disabled={proxiesQuery.isPending || isFlowActive}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {proxyItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
              <p className='text-muted-foreground text-xs sm:col-span-2'>
                {t(
                  'A coherent fingerprint is generated once, stored with the new Profile, and reused for every browser launch.'
                )}
              </p>
              {(runtimesQuery.isError || proxiesQuery.isError) && (
                <p className='text-destructive text-xs sm:col-span-2'>
                  {automaticOptionsError ||
                    t('Failed to load automatic Profile options')}
                </p>
              )}
            </div>
          )}
          {profilesQuery.isError && (
            <p className='text-destructive text-xs'>
              {profilesQuery.error instanceof Error
                ? profilesQuery.error.message
                : t('Failed to load browser profiles')}
            </p>
          )}
          {!autoGenerateProfile &&
            !profilesQuery.isPending &&
            !profilesQuery.isError &&
            profiles.length === 0 && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'No available profiles. Check that the profile, proxy, fingerprint, and agent are enabled and the agent is online.'
                )}
              </p>
            )}
          {!autoGenerateProfile &&
            !profilesQuery.isPending &&
            !profilesQuery.isError &&
            profiles.length > 0 &&
            selectableProfiles.length === 0 && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'All available profiles use managed proxies that have reached their channel account limit.'
                )}
              </p>
            )}
        </div>

        {flow?.status === 'completed' && (
          <div className='grid gap-3 rounded-lg border p-3 sm:grid-cols-2'>
            <div>
              <p className='text-muted-foreground text-xs'>{t('Email')}</p>
              <p className='text-sm font-medium break-all'>
                {flow.email || '-'}
              </p>
            </div>
            <div>
              <p className='text-muted-foreground text-xs'>{t('Account ID')}</p>
              <p className='font-mono text-xs break-all'>
                {flow.account_id || '-'}
              </p>
            </div>
            <div>
              <p className='text-muted-foreground text-xs'>{t('Plan')}</p>
              <p className='text-sm font-medium'>{flow.plan_type || '-'}</p>
            </div>
            <div>
              <p className='text-muted-foreground text-xs'>
                {t('Credential expires')}
              </p>
              <p className='text-sm font-medium'>
                {formatTime(flow.credential_expires_at)}
              </p>
            </div>
            {!flow.bound && (
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Credential must be saved by')}
                </p>
                <p className='text-sm font-medium'>
                  {formatTime(flow.expires_at)}
                </p>
              </div>
            )}
          </div>
        )}

        {(flow?.status === 'failed' || flowQuery.isError) && (
          <Alert variant='destructive'>
            <CircleAlert />
            <AlertDescription>
              {flow?.error_message ||
                (flowQuery.error instanceof Error
                  ? flowQuery.error.message
                  : t('OAuth flow failed'))}
            </AlertDescription>
          </Alert>
        )}
      </CardContent>
      <CardFooter className='flex flex-wrap justify-end gap-2'>
        {isFlowActive && (
          <Button
            type='button'
            variant='outline'
            onClick={() => cancelMutation.mutate()}
            disabled={cancelMutation.isPending}
          >
            {cancelMutation.isPending && (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            )}
            {t('Cancel OAuth')}
          </Button>
        )}
        {!isFlowActive && (
          <Button
            type='button'
            onClick={() => startMutation.mutate()}
            disabled={
              props.disabled ||
              (autoGenerateProfile
                ? !selectedRuntime ||
                  !selectedProxyId ||
                  runtimesQuery.isPending ||
                  proxiesQuery.isPending
                : !selectedProfileId || profilesQuery.isPending) ||
              startMutation.isPending
            }
          >
            {startIcon}
            {status ? t('Start new login') : t('Open browser login')}
          </Button>
        )}
      </CardFooter>
    </Card>
  )
}
