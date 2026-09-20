// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { WebsitesPage } from './websites-page'

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

function mockMultiPHP (sites = [
	{ id: 'site-1', domain_id: 'dom-1', document_root: '/home/alpha/public_html', runtime: 'php', runtime_version: '8.3', enabled: true },
	{ id: 'site-2', domain_id: 'dom-2', document_root: '/home/alpha/shop.alpha.test', runtime: 'php', runtime_version: '8.3', enabled: true },
]) {
	vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
		const url = String(path)
		if (url === '/api/v1/accounts') {
			return Promise.resolve({ items: [{ id: 'acc-1', username: 'alpha', primary_domain: 'a.test', home_path: '/home/alpha', status: 'active' }] })
		}
		if (url.endsWith('/domains')) return Promise.resolve({ items: [{ id: 'dom-1', ascii_fqdn: 'a.test' }] })
		if (url.endsWith('/websites') && (!init || !init.method || init.method === 'GET')) {
			return Promise.resolve({ items: sites })
		}
		if (init?.method === 'POST') return Promise.resolve({ operation_id: 'job-php-1' })
		return Promise.resolve({ items: [] })
	})
}

function renderWebsites () {
	return render(
		<MemoryRouter initialEntries={['/websites?account=acc-1']}>
			<CapProvider caps={{ 'websites.write': true, 'websites.read': true }}>
				<Routes>
					<Route path="/websites" element={<WebsitesPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

function postCalls () {
	return vi.mocked(api).mock.calls.filter((call) => String(call[1]?.method) === 'POST')
}

describe('WebsitesPage MultiPHP', () => {
	it('stages a PHP selector change for Review instead of applying immediately', async () => {
		mockMultiPHP()
		renderWebsites()
		const user = userEvent.setup()
		const version = await screen.findByLabelText('PHP version for /home/alpha/public_html')
		expect(screen.queryByRole('option', { name: '8.1' })).not.toBeInTheDocument()
		await user.selectOptions(version, '8.4')
		expect(screen.getByRole('button', { name: 'Review' })).toBeInTheDocument()
		expect(postCalls()).toHaveLength(0)

		await user.click(screen.getByRole('button', { name: 'Review' }))
		expect(screen.getByRole('heading', { name: 'Review' })).toBeInTheDocument()
		expect(screen.getByText('8.3 → 8.4')).toBeInTheDocument()
		expect(screen.getByText('/home/alpha/public_html')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Back' }))
		expect(screen.queryByRole('heading', { name: 'Review' })).not.toBeInTheDocument()
		expect(postCalls()).toHaveLength(0)

		await user.click(screen.getByRole('button', { name: 'Review' }))
		await user.click(screen.getByRole('button', { name: 'Apply' }))
		await waitFor(() => {
			expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/websites', expect.objectContaining({
				method: 'POST',
				body: expect.stringContaining('"runtime_version":"8.4"'),
			}))
		})
		expect(postCalls()).toHaveLength(1)
	})

	it('stages bulk PHP changes for selected vhosts with the same Review/Apply flow', async () => {
		mockMultiPHP()
		renderWebsites()
		const user = userEvent.setup()
		await screen.findByLabelText('PHP version for /home/alpha/public_html')
		await user.click(screen.getByRole('checkbox', { name: 'Select /home/alpha/public_html' }))
		await user.click(screen.getByRole('checkbox', { name: 'Select /home/alpha/shop.alpha.test' }))
		await user.selectOptions(screen.getByLabelText('PHP version for selected sites'), '8.5')
		await user.click(screen.getByRole('button', { name: 'Stage selected' }))
		await user.click(screen.getByRole('button', { name: 'Review' }))
		expect(screen.getAllByText('8.3 → 8.5')).toHaveLength(2)
		await user.click(screen.getByRole('button', { name: 'Apply' }))
		await waitFor(() => expect(postCalls()).toHaveLength(2))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/websites', expect.objectContaining({
			body: expect.stringContaining('"runtime_version":"8.5"'),
		}))
	})

	it('rejects unsupported PHP versions without queueing a job', async () => {
		mockMultiPHP([
			{ id: 'site-1', domain_id: 'dom-1', document_root: '/home/alpha/public_html', runtime: 'php', runtime_version: '8.3', enabled: true },
		])
		renderWebsites()
		const version = await screen.findByLabelText('PHP version for /home/alpha/public_html')
		const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value')?.set
		nativeSetter?.call(version, '8.1')
		version.dispatchEvent(new Event('change', { bubbles: true }))
		await screen.findByText(/unsupported PHP version/i)
		expect(screen.queryByRole('button', { name: 'Review' })).not.toBeInTheDocument()
		expect(postCalls()).toHaveLength(0)
	})
})
