import { Link } from 'react-router-dom'
import { api, listOf } from '../client'
import { Icon } from '../components/icons'
import { EmptyState, Loading, Meter, Notice, Panel, Pill, statusTone } from '../components/ui'
import { useFavorites, useLoad } from '../lib/hooks'
import { formatBytes, formatCount, formatUptime, meterClass, usedPercent } from '../lib/format'
import { allTools, visibleCatalog } from '../nav/catalog'

interface SystemInfo {
	hostname: string
	load1: number
	load5?: number
	load15?: number
	memory_used: number
	memory_total: number
	disk_used: number
	disk_total: number
	inodes_used: number
	inodes_total: number
	uptime_seconds: number
	kernel?: string
}

interface ServiceRow {
	name: string
	health: string
	observed_running: boolean
}

interface Overview {
	system: SystemInfo
	stats: Record<string, number>
	services: ServiceRow[]
}

interface JobRow {
	id: string
	type: string
	state: string
}

export function Home ({ caps }: { caps: Record<string, boolean> }) {
	const overview = useLoad<Overview>(() => api<Overview>('/api/v1/server'), [], 30000)
	const jobs = useLoad<JobRow[]>(
		() => (caps['server.read'] || caps['accounts.read'] ? listOf<JobRow>('/api/v1/jobs') : Promise.resolve([])),
		[],
		15000,
	)
	const categories = visibleCatalog(caps)

	return (
		<>
			<div className="crumbs"><span>Home</span></div>
			<div className="page-head">
				<div>
					<h1>Home</h1>
					<p>
						Kelmor Director manages this host: accounts, packages, resellers, DNS, mail, backups and every privileged
						job. Pin the tools you use most and they appear in Favourites below.
					</p>
				</div>
			</div>

			<Favourites />

			<div className="grid-2">
				<Statistics overview={overview.data} loading={overview.loading} error={overview.error} jobs={jobs.data ?? []} />
				<ServerMonitoring overview={overview.data} loading={overview.loading} error={overview.error} />
			</div>

			{categories.length === 0 ? (
				<Panel title="Tools">
					<EmptyState icon="lock" title="No tools are available to your role">
						Your account has no Director capabilities. Ask a server administrator to grant a role.
					</EmptyState>
				</Panel>
			) : (
				categories.map((category) => (
					<Panel key={category.id} title={category.name} icon={category.icon} subtitle={`${category.tools.length} tools`}>
						<div className="tile-grid">
							{category.tools.map((tool) => (
								<Link key={tool.path} className="tile" to={tool.path}>
									<Icon name={tool.icon} size={19} className="ico" />
									<span>
										<strong>{tool.name}</strong>
										<small>{tool.description}</small>
									</span>
								</Link>
							))}
						</div>
					</Panel>
				))
			)}
		</>
	)
}

function Favourites () {
	const { favorites } = useFavorites()
	const pinned = favorites.map((path) => allTools.find((tool) => tool.path === path)).filter(Boolean)

	return (
		<Panel title="Favourites" icon="star" subtitle={`${pinned.length} pinned`}>
			{pinned.length === 0 ? (
				<Notice tone="info">
					No favourites yet. Open any tool and use the <Icon name="star" size={12} /> star beside its title to pin it here.
				</Notice>
			) : (
				<div className="tile-grid">
					{pinned.map((tool) => (
						<Link key={tool!.path} className="tile" to={tool!.path}>
							<Icon name={tool!.icon} size={19} className="ico" />
							<span>
								<strong>{tool!.name}</strong>
								<small>{tool!.description}</small>
							</span>
						</Link>
					))}
				</div>
			)}
		</Panel>
	)
}

