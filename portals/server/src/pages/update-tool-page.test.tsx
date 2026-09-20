// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { DirectorShell } from '../layout/director-shell'
import { Sidebar } from '../layout/sidebar'
import { toolCatalog } from '../tool-catalog'
import type { Me } from '../types'
import { UpdatesPage } from './updates-page'
import { CHANGELOG_EMPTY_DETAIL, CHANGELOG_EMPTY_TITLE, SOFTWARE_UPDATES_CTA, UpdateToolPage } from './update-tool-page'
import * as client from '../client'
import * as rbac from '../rbac'

const me: Me = {
	user: { username: 'operator', roles: [], email: 'operator@example.com' },
	actor: { capabilities: {} },
}

afterEach(() => {
	cleanup()
	vi.unstubAllGlobals()
})

beforeEach(() => {
	vi.spyOn(rbac, 'useCan').mockImplementation((capability) => capability === 'server.settings.write')
	vi.spyOn(client, 'api').mockImplementation(async (path) => {
		if (path === '/api/v1/server/updates') {
			return {
				state: 'available',
				installed_release: '0.1.0',
				available_release: '0.2.0',
				automatic: true,
				channel: 'stable',
			}
		}
		if (path === '/api/v1/server/updates/changelog') {
			return {
				installed_release: '0.1.0',
				available_release: '0.2.0',
				channel: 'stable',
				items: [{
					version: '0.2.0',
					status: 'available',
					source: 'update-feed',
					notes: ['Host-backed Feature Showcase and Change Log notes.'],
				}],
			}
		}
		return { ok: true }
	})
})

function renderUpdateRoute (path: string) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'server.read': true, 'server.settings.write': true }}>
				<Sidebar tools={toolCatalog} collapsed={false} onCollapse={vi.fn()} mobileOpen onNavigate={vi.fn()} onMobileDismiss={vi.fn()} />
				<Routes>
					<Route path="updates" element={<UpdatesPage />} />
					<Route path="updates/preferences" element={<UpdateToolPage toolId="preferences" />} />
					<Route path="updates/changelog" element={<UpdateToolPage toolId="changelog" />} />
					<Route path="*" element={<p>404 shell</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('dedicated update tool pages', () => {
	test('Update Preferences is a dedicated page with exclusive nav', async () => {
		renderUpdateRoute('/updates/preferences')

		expect(screen.queryByText('404 shell')).not.toBeInTheDocument()
		expect(screen.getByRole('heading', { name: 'Update Preferences' })).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Software Updates' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Change Log' })).not.toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Update Preferences' })).toHaveAttribute('href', '/updates/preferences')
		expect(screen.getByRole('link', { name: 'Update Preferences' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Software Updates' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Change Log' })).not.toHaveAttribute('aria-current')
		expect(await screen.findByText('stable')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Disable automatic updates' })).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Install verified release' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Check now' })).not.toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Open Software Updates' })).toHaveAttribute('href', '/updates')
	})

	test('Change Log is a dedicated reading page with exclusive nav', async () => {
		renderUpdateRoute('/updates/changelog')

		expect(screen.queryByText('404 shell')).not.toBeInTheDocument()
		expect(screen.getByRole('heading', { name: 'Change Log' })).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Software Updates' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Update Preferences' })).not.toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Change Log' })).toHaveAttribute('href', '/updates/changelog')
		expect(screen.getByRole('link', { name: 'Change Log' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'Software Updates' })).not.toHaveAttribute('aria-current')
		expect(screen.getByRole('link', { name: 'Update Preferences' })).not.toHaveAttribute('aria-current')
		expect((await screen.findAllByText('0.1.0')).length).toBeGreaterThan(0)
		expect(screen.getAllByText('0.2.0').length).toBeGreaterThan(0)
		expect(await screen.findByText('Host-backed Feature Showcase and Change Log notes.')).toBeInTheDocument()
		expect(screen.getByText(/update-feed/)).toBeInTheDocument()
		expect(screen.queryByText(/does not publish release notes/i)).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Install verified release' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Check now' })).not.toBeInTheDocument()
		expect(screen.getByRole('link', { name: SOFTWARE_UPDATES_CTA })).toHaveAttribute('href', '/updates')
	})

	test('Change Log empty state explains missing notes and points at Software Updates', async () => {
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'idle',
					installed_release: '0.1.0',
					automatic: true,
					channel: 'stable',
				}
			}
			if (path === '/api/v1/server/updates/changelog') {
				return { installed_release: '0.1.0', channel: 'stable', items: [] }
			}
			return { ok: true }
		})
		renderUpdateRoute('/updates/changelog')
		expect(await screen.findByText(CHANGELOG_EMPTY_TITLE)).toBeInTheDocument()
		expect(screen.getByText(CHANGELOG_EMPTY_DETAIL)).toBeInTheDocument()
		expect(screen.getByRole('link', { name: SOFTWARE_UPDATES_CTA })).toHaveAttribute('href', '/updates')
		expect(screen.queryByText(/does not publish release notes/i)).not.toBeInTheDocument()
	})

	test('Director breadcrumbs name the dedicated update tools', async () => {
		vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
			matches: false,
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		}))
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/updates') {
				return {
					state: 'available',
					installed_release: '0.1.0',
					available_release: '0.2.0',
					automatic: true,
					channel: 'stable',
				}
			}
			if (path === '/api/v1/server') {
				return { system: { hostname: 'host.example' }, stats: {}, services: [] }
			}
			return { values: {}, items: [] }
		})

		function renderShell (path: string) {
			return render(
				<MemoryRouter initialEntries={[path]}>
					<CapProvider caps={{ 'server.read': true, 'server.settings.write': true }}>
						<Routes>
							<Route element={<DirectorShell me={me} onSignOut={vi.fn()} />}>
								<Route path="updates" element={<UpdatesPage />} />
								<Route path="updates/preferences" element={<UpdateToolPage toolId="preferences" />} />
								<Route path="updates/changelog" element={<UpdateToolPage toolId="changelog" />} />
								<Route path="*" element={<p>404 shell</p>} />
							</Route>
						</Routes>
					</CapProvider>
				</MemoryRouter>,
			)
		}

		renderShell('/updates/preferences')
		const preferencesCrumbs = screen.getByRole('navigation', { name: 'Breadcrumb' })
		expect(preferencesCrumbs).toHaveTextContent('Home / Software Updates / Update Preferences')
		expect(screen.queryByText('404 shell')).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Update Preferences' })).toBeInTheDocument()
		cleanup()

		renderShell('/updates/changelog')
		const changelogCrumbs = screen.getByRole('navigation', { name: 'Breadcrumb' })
		expect(changelogCrumbs).toHaveTextContent('Home / Software Updates / Change Log')
		expect(screen.queryByText('404 shell')).not.toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Change Log' })).toBeInTheDocument()
	})

	test('Update Preferences saves the automatic install policy', async () => {
		const user = userEvent.setup()
		renderUpdateRoute('/updates/preferences')
		await screen.findByText('stable')
		await user.click(screen.getByRole('button', { name: 'Disable automatic updates' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/updates/settings', {
			method: 'PATCH',
			body: JSON.stringify({ automatic: false }),
		})
	})
})
