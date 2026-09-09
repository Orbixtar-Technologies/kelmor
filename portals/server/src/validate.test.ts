import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
	validateCreateAccount,
	validateDomain,
	validateUsername,
} from './validate.ts'

test('username matches control-plane rules', () => {
	assert.equal(validateUsername('ab'), '')
	assert.equal(validateUsername('a'), 'Username must be 2-32 characters')
	assert.equal(validateUsername('1abc'), 'Username must be lowercase alphanumeric and start with a letter')
	assert.equal(validateUsername('Ada'), 'Username must be lowercase alphanumeric and start with a letter')
	assert.equal(validateUsername('root'), 'Reserved username')
	assert.equal(validateUsername('kelmor'), 'Reserved username')
})

test('domain requires two labels', () => {
	assert.equal(validateDomain('uitest.test'), '')
	assert.equal(validateDomain('localhost'), 'Domain requires at least two labels')
	assert.equal(validateDomain('bad domain.test'), 'Invalid domain characters')
})

test('create account gathers field errors', () => {
	const errors = validateCreateAccount({
		username: 'x',
		domain: 'nope',
		email: 'not-an-email',
		password: '',
		package_id: '',
	})
	assert.ok(errors.username)
	assert.ok(errors.domain)
	assert.ok(errors.email)
	assert.ok(errors.password)
	assert.ok(errors.package_id)
	assert.deepEqual(validateCreateAccount({
		username: 'uitest',
		domain: 'uitest.test',
		email: 'ops@uitest.test',
		password: 'TenantPass!2026',
		package_id: 'pkg',
	}), {})
})
