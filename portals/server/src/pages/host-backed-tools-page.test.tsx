// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { LOCAL_SETTINGS_BANNER, LOCAL_SETTINGS_LABEL } from '../catalog-honesty'
import { CapProvider } from '../rbac'
import { HubPage } from './hub-page'
import * as client from '../client'

afterEach(() => {
	cleanup()
	vi.restoreAllMocks()
})

beforeEach(() => {
	vi.spyOn(client, 'api').mockImplementation(async (path, init) => {
		if (path.startsWith('/api/v1/server/modules') && init?.method === 'POST') {
			return { operation_id: 'job-mod-1', status: 'provisioning' }
		}
		if (path.startsWith('/api/v1/server/modules')) {
			return { items: [{ kind: 'pecl', name: 'redis', status: 'installed' }] }
		}
		if (path === '/api/v1/server/support-access' && init?.method === 'POST') {
			return {
				id: 'grant-1', token: 'tok-support', username: 'kelmor-support',
				password: 'TempPass!x', expires_at: '2099-01-01T00:00:00Z',
				ticket: 'CASE-1', role: 'server_operator',
			}
		}
		if (path === '/api/v1/server/support-access') {
			return { items: [{ id: 'grant-1', ticket: 'CASE-1', expires_at: '2099-01-01T00:00:00Z' }] }
		}
		if (path === '/api/v1/server/cluster/publish') {
			return { operation_id: 'job-cluster-1', snapshot: { published_at: '2099-01-01T00:00:00Z', packages: [{}], feature_sets: [{}], note: 'snapshot' } }
		}
		if (path === '/api/v1/server/cluster/snapshot/import') {
			return { operation_id: 'job-cluster-import' }
		}
		if (path === '/api/v1/server/cluster/probe') {
			return { operation_id: 'job-cluster-probe', items: [{ url: 'https://peer.example.test:2087', ok: true, status: 200 }] }
		}
		if (path === '/api/v1/server/cluster/snapshot') {
			return {
				packages: [{ name: 'Starter' }],
				feature_sets: [{ name: 'full-hosting' }],
				capabilities: {
					peer_membership: true, snapshot_publish: true, snapshot_export: true,
					snapshot_import: true, peer_health_probe: true, live_multi_node: false,
				},
			}
		}
		if (path === '/api/v1/server/remote-access-key' && init?.method === 'POST') {
			return { operation_id: 'job-rak-1', key: 'hp_remote_secret', prefix: 'hp_remote_se', host_path: '/etc/panel/remote-access-key', created_at: '2099-01-01T00:00:00Z' }
		}
		if (path === '/api/v1/server/remote-access-key' && init?.method === 'DELETE') {
			return { operation_id: 'job-rak-2', revoked: true }
		}
		if (path === '/api/v1/server/remote-access-key') {
			return { applied: true, prefix: 'hp_remote_se', host_path: '/etc/panel/remote-access-key', created_at: '2099-01-01T00:00:00Z' }
		}
		if (path === '/api/v1/server/settings' && init?.method === 'PATCH') {
			return { operation_id: 'job-host-1', values: {} }
		}
		if (path === '/api/v1/server/settings') {
			return { values: { configuration_cluster: { peers: '' }, server_profile: { profile: 'standard' } } }
		}
		return { items: [], values: {} }
	})
	vi.spyOn(client, 'download').mockResolvedValue()
})

