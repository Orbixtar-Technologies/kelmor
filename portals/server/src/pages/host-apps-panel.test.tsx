// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { HostAppsPanel } from './host-apps-panel'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('HostAppsPanel', () => {
	test('publishes HTTPS phpMyAdmin and webmail launches for the selected account', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/apps') {
				return Promise.resolve({
					items: [
						{ id: 'phpmyadmin', label: 'phpMyAdmin', kind: 'sql', status: 'installed', description: 'SQL browser' },
						{ id: 'roundcube', label: 'Roundcube', kind: 'mail', status: 'installed', description: 'Webmail' },
					],
				})
			}
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.com' }] })
			}
			if (String(path).endsWith('/admin-tools')) {
				return Promise.resolve({
					phpmyadmin_url: 'http://phpmyadmin.orbixtar.com/',
					webmail_url: 'http://webmail.orbixtar.com/',
				})
			}
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.settings.write': true }}>
					<HostAppsPanel kind="market" />
				</CapProvider>
			</MemoryRouter>,
		)
		const phpmyadmin = await screen.findByRole('link', { name: 'Open phpMyAdmin' })
		const webmail = screen.getByRole('link', { name: 'Open webmail' })
		expect(phpmyadmin).toHaveAttribute('href', 'https://phpmyadmin.orbixtar.com/')
		expect(phpmyadmin).toHaveAttribute('target', '_blank')
		expect(webmail).toHaveAttribute('href', 'https://webmail.orbixtar.com/')
		expect(webmail).toHaveAttribute('target', '_blank')
	})

	test('shows host-backed actions for installed rspamd instead of a blank cell', async () => {
		api.mockImplementation((path: string, init?: { method?: string; body?: string }) => {
			if (String(path) === '/api/v1/server/apps') {
				return Promise.resolve({
					items: [{
						id: 'rspamd',
						label: 'rspamd',
						kind: 'plugin',
						status: 'installed',
						description: 'Mail filter already wired as the Postfix milter.',
						actions: [
							{ id: 'disable', label: 'Disable', kind: 'job', available: true },
							{ id: 'restart', label: 'Restart', kind: 'job', available: true },
							{ id: 'review', label: 'Review config', kind: 'href', available: true, href: '/section/server?tool=exim-config' },
							{ id: 'status', label: 'View status', kind: 'href', available: true, href: '/status/services' },
						],
					}],
				})
			}
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.com' }] })
			}
			if (String(path) === '/api/v1/server/apps/rspamd/actions') {
				expect(init?.method).toBe('POST')
				expect(JSON.parse(String(init?.body || '{}'))).toEqual({ action: 'restart' })
				return Promise.resolve({ operation_id: 'job-rspamd-1', status: 'queued' })
			}
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.settings.write': true, 'server.services.restart': true }}>
					<HostAppsPanel kind="plugin" />
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByText('rspamd')).toBeTruthy()
		expect(screen.getByRole('link', { name: 'Review config' })).toHaveAttribute('href', '/section/server?tool=exim-config')
		expect(screen.getByRole('link', { name: 'View status' })).toHaveAttribute('href', '/status/services')
		await userEvent.click(screen.getByRole('button', { name: 'Restart' }))
		expect(await screen.findByText(/Job job-rspamd-1/)).toBeTruthy()
		expect(api).toHaveBeenCalledWith('/api/v1/server/apps/rspamd/actions', expect.objectContaining({ method: 'POST' }))
	})

	test('shows an honest disabled reason when an installed plugin has no safe action', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/apps') {
				return Promise.resolve({
					items: [{
						id: 'rspamd',
						label: 'rspamd',
						kind: 'plugin',
						status: 'installed',
						description: 'Mail filter already wired as the Postfix milter.',
						actions: [{
							id: 'restart',
							label: 'Restart',
							kind: 'job',
							available: false,
							reason: 'Requires server.services.restart',
						}],
					}],
				})
			}
			if (String(path) === '/api/v1/accounts') return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.read': true }}>
					<HostAppsPanel kind="plugin" />
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByText('Requires server.services.restart')).toBeTruthy()
		expect(screen.queryByRole('button', { name: 'Restart' })).toBeNull()
	})
})
