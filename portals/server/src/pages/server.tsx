import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, post } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { ConfirmDialog, EmptyState, KeyValues, Loading, Meter, Notice, PageHeader, Panel, Pill, statusTone } from '../components/ui'
import { formatBytes, formatCount, formatUptime, meterClass, usedPercent } from '../lib/format'

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
	cpu_count?: number
}

interface ServiceRow {
	name: string
	health: string
	observed_running: boolean
	desired_running?: boolean
}

interface Overview {
	system: SystemInfo
	stats: Record<string, number>
	services: ServiceRow[]
}

interface ProcessRow {
	pid: number
	user: string
	cmd: string
	cpu: number
	rss: number
}

export function ServiceStatus () {
	const overview = useLoad<Overview>(() => api<Overview>('/api/v1/server'), [], 15000)
	const services = overview.data?.services ?? []
	const down = services.filter((s) => !s.observed_running)

	const columns: Column<ServiceRow>[] = [
		{ key: 'name', header: 'Service', sort: (s) => s.name, className: 'mono' },
		{ key: 'health', header: 'Health', sort: (s) => s.health, render: (s) => <Pill tone={statusTone(s.health)}>{s.health}</Pill> },
		{
			key: 'running',
			header: 'Observed',
			sort: (s) => (s.observed_running ? 'running' : 'stopped'),
			render: (s) => (s.observed_running ? <Pill tone="ok">running</Pill> : <Pill tone="bad">stopped</Pill>),
		},
	]

	return (
		<>
			<PageHeader
				title="Service Status"
				description="Health of every service Kelmor manages on this host, as observed by the privileged agent — not as configured on paper."
				favoritePath="/server/status"
				actions={<button type="button" className="btn secondary" onClick={() => overview.reload()}>Refresh</button>}
			/>
			{down.length > 0 ? (
				<Notice tone="warn">
					{down.length} managed service{down.length === 1 ? ' is' : 's are'} not running: {down.map((s) => s.name).join(', ')}.
					On a full Ubuntu host these are started by systemd through the installer phases.
				</Notice>
			) : services.length > 0 ? (
				<Notice tone="ok">Every managed service is running.</Notice>
			) : null}
			<DataTable
				rows={services}
				columns={columns}
				rowKey={(s) => s.name}
				loading={overview.loading}
				error={overview.error}
				searchPlaceholder="Search services"
				noun="services"
				pageSize={50}
				initialSort={{ key: 'running', dir: 'asc' }}
			/>
		</>
	)
}

export function HostVitals () {
	const overview = useLoad<Overview>(() => api<Overview>('/api/v1/server'), [], 15000)

	if (overview.loading && !overview.data) return <Loading label="Reading host sensors" />
	if (overview.error) {
		return (
			<>
				<PageHeader title="Host Vitals" favoritePath="/server/vitals" />
				<Panel><EmptyState icon="alertCircle" title="Host metrics are unavailable">{overview.error}</EmptyState></Panel>
			</>
		)
	}

	const s = overview.data!.system
	const stats = overview.data!.stats ?? {}

	return (
		<>
			<PageHeader
				title="Host Vitals"
				description="Load, memory, disk, inodes and uptime read from the privileged agent on this node."
				favoritePath="/server/vitals"
				actions={<button type="button" className="btn secondary" onClick={() => overview.reload()}>Refresh</button>}
			/>
			<div className="grid-2">
				<Panel title="System" icon="server" tight>
					<ul className="stat-list">
						<li><span className="label">Hostname</span><span className="value mono">{s.hostname}</span></li>
						<li><span className="label">Kernel</span><span className="value mono">{s.kernel || '—'}</span></li>
						<li><span className="label">Uptime</span><span className="value">{formatUptime(s.uptime_seconds)}</span></li>
						<li><span className="label">Load (1m)</span><span className="value">{Number(s.load1 ?? 0).toFixed(2)}</span></li>
						{s.load5 !== undefined ? <li><span className="label">Load (5m)</span><span className="value">{Number(s.load5).toFixed(2)}</span></li> : null}
						{s.load15 !== undefined ? <li><span className="label">Load (15m)</span><span className="value">{Number(s.load15).toFixed(2)}</span></li> : null}
					</ul>
				</Panel>
				<Panel title="Capacity" icon="hardDrive" tight>
					<ul className="stat-list">
						<li>
							<span className="label">Memory</span>
							<Meter used={s.memory_used} limit={s.memory_total} className={meterClass(usedPercent(s.memory_used, s.memory_total))} />
							<span className="value">{formatBytes(s.memory_used)} / {formatBytes(s.memory_total)}</span>
						</li>
						<li>
							<span className="label">Disk</span>
							<Meter used={s.disk_used} limit={s.disk_total} className={meterClass(usedPercent(s.disk_used, s.disk_total))} />
							<span className="value">{formatBytes(s.disk_used)} / {formatBytes(s.disk_total)}</span>
						</li>
						<li>
							<span className="label">Inodes</span>
							<Meter used={s.inodes_used} limit={s.inodes_total} className={meterClass(usedPercent(s.inodes_used, s.inodes_total))} />
							<span className="value">{formatCount(s.inodes_used)} / {formatCount(s.inodes_total)}</span>
						</li>
					</ul>
				</Panel>
			</div>
			<Panel title="Hosting totals" icon="barChart">
				<KeyValues
					rows={Object.entries(stats).map(([key, value]) => [
						key.replace(/([A-Z])/g, ' $1').replace(/^./, (c) => c.toUpperCase()),
						formatCount(value),
					])}
				/>
			</Panel>
		</>
	)
}

