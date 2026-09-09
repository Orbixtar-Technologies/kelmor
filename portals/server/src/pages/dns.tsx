import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, listOf, post } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, KeyValues, Loading, Notice, PageHeader, Panel, Pill } from '../components/ui'
import { ZoneRecords } from './account-detail'

interface ZoneSummary {
	id: string
	account_id: string
	account_username: string
	domain_id: string
	name: string
	dnssec_enabled: boolean
	provider: string
	records: number
	desired_revision: number
	observed_revision: number
}

interface DSRecord {
	key_tag?: number
	algorithm?: number
	digest_type?: number
	digest?: string
	record?: string
}

export function DNSZoneList () {
	const navigate = useNavigate()
	const zones = useLoad<ZoneSummary[]>(() => listOf<ZoneSummary>('/api/v1/server/dns/zones'), [])

	const columns: Column<ZoneSummary>[] = [
		{
			key: 'name',
			header: 'Zone',
			sort: (z) => z.name,
			render: (z) => <Link to={`/dns/zones/${z.account_id}/${z.id}`}><strong className="mono">{z.name}</strong></Link>,
		},
		{
			key: 'account',
			header: 'Account',
			sort: (z) => z.account_username,
			render: (z) => <Link to={`/accounts/${z.account_id}`}>{z.account_username || '—'}</Link>,
		},
		{ key: 'records', header: 'Records', align: 'right', sort: (z) => z.records },
		{ key: 'provider', header: 'Provider', sort: (z) => z.provider, render: (z) => <span className="mono small">{z.provider || 'powerdns'}</span> },
		{
			key: 'dnssec',
			header: 'DNSSEC',
			sort: (z) => (z.dnssec_enabled ? 'signed' : 'unsigned'),
			render: (z) => (z.dnssec_enabled ? <Pill tone="ok">signed</Pill> : <Pill tone="idle">unsigned</Pill>),
		},
		{
			key: 'drift',
			header: 'Reconciliation',
			sort: (z) => z.desired_revision - z.observed_revision,
			render: (z) =>
				z.desired_revision > z.observed_revision ? <Pill tone="busy">pending</Pill> : <Pill tone="ok">in sync</Pill>,
		},
	]

	return (
		<>
			<PageHeader
				title="DNS Zone Manager"
				description="Every authoritative zone Kelmor serves from this host. Zones are created with their account primary domain and published to PowerDNS by the reconcile job."
				favoritePath="/dns/zones"
				actions={<button type="button" className="btn secondary" onClick={() => zones.reload()}>Refresh</button>}
			/>
			<DataTable
				rows={zones.data ?? []}
				columns={columns}
				rowKey={(z) => z.id}
				loading={zones.loading}
				error={zones.error}
				searchPlaceholder="Search zone or account"
				noun="zones"
				initialSort={{ key: 'name', dir: 'asc' }}
				empty={
					<EmptyState icon="globe" title="No DNS zone is served yet" action={<Link className="btn secondary" to="/dns/add-zone">How zones are created</Link>}>
						A zone appears here as soon as the first hosting account finishes provisioning.
					</EmptyState>
				}
				rowActions={[
					{ label: 'Open zone manager', onSelect: (z) => navigate(`/dns/zones/${z.account_id}/${z.id}`) },
					{ label: 'Open account', onSelect: (z) => navigate(`/accounts/${z.account_id}`) },
				]}
			/>
		</>
	)
}

