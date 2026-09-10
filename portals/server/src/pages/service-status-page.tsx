import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { Dialog, ErrorState, LoadingState, Metric, PageHeader, SectionHeading, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom, percent } from '../helpers'
import { useCan } from '../rbac'
import type { ServerOverview, Service } from '../types'
import { isDisruptiveServiceAction, serviceActionImpact, type ServiceAction } from './service-control-copy'

interface ProcessObservation {
	pid: number
	name: string
	scope: string
}
interface ProcessesResponse { processes?: ProcessObservation[] }
interface PendingControl {
	name: string
	action: ServiceAction
}
interface LastAction {
	action: ServiceAction
	at: string
}

const serviceActions: ServiceAction[] = ['reload', 'restart', 'start', 'stop']

export function ServiceStatusPage () {
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [services, setServices] = useState<Service[]>([])
	const [processes, setProcesses] = useState<ProcessObservation[]>([])
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [loading, setLoading] = useState(true)
	const [updatedAt, setUpdatedAt] = useState('')
	const [pending, setPending] = useState<PendingControl | null>(null)
	const [lastActions, setLastActions] = useState<Record<string, LastAction>>({})
	const canControl = useCan('server.services.restart')
	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([api<ServerOverview>('/api/v1/server'), api<ProcessesResponse>('/api/v1/server/processes')]).then(([overview, processResult]) => {
			if (overview.status === 'fulfilled') {
				setServer(overview.value)
				setServices(overview.value.services)
				setUpdatedAt(new Date().toISOString())
			} else setError(messageFrom(overview.reason))
			if (processResult.status === 'fulfilled') setProcesses(processResult.value.processes || [])
		}).finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function confirmControl () {
		if (!pending) return
		const { name, action } = pending
		setMessage('')
		try {
			await api(`/api/v1/server/services/${encodeURIComponent(name)}/${action}`, { method: 'POST', body: '{}' })
			setLastActions((current) => ({ ...current, [name]: { action, at: new Date().toISOString() } }))
			setMessage(`${action} queued for ${name}. Track the resulting job in Jobs.`)
			setPending(null)
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	const system = server?.system
	return <>
		<PageHeader title="Server & Service Status" description="Measured host vitals, managed service state, and service control actions." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
		{message ? <p className="feedback" role="status">{message}</p> : null}
		{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Reading host telemetry…" /> : null}
		<section className="metric-grid"><Metric label="Hostname" value={system?.hostname || '—'} /><Metric label="Load (1m)" value={system ? system.load1.toFixed(2) : '—'} /><Metric label="Memory" value={system ? `${percent(system.memory_used, system.memory_total)}%` : '—'} detail={system ? `${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}` : ''} /><Metric label="Disk" value={system ? `${percent(system.disk_used, system.disk_total)}%` : '—'} detail={system ? `${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}` : ''} /><Metric label="Uptime" value={system ? `${Math.floor(system.uptime_seconds / 3600)} hours` : '—'} /></section>
		<section className="panel"><SectionHeading title="Managed services" detail="Reload and start stay secondary. Restart and stop are disruptive and ask for impact confirmation. Last action times are recorded in this session only." /><div className="table-wrap"><table className="dense-table"><thead><tr><th>Service</th><th>Health</th><th>Desired</th><th>Observed</th><th>Last action</th>{canControl ? <th>Actions</th> : null}</tr></thead><tbody>{(services.length ? services : server?.services || []).map((service) => <tr key={service.name}><td><strong>{service.name}</strong></td><td><StatusBadge value={service.health} /></td><td>{service.desired_enabled ? 'Enabled' : 'Disabled'}</td><td>{service.observed_running ? 'Running' : 'Stopped'}</td><td>{lastActions[service.name] ? <>{lastActions[service.name].action}<small>{formatDate(lastActions[service.name].at)}</small></> : '—'}</td>{canControl ? <td><div className="row-actions">{serviceActions.map((action) => <button key={action} type="button" className={isDisruptiveServiceAction(action) ? 'link-button danger-text' : 'link-button'} onClick={() => setPending({ name: service.name, action })}>{action}</button>)}</div></td> : null}</tr>)}</tbody></table></div></section>
		<section className="panel"><SectionHeading title="Current control-plane process" detail="This endpoint identifies only the API process serving the request; it does not claim host-wide process telemetry." /><div className="table-wrap"><table><thead><tr><th>PID</th><th>Process</th><th>Scope</th></tr></thead><tbody>{processes.map((process) => <tr key={process.pid}><td>{process.pid}</td><td><code>{process.name}</code></td><td>{process.scope}</td></tr>)}</tbody></table></div></section>
		<Dialog open={Boolean(pending)} title={pending ? `${pending.action} ${pending.name}` : 'Service action'} onClose={() => setPending(null)} actions={<>
			<button type="button" className="secondary" onClick={() => setPending(null)}>Cancel</button>
			<button type="button" className={pending && isDisruptiveServiceAction(pending.action) ? 'danger' : undefined} onClick={() => void confirmControl()}>{pending ? `Confirm ${pending.action}` : 'Confirm'}</button>
		</>}>
			{pending ? <>
				<p>{serviceActionImpact(pending.action)}</p>
				<p className="subtle">The API does not return a per-service last-action timestamp. This page records the time locally after you confirm. Queued work appears in <Link to="/jobs">Jobs</Link>.</p>
			</> : null}
		</Dialog>
	</>
}
