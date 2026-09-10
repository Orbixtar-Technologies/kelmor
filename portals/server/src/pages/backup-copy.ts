export interface BackupDestinationReadiness {
	ready: boolean
	label: string
	detail: string
}

export function backupDestinationReadiness (destination: string): BackupDestinationReadiness {
	if (destination === 'local') {
		return {
			ready: true,
			label: 'Ready on this host',
			detail: 'The encrypted archive is stored on this server. No extra destination credentials are required.',
		}
	}
	if (destination === 'sftp') {
		return {
			ready: false,
			label: 'Needs host SFTP credentials',
			detail: 'Queueing fails until the host SFTP backup destination is configured. Local remains available without those credentials.',
		}
	}
	if (destination === 's3') {
		return {
			ready: false,
			label: 'Needs host S3 credentials',
			detail: 'Queueing fails until the host S3 backup destination is configured. Local remains available without those credentials.',
		}
	}
	return {
		ready: false,
		label: 'Unknown destination',
		detail: 'Choose local, SFTP, or S3 before queueing an encrypted backup.',
	}
}

export function backupScopeSummary (retentionDays?: number): string {
	const retention = retentionDays && retentionDays > 0
		? `This package retains backups for ${retentionDays} days.`
		: 'Retention follows the account package backup setting.'
	return `A full encrypted backup includes website files, databases, and mail for this account. ${retention} Archives are encrypted with the host backup key; restore uses the same key and does not ask for a passphrase.`
}
