// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { DnsCleanupPage, DnsSynchronizePage } from './dns-inventory-tools-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderCleanup (path = '/dns/cleanup') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'dns.read': true, 'dns.write': true }}>
				<Routes>
					<Route path="dns/cleanup" element={<DnsCleanupPage />} />
					<Route path="dns" element={<p>DNS Zone Manager hub</p>} />
					<Route path="mail/delivery-reports" element={<p>Mail Delivery Reports hub</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

function renderSync (path = '/dns/synchronize') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'dns.read': true, 'dns.write': true }}>
				<Routes>
					<Route path="dns/synchronize" element={<DnsSynchronizePage />} />
					<Route path="dns" element={<p>DNS Zone Manager hub</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('DNS inventory tools', () => {
	test('cleanup lists leftover zones and queues selected cleanup', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string; body?: string }) => {
			if (path === '/api/v1/dns/cleanup' && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'job-clean-1' })
			}
			if (path === '/api/v1/dns/cleanup') {
				return Promise.resolve({
					items: [
						{
							id: 'z-old', name: 'oldshop.test', account_username: 'oldshop',
							account_status: 'terminated', reason: 'terminated_account', records: 4,
						},
						{
							id: 'z-orphan', name: 'orphan.test', reason: 'untied_zone', records: 1,
						},
					],
				})
			}
			return Promise.resolve({ items: [] })
		})
		renderCleanup()
		expect(await screen.findByRole('heading', { name: 'Perform a DNS Cleanup' })).toBeInTheDocument()
		expect(screen.getByText('oldshop.test')).toBeInTheDocument()
		expect(screen.getByText('orphan.test')).toBeInTheDocument()
		expect(screen.getByText('Terminated account')).toBeInTheDocument()
		expect(screen.getByText('Not tied to an account')).toBeInTheDocument()
		expect(screen.queryByText('DNS Zone Manager hub')).not.toBeInTheDocument()
		expect(screen.queryByText('Mail Delivery Reports hub')).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Review' })).not.toBeInTheDocument()
		await user.click(screen.getByRole('checkbox', { name: 'Select oldshop.test' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Confirm cleanup' })).toBeInTheDocument()
		expect(screen.getByText('oldshop.test · oldshop')).toBeInTheDocument()
		expect(screen.queryByText(/orphan\.test/)).not.toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Queue cleanup' }))
		expect(api).toHaveBeenCalledWith('/api/v1/dns/cleanup', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ zone_ids: ['z-old'] }),
		}))
		expect(await screen.findByText(/DNS cleanup queued/)).toBeInTheDocument()
	})

	test('cleanup empty state is honest and does not open a blank confirm shell', async () => {
		api.mockResolvedValue({ items: [] })
		renderCleanup()
		expect(await screen.findByText('No leftover zones')).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Review' })).not.toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Confirm cleanup' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Continue' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Queue cleanup' })).not.toBeInTheDocument()
		expect(screen.queryByText('DNS Zone Manager hub')).not.toBeInTheDocument()
	})

	test('synchronize lists managed zones and queues selected sync', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (path === '/api/v1/dns/synchronize' && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'job-sync-1' })
			}
			if (path === '/api/v1/dns/synchronize') {
				return Promise.resolve({
					items: [
						{
							id: 'z-shop', name: 'shop.test', account_username: 'shop',
							account_status: 'active', reason: 'managed', records: 6,
							desired_revision: 3, observed_revision: 1,
						},
					],
				})
			}
			return Promise.resolve({ items: [] })
		})
		renderSync()
		expect(await screen.findByRole('heading', { name: 'Synchronize DNS Records' })).toBeInTheDocument()
		expect(screen.getByText('shop.test')).toBeInTheDocument()
		expect(screen.getByText('Needs republish')).toBeInTheDocument()
		expect(screen.queryByText('DNS Zone Manager hub')).not.toBeInTheDocument()
		await user.click(screen.getByRole('checkbox', { name: 'Select shop.test' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Queue sync' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Queue synchronize' }))
		expect(api).toHaveBeenCalledWith('/api/v1/dns/synchronize', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ zone_ids: ['z-shop'] }),
		}))
		expect(await screen.findByText(/DNS synchronize queued/)).toBeInTheDocument()
	})

	test('synchronize empty state is honest', async () => {
		api.mockResolvedValue({ items: [] })
		renderSync()
		expect(await screen.findByText('No zones to synchronize')).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Review' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Continue' })).not.toBeInTheDocument()
	})
})
