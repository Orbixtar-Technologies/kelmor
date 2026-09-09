import type { PageResult } from './types'

export function filterRows<T extends object> (rows: T[], query: string, keys: Array<keyof T>): T[] {
	const needle = query.trim().toLocaleLowerCase()
	if (!needle) return rows
	return rows.filter((row) => keys.some((key) => String(row[key] ?? '').toLocaleLowerCase().includes(needle)))
}

export function sortRows<T extends object> (rows: T[], key: keyof T, direction: 'asc' | 'desc'): T[] {
	const factor = direction === 'asc' ? 1 : -1
	return [...rows].sort((left, right) => {
		const a = left[key]
		const b = right[key]
		if (typeof a === 'number' && typeof b === 'number') return (a - b) * factor
		return String(a ?? '').localeCompare(String(b ?? ''), undefined, { numeric: true }) * factor
	})
}

export function paginateRows<T> (rows: T[], requestedPage: number, pageSize: number): PageResult<T> {
	const safeSize = Math.max(1, pageSize)
	const pageCount = Math.max(1, Math.ceil(rows.length / safeSize))
	const page = Math.min(Math.max(1, requestedPage), pageCount)
	const start = (page - 1) * safeSize
	return { items: rows.slice(start, start + safeSize), page, pageSize: safeSize, pageCount, total: rows.length }
}
