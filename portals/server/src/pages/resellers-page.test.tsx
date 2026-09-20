// @vitest-environment jsdom
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { ResellersPage } from './resellers-page'

const { reseller } = vi.hoisted(() => ({
	reseller: {
		id: 'reseller-1',
		user_id: 'user-1',
		name: 'Restricted',
		brand_name: 'Restricted Host',
		privilege_mask: ['accounts.read', 'dns.read'],
		nameservers: ['ns1.localhost'],
		status: 'active',
		accounts: 2,
		active: 1,
		suspended: 1,
		disk_bytes: 1024,
		disk_limit: 2048,
		bandwidth_bytes: 0,
		bandwidth_limit: 8192,
		packages: ['Starter'],
	},
}))

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

const expectedPrivileges = [
	'accounts.read',
	'accounts.create',
	'accounts.modify',
	'accounts.suspend',
	'packages.read',
	'packages.write',
	'domains.read',
	'domains.write',
	'dns.read',
	'websites.read',
	'backups.read',
	'backups.create',
	'backups.restore',
	'billing.usage.read',
]

function privilegeInputs (dialog: HTMLElement) {
	return within(dialog).getAllByRole('checkbox') as HTMLInputElement[]
}

function renderResellers (caps: Record<string, boolean> = {
	'resellers.create': true,
	'resellers.modify': true,
}) {
	return render(
		<MemoryRouter>
			<CapProvider caps={caps}>
				<ResellersPage />
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('ResellersPage privilege masks', () => {
	test('checks every reseller-safe capability by default on create', async () => {
		const user = userEvent.setup()
		api.mockResolvedValue({ items: [reseller] })
		renderResellers({ 'resellers.create': true })
		await user.click(await screen.findByRole('button', { name: 'Create reseller' }))
		const dialog = screen.getByRole('dialog')
		const inputs = privilegeInputs(dialog)

		expect(inputs.map((input) => input.value)).toEqual(expectedPrivileges)
		expect(inputs.every((input) => input.checked)).toBe(true)
	})

	test('reflects only the stored capabilities when editing', async () => {
		const user = userEvent.setup()
		api.mockResolvedValue({ items: [reseller] })
		renderResellers({ 'resellers.modify': true })
		await user.click(await screen.findByRole('button', { name: 'Edit' }))
		const inputs = privilegeInputs(screen.getByRole('dialog'))

		await waitFor(() => expect(inputs.filter((input) => input.checked).map((input) => input.value)).toEqual(reseller.privilege_mask))
	})
})

describe('ResellersPage manager', () => {
	test('keeps table chrome and create when the list is empty', async () => {
		api.mockResolvedValue({ items: [] })
		renderResellers()
		expect(await screen.findByRole('heading', { name: 'Edit Reseller Nameservers and Privileges' })).toBeInTheDocument()
		expect(screen.getByRole('columnheader', { name: 'Reseller' })).toBeInTheDocument()
		expect(screen.getByRole('columnheader', { name: 'Accounts' })).toBeInTheDocument()
		expect(screen.getByRole('columnheader', { name: 'Package / limits' })).toBeInTheDocument()
		expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument()
		expect(screen.getByText('No resellers yet')).toBeInTheDocument()
		expect(screen.getByText(/Create a reseller to delegate packages/)).toBeInTheDocument()
		expect(screen.getAllByRole('button', { name: 'Create reseller' }).length).toBeGreaterThan(0)
		expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument()
	})

	test('lists resellers with usage columns and manager actions', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (path === '/api/v1/resellers') return Promise.resolve({ items: [reseller] })
			if (path === '/api/v1/resellers/reseller-1') {
				return Promise.resolve({
					reseller,
					accounts: [{ id: 'acc-1', username: 'shop' }],
					packages: [{ id: 'pkg-1', name: 'Starter' }],
				})
			}
			return Promise.resolve({ items: [] })
		})
		renderResellers()
		expect(await screen.findByText('Restricted')).toBeInTheDocument()
		expect(screen.getByText('reseller-1')).toBeInTheDocument()
		expect(screen.getByText(/2 · 1 active · 1 suspended/)).toBeInTheDocument()
		expect(screen.getByText('Starter')).toBeInTheDocument()
		expect(screen.getByLabelText(/Disk/)).toBeInTheDocument()
		expect(screen.getByRole('link', { name: 'View usage' })).toHaveAttribute('href', '/resellers/usage?reseller=reseller-1')
		await user.click(screen.getByRole('button', { name: 'Manage' }))
		expect(api).toHaveBeenCalledWith('/api/v1/resellers/reseller-1')
		expect(await screen.findByText('shop')).toBeInTheDocument()
	})
})
