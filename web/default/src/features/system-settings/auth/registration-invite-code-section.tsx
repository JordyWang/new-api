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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
import { formatQuota } from '@/lib/format'

import { getRegistrationInviteCode, updateRegistrationInviteCode } from '../api'
import {
  SettingsControlGroup,
  SettingsForm,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import type { RegistrationInviteCode } from '../types'

const MAX_INT32 = 2_147_483_647

const registrationInviteCodeSchema = z.object({
  code: z.string().max(128),
  group: z.string().trim().min(1).max(64),
  expiredTime: z.string(),
  initialQuota: z.coerce.number().int().min(0).max(MAX_INT32),
  maxRegistrations: z.coerce.number().int().min(0).max(MAX_INT32),
})

type RegistrationInviteCodeFormValues = z.infer<
  typeof registrationInviteCodeSchema
>

function formatDateTimeLocal(timestamp: number): string {
  if (!timestamp) return ''
  const date = new Date(timestamp * 1000)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function parseDateTimeLocal(value: string): number {
  if (!value) return 0
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? 0 : Math.floor(timestamp / 1000)
}

function toFormValues(
  config: RegistrationInviteCode
): RegistrationInviteCodeFormValues {
  return {
    code: config.code,
    group: config.group || 'default',
    expiredTime: formatDateTimeLocal(config.expired_time),
    initialQuota: config.initial_quota,
    maxRegistrations: config.max_registrations,
  }
}

export function RegistrationInviteCodeSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const registrationInviteCode = useQuery({
    queryKey: ['registration-invite-code'],
    queryFn: getRegistrationInviteCode,
  })
  const update = useMutation({
    mutationFn: updateRegistrationInviteCode,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to update setting'))
        return
      }
      queryClient.setQueryData(['registration-invite-code'], response)
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update setting'))
    },
  })

  const form = useForm<RegistrationInviteCodeFormValues>({
    resolver: zodResolver(registrationInviteCodeSchema) as Resolver<
      RegistrationInviteCodeFormValues,
      unknown,
      RegistrationInviteCodeFormValues
    >,
    defaultValues: {
      code: '',
      group: 'default',
      expiredTime: '',
      initialQuota: 0,
      maxRegistrations: 0,
    },
  })

  useEffect(() => {
    const config = registrationInviteCode.data?.data
    if (!config) return
    form.reset(toFormValues(config))
  }, [form, registrationInviteCode.data])

  const config = registrationInviteCode.data?.data
  const registeredCount = config?.registered_count ?? 0
  const maxRegistrations = config?.max_registrations ?? 0
  const remaining =
    maxRegistrations > 0
      ? Math.max(maxRegistrations - registeredCount, 0)
      : null
  const { isDirty, isSubmitting } = form.formState

  async function onSubmit(values: RegistrationInviteCodeFormValues) {
    const response = await update.mutateAsync({
      code: values.code.trim(),
      group: values.group.trim(),
      expired_time: parseDateTimeLocal(values.expiredTime),
      initial_quota: values.initialQuota,
      max_registrations: values.maxRegistrations,
    })
    if (response.success && response.data) {
      form.reset(toFormValues(response.data))
    }
  }

  return (
    <SettingsSection title={t('Invitation Code')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={
              update.isPending ||
              isSubmitting ||
              registrationInviteCode.isLoading
            }
            isSaveDisabled={!isDirty || registrationInviteCode.isLoading}
            saveLabel='Save Settings'
          />

          <FormField
            control={form.control}
            name='code'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Invitation Code')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('Invitation Code')} {...field} />
                </FormControl>
                <FormDescription>
                  {t('Invitation code is required for all new registrations.')}
                </FormDescription>
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
                  <Input placeholder='default' {...field} />
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
                  <Input type='number' min={0} max={MAX_INT32} {...field} />
                </FormControl>
                <FormDescription>
                  {t('Initial quota given to new users ({{formattedQuota}})', {
                    formattedQuota: formatQuota(field.value),
                  })}
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
                  <Input type='datetime-local' {...field} />
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
                  <Input type='number' min={0} max={MAX_INT32} {...field} />
                </FormControl>
                <FormDescription>{t('Unlimited')} = 0</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <SettingsControlGroup>
            <div className='flex flex-wrap gap-x-6 gap-y-2 text-sm'>
              <span>
                {t('Registrations Used')}: {registeredCount}
              </span>
              <span>
                {t('Remaining')}:{' '}
                {remaining === null ? t('Unlimited') : remaining}
              </span>
            </div>
            {!form.watch('code').trim() && (
              <p className='text-destructive text-xs'>
                {t('An empty invitation code blocks all new registrations.')}
              </p>
            )}
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
