export interface ExportSummary {
	username: string
	domain: string
	exportedAt: string
	formatVersion: string
	sizeLabel: string
	counts: Record<string, number>
}

export function summarizeExport (raw: string): ExportSummary | null {
	try {
		const parsed = JSON.parse(raw) as Record<string, unknown>
		const account = (parsed.account && typeof parsed.account === 'object') ? parsed.account as Record<string, unknown> : parsed
		const counts: Record<string, number> = {}
		for (const key of ['domains', 'websites', 'databases', 'mailboxes', 'dns_zones', 'dns_records', 'cron', 'ftp', 'ssh_keys']) {
			counts[key] = Array.isArray(parsed[key]) ? (parsed[key] as unknown[]).length : 0
		}
		return {
			username: String(account.username || parsed.username || ''),
			domain: String(account.primary_domain || parsed.primary_domain || parsed.domain || ''),
			exportedAt: String(parsed.exported_at || ''),
			formatVersion: String(parsed.format_version || 'unknown'),
			sizeLabel: `${new Blob([raw]).size} bytes`,
			counts,
		}
	} catch {
		return null
	}
}

export function transferRollbackCopy (): string {
	return 'A confirmed import queues a new account. It does not modify the source account. Failed imports leave the new identity for cleanup from Accounts and Jobs.'
}
