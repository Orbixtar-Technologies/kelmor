import { useState } from 'react'
import { Link } from 'react-router-dom'
import { listOf, post } from '../client'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { AccountPicker, type AccountRow } from '../components/account-picker'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, Field, Notice, PageHeader, Panel, Pill } from '../components/ui'
import { formatDateTime } from '../lib/format'

interface CertRow {
	id: string
	account_id: string
	hostname: string
	kind: string
	status: string
	not_after?: string
	issuer?: string
}

interface Row extends CertRow {
	username: string
}

/** Days until expiry, or null when the certificate has no recorded end date. */
function daysLeft (notAfter?: string) {
	if (!notAfter) return null
	const end = new Date(notAfter).getTime()
	if (Number.isNaN(end)) return null
	return Math.round((end - Date.now()) / 86400000)
}

export function Certificates () {
	const toast = useToast()
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts'), [])
	const list = accounts.data ?? []
	const key = list.map((a) => a.id).join(',')
	const [accountId, setAccountId] = useState('')
	const [hostname, setHostname] = useState('')
	const [busy, setBusy] = useState(false)

	const certs = useLoad<Row[]>(async () => {
		const perAccount = await Promise.all(
			list.map(async (account) => {
				const rows = await listOf<CertRow>(`/api/v1/accounts/${account.id}/certificates`).catch(() => [])
				return rows.map((row) => ({ ...row, username: account.username }))
			}),
		)
		return perAccount.flat()
	}, [key])

	const rows = certs.data ?? []
	const expiring = rows.filter((cert) => {
		const days = daysLeft(cert.not_after)
		return days !== null && days <= 14
	})
	const selectedAccount = list.find((a) => a.id === accountId)

	const columns: Column<Row>[] = [
		{ key: 'hostname', header: 'Hostname', sort: (c) => c.hostname, render: (c) => <span className="mono"><strong>{c.hostname}</strong></span> },
		{ key: 'account', header: 'Account', sort: (c) => c.username, render: (c) => <Link to={`/accounts/${c.account_id}?tab=ssl`}>{c.username}</Link> },
		{ key: 'kind', header: 'Kind', sort: (c) => c.kind, render: (c) => <Pill tone="idle">{c.kind}</Pill> },
		{ key: 'issuer', header: 'Issuer', sort: (c) => c.issuer || '', render: (c) => c.issuer || <span className="muted">—</span> },
		{
			key: 'expiry',
			header: 'Expires',
			sort: (c) => c.not_after || '',
			render: (c) => {
				const days = daysLeft(c.not_after)
				if (days === null) return <span className="muted">—</span>
				return (
					<span>
						{formatDateTime(c.not_after)}{' '}
						{days < 0 ? <Pill tone="bad">expired</Pill> : days <= 14 ? <Pill tone="warn">{days}d left</Pill> : <Pill tone="ok">{days}d left</Pill>}
					</span>
				)
			},
		},
		{ key: 'status', header: 'Status', sort: (c) => c.status, render: (c) => <Pill tone={c.status === 'issued' ? 'ok' : c.status === 'failed' ? 'bad' : 'busy'}>{c.status}</Pill> },
	]

	return (
		<>
			<PageHeader
				title="Certificates"
				description="TLS certificates Kelmor holds for hosted domains, with their issuer and remaining lifetime. Renewals are queued as jobs before expiry."
				favoritePath="/ssl"
			/>

			{expiring.length > 0 ? (
				<Notice tone="warn">
					{expiring.length} certificate{expiring.length === 1 ? '' : 's'} expire within 14 days:{' '}
					{expiring.map((c) => c.hostname).join(', ')}. Confirm each hostname still resolves to this host so the HTTP-01
					challenge can complete.
				</Notice>
			) : null}

			<Panel title="Request a certificate" icon="lock">
				<AccountPicker
					accounts={list}
					value={accountId}
					onChange={(id) => {
						setAccountId(id)
						setHostname(list.find((a) => a.id === id)?.primary_domain ?? '')
					}}
					loading={accounts.loading}
				/>
				<div className="form-grid" style={{ marginTop: 14 }}>
					<Field label="Hostname" hint="Must resolve to this host on port 80 for the ACME HTTP-01 challenge.">
						<input value={hostname} onChange={(e) => setHostname(e.target.value.toLowerCase().trim())} spellCheck={false} placeholder="example.com" />
					</Field>
					<div className="field" style={{ alignSelf: 'end' }}>
						<button
							type="button"
							className="btn"
							disabled={!accountId || !hostname || busy}
							onClick={async () => {
								setBusy(true)
								await toast.run(
									() => post(`/api/v1/accounts/${accountId}/certificates`, { hostname, kind: 'acme' }),
									() => `ACME order queued for ${hostname}`,
								)
								await certs.reload()
								setBusy(false)
							}}
						>
							{busy ? 'Queuing…' : 'Request certificate'}
						</button>
					</div>
				</div>
				{selectedAccount ? (
					<p className="small muted" style={{ marginTop: 10 }}>
						The order runs as a job against {selectedAccount.username}. Follow it on the{' '}
						<Link to="/jobs">Job Queue</Link>.
					</p>
				) : null}
			</Panel>

			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(c) => c.id}
				loading={accounts.loading || certs.loading}
				error={certs.error}
				searchPlaceholder="Search hostname, account or issuer"
				noun="certificates"
				initialSort={{ key: 'expiry', dir: 'asc' }}
				empty={
					<EmptyState icon="lock" title="No certificate has been issued yet">
						Request one above. Kelmor keeps the ACME challenge location in the vhost even when an account is over its
						transfer limit, so renewals do not break.
					</EmptyState>
				}
			/>
		</>
	)
}
