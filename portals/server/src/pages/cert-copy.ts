export function certDaysRemaining (notAfter?: string, now = Date.now()): number | null {
	if (!notAfter) return null
	const stamp = new Date(notAfter).valueOf()
	if (Number.isNaN(stamp)) return null
	return Math.ceil((stamp - now) / 86_400_000)
}

export function certExpiryLabel (notAfter?: string, now = Date.now()): string {
	const days = certDaysRemaining(notAfter, now)
	if (days === null) return 'Expiry unknown'
	if (days < 0) return `Expired ${Math.abs(days)} day${Math.abs(days) === 1 ? '' : 's'} ago`
	if (days === 0) return 'Expires today'
	if (days <= 14) return `${days} days remaining · renew soon`
	return `${days} days remaining`
}

export function certRenewalState (status?: string, notAfter?: string, now = Date.now()): string {
	const normalized = (status || '').toLocaleLowerCase()
	if (normalized === 'failed') return 'Last request failed'
	if (normalized === 'requested' || normalized === 'pending') return 'Renewal requested'
	const days = certDaysRemaining(notAfter, now)
	if (days !== null && days <= 30 && normalized === 'active') return 'Renewal due'
	if (normalized === 'active') return 'Current'
	return status || 'unknown'
}
