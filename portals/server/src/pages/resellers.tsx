import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, listOf, patch, post } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, Field, KeyValues, Loading, Notice, PageHeader, Panel, Pill, StatusPill } from '../components/ui'
import type { AccountRow } from '../components/account-picker'
import type { PackageRow } from './packages'

export interface ResellerRow {
	id: string
	user_id: string
	name: string
	brand_name?: string
	privilege_mask: string[]
	nameservers: string[]
	status: string
}

interface ResellerDetailResponse {
	reseller: ResellerRow
	accounts: AccountRow[] | null
	packages: PackageRow[] | null
	owner: { username?: string; email?: string } | null
}

/** Capabilities the control plane accepts in a reseller privilege mask. */
const resellerPrivileges = [
	{ cap: 'accounts.create', label: 'Create hosting accounts' },
	{ cap: 'accounts.modify', label: 'Modify their accounts' },
	{ cap: 'accounts.suspend', label: 'Suspend and restore their accounts' },
	{ cap: 'packages.write', label: 'Create and edit their own packages' },
	{ cap: 'domains.write', label: 'Add domains to their accounts' },
	{ cap: 'backups.create', label: 'Queue backups' },
	{ cap: 'backups.restore', label: 'Restore backups' },
]

export function ResellerList () {
	const navigate = useNavigate()
	const canCreate = useCan('resellers.create')
	const resellers = useLoad<ResellerRow[]>(() => listOf<ResellerRow>('/api/v1/resellers'), [])
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts').catch(() => []), [])

	const owned = (id: string) => (accounts.data ?? []).filter((a) => a.reseller_id === id).length

	const columns: Column<ResellerRow>[] = [
		{ key: 'name', header: 'Reseller', sort: (r) => r.name, render: (r) => <Link to={`/resellers/${r.id}`}><strong>{r.name}</strong></Link> },
		{ key: 'brand', header: 'Brand', sort: (r) => r.brand_name || '', render: (r) => r.brand_name || <span className="muted">—</span> },
		{
			key: 'nameservers',
			header: 'Nameservers',
			sort: (r) => (r.nameservers || []).join(' '),
			render: (r) => <span className="mono small">{(r.nameservers || []).join(', ') || '—'}</span>,
		},
		{
			key: 'privileges',
			header: 'Privileges',
			sort: (r) => (r.privilege_mask || []).length,
			render: (r) => (r.privilege_mask?.length ? <Pill tone="busy">{r.privilege_mask.length} granted</Pill> : <Pill tone="idle">role default</Pill>),
		},
		{ key: 'accounts', header: 'Accounts', align: 'right', sort: (r) => owned(r.id), render: (r) => (owned(r.id) ? <Link to={`/resellers/${r.id}`}>{owned(r.id)}</Link> : <span className="muted">0</span>) },
		{ key: 'status', header: 'Status', sort: (r) => r.status, render: (r) => <StatusPill status={r.status} /> },
	]

	return (
		<>
			<PageHeader
				title="Resellers"
				description="A reseller owns a slice of this server: their own sign-in, their own packages and only the accounts they created. They never see foreign customers or host secrets."
				favoritePath="/resellers"
				actions={canCreate ? <Link className="btn" to="/resellers/new">Add a reseller</Link> : null}
			/>
			<DataTable
				rows={resellers.data ?? []}
				columns={columns}
				rowKey={(r) => r.id}
				loading={resellers.loading}
				error={resellers.error}
				searchPlaceholder="Search resellers"
				noun="resellers"
				initialSort={{ key: 'name', dir: 'asc' }}
				empty={
					<EmptyState
						icon="briefcase"
						title="No resellers yet"
						action={canCreate ? <Link className="btn" to="/resellers/new">Add a reseller</Link> : undefined}
					>
						Create one to delegate account creation and package management without handing over server administration.
					</EmptyState>
				}
				rowActions={[{ label: 'Open reseller', onSelect: (r) => navigate(`/resellers/${r.id}`) }]}
			/>
		</>
	)
}

