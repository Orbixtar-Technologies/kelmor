// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
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
})
