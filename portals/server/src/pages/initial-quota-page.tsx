import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface QuotaStatus {
	kernel_quota?: boolean
	setquota?: boolean
	quota_unavailable?: string
	homes_present?: boolean
	policy_bytes?: number
	enforce?: boolean
	applied?: boolean
	host_path?: string
}

export function InitialQuotaPage () {
	const canWrite = useCan('server.settings.write')
	const [status, setStatus] = useState<QuotaStatus | null>(null)
	const [diskMB, setDiskMB] = useState(1024)
	const [enforce, setEnforce] = useState(true)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')

	function load () {
		setLoading(true)
		api<QuotaStatus>('/api/v1/server/quota').then((result) => {
			setStatus(result)
			if (result.policy_bytes) setDiskMB(Math.round(result.policy_bytes / 1024 / 1024))
			if (typeof result.enforce === 'boolean') setEnforce(result.enforce)
		}).catch((reason) => setError(messageFrom(reason))).finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function handleSetup (event: FormEvent) {
		event.preventDefault()
		setError('')
		try {
			const result = await api<{ operation_id: string }>('/api/v1/server/quota/setup', {
				method: 'POST',
				body: JSON.stringify({ default_disk_mb: diskMB, enforce }),
			})
			setJobId(result.operation_id)
			setMessage(queuedOpMessage(result, 'Initial quota policy queued.'))
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	return (
		<>
			<PageHeader
				title="Initial Quota Setup"
				description="Probe host quota tooling and apply the default disk policy written to /etc/panel/initial-quota. Per-account caps still come from the package."
			/>
			<p className="subtle"><Link to="/packages">Edit packages</Link> · <Link to="/usage">Account Usage</Link></p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <QueuedOpNotice message={message} jobId={jobId} /> : null}
			{loading ? <LoadingState label="Probing quota status…" /> : (
				<section className="panel">
					<p>Kernel quota: <strong>{status?.kernel_quota ? 'available' : 'unavailable'}</strong></p>
					<p>setquota: <strong>{status?.setquota ? 'present' : 'missing'}</strong></p>
					<p>Homes volume: <strong>{status?.homes_present ? 'present' : 'not mounted'}</strong></p>
					<p>Applied policy: <strong>{status?.applied ? formatBytes(status.policy_bytes) : 'none'}</strong> · enforce {status?.enforce ? 'on' : 'off'}</p>
					{status?.quota_unavailable ? <p>Host note: {status.quota_unavailable}</p> : null}
					<p>Host path: <code>{status?.host_path || '/etc/panel/initial-quota'}</code></p>
				</section>
			)}
			{canWrite ? (
				<form className="panel" onSubmit={handleSetup}>
					<label>
						Default disk MB for new accounts
						<input type="number" min={0} value={diskMB} onChange={(event) => setDiskMB(Number(event.target.value))} />
					</label>
					<label>
						<input type="checkbox" checked={enforce} onChange={(event) => setEnforce(event.target.checked)} />
						Enforce with setquota when available
					</label>
					<button type="submit">Apply on host</button>
				</form>
			) : null}
		</>
	)
}
