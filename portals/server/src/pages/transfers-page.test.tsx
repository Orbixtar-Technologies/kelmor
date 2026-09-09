// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { CapProvider } from '../rbac'
import { TransfersPage } from './transfers-page'

vi.mock('../client', () => ({
	api: vi.fn().mockResolvedValue({ items: [] }),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(cleanup)

describe('TransfersPage capabilities', () => {
	test('hides native host import from a reseller while retaining export and account copy', () => {
		render(
			<MemoryRouter>
				<CapProvider caps={{ 'accounts.read': true, 'accounts.create': true }}>
					<TransfersPage />
				</CapProvider>
			</MemoryRouter>,
		)

		expect(screen.getByText('Native account export')).toBeInTheDocument()
		expect(screen.getByText('Copy existing account')).toBeInTheDocument()
		expect(screen.queryByText('Native account import')).not.toBeInTheDocument()
	})

	test('shows native host import when account creation and server read are granted', () => {
		render(
			<MemoryRouter>
				<CapProvider caps={{ 'accounts.create': true, 'server.read': true }}>
					<TransfersPage />
				</CapProvider>
			</MemoryRouter>,
		)

		expect(screen.getByText('Native account import')).toBeInTheDocument()
	})
})
