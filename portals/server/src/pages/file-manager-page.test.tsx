// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { FileManagerPage } from './file-manager-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('FileManagerPage', () => {
	test('chmods a file through the real files PATCH API', async () => {
		const user = userEvent.setup()
		vi.spyOn(window, 'prompt').mockReturnValue('0640')
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (String(path) === '/api/v1/accounts') {
				return Promise.resolve({ items: [{ id: 'acc-1', username: 'shop', primary_domain: 'shop.test', home_path: '/home/shop' }] })
			}
			if (String(path).includes('/files') && options?.method === 'PATCH') return Promise.resolve({ ok: true })
			if (String(path).includes('/files')) return Promise.resolve({ items: [{ name: 'index.html', size: 12, dir: false }], path: '/' })
			return Promise.resolve({ items: [] })
		})
		render(
			<MemoryRouter initialEntries={['/files?account=acc-1']}>
				<CapProvider caps={{ 'files.read': true, 'files.write': true }}>
					<Routes>
						<Route path="files" element={<FileManagerPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)
		expect(await screen.findByRole('button', { name: /index.html/ })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'chmod' }))
		await waitFor(() => {
			expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/files', expect.objectContaining({
				method: 'PATCH',
				body: JSON.stringify({ path: '/index.html', mode: '0640' }),
			}))
		})
	})
})
