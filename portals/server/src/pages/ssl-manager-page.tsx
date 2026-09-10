import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

export function SSLManagerPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [certificates, setCertificates] = useState<ResourceItem[]>([])
	const [allCertificates, setAllCertificates] = useState<Array<ResourceItem & { account_id: string; account_name: string }>>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [viewAll, setViewAll] = useState(false)
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('websites.write')
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

	const loadCertificates = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('certificates')
		setLoading(true)
		setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/certificates`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setCertificates(asList(result))
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	const loadAllCertificates = useCallback(async () => {
		const request = requests.begin('all-certificates')
		setLoading(true)
		setError('')
		try {
			const accountList = accounts.length ? accounts : asList(await api<{ items: Account[] }>('/api/v1/accounts'))
			const results = await Promise.allSettled(accountList.map(async (entry) => {
				const result = await api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/certificates`)
				return asList(result).map((certificate) => ({
					...certificate,
					account_id: entry.id,
					account_name: entry.username,
				}))
			}))
			if (!requests.isCurrent(request)) return
			setAllCertificates(results.flatMap((result) => result.status === 'fulfilled' ? result.value : []))
		} catch (requestError) {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		} finally {
			if (requests.isCurrent(request)) setLoading(false)
		}
	}, [accounts, requests])

	useEffect(() => {
		setMessage('')
		if (viewAll) {
			loadAllCertificates()
			return
		}
		requests.invalidate('certificates')
		setCertificates([])
		if (accountId) loadCertificates(accountId)
		else setLoading(false)
	}, [accountId, loadAllCertificates, loadCertificates, requests, viewAll])

	async function requestCertificate (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/certificates`, {
				method: 'POST',
				body: JSON.stringify({ hostname: data.get('hostname') }),
			})
			setMessage('Certificate request queued.')
			event.currentTarget.reset()
			loadCertificates(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	type CertificateRow = ResourceItem & { account_id: string; account_name: string }
	const rows: CertificateRow[] = viewAll ? allCertificates : certificates.map((certificate) => ({
		...certificate,
		account_id: accountId,
		account_name: account?.username || '—',
	}))

	return (
		<>
			<PageHeader
				title="SSL / TLS Management"
				description="Review certificate inventory, request AutoSSL certificates, and monitor expiry across accounts."
				actions={<button type="button" className="secondary" onClick={() => setViewAll((current) => !current)}>{viewAll ? 'Account view' : 'Server-wide inventory'}</button>}
			/>
			{!viewAll ? <div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/>
			</div> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => viewAll ? loadAllCertificates() : loadCertificates(accountId)} /> : null}
			{!viewAll && !accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage SSL certificates." /> : null}
			{!viewAll && accountId && canWrite ? <section className="panel">
				<h2>Request certificate</h2>
				<form className="inline-form" onSubmit={requestCertificate}>
					<label>Hostname<input name="hostname" defaultValue={account?.primary_domain} required /></label>
					<button type="submit">Request AutoSSL</button>
				</form>
				<p className="subtle">Certificates are issued through the ACME workflow and applied to the account&apos;s nginx vhosts automatically.</p>
			</section> : null}
			{(viewAll || accountId) ? <section className="panel">
				<h2>{viewAll ? 'All account certificates' : `Certificates for ${account?.username}`}</h2>
				{loading ? <LoadingState label="Loading certificates…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Hostname</th><th>Account</th><th>Kind</th><th>Status</th><th>Expires</th><th>Actions</th></tr></thead>
					<tbody>
						{rows.map((certificate) => (
							<tr key={`${certificate.account_id}-${certificate.id}`}>
								<td><strong>{valueOf(certificate, 'hostname')}</strong></td>
								<td>{viewAll ? <Link to={`/ssl?account=${certificate.account_id}`}>{certificate.account_name}</Link> : certificate.account_name}</td>
								<td>{valueOf(certificate, 'kind')}</td>
								<td><StatusBadge value={valueOf(certificate, 'status')} /></td>
								<td>{certificate.not_after ? formatDate(String(certificate.not_after)) : '—'}</td>
								<td><Link className="link-button" to={`/accounts/${certificate.account_id}/services?service=certificates`}>Manage</Link></td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading && !rows.length ? <EmptyState title="No certificates yet" detail="Request a certificate for a hostname using the form above." /> : null}
			</section> : null}
		</>
	)
}
