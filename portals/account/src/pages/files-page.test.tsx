// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { FilesPage } from './files-page'

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

describe('FilesPage', () => {
	it('saves against the account that produced the editor after a later account switch', async () => {
		const contentA = deferred<{ content: string }>()
		vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
			const url = String(path)
			if (url.includes('/files/content')) return contentA.promise
			if (url.includes('/files') && (!init || !init.method || init.method === 'GET')) {
				return Promise.resolve({ items: [{ name: 'note.txt', size: 4, dir: false }] })
			}
			if (url.includes('/ftp') || url.includes('/ssh-keys')) return Promise.resolve({ items: [] })
			return Promise.resolve({ ok: true })
		})
		const view = render(
			<CapProvider caps={{ 'files.read': true, 'files.write': true }}>
				<FilesPage accountId="acc-a" />
			</CapProvider>,
		)
		const user = userEvent.setup()
		await user.click(await screen.findByRole('button', { name: /note.txt/ }))
		view.rerender(
			<CapProvider caps={{ 'files.read': true, 'files.write': true }}>
				<FilesPage accountId="acc-b" />
			</CapProvider>,
		)
		contentA.resolve({ content: 'from-a' })
		expect(await screen.findByDisplayValue('from-a')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Save file' }))
		await waitFor(() => {
			const save = vi.mocked(api).mock.calls.find((call) => String(call[1]?.method) === 'POST' && String(call[0]).includes('/files') && !String(call[0]).includes('content'))
			expect(save?.[0]).toBe('/api/v1/accounts/acc-a/files')
		})
	})
})
