import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DIRECTOR_FUNCTIONS, matchFind, NAV_GROUPS, visibleFunctions } from './find.ts'

test('Find matches functions and accounts', () => {
	const fns = visibleFunctions({
		'server.read': true,
		'accounts.read': true,
		'accounts.create': true,
		'packages.read': true,
		'security.audit.read': true,
		'billing.usage.read': true,
		'resellers.read': true,
	})
	assert.equal(fns.length, DIRECTOR_FUNCTIONS.length)
	const hits = matchFind('list', fns, [
		{ id: 'a1', username: 'ada', primary_domain: 'ada.test' },
		{ id: 'a2', username: 'listed', primary_domain: 'other.test' },
	])
	assert.deepEqual(hits.map((h) => h.to), ['/accounts', '/accounts/a2'])
	assert.equal(matchFind('   ', fns, []).length, 0)
	assert.ok(NAV_GROUPS.includes('Account Functions'))
	assert.ok(NAV_GROUPS.includes('Host/Service Status'))
})

test('Find hides functions the actor cannot use', () => {
	const fns = visibleFunctions({ 'accounts.read': true })
	assert.deepEqual(fns.map((f) => f.to), ['/accounts', '/jobs'])
	assert.equal(matchFind('create', fns, []).length, 0)
})
