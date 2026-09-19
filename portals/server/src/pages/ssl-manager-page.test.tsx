// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { SSLManagerPage } from './ssl-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('SSLManagerPage', () => {
	test('labels custom PEM as a stub on the request journey', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path).includes('/certificates')) return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/ssl/request?account=acc-1']}>
				<CapProvider caps={{ 'websites.read': true, 'websites.write': true }}>
					<Routes>
						<Route path="ssl" element={<SSLManagerPage />} />
						<Route path="ssl/request" element={<SSLManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByText(/Custom PEM — labeled stub/)).toBeInTheDocument()
		expect(screen.getByText(/no certificate upload API/i)).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Request AutoSSL' })).toBeInTheDocument()
	})
})
