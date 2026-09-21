import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import type { Account } from '../types'

interface GitRepo {
	path?: string
	name?: string
}

export function GitPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [repos, setRepos] = useState<GitRepo[]>([])
	const [home, setHome] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const requests = useRef(new RequestSequence()).current
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

	const loadRepos = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('git')
		setLoading(true)
		setError('')
		api<{ items?: GitRepo[]; home?: string }>(`/api/v1/accounts/${requestedAccountId}/git`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setRepos(asList(result))
			setHome(result.home || '')
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) {
				setError(messageFrom(requestError))
			}
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) {
				setLoading(false)
			}
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('git')
		setRepos([])
		setHome('')
		if (accountId) loadRepos(accountId)
		else setLoading(false)
	}, [accountId, loadRepos, requests])

	return (
		<>
			<PageHeader
				title={account ? `Git Version Control · ${account.username}` : 'Git Version Control'}
				description="Discover .git repositories under an account home. Kelmor does not create, clone, or invent repositories."
				actions={accountId ? <Link className="button-link secondary-link" to={`/files?account=${accountId}`}>File Manager</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Git" onChange={(next) => setParams({ account: next }, { replace: true })} />
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/>
			</div>
			{error ? <ErrorState error={error} onRetry={() => loadRepos(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to discover Git repositories under its home." /> : null}
			{accountId ? <section className="panel">
				<h2>Repositories for {account?.username}</h2>
				{home ? <p className="subtle">Home: <code>{home}</code></p> : null}
				{loading ? <LoadingState label="Scanning for .git directories…" /> : null}
				{!loading && repos.length ? (
					<div className="table-wrap">
						<table className="dense-table">
							<thead><tr><th>Name</th><th>Path under home</th></tr></thead>
							<tbody>
								{repos.map((repo) => (
									<tr key={repo.path || repo.name}>
										<td><strong>{repo.name || repo.path}</strong></td>
										<td><code>{repo.path || '—'}</code></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				) : null}
				{!loading && !repos.length ? (
					<EmptyState
						title="No Git repositories"
						detail="No .git directories were found under this account home. Create a repository on the host, then refresh."
						action={<Link to={`/files?account=${accountId}`}>Open File Manager</Link>}
					/>
				) : null}
			</section> : null}
		</>
	)
}
