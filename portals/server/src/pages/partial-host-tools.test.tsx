// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { LOCAL_SETTINGS_BANNER, LOCAL_SETTINGS_LABEL } from '../catalog-honesty'
import { CapProvider } from '../rbac'
import { ExternalAuthPage } from './external-auth-page'
import { InitialQuotaPage } from './initial-quota-page'
import { LinkNodesPage } from './link-nodes-page'
import { LoginPage } from './login-page'
import { ResetBandwidthPage } from './reset-bandwidth-page'
import { ResellerUsagePage } from './reseller-usage-page'
import { SkeletonDirectoryPage } from './skeleton-directory-page'
import { TwoFactorPage } from './two-factor-page'
import * as client from '../client'

afterEach(() => {
	cleanup()
	vi.restoreAllMocks()
})

function renderPage (path: string, element: ReactNode, caps: Record<string, boolean> = {
	'accounts.modify': true, 'server.read': true, 'server.settings.write': true, 'resellers.read': true,
}) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={caps}>
				<Routes>
					<Route path="accounts/reset-bandwidth" element={element} />
					<Route path="accounts/skeleton" element={element} />
					<Route path="resellers/usage" element={element} />
					<Route path="security/external-auth" element={element} />
					<Route path="security/two-factor" element={element} />
					<Route path="server/link-nodes" element={element} />
					<Route path="server/initial-quota" element={element} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('host-backed PARTIAL tools', () => {
	test('reset bandwidth queues a per-account Agent job', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
			if (path === '/api/v1/accounts') return { items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test', status: 'active' }] }
			if (path === '/api/v1/accounts/acc-1/usage') return { account_id: 'acc-1', bandwidth_bytes: 4096, bandwidth_hold: true }
			if (String(path).includes('/bandwidth/reset') && init?.method === 'POST') return { operation_id: 'job-bw-1' }
			return { items: [] }
		})
		renderPage('/accounts/reset-bandwidth?account=acc-1', <ResetBandwidthPage />)
		expect(await screen.findByRole('heading', { name: 'Reset Account Bandwidth Limit' })).toBeInTheDocument()
		expect(screen.getByText(/4\.0 KB/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Reset bandwidth usage' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/bandwidth/reset', expect.objectContaining({ method: 'POST' }))
		expect(await screen.findByText(/Job job-bw-1/)).toBeInTheDocument()
	})

	test('skeleton directory edits a host file', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
			if (path.startsWith('/api/v1/server/skeleton/file') && init?.method === 'PUT') return { ok: true }
			if (path.startsWith('/api/v1/server/skeleton/file')) return { content: 'export PATH' }
			if (path.startsWith('/api/v1/server/skeleton')) return { items: [{ name: '.bashrc', dir: false, size: 12 }] }
			return { items: [] }
		})
		renderPage('/accounts/skeleton', <SkeletonDirectoryPage />)
		expect(await screen.findByRole('heading', { name: 'Skeleton Directory' })).toBeInTheDocument()
		expect(screen.getByText('/etc/skel')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: '.bashrc' }))
		expect(await screen.findByDisplayValue('export PATH')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Save on host' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/skeleton/file', expect.objectContaining({ method: 'PUT' }))
	})

	test('reseller usage shows meters and a reset action', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
			if (path === '/api/v1/resellers/usage') {
				return {
					items: [{
						id: 'res-1', name: 'North', brand_name: 'North Host', status: 'active',
						accounts: 2, active: 1, suspended: 1, disk_bytes: 1024, disk_limit: 2048,
						bandwidth_bytes: 4096, bandwidth_limit: 8192, bandwidth_holds: 1,
					}],
				}
			}
			if (String(path).includes('/bandwidth/reset') && init?.method === 'POST') return { operations: ['job-r-1'], cleared: 1 }
			return { items: [] }
		})
		renderPage('/resellers/usage', <ResellerUsagePage />)
		expect(await screen.findByRole('heading', { name: 'View Reseller Usage and Manage Account Status' })).toBeInTheDocument()
		expect(screen.getByText('North')).toBeInTheDocument()
		expect(screen.getByText(/1\.0 KB \/ 2\.0 KB/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Reset bandwidth' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/resellers/res-1/bandwidth/reset', expect.objectContaining({ method: 'POST' }))
		expect(screen.getByLabelText(/Disk/)).toBeInTheDocument()
		expect(screen.getByLabelText(/Bandwidth/)).toBeInTheDocument()
	})

	test('reseller usage empty state has a create CTA and no meter table', async () => {
		vi.spyOn(client, 'api').mockResolvedValue({ items: [] })
		renderPage('/resellers/usage', <ResellerUsagePage />)
		expect(await screen.findByText('No resellers')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Create a reseller' })).toHaveAttribute('href', '/resellers')
		expect(screen.queryByRole('columnheader', { name: 'Disk' })).not.toBeInTheDocument()
	})

	test('reseller usage shows meter chrome when counts are zero', async () => {
		vi.spyOn(client, 'api').mockResolvedValue({
			items: [{
				id: 'res-empty', name: 'Idle', status: 'active',
				accounts: 0, active: 0, suspended: 0, disk_bytes: 0, disk_limit: 0,
				bandwidth_bytes: 0, bandwidth_limit: 0, bandwidth_holds: 0,
			}],
		})
		renderPage('/resellers/usage', <ResellerUsagePage />)
		expect(await screen.findByText('Idle')).toBeInTheDocument()
		expect(screen.getAllByText(/0 B \/ No package limits/).length).toBe(2)
		expect(screen.getByLabelText(/Disk/)).toBeInTheDocument()
		expect(screen.queryByText('No resellers')).not.toBeInTheDocument()
	})

	test('external auth and link nodes are host-applied, not local banners', async () => {
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/server/nodes') return { items: [{ url: 'https://peer.example.test:2087', source: 'linked' }] }
			if (path === '/api/v1/server/settings') return { values: { external_auth: { provider: 'ldap' }, linked_nodes: { nodes: '' } } }
			return { items: [], values: {} }
		})
		renderPage('/security/external-auth', <ExternalAuthPage />)
		expect(await screen.findByRole('heading', { name: 'Manage External Authentications' })).toBeInTheDocument()
		expect(screen.getAllByText(/external-auth.json/).length).toBeGreaterThan(0)
		expect(screen.queryByText(LOCAL_SETTINGS_BANNER)).not.toBeInTheDocument()
		expect(screen.queryByText(LOCAL_SETTINGS_LABEL)).not.toBeInTheDocument()
		cleanup()
		renderPage('/server/link-nodes', <LinkNodesPage />)
		expect(await screen.findByRole('heading', { name: 'Link Server Nodes' })).toBeInTheDocument()
		expect(screen.getByText('https://peer.example.test:2087')).toBeInTheDocument()
		expect(screen.queryByText(LOCAL_SETTINGS_BANNER)).not.toBeInTheDocument()
	})

	test('two-factor enrolls TOTP and applies host policy', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
			if (path === '/api/v1/me') return { user: { username: 'admin', totp_enabled: false, roles: [], email: '' }, actor: {} }
			if (path === '/api/v1/auth/totp/enroll') return { secret: 'ABC123', otpauth_url: 'otpauth://totp/Kelmor' }
			if (path === '/api/v1/auth/totp/confirm') return { totp_enabled: true }
			if (path === '/api/v1/server/settings' && init?.method === 'PATCH') return { operation_id: 'job-2fa-1' }
			if (path === '/api/v1/server/settings') return { values: { two_factor: { required: 'off' } } }
			return {}
		})
		renderPage('/security/two-factor', <TwoFactorPage />)
		expect(await screen.findByRole('heading', { name: 'Two-Factor Authentication' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Generate authenticator secret' }))
		expect(await screen.findByText('ABC123')).toBeInTheDocument()
		await user.type(screen.getByLabelText('Authenticator code'), '123456')
		await user.click(screen.getByRole('button', { name: 'Confirm enroll' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/auth/totp/confirm', expect.objectContaining({ method: 'POST' }))
		await user.click(screen.getByRole('button', { name: 'Apply on host' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({ method: 'PATCH' }))
	})

	test('initial quota applies a host setup job', async () => {
		const user = userEvent.setup()
		vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
			if (path === '/api/v1/server/quota/setup') return { operation_id: 'job-quota-1' }
			if (path === '/api/v1/server/quota') {
				return { kernel_quota: true, setquota: true, homes_present: true, policy_bytes: 0, enforce: false, host_path: '/etc/panel/initial-quota' }
			}
			return {}
		})
		renderPage('/server/initial-quota', <InitialQuotaPage />)
		expect(await screen.findByRole('heading', { name: 'Initial Quota Setup' })).toBeInTheDocument()
		expect(screen.getByText('/etc/panel/initial-quota')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Apply on host' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/quota/setup', expect.objectContaining({ method: 'POST' }))
	})

	test('login submits an optional authenticator code', async () => {
		const user = userEvent.setup()
		const onLogin = vi.fn()
		vi.spyOn(client, 'api').mockImplementation(async (path) => {
			if (path === '/api/v1/auth/login') return { token: 'tok', user: { username: 'admin' } }
			if (path === '/api/v1/me') return { user: { username: 'admin' }, actor: { capabilities: {} } }
			return {}
		})
		vi.spyOn(client, 'setToken').mockImplementation(() => undefined)
		render(<LoginPage onLogin={onLogin} />)
		await user.type(screen.getByLabelText('Username'), 'admin')
		await user.type(screen.getByLabelText('Password'), 'secret')
		await user.type(screen.getByLabelText('Authenticator code'), '654321')
		await user.click(screen.getByRole('button', { name: 'Sign in to Kelmor Director' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ username: 'admin', password: 'secret', totp_code: '654321' }),
		}))
	})
})
