import { Link } from 'react-router-dom'
import { api, listOf } from '../client'
import { useLoad } from '../lib/hooks'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, Meter, Notice, PageHeader, Panel, Pill } from '../components/ui'
import { formatBytes, formatCount, formatRelative, meterClass, usedPercent } from '../lib/format'
import type { AccountRow } from '../components/account-picker'
import type { PackageRow } from './packages'

interface UsageRow {
	account_id: string
	collected_at: string
	disk_bytes: number
	inode_count: number
	bandwidth_bytes: number
	memory_bytes: number
	process_count: number
}

interface MonitorResponse {
	accounts: UsageRow[] | null
	failed_jobs: number
	certs_expiring: number
}

interface Row extends UsageRow {
	username: string
	primaryDomain: string
	packageName: string
	diskLimit: number
	bandwidthLimit: number
}

export function UsageReport () {
	const monitor = useLoad<MonitorResponse>(() => api<MonitorResponse>('/api/v1/server/monitor'), [], 30000)
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts').catch(() => []), [])
	const packages = useLoad<PackageRow[]>(() => listOf<PackageRow>('/api/v1/packages').catch(() => []), [])

	const packageById = new Map((packages.data ?? []).map((p) => [p.id, p]))
	const accountById = new Map((accounts.data ?? []).map((a) => [a.id, a]))

	const rows: Row[] = (monitor.data?.accounts ?? []).map((usage) => {
		const account = accountById.get(usage.account_id)
		const pkg = account ? packageById.get(account.package_id || '') : undefined
		return {
			...usage,
			username: account?.username ?? usage.account_id.slice(0, 8),
			primaryDomain: account?.primary_domain ?? '',
			packageName: pkg?.name ?? '—',
			diskLimit: pkg?.disk_bytes ?? 0,
			bandwidthLimit: pkg?.bandwidth_bytes_monthly ?? 0,
		}
	})

	const totalDisk = rows.reduce((sum, row) => sum + (row.disk_bytes || 0), 0)
	const totalBandwidth = rows.reduce((sum, row) => sum + (row.bandwidth_bytes || 0), 0)
	const overQuota = rows.filter(
		(row) =>
			(row.diskLimit > 0 && row.disk_bytes >= row.diskLimit) ||
			(row.bandwidthLimit > 0 && row.bandwidth_bytes >= row.bandwidthLimit),
	)

	const columns: Column<Row>[] = [
		{
			key: 'account',
			header: 'Account',
			sort: (r) => r.username,
			render: (r) => (
				<span>
					<Link to={`/accounts/${r.account_id}`}><strong>{r.username}</strong></Link>
					{r.primaryDomain ? <><br /><span className="mono small muted">{r.primaryDomain}</span></> : null}
				</span>
			),
		},
		{ key: 'package', header: 'Package', sort: (r) => r.packageName },
		{
			key: 'disk',
			header: 'Disk',
			align: 'right',
			sort: (r) => r.disk_bytes,
			render: (r) => <UsageCell used={r.disk_bytes} limit={r.diskLimit} />,
		},
		{
			key: 'bandwidth',
			header: 'Transfer (month)',
			align: 'right',
			sort: (r) => r.bandwidth_bytes,
			render: (r) => <UsageCell used={r.bandwidth_bytes} limit={r.bandwidthLimit} />,
		},
		{ key: 'inodes', header: 'Inodes', align: 'right', sort: (r) => r.inode_count, render: (r) => formatCount(r.inode_count) },
		{ key: 'processes', header: 'Processes', align: 'right', sort: (r) => r.process_count, render: (r) => formatCount(r.process_count) },
		{ key: 'memory', header: 'Memory', align: 'right', sort: (r) => r.memory_bytes, render: (r) => formatBytes(r.memory_bytes) },
		{ key: 'collected', header: 'Collected', sort: (r) => r.collected_at, render: (r) => formatRelative(r.collected_at) },
	]

	return (
		<>
			<PageHeader
				title="Bandwidth and Disk Usage"
				description="Measured consumption per account: disk and inodes walked from the home tree, monthly transfer summed from nginx access logs, memory and process counts from the account slice."
				favoritePath="/usage"
				actions={<button type="button" className="btn secondary" onClick={() => monitor.reload()}>Recollect</button>}
			/>

			<div className="grid-3">
				<Panel title="Disk in use" icon="hardDrive" tight>
					<ul className="stat-list">
						<li><span className="label">Across all accounts</span><span className="value">{formatBytes(totalDisk)}</span></li>
						<li><span className="label">Transfer this month</span><span className="value">{formatBytes(totalBandwidth)}</span></li>
						<li><span className="label">Accounts measured</span><span className="value">{formatCount(rows.length)}</span></li>
					</ul>
				</Panel>
				<Panel title="At the limit" icon="alertTriangle" tight>
					<ul className="stat-list">
						<li>
							<span className="label">Over quota</span>
							<span className="value">
								{overQuota.length > 0 ? <Link to="/accounts/over-quota"><Pill tone="bad">{overQuota.length}</Pill></Link> : <Pill tone="ok">0</Pill>}
							</span>
						</li>
						<li>
							<span className="label">Certificates expiring</span>
							<span className="value">
								{monitor.data?.certs_expiring ? <Link to="/ssl"><Pill tone="warn">{monitor.data.certs_expiring}</Pill></Link> : <Pill tone="ok">0</Pill>}
							</span>
						</li>
						<li>
							<span className="label">Failed jobs</span>
							<span className="value">
								{monitor.data?.failed_jobs ? <Link to="/jobs"><Pill tone="bad">{monitor.data.failed_jobs}</Pill></Link> : <Pill tone="ok">0</Pill>}
							</span>
						</li>
					</ul>
				</Panel>
				<Panel title="How this is measured" icon="info">
					<p className="small muted">
						Disk and inodes are walked per home directory by the privileged agent. Monthly transfer is the sum of
						<span className="mono"> $body_bytes_sent</span> in each site nginx access log for the current calendar month.
						Nothing here is estimated from a sample.
					</p>
				</Panel>
			</div>

			{overQuota.length > 0 ? (
				<Notice tone="warn">
					{overQuota.length} account{overQuota.length === 1 ? ' has' : 's have'} reached a package limit. At the transfer
					limit Kelmor rewrites the vhost to HTTP 509 while keeping the ACME challenge location, so certificate renewals
					still succeed.
				</Notice>
			) : null}

			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(r) => r.account_id}
				loading={monitor.loading}
				error={monitor.error}
				searchPlaceholder="Search account, domain or package"
				noun="accounts"
				initialSort={{ key: 'disk', dir: 'desc' }}
				empty={
					<EmptyState icon="barChart" title="No usage has been collected yet" action={<Link className="btn secondary" to="/accounts">Open List Accounts</Link>}>
						Usage appears after the first collection run against a provisioned account.
					</EmptyState>
				}
			/>
		</>
	)
}

function UsageCell ({ used, limit }: { used: number; limit: number }) {
	const percent = usedPercent(used, limit)
	return (
		<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center', justifyContent: 'flex-end' }}>
			{limit > 0 ? <Meter used={used} limit={limit} className={meterClass(percent)} /> : null}
			<span className="nowrap">
				{formatBytes(used)}
				{limit > 0 ? <span className="muted"> / {formatBytes(limit)}</span> : <span className="muted"> / unmetered</span>}
			</span>
		</span>
	)
}
