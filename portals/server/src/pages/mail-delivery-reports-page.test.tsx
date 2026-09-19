// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { MailDeliveryReportsPage } from './mail-delivery-reports-page'
import { JobsPage } from './jobs-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderReports (path = '/mail/delivery-reports') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'mail.read': true, 'accounts.read': true }}>
				<Routes>
					<Route path="mail/delivery-reports" element={<MailDeliveryReportsPage />} />
					<Route path="jobs" element={<JobsPage />} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('MailDeliveryReportsPage', () => {
	test('runs a month/day/year report and does not open Jobs', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path).startsWith('/api/v1/mail/delivery-reports')) {
				return Promise.resolve({
					mode: 'report',
					date: '2026-09-19',
					source: '/var/log/mail.log',
					items: [{
						id: 'ABC123:user@example.com',
						queue_id: 'ABC123',
						timestamp: '2026-09-19T04:12:01Z',
						sender: 'shop@shop.test',
						recipient: 'user@example.com',
						status: 'delivered',
						message: '250 2.0.0 OK',
						relay: 'mx.example.com',
						dsn: '2.0.0',
						direction: 'outbound',
					}],
				})
			}
			return Promise.resolve({ items: [] })
		})
		renderReports()
		expect(screen.getByRole('heading', { name: 'Mail Delivery Reports' })).toBeInTheDocument()
		expect(screen.queryByRole('heading', { name: 'Jobs' })).not.toBeInTheDocument()
		await user.selectOptions(screen.getByLabelText('Month'), '9')
		await user.selectOptions(screen.getByLabelText('Day of month'), '19')
		await user.clear(screen.getByLabelText('Year'))
		await user.type(screen.getByLabelText('Year'), '2026')
		await user.click(screen.getByRole('button', { name: 'Run report' }))
		expect(await screen.findByText('user@example.com')).toBeInTheDocument()
		expect(screen.getByText('Delivered')).toBeInTheDocument()
		expect(api).toHaveBeenCalledWith('/api/v1/mail/delivery-reports?year=2026&month=9&day=19')
		expect(api).not.toHaveBeenCalledWith(expect.stringContaining('/api/v1/jobs'))
		await user.click(screen.getByRole('button', { name: 'Details' }))
		expect(screen.getByRole('article', { name: 'Delivery attempt details' })).toHaveTextContent('250 2.0.0 OK')
		expect(screen.getByRole('link', { name: 'Track Delivery' })).toHaveAttribute('href', '/mail/track-delivery')
	})
})
