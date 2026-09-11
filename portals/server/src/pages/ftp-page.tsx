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

export function FTPPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [users, setUsers] = useState<ResourceItem[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('files.write')
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

	const loadFTP = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('ftp')
		setLoading(true)
		setError('')
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/ftp`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setUsers(asList(result))
			setUpdatedAt(new Date().toISOString())
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('ftp')
		setUsers([])
		setMessage('')
		if (accountId) loadFTP(accountId)
		else setLoading(false)
	}, [accountId, loadFTP, requests])

	async function createFTP (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/ftp`, {
				method: 'POST',
				body: JSON.stringify({
					username: data.get('username'),
					password: data.get('password'),
					home_path: data.get('home_path'),
				}),
			})
			setMessage('FTP user queued. The same username updates home path and password.')
			event.currentTarget.reset()
			loadFTP(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeFTP (ftpId: string) {
		if (!accountId || !window.confirm('Delete this FTP user?')) return
		try {
			await api(`/api/v1/accounts/${accountId}/ftp/${ftpId}`, { method: 'DELETE' })
			setMessage('FTP user deletion queued.')
			loadFTP(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<>
			<PageHeader
				title={account ? `FTP Accounts · ${account.username}` : 'FTP Accounts'}
				description="Create virtual FTP users chrooted to account content. Users map to the Linux tenant and honor disk caps."
				actions={accountId ? <Link className="button-link secondary-link" to={`/files?account=${accountId}`}>File Manager</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="FTP" onChange={(next) => setParams({ account: next }, { replace: true })} />
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
			{error ? <ErrorState error={error} onRetry={() => loadFTP(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage FTP users." /> : null}
			{accountId ? <>
				{canWrite ? <section className="panel">
					<h2>Create or update an FTP user</h2>
					<form className="inline-form" onSubmit={createFTP}>
						<label>Username<input name="username" required autoComplete="off" /></label>
						<label>Password<input name="password" type="password" minLength={8} required autoComplete="new-password" /></label>
						<label>Home path<input name="home_path" defaultValue={`${account?.home_path || ''}/public_html`} required /></label>
						<button type="submit">Save FTP user</button>
					</form>
				</section> : null}
				<section className="panel">
					<h2>FTP users for {account?.username}</h2>
					{loading ? <LoadingState label="Loading FTP users…" /> : null}
					{!loading ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Username</th><th>Home</th><th>Status</th><th>Actions</th></tr></thead>
						<tbody>
							{users.map((user) => (
								<tr key={user.id}>
									<td><strong>{valueOf(user, 'username')}</strong></td>
									<td><code>{valueOf(user, 'home_path')}</code></td>
									<td><StatusBadge value={valueOf(user, 'status')} /></td>
									<td>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeFTP(user.id)}>Delete</button> : 'View only'}</td>
								</tr>
							))}
						</tbody>
					</table></div> : null}
					{!loading && !users.length ? <EmptyState title="No FTP users yet" detail="Create a virtual FTP user chrooted to public_html or another account path." /> : null}
				</section>
			</> : null}
		</>
	)
}
