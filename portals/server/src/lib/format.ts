export function formatBytes (n: number | null | undefined, digits = 1) {
	if (n === null || n === undefined) return '—'
	if (n === 0) return '0 B'
	const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
	let value = Math.abs(n)
	let unit = 0
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024
		unit++
	}
	const sign = n < 0 ? '-' : ''
	return `${sign}${value.toFixed(unit === 0 ? 0 : digits)} ${units[unit]}`
}

export function formatCount (n: number | null | undefined) {
	if (n === null || n === undefined) return '—'
	return n.toLocaleString('en-US')
}

export function formatUptime (seconds: number | null | undefined) {
	if (!seconds) return '—'
	const days = Math.floor(seconds / 86400)
	const hours = Math.floor((seconds % 86400) / 3600)
	const minutes = Math.floor((seconds % 3600) / 60)
	if (days > 0) return `${days}d ${hours}h`
	if (hours > 0) return `${hours}h ${minutes}m`
	return `${minutes}m`
}

export function formatDateTime (value: string | null | undefined) {
	if (!value) return '—'
	const d = new Date(value)
	if (Number.isNaN(d.getTime())) return String(value)
	return d.toLocaleString('en-GB', { year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function formatRelative (value: string | null | undefined) {
	if (!value) return '—'
	const d = new Date(value)
	if (Number.isNaN(d.getTime())) return String(value)
	const delta = Math.round((Date.now() - d.getTime()) / 1000)
	if (delta < 60) return `${Math.max(delta, 0)}s ago`
	if (delta < 3600) return `${Math.floor(delta / 60)}m ago`
	if (delta < 86400) return `${Math.floor(delta / 3600)}h ago`
	return `${Math.floor(delta / 86400)}d ago`
}

/** Percentage of a limit, clamped to 0–100. A zero limit means unmetered. */
export function usedPercent (used: number | null | undefined, limit: number | null | undefined) {
	if (!limit || limit <= 0 || !used || used <= 0) return 0
	return Math.min(100, Math.round((used / limit) * 100))
}

export function meterClass (percent: number) {
	if (percent >= 90) return 'meter crit'
	if (percent >= 75) return 'meter warn'
	return 'meter'
}

/**
 * Abbreviates an identifier for a table cell. UUIDv7 ids share a time-ordered
 * prefix, so the tail is what actually distinguishes two rows.
 */
export function shortId (value: string | null | undefined, length = 8) {
	if (!value) return '—'
	if (value.length <= length) return value
	return `…${value.slice(-length)}`
}

export function titleCase (value: string) {
	return value.replace(/[_.-]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}
