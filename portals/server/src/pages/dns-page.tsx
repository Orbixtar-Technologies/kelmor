import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

export function DNSPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [zones, setZones] = useState<ResourceItem[]>([])
	const [records, setRecords] = useState<ResourceItem[]>([])
	const [selectedZone, setSelectedZone] = useState('')
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [loading, setLoading] = useState(true)
	const canWrite = useCan('dns.write')
	const accountId = params.get('account') || ''

	useEffect(() => {
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			const next = asList(result)
			setAccounts(next)
			if (!accountId && next[0]) setParams({ account: next[0].id }, { replace: true })
		}).catch((requestError) => setError(messageFrom(requestError)))
	}, [setParams])
	const loadZones = useCallback(() => {
		if (!accountId) return
		setLoading(true); setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${accountId}/dns/zones`).then((result) => {
			const next = asList(result); setZones(next)
			setSelectedZone((current) => current && next.some((zone) => zone.id === current) ? current : next[0]?.id || '')
		}).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
	}, [accountId])
	useEffect(loadZones, [loadZones])
	const loadRecords = useCallback(() => {
		if (!accountId || !selectedZone) { setRecords([]); return }
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${accountId}/dns/zones/${selectedZone}/records`).then((result) => setRecords(asList(result))).catch((requestError) => setError(messageFrom(requestError)))
	}, [accountId, selectedZone])
	useEffect(loadRecords, [loadRecords])
	const zone = zones.find((entry) => entry.id === selectedZone)

	async function addRecord (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/dns/zones/${selectedZone}/records`, { method: 'POST', body: JSON.stringify({ name: data.get('name'), type: data.get('type'), content: data.get('content'), ttl: Number(data.get('ttl')), priority: data.get('priority') ? Number(data.get('priority')) : undefined }) })
			setMessage('DNS record queued.'); loadRecords(); event.currentTarget.reset()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	return (
		<>
			<PageHeader title="DNS Management" description="Select an account, inspect its zones, and safely manage records and DNSSEC." />
			<div className="filter-bar">
				<label>Account<select value={accountId} onChange={(event) => setParams({ account: event.target.value })}>{accounts.map((account) => <option key={account.id} value={account.id}>{account.username} — {account.primary_domain}</option>)}</select></label>
				<label>Zone<select value={selectedZone} onChange={(event) => setSelectedZone(event.target.value)}>{zones.map((entry) => <option key={entry.id} value={entry.id}>{String(entry.name)}</option>)}</select></label>
			</div>
			{message ? <p className="feedback">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={loadZones} /> : null}
			{loading ? <LoadingState label="Loading DNS zones…" /> : null}
			{zone ? <section className="panel"><div className="section-heading"><div><h2>{String(zone.name)}</h2><p>Provider {String(zone.provider || 'local')} · revision {String(zone.observed_revision || 0)} / {String(zone.desired_revision || 0)}</p></div><div className="button-row"><StatusBadge value={Boolean(zone.dnssec_enabled)} />{canWrite ? <button type="button" className="secondary" onClick={async () => { try { await api(`/api/v1/accounts/${accountId}/dns/zones/${selectedZone}/dnssec`, { method: 'POST', body: JSON.stringify({ enabled: !zone.dnssec_enabled }) }); setMessage(`DNSSEC ${zone.dnssec_enabled ? 'disable' : 'enable'} queued.`); loadZones() } catch (requestError) { setMessage(messageFrom(requestError)) } }}>{zone.dnssec_enabled ? 'Disable DNSSEC' : 'Enable DNSSEC'}</button> : null}</div></div>
				{canWrite ? <form className="inline-form" onSubmit={addRecord}><label>Name<input name="name" placeholder="www" required /></label><label>Type<select name="type"><option>A</option><option>AAAA</option><option>CNAME</option><option>MX</option><option>TXT</option><option>CAA</option><option>SRV</option><option>NS</option></select></label><label>Content<input name="content" required /></label><label>TTL<input name="ttl" type="number" min={60} defaultValue={300} required /></label><label>Priority<input name="priority" type="number" min={0} /></label><button type="submit">Add record</button></form> : <p className="subtle">Your role can inspect records but cannot modify this zone.</p>}
				<div className="table-wrap"><table className="dense-table"><thead><tr><th>Name</th><th>Type</th><th>Content</th><th>TTL</th><th>Priority</th><th>Actions</th></tr></thead><tbody>{records.map((record) => <tr key={record.id}><td>{String(record.name)}</td><td><strong>{String(record.type)}</strong></td><td><code>{String(record.content)}</code></td><td>{String(record.ttl)}</td><td>{record.priority === undefined ? '—' : String(record.priority)}</td><td>{canWrite ? <button type="button" className="link-button danger-text" onClick={async () => { if (!window.confirm('Delete this DNS record?')) return; try { await api(`/api/v1/accounts/${accountId}/dns/zones/${selectedZone}/records/${record.id}`, { method: 'DELETE' }); loadRecords() } catch (requestError) { setMessage(messageFrom(requestError)) } }}>Delete</button> : 'View only'}</td></tr>)}</tbody></table></div>
				{!records.length ? <EmptyState title="No records in this zone" detail="Use the record form above to create the first entry." /> : null}
			</section> : !loading ? <EmptyState title="No managed DNS zone" detail="Provision the account’s primary domain to create a zone." /> : null}
		</>
	)
}
