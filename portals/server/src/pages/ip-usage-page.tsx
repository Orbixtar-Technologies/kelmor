import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import type { Account } from '../types'

function addressKind (value: string): string {
	if (value.includes(':')) return 'IPv6'
	if (value) return 'IPv4'
	return 'shared / unset'
}

export function IPUsagePage () {
	const [accounts, setAccounts] = useState<Account[]>([])
	const [query, setQuery] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	function load () {
		setLoading(true)
		setError('')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			setAccounts(asList(result))
		}).catch((requestError) => {
			setError(messageFrom(requestError))
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])

	const rows = useMemo(() => {
		const needle = query.toLocaleLowerCase()
		return accounts.filter((account) => {
			const blob = `${account.username} ${account.primary_domain} ${account.ip_address || ''}`.toLocaleLowerCase()
			return blob.includes(needle)
		})
	}, [accounts, query])

	return (
		<>
			<PageHeader
				title="Show IP Address Usage"
				description="Which accounts currently use a dedicated IPv4 or IPv6 address. Shared addresses stay blank until Change Site IP assigns one."
			/>
			<div className="filter-bar">
				<label>Search accounts or IPs
					<input type="search" value={query} onChange={(event) => setQuery(event.target.value)} />
				</label>
			</div>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading account IP assignments…" /> : null}
			{!loading ? (
				<div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Account</th><th>Domain</th><th>Status</th><th>Address</th><th>Kind</th></tr></thead>
					<tbody>
						{rows.map((account) => (
							<tr key={account.id}>
								<td><Link to={`/accounts/${account.id}`}>{account.username}</Link></td>
								<td>{account.primary_domain}</td>
								<td><StatusBadge value={account.status} /></td>
								<td>{account.ip_address || '—'}</td>
								<td>{addressKind(account.ip_address || '')}</td>
							</tr>
						))}
					</tbody>
				</table></div>
			) : null}
			{!loading && !rows.length ? <EmptyState title="No IP assignments match" detail="Accounts without a dedicated address still appear when the search is empty." /> : null}
		</>
	)
}
