import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pageSlice } from './pager.ts'

test('pageSlice clamps and slices', () => {
	const items = [1, 2, 3, 4, 5]
	const first = pageSlice(items, 1, 2)
	assert.deepEqual(first.rows, [1, 2])
	assert.equal(first.pages, 3)
	assert.equal(first.total, 5)
	assert.deepEqual(pageSlice(items, 9, 2).rows, [5])
	assert.deepEqual(pageSlice([], 3, 25).rows, [])
	assert.equal(pageSlice([], 3, 25).page, 1)
})
