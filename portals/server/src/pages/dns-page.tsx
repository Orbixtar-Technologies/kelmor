import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountScopeBar } from '../components/account-scope-bar'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'
import { describeDnsChange, dnssecStateLabel, validateDnsRecord, zoneSyncLabel } from './dns-copy'

interface DnsDraft extends ResourceItem {
	_mode: 'edit' | 'duplicate'
}

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
	const [updatedAt, setUpdatedAt] = useState('')
	const [draft, setDraft] = useState<DnsDraft | null>(null)
	const [draftError, setDraftError] = useState('')
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
			setUpdatedAt(new Date().toISOString())
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

	async function submitRecord (payload: { name: string; type: string; content: string; ttl: number; priority?: number }, replaceId?: string) {
		const requestedAccountId = accountId
		const requestedZoneId = selectedZone
		const invalid = validateDnsRecord(payload.type, payload.content, payload.priority)
		if (invalid) {
			setDraftError(invalid)
			return
		}
		try {
			if (replaceId) await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records/${replaceId}`, { method: 'DELETE' })
			await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records`, { method: 'POST', body: JSON.stringify(payload) })
			if (currentAccountId.current !== requestedAccountId || currentZoneId.current !== requestedZoneId) return
			setMessage(replaceId ? 'DNS record replacement queued. Use Jobs if you need to inspect or retry the change.' : 'DNS record queued.')
			setDraft(null)
			setDraftError('')
			loadRecords(requestedAccountId, requestedZoneId)
		} catch (requestError) {
			if (currentAccountId.current === requestedAccountId && currentZoneId.current === requestedZoneId) setMessage(messageFrom(requestError))
		}
	}

	async function addRecord (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const form = event.currentTarget
		const data = new FormData(form)
		const payload = { name: String(data.get('name')), type: String(data.get('type')), content: String(data.get('content')), ttl: Number(data.get('ttl')), priority: data.get('priority') ? Number(data.get('priority')) : undefined }
		const invalid = validateDnsRecord(payload.type, payload.content, payload.priority)
		if (invalid) {
			setMessage(invalid)
			return
		}
		await submitRecord(payload)
		form.reset()
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
	const account = accounts.find((entry) => entry.id === accountId)
	return (
		<>
			<PageHeader title={account ? `DNS Management · ${account.username}` : 'DNS Management'} description="Select an account, inspect its zones, and safely manage records and DNSSEC." />
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="DNS" onChange={(next) => setParams({ account: next })} />
			<div className="filter-bar">
				<label>Zone<select value={selectedZone} onChange={(event) => { requests.invalidate('records'); setRecords([]); setRecordsContext(''); setSelectedZone(event.target.value) }}>{visibleZones.map((entry) => <option key={entry.id} value={entry.id}>{String(entry.name)}</option>)}</select></label>
			</div>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}. Queued record changes appear in <Link to={`/jobs?account=${accountId}`}>Jobs</Link>.</p> : null}
			{message ? <p className="feedback">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadZones(accountId, selectedZone)} /> : null}
			{loading ? <LoadingState label="Loading DNS zones…" /> : null}
			{zone ? <section className="panel"><div className="section-heading"><div><h2>{String(zone.name)}</h2><p>Provider {String(zone.provider || 'local')} · {zoneSyncLabel(zone)} · revision {String(zone.observed_revision || 0)} / {String(zone.desired_revision || 0)}</p></div><div className="button-row"><StatusBadge value={dnssecStateLabel(zone.dnssec_enabled)} />{canWrite ? <button type="button" className="secondary" onClick={toggleDNSSEC}>{zone.dnssec_enabled ? 'Disable DNSSEC' : 'Enable DNSSEC'}</button> : null}</div></div>
				{canWrite ? <form className="inline-form" onSubmit={addRecord}><label>Name<input name="name" placeholder="www" required /></label><label>Type<select name="type"><option>A</option><option>AAAA</option><option>CNAME</option><option>MX</option><option>TXT</option><option>CAA</option><option>SRV</option><option>NS</option></select></label><label>Content<input name="content" required /></label><label>TTL<input name="ttl" type="number" min={60} defaultValue={300} required /></label><label>Priority<input name="priority" type="number" min={0} /></label><button type="submit">Add record</button></form> : <p className="subtle">Your role can inspect records but cannot modify this zone.</p>}
				<div className="table-wrap"><table className="dense-table"><thead><tr><th>Name</th><th>Type</th><th>Content</th><th>TTL</th><th>Priority</th><th>Actions</th></tr></thead><tbody>{visibleRecords.map((record) => <tr key={record.id}><td>{String(record.name)}</td><td><strong>{String(record.type)}</strong></td><td><code>{String(record.content)}</code></td><td>{String(record.ttl)}</td><td>{record.priority === undefined ? '—' : String(record.priority)}</td><td><div className="row-actions">{canWrite ? <><button type="button" className="link-button" onClick={() => { setDraft({ ...record, _mode: 'edit' }); setDraftError('') }}>Edit</button><button type="button" className="link-button" onClick={() => { setDraft({ ...record, id: '', _mode: 'duplicate' }); setDraftError('') }}>Duplicate</button><button type="button" className="link-button danger-text" onClick={() => deleteRecord(record.id)}>Delete</button></> : 'View only'}</div></td></tr>)}</tbody></table></div>
				{!visibleRecords.length ? <EmptyState title="No records in this zone" detail="Use the record form above to create the first entry." /> : null}
			</section> : !loading ? <EmptyState title="No managed DNS zone" detail="Provision the account’s primary domain to create a zone." /> : null}
			<Dialog open={Boolean(draft)} title={draft?._mode === 'edit' ? 'Replace DNS record' : 'Duplicate DNS record'} onClose={() => setDraft(null)} actions={<>
				<button type="button" className="secondary" onClick={() => setDraft(null)}>Cancel</button>
				<button type="button" onClick={() => {
					if (!draft) return
					void submitRecord({
						name: String(draft.name),
						type: String(draft.type),
						content: String(draft.content),
						ttl: Number(draft.ttl || 300),
						priority: draft.priority === undefined || draft.priority === '' ? undefined : Number(draft.priority),
					}, draft._mode === 'edit' ? String(draft.id) : undefined)
				}}>{draft?._mode === 'edit' ? 'Queue replacement' : 'Queue duplicate'}</button>
			</>}>
				{draft ? <>
					<p>{describeDnsChange(draft._mode === 'edit' ? 'replace' : 'add', { name: String(draft.name), type: String(draft.type), content: String(draft.content) })}</p>
					<label>Name<input value={String(draft.name || '')} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
					<label>Type<select value={String(draft.type || 'A')} onChange={(event) => setDraft({ ...draft, type: event.target.value })}><option>A</option><option>AAAA</option><option>CNAME</option><option>MX</option><option>TXT</option><option>CAA</option><option>SRV</option><option>NS</option></select></label>
					<label>Content<input value={String(draft.content || '')} onChange={(event) => setDraft({ ...draft, content: event.target.value })} /></label>
					<label>TTL<input type="number" min={60} value={Number(draft.ttl || 300)} onChange={(event) => setDraft({ ...draft, ttl: Number(event.target.value) })} /></label>
					<label>Priority<input type="number" min={0} value={draft.priority === undefined || draft.priority === '' ? '' : Number(draft.priority)} onChange={(event) => setDraft({ ...draft, priority: event.target.value === '' ? undefined : Number(event.target.value) })} /></label>
					{draftError ? <p className="field-error">{draftError}</p> : null}
					<p className="subtle">There is no in-page undo. Re-create the previous record or inspect the queued jobs if this change needs reversal.</p>
				</> : null}
			</Dialog>
		</>
	)
}
