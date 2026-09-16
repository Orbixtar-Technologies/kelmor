// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { AccountSummaryPage } from './account-summary-page'

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

function account (id: string, username: string) {
	return {
		id,
		username,
		primary_domain: `${username}.test`,
		home_path: `/home/${username}`,
		status: 'active',
		linux_uid: 20001,
		linux_gid: 20001,
		package_id: 'pkg-1',
		desired_revision: 1,
		observed_revision: 1,
		reseller_id: '',
		ip_address: '',
		shell_class: 'nologin',
		login_disabled: false,
	}
}

function Switcher () {
	const navigate = useNavigate()
	return (
		<>
			<button type="button" onClick={() => navigate('/accounts/acc-b')}>Open bravo</button>
			<Routes>
				<Route path="/accounts/:id" element={<AccountSummaryPage />} />
			</Routes>
		</>
	)
}

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('AccountSummaryPage request races', () => {
	it('commits only the newest account load when older work resolves last', async () => {
		const firstAccount = deferred<ReturnType<typeof account>>()
		const secondAccount = deferred<ReturnType<typeof account>>()
		let accountCalls = 0
		vi.mocked(api).mockImplementation((path: string) => {
			const url = String(path)
			if (url === '/api/v1/packages' || url === '/api/v1/resellers' || url === '/api/v1/jobs') {
				return Promise.resolve({ items: [] })
			}
			if (url.endsWith('/usage')) return Promise.resolve({ disk_bytes: 1, bandwidth_bytes: 1 })
			if (url === '/api/v1/accounts/acc-a') {
				accountCalls += 1
				return firstAccount.promise
			}
			if (url === '/api/v1/accounts/acc-b') {
				accountCalls += 1
				return secondAccount.promise
			}
			return Promise.resolve({})
		})

		render(
			<MemoryRouter initialEntries={['/accounts/acc-a']}>
				<CapProvider caps={{ 'accounts.modify': true, 'accounts.suspend': true }}>
					<Switcher />
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		await waitFor(() => expect(accountCalls).toBe(1))
		await user.click(screen.getByRole('button', { name: 'Open bravo' }))
		await waitFor(() => expect(accountCalls).toBe(2))

		secondAccount.resolve(account('acc-b', 'bravo'))
		expect(await screen.findByRole('heading', { name: 'bravo' })).toBeInTheDocument()

		firstAccount.resolve(account('acc-a', 'alpha'))
		await waitFor(() => {
			expect(screen.queryByRole('heading', { name: 'alpha' })).not.toBeInTheDocument()
		})
		expect(screen.getByRole('heading', { name: 'bravo' })).toBeInTheDocument()
	})
})
