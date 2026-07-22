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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Fingerprint, Plus, UserRoundCog } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  browserManagementQueryKeys,
  cancelBrowserLaunch,
  deleteBrowserAgent,
  deleteBrowserFingerprint,
  deleteBrowserProfile,
  launchBrowserProfile,
  listBrowserAgents,
  listBrowserFingerprints,
  listBrowserProfileChannels,
  listBrowserProfiles,
  listBrowserProxies,
  resetBrowserProfile,
  rotateBrowserAgentToken,
} from './api'
import { AgentDialog } from './components/agent-proxy-dialogs'
import { AgentTokenDialog } from './components/agent-token-dialog'
import { FingerprintDialog } from './components/fingerprint-dialog'
import { ProfileDialog } from './components/profile-dialog'
import {
  AgentsTable,
  FingerprintsTable,
  ProfilesTable,
} from './components/resource-tables'
import type { BrowserAgent, BrowserFingerprint, BrowserProfile } from './types'

type ResourceTab = 'agents' | 'fingerprints' | 'profiles'

type DeleteTarget =
  | { kind: 'agent'; id: number; name: string }
  | { kind: 'fingerprint'; id: number; name: string }
  | { kind: 'profile'; id: number; name: string }

const TAB_ICONS = {
  agents: Bot,
  fingerprints: Fingerprint,
  profiles: UserRoundCog,
} as const

