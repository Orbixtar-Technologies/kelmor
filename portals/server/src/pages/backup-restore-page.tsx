import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface RestoreArchive {
	id: string
	account_id?: string
	account_username?: string
	primary_domain?: string
	kind: string
	state: string
	destination: string
	checksum?: string
	size_bytes?: number
	created_at?: string
	scope?: string
	restorable?: boolean
	restore_note?: string
}

export function BackupRestorePage () {
	const canRestore = useCan('backups.restore')
	const [archives, setArchives] = useState<RestoreArchive[]>([])
	const [selectedId, setSelectedId] = useState('')
	const [accountFilter, setAccountFilter] = useState('')
	const [step, setStep] = useState(0)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		setLoading(true)
		setError('')
		api<{ items?: RestoreArchive[] }>('/api/v1/backups').then((result) => {
			setArchives(asList(result))
			setSelectedId('')
			setStep(0)
		}).catch((requestError) => {
			setError(messageFrom(requestError))
			setArchives([])
		}).finally(() => setLoading(false))
	}
	useEffect(load, [])

	const visible = useMemo(() => {
		const query = accountFilter.trim().toLowerCase()
		if (!query) return archives
		return archives.filter((archive) => {
			const haystack = [
				archive.account_username, archive.primary_domain, archive.id,
				archive.kind, archive.scope, archive.destination,
			].join(' ').toLowerCase()
			return haystack.includes(query)
		})
	}, [accountFilter, archives])
	const selected = useMemo(
		() => archives.find((archive) => archive.id === selectedId) || null,
		[archives, selectedId],
	)
	const isConfirm = step === 1

	async function handleSubmit (event: FormEvent) {
		event.preventDefault()
		if (!selected) return
		if (!isConfirm) {
			setStep(1)
			return
		}
		if (!canRestore || !selected.account_id || !selected.restorable) return
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${selected.account_id}/restores`, {
				method: 'POST',
				body: JSON.stringify({ backup_id: selected.id, mode: 'in_place' }),
			})
			setMessage(queuedOpMessage(result, 'Restore queued.'))
			setJobId(result.operation_id || '')
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			<PageHeader
				title="Backup Restoration"
				description="Select a restorable archive from host inventory and queue an in-place restore job."
			/>
			<p className="subtle">
				<Link to="/transfers">Transfers & Backups</Link> · <Link to="/jobs?q=backup">Backup queue</Link>
			</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading restorable backups…" /> : null}
			{!loading ? (
				<form className="form-panel" onSubmit={handleSubmit}>
					<ol className="steps" aria-label="Workflow">
						{['Review archives', 'Review restore'].map((label, index) => (
							<li key={label} className={index === step ? 'active' : index < step ? 'complete' : ''}>
								<span>{index + 1}</span>{label}
							</li>
						))}
					</ol>
					<div className="form-section-heading">
						<h2>{isConfirm ? 'Review restore' : 'Restorable archives'}</h2>
						<p>{isConfirm
							? `Director will queue an in-place restore of ${selected?.id} onto ${selected?.account_username || 'the selected account'}. Current account files may be replaced.`
							: 'Choose one succeeded account archive. Queued, failed, and system archives stay listed but cannot be restored in place.'}</p>
					</div>
					{!isConfirm ? (
						<>
							<label>Filter inventory
								<input
									value={accountFilter}
									onChange={(event) => setAccountFilter(event.target.value)}
									placeholder="Username, domain, or archive id"
								/>
							</label>
							<div className="table-wrap"><table className="dense-table">
								<thead>
									<tr>
										<th>Select</th>
										<th>Archive</th>
										<th>Account</th>
										<th>Kind</th>
										<th>State</th>
										<th>Size</th>
										<th>Created</th>
									</tr>
								</thead>
								<tbody>
									{visible.map((archive) => {
										const canSelect = Boolean(archive.restorable && archive.account_id)
										const label = `Select ${archive.id} for ${archive.account_username || archive.scope || 'archive'}`
										return (
											<tr key={archive.id}>
												<td>
													<input
														type="radio"
														name="restore-archive"
														checked={selectedId === archive.id}
														disabled={!canSelect}
														aria-label={label}
														onChange={() => setSelectedId(archive.id)}
													/>
												</td>
												<td><strong>{archive.id}</strong><small>{archive.destination} · {archive.scope || 'account'}</small></td>
												<td>{archive.account_username || 'System'}{archive.primary_domain ? <small>{archive.primary_domain}</small> : null}</td>
												<td>{archive.kind}</td>
												<td><StatusBadge value={archive.state} />{archive.restore_note ? <small>{archive.restore_note}</small> : null}</td>
												<td>{formatBytes(archive.size_bytes)}</td>
												<td>{formatDate(archive.created_at)}</td>
											</tr>
										)
									})}
								</tbody>
							</table></div>
							{!archives.length ? (
								<EmptyState
									title="No backups available to restore"
									detail="Director has no account or system archives in inventory yet. The restore workflow stays here — configure a destination and queue a backup, then return to select an archive."
									action={<Link className="secondary" to="/transfers">Configure or run backups</Link>}
								/>
							) : null}
							{archives.length && !visible.length ? (
								<EmptyState
									title="No archives match this filter"
									detail="Clear the filter to see the full host inventory."
								/>
							) : null}
						</>
					) : selected ? (
						<dl className="detail-list">
							<div><dt>Archive</dt><dd>{selected.id}</dd></div>
							<div><dt>Account</dt><dd>{selected.account_username || selected.account_id}</dd></div>
							<div><dt>Destination</dt><dd>{selected.destination}</dd></div>
							<div><dt>Checksum</dt><dd>{selected.checksum || '—'}</dd></div>
						</dl>
					) : null}
					<div className="page-actions">
						{isConfirm ? <button type="button" className="secondary" onClick={() => setStep(0)}>Back</button> : null}
						<button type="submit" disabled={busy || !selected || (isConfirm && !canRestore)}>
							{busy ? 'Working…' : isConfirm ? 'Queue restore' : 'Continue'}
						</button>
						{!isConfirm ? <Link className="secondary" to="/transfers">Configure backups</Link> : null}
					</div>
					{isConfirm && !canRestore ? <p className="subtle">Your role can review this inventory, but the API will refuse the restore.</p> : null}
				</form>
			) : null}
		</>
	)
}
