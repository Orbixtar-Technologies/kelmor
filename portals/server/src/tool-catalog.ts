import { canonicalAccountToolPath } from './account-tool-routes'
import { accountFunctionHref } from './dedicated-tool-routes'
import type { ToolDefinition } from './types'
import { toToolDefinition, whmFeatures } from './whm-catalog'

const dedicatedAccountTasks: Record<string, string> = {
	password: '/accounts/password',
	terminate: '/accounts/terminate',
	remove: '/accounts/remove',
	package: '/accounts/change-package',
	modify: '/accounts/modify',
	suspension: '/accounts/suspension',
	summary: '/accounts/summary',
	login: '/accounts/login-control',
	ownership: '/accounts/ownership',
	tokens: '/accounts/tokens',
}

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
		deliverability: 'deliverability',
		dns: 'dns',
	}
	const service = serviceByTask[task]
	if (service) return canonicalAccountToolPath(service, accountId) || `/accounts/${accountId}`
	const dedicated = dedicatedAccountTasks[task]
	if (dedicated) return accountFunctionHref(dedicated, accountId)
	return `/accounts/${accountId}`
}
