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
import { Combobox } from '@/components/ui/combobox'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  browserManagementQueryKeys,
  createBrowserProfile,
  updateBrowserProfile,
} from '../api'
import type {
  BrowserAgent,
  BrowserFingerprint,
  BrowserProfile,
  BrowserProfileChannel,
  BrowserProxy,
} from '../types'

function createProfileSchema(t: TFunction, channels: BrowserProfileChannel[]) {
  return z
    .object({
      name: z
        .string()
        .trim()
        .min(1, t('Name is required'))
        .max(128, t('Name must not exceed 128 characters')),
      channel_id: z.number().int().positive().nullable(),
      agent_id: z.number().int().positive(t('Select a browser agent')),
      proxy_id: z.number().int().positive(t('Select a managed proxy')),
      fingerprint_id: z.number().int().min(0),
      auto_generate_fingerprint: z.boolean(),
      runtime_key: z
        .string()
        .trim()
        .min(1, t('Runtime key is required'))
        .regex(
          /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/,
          t('Runtime key is invalid')
        ),
      persistent: z.boolean(),
      enabled: z.boolean(),
    })
    .superRefine((values, context) => {
      if (!values.auto_generate_fingerprint && values.fingerprint_id <= 0) {
        context.addIssue({
          code: 'custom',
          path: ['fingerprint_id'],
          message: t('Select a browser fingerprint'),
        })
      }
      if (!values.channel_id) return
      const channel = channels.find((item) => item.id === values.channel_id)
      if (
        channel?.browser_proxy_id &&
        channel.browser_proxy_id !== values.proxy_id
      ) {
        context.addIssue({
          code: 'custom',
          path: ['proxy_id'],
          message: t('The profile and channel must use the same managed proxy'),
        })
      }
    })
}

type ProfileFormValues = z.infer<ReturnType<typeof createProfileSchema>>

type ProfileDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile: BrowserProfile | null
  agents: BrowserAgent[]
  proxies: BrowserProxy[]
  fingerprints: BrowserFingerprint[]
  channels: BrowserProfileChannel[]
}

function profileDefaults(
  profile: BrowserProfile | null,
  agents: BrowserAgent[],
  proxies: BrowserProxy[],
  fingerprints: BrowserFingerprint[]
): ProfileFormValues {
  const availableProxy = proxies.find(
    (proxy) =>
      proxy.enabled &&
      (proxy.max_channel_accounts === 0 ||
        proxy.channel_account_count < proxy.max_channel_accounts)
  )
  return {
    name: profile?.name ?? '',
    channel_id: profile?.channel_id ?? null,
    agent_id: profile?.agent_id ?? agents[0]?.id ?? 0,
    proxy_id: profile?.proxy_id ?? availableProxy?.id ?? 0,
    fingerprint_id: profile?.fingerprint_id ?? fingerprints[0]?.id ?? 0,
    auto_generate_fingerprint: profile === null,
    runtime_key: profile?.runtime_key ?? '',
    persistent: profile?.persistent ?? true,
    enabled: profile?.enabled ?? true,
  }
}

