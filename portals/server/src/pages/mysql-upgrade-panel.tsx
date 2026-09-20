import { FormEvent, useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface MySQLUpgradeStatus {
	installed?: boolean
	engine?: string
	version?: string
	major?: string
	upgrade_tool?: string
	targets?: string[]
	message?: string
}

export function MySQLUpgradePanel () {
	const canWrite = useCan('server.settings.write')
	const [status, setStatus] = useState<MySQLUpgradeStatus>({})
	const [target, setTarget] = useState('10.11')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const targets = status.targets?.length ? status.targets : ['10.11', '11.4']

	function load () {
		setLoading(true)
		setError('')
		api<MySQLUpgradeStatus>('/api/v1/server/mysql-upgrade')
			.then((result) => {
				setStatus(result)
				const nextTargets = result.targets?.length ? result.targets : ['10.11', '11.4']
				if (result.major && nextTargets.includes(result.major)) setTarget(result.major)
			})
			.catch((reason) => setError(messageFrom(reason)))
			.finally(() => setLoading(false))
	}

	useEffect(load, [])

	async function handleUpgrade (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/mysql-upgrade', {
				method: 'POST',
				body: JSON.stringify({ target }),
			})
			setMessage(queuedOpMessage(result, 'MariaDB/MySQL upgrade queued.'))
			setJobId(result.operation_id || '')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	const engineLabel = status.engine === 'mysql' ? 'MySQL' : status.engine === 'mariadb' ? 'MariaDB' : 'not detected'

	return (
		<section className="panel">
			<h2>MariaDB / MySQL upgrade</h2>
			<p>This queues a typed Agent job. Ubuntu 24.04 ships MariaDB. If the selected major version is not in this host’s apt sources, the job fails honestly instead of inventing a repository.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading SQL engine status…" /> : null}
			{!loading && !status.installed ? (
				<EmptyState title="MariaDB/MySQL is not installed" detail={status.message || 'The Agent did not find mariadbd or mysqld on this host.'} />
			) : null}
			{!loading && status.installed ? (
				<dl className="detail-list">
					<div><dt>Engine</dt><dd>{engineLabel}</dd></div>
					<div><dt>Version</dt><dd>{status.version || status.major || 'unknown'}</dd></div>
					<div><dt>Upgrade helper</dt><dd><code>{status.upgrade_tool || 'missing'}</code></dd></div>
					<div><dt>Status</dt><dd><StatusBadge value="installed" /></dd></div>
				</dl>
			) : null}
			{canWrite ? (
				<form className="form-grid" onSubmit={handleUpgrade}>
					<label>Target MariaDB version
						<select value={target} onChange={(event) => setTarget(event.target.value)}>
							{targets.map((entry) => <option key={entry} value={entry}>{entry}</option>)}
						</select>
					</label>
					<button type="submit" disabled={busy}>{busy ? 'Queueing…' : 'Queue upgrade'}</button>
				</form>
			) : null}
			{status.message && status.installed ? <p className="subtle">{status.message}</p> : null}
		</section>
	)
}