export function ProcessManager () {
	const processes = useLoad<{ processes: ProcessRow[] }>(() => api('/api/v1/server/processes'), [], 10000)
	const rows = processes.data?.processes ?? []

	const columns: Column<ProcessRow>[] = [
		{ key: 'pid', header: 'PID', align: 'right', sort: (p) => p.pid },
		{ key: 'user', header: 'User', sort: (p) => p.user, className: 'mono' },
		{ key: 'cmd', header: 'Command', sort: (p) => p.cmd, className: 'mono' },
		{ key: 'cpu', header: 'CPU %', align: 'right', sort: (p) => p.cpu, render: (p) => p.cpu.toFixed(1) },
		{ key: 'rss', header: 'Resident', align: 'right', sort: (p) => p.rss, render: (p) => `${p.rss} MB` },
	]

	return (
		<>
			<PageHeader
				title="Process Manager"
				description="Processes reported by the privileged agent for this host."
				favoritePath="/server/processes"
				actions={<button type="button" className="btn secondary" onClick={() => processes.reload()}>Refresh</button>}
			/>
			<Notice tone="info">
				Kelmor does not expose a process kill endpoint. Runaway account workloads are contained by the package limits
				written to each account systemd slice (CPU, memory, process count and I/O), not by ad-hoc signals from a browser.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(p) => String(p.pid)}
				loading={processes.loading}
				error={processes.error}
				searchPlaceholder="Search command or user"
				noun="processes"
				initialSort={{ key: 'cpu', dir: 'desc' }}
				empty={<EmptyState icon="terminal" title="No process detail reported">The agent reports process detail on a full host install.</EmptyState>}
			/>
		</>
	)
}

