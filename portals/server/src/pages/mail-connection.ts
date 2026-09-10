export function mailHostFromHostname (hostname: string): string {
	if (!hostname) return ''
	const parts = hostname.split('.').filter(Boolean)
	if (parts.length < 2) return `mail.${hostname}`
	return `mail.${parts.slice(-2).join('.')}`
}

export function mailConnectionSettings (hostname: string) {
	const host = mailHostFromHostname(hostname)
	return {
		imap: `${host}:993 (SSL/TLS)`,
		smtp: `${host}:587 (STARTTLS)`,
		imapHost: host,
		smtpHost: host,
	}
}

export function resolvedWebmailUrl (fetched?: string, domain?: string): { url: string; configured: boolean } {
	if (fetched && !fetched.includes('<domain>')) return { url: fetched, configured: true }
	if (domain) return { url: `https://webmail.${domain}/`, configured: false }
	return { url: '', configured: false }
}
