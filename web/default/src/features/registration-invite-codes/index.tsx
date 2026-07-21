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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'

import { InviteCodeDialogs } from './components/invite-code-dialogs'
import {
  InviteCodesProvider,
  useInviteCodes,
} from './components/invite-codes-provider'
import { InviteCodesTable } from './components/invite-codes-table'

function InviteCodesPageContent() {
  const { t } = useTranslation()
  const { setOpen } = useInviteCodes()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Invitation Codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={() => setOpen('create')}>
            <Plus className='h-4 w-4' />
            {t('Create Invitation Code')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <InviteCodesTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <InviteCodeDialogs />
    </>
  )
}

export function RegistrationInviteCodesPage() {
  return (
    <InviteCodesProvider>
      <InviteCodesPageContent />
    </InviteCodesProvider>
  )
}
