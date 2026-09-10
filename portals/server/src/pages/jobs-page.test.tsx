// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { canRetryJob, JobsPage } from './jobs-page'
import type { Job } from '../types'

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

const failedJob: Job = {
	id: 'job-1',
	type: 'website.provision',
	resource_type: 'website',
	resource_id: 'site-1',
	payload: {},
	state: 'failed',
	priority: 0,
	attempts: 1,
	max_attempts: 5,
	progress: 0,
	run_after: '2026-09-09T00:00:00Z',
	created_at: '2026-09-09T00:00:00Z',
}

describe('canRetryJob', () => {
	it('hides retry when the API marks a job ineligible', () => {
		expect(canRetryJob({ ...failedJob, retryable: false }, { 'websites.write': true })).toBe(false)
	})

	it('allows an eligible job when the capability is present', () => {
		expect(canRetryJob({ ...failedJob, retryable: true }, { 'websites.write': true })).toBe(true)
	})
})

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('JobsPage account scope', () => {
	it('keeps card totals aligned with the account-filtered list', async () => {
		vi.mocked(api).mockResolvedValue({
			items: [
				{ ...failedJob, id: 'job-acc', payload: { account_id: 'acc-1' }, last_error: '{"error":"dns authorization failed"}', state: 'failed' },
				{ ...failedJob, id: 'job-other', payload: { account_id: 'other' }, state: 'succeeded', last_error: undefined },
				{ ...failedJob, id: 'job-acc-ok', payload: { account_id: 'acc-1' }, state: 'succeeded', last_error: undefined },
			],
		})

		render(
			<MemoryRouter initialEntries={['/jobs?account=acc-1']}>
				<CapProvider caps={{ 'accounts.read': true, 'websites.write': true }}>
					<Routes>
						<Route path="/jobs" element={<JobsPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		await waitFor(() => {
			expect(screen.getByText(/Showing jobs for this account/)).toBeInTheDocument()
		})
		expect(screen.getByRole('link', { name: 'Return to account' })).toHaveAttribute('href', '/accounts/acc-1')
		expect(screen.getAllByText('Create website').length).toBeGreaterThan(0)
		expect(screen.getAllByText('dns authorization failed').length).toBeGreaterThan(0)
		expect(screen.queryByText('{"error":"dns authorization failed"}')).not.toBeInTheDocument()
		expect(screen.getByLabelText('Account job totals')).toHaveTextContent('Succeeded1')
		expect(screen.getByLabelText('Account job totals')).toHaveTextContent('Failed1')
	})

	it('keeps recovery actions visible and summarizes earlier log failures', async () => {
		vi.mocked(api).mockResolvedValue({
			items: [{
				...failedJob,
				payload: { account_id: 'acc-1' },
				started_at: '2026-09-09T00:01:00Z',
				finished_at: '2026-09-09T00:02:00Z',
				attempts: 2,
				logs: ['dns authorization failed', 'missing website resource'],
				last_error: 'missing website resource',
			}],
		})

		render(
			<MemoryRouter initialEntries={['/jobs']}>
				<CapProvider caps={{ 'websites.write': true }}>
					<Routes>
						<Route path="/jobs" element={<JobsPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		await waitFor(() => {
			expect(screen.getAllByRole('button', { name: 'Details' }).length).toBeGreaterThan(0)
		})
		screen.getAllByRole('button', { name: 'Details' })[0].click()
		expect(await screen.findByText('Earlier failure')).toBeInTheDocument()
		expect(screen.getByText('Latest error')).toBeInTheDocument()
		expect(screen.getByText(/named resource exists/i)).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Retry failed job' })).toBeInTheDocument()
		expect(screen.getByText('Technical details')).toBeInTheDocument()
	})
})
