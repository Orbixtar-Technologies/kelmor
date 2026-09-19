import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

export interface DnsInventoryZone {
	id: string
	name: string
	account_id?: string
	account_username?: string
	account_status?: string
	domain?: string
	records?: number
	desired_revision?: number
	observed_revision?: number
	reason?: string
}

type DnsInventoryKind = 'cleanup' | 'synchronize'

interface DnsInventoryCopy {
	title: string
	description: string
	listPath: string
	emptyTitle: string
	emptyDetail: string
	reviewStep: string
	confirmStep: string
	confirmLabel: string
	success: string
	columns: [string, string, string]
}

const COPY: Record<DnsInventoryKind, DnsInventoryCopy> = {
	cleanup: {
		title: 'Perform a DNS Cleanup',
		description: 'Remove leftover records for terminated accounts. Review before applying.',
		listPath: '/api/v1/dns/cleanup',
		emptyTitle: 'No leftover zones',
		emptyDetail: 'Every zone on this host is still tied to a live account. Cleanup has nothing to retire.',
		reviewStep: 'Review leftover zones',
		confirmStep: 'Confirm cleanup',
		confirmLabel: 'Queue cleanup',
		success: 'DNS cleanup queued.',
		columns: ['Zone', 'Account', 'Why leftover'],
	},
	synchronize: {
		title: 'Synchronize DNS Records',
		description: 'Re-publish every managed zone from the control plane to PowerDNS.',
		listPath: '/api/v1/dns/synchronize',
		emptyTitle: 'No zones to synchronize',
		emptyDetail: 'There are no managed zones on live accounts to re-publish.',
		reviewStep: 'Review',
		confirmStep: 'Queue sync',
		confirmLabel: 'Queue synchronize',
		success: 'DNS synchronize queued.',
		columns: ['Zone', 'Account', 'Publish state'],
	},
}

export function DnsCleanupPage () {
	return <DnsInventoryToolPage kind="cleanup" />
}

export function DnsSynchronizePage () {
	return <DnsInventoryToolPage kind="synchronize" />
}

function DnsInventoryToolPage ({ kind }: { kind: DnsInventoryKind }) {
	const copy = COPY[kind]
	const canWrite = useCan('dns.write')
	const [zones, setZones] = useState<DnsInventoryZone[]>([])
	const [selected, setSelected] = useState<Record<string, boolean>>({})
	const [step, setStep] = useState(0)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		setLoading(true)
		setError('')
		api<{ items?: DnsInventoryZone[] }>(copy.listPath).then((result) => {
			const next = asList(result)
			setZones(next)
			setSelected({})
			setStep(0)
		}).catch((requestError) => {
			setError(messageFrom(requestError))
			setZones([])
		}).finally(() => setLoading(false))
	}

	useEffect(load, [copy.listPath])

	const selectedZones = useMemo(
		() => zones.filter((zone) => selected[zone.id]),
		[selected, zones],
	)
	const allSelected = zones.length > 0 && selectedZones.length === zones.length
	const isConfirm = step === 1

	function handleToggle (zoneId: string, checked: boolean) {
		setSelected((current) => ({ ...current, [zoneId]: checked }))
	}

	function handleToggleAll (checked: boolean) {
		const next: Record<string, boolean> = {}
		if (checked) {
			for (const zone of zones) next[zone.id] = true
		}
		setSelected(next)
	}

	async function handleSubmit (event: FormEvent) {
		event.preventDefault()
		if (!isConfirm) {
			if (!selectedZones.length) return
			setStep(1)
			return
		}
		if (!canWrite || !selectedZones.length) return
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>(copy.listPath, {
				method: 'POST',
				body: JSON.stringify({ zone_ids: selectedZones.map((zone) => zone.id) }),
			})
			setMessage(queuedOpMessage(result, copy.success))
			setJobId(result.operation_id || '')
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			<PageHeader title={copy.title} description={copy.description} />
			<p className="subtle">DNS Functions · reviewed host job</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading DNS inventory…" /> : null}
			{!loading && !zones.length ? (
				<EmptyState title={copy.emptyTitle} detail={copy.emptyDetail} action={<Link to="/dns">Open DNS Zone Manager</Link>} />
			) : null}
			{!loading && zones.length ? (
				<form className="form-panel" onSubmit={handleSubmit}>
					<ol className="steps" aria-label="Workflow">
						{[copy.reviewStep, copy.confirmStep].map((label, index) => (
							<li key={label} className={index === step ? 'active' : index < step ? 'complete' : ''}>
								<span>{index + 1}</span>{label}
							</li>
						))}
					</ol>
					<div className="form-section-heading">
						<h2>{isConfirm ? copy.confirmStep : copy.reviewStep}</h2>
						<p>{isConfirm
							? `Director will queue a host job for ${selectedZones.length} selected zone(s).`
							: 'Select the zones this job should touch. Confirm only after you have reviewed the list.'}</p>
					</div>
					{!isConfirm ? (
						<div className="table-wrap"><table className="dense-table">
							<thead>
								<tr>
									<th>
										<input
											type="checkbox"
											checked={allSelected}
											aria-label="Select all zones"
											onChange={(event) => handleToggleAll(event.target.checked)}
										/>
									</th>
									<th>{copy.columns[0]}</th>
									<th>{copy.columns[1]}</th>
									<th>{copy.columns[2]}</th>
								</tr>
							</thead>
							<tbody>
								{zones.map((zone) => (
									<tr key={zone.id}>
										<td>
											<input
												type="checkbox"
												checked={Boolean(selected[zone.id])}
												aria-label={`Select ${zone.name}`}
												onChange={(event) => handleToggle(zone.id, event.target.checked)}
											/>
										</td>
										<td>{zone.name}</td>
										<td>{zone.account_username || '—'}{zone.account_status ? <> · <StatusBadge value={zone.account_status} /></> : null}</td>
										<td>{kind === 'cleanup' ? leftoverReason(zone.reason) : syncState(zone)}</td>
									</tr>
								))}
							</tbody>
						</table></div>
					) : (
						<ul className="list-plain">
							{selectedZones.map((zone) => (
								<li key={zone.id}>{zone.account_username ? `${zone.name} · ${zone.account_username}` : zone.name}</li>
							))}
						</ul>
					)}
					<div className="page-actions">
						{isConfirm ? <button type="button" className="secondary" onClick={() => setStep(0)}>Back</button> : null}
						<button
							type="submit"
							disabled={busy || !selectedZones.length || (isConfirm && !canWrite)}
						>
							{busy ? 'Working…' : isConfirm ? copy.confirmLabel : 'Continue'}
						</button>
					</div>
					{isConfirm && !canWrite ? <p className="subtle">Your role can review this inventory, but the API will refuse the write.</p> : null}
				</form>
			) : null}
		</>
	)
}

const leftoverReasons: Record<string, string> = {
	terminated_account: 'Terminated account',
	untied_zone: 'Not tied to an account',
	managed: 'Managed zone',
}

function leftoverReason (reason?: string) {
	if (!reason) return 'Unknown'
	return leftoverReasons[reason] || reason.replaceAll('_', ' ')
}

function syncState (zone: DnsInventoryZone) {
	const desired = zone.desired_revision || 0
	const observed = zone.observed_revision || 0
	if (desired > observed) return 'Needs republish'
	return 'In sync'
}
