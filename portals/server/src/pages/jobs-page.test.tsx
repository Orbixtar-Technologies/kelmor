import { describe, expect, it } from 'vitest'
import { canRetryJob } from './jobs-page'
import type { Job } from '../types'

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
