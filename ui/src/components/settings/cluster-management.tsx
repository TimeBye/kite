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
  useClusterListPaged,
  useVersionInfo,
} from '@/lib/api'
import { ResourceTableView } from '@/components/resource-table-view'
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
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DeleteConfirmationDialog } from '@/components/delete-confirmation-dialog'

import { ClusterDialog } from './cluster-dialog'
import { ClusterImportDialog } from './cluster-import-dialog'

export function ClusterManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data: versionInfo } = useVersionInfo()

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const { data, isLoading, error } = useClusterListPaged(
    pagination.pageIndex + 1,
    pagination.pageSize,
    { refetchInterval: 5000 }
  )
  const clusters = data?.data ?? []

  const [showClusterDialog, setShowClusterDialog] = useState(false)
  const [showImportDialog, setShowImportDialog] = useState(false)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)
  const [deletingCluster, setDeletingCluster] = useState<Cluster | null>(null)
  const [connectorCommand, setConnectorCommand] = useState('')
  const [connectorYaml, setConnectorYaml] = useState('')
  const [connectorYamlError, setConnectorYamlError] = useState<string | null>(
    null
  )
  const [isConnectorYamlLoading, setIsConnectorYamlLoading] = useState(false)
  const [connectorManifestURL, setConnectorManifestURL] = useState('')
  const [connectorCopyError, setConnectorCopyError] = useState<
    'command' | 'yaml' | null
  >(null)
  const connectorYamlRequestID = useRef(0)

  const getClusterTypeBadge = useCallback(
    (cluster: Cluster) => {
      if (cluster.connector) {
        return (
          <Badge
            variant="outline"
            className="bg-violet-50 text-violet-700 border-violet-200"
          >
            {t('clusterManagement.type.connector', 'Kite Connector')}
          </Badge>
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
      if (cluster.connector && !cluster.connected) {
        return (
          <Badge variant="outline">
            {t('clusterManagement.status.waiting', 'Waiting for Connector')}
          </Badge>
        )
      }
      if (cluster.connector) {
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
              {cluster.isDefault && <Badge variant="secondary">Default</Badge>}
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
          if (cluster.connector && !cluster.connected) {
            return <span className="text-muted-foreground">-</span>
          }
          if (cluster.error) {
            return (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="destructive">Error</Badge>
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
        id: 'connectorVersion',
        header: t('clusterManagement.connectorVersion', 'Connector Version'),
        cell: ({ row: { original: cluster } }) => {
          if (!cluster.connector) {
            return <span className="text-muted-foreground">-</span>
          }
          const ver = cluster.connectorVersion
          if (!ver) {
            return <span className="text-muted-foreground">-</span>
          }
          const isOutdated = versionInfo?.version && ver !== versionInfo.version
          return (
            <Badge
              variant="outline"
              className={
                isOutdated
                  ? 'border-yellow-500/50 bg-yellow-500/10 text-yellow-700 dark:text-yellow-400 font-mono'
                  : 'border-green-500/50 bg-green-500/10 text-green-700 dark:text-green-400 font-mono'
              }
            >
              {ver}
              {isOutdated && (
                <span className="ml-1">
                  {t('clusterManagement.needsUpgrade', 'Needs upgrade')}
                </span>
              )}
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
    [getClusterTypeBadge, getStatusBadge, t, versionInfo]
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
              <DropdownMenuItem
                onClick={() => {
                  setEditingCluster(row.original)
                  setShowClusterDialog(true)
                }}
                className="gap-2"
              >
                <IconEdit className="h-4 w-4" />
                {t('common.actions.edit', 'Edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={row.original.isDefault}
                onClick={() => setDeletingCluster(row.original)}
                className="gap-2"
              >
                <div className="inline-flex items-center gap-2 text-destructive">
                  <IconTrash className="h-4 w-4" />
                  {t('common.actions.delete', 'Delete')}
                </div>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ),
    }
    return [...columns, actionColumn]
  }, [columns, t])

  const table = useReactTable({
    data: clusters,
    columns: tableColumns,
    getCoreRowModel: getCoreRowModel(),
    state: { pagination },
    onPaginationChange: setPagination,
    manualPagination: true,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize) || 0,
  })

  const createMutation = useMutation({
    mutationFn: createCluster,
    onSuccess: async ({
      connectorServer,
      connectorToken,
      connectorManifestURL,
    }: {
      connectorServer?: string
      connectorToken?: string
      connectorManifestURL?: string
    }) => {
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paged'] })
      toast.success(
        t('clusterManagement.messages.created', 'Cluster created successfully')
      )
      setShowClusterDialog(false)
      if (connectorServer && connectorToken) {
        setConnectorCopyError(null)
        setConnectorCommand(
          `kite connector --server='${connectorServer}' --token='${connectorToken}'`
        )
        setConnectorYaml('')
        setConnectorYamlError(null)
        setConnectorManifestURL(connectorManifestURL || '')
        setIsConnectorYamlLoading(true)
        const requestID = ++connectorYamlRequestID.current
        try {
          if (!connectorManifestURL) throw new Error('Missing manifest URL')
          const manifestURL = new URL(
            connectorManifestURL,
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
          if (requestID === connectorYamlRequestID.current) {
            setConnectorYaml(yaml)
          }
        } catch {
          if (requestID === connectorYamlRequestID.current) {
            setConnectorYamlError(
              t(
                'clusterManagement.connector.loadYamlError',
                'Failed to load YAML from the manifest URL.'
              )
            )
          }
        } finally {
          if (requestID === connectorYamlRequestID.current) {
            setIsConnectorYamlLoading(false)
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
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paged'] })
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
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paged'] })
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
      queryClient.invalidateQueries({ queryKey: ['cluster-list-paged'] })
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
            data={clusters}
            allPageSize={data?.total ?? 0}
            emptyState={
              isLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="text-muted-foreground">
                    {t('common.messages.loading', 'Loading...')}
                  </div>
                </div>
              ) : error ? (
                <div className="flex items-center justify-center py-8">
                  <div className="text-destructive">
                    {t(
                      'clusterManagement.errors.loadFailed',
                      'Failed to load clusters'
                    )}
                  </div>
                </div>
              ) : clusters.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  <IconServer className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>
                    {t(
                      'clusterManagement.empty.title',
                      'No clusters configured'
                    )}
                  </p>
                  <p className="text-sm mt-1">
                    {t(
                      'clusterManagement.empty.description',
                      'Add your first cluster to get started'
                    )}
                  </p>
                </div>
              ) : null
            }
            hasActiveFilters={false}
            filteredRowCount={clusters.length}
            totalRowCount={data?.total ?? 0}
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
        open={!!connectorCommand}
        onOpenChange={(open) => {
          if (!open) {
            connectorYamlRequestID.current += 1
            setConnectorCommand('')
            setConnectorYaml('')
            setConnectorYamlError(null)
            setIsConnectorYamlLoading(false)
            setConnectorManifestURL('')
            setConnectorCopyError(null)
          }
        }}
      >
        <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle className="text-balance">
              {t('clusterManagement.connector.title', 'Connect Kite Connector')}
            </DialogTitle>
            <DialogDescription className="text-pretty">
              {t(
                'clusterManagement.connector.description',
                'Choose a command or Kubernetes YAML to run inside the target cluster. This connection information is shown only once.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {/* Section 1: Command */}
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">
                {t(
                  'clusterManagement.connector.runCommand',
                  'Run Connector directly'
                )}
              </Label>
              <div className="flex gap-2">
                <Input
                  readOnly
                  className="font-mono"
                  aria-label={t(
                    'clusterManagement.connector.command',
                    'Command'
                  )}
                  value={connectorCommand}
                />
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  aria-label={t(
                    'clusterManagement.connector.copyCommand',
                    'Copy command'
                  )}
                  onClick={async () => {
                    if (!connectorCommand) return
                    try {
                      await navigator.clipboard.writeText(connectorCommand)
                      setConnectorCopyError(null)
                      toast.success(t('common.messages.copied', 'Copied'))
                    } catch {
                      setConnectorCopyError('command')
                    }
                  }}
                >
                  <IconCopy className="size-4" />
                </Button>
              </div>
              {connectorCopyError === 'command' && (
                <p role="alert" className="text-sm text-destructive">
                  {t(
                    'clusterManagement.connector.copyError',
                    'Failed to copy. Copy the content manually.'
                  )}
                </p>
              )}
            </div>

            <div className="border-t" />

            {/* Section 2: URL deploy */}
            {connectorManifestURL && (
              <div className="space-y-2">
                <Label className="text-xs text-muted-foreground">
                  {t(
                    'clusterManagement.connector.applyUrl',
                    'Deploy via Kubernetes YAML URL'
                  )}
                </Label>
                <div className="flex gap-2">
                  <Input
                    readOnly
                    className="font-mono text-xs"
                    value={`kubectl apply -f "${connectorManifestURL}"`}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    aria-label={t(
                      'clusterManagement.connector.copyApplyCommand',
                      'Copy apply command'
                    )}
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(
                          `kubectl apply -f "${connectorManifestURL}"`
                        )
                        setConnectorCopyError(null)
                        toast.success(t('common.messages.copied', 'Copied'))
                      } catch {
                        setConnectorCopyError('yaml')
                      }
                    }}
                  >
                    <IconCopy className="size-4" />
                  </Button>
                </div>
              </div>
            )}

            <div className="border-t" />

            {/* Section 3: Copy YAML */}
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">
                {t(
                  'clusterManagement.connector.copyYamlManually',
                  'Copy Kubernetes YAML manually'
                )}
              </Label>
              {isConnectorYamlLoading ? (
                <Skeleton className="h-96 w-full" />
              ) : connectorYamlError ? (
                <p role="alert" className="text-sm text-destructive">
                  {connectorYamlError}
                </p>
              ) : (
                <Textarea
                  readOnly
                  className="h-96 resize-none font-mono text-xs"
                  aria-label={t(
                    'clusterManagement.connector.yaml',
                    'Kubernetes YAML'
                  )}
                  value={connectorYaml}
                />
              )}
              {connectorYaml && (
                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(connectorYaml)
                        setConnectorCopyError(null)
                        toast.success(t('common.messages.copied', 'Copied'))
                      } catch {
                        setConnectorCopyError('yaml')
                      }
                    }}
                  >
                    <IconCopy className="size-4" />
                    {t('clusterManagement.connector.copyYaml', 'Copy YAML')}
                  </Button>
                </div>
              )}
              {connectorCopyError === 'yaml' && (
                <p role="alert" className="text-sm text-destructive">
                  {t(
                    'clusterManagement.connector.copyError',
                    'Failed to copy. Copy the content manually.'
                  )}
                </p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              onClick={() => {
                connectorYamlRequestID.current += 1
                setConnectorCommand('')
                setConnectorYaml('')
                setConnectorYamlError(null)
                setIsConnectorYamlLoading(false)
                setConnectorManifestURL('')
                setConnectorCopyError(null)
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
        resourceType="cluster"
        additionalNote={t(
          'clusterManagement.deleteConfirmation',
          "This action will only remove the current cluster's configuration in kite and will not delete any cluster resources."
        )}
      />
    </div>
  )
}
