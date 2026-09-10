// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test } from 'vitest'
import { SecretValue } from './ui'

afterEach(cleanup)

describe('SecretValue', () => {
	test('masks a credential until it is explicitly revealed', async () => {
		const user = userEvent.setup()
		render(<SecretValue value="super-secret" />)

		expect(screen.queryByText('super-secret')).not.toBeInTheDocument()
		expect(screen.getByText('••••••••')).toBeInTheDocument()
		await user.click(screen.getByRole('button', { name: 'Reveal password' }))
		expect(screen.getByText('super-secret')).toBeInTheDocument()
	})
})
