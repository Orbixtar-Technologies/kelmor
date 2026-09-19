// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { WebmailPage } from './webmail-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('WebmailPage', () => {
	test('launches HTTPS webmail in a new tab instead of a dead button', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server') {
				return Promise.resolve({ system: { hostname: 'kelmor.host' } })
			}
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.com' }] })
			}
			if (String(path).endsWith('/mail/mailboxes')) {
				return Promise.resolve({ items: [{ id: 'mb-1', local_part: 'info', domain_id: 'dom-1', status: 'active' }] })
			}
			if (String(path).endsWith('/admin-tools')) {
				return Promise.resolve({ webmail_url: 'http://webmail.orbixtar.com/' })
			}
			if (String(path).endsWith('/domains')) {
				return Promise.resolve({ items: [{ id: 'dom-1', ascii_fqdn: 'orbixtar.com' }] })
			}
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/webmail?account=acc-1']}>
				<CapProvider caps={{ 'mail.read': true }}>
					<Routes>
						<Route path="webmail" element={<WebmailPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		const launch = await screen.findByRole('link', { name: 'Open webmail' })
		expect(launch).toHaveAttribute('href', 'https://webmail.orbixtar.com/')
		expect(launch).toHaveAttribute('target', '_blank')
	})
})
