// @vitest-environment jsdom
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { DirectorShell } from './director-shell'
import { Sidebar } from './sidebar'
import type { Me, ToolDefinition } from '../types'

vi.mock('../client', () => ({
	api: vi.fn().mockResolvedValue({ items: [] }),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

const tools: ToolDefinition[] = [
	{ id: 'home', label: 'Home', description: 'Overview', category: 'Kelmor Director', path: '/', icon: 'home', capabilities: ['server.read'] },
	{ id: 'accounts', label: 'List Accounts', description: 'Accounts', category: 'Account Information', path: '/accounts', icon: 'users', capabilities: ['accounts.read'] },
	{ id: 'account-summary', label: 'Account Summary', description: 'Account hub', category: 'Account Information', path: '/accounts?task=summary', icon: 'account', capabilities: ['accounts.read'] },
	{ id: 'suspended', label: 'Suspended Accounts', description: 'Suspended', category: 'Account Information', path: '/accounts?view=suspended', icon: 'pause', capabilities: ['accounts.read'] },
	{ id: 'modify-account', label: 'Modify an Account', description: 'Modify', category: 'Account Functions', path: '/accounts?task=modify', icon: 'edit', capabilities: ['accounts.read'] },
	{ id: 'terminate-account', label: 'Terminate an Account', description: 'Terminate', category: 'Account Functions', path: '/accounts?task=terminate', icon: 'trash', capabilities: ['accounts.read'] },
]

const me: Me = {
	user: { username: 'operator', roles: [], email: 'operator@example.com' },
	actor: { capabilities: {} },
}

beforeEach(() => {
	vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
		matches: true,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
	}))
})

afterEach(() => {
	cleanup()
	vi.unstubAllGlobals()
})

describe('Sidebar interactions', () => {
	test('filters categories and supports per-category and global expand/collapse', async () => {
		const user = userEvent.setup()
		render(
			<MemoryRouter>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		await user.type(screen.getByRole('textbox', { name: 'Filter features' }), 'account')
		expect(screen.queryByRole('button', { name: /Kelmor Director/ })).not.toBeInTheDocument()
		const category = screen.getByRole('button', { name: /Account Information/ })
		expect(screen.getByRole('link', { name: /List Accounts/ })).toBeVisible()

		await user.click(category)
		expect(category).toHaveAttribute('aria-expanded', 'false')
		expect(screen.queryByRole('link', { name: /List Accounts/ })).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Expand' }))
		expect(screen.getByRole('link', { name: /List Accounts/ })).toBeVisible()
		await user.click(screen.getByRole('button', { name: 'Collapse' }))
		expect(screen.queryByRole('link', { name: /List Accounts/ })).not.toBeInTheDocument()
	})

	test('focuses the mobile sidebar and restores menu focus after Escape', async () => {
		const user = userEvent.setup()
		render(
			<MemoryRouter>
				<CapProvider caps={{}}>
					<Routes>
						<Route element={<DirectorShell me={me} onSignOut={vi.fn()} />}>
							<Route index element={<p>Home content</p>} />
						</Route>
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const menuButton = screen.getByRole('button', { name: 'Open navigation' })
		const sidebar = screen.getByRole('complementary', { hidden: true })
		expect(sidebar).toHaveAttribute('aria-hidden', 'true')
		expect(sidebar).toHaveAttribute('inert')

		await user.click(menuButton)
		expect(screen.getByRole('textbox', { name: 'Filter features' })).toHaveFocus()
		await user.keyboard('{Escape}')

		expect(menuButton).toHaveFocus()
		expect(menuButton).toHaveAttribute('aria-expanded', 'false')
		expect(within(sidebar).queryByRole('textbox')).not.toBeInTheDocument()
		expect(sidebar).toHaveAttribute('aria-hidden', 'true')
		expect(sidebar).toHaveAttribute('inert')
	})

	test('marks only the current destination active on an account hub', () => {
		render(
			<MemoryRouter initialEntries={['/accounts/acc-1']}>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: /Account Summary/ })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: /List Accounts/ })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: /Modify an Account/ })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: /Terminate an Account/ })).not.toHaveAttribute('aria-current')
	})
})
