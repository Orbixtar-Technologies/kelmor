import { describe, expect, test } from 'vitest'
import { buildAccountPayload, validateAccountStep } from './account-wizard'
import type { AccountDraft } from './types'

const validDraft: AccountDraft = {
	username: 'acmehost',
	primaryDomain: 'www.acme.test',
	ownerEmail: 'ops@acme.test',
	ownerPassword: 'Correct-Horse-2026!',
	packageId: 'pkg-1',
	resellerId: 'reseller-1',
}

describe('account wizard', () => {
	test('blocks invalid identity values', () => {
		expect(validateAccountStep(1, { ...validDraft, username: 'Bad User' })).toHaveProperty('username')
		expect(validateAccountStep(1, { ...validDraft, primaryDomain: 'not a domain' })).toHaveProperty('primaryDomain')
		expect(validateAccountStep(1, { ...validDraft, ownerEmail: 'missing-at' })).toHaveProperty('ownerEmail')
	})

	test('requires package ownership selection', () => {
		expect(validateAccountStep(2, { ...validDraft, packageId: '' })).toEqual({
			packageId: 'Select a package.',
		})
	})

	test('builds the exact reviewed API payload', () => {
		expect(buildAccountPayload(validDraft)).toEqual({
			username: 'acmehost',
			primary_domain: 'www.acme.test',
			owner_email: 'ops@acme.test',
			owner_password: 'Correct-Horse-2026!',
			package_id: 'pkg-1',
			reseller_id: 'reseller-1',
		})
	})

	test('normalizes the submitted domain before generating a blank owner email', () => {
		expect(buildAccountPayload({
			...validDraft,
			username: 'caseowner',
			primaryDomain: 'WWW.Example.COM',
			ownerEmail: '',
		})).toMatchObject({
			username: 'caseowner',
			primary_domain: 'www.example.com',
			owner_email: 'caseowner@www.example.com',
		})
	})
})
