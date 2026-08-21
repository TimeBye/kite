import { useEffect, useState, type FormEvent } from 'react'
import { useAuth } from '@/contexts/auth-context'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Fingerprint, KeyRound, ShieldCheck, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  beginCurrentUserPasskeyRegistration,
  changeCurrentUserPassword,
  deleteCurrentUserKubeconfigToken,
  deleteCurrentUserPasskey,
  disableCurrentUserMFA,
  enableCurrentUserMFA,
  finishCurrentUserPasskeyRegistration,
  listCurrentUserPasskeys,
  requestCurrentUserSecurityOTP,
  setupCurrentUserMFA,
  updateCurrentUser,
  useCurrentUserKubeconfigTokens,
  type MFASetupResponse,
  type PasskeyCredential,
} from '@/lib/api'
import { createPasskeyCredential } from '@/lib/webauthn'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { KubeconfigTokenList } from '@/components/kubeconfig-token-list'
import { EmailVerificationDialog } from '@/components/email-verification-dialog'
import { SecurityOTPDialog } from '@/components/security-otp-dialog'

interface AccountSettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type SecurityAction =
  | 'mfa-setup'
  | 'mfa-enable'
  | 'mfa-disable'
  | 'passkey-add'
  | 'passkey-delete'

export function AccountSettingsDialog({
  open,
  onOpenChange,
}: AccountSettingsDialogProps) {
  const { t } = useTranslation()
  const { user, checkAuth, mfaEnabled, passkeyLoginEnabled } = useAuth()
  const queryClient = useQueryClient()
  const { data: kubeconfigTokens = [], isLoading: kubeconfigTokensLoading } =
    useCurrentUserKubeconfigTokens(open)
  const deleteKubeconfigMutation = useMutation({
    mutationFn: deleteCurrentUserKubeconfigToken,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['current-user-kubeconfig-tokens'],
      })
      toast.success(
        t('kubeconfigTokens.deleteSuccess', 'Kubeconfig token deleted')
      )
    },
    onError: (error) => toast.error(error.message),
  })

  // Profile state
  const [nickname, setNickname] = useState('')
  const [email, setEmail] = useState('')
  const [avatarURL, setAvatarURL] = useState('')
  const [profileError, setProfileError] = useState('')
  const [savingProfile, setSavingProfile] = useState(false)
  const [emailDialogOpen, setEmailDialogOpen] = useState(false)

  // Password state
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)

  // MFA state
  const [mfaSetup, setMFASetup] = useState<MFASetupResponse | null>(null)
  const [mfaCode, setMFACode] = useState('')
  const [mfaError, setMFAError] = useState('')
  const [savingMFA, setSavingMFA] = useState(false)

  // Passkey state
  const [passkeys, setPasskeys] = useState<PasskeyCredential[]>([])
  const [passkeyName, setPasskeyName] = useState('')
  const [passkeyError, setPasskeyError] = useState('')
  const [savingPasskey, setSavingPasskey] = useState(false)

  // Security OTP dialog state
  const [securityAction, setSecurityAction] =
    useState<SecurityAction | null>(null)
  const [passkeyToDelete, setPasskeyToDelete] = useState<number | null>(null)

  useEffect(() => {
    if (!open) return

    setNickname(user?.name || '')
    setEmail(user?.email || '')
    setAvatarURL(user?.avatar_url || '')
    setProfileError('')
    setCurrentPassword('')
    setNewPassword('')
    setConfirmPassword('')
    setPasswordError('')
    setMFASetup(null)
    setMFACode('')
    setMFAError('')
    setPasskeys([])
    setPasskeyName('')
    setPasskeyError('')
    setSecurityAction(null)
    setPasskeyToDelete(null)

    if (!passkeyLoginEnabled) return

    let cancelled = false
    listCurrentUserPasskeys()
      .then((items) => {
        if (!cancelled) setPasskeys(items)
      })
      .catch((error) => {
        if (cancelled) return
        setPasskeyError(
          error instanceof Error
            ? error.message
            : t(
                'accountSettings.security.passkeys.loadError',
                'Failed to load passkeys'
              )
        )
      })

    return () => {
      cancelled = true
    }
  }, [open, passkeyLoginEnabled, user?.name, user?.email, user?.avatar_url, t])

  if (!user) return null

  const isPasswordUser = !user.provider || user.provider === 'password'
  const canChangeName = !user.nameSource
  const canChangeEmail = !user.emailSource
  const canChangeAvatarURL = !user.avatarUrlSource
  const hasManagedProfileField =
    !canChangeName || !canChangeEmail || !canChangeAvatarURL
  const hasEditableProfileField =
    canChangeName || canChangeEmail || canChangeAvatarURL

  const mfaControlsDisabled = !mfaEnabled || savingMFA
  const passkeyControlsDisabled = !passkeyLoginEnabled || savingPasskey

  // ── Profile ──

  const handleSaveProfile = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setProfileError('')

    const emailChanged = email.trim() !== (user.email || '').trim()
    if (emailChanged) {
      setEmailDialogOpen(true)
      return
    }

    setSavingProfile(true)
    try {
      await updateCurrentUser({
        name: nickname.trim(),
        email: email.trim(),
        avatar_url: avatarURL.trim(),
      })
      await checkAuth()
      toast.success(t('accountSettings.profile.saved', 'Account updated'))
    } catch (error) {
      setProfileError(
        error instanceof Error
          ? error.message
          : t('accountSettings.profile.error', 'Failed to update account')
      )
    } finally {
      setSavingProfile(false)
    }
  }

  const handleConfirmEmailChange = async (otp: string) => {
    setSavingProfile(true)
    setProfileError('')
    try {
      await updateCurrentUser({
        name: nickname.trim(),
        email: email.trim(),
        avatar_url: avatarURL.trim(),
        email_otp: otp,
      })
      await checkAuth()
      setEmailDialogOpen(false)
      toast.success(t('accountSettings.profile.saved', 'Account updated'))
    } catch (error) {
      setProfileError(
        error instanceof Error
          ? error.message
          : t('accountSettings.profile.error', 'Failed to update account')
      )
      throw error
    } finally {
      setSavingProfile(false)
    }
  }

  // ── Password ──

  const handleChangePassword = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setPasswordError('')

    if (newPassword !== confirmPassword) {
      setPasswordError(
        t('accountSettings.password.mismatch', 'New passwords do not match')
      )
      return
    }

    setSavingPassword(true)
    try {
      await changeCurrentUserPassword(currentPassword, newPassword)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      toast.success(t('accountSettings.password.changed', 'Password changed'))
    } catch (error) {
      setPasswordError(
        error instanceof Error
          ? error.message
          : t('accountSettings.password.error', 'Failed to change password')
      )
    } finally {
      setSavingPassword(false)
    }
  }

  // ── Security OTP (MFA + Passkey) ──

  const openSecurityOTP = async (action: SecurityAction, passkeyID?: number) => {
    const purpose =
      action === 'mfa-setup'
        ? 'setup_mfa'
        : action === 'mfa-disable'
          ? 'disable_mfa'
          : action === 'passkey-add'
            ? 'add_passkey'
            : action === 'passkey-delete'
              ? 'delete_passkey'
              : 'enable_mfa'
    try {
      await requestCurrentUserSecurityOTP(purpose)
      setSecurityAction(action)
      setPasskeyToDelete(passkeyID ?? null)
      toast.success(
        t('accountSettings.security.emailCodeSent', 'Verification code sent')
      )
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('accountSettings.security.sendCodeError', 'Failed to send verification code')
      if (action.startsWith('mfa')) setMFAError(message)
      else setPasskeyError(message)
    }
  }

  const handleConfirmSecurityOTP = async (otp: string) => {
    if (!securityAction) return
    const action = securityAction
    const isMFAAction = action.startsWith('mfa')

    if (isMFAAction) {
      setSavingMFA(true)
      setMFAError('')
    } else {
      setSavingPasskey(true)
      setPasskeyError('')
    }

    try {
      if (action === 'mfa-setup') {
        setMFASetup(await setupCurrentUserMFA(otp))
        setMFACode('')
      } else if (action === 'mfa-enable') {
        await enableCurrentUserMFA(mfaCode, otp)
        await checkAuth()
        setMFASetup(null)
        setMFACode('')
        toast.success(t('accountSettings.security.mfa.enabled', 'MFA enabled'))
      } else if (action === 'mfa-disable') {
        await disableCurrentUserMFA(mfaCode, otp)
        await checkAuth()
        setMFACode('')
        toast.success(t('accountSettings.security.mfa.disabled', 'MFA disabled'))
      } else if (action === 'passkey-add') {
        const options = await beginCurrentUserPasskeyRegistration(passkeyName, otp)
        setSecurityAction(null)
        const credential = await createPasskeyCredential(options)
        await finishCurrentUserPasskeyRegistration(credential)
        setPasskeyName('')
        setPasskeys(await listCurrentUserPasskeys())
        toast.success(t('accountSettings.security.passkeys.added', 'Passkey added'))
        return
      } else if (passkeyToDelete) {
        await deleteCurrentUserPasskey(passkeyToDelete, otp)
        setPasskeys(await listCurrentUserPasskeys())
        toast.success(t('accountSettings.security.passkeys.deleted', 'Passkey deleted'))
      }
      setSecurityAction(null)
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('accountSettings.security.changeFailed', 'Security change failed')
      if (isMFAAction) setMFAError(message)
      else setPasskeyError(message)
      throw error
    } finally {
      if (isMFAAction) setSavingMFA(false)
      else setSavingPasskey(false)
    }
  }

  const securityOTPLoading = securityAction?.startsWith('mfa')
    ? savingMFA
    : savingPasskey

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>
              {t('accountSettings.title', 'Account Settings')}
            </DialogTitle>
          </DialogHeader>

          <Tabs defaultValue="profile" className="gap-4">
            <TabsList
              className={`grid w-full ${isPasswordUser ? 'grid-cols-3' : 'grid-cols-2'}`}
            >
              <TabsTrigger value="profile">
                {t('accountSettings.tabs.profile', 'Profile')}
              </TabsTrigger>
              {isPasswordUser && (
                <TabsTrigger value="password">
                  {t('accountSettings.tabs.password', 'Password')}
                </TabsTrigger>
              )}
              <TabsTrigger value="security">
                {t('accountSettings.tabs.security', 'Security')}
              </TabsTrigger>
            </TabsList>

            {/* Profile */}
            <TabsContent value="profile">
              <form onSubmit={handleSaveProfile} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="account-nickname">
                    {t('accountSettings.profile.nickname', 'Nickname')}
                  </Label>
                  <Input
                    id="account-nickname"
                    autoComplete="name"
                    disabled={!canChangeName}
                    value={nickname}
                    onChange={(e) => setNickname(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="account-email">
                    {t('common.fields.email', 'Email')}
                  </Label>
                  <Input
                    id="account-email"
                    autoComplete="email"
                    type="email"
                    disabled={!canChangeEmail}
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="account-avatar-url">
                    {t('common.fields.avatarUrl', 'Avatar URL')}
                  </Label>
                  <Input
                    id="account-avatar-url"
                    autoComplete="url"
                    disabled={!canChangeAvatarURL}
                    value={avatarURL}
                    onChange={(e) => setAvatarURL(e.target.value)}
                  />
                </div>
                {profileError && (
                  <Alert variant="destructive">
                    <AlertDescription>{profileError}</AlertDescription>
                  </Alert>
                )}
                {hasEditableProfileField && (
                  <div className="flex justify-end">
                    <Button type="submit" disabled={savingProfile}>
                      {savingProfile
                        ? t('common.actions.saving', 'Saving...')
                        : t('accountSettings.profile.saveButton', 'Save Profile')}
                    </Button>
                  </div>
                )}
              </form>
              {hasManagedProfileField && (
                <p className="mt-4 text-xs text-muted-foreground">
                  {t(
                    'accountSettings.profile.managedNotice',
                    'The read-only information above is managed by your identity provider.'
                  )}
                </p>
              )}
            </TabsContent>

            {/* Password */}
            {isPasswordUser && (
              <TabsContent value="password">
                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <KeyRound className="h-4 w-4" />
                    <span>{t('common.fields.password', 'Password')}</span>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="account-current-password">
                      {t(
                        'accountSettings.password.currentPassword',
                        'Current Password'
                      )}
                    </Label>
                    <Input
                      id="account-current-password"
                      autoComplete="current-password"
                      type="password"
                      required
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="account-new-password">
                      {t('accountSettings.password.newPassword', 'New Password')}
                    </Label>
                    <Input
                      id="account-new-password"
                      autoComplete="new-password"
                      type="password"
                      required
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="account-confirm-password">
                      {t(
                        'accountSettings.password.confirmNewPassword',
                        'Confirm New Password'
                      )}
                    </Label>
                    <Input
                      id="account-confirm-password"
                      autoComplete="new-password"
                      type="password"
                      required
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                    />
                  </div>
                  {passwordError && (
                    <Alert variant="destructive">
                      <AlertDescription>{passwordError}</AlertDescription>
                    </Alert>
                  )}
                  <div className="flex justify-end">
                    <Button type="submit" disabled={savingPassword}>
                      {savingPassword
                        ? t('common.actions.saving', 'Saving...')
                        : t(
                            'accountSettings.password.changeButton',
                            'Change Password'
                          )}
                    </Button>
                  </div>
                </form>
              </TabsContent>
            )}

            {/* Security */}
            <TabsContent value="security">
              <div className="space-y-6">
                {/* MFA */}
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-sm font-medium">
                      <ShieldCheck className="h-4 w-4" />
                      <span>{t('authenticationManagement.mfa.title', 'MFA')}</span>
                    </div>
                    <Badge variant={user.mfa_enabled ? 'default' : 'secondary'}>
                      {user.mfa_enabled
                        ? t('common.fields.enabled', 'Enabled')
                        : t('common.fields.disabled', 'Disabled')}
                    </Badge>
                  </div>

                  {mfaSetup && !user.mfa_enabled && (
                    <div className="space-y-3 rounded-md border p-3">
                      <div className="flex justify-center">
                        <img
                          src={mfaSetup.qr_code}
                          alt={t(
                            'accountSettings.security.mfa.qrCodeAlt',
                            'MFA QR code'
                          )}
                          className="size-48"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="account-mfa-otpauth-url">
                          {t(
                            'accountSettings.security.mfa.otpauthUrl',
                            'otpauth URL'
                          )}
                        </Label>
                        <Textarea
                          id="account-mfa-otpauth-url"
                          value={mfaSetup.otpauth_url}
                          readOnly
                          className="min-h-20 font-mono text-xs"
                        />
                      </div>
                    </div>
                  )}

                  {mfaError && (
                    <Alert variant="destructive">
                      <AlertDescription>{mfaError}</AlertDescription>
                    </Alert>
                  )}

                  {user.mfa_enabled ? (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault()
                        openSecurityOTP('mfa-disable')
                      }}
                      className="space-y-4"
                    >
                      <div className="space-y-2">
                        <Label htmlFor="account-mfa-disable-code">
                          {t(
                            'accountSettings.security.mfa.authCode',
                            'Authentication Code'
                          )}
                        </Label>
                        <Input
                          id="account-mfa-disable-code"
                          autoComplete="one-time-code"
                          inputMode="numeric"
                          required
                          disabled={!mfaEnabled}
                          value={mfaCode}
                          onChange={(e) => setMFACode(e.target.value)}
                        />
                      </div>
                      <div className="flex justify-end">
                        <Button
                          type="submit"
                          variant="outline"
                          disabled={mfaControlsDisabled}
                        >
                          {savingMFA
                            ? t('common.actions.saving', 'Saving...')
                            : t(
                                'accountSettings.security.mfa.disableButton',
                                'Disable MFA'
                              )}
                        </Button>
                      </div>
                    </form>
                  ) : mfaSetup ? (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault()
                        openSecurityOTP('mfa-enable')
                      }}
                      className="space-y-4"
                    >
                      <div className="space-y-2">
                        <Label htmlFor="account-mfa-enable-code">
                          {t(
                            'accountSettings.security.mfa.authCode',
                            'Authentication Code'
                          )}
                        </Label>
                        <Input
                          id="account-mfa-enable-code"
                          autoComplete="one-time-code"
                          inputMode="numeric"
                          required
                          disabled={!mfaEnabled}
                          value={mfaCode}
                          onChange={(e) => setMFACode(e.target.value)}
                        />
                      </div>
                      <div className="flex justify-end">
                        <Button type="submit" disabled={mfaControlsDisabled}>
                          {savingMFA
                            ? t('common.actions.saving', 'Saving...')
                            : t(
                                'accountSettings.security.mfa.enableButton',
                                'Enable MFA'
                              )}
                        </Button>
                      </div>
                    </form>
                  ) : (
                    <div className="flex justify-end">
                      <Button
                        type="button"
                        variant="outline"
                        disabled={mfaControlsDisabled}
                        onClick={() => openSecurityOTP('mfa-setup')}
                      >
                        {savingMFA
                          ? t('common.actions.saving', 'Saving...')
                          : t(
                              'accountSettings.security.mfa.setupButton',
                              'Set Up MFA'
                            )}
                      </Button>
                    </div>
                  )}
                </div>

                {/* Passkeys */}
                <div className="space-y-4">
                  <Separator />
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Fingerprint className="h-4 w-4" />
                    <span>
                      {t('accountSettings.security.passkeys.title', 'Passkeys')}
                    </span>
                  </div>

                  <div className="flex gap-2">
                    <Input
                      aria-label={t(
                        'accountSettings.security.passkeys.namePlaceholder',
                        'Passkey name'
                      )}
                      placeholder={t(
                        'accountSettings.security.passkeys.namePlaceholder',
                        'Passkey name'
                      )}
                      disabled={!passkeyLoginEnabled}
                      value={passkeyName}
                      onChange={(e) => setPasskeyName(e.target.value)}
                    />
                    <Button
                      type="button"
                      disabled={passkeyControlsDisabled || !passkeyName.trim()}
                      onClick={() => openSecurityOTP('passkey-add')}
                    >
                      {savingPasskey
                        ? t('common.actions.saving', 'Saving...')
                        : t(
                            'accountSettings.security.passkeys.addButton',
                            'Add Passkey'
                          )}
                    </Button>
                  </div>

                  {passkeyError && (
                    <Alert variant="destructive">
                      <AlertDescription>{passkeyError}</AlertDescription>
                    </Alert>
                  )}

                  <div className="space-y-2">
                    {passkeys.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        {t(
                          'accountSettings.security.passkeys.empty',
                          'No passkeys added.'
                        )}
                      </p>
                    ) : (
                      passkeys.map((passkey) => (
                        <div
                          key={passkey.id}
                          className="flex items-center justify-between gap-3 rounded-md border p-3"
                        >
                          <div className="min-w-0">
                            <p className="truncate text-sm font-medium">
                              {passkey.name}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {passkey.last_used_at
                                ? t(
                                    'accountSettings.security.passkeys.lastUsed',
                                    'Last used {{date}}',
                                    {
                                      date: new Date(
                                        passkey.last_used_at
                                      ).toLocaleString(),
                                    }
                                  )
                                : t(
                                    'accountSettings.security.passkeys.addedOn',
                                    'Added {{date}}',
                                    {
                                      date: new Date(
                                        passkey.createdAt
                                      ).toLocaleString(),
                                    }
                                  )}
                            </p>
                          </div>
                          <Button
                            type="button"
                            size="icon"
                            variant="ghost"
                            aria-label={t(
                              'accountSettings.security.passkeys.deleteAriaLabel',
                              'Delete {{name}}',
                              { name: passkey.name }
                            )}
                            disabled={passkeyControlsDisabled}
                            onClick={() =>
                              openSecurityOTP('passkey-delete', passkey.id)
                            }
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      ))
                    )}
                  </div>
                </div>

                {/* Kubeconfig Tokens */}
                <div className="space-y-4">
                  <Separator />
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <KeyRound className="h-4 w-4" />
                    <span>
                      {t(
                        'accountSettings.tabs.kubeconfigTokens',
                        'Kubeconfig Tokens'
                      )}
                    </span>
                  </div>
                  {kubeconfigTokensLoading ? (
                    <p className="text-sm text-muted-foreground">
                      {t('common.messages.loading', 'Loading...')}
                    </p>
                  ) : (
                    <KubeconfigTokenList
                      tokens={kubeconfigTokens}
                      onDelete={(token) =>
                        deleteKubeconfigMutation.mutate(token.id)
                      }
                      deletingId={deleteKubeconfigMutation.variables}
                    />
                  )}
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </DialogContent>
      </Dialog>

      <EmailVerificationDialog
        open={emailDialogOpen}
        onOpenChange={setEmailDialogOpen}
        email={email.trim()}
        isPasswordUser={isPasswordUser}
        loading={savingProfile}
        onConfirm={handleConfirmEmailChange}
      />

      <SecurityOTPDialog
        open={securityAction !== null}
        onOpenChange={(next) => {
          if (!next) setSecurityAction(null)
        }}
        loading={securityOTPLoading}
        onConfirm={handleConfirmSecurityOTP}
      />
    </>
  )
}
