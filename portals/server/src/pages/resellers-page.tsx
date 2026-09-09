import { useEffect, useState } from 'react'
import { api, asList } from '../client'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Reseller } from '../types'

const privilegeOptions = ['accounts.read', 'accounts.create', 'accounts.modify', 'accounts.suspend', 'domains.read', 'domains.write', 'websites.write', 'databases.write', 'dns.read', 'dns.write', 'mail.write', 'files.write', 'backups.create', 'backups.restore', 'packages.read', 'packages.write']

export function ResellersPage () {
	const [items, setItems] = useState<Reseller[]>([])
	const [editing, setEditing] = useState<Reseller | null>(null)
	const [creating, setCreating] = useState(false)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const canCreate = useCan('resellers.create')
	const canModify = useCan('resellers.modify')
	function load () {
		setLoading(true)
		api<{ items: Reseller[] }>('/api/v1/resellers').then((result) => setItems(asList(result))).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function create (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const data = new FormData(event.currentTarget)
		try {
			await api('/api/v1/resellers', { method: 'POST', body: JSON.stringify({ name: data.get('name'), username: data.get('username'), email: data.get('email'), password: data.get('password'), brand_name: data.get('brand_name'), nameservers: String(data.get('nameservers')).split(',').map((value) => value.trim()).filter(Boolean), privilege_mask: data.getAll('privilege') }) })
			setCreating(false); setMessage('Reseller created.'); load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	async function update (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!editing) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/resellers/${editing.id}`, { method: 'PATCH', body: JSON.stringify({ ...editing, name: data.get('name'), brand_name: data.get('brand_name'), status: data.get('status'), nameservers: String(data.get('nameservers')).split(',').map((value) => value.trim()).filter(Boolean), privilege_mask: data.getAll('privilege') }) })
			setEditing(null); setMessage('Reseller updated.'); load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	return (
		<>
			<PageHeader title="Resellers" description="Delegate account operations through explicit privilege masks and account membership." actions={canCreate ? <button type="button" onClick={() => setCreating(true)}>Create reseller</button> : undefined} />
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading resellers…" /> : <div className="table-wrap"><table className="dense-table"><thead><tr><th>Name</th><th>Brand</th><th>Status</th><th>Nameservers</th><th>Privileges</th><th>Actions</th></tr></thead><tbody>
				{items.map((reseller) => <tr key={reseller.id}><td><strong>{reseller.name}</strong></td><td>{reseller.brand_name || '—'}</td><td><StatusBadge value={reseller.status} /></td><td>{reseller.nameservers.join(', ') || '—'}</td><td>{reseller.privilege_mask.length} granted</td><td>{canModify ? <button type="button" className="link-button" onClick={() => setEditing(reseller)}>Edit</button> : 'View only'}</td></tr>)}
			</tbody></table></div>}
			{!loading && !items.length ? <EmptyState title="No resellers" detail="Create one to delegate packages and customer accounts." /> : null}
			<Dialog open={creating} title="Create reseller" onClose={() => setCreating(false)}><ResellerForm onSubmit={create} onCancel={() => setCreating(false)} /></Dialog>
			<Dialog open={Boolean(editing)} title={`Edit ${editing?.name || 'reseller'}`} onClose={() => setEditing(null)}>{editing ? <ResellerForm value={editing} onSubmit={update} onCancel={() => setEditing(null)} /> : null}</Dialog>
		</>
	)
}

function ResellerForm ({ value, onSubmit, onCancel }: { value?: Reseller; onSubmit: (event: React.FormEvent<HTMLFormElement>) => void; onCancel: () => void }) {
	return <form onSubmit={onSubmit}>
		<div className="form-grid">
			<label>Display name<input name="name" defaultValue={value?.name} required autoFocus /></label><label>Brand name<input name="brand_name" defaultValue={value?.brand_name} /></label>
			{!value ? <><label>Login username<input name="username" required /></label><label>Contact email<input name="email" type="email" /></label><label>Initial password<input name="password" type="password" minLength={12} required /></label></> : <label>Status<select name="status" defaultValue={value.status}><option>active</option><option>suspended</option><option>inactive</option></select></label>}
			<label className="wide-field">Nameservers (comma-separated)<input name="nameservers" defaultValue={value?.nameservers.join(', ') || 'ns1.localhost, ns2.localhost'} /></label>
		</div>
		<fieldset><legend>Privilege mask</legend><div className="checkbox-grid">{privilegeOptions.map((privilege) => <label className="checkbox-label" key={privilege}><input type="checkbox" name="privilege" value={privilege} defaultChecked={value?.privilege_mask.includes(privilege)} />{privilege}</label>)}</div></fieldset>
		<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={onCancel}>Cancel</button><button type="submit">{value ? 'Save reseller' : 'Create reseller'}</button></footer>
	</form>
}
