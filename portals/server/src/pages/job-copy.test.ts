import { describe, expect, test } from 'vitest'
import { describeJobFailure, describeJobTimeline, formatJobType, jobMatchesAccount, jobRecoveryGuidance, summarizeJobCounts } from './job-copy'
import type { Job } from '../types'

function job (overrides: Partial<Job> = {}): Job {
	return {
		id: 'job-1',
		type: 'website.provision',
		resource_type: 'website',
		resource_id: 'site-1',
		payload: { account_id: 'acc-1', hostname: 'shop.example.com' },
		state: 'failed',
		priority: 0,
		attempts: 1,
		max_attempts: 5,
		progress: 0,
		run_after: '2026-09-10T00:00:00Z',
		created_at: '2026-09-10T00:00:00Z',
		...overrides,
	}
}

describe('formatJobType', () => {
	test('uses a plain-language label for known operations', () => {
		expect(formatJobType('website.provision')).toBe('Create website')
		expect(formatJobType('account.reconcile')).toBe('Apply account settings')
		expect(formatJobType('database.create')).toBe('Create database')
	})
})

describe('describeJobFailure', () => {
	test('prefers a readable reason and hides raw JSON in the summary', () => {
		const described = describeJobFailure(job({
			last_error: '{"error":"dns authorization failed for mail.shop.example.com","code":"dns_auth"}',
		}))
		expect(described.title).toBe('Create website')
		expect(described.resource).toContain('shop.example.com')
		expect(described.reason.toLocaleLowerCase()).toContain('dns')
		expect(described.reason).not.toMatch(/\{/)
		expect(described.technical).toContain('dns_auth')
	})

	test('keeps a non-JSON error readable', () => {
		const described = describeJobFailure(job({ last_error: 'missing website resource' }))
		expect(described.reason).toBe('missing website resource')
	})
})

describe('jobMatchesAccount', () => {
	test('matches resource or payload account ids', () => {
		expect(jobMatchesAccount(job(), 'acc-1')).toBe(true)
		expect(jobMatchesAccount(job({ resource_id: 'acc-1', payload: {} }), 'acc-1')).toBe(true)
		expect(jobMatchesAccount(job(), 'other')).toBe(false)
	})
})

describe('describeJobTimeline', () => {
	test('orders queued, started, later log failures, and the latest error', () => {
		const entries = describeJobTimeline(job({
			attempts: 2,
			started_at: '2026-09-10T00:01:00Z',
			finished_at: '2026-09-10T00:02:00Z',
			logs: [
				'dns authorization failed for mail.shop.example.com',
				'missing website resource',
			],
			last_error: 'missing website resource',
		}))
		expect(entries.map((entry) => entry.label)).toEqual([
			'Queued',
			'Started',
			'Failed',
			'Attempts',
			'Earlier failure',
			'Latest error',
		])
		expect(entries[4].detail).toContain('dns authorization')
		expect(entries[5].detail).toBe('missing website resource')
	})
})

describe('jobRecoveryGuidance', () => {
	test('asks operators to recreate a missing resource before retry', () => {
		expect(jobRecoveryGuidance(job({ last_error: 'missing website resource' }))).toMatch(/exists|create/i)
	})
})

describe('summarizeJobCounts', () => {
	test('counts only the provided rows so cards match the visible list', () => {
		const counts = summarizeJobCounts([
			job({ id: '1', state: 'succeeded' }),
			job({ id: '2', state: 'succeeded' }),
			job({ id: '3', state: 'failed' }),
			job({ id: '4', state: 'queued' }),
		])
		expect(counts).toEqual({ queued: 1, running: 0, succeeded: 2, failed: 1, total: 4 })
	})
})
