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
import {
  Bot,
  Edit,
  Fingerprint,
  KeyRound,
  RotateCcw,
  Trash2,
  UserRoundCog,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  DataTableRowActionMenu,
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'

import type {
  BrowserAgent,
  BrowserFingerprint,
  BrowserProfile,
  BrowserProxy,
} from '../types'

type AgentsTableProps = {
  data: BrowserAgent[]
  onEdit: (agent: BrowserAgent) => void
  onRotateToken: (agent: BrowserAgent) => void
  onDelete: (agent: BrowserAgent) => void
}

type ProxiesTableProps = {
  data: BrowserProxy[]
  onEdit: (proxy: BrowserProxy) => void
  onDelete: (proxy: BrowserProxy) => void
}

type FingerprintsTableProps = {
  data: BrowserFingerprint[]
  onEdit: (fingerprint: BrowserFingerprint) => void
  onDelete: (fingerprint: BrowserFingerprint) => void
}

type ProfilesTableProps = {
  data: BrowserProfile[]
  onEdit: (profile: BrowserProfile) => void
  onReset: (profile: BrowserProfile) => void
  onDelete: (profile: BrowserProfile) => void
}

function formatTime(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function ResourceEmpty(props: {
  icon: LucideIcon
  title: string
  description: string
}) {
  const Icon = props.icon
  return (
    <Empty className='min-h-48 border-0'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <Icon />
        </EmptyMedia>
        <EmptyTitle>{props.title}</EmptyTitle>
        <EmptyDescription>{props.description}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

function EnabledBadge(props: { enabled: boolean }) {
  const { t } = useTranslation()
  return (
    <Badge variant={props.enabled ? 'secondary' : 'outline'}>
      {props.enabled ? t('Enabled') : t('Disabled')}
    </Badge>
  )
}

export function AgentsTable(props: AgentsTableProps) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<BrowserAgent>[] = [
    {
      id: 'name',
      header: t('Agent'),
      cell: (agent) => (
        <div className='flex min-w-40 flex-col gap-1'>
          <span className='font-medium'>{agent.name}</span>
          <span className='text-muted-foreground text-xs'>
            {agent.platform && agent.arch
              ? `${agent.platform}/${agent.arch}`
              : t('No heartbeat yet')}
          </span>
        </div>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (agent) => (
        <div className='flex flex-wrap gap-1.5'>
          <Badge variant={agent.online ? 'secondary' : 'outline'}>
            {agent.online ? t('Online') : t('Offline')}
          </Badge>
          <EnabledBadge enabled={agent.enabled} />
        </div>
      ),
    },
    {
      id: 'runtimes',
      header: t('Runtimes'),
      cell: (agent) =>
        agent.runtimes.length ? (
          <div className='flex max-w-80 flex-wrap gap-1'>
            {agent.runtimes.map((runtime) => (
              <Badge key={runtime} variant='outline'>
                {runtime}
              </Badge>
            ))}
          </div>
        ) : (
          '-'
        ),
    },
    {
      id: 'last_seen',
      header: t('Last seen'),
      cell: (agent) => formatTime(agent.last_seen_at),
    },
    {
      id: 'actions',
      header: '',
      className: 'w-12',
      cell: (agent) => (
        <DataTableRowActionMenu ariaLabel={t('Actions')}>
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => props.onEdit(agent)}>
              <Edit />
              {t('Edit')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => props.onRotateToken(agent)}>
              <KeyRound />
              {t('Rotate token')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem
              variant='destructive'
              onClick={() => props.onDelete(agent)}
            >
              <Trash2 />
              {t('Delete')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DataTableRowActionMenu>
      ),
    },
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(agent) => agent.id}
      emptyContent={
        <ResourceEmpty
          icon={Bot}
          title={t('No browser agents')}
          description={t(
            'Create an agent, copy its one-time token, then start it beside your Chromium runtime.'
          )}
        />
      }
    />
  )
}

export function ProxiesTable(props: ProxiesTableProps) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<BrowserProxy>[] = [
    {
      id: 'name',
      header: t('Proxy'),
      cell: (proxy) => <span className='font-medium'>{proxy.name}</span>,
    },
    {
      id: 'endpoint',
      header: t('Masked endpoint'),
      cell: (proxy) => (
        <code className='text-xs'>{proxy.url_masked || '-'}</code>
      ),
    },
    {
      id: 'scheme',
      header: t('Protocol'),
      cell: (proxy) => <Badge variant='outline'>{proxy.scheme}</Badge>,
    },
    {
      id: 'auth',
      header: t('Authentication'),
      cell: (proxy) =>
        proxy.has_credentials ? t('Credentials stored') : t('No credentials'),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (proxy) => <EnabledBadge enabled={proxy.enabled} />,
    },
    {
      id: 'actions',
      header: '',
      className: 'w-12',
      cell: (proxy) => (
        <DataTableRowActionMenu ariaLabel={t('Actions')}>
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => props.onEdit(proxy)}>
              <Edit />
              {t('Edit')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem
              variant='destructive'
              onClick={() => props.onDelete(proxy)}
            >
              <Trash2 />
              {t('Delete')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DataTableRowActionMenu>
      ),
    },
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(proxy) => proxy.id}
      emptyContent={
        <ResourceEmpty
          icon={KeyRound}
          title={t('No managed proxies')}
          description={t(
            'Add the public proxy that must be shared by browser login and all Codex token requests.'
          )}
        />
      }
    />
  )
}

export function FingerprintsTable(props: FingerprintsTableProps) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<BrowserFingerprint>[] = [
    {
      id: 'name',
      header: t('Fingerprint'),
      cell: (fingerprint) => (
        <div className='flex min-w-40 flex-col gap-1'>
          <span className='font-medium'>{fingerprint.name}</span>
          <span className='text-muted-foreground text-xs'>
            {fingerprint.locale} · {fingerprint.timezone}
          </span>
        </div>
      ),
    },
    {
      id: 'viewport',
      header: t('Viewport'),
      cell: (fingerprint) =>
        `${fingerprint.viewport_width} × ${fingerprint.viewport_height}`,
    },
    {
      id: 'user_agent',
      header: t('User Agent'),
      cell: (fingerprint) => fingerprint.user_agent || '-',
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (fingerprint) => <EnabledBadge enabled={fingerprint.enabled} />,
    },
    {
      id: 'actions',
      header: '',
      className: 'w-12',
      cell: (fingerprint) => (
        <DataTableRowActionMenu ariaLabel={t('Actions')}>
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => props.onEdit(fingerprint)}>
              <Edit />
              {t('Edit')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem
              variant='destructive'
              onClick={() => props.onDelete(fingerprint)}
            >
              <Trash2 />
              {t('Delete')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DataTableRowActionMenu>
      ),
    },
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(fingerprint) => fingerprint.id}
      emptyContent={
        <ResourceEmpty
          icon={Fingerprint}
          title={t('No browser fingerprints')}
          description={t(
            'Create a fingerprint configuration for your custom Chromium kernel.'
          )}
        />
      }
    />
  )
}

export function ProfilesTable(props: ProfilesTableProps) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<BrowserProfile>[] = [
    {
      id: 'name',
      header: t('Profile'),
      cell: (profile) => (
        <div className='flex min-w-40 flex-col gap-1'>
          <span className='font-medium'>{profile.name}</span>
          <span className='text-muted-foreground text-xs'>
            {profile.runtime_key}
          </span>
        </div>
      ),
    },
    {
      id: 'agent',
      header: t('Agent'),
      cell: (profile) => (
        <div className='flex items-center gap-2'>
          <span>{profile.agent_name || `#${profile.agent_id}`}</span>
          <Badge variant={profile.agent_online ? 'secondary' : 'outline'}>
            {profile.agent_online ? t('Online') : t('Offline')}
          </Badge>
        </div>
      ),
    },
    {
      id: 'proxy',
      header: t('Managed proxy'),
      cell: (profile) => profile.proxy_name || `#${profile.proxy_id}`,
    },
    {
      id: 'fingerprint',
      header: t('Fingerprint'),
      cell: (profile) =>
        profile.fingerprint_name || `#${profile.fingerprint_id}`,
    },
    {
      id: 'mode',
      header: t('Storage'),
      cell: (profile) => (
        <Badge variant='outline'>
          {profile.persistent ? t('Persistent') : t('Ephemeral')}
        </Badge>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (profile) => <EnabledBadge enabled={profile.enabled} />,
    },
    {
      id: 'actions',
      header: '',
      className: 'w-12',
      cell: (profile) => (
        <DataTableRowActionMenu ariaLabel={t('Actions')}>
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => props.onEdit(profile)}>
              <Edit />
              {t('Edit')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => props.onReset(profile)}>
              <RotateCcw />
              {t('Reset profile data')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem
              variant='destructive'
              onClick={() => props.onDelete(profile)}
            >
              <Trash2 />
              {t('Delete')}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DataTableRowActionMenu>
      ),
    },
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(profile) => profile.id}
      emptyContent={
        <ResourceEmpty
          icon={UserRoundCog}
          title={t('No browser profiles')}
          description={t(
            'Profiles combine an agent, runtime, proxy, fingerprint, and persistent browser directory.'
          )}
        />
      }
    />
  )
}
