import { FormEvent, useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account } from '../types'

interface WebsiteRow {
	id: string
	domain_id?: string
	document_root?: string
}

interface DomainRow {
	id: string
	ascii_fqdn?: string
}

interface RedirectRow {
	id: string
	website_id?: string
	source?: string
	target?: string
	status?: number
	wildcard?: boolean
}

function websiteLabel (website: WebsiteRow, domains: DomainRow[]) {
	const domain = domains.find((entry) => entry.id === website.domain_id)
	return domain?.ascii_fqdn || website.document_root || website.id
}

export function RedirectsPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [websites, setWebsites] = useState<WebsiteRow[]>([])
	const [domains, setDomains] = useState<DomainRow[]>([])
	const [rows, setRows] = useState<RedirectRow[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('websites.write')
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

	const loadRedirects = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('redirects')
		setLoading(true)
		setError('')
		Promise.all([
			api<{ items: RedirectRow[] }>(`/api/v1/accounts/${requestedAccountId}/redirects`),
			api<{ items: WebsiteRow[] }>(`/api/v1/accounts/${requestedAccountId}/websites`),
			api<{ items: DomainRow[] }>(`/api/v1/accounts/${requestedAccountId}/domains`),
		]).then(([redirectResult, siteResult, domainResult]) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setRows(asList(redirectResult))
			setWebsites(asList(siteResult))
			setDomains(asList(domainResult))
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
		requests.invalidate('redirects')
		setRows([])
		setMessage('')
		if (accountId) loadRedirects(accountId)
		else setLoading(false)
	}, [accountId, loadRedirects, requests])

	async function createRedirect (event: FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		setMessage('')
		try {
			await api(`/api/v1/accounts/${accountId}/redirects`, {
				method: 'POST',
				body: JSON.stringify({
					website_id: data.get('website_id'),
					source: data.get('source'),
					target: data.get('target'),
					status: Number(data.get('status') || 301),
					wildcard: data.get('wildcard') === 'on',
				}),
			})
			setMessage('Redirect saved. nginx apply was queued on the host.')
			event.currentTarget.reset()
			loadRedirects(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeRedirect (redirectId: string) {
		if (!accountId || !window.confirm('Delete this redirect?')) return
		setMessage('')
		try {
			await api(`/api/v1/accounts/${accountId}/redirects/${redirectId}`, { method: 'DELETE' })
			setMessage('Redirect removed. nginx apply was queued on the host.')
			loadRedirects(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<>
			<PageHeader
				title={account ? `Redirects · ${account.username}` : 'Redirects'}
				description="List and apply nginx path redirects for an account vhost. Empty until you add a host-backed rule."
				actions={accountId ? <Link className="button-link secondary-link" to={`/websites?account=${accountId}`}>MultiPHP Manager</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Redirects" onChange={(next) => setParams({ account: next }, { replace: true })} />
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
			{error ? <ErrorState error={error} onRetry={() => loadRedirects(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage nginx path redirects." /> : null}
			{accountId ? <>
				{canWrite && websites.length ? <section className="panel">
					<h2>Add a path redirect</h2>
					<form className="inline-form" onSubmit={createRedirect}>
						<label>
							Website
							<select name="website_id" required defaultValue={websites[0]?.id}>
								{websites.map((website) => (
									<option key={website.id} value={website.id}>{websiteLabel(website, domains)}</option>
								))}
							</select>
						</label>
						<label>Source path<input name="source" placeholder="/old" required /></label>
						<label>Target URL<input name="target" placeholder="https://example.com/new" required /></label>
						<label>
							Status
							<select name="status" defaultValue="301">
								<option value="301">301 permanent</option>
								<option value="302">302 temporary</option>
							</select>
						</label>
						<label className="checkbox-label"><input type="checkbox" name="wildcard" /> Prefix match</label>
						<button type="submit">Add redirect</button>
					</form>
				</section> : null}
				<section className="panel">
					<h2>Redirects for {account?.username}</h2>
					{loading ? <LoadingState label="Loading redirects…" /> : null}
					{!loading && websites.length === 0 ? (
						<EmptyState
							title="No websites"
							detail="This account has no nginx vhosts yet."
							action={<Link to={`/websites?account=${accountId}`}>Open MultiPHP Manager</Link>}
						/>
					) : null}
					{!loading && websites.length ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Source</th><th>Target</th><th>Status</th><th>Website</th><th>Actions</th></tr></thead>
						<tbody>
							{rows.map((row) => (
								<tr key={row.id}>
									<td><code>{row.source}</code>{row.wildcard ? ' (prefix)' : ''}</td>
									<td><code>{row.target}</code></td>
									<td>{row.status || 301}</td>
									<td>{websiteLabel({ id: row.website_id || '', domain_id: websites.find((site) => site.id === row.website_id)?.domain_id, document_root: websites.find((site) => site.id === row.website_id)?.document_root }, domains)}</td>
									<td>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeRedirect(row.id)}>Delete</button> : 'View only'}</td>
								</tr>
							))}
						</tbody>
					</table></div> : null}
					{!loading && websites.length && !rows.length ? <EmptyState title="No redirects yet" detail="Add a source path and target URL. Kelmor writes the rule into the account nginx vhost." /> : null}
				</section>
			</> : null}
		</>
	)
}
