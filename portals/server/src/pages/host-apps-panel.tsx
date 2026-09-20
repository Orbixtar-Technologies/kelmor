import { useEffect, useState, type ReactElement } from 'react'
import { Link } from 'react-router-dom'
import { adminToolUrl } from '../admin-tool-url'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Account } from '../types'

interface HostAppAction {
	id: string
	label: string
	kind?: 'job' | 'href' | 'disabled'
	available: boolean
	reason?: string
	href?: string
}

interface HostApp {
	id: string
	label: string
	kind: string
	status: string
	path?: string
	description: string
	actions?: HostAppAction[]
}

interface HostAppsPanelProps {
	kind?: 'sql' | 'mail' | 'market' | 'plugin'
}

export function HostAppsPanel ({ kind }: HostAppsPanelProps) {
	const canWrite = useCan('server.settings.write')
	const [apps, setApps] = useState<HostApp[]>([])
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountId, setAccountId] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState('')
	const [urls, setUrls] = useState<{ phpmyadmin_url?: string; webmail_url?: string }>({})

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items?: HostApp[] }>('/api/v1/server/apps'),
			api<{ items?: Account[] }>('/api/v1/accounts'),
		]).then(([appResult, accountResult]) => {
			if (appResult.status === 'fulfilled') setApps(asList(appResult.value))
			else setError(messageFrom(appResult.reason))
			if (accountResult.status === 'fulfilled') {
				const next = asList(accountResult.value)
				setAccounts(next)
				if (!accountId && next[0]) setAccountId(next[0].id)
			}
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])

	useEffect(() => {
		if (!accountId) return
		api<{ phpmyadmin_url?: string; webmail_url?: string }>(`/api/v1/accounts/${accountId}/admin-tools`).then(setUrls).catch(() => setUrls({}))
	}, [accountId])

	const visible = apps.filter((app) => !kind || app.kind === kind || (kind === 'market' && (app.kind === 'market' || app.kind === 'sql' || app.kind === 'mail')))
	const selectedAccount = accounts.find((account) => account.id === accountId)
	const phpmyadminHref = adminToolUrl('phpmyadmin', urls.phpmyadmin_url, selectedAccount?.primary_domain)
	const webmailHref = adminToolUrl('webmail', urls.webmail_url, selectedAccount?.primary_domain)
	const showAccountPicker = kind !== 'plugin' && accounts.length > 0

	async function enable (app: HostApp) {
		setMessage('')
		setJobId('')
		try {
			await api(`/api/v1/server/apps/${app.id}/enable`, {
				method: 'POST',
				body: JSON.stringify({ account_id: accountId }),
			})
			setMessage(`${app.label} published for the selected account.`)
			if (accountId) {
				const next = await api<{ phpmyadmin_url?: string; webmail_url?: string }>(`/api/v1/accounts/${accountId}/admin-tools`)
				setUrls(next)
			}
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function runHostAction (app: HostApp, action: HostAppAction) {
		setBusy(`${app.id}:${action.id}`)
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/server/apps/${app.id}/actions`, {
				method: 'POST',
				body: JSON.stringify({ action: action.id }),
			})
			setJobId(result.operation_id || '')
			setMessage(queuedOpMessage(result, `${action.label} queued for ${app.label}.`))
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusy('')
		}
	}

	return (
		<section className="panel">
			<h2>Host applications</h2>
			<p>{kind === 'plugin'
				? 'These are first-party host packages Kelmor already wires, such as rspamd. Enable, disable, restart, or review them through Agent jobs.'
				: 'These are the packages Kelmor installs on Ubuntu and publishes as account tools. Enable writes the nginx tool vhost through the agent.'}</p>
			{showAccountPicker ? <label>Account
				<select value={accountId} onChange={(event) => setAccountId(event.target.value)}>
					{accounts.map((account) => <option key={account.id} value={account.id}>{account.username} · {account.primary_domain}</option>)}
				</select>
			</label> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			<QueuedOpNotice message={message} jobId={jobId} />
			{loading ? <LoadingState label="Loading host apps…" /> : null}
			{!loading ? <div className="table-wrap"><table className="dense-table">
				<thead><tr><th>App</th><th>Kind</th><th>Status</th><th>Actions</th></tr></thead>
				<tbody>
					{visible.map((app) => (
						<tr key={app.id}>
							<td><strong>{app.label}</strong><small>{app.description}</small></td>
							<td>{app.kind}</td>
							<td><StatusBadge value={app.status} /></td>
							<td><HostAppActionCell
								app={app}
								busy={busy}
								canWrite={canWrite}
								phpmyadminHref={phpmyadminHref}
								webmailHref={webmailHref}
								onEnable={enable}
								onHostAction={runHostAction}
							/></td>
						</tr>
					))}
				</tbody>
			</table></div> : null}
			{!loading && !visible.length ? <EmptyState title="No host apps" detail="The API did not return a host application catalog." /> : null}
		</section>
	)
}

interface HostAppActionCellProps {
	app: HostApp
	busy: string
	canWrite: boolean
	phpmyadminHref?: string
	webmailHref?: string
	onEnable: (app: HostApp) => void
	onHostAction: (app: HostApp, action: HostAppAction) => void
}

function HostAppActionCell ({
	app, busy, canWrite, phpmyadminHref, webmailHref, onEnable, onHostAction,
}: HostAppActionCellProps) {
	const catalog = app.actions || []
	const extras = marketExtras(app, canWrite, phpmyadminHref, webmailHref, onEnable)
	if (!catalog.length && !extras.length) {
		return <span className="subtle">No host-backed action is available for this app yet.</span>
	}
	return (
		<div className="row-actions">
			{catalog.map((action) => (
				<CatalogAction
					key={action.id}
					app={app}
					action={action}
					busy={busy === `${app.id}:${action.id}`}
					onHostAction={onHostAction}
				/>
			))}
			{extras}
		</div>
	)
}

function CatalogAction ({
	app, action, busy, onHostAction,
}: {
	app: HostApp
	action: HostAppAction
	busy: boolean
	onHostAction: (app: HostApp, action: HostAppAction) => void
}) {
	const kind = normalizeActionKind(action)
	switch (kind) {
		case 'href':
			if (action.available && action.href) return <ActionHref href={action.href} label={action.label} />
			return <span className="subtle">{action.reason || action.label}</span>
		case 'job':
			if (action.available) {
				return (
					<button type="button" className="link-button" disabled={busy} onClick={() => onHostAction(app, action)}>
						{busy ? `${action.label}…` : action.label}
					</button>
				)
			}
			return <span className="subtle">{action.reason || `${action.label} unavailable`}</span>
		case 'disabled':
			return <span className="subtle">{action.reason || action.label}</span>
		default: {
			const _exhaustive: never = kind
			return _exhaustive
		}
	}
}

function normalizeActionKind (action: HostAppAction): 'job' | 'href' | 'disabled' {
	if (action.kind === 'href' || action.kind === 'job' || action.kind === 'disabled') return action.kind
	if (action.href) return 'href'
	if (action.available) return 'job'
	return 'disabled'
}

function ActionHref ({ href, label }: { href: string; label: string }) {
	if (href.startsWith('/') && !href.startsWith('//')) return <Link to={href}>{label}</Link>
	return <a href={href} target="_blank" rel="noopener noreferrer">{label}</a>
}

function marketExtras (
	app: HostApp,
	canWrite: boolean,
	phpmyadminHref: string | undefined,
	webmailHref: string | undefined,
	onEnable: (app: HostApp) => void,
) {
	const extras: ReactElement[] = []
	if (canWrite && (app.id === 'phpmyadmin' || app.id === 'roundcube')) {
		extras.push(<button key="publish" type="button" className="link-button" onClick={() => onEnable(app)}>Enable / publish</button>)
	}
	if (app.id === 'phpmyadmin' && phpmyadminHref) {
		extras.push(<a key="open-pma" href={phpmyadminHref} target="_blank" rel="noopener noreferrer">Open phpMyAdmin</a>)
	}
	if (app.id === 'roundcube' && webmailHref) {
		extras.push(<a key="open-webmail" href={webmailHref} target="_blank" rel="noopener noreferrer">Open webmail</a>)
	}
	if (app.id === 'wordpress') extras.push(<Link key="wp" to="/tools/wp-toolkit">WP Toolkit</Link>)
	return extras
}

export function PHPRuntimePanel () {
	const canWrite = useCan('server.settings.write')
	const [items, setItems] = useState<Array<{ version: string; status: string }>>([])
	const [message, setMessage] = useState('')
	const [busy, setBusy] = useState('')

	function load () {
		api<{ items?: Array<{ version: string; status: string }> }>('/api/v1/server/runtimes').then((result) => {
			setItems(asList(result))
		}).catch((requestError) => setMessage(messageFrom(requestError)))
	}

	useEffect(load, [])

	async function ensure (version: string) {
		setBusy(version)
		setMessage('')
		try {
			await api('/api/v1/server/runtimes', { method: 'POST', body: JSON.stringify({ version }) })
			setMessage(`PHP ${version} install queued on this host.`)
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusy('')
		}
	}

	return (
		<section className="panel">
			<h2>PHP runtimes</h2>
			<p>Kelmor installs php-fpm packages through the typed agent, then MultiPHP Manager assigns a version to each site.</p>
			<div className="table-wrap"><table className="dense-table">
				<thead><tr><th>Version</th><th>Status</th><th>Actions</th></tr></thead>
				<tbody>
					{items.map((runtime) => (
						<tr key={runtime.version}>
							<td>PHP {runtime.version}</td>
							<td><StatusBadge value={runtime.status} /></td>
							<td>{canWrite && runtime.status !== 'installed' ? <button type="button" className="link-button" disabled={Boolean(busy)} onClick={() => ensure(runtime.version)}>{busy === runtime.version ? 'Installing…' : 'Install'}</button> : 'Ready'}</td>
						</tr>
					))}
				</tbody>
			</table></div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
		</section>
	)
}
