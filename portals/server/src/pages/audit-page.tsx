import { useEffect, useMemo, useState } from 'react'
import { api, asList } from '../client'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader, Pagination, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { paginateRows, sortRows } from '../table-helpers'
import type { AuditEvent } from '../types'

export function AuditPage () {
	const [items, setItems] = useState<AuditEvent[]>([])
	const [query, setQuery] = useState('')
	const [action, setAction] = useState('')
	const [success, setSuccess] = useState('')
	const [from, setFrom] = useState('')
	const [to, setTo] = useState('')
	const [page, setPage] = useState(1)
	const [selected, setSelected] = useState<AuditEvent | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	function load () { setLoading(true); setError(''); api<{ items: AuditEvent[] }>('/api/v1/audit-events').then((result) => setItems(asList(result))).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false)) }
	useEffect(load, [])
	const actions = [...new Set(items.map((event) => event.action))].sort()
	const visible = useMemo(() => items.filter((event) => {
		const haystack = `${event.action} ${event.actor_id} ${event.resource_type} ${event.resource_id} ${event.source_ip} ${event.request_id}`.toLocaleLowerCase()
		if (query && !haystack.includes(query.toLocaleLowerCase())) return false
		if (action && event.action !== action) return false
		if (success && String(event.success) !== success) return false
		const stamp = new Date(event.occurred_at).valueOf()
		if (from && stamp < new Date(`${from}T00:00:00`).valueOf()) return false
		if (to && stamp > new Date(`${to}T23:59:59`).valueOf()) return false
		return true
	}), [items, query, action, success, from, to])
	const paged = paginateRows(sortRows(visible, 'occurred_at', 'desc'), page, 25)
	return <>
		<PageHeader title="Audit Trail" description="Search privileged activity with actor, request, outcome, and state-change context." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		<div className="filter-bar audit-filters"><label>Search<input type="search" value={query} placeholder="Actor, resource, IP, or request ID" onChange={(event) => { setQuery(event.target.value); setPage(1) }} /></label><label>Action<select value={action} onChange={(event) => { setAction(event.target.value); setPage(1) }}><option value="">All actions</option>{actions.map((value) => <option key={value}>{value}</option>)}</select></label><label>Outcome<select value={success} onChange={(event) => setSuccess(event.target.value)}><option value="">All</option><option value="true">Success</option><option value="false">Rejected / failed</option></select></label><label>From<input type="date" value={from} onChange={(event) => setFrom(event.target.value)} /></label><label>To<input type="date" value={to} onChange={(event) => setTo(event.target.value)} /></label></div>
		{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Loading audit events…" /> : null}
		{!loading && <div className="table-wrap"><table className="dense-table"><thead><tr><th>When</th><th>Action</th><th>Actor</th><th>Resource</th><th>Outcome</th><th>Source IP</th><th>Details</th></tr></thead><tbody>{paged.items.map((event) => <tr key={event.id}><td>{formatDate(event.occurred_at)}</td><td><strong>{event.action}</strong></td><td>{event.actor_id || event.actor_type}</td><td>{event.resource_type || '—'}<small>{event.resource_id || '—'}</small></td><td><StatusBadge value={event.success} /></td><td>{event.source_ip || '—'}</td><td><button type="button" className="link-button" onClick={() => setSelected(event)}>Inspect</button></td></tr>)}</tbody></table></div>}
		{!loading && !paged.items.length ? <EmptyState title="No audit events match" detail="Filters remain available above. Broaden the date range or outcome selection." /> : null}
		<Pagination page={paged.page} pageCount={paged.pageCount} total={paged.total} onPage={setPage} />
		<Dialog open={Boolean(selected)} title={selected ? selected.action : 'Audit details'} onClose={() => setSelected(null)}>{selected ? <><dl className="detail-list"><div><dt>Request ID</dt><dd><code>{selected.request_id}</code></dd></div><div><dt>Actor</dt><dd>{selected.actor_id || selected.actor_type}</dd></div><div><dt>Effective actor</dt><dd>{selected.effective_actor_id || 'Same as actor'}</dd></div><div><dt>Account</dt><dd>{selected.account_id || '—'}</dd></div></dl><div className="state-columns"><div><h3>Before</h3><pre>{JSON.stringify(selected.before_state || {}, null, 2)}</pre></div><div><h3>After</h3><pre>{JSON.stringify(selected.after_state || {}, null, 2)}</pre></div></div><h3>Metadata</h3><pre>{JSON.stringify(selected.metadata || {}, null, 2)}</pre></> : null}</Dialog>
	</>
}
