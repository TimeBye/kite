import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { SecurityMethod } from '@/lib/api'

interface SecurityOTPDialogProps {
  open: boolean
  method: SecurityMethod
  onOpenChange: (open: boolean) => void
  loading: boolean
  onConfirm: (value: string) => Promise<void>
}

export function SecurityOTPDialog({
  open,
  method,
  onOpenChange,
  loading,
  onConfirm,
}: SecurityOTPDialogProps) {
  const { t } = useTranslation()
  const [value, setValue] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) {
      setValue('')
      setError('')
    }
  }, [open])

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')
    try {
      await onConfirm(value)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('accountSettings.security.changeFailed', 'Security change failed'))
    }
  }

  const isPasswordMode = method === 'password'

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !loading) onOpenChange(false)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isPasswordMode
              ? t(
                  'accountSettings.security.passwordVerification.title',
                  'Verify password'
                )
              : t(
                  'accountSettings.security.emailVerification.title',
                  'Verify email'
                )}
          </DialogTitle>
          <DialogDescription>
            {isPasswordMode
              ? t(
                  'accountSettings.security.passwordVerification.description',
                  'Enter your current password to continue.'
                )
              : t(
                  'accountSettings.security.emailVerification.description',
                  'Enter the verification code sent to your email to continue.'
                )}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="security-otp">
              {isPasswordMode
                ? t(
                    'accountSettings.password.currentPassword',
                    'Current Password'
                  )
                : t(
                    'accountSettings.profile.emailVerificationCode',
                    'Verification Code'
                  )}
            </Label>
            <Input
              id="security-otp"
              autoComplete={isPasswordMode ? 'current-password' : 'one-time-code'}
              inputMode={isPasswordMode ? undefined : 'numeric'}
              type={isPasswordMode ? 'password' : 'text'}
              required
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </div>
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={loading}
              onClick={() => onOpenChange(false)}
            >
              {t('common.actions.cancel', 'Cancel')}
            </Button>
            <Button type="submit" disabled={loading}>
              {loading
                ? t('common.actions.saving', 'Saving...')
                : t('common.actions.continue', 'Continue')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
