// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { DNSPage } from './dns-page'

function deferred<T> () {
	let resolve!: (value: T) => void
	const promise = new Promise<T>((next) => { resolve = next })
	return { promise, resolve }
}

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

const accounts = [
	{ id: 'acc-a', username: 'alpha', primary_domain: 'a.test', home_path: '/home/alpha', status: 'active' },
	{ id: 'acc-b', username: 'bravo', primary_domain: 'b.test', home_path: '/home/bravo', status: 'active' },
]

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('DNSPage request races', () => {
	it('commits only the newest zone list when older work resolves last', async () => {
		const firstZones = deferred<{ items: Array<{ id: string; name: string }> }>()
		const secondZones = deferred<{ items: Array<{ id: string; name: string }> }>()
		vi.mocked(api).mockImplementation((path: string) => {
			const url = String(path)
			if (url === '/api/v1/accounts') return Promise.resolve({ items: accounts })
			if (url === '/api/v1/accounts/acc-a/dns/zones') return firstZones.promise
			if (url === '/api/v1/accounts/acc-b/dns/zones') return secondZones.promise
			if (url.includes('/records')) return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})

		render(
			<MemoryRouter initialEntries={['/dns?account=acc-a']}>
				<CapProvider caps={{ 'dns.read': true, 'dns.write': true }}>
					<Routes>
						<Route path="/dns" element={<DNSPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-a/dns/zones'))
		await user.selectOptions(screen.getByLabelText('Change account'), 'acc-b')
		await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-b/dns/zones'))

		secondZones.resolve({ items: [{ id: 'zone-b', name: 'bravo.test' }] })
		expect(await screen.findByText(/bravo.test/)).toBeInTheDocument()

		firstZones.resolve({ items: [{ id: 'zone-a', name: 'stale.alpha.test' }] })
		await waitFor(() => {
			expect(screen.queryByText(/stale.alpha.test/)).not.toBeInTheDocument()
		})
		expect(screen.getByText(/bravo.test/)).toBeInTheDocument()
	})
})
