import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { UsageMeter } from '../components/usage-meter'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Reseller } from '../types'

const privilegeOptions = ['accounts.read', 'accounts.create', 'accounts.modify', 'accounts.suspend', 'packages.read', 'packages.write', 'domains.read', 'domains.write', 'dns.read', 'websites.read', 'backups.read', 'backups.create', 'backups.restore', 'billing.usage.read']

interface ResellerAccountRow {
	id: string
	username: string
	status?: string
}

interface ResellerPackageRow {
	id: string
	name: string
}

interface ResellerDetail {
	reseller?: Reseller
	accounts?: ResellerAccountRow[]
	packages?: ResellerPackageRow[]
}

export function ResellersPage () {
	const [items, setItems] = useState<Reseller[]>([])
	const [editing, setEditing] = useState<Reseller | null>(null)
	const [managing, setManaging] = useState<Reseller | null>(null)
	const [detail, setDetail] = useState<ResellerDetail | null>(null)
	const [creating, setCreating] = useState(false)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const canCreate = useCan('resellers.create')
	const canModify = useCan('resellers.modify')

	function load () {
		setLoading(true)
		setError('')
		api<{ items: Reseller[] }>('/api/v1/resellers')
			.then((result) => setItems(asList(result).map(normalizeReseller)))
			.catch((requestError) => setError(messageFrom(requestError)))
			.finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function create (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const data = new FormData(event.currentTarget)
		try {
			await api('/api/v1/resellers', { method: 'POST', body: JSON.stringify(resellerPayload(data)) })
			setCreating(false)
			setMessage('Reseller created.')
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}

	async function update (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!editing) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/resellers/${editing.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ ...editing, ...resellerPayload(data), status: data.get('status') }),
			})
			setEditing(null)
			setMessage('Reseller updated.')
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}

	async function handleManage (reseller: Reseller) {
		setManaging(reseller)
		setDetail(null)
		try {
			setDetail(await api<ResellerDetail>(`/api/v1/resellers/${reseller.id}`))
		} catch (requestError) {
			setMessage(messageFrom(requestError))
			setManaging(null)
		}
	}

	return (
		<>
			<PageHeader
				title="Edit Reseller Nameservers and Privileges"
				description="Manage reseller identity, account counts, package limits, and privilege masks."
				actions={canCreate ? <button type="button" onClick={() => setCreating(true)}>Create reseller</button> : undefined}
			/>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading resellers…" /> : (
				<div className="table-wrap"><table className="dense-table">
					<thead>
						<tr>
							<th>Reseller</th>
							<th>Accounts</th>
							<th>Package / limits</th>
							<th>Status</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{items.length ? items.map((reseller) => (
							<tr key={reseller.id}>
								<td>
									<strong>{reseller.name}</strong>
									<small>{reseller.id}</small>
									<small>{reseller.brand_name || 'No brand'}</small>
								</td>
								<td>{reseller.accounts ?? 0} · {reseller.active ?? 0} active · {reseller.suspended ?? 0} suspended</td>
								<td>
									<div>{(reseller.packages || []).join(', ') || 'No assigned packages'}</div>
									<UsageMeter label="Disk" used={reseller.disk_bytes} limit={reseller.disk_limit} />
									<UsageMeter label="Bandwidth" used={reseller.bandwidth_bytes} limit={reseller.bandwidth_limit} />
								</td>
								<td><StatusBadge value={reseller.status} /></td>
								<td>
									<div className="row-actions">
										{canModify ? <button type="button" className="link-button" onClick={() => setEditing(reseller)}>Edit</button> : <span>View only</span>}
										<Link to={`/resellers/usage?reseller=${encodeURIComponent(reseller.id)}`}>View usage</Link>
										<button type="button" className="link-button" onClick={() => handleManage(reseller)}>Manage</button>
									</div>
								</td>
							</tr>
						)) : (
							<tr className="table-empty-row"><td colSpan={5}>No resellers yet</td></tr>
						)}
					</tbody>
				</table></div>
			)}
			{!loading && !items.length ? (
				<EmptyState
					title="No resellers"
					detail="Create a reseller to delegate packages and customer accounts. The table stays available so create, edit, and usage actions keep the same chrome."
					action={canCreate ? <button type="button" onClick={() => setCreating(true)}>Create reseller</button> : undefined}
				/>
			) : null}
			<Dialog open={creating} title="Create reseller" onClose={() => setCreating(false)}>
				<ResellerForm onSubmit={create} onCancel={() => setCreating(false)} />
			</Dialog>
			<Dialog open={Boolean(editing)} title={`Edit ${editing?.name || 'reseller'}`} onClose={() => setEditing(null)}>
				{editing ? <ResellerForm value={editing} onSubmit={update} onCancel={() => setEditing(null)} /> : null}
			</Dialog>
			<Dialog open={Boolean(managing)} title={`Manage ${managing?.name || 'reseller'}`} onClose={() => { setManaging(null); setDetail(null) }}>
				{managing && detail ? (
					<div>
						<p>{detail.accounts?.length || 0} accounts · {detail.packages?.length || 0} packages</p>
						<ul className="link-list">
							{(detail.accounts || []).map((account) => (
								<li key={account.id}>{account.username}{account.status ? ` · ${account.status}` : ''}</li>
							))}
							{(detail.packages || []).map((pkg) => (
								<li key={pkg.id}>{pkg.name}</li>
							))}
						</ul>
						<div className="row-actions">
							<Link to="/accounts/ownership">Change ownership</Link>
							<Link to="/accounts/suspension">Suspension</Link>
							<Link to={`/resellers/usage?reseller=${encodeURIComponent(managing.id)}`}>View usage</Link>
						</div>
						<footer className="dialog-form-actions">
							<button type="button" className="secondary" onClick={() => { setManaging(null); setDetail(null) }}>Close</button>
						</footer>
					</div>
				) : <LoadingState label="Loading reseller…" />}
			</Dialog>
		</>
	)
}

