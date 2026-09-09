import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, post, query } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { DataTable, type Column } from '../components/data-table'
import { ConfirmDialog, EmptyState, Meter, Notice, PageHeader, Pill, StatusPill } from '../components/ui'
import { formatBytes, meterClass, usedPercent } from '../lib/format'
import type { AccountRow } from '../components/account-picker'

interface UsageRow {
	disk_bytes: number
	bandwidth_bytes: number
	inode_count: number
}

interface PackageRow {
	id: string
	name: string
	disk_bytes: number
	bandwidth_bytes_monthly: number
}

interface AccountsResponse {
	items: AccountRow[] | null
	total: number
	usage: Record<string, UsageRow> | null
}

export type AccountListVariant = 'all' | 'suspended' | 'over-quota'

const copy: Record<AccountListVariant, { title: string; description: string; noun: string; emptyTitle: string; emptyBody: string }> = {
	all: {
		title: 'List Accounts',
		description: 'Every hosting account on this server with its package, measured consumption and reconciliation status.',
		noun: 'accounts',
		emptyTitle: 'No hosting accounts yet',
		emptyBody: 'Create the first account and Kelmor provisions the Linux identity, document root, DNS zone and mail routing.',
	},
	suspended: {
		title: 'List Suspended Accounts',
		description: 'Accounts held out of service. Sites answer with a suspension page and mail delivery stops until they are restored.',
		noun: 'suspended accounts',
		emptyTitle: 'No accounts are suspended',
		emptyBody: 'Every hosting account on this server is currently in service.',
	},
	'over-quota': {
		title: 'Accounts Over Quota',
		description: 'Accounts whose measured disk or monthly transfer has reached the limit set by their package.',
		noun: 'accounts over quota',
		emptyTitle: 'No account is over quota',
		emptyBody: 'Every account is inside the disk and monthly transfer limits set by its package.',
	},
}

