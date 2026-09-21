// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { DirectorShell } from '../layout/director-shell'
import { CapProvider } from '../rbac'
import type { Me } from '../types'
import { HubPage, HubTabs, ToolRedirect } from './hub-page'

const api = vi.fn().mockResolvedValue({ values: {}, items: [] })

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

const me: Me = {
	user: { username: 'operator', roles: [], email: 'operator@example.com' },
	actor: { capabilities: {} },
}

beforeEach(() => {
	vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
		matches: false,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
	}))
})

afterEach(() => {
	cleanup()
	api.mockClear()
	vi.unstubAllGlobals()
})

function renderPath (path: string) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'server.settings.write': true, 'server.read': true, 'accounts.read': true }}>
				<Routes>
					<Route path="section/:hubId" element={<HubPage />} />
					<Route path="tools/:toolId" element={<ToolRedirect />} />
					<Route path="accounts" element={<p>Account inventory</p>} />
					<Route path="dns/cleanup" element={<p>DNS cleanup inventory</p>} />
					<Route path="dns/synchronize" element={<p>DNS synchronize inventory</p>} />
					<Route path="mail/delivery-reports" element={<p>Mail Delivery Reports page</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('HubPage', () => {
	test('combines server configuration tools onto one page', async () => {
		renderPath('/section/server?tool=tweak-settings')
		expect(await screen.findByRole('heading', { name: 'Tweak Settings' })).toBeInTheDocument()
		expect(screen.getByText(/Server Configuration/)).toBeInTheDocument()
	})

	test('redirects dedicated hub tools to their manager', () => {
		renderPath('/section/accounts?tool=list-accounts')
		expect(screen.getByText('Account inventory')).toBeInTheDocument()
	})

	test('redirects DNS cleanup and synchronize onto dedicated inventory pages', () => {
		renderPath('/section/dns?tool=dns-cleanup')
		expect(screen.getByText('DNS cleanup inventory')).toBeInTheDocument()
		cleanup()
		renderPath('/tools/synchronize-dns')
		expect(screen.getByText('DNS synchronize inventory')).toBeInTheDocument()
		cleanup()
		renderPath('/section/email?tool=mail-delivery-reports')
		expect(screen.getByText('Mail Delivery Reports page')).toBeInTheDocument()
	})

	test('sends legacy /tools/:id links to the combined hub', async () => {
		renderPath('/tools/change-hostname')
		expect(await screen.findByRole('heading', { name: 'Change Hostname' })).toBeInTheDocument()
	})

	test('Security and Websites and Files deep links are catalog members', async () => {
		renderPath('/section/security?tool=firewall')
		expect(screen.queryByText(/This tool is not part of Security/)).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Firewall' })).toBeInTheDocument()
		cleanup()
		renderPath('/section/security?tool=csf')
		expect(screen.queryByText(/This tool is not part of Security/)).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Firewall' })).toBeInTheDocument()
		cleanup()
		renderPath('/section/websites?tool=hotlink')
		expect(screen.queryByText(/This tool is not part of Websites/)).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: /Hotlink/ })).toBeInTheDocument()
		cleanup()
		renderPath('/section/files?tool=image-manager')
		expect(screen.queryByText(/This tool is not part of Files/)).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: /Image Manager/ })).toBeInTheDocument()
	})

	test('redirects dedicated Redirects and Git tools off the hub stub', () => {
		render(
			<MemoryRouter initialEntries={['/section/websites?tool=redirects']}>
				<CapProvider caps={{ 'websites.read': true, 'accounts.read': true }}>
					<Routes>
						<Route path="section/:hubId" element={<HubPage />} />
						<Route path="redirects" element={<p>Redirects manager</p>} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(screen.getByText('Redirects manager')).toBeInTheDocument()
		cleanup()
		render(
			<MemoryRouter initialEntries={['/section/files?tool=git']}>
				<CapProvider caps={{ 'files.read': true, 'accounts.read': true }}>
					<Routes>
						<Route path="section/:hubId" element={<HubPage />} />
						<Route path="git" element={<p>Git Version Control</p>} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(screen.getByText('Git Version Control')).toBeInTheDocument()
	})

	test('selected section tools render the journey without hub catalogs', async () => {
		function renderTool (path: string) {
			return render(
				<MemoryRouter initialEntries={[path]}>
					<CapProvider caps={{ 'accounts.modify': true, 'accounts.read': true, 'resellers.read': true, 'server.settings.write': true }}>
						<Routes>
							<Route element={<DirectorShell me={me} onSignOut={() => undefined} />}>
								<Route path="section/:hubId" element={<HubPage />} />
							</Route>
						</Routes>
					</CapProvider>
				</MemoryRouter>,
			)
		}

		renderTool('/section/accounts?tool=change-site-ip')
		expect(await screen.findByRole('heading', { name: "Change Site's IP Address" })).toBeInTheDocument()
		expect(screen.getByRole('list', { name: 'Workflow' })).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		expect(screen.queryByRole('heading', { name: 'Account Functions' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Account Information' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Multi Account Functions' })).not.toBeInTheDocument()
		cleanup()

		renderTool('/section/packages?tool=email-resellers')
		expect(await screen.findByRole('heading', { name: 'Email All Resellers' })).toBeInTheDocument()
		expect(screen.getByRole('list', { name: 'Workflow' })).toBeInTheDocument()
		expect(document.querySelector('.hub-chrome')).toBeNull()
		expect(screen.queryByRole('heading', { name: 'Packages' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Resellers' })).not.toBeInTheDocument()
	})
})

describe('HubTabs', () => {
	test('lists every server configuration menu on one tab strip', () => {
		render(
			<MemoryRouter initialEntries={['/section/server?tool=tweak-settings']}>
				<HubTabs hubId="server" pathname="/section/server" search="?tool=tweak-settings" />
			</MemoryRouter>,
		)
		expect(screen.getByRole('link', { name: 'Tweak Settings' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Basic WebHost Manager Setup' })).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Change Hostname' })).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Contact Manager' })).toBeInTheDocument()
		expect(screen.getByRole('navigation', { name: 'Server Configuration tools' })).toBeInTheDocument()
	})
})
