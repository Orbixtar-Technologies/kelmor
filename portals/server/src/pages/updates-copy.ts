import { formatDate } from '../helpers'

export interface UpdateCheckStatus {
	state: string
	installed_release?: string
	available_release?: string
	last_checked_at?: string
}

export function formatLastChecked (value?: string): string {
	if (!value) return '—'
	return `${formatDate(value)} (local time)`
}

export function updateCheckFeedback (status: UpdateCheckStatus): string {
	if (status.state === 'available' &&
		status.available_release &&
		status.available_release !== status.installed_release) {
		return 'Checked — update available'
	}
	return "Checked — you're up to date"
}
