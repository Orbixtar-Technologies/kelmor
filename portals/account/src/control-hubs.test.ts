import { describe, expect, test } from 'vitest'
import {
	activeControlTab,
	canSeeControlHub,
	controlHubEntryPath,
	controlHubs,
	visibleControlTabs,
} from './control-hubs'

describe('Control hubs', () => {
	test('combines related tenant menus onto shared sidebar entries', () => {
		expect(controlHubs.map((hub) => hub.id)).toEqual([
			'dashboard', 'websites', 'domains', 'email', 'databases', 'files', 'backups',
		])
		expect(controlHubs.find((hub) => hub.id === 'websites')?.tabs.map((tab) => tab.id)).toEqual(['sites', 'ssl'])
		expect(controlHubs.find((hub) => hub.id === 'domains')?.tabs.map((tab) => tab.id)).toEqual(['domains', 'dns'])
		expect(controlHubs.find((hub) => hub.id === 'backups')?.tabs.map((tab) => tab.id)).toEqual(['backups', 'cron'])
	})

	test('shows a hub when any of its tabs are allowed', () => {
		const domains = controlHubs.find((hub) => hub.id === 'domains')!
		expect(canSeeControlHub(domains, { 'dns.read': true })).toBe(true)
		expect(controlHubEntryPath(domains, { 'dns.read': true })).toBe('/domains?tab=dns')
		expect(visibleControlTabs(domains, { 'dns.read': true }).map((tab) => tab.id)).toEqual(['dns'])
		expect(activeControlTab(domains, null, { 'dns.read': true })?.id).toBe('dns')
	})
})
