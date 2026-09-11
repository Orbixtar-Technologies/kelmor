// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { ProcessManagerPage } from './process-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderPage (caps: Record<string, boolean> = { 'server.read': true, 'server.settings.write': true }) {
	return render(
		<MemoryRouter>
			<CapProvider caps={caps}>
				<ProcessManagerPage />
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('ProcessManagerPage', () => {
	test('lists live processes and sends a typed TERM signal', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/processes') {
				return Promise.resolve({
					processes: [
						{ pid: 1, name: 'systemd', user: 'root', command: '/sbin/init', scope: 'host' },
						{ pid: 4421, name: 'sleep', user: 'shop', command: 'sleep 3600', scope: 'account:shop' },
					],
				})
			}
			return Promise.resolve({ message: 'sent TERM to pid 4421' })
		})
		vi.spyOn(window, 'confirm').mockReturnValue(true)
		renderPage()
		expect(await screen.findByText('sleep')).toBeInTheDocument()
		expect(screen.getByText('protected')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'TERM' }))
		expect(api).toHaveBeenCalledWith('/api/v1/server/processes/4421/signal', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ signal: 'TERM' }),
		}))
	})
})
