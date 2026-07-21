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
import type { Row } from '@tanstack/react-table'
import { Edit, Power, PowerOff, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { updateRegistrationInviteCodeStatus } from '../api'
import { INVITE_CODE_STATUS } from '../lib/status'
import { registrationInviteCodeSchema } from '../types'
import { useInviteCodes } from './invite-codes-provider'

export function InviteCodeRowActions<TData>(props: { row: Row<TData> }) {
  const { t } = useTranslation()
  const inviteCode = registrationInviteCodeSchema.parse(props.row.original)
  const { setCurrentRow, setOpen, triggerRefresh } = useInviteCodes()
  const isEnabled = inviteCode.status === INVITE_CODE_STATUS.ENABLED

  async function handleToggleStatus() {
    const response = await updateRegistrationInviteCodeStatus(
      inviteCode.id,
      isEnabled ? INVITE_CODE_STATUS.DISABLED : INVITE_CODE_STATUS.ENABLED
    )
    if (!response.success) {
      toast.error(response.message || t('Failed to update invitation code'))
      return
    }
    toast.success(
      t(isEnabled ? 'Invitation code disabled' : 'Invitation code enabled')
    )
    triggerRefresh()
  }

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => {
                setCurrentRow(inviteCode)
                setOpen('update')
              }}
              aria-label={t('Edit')}
            />
          }
        >
          <Edit />
        </TooltipTrigger>
        <TooltipContent>{t('Edit')}</TooltipContent>
      </Tooltip>
      <DataTableRowActionMenu ariaLabel={t('Open menu')} modal={false}>
        <DropdownMenuItem onClick={handleToggleStatus}>
          {t(isEnabled ? 'Disable' : 'Enable')}
          <DropdownMenuShortcut>
            {isEnabled ? <PowerOff size={16} /> : <Power size={16} />}
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className='text-destructive focus:text-destructive'
          onClick={() => {
            setCurrentRow(inviteCode)
            setOpen('delete')
          }}
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DataTableRowActionMenu>
    </div>
  )
}
