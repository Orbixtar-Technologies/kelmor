import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { WidgetCard } from '../components/widget-card'
import { ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { formatBytes, messageFrom, percent } from '../helpers'
import { LOCAL_SETTINGS_LABEL, isLocalSettingsTool } from '../catalog-honesty'
import { hrefForFeature, hrefForTool } from '../nav-hubs'
import { failedJobDisplay, measuredVital, operatorNavGroups } from '../operator-nav'
import { discoverTools, toolCatalog } from '../tool-catalog'
import { featureById } from '../whm-catalog'
import { hasCapabilities, useCapabilities } from '../rbac'
import { describeJobFailure, formatJobType } from './job-copy'
import { resolveFavoriteIds } from './theme-favorites'
import type { Account, Job, ServerOverview } from '../types'

export function HomePage () {
	const capabilities = useCapabilities()
	const canCreate = hasCapabilities(capabilities, ['accounts.create', 'packages.read'])
	const canListAccounts = Boolean(capabilities['accounts.read'])
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [failedJobItems, setFailedJobItems] = useState<Job[]>([])
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	const [favoriteRaw, setFavoriteRaw] = useState<string | undefined>()
	const tools = discoverTools(toolCatalog, capabilities)
	const toolById = new Map(tools.map((tool) => [tool.id, tool]))
	const featuredIds = resolveFavoriteIds(favoriteRaw)
	const featured = featuredIds.map((id) => toolById.get(id)).filter(Boolean)
	const hasCustomFavorites = Boolean(favoriteRaw && favoriteRaw.trim())
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
		if (capabilities['server.read'] || capabilities['accounts.read']) {
			requests.push(api<{ items: Job[] }>('/api/v1/jobs?state=failed').then((result) => setFailedJobItems(asList(result))).catch(() => setFailedJobItems([])))
		}
		if (capabilities['server.read'] || capabilities['server.settings.write']) {
			requests.push(api<{ values?: Record<string, Record<string, string>> }>('/api/v1/server/settings')
				.then((result) => setFavoriteRaw(result.values?.theme?.favorites))
				.catch(() => setFavoriteRaw(undefined)))
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
				<article className={failedJobs.tone === 'bad' ? 'home-vital-alert' : undefined}>
					<span>Failed jobs</span>
					<strong>{failedJobs.value}</strong>
					<small>{failedJobs.detail}</small>
				</article>
			</section>
			{failedJobs.count > 0 || failedJobItems.length ? <section className="panel home-failed-jobs" aria-label="Failed job details">
				<h2>Failed jobs</h2>
				<p>This host has {failedJobs.detail}. Review the queue and retry individual operations when you are ready — Director does not mass-retry.</p>
				{failedJobItems.length ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Operation</th><th>Resource</th><th>Reason</th><th /></tr></thead>
					<tbody>
						{failedJobItems.slice(0, 8).map((job) => {
							const failure = describeJobFailure(job)
							return (
								<tr key={job.id}>
									<td><strong>{formatJobType(job.type)}</strong></td>
									<td>{failure.resource}</td>
									<td className="truncate">{failure.reason}</td>
									<td><Link className="link-button" to={`/jobs?state=failed&selected=${job.id}`}>Details</Link></td>
								</tr>
							)
						})}
					</tbody>
				</table></div> : null}
				<p><Link to="/jobs?state=failed">Open failed jobs</Link>{failedJobItems.length > 8 ? ` · showing 8 of ${failedJobItems.length}` : ''}</p>
			</section> : null}
			<nav className="home-quick-links" aria-label="Operations shortcuts">
				{canCreate ? <Link to="/accounts/create"><strong>Create account</strong><span>Identity, package, review, then queue provision</span></Link> : null}
				{canListAccounts ? <Link to="/accounts"><strong>List accounts</strong><span>Search, sort, and operate tenants</span></Link> : null}
				{toolById.has('jobs') ? <Link to="/jobs?state=failed"><strong>Failed jobs</strong><span>{failedJobs.detail}</span></Link> : null}
				{toolById.has('services') ? <Link to="/status"><strong>Service health</strong><span>Managed services and host probes</span></Link> : null}
			</nav>
			<SectionHeading title={hasCustomFavorites ? 'Favorite tools' : 'Frequent tools'} detail={hasCustomFavorites ? 'Pinned in Theme Manager. Firewall and reboot stay under Security.' : 'Account, job, and service work first. Firewall and reboot stay under Security.'} />
			<div className="widget-grid" aria-label={hasCustomFavorites ? 'Favorite tools' : 'Frequent tools'}>
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
				<SectionHeading title="All tools" detail="Dedicated managers and host-settings tiles talk to the API and Agent. Tiles marked Settings (local) are policy records that are not a live host product yet." />
				<div className="tool-groups">
					{grouped.map((group) => (
						<section className="panel tool-group" key={group.id}>
							<h3>{group.label}</h3>
							{group.tools.map((tool) => (
								<Link key={tool.id} to={hrefForTool(tool)}>
									<strong>{tool.label}{isLocalSettingsTool(tool) ? <em className="nav-local-badge">{LOCAL_SETTINGS_LABEL}</em> : null}</strong>
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
