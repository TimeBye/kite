import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ColumnDef,
  getCoreRowModel,
  PaginationState,
  useReactTable,
} from '@tanstack/react-table'
import { KeyRound, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  deleteAdminKubeconfigToken,
  KubeconfigToken,
  KubeconfigTokenStatus,
  useAdminKubeconfigTokens,
} from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ResourceTableView } from '@/components/resource-table-view'
import { DeleteConfirmationDialog } from '@/components/delete-confirmation-dialog'

export function KubeconfigTokenManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [owner, setOwner] = useState('')
  const [status, setStatus] = useState<KubeconfigTokenStatus | undefined>()
  const [tokenToDelete, setTokenToDelete] = useState<KubeconfigToken | null>(
    null
  )
  const { data, isLoading, error } = useAdminKubeconfigTokens({
    page: pagination.pageIndex + 1,
    size: pagination.pageSize,
    owner: owner || undefined,
    status,
  })
  const deleteMutation = useMutation({
    mutationFn: deleteAdminKubeconfigToken,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-kubeconfig-tokens'] })
      toast.success(
        t('kubeconfigTokens.deleteSuccess', 'Kubeconfig token deleted')
      )
      setTokenToDelete(null)
    },
    onError: (error) => toast.error(error.message),
  })

  const tokens = data?.tokens ?? []
  const totalRowCount = data?.total ?? 0

  const columns = useMemo<ColumnDef<KubeconfigToken>[]>(
    () => [
      {
        id: 'owner',
        header: t('common.fields.owner', 'Owner'),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.owner || '-'}</span>
        ),
      },
      {
        id: 'createdAt',
        header: t('common.fields.createdAt', 'Created At'),
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {new Date(row.original.createdAt).toLocaleString()}
          </span>
        ),
      },
      {
        id: 'expiresAt',
        header: t('kubeconfigTokens.expiresAt', 'Expires At'),
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {new Date(row.original.expiresAt).toLocaleString()}
          </span>
        ),
      },
      {
        id: 'lastUsedAt',
        header: t('common.fields.lastUsed', 'Last Used'),
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.lastUsedAt
              ? new Date(row.original.lastUsedAt).toLocaleString()
              : t('common.messages.neverUsed', 'Never')}
          </span>
        ),
      },
      {
        id: 'status',
        header: t('common.fields.status', 'Status'),
        cell: ({ row }) => {
          const expired = new Date(row.original.expiresAt) <= new Date()
          return (
            <Badge
              variant="outline"
              className={
                expired
                  ? 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
                  : 'border-green-500/30 bg-green-500/10 text-green-700 dark:text-green-300'
              }
            >
              {expired
                ? t('kubeconfigTokens.expired', 'Expired')
                : t('kubeconfigTokens.active', 'Active')}
            </Badge>
          )
        },
      },
      {
        id: 'actions',
        header: t('common.fields.actions', 'Actions'),
        cell: ({ row }) => (
          <div className="text-right">
            <Button
              variant="ghost"
              size="icon"
              disabled={deleteMutation.isPending && tokenToDelete?.id === row.original.id}
              aria-label={t('kubeconfigTokens.delete', 'Delete')}
              onClick={() => setTokenToDelete(row.original)}
            >
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        ),
      },
    ],
    [deleteMutation.isPending, t, tokenToDelete?.id]
  )

  const table = useReactTable({
    data: tokens,
    columns,
    getCoreRowModel: getCoreRowModel(),
    state: { pagination },
    onPaginationChange: setPagination,
    manualPagination: true,
    pageCount: Math.ceil(totalRowCount / pagination.pageSize) || 0,
  })

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
          {error.message}
        </div>
      )
    }
    if (tokens.length === 0) {
      return (
        <div className="py-10 text-center text-muted-foreground">
          {t('kubeconfigTokens.empty', 'No kubeconfig tokens found.')}
        </div>
      )
    }
    return null
  })()

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="h-5 w-5" />
              {t('kubeconfigTokens.adminTitle', 'Kubeconfig Tokens')}
            </CardTitle>
            <p className="text-sm text-muted-foreground">
              {t(
                'kubeconfigTokens.adminDescription',
                'Review and delete kubeconfig tokens for all users.'
              )}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Input
              className="w-64"
              value={owner}
              onChange={(event) => {
                setOwner(event.target.value)
                setPagination((prev) => ({ ...prev, pageIndex: 0 }))
              }}
              placeholder={t(
                'kubeconfigTokens.ownerPlaceholder',
                'Filter by username'
              )}
            />
            <Select
              value={status ?? 'all'}
              onValueChange={(value) => {
                setStatus(
                  value === 'all' ? undefined : (value as KubeconfigTokenStatus)
                )
                setPagination((prev) => ({ ...prev, pageIndex: 0 }))
              }}
            >
              <SelectTrigger className="w-44">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {t('kubeconfigTokens.allStatuses', 'All statuses')}
                </SelectItem>
                <SelectItem value="active">
                  {t('kubeconfigTokens.active', 'Active')}
                </SelectItem>
                <SelectItem value="expired">
                  {t('kubeconfigTokens.expired', 'Expired')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ResourceTableView
          table={table}
          columnCount={columns.length}
          isLoading={isLoading}
          data={tokens}
          allPageSize={totalRowCount}
          emptyState={emptyState}
          hasActiveFilters={Boolean(owner) || Boolean(status)}
          filteredRowCount={tokens.length}
          totalRowCount={totalRowCount}
          searchQuery={owner}
          pagination={pagination}
          setPagination={setPagination}
          fitViewportHeight
        />
      </CardContent>

      <DeleteConfirmationDialog
        open={!!tokenToDelete}
        onOpenChange={() => setTokenToDelete(null)}
        onConfirm={() => {
          if (tokenToDelete) deleteMutation.mutate(tokenToDelete.id)
        }}
        resourceName={tokenToDelete?.name || ''}
        resourceType={t('kubeconfigTokens.adminTitle', 'Kubeconfig Tokens')}
        additionalNote={t(
          'kubeconfigTokens.deleteConfirmDescription',
          'This token will be deleted, become invalid immediately, and cannot be recovered.'
        )}
      />
    </Card>
  )
}
