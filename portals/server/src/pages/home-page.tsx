import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { WidgetCard } from '../components/widget-card'
import { ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { formatBytes, messageFrom, percent } from '../helpers'
import { hrefForFeature, hrefForTool } from '../nav-hubs'
import { failedJobDisplay, measuredVital, operatorNavGroups } from '../operator-nav'
import { discoverTools, toolCatalog } from '../tool-catalog'
import { featureById } from '../whm-catalog'
import { hasCapabilities, useCapabilities } from '../rbac'
import type { Account, ServerOverview } from '../types'

const featuredToolIds = [
	'list-accounts', 'create-account', 'jobs', 'services',
	'packages', 'dns', 'email', 'sql',
]

export function HomePage () {
	const capabilities = useCapabilities()
	const canCreate = hasCapabilities(capabilities, ['accounts.create', 'packages.read'])
	const canListAccounts = Boolean(capabilities['accounts.read'])
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	const tools = discoverTools(toolCatalog, capabilities)
	const toolById = new Map(tools.map((tool) => [tool.id, tool]))
	const featured = featuredToolIds.map((id) => toolById.get(id)).filter(Boolean)
	const grouped = operatorNavGroups(tools)
	const failedJobs = failedJobDisplay(server?.stats.failedJobs)
	const runningServices = server
		? server.services.filter((service) => service.observed_running).length
		: undefined

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
		Promise.allSettled(requests).finally(() => setLoading(false))
	}

	useEffect(load, [])
	if (loading) return <><PageHeader title="Home" description="Live host health and operator activity." /><LoadingState label="Loading Kelmor Director overview…" /></>
	const system = server?.system
	const widgetMetrics: Record<string, { value?: string | number; detail?: string }> = {
		'list-accounts': {
			value: server?.stats.accounts ?? accounts.length,
			detail: `${accounts.filter((account) => account.status === 'suspended').length} suspended`,
		},
		services: {
			value: server && runningServices !== undefined ? `${runningServices}/${server.services.length}` : 'Not reported',
			detail: 'services running',
		},
		jobs: failedJobs,
	}

	return (
		<>
			<PageHeader title="Home" description="Host vitals, account health, and the operator tools you use first." actions={canCreate ? <Link className="button-link" to="/accounts/create">Create account</Link> : undefined} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			<section className="home-status-strip" aria-label="Host vitals">
				<article>
					<span>Load</span>
					<strong>{measuredVital(Boolean(system), system ? system.load1.toFixed(2) : '')}</strong>
					<small>{system ? '1 minute average' : 'Requires server.read'}</small>
				</article>
				<article>
					<span>Memory</span>
					<strong>{measuredVital(Boolean(system), system ? `${percent(system.memory_used, system.memory_total)}%` : '')}</strong>
					<small>{system ? `${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}` : 'Not reported'}</small>
				</article>
				<article>
					<span>Disk</span>
					<strong>{measuredVital(Boolean(system), system ? `${percent(system.disk_used, system.disk_total)}%` : '')}</strong>
					<small>{system ? `${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}` : 'Not reported'}</small>
				</article>
				<article>
					<span>Accounts</span>
					<strong>{server?.stats.accounts ?? accounts.length}</strong>
					<small>{accounts.filter((account) => account.status === 'suspended').length} suspended</small>
				</article>
				<article>
					<span>Services</span>
					<strong>{measuredVital(Boolean(server), server && runningServices !== undefined ? `${runningServices}/${server.services.length}` : '')}</strong>
					<small>agent-observed health</small>
				</article>
				<article>
					<span>Failed jobs</span>
					<strong>{failedJobs.value}</strong>
					<small>{failedJobs.detail}</small>
				</article>
			</section>
			<nav className="home-quick-links" aria-label="Operations shortcuts">
				{canCreate ? <Link to="/accounts/create"><strong>Create account</strong><span>Identity, package, review, then queue provision</span></Link> : null}
				{canListAccounts ? <Link to="/accounts"><strong>List accounts</strong><span>Search, sort, and operate tenants</span></Link> : null}
				{toolById.has('jobs') ? <Link to="/jobs?state=failed"><strong>Failed jobs</strong><span>{failedJobs.detail}</span></Link> : null}
				{toolById.has('services') ? <Link to="/status"><strong>Service health</strong><span>Managed services and host probes</span></Link> : null}
			</nav>
			<SectionHeading title="Frequent tools" detail="Account, job, and service work first. Firewall and reboot stay under Security." />
			<div className="widget-grid">
				{featured.map((tool, index) => {
					const feature = featureById(tool!.id)
					return (
						<WidgetCard
							key={tool!.id}
							id={tool!.id}
							label={tool!.label}
							description={tool!.description}
							path={feature ? hrefForFeature(feature) : tool!.path}
							icon={tool!.icon}
							toneIndex={index}
							value={widgetMetrics[tool!.id]?.value}
							detail={widgetMetrics[tool!.id]?.detail}
						/>
					)
				})}
			</div>
			{grouped.length ? <>
				<SectionHeading title="All tools" detail="Every Director tool, grouped in WHM-style categories that match the sidebar. Missing APIs stay as honest empty or disabled states." />
				<div className="tool-groups">
					{grouped.map((group) => (
						<section className="panel tool-group" key={group.id}>
							<h3>{group.label}</h3>
							{group.tools.map((tool) => (
								<Link key={tool.id} to={hrefForTool(tool)}>
									<strong>{tool.label}</strong>
									<span>{tool.description}</span>
								</Link>
							))}
						</section>
					))}
				</div>
			</> : null}
		</>
	)
}
