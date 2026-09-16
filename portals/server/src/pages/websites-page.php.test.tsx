// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { WebsitesPage } from './websites-page'

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('WebsitesPage MultiPHP', () => {
	it('persists a supported PHP version and rejects unsupported values', async () => {
		vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
			const url = String(path)
			if (url === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'alpha', primary_domain: 'a.test', home_path: '/home/alpha', status: 'active' }] })
			}
			if (url.endsWith('/domains')) return Promise.resolve({ items: [{ id: 'dom-1', ascii_fqdn: 'a.test' }] })
			if (url.endsWith('/websites') && (!init || !init.method || init.method === 'GET')) {
				return Promise.resolve({ items: [{ id: 'site-1', domain_id: 'dom-1', document_root: '/home/alpha/public_html', runtime: 'php', runtime_version: '8.3', enabled: true }] })
			}
			if (init?.method === 'POST') return Promise.resolve({ operation_id: 'job-1' })
			return Promise.resolve({ items: [] })
		})

		render(
			<MemoryRouter initialEntries={['/websites?account=acc-1']}>
				<CapProvider caps={{ 'websites.write': true, 'websites.read': true }}>
					<Routes>
						<Route path="/websites" element={<WebsitesPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		const version = await screen.findByLabelText(/PHP version for/)
		expect(screen.queryByRole('option', { name: '8.1' })).not.toBeInTheDocument()
		await user.selectOptions(version, '8.4')
		await waitFor(() => {
			expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/websites', expect.objectContaining({
				method: 'POST',
				body: expect.stringContaining('"runtime_version":"8.4"'),
			}))
		})

		const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value')?.set
		nativeSetter?.call(version, '8.1')
		version.dispatchEvent(new Event('change', { bubbles: true }))
		await screen.findByText(/unsupported PHP version/i)
		const posts = vi.mocked(api).mock.calls.filter((call) => String(call[1]?.method) === 'POST')
		expect(posts).toHaveLength(1)
	})
})
