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

export const POLICY_SETTINGS_BANNER = 'Save updates a Director policy. nginx, Postfix, PowerDNS, and the Agent do not apply this as host configuration.'

export const POLICY_SETTINGS_SAVED = 'Saved the Director demo set. Demo accounts keep services but block destructive owner writes.'

export const MAIL_NOTIFY_BANNER = 'Queue sends through POST /mail/notify (SendSystemMail). This is not an nginx or Postfix configuration apply.'

export const REARRANGE_NOT_IMPLEMENTED = 'Kelmor homes stay under /home/<user>. Moving a home to another filesystem is not implemented.'

export const NGINX_LOG_INSPECT_COPY = 'Download or copy a listed path. This tool does not rewrite nginx or queue a no-op Apply.'

export const MAILBOX_PASSWORD_STUB = 'Mailbox password and quota changes queue a mailbox reconcile. Maps apply the new hash and Dovecot quota.'

export const DATABASE_USER_MODEL = 'Kelmor provisions one database user per engine for the account. Extra MySQL users and GRANTs are not supported.'

const DEFERRED_SETTING_KEYS = new Set<string>([])

const CHROME_SETTING_KEYS = new Set([
	'theme',
	'locale',
	'customization',
])

const POLICY_SETTING_KEYS = new Set([
	'demo_accounts',
])

export function isDeferredSettingsKey (key?: string): boolean {
	return Boolean(key && DEFERRED_SETTING_KEYS.has(key))
}

export function isChromeSettingsKey (key?: string): boolean {
	return Boolean(key && CHROME_SETTING_KEYS.has(key))
}

export function isPolicySettingsKey (key?: string): boolean {
	return Boolean(key && POLICY_SETTING_KEYS.has(key))
}

export function isLocalSettingsFeature (feature: Pick<WhmFeature, 'settingKey'>): boolean {
	return isDeferredSettingsKey(feature.settingKey)
}

export function isHostSettingsFeature (feature: Pick<WhmFeature, 'settingKey'>): boolean {
	return Boolean(feature.settingKey) && !isDeferredSettingsKey(feature.settingKey) && !isChromeSettingsKey(feature.settingKey) && !isPolicySettingsKey(feature.settingKey)
}

export function isPolicySettingsFeature (feature: Pick<WhmFeature, 'settingKey' | 'id'>): boolean {
	return isPolicySettingsKey(feature.settingKey) || feature.id === 'manage-demo-mode'
}

export function isMailNotifyFeature (feature: Pick<WhmFeature, 'id'>): boolean {
	return feature.id === 'email-all-users' || feature.id === 'email-resellers'
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

export function confirmLabelForFeature (feature: Pick<WhmFeature, 'id' | 'settingKey' | 'layout' | 'accountAction' | 'inspectOnly'>): string | undefined {
	if (feature.inspectOnly) return undefined
	if (isMailNotifyFeature(feature)) return 'Queue mail'
	if (isDeferredSettingsKey(feature.settingKey)) return 'Save policy record'
	if (isChromeSettingsKey(feature.settingKey)) return 'Apply to chrome'
	if (isPolicySettingsKey(feature.settingKey)) return 'Save demo set'
	if (feature.settingKey || feature.layout === 'settings') return 'Apply on host'
	if (feature.accountAction === 'patch' && !feature.settingKey) return 'Apply host change'
	return undefined
}
