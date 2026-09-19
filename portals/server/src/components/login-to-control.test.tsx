// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { LoginToControl } from './login-to-control'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
}))

afterEach(() => {
	cleanup()
	api.mockReset()
	vi.unstubAllGlobals()
})

describe('LoginToControl', () => {
	test('opens the advertised Control URL in a window reserved during the click', async () => {
		const user = userEvent.setup()
		const popup = { closed: false, opener: window, close: vi.fn(), location: { replace: vi.fn() } }
		const open = vi.fn(() => popup)
		vi.stubGlobal('open', open)
		api.mockResolvedValue({
			token: 'sess-token',
			control_url: 'https://kelmor.host:2083/',
		})
		render(
			<CapProvider caps={{ 'accounts.impersonate': true }}>
				<LoginToControl accountId="acc-1" username="orbixtar" autoOpen />
			</CapProvider>,
		)
		await user.click(screen.getByRole('button', { name: 'Open Kelmor Control' }))
		expect(open).toHaveBeenCalledWith('about:blank', '_blank')
		expect(popup.location.replace).toHaveBeenCalledWith('https://kelmor.host:2083/#session=sess-token')
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/impersonate', expect.objectContaining({ method: 'POST' }))
	})
})
