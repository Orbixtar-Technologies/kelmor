import { FormEvent, useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { ErrorState } from '../components/ui'
import { HOST_SETTINGS_BANNER } from '../catalog-honesty'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface ClusterSnapshot {
	published_at?: string
	note?: string
	packages?: unknown[]
	feature_sets?: unknown[]
}

export function ClusterPanel () {
	const canWrite = useCan('server.settings.write')
	const [peers, setPeers] = useState('')
	const [snapshot, setSnapshot] = useState<ClusterSnapshot | null>(null)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		api<{ values?: Record<string, Record<string, string>> }>('/api/v1/server/settings')
			.then((result) => setPeers(result.values?.configuration_cluster?.peers || ''))
			.catch(() => undefined)
		api<ClusterSnapshot>('/api/v1/server/cluster/snapshot')
			.then(setSnapshot)
			.catch(() => setSnapshot(null))
	}

	useEffect(load, [])

	async function handlePeers (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/settings', {
				method: 'PATCH',
				body: JSON.stringify({ values: { configuration_cluster: { peers } } }),
			})
			setMessage(queuedOpMessage(result, 'Cluster membership queued for the host.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	async function handlePublish () {
		setBusy(true)
		setError('')
		setMessage('')
		try {
			const result = await api<{ snapshot?: ClusterSnapshot }>('/api/v1/server/cluster/publish', {
				method: 'POST',
				body: '{}',
			})
			setSnapshot(result.snapshot || result)
			setMessage('Package and feature-set snapshot written on the host.')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<h2>Configuration cluster</h2>
			<p className="settings-local-banner host-applied" role="status">{HOST_SETTINGS_BANNER}</p>
			<p>Peer URLs are written to <code>/etc/panel/cluster.json</code>. Publishing writes a package and feature-set snapshot for peer Directors. Kelmor does not live-replicate every host setting across nodes.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} /> : null}
			<form className="form-grid" onSubmit={handlePeers}>
				<label>Peer Director URLs
					<textarea value={peers} onChange={(event) => setPeers(event.target.value)} rows={4} />
				</label>
				<button type="submit" disabled={!canWrite || busy}>{busy ? 'Saving…' : 'Apply peers on host'}</button>
			</form>
			<button type="button" disabled={!canWrite || busy} onClick={() => { void handlePublish() }}>
				Publish package snapshot
			</button>
			{snapshot?.note ? <p className="subtle">{snapshot.note}</p> : null}
			{snapshot?.published_at ? <p>Last snapshot: {snapshot.published_at} · {snapshot.packages?.length || 0} packages · {snapshot.feature_sets?.length || 0} feature sets</p> : null}
		</section>
	)
}
