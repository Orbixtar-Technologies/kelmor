import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState } from '../components/ui'
import { dedicatedPath } from '../dedicated-tool-routes'
import { downloadJSON, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface ClusterCapabilities {
	peer_membership?: boolean
	snapshot_publish?: boolean
	snapshot_export?: boolean
	snapshot_import?: boolean
	peer_health_probe?: boolean
	live_multi_node?: boolean
	linked_peers?: number
}

interface ClusterSnapshot {
	published_at?: string
	note?: string
	packages?: unknown[]
	feature_sets?: unknown[]
	capabilities?: ClusterCapabilities
}

export const CLUSTER_NO_PEERS_TITLE = 'No linked nodes'
export const CLUSTER_NO_PEERS_DETAIL = 'Apply to peers needs at least one linked node. Register a peer Director URL in Link Server Nodes. Director does not invent remote nodes.'
export const CLUSTER_LINK_NODES_LABEL = 'Open Link Server Nodes'

export function linkedPeerCount (capabilities: ClusterCapabilities): number {
	const raw = capabilities.linked_peers
	if (typeof raw === 'number' && Number.isFinite(raw) && raw >= 0)
		return Math.floor(raw)
	return capabilities.live_multi_node ? 1 : 0
}

export function linkedNodesStatus ({
	linkedPeers,
	canApplyRemote,
}: {
	linkedPeers: number
	canApplyRemote: boolean
}): string {
	const countLabel = linkedPeers === 1 ? '1 linked node' : `${linkedPeers} linked nodes`
	if (canApplyRemote) return `${countLabel} — Apply to peers is available.`
	return `${countLabel} — Apply to peers needs at least one linked node.`
}

function capabilityAvailable (
	capabilities: ClusterCapabilities,
	key: string,
	fallback: boolean,
): boolean {
	if (key === 'linked_peers') return fallback
	const value = capabilities[key as keyof ClusterCapabilities]
	return typeof value === 'boolean' ? value : fallback
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
	{ key: 'live_multi_node', label: 'Live multi-node apply to linked nodes', available: false },
]

export function ClusterPanel () {
	const canWrite = useCan('server.settings.write')
	const [peers, setPeers] = useState('')
	const [snapshot, setSnapshot] = useState<ClusterSnapshot | null>(null)
	const [probeRows, setProbeRows] = useState<ProbeRow[]>([])
	const [importText, setImportText] = useState('')
	const [applyToken, setApplyToken] = useState('')
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const [snapshotLoaded, setSnapshotLoaded] = useState(false)

	function load () {
		api<{ values?: Record<string, Record<string, string>> }>('/api/v1/server/settings')
			.then((result) => setPeers(result.values?.configuration_cluster?.peers || ''))
			.catch(() => undefined)
		api<ClusterSnapshot>('/api/v1/server/cluster/snapshot')
			.then(setSnapshot)
			.catch(() => setSnapshot(null))
			.finally(() => setSnapshotLoaded(true))
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
			load()
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

	async function handleApply () {
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ items?: ProbeRow[]; operation_id?: string }>('/api/v1/server/cluster/apply', {
				method: 'POST',
				body: JSON.stringify(applyToken.trim() ? { token: applyToken.trim() } : {}),
			})
			setProbeRows(result.items || [])
			setMessage(queuedOpMessage(result, 'Multi-node snapshot apply queued for linked Directors.'))
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
	const canApplyRemote = Boolean(capabilities.live_multi_node)
	const linkedPeers = linkedPeerCount(capabilities)
	const nodesStatus = linkedNodesStatus({ linkedPeers, canApplyRemote })

	return (
		<section className="panel">
			<h2>Configuration cluster</h2>
			<p>{canApplyRemote
				? 'This node writes peer URLs and package snapshots on the host. Linked Directors can receive the current snapshot through multi-node apply.'
				: 'This node writes peer URLs and package snapshots on the host. Multi-node apply stays unavailable until at least one linked node or cluster peer is registered.'}</p>
			<p aria-live="polite">{snapshotLoaded ? nodesStatus : 'Checking linked nodes…'}</p>
			{snapshotLoaded && !canApplyRemote ? (
				<div id="cluster-no-peers-help">
					<EmptyState
						title={CLUSTER_NO_PEERS_TITLE}
						detail={CLUSTER_NO_PEERS_DETAIL}
						action={<Link className="button-link" to={dedicatedPath('link-nodes')}>{CLUSTER_LINK_NODES_LABEL}</Link>}
					/>
				</div>
			) : null}
			<table className="dense-table" aria-label="Cluster capability matrix">
				<thead><tr><th>Capability</th><th>This Ubuntu stack</th></tr></thead>
				<tbody>
					{CAPABILITY_ROWS.map((row) => {
						const available = capabilityAvailable(capabilities, row.key, row.available)
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
			<label>Peer API token
				<input
					type="password"
					value={applyToken}
					onChange={(event) => setApplyToken(event.target.value)}
					autoComplete="off"
					placeholder="Optional Bearer token on the peer Director"
				/>
			</label>
			<div className="row-actions">
				<button type="button" disabled={!canWrite || busy} onClick={() => { void handlePublish() }}>
					Publish package snapshot
				</button>
				<button type="button" disabled={!canWrite || busy} onClick={() => { void handleProbe() }}>
					Probe peer health
				</button>
				<button
					type="button"
					disabled={!canWrite || busy || !canApplyRemote}
					aria-describedby={snapshotLoaded && !canApplyRemote ? 'cluster-no-peers-help' : undefined}
					title={snapshotLoaded && !canApplyRemote ? CLUSTER_NO_PEERS_DETAIL : undefined}
					onClick={() => { void handleApply() }}
				>
					Apply to linked nodes
				</button>
				<button type="button" disabled={!snapshot} onClick={() => downloadJSON('kelmor-cluster-snapshot.json', snapshot)}>
					Export snapshot
				</button>
			</div>
			{snapshotLoaded && !canApplyRemote ? <p className="subtle">Saving peer URLs on this page also enables Apply after the host records them. Director does not invent remote nodes.</p> : null}
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
