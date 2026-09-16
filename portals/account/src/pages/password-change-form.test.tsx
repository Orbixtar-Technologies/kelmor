// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PasswordChangeForm } from './password-change-form'

function deferred<T> () {
	let resolve!: (value: T) => void
	const promise = new Promise<T>((next) => { resolve = next })
	return { promise, resolve }
}

vi.mock('../client', () => ({
	api: vi.fn(),
	APIClientError: class extends Error { code = '' },
	clearToken: vi.fn(),
	setToken: vi.fn(),
}))

import { api } from '../client'

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('PasswordChangeForm', () => {
	it('submits one password-change mutation when clicked twice', async () => {
		const change = deferred<Record<string, never>>()
		vi.mocked(api).mockImplementation((path: string) => {
			if (String(path).includes('complete-password-change')) return change.promise
			if (String(path).includes('login')) return Promise.resolve({ token: 't' })
			return Promise.resolve({ user: { username: 'livehost', roles: [] }, actor: {} })
		})
		const onSignedIn = vi.fn()
		render(<PasswordChangeForm username="livehost" currentPassword="old" onSignedIn={onSignedIn} />)
		const user = userEvent.setup()
		await user.type(screen.getByLabelText('New password'), 'Replacement!2026')
		await user.type(screen.getByLabelText('Confirm new password'), 'Replacement!2026')
		const submit = screen.getByRole('button', { name: 'Change password and sign in' })
		await user.click(submit)
		await user.click(submit)
		expect(vi.mocked(api).mock.calls.filter((call) => String(call[0]).includes('complete-password-change'))).toHaveLength(1)
		change.resolve({})
	})
})
