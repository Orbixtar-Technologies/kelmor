import { describe, expect, test } from 'vitest'
import {
	accountDatabasePrefix,
	accountHomePath,
	bandwidthEnforcementCopy,
	isolationLines,
	lifecycleImpact,
	provisionPipelineSteps,
	quotaEnforcementCopy,
} from './account-lifecycle-copy'

describe('account lifecycle copy', () => {
	test('derives home and database prefix from the username', () => {
		expect(accountHomePath('acme42')).toBe('/home/acme42')
		expect(accountDatabasePrefix('acme42')).toBe('acme42_')
	})

	test('describes the provision pipeline without inventing cPanel registry files', () => {
		const steps = provisionPipelineSteps('acme42', 'acme.test')
		expect(steps[0]).toMatch(/Linux user acme42/)
		expect(steps.some((step) => step.includes('/home/acme42/public_html'))).toBe(true)
		expect(steps.some((step) => step.includes('www'))).toBe(true)
		expect(steps.some((step) => step.includes('acme42_db'))).toBe(true)
		expect(steps.join(' ')).not.toMatch(/\/var\/cpanel/)
	})

	test('explains POSIX isolation and the SFTP jail', () => {
		const lines = isolationLines({
			username: 'acme42',
			linux_uid: 20010,
			linux_gid: 20010,
			home_path: '/home/acme42',
			shell_class: 'sftp-only',
		})
		expect(lines[0]).toMatch(/UID 20010/)
		expect(lines.join(' ')).toMatch(/chroot/)
		expect(lines.join(' ')).toMatch(/acme42_/)
	})

	test('lists unequal lifecycle impact', () => {
		expect(lifecycleImpact('suspend').join(' ')).toMatch(/503/)
		expect(lifecycleImpact('terminate').join(' ')).toMatch(/cannot be undone/)
		expect(bandwidthEnforcementCopy()).toMatch(/509/)
		expect(quotaEnforcementCopy()).toMatch(/setquota/)
	})
})
