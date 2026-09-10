export function firewallImpactLines (): string[] {
	return [
		'Default inbound policy becomes drop except loopback and established flows.',
		'Hosting ports stay open: 80, 443, 25, 465, 587, 993, 21, and the FTP PASV range.',
		'Bound management ports remain reachable so this session is not locked out.',
	]
}

export function firewallPolicyPreview (): string {
	return [
		'table inet panel',
		'  policy inbound drop',
		'  allow loopback, established, related',
		'  allow 80, 443, 25, 465, 587, 993, 21, 40000-40100',
		'  allow bound management ports',
	].join('\n')
}

export function firewallRollbackCopy (): string {
	return 'Re-applying restores the Kelmor-managed policy. Manual nft edits outside this table are not rolled back.'
}

export function rebootImpactLines (): string[] {
	return [
		'Active websites, mail sessions, and operator consoles disconnect until services return.',
		'Queued jobs may fail mid-run and need a retry after the host is back.',
		'There is no scheduled-reboot window in this control; the request starts immediately.',
	]
}
