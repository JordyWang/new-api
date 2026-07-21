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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

import { deleteRegistrationInviteCode } from '../api'
import { InviteCodeDrawer } from './invite-code-drawer'
import { useInviteCodes } from './invite-codes-provider'

export function InviteCodeDialogs() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useInviteCodes()
  const [isDeleting, setIsDeleting] = useState(false)
  const isUpdate = open === 'update'

  async function handleDelete() {
    if (!currentRow) return
    setIsDeleting(true)
    try {
      const response = await deleteRegistrationInviteCode(currentRow.id)
      if (!response.success) {
        toast.error(response.message || t('Failed to delete invitation code'))
        return
      }
      toast.success(t('Invitation code deleted successfully'))
      setOpen(null)
      triggerRefresh()
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <InviteCodeDrawer
        open={open === 'create' || isUpdate}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={isUpdate ? currentRow || undefined : undefined}
      />
      <AlertDialog
        open={open === 'delete'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Invitation Code?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will permanently delete invitation code {{code}}. This action cannot be undone.',
                { code: currentRow?.code || '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={isDeleting}
              onClick={handleDelete}
            >
              {isDeleting ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