function renderTool (path: string) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'server.settings.write': true, 'server.read': true, 'api_tokens.write': true, 'api_tokens.read': true }}>
				<Routes>
					<Route path="section/:hubId" element={<HubPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('host-backed Director tools', () => {
	test.each([
		['/section/websites?tool=module-installers', 'Module Installers'],
		['/section/websites?tool=perl-modules', 'Install a Perl Module'],
		['/section/websites?tool=php-pear', 'Install a PHP PEAR Module'],
		['/section/websites?tool=php-pecl', 'Install a PHP PECL Module'],
		['/section/websites?tool=ruby-gems', 'Install a Ruby Gem'],
		['/section/server?tool=configuration-cluster', 'Configuration Cluster'],
		['/section/server?tool=remote-access-key', 'Remote Access Key'],
		['/section/server?tool=server-profile', 'Server Profile'],
		['/section/system?tool=grant-support-access', 'Grant Support Access'],
		['/section/system?tool=diagnostics-log', 'Download a Diagnostics File'],
	])('%s is not a local-only stub', async (path, title) => {
		renderTool(path)
		expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
		expect(screen.queryByText(LOCAL_SETTINGS_BANNER)).not.toBeInTheDocument()
		expect(screen.queryByText(LOCAL_SETTINGS_LABEL)).not.toBeInTheDocument()
	})

	test('module installers queue a host job', async () => {
		const user = userEvent.setup()
		renderTool('/section/websites?tool=php-pecl')
		expect(await screen.findByText('redis')).toBeInTheDocument()
		await user.type(screen.getByRole('textbox'), 'imagick')
		await user.click(screen.getByRole('button', { name: 'Install on host' }))
		expect(await screen.findByText(/install queued/i)).toBeInTheDocument()
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/modules', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ kind: 'pecl', name: 'imagick' }),
		}))
	})

	test('support access issues a server_operator session', async () => {
		const user = userEvent.setup()
		renderTool('/section/system?tool=grant-support-access')
		expect(await screen.findByText('CASE-1')).toBeInTheDocument()
		await user.type(screen.getByLabelText('Ticket id'), 'CASE-99')
		await user.click(screen.getByRole('button', { name: 'Issue support session' }))
		expect(await screen.findByText('tok-support')).toBeInTheDocument()
		expect(screen.getByText('TempPass!x')).toBeInTheDocument()
		expect(screen.getAllByText('server_operator').length).toBeGreaterThan(0)
	})

	test('diagnostics page has a download control', async () => {
		const user = userEvent.setup()
		renderTool('/section/system?tool=diagnostics-log')
		const button = await screen.findByRole('button', { name: 'Download diagnostics archive' })
		await user.click(button)
		expect(client.download).toHaveBeenCalledWith('/api/v1/server/diagnostics', 'kelmor-diagnostics.tar.gz')
		expect(await screen.findByText(/Diagnostics archive downloaded/)).toBeInTheDocument()
	})

	test('configuration cluster publishes a host snapshot', async () => {
		const user = userEvent.setup()
		renderTool('/section/server?tool=configuration-cluster')
		expect(await screen.findByText(/not live-replicate/i)).toBeInTheDocument()
		expect(screen.getByRole('table', { name: 'Cluster capability matrix' })).toHaveTextContent('Unavailable')
		expect(screen.queryByRole('button', { name: /apply to all nodes/i })).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Publish package snapshot' }))
		expect(await screen.findByText(/Job job-cluster-1/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Probe peer health' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/cluster/probe', expect.objectContaining({ method: 'POST' }))
	})

	test('remote access key issues and revokes a host-applied secret', async () => {
		const user = userEvent.setup()
		renderTool('/section/server?tool=remote-access-key')
		expect(await screen.findByRole('heading', { name: 'Remote Access Key' })).toBeInTheDocument()
		expect(screen.getAllByText('/etc/panel/remote-access-key').length).toBeGreaterThan(0)
		expect(screen.queryByText(LOCAL_SETTINGS_BANNER)).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Regenerate key' }))
		expect(await screen.findByText('hp_remote_secret')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Revoke key' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/remote-access-key', expect.objectContaining({ method: 'DELETE' }))
	})

	test('server profile save queues host apply', async () => {
		const user = userEvent.setup()
		renderTool('/section/server?tool=server-profile')
		expect(await screen.findByText(/host apply job/i)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Apply on host' }))
		expect(client.api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({
			method: 'PATCH',
		}))
	})
})