export function ResellerCreate () {
	const navigate = useNavigate()
	const toast = useToast()
	const [busy, setBusy] = useState(false)

	return (
		<>
			<PageHeader
				title="Add a Reseller"
				description="Creates the reseller brand and a sign-in with the reseller role. They can then create accounts against the packages you make available."
				crumbs={[{ label: 'Resellers', to: '/resellers' }, { label: 'Add a Reseller' }]}
				favoritePath="/resellers/new"
				actions={<Link className="btn secondary" to="/resellers">Back to Resellers</Link>}
			/>
			<Panel title="Reseller details" icon="briefcase">
				<form
					onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						setBusy(true)
						const created = await toast.run(
							() => post<ResellerRow>('/api/v1/resellers', {
								name: fd.get('name'),
								brand_name: fd.get('brand_name') || undefined,
								username: fd.get('username'),
								password: fd.get('password'),
								email: fd.get('email') || undefined,
								nameservers: String(fd.get('nameservers') || '')
									.split(',')
									.map((s) => s.trim())
									.filter(Boolean),
							}),
							(reseller) => `${reseller.name} created`,
						)
						setBusy(false)
						if (created) navigate(`/resellers/${created.id}`)
					}}
				>
					<div className="form-grid">
						<Field label="Reseller name" hint="Shown in Director and on account ownership.">
							<input name="name" required autoFocus />
						</Field>
						<Field label="Brand name" hint="Optional customer-facing brand for this reseller.">
							<input name="brand_name" />
						</Field>
						<Field label="Sign-in username" hint="Creates a Director login with the reseller role.">
							<input name="username" required spellCheck={false} />
						</Field>
						<Field label="Contact email">
							<input name="email" type="email" />
						</Field>
						<Field label="Sign-in password" hint="At least 12 characters.">
							<input name="password" type="password" minLength={12} required autoComplete="new-password" />
						</Field>
						<Field label="Nameservers" hint="Comma separated. Used for the zones this reseller creates.">
							<input name="nameservers" placeholder="ns1.example.net, ns2.example.net" spellCheck={false} />
						</Field>
					</div>
					<div className="form-actions">
						<button type="submit" className="btn" disabled={busy}>{busy ? 'Creating…' : 'Create reseller'}</button>
						<Link className="btn secondary" to="/resellers">Cancel</Link>
					</div>
				</form>
			</Panel>
		</>
	)
}

