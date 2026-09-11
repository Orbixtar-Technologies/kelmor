// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { EmailManagerPage } from './email-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderEmail (path = '/email?account=acc-1&tab=lists') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'mail.read': true, 'mail.write': true }}>
				<Routes>
					<Route path="email" element={<EmailManagerPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('EmailManagerPage lists', () => {
	test('creates a mailing list through the lists API', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path).endsWith('/mail/domains')) {
				return Promise.resolve({ items: [{ id: 'md-1', domain_id: 'dom-uuid', ascii_fqdn: 'shop.test' }] })
			}
			if (String(path).endsWith('/mail/lists') && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'job-1' })
			}
			if (String(path).includes('/mail/')) {
				return Promise.resolve({ items: [] })
			}
			if (String(path).endsWith('/domains')) {
				return Promise.resolve({ items: [{ id: 'dom-1', ascii_fqdn: 'shop.test' }] })
			}
			return Promise.resolve({ items: [] })
		})
		renderEmail()
		expect(await screen.findByRole('heading', { name: 'Create mailing list' })).toBeInTheDocument()
		expect(screen.getByRole('option', { name: 'shop.test' })).toBeInTheDocument()
		await user.type(screen.getByPlaceholderText('staff'), 'staff')
		await user.type(screen.getByPlaceholderText('owner@example.com, ops@example.com'), 'owner@shop.test, ops@shop.test')
		await user.click(screen.getByRole('button', { name: 'Create list' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/mail/lists', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({
				domain_id: 'md-1',
				local_part: 'staff',
				members: ['owner@shop.test', 'ops@shop.test'],
			}),
		}))
	})
})
