import { Link } from 'react-router-dom'
import { formatBytes, percent } from '../helpers'
import type { ServerOverview } from '../types'

interface ServerResourceStripProps {
	server: ServerOverview | null
	collapsed: boolean
	canViewStatus: boolean
}

export function ServerResourceStrip ({ server, collapsed, canViewStatus }: ServerResourceStripProps) {
	if (!server || !canViewStatus) return null
	const system = server.system
	const loadTone = system.load1 >= 4 ? 'critical' : system.load1 >= 2 ? 'warn' : 'good'
	const memPct = percent(system.memory_used, system.memory_total)
	const diskPct = percent(system.disk_used, system.disk_total)
	const memTone = memPct >= 90 ? 'critical' : memPct >= 75 ? 'warn' : 'good'
	const diskTone = diskPct >= 90 ? 'critical' : diskPct >= 75 ? 'warn' : 'good'

	if (collapsed) {
		return (
			<div className="sidebar-resources collapsed" title={`Load ${system.load1.toFixed(1)} · Mem ${memPct}% · Disk ${diskPct}%`}>
				<span className={`resource-dot ${loadTone}`} aria-hidden="true" />
			</div>
		)
	}

	return (
		<section className="sidebar-resources" aria-label="Server resources">
			<div className="sidebar-resources-head">
				<strong>{system.hostname}</strong>
				{canViewStatus ? <Link to="/status">Details</Link> : null}
			</div>
			<div className="resource-bars">
				<ResourceBar label="Load" value={`${system.load1.toFixed(2)}`} percent={Math.min(system.load1 * 25, 100)} tone={loadTone} detail="1 min avg" />
				<ResourceBar label="Memory" value={`${memPct}%`} percent={memPct} tone={memTone} detail={`${formatBytes(system.memory_used)} / ${formatBytes(system.memory_total)}`} />
				<ResourceBar label="Disk" value={`${diskPct}%`} percent={diskPct} tone={diskTone} detail={`${formatBytes(system.disk_used)} / ${formatBytes(system.disk_total)}`} />
			</div>
			<div className="resource-stats">
				<span>{server.stats.accounts} accounts</span>
				<span>{server.services.filter((service) => service.observed_running).length}/{server.services.length} services up</span>
			</div>
		</section>
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
