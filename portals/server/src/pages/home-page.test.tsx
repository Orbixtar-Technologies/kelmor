// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { HomePage } from './home-page'

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('HomePage', () => {
	it('shows measured vitals and P0 shortcuts without inventing job counts', async () => {
		vi.mocked(api).mockImplementation(async (path: string) => {
			if (path === '/api/v1/server') {
				return {
					system: {
						hostname: 'host.example',
						load1: 0.42,
						memory_used: 4,
						memory_total: 8,
						disk_used: 20,
						disk_total: 100,
						inodes_used: 1,
						inodes_total: 10,
						uptime_seconds: 7200,
					},
					stats: { accounts: 3, failedJobs: 2 },
					services: [
						{ name: 'nginx', health: 'ok', desired_enabled: true, observed_running: true },
						{ name: 'pdns', health: 'ok', desired_enabled: true, observed_running: false },
					],
				}
			}
			return { items: [] }
		})

		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.read': true, 'accounts.read': true, 'accounts.create': true, 'packages.read': true }}>
					<HomePage />
				</CapProvider>
			</MemoryRouter>,
		)

		await waitFor(() => {
			expect(screen.getByLabelText('Host vitals')).toHaveTextContent('0.42')
		})
		expect(screen.getByLabelText('Host vitals')).toHaveTextContent('2')
		const shortcuts = screen.getByLabelText('Operations shortcuts')
		expect(shortcuts.querySelector('a[href="/accounts/create"]')).toBeTruthy()
		expect(shortcuts.querySelector('a[href="/accounts"]')).toBeTruthy()
		expect(shortcuts.querySelector('a[href="/jobs?state=failed"]')).toBeTruthy()
		expect(shortcuts.querySelector('a[href="/status"]')).toBeTruthy()
		expect(shortcuts.textContent).not.toMatch(/Firewall|Reboot/)
		expect(screen.getByRole('heading', { name: 'Jobs & Audit' })).toBeInTheDocument()
		expect(screen.getByRole('heading', { name: 'Account Functions' })).toBeInTheDocument()
	})
})
