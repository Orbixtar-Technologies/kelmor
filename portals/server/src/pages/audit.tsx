import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, query } from '../client'
import { useLoad } from '../lib/hooks'
import { Drawer, EmptyState, KeyValues, Loading, Notice, PageHeader, Panel, Pill } from '../components/ui'
import { Icon } from '../components/icons'
import { formatDateTime, formatRelative, shortId } from '../lib/format'
import type { AccountRow } from '../components/account-picker'
import { listOf } from '../client'

interface AuditEvent {
	id: string
	occurred_at: string
	actor_type: string
	actor_id?: string
	effective_actor_id?: string
	account_id?: string
	action: string
	resource_type?: string
	resource_id?: string
	source_ip?: string
	user_agent?: string
	request_id: string
	success: boolean
	before_state?: Record<string, unknown>
	after_state?: Record<string, unknown>
	metadata?: Record<string, unknown>
}

interface AuditResponse {
	items: AuditEvent[] | null
	total: number
	offset: number
	limit: number
}

interface Filters {
	q: string
	action: string
	resourceType: string
	success: string
	since: string
	until: string
}

const emptyFilters: Filters = { q: '', action: '', resourceType: '', success: '', since: '', until: '' }

const resourceTypes = ['account', 'package', 'reseller', 'server', 'job', 'domain', 'website', 'database', 'mailbox', 'certificate', 'backup']

