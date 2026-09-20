import type { ResourceItem } from './types'

export interface PhpVersionChange {
	websiteId: string
	accountId: string
	domainId: string
	documentRoot: string
	accountUsername: string
	runtime: string
	current: string
	proposed: string
}

export interface PhpWebsiteRow extends ResourceItem {
	account_id?: string
	account_username?: string
}

export function websitePhpVersion (website: PhpWebsiteRow) {
	const value = website.runtime_version
	return typeof value === 'string' && value ? value : '8.3'
}

export function pendingPhpChanges (
	rows: PhpWebsiteRow[],
	drafts: Record<string, string>,
): PhpVersionChange[] {
	const changes: PhpVersionChange[] = []
	for (const website of rows) {
		const proposed = drafts[website.id]
		if (!proposed) continue
		const current = websitePhpVersion(website)
		if (proposed === current) continue
		changes.push({
			websiteId: website.id,
			accountId: String(website.account_id || ''),
			domainId: String(website.domain_id || ''),
			documentRoot: String(website.document_root || website.id),
			accountUsername: website.account_username || String(website.account_id || ''),
			runtime: String(website.runtime || 'php'),
			current,
			proposed,
		})
	}
	return changes
}

export function stageSelectedPhpVersions (
	rows: PhpWebsiteRow[],
	selected: Record<string, boolean>,
	version: string,
	drafts: Record<string, string>,
) {
	const next = { ...drafts }
	for (const website of rows) {
		if (!selected[website.id]) continue
		if (String(website.runtime || 'php') !== 'php') continue
		next[website.id] = version
	}
	return next
}
