export type DeliverabilityStatus = 'pass' | 'missing' | 'warn'

export interface DeliverabilityRecord {
	name: string
	type: string
	content: string
}

export interface DeliverabilityCheck {
	name: 'SPF' | 'DKIM' | 'DMARC'
	status: DeliverabilityStatus
	detail: string
}

export function recordHost (name: string): string {
	return name.replace(/\.$/, '').toLocaleLowerCase()
}

export function analyzeDeliverabilityRecords (domain: string, records: DeliverabilityRecord[]): DeliverabilityCheck[] {
	const zone = domain.replace(/\.$/, '').toLocaleLowerCase()
	const texts = records.filter((record) => record.type.toLocaleUpperCase() === 'TXT')
	const spf = texts.find((record) => {
		const host = recordHost(record.name)
		return (host === zone || host === '@' || host === '') && /v=spf1/i.test(record.content)
	})
	const dkim = texts.find((record) => {
		const host = recordHost(record.name)
		return host.includes('_domainkey') && /v=DKIM1/i.test(record.content)
	})
	const dmarc = texts.find((record) => {
		const host = recordHost(record.name)
		return (host === `_dmarc.${zone}` || host === '_dmarc') && /v=DMARC1/i.test(record.content)
	})
	return [
		{
			name: 'SPF',
			status: spf ? 'pass' : 'missing',
			detail: spf ? spf.content : `No SPF TXT record found for ${zone}. Add v=spf1 covering this Kelmor host.`,
		},
		{
			name: 'DKIM',
			status: dkim ? 'pass' : 'missing',
			detail: dkim ? `Selector ${recordHost(dkim.name)} is publishing DKIM.` : `No DKIM TXT record found. Kelmor publishes default._domainkey.${zone} after mail-domain provision.`,
		},
		{
			name: 'DMARC',
			status: dmarc ? 'pass' : 'warn',
			detail: dmarc ? dmarc.content : `No DMARC policy at _dmarc.${zone}. Add a TXT record such as v=DMARC1; p=none.`,
		},
	]
}

export function deliverabilitySummary (checks: DeliverabilityCheck[]): string {
	const missing = checks.filter((check) => check.status !== 'pass')
	if (!missing.length) return 'SPF, DKIM, and DMARC are present.'
	return missing.map((check) => `${check.name} ${check.status}`).join(' · ')
}
