import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, Pagination, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, percent } from '../helpers'
import { useCan } from '../rbac'
import { filterRows, paginateRows, sortRows } from '../table-helpers'
import { accountTaskTarget } from '../tool-catalog'
import type { Account, Package, Usage } from '../types'

interface MonitorResponse {
	accounts?: Usage[]
}

export function AccountsPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [usage, setUsage] = useState<Usage[]>([])
	const [selected, setSelected] = useState<Set<string>>(new Set())
	const [query, setQuery] = useState('')
	const [sort, setSort] = useState<keyof Account>('username')
	const [direction, setDirection] = useState<'asc' | 'desc'>('asc')
	const [page, setPage] = useState(1)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const canCreate = useCan('accounts.create')
	const canSuspend = useCan('accounts.suspend')
	const canReadUsage = useCan('billing.usage.read')
	const view = params.get('view') || 'all'
	const task = params.get('task') || ''
	const packageFilter = params.get('package') || ''

	async function loadAccounts () {
		const result = await api<{ items: Account[] }>('/api/v1/accounts')
		setAccounts(asList(result))
	}
	async function load () {
		setLoading(true)
		setError('')
		setUsage([])
		const [accountResult, packageResult, usageResult] = await Promise.allSettled([
			api<{ items: Account[] }>('/api/v1/accounts'),
			api<{ items: Package[] }>('/api/v1/packages'),
			api<MonitorResponse>('/api/v1/server/monitor'),
		])
		if (accountResult.status === 'rejected') {
			setError(messageFrom(accountResult.reason))
			setLoading(false)
			return
		}
		const nextAccounts = asList(accountResult.value)
		setAccounts(nextAccounts)
		if (packageResult.status === 'fulfilled') setPackages(asList(packageResult.value))
		if (usageResult.status === 'fulfilled') {
			setUsage(usageResult.value.accounts || [])
		} else if (canReadUsage) {
			const usageResults = await Promise.allSettled(nextAccounts.map((account) => api<Usage>(`/api/v1/accounts/${account.id}/usage`)))
			setUsage(usageResults.flatMap((result) => result.status === 'fulfilled' ? [result.value] : []))
		}
		setLoading(false)
	}
	useEffect(() => { void load() }, [])

	const packagesById = useMemo(() => new Map(packages.map((pkg) => [pkg.id, pkg])), [packages])
	const usageByAccountId = useMemo(() => new Map(usage.map((entry) => [entry.account_id, entry])), [usage])
	const enriched = useMemo(() => accounts.map((account) => ({ ...account, usage: usageByAccountId.get(account.id) })), [accounts, usageByAccountId])
	const viewed = enriched.filter((account) => {
		if (packageFilter && account.package_id !== packageFilter) return false
		if (view === 'suspended') return account.status === 'suspended'
		if (view === 'over-quota') {
			const pkg = packagesById.get(account.package_id)
			return Boolean(account.usage && pkg && (account.usage.disk_bytes > pkg.disk_bytes || account.usage.bandwidth_bytes > pkg.bandwidth_bytes_monthly))
		}
		return true
	})
	const filtered = filterRows(viewed, query, ['username', 'primary_domain', 'status', 'ip_address'])
	const paged = paginateRows(sortRows(filtered, sort, direction), page, 20)

	function changeSort (key: keyof Account) {
		if (sort === key) setDirection(direction === 'asc' ? 'desc' : 'asc')
		else { setSort(key); setDirection('asc') }
	}
	async function stateAction (id: string, action: 'suspend' | 'unsuspend') {
		await api(`/api/v1/accounts/${id}/${action}`, { method: 'POST', body: '{}' })
	}
	async function bulkAction (action: 'suspend' | 'unsuspend') {
		if (action === 'suspend' && !window.confirm(`Suspend ${selected.size} selected account${selected.size === 1 ? '' : 's'}? Their hosted services will become unavailable.`)) return
		setMessage('')
		try {
			if (action === 'suspend') await api('/api/v1/accounts/bulk/suspend', { method: 'POST', body: JSON.stringify({ ids: [...selected] }) })
			else await Promise.all([...selected].map((id) => stateAction(id, action)))
			setMessage(`${selected.size} account${selected.size === 1 ? '' : 's'} queued for ${action}.`)
			setSelected(new Set())
			await loadAccounts()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}

	return (
		<>
			<PageHeader title="List Accounts" description="Search, compare, and operate hosting identities from one dense account list." actions={canCreate ? <Link className="button-link" to="/accounts/create">Create account</Link> : undefined} />
			{task && accountTaskGuidance[task] ? (
				<section className="task-guidance" aria-label="Selected account task">
					<div><strong>{accountTaskGuidance[task].title}</strong><p>{accountTaskGuidance[task].detail}</p></div>
					<span>Choose an account below to continue.</span>
				</section>
			) : null}
			<div className="view-tabs" role="group" aria-label="Account views">
				{[['all', 'All accounts'], ['suspended', 'Suspended'], ['over-quota', 'Over quota']].map(([value, label]) => <button key={value} type="button" className={view === value ? 'active' : 'secondary'} onClick={() => {
					const next = new URLSearchParams(params)
					if (value === 'all') next.delete('view')
					else next.set('view', value)
					setParams(next)
					setPage(1)
				}}>{label}</button>)}
			</div>
			{packageFilter ? <p className="filter-context">Showing accounts assigned to package <code>{packageFilter}</code>. <button type="button" className="link-button" onClick={() => { const next = new URLSearchParams(params); next.delete('package'); setParams(next) }}>Clear package filter</button></p> : null}
			<div className="filter-bar">
				<label>Search accounts<input type="search" value={query} placeholder="Username, domain, status, or IP" onChange={(event) => { setQuery(event.target.value); setPage(1) }} /></label>
				<div className="bulk-actions"><span>{selected.size} selected</span>{canSuspend ? <><button type="button" disabled={!selected.size} onClick={() => bulkAction('suspend')}>Suspend</button><button type="button" className="secondary" disabled={!selected.size} onClick={() => bulkAction('unsuspend')}>Unsuspend</button></> : null}</div>
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading account inventory…" /> : (
				<div className="table-wrap"><table className="dense-table">
					<thead><tr><th><input type="checkbox" aria-label="Select page" checked={paged.items.length > 0 && paged.items.every((account) => selected.has(account.id))} onChange={(event) => {
						const next = new Set(selected)
						paged.items.forEach((account) => event.target.checked ? next.add(account.id) : next.delete(account.id))
						setSelected(next)
					}} /></th>
					{[['username', 'User'], ['primary_domain', 'Primary domain'], ['status', 'Status'], ['linux_uid', 'UID']].map(([key, label]) => <th key={key}><button type="button" className="sort-button" onClick={() => changeSort(key as keyof Account)}>{label}{sort === key ? (direction === 'asc' ? ' ↑' : ' ↓') : ''}</button></th>)}
					<th>Package</th><th>Disk quota</th><th>Actions</th></tr></thead>
					<tbody>{paged.items.map((account) => {
						const pkg = packagesById.get(account.package_id)
						const diskPercent = percent(account.usage?.disk_bytes, pkg?.disk_bytes)
						return <tr key={account.id} className={account.status === 'suspended' ? 'muted-row' : ''}>
							<td><input type="checkbox" aria-label={`Select ${account.username}`} checked={selected.has(account.id)} onChange={(event) => { const next = new Set(selected); event.target.checked ? next.add(account.id) : next.delete(account.id); setSelected(next) }} /></td>
							<td><Link to={`/accounts/${account.id}`}><strong>{account.username}</strong></Link><small>{account.ip_address || 'Shared IP'}</small></td>
							<td>{account.primary_domain}</td><td><StatusBadge value={account.status} /></td><td>{account.linux_uid}</td><td>{pkg?.name || account.package_id}</td>
							<td>{account.usage ? <><span className={diskPercent > 100 ? 'danger-text' : ''}>{diskPercent}%</span><small>{formatBytes(account.usage.disk_bytes)}</small></> : '—'}</td>
							<td><div className="row-actions"><Link to={accountTaskTarget(task, account.id)}>{task ? 'Continue' : 'Manage'}</Link>{canSuspend ? <button type="button" className="link-button" onClick={async () => {
								const action = account.status === 'suspended' ? 'unsuspend' : 'suspend'
								if (action === 'suspend' && !window.confirm(`Suspend ${account.username}? Its hosted services will become unavailable.`)) return
								try { await stateAction(account.id, action); await loadAccounts() } catch (requestError) { setMessage(messageFrom(requestError)) }
							}}>{account.status === 'suspended' ? 'Unsuspend' : 'Suspend'}</button> : null}</div></td>
						</tr>
					})}</tbody>
				</table></div>
			)}
			{!loading && !error && paged.items.length === 0 ? <EmptyState title={`No ${view === 'all' ? '' : `${view} `}accounts match`} detail="Filters remain available above. Clear the search or choose another account view." /> : null}
			<Pagination page={paged.page} pageCount={paged.pageCount} total={paged.total} onPage={setPage} />
		</>
	)
}

const accountTaskGuidance: Record<string, { title: string; detail: string }> = {
	summary: { title: 'Account Summary', detail: 'Open a consolidated status, ownership, usage, and service hub.' },
	modify: { title: 'Modify an Account', detail: 'Change the primary domain, IP address, reseller ownership, and login access.' },
	package: { title: 'Change Account Package', detail: 'Review the current assignment and select a different package with enforced limits.' },
	suspension: { title: 'Suspend or Unsuspend', detail: 'Review account status and queue the appropriate availability change.' },
	terminate: { title: 'Terminate an Account', detail: 'Open the account summary and complete a typed destructive confirmation.' },
	password: { title: 'Force Password Change', detail: 'Rotate owner credentials and optionally require another change at next sign-in.' },
	databases: { title: 'SQL Services', detail: 'Manage account-scoped databases and their provisioning state.' },
	email: { title: 'Email Services', detail: 'Manage mail domains, mailboxes, aliases, and routing policy.' },
	certificates: { title: 'SSL Certificates', detail: 'Inspect issued certificates and request account-owned hostnames.' },
}
