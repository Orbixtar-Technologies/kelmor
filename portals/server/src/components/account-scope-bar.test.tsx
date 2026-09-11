// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { AccountScopeBar } from './account-scope-bar'
import type { Account } from '../types'

const accounts: Account[] = [{
	id: 'acc-1',
	owner_user_id: 'u1',
	username: 'usmanliaqat',
	primary_domain: 'orbixtar.dpdns.org',
	linux_uid: 1001,
	linux_gid: 1001,
	package_id: 'pkg',
	status: 'active',
	home_path: '/home/usmanliaqat',
	shell_class: 'nologin',
	login_disabled: false,
	desired_revision: 1,
	observed_revision: 1,
}]

afterEach(cleanup)

describe('AccountScopeBar', () => {
	test('keeps readable copy and themed account controls', () => {
		render(
			<MemoryRouter>
				<AccountScopeBar accountId="acc-1" accounts={accounts} toolLabel="Files" onChange={vi.fn()} />
			</MemoryRouter>,
		)

		expect(screen.getByText('Files for usmanliaqat · orbixtar.dpdns.org.')).toBeInTheDocument()
		expect(screen.getByText('Change account')).toBeInTheDocument()
		expect(screen.getByRole('combobox', { name: 'Change account' })).toHaveValue('acc-1')
		expect(screen.getByRole('link', { name: 'Return to account' })).toHaveAttribute('href', '/accounts/acc-1')
	})
})
