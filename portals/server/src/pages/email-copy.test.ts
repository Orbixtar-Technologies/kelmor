import { describe, expect, test } from 'vitest'
import { mailDomainLabel, mailboxAddress } from './email-copy'

describe('mailboxAddress', () => {
	test('uses the mailbox domain instead of the account primary domain', () => {
		const address = mailboxAddress(
			{ id: 'mb-1', local_part: 'info', domain_id: 'md-2' },
			[{ id: 'md-1', ascii_fqdn: 'shop.test' }, { id: 'md-2', ascii_fqdn: 'mail.shop.test' }],
			'shop.test',
		)
		expect(address).toBe('info@mail.shop.test')
	})

	test('labels mail domains by ascii_fqdn instead of UUID', () => {
		expect(mailDomainLabel({ id: 'dom-uuid', ascii_fqdn: 'orbixtar.dpdns.org' })).toBe('orbixtar.dpdns.org')
		expect(mailDomainLabel({ id: 'dom-uuid', domain_id: 'dom-uuid' })).toBe('dom-uuid')
	})
})
