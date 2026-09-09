import type { ResourceItem } from './types'

export function formatBytes (value: number | undefined): string {
	if (!value) return '0 B'
	const units = ['B', 'KB', 'MB', 'GB', 'TB']
	let amount = value
	let index = 0
	while (amount >= 1024 && index < units.length - 1) {
		amount /= 1024
		index += 1
	}
	return `${amount.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

export function formatDate (value: string | undefined): string {
	if (!value) return '—'
	const date = new Date(value)
	return Number.isNaN(date.valueOf()) ? value : date.toLocaleString()
}

export function percent (used: number | undefined, total: number | undefined): number {
	if (!used || !total) return 0
	return Math.min(999, Math.round((used / total) * 100))
}

export function messageFrom (error: unknown): string {
	return error instanceof Error ? error.message : 'Request failed.'
}

export function valueOf (item: ResourceItem, key: string): string {
	const value = item[key]
	if (typeof value === 'boolean') return value ? 'Yes' : 'No'
	if (value === null || value === undefined || value === '') return '—'
	return String(value)
}

export function downloadJSON (filename: string, data: unknown) {
	const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
	const url = URL.createObjectURL(blob)
	const anchor = document.createElement('a')
	anchor.href = url
	anchor.download = filename
	anchor.click()
	URL.revokeObjectURL(url)
}