export function ResellerDetail () {
	const { resellerId = '' } = useParams()
	const toast = useToast()
	const canModify = useCan('resellers.modify')
	const detail = useLoad<ResellerDetailResponse>(() => api<ResellerDetailResponse>(`/api/v1/resellers/${resellerId}`), [resellerId])
	const [name, setName] = useState('')
	const [brand, setBrand] = useState('')
	const [status, setStatus] = useState('active')
	const [nameservers, setNameservers] = useState('')
	const [mask, setMask] = useState<string[]>([])
	const [busy, setBusy] = useState(false)

	useEffect(() => {
		const reseller = detail.data?.reseller
		if (!reseller) return
		setName(reseller.name)
		setBrand(reseller.brand_name || '')
		setStatus(reseller.status)
		setNameservers((reseller.nameservers || []).join(', '))
		setMask(reseller.privilege_mask || [])
	}, [detail.data])

	if (detail.loading) return <Loading label="Loading reseller" />
	if (detail.error || !detail.data) {
		return (
			<>
				<PageHeader title="Reseller" crumbs={[{ label: 'Resellers', to: '/resellers' }, { label: 'Not found' }]} />
				<Panel><EmptyState icon="alertCircle" title="This reseller could not be loaded">{detail.error}</EmptyState></Panel>
			</>
		)
	}

	const { reseller, accounts, packages, owner } = detail.data

	async function save () {
		setBusy(true)
		await toast.run(
			() => patch(`/api/v1/resellers/${resellerId}`, {
				name,
				brand_name: brand,
				status,
				nameservers: nameservers.split(',').map((s) => s.trim()).filter(Boolean),
				privilege_mask: mask,
			}),
			() => `${name} saved`,
		)
		await detail.reload()
		setBusy(false)
	}

	return (
		<>
			<PageHeader
				title={reseller.name}
				description="Ownership, delegated privileges and the accounts and packages this reseller controls."
				crumbs={[{ label: 'Resellers', to: '/resellers' }, { label: reseller.name }]}
				actions={<Link className="btn secondary" to="/resellers">Back to Resellers</Link>}
			/>

			<div className="grid-2">
				<Panel title="Identity" icon="briefcase">
					{canModify ? (
						<>
							<div className="form-grid">
								<Field label="Reseller name"><input value={name} onChange={(e) => setName(e.target.value)} /></Field>
								<Field label="Brand name"><input value={brand} onChange={(e) => setBrand(e.target.value)} /></Field>
								<Field label="Status" hint="Suspending a reseller does not suspend the accounts they own.">
									<select value={status} onChange={(e) => setStatus(e.target.value)}>
										<option value="active">active</option>
										<option value="suspended">suspended</option>
									</select>
								</Field>
								<Field label="Nameservers" hint="Comma separated.">
									<input value={nameservers} onChange={(e) => setNameservers(e.target.value)} spellCheck={false} />
								</Field>
							</div>
							<div className="form-actions">
								<button type="button" className="btn" disabled={busy} onClick={save}>{busy ? 'Saving…' : 'Save reseller'}</button>
							</div>
						</>
					) : (
						<KeyValues
							rows={[
								['Name', reseller.name],
								['Brand', reseller.brand_name || '—'],
								['Status', <StatusPill key="s" status={reseller.status} />],
								['Nameservers', <span key="ns" className="mono small">{(reseller.nameservers || []).join(', ') || '—'}</span>],
								['Sign-in', owner?.username || '—'],
								['Contact', owner?.email || '—'],
							]}
						/>
					)}
				</Panel>

				<Panel title="Delegated privileges" icon="shield" subtitle="Bounded by the reseller role">
					<Notice tone="info">
						An empty mask means the reseller keeps the default reseller role. Anything you tick here is still checked by
						the control plane on every request — Director only hides what the API would refuse.
					</Notice>
					<div style={{ display: 'grid', gap: 8 }}>
						{resellerPrivileges.map((privilege) => (
							<div className="field inline" key={privilege.cap}>
								<input
									type="checkbox"
									id={privilege.cap}
									disabled={!canModify}
									checked={mask.includes(privilege.cap)}
									onChange={(e) =>
										setMask((prev) => (e.target.checked ? [...prev, privilege.cap] : prev.filter((c) => c !== privilege.cap)))
									}
								/>
								<label htmlFor={privilege.cap}>
									{privilege.label} <span className="mono small muted">{privilege.cap}</span>
								</label>
							</div>
						))}
					</div>
					{canModify ? (
						<div className="form-actions">
							<button type="button" className="btn" disabled={busy} onClick={save}>Save privileges</button>
							<button type="button" className="btn secondary" onClick={() => setMask([])}>Clear mask</button>
						</div>
					) : null}
				</Panel>
			</div>

			<Panel title="Accounts owned" icon="users" subtitle={`${accounts?.length ?? 0} accounts`} tight>
				{(accounts ?? []).length === 0 ? (
					<EmptyState icon="users" title="This reseller has no accounts yet">Accounts they create are listed here with their live status.</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>User</th><th>Primary domain</th><th>Status</th></tr></thead>
							<tbody>
								{(accounts ?? []).map((account) => (
									<tr key={account.id}>
										<td><Link to={`/accounts/${account.id}`}>{account.username}</Link></td>
										<td className="mono">{account.primary_domain}</td>
										<td><StatusPill status={account.status} /></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>

			<Panel title="Packages owned" icon="box" subtitle={`${packages?.length ?? 0} packages`} tight>
				{(packages ?? []).length === 0 ? (
					<EmptyState icon="box" title="No reseller-owned packages">
						This reseller uses the server packages. Packages they create themselves appear here and stay private to them.
					</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Package</th><th className="num">Disk</th><th className="num">Transfer</th></tr></thead>
							<tbody>
								{(packages ?? []).map((pkg) => (
									<tr key={pkg.id}>
										<td><Link to={`/packages/${pkg.id}`}>{pkg.name}</Link></td>
										<td className="num">{(pkg.disk_bytes / 1024 ** 3).toFixed(1)} GB</td>
										<td className="num">{(pkg.bandwidth_bytes_monthly / 1024 ** 3).toFixed(1)} GB</td>
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
