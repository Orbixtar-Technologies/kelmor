import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { EmptyState, ErrorState, LoadingState, PageHeader, SecretValue, StatusBadge } from '../components/ui'
import { messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

export function SQLManagerPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [databases, setDatabases] = useState<ResourceItem[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [credentials, setCredentials] = useState<Record<string, string> | null>(null)
	const [toolUrls, setToolUrls] = useState<{ phpmyadmin_url?: string; webmail_url?: string } | null>(null)
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('databases.write')
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

	const loadDatabases = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('databases')
		setLoading(true)
		setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/databases`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setDatabases(asList(result))
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('databases')
		setDatabases([])
		setCredentials(null)
		setToolUrls(null)
		setMessage('')
		if (accountId) {
			loadDatabases(accountId)
			api<{ credentials: Record<string, string> }>(`/api/v1/accounts/${accountId}/databases/credentials?engine=mariadb`).then((result) => setCredentials(result.credentials)).catch(() => setCredentials(null))
			api<{ phpmyadmin_url: string; webmail_url: string }>(`/api/v1/accounts/${accountId}/admin-tools`).then(setToolUrls).catch(() => setToolUrls(null))
		} else setLoading(false)
	}, [accountId, loadDatabases, requests])

	async function createDatabase (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/databases`, {
				method: 'POST',
				body: JSON.stringify({ name: data.get('name'), engine: data.get('engine') }),
			})
			setMessage('Database creation queued.')
			event.currentTarget.reset()
			loadDatabases(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeDatabase (databaseId: string) {
		if (!accountId || !window.confirm('Delete this database?')) return
		try {
			await api(`/api/v1/accounts/${accountId}/databases/${databaseId}`, { method: 'DELETE' })
			setMessage('Database deletion queued.')
			loadDatabases(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<>
			<PageHeader
				title={account ? `Database Manager · ${account.username}` : 'Database Manager'}
				description="Create and manage MariaDB, MySQL, and PostgreSQL databases across hosting accounts."
				actions={account ? <Link className="button-link secondary-link" to={`/accounts/${accountId}`}>Return to account</Link> : undefined}
			/>
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
			{error ? <ErrorState error={error} onRetry={() => loadDatabases(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage its databases." /> : null}
			{accountId ? <>
				<section className="panel">
					<h2>Database administration</h2>
					<p className="subtle">Database users are provisioned automatically. Use phpMyAdmin or direct SQL clients with the credentials below.</p>
					{credentials ? <dl className="detail-list">
						<div><dt>Host</dt><dd>{credentials.host || '127.0.0.1'}</dd></div>
						<div><dt>Username</dt><dd><code>{credentials.username}</code></dd></div>
						<div><dt>Password</dt><dd><SecretValue value={credentials.password || ''} /></dd></div>
					</dl> : <p className="subtle">Credentials appear after the first database is provisioned.</p>}
					<div className="admin-links">
						{toolUrls?.phpmyadmin_url ? <a href={toolUrls.phpmyadmin_url} target="_blank" rel="noreferrer">Open phpMyAdmin</a> : null}
						<Link to={`/accounts/${accountId}/services?service=databases`}>Account services</Link>
						<Link to={`/files?account=${accountId}`}>File manager</Link>
					</div>
				</section>
				{canWrite ? <section className="panel">
					<h2>Create database</h2>
					<form className="inline-form" onSubmit={createDatabase}>
						<label>Name<input name="name" placeholder="app_db" required /></label>
						<label>Engine<select name="engine"><option value="mariadb">MariaDB</option><option value="mysql">MySQL</option><option value="postgres">PostgreSQL</option></select></label>
						<button type="submit">Create database</button>
					</form>
				</section> : null}
				<section className="panel">
					<h2>Databases for {account?.username}</h2>
					{loading ? <LoadingState label="Loading databases…" /> : null}
					{!loading ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Name</th><th>Engine</th><th>Status</th><th>Users</th><th>Actions</th></tr></thead>
						<tbody>
							{databases.map((database) => (
								<tr key={database.id}>
									<td><strong>{valueOf(database, 'name')}</strong></td>
									<td>{valueOf(database, 'engine')}</td>
									<td><StatusBadge value={valueOf(database, 'status')} /></td>
									<td>{Array.isArray(database.users) ? database.users.length : '—'}</td>
									<td><div className="row-actions"><Link to={`/accounts/${accountId}/services?service=databases`}>Details</Link>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeDatabase(database.id)}>Delete</button> : null}</div></td>
								</tr>
							))}
						</tbody>
					</table></div> : null}
					{!loading && !databases.length ? <EmptyState title="No databases yet" detail="Create a database using the form above." /> : null}
				</section>
			</> : null}
		</>
	)
}
