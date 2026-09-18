// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { ControlHubPage } from './control-hub-page'
import { controlHubs } from './control-hubs'

afterEach(() => {
	cleanup()
})

const websites = controlHubs.find((hub) => hub.id === 'websites')!

describe('ControlHubPage', () => {
	test('shows website and SSL tools on one page', () => {
		render(
			<MemoryRouter initialEntries={['/websites']}>
				<ControlHubPage
					hub={websites}
					capabilities={{ 'websites.read': true }}
					pages={{ sites: <p>Sites panel</p>, ssl: <p>SSL panel</p> }}
				/>
			</MemoryRouter>,
		)
		expect(screen.getByRole('link', { name: 'Sites' })).toHaveAttribute('aria-current', 'page')
		expect(screen.getByRole('link', { name: 'SSL/TLS' })).toHaveAttribute('href', '/websites?tab=ssl')
		expect(screen.getByText('Sites panel')).toBeInTheDocument()
	})
})
