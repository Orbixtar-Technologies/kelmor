import { useEffect, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { api, asList } from './client'
import { Can } from './rbac'
import { Empty, fmtBytes, Metric, Notice, PageHeader } from './ui'

interface HostStats {
	accounts?: number
	failedJobs?: number
}

interface HostSystem {
	hostname?: string
	load1?: number
	memory_used?: number
	memory_total?: number
	disk_used?: number
	disk_total?: number
	inodes_used?: number
	inodes_total?: number
	uptime_seconds?: number
}

interface HostService {
	name: string
	health: string
	observed_running: boolean
}

interface HostOverview {
	system: HostSystem
	stats: HostStats
	services: HostService[]
}

interface JobRow {
	id: string
	type: string
	state: string
	progress: number
	last_error?: string
	resource_id?: string
}

export function Dashboard () {
	const [data, setData] = useState<HostOverview | null>(null)
	const [failed, setFailed] = useState<JobRow[]>([])
	const [err, setErr] = useState('')

	useEffect(() => {
		api<HostOverview>('/api/v1/server')
			.then(setData)
			.catch((e) => setErr(e.message))
		api<{ items: JobRow[] }>('/api/v1/jobs?state=failed')
			.then((r) => setFailed(asList(r)))
			.catch(() => setFailed([]))
	}, [])

	if (err) return <Empty title="Could not load host metrics" detail={err} />
	if (!data) {
		return (
			<Empty
				title="Reading host sensors"
				detail="Contacting Kelmor Agent for CPU, memory, disk and services."
			/>
		)
	}

	const s = data.system
	const failedCount = data.stats.failedJobs ?? failed.length
	return (
		<>
			<PageHeader
				title="Host operations"
				detail="Live observed state from Kelmor Agent, not decorative charts."
			/>
			<section className="metrics">
				<Metric label="Hostname" value={s.hostname || '—'} />
				<Metric label="Load 1" value={Number(s.load1 || 0).toFixed(2)} />
				<Metric
					label="Memory"
					value={fmtBytes(s.memory_used || 0) + ' / ' + fmtBytes(s.memory_total || 0)}
				/>
				<Metric
					label="Disk"
					value={fmtBytes(s.disk_used || 0) + ' / ' + fmtBytes(s.disk_total || 0)}
				/>
				<Metric
					label="Inodes"
					value={`${s.inodes_used || 0} / ${s.inodes_total || 0}`}
				/>
				<Metric
					label="Uptime"
					value={`${Math.floor((s.uptime_seconds || 0) / 3600)}h`}
				/>
				<Metric label="Accounts" value={String(data.stats.accounts || 0)} />
				<Metric label="Failed jobs" value={String(failedCount)} />
			</section>
			<section className="quick-links" aria-label="Quick links">
				<h2>Quick links</h2>
				<div className="quick-grid">
					<Can cap="accounts.read">
						<NavLink to="/accounts">List Accounts</NavLink>
					</Can>
					<Can cap="accounts.create">
						<NavLink to="/accounts/create">Create Account</NavLink>
					</Can>
					<Can cap="packages.read">
						<NavLink to="/packages">Packages</NavLink>
					</Can>
					<NavLink to="/jobs">Jobs</NavLink>
					<Can cap="security.audit.read">
						<NavLink to="/audit">Audit</NavLink>
					</Can>
					<Can cap="accounts.create">
						<NavLink to="/import">Import</NavLink>
					</Can>
					<Can cap="billing.usage.read">
						<NavLink to="/monitor">Usage</NavLink>
					</Can>
					<Can cap="resellers.read">
						<NavLink to="/resellers">Resellers</NavLink>
					</Can>
				</div>
			</section>
			<section className="failed-jobs">
				<h2>Failed jobs</h2>
				{failed.length === 0 ? (
					<p className="muted">
						No failed jobs in the queue
						{failedCount ? ` (host counter still reports ${failedCount}).` : '.'}
					</p>
				) : (
					<table>
						<thead>
							<tr>
								<th>Type</th>
								<th>State</th>
								<th>%</th>
								<th>Error</th>
							</tr>
						</thead>
						<tbody>
							{failed.map((j) => (
								<tr key={j.id}>
									<td><NavLink to="/jobs">{j.type}</NavLink></td>
									<td>{j.state}</td>
									<td>{j.progress}</td>
									<td>{j.last_error}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</section>
			<div className="ops-split">
				<FirewallPanel />
				<RebootPanel />
			</div>
			<h2>Service status</h2>
			<table>
				<thead>
					<tr>
						<th>Service</th>
						<th>Health</th>
						<th>Running</th>
					</tr>
				</thead>
				<tbody>
					{data.services.map((svc) => (
						<tr key={svc.name}>
							<td>{svc.name}</td>
							<td>{svc.health}</td>
							<td>{svc.observed_running ? 'yes' : 'no'}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

function FirewallPanel () {
	const [msg, setMsg] = useState('')
	return (
		<section>
			<h2>Host firewall</h2>
			<p>
				Applies the host nftables filter (on-disk table <code>inet panel</code>)
				with a drop policy on inbound traffic, keeping loopback, established
				flows, and already-bound hosting plus management ports.
			</p>
			<Can cap="server.firewall.write">
				<button
					type="button"
					onClick={async () => {
						setMsg('')
						try {
							const r = await api<any>('/api/v1/server/firewall/apply', {
								method: 'POST',
								body: '{}',
							})
							setMsg(r.message || 'host filter table applied')
						} catch (e) {
							setMsg(e instanceof Error ? e.message : 'apply failed')
						}
					}}
				>
					Apply host filter table
				</button>
			</Can>
			<Notice>{msg}</Notice>
		</section>
	)
}

function RebootPanel () {
	const [msg, setMsg] = useState('')
	return (
		<section>
			<h2>Host reboot</h2>
			<p>
				Records a reboot through Kelmor Agent. The node only shuts down when
				the agent is live and reboot is explicitly allowed.
			</p>
			<Can cap="server.settings.write">
				<button
					type="button"
					onClick={async () => {
						setMsg('')
						if (!window.confirm('Reboot this hosting node? Active sites will drop until systemd brings them back.'))
							return
						try {
							const r = await api<any>('/api/v1/server/reboot', {
								method: 'POST',
								body: JSON.stringify({ confirm: 'REBOOT' }),
							})
							setMsg(r.message || r.observed_state || 'reboot recorded')
						} catch (e) {
							setMsg(e instanceof Error ? e.message : 'reboot failed')
						}
					}}
				>
					Request host reboot
				</button>
			</Can>
			<Notice>{msg}</Notice>
		</section>
	)
}
