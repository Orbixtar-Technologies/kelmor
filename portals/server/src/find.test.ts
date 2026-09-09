import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DIRECTOR_FUNCTIONS, matchFind, NAV_GROUPS, visibleFunctions } from './find.ts'

const adminCaps = {
	'server.read': true,
	'accounts.read': true,
	'accounts.create': true,
	'accounts.suspend': true,
	'accounts.terminate': true,
	'accounts.modify': true,
	'packages.read': true,
	'security.audit.read': true,
	'billing.usage.read': true,
	'resellers.read': true,
	'domains.read': true,
	'dns.read': true,
}

test('Find matches functions and accounts', () => {
	const fns = visibleFunctions(adminCaps)
	assert.equal(fns.length, DIRECTOR_FUNCTIONS.length)
	const hits = matchFind('list', fns, [
		{ id: 'a1', username: 'ada', primary_domain: 'ada.test' },
		{ id: 'a2', username: 'listed', primary_domain: 'other.test' },
	])
	assert.deepEqual(hits.map((h) => h.to), ['/accounts', '/accounts/a2'])
	assert.equal(matchFind('   ', fns, []).length, 0)
	assert.ok(NAV_GROUPS.includes('Account Functions'))
	assert.ok(NAV_GROUPS.includes('Service Status/Host'))
	assert.ok(NAV_GROUPS.includes('DNS/Domains'))
	assert.ok(NAV_GROUPS.includes('Jobs & Audit'))
	assert.ok(NAV_GROUPS.includes('Import/Migration'))
	assert.ok(NAV_GROUPS.includes('Usage/Quotas'))
})

test('Find hides functions the actor cannot use', () => {
	const fns = visibleFunctions({ 'accounts.read': true })
	assert.deepEqual(fns.map((f) => f.to), [
		'/accounts',
		'/accounts/summary',
		'/domains',
		'/jobs',
	])
	assert.equal(matchFind('create', fns, []).length, 0)
	assert.ok(matchFind('suspend', fns, []).length === 0)
})

test('Find locates dedicated account functions', () => {
	const fns = visibleFunctions(adminCaps)
	assert.equal(matchFind('terminate', fns, [])[0]?.to, '/accounts/terminate')
	assert.equal(matchFind('change package', fns, [])[0]?.to, '/accounts/package')
	assert.equal(matchFind('dns', fns, [])[0]?.to, '/domains')
})
