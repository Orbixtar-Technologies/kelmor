import { describe, expect, test } from 'vitest'
import { hrefForFeature, isSidebarToolActive } from './nav-hubs'
import { catalogJourneyForLocation } from './catalog-journey'
import { featureById, toToolDefinition, whmFeatures } from './whm-catalog'

const COLLIDED_PAIRS = [
	['mail-delivery-reports', 'track-delivery'],
	['mail-delivery-reports', 'jobs'],
	['track-delivery', 'jobs'],
	['review-transfers', 'jobs'],
	['review-transfers', 'task-queue'],
	['task-queue', 'jobs'],
	['show-ip-usage', 'list-accounts'],
	['api-tokens-whm', 'list-accounts'],
	['reseller-usage', 'resellers'],
	['configure-email-client', 'webmail'],
	['mailman', 'reset-mailman'],
	['email', 'mailman'],
	['email', 'email-routing-dns'],
	['backup-restoration', 'transfers'],
	['copy-account', 'transfers'],
	['force-password', 'password-modification'],
	['change-ownership', 'modify-account'],
	['db-user-password', 'sql'],
	['show-mysql-processes', 'sql'],
	['disk-usage', 'usage'],
	['daily-process-log', 'processes'],
	['security-advisor', 'security'],
	['graceful-reboot', 'forceful-reboot'],
	['server-information', 'services'],
	['service-manager', 'apache-status'],
	['add-package', 'delete-package'],
	['add-dns-zone', 'delete-dns-zone'],
	['park-domain', 'list-parked'],
	['install-ssl', 'generate-csr'],
	['update-preferences', 'change-log'],
] as const

describe('Director nav collisions', () => {
	test.each(COLLIDED_PAIRS)('%s and %s have distinct operator hrefs', (leftId, rightId) => {
		const left = featureById(leftId)
		const right = featureById(rightId)
		expect(left, leftId).toBeDefined()
		expect(right, rightId).toBeDefined()
		expect(hrefForFeature(left!)).not.toBe(hrefForFeature(right!))
	})

	test('Email tools each have their own href', () => {
		const email = whmFeatures.filter((feature) => feature.category === 'Email')
		const hrefs = email.map((feature) => hrefForFeature(feature))
		expect(new Set(hrefs).size).toBe(hrefs.length)
	})

	test('Mail Delivery Reports and Track Delivery never highlight together', () => {
		const reports = toToolDefinition(featureById('mail-delivery-reports')!)
		const track = toToolDefinition(featureById('track-delivery')!)
		const jobs = toToolDefinition(featureById('jobs')!)
		expect(isSidebarToolActive(reports, '/mail/delivery-reports')).toBe(true)
		expect(isSidebarToolActive(track, '/mail/delivery-reports')).toBe(false)
		expect(isSidebarToolActive(jobs, '/mail/delivery-reports')).toBe(false)
		expect(isSidebarToolActive(track, '/mail/track-delivery')).toBe(true)
		expect(isSidebarToolActive(reports, '/mail/track-delivery')).toBe(false)
		expect(isSidebarToolActive(jobs, '/mail/track-delivery', '?q=mail')).toBe(false)
	})

	test('Jobs search for transfers highlights only Review Transfers', () => {
		const review = toToolDefinition(featureById('review-transfers')!)
		const jobs = toToolDefinition(featureById('jobs')!)
		const queue = toToolDefinition(featureById('task-queue')!)
		expect(isSidebarToolActive(review, '/jobs', '?q=transfer')).toBe(true)
		expect(isSidebarToolActive(jobs, '/jobs', '?q=transfer')).toBe(false)
		expect(isSidebarToolActive(queue, '/jobs', '?q=transfer')).toBe(false)
		expect(isSidebarToolActive(jobs, '/jobs', '?account=acc-1')).toBe(true)
		expect(isSidebarToolActive(review, '/jobs', '?account=acc-1')).toBe(false)
	})

	test('dedicated tool pages are self-describing; leftover list views still banner', () => {
		expect(catalogJourneyForLocation('/transfers/copy')).toBeNull()
		expect(catalogJourneyForLocation('/accounts/ownership')).toBeNull()
		expect(catalogJourneyForLocation('/domains', '?view=subdomain')?.id).toBe('list-subdomains')
		expect(catalogJourneyForLocation('/mail/delivery-reports')).toBeNull()
	})

	test('every dedicated catalog href is unique', () => {
		const dedicated = whmFeatures.filter((feature) => feature.dedicated)
		const hrefs = dedicated.map((feature) => hrefForFeature(feature))
		expect(new Set(hrefs).size).toBe(hrefs.length)
	})
})
