import { describe, expect, it, vi } from 'vitest'

vi.mock('../api-client', () => ({
  apiClient: {
    get: vi.fn(),
    request: vi.fn(),
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
import {
  downloadSingleYAML,
  downloadBatchYAML,
  triggerBrowserDownload,
} from './core'

function makeMockResponse(
  ok: boolean,
  body: string,
  contentType = 'application/yaml'
) {
  const blob = new Blob([body], { type: contentType })
  return {
    ok,
    status: ok ? 200 : 400,
    blob: () => Promise.resolve(blob),
    json: () => Promise.resolve({ error: body }),
  } as unknown as Response
}

describe('downloadSingleYAML', () => {
  it('constructs namespace-scoped URL correctly', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'apiVersion: v1\nkind: Pod\n')
    )

    await downloadSingleYAML('pods', 'nginx', 'default', false)

    expect(apiClient.request).toHaveBeenCalledWith(
      '/pods/default/nginx/download?neat=false',
      { method: 'GET' }
    )
  })

  it('constructs cluster-scoped URL correctly', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'apiVersion: v1\nkind: Node\n')
    )

    await downloadSingleYAML('nodes', 'node-1', undefined, false)

    expect(apiClient.request).toHaveBeenCalledWith(
      '/nodes/_all/node-1/download?neat=false',
      { method: 'GET' }
    )
  })

  it('passes neat=true query parameter', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'apiVersion: v1\n')
    )

    await downloadSingleYAML('pods', 'nginx', 'default', true)

    expect(apiClient.request).toHaveBeenCalledWith(
      '/pods/default/nginx/download?neat=true',
      { method: 'GET' }
    )
  })

  it('returns blob on success', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'apiVersion: v1\nkind: Pod\n')
    )

    const blob = await downloadSingleYAML('pods', 'nginx', 'default', false)
    expect(blob).toBeInstanceOf(Blob)
  })

  it('throws on error response', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(false, 'resource not found')
    )

    await expect(
      downloadSingleYAML('pods', 'missing', 'default', false)
    ).rejects.toThrow('resource not found')
  })
})

describe('downloadBatchYAML', () => {
  it('constructs namespace-scoped batch URL', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'PK\x03\x04', 'application/zip')
    )

    const items = [
      { name: 'pod-1', namespace: 'default' },
      { name: 'pod-2', namespace: 'default' },
    ]

    await downloadBatchYAML('pods', items, false, false)

    expect(apiClient.request).toHaveBeenCalledWith(
      '/pods/download?neat=false',
      { method: 'POST', body: JSON.stringify(items) }
    )
  })

  it('constructs cluster-scoped batch URL', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'PK\x03\x04', 'application/zip')
    )

    const items = [
      { name: 'node-1', namespace: undefined },
      { name: 'node-2', namespace: undefined },
    ]

    await downloadBatchYAML('nodes', items, false, true)

    expect(apiClient.request).toHaveBeenCalledWith(
      '/nodes/_all/download?neat=false',
      { method: 'POST', body: JSON.stringify(items) }
    )
  })

  it('passes neat=true query parameter', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(true, 'PK\x03\x04', 'application/zip')
    )

    await downloadBatchYAML(
      'pods',
      [{ name: 'pod-1', namespace: 'default' }],
      true,
      false
    )

    expect(apiClient.request).toHaveBeenCalledWith(
      '/pods/download?neat=true',
      expect.objectContaining({ method: 'POST' })
    )
  })

  it('throws on error response', async () => {
    vi.mocked(apiClient.request).mockResolvedValueOnce(
      makeMockResponse(false, 'internal server error')
    )

    await expect(
      downloadBatchYAML(
        'pods',
        [{ name: 'pod-1', namespace: 'default' }],
        false,
        false
      )
    ).rejects.toThrow('internal server error')
  })
})

describe('triggerBrowserDownload', () => {
  it('creates an anchor element and triggers click', () => {
    const blob = new Blob(['test'], { type: 'text/plain' })

    const createObjectURLSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:test')
    const revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL')

    const clickSpy = vi.fn()
    const appendChildSpy = vi.spyOn(document.body, 'appendChild')
    const removeChildSpy = vi.spyOn(document.body, 'removeChild')

    // Mock createElement to return a trackable element
    const originalCreateElement = document.createElement
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = originalCreateElement.call(document, tag)
      el.click = clickSpy
      return el
    })

    triggerBrowserDownload(blob, 'test.yaml')

    expect(createObjectURLSpy).toHaveBeenCalledWith(blob)
    expect(clickSpy).toHaveBeenCalled()
    expect(appendChildSpy).toHaveBeenCalled()
    expect(removeChildSpy).toHaveBeenCalled()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:test')

    vi.restoreAllMocks()
  })
})