export function AuditLog () {
	const [filters, setFilters] = useState<Filters>(emptyFilters)
	const [applied, setApplied] = useState<Filters>(emptyFilters)
	const [page, setPage] = useState(0)
	const [selected, setSelected] = useState<AuditEvent | null>(null)
	const limit = 50

	const url = `/api/v1/audit-events${query({
		q: applied.q,
		action: applied.action,
		resource_type: applied.resourceType,
		success: applied.success,
		since: applied.since ? new Date(applied.since).toISOString() : '',
		until: applied.until ? new Date(applied.until).toISOString() : '',
		limit,
		offset: page * limit,
	})}`

	const events = useLoad<AuditResponse>(() => api<AuditResponse>(url), [url])
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts').catch(() => []), [])

	const rows = events.data?.items ?? []
	const total = events.data?.total ?? 0
	const pageCount = Math.max(1, Math.ceil(total / limit))
	const accountFor = (id?: string) => (accounts.data ?? []).find((a) => a.id === id)
	const filtersActive = JSON.stringify(applied) !== JSON.stringify(emptyFilters)

	function apply () {
		setPage(0)
		setApplied(filters)
	}

	function reset () {
		setFilters(emptyFilters)
		setApplied(emptyFilters)
		setPage(0)
	}

	return (
		<>
			<PageHeader
				title="Audit Log"
				description="Every privileged action Kelmor performed, who performed it and whether it succeeded. Secrets are redacted and an impersonated action keeps the original actor."
				favoritePath="/security/audit"
				actions={<button type="button" className="btn secondary" onClick={() => events.reload()}>Refresh</button>}
			/>

			<Panel title="Filters" icon="filter">
				<div className="form-grid">
					<div className="field">
						<label htmlFor="audit-q">Search</label>
						<input
							id="audit-q"
							type="search"
							placeholder="Action, resource, IP or request ID"
							value={filters.q}
							onChange={(e) => setFilters({ ...filters, q: e.target.value })}
							onKeyDown={(e) => e.key === 'Enter' && apply()}
						/>
					</div>
					<div className="field">
						<label htmlFor="audit-action">Action starts with</label>
						<input
							id="audit-action"
							placeholder="account.suspend"
							value={filters.action}
							onChange={(e) => setFilters({ ...filters, action: e.target.value })}
							onKeyDown={(e) => e.key === 'Enter' && apply()}
						/>
					</div>
					<div className="field">
						<label htmlFor="audit-resource">Resource type</label>
						<select id="audit-resource" value={filters.resourceType} onChange={(e) => setFilters({ ...filters, resourceType: e.target.value })}>
							<option value="">Any resource</option>
							{resourceTypes.map((type) => <option key={type} value={type}>{type}</option>)}
						</select>
					</div>
					<div className="field">
						<label htmlFor="audit-success">Outcome</label>
						<select id="audit-success" value={filters.success} onChange={(e) => setFilters({ ...filters, success: e.target.value })}>
							<option value="">Any outcome</option>
							<option value="true">Succeeded</option>
							<option value="false">Failed</option>
						</select>
					</div>
					<div className="field">
						<label htmlFor="audit-since">From</label>
						<input id="audit-since" type="datetime-local" value={filters.since} onChange={(e) => setFilters({ ...filters, since: e.target.value })} />
					</div>
					<div className="field">
						<label htmlFor="audit-until">To</label>
						<input id="audit-until" type="datetime-local" value={filters.until} onChange={(e) => setFilters({ ...filters, until: e.target.value })} />
					</div>
				</div>
				<div className="form-actions">
					<button type="button" className="btn" onClick={apply}>Apply filters</button>
					<button type="button" className="btn secondary" onClick={reset}>Reset</button>
					<span className="small muted">{total} matching event{total === 1 ? '' : 's'}</span>
				</div>
			</Panel>

			{filtersActive && total === 0 && !events.loading ? (
				<Notice tone="info">No audit event matches these filters. Widen the date range or clear the search box.</Notice>
			) : null}

			<Panel title="Events" icon="fileText" subtitle={`page ${page + 1} of ${pageCount}`} tight>
				{events.loading && rows.length === 0 ? <Loading label="Reading the audit trail" /> : events.error ? (
					<EmptyState icon="alertCircle" title="The audit trail could not be read">{events.error}</EmptyState>
				) : rows.length === 0 ? (
					<EmptyState icon="fileText" title="No audit events yet">
						Every privileged write is recorded here as soon as it happens — account changes, package edits, firewall
						applies, impersonation and reboots.
					</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead>
								<tr>
									<th>When</th>
									<th>Action</th>
									<th>Resource</th>
									<th>Account</th>
									<th>Source IP</th>
									<th>Outcome</th>
									<th />
								</tr>
							</thead>
							<tbody>
								{rows.map((event) => {
									const account = accountFor(event.account_id)
									return (
										<tr key={event.id}>
											<td title={formatDateTime(event.occurred_at)}>{formatRelative(event.occurred_at)}</td>
											<td className="mono">{event.action}</td>
											<td>
												{event.resource_type ? (
													<>
														<Pill tone="idle">{event.resource_type}</Pill>{' '}
														<span className="mono small muted">{shortId(event.resource_id)}</span>
													</>
												) : (
													<span className="muted">—</span>
												)}
											</td>
											<td>{account ? <Link to={`/accounts/${account.id}`}>{account.username}</Link> : <span className="muted">—</span>}</td>
											<td className="mono small">{event.source_ip || '—'}</td>
											<td>{event.success ? <Pill tone="ok">success</Pill> : <Pill tone="bad">failed</Pill>}</td>
											<td className="right">
												<button type="button" className="linkish" onClick={() => setSelected(event)}>Detail</button>
											</td>
										</tr>
									)
								})}
							</tbody>
						</table>
					</div>
				)}
				{total > 0 ? (
					<div className="table-footer">
						<span>Showing {page * limit + 1}–{Math.min(total, page * limit + rows.length)} of {total} events</span>
						<div className="pager">
							<button type="button" className="btn secondary small" disabled={page === 0} onClick={() => setPage(page - 1)}>
								<Icon name="chevronLeft" size={12} /> Newer
							</button>
							<span className="small">Page {page + 1} of {pageCount}</span>
							<button type="button" className="btn secondary small" disabled={page >= pageCount - 1} onClick={() => setPage(page + 1)}>
								Older <Icon name="chevronRight" size={12} />
							</button>
						</div>
					</div>
				) : null}
			</Panel>

			{selected ? (
				<Drawer title={<span className="mono">{selected.action}</span>} onClose={() => setSelected(null)}>
					<KeyValues
						rows={[
							['Occurred', formatDateTime(selected.occurred_at)],
							['Outcome', selected.success ? <Pill key="o" tone="ok">success</Pill> : <Pill key="o" tone="bad">failed</Pill>],
							['Actor type', selected.actor_type],
							['Actor', <span key="a" className="mono small">{selected.actor_id || '—'}</span>],
							['Effective actor', <span key="e" className="mono small">{selected.effective_actor_id || 'same as actor'}</span>],
							['Resource', selected.resource_type ? `${selected.resource_type} ${shortId(selected.resource_id, 12)}` : '—'],
							['Account', <span key="ac" className="mono small">{selected.account_id || '—'}</span>],
							['Source IP', <span key="ip" className="mono">{selected.source_ip || '—'}</span>],
							['User agent', <span key="ua" className="small">{selected.user_agent || '—'}</span>],
							['Request ID', <span key="r" className="mono small">{selected.request_id}</span>],
						]}
					/>
					{selected.effective_actor_id && selected.effective_actor_id !== selected.actor_id ? (
						<div style={{ marginTop: 14 }}>
							<Notice tone="warn">
								This action was taken while impersonating another user. The audit trail keeps the original operator as
								the actor so accountability survives the impersonation.
							</Notice>
						</div>
					) : null}
					<h3 style={{ margin: '18px 0 8px', fontSize: 13 }}>Before</h3>
					<pre className="code">{JSON.stringify(selected.before_state ?? {}, null, 2)}</pre>
					<h3 style={{ margin: '18px 0 8px', fontSize: 13 }}>After</h3>
					<pre className="code">{JSON.stringify(selected.after_state ?? {}, null, 2)}</pre>
					{selected.metadata && Object.keys(selected.metadata).length > 0 ? (
						<>
							<h3 style={{ margin: '18px 0 8px', fontSize: 13 }}>Metadata</h3>
							<pre className="code">{JSON.stringify(selected.metadata, null, 2)}</pre>
						</>
					) : null}
				</Drawer>
			) : null}
		</>
	)
}
