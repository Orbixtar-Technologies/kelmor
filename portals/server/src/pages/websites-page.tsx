import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

const phpVersions = ['8.1', '8.2', '8.3', '8.4']

interface WebsiteRow extends ResourceItem {
	account_username?: string
}

export function WebsitesPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [domains, setDomains] = useState<ResourceItem[]>([])
	const [rows, setRows] = useState<WebsiteRow[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
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
			setAccounts(asList(result))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests])

	const loadWebsites = useCallback((requestedAccountId: string, inventory: Account[]) => {
		const request = requests.begin('websites')
		setLoading(true)
		setError('')
		const targets = requestedAccountId
			? inventory.filter((entry) => entry.id === requestedAccountId)
			: inventory
		if (!targets.length) {
			setRows([])
			setDomains([])
			setLoading(false)
			return
		}
		const domainLoad = requestedAccountId
			? api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/domains`).then((result) => {
				if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setDomains(asList(result))
			}).catch(() => {
				if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setDomains([])
			})
			: Promise.resolve(setDomains([]))
		const siteLoads = Promise.allSettled(targets.map((entry) => api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/websites`).then((result) => ({
			account: entry,
			items: asList(result),
		})))).then((results) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			const next: WebsiteRow[] = []
			let failures = 0
			results.forEach((result) => {
				if (result.status === 'rejected') {
					failures += 1
					return
				}
				result.value.items.forEach((item) => next.push({
					...item,
					account_id: result.value.account.id,
					account_username: result.value.account.username,
				}))
			})
			setRows(next)
			setUpdatedAt(new Date().toISOString())
			if (failures === results.length) setError('Could not load websites.')
		})
		Promise.allSettled([domainLoad, siteLoads]).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		if (!accounts.length) {
			setLoading(false)
			return
		}
		loadWebsites(accountId, accounts)
	}, [accountId, accounts, loadWebsites])

	async function createWebsite (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/websites`, {
				method: 'POST',
				body: JSON.stringify({
					domain_id: data.get('domain_id'),
					runtime: data.get('runtime'),
					runtime_version: data.get('runtime_version') || undefined,
					document_root: `${account?.home_path || ''}/public_html`,
				}),
			})
			setMessage('Website provision queued. Changing PHP on an existing domain updates that site.')
			event.currentTarget.reset()
			loadWebsites(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function changeRuntime (ownerAccountId: string, website: WebsiteRow, runtime: string, runtimeVersion: string) {
		try {
			await api(`/api/v1/accounts/${ownerAccountId}/websites`, {
				method: 'POST',
				body: JSON.stringify({
					domain_id: website.domain_id,
					runtime,
					runtime_version: runtimeVersion || undefined,
					document_root: website.document_root,
				}),
			})
			setMessage('PHP / runtime change queued.')
			loadWebsites(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<>
			<PageHeader
				title={account ? `MultiPHP Manager · ${account.username}` : 'MultiPHP Manager'}
				description="Review website runtimes and queue PHP version changes for account vhosts, matching the WHM MultiPHP Manager journey."
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Websites" onChange={(next) => setParams(next ? { account: next } : {}, { replace: true })} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams(next ? { account: next } : {}, { replace: true })}
				/>
				<p className="subtle">Select an account to create a site or change its PHP version. The inventory can list every visible website.</p>
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadWebsites(accountId, accounts)} /> : null}
			{canWrite && accountId ? <section className="panel">
				<h2>Create or update a website</h2>
				<form className="inline-form" onSubmit={createWebsite}>
					<label>Domain<select name="domain_id" required>{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label>
					<label>Runtime<select name="runtime"><option value="php">PHP</option><option value="static">Static</option><option value="node">Node</option><option value="python">Python</option></select></label>
					<label>PHP version<select name="runtime_version">{phpVersions.map((version) => <option key={version} value={version}>{version}</option>)}</select></label>
					<button type="submit" disabled={!domains.length}>Save website</button>
				</form>
			</section> : null}
			<section className="panel">
				<h2>Websites</h2>
				{loading ? <LoadingState label="Loading websites…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Document root</th><th>Runtime</th><th>Version</th><th>Account</th><th>Enabled</th><th>Actions</th></tr></thead>
					<tbody>
						{rows.map((website) => {
							const ownerId = String(website.account_id || accountId)
							return (
								<tr key={`${ownerId}:${website.id}`}>
									<td><strong>{valueOf(website, 'document_root')}</strong></td>
									<td>{valueOf(website, 'runtime')}</td>
									<td>{canWrite ? (
										<label className="inline-select">
											<span className="sr-only">PHP version for {valueOf(website, 'document_root')}</span>
											<select
												defaultValue={String(website.runtime_version || '8.3')}
												onChange={(event) => changeRuntime(ownerId, website, String(website.runtime || 'php'), event.target.value)}
											>
												{phpVersions.map((version) => <option key={version} value={version}>{version}</option>)}
											</select>
										</label>
									) : valueOf(website, 'runtime_version')}
									</td>
									<td><Link to={`/accounts/${ownerId}`}>{website.account_username || ownerId}</Link></td>
									<td><StatusBadge value={Boolean(website.enabled)} /></td>
									<td>
										<div className="row-actions">
											<Link to={`/files?account=${ownerId}`}>Files</Link>
											<Link to={`/ssl?account=${ownerId}`}>SSL</Link>
											<Link to={`/accounts/${ownerId}/services?service=websites`}>Manage</Link>
										</div>
									</td>
								</tr>
							)
						})}
					</tbody>
				</table></div> : null}
				{!loading && !rows.length ? <EmptyState title="No websites yet" detail="Provision a domain, then create its website and PHP version here." /> : null}
			</section>
		</>
	)
}
