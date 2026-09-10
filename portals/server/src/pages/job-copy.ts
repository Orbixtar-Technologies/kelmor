import type { Job } from '../types'

const JOB_LABELS: Record<string, string> = {
	'account.provision': 'Create account',
	'account.reconcile': 'Apply account settings',
	'account.copy_homedir': 'Copy account files',
	'account.suspend': 'Suspend account',
	'account.unsuspend': 'Unsuspend account',
	'account.terminate': 'Terminate account',
	'website.provision': 'Create website',
	'website.update': 'Update website',
	'database.create': 'Create database',
	'database.delete': 'Delete database',
	'domain.create': 'Add domain',
	'dns.zone': 'Update DNS zone',
	'dns.record': 'Update DNS record',
	'mail.mailbox': 'Update mailbox',
	'certificate.provision': 'Request certificate',
	'certificate.portal': 'Request panel certificate',
	'backup.create': 'Create backup',
	'backup.restore': 'Restore backup',
	'cron.create': 'Add scheduled task',
	'ftp.create': 'Create FTP user',
}

export interface JobFailureSummary {
	title: string
	resource: string
	reason: string
	technical: string
}

export function formatJobType (type: string): string {
	if (JOB_LABELS[type]) return JOB_LABELS[type]
	const prefix = Object.keys(JOB_LABELS).find((key) => type.startsWith(key.replace(/\.[^.]+$/, '.')) || type.startsWith(`${key.split('.')[0]}.`))
	if (prefix && JOB_LABELS[prefix] && type.startsWith(prefix)) return JOB_LABELS[prefix]
	const family = type.split('.')[0]
	const familyLabels: Record<string, string> = {
		account: 'Account operation',
		website: 'Website operation',
		database: 'Database operation',
		domain: 'Domain operation',
		dns: 'DNS update',
		mail: 'Mail operation',
		certificate: 'Certificate request',
		backup: 'Backup operation',
		cron: 'Scheduled task',
		ftp: 'FTP operation',
		application: 'Application deployment',
	}
	if (familyLabels[family]) return familyLabels[family]
	return type.replaceAll('.', ' ')
}

function extractErrorMessage (raw: string): { reason: string; technical: string } {
	const trimmed = raw.trim()
	if (!trimmed) return { reason: 'The operation failed. Open details for logs and recovery options.', technical: '' }
	if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
		try {
			const parsed = JSON.parse(trimmed) as Record<string, unknown> | unknown[]
			if (parsed && !Array.isArray(parsed) && typeof parsed === 'object') {
				const reason = String(parsed.error || parsed.message || parsed.reason || parsed.detail || 'The operation failed.')
				return { reason, technical: trimmed }
			}
		} catch {
			return { reason: 'The operation failed. Open details for the recorded diagnostic.', technical: trimmed }
		}
	}
	return { reason: trimmed, technical: trimmed === raw ? '' : raw }
}

export function describeJobFailure (job: Job): JobFailureSummary {
	const extracted = extractErrorMessage(job.last_error || '')
	const hostname = typeof job.payload.hostname === 'string' ? job.payload.hostname : ''
	const resource = hostname || job.resource_type || job.resource_id || 'this account'
	return {
		title: formatJobType(job.type),
		resource,
		reason: extracted.reason,
		technical: extracted.technical || job.last_error || '',
	}
}

export function jobMatchesAccount (job: Job, accountId: string): boolean {
	if (!accountId) return true
	return job.resource_id === accountId || job.payload.account_id === accountId
}

export function summarizeJobCounts (jobs: Job[]): { queued: number; running: number; succeeded: number; failed: number; total: number } {
	return {
		queued: jobs.filter((job) => job.state === 'queued').length,
		running: jobs.filter((job) => job.state === 'running').length,
		succeeded: jobs.filter((job) => job.state === 'succeeded').length,
		failed: jobs.filter((job) => job.state === 'failed').length,
		total: jobs.length,
	}
}