export function ProfileDialog(props: ProfileDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const profileSchema = useMemo(
    () => createProfileSchema(t, props.channels),
    [props.channels, t]
  )
  const orderedProxies = useMemo(
    () =>
      [...props.proxies].sort((left, right) => {
        const leftAvailable =
          left.enabled &&
          (left.max_channel_accounts === 0 ||
            left.channel_account_count < left.max_channel_accounts)
        const rightAvailable =
          right.enabled &&
          (right.max_channel_accounts === 0 ||
            right.channel_account_count < right.max_channel_accounts)
        if (leftAvailable !== rightAvailable) return leftAvailable ? -1 : 1
        if (left.profile_count !== right.profile_count) {
          return left.profile_count - right.profile_count
        }
        if (left.channel_account_count !== right.channel_account_count) {
          return left.channel_account_count - right.channel_account_count
        }
        return left.id - right.id
      }),
    [props.proxies]
  )
  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: profileDefaults(
      props.profile,
      props.agents,
      orderedProxies,
      props.fingerprints
    ),
  })
  const selectedAgentId = form.watch('agent_id')
  const selectedRuntimeKey = form.watch('runtime_key')
  const selectedChannelId = form.watch('channel_id')
  const autoGenerateFingerprint = form.watch('auto_generate_fingerprint')
  const selectedAgent = props.agents.find(
    (agent) => agent.id === selectedAgentId
  )
  const selectedChannel = props.channels.find(
    (channel) => channel.id === selectedChannelId
  )
  const runtimeOptions = useMemo(() => {
    const runtimes = new Set(selectedAgent?.runtimes ?? [])
    if (selectedRuntimeKey) runtimes.add(selectedRuntimeKey)
    return [...runtimes].sort().map((runtime) => ({
      value: runtime,
      label: runtime,
    }))
  }, [selectedAgent, selectedRuntimeKey])

  useEffect(() => {
    if (props.open) {
      form.reset(
        profileDefaults(
          props.profile,
          props.agents,
          orderedProxies,
          props.fingerprints
        )
      )
    }
  }, [
    form,
    props.open,
    props.profile,
    props.agents,
    orderedProxies,
    props.fingerprints,
  ])

  useEffect(() => {
    if (!props.open || props.profile) return
    const currentRuntime = form.getValues('runtime_key')
    if (!selectedAgent?.runtimes.includes(currentRuntime)) {
      form.setValue('runtime_key', selectedAgent?.runtimes[0] ?? '')
    }
  }, [form, props.open, props.profile, selectedAgent])

  const mutation = useMutation({
    mutationFn: (values: ProfileFormValues) =>
      props.profile
        ? updateBrowserProfile(props.profile.id, values)
        : createBrowserProfile(values),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: browserManagementQueryKeys.profiles(),
      })
      props.onOpenChange(false)
      toast.success(
        props.profile
          ? t('Browser profile updated')
          : t('Browser profile created')
      )
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save browser profile')
      )
    },
  })

  const agentItems = props.agents.map((agent) => ({
    value: String(agent.id),
    label: `${agent.name}${agent.online ? '' : ` (${t('Offline')})`}`,
  }))
  const proxyItems = orderedProxies.map((proxy) => {
    const atCapacity =
      proxy.max_channel_accounts > 0 &&
      proxy.channel_account_count >= proxy.max_channel_accounts
    const isCurrent = proxy.id === props.profile?.proxy_id
    const requiredByChannel = proxy.id === selectedChannel?.browser_proxy_id
    return {
      value: String(proxy.id),
      label: `${proxy.name} · ${t('Profiles')}: ${proxy.profile_count} · ${t('Channel accounts')}: ${proxy.channel_account_count}/${proxy.max_channel_accounts || t('Unlimited')}`,
      disabled:
        !proxy.enabled || (atCapacity && !isCurrent && !requiredByChannel),
    }
  })
  const fingerprintItems = props.fingerprints.map((fingerprint) => ({
    value: String(fingerprint.id),
    label: fingerprint.name,
  }))
  const channelItems = [
    {
      value: 'unbound',
      label: t('Unbound — bind automatically after first OAuth login'),
      disabled: false,
    },
    ...props.channels.map((channel) => ({
      value: String(channel.id),
      label:
        channel.browser_profile_id &&
        channel.browser_profile_id !== props.profile?.id
          ? `${channel.name} (${t('Bound to another profile')})`
          : channel.name,
      disabled: Boolean(
        channel.browser_profile_id &&
        channel.browser_profile_id !== props.profile?.id
      ),
    })),
  ]

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        props.profile ? t('Edit browser profile') : t('Create browser profile')
      }
      description={t(
        'A profile pins one agent, runtime, managed proxy, fingerprint, and browser data directory.'
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
            form='browser-profile-form'
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
          id='browser-profile-form'
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
              name='channel_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Codex channel')}</FormLabel>
                  <Select
                    items={channelItems}
                    value={field.value ? String(field.value) : 'unbound'}
                    onValueChange={(value) => {
                      const channelId =
                        value === 'unbound' ? null : Number(value)
                      field.onChange(channelId)
                      const channel = props.channels.find(
                        (item) => item.id === channelId
                      )
                      if (channel?.browser_proxy_id) {
                        form.setValue('proxy_id', channel.browser_proxy_id, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }
                    }}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {channelItems.map((item) => (
                          <SelectItem
                            key={item.value}
                            value={item.value}
                            disabled={item.disabled}
                          >
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {selectedChannel?.browser_proxy_id
                      ? t(
                          'This channel requires managed proxy #{{proxyId}}; the profile proxy is kept in sync.',
                          { proxyId: selectedChannel.browser_proxy_id }
                        )
                      : t(
                          'Leave unbound when this profile will create its channel through the first OAuth login.'
                        )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='agent_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Browser agent')}</FormLabel>
                  <Select
                    items={agentItems}
                    value={String(field.value || '')}
                    onValueChange={(value) => {
                      const nextAgentId = Number(value)
                      field.onChange(nextAgentId)
                      const nextAgent = props.agents.find(
                        (agent) => agent.id === nextAgentId
                      )
                      const currentRuntime = form.getValues('runtime_key')
                      if (!nextAgent?.runtimes.includes(currentRuntime)) {
                        form.setValue(
                          'runtime_key',
                          nextAgent?.runtimes[0] ?? '',
                          {
                            shouldDirty: true,
                            shouldValidate: true,
                          }
                        )
                      }
                    }}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {agentItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='runtime_key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Runtime key')}</FormLabel>
                  <FormControl>
                    <Combobox
                      options={runtimeOptions}
                      value={field.value}
                      onValueChange={(value) => field.onChange(value ?? '')}
                      allowCustomValue
                      openOnFocus
                      placeholder={t('Select an advertised Chromium runtime')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Executable paths stay on the agent host; new-api stores only this runtime key.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='proxy_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Managed proxy')}</FormLabel>
                  <Select
                    items={proxyItems}
                    value={String(field.value || '')}
                    onValueChange={(value) => field.onChange(Number(value))}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {proxyItems.map((item) => (
                          <SelectItem
                            key={item.value}
                            value={item.value}
                            disabled={item.disabled}
                          >
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='auto_generate_fingerprint'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>
                      {t('Automatically generate fingerprint')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Generate one coherent fingerprint when the Profile is created and reuse it for every launch.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      disabled={Boolean(props.profile)}
                      onCheckedChange={(checked) => {
                        field.onChange(checked)
                        if (!checked && form.getValues('fingerprint_id') <= 0) {
                          form.setValue(
                            'fingerprint_id',
                            props.fingerprints[0]?.id ?? 0,
                            { shouldValidate: true }
                          )
                        }
                      }}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            {!autoGenerateFingerprint && (
              <FormField
                control={form.control}
                name='fingerprint_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Browser fingerprint')}</FormLabel>
                    <Select
                      items={fingerprintItems}
                      value={String(field.value || '')}
                      onValueChange={(value) => field.onChange(Number(value))}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {fingerprintItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name='persistent'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>{t('Persistent profile')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Keep cookies and browser state between OAuth sessions on the agent host.'
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
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Disabled profiles are hidden from the channel OAuth flow.'
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
