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
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

type AgentTokenDialogProps = {
  token: string | null
  onClose: () => void
}

export function AgentTokenDialog(props: AgentTokenDialogProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()

  return (
    <Dialog
      open={Boolean(props.token)}
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Browser agent token')}
      description={t(
        'This token is shown only once. Store it securely on the agent host.'
      )}
      footer={<Button onClick={props.onClose}>{t('Done')}</Button>}
    >
      <div className='flex flex-col gap-4'>
        <Alert>
          <AlertDescription>
            {t(
              'Rotating this token immediately disconnects any agent still using the previous token.'
            )}
          </AlertDescription>
        </Alert>
        <div className='flex items-center gap-2'>
          <Input
            readOnly
            value={props.token ?? ''}
            className='font-mono text-xs'
          />
          <Button
            type='button'
            variant='outline'
            size='icon'
            aria-label={t('Copy token')}
            onClick={() => {
              if (props.token) void copyToClipboard(props.token)
            }}
          >
            <Copy />
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
