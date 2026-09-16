import { describe, expect, test } from 'vitest'
import { RequestSequence } from './request-sequence'

describe('RequestSequence', () => {
	test('invalidates an older request in the same scope', () => {
		const requests = new RequestSequence()
		const first = requests.begin('records')
		const second = requests.begin('records')
		expect(requests.isCurrent(first)).toBe(false)
		expect(requests.isCurrent(second)).toBe(true)
	})
})
