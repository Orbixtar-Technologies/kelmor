// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { TrackDeliveryPage } from './track-delivery-page'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

function renderTrack (path = '/mail/track-delivery') {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<CapProvider caps={{ 'mail.read': true }}>
				<Routes>
					<Route path="mail/track-delivery" element={<TrackDeliveryPage />} />
					<Route path="jobs" element={<p>Jobs dump</p>} />
				</Routes>
			</CapProvider>
		</MemoryRouter>,
	)
}

describe('TrackDeliveryPage', () => {
	test('tracks a recipient without redirecting to Jobs', async () => {
		const user = userEvent.setup()
		api.mockImplementation((path: string) => {
			if (String(path).startsWith('/api/v1/mail/delivery-track')) {
				return Promise.resolve({
					mode: 'track',
					source: '/var/log/mail.log',
					queue: [{
						queue_id: 'ABC999',
						sender: 'shop@shop.test',
						recipient: 'user@example.com',
						arrival: 'Sat Sep 19 04:20:01',
						state: 'queued',
					}],
					items: [{
						id: 'DEF456',
						queue_id: 'DEF456',
						timestamp: '2026-09-19T04:15:00Z',
						sender: 'shop@shop.test',
						recipient: 'user@example.com',
						status: 'deferred',
						message: 'connect timed out',
						direction: 'outbound',
					}],
				})
			}
			return Promise.resolve({ items: [] })
		})
		renderTrack()
		expect(screen.getByRole('heading', { name: 'Track Delivery' })).toBeInTheDocument()
		expect(screen.queryByText('Jobs dump')).not.toBeInTheDocument()
		await user.type(screen.getByLabelText('Recipient or sender'), 'user@example.com')
		await user.click(screen.getByRole('button', { name: 'Track delivery' }))
		expect(await screen.findByText('connect timed out')).toBeInTheDocument()
		expect(screen.getByText('ABC999')).toBeInTheDocument()
		expect(api).toHaveBeenCalledWith('/api/v1/mail/delivery-track?q=user%40example.com')
		expect(api).not.toHaveBeenCalledWith(expect.stringContaining('/api/v1/jobs'))
		const reportLinks = screen.getAllByRole('link', { name: 'Mail Delivery Reports' })
		expect(reportLinks.length).toBeGreaterThan(0)
		for (const link of reportLinks) {
			expect(link).toHaveAttribute('href', expect.stringContaining('/mail/delivery-reports'))
		}
	})
})
