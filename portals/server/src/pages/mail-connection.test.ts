import { describe, expect, test } from 'vitest'
import { mailConnectionSettings, mailHostFromHostname, resolvedWebmailUrl } from './mail-connection'

describe('mail connection', () => {
	test('derives mail.hosting from a panel hostname', () => {
		expect(mailHostFromHostname('panel.kelmor.test')).toBe('mail.kelmor.test')
		expect(mailConnectionSettings('kelmor.test').imap).toContain('mail.kelmor.test:993')
	})

	test('prefers a resolved webmail URL over the placeholder pattern', () => {
		expect(resolvedWebmailUrl('https://webmail.shop.example.com/', 'shop.example.com')).toEqual({
			url: 'https://webmail.shop.example.com/',
			configured: true,
		})
		expect(resolvedWebmailUrl(undefined, 'shop.example.com').configured).toBe(false)
	})
})
