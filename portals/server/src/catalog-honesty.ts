import type { ToolDefinition } from './types'
import { featureById, type WhmFeature } from './whm-catalog'

export const LOCAL_SETTINGS_LABEL = 'Settings (local)'
export const NOT_APPLIED_TO_HOST = 'Not applied to host'

export const LOCAL_SETTINGS_BANNER = 'Settings (local) — Not applied to host. Save writes a Director preference file. nginx, Postfix, PowerDNS, and the Agent do not read this value.'

export const LOCAL_SETTINGS_SAVED = 'Saved a Director preference. This is not applied to nginx, Postfix, PowerDNS, or the Agent.'

export const HOST_SETTINGS_BANNER = 'Save queues a host apply job. The Agent writes nginx, Postfix, rspamd, PowerDNS, sshd, or firewall configuration from this page.'

export const HOST_SETTINGS_SAVED = 'Host apply queued. Open Jobs to follow the operation.'

export const CHROME_SETTINGS_SAVED = 'Applied to Director chrome.'

export const DEFERRED_SETTINGS_SAVED = 'Recorded as a Director policy. This product is not live on the host yet.'

export const QUOTA_PACKAGE_COPY = 'Disk and monthly bandwidth caps come from the account package. The Agent enforces those package limits. There is no separate per-account override.'

export const MAILBOX_PASSWORD_STUB = 'Mailbox password and quota changes have no API yet. Create and delete work. Rotate the password from the host only after a mailbox PATCH exists.'

export const CUSTOM_PEM_STUB = 'Custom PEM install is not available. Kelmor issues certificates through AutoSSL. There is no certificate upload API.'

export const DATABASE_USER_MODEL = 'Kelmor provisions one database user per engine for the account. Extra MySQL users and GRANTs are not supported.'

const DEFERRED_SETTING_KEYS = new Set([
	'external_auth',
	'two_factor',
	'configuration_cluster',
	'linked_nodes',
	'remote_access_key',
	'module_installers',
	'perl_modules',
	'php_pear',
	'php_pecl',
	'ruby_gems',
	'mariadb_upgrade',
	'support_access',
	'server_profile',
])

const CHROME_SETTING_KEYS = new Set([
	'theme',
	'locale',
	'customization',
])

export function isDeferredSettingsKey (key?: string): boolean {
	return Boolean(key && DEFERRED_SETTING_KEYS.has(key))
}

export function isChromeSettingsKey (key?: string): boolean {
	return Boolean(key && CHROME_SETTING_KEYS.has(key))
}

export function isLocalSettingsFeature (feature: Pick<WhmFeature, 'settingKey'>): boolean {
	return isDeferredSettingsKey(feature.settingKey)
}

export function isHostSettingsFeature (feature: Pick<WhmFeature, 'settingKey'>): boolean {
	return Boolean(feature.settingKey) && !isDeferredSettingsKey(feature.settingKey) && !isChromeSettingsKey(feature.settingKey)
}

export function isLocalSettingsToolId (toolId: string): boolean {
	const feature = featureById(toolId)
	return Boolean(feature && isLocalSettingsFeature(feature))
}

export function isLocalSettingsTool (tool: Pick<ToolDefinition, 'id'>): boolean {
	return isLocalSettingsToolId(tool.id)
}

export function localSettingsBadgeTitle (label: string): string {
	return `${label} — ${LOCAL_SETTINGS_LABEL}. ${NOT_APPLIED_TO_HOST}.`
}

export function confirmLabelForFeature (feature: Pick<WhmFeature, 'settingKey' | 'layout' | 'accountAction'>): string | undefined {
	if (isDeferredSettingsKey(feature.settingKey)) return 'Save policy record'
	if (isChromeSettingsKey(feature.settingKey)) return 'Apply to chrome'
	if (feature.settingKey || feature.layout === 'settings') return 'Apply on host'
	if (feature.accountAction === 'patch' && !feature.settingKey) return 'Apply host change'
	return undefined
}
