import { useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { ErrorState } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface RemoteAccessStatus {
	applied?: boolean
	revoked?: boolean
	prefix?: string
	created_at?: string
	host_path?: string
}

interface IssuedKey extends RemoteAccessStatus {
	key?: string
	operation_id?: string
}

export function RemoteAccessPanel () {
	const canWrite = useCan('api_tokens.write')
	const canRead = useCan('api_tokens.read') || canWrite
	const [status, setStatus] = useState<RemoteAccessStatus | null>(null)
	const [issued, setIssued] = useState<IssuedKey | null>(null)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		if (!canRead) return
		api<RemoteAccessStatus>('/api/v1/server/remote-access-key')
			.then(setStatus)
			.catch((reason) => setError(messageFrom(reason)))
	}

	useEffect(load, [canRead])

	async function handleIssue () {
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<IssuedKey>('/api/v1/server/remote-access-key', {
				method: 'POST',
				body: '{}',
			})
			setIssued(result)
			setStatus({
				applied: true,
				revoked: false,
				prefix: result.prefix,
				created_at: result.created_at,
				host_path: result.host_path,
			})
			setMessage(queuedOpMessage(result, 'Remote access key applied on the host. Copy it now; it is not shown again.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	async function handleRevoke () {
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/remote-access-key', { method: 'DELETE' })
			setIssued(null)
			setStatus({ applied: false, revoked: true, host_path: status?.host_path })
			setMessage(queuedOpMessage(result, 'Remote access key revoked on the host.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<p>Kelmor writes a hashed remote access key to <code>/etc/panel/remote-access-key</code>. Present the plaintext as a Bearer token to authenticate Director API calls with server-administrator capabilities.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} /> : null}
			<dl className="detail-list">
				<div><dt>Host file</dt><dd><code>{status?.host_path || '/etc/panel/remote-access-key'}</code></dd></div>
				<div><dt>Status</dt><dd>{status?.applied ? 'Applied on host' : status?.revoked ? 'Revoked' : 'Not issued'}</dd></div>
				<div><dt>Prefix</dt><dd>{status?.prefix || '—'}</dd></div>
				<div><dt>Created</dt><dd>{status?.created_at || '—'}</dd></div>
			</dl>
			{issued?.key ? (
				<p role="status">New key (copy now): <code>{issued.key}</code></p>
			) : null}
			<div className="row-actions">
				<button type="button" disabled={!canWrite || busy} onClick={() => { void handleIssue() }}>
					{status?.applied ? 'Regenerate key' : 'Issue key'}
				</button>
				<button type="button" className="danger-text" disabled={!canWrite || busy || !status?.applied} onClick={() => { void handleRevoke() }}>
					Revoke key
				</button>
			</div>
		</section>
	)
}
