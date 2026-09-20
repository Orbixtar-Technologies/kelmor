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
			if (path === '/api/v1/jobs?state=failed') {
				return {
					items: [{
						id: 'job-fail',
						type: 'certificate.provision',
						payload: { hostname: 'mail.shop.test' },
						state: 'failed',
						last_error: 'account missing',
					}],
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
		const failed = screen.getByLabelText('Failed job details')
		expect(failed).toHaveTextContent('2 failed jobs')
		expect(failed).toHaveTextContent('account missing')
		expect(failed).toHaveTextContent('does not mass-retry')
		expect(failed.querySelector('a[href="/jobs?state=failed"]')).toBeTruthy()
		expect(failed.querySelector('button')).toBeNull()
		expect(screen.getByRole('heading', { name: 'Frequent tools' })).toBeInTheDocument()
	})

	it('renders saved favorite tools instead of the default frequent set', async () => {
		vi.mocked(api).mockImplementation(async (path: string) => {
			if (path === '/api/v1/server/settings') {
				return { values: { theme: { density: 'compact', favorites: 'jobs,services' } } }
			}
			if (path === '/api/v1/server') {
				return {
					system: {
						hostname: 'host.example',
						load1: 0.1,
						memory_used: 1,
						memory_total: 2,
						disk_used: 1,
						disk_total: 2,
						inodes_used: 1,
						inodes_total: 2,
						uptime_seconds: 10,
					},
					stats: { accounts: 0, failedJobs: 0 },
					services: [{ name: 'nginx', health: 'ok', desired_enabled: true, observed_running: true }],
				}
			}
			return { items: [] }
		})

		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.read': true, 'accounts.read': true }}>
					<HomePage />
				</CapProvider>
			</MemoryRouter>,
		)

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Favorite tools' })).toBeInTheDocument()
		})
		const favorites = screen.getByLabelText('Favorite tools')
		expect(favorites.querySelector('a[href="/jobs"]')).toBeTruthy()
		expect(favorites.querySelector('a[href="/status"]')).toBeTruthy()
		expect(favorites.querySelector('a[href="/accounts"]')).toBeNull()
	})
})
