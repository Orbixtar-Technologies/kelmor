import { FormEvent, useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface PostgresStatus {
	installed?: boolean
	version?: string
	cluster?: string
	listen?: string
	auth?: string
	message?: string
}

interface ServerSettings {
	values?: Record<string, Record<string, string>>
}

const AUTH_OPTIONS = [
	{ value: 'scram-sha-256', label: 'scram-sha-256' },
	{ value: 'md5', label: 'md5' },
	{ value: 'peer', label: 'peer (unix socket; TCP uses scram-sha-256)' },
]

export function PostgresConfigPanel () {
	const canWrite = useCan('server.settings.write')
	const [status, setStatus] = useState<PostgresStatus>({})
	const [listen, setListen] = useState('127.0.0.1')
	const [auth, setAuth] = useState('scram-sha-256')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<PostgresStatus>('/api/v1/server/postgres'),
			api<ServerSettings>('/api/v1/server/settings'),
		]).then(([hostResult, settingsResult]) => {
			if (hostResult.status === 'fulfilled') {
				setStatus(hostResult.value)
				if (hostResult.value.listen) setListen(hostResult.value.listen)
				if (hostResult.value.auth) setAuth(hostResult.value.auth)
			} else {
				setError(messageFrom(hostResult.reason))
			}
			if (settingsResult.status === 'fulfilled') {
				const stored = settingsResult.value.values?.postgres || {}
				if (stored.listen) setListen(stored.listen)
				if (stored.auth) setAuth(stored.auth)
			}
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])

	async function handleSave (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/settings', {
				method: 'PATCH',
				body: JSON.stringify({ values: { postgres: { listen: listen.trim(), auth } } }),
			})
			setMessage(queuedOpMessage(result, 'PostgreSQL host apply queued.'))
			setJobId(result.operation_id || '')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<h2>PostgreSQL host configuration</h2>
			<p>Save queues a host apply job. The Agent writes listen_addresses and TCP pg_hba methods. Unix-socket peer stays in place so the control plane can still connect.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading PostgreSQL status…" /> : null}
			{!loading && !status.installed ? (
				<EmptyState title="PostgreSQL is not installed" detail={status.message || 'The Agent did not find a PostgreSQL cluster on this host.'} />
			) : null}
			{!loading && status.installed ? (
				<dl className="detail-list">
					<div><dt>Cluster</dt><dd><code>{status.cluster || 'unknown'}</code></dd></div>
					<div><dt>Version</dt><dd>{status.version || 'unknown'}</dd></div>
					<div><dt>Status</dt><dd><StatusBadge value="installed" /></dd></div>
				</dl>
			) : null}
			{canWrite ? (
				<form className="form-grid" onSubmit={handleSave}>
					<label>Listen address
						<input value={listen} onChange={(event) => setListen(event.target.value)} required />
					</label>
					<label>TCP auth method
						<select value={auth} onChange={(event) => setAuth(event.target.value)}>
							{AUTH_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
						</select>
					</label>
					<button type="submit" disabled={busy}>{busy ? 'Queueing…' : 'Apply on host'}</button>
				</form>
			) : null}
		</section>
	)
}
