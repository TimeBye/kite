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

interface SecurityOTPDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  loading: boolean
  onConfirm: (otp: string) => Promise<void>
}

export function SecurityOTPDialog({
  open,
  onOpenChange,
  loading,
  onConfirm,
}: SecurityOTPDialogProps) {
  const { t } = useTranslation()
  const [otp, setOtp] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) {
      setOtp('')
      setError('')
    }
  }, [open])

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')
    try {
      await onConfirm(otp)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('accountSettings.security.changeFailed', 'Security change failed'))
    }
  }

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
          <div className="space-y-2">
            <Label htmlFor="security-otp">
              {t(
                'accountSettings.profile.emailVerificationCode',
                'Verification Code'
              )}
            </Label>
            <Input
              id="security-otp"
              autoComplete="one-time-code"
              inputMode="numeric"
              required
              value={otp}
              onChange={(e) => setOtp(e.target.value)}
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
