// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { SQLManagerPage } from './sql-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('SQLManagerPage', () => {
	test('explains the one-user-per-engine model and surfaces the create job', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path).endsWith('/databases') && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'db-job' })
			}
			if (String(path).endsWith('/databases')) return Promise.resolve({ items: [] })
			if (String(path).includes('/credentials')) return Promise.resolve({ credentials: { host: '127.0.0.1', username: 'shop_u', password: 'secret' } })
			if (String(path).includes('/admin-tools')) return Promise.resolve({ phpmyadmin_url: 'https://pma.shop.test/' })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/sql?account=acc-1']}>
				<CapProvider caps={{ 'databases.read': true, 'databases.write': true }}>
					<Routes>
						<Route path="sql" element={<SQLManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByText(/one database user per engine/i)).toBeInTheDocument()
		const phpmyadmin = screen.getByRole('link', { name: 'Open phpMyAdmin' })
		expect(phpmyadmin).toHaveAttribute('href', 'https://pma.shop.test/')
		expect(phpmyadmin).toHaveAttribute('target', '_blank')
		await user.type(screen.getByPlaceholderText('app_db'), 'app_db')
		await user.click(screen.getByRole('button', { name: 'Create database' }))
		expect(await screen.findByRole('link', { name: 'Open Jobs' })).toHaveAttribute('href', '/jobs?account=acc-1&selected=db-job')
	})
})
