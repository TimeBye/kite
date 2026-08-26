import { useEffect, useMemo, useState, type FormEvent } from 'react'
import * as yaml from 'js-yaml'
import { ChevronDown, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

import type {
  HelmChartDetail,
  HelmChartVersion,
  HelmReleaseDryRunResponse,
  HelmReleaseInstallRequest,
} from '@/types/api'
import {
  dryRunInstallHelmRelease,
  fetchHelmTask,
  installHelmRelease,
  previewInstallValues,
  useHelmChart,
  useHelmChartContent,
  useResources,
} from '@/lib/api'
import { isSameHelmVersion } from '@/pages/helmrelease-chart-selection'
import { formatDate, translateError } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { NamespaceSelector } from '@/components/selector/namespace-selector'
import { SimpleYamlEditor } from '@/components/simple-yaml-editor'
import { Textarea } from '@/components/ui/textarea'
import { YamlFileTreeViewerNative as YamlFileTreeViewer } from '@/components/yaml-file-tree-viewer-native'

function defaultReleaseName(name: string) {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9-]+/g, '-')
      .replace(/^-+|-+$/g, '') || name
  )
}

export function HelmInstallDialog({
  chart,
  open,
  onOpenChange,
}: {
  chart: HelmChartDetail
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [releaseName, setReleaseName] = useState(() =>
    defaultReleaseName(chart.name)
  )
  const [namespace, setNamespace] = useState('default')
  const [createNamespace, setCreateNamespace] = useState(true)
  const [valuesYaml, setValuesYaml] = useState('')
  const [setValuesStr, setSetValuesStr] = useState('')
  const [wait, setWait] = useState(false)
  const [forceConflicts, setForceConflicts] = useState(false)
  const [rollbackOnFailure, setRollbackOnFailure] = useState(false)
  const [error, setError] = useState('')
  const [isInstalling, setIsInstalling] = useState(false)
  const [installTaskID, setInstallTaskID] = useState<number | null>(null)
  const [isDryRunning, setIsDryRunning] = useState(false)
  const [dryRunPreview, setDryRunPreview] =
    useState<HelmReleaseDryRunResponse | null>(null)
  const [mergedPreview, setMergedPreview] = useState('')
  const [isMergedLoading, setIsMergedLoading] = useState(false)
  const [mergedError, setMergedError] = useState('')
  const [selectedVersion, setSelectedVersion] = useState('')

  const versionOptions = useMemo<HelmChartVersion[]>(
    () => chart.versions?.length ?? 0 ? chart.versions : chart.version ? [{ version: chart.version }] : [],
    [chart.versions, chart.version]
  )
  const visibleVersionOptions = useMemo<HelmChartVersion[]>(() => {
    if (
      !chart.version ||
      versionOptions.some((v) => isSameHelmVersion(v.version, chart.version))
    ) {
      return versionOptions
    }
    return [{ version: chart.version }, ...versionOptions]
  }, [chart.version, versionOptions])
  const activeVersion = selectedVersion || chart.version
  const isDefaultVersion = isSameHelmVersion(activeVersion, chart.version)
  const selectedChartQuery = useHelmChart(
    chart.repositoryName,
    chart.name,
    activeVersion,
    chart.source,
    !isDefaultVersion && open,
    open
  )
  const activeChartUrl = isDefaultVersion
    ? chart.chartUrl
    : selectedChartQuery.data?.chartUrl
  const isVersionLoading = !isDefaultVersion && selectedChartQuery.isLoading
  const defaultValuesQuery = useHelmChartContent(
    chart.repositoryName,
    chart.name,
    'values',
    activeVersion,
    chart.source,
    open
  )
  const defaultValues = defaultValuesQuery.isLoading
    ? t('common.messages.loading')
    : defaultValuesQuery.data?.content || ''
  const { data: namespaces } = useResources('namespaces')
  const namespaceExists = useMemo(() => {
    if (!namespace.trim()) return false
    return (namespaces || []).some(
      (ns) => ns.metadata?.name === namespace.trim()
    )
  }, [namespaces, namespace])
  const showCreateNamespace = !dryRunPreview && !namespaceExists && !!namespace.trim()
  const readableError = error.replace(/\s&&\s/g, '\n')

  const buildInstallRequest = (): {
    targetNamespace: string
    request: HelmReleaseInstallRequest
  } | null => {
    setError('')

    if (!activeChartUrl) {
      setError(
        t('helmCharts.messages.noChartUrl', {
          defaultValue: 'Chart package URL is missing.',
        })
      )
      return null
    }

    let values: Record<string, unknown> = {}
    if (valuesYaml.trim()) {
      try {
        const parsed = yaml.load(valuesYaml)
        if (parsed && (typeof parsed !== 'object' || Array.isArray(parsed))) {
          setError(
            t('helmCharts.messages.invalidValues', {
              defaultValue: 'Values must be a YAML object.',
            })
          )
          return null
        }
        values = (parsed || {}) as Record<string, unknown>
      } catch (err) {
        setError(translateError(err, t))
        return null
      }
    }

    const targetNamespace = namespace.trim()
    const setValues = setValuesStr
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
    const request = {
      releaseName: releaseName.trim(),
      namespace: targetNamespace,
      chartUrl: activeChartUrl,
      repositoryName: chart.repositoryName,
      source: chart.source,
      createNamespace: !namespaceExists && createNamespace,
      values,
      setValues: setValues.length > 0 ? setValues : undefined,
      wait,
      forceConflicts,
      rollbackOnFailure,
    }

    return { targetNamespace, request }
  }

  // Fetch merged values preview with debounce
  useEffect(() => {
    if (!open || dryRunPreview) {
      return
    }
    const payload = buildInstallRequest()
    if (!payload) {
      setMergedPreview('')
      setMergedError('')
      return
    }
    let cancelled = false
    setIsMergedLoading(true)
    const timer = setTimeout(async () => {
      try {
        const result = await previewInstallValues(
          payload.targetNamespace,
          payload.request
        )
        if (!cancelled) {
          setMergedPreview(result.values)
          setMergedError('')
        }
      } catch (err) {
        if (!cancelled) {
          setMergedError(translateError(err, t))
          setMergedPreview('')
        }
      } finally {
        if (!cancelled) {
          setIsMergedLoading(false)
        }
      }
    }, 500)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, dryRunPreview, valuesYaml, setValuesStr, activeChartUrl])

  const handleDryRun = async () => {
    const payload = buildInstallRequest()
    if (!payload) {
      return
    }

    setIsDryRunning(true)
    try {
      const preview = await dryRunInstallHelmRelease(
        payload.targetNamespace,
        payload.request
      )
      setDryRunPreview(preview)
    } catch (err) {
      setError(translateError(err, t))
    } finally {
      setIsDryRunning(false)
    }
  }

  const handleInstall = async () => {
    const payload = buildInstallRequest()
    if (!payload) {
      return
    }

    setIsInstalling(true)
    setError('')
    try {
      const response = await installHelmRelease(
        payload.targetNamespace,
        payload.request
      )
      setInstallTaskID(response.taskId)
    } catch (err) {
      setError(translateError(err, t))
      setIsInstalling(false)
    }
  }

  // Poll install task status
  useEffect(() => {
    if (!installTaskID) {
      return
    }
    let cancelled = false
    const poll = async () => {
      try {
        const task = await fetchHelmTask(installTaskID)
        if (cancelled) {
          return
        }
        if (task.status === 'succeeded') {
          setIsInstalling(false)
          setInstallTaskID(null)
          toast.success(
            t('helmCharts.messages.installed', {
              defaultValue: 'Helm release installed',
            })
          )
          onOpenChange(false)
          navigate(
            `/helmrelease/${encodeURIComponent(task.namespace)}/${encodeURIComponent(task.releaseName)}`
          )
        } else if (task.status === 'failed') {
          setIsInstalling(false)
          setInstallTaskID(null)
          setError(task.error || t('common.messages.operationFailed'))
        }
      } catch (err) {
        if (!cancelled) {
          setIsInstalling(false)
          setInstallTaskID(null)
          setError(translateError(err, t))
        }
      }
    }
    const interval = setInterval(poll, 2000)
    poll()
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [installTaskID, navigate, onOpenChange, t])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (dryRunPreview) {
      await handleInstall()
      return
    }
    await handleDryRun()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="flex h-[calc(100dvh-4rem)] max-h-[calc(100dvh-4rem)] w-[calc(100vw-4rem)] !max-w-[calc(100vw-4rem)] flex-col overflow-hidden"
        onPointerDownOutside={(event) => {
          event.preventDefault()
        }}
        onEscapeKeyDown={(event) => {
          event.preventDefault()
        }}
      >
        <form
          onSubmit={handleSubmit}
          className="flex h-full min-h-0 flex-col gap-4"
        >
          <DialogHeader>
            <DialogTitle>
              {t('helmCharts.actions.install', { defaultValue: 'Install' })}
            </DialogTitle>
            <DialogDescription>
              {chart.repositoryName}/{chart.name}:{activeVersion}
            </DialogDescription>
          </DialogHeader>

          {error ? (
            <div
              role="alert"
              className="max-h-40 overflow-y-auto rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm leading-5"
            >
              <div className="mb-1 font-medium text-destructive">
                {t('common.fields.errorDetails')}
              </div>
              <pre className="m-0 whitespace-pre-wrap break-words font-mono text-xs leading-5 text-foreground">
                {readableError}
              </pre>
            </div>
          ) : null}

          <div
            className={
              dryRunPreview
                ? 'flex min-h-0 flex-1 flex-col gap-4 overflow-hidden pr-1'
                : 'min-h-0 flex-1 space-y-4 overflow-y-auto pr-1'
            }
          >
            <div className="grid gap-4 md:grid-cols-3">
              <div className="grid gap-2">
                <Label htmlFor="helm-release-name">
                  {t('helm.fields.releaseName')}
                </Label>
                <Input
                  id="helm-release-name"
                  value={releaseName}
                  onChange={(event) => {
                    setReleaseName(event.target.value)
                    setDryRunPreview(null)
                  }}
                  disabled={isInstalling || isDryRunning || !!dryRunPreview}
                  required
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="helm-release-namespace">
                  {t('common.fields.namespace', { defaultValue: 'Namespace' })}
                </Label>
                <div className="flex flex-wrap items-center gap-2">
                  <NamespaceSelector
                    selectedNamespace={namespace}
                    handleNamespaceChange={(value) => {
                      setNamespace(value)
                      setDryRunPreview(null)
                    }}
                    disabled={isInstalling || isDryRunning || !!dryRunPreview}
                    triggerClassName="w-full sm:w-48 sm:min-w-0"
                    modal
                    allowCreate
                  />
                </div>
                {showCreateNamespace ? (
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="helm-create-namespace"
                      checked={createNamespace}
                      onCheckedChange={(value) => {
                        setCreateNamespace(value === true)
                        setDryRunPreview(null)
                      }}
                      disabled={isInstalling || isDryRunning}
                    />
                    <Label
                      htmlFor="helm-create-namespace"
                      className="text-sm font-normal text-muted-foreground"
                    >
                      {t('helm.fields.createNamespace')}
                    </Label>
                  </div>
                ) : null}
              </div>

              <div className="grid gap-2">
                <Label>{t('helm.fields.version')}</Label>
                {visibleVersionOptions.length > 0 ? (
                  <Select
                    value={activeVersion}
                    onValueChange={(value) => {
                      setSelectedVersion(value)
                      setDryRunPreview(null)
                    }}
                    disabled={isInstalling || isDryRunning || !!dryRunPreview}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent
                      className="min-w-80"
                      viewportClassName="h-auto max-h-72 overflow-y-auto"
                    >
                      {visibleVersionOptions.map((version) => (
                        <SelectItem
                          key={version.version}
                          value={version.version}
                          textValue={version.version}
                        >
                          <span className="tabular-nums">
                            {version.version}
                          </span>
                          {isSameHelmVersion(
                            version.version,
                            chart.version
                          ) ? (
                            <span className="text-xs text-muted-foreground">
                              {t('common.fields.current')}
                            </span>
                          ) : null}
                          {version.appVersion ? (
                            <span className="text-xs text-muted-foreground">
                              {version.appVersion}
                            </span>
                          ) : null}
                          {version.publishedAt ? (
                            <span className="text-xs text-muted-foreground tabular-nums">
                              {formatDate(version.publishedAt)}
                            </span>
                          ) : null}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <div className="flex h-9 items-center rounded-md border bg-muted/30 px-3 text-sm text-muted-foreground">
                    {isVersionLoading ? (
                      <>
                        <Loader2 className="mr-2 size-4 animate-spin" />
                        {t('helm.messages.loadingVersions', {
                          defaultValue: 'Loading versions...',
                        })}
                      </>
                    ) : (
                      activeVersion || '-'
                    )}
                  </div>
                )}
                {isVersionLoading ? (
                  <p className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                    <Loader2 className="size-3 animate-spin" />
                    {t('helm.messages.loadingChartPackage', {
                      defaultValue: 'Loading chart package...',
                    })}
                  </p>
                ) : null}
              </div>
            </div>

            {dryRunPreview ? (
              <YamlFileTreeViewer
                files={dryRunPreview.resources}
                title={t('helm.fields.dryRunPreview')}
                emptyMessage={t('helm.messages.noDryRunResources')}
                fillHeight
              />
            ) : (
              <div className="grid min-h-0 gap-4 lg:grid-cols-3">
                <div className="grid min-h-0 gap-2">
                  <Label>{t('helmCharts.fields.defaultValues')}</Label>
                  <SimpleYamlEditor
                    value={defaultValues}
                    onChange={() => undefined}
                    disabled
                    height="calc(100dvh - 20rem)"
                  />
                </div>

                <div className="grid min-h-0 gap-2">
                  <Label>{t('helmCharts.fields.customValues')}</Label>
                  <SimpleYamlEditor
                    value={valuesYaml}
                    onChange={(value) => {
                      setValuesYaml(value || '')
                      setDryRunPreview(null)
                    }}
                    disabled={isInstalling || isDryRunning}
                    height="calc(100dvh - 20rem)"
                  />
                </div>

                <div className="grid min-h-0 gap-2">
                  <div className="flex items-center justify-between gap-2">
                    <Label>{t('helm.fields.mergedPreview')}</Label>
                    {isMergedLoading ? (
                      <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                        <Loader2 className="size-3 animate-spin" />
                        {t('common.messages.loading')}
                      </span>
                    ) : null}
                  </div>
                  {mergedError ? (
                    <p className="text-sm text-destructive">{mergedError}</p>
                  ) : (
                    <SimpleYamlEditor
                      value={mergedPreview}
                      onChange={() => undefined}
                      disabled
                      height="calc(100dvh - 20rem)"
                    />
                  )}
                </div>
              </div>
            )}

            {defaultValuesQuery.error ? (
              <p className="text-sm text-destructive">
                {translateError(defaultValuesQuery.error, t)}
              </p>
            ) : null}
          </div>

          {!dryRunPreview ? (
            <Collapsible>
              <CollapsibleTrigger className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
                <ChevronDown className="size-4" />
                {t('helm.fields.advancedSettings')}
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-3 space-y-3">
                <div className="grid gap-1.5">
                  <Label htmlFor="helm-install-set-values">
                    {t('helm.fields.setValues')}
                  </Label>
                  <Textarea
                    id="helm-install-set-values"
                    value={setValuesStr}
                    onChange={(e) => setSetValuesStr(e.target.value)}
                    disabled={isInstalling || isDryRunning}
                    placeholder={t('helm.placeholders.setValues')}
                    className="min-h-[80px] font-mono text-sm"
                  />
                </div>
                <div className="flex flex-wrap items-center gap-3 text-sm">
                  <Label
                    htmlFor="helm-install-force-conflicts"
                    className="flex items-center gap-2 font-normal text-muted-foreground"
                  >
                    <Checkbox
                      id="helm-install-force-conflicts"
                      checked={forceConflicts}
                      onCheckedChange={(value) => setForceConflicts(value === true)}
                      disabled={isInstalling || isDryRunning}
                    />
                    {t('helm.fields.forceConflicts')}
                  </Label>
                  <Label
                    htmlFor="helm-install-wait"
                    className="flex items-center gap-2 font-normal text-muted-foreground"
                  >
                    <Checkbox
                      id="helm-install-wait"
                      checked={wait}
                      onCheckedChange={(value) => setWait(value === true)}
                      disabled={isInstalling || isDryRunning}
                    />
                    {t('helm.fields.wait')}
                  </Label>
                  <Label
                    htmlFor="helm-install-rollback-on-failure"
                    className="flex items-center gap-2 font-normal text-muted-foreground"
                  >
                    <Checkbox
                      id="helm-install-rollback-on-failure"
                      checked={rollbackOnFailure}
                      onCheckedChange={(value) =>
                        setRollbackOnFailure(value === true)
                      }
                      disabled={isInstalling || isDryRunning}
                    />
                    {t('helm.fields.rollbackOnFailure')}
                  </Label>
                </div>
              </CollapsibleContent>
            </Collapsible>
          ) : null}

          <DialogFooter className="items-center gap-3 sm:justify-end">
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              {dryRunPreview ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setDryRunPreview(null)}
                  disabled={isInstalling || isDryRunning}
                >
                  {t('helm.actions.backToValues')}
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => onOpenChange(false)}
                  disabled={isInstalling || isDryRunning}
                >
                  {t('common.actions.cancel')}
                </Button>
              )}
              {!dryRunPreview ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void handleDryRun()}
                  disabled={
                    !releaseName.trim() ||
                    !namespace.trim() ||
                    !activeChartUrl ||
                    isInstalling ||
                    isDryRunning
                  }
                >
                  {isDryRunning ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : null}
                  {t('helm.actions.dryRun')}
                </Button>
              ) : null}
              <Button
                type="button"
                onClick={() => void handleInstall()}
                disabled={
                  !releaseName.trim() ||
                  !namespace.trim() ||
                  !activeChartUrl ||
                  isInstalling ||
                  isDryRunning
                }
              >
                {isInstalling ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : null}
                {t('helmCharts.actions.install', { defaultValue: 'Install' })}
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
