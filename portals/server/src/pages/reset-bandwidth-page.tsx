import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { ErrorState, LoadingState, PageHeader } from '../components/ui'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { formatBytes, messageFrom } from '../helpers'
import type { Account, Usage } from '../types'

export function ResetBandwidthPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [filter, setFilter] = useState('')
	const [usage, setUsage] = useState<Usage | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const accountId = params.get('account') || ''

	function loadAccounts () {
		setLoading(true)
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			const next = asList(result)
			setAccounts(next)
			if (!accountId && next[0]) setParams({ account: next[0].id }, { replace: true })
		}).catch((reason) => setError(messageFrom(reason))).finally(() => setLoading(false))
	}
	useEffect(loadAccounts, [])

	useEffect(() => {
		if (!accountId) {
			setUsage(null)
			return
		}
		api<Usage>(`/api/v1/accounts/${accountId}/usage`).then(setUsage).catch(() => setUsage(null))
	}, [accountId])

	async function handleReset () {
		if (!accountId) return
		setBusy(true)
		setError('')
		setMessage('')
		try {
			const result = await api<{ operation_id: string }>(`/api/v1/accounts/${accountId}/bandwidth/reset`, { method: 'POST', body: '{}' })
			setJobId(result.operation_id)
			setMessage(queuedOpMessage(result, 'Bandwidth counter reset queued.'))
			setUsage(await api<Usage>(`/api/v1/accounts/${accountId}/usage`))
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			<PageHeader
				title="Reset Account Bandwidth Limit"
				description="Zero the observed monthly transfer counter and queue the Agent reset for this account. Package limits stay in place."
			/>
			<p className="subtle">
				<Link to="/usage">Account Usage</Link> · <Link to="/accounts/suspension">Suspension</Link> · <Link to="/tools/unsuspend-bandwidth">Release bandwidth holds</Link>
			</p>
			{error ? <ErrorState error={error} onRetry={loadAccounts} /> : null}
			{message ? <QueuedOpNotice message={message} accountId={accountId} jobId={jobId} /> : null}
			{loading ? <LoadingState label="Loading accounts…" /> : (
				<section className="panel">
					<AccountPicker accounts={accounts} value={accountId} onChange={(id) => setParams({ account: id })} filter={filter} onFilterChange={setFilter} />
					{usage ? (
						<p>Current transfer: <strong>{formatBytes(usage.bandwidth_bytes)}</strong>{usage.bandwidth_hold ? ' · held for bandwidth' : ''}</p>
					) : null}
					<button type="button" onClick={handleReset} disabled={!accountId || busy}>Reset bandwidth usage</button>
				</section>
			)}
		</>
	)
}
