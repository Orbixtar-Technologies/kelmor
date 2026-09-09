import { describe, expect, test } from 'vitest'
import { filterRows, paginateRows, sortRows } from './table-helpers'

interface Row {
	id: string
	name: string
	state: string
	created: string
}

const rows: Row[] = [
	{ id: '1', name: 'Zulu', state: 'failed', created: '2026-09-03' },
	{ id: '2', name: 'Alpha', state: 'queued', created: '2026-09-02' },
	{ id: '3', name: 'Bravo', state: 'failed', created: '2026-09-01' },
]

describe('table helpers', () => {
	test('filters across selected searchable fields', () => {
		expect(filterRows(rows, 'fail', ['name', 'state']).map((row) => row.id)).toEqual(['1', '3'])
	})

	test('sorts without mutating source rows', () => {
		const sorted = sortRows(rows, 'name', 'asc')

		expect(sorted.map((row) => row.name)).toEqual(['Alpha', 'Bravo', 'Zulu'])
		expect(rows[0]?.name).toBe('Zulu')
	})

	test('returns stable page metadata and clamps out-of-range pages', () => {
		expect(paginateRows(rows, 9, 2)).toEqual({
			items: [rows[2]],
			page: 2,
			pageSize: 2,
			pageCount: 2,
			total: 3,
		})
	})
})
