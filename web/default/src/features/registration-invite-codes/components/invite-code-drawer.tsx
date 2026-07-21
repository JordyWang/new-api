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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { formatQuota } from '@/lib/format'

import {
  createRegistrationInviteCode,
  getRegistrationInviteCode,
  updateRegistrationInviteCode,
} from '../api'
import {
  inviteCodeFormToPayload,
  inviteCodeToFormValues,
  REGISTRATION_INVITE_CODE_FORM_DEFAULTS,
  registrationInviteCodeFormSchema,
  type RegistrationInviteCodeFormValues,
} from '../lib/invite-code-form'
import type { RegistrationInviteCode } from '../types'
import { useInviteCodes } from './invite-codes-provider'

type InviteCodeDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: RegistrationInviteCode
}

export function InviteCodeDrawer(props: InviteCodeDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useInviteCodes()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const isUpdate = Boolean(props.currentRow)
  const form = useForm<RegistrationInviteCodeFormValues>({
    resolver: zodResolver(registrationInviteCodeFormSchema),
    defaultValues: REGISTRATION_INVITE_CODE_FORM_DEFAULTS,
  })

  useEffect(() => {
    if (!props.open) return
    if (!props.currentRow) {
      form.reset(REGISTRATION_INVITE_CODE_FORM_DEFAULTS)
      return
    }
    getRegistrationInviteCode(props.currentRow.id)
      .then((response) => {
        if (response.success && response.data) {
          form.reset(inviteCodeToFormValues(response.data))
        }
      })
      .catch(() => toast.error(t('Failed to load invitation codes')))
  }, [form, props.currentRow, props.open, t])

  async function onSubmit(values: RegistrationInviteCodeFormValues) {
    setIsSubmitting(true)
    try {
      const payload = inviteCodeFormToPayload(values)
      const response = props.currentRow
        ? await updateRegistrationInviteCode(props.currentRow.id, payload)
        : await createRegistrationInviteCode(payload)
      if (!response.success) {
        toast.error(response.message || t('Failed to save invitation code'))
        return
      }
      toast.success(
        t(
          props.currentRow
            ? 'Invitation code updated successfully'
            : 'Invitation code created successfully'
        )
      )
      props.onOpenChange(false)
      triggerRefresh()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t(isUpdate ? 'Update Invitation Code' : 'Create Invitation Code')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Configure the code, user group, initial quota, expiration, and registration limit.'
            )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='registration-invite-code-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Invitation Code')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        autoComplete='off'
                        placeholder={t('Enter invitation code')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('User Group')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='default' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='initialQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('New User Quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Initial quota given to new users ({{formattedQuota}})',
                        {
                          formattedQuota: formatQuota(field.value),
                        }
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='expiredTime'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <FormControl>
                      <DateTimePicker
                        value={field.value}
                        onChange={field.onChange}
                        placeholder={t('Never expires')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Leave empty for never expires')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='maxRegistrations'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum Registrations')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Set to 0 for unlimited registrations')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type='submit'
            form='registration-invite-code-form'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
