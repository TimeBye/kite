import { useMemo, useState } from 'react'
import { IconCheck, IconChevronDown, IconServer } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'
import { useCluster } from '@/hooks/use-cluster'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export function ClusterSelector() {
  const {
    clusters,
    currentCluster,
    setCurrentCluster,
    isSwitching,
    isLoading,
  } = useCluster()
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')

  const filteredClusters = useMemo(() => {
    const query = searchTerm.trim().toLowerCase()
    if (!query) return clusters
    return clusters.filter((cluster) =>
      cluster.name.toLowerCase().includes(query)
    )
  }, [clusters, searchTerm])

  if (isLoading || isSwitching) {
    return (
      <div className="flex items-center justify-center">
        <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600" />
        {isSwitching && (
          <span className="ml-2 text-sm text-muted-foreground">
            Switching cluster...
          </span>
        )}
      </div>
    )
  }

  const currentClusterData = clusters.find((c) => c.name === currentCluster)

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) {
          setSearchTerm('')
        }
      }}
    >
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="flex items-center gap-2 h-8 px-3 max-w-full focus-visible:ring-0 focus-visible:border-transparent"
          disabled={isSwitching}
        >
          <IconServer className="h-4 w-4" />
          <span className="text-sm font-medium truncate">
            {isSwitching
              ? 'Switching...'
              : currentClusterData?.name || 'Select Cluster'}
          </span>
          <IconChevronDown className="h-3 w-3 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 p-0">
        <div className="p-2 border-b">
          <Input
            placeholder={t(
              'common.placeholders.searchClusterName',
              'Search cluster name...'
            )}
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
            autoFocus
          />
        </div>
        <div
          className="max-h-[300px] overflow-y-auto"
          onWheelCapture={(event) => event.stopPropagation()}
          onTouchMove={(event) => event.stopPropagation()}
        >
          {filteredClusters.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">
              {t('common.empty.noClustersFound', 'No clusters found.')}
            </div>
          ) : (
            filteredClusters.map((cluster) => {
              const isSelected = currentCluster === cluster.name
              return (
                <button
                  key={cluster.name}
                  type="button"
                  disabled={!!cluster.error}
                  className={cn(
                    'flex w-full items-start gap-2 px-3 py-2 text-left text-sm hover:bg-muted focus:bg-muted focus:outline-none disabled:opacity-50 disabled:hover:bg-transparent',
                    isSelected && 'bg-muted'
                  )}
                  onClick={() => {
                    setCurrentCluster(cluster.name)
                    setOpen(false)
                  }}
                >
                  <IconCheck
                    className={cn(
                      'mt-0.5 h-4 w-4 shrink-0',
                      isSelected ? 'opacity-100' : 'opacity-0'
                    )}
                  />
                  <div className="flex flex-col overflow-hidden">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{cluster.name}</span>
                      {cluster.isDefault && (
                        <Badge className="text-xs">Default</Badge>
                      )}
                      {cluster.error && (
                        <Badge variant="destructive" className="text-xs">
                          Sync Error
                        </Badge>
                      )}
                    </div>
                    <span
                      className={cn(
                        'text-xs truncate',
                        cluster.error
                          ? 'text-red-500'
                          : 'text-muted-foreground font-mono'
                      )}
                      title={cluster.error}
                    >
                      {cluster.error || cluster.version}
                    </span>
                  </div>
                </button>
              )
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
