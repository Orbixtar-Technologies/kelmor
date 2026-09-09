import { useEffect, useState } from 'react'
import { api } from '../client'
import { ErrorState, LoadingState, Metric, PageHeader, SectionHeading, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, percent } from '../helpers'
import type { ResourceItem, ServerOverview, Service } from '../types'

interface ServicesResponse { items?: Service[]; services?: Service[] }
interface ProcessesResponse { processes?: ResourceItem[] }

export function ServiceStatusPage () {
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [services, setServices] = useState<Service[]>([])
	const [processes, setProcesses] = useState<ResourceItem[]>([])
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	function load () {
		setLoading(true); setError('')
		Promise.allSettled([api<ServerOverview>('/api/v1/server'), api<ServicesResponse>('/api/v1/server/services'), api<ProcessesResponse>('/api/v1/server/processes')]).then(([overview, serviceResult, processResult]) => {
			if (overview.status === 'fulfilled') setServer(overview.value)
			else setError(messageFrom(overview.reason))
			if (serviceResult.status === 'fulfilled') setServices(serviceResult.value.items || serviceResult.value.services || [])
			if (processResult.status === 'fulfilled') setProcesses(processResult.value.processes || [])
		}).finally(() => setLoading(false))
	}
	useEffect(load, [])
	const system = server?.system
	return <>
		<PageHeader title="Server & Service Status" description="Measured host vitals, managed service state, and current process observations." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Reading host telemetry…" /> : null}
		<section className="metric-grid"><Metric label="Hostname" value={system?.hostname || '—'} /><Metric label="Load (1m)" value={system ? system.load1.toFixed(2) : '—'} /><Metric label="Memory" value={system ? `${percent(system.memory_used, system.memory_total)}%` : '—'} detail={system ? `${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}` : ''} /><Metric label="Disk" value={system ? `${percent(system.disk_used, system.disk_total)}%` : '—'} detail={system ? `${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}` : ''} /><Metric label="Uptime" value={system ? `${Math.floor(system.uptime_seconds / 3600)} hours` : '—'} /></section>
		<section className="panel"><SectionHeading title="Managed services" detail="Desired state is compared with observed process state." /><div className="table-wrap"><table><thead><tr><th>Service</th><th>Health</th><th>Desired</th><th>Observed</th></tr></thead><tbody>{(services.length ? services : server?.services || []).map((service) => <tr key={service.name}><td><strong>{service.name}</strong></td><td><StatusBadge value={service.health} /></td><td>{service.desired_enabled ? 'Enabled' : 'Disabled'}</td><td>{service.observed_running ? 'Running' : 'Stopped'}</td></tr>)}</tbody></table></div></section>
		<section className="panel"><SectionHeading title="Processes" detail="A focused view of processes reported by the host API." /><div className="table-wrap"><table><thead><tr><th>PID</th><th>User</th><th>Command</th><th>CPU</th><th>RSS</th></tr></thead><tbody>{processes.map((process) => <tr key={process.id || String(process.pid)}><td>{String(process.pid)}</td><td>{String(process.user)}</td><td><code>{String(process.cmd)}</code></td><td>{String(process.cpu)}%</td><td>{String(process.rss)} MB</td></tr>)}</tbody></table></div></section>
	</>
}
