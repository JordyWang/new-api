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
import { Globe2, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  browserManagementQueryKeys,
  deleteBrowserProxy,
  listBrowserProxies,
} from '@/features/browser-management/api'
import { ProxyDialog } from '@/features/browser-management/components/agent-proxy-dialogs'
import { ProxiesTable } from '@/features/browser-management/components/resource-tables'
import type { BrowserProxy } from '@/features/browser-management/types'

export function ProxyManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<BrowserProxy | null>(null)

  const proxiesQuery = useQuery({
    queryKey: browserManagementQueryKeys.proxies(),
    queryFn: listBrowserProxies,
  })
  const deleteMutation = useMutation({
    mutationFn: (proxy: BrowserProxy) => deleteBrowserProxy(proxy.id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.proxies(),
      })
      setDeleteTarget(null)
      toast.success(t('Managed proxy deleted'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to delete managed proxy')
      )
    },
  })

  let content = null
  if (proxiesQuery.isPending) {
    content = <LoadingState message={t('Loading managed proxies...')} />
  } else if (proxiesQuery.isError) {
    content = (
      <ErrorState
        title={t('Failed to load managed proxies')}
        description={
          proxiesQuery.error instanceof Error
            ? proxiesQuery.error.message
            : t('Request failed')
        }
        onRetry={() => void proxiesQuery.refetch()}
      />
    )
  } else {
    content = (
      <ProxiesTable
        data={proxiesQuery.data}
        onEdit={(proxy) => {
          setEditingProxy(proxy)
          setEditorOpen(true)
        }}
        onDelete={setDeleteTarget}
      />
    )
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Proxy Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            onClick={() => {
              setEditingProxy(null)
              setEditorOpen(true)
            }}
          >
            <Plus data-icon='inline-start' />
            {t('Create')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex min-h-full flex-col gap-4'>
            <Alert>
              <Globe2 />
              <AlertTitle>{t('Central proxy library')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Channels and fingerprint browser profiles select proxies from this page. Updating a proxy here updates every reference without exposing its credentials.'
                )}
              </AlertDescription>
            </Alert>
            {content}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ProxyDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        proxy={editingProxy}
      />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete managed proxy?')}
        desc={t(
          'Delete “{{name}}”? Proxies referenced by channels or browser profiles must be reassigned first.',
          { name: deleteTarget?.name ?? '' }
        )}
        destructive
        confirmText={t('Delete')}
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget)
        }}
      />
    </>
  )
}
