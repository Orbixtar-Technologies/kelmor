export function createPendingGuard () {
	let pending = false
	return {
		tryStart () {
			if (pending) return false
			pending = true
			return true
		},
		finish () {
			pending = false
		},
		isPending () {
			return pending
		},
	}
}
