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
  BrowserProxy,
} from '../types'

function createProfileSchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Name is required'))
      .max(128, t('Name must not exceed 128 characters')),
    agent_id: z.number().int().positive(t('Select a browser agent')),
    proxy_id: z.number().int().positive(t('Select a managed proxy')),
    fingerprint_id: z
      .number()
      .int()
      .positive(t('Select a browser fingerprint')),
    runtime_key: z
      .string()
      .trim()
      .min(1, t('Runtime key is required'))
      .regex(/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/, t('Runtime key is invalid')),
    persistent: z.boolean(),
    enabled: z.boolean(),
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
}

function profileDefaults(
  profile: BrowserProfile | null,
  agents: BrowserAgent[],
  proxies: BrowserProxy[],
  fingerprints: BrowserFingerprint[]
): ProfileFormValues {
  return {
    name: profile?.name ?? '',
    agent_id: profile?.agent_id ?? agents[0]?.id ?? 0,
    proxy_id: profile?.proxy_id ?? proxies[0]?.id ?? 0,
    fingerprint_id: profile?.fingerprint_id ?? fingerprints[0]?.id ?? 0,
    runtime_key: profile?.runtime_key ?? '',
    persistent: profile?.persistent ?? true,
    enabled: profile?.enabled ?? true,
  }
}

export function ProfileDialog(props: ProfileDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const profileSchema = useMemo(() => createProfileSchema(t), [t])
  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: profileDefaults(
      props.profile,
      props.agents,
      props.proxies,
      props.fingerprints
    ),
  })
  const selectedAgentId = form.watch('agent_id')
  const selectedRuntimeKey = form.watch('runtime_key')
  const selectedAgent = props.agents.find(
    (agent) => agent.id === selectedAgentId
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
          props.proxies,
          props.fingerprints
        )
      )
    }
  }, [
    form,
    props.open,
    props.profile,
    props.agents,
    props.proxies,
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
  const proxyItems = props.proxies.map((proxy) => ({
    value: String(proxy.id),
    label: proxy.name,
  }))
  const fingerprintItems = props.fingerprints.map((fingerprint) => ({
    value: String(fingerprint.id),
    label: fingerprint.name,
  }))

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
