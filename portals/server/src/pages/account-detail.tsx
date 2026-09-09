import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, del, listOf, post } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { Icon } from '../components/icons'
import {
	ConfirmDialog, EmptyState, Field, KeyValues, Loading, Meter, Notice,
	Panel, Pill, StatusPill, statusTone,
} from '../components/ui'
import { formatBytes, formatDateTime, formatRelative, meterClass, shortId, usedPercent } from '../lib/format'
import type { AccountRow } from '../components/account-picker'

interface DomainRow { id: string; ascii_fqdn: string; type: string; status: string; document_root?: string }
interface WebsiteRow { id: string; domain_id: string; runtime: string; runtime_version?: string; document_root: string; enabled: boolean }
interface ZoneRow { id: string; name: string; dnssec_enabled: boolean; provider: string }
interface RecordRow { id: string; name: string; type: string; content: string; ttl: number }
interface MailDomainRow { id: string; domain_id: string; catchall_policy: string; status: string }
interface MailboxRow { id: string; local_part: string; domain_id: string; status: string; quota_bytes: number }
interface AliasRow { id: string; address: string; destination: string }
interface DatabaseRow { id: string; engine: string; name: string; status: string }
interface CertRow { id: string; hostname: string; kind: string; status: string; not_after?: string; issuer?: string }
interface BackupRow { id: string; kind: string; state: string; destination: string; checksum?: string; size_bytes: number; created_at: string }
interface FileRow { name: string; size: number; dir: boolean }
interface JobRow { id: string; type: string; state: string; progress: number; last_error?: string; created_at: string; resource_id?: string; payload?: Record<string, unknown> }
interface PackageRow { id: string; name: string; disk_bytes: number; bandwidth_bytes_monthly: number; domains: number; databases: number; mailboxes: number }
interface UsageRow { disk_bytes: number; bandwidth_bytes: number; inode_count: number; process_count: number; memory_bytes: number; collected_at: string }

const tabs = [
	{ id: 'overview', label: 'Overview', icon: 'compass' },
	{ id: 'domains', label: 'Domains and sites', icon: 'globe' },
	{ id: 'dns', label: 'DNS', icon: 'globe' },
	{ id: 'email', label: 'Email', icon: 'mail' },
	{ id: 'databases', label: 'Databases', icon: 'database' },
	{ id: 'ssl', label: 'SSL', icon: 'lock' },
	{ id: 'files', label: 'Files', icon: 'folder' },
	{ id: 'backups', label: 'Backups', icon: 'archive' },
	{ id: 'jobs', label: 'Jobs', icon: 'list' },
] as const

