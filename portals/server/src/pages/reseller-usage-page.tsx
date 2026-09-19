import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import type { Account, Reseller } from '../types'

export function ResellerUsagePage () {
	const [resellers, setResellers] = useState<Reseller[]>([])
	const [accounts, setAccounts] = useState<Account[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: Reseller[] }>('/api/v1/resellers'),
			api<{ items: Account[] }>('/api/v1/accounts'),
		]).then(([resellerResult, accountResult]) => {
			if (resellerResult.status === 'fulfilled') setResellers(asList(resellerResult.value))
			else setError(messageFrom(resellerResult.reason))
			if (accountResult.status === 'fulfilled') setAccounts(asList(accountResult.value))
		}).finally(() => setLoading(false))
	}
	useEffect(load, [])

	const rows = useMemo(() => resellers.map((reseller) => {
		const owned = accounts.filter((account) => account.reseller_id === reseller.id)
		return {
			reseller,
			total: owned.length,
			suspended: owned.filter((account) => account.status === 'suspended').length,
			active: owned.filter((account) => account.status === 'active').length,
		}
	}), [accounts, resellers])

	return (
		<>
			<PageHeader
				title="View Reseller Usage and Manage Account Status"
				description="Assigned account counts for each reseller. Kelmor does not collect WHM-style reseller bandwidth meters."
			/>
			<p className="subtle">Open Change Ownership to move an account. Suspend or unsuspend from Manage Account Suspension.</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading reseller usage…" /> : (
				<div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Reseller</th><th>Status</th><th>Accounts</th><th>Active</th><th>Suspended</th><th>Actions</th></tr></thead>
					<tbody>
						{rows.map(({ reseller, total, active, suspended }) => (
							<tr key={reseller.id}>
								<td><strong>{reseller.name}</strong><small>{reseller.brand_name || 'No brand'}</small></td>
								<td><StatusBadge value={reseller.status} /></td>
								<td>{total}</td>
								<td>{active}</td>
								<td>{suspended}</td>
								<td><div className="row-actions">
									<Link to="/resellers">Edit reseller</Link>
									<Link to="/accounts/ownership">Change ownership</Link>
									<Link to="/accounts/suspension">Suspension</Link>
								</div></td>
							</tr>
						))}
					</tbody>
				</table></div>
			)}
			{!loading && !rows.length ? <EmptyState title="No resellers" detail="Create a reseller before reviewing usage." action={<Link className="button-link" to="/resellers">Open resellers</Link>} /> : null}
		</>
	)
}