export function BrowserManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<ResourceTab>('agents')
  const [agentEditorOpen, setAgentEditorOpen] = useState(false)
  const [editingAgent, setEditingAgent] = useState<BrowserAgent | null>(null)
  const [fingerprintEditorOpen, setFingerprintEditorOpen] = useState(false)
  const [editingFingerprint, setEditingFingerprint] =
    useState<BrowserFingerprint | null>(null)
  const [profileEditorOpen, setProfileEditorOpen] = useState(false)
  const [editingProfile, setEditingProfile] = useState<BrowserProfile | null>(
    null
  )
  const [agentToken, setAgentToken] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [rotateTarget, setRotateTarget] = useState<BrowserAgent | null>(null)
  const [resetTarget, setResetTarget] = useState<BrowserProfile | null>(null)

  const agentsQuery = useQuery({
    queryKey: browserManagementQueryKeys.agents(),
    queryFn: listBrowserAgents,
    refetchInterval: 15_000,
  })
  const proxiesQuery = useQuery({
    queryKey: browserManagementQueryKeys.proxies(),
    queryFn: listBrowserProxies,
  })
  const fingerprintsQuery = useQuery({
    queryKey: browserManagementQueryKeys.fingerprints(),
    queryFn: listBrowserFingerprints,
  })
  const profilesQuery = useQuery({
    queryKey: browserManagementQueryKeys.profiles(),
    queryFn: listBrowserProfiles,
    refetchInterval: 5_000,
  })
  const profileChannelsQuery = useQuery({
    queryKey: browserManagementQueryKeys.profileChannels(),
    queryFn: listBrowserProfileChannels,
  })

  const deleteMutation = useMutation({
    mutationFn: async (target: DeleteTarget) => {
      switch (target.kind) {
        case 'agent':
          return deleteBrowserAgent(target.id)
        case 'fingerprint':
          return deleteBrowserFingerprint(target.id)
        case 'profile':
          return deleteBrowserProfile(target.id)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.all,
      })
      setDeleteTarget(null)
      toast.success(t('Browser resource deleted'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to delete browser resource')
      )
    },
  })

  const rotateMutation = useMutation({
    mutationFn: (agent: BrowserAgent) => rotateBrowserAgentToken(agent.id),
    onSuccess: (result) => {
      setRotateTarget(null)
      setAgentToken(result.token)
      toast.success(t('Browser agent token rotated'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to rotate token')
      )
    },
  })

  const resetMutation = useMutation({
    mutationFn: (profile: BrowserProfile) => resetBrowserProfile(profile.id),
    onSuccess: () => {
      setResetTarget(null)
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.profiles(),
      })
      toast.success(t('Browser profile data key rotated'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to reset profile')
      )
    },
  })

  const launchMutation = useMutation({
    mutationFn: (profile: BrowserProfile) => launchBrowserProfile(profile.id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.profiles(),
      })
      toast.success(t('Browser launch requested'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to open browser profile')
      )
    },
  })

  const stopMutation = useMutation({
    mutationFn: (profile: BrowserProfile) =>
      cancelBrowserLaunch(profile.active_launch_id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.profiles(),
      })
      toast.success(t('Browser stop requested'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to stop browser profile')
      )
    },
  })

  const agents = agentsQuery.data ?? []
  const proxies = proxiesQuery.data ?? []
  const fingerprints = fingerprintsQuery.data ?? []
  const profiles = profilesQuery.data ?? []
  const profileChannels = profileChannelsQuery.data ?? []
  const activeQuery = {
    agents: agentsQuery,
    fingerprints: fingerprintsQuery,
    profiles: profilesQuery,
  }[activeTab]
  const canCreateProfile =
    agents.length > 0 && proxies.length > 0 && fingerprints.length > 0

  const openCreateDialog = () => {
    switch (activeTab) {
      case 'agents':
        setEditingAgent(null)
        setAgentEditorOpen(true)
        break
      case 'fingerprints':
        setEditingFingerprint(null)
        setFingerprintEditorOpen(true)
        break
      case 'profiles':
        setEditingProfile(null)
        setProfileEditorOpen(true)
        break
    }
  }

  const ActiveIcon = TAB_ICONS[activeTab]
  let tabContent = null
  if (activeQuery.isPending) {
    tabContent = <LoadingState message={t('Loading browser resources...')} />
  } else if (activeQuery.isError) {
    tabContent = (
      <ErrorState
        title={t('Failed to load browser resources')}
        description={
          activeQuery.error instanceof Error
            ? activeQuery.error.message
            : t('Request failed')
        }
        onRetry={() => void activeQuery.refetch()}
      />
    )
  } else {
    switch (activeTab) {
      case 'agents':
        tabContent = (
          <AgentsTable
            data={agents}
            onEdit={(agent) => {
              setEditingAgent(agent)
              setAgentEditorOpen(true)
            }}
            onRotateToken={setRotateTarget}
            onDelete={(agent) =>
              setDeleteTarget({
                kind: 'agent',
                id: agent.id,
                name: agent.name,
              })
            }
          />
        )
        break
      case 'fingerprints':
        tabContent = (
          <FingerprintsTable
            data={fingerprints}
            onEdit={(fingerprint) => {
              setEditingFingerprint(fingerprint)
              setFingerprintEditorOpen(true)
            }}
            onDelete={(fingerprint) =>
              setDeleteTarget({
                kind: 'fingerprint',
                id: fingerprint.id,
                name: fingerprint.name,
              })
            }
          />
        )
        break
      case 'profiles':
        tabContent = (
          <ProfilesTable
            data={profiles}
            onLaunch={(profile) => launchMutation.mutate(profile)}
            onStop={(profile) => stopMutation.mutate(profile)}
            onEdit={(profile) => {
              setEditingProfile(profile)
              setProfileEditorOpen(true)
            }}
            onReset={setResetTarget}
            onDelete={(profile) =>
              setDeleteTarget({
                kind: 'profile',
                id: profile.id,
                name: profile.name,
              })
            }
          />
        )
        break
    }
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Browser Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            onClick={openCreateDialog}
            disabled={activeTab === 'profiles' && !canCreateProfile}
            title={
              activeTab === 'profiles' && !canCreateProfile
                ? t(
                    'Create an agent, fingerprint, and a proxy in Proxy Management first'
                  )
                : undefined
            }
          >
            <Plus data-icon='inline-start' />
            {t('Create')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex min-h-full flex-col gap-4'>
            <Alert>
              <ActiveIcon />
              <AlertTitle>
                {t('Control-plane managed browser OAuth')}
              </AlertTitle>
              <AlertDescription>
                {t(
                  'new-api owns agent registration, public proxies, fingerprints, and profiles. The agent launches your local Chromium build and routes both browser traffic and OAuth token exchange through the same managed proxy.'
                )}
              </AlertDescription>
            </Alert>
            <Tabs
              value={activeTab}
              onValueChange={(value) => setActiveTab(value as ResourceTab)}
              className='min-h-0 flex-1'
            >
              <TabsList className='max-w-full overflow-x-auto' variant='line'>
                <TabsTrigger value='agents'>
                  <Bot />
                  {t('Agents')}
                </TabsTrigger>
                <TabsTrigger value='fingerprints'>
                  <Fingerprint />
                  {t('Fingerprints')}
                </TabsTrigger>
                <TabsTrigger value='profiles'>
                  <UserRoundCog />
                  {t('Profiles')}
                </TabsTrigger>
              </TabsList>
              <TabsContent value={activeTab} className='pt-3'>
                {tabContent}
              </TabsContent>
            </Tabs>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AgentDialog
        open={agentEditorOpen}
        onOpenChange={setAgentEditorOpen}
        agent={editingAgent}
        onToken={setAgentToken}
      />
      <FingerprintDialog
        open={fingerprintEditorOpen}
        onOpenChange={setFingerprintEditorOpen}
        fingerprint={editingFingerprint}
      />
      <ProfileDialog
        open={profileEditorOpen}
        onOpenChange={setProfileEditorOpen}
        profile={editingProfile}
        agents={agents}
        proxies={proxies}
        fingerprints={fingerprints}
        channels={profileChannels}
      />
      <AgentTokenDialog
        token={agentToken}
        onClose={() => setAgentToken(null)}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete browser resource?')}
        desc={t(
          'Delete “{{name}}”? Referenced resources must be reassigned before deletion.',
          { name: deleteTarget?.name ?? '' }
        )}
        destructive
        confirmText={t('Delete')}
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget)
        }}
      />
      <ConfirmDialog
        open={Boolean(rotateTarget)}
        onOpenChange={(open) => {
          if (!open) setRotateTarget(null)
        }}
        title={t('Rotate browser agent token?')}
        desc={t(
          'The current token for “{{name}}” stops working immediately. The replacement is shown only once.',
          { name: rotateTarget?.name ?? '' }
        )}
        confirmText={t('Rotate token')}
        isLoading={rotateMutation.isPending}
        handleConfirm={() => {
          if (rotateTarget) rotateMutation.mutate(rotateTarget)
        }}
      />
      <ConfirmDialog
        open={Boolean(resetTarget)}
        onOpenChange={(open) => {
          if (!open) setResetTarget(null)
        }}
        title={t('Reset browser profile data?')}
        desc={t(
          'This rotates the profile directory key for “{{name}}”. The old directory remains on the agent host for manual cleanup.',
          { name: resetTarget?.name ?? '' }
        )}
        confirmText={t('Reset profile data')}
        isLoading={resetMutation.isPending}
        handleConfirm={() => {
          if (resetTarget) resetMutation.mutate(resetTarget)
        }}
      />
    </>
  )
}
