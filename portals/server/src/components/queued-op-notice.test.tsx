// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { jobsHref, queuedOpMessage, QueuedOpNotice } from './queued-op-notice'

const api = vi.fn()

vi.mock('../client', () => ({
	api: (...args: unknown[]) => api(...args),
}))

afterEach(() => {
	cleanup()
	api.mockReset()
})

describe('queued operation notice', () => {
	test('builds a jobs deep-link and mentions the operation id', () => {
		expect(jobsHref('acc-1', 'job-9')).toBe('/jobs?account=acc-1&selected=job-9')
		expect(queuedOpMessage({ operation_id: 'job-9' }, 'Mailbox creation queued.')).toBe('Mailbox creation queued. Job job-9.')
		expect(queuedOpMessage(undefined, 'Saved.')).toBe('Saved.')
	})

	test('polls job status and links to the jobs page', async () => {
		api.mockResolvedValue({
			id: 'job-9',
			type: 'mail.mailbox',
			payload: {},
			state: 'running',
			priority: 0,
			attempts: 1,
			max_attempts: 3,
			progress: 40,
			run_after: '',
			created_at: '2026-09-19T00:00:00Z',
		})
		render(
			<MemoryRouter>
				<QueuedOpNotice message="Mailbox creation queued. Job job-9." accountId="acc-1" jobId="job-9" />
			</MemoryRouter>,
		)
		expect(screen.getByRole('link', { name: 'Open Jobs' })).toHaveAttribute('href', '/jobs?account=acc-1&selected=job-9')
		await waitFor(() => {
			expect(screen.getByText(/Job is running/)).toBeInTheDocument()
		})
		expect(api).toHaveBeenCalledWith('/api/v1/jobs/job-9')
	})
})
