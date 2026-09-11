import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

export function CronPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [jobs, setJobs] = useState<ResourceItem[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('cron.write')
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const account = accounts.find((entry) => entry.id === accountId)

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

	const loadCron = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('cron')
		setLoading(true)
		setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/cron`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setJobs(asList(result))
			setUpdatedAt(new Date().toISOString())
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('cron')
		setJobs([])
		setMessage('')
		if (accountId) loadCron(accountId)
		else setLoading(false)
	}, [accountId, loadCron, requests])

	async function createCron (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/cron`, {
				method: 'POST',
				body: JSON.stringify({
					schedule: data.get('schedule'),
					command: data.get('command'),
					working_directory: account?.home_path,
					enabled: true,
				}),
			})
			setMessage('Cron job queued.')
			event.currentTarget.reset()
			loadCron(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeCron (cronId: string) {
		if (!accountId || !window.confirm('Delete this scheduled task?')) return
		try {
			await api(`/api/v1/accounts/${accountId}/cron/${cronId}`, { method: 'DELETE' })
			setMessage('Cron deletion queued.')
			loadCron(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<>
			<PageHeader
				title={account ? `Cron Jobs · ${account.username}` : 'Cron Jobs'}
				description="Create and remove account-scoped scheduled tasks. Jobs run as the POSIX tenant after the worker applies them."
				actions={accountId ? <Link className="button-link secondary-link" to={`/jobs?account=${accountId}`}>Related jobs</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Cron" onChange={(next) => setParams({ account: next }, { replace: true })} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/>
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadCron(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage its cron jobs." /> : null}
			{accountId ? <>
				{canWrite ? <section className="panel">
					<h2>Add a scheduled task</h2>
					<form className="inline-form" onSubmit={createCron}>
						<label>Schedule<input name="schedule" defaultValue="0 * * * *" required /></label>
						<label>Command<input name="command" placeholder="/usr/bin/php cron.php" required /></label>
						<button type="submit">Add cron job</button>
					</form>
					<p className="subtle">The working directory defaults to {account?.home_path || 'the account home'}.</p>
				</section> : null}
				<section className="panel">
					<h2>Scheduled tasks for {account?.username}</h2>
					{loading ? <LoadingState label="Loading cron jobs…" /> : null}
					{!loading ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Schedule</th><th>Command</th><th>Directory</th><th>Enabled</th><th>Actions</th></tr></thead>
						<tbody>
							{jobs.map((job) => (
								<tr key={job.id}>
									<td><code>{valueOf(job, 'schedule')}</code></td>
									<td>{valueOf(job, 'command')}</td>
									<td><code>{valueOf(job, 'working_directory')}</code></td>
									<td><StatusBadge value={Boolean(job.enabled)} /></td>
									<td>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeCron(job.id)}>Delete</button> : 'View only'}</td>
								</tr>
							))}
						</tbody>
					</table></div> : null}
					{!loading && !jobs.length ? <EmptyState title="No cron jobs yet" detail="Add a schedule and command for this account." /> : null}
				</section>
			</> : null}
		</>
	)
}
