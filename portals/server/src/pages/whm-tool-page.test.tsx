// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { HubPage } from './hub-page'
import { WhmToolPage } from './whm-tool-page'

const api = vi.fn().mockResolvedValue({ values: {}, items: [] })
const download = vi.fn().mockResolvedValue(undefined)

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
	download: (...args: unknown[]) => download(...args),
}))

afterEach(() => {
	cleanup()
	api.mockClear()
	download.mockClear()
})

function renderTool (path: string, caps: Record<string, boolean> = { 'server.settings.write': true, 'server.read': true }) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={caps}>
				<Routes>
					<Route path="tools/:toolId" element={<WhmToolPage />} />
					<Route path="section/:hubId" element={<HubPage />} />
					<Route path="domains" element={<p>List Domains hub</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('WhmToolPage', () => {
	test('explains an unknown catalog id', () => {
		renderTool('/tools/not-a-real-tool')
		expect(screen.getByRole('heading', { name: 'Tool not found' })).toBeInTheDocument()
	})

	test('ftp server config exposes banner and queues host apply', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/server/settings' && options?.method === 'PATCH') {
				return Promise.resolve({ operation_id: 'job-ftp-1', values: {} })
			}
			if (String(path) === '/api/v1/server/settings') {
				return Promise.resolve({ values: { ftp_server: { pasv_min: '40000', pasv_max: '40100' } } })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/ftp-server-config')
		expect(await screen.findByRole('heading', { name: 'FTP Server Configuration' })).toBeInTheDocument()
		expect(await screen.findByText(/Save queues a host apply job/)).toBeInTheDocument()
		expect(screen.getByLabelText(/FTP banner/i)).toBeInTheDocument()
		expect(screen.getByLabelText(/PASV min port/i)).toBeInTheDocument()
		expect(screen.getByLabelText(/PASV max port/i)).toBeInTheDocument()
		await user.clear(screen.getByLabelText(/FTP banner/i))
		await user.type(screen.getByLabelText(/FTP banner/i), 'Kelmor FTP ready.')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Apply on host' }))
		expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({
			method: 'PATCH',
			body: JSON.stringify({
				values: {
					ftp_server: {
						pasv_min: '40000',
						pasv_max: '40100',
						banner: 'Kelmor FTP ready.',
					},
				},
			}),
		}))
	})

	test('renders a settings journey and saves host preferences', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/settings') return Promise.resolve({ values: { 'tweak-settings': {} } })
			return Promise.resolve({ values: {} })
		})
		renderTool('/tools/tweak-settings')
		expect(await screen.findByRole('heading', { name: 'Tweak Settings' })).toBeInTheDocument()
		expect(screen.getByText(/Server Configuration/)).toBeInTheDocument()
		expect(await screen.findByText(/Save queues a host apply job/)).toBeInTheDocument()
		expect(screen.queryByText(/Settings \(local\) — Not applied to host/)).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Apply on host' }))
		expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({
			method: 'PATCH',
			body: JSON.stringify({
				values: {
					tweak_settings: {
						max_emails_hour: '500',
						allow_parked: 'on',
						notify_disk: '90',
						default_php: '8.3',
					},
				},
			}),
		}))
	})

	test('opens the terminal on audited host recipes, not a freeform root shell', async () => {
		api.mockResolvedValue({ items: [
			{ id: 'nginx-test', label: 'Test nginx configuration', description: 'Run nginx -t without reloading.' },
		] })
		renderTool('/tools/terminal', { 'server.read': true })
		expect(await screen.findByRole('heading', { name: 'Terminal' })).toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Host console' })).toBeInTheDocument()
		expect(screen.getAllByText(/not a freeform root shell/i).length).toBeGreaterThan(0)
		expect(screen.getByText('Test nginx configuration')).toBeInTheDocument()
	})

	test('quota modification applies a package change instead of a preference stub', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path) === '/api/v1/packages') {
				return Promise.resolve({ items: [{ id: 'pkg-2', name: 'Business', disk_bytes: 10 * 1024 * 1024 * 1024, bandwidth_bytes_monthly: 100 * 1024 * 1024 * 1024 }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1' && options?.method === 'PATCH') {
				return Promise.resolve({ operation_id: 'rec-1' })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/quota-modification', { 'accounts.modify': true, 'accounts.read': true })
		expect(await screen.findByText(/no separate per-account override/i)).toBeInTheDocument()
		expect(screen.queryByText(/Settings \(local\) — Not applied to host/)).not.toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		await user.selectOptions(screen.getByLabelText('Package'), 'pkg-2')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getByText('Business')).toBeInTheDocument()
		expect(screen.queryByText('pkg-2')).not.toBeInTheDocument()
		expect(screen.getByText('10.0 GB')).toBeInTheDocument()
		expect(screen.getByText('100.0 GB')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Apply host change' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1', expect.objectContaining({
			method: 'PATCH',
			body: JSON.stringify({ package_id: 'pkg-2' }),
		}))
		expect(api).not.toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({ method: 'PATCH' }))
	})

	test('file and directory restore posts a path-scoped restore', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/backups') {
				return Promise.resolve({ items: [{ id: 'bak-9', kind: 'full', state: 'succeeded', created_at: '2026-09-20T12:00:00Z', restorable: true }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/restores' && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'restore-1' })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/file-dir-restore', { 'backups.restore': true, 'accounts.read': true })
		expect(await screen.findByRole('heading', { name: 'File and Directory Restoration' })).toBeInTheDocument()
		await screen.findByRole('option', { name: /shop/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getAllByText(/bak-9/).length).toBeGreaterThan(0)
		expect(screen.getByText(/public_html/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Apply' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/restores', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ path: 'public_html' }),
		}))
	})

	test('unsuspend bandwidth clears holds instead of bulk-unsuspending everyone', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', status: 'suspended' }] })
			}
			if (String(path) === '/api/v1/accounts/bulk/clear-bandwidth-hold') {
				return Promise.resolve({ operations: ['hold-1'] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/unsuspend-bandwidth', { 'accounts.suspend': true, 'accounts.read': true })
		expect(await screen.findByRole('heading', { name: 'Unsuspend Bandwidth Exceeders' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Confirm' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/bulk/clear-bandwidth-hold', expect.objectContaining({ method: 'POST' }))
		expect(api).not.toHaveBeenCalledWith('/api/v1/accounts/bulk/unsuspend', expect.anything())
		expect(api).not.toHaveBeenCalledWith('/api/v1/accounts/bulk/unsuspend-bandwidth', expect.anything())
	})

	test('mail queue Show reads GET /mail/queue and does not POST the console recipe', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/mail/queue') {
				return Promise.resolve({ items: [], message: 'Postfix queue is empty.' })
			}
			if (String(path) === '/api/v1/server/console/recipes') {
				return Promise.resolve({ items: [
					{ id: 'nginx-test', label: 'Test nginx configuration', description: 'Run nginx -t without reloading.' },
					{ id: 'postfix-queue', label: 'Show mail queue', description: 'List deferred and active Postfix queue entries.' },
					{ id: 'postfix-flush', label: 'Flush mail queue', description: 'Ask Postfix to retry deferred mail.' },
					{ id: 'postfix-status', label: 'Postfix status', description: 'Show Postfix service status.' },
				] })
			}
			return Promise.resolve({ items: [] })
		})
		renderTool('/tools/mail-queue', { 'mail.read': true, 'server.settings.write': true })
		expect(await screen.findByRole('heading', { name: 'Mail Queue Manager' })).toBeInTheDocument()
		expect(await screen.findByRole('button', { name: 'Show mail queue' })).toBeInTheDocument()
		expect(await screen.findByText('Mail queue is empty')).toBeInTheDocument()
		expect(screen.getByText('Flush mail queue')).toBeInTheDocument()
		expect(screen.queryByText('Test nginx configuration')).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Show mail queue' }))
		expect(api).toHaveBeenCalledWith('/api/v1/mail/queue')
		expect(api).not.toHaveBeenCalledWith('/api/v1/server/console', expect.anything())
	})

	test('convert addon shows an honest empty state when the account has no addons', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'alpha', primary_domain: 'alpha.test' }] })
			}
			if (String(path) === '/api/v1/packages') {
				return Promise.resolve({ items: [{ id: 'pkg-1', name: 'Starter' }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/domains') {
				return Promise.resolve({ items: [
					{ id: 'dom-1', ascii_fqdn: 'alpha.test', type: 'primary' },
					{ id: 'dom-2', ascii_fqdn: 'park.alpha.test', type: 'alias' },
				] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/section/system?tool=convert-addon', {
			'accounts.create': true, 'domains.write': true, 'accounts.read': true,
		})
		expect(await screen.findByRole('heading', { name: 'Convert Addon Domain to Account' })).toBeInTheDocument()
		await screen.findByRole('option', { name: /alpha/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		expect(await screen.findByText('No addon domains to convert')).toBeInTheDocument()
		const cta = screen.getByRole('link', { name: 'Create or manage domains' })
		expect(cta).toHaveAttribute('href', '/domains?account=acc-1&view=addon')
		expect(screen.queryByRole('button', { name: 'Continue' })).not.toBeInTheDocument()
		expect(screen.queryByLabelText('Addon domain')).not.toBeInTheDocument()
		expect(screen.queryByText('List Domains hub')).not.toBeInTheDocument()
	})

	test('convert addon reviews a real addon then queues host-backed conversion', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'alpha', primary_domain: 'alpha.test' }] })
			}
			if (String(path) === '/api/v1/packages') {
				return Promise.resolve({ items: [{ id: 'pkg-1', name: 'Starter' }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/domains') {
				return Promise.resolve({ items: [
					{ id: 'dom-1', ascii_fqdn: 'alpha.test', type: 'primary' },
					{ id: 'dom-2', ascii_fqdn: 'shop.alpha.test', type: 'addon' },
				] })
			}
			if (String(path) === '/api/v1/accounts/convert-addon' && options?.method === 'POST') {
				return Promise.resolve({ resource_id: 'acc-new', operation_id: 'job-convert-1' })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/section/system?tool=convert-addon', {
			'accounts.create': true, 'domains.write': true, 'accounts.read': true,
		})
		await screen.findByRole('option', { name: /alpha/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		expect(await screen.findByLabelText('Addon domain')).toBeInTheDocument()
		expect(screen.getByRole('option', { name: 'shop.alpha.test' })).toBeInTheDocument()
		expect(screen.queryByRole('option', { name: 'alpha.test' })).not.toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('Addon domain'), 'shop.alpha.test')
		await user.type(screen.getByLabelText('New username'), 'shop')
		await user.type(screen.getByLabelText('Owner password'), 'TenantPass!2026')
		await user.selectOptions(screen.getByLabelText('Package'), 'pkg-1')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getByText('shop.alpha.test')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Apply' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/convert-addon', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({
				account_id: 'acc-1',
				addon_domain: 'shop.alpha.test',
				username: 'shop',
				package_id: 'pkg-1',
				owner_password: 'TenantPass!2026',
			}),
		}))
		expect(await screen.findByText(/Addon conversion queued/)).toBeInTheDocument()
	})

	test('raw nginx log review lists host paths and downloads without a dead Apply', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org', home_path: '/home/orbixtar' }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/nginx-logs') {
				return Promise.resolve({ items: [
					{ kind: 'access', path: '/home/orbixtar/logs/access.log', present: true, size_bytes: 32 },
					{ kind: 'error', path: '/home/orbixtar/logs/error.log', present: false, size_bytes: 0 },
					{ kind: 'access', path: '/var/log/nginx/site-1.access.log', website_id: 'site-1', domain: 'orbixtar.dpdns.org', present: true, size_bytes: 128 },
				] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/raw-nginx-log', { 'accounts.read': true })
		await screen.findByRole('option', { name: /orbixtar/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(await screen.findByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(await screen.findByText('/home/orbixtar/logs/access.log')).toBeInTheDocument()
		expect(screen.getByText('/home/orbixtar/logs/error.log')).toBeInTheDocument()
		expect(screen.getByText('/var/log/nginx/site-1.access.log')).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
		expect(screen.queryByText('Nothing to apply.')).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Download access.log' }))
		expect(download).toHaveBeenCalledWith(
			'/api/v1/accounts/acc-1/nginx-logs/content?path=%2Fhome%2Forbixtar%2Flogs%2Faccess.log',
			'access.log',
		)
	})

	test('ip migration review lists tenants on the source IP', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [
					{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org', ip_address: '203.0.113.40' },
					{ id: 'acc-2', username: 'other', primary_domain: 'other.test', ip_address: '' },
				] })
			}
			if (String(path).startsWith('/api/v1/accounts/ip-migration') && !options?.method) {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org', ip_address: '203.0.113.40' }] })
			}
			if (String(path) === '/api/v1/accounts/ip-migration' && options?.method === 'POST') {
				return Promise.resolve({ operations: ['job-ip-1'] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/ip-migration', { 'accounts.modify': true, 'accounts.read': true })
		await user.type(screen.getByLabelText('Source IP'), '203.0.113.40')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.type(screen.getByLabelText('Destination IP'), '203.0.113.50')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(await screen.findByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(await screen.findByText(/orbixtar · orbixtar.dpdns.org/)).toBeInTheDocument()
		expect(screen.queryByText(/other\.test/)).not.toBeInTheDocument()
		expect(screen.queryByLabelText('Source IP')).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Apply' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/ip-migration', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ from_ip: '203.0.113.40', to_ip: '203.0.113.50' }),
		}))
	})

	test('email all users review lists recipients and queues POST /mail/notify', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org' }] })
			}
			if (String(path).startsWith('/api/v1/mail/notify') && !options?.method) {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', email: 'owner@orbixtar.dpdns.org', primary_domain: 'orbixtar.dpdns.org' }] })
			}
			if (String(path) === '/api/v1/mail/notify' && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'mail-1' })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/email-all-users', { 'accounts.read': true, 'server.settings.write': true })
		expect(await screen.findByRole('status')).toHaveTextContent(/POST \/mail\/notify/)
		expect(screen.queryByText(/Save queues a host apply job/)).not.toBeInTheDocument()
		await user.type(screen.getByLabelText('Subject'), 'Host notice')
		await user.type(screen.getByLabelText('Message'), 'Maintenance window tonight.')
		await user.type(screen.getByLabelText('From address'), 'ops@kelmor.host')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getByText(/orbixtar · owner@orbixtar.dpdns.org/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Queue mail' }))
		expect(api).toHaveBeenCalledWith('/api/v1/mail/notify', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({
				from: 'ops@kelmor.host',
				subject: 'Host notice',
				body: 'Maintenance window tonight.',
				audience: 'owners',
			}),
		}))
		expect(api).not.toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({ method: 'PATCH' }))
	})

	test('rearrange account review shows the home path and disables Apply', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org', home_path: '/home/orbixtar', linux_uid: 20001, linux_gid: 20001 }] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/rearrange-account', { 'accounts.read': true })
		await screen.findByRole('option', { name: /orbixtar/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByText('/home/orbixtar')).toBeInTheDocument()
		expect(screen.getByText(/not implemented/i)).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
	})

	test('manage demo mode review shows the current set without a host-apply banner', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'orbixtar', primary_domain: 'orbixtar.dpdns.org', home_path: '/home/orbixtar' }] })
			}
			if (String(path) === '/api/v1/server/settings' && options?.method === 'PATCH') {
				return Promise.resolve({ values: {} })
			}
			if (String(path) === '/api/v1/server/settings') {
				return Promise.resolve({ values: { demo_accounts: { usernames: 'orbixtar' } } })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/manage-demo-mode', { 'accounts.modify': true, 'server.settings.write': true })
		expect(await screen.findByText(/Director policy/)).toBeInTheDocument()
		expect(await screen.findByDisplayValue('orbixtar')).toBeInTheDocument()
		expect(screen.queryByText(/Save queues a host apply job/)).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getByText(/orbixtar · orbixtar.dpdns.org/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Save demo set' }))
		expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({
			method: 'PATCH',
			body: JSON.stringify({ values: { demo_accounts: { usernames: 'orbixtar' } } }),
		}))
	})

	test('file restore review shows an honest empty inventory', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test' }] })
			}
			if (String(path) === '/api/v1/accounts/acc-1/backups') {
				return Promise.resolve({ items: [] })
			}
			return Promise.resolve({ values: {}, items: [] })
		})
		renderTool('/tools/file-dir-restore', { 'backups.restore': true, 'accounts.read': true })
		await screen.findByRole('option', { name: /shop/ })
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-1')
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByText('No restorable backups')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Configure or run backups' })).toHaveAttribute('href', '/transfers')
		expect(screen.getByRole('button', { name: 'Apply' })).toBeDisabled()
	})
})
