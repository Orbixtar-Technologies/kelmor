import { Link } from 'react-router-dom'
import { formatBytes, formatDate, percent } from '../helpers'
import type { ServerOverview } from '../types'

interface ResourcesSidebarProps {
	server: ServerOverview | null
	canViewStatus: boolean
	updatedAt?: string
	collapsed?: boolean
	onToggle?: () => void
}

export function ResourcesSidebar ({ server, canViewStatus, updatedAt, collapsed, onToggle }: ResourcesSidebarProps) {
	if (!server || !canViewStatus) return null
	const system = server.system
	const loadTone = system.load1 >= 4 ? 'critical' : system.load1 >= 2 ? 'warn' : 'good'
	const memPct = percent(system.memory_used, system.memory_total)
	const diskPct = percent(system.disk_used, system.disk_total)
	const memTone = memPct >= 90 ? 'critical' : memPct >= 75 ? 'warn' : 'good'
	const diskTone = diskPct >= 90 ? 'critical' : diskPct >= 75 ? 'warn' : 'good'

	if (collapsed) {
		return (
			<aside className="resources-sidebar collapsed" aria-label="Host resources">
				<button type="button" className="resources-toggle" onClick={onToggle} aria-expanded={false}>Show host resources</button>
			</aside>
		)
	}

	return (
		<aside className="resources-sidebar" aria-label="Host resources">
			<header className="resources-sidebar-head">
				<div>
					<p className="resource-scope">Host</p>
					<h2>Host resources</h2>
					<p>{system.hostname}</p>
				</div>
				<div className="resources-sidebar-actions">
					{onToggle ? <button type="button" className="link-button" onClick={onToggle} aria-expanded>Hide</button> : null}
					<Link to="/status" className="resources-sidebar-link">Details</Link>
				</div>
			</header>
			<p className="resources-freshness">{updatedAt ? `Updated ${formatDate(updatedAt)}` : 'Host vitals for this server'}</p>
			<div className="resource-bars">
				<ResourceBar label="Load" value={`${system.load1.toFixed(2)}`} percent={Math.min(system.load1 * 25, 100)} tone={loadTone} detail="1 min avg" />
				<ResourceBar label="Memory" value={`${memPct}%`} percent={memPct} tone={memTone} detail={`${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}`} />
				<ResourceBar label="Disk" value={`${diskPct}%`} percent={diskPct} tone={diskTone} detail={`${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}`} />
			</div>
			<dl className="resources-sidebar-stats">
				<div><dt>Accounts</dt><dd>{server.stats.accounts}</dd></div>
				<div><dt>Services up</dt><dd>{server.services.filter((service) => service.observed_running).length}/{server.services.length}</dd></div>
				<div><dt>Uptime</dt><dd>{system.uptime_seconds ? `${Math.floor(system.uptime_seconds / 3600)}h` : 'Online'}</dd></div>
			</dl>
		</aside>
	)
}

interface ResourceBarProps {
	label: string
	value: string
	percent: number
	tone: string
	detail: string
}

function ResourceBar ({ label, value, percent, tone, detail }: ResourceBarProps) {
	return (
		<div className="resource-bar">
			<div className="resource-bar-head">
				<span>{label}</span>
				<strong>{value}</strong>
			</div>
			<div className="resource-track" aria-hidden="true">
				<span className={`resource-fill ${tone}`} style={{ width: `${Math.min(100, Math.max(0, percent))}%` }} />
			</div>
			<small>{detail}</small>
		</div>
	)
}
