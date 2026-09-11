import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { Dialog, EmptyState, ErrorState, LoadingState, Metric, PageHeader, Pagination, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCapabilities } from '../rbac'
import { filterRows, paginateRows, sortRows } from '../table-helpers'
import type { Job } from '../types'
import { describeJobFailure, describeJobTimeline, formatJobType, jobMatchesAccount, jobRecoveryGuidance, summarizeJobCounts } from './job-copy'

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
	const scoped = useMemo(() => items.filter((job) => jobMatchesAccount(job, accountId || '')), [items, accountId])
	const visible = useMemo(() => filterRows(scoped, query, ['id', 'type', 'state', 'resource_type', 'resource_id', 'last_error']), [scoped, query])
	const counts = summarizeJobCounts(scoped)
	const paged = paginateRows(sortRows(visible, 'created_at', 'desc'), page, 20)
	async function retry (job: Job) {
		try {
			const result = await api<{ operation_id?: string; id?: string }>(`/api/v1/jobs/${job.id}/retry`, { method: 'POST', body: '{}' })
			setMessage(`Retry queued as ${result.operation_id || result.id || 'a new job'}.`); setSelected(null); load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	const selectedFailure = selected ? describeJobFailure(selected) : null
	return <>
		<PageHeader title="Jobs" description={accountId ? 'Operations for the selected account.' : 'Search background work, inspect failures, and retry eligible jobs.'} actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		{accountId ? (
			<div className="context-chip" role="status">
				<p className="context-chip-copy">Showing jobs for this account.</p>
				<div className="context-chip-actions">
					<Link className="context-chip-link" to={`/accounts/${accountId}`}>Return to account</Link>
					<Link className="context-chip-link" to="/jobs">Show all jobs</Link>
				</div>
			</div>
		) : null}
		<section className="metric-grid compact-metrics" aria-label={accountId ? 'Account job totals' : 'Job totals'}><Metric label="Queued" value={counts.queued} /><Metric label="Running" value={counts.running} /><Metric label="Succeeded" value={counts.succeeded} /><Metric label="Failed" value={counts.failed} /></section>
		<div className="filter-bar"><label>Search jobs<input type="search" value={query} placeholder="Type, resource, error, or ID" onChange={(event) => { setQuery(event.target.value); setPage(1) }} /></label><label>State<select value={state} onChange={(event) => { setState(event.target.value); setPage(1) }}><option value="">All states</option><option>queued</option><option>running</option><option>succeeded</option><option>failed</option></select></label></div>
		{message ? <p className="feedback">{message}</p> : null}{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Loading jobs…" /> : null}
		{!loading && <ul className="job-cards">{paged.items.map((job) => {
			const failure = describeJobFailure(job)
			return <li key={`card-${job.id}`} className="job-card">
				<div><strong>{formatJobType(job.type)}</strong><StatusBadge value={job.state} /></div>
				<p>{job.state === 'failed' ? failure.reason : failure.resource}</p>
				<small>{formatDate(job.created_at)}</small>
				<button type="button" className="link-button" onClick={() => setSelected(job)}>Details</button>
			</li>
		})}</ul>}
		{!loading && <div className="table-wrap job-table"><table className="dense-table"><thead><tr><th scope="col">Created</th><th scope="col">Operation</th><th scope="col">Resource</th><th scope="col">State</th><th scope="col">Progress</th><th scope="col">Attempts</th><th scope="col">Result</th><th scope="col">Actions</th></tr></thead><tbody>{paged.items.map((job) => {
			const failure = describeJobFailure(job)
			return <tr key={job.id}><td>{formatDate(job.created_at)}</td><td><strong>{formatJobType(job.type)}</strong></td><td>{failure.resource}</td><td><StatusBadge value={job.state} /></td><td><progress max={100} value={job.progress}>{job.progress}%</progress><small>{job.progress}%</small></td><td>{job.attempts}/{job.max_attempts}</td><td className="truncate">{job.state === 'failed' ? failure.reason : '—'}</td><td><button type="button" className="link-button" onClick={() => setSelected(job)}>Details</button></td></tr>
		})}</tbody></table></div>}
		{!loading && !paged.items.length ? <EmptyState title="No jobs match" detail="Filters and state summaries remain available. Clear filters or start an account operation." /> : null}
		<Pagination page={paged.page} pageCount={paged.pageCount} total={paged.total} onPage={setPage} />
		<Dialog
			open={Boolean(selected)}
			title={selected ? `${formatJobType(selected.type)} details` : 'Job details'}
			onClose={() => setSelected(null)}
			actions={selected ? <>
				<button type="button" className="secondary" onClick={() => setSelected(null)}>Close</button>
				{selected.state === 'failed' && canRetryJob(selected, capabilities) ? <button type="button" onClick={() => retry(selected)}>Retry failed job</button> : null}
			</> : undefined}
		>{selected && selectedFailure ? <>
			<p className="job-summary">{selectedFailure.reason}</p>
			<dl className="detail-list">
				<div><dt>Affected</dt><dd>{selectedFailure.resource}</dd></div>
				<div><dt>State</dt><dd><StatusBadge value={selected.state} /></dd></div>
			</dl>
			<ol className="job-timeline">
				{describeJobTimeline(selected).map((entry) => (
					<li key={`${entry.label}-${entry.detail}`}><strong>{entry.label}</strong>{entry.detail}</li>
				))}
			</ol>
			{selected.state === 'failed' ? <p className="subtle">{jobRecoveryGuidance(selected)}</p> : null}
			<details className="job-technical"><summary>Technical details</summary><pre>{JSON.stringify(selected.payload, null, 2)}</pre>{selectedFailure.technical ? <pre>{selectedFailure.technical}</pre> : null}</details>
		</> : null}</Dialog>
	</>
}

export function canRetryJob (job: Job, capabilities: Record<string, boolean>): boolean {
	if (job.retryable === false) return false
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
