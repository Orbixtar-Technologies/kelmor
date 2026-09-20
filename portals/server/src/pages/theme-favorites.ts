export const DEFAULT_FAVORITE_TOOL_IDS = [
	'list-accounts', 'create-account', 'jobs', 'services',
	'packages', 'dns', 'email', 'sql',
]

export const THEME_CHANGED_EVENT = 'kelmor-theme-changed'

export function parseFavoriteIds (raw?: string): string[] {
	if (!raw || !raw.trim()) return []
	const seen = new Set<string>()
	const out: string[] = []
	for (const part of raw.split(/[,\n]/)) {
		const id = part.trim()
		if (!id || seen.has(id)) continue
		seen.add(id)
		out.push(id)
	}
	return out
}

export function serializeFavoriteIds (ids: string[]): string {
	return ids.filter(Boolean).join(',')
}

export function resolveFavoriteIds (raw?: string): string[] {
	const parsed = parseFavoriteIds(raw)
	return parsed.length ? parsed : DEFAULT_FAVORITE_TOOL_IDS
}

export function moveFavorite (ids: string[], index: number, direction: -1 | 1): string[] {
	const next = ids.slice()
	const dest = index + direction
	if (index < 0 || dest < 0 || dest >= next.length) return next
	const current = next[index]
	next[index] = next[dest]
	next[dest] = current
	return next
}

export function notifyThemeChanged (): void {
	window.dispatchEvent(new Event(THEME_CHANGED_EVENT))
}
