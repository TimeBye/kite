/// <reference types="@testing-library/jest-dom" />
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { UserDisplayName } from './user-display-name'

describe('UserDisplayName', () => {
  it('shows login when name is undefined', () => {
    render(<UserDisplayName login="alice" />)
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.queryByText('Alice Smith')).not.toBeInTheDocument()
  })

  it('shows login when name is empty string', () => {
    render(<UserDisplayName name="" login="alice" />)
    expect(screen.getByText('alice')).toBeInTheDocument()
  })

  it('shows login when name equals login', () => {
    render(<UserDisplayName name="alice" login="alice" />)
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('shows name and reveals login on hover', async () => {
    const user = userEvent.setup()
    render(<UserDisplayName name="Alice Smith" login="alice" />)
    expect(screen.getByText('Alice Smith')).toBeInTheDocument()
    expect(screen.queryByText('alice')).not.toBeInTheDocument()

    await user.hover(screen.getByText('Alice Smith'))
    expect(await screen.findByText('alice')).toBeInTheDocument()
  })
})
