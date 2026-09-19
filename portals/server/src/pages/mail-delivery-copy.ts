export interface MailDeliveryAttempt {
	id: string
	queue_id: string
	timestamp: string
	sender?: string
	recipient?: string
	status: string
	message: string
	relay?: string
	dsn?: string
	direction: string
}

export interface MailQueueEntry {
	queue_id: string
	size?: number
	sender?: string
	recipient?: string
	arrival?: string
	state: string
}

export interface MailDeliveryReport {
	mode: string
	date?: string
	query?: string
	source?: string
	partial?: boolean
	message?: string
	items?: MailDeliveryAttempt[]
	queue?: MailQueueEntry[]
}

export interface ReportDate {
	year: number
	month: number
	day: number
}

export const MONTH_OPTIONS = [
	{ value: 1, label: 'January' },
	{ value: 2, label: 'February' },
	{ value: 3, label: 'March' },
	{ value: 4, label: 'April' },
	{ value: 5, label: 'May' },
	{ value: 6, label: 'June' },
	{ value: 7, label: 'July' },
	{ value: 8, label: 'August' },
	{ value: 9, label: 'September' },
	{ value: 10, label: 'October' },
	{ value: 11, label: 'November' },
	{ value: 12, label: 'December' },
] as const

export function defaultReportDate (now = new Date()): ReportDate {
	return {
		year: now.getUTCFullYear(),
		month: now.getUTCMonth() + 1,
		day: now.getUTCDate(),
	}
}

export function parseReportDate (input: {
	year: string
	month: string
	day: string
}): { date: ReportDate } | { error: string } {
	const year = Number(input.year)
	const month = Number(input.month)
	const day = Number(input.day)
	if (!Number.isInteger(year) || year < 1970 || year > 2100) {
		return { error: 'Enter a year between 1970 and 2100.' }
	}
	if (!Number.isInteger(month) || month < 1 || month > 12) {
		return { error: 'Choose a month.' }
	}
	if (!Number.isInteger(day) || day < 1 || day > 31) {
		return { error: 'Choose a day of the month.' }
	}
	const stamp = new Date(Date.UTC(year, month - 1, day))
	if (
		stamp.getUTCFullYear() !== year ||
		stamp.getUTCMonth() !== month - 1 ||
		stamp.getUTCDate() !== day
	) {
		return { error: 'That calendar date does not exist.' }
	}
	return { date: { year, month, day } }
}

export function deliveryReportsHref (date: ReportDate, query = ''): string {
	const params = new URLSearchParams({
		year: String(date.year),
		month: String(date.month),
		day: String(date.day),
	})
	if (query.trim()) params.set('q', query.trim())
	return `/mail/delivery-reports?${params}`
}

export function trackDeliveryHref (query = ''): string {
	const params = new URLSearchParams()
	if (query.trim()) params.set('q', query.trim())
	const search = params.toString()
	return search ? `/mail/track-delivery?${search}` : '/mail/track-delivery'
}

export function deliveryReportsApiPath (date: ReportDate, query = ''): string {
	const params = new URLSearchParams({
		year: String(date.year),
		month: String(date.month),
		day: String(date.day),
	})
	if (query.trim()) params.set('q', query.trim())
	return `/api/v1/mail/delivery-reports?${params}`
}

export function trackDeliveryApiPath (query = ''): string {
	const params = new URLSearchParams()
	if (query.trim()) params.set('q', query.trim())
	const search = params.toString()
	return search ? `/api/v1/mail/delivery-track?${search}` : '/api/v1/mail/delivery-track'
}

export function formatDeliveryStatus (status: string): string {
	switch (status) {
		case 'delivered':
			return 'Delivered'
		case 'deferred':
			return 'Deferred'
		case 'bounced':
			return 'Bounced'
		case 'rejected':
			return 'Rejected'
		case 'expired':
			return 'Expired'
		case 'accepted':
			return 'Accepted'
		case 'queued':
			return 'Queued'
		default:
			return status || 'Unknown'
	}
}

export function deliveryStatusBadge (status: string): string {
	switch (status) {
		case 'delivered':
			return 'active'
		case 'deferred':
		case 'queued':
		case 'accepted':
			return 'pending'
		case 'bounced':
		case 'rejected':
		case 'expired':
			return 'failed'
		default:
			return 'inactive'
	}
}
