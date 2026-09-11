import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { HostProcess } from '../types'

const PROTECTED_PROCESS_NAMES = new Set(['systemd', 'init', 'panel-agent', 'panel-api'])

function isProtectedProcess (process: HostProcess) {
	return process.pid <= 1 || PROTECTED_PROCESS_NAMES.has(process.name)
}

export function ProcessManagerPage () {
	const canSignal = useCan('server.settings.write')
	const [processes, setProcesses] = useState<HostProcess[]>([])
	const [query, setQuery] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const [busyPid, setBusyPid] = useState(0)

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

	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase()
		if (!needle) return processes
		return processes.filter((process) => {
			const haystack = [process.name, process.user, process.command, process.scope, String(process.pid)]
				.filter(Boolean)
				.join(' ')
				.toLowerCase()
			return haystack.includes(needle)
		})
	}, [processes, query])

	async function signal (pid: number, signalName: 'TERM' | 'KILL') {
		if (!canSignal) return
		if (!window.confirm(`Send SIG${signalName} to pid ${pid}?`)) return
		setBusyPid(pid)
		setMessage('')
		try {
			const result = await api<{ message?: string }>(`/api/v1/server/processes/${pid}/signal`, {
				method: 'POST',
				body: JSON.stringify({ signal: signalName }),
			})
			setMessage(result.message || `Sent ${signalName} to pid ${pid}.`)
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusyPid(0)
		}
	}

	return (
		<>
			<PageHeader
				title="Process Manager"
				description="Live /proc snapshot from the privileged agent. TERM and KILL are typed signals; pid 1, systemd, init, and the control-plane binaries stay protected."
				actions={<Link className="button-link secondary-link" to="/status">Service Status</Link>}
			/>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel">
				<h2>Host processes</h2>
				<p className="subtle">Restart managed daemons from Service Status. Use signals here only for leftover or tenant processes.</p>
				<label>Filter
					<input
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="PID, user, name, or command"
					/>
				</label>
				{loading ? <LoadingState label="Loading process snapshot…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead>
						<tr>
							<th>PID</th>
							<th>User</th>
							<th>Name</th>
							<th>Scope</th>
							<th>Command</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{filtered.map((process) => {
							const isProtected = isProtectedProcess(process)
							return (
								<tr key={`${process.pid}-${process.name}`}>
									<td><code>{process.pid}</code></td>
									<td>{process.user || '—'}</td>
									<td>{process.name}</td>
									<td>{process.scope || '—'}</td>
									<td>{process.command || process.name}</td>
									<td>
										{canSignal && !isProtected ? (
											<div className="row-actions">
												<button
													type="button"
													className="link-button"
													disabled={busyPid === process.pid}
													onClick={() => void signal(process.pid, 'TERM')}
												>
													TERM
												</button>
												<button
													type="button"
													className="link-button danger-text"
													disabled={busyPid === process.pid}
													onClick={() => void signal(process.pid, 'KILL')}
												>
													KILL
												</button>
											</div>
										) : (
											<span className="subtle">protected</span>
										)}
									</td>
								</tr>
							)
						})}
					</tbody>
				</table></div> : null}
				{!loading && !filtered.length ? <EmptyState title="No matching processes" detail="The host did not return a process that matches this filter." /> : null}
			</section>
		</>
	)
}
