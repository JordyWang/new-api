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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  browserManagementQueryKeys,
  createBrowserFingerprint,
  updateBrowserFingerprint,
} from '../api'
import type { BrowserFingerprint } from '../types'

function parsesAs(
  value: string,
  predicate: (parsed: unknown) => boolean
): boolean {
  try {
    return predicate(JSON.parse(value))
  } catch {
    return false
  }
}

function createFingerprintSchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Name is required'))
      .max(128, t('Name must not exceed 128 characters')),
    user_agent: z.string().trim(),
    locale: z
      .string()
      .trim()
      .min(1, t('Locale is required'))
      .max(64, t('Locale must not exceed 64 characters')),
    timezone: z
      .string()
      .trim()
      .min(1, t('Timezone is required'))
      .max(128, t('Timezone must not exceed 128 characters')),
    viewport_width: z
      .number()
      .int(t('Viewport width must be a whole number'))
      .min(320, t('Viewport width must be between 320 and 7680'))
      .max(7680, t('Viewport width must be between 320 and 7680')),
    viewport_height: z
      .number()
      .int(t('Viewport height must be a whole number'))
      .min(240, t('Viewport height must be between 240 and 4320'))
      .max(4320, t('Viewport height must be between 240 and 4320')),
    payload: z
      .string()
      .refine(
        (value) =>
          parsesAs(
            value,
            (parsed) =>
              typeof parsed === 'object' &&
              parsed !== null &&
              !Array.isArray(parsed)
          ),
        { message: t('Fingerprint payload must be a JSON object') }
      ),
    launch_args: z
      .string()
      .refine(
        (value) =>
          parsesAs(
            value,
            (parsed) =>
              Array.isArray(parsed) &&
              parsed.every((argument) => typeof argument === 'string')
          ),
        { message: t('Launch arguments must be a JSON string array') }
      ),
    environment: z
      .string()
      .refine(
        (value) =>
          parsesAs(
            value,
            (parsed) =>
              typeof parsed === 'object' &&
              parsed !== null &&
              !Array.isArray(parsed) &&
              Object.values(parsed).every((item) => typeof item === 'string')
          ),
        { message: t('Environment must be a JSON string map') }
      ),
    enabled: z.boolean(),
  })
}

type FingerprintFormValues = z.infer<ReturnType<typeof createFingerprintSchema>>

type FingerprintDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  fingerprint: BrowserFingerprint | null
}

function defaultValues(
  fingerprint: BrowserFingerprint | null
): FingerprintFormValues {
  return {
    name: fingerprint?.name ?? '',
    user_agent: fingerprint?.user_agent ?? '',
    locale: fingerprint?.locale ?? 'en-US',
    timezone: fingerprint?.timezone ?? 'UTC',
    viewport_width: fingerprint?.viewport_width ?? 1280,
    viewport_height: fingerprint?.viewport_height ?? 800,
    payload: fingerprint?.payload ?? '{}',
    launch_args: fingerprint?.launch_args ?? '[]',
    environment: fingerprint?.environment ?? '{}',
    enabled: fingerprint?.enabled ?? true,
  }
}

export function FingerprintDialog(props: FingerprintDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const fingerprintSchema = useMemo(() => createFingerprintSchema(t), [t])
  const form = useForm<FingerprintFormValues>({
    resolver: zodResolver(fingerprintSchema),
    defaultValues: defaultValues(null),
  })

  useEffect(() => {
    if (props.open) form.reset(defaultValues(props.fingerprint))
  }, [form, props.fingerprint, props.open])

  const mutation = useMutation({
    mutationFn: (values: FingerprintFormValues) =>
      props.fingerprint
        ? updateBrowserFingerprint(props.fingerprint.id, values)
        : createBrowserFingerprint(values),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.fingerprints(),
      })
      props.onOpenChange(false)
      toast.success(
        props.fingerprint
          ? t('Browser fingerprint updated')
          : t('Browser fingerprint created')
      )
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save browser fingerprint')
      )
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        props.fingerprint
          ? t('Edit browser fingerprint')
          : t('Create browser fingerprint')
      }
      description={t(
        'Fingerprint payload and launch templates are stored centrally and applied by the selected browser agent.'
      )}
      contentClassName='sm:max-w-3xl'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={mutation.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='browser-fingerprint-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending && (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            )}
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='browser-fingerprint-form'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <FieldGroup>
            <div className='grid gap-5 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} autoComplete='off' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='user_agent'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('User Agent')}</FormLabel>
                    <FormControl>
                      <Input {...field} autoComplete='off' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='locale'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Locale')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='en-US' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='timezone'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Timezone')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='America/Los_Angeles' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='viewport_width'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Viewport width')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='viewport_height'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Viewport height')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='payload'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Fingerprint payload')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={6}
                      className='font-mono text-xs'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Written to .new-api/fingerprint.json inside the managed profile.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='launch_args'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Launch arguments')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={5}
                      className='font-mono text-xs'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'JSON string array. Agent-enforced proxy and profile arguments cannot be overridden.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='environment'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Environment variables')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={5}
                      className='font-mono text-xs'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'JSON string map. Loader, PATH, HOME, and other unsafe variables are rejected by the agent.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormDescription>
                      {t('Disabled fingerprints cannot start new OAuth flows.')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FieldGroup>
        </form>
      </Form>
    </Dialog>
  )
}
