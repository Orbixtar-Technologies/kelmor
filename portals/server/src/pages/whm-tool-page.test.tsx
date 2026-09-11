// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { WhmToolPage } from './whm-tool-page'

const api = vi.fn().mockResolvedValue({ values: {}, items: [] })

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockClear()
})

function renderTool (path: string, caps: Record<string, boolean> = { 'server.settings.write': true, 'server.read': true }) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={caps}>
				<Routes>
					<Route path="tools/:toolId" element={<WhmToolPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('WhmToolPage', () => {
	test('explains an unknown catalog id', () => {
		renderTool('/tools/not-a-real-tool')
		expect(screen.getByRole('heading', { name: 'Tool not found' })).toBeInTheDocument()
	})

	test('renders a settings journey and saves host preferences', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path) === '/api/v1/server/settings') return Promise.resolve({ values: { 'tweak-settings': {} } })
			return Promise.resolve({ values: {} })
		})
		renderTool('/tools/tweak-settings')
		expect(await screen.findByRole('heading', { name: 'Tweak Settings' })).toBeInTheDocument()
		expect(screen.getByText(/Server Configuration/)).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		await user.click(screen.getByRole('button', { name: 'Save settings' }))
		expect(api).toHaveBeenCalledWith('/api/v1/server/settings', expect.objectContaining({ method: 'PATCH' }))
	})

	test('opens the terminal on audited host recipes, not a freeform root shell', async () => {
		api.mockResolvedValue({ items: [
			{ id: 'nginx-test', label: 'Test nginx configuration', description: 'Run nginx -t without reloading.' },
		] })
		renderTool('/tools/terminal', { 'server.read': true })
		expect(await screen.findByRole('heading', { name: 'Terminal' })).toBeInTheDocument()
		expect(await screen.findByRole('heading', { name: 'Host console' })).toBeInTheDocument()
		expect(screen.getByText(/not a freeform root shell/i)).toBeInTheDocument()
		expect(screen.getByText('Test nginx configuration')).toBeInTheDocument()
	})
})
