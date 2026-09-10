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
		expect(await screen.findByText('0.1.0')).toBeInTheDocument()
		expect(screen.getByText('0.2.0')).toBeInTheDocument()
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
})
