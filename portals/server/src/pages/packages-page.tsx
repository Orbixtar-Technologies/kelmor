import { useEffect, useState } from 'react'
import { api, asList } from '../client'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Package } from '../types'

const numericPackageFields: NumericPackageFieldDefinition[] = [
	{ field: 'disk_bytes', label: 'Disk bytes' },
	{ field: 'bandwidth_bytes_monthly', label: 'Monthly bandwidth bytes' },
	{ field: 'domains', label: 'Domains' },
	{ field: 'subdomains', label: 'Subdomains' },
	{ field: 'alias_domains', label: 'Alias domains' },
	{ field: 'databases', label: 'Databases' },
	{ field: 'database_users', label: 'Database users' },
	{ field: 'mailboxes', label: 'Mailboxes' },
	{ field: 'mailbox_storage_bytes', label: 'Mailbox storage bytes' },
	{ field: 'ftp_users', label: 'FTP users' },
	{ field: 'cron_jobs', label: 'Cron jobs' },
	{ field: 'application_instances', label: 'Application instances' },
	{ field: 'backup_retention_days', label: 'Backup retention days' },
	{ field: 'cpu_percent', label: 'CPU percent' },
	{ field: 'memory_bytes', label: 'Memory bytes' },
	{ field: 'process_limit', label: 'Process limit' },
	{ field: 'io_weight', label: 'I/O weight' },
	{ field: 'iops', label: 'IOPS' },
	{ field: 'concurrent_web_requests', label: 'Concurrent web requests' },
	{ field: 'email_daily_limit', label: 'Daily email limit' },
]

const defaults: Package = {
	id: '', name: '', feature_set_id: '', disk_bytes: 10737418240, bandwidth_bytes_monthly: 107374182400,
	domains: 5, subdomains: 20, alias_domains: 10, databases: 5, database_users: 10,
	mailboxes: 20, mailbox_storage_bytes: 2147483648, ftp_users: 5, cron_jobs: 10,
	application_instances: 3, backup_retention_days: 7, cpu_percent: 200, memory_bytes: 2147483648,
	process_limit: 150, io_weight: 100, iops: 800, concurrent_web_requests: 100, email_daily_limit: 200,
}

function packageFromForm (data: FormData, current: Package): Package {
	const next = { ...current, name: String(data.get('name') || ''), reseller_id: String(data.get('reseller_id') || ''), feature_set_id: String(data.get('feature_set_id') || '') }
	numericPackageFields.forEach(({ field }) => { next[field] = Number(data.get(field)) })
	return next
}

export function PackagesPage () {
	const [items, setItems] = useState<Package[]>([])
	const [editing, setEditing] = useState<Package | null>(null)
	const [deleting, setDeleting] = useState<Package | null>(null)
	const [confirmation, setConfirmation] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const canWrite = useCan('packages.write')

	function load () {
		setLoading(true)
		api<{ items: Package[] }>('/api/v1/packages').then((result) => setItems(asList(result))).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
	}
	useEffect(load, [])
	async function save (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const current = editing || defaults
		const payload = packageFromForm(new FormData(event.currentTarget), current)
		const isEdit = Boolean(editing?.id)
		try {
			await api(isEdit ? `/api/v1/packages/${editing?.id}` : '/api/v1/packages', { method: isEdit ? 'PATCH' : 'POST', body: JSON.stringify(payload) })
			setMessage(`${payload.name} ${isEdit ? 'updated' : 'created'}.`)
			setEditing(null)
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	async function remove () {
		if (!deleting || confirmation !== deleting.name) return
		try {
			await api(`/api/v1/packages/${deleting.id}`, { method: 'DELETE' })
			setMessage(`${deleting.name} deleted.`); setDeleting(null); setConfirmation(''); load()
		} catch (requestError) { setMessage(`${messageFrom(requestError)} Assigned packages must be moved to another package before deletion.`) }
	}

	return (
		<>
			<PageHeader title="Packages" description="Define reusable resource, service, and retention limits for accounts." actions={canWrite ? <button type="button" onClick={() => setEditing({ ...defaults })}>Add package</button> : undefined} />
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading packages…" /> : <div className="table-wrap"><table className="dense-table"><thead><tr><th>Name</th><th>CPU</th><th>Memory</th><th>Disk</th><th>Bandwidth</th><th>Domains</th><th>Mailboxes</th><th>Actions</th></tr></thead><tbody>
				{items.map((pkg) => <tr key={pkg.id}><td><strong>{pkg.name}</strong><small>{pkg.reseller_id ? 'Reseller package' : 'Global package'}</small></td><td>{pkg.cpu_percent}%</td><td>{formatBytes(pkg.memory_bytes)}</td><td>{formatBytes(pkg.disk_bytes)}</td><td>{formatBytes(pkg.bandwidth_bytes_monthly)}</td><td>{pkg.domains}</td><td>{pkg.mailboxes}</td><td>{canWrite ? <div className="row-actions"><button type="button" className="link-button" onClick={() => setEditing(pkg)}>Edit</button><button type="button" className="link-button danger-text" onClick={() => setDeleting(pkg)}>Delete</button></div> : 'View only'}</td></tr>)}
			</tbody></table></div>}
			{!loading && !items.length ? <EmptyState title="No packages" detail="Create a package to define account capacity." /> : null}
			<Dialog open={Boolean(editing)} title={editing?.id ? `Edit ${editing.name}` : 'Add package'} onClose={() => setEditing(null)}>
				{editing ? <PackageForm value={editing} onSubmit={save} onCancel={() => setEditing(null)} /> : null}
			</Dialog>
			<Dialog open={Boolean(deleting)} title={`Delete ${deleting?.name || 'package'}`} onClose={() => { setDeleting(null); setConfirmation('') }}>
				<p>Deletion fails safely when accounts still use this package. Enter <strong>{deleting?.name}</strong> to confirm.</p>
				<label>Package name<input autoFocus value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setDeleting(null)}>Cancel</button><button type="button" className="danger" disabled={confirmation !== deleting?.name} onClick={remove}>Delete package</button></footer>
			</Dialog>
		</>
	)
}

function PackageForm ({ value, onSubmit, onCancel }: { value: Package; onSubmit: (event: React.FormEvent<HTMLFormElement>) => void; onCancel: () => void }) {
	return <form onSubmit={onSubmit}><div className="form-grid"><label>Package name<input name="name" defaultValue={value.name} required autoFocus /></label><label>Reseller ID<input name="reseller_id" defaultValue={value.reseller_id} placeholder="Optional" /></label><label>Feature set ID<input name="feature_set_id" defaultValue={value.feature_set_id} placeholder="Optional" /></label>{numericPackageFields.map(({ field, label }) => <label key={field}>{label}<input name={field} type="number" min={0} defaultValue={value[field]} required /></label>)}</div><footer className="dialog-form-actions"><button type="button" className="secondary" onClick={onCancel}>Cancel</button><button type="submit">Save package</button></footer></form>
}

type NumericPackageField = Exclude<keyof Package, 'id' | 'reseller_id' | 'name' | 'feature_set_id'>

interface NumericPackageFieldDefinition {
	field: NumericPackageField
	label: string
}
