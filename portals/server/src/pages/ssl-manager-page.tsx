import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { certExpiryLabel, certRenewalState } from './cert-copy'
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
	const [updatedAt, setUpdatedAt] = useState('')
	const [viewAll, setViewAll] = useState(false)
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('websites.write')
	const accountId = params.get('account') || ''
	const task = params.get('task') || 'inventory'
	const currentAccountId = useRef(accountId)
	const currentTask = useRef(task)
	currentAccountId.current = accountId
	currentTask.current = task
	const account = accounts.find((entry) => entry.id === accountId)

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!currentAccountId.current && next[0]) {
				const nextParams: Record<string, string> = { account: next[0].id }
				if (currentTask.current && currentTask.current !== 'inventory') nextParams.task = currentTask.current
				setParams(nextParams, { replace: true })
			}
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
			setUpdatedAt(new Date().toISOString())
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
			setUpdatedAt(new Date().toISOString())
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
				title={account && !viewAll ? `SSL / TLS · ${account.username}` : 'SSL / TLS Management'}
				description="Inventory, AutoSSL requests, expiry status, and host certificate policy live on this page. Account Services is not part of this journey."
				actions={<button type="button" className="secondary" onClick={() => setViewAll((current) => !current)}>{viewAll ? 'Account view' : 'Server-wide inventory'}</button>}
			/>
			<nav className="ssl-task-nav" aria-label="SSL tasks">
				{[
					['inventory', 'Storage'],
					['request', 'Request / Install'],
					['status', 'Status'],
					['autossl', 'AutoSSL'],
					['service', 'Service certificates'],
				].map(([id, label]) => {
					const search = viewAll ? `task=${id}` : `account=${accountId}&task=${id}`
					const href = `/ssl?${search}`
					return task === id ? <strong key={id}>{label}</strong> : <Link key={id} to={href}>{label}</Link>
				})}
			</nav>
			{!viewAll ? <AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="SSL" onChange={(next) => setParams(task === 'inventory' ? { account: next } : { account: next, task }, { replace: true })} /> : null}
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{!viewAll ? <div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams(task === 'inventory' ? { account: next } : { account: next, task }, { replace: true })}
				/>
			</div> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => viewAll ? loadAllCertificates() : loadCertificates(accountId)} /> : null}
			{!viewAll && !accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage SSL certificates." /> : null}
			{!viewAll && accountId && canWrite && (task === 'request' || task === 'autossl') ? <section className="panel">
				<h2>{task === 'autossl' ? 'AutoSSL policy and request' : 'Request / install certificate'}</h2>
				<form className="inline-form" onSubmit={requestCertificate}>
					<label>Hostname<input name="hostname" defaultValue={account?.primary_domain} required /></label>
					<button type="submit">Request AutoSSL</button>
				</form>
				<p className="subtle">Kelmor issues the certificate through ACME and applies it to the account nginx vhost. There is no separate CSR download or bounce through Account Services.</p>
			</section> : null}
			{task === 'service' ? <section className="panel">
				<h2>Service certificates</h2>
				<p>Director, Control, and mail submission use the host TLS material installed with Kelmor. Account site certificates are listed below and requested with AutoSSL on this same page.</p>
			</section> : null}
			{(viewAll || accountId) ? <section className="panel">
				<h2>{viewAll ? 'All account certificates' : `Certificates for ${account?.username}`}</h2>
				{loading ? <LoadingState label="Loading certificates…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Hostname</th><th>Account</th><th>Kind</th><th>Status</th><th>Expiry</th><th>Issuer</th><th>Actions</th></tr></thead>
					<tbody>
						{rows.map((certificate) => (
							<tr key={`${certificate.account_id}-${certificate.id}`}>
								<td><strong>{valueOf(certificate, 'hostname')}</strong></td>
								<td>{viewAll ? <Link to={`/ssl?account=${certificate.account_id}`}>{certificate.account_name}</Link> : certificate.account_name}</td>
								<td>{valueOf(certificate, 'kind')}</td>
								<td><StatusBadge value={valueOf(certificate, 'status')} /><small>{certRenewalState(valueOf(certificate, 'status'), certificate.not_after ? String(certificate.not_after) : undefined)}</small></td>
								<td>{certExpiryLabel(certificate.not_after ? String(certificate.not_after) : undefined)}<small>{certificate.not_after ? formatDate(String(certificate.not_after)) : ''}</small></td>
								<td>{valueOf(certificate, 'issuer')}</td>
								<td><div className="row-actions">{valueOf(certificate, 'status').toLocaleLowerCase() === 'failed' && canWrite ? <button type="button" className="link-button" onClick={() => api(`/api/v1/accounts/${certificate.account_id}/certificates`, { method: 'POST', body: JSON.stringify({ hostname: valueOf(certificate, 'hostname') }) }).then(() => setMessage('Certificate retry queued.')).catch((requestError) => setMessage(messageFrom(requestError)))}>Retry request</button> : null}<Link to={`/ssl?account=${certificate.account_id}&task=request`}>Request again</Link><Link to={`/jobs?account=${certificate.account_id}`}>Jobs</Link></div></td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading && !rows.length ? <EmptyState title="No certificates yet" detail="Request a certificate for a hostname using the form above." /> : null}
			</section> : null}
		</>
	)
}
