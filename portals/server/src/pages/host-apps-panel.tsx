import { useEffect, useState } from 'react'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { messageFrom, valueOf } from '../helpers'
import { useCan } from '../rbac'
import type { Account } from '../types'

interface HostApp {
	id: string
	label: string
	kind: string
	status: string
	path?: string
	description: string
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

	async function enable (app: HostApp) {
		setMessage('')
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

	return (
		<section className="panel">
			<h2>Host applications</h2>
			<p>These are the packages Kelmor installs on Ubuntu and publishes as account tools. Enable writes the nginx tool vhost through the agent.</p>
			{accounts.length ? <label>Account
				<select value={accountId} onChange={(event) => setAccountId(event.target.value)}>
					{accounts.map((account) => <option key={account.id} value={account.id}>{account.username} · {account.primary_domain}</option>)}
				</select>
			</label> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{loading ? <LoadingState label="Loading host apps…" /> : null}
			{!loading ? <div className="table-wrap"><table className="dense-table">
				<thead><tr><th>App</th><th>Kind</th><th>Status</th><th>Actions</th></tr></thead>
				<tbody>
					{visible.map((app) => (
						<tr key={app.id}>
							<td><strong>{app.label}</strong><small>{app.description}</small></td>
							<td>{app.kind}</td>
							<td><StatusBadge value={app.status} /></td>
							<td><div className="row-actions">
								{canWrite && (app.id === 'phpmyadmin' || app.id === 'roundcube') ? <button type="button" className="link-button" onClick={() => enable(app)}>Enable / publish</button> : null}
								{app.id === 'phpmyadmin' && urls.phpmyadmin_url ? <a href={urls.phpmyadmin_url} target="_blank" rel="noreferrer">Open phpMyAdmin</a> : null}
								{app.id === 'roundcube' && urls.webmail_url ? <a href={urls.webmail_url} target="_blank" rel="noreferrer">Open webmail</a> : null}
								{app.id === 'wordpress' ? <a href="/tools/wp-toolkit">WP Toolkit</a> : null}
							</div></td>
						</tr>
					))}
				</tbody>
			</table></div> : null}
			{!loading && !visible.length ? <EmptyState title="No host apps" detail="The API did not return a host application catalog." /> : null}
		</section>
	)
}

export function PHPRuntimePanel () {
	const canWrite = useCan('server.settings.write')
	const [items, setItems] = useState<Array<{ version: string; status: string }>>([])
	const [message, setMessage] = useState('')

	function load () {
		api<{ items?: Array<{ version: string; status: string }> }>('/api/v1/server/runtimes').then((result) => {
			setItems(asList(result))
		}).catch((requestError) => setMessage(messageFrom(requestError)))
	}

	useEffect(load, [])

	async function ensure (version: string) {
		setMessage('')
		try {
			await api('/api/v1/server/runtimes', { method: 'POST', body: JSON.stringify({ version }) })
			setMessage(`PHP ${version} install queued on this host.`)
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
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
							<td><StatusBadge value={valueOf(runtime, 'status')} /></td>
							<td>{canWrite && runtime.status !== 'installed' ? <button type="button" className="link-button" onClick={() => ensure(runtime.version)}>Install</button> : 'Ready'}</td>
						</tr>
					))}
				</tbody>
			</table></div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
		</section>
	)
}
