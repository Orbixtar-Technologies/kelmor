// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { IPUsagePage } from './ip-usage-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('IPUsagePage', () => {
	test('lists dedicated account addresses instead of the account inventory hub', async () => {
		api.mockResolvedValue({
			items: [
				{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test', status: 'active', ip_address: '203.0.113.10' },
				{ id: 'acc-2', username: 'blog', primary_domain: 'blog.test', status: 'active' },
			],
		})
		render(
			<MemoryRouter initialEntries={['/ip-usage']}>
				<CapProvider caps={{ 'accounts.read': true }}>
					<Routes>
						<Route path="ip-usage" element={<IPUsagePage />} />
						<Route path="accounts" element={<p>Account list hub</p>} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('heading', { name: 'Show IP Address Usage' })).toBeInTheDocument()
		expect(screen.getByText('203.0.113.10')).toBeInTheDocument()
		expect(screen.getByText('IPv4')).toBeInTheDocument()
		expect(screen.queryByText('Account list hub')).not.toBeInTheDocument()
	})
})
