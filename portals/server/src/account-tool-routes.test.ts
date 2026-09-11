import { describe, expect, test } from 'vitest'
import { canonicalAccountToolPath, isHubAccountService, sslToolPath } from './account-tool-routes'

describe('canonical account tools', () => {
	test('sends hub resources to their first-class manager, never through Account Services', () => {
		expect(canonicalAccountToolPath('certificates', 'acc-1')).toBe('/ssl?account=acc-1')
		expect(canonicalAccountToolPath('databases', 'acc-1')).toBe('/sql?account=acc-1')
		expect(canonicalAccountToolPath('mailboxes', 'acc-1')).toBe('/email?account=acc-1&tab=mailboxes')
		expect(canonicalAccountToolPath('mail-domains', 'acc-1')).toBe('/email?account=acc-1&tab=domains')
		expect(canonicalAccountToolPath('websites', 'acc-1')).toBe('/websites?account=acc-1')
		expect(canonicalAccountToolPath('domains', 'acc-1')).toBe('/domains?account=acc-1')
		expect(canonicalAccountToolPath('files', 'acc-1')).toBe('/files?account=acc-1')
		expect(canonicalAccountToolPath('cron', 'acc-1')).toBe('/cron?account=acc-1')
		expect(canonicalAccountToolPath('ftp', 'acc-1')).toBe('/ftp?account=acc-1')
	})

	test('keeps leftover access tools on Account Services', () => {
		expect(canonicalAccountToolPath('ssh', 'acc-1')).toBe('/accounts/acc-1/services?service=ssh')
		expect(canonicalAccountToolPath('tokens', 'acc-1')).toBe('/accounts/acc-1/services?service=tokens')
		expect(canonicalAccountToolPath('applications', 'acc-1')).toBe('/accounts/acc-1/services?service=applications')
		expect(canonicalAccountToolPath('backups', 'acc-1')).toBe('/accounts/acc-1/services?service=backups')
	})

	test('marks only dedicated hubs as redirect sources', () => {
		expect(isHubAccountService('certificates')).toBe(true)
		expect(isHubAccountService('ssh')).toBe(false)
		expect(isHubAccountService('backups')).toBe(false)
	})

	test('keeps SSL family tasks on the SSL manager', () => {
		expect(sslToolPath('acc-1', 'request')).toBe('/ssl?account=acc-1&task=request')
		expect(sslToolPath('acc-1', 'status')).toBe('/ssl?account=acc-1&task=status')
	})
})