export function AccountDetail () {
	const { accountId = '' } = useParams()
	const [params, setParams] = useSearchParams()
	const toast = useToast()
	const canSuspend = useCan('accounts.suspend')
	const canTerminate = useCan('accounts.terminate')
	const canImpersonate = useCan('accounts.impersonate')
	const [confirmTerminate, setConfirmTerminate] = useState(false)
	const [busy, setBusy] = useState(false)

	const tab = params.get('tab') || 'overview'
	const account = useLoad<AccountRow>(() => api<AccountRow>(`/api/v1/accounts/${accountId}`), [accountId])
	const usage = useLoad<UsageRow>(() => api<UsageRow>(`/api/v1/accounts/${accountId}/usage`).catch(() => null as never), [accountId])
	const packages = useLoad<PackageRow[]>(() => listOf<PackageRow>('/api/v1/packages').catch(() => []), [])

	if (account.error) {
		return (
			<>
				<div className="crumbs"><Link to="/accounts">List Accounts</Link></div>
				<Panel title="Account">
					<EmptyState icon="alertCircle" title="This account could not be loaded" action={<Link className="btn secondary" to="/accounts">Back to List Accounts</Link>}>
						{account.error}
					</EmptyState>
				</Panel>
			</>
		)
	}
	if (!account.data) return <Loading label="Loading account" />

	const acc = account.data
	const pkg = (packages.data ?? []).find((p) => p.id === acc.package_id)

	async function act (path: string, message: string) {
		setBusy(true)
		await toast.run(() => post(path), () => message)
		await account.reload()
		setBusy(false)
	}

	return (
		<>
			<nav className="crumbs" aria-label="Breadcrumb">
				<Link to="/">Home</Link>
				<span className="sep"><Icon name="chevronRight" size={11} /></span>
				<Link to="/accounts">List Accounts</Link>
				<span className="sep"><Icon name="chevronRight" size={11} /></span>
				<span>{acc.username}</span>
			</nav>
			<div className="page-head">
				<div>
					<h1>{acc.username}</h1>
					<p>
						<span className="mono">{acc.primary_domain}</span> · UID {acc.linux_uid} ·{' '}
						<span className="mono">{acc.home_path}</span> · {pkg ? <Link to={`/packages/${pkg.id}`}>{pkg.name}</Link> : 'no package'}
						{' '}<StatusPill status={acc.status} />
					</p>
				</div>
				<div className="head-actions">
					{canSuspend && acc.status !== 'suspended' ? (
						<button type="button" className="btn secondary" disabled={busy} onClick={() => act(`/api/v1/accounts/${acc.id}/suspend`, `${acc.username} suspended`)}>
							Suspend
						</button>
					) : null}
					{canSuspend && acc.status === 'suspended' ? (
						<button type="button" className="btn secondary" disabled={busy} onClick={() => act(`/api/v1/accounts/${acc.id}/unsuspend`, `${acc.username} restored`)}>
							Unsuspend
						</button>
					) : null}
					{canImpersonate ? (
						<button
							type="button"
							className="btn secondary"
							disabled={busy}
							onClick={async () => {
								const reason = window.prompt('Reason for signing in as this customer (recorded in the audit trail):')
								if (!reason) return
								setBusy(true)
								await toast.run(
									() => post<{ expires_at: string }>(`/api/v1/accounts/${acc.id}/impersonate`, { reason }),
									(r) => `Impersonation session issued until ${formatDateTime(r.expires_at)}`,
								)
								setBusy(false)
							}}
						>
							Sign in as owner
						</button>
					) : null}
					{canTerminate ? (
						<button type="button" className="btn danger" disabled={busy} onClick={() => setConfirmTerminate(true)}>Terminate</button>
					) : null}
				</div>
			</div>

			<nav className="tabs">
				{tabs.map((entry) => (
					<a
						key={entry.id}
						href={`?tab=${entry.id}`}
						className={tab === entry.id ? 'active' : undefined}
						onClick={(e) => {
							e.preventDefault()
							setParams({ tab: entry.id })
						}}
					>
						<Icon name={entry.icon} size={13} /> {entry.label}
					</a>
				))}
			</nav>

			{tab === 'overview' ? <OverviewTab account={acc} pkg={pkg} usage={usage.data} /> : null}
			{tab === 'domains' ? <DomainsTab accountId={acc.id} /> : null}
			{tab === 'dns' ? <DNSTab accountId={acc.id} /> : null}
			{tab === 'email' ? <EmailTab accountId={acc.id} /> : null}
			{tab === 'databases' ? <DatabasesTab accountId={acc.id} /> : null}
			{tab === 'ssl' ? <SSLTab account={acc} /> : null}
			{tab === 'files' ? <FilesTab accountId={acc.id} /> : null}
			{tab === 'backups' ? <BackupsTab accountId={acc.id} /> : null}
			{tab === 'jobs' ? <JobsTab accountId={acc.id} /> : null}

			{confirmTerminate ? (
				<ConfirmDialog
					title={`Terminate ${acc.username}?`}
					confirmLabel="Terminate permanently"
					typeToConfirm={acc.username}
					busy={busy}
					onCancel={() => setConfirmTerminate(false)}
					onConfirm={async () => {
						setConfirmTerminate(false)
						await act(`/api/v1/accounts/${acc.id}/terminate`, `${acc.username} termination queued`)
					}}
				>
					<p>
						This removes the Linux user, home directory, websites, databases, mailboxes and the DNS zone for{' '}
						<strong>{acc.primary_domain}</strong>. Take a backup first if the data may still be needed.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

function OverviewTab ({ account, pkg, usage }: { account: AccountRow; pkg?: PackageRow; usage: UsageRow | null }) {
	const diskPercent = usedPercent(usage?.disk_bytes, pkg?.disk_bytes)
	const bwPercent = usedPercent(usage?.bandwidth_bytes, pkg?.bandwidth_bytes_monthly)

	return (
		<div className="grid-2">
			<Panel title="Account" icon="users">
				<KeyValues
					rows={[
						['Username', <strong key="u">{account.username}</strong>],
						['Primary domain', <span key="d" className="mono">{account.primary_domain}</span>],
						['Status', <StatusPill key="s" status={account.status} />],
						['Package', pkg ? <Link key="p" to={`/packages/${pkg.id}`}>{pkg.name}</Link> : 'unassigned'],
						['Linux UID / GID', `${account.linux_uid ?? '—'}`],
						['Home directory', <span key="h" className="mono">{account.home_path}</span>],
						['IP address', <span key="i" className="mono">{account.ip_address || 'shared server address'}</span>],
						['Interactive login', account.login_disabled ? <Pill key="l" tone="warn">disabled</Pill> : <Pill key="l" tone="ok">enabled</Pill>],
					]}
				/>
			</Panel>
			<Panel title="Measured consumption" icon="barChart" subtitle={usage ? `Collected ${formatRelative(usage.collected_at)}` : undefined}>
				{!usage ? (
					<EmptyState icon="barChart" title="No usage collected yet">
						Usage is measured by the privileged agent. It appears after the first collection run.
					</EmptyState>
				) : (
					<ul className="stat-list">
						<li>
							<span className="label">Disk</span>
							{pkg?.disk_bytes ? <Meter used={usage.disk_bytes} limit={pkg.disk_bytes} className={meterClass(diskPercent)} /> : null}
							<span className="value">{formatBytes(usage.disk_bytes)}{pkg?.disk_bytes ? ` / ${formatBytes(pkg.disk_bytes)}` : ''}</span>
						</li>
						<li>
							<span className="label">Transfer this month</span>
							{pkg?.bandwidth_bytes_monthly ? <Meter used={usage.bandwidth_bytes} limit={pkg.bandwidth_bytes_monthly} className={meterClass(bwPercent)} /> : null}
							<span className="value">{formatBytes(usage.bandwidth_bytes)}{pkg?.bandwidth_bytes_monthly ? ` / ${formatBytes(pkg.bandwidth_bytes_monthly)}` : ''}</span>
						</li>
						<li><span className="label">Inodes</span><span className="value">{usage.inode_count?.toLocaleString() ?? '—'}</span></li>
						<li><span className="label">Processes</span><span className="value">{usage.process_count ?? '—'}</span></li>
						<li><span className="label">Memory</span><span className="value">{formatBytes(usage.memory_bytes)}</span></li>
					</ul>
				)}
			</Panel>
		</div>
	)
}

function DomainsTab ({ accountId }: { accountId: string }) {
	const domains = useLoad<DomainRow[]>(() => listOf<DomainRow>(`/api/v1/accounts/${accountId}/domains`), [accountId])
	const sites = useLoad<WebsiteRow[]>(() => listOf<WebsiteRow>(`/api/v1/accounts/${accountId}/websites`), [accountId])

	return (
		<>
			<Panel title="Domains" icon="globe" subtitle={`${domains.data?.length ?? 0} configured`} tight>
				{domains.loading ? <Loading /> : (domains.data ?? []).length === 0 ? (
					<EmptyState icon="globe" title="No domains yet">The primary domain is created with the account. Additional domains are added from Kelmor Control.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Domain</th><th>Type</th><th>Document root</th><th>Status</th></tr></thead>
							<tbody>
								{(domains.data ?? []).map((domain) => (
									<tr key={domain.id}>
										<td className="mono">{domain.ascii_fqdn}</td>
										<td>{domain.type}</td>
										<td className="mono small">{domain.document_root || '—'}</td>
										<td><StatusPill status={domain.status} /></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
			<Panel title="Websites" icon="server" subtitle={`${sites.data?.length ?? 0} vhosts`} tight>
				{sites.loading ? <Loading /> : (sites.data ?? []).length === 0 ? (
					<EmptyState icon="server" title="No website is configured">A vhost is written once the provisioning job finishes.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Runtime</th><th>Document root</th><th>HTTPS redirect</th><th>Enabled</th></tr></thead>
							<tbody>
								{(sites.data ?? []).map((site) => (
									<tr key={site.id}>
										<td>{site.runtime} {site.runtime_version}</td>
										<td className="mono small">{site.document_root}</td>
										<td>{(site as WebsiteRow & { https_redirect?: boolean }).https_redirect ? 'yes' : 'no'}</td>
										<td>{site.enabled ? <Pill tone="ok">enabled</Pill> : <Pill tone="idle">disabled</Pill>}</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
		</>
	)
}

function DNSTab ({ accountId }: { accountId: string }) {
	const zones = useLoad<ZoneRow[]>(() => listOf<ZoneRow>(`/api/v1/accounts/${accountId}/dns/zones`), [accountId])
	const list = zones.data ?? []

	if (zones.loading) return <Panel title="DNS" icon="globe" tight><Loading /></Panel>
	if (list.length === 0) {
		return (
			<Panel title="DNS" icon="globe">
				<EmptyState icon="globe" title="No DNS zone for this account">
					A zone is created with the primary domain during provisioning. If the provisioning job failed, retry it from the
					Jobs tab.
				</EmptyState>
			</Panel>
		)
	}

	return (
		<>
			{list.map((zone) => (
				<Panel
					key={zone.id}
					title={zone.name}
					icon="globe"
					subtitle={zone.dnssec_enabled ? <Pill tone="ok">DNSSEC signed</Pill> : <Pill tone="idle">unsigned</Pill>}
					actions={<Link className="btn secondary small" to={`/dns/zones/${accountId}/${zone.id}`}>Open zone manager</Link>}
					tight
				>
					<ZoneRecords accountId={accountId} zoneId={zone.id} compact />
				</Panel>
			))}
		</>
	)
}

export function ZoneRecords ({ accountId, zoneId, compact }: { accountId: string; zoneId: string; compact?: boolean }) {
	const toast = useToast()
	const canWrite = useCan('dns.write')
	const records = useLoad<RecordRow[]>(() => listOf<RecordRow>(`/api/v1/accounts/${accountId}/dns/zones/${zoneId}/records`), [accountId, zoneId])
	const [pending, setPending] = useState<RecordRow | null>(null)

	const rows = records.data ?? []

	return (
		<>
			{!compact && canWrite ? (
				<div className="body" style={{ borderBottom: '1px solid var(--line-soft)' }}>
					<form
						className="form-grid"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							const form = e.currentTarget
							await toast.run(
								() => post(`/api/v1/accounts/${accountId}/dns/zones/${zoneId}/records`, {
									name: fd.get('name'),
									type: fd.get('type'),
									content: fd.get('content'),
									ttl: Number(fd.get('ttl') || 300),
								}),
								() => 'Record queued. PowerDNS is updated by the reconcile job.',
							)
							form.reset()
							await records.reload()
						}}
					>
						<Field label="Name" hint="Use @ for the zone apex.">
							<input name="name" placeholder="www" required spellCheck={false} />
						</Field>
						<Field label="Type">
							<select name="type" defaultValue="A">
								{['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA'].map((t) => <option key={t} value={t}>{t}</option>)}
							</select>
						</Field>
						<Field label="Content">
							<input name="content" placeholder="203.0.113.10" required spellCheck={false} />
						</Field>
						<Field label="TTL (seconds)">
							<input name="ttl" type="number" defaultValue={300} min={60} />
						</Field>
						<div className="field" style={{ alignSelf: 'end' }}>
							<button type="submit" className="btn">Add record</button>
						</div>
					</form>
				</div>
			) : null}

			{records.loading ? <Loading /> : rows.length === 0 ? (
				<EmptyState icon="globe" title="This zone has no records">
					{canWrite ? 'Add the first record above — Kelmor writes it into PowerDNS through the reconcile job.' : 'No records have been published for this zone.'}
				</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead><tr><th>Name</th><th>Type</th><th>Content</th><th className="num">TTL</th>{canWrite ? <th /> : null}</tr></thead>
						<tbody>
							{rows.map((record) => (
								<tr key={record.id}>
									<td className="mono">{record.name}</td>
									<td><Pill tone="idle">{record.type}</Pill></td>
									<td className="mono small">{record.content}</td>
									<td className="num">{record.ttl}</td>
									{canWrite ? (
										<td className="right">
											<button type="button" className="linkish danger" onClick={() => setPending(record)}>Delete</button>
										</td>
									) : null}
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}

			{pending ? (
				<ConfirmDialog
					title="Delete this DNS record?"
					confirmLabel="Delete record"
					onCancel={() => setPending(null)}
					onConfirm={async () => {
						const record = pending
						setPending(null)
						await toast.run(
							() => del(`/api/v1/accounts/${accountId}/dns/zones/${zoneId}/records/${record.id}`),
							() => 'Record deleted',
						)
						await records.reload()
					}}
				>
					<p>
						<span className="mono">{pending.name} {pending.type} {pending.content}</span> is removed from the zone.
						Resolvers may keep the old answer until the {pending.ttl}s TTL expires.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

function EmailTab ({ accountId }: { accountId: string }) {
	const domains = useLoad<MailDomainRow[]>(() => listOf<MailDomainRow>(`/api/v1/accounts/${accountId}/mail/domains`), [accountId])
	const mailboxes = useLoad<MailboxRow[]>(() => listOf<MailboxRow>(`/api/v1/accounts/${accountId}/mail/mailboxes`), [accountId])
	const aliases = useLoad<AliasRow[]>(() => listOf<AliasRow>(`/api/v1/accounts/${accountId}/mail/aliases`).catch(() => []), [accountId])

	return (
		<>
			<Panel title="Mail domains" icon="mail" subtitle={`${domains.data?.length ?? 0} routed`} tight>
				{domains.loading ? <Loading /> : (domains.data ?? []).length === 0 ? (
					<EmptyState icon="mail" title="No mail domain">A mail domain with a DKIM key is created with the primary domain.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Domain</th><th>Catch-all policy</th><th>Status</th></tr></thead>
							<tbody>
								{(domains.data ?? []).map((domain) => (
									<tr key={domain.id}>
										<td className="mono">{shortId(domain.domain_id, 8)}</td>
										<td>{domain.catchall_policy}</td>
										<td><StatusPill status={domain.status} /></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
			<Panel title="Mailboxes" icon="inbox" subtitle={`${mailboxes.data?.length ?? 0} mailboxes`} tight>
				{mailboxes.loading ? <Loading /> : (mailboxes.data ?? []).length === 0 ? (
					<EmptyState icon="inbox" title="No mailboxes yet">Mailboxes are created by the customer in Kelmor Control, or here through the API.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Local part</th><th className="num">Quota</th><th>Status</th></tr></thead>
							<tbody>
								{(mailboxes.data ?? []).map((mailbox) => (
									<tr key={mailbox.id}>
										<td className="mono">{mailbox.local_part}</td>
										<td className="num">{mailbox.quota_bytes ? formatBytes(mailbox.quota_bytes) : 'package default'}</td>
										<td><StatusPill status={mailbox.status} /></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
			<Panel title="Aliases" icon="externalLink" subtitle={`${aliases.data?.length ?? 0} aliases`} tight>
				{(aliases.data ?? []).length === 0 ? (
					<EmptyState icon="externalLink" title="No aliases">Forwarding addresses appear here once they are created.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Address</th><th>Delivers to</th></tr></thead>
							<tbody>
								{(aliases.data ?? []).map((alias) => (
									<tr key={alias.id}><td className="mono">{alias.address}</td><td className="mono">{alias.destination}</td></tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
		</>
	)
}

function DatabasesTab ({ accountId }: { accountId: string }) {
	const databases = useLoad<DatabaseRow[]>(() => listOf<DatabaseRow>(`/api/v1/accounts/${accountId}/databases`), [accountId])

	return (
		<Panel title="Databases" icon="database" subtitle={`${databases.data?.length ?? 0} databases`} tight>
			{databases.loading ? <Loading /> : (databases.data ?? []).length === 0 ? (
				<EmptyState icon="database" title="No databases">
					MariaDB and PostgreSQL databases created for this account appear here, including the ones created by a WordPress install.
				</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead><tr><th>Name</th><th>Engine</th><th>Status</th></tr></thead>
						<tbody>
							{(databases.data ?? []).map((database) => (
								<tr key={database.id}>
									<td className="mono">{database.name}</td>
									<td>{database.engine}</td>
									<td><StatusPill status={database.status} /></td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</Panel>
	)
}

function SSLTab ({ account }: { account: AccountRow }) {
	const toast = useToast()
	const certs = useLoad<CertRow[]>(() => listOf<CertRow>(`/api/v1/accounts/${account.id}/certificates`), [account.id])
	const [busy, setBusy] = useState(false)

	return (
		<Panel
			title="Certificates"
			icon="lock"
			subtitle={`${certs.data?.length ?? 0} issued`}
			actions={
				<button
					type="button"
					className="btn small"
					disabled={busy}
					onClick={async () => {
						setBusy(true)
						await toast.run(
							() => post(`/api/v1/accounts/${account.id}/certificates`, { hostname: account.primary_domain, kind: 'acme' }),
							() => `ACME order queued for ${account.primary_domain}`,
						)
						await certs.reload()
						setBusy(false)
					}}
				>
					Request certificate
				</button>
			}
			tight
		>
			{certs.loading ? <Loading /> : (certs.data ?? []).length === 0 ? (
				<EmptyState icon="lock" title="No certificate yet">
					Request an ACME certificate for {account.primary_domain}. The order needs the domain to resolve to this host on
					port 80 for the HTTP-01 challenge.
				</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead><tr><th>Hostname</th><th>Kind</th><th>Issuer</th><th>Expires</th><th>Status</th></tr></thead>
						<tbody>
							{(certs.data ?? []).map((cert) => (
								<tr key={cert.id}>
									<td className="mono">{cert.hostname}</td>
									<td>{cert.kind}</td>
									<td>{cert.issuer || '—'}</td>
									<td>{formatDateTime(cert.not_after)}</td>
									<td><StatusPill status={cert.status} /></td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</Panel>
	)
}

function FilesTab ({ accountId }: { accountId: string }) {
	const [path, setPath] = useState('/public_html')
	const files = useLoad<FileRow[]>(() => listOf<FileRow>(`/api/v1/accounts/${accountId}/files?path=${encodeURIComponent(path)}`), [accountId, path])

	return (
		<Panel
			title="Files"
			icon="folder"
			subtitle={<span className="mono">{path}</span>}
			actions={
				<div className="btn-row">
					<button type="button" className="btn secondary small" disabled={path === '/'} onClick={() => setPath(path.split('/').slice(0, -1).join('/') || '/')}>
						Up one level
					</button>
					<button type="button" className="btn secondary small" onClick={() => files.reload()}>Refresh</button>
				</div>
			}
			tight
		>
			{files.loading ? <Loading /> : files.error ? (
				<EmptyState icon="alertCircle" title="This directory could not be read">{files.error}</EmptyState>
			) : (files.data ?? []).length === 0 ? (
				<EmptyState icon="folder" title="This directory is empty">Nothing has been written under {path} yet.</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead><tr><th>Name</th><th className="num">Size</th></tr></thead>
						<tbody>
							{(files.data ?? []).map((file) => (
								<tr key={file.name}>
									<td className="mono">
										{file.dir ? (
											<button type="button" className="linkish" onClick={() => setPath(`${path}/${file.name}`.replace('//', '/'))}>
												<Icon name="folder" size={12} /> {file.name}/
											</button>
										) : (
											<>{file.name}</>
										)}
									</td>
									<td className="num">{file.dir ? '—' : formatBytes(file.size)}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</Panel>
	)
}

function BackupsTab ({ accountId }: { accountId: string }) {
	const toast = useToast()
	const canCreate = useCan('backups.create')
	const canRestore = useCan('backups.restore')
	const backups = useLoad<BackupRow[]>(() => listOf<BackupRow>(`/api/v1/accounts/${accountId}/backups`), [accountId], 4000)
	const [destination, setDestination] = useState('local')
	const [pending, setPending] = useState<BackupRow | null>(null)

	return (
		<>
			{canCreate ? (
				<Panel title="Queue a backup" icon="archive">
					<div className="form-grid">
						<Field label="Destination" hint="Encrypted HPM1 archive: home tree, database dumps and mailbox Maildirs.">
							<select value={destination} onChange={(e) => setDestination(e.target.value)}>
								<option value="local">Local disk</option>
								<option value="sftp">Offsite SFTP</option>
								<option value="s3">S3-compatible object store</option>
							</select>
						</Field>
						<div className="field" style={{ alignSelf: 'end' }}>
							<button
								type="button"
								className="btn"
								onClick={async () => {
									await toast.run(
										() => post(`/api/v1/accounts/${accountId}/backups`, { kind: 'full', destination }),
										() => `Backup queued to ${destination}`,
									)
									await backups.reload()
								}}
							>
								Queue encrypted backup
							</button>
						</div>
					</div>
				</Panel>
			) : null}
			<Panel title="Backup runs" icon="archive" subtitle={`${backups.data?.length ?? 0} runs`} tight>
				{backups.loading ? <Loading /> : (backups.data ?? []).length === 0 ? (
					<EmptyState icon="archive" title="No backup has run for this account">
						{canCreate ? 'Queue one above. Completed runs can be restored in place.' : 'Backups queued by an administrator appear here.'}
					</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Started</th><th>Kind</th><th>Destination</th><th className="num">Size</th><th>Checksum</th><th>State</th><th /></tr></thead>
							<tbody>
								{(backups.data ?? []).map((backup) => (
									<tr key={backup.id} data-backup-id={backup.id} data-backup-state={backup.state} data-backup-destination={backup.destination}>
										<td>{formatDateTime(backup.created_at)}</td>
										<td>{backup.kind}</td>
										<td>{backup.destination}</td>
										<td className="num">{backup.size_bytes ? formatBytes(backup.size_bytes) : '—'}</td>
										<td className="mono small">{backup.checksum ? backup.checksum.slice(0, 12) : '—'}</td>
										<td><StatusPill status={backup.state} /></td>
										<td className="right">
											{backup.state === 'succeeded' && canRestore ? (
												<button type="button" className="linkish" data-restore={backup.id} onClick={() => setPending(backup)}>Restore</button>
											) : null}
										</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>
			{pending ? (
				<ConfirmDialog
					title="Restore this backup in place?"
					confirmLabel="Restore in place"
					typeToConfirm="RESTORE"
					onCancel={() => setPending(null)}
					onConfirm={async () => {
						const backup = pending
						setPending(null)
						await toast.run(
							() => post(`/api/v1/accounts/${accountId}/restores`, { backup_id: backup.id, mode: 'in_place' }),
							() => 'Restore queued',
						)
						await backups.reload()
					}}
				>
					<p>
						Restoring overwrites the current home tree, database contents and mailboxes with the {formatDateTime(pending.created_at)}{' '}
						{pending.destination} backup. Anything written since then is lost.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

function JobsTab ({ accountId }: { accountId: string }) {
	const jobs = useLoad<JobRow[]>(() => listOf<JobRow>('/api/v1/jobs'), [], 4000)
	const mine = (jobs.data ?? []).filter((job) => job.resource_id === accountId || (job.payload as { account_id?: string })?.account_id === accountId)

	return (
		<Panel title="Job history" icon="list" subtitle={`${mine.length} jobs`} actions={<Link className="btn secondary small" to="/jobs">Full job queue</Link>} tight>
			{jobs.loading && mine.length === 0 ? <Loading /> : mine.length === 0 ? (
				<EmptyState icon="list" title="No job has run for this account">Provisioning, reconcile, backup and certificate work for this account shows up here.</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead><tr><th>Started</th><th>Type</th><th>State</th><th className="num">Progress</th><th>Last error</th></tr></thead>
						<tbody>
							{mine.map((job) => (
								<tr key={job.id}>
									<td>{formatDateTime(job.created_at)}</td>
									<td className="mono">{job.type}</td>
									<td><Pill tone={statusTone(job.state)}>{job.state}</Pill></td>
									<td className="num">{job.progress}%</td>
									<td className="small">{job.last_error || '—'}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</Panel>
	)
}
