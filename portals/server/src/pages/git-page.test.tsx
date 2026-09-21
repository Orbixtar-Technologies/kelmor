// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { GitPage } from './git-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('GitPage', () => {
	test('lists discovered repositories and stays empty when none exist', async () => {
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test', home_path: '/home/shop' }] })
			}
			if (String(path).endsWith('/git')) {
				return Promise.resolve({ items: [], home: '/home/shop' })
			}
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/git?account=acc-1']}>
				<CapProvider caps={{ 'accounts.read': true, 'files.read': true }}>
					<Routes>
						<Route path="git" element={<GitPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('heading', { name: 'Git Version Control · shop' })).toBeInTheDocument()
		expect(await screen.findByText('No Git repositories')).toBeInTheDocument()
		expect(screen.getByText('/home/shop')).toBeInTheDocument()
	})
})