export function DNSZoneDetail () {
	const { accountId = '', zoneId = '' } = useParams()
	const toast = useToast()
	const canWrite = useCan('dns.write')
	const zones = useLoad<ZoneSummary[]>(() => listOf<ZoneSummary>('/api/v1/server/dns/zones'), [])
	const [busy, setBusy] = useState(false)
	const [ds, setDs] = useState<DSRecord[] | null>(null)

	const zone = (zones.data ?? []).find((z) => z.id === zoneId)

	if (zones.loading) return <Loading label="Loading zone" />
	if (!zone) {
		return (
			<>
				<PageHeader title="DNS zone" crumbs={[{ label: 'DNS Functions' }, { label: 'Zone Manager', to: '/dns/zones' }, { label: 'Not found' }]} />
				<Panel><EmptyState icon="alertCircle" title="This zone is no longer served">It may have been removed with its account.</EmptyState></Panel>
			</>
		)
	}

	async function toggleDNSSEC () {
		setBusy(true)
		await toast.run(
			() => post(`/api/v1/accounts/${accountId}/dns/zones/${zoneId}/dnssec`, { enabled: !zone!.dnssec_enabled }),
			() => (zone!.dnssec_enabled ? 'Zone unsigned' : 'Zone signing queued — collect the DS record for the registrar'),
		)
		await zones.reload()
		setBusy(false)
	}

	async function loadDS () {
		setBusy(true)
		await toast.run(
			async () => {
				const r = await api<{ items?: DSRecord[]; records?: DSRecord[] }>(`/api/v1/accounts/${accountId}/dns/zones/${zoneId}/ds`)
				setDs(r.items ?? r.records ?? [])
				return r
			},
			() => 'DS records loaded',
		)
		setBusy(false)
	}

	return (
		<>
			<PageHeader
				title={zone.name}
				description="Records published to PowerDNS for this zone. Changes are written by the reconcile job, not directly from the browser."
				crumbs={[{ label: 'DNS Functions' }, { label: 'Zone Manager', to: '/dns/zones' }, { label: zone.name }]}
				actions={<Link className="btn secondary" to={`/accounts/${accountId}`}>Open account</Link>}
			/>

			<div className="grid-2">
				<Panel title="Zone" icon="globe">
					<KeyValues
						rows={[
							['Zone', <span key="z" className="mono">{zone.name}</span>],
							['Account', <Link key="a" to={`/accounts/${zone.account_id}`}>{zone.account_username}</Link>],
							['Provider', zone.provider || 'powerdns'],
							['Records', zone.records],
							['DNSSEC', zone.dnssec_enabled ? <Pill key="d" tone="ok">signed</Pill> : <Pill key="d" tone="idle">unsigned</Pill>],
							['Reconciliation', zone.desired_revision > zone.observed_revision ? <Pill key="r" tone="busy">pending</Pill> : <Pill key="r" tone="ok">in sync</Pill>],
						]}
					/>
				</Panel>
				<Panel
					title="DNSSEC"
					icon="shield"
					actions={canWrite ? (
						<button type="button" className="btn secondary small" disabled={busy} onClick={toggleDNSSEC}>
							{zone.dnssec_enabled ? 'Unsign zone' : 'Sign zone'}
						</button>
					) : null}
				>
					<p className="small muted">
						Signing runs <code>pdnsutil secure-zone</code> on the bind backend. Hand the DS record below to the domain
						registrar — the chain of trust is not complete until they publish it.
					</p>
					<div className="btn-row" style={{ marginTop: 12 }}>
						<button type="button" className="btn secondary small" disabled={busy} onClick={loadDS}>Show DS records</button>
					</div>
					{ds ? (
						ds.length === 0 ? (
							<div style={{ marginTop: 12 }}><Notice tone="warn">No DS record is published yet. Sign the zone first.</Notice></div>
						) : (
							<pre className="code" style={{ marginTop: 12 }}>{ds.map((d) => d.record ?? `${d.key_tag} ${d.algorithm} ${d.digest_type} ${d.digest}`).join('\n')}</pre>
						)
					) : null}
				</Panel>
			</div>

			<Panel title="Records" icon="list" subtitle={`${zone.records} published`} tight>
				<ZoneRecords accountId={accountId} zoneId={zoneId} />
			</Panel>
		</>
	)
}

