import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

type DomainView = 'all' | 'addon' | 'subdomain' | 'alias'

const viewLabels: Record<DomainView, string> = {
	all: 'All domains',
	addon: 'Addon domains',
	subdomain: 'Subdomains',
	alias: 'Parked domains',
}

interface DomainRow extends ResourceItem {
	account_username?: string
}

export function DomainsPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [rows, setRows] = useState<DomainRow[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('domains.write')
	const accountId = params.get('account') || ''
	const view = ((params.get('view') as DomainView) || 'all')
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

	const loadDomains = useCallback((requestedAccountId: string, inventory: Account[]) => {
		const request = requests.begin('domains')
		setLoading(true)
		setError('')
		const targets = requestedAccountId
			? inventory.filter((entry) => entry.id === requestedAccountId)
			: inventory
		if (!targets.length) {
			setRows([])
			setLoading(false)
			return
		}
		Promise.allSettled(targets.map((entry) => api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/domains`).then((result) => ({
			account: entry,
			items: asList(result),
		})))).then((results) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			const next: DomainRow[] = []
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
			if (failures === results.length) setError('Could not load domains.')
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		if (!accounts.length) {
			setLoading(false)
			return
		}
		loadDomains(accountId, accounts)
	}, [accountId, accounts, loadDomains])

	function setView (next: DomainView) {
		const search = new URLSearchParams(params)
		if (next === 'all') search.delete('view')
		else search.set('view', next)
		setParams(search, { replace: true })
	}

	async function createDomain (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/domains`, {
				method: 'POST',
				body: JSON.stringify({
					fqdn: data.get('fqdn'),
					type: data.get('type'),
					dns_managed: true,
				}),
			})
			setMessage('Domain creation queued.')
			event.currentTarget.reset()
			loadDomains(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeDomain (ownerAccountId: string, domainId: string, type: string) {
		if (type === 'primary') return
		if (!window.confirm('Delete this domain? The primary domain cannot be removed.')) return
		try {
			await api(`/api/v1/accounts/${ownerAccountId}/domains/${domainId}`, { method: 'DELETE' })
			setMessage('Domain deletion queued.')
			loadDomains(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	const visible = useMemo(() => rows.filter((row) => view === 'all' || valueOf(row, 'type') === view), [rows, view])

	return (
		<>
			<PageHeader
				title={account ? `List Domains · ${account.username}` : 'List Domains'}
				description="Inventory addon domains, subdomains, and parked (alias) domains the way a WHM operator expects, using Kelmor account APIs."
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Domains" onChange={(next) => {
				const search = new URLSearchParams(params)
				if (next) search.set('account', next)
				else search.delete('account')
				setParams(search, { replace: true })
			}} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => {
						const search = new URLSearchParams(params)
						if (next) search.set('account', next)
						else search.delete('account')
						setParams(search, { replace: true })
					}}
				/>
				<p className="subtle">Leave the account blank to list every visible domain. Creating or deleting a domain requires a selected account.</p>
			</div>
			<div className="view-tabs" role="group" aria-label="Domain types">
				{(Object.keys(viewLabels) as DomainView[]).map((entry) => (
					<button key={entry} type="button" className={view === entry ? 'active' : 'secondary'} onClick={() => setView(entry)}>
						{viewLabels[entry]}
					</button>
				))}
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadDomains(accountId, accounts)} /> : null}
			{canWrite && accountId ? <section className="panel">
				<h2>Add a domain to {account?.username}</h2>
				<form className="inline-form" onSubmit={createDomain}>
					<label>Domain<input name="fqdn" placeholder="shop.example.com" required /></label>
					<label>Type<select name="type">
						<option value="addon">Addon</option>
						<option value="subdomain">Subdomain</option>
						<option value="alias">Parked / alias</option>
					</select></label>
					<button type="submit">Add domain</button>
				</form>
			</section> : null}
			<section className="panel">
				<h2>{viewLabels[view]}</h2>
				{loading ? <LoadingState label="Loading domains…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Domain</th><th>Type</th><th>Account</th><th>Status</th><th>DNS</th><th>Actions</th></tr></thead>
					<tbody>
						{visible.map((domain) => {
							const ownerId = String(domain.account_id || accountId)
							const type = valueOf(domain, 'type')
							return (
								<tr key={`${ownerId}:${domain.id}`}>
									<td><strong>{valueOf(domain, 'ascii_fqdn') || valueOf(domain, 'fqdn')}</strong></td>
									<td>{type === 'alias' ? 'parked' : type}</td>
									<td><Link to={`/accounts/${ownerId}`}>{domain.account_username || ownerId}</Link></td>
									<td><StatusBadge value={valueOf(domain, 'status')} /></td>
									<td>{valueOf(domain, 'dns_managed')}</td>
									<td>
										<div className="row-actions">
											<Link to={`/dns?account=${ownerId}`}>DNS</Link>
											<Link to={`/websites?account=${ownerId}`}>PHP / websites</Link>
											{canWrite && type !== 'primary' ? <button type="button" className="link-button danger-text" onClick={() => removeDomain(ownerId, domain.id, type)}>Delete</button> : null}
										</div>
									</td>
								</tr>
							)
						})}
					</tbody>
				</table></div> : null}
				{!loading && !visible.length ? <EmptyState title={`No ${viewLabels[view].toLocaleLowerCase()}`} detail="Add a domain for the selected account, or clear the type filter." /> : null}
			</section>
		</>
	)
}
