// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { FileManagerPage } from './file-manager-page'

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

describe('FileManagerPage request races', () => {
	it('commits only the newest directory listing when older work resolves last', async () => {
		const firstDir = deferred<{ items: Array<{ name: string; dir: boolean }>; path: string }>()
		const secondDir = deferred<{ items: Array<{ name: string; dir: boolean }>; path: string }>()
		vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
			const url = String(path)
			if (url === '/api/v1/accounts') return Promise.resolve({ items: accounts })
			if (url.includes('/acc-a/files') && (!init || !init.method || init.method === 'GET')) return firstDir.promise
			if (url.includes('/acc-b/files') && (!init || !init.method || init.method === 'GET')) return secondDir.promise
			return Promise.resolve({ items: [] })
		})

		render(
			<MemoryRouter initialEntries={['/files?account=acc-a']}>
				<CapProvider caps={{ 'files.read': true, 'files.write': true }}>
					<Routes>
						<Route path="/files" element={<FileManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		await waitFor(() => expect(api).toHaveBeenCalledWith(expect.stringContaining('/api/v1/accounts/acc-a/files')))
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-b')
		await waitFor(() => expect(api).toHaveBeenCalledWith(expect.stringContaining('/api/v1/accounts/acc-b/files')))

		secondDir.resolve({ items: [{ name: 'bravo-only.txt', dir: false }], path: '/' })
		expect(await screen.findByRole('button', { name: /bravo-only.txt/ })).toBeInTheDocument()

		firstDir.resolve({ items: [{ name: 'stale-alpha.txt', dir: false }], path: '/' })
		await waitFor(() => {
			expect(screen.queryByRole('button', { name: /stale-alpha.txt/ })).not.toBeInTheDocument()
		})
		expect(screen.getByRole('button', { name: /bravo-only.txt/ })).toBeInTheDocument()
	})

	it('saves against the account that produced the editor, not the later selection', async () => {
		const contentA = deferred<{ content: string }>()
		vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
			const url = String(path)
			if (url === '/api/v1/accounts') return Promise.resolve({ items: accounts })
			if (url.includes('/files/content')) return contentA.promise
			if (url.includes('/files') && (!init || !init.method || init.method === 'GET')) {
				return Promise.resolve({ items: [{ name: 'note.txt', size: 4, dir: false }], path: '/' })
			}
			if (init?.method === 'POST') return Promise.resolve({ ok: true })
			return Promise.resolve({ items: [] })
		})

		render(
			<MemoryRouter initialEntries={['/files?account=acc-a']}>
				<CapProvider caps={{ 'files.read': true, 'files.write': true }}>
					<Routes>
						<Route path="/files" element={<FileManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		expect(await screen.findByRole('button', { name: /note.txt/ })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Edit' }))
		await user.selectOptions(screen.getByLabelText('Account'), 'acc-b')
		contentA.resolve({ content: 'from-a' })
		expect(await screen.findByDisplayValue('from-a')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Save file' }))
		await waitFor(() => {
			const save = vi.mocked(api).mock.calls.find((call) => String(call[1]?.method) === 'POST' && String(call[0]).includes('/files'))
			expect(save?.[0]).toBe('/api/v1/accounts/acc-a/files')
		})
	})
})
