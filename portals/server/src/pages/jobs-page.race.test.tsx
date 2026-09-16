// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { CapProvider } from '../rbac'
import { JobsPage } from './jobs-page'
import type { Job } from '../types'

function deferred<T> () {
	let resolve!: (value: T) => void
	const promise = new Promise<T>((next) => { resolve = next })
	return { promise, resolve }
}

vi.mock('../client', () => ({
	api: vi.fn(),
	asList: (value: { items?: unknown[] }) => value.items || [],
}))

import { api } from '../client'

const baseJob: Job = {
	id: 'job-1',
	type: 'website.provision',
	resource_type: 'website',
	resource_id: 'site-1',
	payload: {},
	state: 'queued',
	priority: 0,
	attempts: 0,
	max_attempts: 5,
	progress: 0,
	run_after: '2026-09-09T00:00:00Z',
	created_at: '2026-09-09T00:00:00Z',
}

afterEach(() => {
	cleanup()
	vi.clearAllMocks()
})

describe('JobsPage request races', () => {
	it('commits only the newest job-filter request when older work resolves last', async () => {
		const first = deferred<{ items: Job[] }>()
		const second = deferred<{ items: Job[] }>()
		vi.mocked(api).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)

		render(
			<MemoryRouter initialEntries={['/jobs']}>
				<CapProvider caps={{ 'accounts.read': true }}>
					<Routes>
						<Route path="/jobs" element={<JobsPage />} />
					</Routes>
				</CapProvider>
			</MemoryRouter>,
		)

		const user = userEvent.setup()
		await waitFor(() => expect(api).toHaveBeenCalledTimes(1))
		await user.selectOptions(screen.getByLabelText('State'), 'failed')
		await waitFor(() => expect(api).toHaveBeenCalledTimes(2))

		second.resolve({ items: [{ ...baseJob, id: 'failed-job', state: 'failed', last_error: 'latest filter' }] })
		await screen.findAllByText('latest filter')

		first.resolve({ items: [{ ...baseJob, id: 'stale-job', state: 'queued', last_error: 'stale filter' }] })
		await waitFor(() => {
			expect(screen.queryByText('stale filter')).not.toBeInTheDocument()
		})
		expect(screen.getAllByText('latest filter').length).toBeGreaterThan(0)
	})
})
