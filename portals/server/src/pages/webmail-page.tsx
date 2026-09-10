import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { CopyableValue, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import { mailConnectionSettings, resolvedWebmailUrl } from './mail-connection'
import { RequestSequence } from '../request-sequence'
import type { Account, ResourceItem } from '../types'

interface MailboxRow extends ResourceItem {
	account_id: string
	account_name: string
	domain_name: string
	address: string
}

export function WebmailPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [mailboxes, setMailboxes] = useState<MailboxRow[]>([])
	const [hostname, setHostname] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const [viewAll, setViewAll] = useState(!params.get('account'))
	const [toolUrls, setToolUrls] = useState<Record<string, { webmail_url?: string }>>({})
	const requests = useRef(new RequestSequence()).current
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId

	useEffect(() => {
		api<{ system: { hostname: string } }>('/api/v1/server').then((result) => setHostname(result.system.hostname)).catch(() => setHostname(window.location.hostname))
	}, [])

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			setAccounts(asList(result))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests])

	const loadMailboxes = useCallback(async (scopeAccounts: Account[]) => {
		const request = requests.begin('mailboxes')
		setLoading(true)
		setError('')
		try {
			const results = await Promise.allSettled(scopeAccounts.map(async (entry) => {
				const [mailboxResult, domainResult] = await Promise.all([
					api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/mail/mailboxes`),
					api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/domains`).catch(() => ({ items: [] as ResourceItem[] })),
				])
				const domains = asList(domainResult)
				const domainById = new Map(domains.map((domain) => [domain.id, valueOf(domain, 'ascii_fqdn') || entry.primary_domain]))
				return asList(mailboxResult).map((mailbox) => ({
					...mailbox,
					account_id: entry.id,
					account_name: entry.username,
					domain_name: domainById.get(String(mailbox.domain_id)) || entry.primary_domain,
					address: `${valueOf(mailbox, 'local_part')}@${domainById.get(String(mailbox.domain_id)) || entry.primary_domain}`,
				}))
			}))
			if (!requests.isCurrent(request)) return
			setMailboxes(results.flatMap((result) => result.status === 'fulfilled' ? result.value : []))
			setUpdatedAt(new Date().toISOString())
		} catch (requestError) {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		} finally {
			if (requests.isCurrent(request)) setLoading(false)
		}
	}, [requests])

	useEffect(() => {
		if (!accounts.length) return
		if (viewAll) {
			loadMailboxes(accounts)
			return
		}
		const account = accounts.find((entry) => entry.id === accountId)
		if (account) loadMailboxes([account])
		else setLoading(false)
	}, [accountId, accounts, loadMailboxes, viewAll])

	const settings = useMemo(() => mailConnectionSettings(hostname), [hostname])
	const scopedAccount = accounts.find((entry) => entry.id === accountId)
	const resolvedMail = resolvedWebmailUrl(accountId ? toolUrls[accountId]?.webmail_url : undefined, scopedAccount?.primary_domain)

	const ensureToolUrls = useCallback(async (requestedAccountId: string) => {
		try {
			const result = await api<{ webmail_url: string }>(`/api/v1/accounts/${requestedAccountId}/admin-tools`)
			setToolUrls((current) => current[requestedAccountId] ? current : { ...current, [requestedAccountId]: result })
			return result
		} catch {
			const domain = accounts.find((entry) => entry.id === requestedAccountId)?.primary_domain || 'example.com'
			return { webmail_url: `https://webmail.${domain}/` }
		}
	}, [accounts])

	useEffect(() => {
		if (accountId) void ensureToolUrls(accountId)
	}, [accountId, ensureToolUrls])

	async function openWebmail (mailbox: MailboxRow) {
		const tools = await ensureToolUrls(mailbox.account_id)
		window.open(tools.webmail_url || `https://webmail.${mailbox.domain_name}/`, '_blank', 'noopener,noreferrer')
	}

	return (
		<>
			<PageHeader
				title={scopedAccount ? `Webmail · ${scopedAccount.username}` : 'Webmail'}
				description="Launch webmail sessions and review IMAP/SMTP connection settings for account mailboxes."
				actions={<Link className="button-link secondary-link" to={accountId ? `/email?account=${accountId}` : '/email'}>Email management</Link>}
			/>
			{!viewAll ? <AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Webmail" onChange={(next) => setParams({ account: next }, { replace: true })} /> : null}
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<label className="checkbox-label"><input type="checkbox" checked={viewAll} onChange={(event) => setViewAll(event.target.checked)} />Show all accounts</label>
				{!viewAll ? <AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/> : null}
			</div>
			<section className="panel">
				<h2>Mail server settings</h2>
				<dl className="detail-list">
					<div><dt>IMAP</dt><dd><CopyableValue value={settings.imap} label="IMAP" /></dd></div>
					<div><dt>SMTP</dt><dd><CopyableValue value={settings.smtp} label="SMTP" /></dd></div>
					<div><dt>Webmail URL</dt><dd>{resolvedMail.url ? <CopyableValue value={resolvedMail.url} label="webmail URL" /> : <code>https://webmail.&lt;domain&gt;/</code>}</dd></div>
				</dl>
				<p className="subtle">{resolvedMail.configured ? 'This URL is the resolved webmail vhost for the selected account.' : 'The URL is inferred from the account domain until admin-tools reports a configured vhost. mail.hosting-style hosts come from the live panel hostname, not a placeholder product name.'}</p>
			</section>
			{error ? <ErrorState error={error} onRetry={() => loadMailboxes(viewAll ? accounts : accounts.filter((entry) => entry.id === accountId))} /> : null}
			<section className="panel">
				<h2>{viewAll ? 'All mailboxes' : 'Account mailboxes'}</h2>
				{loading ? <LoadingState label="Loading mailboxes…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Mailbox</th><th>Account</th><th>Status</th><th>Actions</th></tr></thead>
					<tbody>
						{mailboxes.map((mailbox) => (
							<tr key={`${mailbox.account_id}-${mailbox.id}`}>
								<td><strong>{mailbox.address}</strong></td>
								<td><Link to={`/email?account=${mailbox.account_id}`}>{mailbox.account_name}</Link></td>
								<td><StatusBadge value={valueOf(mailbox, 'status')} /></td>
								<td className="row-actions">
									<button type="button" className="link-button" onClick={() => openWebmail(mailbox)}>Open webmail</button>
									<Link className="link-button" to={`/accounts/${mailbox.account_id}/services?service=mailboxes`}>Manage</Link>
								</td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading && !mailboxes.length ? <EmptyState title="No mailboxes found" detail="Create mailboxes in Email Management before launching webmail." action={<Link className="button-link" to="/email">Email Management</Link>} /> : null}
			</section>
		</>
	)
}
