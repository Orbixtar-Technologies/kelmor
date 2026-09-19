import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { formatBytes, messageFrom, percent } from '../helpers'
import { useCan } from '../rbac'

interface ResellerUsageRow {
	id: string
	name: string
	brand_name?: string
	status: string
	accounts: number
	active: number
	suspended: number
	disk_bytes: number
	disk_limit: number
	bandwidth_bytes: number
	bandwidth_limit: number
	bandwidth_holds: number
}

export function ResellerUsagePage () {
	const canReset = useCan('accounts.modify')
	const [rows, setRows] = useState<ResellerUsageRow[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')

	function load () {
		setLoading(true)
		setError('')
		api<{ items: ResellerUsageRow[] }>('/api/v1/resellers/usage')
			.then((result) => setRows(asList(result)))
			.catch((reason) => setError(messageFrom(reason)))
			.finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function handleReset (resellerId: string) {
		setError('')
		try {
			const result = await api<{ operations?: string[] }>(`/api/v1/resellers/${resellerId}/bandwidth/reset`, { method: 'POST', body: '{}' })
			setJobId(result.operations?.[0] || '')
			setMessage(queuedOpMessage({ operation_id: result.operations?.[0] }, 'Reseller bandwidth reset queued.'))
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	return (
		<>
			<PageHeader
				title="View Reseller Usage and Manage Account Status"
				description="Disk and monthly bandwidth totals from control-plane usage, against the packages assigned to each reseller’s accounts."
			/>
			<p className="subtle">
				<Link to="/resellers">Edit reseller</Link> · <Link to="/accounts/ownership">Change ownership</Link> · <Link to="/accounts/suspension">Suspension</Link>
			</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <QueuedOpNotice message={message} jobId={jobId} /> : null}
			{loading ? <LoadingState label="Loading reseller usage…" /> : (
				<div className="table-wrap"><table className="dense-table">
					<thead>
						<tr>
							<th>Reseller</th>
							<th>Status</th>
							<th>Accounts</th>
							<th>Disk</th>
							<th>Bandwidth</th>
							<th>Holds</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{rows.map((row) => (
							<tr key={row.id}>
								<td><strong>{row.name}</strong><small>{row.brand_name || 'No brand'}</small></td>
								<td><StatusBadge value={row.status} /></td>
								<td>{row.accounts} · {row.active} active · {row.suspended} suspended</td>
								<td>{formatBytes(row.disk_bytes)} / {formatBytes(row.disk_limit)} ({percent(row.disk_bytes, row.disk_limit)}%)</td>
								<td>{formatBytes(row.bandwidth_bytes)} / {formatBytes(row.bandwidth_limit)} ({percent(row.bandwidth_bytes, row.bandwidth_limit)}%)</td>
								<td>{row.bandwidth_holds}</td>
								<td><div className="row-actions">
									<Link to="/accounts/ownership">Ownership</Link>
									<Link to="/accounts/suspension">Suspension</Link>
									{canReset ? <button type="button" onClick={() => handleReset(row.id)}>Reset bandwidth</button> : null}
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