export function DNSAddZone () {
	const zones = useLoad<ZoneSummary[]>(() => listOf<ZoneSummary>('/api/v1/server/dns/zones'), [])
	const canCreateAccount = useCan('accounts.create')

	return (
		<>
			<PageHeader
				title="Add a DNS Zone"
				description="Kelmor derives zones from hosting state instead of letting them drift apart from the accounts they belong to."
				favoritePath="/dns/add-zone"
			/>
			<Notice tone="info">
				Zones are not created standalone. Every zone belongs to a domain, and every domain belongs to a hosting account,
				so the zone, the vhost and the mail routing are always created and removed together.
			</Notice>
			<Panel title="How to get a new zone" icon="globe">
				<ol style={{ margin: 0, paddingLeft: 20, lineHeight: 2 }}>
					<li>
						For a new customer, {canCreateAccount ? <Link to="/accounts/create">create a hosting account</Link> : 'ask an administrator to create a hosting account'}.
						The primary domain gets its zone during provisioning.
					</li>
					<li>For an extra domain on an existing account, add the domain to that account; Kelmor creates the matching zone.</li>
					<li>
						To edit records on an existing zone, open the <Link to="/dns/zones">DNS Zone Manager</Link>.
					</li>
				</ol>
				<p className="small muted" style={{ marginTop: 14 }}>
					A standalone zone API — a zone with no hosting account behind it, for secondary DNS or vanity delegation — is
					not exposed by this control plane build. It requires a server-scoped zone endpoint in the API and a matching
					PowerDNS operation in the privileged agent.
				</p>
			</Panel>
			<Panel title="Zones currently served" icon="list" subtitle={`${zones.data?.length ?? 0} zones`} tight>
				{zones.loading ? <Loading /> : (zones.data ?? []).length === 0 ? (
					<EmptyState icon="globe" title="No zone is served yet">The first hosting account brings the first zone.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Zone</th><th>Account</th><th className="num">Records</th></tr></thead>
							<tbody>
								{(zones.data ?? []).map((zone) => (
									<tr key={zone.id}>
										<td className="mono"><Link to={`/dns/zones/${zone.account_id}/${zone.id}`}>{zone.name}</Link></td>
										<td><Link to={`/accounts/${zone.account_id}`}>{zone.account_username}</Link></td>
										<td className="num">{zone.records}</td>
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

export function DNSSECOverview () {
	const zones = useLoad<ZoneSummary[]>(() => listOf<ZoneSummary>('/api/v1/server/dns/zones'), [])
	const rows = zones.data ?? []
	const signed = rows.filter((z) => z.dnssec_enabled)

	const columns: Column<ZoneSummary>[] = [
		{ key: 'name', header: 'Zone', sort: (z) => z.name, render: (z) => <Link className="mono" to={`/dns/zones/${z.account_id}/${z.id}`}>{z.name}</Link> },
		{ key: 'account', header: 'Account', sort: (z) => z.account_username, render: (z) => <Link to={`/accounts/${z.account_id}`}>{z.account_username}</Link> },
		{
			key: 'dnssec',
			header: 'Signing',
			sort: (z) => (z.dnssec_enabled ? 'signed' : 'unsigned'),
			render: (z) => (z.dnssec_enabled ? <Pill tone="ok">signed</Pill> : <Pill tone="idle">unsigned</Pill>),
		},
		{
			key: 'action',
			header: '',
			sortable: false,
			render: (z) => <Link className="btn secondary small" to={`/dns/zones/${z.account_id}/${z.id}`}>Manage signing</Link>,
		},
	]

	return (
		<>
			<PageHeader
				title="DNSSEC"
				description="Signing state per zone and where to collect the DS record each registrar needs."
				favoritePath="/dns/dnssec"
			/>
			<Notice tone="info">
				{signed.length} of {rows.length} zones are signed. Signing alone does not protect a domain: the registrar must
				publish the DS record before resolvers validate it.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(z) => z.id}
				loading={zones.loading}
				error={zones.error}
				searchPlaceholder="Search zone or account"
				noun="zones"
				initialSort={{ key: 'dnssec', dir: 'asc' }}
				empty={<EmptyState icon="shield" title="No zone to sign yet">Zones appear once the first hosting account is provisioned.</EmptyState>}
			/>
		</>
	)
}
