import { useMemo, useState } from 'react'
import { IconArrowsMaximize, IconReload } from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { createColumnHelper } from '@tanstack/react-table'
import { Deployment } from 'kubernetes-types/apps/v1'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'

import { patchResource, restartWorkload } from '@/lib/api'
import { createSearchFilter, getDeploymentStatus } from '@/lib/k8s'
import { formatDate } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { DeploymentStatusIcon } from '@/components/deployment-status-icon'
import { DeploymentCreateDialog } from '@/components/editors/deployment-create-dialog'
import { ResourceBatchActionDialog } from '@/components/resource-batch-action-dialog'
import {
  ResourceTable,
  type ResourceTableBatchAction,
} from '@/components/resource-table'

const deploymentSearchFilter = createSearchFilter<Deployment>(
  (d) => d.metadata?.name,
  (d) => d.metadata?.namespace
)

const columnHelper = createColumnHelper<Deployment>()

export function DeploymentListPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isRestartDialogOpen, setIsRestartDialogOpen] = useState(false)
  const [isScaleDialogOpen, setIsScaleDialogOpen] = useState(false)
  const [replicas, setReplicas] = useState('1')
  const validReplicas =
    replicas.trim() !== '' &&
    Number.isInteger(Number(replicas)) &&
    Number(replicas) >= 0
  const [batchDeployments, setBatchDeployments] = useState<Deployment[]>([])

  const batchActions = useMemo<ResourceTableBatchAction<Deployment>[]>(
    () => [
      {
        id: 'scale',
        label: t('common.actions.scale'),
        icon: <IconArrowsMaximize className="size-4" />,
        onSelect: (deployments) => {
          setBatchDeployments(deployments)
          setReplicas('1')
          setIsScaleDialogOpen(true)
        },
      },
      {
        id: 'restart',
        label: t('common.actions.restart'),
        icon: <IconReload className="size-4" />,
        onSelect: (deployments) => {
          setBatchDeployments(deployments)
          setIsRestartDialogOpen(true)
        },
      },
    ],
    [t]
  )

  // Define columns for the deployment table
  const columns = useMemo(
    () => [
      columnHelper.accessor('metadata.name', {
        header: t('common.fields.name'),
        cell: ({ row }) => (
          <div className="font-medium app-link">
            <Link
              to={`/deployments/${row.original.metadata!.namespace}/${
                row.original.metadata!.name
              }`}
            >
              {row.original.metadata!.name}
            </Link>
          </div>
        ),
      }),
      columnHelper.accessor((row) => row.status?.readyReplicas ?? 0, {
        id: 'ready',
        header: t('common.fields.ready'),
        cell: ({ row }) => {
          const status = row.original.status
          const ready = status?.readyReplicas || 0
          const desired = status?.replicas || 0
          return (
            <div>
              {ready} / {desired}
            </div>
          )
        },
      }),
      columnHelper.accessor((row) => getDeploymentStatus(row), {
        id: 'status_conditions',
        header: t('common.fields.status'),
        cell: ({ getValue }) => {
          const status = getValue()
          return (
            <Badge variant="outline" className="text-muted-foreground px-1.5">
              <DeploymentStatusIcon status={status} />
              {status}
            </Badge>
          )
        },
      }),
      columnHelper.accessor('metadata.creationTimestamp', {
        header: t('common.fields.created'),
        cell: ({ getValue }) => {
          const dateStr = formatDate(getValue() || '')

          return (
            <span className="text-muted-foreground text-sm">{dateStr}</span>
          )
        },
      }),
    ],
    [t]
  )

  const handleCreateClick = () => {
    setIsCreateDialogOpen(true)
  }

  const handleCreateSuccess = (deployment: Deployment, namespace: string) => {
    // Navigate to the newly created deployment's detail page
    navigate(`/deployments/${namespace}/${deployment.metadata?.name}`)
  }

  return (
    <>
      <ResourceTable
        resourceName="Deployments"
        columns={columns}
        searchQueryFilter={deploymentSearchFilter}
        showCreateButton={true}
        onCreateClick={handleCreateClick}
        batchActions={batchActions}
      />

      <DeploymentCreateDialog
        open={isCreateDialogOpen}
        onOpenChange={setIsCreateDialogOpen}
        onSuccess={handleCreateSuccess}
      />

      <ResourceBatchActionDialog
        open={isScaleDialogOpen}
        onOpenChange={setIsScaleDialogOpen}
        resources={batchDeployments}
        title={t('resourceTable.batchActions.scaleResourcesTitle', {
          count: batchDeployments.length,
          resource: t('nav.deployments'),
        })}
        description={t('resourceTable.batchActions.scaleResourcesDescription')}
        actionLabel={t('common.actions.scale')}
        confirmDisabled={!validReplicas}
        options={(disabled) => (
          <div className="space-y-2">
            <Label htmlFor="deployment-batch-replicas">
              {t('common.fields.replicas')}
            </Label>
            <Input
              id="deployment-batch-replicas"
              type="number"
              min={0}
              step={1}
              value={replicas}
              onChange={(event) => setReplicas(event.target.value)}
              disabled={disabled}
            />
          </div>
        )}
        onExecute={(deployment) =>
          patchResource(
            'deployments',
            deployment.metadata!.name!,
            deployment.metadata!.namespace!,
            {
              spec: { replicas: Number(replicas) },
            }
          )
        }
        onComplete={() =>
          queryClient.invalidateQueries({ queryKey: ['deployments'] })
        }
      />

      <ResourceBatchActionDialog
        open={isRestartDialogOpen}
        onOpenChange={setIsRestartDialogOpen}
        resources={batchDeployments}
        title={t('resourceTable.batchActions.restartResourcesTitle', {
          count: batchDeployments.length,
          resource: t('nav.deployments'),
        })}
        description={t(
          'resourceTable.batchActions.restartResourcesDescription',
          { resource: t('nav.deployments') }
        )}
        actionLabel={t('common.actions.restart')}
        onExecute={(deployment) =>
          restartWorkload(
            'deployments',
            deployment.metadata!.name!,
            deployment.metadata!.namespace!
          )
        }
        onComplete={() =>
          queryClient.invalidateQueries({ queryKey: ['deployments'] })
        }
        destructive={true}
      />
    </>
  )
}
