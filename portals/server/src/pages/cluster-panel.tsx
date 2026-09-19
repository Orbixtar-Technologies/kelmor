import { FormEvent, useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { ErrorState } from '../components/ui'
import { downloadJSON, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface ClusterSnapshot {
	published_at?: string
	note?: string
	packages?: unknown[]
	feature_sets?: unknown[]
	capabilities?: Record<string, boolean>
}

interface ProbeRow {
	url?: string
	ok?: boolean
	status?: number
	error?: string
	latency_ms?: number
}

const CAPABILITY_ROWS: Array<{ key: string; label: string; available: boolean }> = [
	{ key: 'peer_membership', label: 'Save peer URLs to /etc/panel/cluster.json', available: true },
	{ key: 'snapshot_publish', label: 'Queue a host publish of packages and feature sets', available: true },
	{ key: 'snapshot_export', label: 'Export the current snapshot JSON', available: true },
	{ key: 'snapshot_import', label: 'Import a snapshot and write it on the host', available: true },
	{ key: 'peer_health_probe', label: 'Probe a peer Director /healthz', available: true },
	{ key: 'live_multi_node', label: 'Live multi-node apply to all nodes', available: false },
]

export function ClusterPanel () {
	const canWrite = useCan('server.settings.write')
	const [peers, setPeers] = useState('')
	const [snapshot, setSnapshot] = useState<ClusterSnapshot | null>(null)
	const [probeRows, setProbeRows] = useState<ProbeRow[]>([])
	const [importText, setImportText] = useState('')
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
		setJobId('')
		try {
			const result = await api<{ snapshot?: ClusterSnapshot; operation_id?: string }>('/api/v1/server/cluster/publish', {
				method: 'POST',
				body: '{}',
			})
			setSnapshot(result.snapshot ?? snapshot)
			setMessage(queuedOpMessage(result, 'Package snapshot publish queued on the host.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	async function handleImport (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const parsed = JSON.parse(importText) as unknown
			const result = await api<{ operation_id?: string }>('/api/v1/server/cluster/snapshot/import', {
				method: 'POST',
				body: JSON.stringify({ snapshot: parsed }),
			})
			setMessage(queuedOpMessage(result, 'Snapshot import queued on the host.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	async function handleProbe () {
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const first = peers.split(/\s+/).map((part) => part.trim()).find(Boolean)
			const result = await api<{ items?: ProbeRow[]; operation_id?: string }>('/api/v1/server/cluster/probe', {
				method: 'POST',
				body: JSON.stringify(first ? { url: first } : {}),
			})
			setProbeRows(result.items || [])
			setMessage(queuedOpMessage(result, 'Peer health probe queued on the host.'))
			setJobId(result.operation_id || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	const capabilities = snapshot?.capabilities || {}

	return (
		<section className="panel">
			<h2>Configuration cluster</h2>
			<p>This node writes peer URLs and package snapshots on the host. It does not live-replicate every setting to other Directors.</p>
			<table className="dense-table" aria-label="Cluster capability matrix">
				<thead><tr><th>Capability</th><th>This Ubuntu stack</th></tr></thead>
				<tbody>
					{CAPABILITY_ROWS.map((row) => {
						const available = capabilities[row.key] ?? row.available
						return (
							<tr key={row.key}>
								<td>{row.label}</td>
								<td>{available ? 'Available' : 'Unavailable'}</td>
							</tr>
						)
					})}
				</tbody>
			</table>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} /> : null}
			<form className="form-grid" onSubmit={handlePeers}>
				<label>Peer Director URLs
					<textarea value={peers} onChange={(event) => setPeers(event.target.value)} rows={4} />
				</label>
				<button type="submit" disabled={!canWrite || busy}>{busy ? 'Saving…' : 'Apply peers on host'}</button>
			</form>
			<div className="row-actions">
				<button type="button" disabled={!canWrite || busy} onClick={() => { void handlePublish() }}>
					Publish package snapshot
				</button>
				<button type="button" disabled={!canWrite || busy} onClick={() => { void handleProbe() }}>
					Probe peer health
				</button>
				<button type="button" disabled={!snapshot} onClick={() => downloadJSON('kelmor-cluster-snapshot.json', snapshot)}>
					Export snapshot
				</button>
			</div>
			{probeRows.length ? (
				<table className="dense-table" aria-label="Peer health">
					<thead><tr><th>Peer</th><th>Result</th><th>Detail</th></tr></thead>
					<tbody>
						{probeRows.map((row) => (
							<tr key={row.url || 'peer'}>
								<td>{row.url}</td>
								<td>{row.ok ? 'Reachable' : 'Unreachable'}</td>
								<td>{row.error || (row.status ? `HTTP ${row.status}` : '')}{row.latency_ms != null ? ` · ${row.latency_ms}ms` : ''}</td>
							</tr>
						))}
					</tbody>
				</table>
			) : null}
			<form className="form-grid" onSubmit={handleImport}>
				<label>Import snapshot JSON
					<textarea value={importText} onChange={(event) => setImportText(event.target.value)} rows={6} aria-label="Import snapshot JSON" />
				</label>
				<button type="submit" disabled={!canWrite || busy || !importText.trim()}>Import snapshot on host</button>
			</form>
			{snapshot?.note ? <p className="subtle">{snapshot.note}</p> : null}
			{snapshot?.published_at ? <p>Last snapshot: {snapshot.published_at} · {snapshot.packages?.length || 0} packages · {snapshot.feature_sets?.length || 0} feature sets</p> : null}
		</section>
	)
}
