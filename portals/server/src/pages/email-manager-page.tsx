import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

type EmailTab = 'domains' | 'mailboxes' | 'aliases'

const tabLabels: Record<EmailTab, string> = {
	domains: 'Mail domains',
	mailboxes: 'Mailboxes',
	aliases: 'Aliases',
}

export function EmailManagerPage () {
	const [params, setSearchParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [domains, setDomains] = useState<ResourceItem[]>([])
	const [mailboxes, setMailboxes] = useState<ResourceItem[]>([])
	const [aliases, setAliases] = useState<ResourceItem[]>([])
	const [accountDomains, setAccountDomains] = useState<ResourceItem[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('mail.write')
	const accountId = params.get('account') || ''
	const tab = (params.get('tab') as EmailTab) || 'mailboxes'
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const account = accounts.find((entry) => entry.id === accountId)

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!currentAccountId.current && next[0]) setSearchParams({ account: next[0].id, tab }, { replace: true })
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests, setSearchParams, tab])

	const loadEmail = useCallback((requestedAccountId: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('email')
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/mail/domains`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/mail/mailboxes`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/mail/aliases`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/domains`),
		]).then(([domainResult, mailboxResult, aliasResult, siteDomainResult]) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			if (domainResult.status === 'fulfilled') setDomains(asList(domainResult.value))
			if (mailboxResult.status === 'fulfilled') setMailboxes(asList(mailboxResult.value))
			if (aliasResult.status === 'fulfilled') setAliases(asList(aliasResult.value))
			if (siteDomainResult.status === 'fulfilled') setAccountDomains(asList(siteDomainResult.value))
			const failures = [domainResult, mailboxResult, aliasResult].filter((result) => result.status === 'rejected')
			if (failures.length === 3) setError(messageFrom((failures[0] as PromiseRejectedResult).reason))
			else setUpdatedAt(new Date().toISOString())
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('email')
		setDomains([])
		setMailboxes([])
		setAliases([])
		setMessage('')
		if (accountId) loadEmail(accountId)
		else setLoading(false)
	}, [accountId, loadEmail, requests])

	function setTab (nextTab: EmailTab) {
		setSearchParams({ account: accountId, tab: nextTab }, { replace: true })
	}

	async function createMailbox (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/mail/mailboxes`, {
				method: 'POST',
				body: JSON.stringify({
					domain_id: data.get('domain_id'),
					local_part: data.get('local_part'),
					password: data.get('password'),
				}),
			})
			setMessage('Mailbox creation queued.')
			event.currentTarget.reset()
			loadEmail(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function createAlias (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/mail/aliases`, {
				method: 'POST',
				body: JSON.stringify({
					domain_id: data.get('domain_id'),
					address: data.get('address'),
					destination: data.get('destination'),
				}),
			})
			setMessage('Alias creation queued.')
			event.currentTarget.reset()
			loadEmail(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function patchCatchall (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		try {
			await api(`/api/v1/accounts/${accountId}/mail/domains/${data.get('mail_domain_id')}`, {
				method: 'PATCH',
				body: JSON.stringify({ catchall_policy: data.get('catchall_policy') }),
			})
			setMessage('Catch-all policy update queued.')
			loadEmail(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function removeItem (endpoint: string, resourceId: string) {
		if (!accountId || !window.confirm('Delete this mail resource?')) return
		try {
			await api(`/api/v1/accounts/${accountId}/${endpoint}/${resourceId}`, { method: 'DELETE' })
			setMessage('Delete operation queued.')
			loadEmail(accountId)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	const domainOptions = accountDomains.length ? accountDomains : domains

	return (
		<>
			<PageHeader
				title={account ? `Email Management · ${account.username}` : 'Email Management'}
				description="Manage mail domains, mailboxes, aliases, and routing policies across accounts."
				actions={<Link className="button-link secondary-link" to={accountId ? `/webmail?account=${accountId}` : '/webmail'}>Webmail</Link>}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Email" onChange={(next) => setSearchParams({ account: next, tab }, { replace: true })} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setSearchParams({ account: next, tab }, { replace: true })}
				/>
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadEmail(accountId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to manage email." /> : null}
			{accountId ? <>
				<div className="service-tabs" role="tablist" aria-label="Email sections">
					{(Object.keys(tabLabels) as EmailTab[]).map((entry) => (
						<button key={entry} type="button" role="tab" aria-selected={tab === entry} onClick={() => setTab(entry)}>
							{tabLabels[entry]}<span>{entry === 'domains' ? domains.length : entry === 'mailboxes' ? mailboxes.length : aliases.length}</span>
						</button>
					))}
				</div>
				{canWrite && tab === 'mailboxes' ? <section className="panel">
					<h2>Create mailbox</h2>
					<form className="inline-form" onSubmit={createMailbox}>
						<label>Domain<select name="domain_id" required>{domainOptions.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn') || valueOf(item, 'domain_id')}</option>)}</select></label>
						<label>Local part<input name="local_part" placeholder="info" required /></label>
						<label>Password<input name="password" type="password" required /></label>
						<button type="submit">Create mailbox</button>
					</form>
				</section> : null}
				{canWrite && tab === 'aliases' ? <section className="panel">
					<h2>Create alias</h2>
					<form className="inline-form" onSubmit={createAlias}>
						<label>Domain<select name="domain_id" required>{domainOptions.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn') || valueOf(item, 'domain_id')}</option>)}</select></label>
						<label>Address<input name="address" placeholder="support@example.com" required /></label>
						<label>Destination<input name="destination" placeholder="owner@example.com" required /></label>
						<button type="submit">Create alias</button>
					</form>
				</section> : null}
				{canWrite && tab === 'domains' ? <section className="panel">
					<h2>Catch-all routing</h2>
					<form className="inline-form" onSubmit={patchCatchall}>
						<label>Mail domain<select name="mail_domain_id" required>{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'domain_id')}</option>)}</select></label>
						<label>Policy<input name="catchall_policy" placeholder="reject, discard, or local part" required /></label>
						<button type="submit" disabled={!domains.length}>Update routing</button>
					</form>
				</section> : null}
				<section className="panel">
					<h2>{tabLabels[tab]} for {account?.username}</h2>
					{loading ? <LoadingState label="Loading email resources…" /> : null}
					{!loading && tab === 'domains' ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Domain</th><th>Catch-all</th><th>Status</th></tr></thead>
						<tbody>{domains.map((item) => <tr key={item.id}><td>{valueOf(item, 'domain_id')}</td><td>{valueOf(item, 'catchall_policy')}</td><td><StatusBadge value={valueOf(item, 'status')} /></td></tr>)}</tbody>
					</table></div> : null}
					{!loading && tab === 'mailboxes' ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Address</th><th>Quota</th><th>Status</th><th>Actions</th></tr></thead>
						<tbody>{mailboxes.map((item) => <tr key={item.id}><td>{valueOf(item, 'local_part')}@{account?.primary_domain}</td><td>{formatBytes(Number(item.quota_bytes || 0))} limit</td><td><StatusBadge value={valueOf(item, 'status')} /></td><td><div className="row-actions"><Link to={`/webmail?account=${accountId}`}>Open webmail</Link><Link to={`/accounts/${accountId}/services?service=mailboxes`}>Manage</Link>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeItem('mail/mailboxes', item.id)}>Delete</button> : null}</div></td></tr>)}</tbody>
					</table></div> : null}
					{!loading && tab === 'aliases' ? <div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Address</th><th>Destination</th><th>Actions</th></tr></thead>
						<tbody>{aliases.map((item) => <tr key={item.id}><td>{valueOf(item, 'address')}</td><td>{valueOf(item, 'destination')}</td><td>{canWrite ? <button type="button" className="link-button danger-text" onClick={() => removeItem('mail/aliases', item.id)}>Delete</button> : null}</td></tr>)}</tbody>
					</table></div> : null}
					{!loading && ((tab === 'domains' && !domains.length) || (tab === 'mailboxes' && !mailboxes.length) || (tab === 'aliases' && !aliases.length))
						? <EmptyState title={`No ${tabLabels[tab].toLocaleLowerCase()} yet`} detail="Provision email resources using the forms above or account services." />
						: null}
				</section>
			</> : null}
		</>
	)
}
