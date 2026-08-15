/// <reference types="@testing-library/jest-dom" />

import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { KubeconfigDownloadDialog } from './kubeconfig-download-dialog'

const { useCluster } = vi.hoisted(() => ({
  useCluster: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string) => defaultValue ?? _key,
  }),
}))

vi.mock('@/lib/api', () => ({
  downloadKubeconfig: vi.fn(),
}))

vi.mock('@/hooks/use-cluster', () => ({ useCluster }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const clusters = [
  { uuid: 'current-id', name: 'current', enabled: true },
  { uuid: 'other-id', name: 'other', enabled: true },
  { uuid: 'disabled-id', name: 'disabled', enabled: false },
]

function renderDialog() {
  const onOpenChange = vi.fn()
  render(<KubeconfigDownloadDialog open onOpenChange={onOpenChange} />)
  return onOpenChange
}

describe('KubeconfigDownloadDialog', () => {
  it('defaults to the current cluster and supports selecting multiple clusters and all clusters', async () => {
    useCluster.mockReturnValue({ clusters, currentCluster: 'current' })
    renderDialog()
    const user = userEvent.setup()

    expect(screen.getByRole('checkbox', { name: /current/i })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^other$/i })).not.toBeChecked()
    expect(screen.queryByText('disabled')).not.toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: /^other$/i }))
    expect(screen.getByRole('checkbox', { name: /current/i })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^other$/i })).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Deselect all' }))
    expect(screen.getByRole('checkbox', { name: /current/i })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^other$/i })).not.toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Select all' }))
    expect(screen.getByRole('checkbox', { name: /current/i })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^other$/i })).toBeChecked()
  })

  it('filters clusters by name', async () => {
    useCluster.mockReturnValue({ clusters, currentCluster: 'current' })
    renderDialog()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Search clusters'), 'other')

    expect(screen.getByRole('checkbox', { name: /^other$/i })).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: /current/i })).not.toBeInTheDocument()

    await user.clear(screen.getByPlaceholderText('Search clusters'))
    await user.type(screen.getByPlaceholderText('Search clusters'), 'missing')

    expect(screen.getByText('No matching clusters found.')).toBeInTheDocument()
  })

  it('renders concise expiration presets', () => {
    useCluster.mockReturnValue({ clusters, currentCluster: 'current' })
    renderDialog()

    for (const preset of ['1d', '7d', '30d', '1year']) {
      expect(screen.getByRole('button', { name: preset })).toBeInTheDocument()
    }
  })

  it('uses selectors for a custom expiration time', async () => {
    useCluster.mockReturnValue({ clusters, currentCluster: 'current' })
    renderDialog()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: 'Custom' }))

    for (const field of ['year', 'month', 'day', 'hour', 'minute', 'second']) {
      expect(screen.getByRole('combobox', { name: field })).toBeInTheDocument()
    }
    expect(screen.queryByRole('textbox', { name: 'second' })).not.toBeInTheDocument()
  })
})
