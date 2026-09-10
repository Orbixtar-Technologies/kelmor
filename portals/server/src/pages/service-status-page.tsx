import { useEffect, useState } from 'react'
import { api } from '../client'
import { ErrorState, LoadingState, Metric, PageHeader, SectionHeading, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, percent } from '../helpers'
import { useCan } from '../rbac'
import type { ServerOverview, Service } from '../types'

interface ProcessObservation {
	pid: number
	name: string
	scope: string
}
interface ProcessesResponse { processes?: ProcessObservation[] }

const serviceActions = ['reload', 'restart', 'start', 'stop'] as const

export function ServiceStatusPage () {
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [services, setServices] = useState<Service[]>([])
	const [processes, setProcesses] = useState<ProcessObservation[]>([])
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [loading, setLoading] = useState(true)
	const canControl = useCan('server.services.restart')
	function load () {
		setLoading(true); setError('')
		Promise.allSettled([api<ServerOverview>('/api/v1/server'), api<ProcessesResponse>('/api/v1/server/processes')]).then(([overview, processResult]) => {
			if (overview.status === 'fulfilled') {
				setServer(overview.value)
				setServices(overview.value.services)
			}
			else setError(messageFrom(overview.reason))
			if (processResult.status === 'fulfilled') setProcesses(processResult.value.processes || [])
		}).finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function controlService (name: string, action: typeof serviceActions[number]) {
		if (!window.confirm(`${action} ${name}?`)) return
		setMessage('')
		try {
			await api(`/api/v1/server/services/${encodeURIComponent(name)}/${action}`, { method: 'POST', body: '{}' })
			setMessage(`${action} queued for ${name}`)
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	const system = server?.system
	return <>
		<PageHeader title="Server & Service Status" description="Measured host vitals, managed service state, and service control actions." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		{message ? <p className="feedback" role="status">{message}</p> : null}
		{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Reading host telemetry…" /> : null}
		<section className="metric-grid"><Metric label="Hostname" value={system?.hostname || '—'} /><Metric label="Load (1m)" value={system ? system.load1.toFixed(2) : '—'} /><Metric label="Memory" value={system ? `${percent(system.memory_used, system.memory_total)}%` : '—'} detail={system ? `${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}` : ''} /><Metric label="Disk" value={system ? `${percent(system.disk_used, system.disk_total)}%` : '—'} detail={system ? `${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}` : ''} /><Metric label="Uptime" value={system ? `${Math.floor(system.uptime_seconds / 3600)} hours` : '—'} /></section>
		<section className="panel"><SectionHeading title="Managed services" detail="Restart, reload, start, or stop allow-listed host services." /><div className="table-wrap"><table><thead><tr><th>Service</th><th>Health</th><th>Desired</th><th>Observed</th>{canControl ? <th>Actions</th> : null}</tr></thead><tbody>{(services.length ? services : server?.services || []).map((service) => <tr key={service.name}><td><strong>{service.name}</strong></td><td><StatusBadge value={service.health} /></td><td>{service.desired_enabled ? 'Enabled' : 'Disabled'}</td><td>{service.observed_running ? 'Running' : 'Stopped'}</td>{canControl ? <td><div className="row-actions">{serviceActions.map((action) => <button key={action} type="button" className="link-button" onClick={() => controlService(service.name, action)}>{action}</button>)}</div></td> : null}</tr>)}</tbody></table></div></section>
		<section className="panel"><SectionHeading title="Current control-plane process" detail="This endpoint identifies only the API process serving the request; it does not claim host-wide process telemetry." /><div className="table-wrap"><table><thead><tr><th>PID</th><th>Process</th><th>Scope</th></tr></thead><tbody>{processes.map((process) => <tr key={process.pid}><td>{process.pid}</td><td><code>{process.name}</code></td><td>{process.scope}</td></tr>)}</tbody></table></div></section>
	</>
}
