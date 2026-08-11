import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { apiClient } from '@/lib/api-client'
import { useCluster } from '@/hooks/use-cluster'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface KubeconfigDownloadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function KubeconfigDownloadDialog({
  open,
  onOpenChange,
}: KubeconfigDownloadDialogProps) {
  const { t } = useTranslation()
  const { clusters, currentCluster } = useCluster()
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [downloading, setDownloading] = useState(false)
  const [showAll, setShowAll] = useState(false)

  // Reset state when dialog closes; default-select current cluster on open.
  useEffect(() => {
    if (!open) {
      setSelected(new Set())
      setShowAll(false)
    } else if (currentCluster) {
      const current = clusters.find((c) => c.name === currentCluster)
      if (current?.uuid) {
        setSelected(new Set([current.uuid]))
      }
    }
  }, [open, currentCluster, clusters])

  const clustersWithUuid = clusters.filter((c) => c.uuid)
  const visibleClusters =
    showAll || clustersWithUuid.length <= 8
      ? clustersWithUuid
      : clustersWithUuid.slice(0, 8)

  const handleToggle = (uuid: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(uuid)) {
        next.delete(uuid)
      } else {
        next.add(uuid)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    if (selected.size === clustersWithUuid.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(clustersWithUuid.map((c) => c.uuid!)))
    }
  }

  const handleDownload = async () => {
    if (selected.size === 0) return
    setDownloading(true)
    try {
      const uuids = Array.from(selected).join(',')
      const response = await apiClient.request(
        `/kubeconfig?clusters=${encodeURIComponent(uuids)}`
      )
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error || 'Download failed')
      }
      const text = await response.text()
      const blob = new Blob([text], { type: 'text/yaml' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'kubeconfig'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      toast.success(
        t('clusterManagement.kubeconfig.downloaded', 'Kubeconfig downloaded')
      )
      onOpenChange(false)
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : t('clusterManagement.kubeconfig.downloadError', 'Download failed')
      )
    } finally {
      setDownloading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>
            {t('clusterManagement.kubeconfig.title', 'Download Kubeconfig')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'clusterManagement.kubeconfig.description',
              'Generate a kubeconfig that uses Kite as a proxy to access the cluster.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">
              {t('clusterManagement.kubeconfig.clusters', 'Clusters')}
            </span>
            {clustersWithUuid.length > 0 && (
              <Button variant="ghost" size="sm" onClick={handleSelectAll}>
                {selected.size === clustersWithUuid.length
                  ? t('resourceTable.deselectAll', 'Deselect All')
                  : t('resourceTable.selectAll', 'Select All ({{count}})', {
                      count: clustersWithUuid.length,
                    })}
              </Button>
            )}
          </div>
          <div className="h-[240px] overflow-y-auto rounded-md border p-2">
            <div className="space-y-1">
              {visibleClusters.map((cluster) => (
                <label
                  key={cluster.uuid}
                  className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent"
                >
                  <Checkbox
                    checked={selected.has(cluster.uuid!)}
                    onCheckedChange={() => handleToggle(cluster.uuid!)}
                  />
                  <span className="text-sm">{cluster.name}</span>
                  {cluster.name === currentCluster && (
                    <span className="text-xs text-muted-foreground">
                      (current)
                    </span>
                  )}
                </label>
              ))}
              {clustersWithUuid.length > 8 && !showAll && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full"
                  onClick={() => setShowAll(true)}
                >
                  {t(
                    'clusterManagement.kubeconfig.showAll',
                    'Show all ({{count}})',
                    {
                      count: clustersWithUuid.length,
                    }
                  )}
                </Button>
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={downloading}
          >
            {t('common.actions.cancel', 'Cancel')}
          </Button>
          <Button
            onClick={handleDownload}
            disabled={selected.size === 0 || downloading}
          >
            {downloading
              ? t('common.actions.downloading', 'Downloading...')
              : t('common.actions.download', 'Download')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
