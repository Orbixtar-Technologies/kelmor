import { describe, expect, it } from 'vitest'
import {
	pendingPhpChanges,
	stageSelectedPhpVersions,
	websitePhpVersion,
} from './multiphp-staging'

const alpha = {
	id: 'site-1',
	account_id: 'acc-1',
	account_username: 'alpha',
	domain_id: 'dom-1',
	document_root: '/home/alpha/public_html',
	runtime: 'php',
	runtime_version: '8.3',
}

const blog = {
	id: 'site-2',
	account_id: 'acc-1',
	account_username: 'alpha',
	domain_id: 'dom-2',
	document_root: '/home/alpha/blog.alpha.test',
	runtime: 'php',
	runtime_version: '8.3',
}

describe('pendingPhpChanges', () => {
	it('ignores selector drafts that still match the live version', () => {
		expect(websitePhpVersion(alpha)).toBe('8.3')
		expect(pendingPhpChanges([alpha, blog], { 'site-1': '8.3' })).toEqual([])
	})

	it('returns current to proposed diffs for staged PHP versions', () => {
		expect(pendingPhpChanges([alpha, blog], { 'site-1': '8.4', 'site-2': '8.5' })).toEqual([
			{
				websiteId: 'site-1',
				accountId: 'acc-1',
				domainId: 'dom-1',
				documentRoot: '/home/alpha/public_html',
				accountUsername: 'alpha',
				runtime: 'php',
				current: '8.3',
				proposed: '8.4',
			},
			{
				websiteId: 'site-2',
				accountId: 'acc-1',
				domainId: 'dom-2',
				documentRoot: '/home/alpha/blog.alpha.test',
				accountUsername: 'alpha',
				runtime: 'php',
				current: '8.3',
				proposed: '8.5',
			},
		])
	})
})

describe('stageSelectedPhpVersions', () => {
	it('stages only checked PHP sites', () => {
		const staticSite = {
			...blog,
			id: 'site-3',
			runtime: 'static',
			runtime_version: '',
		}
		expect(stageSelectedPhpVersions(
			[alpha, blog, staticSite],
			{ 'site-1': true, 'site-3': true },
			'8.4',
			{},
		)).toEqual({ 'site-1': '8.4' })
	})
})