function Statistics ({ overview, loading, error, jobs }: { overview: Overview | null; loading: boolean; error: string; jobs: JobRow[] }) {
	if (error) {
		return (
			<Panel title="Statistics" icon="barChart">
				<Notice tone="error">Host metrics are unavailable: {error}</Notice>
			</Panel>
		)
	}
	if (loading && !overview) {
		return <Panel title="Statistics" icon="barChart" tight><Loading label="Reading host sensors" /></Panel>
	}
	if (!overview) return null

	const s = overview.system
	const memPercent = usedPercent(s.memory_used, s.memory_total)
	const diskPercent = usedPercent(s.disk_used, s.disk_total)
	const inodePercent = usedPercent(s.inodes_used, s.inodes_total)
	const failed = jobs.filter((j) => j.state === 'failed' || j.state === 'dead').length
	const running = jobs.filter((j) => j.state === 'running' || j.state === 'queued').length

	return (
		<Panel
			title="Statistics"
			icon="barChart"
			subtitle="Observed by the privileged agent"
			actions={<Link className="btn secondary small" to="/server/vitals">Host vitals</Link>}
			tight
		>
			<ul className="stat-list">
				<li><span className="label">Hostname</span><span className="value mono">{s.hostname}</span></li>
				<li><span className="label">Load average</span><span className="value">{Number(s.load1 ?? 0).toFixed(2)}</span></li>
				<li>
					<span className="label">Memory</span>
					<Meter used={s.memory_used} limit={s.memory_total} className={meterClass(memPercent)} />
					<span className="value">{formatBytes(s.memory_used)} / {formatBytes(s.memory_total)}</span>
				</li>
				<li>
					<span className="label">Disk</span>
					<Meter used={s.disk_used} limit={s.disk_total} className={meterClass(diskPercent)} />
					<span className="value">{formatBytes(s.disk_used)} / {formatBytes(s.disk_total)}</span>
				</li>
				<li>
					<span className="label">Inodes</span>
					<Meter used={s.inodes_used} limit={s.inodes_total} className={meterClass(inodePercent)} />
					<span className="value">{formatCount(s.inodes_used)} / {formatCount(s.inodes_total)}</span>
				</li>
				<li><span className="label">Uptime</span><span className="value">{formatUptime(s.uptime_seconds)}</span></li>
				<li>
					<span className="label">Hosting accounts</span>
					<span className="value"><Link to="/accounts">{formatCount(overview.stats?.accounts ?? 0)}</Link></span>
				</li>
				<li>
					<span className="label">Jobs in flight</span>
					<span className="value"><Link to="/jobs">{formatCount(running)}</Link></span>
				</li>
				<li>
					<span className="label">Failed jobs</span>
					<span className="value">
						{failed > 0 ? <Link to="/jobs"><Pill tone="bad">{failed}</Pill></Link> : <Pill tone="ok">0</Pill>}
					</span>
				</li>
			</ul>
		</Panel>
	)
}

function ServerMonitoring ({ overview, loading, error }: { overview: Overview | null; loading: boolean; error: string }) {
	if (error) {
		return (
			<Panel title="Server monitoring" icon="activity">
				<Notice tone="error">Service health is unavailable: {error}</Notice>
			</Panel>
		)
	}
	if (loading && !overview) {
		return <Panel title="Server monitoring" icon="activity" tight><Loading label="Polling services" /></Panel>
	}

	const services = overview?.services ?? []
	const down = services.filter((s) => !s.observed_running)

	return (
		<Panel
			title="Server monitoring"
			icon="activity"
			subtitle={`${services.length - down.length} of ${services.length} running`}
			actions={<Link className="btn secondary small" to="/server/status">Service status</Link>}
			tight
		>
			{services.length === 0 ? (
				<EmptyState icon="activity" title="No services reported">The agent has not reported any managed service yet.</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead>
							<tr><th>Service</th><th>Health</th><th className="num">Running</th></tr>
						</thead>
						<tbody>
							{services.slice(0, 9).map((service) => (
								<tr key={service.name}>
									<td className="mono">{service.name}</td>
									<td><Pill tone={statusTone(service.health)}>{service.health}</Pill></td>
									<td className="num">{service.observed_running ? 'yes' : 'no'}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
			{services.length > 9 ? (
				<div className="table-footer">
					<span>{services.length - 9} more services</span>
					<span className="pager"><Link className="btn secondary small" to="/server/status">See all</Link></span>
				</div>
			) : null}
		</Panel>
	)
}
