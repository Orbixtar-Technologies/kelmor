// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { App } from './app'

vi.mock('./client', () => ({
	api: vi.fn(async () => ({
		user: { username: 'tenant', roles: ['customer_user'] },
		actor: { account_ids: ['acc-1'], capabilities: { 'mail.read': true } },
	})),
	APIClientError: class extends Error { code = '' },
	asList: (value: { items?: unknown[] }) => value.items || [],
	clearToken: vi.fn(),
	consumeImpersonationSession: vi.fn(() => false),
	getToken: vi.fn(() => 'session-token'),
	setToken: vi.fn(),
}))

import { api } from './client'

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('Control session and capabilities', () => {
	it('hides and rejects tools the session cannot use', async () => {
		vi.mocked(api).mockResolvedValue({
			user: { username: 'tenant', roles: ['customer_user'] },
			actor: { account_ids: ['acc-1'], capabilities: { 'mail.read': true } },
		})
		render(
			<MemoryRouter initialEntries={['/websites']}>
				<App />
			</MemoryRouter>,
		)
		expect(await screen.findByRole('alert')).toHaveTextContent('You do not have access to this tool.')
		expect(screen.queryByRole('link', { name: 'Websites' })).not.toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Email' })).toBeInTheDocument()
	})
})
