import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { Dialog, EmptyState, ErrorState, LoadingState, Metric, PageHeader, Pagination, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCapabilities } from '../rbac'
import { filterRows, paginateRows, sortRows } from '../table-helpers'
import type { Job } from '../types'

export function JobsPage () {
	const capabilities = useCapabilities()
	const [params] = useSearchParams()
	const [items, setItems] = useState<Job[]>([])
	const [query, setQuery] = useState('')
	const [state, setState] = useState('')
	const [page, setPage] = useState(1)
	const [selected, setSelected] = useState<Job | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const load = useCallback(() => {
		setLoading(true); setError('')
		api<{ items: Job[] }>(`/api/v1/jobs${state ? `?state=${encodeURIComponent(state)}` : ''}`).then((result) => {
			const next = asList(result); setItems(next)
			const selectedId = params.get('selected')
			if (selectedId) setSelected(next.find((job) => job.id === selectedId) || null)
		}).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
	}, [params, state])
	useEffect(load, [load])
	const accountId = params.get('account')
	const visible = useMemo(() => filterRows(items.filter((job) => !accountId || job.resource_id === accountId || job.payload.account_id === accountId), query, ['id', 'type', 'state', 'resource_type', 'resource_id', 'last_error']), [items, accountId, query])
	const paged = paginateRows(sortRows(visible, 'created_at', 'desc'), page, 20)
	async function retry (job: Job) {
		try {
			const result = await api<{ operation_id?: string; id?: string }>(`/api/v1/jobs/${job.id}/retry`, { method: 'POST', body: '{}' })
			setMessage(`Retry queued as ${result.operation_id || result.id || 'a new job'}.`); setSelected(null); load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	return <>
		<PageHeader title="Jobs" description="Search durable background work, inspect safe payloads and logs, and retry eligible failures." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		<section className="metric-grid compact-metrics"><Metric label="Queued" value={items.filter((job) => job.state === 'queued').length} /><Metric label="Running" value={items.filter((job) => job.state === 'running').length} /><Metric label="Succeeded" value={items.filter((job) => job.state === 'succeeded').length} /><Metric label="Failed" value={items.filter((job) => job.state === 'failed').length} /></section>
		<div className="filter-bar"><label>Search jobs<input type="search" value={query} placeholder="Type, resource, error, or ID" onChange={(event) => { setQuery(event.target.value); setPage(1) }} /></label><label>State<select value={state} onChange={(event) => { setState(event.target.value); setPage(1) }}><option value="">All states</option><option>queued</option><option>running</option><option>succeeded</option><option>failed</option></select></label></div>
		{message ? <p className="feedback">{message}</p> : null}{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Loading jobs…" /> : null}
		{!loading && <div className="table-wrap"><table className="dense-table"><thead><tr><th>Created</th><th>Type</th><th>Resource</th><th>State</th><th>Progress</th><th>Attempts</th><th>Error</th><th>Actions</th></tr></thead><tbody>{paged.items.map((job) => <tr key={job.id}><td>{formatDate(job.created_at)}</td><td><strong>{job.type}</strong><small>{job.id.slice(0, 12)}</small></td><td>{job.resource_type || '—'}<small>{job.resource_id || '—'}</small></td><td><StatusBadge value={job.state} /></td><td><progress max={100} value={job.progress}>{job.progress}%</progress><small>{job.progress}%</small></td><td>{job.attempts}/{job.max_attempts}</td><td className="truncate">{job.last_error || '—'}</td><td><button type="button" className="link-button" onClick={() => setSelected(job)}>Details</button></td></tr>)}</tbody></table></div>}
		{!loading && !paged.items.length ? <EmptyState title="No jobs match" detail="Filters and state summaries remain available. Clear filters or start an account operation." /> : null}
		<Pagination page={paged.page} pageCount={paged.pageCount} total={paged.total} onPage={setPage} />
		<Dialog open={Boolean(selected)} title={selected ? `${selected.type} details` : 'Job details'} onClose={() => setSelected(null)}>{selected ? <><dl className="detail-list"><div><dt>Job ID</dt><dd><code>{selected.id}</code></dd></div><div><dt>State</dt><dd><StatusBadge value={selected.state} /></dd></div><div><dt>Created</dt><dd>{formatDate(selected.created_at)}</dd></div><div><dt>Run after</dt><dd>{formatDate(selected.run_after)}</dd></div><div><dt>Last error</dt><dd>{selected.last_error || '—'}</dd></div></dl><h3>Safe payload</h3><pre>{JSON.stringify(selected.payload, null, 2)}</pre><h3>Logs</h3><pre>{selected.logs?.join('\n') || 'No logs recorded.'}</pre><footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setSelected(null)}>Close</button>{selected.state === 'failed' && canRetryJob(selected, capabilities) ? <button type="button" onClick={() => retry(selected)}>Retry failed job</button> : null}</footer></> : null}</Dialog>
	</>
}

function canRetryJob (job: Job, capabilities: Record<string, boolean>): boolean {
	const prefixes: Array<[string, string]> = [
		['account.provision', 'accounts.create'], ['account.copy_homedir', 'accounts.create'], ['account.reconcile', 'accounts.modify'],
		['domain.', 'domains.write'], ['website.', 'websites.write'], ['application.', 'applications.write'],
		['wordpress.', 'applications.write'], ['database.', 'databases.write'], ['dns.', 'dns.write'],
		['mail', 'mail.write'], ['certificate.provision', 'websites.write'], ['certificate.portal', 'server.settings.write'],
		['backup.create', 'backups.create'], ['backup.restore', 'backups.restore'], ['cron.', 'cron.write'], ['ftp.', 'files.write'],
	]
	const match = prefixes.find(([prefix]) => job.type.startsWith(prefix))
	return Boolean(match && capabilities[match[1]])
}
