import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { listOf, post } from '../client'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { ConfirmDialog, Drawer, EmptyState, KeyValues, Notice, PageHeader, Panel, Pill, statusTone } from '../components/ui'
import { formatDateTime, formatRelative, shortId } from '../lib/format'
import type { AccountRow } from '../components/account-picker'

interface JobRow {
	id: string
	type: string
	resource_type?: string
	resource_id?: string
	payload?: Record<string, unknown>
	state: string
	priority: number
	attempts: number
	max_attempts: number
	progress: number
	run_after: string
	locked_by?: string
	last_error?: string
	actor_id?: string
	request_id?: string
	created_at: string
	started_at?: string
	finished_at?: string
	logs?: string[]
}

const states = ['queued', 'running', 'succeeded', 'failed', 'cancelled', 'dead']

export function JobQueue () {
	const { jobId } = useParams()
	const navigate = useNavigate()
	const toast = useToast()
	const jobs = useLoad<JobRow[]>(() => listOf<JobRow>('/api/v1/jobs'), [], 4000)
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts').catch(() => []), [])
	const [stateFilter, setStateFilter] = useState('')
	const [typeFilter, setTypeFilter] = useState('')
	const [pending, setPending] = useState<{ job: JobRow; action: 'retry' | 'cancel' } | null>(null)
	const [busy, setBusy] = useState(false)

	const all = jobs.data ?? []
	const accountName = (job: JobRow) => {
		const id = job.resource_type === 'account' ? job.resource_id : (job.payload as { account_id?: string })?.account_id
		return (accounts.data ?? []).find((a) => a.id === id)
	}

	const rows = all.filter((job) => (!stateFilter || job.state === stateFilter) && (!typeFilter || job.type === typeFilter))
	const types = [...new Set(all.map((job) => job.type))].sort()
	const counts = states.map((state) => ({ state, count: all.filter((job) => job.state === state).length }))
	const selected = jobId ? all.find((job) => job.id === jobId) : undefined

	async function run (job: JobRow, action: 'retry' | 'cancel') {
		setBusy(true)
		await toast.run(
			() => post(`/api/v1/jobs/${job.id}/${action}`),
			() => (action === 'retry' ? `${job.type} re-queued` : `${job.type} cancelled`),
		)
		await jobs.reload()
		setBusy(false)
	}

	const columns: Column<JobRow>[] = [
		{
			key: 'created',
			header: 'Queued',
			sort: (j) => j.created_at,
			render: (j) => <span title={formatDateTime(j.created_at)}>{formatRelative(j.created_at)}</span>,
		},
		{ key: 'type', header: 'Type', sort: (j) => j.type, render: (j) => <span className="mono">{j.type}</span> },
		{
			key: 'account',
			header: 'Account',
			sort: (j) => accountName(j)?.username ?? '',
			render: (j) => {
				const account = accountName(j)
				return account ? <Link to={`/accounts/${account.id}`}>{account.username}</Link> : <span className="muted">—</span>
			},
		},
		{ key: 'state', header: 'State', sort: (j) => j.state, render: (j) => <Pill tone={statusTone(j.state)}>{j.state}</Pill> },
		{ key: 'progress', header: 'Progress', align: 'right', sort: (j) => j.progress, render: (j) => `${j.progress}%` },
		{ key: 'attempts', header: 'Attempts', align: 'right', sort: (j) => j.attempts, render: (j) => `${j.attempts} / ${j.max_attempts}` },
		{
			key: 'error',
			header: 'Last error',
			sort: (j) => j.last_error || '',
			render: (j) => (j.last_error ? <span className="small" style={{ color: 'var(--danger)' }}>{j.last_error}</span> : <span className="muted">—</span>),
		},
	]

	return (
		<>
			<PageHeader
				title="Job Queue"
				description="Every privileged operation runs as a durable job: provisioning, reconcile, backups, restores and certificate orders. Progress is observed from the worker, not guessed."
				favoritePath="/jobs"
				actions={<button type="button" className="btn secondary" onClick={() => jobs.reload()}>Refresh</button>}
			/>

			<Panel title="Queue health" icon="activity" subtitle={`${all.length} jobs recorded`}>
				<div className="chip-row">
					{counts.map(({ state, count }) => (
						<button
							key={state}
							type="button"
							className={stateFilter === state ? 'chip on' : 'chip'}
							aria-pressed={stateFilter === state}
							onClick={() => setStateFilter(stateFilter === state ? '' : state)}
						>
							<Pill tone={statusTone(state)}>{count}</Pill>
							<span>{state}</span>
						</button>
					))}
				</div>
			</Panel>

			{all.some((job) => job.state === 'failed') ? (
				<Notice tone="warn">
					Failed jobs stay in the queue with their last error so the cause is visible. Fix the underlying condition, then
					retry the job — Kelmor does not silently drop desired state.
				</Notice>
			) : null}

			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(j) => j.id}
				loading={jobs.loading}
				error={jobs.error}
				searchPlaceholder="Search type, state or error"
				noun="jobs"
				pageSize={25}
				initialSort={{ key: 'created', dir: 'desc' }}
				filters={
					<>
						<select value={stateFilter} onChange={(e) => setStateFilter(e.target.value)} aria-label="Filter by state">
							<option value="">All states</option>
							{states.map((state) => <option key={state} value={state}>{state}</option>)}
						</select>
						<select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} aria-label="Filter by type">
							<option value="">All types</option>
							{types.map((type) => <option key={type} value={type}>{type}</option>)}
						</select>
						{stateFilter || typeFilter ? (
							<button type="button" className="linkish" onClick={() => { setStateFilter(''); setTypeFilter('') }}>Clear filters</button>
						) : null}
					</>
				}
				empty={
					<EmptyState icon="list" title={stateFilter || typeFilter ? 'No job matches these filters' : 'No job has run yet'}>
						{stateFilter || typeFilter
							? 'Clear the filters to see the full queue history.'
							: 'Provisioning an account, queuing a backup or requesting a certificate all create jobs that appear here with their full history.'}
					</EmptyState>
				}
				rowActions={[
					{ label: 'View detail', onSelect: (job) => navigate(`/jobs/${job.id}`) },
					{
						label: 'Retry job',
						onSelect: (job) => setPending({ job, action: 'retry' }),
						hidden: (job) => !['failed', 'cancelled', 'dead'].includes(job.state),
					},
					{
						label: 'Cancel job',
						danger: true,
						onSelect: (job) => setPending({ job, action: 'cancel' }),
						hidden: (job) => !['queued', 'failed'].includes(job.state),
					},
					{
						label: 'Open account',
						onSelect: (job) => {
							const account = accountName(job)
							if (account) navigate(`/accounts/${account.id}`)
						},
						hidden: (job) => !accountName(job),
					},
				]}
			/>

			{selected ? (
				<Drawer title={<span className="mono">{selected.type}</span>} onClose={() => navigate('/jobs')}>
					<KeyValues
						rows={[
							['Job ID', <span key="i" className="mono">{selected.id}</span>],
							['State', <Pill key="s" tone={statusTone(selected.state)}>{selected.state}</Pill>],
							['Progress', `${selected.progress}%`],
							['Attempts', `${selected.attempts} of ${selected.max_attempts}`],
							['Priority', selected.priority],
							['Resource', selected.resource_type ? `${selected.resource_type} ${shortId(selected.resource_id)}` : '—'],
							['Queued', formatDateTime(selected.created_at)],
							['Started', formatDateTime(selected.started_at)],
							['Finished', formatDateTime(selected.finished_at)],
							['Run after', formatDateTime(selected.run_after)],
							['Locked by', selected.locked_by || '—'],
							['Request ID', <span key="r" className="mono small">{selected.request_id || '—'}</span>],
							['Last error', selected.last_error || '—'],
						]}
					/>
					<h3 style={{ margin: '18px 0 8px', fontSize: 13 }}>Payload</h3>
					<pre className="code">{JSON.stringify(redact(selected.payload ?? {}), null, 2)}</pre>
					{selected.logs?.length ? (
						<>
							<h3 style={{ margin: '18px 0 8px', fontSize: 13 }}>Worker log</h3>
							<pre className="code">{selected.logs.join('\n')}</pre>
						</>
					) : null}
					<div className="btn-row" style={{ marginTop: 18 }}>
						{['failed', 'cancelled', 'dead'].includes(selected.state) ? (
							<button type="button" className="btn" disabled={busy} onClick={() => run(selected, 'retry')}>Retry job</button>
						) : null}
						{['queued', 'failed'].includes(selected.state) ? (
							<button type="button" className="btn secondary" disabled={busy} onClick={() => run(selected, 'cancel')}>Cancel job</button>
						) : null}
					</div>
				</Drawer>
			) : null}

			{pending ? (
				<ConfirmDialog
					title={pending.action === 'retry' ? 'Retry this job?' : 'Cancel this job?'}
					tone={pending.action === 'retry' ? 'normal' : 'danger'}
					confirmLabel={pending.action === 'retry' ? 'Retry job' : 'Cancel job'}
					busy={busy}
					onCancel={() => setPending(null)}
					onConfirm={async () => {
						const { job, action } = pending
						setPending(null)
						await run(job, action)
					}}
				>
					<p>
						{pending.action === 'retry'
							? `${pending.job.type} is put back on the queue and picked up by the next free worker. Fix the cause first or it will fail the same way.`
							: `${pending.job.type} will not run. The desired state that created it stays recorded, so a later reconcile may queue it again.`}
					</p>
					{pending.job.last_error ? <Notice tone="error">{pending.job.last_error}</Notice> : null}
				</ConfirmDialog>
			) : null}
		</>
	)
}

/** Job payloads can carry provisioning credentials; never render them. */
function redact (payload: Record<string, unknown>) {
	const out: Record<string, unknown> = {}
	for (const [key, value] of Object.entries(payload)) {
		out[key] = /password|secret|token|key/i.test(key) ? '«redacted»' : value
	}
	return out
}
