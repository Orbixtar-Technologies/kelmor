import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import type { Account, ResourceItem } from '../types'
import { analyzeDeliverabilityRecords, deliverabilitySummary, type DeliverabilityCheck } from './deliverability-copy'

interface DomainReport {
	id: string
	domain: string
	catchall: string
	status: string
	checks: DeliverabilityCheck[]
	summary: string
}

export function DeliverabilityPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [reports, setReports] = useState<DomainReport[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const account = accounts.find((entry) => entry.id === accountId)

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!currentAccountId.current && next[0]) setParams({ account: next[0].id }, { replace: true })
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests, setParams])

	const loadReport = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('deliverability')
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/mail/domains`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/domains`),
		]).then(async ([mailResult, zoneResult, domainResult]) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			if (mailResult.status === 'rejected' && zoneResult.status === 'rejected') {
				setError(messageFrom(mailResult.reason))
				setReports([])
				return
			}
			const mailDomains = mailResult.status === 'fulfilled' ? asList(mailResult.value) : []
			const zones = zoneResult.status === 'fulfilled' ? asList(zoneResult.value) : []
			const domains = domainResult.status === 'fulfilled' ? asList(domainResult.value) : []
			const recordSets = await Promise.all(zones.map((zone) => api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones/${zone.id}/records`).then((result) => ({
				zone,
				records: asList(result),
			})).catch(() => ({ zone, records: [] as ResourceItem[] }))))
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			const sources = mailDomains.length ? mailDomains : domains
			const next = sources.map((item) => {
				const domainId = valueOf(item, 'domain_id')
				const domain = valueOf(item, 'ascii_fqdn') !== '—'
					? valueOf(item, 'ascii_fqdn')
					: domains.find((entry) => entry.id === item.domain_id || entry.id === domainId)
						? valueOf(domains.find((entry) => entry.id === item.domain_id || entry.id === domainId) as ResourceItem, 'ascii_fqdn')
						: valueOf(item, 'name')
				const zone = zones.find((entry) => entry.domain_id === item.domain_id || entry.domain_id === item.id || valueOf(entry, 'name') === domain)
				const records = recordSets.find((entry) => entry.zone.id === zone?.id)?.records || []
				const checks = analyzeDeliverabilityRecords(domain === '—' ? account?.primary_domain || '' : domain, records.map((record) => ({
					name: valueOf(record, 'name'),
					type: valueOf(record, 'type'),
					content: valueOf(record, 'content'),
				})))
				return {
					id: item.id,
					domain: domain === '—' ? account?.primary_domain || item.id : domain,
					catchall: valueOf(item, 'catchall_policy'),
					status: valueOf(item, 'status'),
					checks,
					summary: deliverabilitySummary(checks),
				}
			})
			setReports(next)
			setUpdatedAt(new Date().toISOString())
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [account?.primary_domain, requests])

	useEffect(() => {
		requests.invalidate('deliverability')
		setReports([])
		if (accountId) loadReport(accountId)
		else setLoading(false)
	}, [accountId, loadReport, requests])

	return (
		<>
			<PageHeader
				title={account ? `Email Deliverability · ${account.username}` : 'Email Deliverability'}
				description="Check SPF, DKIM, and DMARC records for mail domains. Fix missing records in DNS Management."
				actions={accountId ? <Link className="button-link" to={`/dns?account=${accountId}`}>DNS Management</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Deliverability" onChange={(next) => setParams({ account: next }, { replace: true })} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/>
			</div>
			{error ? <ErrorState error={error} onRetry={() => loadReport(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to inspect mail authentication records." /> : null}
			{accountId ? <section className="panel">
				<h2>Authentication for {account?.username}</h2>
				{loading ? <LoadingState label="Checking DNS authentication records…" /> : null}
				{!loading ? reports.map((report) => (
					<article key={report.id} className="deliverability-card">
						<header>
							<strong>{report.domain}</strong>
							<span>{report.summary}</span>
							{report.catchall !== '—' ? <small>Catch-all: {report.catchall}</small> : null}
						</header>
						<div className="table-wrap"><table className="dense-table">
							<thead><tr><th>Check</th><th>State</th><th>Detail</th></tr></thead>
							<tbody>
								{report.checks.map((check) => (
									<tr key={check.name}>
										<td>{check.name}</td>
										<td><StatusBadge value={check.status === 'pass' ? 'active' : check.status === 'warn' ? 'pending' : 'inactive'} /></td>
										<td>{check.detail}</td>
									</tr>
								))}
							</tbody>
						</table></div>
					</article>
				)) : null}
				{!loading && !reports.length ? <EmptyState title="No mail domains" detail="Provision mail for this account, then return here to verify SPF, DKIM, and DMARC." /> : null}
			</section> : null}
		</>
	)
}
