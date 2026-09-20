import { FormEvent, useEffect, useState } from 'react'
import { api, asList } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface ServiceCertificate {
	id: string
	label?: string
	kind?: string
	hostname?: string
	ports?: number[]
	cert_path?: string
	key_path?: string
	status?: string
	subject?: string
	issuer?: string
	not_after?: string
	names?: string[]
	note?: string
	has_key?: boolean
	openssl_ok?: boolean
}

export function ServiceSSLPanel () {
	const canWrite = useCan('server.settings.write')
	const [items, setItems] = useState<ServiceCertificate[]>([])
	const [hostname, setHostname] = useState('')
	const [selectedId, setSelectedId] = useState('director')
	const [certPem, setCertPem] = useState('')
	const [keyPem, setKeyPem] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const [note, setNote] = useState('')

	function load () {
		setLoading(true)
		setError('')
		api<{ items?: ServiceCertificate[]; hostname?: string; note?: string }>('/api/v1/server/ssl/service')
			.then((result) => {
				const next = asList(result)
				setItems(next)
				setHostname(result.hostname || '')
				setNote(result.note || '')
				if (next[0] && !next.some((row) => row.id === selectedId)) setSelectedId(next[0].id)
			})
			.catch((requestError) => {
				setError(messageFrom(requestError))
				setItems([])
			})
			.finally(() => setLoading(false))
	}
	useEffect(load, [])

	async function handleInstall (event: FormEvent) {
		event.preventDefault()
		if (!canWrite) return
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/ssl/service', {
				method: 'POST',
				body: JSON.stringify({
					service: selectedId,
					hostname,
					cert_pem: certPem,
					key_pem: keyPem,
				}),
			})
			setMessage(queuedOpMessage(result, 'Service certificate install queued on the host.'))
			setJobId(result.operation_id || '')
			setCertPem('')
			setKeyPem('')
			load()
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}

	const selected = items.find((row) => row.id === selectedId)

	return (
		<section className="panel">
			<h2>Service certificates</h2>
			<p>Director, Control, and mail use host TLS files under <code>/var/lib/panel/certs</code>. This tool reads those files and installs a replacement pair through the Agent. Account AutoSSL stays on the other SSL tabs.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading service certificates…" /> : null}
			{!loading ? (
				<div className="table-wrap"><table className="dense-table" aria-label="Service certificate inventory">
					<thead>
						<tr>
							<th>Select</th>
							<th>Service</th>
							<th>Hostname</th>
							<th>Ports</th>
							<th>Status</th>
							<th>Expiry</th>
							<th>Issuer</th>
						</tr>
					</thead>
					<tbody>
						{items.map((row) => (
							<tr key={row.id}>
								<td>
									<input
										type="radio"
										name="service-cert"
										checked={selectedId === row.id}
										aria-label={`Select ${row.label || row.id}`}
										onChange={() => setSelectedId(row.id)}
									/>
								</td>
								<td><strong>{row.label || row.id}</strong><small>{row.cert_path}</small></td>
								<td>{row.hostname || '—'}</td>
								<td>{(row.ports || []).join(', ') || '—'}</td>
								<td>
									<StatusBadge value={row.status} />
									{row.note ? <small>{row.note}</small> : null}
								</td>
								<td>{row.not_after ? formatDate(row.not_after) : '—'}</td>
								<td>{row.issuer || '—'}</td>
							</tr>
						))}
					</tbody>
				</table></div>
			) : null}
			{!loading && !items.length ? (
				<EmptyState
					title="No service certificate slots reported"
					detail={note || 'The Agent could not read host TLS material. Missing OpenSSL files are not invented.'}
				/>
			) : null}
			<form className="form-grid" onSubmit={handleInstall}>
				<label>Service hostname
					<input
						value={hostname}
						onChange={(event) => setHostname(event.target.value)}
						placeholder="panel hostname"
					/>
				</label>
				<label>Certificate PEM
					<textarea
						value={certPem}
						onChange={(event) => setCertPem(event.target.value)}
						rows={6}
						aria-label="Certificate PEM"
						required
					/>
				</label>
				<label>Private key PEM
					<textarea
						value={keyPem}
						onChange={(event) => setKeyPem(event.target.value)}
						rows={6}
						aria-label="Private key PEM"
						required
					/>
				</label>
				<div className="page-actions">
					<button type="submit" disabled={!canWrite || busy || !certPem.trim() || !keyPem.trim()}>
						{busy ? 'Installing…' : selected?.status === 'installed' ? 'Replace on host' : 'Install on host'}
					</button>
				</div>
				{!canWrite ? <p className="subtle">Your role can review host TLS inventory. Installing requires server.settings.write.</p> : null}
				<p className="subtle">PEM is applied to the selected service files only. Account site certificates are not changed.</p>
			</form>
		</section>
	)
}
