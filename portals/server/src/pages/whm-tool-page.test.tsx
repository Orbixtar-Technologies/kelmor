// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { HubPage } from './hub-page'
import { WhmToolPage } from './whm-tool-page'

const api = vi.fn().mockResolvedValue({ values: {}, items: [] })

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockClear()
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
				return Promise.resolve({ items: [{ id: 'pkg-2', name: 'Business' }] })
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
})
