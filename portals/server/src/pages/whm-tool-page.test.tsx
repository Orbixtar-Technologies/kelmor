// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
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
		expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({ method: 'PATCH' }))
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
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Confirm' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/bulk/clear-bandwidth-hold', expect.objectContaining({ method: 'POST' }))
		expect(api).not.toHaveBeenCalledWith('/api/v1/accounts/bulk/unsuspend', expect.anything())
	})

	test('mail queue only mounts Postfix recipes', async () => {
		api.mockResolvedValue({ items: [
			{ id: 'nginx-test', label: 'Test nginx configuration', description: 'Run nginx -t without reloading.' },
			{ id: 'postfix-queue', label: 'Show mail queue', description: 'List deferred and active Postfix queue entries.' },
			{ id: 'postfix-flush', label: 'Flush mail queue', description: 'Ask Postfix to retry deferred mail.' },
			{ id: 'postfix-status', label: 'Postfix status', description: 'Show Postfix service status.' },
		] })
		renderTool('/tools/mail-queue')
		expect(await screen.findByText('Show mail queue')).toBeInTheDocument()
		expect(screen.getByText('Flush mail queue')).toBeInTheDocument()
		expect(screen.queryByText('Test nginx configuration')).not.toBeInTheDocument()
	})
})
