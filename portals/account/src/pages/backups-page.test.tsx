// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { BackupsPage } from './backups-page'

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
	vi.useRealTimers()
	vi.resetAllMocks()
})

describe('BackupsPage', () => {
	it('never has more than one poll in flight and cleanup drops late commits', async () => {
		vi.useFakeTimers()
		const first = deferred<{ items: Array<{ id: string }> }>()
		const second = deferred<{ items: Array<{ id: string }> }>()
		vi.mocked(api).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
		const view = render(
			<CapProvider caps={{ 'backups.read': true, 'backups.restore': true }}>
				<BackupsPage accountId="acc-1" />
			</CapProvider>,
		)
		expect(api).toHaveBeenCalledTimes(1)
		await vi.advanceTimersByTimeAsync(2000)
		expect(api).toHaveBeenCalledTimes(1)
		view.unmount()
		first.resolve({ items: [{ id: 'late' }] })
		await vi.runOnlyPendingTimersAsync()
		expect(api).toHaveBeenCalledTimes(1)
	})

	it('submits one restore when the button is clicked twice', async () => {
		vi.mocked(api).mockImplementation((path: string, init?: RequestInit) => {
			if (String(path).includes('/restores')) return Promise.resolve({ ok: true })
			return Promise.resolve({ items: [{ id: 'bak-1', state: 'succeeded', destination: 'local', kind: 'full' }] })
		})
		render(
			<CapProvider caps={{ 'backups.read': true, 'backups.restore': true }}>
				<BackupsPage accountId="acc-1" />
			</CapProvider>,
		)
		const user = userEvent.setup()
		const restore = await screen.findByRole('button', { name: 'Restore' })
		await user.click(restore)
		await user.click(restore)
		await waitFor(() => {
			expect(vi.mocked(api).mock.calls.filter((call) => String(call[0]).includes('/restores'))).toHaveLength(1)
		})
	})
})
