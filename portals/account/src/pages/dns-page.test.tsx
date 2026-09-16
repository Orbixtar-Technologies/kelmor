// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
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

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('Control DNSPage request races', () => {
	it('commits only the newest account zones when older work resolves last', async () => {
		const first = deferred<{ items: Array<{ id: string; name: string }> }>()
		const second = deferred<{ items: Array<{ id: string; name: string }> }>()
		vi.mocked(api).mockImplementation((path: string) => {
			const url = String(path)
			if (url.includes('/acc-a/dns/zones') && !url.includes('/records')) return first.promise
			if (url.includes('/acc-b/dns/zones') && !url.includes('/records')) return second.promise
			if (url.includes('/records')) return Promise.resolve({ items: [] })
			return Promise.resolve({ items: [] })
		})

		const view = render(
			<CapProvider caps={{ 'dns.read': true, 'dns.write': true }}>
				<DNSPage accountId="acc-a" />
			</CapProvider>,
		)
		await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-a/dns/zones'))
		view.rerender(
			<CapProvider caps={{ 'dns.read': true, 'dns.write': true }}>
				<DNSPage accountId="acc-b" />
			</CapProvider>,
		)
		await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-b/dns/zones'))

		second.resolve({ items: [{ id: 'zone-b', name: 'bravo.test' }] })
		expect(await screen.findByText(/bravo.test/)).toBeInTheDocument()

		first.resolve({ items: [{ id: 'zone-a', name: 'stale.alpha.test' }] })
		await waitFor(() => {
			expect(screen.queryByText(/stale.alpha.test/)).not.toBeInTheDocument()
		})
		expect(screen.getByText(/bravo.test/)).toBeInTheDocument()
	})
})
