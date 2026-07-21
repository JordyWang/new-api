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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { getRegistrationInviteCodes } from '../api'
import { getInviteCodeDisplayStatus } from '../lib/status'
import { useInviteCodeColumns } from './invite-code-columns'
import { useInviteCodes } from './invite-codes-provider'

const route = getRouteApi('/_authenticated/registration-invite-codes/')

export function InviteCodesTable() {
  const { t } = useTranslation()
  const columns = useInviteCodeColumns()
  const { refreshTrigger } = useInviteCodes()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [{ columnId: 'status', searchKey: 'status', type: 'array' }],
  })
  const status =
    (
      columnFilters.find((filter) => filter.id === 'status')?.value as
        | string[]
        | undefined
    )?.[0] ?? ''

  const query = useQuery({
    queryKey: [
      'registration-invite-codes',
      pagination.pageIndex,
      pagination.pageSize,
      globalFilter,
      status,
      refreshTrigger,
    ],
    queryFn: async () => {
      const response = await getRegistrationInviteCodes({
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        keyword: globalFilter,
        status,
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to load invitation codes'))
        return { items: [], total: 0 }
      }
      return {
        items: response.data?.items ?? [],
        total: response.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const tableState = useDataTable({
    data: query.data?.items ?? [],
    columns,
    globalFilter,
    columnFilters,
    pagination,
    onGlobalFilterChange,
    onColumnFiltersChange,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: query.data?.total ?? 0,
    ensurePageInRange,
  })

  const statusOptions = useMemo(
    () => [
      { label: t('Enabled'), value: 'enabled' },
      { label: t('Disabled'), value: 'disabled' },
      { label: t('Expired'), value: 'expired' },
      { label: t('Exhausted'), value: 'exhausted' },
    ],
    [t]
  )

  return (
    <DataTablePage
      table={tableState.table}
      columns={columns}
      isLoading={query.isLoading}
      isFetching={query.isFetching}
      emptyTitle={t('No Invitation Codes Found')}
      emptyDescription={t(
        'Create an invitation code to allow new users to register.'
      )}
      skeletonKeyPrefix='registration-invite-codes-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by code, group, or ID...'),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: statusOptions,
            singleSelect: true,
          },
        ],
      }}
      getRowClassName={(row, context) => {
        if (getInviteCodeDisplayStatus(row.original) === 'enabled') {
          return undefined
        }
        return context.isMobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
      }}
    />
  )
}
