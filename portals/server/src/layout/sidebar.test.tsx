// @vitest-environment jsdom
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { toolCatalog } from '../tool-catalog'
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
	{ id: 'account-summary', label: 'Account Summary', description: 'Account hub', category: 'Account Information', path: '/accounts/summary', icon: 'account', capabilities: ['accounts.read'] },
	{ id: 'suspended', label: 'Suspended Accounts', description: 'Suspended', category: 'Account Information', path: '/accounts?view=suspended', icon: 'pause', capabilities: ['accounts.read'] },
	{ id: 'modify-account', label: 'Modify an Account', description: 'Modify', category: 'Account Functions', path: '/accounts/modify', icon: 'edit', capabilities: ['accounts.read'] },
	{ id: 'terminate-account', label: 'Terminate an Account', description: 'Terminate', category: 'Account Functions', path: '/accounts/terminate', icon: 'trash', capabilities: ['accounts.read'] },
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
	test('filters tools and supports per-category expand/collapse', async () => {
		const user = userEvent.setup()
		render(
			<MemoryRouter>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		await user.type(screen.getByRole('textbox', { name: 'Filter features' }), 'account')
		expect(screen.queryByRole('link', { name: 'Home' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: /Server Configuration/ })).not.toBeInTheDocument()
		const category = screen.getByRole('button', { name: /Account Information/ })
		expect(screen.getByRole('link', { name: /List Accounts/ })).toBeVisible()
		expect(screen.getByRole('button', { name: /Account Functions/ })).toBeVisible()

		await user.click(category)
		expect(category).toHaveAttribute('aria-expanded', 'false')
		expect(screen.queryByRole('link', { name: /List Accounts/ })).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Expand' }))
		expect(screen.getByRole('link', { name: /List Accounts/ })).toBeVisible()
		await user.click(screen.getByRole('button', { name: 'Collapse' }))
		expect(screen.queryByRole('link', { name: /List Accounts/ })).not.toBeInTheDocument()
	})

	test('lists every catalog tool from Home All tools', () => {
		render(
			<MemoryRouter>
				<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Home' })).toBeVisible()
		const expected = toolCatalog.filter((tool) => tool.id !== 'home')
		for (const tool of expected) {
			expect(screen.getByRole('link', { name: tool.label })).toBeInTheDocument()
		}
		expect(screen.getByRole('link', { name: 'Tweak Settings' })).toHaveAttribute('href', '/section/server?tool=tweak-settings')
		expect(screen.getByRole('link', { name: 'Change Hostname' })).toHaveAttribute('href', '/section/server?tool=change-hostname')
		expect(screen.getByRole('link', { name: 'List Accounts' })).toHaveAttribute('href', '/accounts')
		expect(screen.getByRole('link', { name: 'Change Ownership of an Account' })).toHaveAttribute('href', '/accounts/ownership')
		expect(screen.getByRole('link', { name: 'Mail Delivery Reports' })).toHaveAttribute('href', '/mail/delivery-reports')
		expect(screen.getByRole('link', { name: 'Track Delivery' })).toHaveAttribute('href', '/mail/track-delivery')
		expect(screen.getByRole('link', { name: 'Show IP Address Usage' })).toHaveAttribute('href', '/ip-usage')
		expect(screen.getByRole('link', { name: 'Perform a DNS Cleanup' })).toHaveAttribute('href', '/dns/cleanup')
		expect(screen.getByRole('link', { name: 'Synchronize DNS Records' })).toHaveAttribute('href', '/dns/synchronize')
		expect(screen.getByRole('button', { name: 'Account Functions' })).toBeVisible()
		expect(screen.getByRole('button', { name: 'Jobs & Audit' })).toBeVisible()
		expect(screen.getByRole('button', { name: 'Service / Server Status' })).toBeVisible()
		expect(screen.queryByRole('button', { name: 'cPanel' })).not.toBeInTheDocument()
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

	test('highlights only Mail Delivery Reports on its dedicated page', () => {
		render(
			<MemoryRouter initialEntries={['/mail/delivery-reports']}>
				<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Mail Delivery Reports' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Track Delivery' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Jobs' })).not.toHaveAttribute('aria-current')
	})

	test('highlights only DNS Cleanup on its dedicated page', () => {
		render(
			<MemoryRouter initialEntries={['/dns/cleanup']}>
				<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Perform a DNS Cleanup' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Synchronize DNS Records' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'DNS Zone Manager' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Mail Delivery Reports' })).not.toHaveAttribute('aria-current')
	})

	test('highlights only Track Delivery on its dedicated page', () => {
		render(
			<MemoryRouter initialEntries={['/mail/track-delivery']}>
				<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Track Delivery' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Mail Delivery Reports' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Jobs' })).not.toHaveAttribute('aria-current')
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

	test('keeps every WHM-style category expanded so Home is not the only visible menu', () => {
		render(
			<MemoryRouter initialEntries={['/']}>
				<Sidebar tools={tools} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByRole('link', { name: 'Home' })).toBeVisible()
		expect(screen.getByRole('button', { name: /Account Functions/ })).toHaveAttribute('aria-expanded', 'true')
		expect(screen.getByRole('button', { name: /Account Information/ })).toHaveAttribute('aria-expanded', 'true')
		expect(screen.getByRole('button', { name: /Server Configuration/ })).toHaveAttribute('aria-expanded', 'true')
		expect(screen.getByRole('button', { name: /Networking Setup/ })).toHaveAttribute('aria-expanded', 'true')
		expect(screen.getByRole('link', { name: /List Accounts/ })).toBeVisible()
		expect(screen.getByRole('link', { name: /Account Summary/ })).toBeVisible()
		expect(screen.getByRole('link', { name: /Modify an Account/ })).toBeVisible()
		expect(screen.getByRole('link', { name: 'Tweak Settings' })).toBeVisible()
		expect(screen.getByRole('link', { name: 'Change Hostname' })).toBeVisible()
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

	test('does not dump sibling hub tabs onto dedicated P0 pages', () => {
		function renderShell (path: string) {
			return render(
				<MemoryRouter initialEntries={[path]}>
					<CapProvider caps={{ 'accounts.read': true, 'server.read': true, 'packages.read': true }}>
						<Routes>
							<Route element={<DirectorShell me={me} onSignOut={vi.fn()} />}>
								<Route index element={<p>Home content</p>} />
								<Route path="accounts" element={<p>Account inventory</p>} />
								<Route path="accounts/create" element={<p>Create account wizard</p>} />
								<Route path="packages" element={<p>Package inventory</p>} />
								<Route path="jobs" element={<p>Job queue</p>} />
								<Route path="section/:hubId" element={<p>Catalog tool</p>} />
							</Route>
						</Routes>
					</CapProvider>
				</MemoryRouter>,
			)
		}

		renderShell('/')
		expect(screen.getByText('Home content')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		cleanup()

		renderShell('/accounts')
		expect(screen.getByText('Account inventory')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		cleanup()

		renderShell('/accounts/create')
		expect(screen.getByText('Create account wizard')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		cleanup()

		renderShell('/packages')
		expect(screen.getByText('Package inventory')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		cleanup()

		renderShell('/jobs')
		expect(screen.getByText('Job queue')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		cleanup()

		renderShell('/section/server')
		expect(screen.getByText('Catalog tool')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).not.toBeNull()
	})

	test('does not dump hub catalogs onto dedicated tool routes', () => {
		function renderShell (path: string) {
			return render(
				<MemoryRouter initialEntries={[path]}>
					<CapProvider caps={{ 'accounts.read': true, 'server.read': true, 'resellers.read': true }}>
						<Routes>
							<Route element={<DirectorShell me={me} onSignOut={vi.fn()} />}>
								<Route path="section/:hubId" element={<p>Selected tool workflow</p>} />
							</Route>
						</Routes>
					</CapProvider>
				</MemoryRouter>,
			)
		}

		renderShell('/section/accounts?tool=change-site-ip')
		expect(screen.getByText('Selected tool workflow')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		expect(screen.queryByRole('navigation', { name: /tools$/i })).not.toBeInTheDocument()
		cleanup()

		renderShell('/section/packages?tool=email-resellers')
		expect(screen.getByText('Selected tool workflow')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		expect(screen.queryByRole('navigation', { name: /tools$/i })).not.toBeInTheDocument()
		cleanup()

		renderShell('/section/server?tool=tweak-settings')
		expect(screen.getByText('Selected tool workflow')).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
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
