import { describe, expect, test } from 'vitest'
import {
	DEFAULT_FAVORITE_TOOL_IDS,
	moveFavorite,
	parseFavoriteIds,
	resolveFavoriteIds,
	serializeFavoriteIds,
} from './theme-favorites'

describe('theme favorites', () => {
	test('parses ordered unique tool ids', () => {
		expect(parseFavoriteIds('jobs, dns, jobs\nemail')).toEqual(['jobs', 'dns', 'email'])
		expect(parseFavoriteIds('')).toEqual([])
		expect(parseFavoriteIds(undefined)).toEqual([])
	})

	test('falls back to the Home default set when none are saved', () => {
		expect(resolveFavoriteIds('')).toEqual(DEFAULT_FAVORITE_TOOL_IDS)
		expect(resolveFavoriteIds('jobs,dns')).toEqual(['jobs', 'dns'])
	})

	test('reorders and serializes pinned tools', () => {
		expect(moveFavorite(['jobs', 'dns', 'email'], 1, -1)).toEqual(['dns', 'jobs', 'email'])
		expect(moveFavorite(['jobs', 'dns'], 0, -1)).toEqual(['jobs', 'dns'])
		expect(serializeFavoriteIds(['jobs', 'dns'])).toBe('jobs,dns')
	})
})
