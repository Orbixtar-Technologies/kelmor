// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { RedirectsPage } from './redirects-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderRedirects (path = '/redirects?account=acc-1') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'accounts.read': true, 'websites.read': true, 'websites.write': true }}>
				<Routes>
					<Route path="redirects" element={<RedirectsPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('RedirectsPage', () => {
	test('shows an honest empty list and creates a host-backed redirect', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test', home_path: '/home/shop' }] })
			}
			if (String(path).endsWith('/websites')) {
				return Promise.resolve({ items: [{ id: 'web-1', domain_id: 'dom-1', document_root: '/home/shop/public_html' }] })
			}
			if (String(path).endsWith('/domains')) {
				return Promise.resolve({ items: [{ id: 'dom-1', ascii_fqdn: 'shop.test' }] })
			}
			if (String(path).endsWith('/redirects') && options?.method === 'POST') {
				return Promise.resolve({ id: 'redir-1', source: '/old', target: 'https://shop.test/new', status: 301 })
			}
			if (String(path).endsWith('/redirects')) {
				return Promise.resolve({ items: [] })
			}
			return Promise.resolve({ items: [] })
		})
		renderRedirects()
		expect(await screen.findByRole('heading', { name: 'Redirects · shop' })).toBeInTheDocument()
		expect(await screen.findByText('No redirects yet')).toBeInTheDocument()
		await user.type(screen.getByLabelText('Source path'), '/old')
		await user.type(screen.getByLabelText('Target URL'), 'https://shop.test/new')
		await user.click(screen.getByRole('button', { name: 'Add redirect' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/redirects', expect.objectContaining({ method: 'POST' }))
	})
})
