import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { WidgetCard } from '../components/widget-card'
import { ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { messageFrom } from '../helpers'
import { discoverTools, groupTools, toolCatalog } from '../tool-catalog'
import { hasCapabilities, useCapabilities } from '../rbac'
import type { Account, ServerOverview } from '../types'

const featuredToolIds = [
	'list-accounts', 'create-account', 'list-domains', 'websites', 'file-manager', 'sql',
	'email', 'deliverability', 'webmail', 'ssl', 'dns', 'services',
	'processes', 'security', 'jobs', 'updates',
]

export function HomePage () {
	const capabilities = useCapabilities()
	const canCreate = hasCapabilities(capabilities, ['accounts.create', 'packages.read'])
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	const tools = discoverTools(toolCatalog, capabilities)
	const toolById = new Map(tools.map((tool) => [tool.id, tool]))
	const featured = featuredToolIds.map((id) => toolById.get(id)).filter(Boolean)
	const grouped = groupTools(tools.filter((tool) => tool.path !== '/'))

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
		'list-accounts': { value: server?.stats.accounts ?? accounts.length, detail: `${accounts.filter((account) => account.status === 'suspended').length} suspended` },
		accounts: { value: server?.stats.accounts ?? accounts.length, detail: `${accounts.filter((account) => account.status === 'suspended').length} suspended` },
		services: { value: server ? `${server.services.filter((service) => service.observed_running).length}/${server.services.length}` : '—', detail: 'services running' },
		jobs: { value: '—', detail: 'View in Jobs' },
	}

	return (
		<>
			<PageHeader title="Home" description="Host operations, account health, and frequently used administration tools." actions={canCreate ? <Link className="button-link" to="/accounts/create">Create account</Link> : undefined} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{server ? <section className="home-status-strip" aria-label="Account and service health">
				<article><span>Accounts</span><strong>{server.stats.accounts ?? accounts.length}</strong><small>{accounts.filter((account) => account.status === 'suspended').length} suspended</small></article>
				<article><span>Services</span><strong>{`${server.services.filter((service) => service.observed_running).length}/${server.services.length}`}</strong><small>running</small></article>
				<article><span>Failed jobs</span><strong>{server.stats.failedJobs}</strong><small>open in Jobs</small></article>
				<article><span>Host</span><strong>{system?.hostname || '—'}</strong><small>Vitals are in Host resources</small></article>
			</section> : null}
			<nav className="home-quick-links" aria-label="Operations shortcuts">
				{toolById.has('services') ? <Link to="/status"><strong>Service status</strong><span>Managed services and host vitals</span></Link> : null}
				{toolById.has('jobs') ? <Link to="/jobs"><strong>Jobs</strong><span>Background work and retries</span></Link> : null}
				{capabilities['security.audit.read'] ? <Link to="/audit"><strong>Audit trail</strong><span>Privileged activity history</span></Link> : null}
			</nav>
			<SectionHeading title="Administration" detail="Shortcuts to common server and account tools. Every WHM-mapped surface is listed below — nothing is hidden for missing privileges." />
			<div className="widget-grid">
				{featured.map((tool, index) => (
					<WidgetCard
						key={tool!.id}
						id={tool!.id}
						label={tool!.label}
						description={tool!.description}
						path={tool!.path}
						icon={tool!.icon}
						toneIndex={index}
						value={widgetMetrics[tool!.id]?.value}
						detail={widgetMetrics[tool!.id]?.detail}
					/>
				))}
			</div>
			{grouped.size ? <>
				<SectionHeading title="All tools" detail="Every Director tool grouped the way WHM groups its panel, including journeys that write through settings, jobs, or the typed agent." />
				<div className="tool-groups">
					{[...grouped.entries()].map(([category, entries]) => <section className="panel tool-group" key={category}><h3>{category}</h3>{entries.map((tool) => <Link key={tool.id} to={tool.path}><strong>{tool.label}</strong><span>{tool.description}</span></Link>)}</section>)}
				</div>
			</> : null}
		</>
	)
}
