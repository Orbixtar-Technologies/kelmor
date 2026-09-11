import type { ToolDefinition } from './types'
import { toToolDefinition, whmFeatures } from './whm-catalog'

export const toolCatalog: ToolDefinition[] = whmFeatures.map(toToolDefinition)

export function discoverTools (tools: ToolDefinition[], _capabilities: Record<string, boolean>): ToolDefinition[] {
	return tools
}

export function groupTools (tools: ToolDefinition[]): Map<string, ToolDefinition[]> {
	const groups = new Map<string, ToolDefinition[]>()
	for (const tool of tools) groups.set(tool.category, [...(groups.get(tool.category) ?? []), tool])
	return groups
}

export function accountTaskTarget (task: string, accountId: string): string {
	const serviceByTask: Record<string, string> = {
		databases: 'databases',
		email: 'mailboxes',
		certificates: 'certificates',
		files: 'files',
		cron: 'cron',
		ftp: 'ftp',
		domains: 'domains',
		websites: 'websites',
	}
	const hubByTask: Record<string, string> = {
		databases: '/sql',
		email: '/email',
		certificates: '/ssl',
		files: '/files',
		cron: '/cron',
		ftp: '/ftp',
		domains: '/domains',
		websites: '/websites',
		deliverability: '/deliverability',
	}
	if (hubByTask[task]) return `${hubByTask[task]}?account=${accountId}`
	const service = serviceByTask[task]
	if (service) return `/accounts/${accountId}/services?service=${service}`
	if (['password', 'terminate', 'package', 'modify', 'suspension', 'summary', 'login'].includes(task)) {
		return `/accounts/${accountId}?task=${task}`
	}
	return `/accounts/${accountId}`
}
