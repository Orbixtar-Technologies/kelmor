import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

export function DNSPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [zones, setZones] = useState<ResourceItem[]>([])
	const [zonesAccountId, setZonesAccountId] = useState('')
	const [records, setRecords] = useState<ResourceItem[]>([])
	const [recordsContext, setRecordsContext] = useState('')
	const [selectedZone, setSelectedZone] = useState('')
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [loading, setLoading] = useState(true)
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('dns.write')
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	const currentZoneId = useRef(selectedZone)
	currentAccountId.current = accountId
	currentZoneId.current = selectedZone

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!currentAccountId.current && next[0]) setParams({ account: next[0].id }, { replace: true })
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests, setParams])

	const loadZones = useCallback((requestedAccountId: string, preferredZoneId = '') => {
		if (!requestedAccountId) return
		const request = requests.begin('zones')
		setLoading(true)
		setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			const next = asList(result)
			setZones(next)
			setZonesAccountId(requestedAccountId)
			setSelectedZone((current) => {
				if (current && next.some((zone) => zone.id === current)) return current
				if (preferredZoneId && next.some((zone) => zone.id === preferredZoneId)) return preferredZoneId
				return next[0]?.id || ''
			})
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('zones')
		requests.invalidate('records')
		setZones([])
		setZonesAccountId('')
		setRecords([])
		setRecordsContext('')
		setSelectedZone('')
		setError('')
		setMessage('')
		if (accountId) loadZones(accountId)
		else setLoading(false)
	}, [accountId, loadZones, requests])

	const loadRecords = useCallback((requestedAccountId: string, requestedZoneId: string) => {
		if (!requestedAccountId || !requestedZoneId) return
		const request = requests.begin('records')
		const context = `${requestedAccountId}:${requestedZoneId}`
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId || currentZoneId.current !== requestedZoneId) return
			setRecords(asList(result))
			setRecordsContext(context)
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) setError(messageFrom(requestError))
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('records')
		setRecords([])
		setRecordsContext('')
		setError('')
		if (accountId && selectedZone) loadRecords(accountId, selectedZone)
	}, [accountId, loadRecords, requests, selectedZone])

	const visibleZones = zonesAccountId === accountId ? zones : []
	const context = `${accountId}:${selectedZone}`
	const visibleRecords = recordsContext === context ? records : []
	const zone = visibleZones.find((entry) => entry.id === selectedZone)

	async function addRecord (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const form = event.currentTarget
		const requestedAccountId = accountId
		const requestedZoneId = selectedZone
		const data = new FormData(form)
		try {
			await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records`, { method: 'POST', body: JSON.stringify({ name: data.get('name'), type: data.get('type'), content: data.get('content'), ttl: Number(data.get('ttl')), priority: data.get('priority') ? Number(data.get('priority')) : undefined }) })
			if (currentAccountId.current !== requestedAccountId || currentZoneId.current !== requestedZoneId) return
			setMessage('DNS record queued.')
			loadRecords(requestedAccountId, requestedZoneId)
			form.reset()
		} catch (requestError) {
			if (currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) setMessage(messageFrom(requestError))
		}
	}

	async function toggleDNSSEC () {
		if (!zone) return
		const requestedAccountId = accountId
		const requestedZoneId = selectedZone
		const wasEnabled = Boolean(zone.dnssec_enabled)
		try {
			await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/dnssec`, { method: 'POST', body: JSON.stringify({ enabled: !wasEnabled }) })
			if (currentAccountId.current !== requestedAccountId || currentZoneId.current !== requestedZoneId) return
			setMessage(`DNSSEC ${wasEnabled ? 'disable' : 'enable'} queued.`)
			loadZones(requestedAccountId, requestedZoneId)
		} catch (requestError) {
			if (currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) setMessage(messageFrom(requestError))
		}
	}

	async function deleteRecord (recordId: string) {
		if (!window.confirm('Delete this DNS record?')) return
		const requestedAccountId = accountId
		const requestedZoneId = selectedZone
		try {
			await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records/${recordId}`, { method: 'DELETE' })
			if (currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) loadRecords(requestedAccountId, requestedZoneId)
		} catch (requestError) {
			if (currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) setMessage(messageFrom(requestError))
		}
	}
	return (
		<>
			<PageHeader title="DNS Management" description="Select an account, inspect its zones, and safely manage records and DNSSEC." />
			<div className="filter-bar">
				<label>Account<select value={accountId} onChange={(event) => setParams({ account: event.target.value })}>{accounts.map((account) => <option key={account.id} value={account.id}>{account.username} — {account.primary_domain}</option>)}</select></label>
				<label>Zone<select value={selectedZone} onChange={(event) => { requests.invalidate('records'); setRecords([]); setRecordsContext(''); setSelectedZone(event.target.value) }}>{visibleZones.map((entry) => <option key={entry.id} value={entry.id}>{String(entry.name)}</option>)}</select></label>
			</div>
			{message ? <p className="feedback">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadZones(accountId, selectedZone)} /> : null}
			{loading ? <LoadingState label="Loading DNS zones…" /> : null}
			{zone ? <section className="panel"><div className="section-heading"><div><h2>{String(zone.name)}</h2><p>Provider {String(zone.provider || 'local')} · revision {String(zone.observed_revision || 0)} / {String(zone.desired_revision || 0)}</p></div><div className="button-row"><StatusBadge value={Boolean(zone.dnssec_enabled)} />{canWrite ? <button type="button" className="secondary" onClick={toggleDNSSEC}>{zone.dnssec_enabled ? 'Disable DNSSEC' : 'Enable DNSSEC'}</button> : null}</div></div>
				{canWrite ? <form className="inline-form" onSubmit={addRecord}><label>Name<input name="name" placeholder="www" required /></label><label>Type<select name="type"><option>A</option><option>AAAA</option><option>CNAME</option><option>MX</option><option>TXT</option><option>CAA</option><option>SRV</option><option>NS</option></select></label><label>Content<input name="content" required /></label><label>TTL<input name="ttl" type="number" min={60} defaultValue={300} required /></label><label>Priority<input name="priority" type="number" min={0} /></label><button type="submit">Add record</button></form> : <p className="subtle">Your role can inspect records but cannot modify this zone.</p>}
				<div className="table-wrap"><table className="dense-table"><thead><tr><th>Name</th><th>Type</th><th>Content</th><th>TTL</th><th>Priority</th><th>Actions</th></tr></thead><tbody>{visibleRecords.map((record) => <tr key={record.id}><td>{String(record.name)}</td><td><strong>{String(record.type)}</strong></td><td><code>{String(record.content)}</code></td><td>{String(record.ttl)}</td><td>{record.priority === undefined ? '—' : String(record.priority)}</td><td>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => deleteRecord(record.id)}>Delete</button> : 'View only'}</td></tr>)}</tbody></table></div>
				{!visibleRecords.length ? <EmptyState title="No records in this zone" detail="Use the record form above to create the first entry." /> : null}
			</section> : !loading ? <EmptyState title="No managed DNS zone" detail="Provision the account’s primary domain to create a zone." /> : null}
		</>
	)
}
