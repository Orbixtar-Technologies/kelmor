import { describe, expect, test } from 'vitest'
import { fileKindLabel, isProtectedName, isProtectedPath, protectedPathWarning } from './file-path-guards'

describe('file path guards', () => {
	test('marks ssh, backups, logs, mail, and panel metadata as protected', () => {
		expect(isProtectedName('.ssh')).toBe(true)
		expect(isProtectedName('backups')).toBe(true)
		expect(isProtectedName('.panel-database.mariadb')).toBe(true)
		expect(isProtectedName('public_html')).toBe(false)
		expect(isProtectedPath('/.ssh', 'id_rsa')).toBe(true)
	})

	test('warns only inside protected paths', () => {
		expect(protectedPathWarning('/public_html')).toBe('')
		expect(protectedPathWarning('/.ssh')).toMatch(/system-managed/)
		expect(fileKindLabel('.ssh', true)).toBe('Protected directory')
	})
})
