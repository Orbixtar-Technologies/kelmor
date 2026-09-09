import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, Metric, PageHeader, SectionHeading, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom, percent } from '../helpers'
import { discoverTools, groupTools, toolCatalog } from '../tool-catalog'
import { hasCapabilities, useCapabilities } from '../rbac'
import type { Account, AuditEvent, Job, ServerOverview } from '../types'

export function HomePage () {
	const capabilities = useCapabilities()
	const canCreate = hasCapabilities(capabilities, ['accounts.create', 'packages.read'])
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [jobs, setJobs] = useState<Job[]>([])
	const [activity, setActivity] = useState<AuditEvent[]>([])
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	const tools = discoverTools(toolCatalog, capabilities)
	const toolIds = new Set(tools.map((tool) => tool.id))

	function load () {
		setLoading(true)
		setError('')
		const requests: Array<Promise<void>> = []
		if (capabilities['server.read']) {
			requests.push(api<ServerOverview>('/api/v1/server').then(setServer).catch((reason) => setError(messageFrom(reason))))
		}
		if (capabilities['accounts.read']) {
			requests.push(api<{ items: Account[] }>('/api/v1/accounts').then((result) => setAccounts(asList(result))).catch((reason) => setError(messageFrom(reason))))
		}
		if (capabilities['server.read'] || capabilities['accounts.read']) {
			requests.push(api<{ items: Job[] }>('/api/v1/jobs').then((result) => setJobs(asList(result))).catch((reason) => setError(messageFrom(reason))))
		}
		if (capabilities['security.audit.read']) {
			requests.push(api<{ items: AuditEvent[] }>('/api/v1/audit-events').then((result) => setActivity(asList(result))).catch((reason) => setError(messageFrom(reason))))
		}
		Promise.allSettled(requests).finally(() => setLoading(false))
	}

	useEffect(load, [])
	if (loading) return <><PageHeader title="Home" description="Live host health and operator activity." /><LoadingState label="Loading Kelmor Director overview…" /></>
	const system = server?.system
	const runningJobs = jobs.filter((job) => ['queued', 'running'].includes(job.state)).length
	const grouped = groupTools(tools.filter((tool) => tool.path !== '/'))
	return (
		<>
			<PageHeader title="Home" description="Host operations, account health, and frequently used administration tools." actions={canCreate ? <Link className="button-link" to="/accounts/create">Create account</Link> : undefined} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			<SectionHeading title="Favorites" detail="Fast access to common operator journeys." />
			<div className="favorite-tools">
				{tools.filter((tool) => ['accounts', 'create-account', 'dns', 'jobs', 'services'].includes(tool.id)).map((tool) => (
					<Link key={tool.id} to={tool.path}><span aria-hidden="true">◆</span><strong>{tool.label}</strong><small>{tool.description}</small></Link>
				))}
			</div>
			<section className="metric-grid">
				<Metric label="System load" value={system ? Number(system.load1).toFixed(2) : '—'} detail="1 minute" />
				<Metric label="Memory" value={system ? `${percent(system.memory_used, system.memory_total)}%` : '—'} detail={system ? `${formatBytes(system.memory_used)} of ${formatBytes(system.memory_total)}` : ''} />
				<Metric label="Disk" value={system ? `${percent(system.disk_used, system.disk_total)}%` : '—'} detail={system ? `${formatBytes(system.disk_used)} of ${formatBytes(system.disk_total)}` : ''} />
				<Metric label="Accounts" value={server?.stats.accounts ?? accounts.length} detail={`${accounts.filter((account) => account.status === 'suspended').length} suspended`} />
				<Metric label="Active jobs" value={runningJobs} detail={`${jobs.filter((job) => job.state === 'failed').length} failed`} />
			</section>
			<div className="home-columns">
				<section className="panel">
					<SectionHeading title="Service status" action={toolIds.has('services') ? <Link to="/status">View details</Link> : undefined} />
					<div className="table-wrap"><table><thead><tr><th>Service</th><th>Health</th><th>Observed</th></tr></thead><tbody>
						{server?.services.map((service) => <tr key={service.name}><td>{service.name}</td><td><StatusBadge value={service.health} /></td><td>{service.observed_running ? 'Running' : 'Stopped'}</td></tr>)}
					</tbody></table></div>
					{!server?.services.length ? <EmptyState title="No service readings" detail="Open Service Status to retry host telemetry." /> : null}
				</section>
				<section className="panel">
					<SectionHeading title="Recent jobs" action={toolIds.has('jobs') ? <Link to="/jobs">All jobs</Link> : undefined} />
					<ul className="activity-list">
						{jobs.slice(0, 7).map((job) => <li key={job.id}><span><strong>{job.type}</strong><small>{formatDate(job.created_at)}</small></span><StatusBadge value={job.state} /></li>)}
					</ul>
					{jobs.length === 0 ? <EmptyState title="No recent jobs" detail="Provisioning and maintenance operations appear here." /> : null}
				</section>
			</div>
			<SectionHeading title="Tools" detail="Capability-aware administration tools grouped by task." />
			<div className="tool-groups">
				{[...grouped.entries()].map(([category, entries]) => <section className="panel tool-group" key={category}><h3>{category}</h3>{entries.map((tool) => <Link key={tool.id} to={tool.path}><strong>{tool.label}</strong><span>{tool.description}</span></Link>)}</section>)}
			</div>
			{capabilities['security.audit.read'] ? <section className="panel">
				<SectionHeading title="Recent activity" action={<Link to="/audit">Open audit trail</Link>} />
				<ul className="activity-list">{activity.slice(0, 8).map((event) => <li key={event.id}><span><strong>{event.action}</strong><small>{event.resource_type || 'system'} · {formatDate(event.occurred_at)}</small></span><StatusBadge value={event.success} /></li>)}</ul>
				{activity.length === 0 ? <EmptyState title="No visible activity" detail="Audit entries appear as privileged work occurs." /> : null}
			</section> : null}
		</>
	)
}
