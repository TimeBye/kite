import { useMemo, useState } from 'react'
import {
  IconArrowsMaximize,
  IconCircleCheckFilled,
  IconLoader,
  IconReload,
} from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { createColumnHelper } from '@tanstack/react-table'
import { StatefulSet } from 'kubernetes-types/apps/v1'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { patchResource, restartWorkload } from '@/lib/api'
import { createSearchFilter } from '@/lib/k8s'
import { formatDate } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ResourceBatchActionDialog } from '@/components/resource-batch-action-dialog'
import {
  ResourceTable,
  type ResourceTableBatchAction,
} from '@/components/resource-table'

const statefulSetSearchFilter = createSearchFilter<StatefulSet>(
  (s) => s.metadata?.name,
  (s) => s.metadata?.namespace,
  (s) => s.spec?.serviceName
)

const columnHelper = createColumnHelper<StatefulSet>()

export function StatefulSetListPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [isRestartDialogOpen, setIsRestartDialogOpen] = useState(false)
  const [isScaleDialogOpen, setIsScaleDialogOpen] = useState(false)
  const [replicas, setReplicas] = useState('1')
  const validReplicas =
    replicas.trim() !== '' &&
    Number.isInteger(Number(replicas)) &&
    Number(replicas) >= 0
  const [batchStatefulSets, setBatchStatefulSets] = useState<StatefulSet[]>([])

  const batchActions = useMemo<ResourceTableBatchAction<StatefulSet>[]>(
    () => [
      {
        id: 'scale',
        label: t('common.actions.scale'),
        icon: <IconArrowsMaximize className="size-4" />,
        onSelect: (statefulSets) => {
          setBatchStatefulSets(statefulSets)
          setReplicas('1')
          setIsScaleDialogOpen(true)
        },
      },
      {
        id: 'restart',
        label: t('common.actions.restart'),
        icon: <IconReload className="size-4" />,
        onSelect: (statefulSets) => {
          setBatchStatefulSets(statefulSets)
          setIsRestartDialogOpen(true)
        },
      },
    ],
    [t]
  )

  // Define columns for the statefulset table
  const columns = useMemo(
    () => [
      columnHelper.accessor('metadata.name', {
        header: t('common.fields.name'),
        cell: ({ row }) => (
          <div className="font-medium app-link">
            <Link
              to={`/statefulsets/${row.original.metadata!.namespace}/${
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
      columnHelper.accessor(
        (row) => {
          const readyReplicas = row.status?.readyReplicas || 0
          const replicas = row.status?.replicas || 0
          if (replicas === 0) return '-'
          return readyReplicas === replicas
            ? t('common.fields.available')
            : t('common.messages.loading')
        },
        {
          id: 'status_conditions',
          header: t('common.fields.status'),
          cell: ({ row, getValue }) => {
            const readyReplicas = row.original.status?.readyReplicas || 0
            const replicas = row.original.status?.replicas || 0
            const isAvailable = readyReplicas === replicas
            const status = getValue()
            if (replicas === 0) {
              return (
                <Badge
                  variant="secondary"
                  className="text-muted-foreground px-1.5"
                >
                  -
                </Badge>
              )
            }

            return (
              <Badge variant="outline" className="text-muted-foreground px-1.5">
                {isAvailable ? (
                  <IconCircleCheckFilled className="fill-green-500 dark:fill-green-400" />
                ) : (
                  <IconLoader className="animate-spin" />
                )}
                {status}
              </Badge>
            )
          },
        }
      ),
      columnHelper.accessor('spec.serviceName', {
        header: t('common.fields.serviceName'),
        cell: ({ getValue }) => getValue() || '-',
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

  return (
    <>
      <ResourceTable
        resourceName={'StatefulSets'}
        resourceType="statefulsets"
        columns={columns}
        searchQueryFilter={statefulSetSearchFilter}
        batchActions={batchActions}
      />

      <ResourceBatchActionDialog
        open={isScaleDialogOpen}
        onOpenChange={setIsScaleDialogOpen}
        resources={batchStatefulSets}
        title={t('resourceTable.batchActions.scaleResourcesTitle', {
          count: batchStatefulSets.length,
          resource: t('nav.statefulsets'),
        })}
        description={t('resourceTable.batchActions.scaleResourcesDescription')}
        actionLabel={t('common.actions.scale')}
        confirmDisabled={!validReplicas}
        options={(disabled) => (
          <div className="space-y-2">
            <Label htmlFor="statefulset-batch-replicas">
              {t('common.fields.replicas')}
            </Label>
            <Input
              id="statefulset-batch-replicas"
              type="number"
              min={0}
              step={1}
              value={replicas}
              onChange={(event) => setReplicas(event.target.value)}
              disabled={disabled}
            />
          </div>
        )}
        onExecute={(statefulSet) =>
          patchResource(
            'statefulsets',
            statefulSet.metadata!.name!,
            statefulSet.metadata!.namespace!,
            {
              spec: { replicas: Number(replicas) },
            }
          )
        }
        onComplete={() =>
          queryClient.invalidateQueries({ queryKey: ['statefulsets'] })
        }
      />

      <ResourceBatchActionDialog
        open={isRestartDialogOpen}
        onOpenChange={setIsRestartDialogOpen}
        resources={batchStatefulSets}
        title={t('resourceTable.batchActions.restartResourcesTitle', {
          count: batchStatefulSets.length,
          resource: t('nav.statefulsets'),
        })}
        description={t(
          'resourceTable.batchActions.restartResourcesDescription',
          { resource: t('nav.statefulsets') }
        )}
        actionLabel={t('common.actions.restart')}
        onExecute={(statefulSet) =>
          restartWorkload(
            'statefulsets',
            statefulSet.metadata!.name!,
            statefulSet.metadata!.namespace!
          )
        }
        onComplete={() =>
          queryClient.invalidateQueries({ queryKey: ['statefulsets'] })
        }
        destructive={true}
      />
    </>
  )
}
