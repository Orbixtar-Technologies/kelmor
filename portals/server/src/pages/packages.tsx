import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, del, listOf, post, put } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { ConfirmDialog, EmptyState, Field, Loading, Notice, PageHeader, Panel, Pill } from '../components/ui'
import { formatBytes, formatCount } from '../lib/format'
import type { AccountRow } from '../components/account-picker'

export interface PackageRow {
	id: string
	name: string
	reseller_id?: string
	feature_set_id?: string
	disk_bytes: number
	bandwidth_bytes_monthly: number
	domains: number
	subdomains: number
	alias_domains: number
	databases: number
	database_users: number
	mailboxes: number
	mailbox_storage_bytes: number
	ftp_users: number
	cron_jobs: number
	application_instances: number
	backup_retention_days: number
	cpu_percent: number
	memory_bytes: number
	process_limit: number
	io_weight: number
	iops: number
	concurrent_web_requests: number
	email_daily_limit: number
}

interface FeatureSet {
	id: string
	name: string
	features: Record<string, boolean>
}

const blank: PackageRow = {
	id: '',
	name: '',
	disk_bytes: 10 * 1024 ** 3,
	bandwidth_bytes_monthly: 100 * 1024 ** 3,
	domains: 10,
	subdomains: 50,
	alias_domains: 20,
	databases: 10,
	database_users: 20,
	mailboxes: 50,
	mailbox_storage_bytes: 5 * 1024 ** 3,
	ftp_users: 10,
	cron_jobs: 20,
	application_instances: 5,
	backup_retention_days: 14,
	cpu_percent: 200,
	memory_bytes: 2 * 1024 ** 3,
	process_limit: 200,
	io_weight: 100,
	iops: 1000,
	concurrent_web_requests: 200,
	email_daily_limit: 500,
}

export function PackageList () {
	const navigate = useNavigate()
	const toast = useToast()
	const canWrite = useCan('packages.write')
	const packages = useLoad<PackageRow[]>(() => listOf<PackageRow>('/api/v1/packages'), [])
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts').catch(() => []), [])
	const [pending, setPending] = useState<PackageRow | null>(null)
	const [busy, setBusy] = useState(false)

	const usedBy = (id: string) => (accounts.data ?? []).filter((a) => a.package_id === id).length

	const columns: Column<PackageRow>[] = [
		{ key: 'name', header: 'Package', sort: (p) => p.name, render: (p) => <Link to={`/packages/${p.id}`}><strong>{p.name}</strong></Link> },
		{ key: 'disk', header: 'Disk', align: 'right', sort: (p) => p.disk_bytes, render: (p) => formatBytes(p.disk_bytes) },
		{ key: 'bandwidth', header: 'Transfer / month', align: 'right', sort: (p) => p.bandwidth_bytes_monthly, render: (p) => formatBytes(p.bandwidth_bytes_monthly) },
		{ key: 'domains', header: 'Domains', align: 'right', sort: (p) => p.domains, render: (p) => formatCount(p.domains) },
		{ key: 'databases', header: 'Databases', align: 'right', sort: (p) => p.databases, render: (p) => formatCount(p.databases) },
		{ key: 'mailboxes', header: 'Mailboxes', align: 'right', sort: (p) => p.mailboxes, render: (p) => formatCount(p.mailboxes) },
		{ key: 'cpu', header: 'CPU / memory', sort: (p) => p.cpu_percent, render: (p) => `${p.cpu_percent}% · ${formatBytes(p.memory_bytes)}` },
		{
			key: 'accounts',
			header: 'Accounts',
			align: 'right',
			sort: (p) => usedBy(p.id),
			render: (p) => (usedBy(p.id) > 0 ? <Link to="/accounts">{usedBy(p.id)}</Link> : <span className="muted">0</span>),
		},
	]

	return (
		<>
			<PageHeader
				title="Packages"
				description="A package is the reusable set of limits Kelmor applies to every account that uses it: disk, monthly transfer, mail, databases and the process, CPU and I/O caps written to the account systemd slice."
				favoritePath="/packages"
				actions={canWrite ? <Link className="btn" to="/packages/new">Add a package</Link> : null}
			/>
			<DataTable
				rows={packages.data ?? []}
				columns={columns}
				rowKey={(p) => p.id}
				loading={packages.loading}
				error={packages.error}
				searchPlaceholder="Search packages"
				noun="packages"
				initialSort={{ key: 'name', dir: 'asc' }}
				empty={
					<EmptyState
						icon="box"
						title="No packages yet"
						action={canWrite ? <Link className="btn" to="/packages/new">Add a package</Link> : undefined}
					>
						Accounts cannot be created until at least one package exists — limits are enforced from the package.
					</EmptyState>
				}
				rowActions={[
					{ label: 'Edit package', onSelect: (p) => navigate(`/packages/${p.id}`), hidden: () => !canWrite },
					{ label: 'View accounts', onSelect: () => navigate('/accounts') },
					{
						label: 'Delete package',
						danger: true,
						onSelect: (p) => setPending(p),
						hidden: () => !canWrite,
					},
				]}
			/>

			{pending ? (
				<ConfirmDialog
					title={`Delete ${pending.name}?`}
					confirmLabel="Delete package"
					busy={busy}
					onCancel={() => setPending(null)}
					onConfirm={async () => {
						const target = pending
						setPending(null)
						setBusy(true)
						await toast.run(() => del(`/api/v1/packages/${target.id}`), () => `${target.name} deleted`)
						await packages.reload()
						setBusy(false)
					}}
				>
					{usedBy(pending.id) > 0 ? (
						<Notice tone="error">
							{usedBy(pending.id)} account(s) still use this package. Move them to another package first — the control
							plane refuses the delete while it is in use.
						</Notice>
					) : (
						<p>No account uses this package, so deleting it affects nothing that is already provisioned.</p>
					)}
				</ConfirmDialog>
			) : null}
		</>
	)
}

