export type ServiceAction = 'reload' | 'restart' | 'start' | 'stop'

export function serviceActionImpact (action: ServiceAction): string {
	if (action === 'reload') return 'Reloads configuration with little or no downtime.'
	if (action === 'start') return 'Starts a stopped service. Existing traffic is unaffected until it begins answering.'
	if (action === 'restart') return 'Stops then starts the service. Active connections drop for a short outage.'
	return 'Stops the service until it is started again. Sites or mail that depend on it become unavailable.'
}

export function isDisruptiveServiceAction (action: ServiceAction): boolean {
	return action === 'restart' || action === 'stop'
}

export function updateInstallDisabledReason (installed?: string, available?: string): string {
	if (!available || available === installed) return 'Installed release matches the available release, so install stays disabled.'
	return ''
}
