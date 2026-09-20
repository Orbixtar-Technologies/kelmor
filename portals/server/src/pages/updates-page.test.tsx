import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { UpdatesPage } from './updates-page'
import * as client from '../client'
import * as rbac from '../rbac'

describe('UpdatesPage', () => {
	afterEach(cleanup)
	beforeEach(() => {
		vi.spyOn(rbac, 'useCan').mockImplementation((capability) => capability === 'server.settings.write')
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'available',
					installed_release: '0.1.0',
					available_release: '0.2.0',
					automatic: true,
					channel: 'stable',
				}
			}
			return { ok: true }
		})
	})

	it('renders installed and available releases', async () => {
		render(<MemoryRouter><UpdatesPage /></MemoryRouter>)
		expect((await screen.findAllByText('0.1.0')).length).toBeGreaterThan(0)
		expect(screen.getByText('0.2.0')).toBeInTheDocument()
		expect(screen.getByText(/get-kelmor\.sh/)).toBeInTheDocument()
	})

	it('requires confirmation before install', async () => {
		const user = userEvent.setup()
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
		render(<MemoryRouter><UpdatesPage /></MemoryRouter>)
		await screen.findByText('0.2.0')
		await user.click(screen.getByRole('button', { name: 'Install verified release' }))
		expect(confirmSpy).toHaveBeenCalled()
		expect(client.api).not.toHaveBeenCalledWith('/api/v1/server/updates/install', expect.anything())
	})

	it('shows last checked and up-to-date feedback after Check now', async () => {
		const user = userEvent.setup()
		let lastChecked = ''
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'idle',
					installed_release: '0.2.413',
					automatic: false,
					channel: 'stable',
					last_checked_at: lastChecked || undefined,
				}
			}
			if (path === '/api/v1/server/updates/check') {
				lastChecked = '2026-09-20T13:04:00Z'
				return {
					state: 'idle',
					installed_release: '0.2.413',
					automatic: false,
					channel: 'stable',
					last_checked_at: lastChecked,
				}
			}
			return { ok: true }
		})
		render(<MemoryRouter><UpdatesPage /></MemoryRouter>)
		expect((await screen.findAllByText('0.2.413')).length).toBeGreaterThan(0)
		expect(screen.getByText('Last checked').closest('div')?.querySelector('dd')?.textContent).toBe('—')
		await user.click(screen.getByRole('button', { name: 'Check now' }))
		expect(await screen.findByText("Checked — you're up to date")).toBeInTheDocument()
		expect(screen.getByText(/local time/)).toBeInTheDocument()
		expect(screen.queryByText('Update request accepted.')).not.toBeInTheDocument()
	})

	it('shows update-available feedback after Check now', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'available',
					installed_release: '0.2.413',
					available_release: '0.2.500',
					automatic: false,
					channel: 'stable',
				}
			}
			if (path === '/api/v1/server/updates/check') {
				return {
					state: 'available',
					installed_release: '0.2.413',
					available_release: '0.2.500',
					automatic: false,
					channel: 'stable',
					last_checked_at: '2026-09-20T13:04:00Z',
				}
			}
			return { ok: true }
		})
		render(<MemoryRouter><UpdatesPage /></MemoryRouter>)
		await screen.findByText('0.2.500')
		await user.click(screen.getByRole('button', { name: 'Check now' }))
		expect(await screen.findByText('Checked — update available')).toBeInTheDocument()
	})

	it('keeps the previous last checked when Check now fails', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'idle',
					installed_release: '0.2.413',
					automatic: false,
					channel: 'stable',
					last_checked_at: '2026-09-20T12:00:00Z',
				}
			}
			if (path === '/api/v1/server/updates/check') {
				throw new Error('Could not fetch the signed update feed')
			}
			return { ok: true }
		})
		render(<MemoryRouter><UpdatesPage /></MemoryRouter>)
		expect(await screen.findByText(/local time/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Check now' }))
		expect(await screen.findByText('Could not fetch the signed update feed')).toBeInTheDocument()
		expect(screen.getByText(/local time/)).toBeInTheDocument()
		expect(screen.queryByText("Checked — you're up to date")).not.toBeInTheDocument()
	})
})
