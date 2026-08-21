/// <reference types="@testing-library/jest-dom" />

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ClusterManagement } from './cluster-management'

const { useClusterListPaginated, useVersionInfo } = vi.hoisted(() => ({
  useClusterListPaginated: vi.fn(),
  useVersionInfo: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string) => defaultValue ?? _key,
  }),
}))

vi.mock('@/lib/api', () => ({
  useClusterListPaginated,
  useVersionInfo,
  createCluster: vi.fn(),
  deleteCluster: vi.fn(),
  importClusters: vi.fn(),
  updateCluster: vi.fn(),
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

vi.mock('./cluster-dialog', () => ({ ClusterDialog: () => null }))
vi.mock('./cluster-import-dialog', () => ({ ClusterImportDialog: () => null }))

const clusters = [
  {
    id: 1,
    name: 'up-to-date',
    enabled: true,
    inCluster: false,
    clusterAgent: true,
    clusterAgentVersion: 'v0.15.0',
    connected: true,
    isDefault: false,
    createdAt: '',
    updatedAt: '',
  },
  {
    id: 2,
    name: 'upgrade-required',
    enabled: true,
    inCluster: false,
    clusterAgent: true,
    clusterAgentVersion: 'v0.14.0',
    connected: true,
    isDefault: false,
    createdAt: '',
    updatedAt: '',
  },
  {
    id: 3,
    name: 'unknown',
    enabled: true,
    inCluster: false,
    clusterAgent: true,
    connected: false,
    isDefault: false,
    createdAt: '',
    updatedAt: '',
  },
  {
    id: 4,
    name: 'external',
    enabled: true,
    inCluster: false,
    clusterAgent: false,
    connected: false,
    isDefault: false,
    createdAt: '',
    updatedAt: '',
  },
]

function renderManagement() {
  useClusterListPaginated.mockReturnValue({
    data: { data: clusters, total: clusters.length, page: 1, size: 20 },
    isLoading: false,
  })
  useVersionInfo.mockReturnValue({ data: { version: 'v0.15.0' } })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <ClusterManagement />
    </QueryClientProvider>
  )
}

describe('ClusterManagement Agent Version', () => {
  it('shows the version and upgrade status for Cluster Agent clusters', () => {
    renderManagement()

    expect(screen.getByText('v0.15.0')).toBeInTheDocument()
    expect(screen.getByText('Up to date')).toBeInTheDocument()
    expect(screen.getByText('v0.14.0')).toBeInTheDocument()
    expect(screen.getByText('Upgrade required')).toBeInTheDocument()
    expect(screen.getByText('Unknown')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
  })
})
