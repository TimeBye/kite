import { useEffect, useState } from 'react'
import { Mail } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SMTPSettingUpdateRequest,
  sendTestEmail,
  updateSMTPSetting,
  useSMTPSetting,
} from '@/lib/api'
import { translateError } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface SMTPFormData {
  enabled: boolean
  host: string
  port: number
  username: string
  password: string
  passwordConfigured: boolean
  fromEmail: string
  useTLS: boolean
  envManaged: boolean
}

export function SMTPManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data, isLoading } = useSMTPSetting()
  const [formData, setFormData] = useState<SMTPFormData>({
    enabled: false,
    host: '',
    port: 587,
    username: '',
    password: '',
    passwordConfigured: false,
    fromEmail: '',
    useTLS: true,
    envManaged: false,
  })
  const [testEmail, setTestEmail] = useState('')

  useEffect(() => {
    if (!data) return
    setFormData({
      enabled: data.enabled,
      host: data.host,
      port: data.port,
      username: data.username,
      password: '',
      passwordConfigured: data.passwordConfigured,
      fromEmail: data.fromEmail,
      useTLS: data.useTLS,
      envManaged: data.envManaged,
    })
  }, [data])

  const saveMutation = useMutation({
    mutationFn: (data: SMTPSettingUpdateRequest) => updateSMTPSetting(data),
    onSuccess: () => {
      toast.success(t('common.actions.saved', 'Saved'))
      queryClient.invalidateQueries({ queryKey: ['smtp-setting'] })
    },
    onError: (error) => {
      toast.error(translateError(error, t))
    },
  })

  const testEmailMutation = useMutation({
    mutationFn: (to: string) => sendTestEmail(to),
    onSuccess: () => {
      toast.success(
        t('settings.smtp.testEmailSent', 'Test email sent successfully')
      )
    },
    onError: (error) => {
      toast.error(
        translateError(error, t)
      )
    },
  })

  const handleSave = () => {
    const updates: SMTPSettingUpdateRequest = {
      enabled: formData.enabled,
      host: formData.host,
      port: formData.port,
      username: formData.username,
      fromEmail: formData.fromEmail,
      useTLS: formData.useTLS,
    }
    if (formData.password) {
      updates.password = formData.password
    }
    saveMutation.mutate(updates)
  }

  const handleSendTest = () => {
    if (!testEmail.trim()) return
    testEmailMutation.mutate(testEmail.trim())
  }

  if (isLoading) {
    return null
  }

  const envManaged = formData.envManaged

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Mail className="h-5 w-5" />
          {t('settings.smtp.title', 'SMTP')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {envManaged && (
          <p className="text-sm text-muted-foreground">
            {t(
              'settings.smtp.envManaged',
              'SMTP is managed by environment variables and cannot be modified through the UI'
            )}
          </p>
        )}
        <div className="flex items-center justify-between">
          <Label htmlFor="smtp-enabled">
            {t('settings.smtp.enabled', 'Enabled')}
          </Label>
          <Switch
            id="smtp-enabled"
            checked={formData.enabled}
            disabled={envManaged}
            onCheckedChange={(checked) =>
              setFormData({ ...formData, enabled: checked })
            }
          />
        </div>
        {formData.enabled && (
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="smtp-host">
                {t('settings.smtp.host', 'Host')}
              </Label>
              <Input
                id="smtp-host"
                placeholder={t('settings.smtp.hostPlaceholder', 'smtp.example.com')}
                disabled={envManaged}
                value={formData.host}
                onChange={(e) => setFormData({ ...formData, host: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-port">
                {t('settings.smtp.port', 'Port')}
              </Label>
              <Input
                id="smtp-port"
                type="number"
                disabled={envManaged}
                value={formData.port}
                onChange={(e) =>
                  setFormData({ ...formData, port: parseInt(e.target.value) || 587 })
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-username">
                {t('settings.smtp.username', 'Username')}
              </Label>
              <Input
                id="smtp-username"
                placeholder={t('settings.smtp.usernamePlaceholder', 'user@example.com')}
                disabled={envManaged}
                value={formData.username}
                onChange={(e) =>
                  setFormData({ ...formData, username: e.target.value })
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-password">
                {t('settings.smtp.password', 'Password')}
              </Label>
              <Input
                id="smtp-password"
                type="password"
                placeholder={
                  formData.passwordConfigured
                    ? t('settings.smtp.passwordPlaceholder', 'Leave blank to keep current password')
                    : ''
                }
                disabled={envManaged}
                value={formData.password}
                onChange={(e) =>
                  setFormData({ ...formData, password: e.target.value })
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="smtp-from-email">
                {t('settings.smtp.fromEmail', 'From Email')}
              </Label>
              <Input
                id="smtp-from-email"
                type="email"
                placeholder={t('settings.smtp.fromEmailPlaceholder', 'noreply@example.com')}
                disabled={envManaged}
                value={formData.fromEmail}
                onChange={(e) =>
                  setFormData({ ...formData, fromEmail: e.target.value })
                }
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="smtp-use-tls">
                {t('settings.smtp.useTLS', 'Use TLS')}
              </Label>
              <Switch
                id="smtp-use-tls"
                checked={formData.useTLS}
                disabled={envManaged}
                onCheckedChange={(checked) =>
                  setFormData({ ...formData, useTLS: checked })
                }
              />
            </div>
          </div>
        )}
        {!envManaged && (
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={saveMutation.isPending}>
              {saveMutation.isPending
                ? t('common.actions.saving', 'Saving...')
                : t('settings.smtp.save', 'Save')}
            </Button>
          </div>
        )}
        <div className="space-y-2 border-t pt-4">
          <Label>{t('settings.smtp.testEmail', 'Test Email')}</Label>
          <p className="text-sm text-muted-foreground">
            {t(
              'settings.smtp.testEmailDescription',
              'Send a test email to verify SMTP configuration'
            )}
          </p>
          <div className="flex gap-2">
            <Input
              placeholder={t(
                'settings.smtp.testEmailRecipientPlaceholder',
                'test@example.com'
              )}
              value={testEmail}
              onChange={(e) => setTestEmail(e.target.value)}
            />
            <Button
              variant="outline"
              onClick={handleSendTest}
              disabled={testEmailMutation.isPending || !testEmail.trim()}
            >
              {testEmailMutation.isPending
                ? t('common.actions.sending', 'Sending...')
                : t('settings.smtp.sendTest', 'Send Test')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