export function HostFirewall () {
	const toast = useToast()
	const canApply = useCan('server.firewall.write')
	const firewall = useLoad<{ table: string; file: string }>(() => api('/api/v1/server/firewall'), [])
	const [confirming, setConfirming] = useState(false)
	const [busy, setBusy] = useState(false)

	return (
		<>
			<PageHeader
				title="Host Firewall"
				description="Kelmor keeps one nftables table for this host. Applying it re-asserts the intended policy from the control plane."
				favoritePath="/security/firewall"
			/>
			<div className="grid-2">
				<Panel title="Current policy" icon="shield">
					{firewall.loading ? <Loading /> : (
						<KeyValues
							rows={[
								['Table', <span key="t" className="mono">{firewall.data?.table ?? 'inet panel'}</span>],
								['Ruleset file', <span key="f" className="mono">{firewall.data?.file ?? '—'}</span>],
								['Inbound default', 'drop'],
								['Always allowed', 'loopback, established and related flows'],
								['Hosting ports', '21 + 40000–40100, 22, 25, 53, 80, 443, 587, 993'],
								['Management ports', '8443 (Director), 8444 (Control)'],
							]}
						/>
					)}
				</Panel>
				<Panel title="Apply the policy" icon="refresh">
					<Notice tone="warn">
						Applying replaces the live ruleset. Already-bound management ports are preserved so an operator cannot lock
						themselves out of this panel, but any rule added outside Kelmor is dropped.
					</Notice>
					{canApply ? (
						<div className="btn-row">
							<button type="button" className="btn" disabled={busy} onClick={() => setConfirming(true)}>Apply firewall policy</button>
						</div>
					) : (
						<p className="small muted">Your role can review the policy but not apply it.</p>
					)}
				</Panel>
			</div>

			{confirming ? (
				<ConfirmDialog
					title="Apply the nftables policy?"
					confirmLabel="Apply policy"
					busy={busy}
					onCancel={() => setConfirming(false)}
					onConfirm={async () => {
						setConfirming(false)
						setBusy(true)
						await toast.run(
							() => post<{ message?: string }>('/api/v1/server/firewall/apply'),
							(r) => r.message || 'Firewall policy applied',
						)
						setBusy(false)
					}}
				>
					<p>
						The live ruleset is replaced with the Kelmor table. Inbound traffic defaults to drop, keeping loopback,
						established flows, the hosting ports and the management ports that are already bound.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

export function ServerReboot () {
	const toast = useToast()
	const [confirming, setConfirming] = useState(false)
	const [busy, setBusy] = useState(false)
	const [result, setResult] = useState('')
	const overview = useLoad<Overview>(() => api<Overview>('/api/v1/server'), [])

	return (
		<>
			<PageHeader
				title="Graceful Server Reboot"
				description="Records a reboot request through the privileged agent. Every hosted site on this node goes down until systemd brings the stack back."
				favoritePath="/server/reboot"
			/>
			<Notice tone="error">
				This is the most disruptive action in Director. Every website, mailbox and database on this host stops answering
				until the node comes back. Check the job queue is quiet first, and prefer restarting a single service where that
				is enough.
			</Notice>
			<div className="grid-2">
				<Panel title="Before you reboot" icon="alertTriangle">
					<ol style={{ margin: 0, paddingLeft: 20, lineHeight: 2 }}>
						<li>Confirm no backup or restore is running on the <Link to="/jobs">Job Queue</Link>.</li>
						<li>Check <Link to="/server/status">Service Status</Link> so you know the expected state after the reboot.</li>
						<li>Tell affected customers — sites are unreachable for the duration.</li>
					</ol>
					<p className="small muted" style={{ marginTop: 14 }}>
						The node only actually shuts down when the privileged agent is live and <code>PANEL_ALLOW_REBOOT=1</code>.
						Otherwise the request is recorded in the audit trail and nothing restarts.
					</p>
				</Panel>
				<Panel title="Host" icon="server">
					<KeyValues
						rows={[
							['Hostname', <span key="h" className="mono">{overview.data?.system.hostname ?? '—'}</span>],
							['Uptime', formatUptime(overview.data?.system.uptime_seconds)],
							['Hosting accounts affected', formatCount(overview.data?.stats?.accounts ?? 0)],
						]}
					/>
					<div className="btn-row" style={{ marginTop: 16 }}>
						<button type="button" className="btn danger" disabled={busy} onClick={() => setConfirming(true)}>Request host reboot</button>
					</div>
					{result ? <div style={{ marginTop: 14 }}><Notice tone="info">{result}</Notice></div> : null}
				</Panel>
			</div>

			{confirming ? (
				<ConfirmDialog
					title="Reboot this hosting node?"
					confirmLabel="Reboot the node"
					typeToConfirm="REBOOT"
					busy={busy}
					onCancel={() => setConfirming(false)}
					onConfirm={async () => {
						setConfirming(false)
						setBusy(true)
						const r = await toast.run(
							() => post<{ message?: string; observed_state?: string }>('/api/v1/server/reboot', { confirm: 'REBOOT' }),
							() => 'Reboot recorded',
						)
						if (r) setResult(r.message || r.observed_state || 'Reboot recorded through the privileged agent.')
						setBusy(false)
					}}
				>
					<p>
						All {formatCount(overview.data?.stats?.accounts ?? 0)} hosting accounts on{' '}
						<strong>{overview.data?.system.hostname ?? 'this host'}</strong> lose service until systemd brings the stack
						back. This action is recorded in the audit trail with your username.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}
