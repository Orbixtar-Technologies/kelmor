// @vitest-environment jsdom
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { ResellersPage } from './resellers-page'

const { reseller } = vi.hoisted(() => ({
	reseller: {
		id: 'reseller-1',
		user_id: 'user-1',
		name: 'Restricted',
		privilege_mask: ['accounts.read', 'dns.read'],
		nameservers: [],
		status: 'active',
	},
}))

vi.mock('../client', () => ({
	api: vi.fn().mockResolvedValue({ items: [reseller] }),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(cleanup)

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

describe('ResellersPage privilege masks', () => {
	test('checks every reseller-safe capability by default on create', async () => {
		const user = userEvent.setup()
		render(<CapProvider caps={{ 'resellers.create': true }}><ResellersPage /></CapProvider>)
		await user.click(await screen.findByRole('button', { name: 'Create reseller' }))
		const dialog = screen.getByRole('dialog')
		const inputs = privilegeInputs(dialog)

		expect(inputs.map((input) => input.value)).toEqual(expectedPrivileges)
		expect(inputs.every((input) => input.checked)).toBe(true)
	})

	test('reflects only the stored capabilities when editing', async () => {
		const user = userEvent.setup()
		render(<CapProvider caps={{ 'resellers.modify': true }}><ResellersPage /></CapProvider>)
		await user.click(await screen.findByRole('button', { name: 'Edit' }))
		const inputs = privilegeInputs(screen.getByRole('dialog'))

		await waitFor(() => expect(inputs.filter((input) => input.checked).map((input) => input.value)).toEqual(reseller.privilege_mask))
	})
})
