export const PAGE_SIZE = 25

export function pageSlice<T> (items: T[], page: number, size = PAGE_SIZE) {
	const pages = Math.max(1, Math.ceil(items.length / size))
	const safe = Math.min(Math.max(1, page), pages)
	return {
		page: safe,
		pages,
		total: items.length,
		rows: items.slice((safe - 1) * size, safe * size),
	}
}
