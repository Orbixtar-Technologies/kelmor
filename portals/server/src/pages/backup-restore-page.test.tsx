// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { BackupRestorePage } from './backup-restore-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderRestore (caps: Record<string, boolean> = {
	'accounts.read': true,
	'backups.read': true,
	'backups.restore': true,
	'backups.create': true,
}) {
	return render(
		<MemoryRouter initialEntries={['/transfers/restore']}>
			<CapProvider caps={caps}>
				<Routes>
					<Route path="transfers/restore" element={<BackupRestorePage />} />
					<Route path="transfers" element={<p>Transfers hub</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('BackupRestorePage', () => {
	test('shows an honest empty inventory instead of a 0 shell', async () => {
		api.mockResolvedValue({ items: [] })
		renderRestore()
		expect(await screen.findByRole('heading', { name: 'Backup Restoration' })).toBeInTheDocument()
		expect(screen.getByText('No backups available to restore')).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'Configure or run backups' })).toHaveAttribute('href', '/transfers')
		expect(screen.getByRole('columnheader', { name: 'Archive' })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
		expect(screen.queryByText(/Backups & restore · 0/)).not.toBeInTheDocument()
	})

	test('lists restorable archives and queues an in-place restore job', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string, options?: { method?: string }) => {
			if (path === '/api/v1/backups') {
				return Promise.resolve({
					items: [
						{
							id: 'bak-ok',
							account_id: 'acc-1',
							account_username: 'shop',
							primary_domain: 'shop.test',
							kind: 'full',
							state: 'succeeded',
							destination: 'local',
							size_bytes: 2048,
							checksum: 'abc',
							scope: 'account',
							restorable: true,
							created_at: '2026-09-20T00:00:00Z',
						},
						{
							id: 'bak-queued',
							account_id: 'acc-1',
							account_username: 'shop',
							kind: 'full',
							state: 'queued',
							destination: 'local',
							scope: 'account',
							restorable: false,
						},
					],
				})
			}
			if (path === '/api/v1/accounts/acc-1/restores' && options?.method === 'POST') {
				return Promise.resolve({ operation_id: 'job-restore-1' })
			}
			return Promise.resolve({ items: [] })
		})
		renderRestore()
		expect((await screen.findAllByText('shop')).length).toBeGreaterThan(0)
		expect(screen.getByText('bak-ok')).toBeInTheDocument()
		expect(screen.getByText('queued')).toBeInTheDocument()
		await user.click(screen.getByRole('radio', { name: 'Select bak-ok for shop' }))
		await user.click(screen.getByRole('button', { name: 'Continue' }))
		expect(screen.getByRole('heading', { name: 'Review restore' })).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Queue restore' }))
		expect(api).toHaveBeenCalledWith('/api/v1/accounts/acc-1/restores', expect.objectContaining({
			method: 'POST',
			body: JSON.stringify({ backup_id: 'bak-ok', mode: 'in_place' }),
		}))
		expect(await screen.findByText(/Job job-restore-1/)).toBeInTheDocument()
	})

	test('keeps system archives visible but not selectable', async () => {
		api.mockResolvedValue({
			items: [{
				id: 'sys-1',
				kind: 'system',
				state: 'succeeded',
				destination: 'local',
				scope: 'system',
				restorable: false,
				restore_note: 'System archives are listed from inventory. In-place restore requires an account-scoped archive.',
			}],
		})
		renderRestore()
		expect(await screen.findByText('sys-1')).toBeInTheDocument()
		expect(screen.getByText(/System archives are listed/)).toBeInTheDocument()
		expect(screen.getByRole('radio', { name: 'Select sys-1 for system' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
	})
})