export function PackageEditor ({ mode }: { mode: 'create' | 'edit' }) {
	const { packageId = '' } = useParams()
	const navigate = useNavigate()
	const toast = useToast()
	const canWrite = useCan('packages.write')
	const [draft, setDraft] = useState<PackageRow>(blank)
	const [busy, setBusy] = useState(false)

	const existing = useLoad<PackageRow | null>(
		() => (mode === 'edit' ? api<PackageRow>(`/api/v1/packages/${packageId}`) : Promise.resolve(null)),
		[mode, packageId],
	)

	useEffect(() => {
		if (existing.data) setDraft(existing.data)
	}, [existing.data])

	if (mode === 'edit' && existing.loading) return <Loading label="Loading package" />
	if (mode === 'edit' && existing.error) {
		return (
			<>
				<PageHeader title="Package" description="" crumbs={[{ label: 'Packages', to: '/packages' }, { label: 'Not found' }]} />
				<Panel><EmptyState icon="alertCircle" title="This package could not be loaded">{existing.error}</EmptyState></Panel>
			</>
		)
	}

	function set<K extends keyof PackageRow> (key: K, value: PackageRow[K]) {
		setDraft((prev) => ({ ...prev, [key]: value }))
	}

	async function save () {
		setBusy(true)
		const done = await toast.run(
			() => (mode === 'create' ? post<PackageRow>('/api/v1/packages', draft) : put<PackageRow>(`/api/v1/packages/${packageId}`, draft)),
			(saved) => `${saved.name} ${mode === 'create' ? 'created' : 'saved'}`,
		)
		setBusy(false)
		if (done) navigate('/packages')
	}

	const readOnly = !canWrite

	return (
		<>
			<PageHeader
				title={mode === 'create' ? 'Add a Package' : `Edit ${draft.name || 'package'}`}
				description="Every limit here is enforced by the control plane: quotas on write, nginx and PHP-FPM for concurrency, and the account systemd slice for CPU, memory, processes and I/O."
				crumbs={[{ label: 'Packages', to: '/packages' }, { label: mode === 'create' ? 'Add a Package' : draft.name || 'Edit' }]}
				favoritePath={mode === 'create' ? '/packages/new' : undefined}
				actions={<Link className="btn secondary" to="/packages">Back to Packages</Link>}
			/>

			{readOnly ? <Notice tone="info">You can review this package but your role cannot change package limits.</Notice> : null}

			<Panel title="Identity" icon="box">
				<div className="form-grid">
					<Field label="Package name" hint="Shown when creating accounts and on every account summary.">
						<input value={draft.name} onChange={(e) => set('name', e.target.value)} disabled={readOnly} required />
					</Field>
				</div>
			</Panel>

			<Panel title="Storage and transfer" icon="hardDrive">
				<div className="form-grid">
					<GigabyteField label="Disk quota" hint="Enforced on file writes and SFTP even without kernel quotas." value={draft.disk_bytes} onChange={(v) => set('disk_bytes', v)} disabled={readOnly} />
					<GigabyteField label="Monthly transfer" hint="Summed from nginx access logs. At the limit the vhost answers HTTP 509." value={draft.bandwidth_bytes_monthly} onChange={(v) => set('bandwidth_bytes_monthly', v)} disabled={readOnly} />
					<GigabyteField label="Mailbox storage" hint="Per-account Maildir allowance." value={draft.mailbox_storage_bytes} onChange={(v) => set('mailbox_storage_bytes', v)} disabled={readOnly} />
					<NumberField label="Backup retention (days)" value={draft.backup_retention_days} onChange={(v) => set('backup_retention_days', v)} disabled={readOnly} />
				</div>
			</Panel>

			<Panel title="Feature limits" icon="sliders" subtitle="Checked when the resource is created">
				<div className="form-grid">
					<NumberField label="Domains" value={draft.domains} onChange={(v) => set('domains', v)} disabled={readOnly} />
					<NumberField label="Subdomains" value={draft.subdomains} onChange={(v) => set('subdomains', v)} disabled={readOnly} />
					<NumberField label="Alias domains" value={draft.alias_domains} onChange={(v) => set('alias_domains', v)} disabled={readOnly} />
					<NumberField label="Databases" value={draft.databases} onChange={(v) => set('databases', v)} disabled={readOnly} />
					<NumberField label="Database users" value={draft.database_users} onChange={(v) => set('database_users', v)} disabled={readOnly} />
					<NumberField label="Mailboxes" value={draft.mailboxes} onChange={(v) => set('mailboxes', v)} disabled={readOnly} />
					<NumberField label="FTP users" value={draft.ftp_users} onChange={(v) => set('ftp_users', v)} disabled={readOnly} />
					<NumberField label="Cron jobs" value={draft.cron_jobs} onChange={(v) => set('cron_jobs', v)} disabled={readOnly} />
					<NumberField label="Application instances" value={draft.application_instances} onChange={(v) => set('application_instances', v)} disabled={readOnly} />
					<NumberField label="Outbound mail per day" hint="Enforced by the Postfix policy service." value={draft.email_daily_limit} onChange={(v) => set('email_daily_limit', v)} disabled={readOnly} />
				</div>
			</Panel>

			<Panel title="Compute and I/O" icon="cpu" subtitle="Written to the account systemd slice">
				<div className="form-grid">
					<NumberField label="CPU percent" hint="200 means two full cores." value={draft.cpu_percent} onChange={(v) => set('cpu_percent', v)} disabled={readOnly} />
					<GigabyteField label="Memory" value={draft.memory_bytes} onChange={(v) => set('memory_bytes', v)} disabled={readOnly} />
					<NumberField label="Process limit" value={draft.process_limit} onChange={(v) => set('process_limit', v)} disabled={readOnly} />
					<NumberField label="I/O weight" hint="cgroup v2 io.weight when the controller is available." value={draft.io_weight} onChange={(v) => set('io_weight', v)} disabled={readOnly} />
					<NumberField label="IOPS" value={draft.iops} onChange={(v) => set('iops', v)} disabled={readOnly} />
					<NumberField label="Concurrent web requests" hint="nginx limit_conn and PHP-FPM pm.max_children." value={draft.concurrent_web_requests} onChange={(v) => set('concurrent_web_requests', v)} disabled={readOnly} />
				</div>
			</Panel>

			{!readOnly ? (
				<div className="form-actions" style={{ borderTop: 0 }}>
					<button type="button" className="btn" disabled={busy || !draft.name} onClick={save}>
						{busy ? 'Saving…' : mode === 'create' ? 'Create package' : 'Save package'}
					</button>
					<Link className="btn secondary" to="/packages">Cancel</Link>
				</div>
			) : null}
		</>
	)
}