function ResellerForm ({ value, onSubmit, onCancel }: { value?: Reseller; onSubmit: (event: React.FormEvent<HTMLFormElement>) => void; onCancel: () => void }) {
	const privileges = value?.privilege_mask || []
	const nameservers = value?.nameservers || []
	return <form onSubmit={onSubmit}>
		<div className="form-grid">
			<label>Display name<input name="name" defaultValue={value?.name} required autoFocus /></label>
			<label>Brand name<input name="brand_name" defaultValue={value?.brand_name} /></label>
			{!value ? (
				<>
					<label>Login username<input name="username" required /></label>
					<label>Contact email<input name="email" type="email" /></label>
					<label>Initial password<input name="password" type="password" minLength={12} required /></label>
				</>
			) : (
				<label>Status<select name="status" defaultValue={value.status}><option>active</option><option>suspended</option><option>inactive</option></select></label>
			)}
			<label className="wide-field">Nameservers (comma-separated)<input name="nameservers" defaultValue={nameservers.join(', ') || 'ns1.localhost, ns2.localhost'} /></label>
		</div>
		<fieldset>
			<legend>Privilege mask</legend>
			<div className="checkbox-grid">
				{privilegeOptions.map((privilege) => (
					<label className="checkbox-label" key={privilege}>
						<input type="checkbox" name="privilege" value={privilege} defaultChecked={!value || privileges.includes(privilege)} />
						{privilege}
					</label>
				))}
			</div>
		</fieldset>
		<footer className="dialog-form-actions">
			<button type="button" className="secondary" onClick={onCancel}>Cancel</button>
			<button type="submit">{value ? 'Save reseller' : 'Create reseller'}</button>
		</footer>
	</form>
}

function resellerPayload (data: FormData) {
	return {
		name: data.get('name'),
		username: data.get('username'),
		email: data.get('email'),
		password: data.get('password'),
		brand_name: data.get('brand_name'),
		nameservers: String(data.get('nameservers')).split(',').map((value) => value.trim()).filter(Boolean),
		privilege_mask: data.getAll('privilege'),
	}
}

function normalizeReseller (reseller: Reseller): Reseller {
	return {
		...reseller,
		privilege_mask: reseller.privilege_mask || [],
		nameservers: reseller.nameservers || [],
		packages: reseller.packages || [],
		accounts: reseller.accounts || 0,
		active: reseller.active || 0,
		suspended: reseller.suspended || 0,
		disk_bytes: reseller.disk_bytes || 0,
		disk_limit: reseller.disk_limit || 0,
		bandwidth_bytes: reseller.bandwidth_bytes || 0,
		bandwidth_limit: reseller.bandwidth_limit || 0,
	}
}
