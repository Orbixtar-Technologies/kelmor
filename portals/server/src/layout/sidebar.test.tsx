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
	api: vi.fn().mockImplementation((path: string) => {
		if (String(path).startsWith('/api/v1/server')) {
			return Promise.resolve({
				system: {
					hostname: 'host.example',
					load1: 0.4,
					memory_used: 4,
					memory_total: 8,
					disk_used: 20,
					disk_total: 100,
					inodes_used: 1,
					inodes_total: 10,
					uptime_seconds: 7200,
				},
				stats: { accounts: 3, failedJobs: 0 },
				services: [{ name: 'nginx', health: 'ok', desired_enabled: true, observed_running: true }],
			})
		}
		return Promise.resolve({ items: [] })
	}),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

const tools: ToolDefinition[] = [
	{ id: 'home', label: 'Home', description: 'Overview', category: 'Kelmor Director', path: '/', icon: 'home', capabilities: ['server.read'] },
	{ id: 'list-accounts', label: 'List Accounts', description: 'Accounts', category: 'Account Information', path: '/accounts', icon: 'users', capabilities: ['accounts.read'] },
	{ id: 'account-summary', label: 'Account Summary', description: 'Account hub', category: 'Account Information', path: '/accounts?task=summary', icon: 'account', capabilities: ['accounts.read'] },
	{ id: 'suspended', label: 'Suspended Accounts', description: 'Suspended', category: 'Account Information', path: '/accounts?view=suspended', icon: 'pause', capabilities: ['accounts.read'] },
	{ id: 'modify-account', label: 'Modify an Account', description: 'Modify', category: 'Account Functions', path: '/accounts?task=modify', icon: 'edit', capabilities: ['accounts.read'] },
	{ id: 'terminate-account', label: 'Terminate an Account', description: 'Terminate', category: 'Account Functions', path: '/accounts?task=terminate', icon: 'trash', capabilities: ['accounts.read'] },
	{ id: 'tweak-settings', label: 'Tweak Settings', description: 'Host defaults', category: 'Server Configuration', path: '/tools/tweak-settings', icon: 'edit', capabilities: ['server.settings.write'] },
	{ id: 'change-hostname', label: 'Change Hostname', description: 'Hostname', category: 'Networking Setup', path: '/tools/change-hostname', icon: 'globe', capabilities: ['server.settings.write'] },
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
	test('filters hubs and supports group expand/collapse', async () => {
		const user = userEvent.setup()
		render(
			<MemoryRouter>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		await user.type(screen.getByRole('textbox', { name: 'Filter features' }), 'account')
		expect(screen.queryByRole('link', { name: 'Home' })).not.toBeInTheDocument()
		expect(screen.queryByRole('link', { name: 'Server Configuration' })).not.toBeInTheDocument()
		const category = screen.getByRole('button', { name: /Account services/ })
		expect(screen.getByRole('link', { name: 'Accounts' })).toBeVisible()
		expect(screen.queryByRole('link', { name: /List Accounts/ })).not.toBeInTheDocument()

		await user.click(category)
		expect(category).toHaveAttribute('aria-expanded', 'false')
		expect(screen.queryByRole('link', { name: 'Accounts' })).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Expand' }))
		expect(screen.getByRole('link', { name: 'Accounts' })).toBeVisible()
		await user.click(screen.getByRole('button', { name: 'Collapse' }))
		expect(screen.queryByRole('link', { name: 'Accounts' })).not.toBeInTheDocument()
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

	test('marks the Accounts hub current on an account detail page', () => {
		render(
			<MemoryRouter initialEntries={['/accounts/acc-1']}>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Accounts' })).toHaveAttribute('aria-current', 'page')
		expect(screen.queryByRole('link', { name: /Account Summary/ })).not.toBeInTheDocument()
		expect(screen.queryByRole('link', { name: /Modify an Account/ })).not.toBeInTheDocument()
	})

	test('keeps hub groups other than the current section collapsed by default', () => {
		render(
			<MemoryRouter initialEntries={['/accounts/acc-1']}>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('button', { name: /Account services/ })).toHaveAttribute('aria-expanded', 'true')
		expect(screen.getByRole('button', { name: /Host operations/ })).toHaveAttribute('aria-expanded', 'false')
		expect(screen.queryByRole('link', { name: 'Server Configuration' })).not.toBeInTheDocument()
	})

	test('combines configuration features under one Server Configuration link', async () => {
		const user = userEvent.setup()
		render(
			<MemoryRouter initialEntries={['/section/server?tool=tweak-settings']}>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		await user.click(screen.getByRole('button', { name: 'Expand' }))
		expect(screen.getByRole('link', { name: 'Server Configuration' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Server Configuration' })).toHaveAttribute('href', '/section/server')
		expect(screen.queryByRole('link', { name: 'Tweak Settings' })).not.toBeInTheDocument()
		expect(screen.queryByRole('link', { name: 'Change Hostname' })).not.toBeInTheDocument()
	})

	test('does not render Account or Host badges on sidebar categories', () => {
		render(
			<MemoryRouter>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.queryByText('Account')).not.toBeInTheDocument()
		expect(screen.queryByText('Host')).not.toBeInTheDocument()
	})

	test('shows the host resources bar on Home only', async () => {
		function renderShell (path: string) {
			return render(
				<MemoryRouter initialEntries={[path]}>
					<CapProvider caps={{ 'server.read': true }}>
						<Routes>
							<Route element={<DirectorShell me={me} onSignOut={vi.fn()} />}>
								<Route index element={<p>Home content</p>} />
								<Route path="files" element={<p>Files content</p>} />
							</Route>
						</Routes>
					</CapProvider>
				</MemoryRouter>,
			)
		}

		renderShell('/')
		expect(await screen.findByRole('complementary', { name: 'Host resources' })).toBeInTheDocument()
		cleanup()

		renderShell('/files')
		expect(screen.getByText('Files content')).toBeInTheDocument()
		expect(screen.queryByRole('complementary', { name: 'Host resources' })).not.toBeInTheDocument()
	})
})
