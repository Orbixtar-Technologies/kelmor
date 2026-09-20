import { describe, expect, test } from 'vitest'
import { linkedNodesStatus, linkedPeerCount } from './cluster-panel'

describe('configuration cluster peer status', () => {
	test('linkedPeerCount uses snapshot linked_peers without inventing URLs', () => {
		expect(linkedPeerCount({ linked_peers: 0, live_multi_node: false })).toBe(0)
		expect(linkedPeerCount({ linked_peers: 2, live_multi_node: true })).toBe(2)
		expect(linkedPeerCount({ linked_peers: 1.8, live_multi_node: true })).toBe(1)
		expect(linkedPeerCount({ live_multi_node: true })).toBe(1)
		expect(linkedPeerCount({})).toBe(0)
	})

	test('linkedNodesStatus keeps apply disabled copy when peers are zero', () => {
		expect(linkedNodesStatus({ linkedPeers: 0, canApplyRemote: false }))
			.toBe('0 linked nodes — Apply to peers needs at least one linked node.')
		expect(linkedNodesStatus({ linkedPeers: 1, canApplyRemote: true }))
			.toBe('1 linked node — Apply to peers is available.')
		expect(linkedNodesStatus({ linkedPeers: 3, canApplyRemote: true }))
			.toBe('3 linked nodes — Apply to peers is available.')
	})
})
