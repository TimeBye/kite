import { useCallback, useMemo, useRef, useState } from 'react'
import {
  IconCopy,
  IconEdit,
  IconPlus,
  IconServer,
  IconTrash,
  IconUpload,
} from '@tabler/icons-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ColumnDef,
  getCoreRowModel,
  PaginationState,
  useReactTable,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Cluster } from '@/types/api'
import {
  ClusterCreateRequest,
  ClusterUpdateRequest,
  createCluster,
  deleteCluster,
  importClusters,
  updateCluster,
  useClusterListPaginated,
  useVersionInfo,
} from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DeleteConfirmationDialog } from '@/components/delete-confirmation-dialog'
import { ResourceTableView } from '@/components/resource-table-view'

import { Action } from '../action-table'
import { ClusterDialog } from './cluster-dialog'
import { ClusterImportDialog } from './cluster-import-dialog'

export function ClusterManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })

  const {
    data: clusterData,
    isLoading,
    error,
  } = useClusterListPaginated(pagination.pageIndex + 1, pagination.pageSize, {
    refetchInterval: 5000,
  })
  const clusters = clusterData?.data ?? []
  const { data: versionInfo } = useVersionInfo()

  const [showClusterDialog, setShowClusterDialog] = useState(false)
  const [showImportDialog, setShowImportDialog] = useState(false)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)
  const [deletingCluster, setDeletingCluster] = useState<Cluster | null>(null)
  const [clusterAgentCommand, setClusterAgentCommand] = useState('')
  const [clusterAgentYaml, setClusterAgentYaml] = useState('')
  const [clusterAgentYamlError, setClusterAgentYamlError] = useState<
    string | null
  >(null)
  const [isClusterAgentYamlLoading, setIsClusterAgentYamlLoading] =
    useState(false)
  const [clusterAgentManifestURL, setClusterAgentManifestURL] = useState('')
  const [clusterAgentCopyError, setClusterAgentCopyError] = useState<
    'command' | 'yaml' | null
  >(null)
  const clusterAgentYamlRequestID = useRef(0)

  const getClusterTypeBadge = useCallback(
    (cluster: Cluster) => {
      if (cluster.clusterAgent) {
        const badge = (
          <Badge
            variant="outline"
            className="bg-violet-50 text-violet-700 border-violet-200"
          >
            {t('clusterManagement.type.clusterAgent', 'Cluster Agent')}
          </Badge>
        )
        if (!cluster.clusterAgentVersion) return badge
        return (
          <Tooltip>
            <TooltipTrigger asChild>{badge}</TooltipTrigger>
            <TooltipContent>
              {t('clusterManagement.clusterAgent.version', {
                defaultValue: 'Version: {{version}}',
                version: cluster.clusterAgentVersion,
              })}
            </TooltipContent>
          </Tooltip>
        )
      }
      if (cluster.inCluster) {
        return (
          <Badge
            variant="outline"
            className="bg-blue-50 text-blue-700 border-blue-200"
          >
            {t('clusterManagement.type.inCluster', 'In-Cluster')}
          </Badge>
        )
      }
      return (
        <Badge
          variant="outline"
          className="bg-gray-50 text-gray-700 border-gray-200"
        >
          {t('clusterManagement.type.external', 'External')}
        </Badge>
      )
    },
    [t]
  )

  const getStatusBadge = useCallback(
    (cluster: Cluster) => {
      if (!cluster.enabled) {
        return (
          <Badge variant="secondary">{t('status.disabled', 'Disabled')}</Badge>
        )
      }
      if (cluster.clusterAgent && !cluster.connected) {
        return (
          <Badge variant="outline">
            {t('clusterManagement.status.waiting', 'Waiting for Cluster Agent')}
          </Badge>
        )
      }
      if (cluster.clusterAgent) {
        return (
          <Badge variant="default">
            {t('clusterManagement.status.connected', 'Connected')}
          </Badge>
        )
      }
      return <Badge variant="default">{t('status.enabled', 'Enabled')}</Badge>
    },
    [t]
  )

  const columns = useMemo<ColumnDef<Cluster>[]>(
    () => [
      {
        id: 'name',
        header: t('common.fields.name', 'Name'),
        cell: ({ row: { original: cluster } }) => (
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium">{cluster.name}</span>
              {cluster.isDefault && (
                <Badge variant="secondary">
                  {t('common.values.default', 'Default')}
                </Badge>
              )}
            </div>
            {cluster.description && (
              <div className="text-sm text-muted-foreground">
                {cluster.description}
              </div>
            )}
          </div>
        ),
      },
      {
        id: 'version',
        header: t('common.fields.version', 'Version'),
        cell: ({ row: { original: cluster } }) => {
          if (cluster.clusterAgent && !cluster.connected) {
            return <span className="text-muted-foreground">-</span>
          }
          if (cluster.error) {
            return (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="destructive">
                    {t('common.messages.error')}
                  </Badge>
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs break-all">{cluster.error}</p>
                </TooltipContent>
              </Tooltip>
            )
          }
          return (
            <Badge variant="secondary" className="font-mono">
              {cluster.version || '-'}
            </Badge>
          )
        },
      },
      {
        id: 'type',
        header: t('common.fields.type', 'Type'),
        cell: ({ row: { original: cluster } }) => getClusterTypeBadge(cluster),
      },
      {
        id: 'clusterAgentVersion',
        header: t('clusterManagement.table.agentVersion', 'Agent Version'),
        cell: ({ row: { original: cluster } }) => {
          if (!cluster.clusterAgent) {
            return <span className="text-muted-foreground">—</span>
          }
          if (!cluster.connected || !cluster.clusterAgentVersion || !versionInfo?.version) {
            return (
              <Badge variant="outline">
                {t('clusterManagement.agentVersion.unknown', 'Unknown')}
              </Badge>
            )
          }
          const requiresUpgrade = cluster.clusterAgentVersion !== versionInfo.version
          return (
            <Badge
              variant="outline"
              className={
                requiresUpgrade
                  ? 'w-fit border-amber-200 bg-amber-50 text-amber-700'
                  : 'w-fit border-green-200 bg-green-50 text-green-700'
              }
            >
              <span className="font-mono">{cluster.clusterAgentVersion}</span>
              <span>
                {requiresUpgrade
                  ? t('clusterManagement.agentVersion.upgradeRequired', 'Upgrade required')
                  : t('clusterManagement.agentVersion.upToDate', 'Up to date')}
              </span>
            </Badge>
          )
        },
      },
      {
        id: 'status',
        header: t('common.fields.status', 'Status'),
        cell: ({ row: { original: cluster } }) => (
          <div className="flex items-center gap-3">
            {getStatusBadge(cluster)}
          </div>
        ),
      },
      {
        id: 'Prometheus',
        header: t('common.fields.prometheus', 'Prometheus'),
        cell: ({ row: { original: cluster } }) => (
          <div className="text-sm text-muted-foreground">
            {cluster.prometheusURL ? 'Yes' : 'No'}
          </div>
        ),
      },
    ],
    [getClusterTypeBadge, getStatusBadge, t, versionInfo?.version]
  )

  const actions = useMemo<Action<Cluster>[]>(
    () => [
      {
        label: (
          <>
            <IconEdit className="h-4 w-4" />
            {t('common.actions.edit', 'Edit')}
          </>
        ),
        onClick: (cluster) => {
          setEditingCluster(cluster)
          setShowClusterDialog(true)
        },
      },
      {
        label: (
          <div className="inline-flex items-center gap-2 text-destructive">
            <IconTrash className="h-4 w-4" />
            {t('common.actions.delete', 'Delete')}
          </div>
        ),
        shouldDisable: (cluster) => cluster.isDefault,
        onClick: (cluster) => {
          setDeletingCluster(cluster)
        },
      },
    ],
    [t]
  )

  const tableColumns = useMemo<ColumnDef<Cluster>[]>(() => {
    const actionColumn: ColumnDef<Cluster> = {
      id: 'actions',
      header: t('common.fields.actions', 'Actions'),
      cell: ({ row }) => (
        <div className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                aria-label={t('common.fields.actions', 'Actions')}
              >
                •••
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {actions.map((action, index) => (
                <DropdownMenuItem
                  key={index}
                  disabled={action.shouldDisable?.(row.original)}
                  onClick={() => action.onClick(row.original)}
                  className="gap-2"
                >
                  {action.dynamicLabel
                    ? action.dynamicLabel(row.original)
                    : action.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
    }
    return [...columns, actionColumn]
  }, [actions, columns, t])

  const table = useReactTable({
    data: clusters,
    columns: tableColumns,
    getCoreRowModel: getCoreRowModel(),
    state: { pagination },
    onPaginationChange: setPagination,
    manualPagination: true,
    pageCount: Math.ceil((clusterData?.total ?? 0) / pagination.pageSize) || 0,
  })

  const createMutation = useMutation({
    mutationFn: createCluster,
    onSuccess: async ({
      clusterAgentServer,
      clusterAgentToken,
      clusterAgentPublicKey,
      clusterAgentManifestURL,
    }: {
      clusterAgentServer?: string
      clusterAgentToken?: string
      clusterAgentPublicKey?: string
      clusterAgentManifestURL?: string
    }) => {
      queryClient.invalidateQueries({ queryKey: ['cluster-list'] })
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paginated'] })
      toast.success(
        t('clusterManagement.messages.created', 'Cluster created successfully')
      )
      setShowClusterDialog(false)
      if (clusterAgentServer && clusterAgentToken && clusterAgentPublicKey) {
        setClusterAgentCopyError(null)
        setClusterAgentCommand(
          `kite cluster-agent --server='${clusterAgentServer}' --token='${clusterAgentToken}' --public-key='${clusterAgentPublicKey}'`
        )
        setClusterAgentYaml('')
        setClusterAgentYamlError(null)
        setClusterAgentManifestURL(clusterAgentManifestURL || '')
        setIsClusterAgentYamlLoading(true)
        const requestID = ++clusterAgentYamlRequestID.current
        try {
          if (!clusterAgentManifestURL) throw new Error('Missing manifest URL')
          const manifestURL = new URL(
            clusterAgentManifestURL,
            window.location.origin
          )
          const response = await fetch(
            `${manifestURL.pathname}${manifestURL.search}`,
            {
              cache: 'no-store',
            }
          )
          if (!response.ok) throw new Error(`HTTP ${response.status}`)
          const yaml = await response.text()
          if (requestID === clusterAgentYamlRequestID.current) {
            setClusterAgentYaml(yaml)
          }
        } catch {
          if (requestID === clusterAgentYamlRequestID.current) {
            setClusterAgentYamlError(
              t(
                'clusterManagement.clusterAgent.loadYamlError',
                'Failed to load YAML from the manifest URL.'
              )
            )
          }
        } finally {
          if (requestID === clusterAgentYamlRequestID.current) {
            setIsClusterAgentYamlLoading(false)
          }
        }
      }
    },
    onError: (error: Error) => {
      toast.error(
        error.message ||
          t(
            'clusterManagement.messages.createError',
            'Failed to create cluster'
          )
      )
    },
  })

  const importMutation = useMutation({
    mutationFn: (config: string) => importClusters({ config }),
    onSuccess: ({ importedCount }) => {
      queryClient.invalidateQueries({ queryKey: ['cluster-list'] })
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paginated'] })
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      toast.success(
        t(
          'clusterManagement.messages.imported',
          'Imported or updated {{count}} clusters successfully',
          { count: importedCount }
        )
      )
      setShowImportDialog(false)
    },
  })

  // Update cluster mutation
  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: ClusterUpdateRequest }) =>
      updateCluster(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cluster-list'] })
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paginated'] })
      toast.success(
        t('clusterManagement.messages.updated', 'Cluster updated successfully')
      )
      setShowClusterDialog(false)
      setEditingCluster(null)
    },
    onError: (error: Error) => {
      toast.error(
        error.message ||
          t(
            'clusterManagement.messages.updateError',
            'Failed to update cluster'
          )
      )
    },
  })

  // Delete cluster mutation
  const deleteMutation = useMutation({
    mutationFn: deleteCluster,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cluster-list'] })
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paginated'] })
      toast.success(
        t('clusterManagement.messages.deleted', 'Cluster deleted successfully')
      )
      setDeletingCluster(null)
    },
    onError: (error: Error) => {
      toast.error(
        error.message ||
          t(
            'clusterManagement.messages.deleteError',
            'Failed to delete cluster'
          )
      )
    },
  })

  const handleSubmitCluster = (clusterData: ClusterCreateRequest) => {
    if (editingCluster) {
      // Update existing cluster - use the form data directly
      updateMutation.mutate({
        id: editingCluster.id,
        data: clusterData,
      })
    } else {
      // Create new cluster
      createMutation.mutate(clusterData)
    }
  }

  const handleDeleteCluster = () => {
    if (!deletingCluster) return
    deleteMutation.mutate(deletingCluster.id)
  }

  const emptyState = (() => {
    if (isLoading) {
      return (
        <div className="py-10 text-center text-muted-foreground">
          {t('common.messages.loading', 'Loading...')}
        </div>
      )
    }
    if (error) {
      return (
        <div className="py-10 text-center text-destructive">
          {t('clusterManagement.errors.loadFailed', 'Failed to load clusters')}
        </div>
      )
    }
    if (clusters.length === 0) {
      return (
        <div className="py-10 text-center text-muted-foreground">
          <IconServer className="h-12 w-12 mx-auto mb-4 opacity-50" />
          <p>
            {t('clusterManagement.empty.title', 'No clusters configured')}
          </p>
          <p className="text-sm mt-1">
            {t(
              'clusterManagement.empty.description',
              'Add your first cluster to get started'
            )}
          </p>
        </div>
      )
    }
    return null
  })()

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <IconServer className="h-5 w-5" />
                {t('clusterManagement.title', 'Cluster Management')}
              </CardTitle>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  importMutation.reset()
                  setShowImportDialog(true)
                }}
                className="gap-2"
              >
                <IconUpload className="size-4" />
                {t('clusterManagement.actions.import', 'Import Clusters')}
              </Button>
              <Button
                onClick={() => {
                  setEditingCluster(null)
                  setShowClusterDialog(true)
                }}
                className="gap-2"
              >
                <IconPlus className="h-4 w-4" />
                {t('clusterManagement.actions.add', 'Add Cluster')}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <ResourceTableView
            table={table}
            columnCount={tableColumns.length}
            isLoading={isLoading}
            data={clusterData?.data}
            emptyState={emptyState}
            hasActiveFilters={false}
            filteredRowCount={clusterData?.data?.length ?? 0}
            totalRowCount={clusterData?.total ?? 0}
            searchQuery=""
            pagination={pagination}
            setPagination={setPagination}
            fitViewportHeight
          />
        </CardContent>
      </Card>

      {/* Cluster Dialog (Add/Edit) */}
      <ClusterDialog
        open={showClusterDialog}
        onOpenChange={(open) => {
          setShowClusterDialog(open)
          if (!open) {
            setEditingCluster(null)
          }
        }}
        cluster={editingCluster}
        onSubmit={handleSubmitCluster}
      />

      <ClusterImportDialog
        open={showImportDialog}
        onOpenChange={(open) => {
          setShowImportDialog(open)
          if (!open) importMutation.reset()
        }}
        onSubmit={(config) => importMutation.mutate(config)}
        isSubmitting={importMutation.isPending}
        error={importMutation.error?.message}
      />

      <Dialog
        open={!!clusterAgentCommand}
        onOpenChange={(open) => {
          if (!open) {
            clusterAgentYamlRequestID.current += 1
            setClusterAgentCommand('')
            setClusterAgentYaml('')
            setClusterAgentYamlError(null)
            setIsClusterAgentYamlLoading(false)
            setClusterAgentManifestURL('')
            setClusterAgentCopyError(null)
          }
        }}
      >
        <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle className="text-balance">
              {t(
                'clusterManagement.clusterAgent.title',
                'Connect Cluster Agent'
              )}
            </DialogTitle>
            <DialogDescription className="text-pretty">
              {t(
                'clusterManagement.clusterAgent.description',
                'Choose a command or Kubernetes YAML to run inside the target cluster. This connection information is shown only once.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Tabs defaultValue="command">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="command">
                {t('clusterManagement.clusterAgent.command', 'Command')}
              </TabsTrigger>
              <TabsTrigger value="yaml">
                {t('clusterManagement.clusterAgent.yaml', 'Kubernetes YAML')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value="command" className="space-y-2">
              <div className="flex gap-2">
                <Input
                  readOnly
                  className="font-mono"
                  aria-label={t(
                    'clusterManagement.clusterAgent.command',
                    'Command'
                  )}
                  value={clusterAgentCommand}
                />
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  aria-label={t(
                    'clusterManagement.clusterAgent.copyCommand',
                    'Copy command'
                  )}
                  onClick={async () => {
                    if (!clusterAgentCommand) return
                    try {
                      await navigator.clipboard.writeText(clusterAgentCommand)
                      setClusterAgentCopyError(null)
                      toast.success(t('common.messages.copied', 'Copied'))
                    } catch {
                      setClusterAgentCopyError('command')
                    }
                  }}
                >
                  <IconCopy className="size-4" />
                </Button>
              </div>
              {clusterAgentCopyError === 'command' && (
                <p role="alert" className="text-sm text-destructive">
                  {t(
                    'clusterManagement.clusterAgent.copyError',
                    'Failed to copy. Copy the content manually.'
                  )}
                </p>
              )}
            </TabsContent>
            <TabsContent value="yaml" className="space-y-2">
              {clusterAgentManifestURL && (
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground">
                    {t(
                      'clusterManagement.clusterAgent.applyUrl',
                      'Apply directly with URL'
                    )}
                  </Label>
                  <div className="flex gap-2">
                    <Input
                      readOnly
                      className="font-mono text-xs"
                      value={`kubectl apply -f "${clusterAgentManifestURL}"`}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      aria-label={t(
                        'clusterManagement.clusterAgent.copyApplyCommand',
                        'Copy apply command'
                      )}
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(
                            `kubectl apply -f "${clusterAgentManifestURL}"`
                          )
                          setClusterAgentCopyError(null)
                          toast.success(t('common.messages.copied', 'Copied'))
                        } catch {
                          setClusterAgentCopyError('yaml')
                        }
                      }}
                    >
                      <IconCopy className="size-4" />
                    </Button>
                  </div>
                </div>
              )}
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">
                  {t(
                    'clusterManagement.clusterAgent.orUseYaml',
                    'Or deploy with YAML'
                  )}
                </Label>
                {isClusterAgentYamlLoading ? (
                  <Skeleton className="h-96 w-full" />
                ) : clusterAgentYamlError ? (
                  <p role="alert" className="text-sm text-destructive">
                    {clusterAgentYamlError}
                  </p>
                ) : (
                  <Textarea
                    readOnly
                    className="h-96 resize-none font-mono text-xs"
                    aria-label={t(
                      'clusterManagement.clusterAgent.yaml',
                      'Kubernetes YAML'
                    )}
                    value={clusterAgentYaml}
                  />
                )}
              </div>
              {clusterAgentYaml && (
                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(clusterAgentYaml)
                        setClusterAgentCopyError(null)
                        toast.success(t('common.messages.copied', 'Copied'))
                      } catch {
                        setClusterAgentCopyError('yaml')
                      }
                    }}
                  >
                    <IconCopy className="size-4" />
                    {t('clusterManagement.clusterAgent.copyYaml', 'Copy YAML')}
                  </Button>
                </div>
              )}
              {clusterAgentCopyError === 'yaml' && (
                <p role="alert" className="text-sm text-destructive">
                  {t(
                    'clusterManagement.clusterAgent.copyError',
                    'Failed to copy. Copy the content manually.'
                  )}
                </p>
              )}
            </TabsContent>
          </Tabs>
          <DialogFooter>
            <Button
              type="button"
              onClick={() => {
                clusterAgentYamlRequestID.current += 1
                setClusterAgentCommand('')
                setClusterAgentYaml('')
                setClusterAgentYamlError(null)
                setIsClusterAgentYamlLoading(false)
                setClusterAgentManifestURL('')
                setClusterAgentCopyError(null)
              }}
            >
              {t('common.actions.close', 'Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <DeleteConfirmationDialog
        open={!!deletingCluster}
        onOpenChange={() => setDeletingCluster(null)}
        onConfirm={handleDeleteCluster}
        resourceName={deletingCluster?.name || ''}
        resourceType={t('clusterManagement.title')}
        additionalNote={t(
          'clusterManagement.deleteConfirmation',
          "This action will only remove the current cluster's configuration in kite and will not delete any cluster resources."
        )}
      />
    </div>
  )
}