export function AccountList ({ variant }: { variant: AccountListVariant }) {
	const navigate = useNavigate()
	const toast = useToast()
	const canSuspend = useCan('accounts.suspend')
	const canTerminate = useCan('accounts.terminate')
	const canModify = useCan('accounts.modify')
	const canCreate = useCan('accounts.create')
	const [selected, setSelected] = useState<string[]>([])
	const [confirmSuspend, setConfirmSuspend] = useState<AccountRow | null>(null)
	const [busy, setBusy] = useState(false)

	const url = `/api/v1/accounts${query({
		status: variant === 'suspended' ? 'suspended' : undefined,
		over_quota: variant === 'over-quota' ? 'true' : undefined,
	})}`

	const accounts = useLoad<AccountsResponse>(() => api<AccountsResponse>(url), [url])
	const packages = useLoad<{ items: PackageRow[] | null }>(() => api('/api/v1/packages'), [])

	const rows = accounts.data?.items ?? []
	const usage = accounts.data?.usage ?? {}
	const packageById = new Map((packages.data?.items ?? []).map((p) => [p.id, p]))
	const text = copy[variant]

	async function act (path: string, message: string) {
		setBusy(true)
		await toast.run(() => post(path), () => message)
		await accounts.reload()
		setBusy(false)
	}

	async function bulkSuspend () {
		setBusy(true)
		await toast.run(
			() => post('/api/v1/accounts/bulk/suspend', { ids: selected }),
			() => `Suspension queued for ${selected.length} accounts`,
		)
		setSelected([])
		await accounts.reload()
		setBusy(false)
	}

	const columns: Column<AccountRow>[] = [
		{
			key: 'username',
			header: 'User',
			sort: (a) => a.username,
			render: (a) => <Link to={`/accounts/${a.id}`}><strong>{a.username}</strong></Link>,
		},
		{
			key: 'domain',
			header: 'Primary domain',
			sort: (a) => a.primary_domain,
			render: (a) => <span className="mono">{a.primary_domain}</span>,
		},
		{
			key: 'package',
			header: 'Package',
			sort: (a) => packageById.get(a.package_id || '')?.name ?? '',
			render: (a) => {
				const pkg = packageById.get(a.package_id || '')
				return pkg ? <Link to={`/packages/${pkg.id}`}>{pkg.name}</Link> : <span className="muted">unassigned</span>
			},
		},
		{
			key: 'disk',
			header: 'Disk',
			align: 'right',
			sort: (a) => usage[a.id]?.disk_bytes ?? 0,
			render: (a) => <QuotaCell used={usage[a.id]?.disk_bytes ?? 0} limit={packageById.get(a.package_id || '')?.disk_bytes ?? 0} />,
		},
		{
			key: 'bandwidth',
			header: 'Transfer (month)',
			align: 'right',
			sort: (a) => usage[a.id]?.bandwidth_bytes ?? 0,
			render: (a) => <QuotaCell used={usage[a.id]?.bandwidth_bytes ?? 0} limit={packageById.get(a.package_id || '')?.bandwidth_bytes_monthly ?? 0} />,
		},
		{
			key: 'ip',
			header: 'IP address',
			sort: (a) => a.ip_address || '',
			render: (a) => <span className="mono">{a.ip_address || 'shared'}</span>,
		},
		{
			key: 'status',
			header: 'Status',
			sort: (a) => a.status,
			render: (a) => <StatusPill status={a.status} />,
		},
	]

	return (
		<>
			<PageHeader
				title={text.title}
				description={text.description}
				favoritePath={variant === 'all' ? '/accounts' : variant === 'suspended' ? '/accounts/suspended' : '/accounts/over-quota'}
				actions={
					<>
						<button type="button" className="btn secondary" onClick={() => accounts.reload()}>Refresh</button>
						{canCreate ? <Link className="btn" to="/accounts/create">Create a new account</Link> : null}
					</>
				}
			/>

			{variant === 'over-quota' && rows.length > 0 ? (
				<Notice tone="warn">
					These accounts have reached a package limit. Kelmor rewrites the vhost to HTTP 509 once monthly transfer is
					exhausted, and rejects over-quota writes on disk. Raise the package or move the account to a larger one.
				</Notice>
			) : null}

			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(a) => a.id}
				loading={accounts.loading}
				error={accounts.error}
				searchPlaceholder="Search username, domain, package or IP"
				noun={text.noun}
				initialSort={{ key: 'username', dir: 'asc' }}
				selectable={canSuspend && variant !== 'suspended'}
				selected={selected}
				onSelectedChange={setSelected}
				actions={
					selected.length > 0 ? (
						<button type="button" className="btn secondary" disabled={busy} onClick={bulkSuspend}>
							Suspend {selected.length} selected
						</button>
					) : null
				}
				empty={
					<EmptyState
						icon="users"
						title={text.emptyTitle}
						action={
							variant === 'all' && canCreate ? (
								<Link className="btn" to="/accounts/create">Create a new account</Link>
							) : (
								<Link className="btn secondary" to="/accounts">Go to List Accounts</Link>
							)
						}
					>
						{text.emptyBody}
					</EmptyState>
				}
				rowActions={[
					{ label: 'Manage account', onSelect: (a) => navigate(`/accounts/${a.id}`) },
					{ label: 'Modify account', onSelect: () => navigate('/accounts/modify'), hidden: () => !canModify },
					{ label: 'Change package', onSelect: () => navigate('/accounts/change-package'), hidden: () => !canModify },
					{ label: 'DNS zone', onSelect: (a) => navigate(`/accounts/${a.id}?tab=dns`) },
					{ label: 'Email', onSelect: (a) => navigate(`/accounts/${a.id}?tab=email`) },
					{ label: 'Databases', onSelect: (a) => navigate(`/accounts/${a.id}?tab=databases`) },
					{ label: 'SSL certificates', onSelect: (a) => navigate(`/accounts/${a.id}?tab=ssl`) },
					{
						label: 'Suspend',
						onSelect: (a) => setConfirmSuspend(a),
						hidden: (a) => !canSuspend || a.status === 'suspended',
					},
					{
						label: 'Unsuspend',
						onSelect: (a) => act(`/api/v1/accounts/${a.id}/unsuspend`, `${a.username} restored`),
						hidden: (a) => !canSuspend || a.status !== 'suspended',
					},
					{
						label: 'Terminate',
						danger: true,
						onSelect: () => navigate('/accounts/terminate'),
						hidden: () => !canTerminate,
					},
				]}
			/>

			{confirmSuspend ? (
				<ConfirmDialog
					title={`Suspend ${confirmSuspend.username}?`}
					confirmLabel="Suspend account"
					busy={busy}
					onCancel={() => setConfirmSuspend(null)}
					onConfirm={async () => {
						const target = confirmSuspend
						setConfirmSuspend(null)
						await act(`/api/v1/accounts/${target.id}/suspend`, `${target.username} suspended`)
					}}
				>
					<p>
						Websites for <strong>{confirmSuspend.primary_domain}</strong> stop serving and mail delivery is held. Files,
						databases and mailboxes are kept, so the account can be restored at any time.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

function QuotaCell ({ used, limit }: { used: number; limit: number }) {
	const percent = usedPercent(used, limit)
	return (
		<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center', justifyContent: 'flex-end' }}>
			{limit > 0 ? <Meter used={used} limit={limit} className={meterClass(percent)} /> : null}
			<span className="nowrap">
				{formatBytes(used)}
				{limit > 0 ? <span className="muted"> / {formatBytes(limit)}</span> : <span className="muted"> / ∞</span>}
			</span>
			{percent >= 100 ? <Pill tone="bad">full</Pill> : null}
		</span>
	)
}