export function FeatureManager () {
	const sets = useLoad<FeatureSet[]>(() => listOf<FeatureSet>('/api/v1/feature-sets'), [])
	const packages = useLoad<PackageRow[]>(() => listOf<PackageRow>('/api/v1/packages'), [])

	return (
		<>
			<PageHeader
				title="Feature Manager"
				description="Feature sets decide which tools an account can reach in Kelmor Control. Each package points at one set."
				favoritePath="/packages/features"
			/>
			<Notice tone="info">
				Feature sets are seeded by the control plane and assigned to packages when they are created. Editing a set from
				Director requires a feature-set write API, which this build does not expose yet — the sets below are read-only.
			</Notice>
			{sets.loading ? <Loading /> : (sets.data ?? []).length === 0 ? (
				<Panel title="Feature sets" icon="sliders">
					<EmptyState icon="sliders" title="No feature set is defined">The control plane seeds a full-hosting feature set on first boot.</EmptyState>
				</Panel>
			) : (
				(sets.data ?? []).map((set) => {
					const users = (packages.data ?? []).filter((p) => p.feature_set_id === set.id)
					return (
						<Panel
							key={set.id}
							title={set.name}
							icon="sliders"
							subtitle={`${users.length} package${users.length === 1 ? '' : 's'} use this set`}
						>
							<div className="btn-row">
								{Object.entries(set.features || {}).sort(([a], [b]) => a.localeCompare(b)).map(([feature, enabled]) => (
									<Pill key={feature} tone={enabled ? 'ok' : 'idle'}>{feature}</Pill>
								))}
							</div>
							{users.length > 0 ? (
								<p className="small muted" style={{ marginTop: 12 }}>
									Used by {users.map((p) => <Link key={p.id} to={`/packages/${p.id}`} style={{ marginRight: 8 }}>{p.name}</Link>)}
								</p>
							) : null}
						</Panel>
					)
				})
			)}
		</>
	)
}

function NumberField ({ label, hint, value, onChange, disabled }: { label: string; hint?: string; value: number; onChange: (v: number) => void; disabled?: boolean }) {
	return (
		<Field label={label} hint={hint}>
			<input type="number" min={0} value={value} onChange={(e) => onChange(Number(e.target.value))} disabled={disabled} />
		</Field>
	)
}

function GigabyteField ({ label, hint, value, onChange, disabled }: { label: string; hint?: string; value: number; onChange: (v: number) => void; disabled?: boolean }) {
	const gb = Math.round((value / 1024 ** 3) * 100) / 100
	return (
		<Field label={`${label} (GB)`} hint={hint ? `${hint} Currently ${formatBytes(value)}.` : `Currently ${formatBytes(value)}.`}>
			<input
				type="number"
				min={0}
				step={0.5}
				value={gb}
				onChange={(e) => onChange(Math.round(Number(e.target.value) * 1024 ** 3))}
				disabled={disabled}
			/>
		</Field>
	)
}
