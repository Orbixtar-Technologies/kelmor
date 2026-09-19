// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { Sidebar } from '../layout/sidebar'
import { toolCatalog } from '../tool-catalog'
import { AccountFunctionPage } from './account-function-page'
import { AccountsPage } from './accounts-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

const shop = {
	id: 'acc-1',
	username: 'shop',
	primary_domain: 'shop.test',
	status: 'active',
	reseller_id: '',
	owner_user_id: 'u1',
	linux_uid: 1001,
	linux_gid: 1001,
	package_id: 'pkg-1',
	home_path: '/home/shop',
	shell_class: 'nologin',
	login_disabled: false,
	desired_revision: 1,
	observed_revision: 1,
}

const acme = { id: 'res-1', user_id: 'u2', name: 'Acme Hosting', privilege_mask: [], nameservers: [], status: 'active' }

function mockApis () {
	api.mockImplementation((path: string, options?: { method?: string; body?: string }) => {
		if (String(path) === '/api/v1/accounts' && !options?.method) return Promise.resolve({ items: [shop] })
		if (String(path) === '/api/v1/resellers') return Promise.resolve({ items: [acme] })
		if (String(path) === '/api/v1/packages') return Promise.resolve({ items: [] })
		if (String(path) === '/api/v1/accounts/acc-1' && options?.method === 'PATCH') {
			return Promise.resolve({ operation_id: 'job-ownership-1' })
		}
		if (String(path) === '/api/v1/accounts/acc-1') return Promise.resolve(shop)
		return Promise.resolve({ items: [] })
	})
}

function renderOwnership (path = '/accounts/ownership') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'accounts.read': true, 'accounts.modify': true, 'resellers.read': true }}>
				<Routes>
					<Route path="accounts" element={<AccountsPage />} />
					<Route path="accounts/ownership" element={<AccountFunctionPage toolId="change-ownership" />} />
					<Route path="accounts/:id" element={<p>Account summary hub</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('Change Ownership dedicated page', () => {
	test('sidebar opens the ownership wizard without List Accounts Continue', async () => {
		mockApis()
		render(
			<MemoryRouter initialEntries={['/accounts/ownership']}>
				<CapProvider caps={{ 'accounts.read': true, 'accounts.modify': true }}>
					<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
					<AccountFunctionPage toolId="change-ownership" />
				</CapProvider>
			</MemoryRouter>,
		)

		const navLink = screen.getByRole('link', { name: 'Change Ownership of an Account' })
		expect(navLink).toHaveAttribute('href', '/accounts/ownership')
		expect(navLink).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'List Accounts' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Modify an Account' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('heading', { name: 'Change Ownership of an Account' })).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'List Accounts' })).not.toBeInTheDocument()
		expect(screen.queryByRole('link', { name: 'Continue' })).not.toBeInTheDocument()
		expect(await screen.findByLabelText('Search accounts')).toBeInTheDocument()
		expect(screen.getByLabelText('Account')).toBeInTheDocument()
	})

	test('moves the selected account to a reseller from the ownership page', async () => {
		const user = userEvent.setup()
		mockApis()
		renderOwnership()
		expect(await screen.findByRole('option', { name: /shop · shop.test/ })).toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		expect(await screen.findByText(/Current owner/)).toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('New owner'), 'res-1')
		await user.click(screen.getByRole('button', { name: 'Change ownership' }))
		await waitFor(() => {
			expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1', {
				method: 'PATCH',
				body: JSON.stringify({ reseller_id: 'res-1' }),
			})
		})
		expect(await screen.findByRole('status')).toHaveTextContent('job-ownership-1')
		expect(screen.queryByText('Account summary hub')).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'List Accounts' })).not.toBeInTheDocument()
	})
})
