import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import type { HostProcess } from '../types'

export function ProcessManagerPage () {
	const [processes, setProcesses] = useState<HostProcess[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')

	function load () {
		setLoading(true)
		setError('')
		api<{ processes?: HostProcess[] }>('/api/v1/server/processes').then((result) => {
			setProcesses(Array.isArray(result.processes) ? result.processes : [])
			setUpdatedAt(new Date().toISOString())
		}).catch((requestError) => {
			setError(messageFrom(requestError))
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])

	return (
		<>
			<PageHeader
				title="Process Manager"
				description="Inspect the live control-plane process snapshot. Kelmor does not expose arbitrary process kill from Director."
				actions={<Link className="button-link secondary-link" to="/status">Service Status</Link>}
			/>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			<section className="panel">
				<h2>Host processes</h2>
				<p className="subtle">This list is the bounded process view returned by the API. Use Service Status to restart managed services through typed jobs.</p>
				{loading ? <LoadingState label="Loading process snapshot…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>PID</th><th>Name</th><th>Scope</th></tr></thead>
					<tbody>
						{processes.map((process) => (
							<tr key={`${process.pid}-${process.name}`}>
								<td><code>{process.pid}</code></td>
								<td>{process.name}</td>
								<td>{process.scope || '—'}</td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading && !processes.length ? <EmptyState title="No process snapshot" detail="The host did not return a process list." /> : null}
			</section>
		</>
	)
}
