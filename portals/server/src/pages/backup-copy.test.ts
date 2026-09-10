import { describe, expect, test } from 'vitest'
import { backupDestinationReadiness, backupScopeSummary } from './backup-copy'

describe('backupDestinationReadiness', () => {
	test('marks local as ready and remote destinations as needing host credentials', () => {
		expect(backupDestinationReadiness('local').ready).toBe(true)
		expect(backupDestinationReadiness('sftp').ready).toBe(false)
		expect(backupDestinationReadiness('s3').ready).toBe(false)
		expect(backupDestinationReadiness('sftp').detail.toLocaleLowerCase()).toContain('credential')
	})
})

describe('backupScopeSummary', () => {
	test('explains included data, retention, and host-held encryption', () => {
		const copy = backupScopeSummary(14)
		expect(copy.toLocaleLowerCase()).toContain('website files')
		expect(copy).toContain('14 days')
		expect(copy.toLocaleLowerCase()).toContain('encrypt')
		expect(copy.toLocaleLowerCase()).toContain('host')
	})
})
