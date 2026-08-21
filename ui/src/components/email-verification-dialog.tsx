import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { requestCurrentUserEmailUpdate } from '@/lib/api'
import { translateError } from '@/lib/utils'
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

interface EmailVerificationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  email: string
  isPasswordUser: boolean
  loading: boolean
  onConfirm: (emailOtp: string) => Promise<void>
}

export function EmailVerificationDialog({
  open,
  onOpenChange,
  email,
  isPasswordUser,
  loading,
  onConfirm,
}: EmailVerificationDialogProps) {
  const { t } = useTranslation()
  const [currentPassword, setCurrentPassword] = useState('')
  const [otp, setOtp] = useState('')
  const [otpSent, setOtpSent] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) {
      setCurrentPassword('')
      setOtp('')
      setOtpSent(false)
      setError('')
    }
  }, [open])

  const handleSendOtp = async () => {
    setSending(true)
    setError('')
    try {
      await requestCurrentUserEmailUpdate(
        email,
        isPasswordUser ? currentPassword : undefined
      )
      setOtpSent(true)
      setOtp('')
      toast.success(
        t('accountSettings.profile.emailCodeSent', 'Verification code sent')
      )
    } catch (err) {
      setError(translateError(err, t))
    } finally {
      setSending(false)
    }
  }

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    await onConfirm(otp)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !loading && !sending) onOpenChange(false)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {t(
              'accountSettings.security.emailVerification.title',
              'Verify email'
            )}
          </DialogTitle>
          <DialogDescription>
            {t(
              'accountSettings.security.emailVerification.description',
              'Enter the verification code sent to your email to continue.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {isPasswordUser && (
            <div className="space-y-2">
              <Label htmlFor="email-current-password">
                {t(
                  'accountSettings.password.currentPassword',
                  'Current Password'
                )}
              </Label>
              <Input
                id="email-current-password"
                autoComplete="current-password"
                type="password"
                required
                disabled={otpSent}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="email-otp">
              {t(
                'accountSettings.profile.emailVerificationCode',
                'Verification Code'
              )}
            </Label>
            <div className="flex gap-2">
              <Input
                id="email-otp"
                autoComplete="one-time-code"
                inputMode="numeric"
                required
                disabled={!otpSent}
                value={otp}
                onChange={(e) => setOtp(e.target.value)}
              />
              <Button
                type="button"
                variant="outline"
                disabled={sending || (isPasswordUser && !currentPassword)}
                onClick={handleSendOtp}
              >
                {sending
                  ? t('common.actions.saving', 'Saving...')
                  : t(
                      'accountSettings.profile.sendEmailCode',
                      'Send Verification Code'
                    )}
              </Button>
            </div>
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
              disabled={loading || sending}
              onClick={() => onOpenChange(false)}
            >
              {t('common.actions.cancel', 'Cancel')}
            </Button>
            <Button type="submit" disabled={loading || !otpSent}>
              {loading
                ? t('common.actions.saving', 'Saving...')
                : t('common.actions.confirm', 'Confirm')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
