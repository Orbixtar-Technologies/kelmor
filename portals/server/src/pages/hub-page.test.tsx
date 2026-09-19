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

	test('sends legacy /tools/:id links to the combined hub', async () => {
		renderPath('/tools/change-hostname')
		expect(await screen.findByRole('heading', { name: 'Change Hostname' })).toBeInTheDocument()
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
