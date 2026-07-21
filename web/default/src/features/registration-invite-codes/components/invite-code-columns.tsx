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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatQuota, formatTimestampToDate } from '@/lib/format'

import {
  getInviteCodeDisplayStatus,
  type InviteCodeDisplayStatus,
} from '../lib/status'
import type { RegistrationInviteCode } from '../types'
import { InviteCodeRowActions } from './invite-code-row-actions'

const STATUS_VARIANTS: Record<InviteCodeDisplayStatus, StatusVariant> = {
  enabled: 'success',
  disabled: 'neutral',
  expired: 'warning',
  exhausted: 'danger',
}

const STATUS_LABELS: Record<InviteCodeDisplayStatus, string> = {
  enabled: 'Enabled',
  disabled: 'Disabled',
  expired: 'Expired',
  exhausted: 'Exhausted',
}

export function useInviteCodeColumns(): ColumnDef<RegistrationInviteCode>[] {
  const { t } = useTranslation()
  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => <TableId value={row.original.id} />,
      size: 70,
    },
    {
      accessorKey: 'code',
      header: t('Invitation Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => {
        const code = row.original.code
        const masked =
          code.length <= 8
            ? `${code.slice(0, 2)}${'*'.repeat(Math.max(code.length - 2, 2))}`
            : `${code.slice(0, 4)}${'*'.repeat(6)}${code.slice(-4)}`
        return (
          <MaskedValueDisplay
            label={t('Full Code')}
            fullValue={code}
            maskedValue={masked}
            copyTooltip={t('Copy code')}
            copyAriaLabel={t('Copy invitation code')}
          />
        )
      },
      size: 260,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const status = getInviteCodeDisplayStatus(row.original)
        return (
          <StatusBadge
            label={t(STATUS_LABELS[status])}
            variant={STATUS_VARIANTS[status]}
            copyable={false}
          />
        )
      },
      size: 110,
    },
    {
      accessorKey: 'group',
      header: t('User Group'),
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.group}
          variant='neutral'
          copyable={false}
        />
      ),
      size: 120,
    },
    {
      accessorKey: 'initial_quota',
      header: t('New User Quota'),
      cell: ({ row }) => formatQuota(row.original.initial_quota),
      size: 140,
    },
    {
      id: 'usage',
      header: t('Registrations Used'),
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {row.original.registered_count} /{' '}
          {row.original.max_registrations || t('Unlimited')}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'expired_time',
      header: t('Expiration Time'),
      meta: { mobileHidden: true },
      cell: ({ row }) =>
        row.original.expired_time > 0
          ? formatTimestampToDate(row.original.expired_time)
          : t('Never'),
      size: 180,
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => formatTimestampToDate(row.original.created_time),
      size: 180,
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => <InviteCodeRowActions row={row} />,
      meta: { pinned: 'right' as const },
    },
  ]
}
