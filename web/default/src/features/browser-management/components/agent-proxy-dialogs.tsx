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

import {
  browserManagementQueryKeys,
  createBrowserAgent,
  createBrowserProxy,
  updateBrowserAgent,
  updateBrowserProxy,
} from '../api'
import type { BrowserAgent, BrowserProxy } from '../types'

function createAgentSchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Name is required'))
      .max(128, t('Name must not exceed 128 characters')),
    enabled: z.boolean(),
  })
}

function createProxySchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Name is required'))
      .max(128, t('Name must not exceed 128 characters')),
    url: z
      .string()
      .trim()
      .refine(
        (value) => value === '' || /^(https?|socks5h?):\/\//i.test(value),
        {
          message: t('Enter an HTTP, HTTPS, SOCKS5, or SOCKS5H proxy URL'),
        }
      ),
    enabled: z.boolean(),
  })
}

type AgentFormValues = z.infer<ReturnType<typeof createAgentSchema>>
type ProxyFormValues = z.infer<ReturnType<typeof createProxySchema>>

type AgentDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  agent: BrowserAgent | null
  onToken: (token: string) => void
}

type ProxyDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  proxy: BrowserProxy | null
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

export function AgentDialog(props: AgentDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const agentSchema = useMemo(() => createAgentSchema(t), [t])
  const form = useForm<AgentFormValues>({
    resolver: zodResolver(agentSchema),
    defaultValues: { name: '', enabled: true },
  })

  useEffect(() => {
    if (!props.open) return
    form.reset({
      name: props.agent?.name ?? '',
      enabled: props.agent?.enabled ?? true,
    })
  }, [form, props.agent, props.open])

  const mutation = useMutation({
    mutationFn: async (values: AgentFormValues) => {
      if (props.agent) {
        await updateBrowserAgent(props.agent.id, values)
        return null
      }
      return createBrowserAgent(values)
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.agents(),
      })
      props.onOpenChange(false)
      if (result?.token) {
        props.onToken(result.token)
      }
      toast.success(
        props.agent ? t('Browser agent updated') : t('Browser agent created')
      )
    },
    onError: (error) => {
      toast.error(errorMessage(error, t('Failed to save browser agent')))
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.agent ? t('Edit browser agent') : t('Create browser agent')}
      description={t(
        'Agents launch the locally configured Chromium runtime and keep OAuth credentials out of the browser UI.'
      )}
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
            form='browser-agent-form'
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
          id='browser-agent-form'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <FieldGroup>
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
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Disabled agents cannot heartbeat or claim OAuth flows.'
                      )}
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

export function ProxyDialog(props: ProxyDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const proxySchema = useMemo(() => createProxySchema(t), [t])
  const form = useForm<ProxyFormValues>({
    resolver: zodResolver(proxySchema),
    defaultValues: { name: '', url: '', enabled: true },
  })

  useEffect(() => {
    if (!props.open) return
    form.reset({
      name: props.proxy?.name ?? '',
      url: '',
      enabled: props.proxy?.enabled ?? true,
    })
  }, [form, props.open, props.proxy])

  const mutation = useMutation({
    mutationFn: async (values: ProxyFormValues) => {
      if (props.proxy) {
        return updateBrowserProxy(props.proxy.id, values)
      }
      return createBrowserProxy(values)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.proxies(),
      })
      props.onOpenChange(false)
      toast.success(
        props.proxy ? t('Managed proxy updated') : t('Managed proxy created')
      )
    },
    onError: (error) => {
      toast.error(errorMessage(error, t('Failed to save managed proxy')))
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.proxy ? t('Edit managed proxy') : t('Create managed proxy')}
      description={t(
        'The same managed proxy is used by Chromium, OAuth code exchange, token refresh, and channel requests.'
      )}
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
            form='browser-proxy-form'
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
          id='browser-proxy-form'
          onSubmit={form.handleSubmit((values) => {
            if (!props.proxy && !values.url) {
              form.setError('url', {
                type: 'manual',
                message: t('Proxy URL is required'),
              })
              return
            }
            mutation.mutate(values)
          })}
        >
          <FieldGroup>
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
              name='url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Proxy URL')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='password'
                      autoComplete='new-password'
                      placeholder='socks5h://user:password@proxy.example.com:1080'
                    />
                  </FormControl>
                  <FormDescription>
                    {props.proxy
                      ? t('Leave empty to keep the stored proxy URL.')
                      : t('Supports HTTP, HTTPS, SOCKS5, and SOCKS5H.')}
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
                      {t('Disabled proxies cannot start new OAuth flows.')}
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
