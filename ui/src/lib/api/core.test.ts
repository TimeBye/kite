import { describe, expect, it, vi } from 'vitest'

import { ApiError } from '@/lib/api-error'

vi.mock('../api-client', () => ({
  apiClient: {
    get: vi.fn(),
  },
  API_BASE_URL: '/api/v1',
}))

vi.mock('@/lib/resource-metadata', () => ({
  getResourceQueryKey: vi.fn(() => ['resource']),
}))

vi.mock('@/hooks/use-cluster', () => ({
  useCluster: vi.fn(() => ({ currentCluster: '' })),
}))

vi.mock('@/i18n', () => ({
  default: { t: (key: string) => key },
}))

import { apiClient } from '../api-client'
import { getRelatedResources } from './core'

describe('getRelatedResources', () => {
  it('returns empty array on 404 (route not registered)', async () => {
    vi.mocked(apiClient.get).mockRejectedValueOnce(
      new ApiError('HTTP error! status: 404')
    )

    const result = await getRelatedResources('nodes', 'my-node')
    expect(result).toEqual([])
  })

  it('rethrows non-404 errors', async () => {
    vi.mocked(apiClient.get).mockRejectedValueOnce(
      new ApiError('HTTP error! status: 500')
    )

    await expect(
      getRelatedResources('pods', 'my-pod', 'default')
    ).rejects.toThrow('HTTP error! status: 500')
  })

  it('returns data on success', async () => {
    const mockData = [
      { type: 'services' as const, name: 'my-svc', namespace: 'default' },
    ]
    vi.mocked(apiClient.get).mockResolvedValueOnce(mockData)

    const result = await getRelatedResources('pods', 'my-pod', 'default')
    expect(result).toEqual(mockData)
  })

  it('rethrows non-ApiError errors', async () => {
    vi.mocked(apiClient.get).mockRejectedValueOnce(new Error('Network error'))

    await expect(
      getRelatedResources('pods', 'my-pod', 'default')
    ).rejects.toThrow('Network error')
  })
})
