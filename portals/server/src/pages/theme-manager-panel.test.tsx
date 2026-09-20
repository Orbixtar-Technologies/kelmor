// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { ThemeManagerPanel } from './theme-manager-panel'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('ThemeManagerPanel', () => {
	test('pins, reorders, and saves chrome favorites', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/settings') {
				return Promise.resolve({
					values: { theme: { density: 'comfortable', favorites: 'jobs,dns' } },
				})
			}
			return Promise.resolve({})
		})

		render(
			<MemoryRouter>
				<CapProvider caps={{ 'server.settings.write': true, 'server.read': true, 'dns.write': true }}>
					<ThemeManagerPanel />
				</CapProvider>
			</MemoryRouter>,
		)

		expect(await screen.findByText('Jobs')).toBeInTheDocument()
		expect(screen.getByText('DNS Zone Manager')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Move DNS Zone Manager up' }))
		await user.selectOptions(screen.getByLabelText('Add a favorite'), 'email')
		await user.click(screen.getByRole('button', { name: 'Pin tool' }))
		await user.click(screen.getByRole('button', { name: 'Apply to chrome' }))

		await waitFor(() => {
			expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({
				method: 'PATCH',
			}))
		})
		const saveCall = api.mock.calls.find((call) => call[0] === '/api/v1/server/settings' && call[1]?.method === 'PATCH')
		expect(saveCall).toBeTruthy()
		expect(JSON.parse(String(saveCall?.[1]?.body))).toEqual({
			values: { theme: { density: 'comfortable', favorites: 'dns,jobs,email' } },
		})
	})
})
