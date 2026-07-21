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
import { useEffect, useRef, useState } from 'react'
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
  getCodexBrowserOAuth,
  startCodexBrowserOAuth,
} from '../api'
import type {
  BrowserOAuthProfile,
  CodexBrowserOAuthFlow,
  CodexBrowserOAuthFlowStatus,
} from '../types'

const ACTIVE_FLOW_STATUSES = new Set<CodexBrowserOAuthFlowStatus>([
  'pending',
  'claimed',
  'running',
])
const EMPTY_PROFILES: BrowserOAuthProfile[] = []

type CodexBrowserOAuthCardProps = {
  open: boolean
  channelId: number | null
  isEditing: boolean
  disabled: boolean
  flowId: string
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
  const isEditing = props.isEditing
  const [selectedProfileId, setSelectedProfileId] = useState(0)
  const [activeFlowId, setActiveFlowId] = useState<string | null>(
    props.flowId || null
  )
  const completedFlowRef = useRef<string | null>(null)
  const wasOpenRef = useRef(props.open)

  const profilesQuery = useQuery({
    queryKey: ['browser-oauth', 'profiles'],
    queryFn: async () => {
      const response = await getBrowserOAuthProfiles()
      return requireData(response, t('Failed to load browser profiles'))
    },
    enabled: props.open && !props.disabled,
    staleTime: 15_000,
  })
  const profiles = profilesQuery.data ?? EMPTY_PROFILES

  useEffect(() => {
    if (!profiles.length) {
      setSelectedProfileId(0)
      return
    }
    if (!profiles.some((profile) => profile.id === selectedProfileId)) {
      setSelectedProfileId(profiles[0].id)
    }
  }, [profiles, selectedProfileId])

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
    completedFlowRef.current = null
  }, [activeFlowId, flow, props.open])

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
      const response = await startCodexBrowserOAuth({
        profile_id: selectedProfileId,
        channel_id: props.channelId ?? 0,
      })
      return requireData(response, t('Failed to start Codex OAuth'))
    },
    onSuccess: (nextFlow) => {
      completedFlowRef.current = null
      props.onFlowReset()
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

  const profileItems = profiles.map((profile) => ({
    value: String(profile.id),
    label: `${profile.name} · ${profile.agent_name} · ${profile.proxy_name}`,
  }))
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
            value={selectedProfileId ? String(selectedProfileId) : ''}
            onValueChange={(value) => setSelectedProfileId(Number(value))}
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
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {profilesQuery.isError && (
            <p className='text-destructive text-xs'>
              {profilesQuery.error instanceof Error
                ? profilesQuery.error.message
                : t('Failed to load browser profiles')}
            </p>
          )}
          {!profilesQuery.isPending &&
            !profilesQuery.isError &&
            profiles.length === 0 && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'No available profiles. Check that the profile, proxy, fingerprint, and agent are enabled and the agent is online.'
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
              !selectedProfileId ||
              profilesQuery.isPending ||
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
