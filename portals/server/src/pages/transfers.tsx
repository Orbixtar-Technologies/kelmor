import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, listOf, post, query } from '../client'
import { useCan } from '../rbac'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { AccountPicker, type AccountRow } from '../components/account-picker'
import { DataTable, type Column } from '../components/data-table'
import { ConfirmDialog, EmptyState, Field, Loading, Notice, PageHeader, Panel, StatusPill } from '../components/ui'
import { formatBytes, formatDateTime } from '../lib/format'

interface BackupRow {
	id: string
	account_id: string
	kind: string
	state: string
	destination: string
	checksum?: string
	size_bytes: number
	created_at: string
}

function useAccounts () {
	return useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts'), [])
}

/** Fans out the per-account backup endpoint so Director can show one server-wide list. */
function useAllBackups (accounts: AccountRow[]) {
	const key = accounts.map((a) => a.id).join(',')
	return useLoad<Array<BackupRow & { username: string }>>(
		async () => {
			const lists = await Promise.all(
				accounts.map(async (account) => {
					const runs = await listOf<BackupRow>(`/api/v1/accounts/${account.id}/backups`).catch(() => [])
					return runs.map((run) => ({ ...run, username: account.username }))
				}),
			)
			return lists.flat().sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
		},
		[key],
		accounts.length > 0 ? 6000 : 0,
	)
}

export function AccountBackups () {
	const toast = useToast()
	const canCreate = useCan('backups.create')
	const accounts = useAccounts()
	const list = accounts.data ?? []
	const backups = useAllBackups(list)
	const [accountId, setAccountId] = useState('')
	const [destination, setDestination] = useState('local')
	const [busy, setBusy] = useState(false)

	const columns: Column<BackupRow & { username: string }>[] = [
		{ key: 'created', header: 'Started', sort: (b) => b.created_at, render: (b) => formatDateTime(b.created_at) },
		{ key: 'account', header: 'Account', sort: (b) => b.username, render: (b) => <Link to={`/accounts/${b.account_id}?tab=backups`}>{b.username}</Link> },
		{ key: 'kind', header: 'Kind', sort: (b) => b.kind },
		{ key: 'destination', header: 'Destination', sort: (b) => b.destination },
		{ key: 'size', header: 'Size', align: 'right', sort: (b) => b.size_bytes, render: (b) => (b.size_bytes ? formatBytes(b.size_bytes) : '—') },
		{ key: 'checksum', header: 'Checksum', sort: (b) => b.checksum || '', render: (b) => <span className="mono small">{b.checksum ? b.checksum.slice(0, 12) : '—'}</span> },
		{ key: 'state', header: 'State', sort: (b) => b.state, render: (b) => <StatusPill status={b.state} /> },
	]

	return (
		<>
			<PageHeader
				title="Account Backups"
				description="Encrypted HPM1 archives covering the home tree, MariaDB and PostgreSQL dumps and mailbox Maildirs. Queue a run here and follow it to completion."
				favoritePath="/transfers/backups"
				actions={<Link className="btn secondary" to="/transfers/restore">Restore a backup</Link>}
			/>

			{canCreate ? (
				<Panel title="Queue a backup" icon="archive">
					<AccountPicker accounts={list} value={accountId} onChange={setAccountId} loading={accounts.loading} />
					<div className="form-grid" style={{ marginTop: 14 }}>
						<Field label="Destination" hint="Offsite destinations need their credentials configured on the host.">
							<select value={destination} onChange={(e) => setDestination(e.target.value)}>
								<option value="local">Local disk</option>
								<option value="sftp">Offsite SFTP</option>
								<option value="s3">S3-compatible object store</option>
							</select>
						</Field>
						<div className="field" style={{ alignSelf: 'end' }}>
							<button
								type="button"
								className="btn"
								disabled={!accountId || busy}
								onClick={async () => {
									setBusy(true)
									await toast.run(
										() => post(`/api/v1/accounts/${accountId}/backups`, { kind: 'full', destination }),
										() => `Backup queued to ${destination}`,
									)
									await backups.reload()
									setBusy(false)
								}}
							>
								Queue encrypted backup
							</button>
						</div>
					</div>
				</Panel>
			) : (
				<Notice tone="info">Your role can review backup runs but not queue new ones.</Notice>
			)}

			<DataTable
				rows={backups.data ?? []}
				columns={columns}
				rowKey={(b) => b.id}
				loading={backups.loading || accounts.loading}
				error={backups.error}
				searchPlaceholder="Search account, destination or state"
				noun="backup runs"
				initialSort={{ key: 'created', dir: 'desc' }}
				empty={
					<EmptyState icon="archive" title="No backup has run on this server">
						Queue one above. Completed runs can be restored in place from Restore a Backup.
					</EmptyState>
				}
			/>
		</>
	)
}

export function BackupRestore () {
	const toast = useToast()
	const accounts = useAccounts()
	const list = accounts.data ?? []
	const [accountId, setAccountId] = useState('')
	const backups = useLoad<BackupRow[]>(
		() => (accountId ? listOf<BackupRow>(`/api/v1/accounts/${accountId}/backups`) : Promise.resolve([])),
		[accountId],
	)
	const [pending, setPending] = useState<BackupRow | null>(null)
	const [busy, setBusy] = useState(false)

	const restorable = (backups.data ?? []).filter((b) => b.state === 'succeeded')
	const account = list.find((a) => a.id === accountId)

	return (
		<>
			<PageHeader
				title="Restore a Backup"
				description="Restores the home tree, database contents and mailboxes for one account from a completed backup run."
				favoritePath="/transfers/restore"
				actions={<Link className="btn secondary" to="/transfers/backups">Account backups</Link>}
			/>
			<Notice tone="warn">
				An in-place restore overwrites current data with the contents of the chosen run. Anything written since that
				backup finished is lost. Take a fresh backup first if you are unsure.
			</Notice>
			<Panel title="Choose an account" icon="users">
				<AccountPicker accounts={list} value={accountId} onChange={setAccountId} loading={accounts.loading} />
			</Panel>
			<Panel title="Completed backup runs" icon="refresh" subtitle={account ? account.username : undefined} tight>
				{!accountId ? (
					<EmptyState icon="archive" title="Pick an account first">Its completed backup runs are listed here, newest first.</EmptyState>
				) : backups.loading ? (
					<Loading />
				) : restorable.length === 0 ? (
					<EmptyState icon="archive" title="No completed backup for this account" action={<Link className="btn" to="/transfers/backups">Queue a backup</Link>}>
						Only runs in the succeeded state can be restored.
					</EmptyState>
				) : (
					<div className="table-wrap">
						<table className="data">
							<thead><tr><th>Started</th><th>Kind</th><th>Destination</th><th className="num">Size</th><th>Checksum</th><th /></tr></thead>
							<tbody>
								{restorable.map((backup) => (
									<tr key={backup.id}>
										<td>{formatDateTime(backup.created_at)}</td>
										<td>{backup.kind}</td>
										<td>{backup.destination}</td>
										<td className="num">{backup.size_bytes ? formatBytes(backup.size_bytes) : '—'}</td>
										<td className="mono small">{backup.checksum ? backup.checksum.slice(0, 12) : '—'}</td>
										<td className="right">
											<button type="button" className="btn secondary small" onClick={() => setPending(backup)}>Restore in place</button>
										</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</Panel>

			{pending && account ? (
				<ConfirmDialog
					title={`Restore ${account.username} in place?`}
					confirmLabel="Restore in place"
					typeToConfirm="RESTORE"
					busy={busy}
					onCancel={() => setPending(null)}
					onConfirm={async () => {
						const backup = pending
						setPending(null)
						setBusy(true)
						await toast.run(
							() => post(`/api/v1/accounts/${account.id}/restores`, { backup_id: backup.id, mode: 'in_place' }),
							() => 'Restore queued — follow it on the job queue',
						)
						await backups.reload()
						setBusy(false)
					}}
				>
					<p>
						The current home tree, databases and mailboxes for <strong>{account.primary_domain}</strong> are replaced with
						the {formatDateTime(pending.created_at)} {pending.destination} backup.
					</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

export function TransferMigrate () {
	const toast = useToast()
	const accounts = useAccounts()
	const list = accounts.data ?? []
	const [accountId, setAccountId] = useState('')
	const [username, setUsername] = useState('')
	const [domain, setDomain] = useState('')
	const [busy, setBusy] = useState(false)

	const source = list.find((a) => a.id === accountId)

	return (
		<>
			<PageHeader
				title="Transfer or Migrate an Account"
				description="Copies an existing account onto a new username and primary domain on this server, leaving the original untouched."
				favoritePath="/transfers/migrate"
			/>
			<Notice tone="info">
				This is the rename and re-home path: useful when a customer changes domain, or when splitting a shared account.
				To bring an account in from another host, use <Link to="/transfers/import">Import an Account</Link> instead.
			</Notice>
			<Panel title="Source account" icon="users">
				<AccountPicker accounts={list} value={accountId} onChange={setAccountId} loading={accounts.loading} />
			</Panel>
			<Panel title="Destination" icon="externalLink">
				{!source ? (
					<EmptyState icon="externalLink" title="Pick the account to migrate">The destination form unlocks once a source account is chosen.</EmptyState>
				) : (
					<>
						<div className="form-grid">
							<Field label="New username" hint="Must be free. Becomes the new Linux identity.">
								<input value={username} onChange={(e) => setUsername(e.target.value.toLowerCase().trim())} spellCheck={false} />
							</Field>
							<Field label="New primary domain" hint="Must not already be assigned on this server.">
								<input value={domain} onChange={(e) => setDomain(e.target.value.toLowerCase().trim())} spellCheck={false} placeholder="example.com" />
							</Field>
						</div>
						<div className="form-actions">
							<button
								type="button"
								className="btn"
								disabled={busy || !username || !domain}
								onClick={async () => {
									setBusy(true)
									await toast.run(
										() => post<{ account?: { username?: string }; resource_id?: string }>(`/api/v1/accounts/${source.id}/migrate`, { username, domain }),
										(r) => `Migrated to ${r.account?.username || r.resource_id}`,
									)
									await accounts.reload()
									setBusy(false)
								}}
							>
								{busy ? 'Migrating…' : 'Migrate account'}
							</button>
							<span className="small muted">{source.username} → {username || 'new username'}</span>
						</div>
					</>
				)}
			</Panel>
		</>
	)
}

export function TransferImport () {
	const toast = useToast()
	const [busy, setBusy] = useState(false)

	return (
		<>
			<PageHeader
				title="Import an Account"
				description="Bring an account onto this server from a Kelmor export or from an extracted cPanel cpmove tree."
				favoritePath="/transfers/import"
			/>
			<div className="grid-2">
				<Panel title="Kelmor native export" icon="upload">
					<p className="small muted" style={{ marginBottom: 12 }}>
						Restores an export produced by <Link to="/transfers/export">Export Accounts</Link> on any Kelmor host. The
						importer refuses colliding usernames and domains, then queues a reconcile job.
					</p>
					<form
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							const file = fd.get('export') as File
							setBusy(true)
							await toast.run(
								async () => {
									const raw = await file.text()
									const suffix = query({ username: String(fd.get('username') || ''), domain: String(fd.get('domain') || '') })
									return api<{ account?: { username?: string }; resource_id?: string }>(`/api/v1/accounts/import${suffix}`, {
										method: 'POST',
										body: raw,
									})
								},
								(r) => `Imported ${r.account?.username || r.resource_id}`,
							)
							setBusy(false)
						}}
					>
						<div style={{ display: 'grid', gap: 14 }}>
							<Field label="Export file" hint="The .hpm-account.json file downloaded from a Kelmor host.">
								<input name="export" type="file" accept="application/json" required />
							</Field>
							<Field label="Override username" hint="Optional. Leave blank to keep the exported username.">
								<input name="username" spellCheck={false} />
							</Field>
							<Field label="Override primary domain" hint="Optional. Useful when staging a copy alongside the original.">
								<input name="domain" spellCheck={false} />
							</Field>
						</div>
						<div className="form-actions">
							<button type="submit" className="btn" disabled={busy}>{busy ? 'Importing…' : 'Import native export'}</button>
						</div>
					</form>
				</Panel>

				<Panel title="cPanel cpmove archive" icon="inbox">
					<p className="small muted" style={{ marginBottom: 12 }}>
						Point Kelmor at an already-extracted <code>cpmove-user</code> directory on this host. Kelmor reads the account
						metadata, home tree, databases and mail from that tree.
					</p>
					<Notice tone="info">
						Extract the archive on the server first, for example{' '}
						<span className="mono">tar -xzf cpmove-user.tar.gz -C /var/tmp</span>. Director does not upload multi-gigabyte
						archives through the browser.
					</Notice>
					<form
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							setBusy(true)
							await toast.run(
								() => post<{ account?: { username?: string }; resource_id?: string }>('/api/v1/accounts/import/cpanel', {
									root: fd.get('root'),
									username: fd.get('username'),
								}),
								(r) => `cPanel import queued for ${r.account?.username || r.resource_id}`,
							)
							setBusy(false)
						}}
					>
						<div style={{ display: 'grid', gap: 14 }}>
							<Field label="Extracted path on this host">
								<input name="root" placeholder="/var/tmp/cpmove-user" required spellCheck={false} />
							</Field>
							<Field label="Target username" hint="The username this account takes on Kelmor.">
								<input name="username" required spellCheck={false} />
							</Field>
						</div>
						<div className="form-actions">
							<button type="submit" className="btn" disabled={busy}>{busy ? 'Queuing…' : 'Import cpmove tree'}</button>
						</div>
					</form>
				</Panel>
			</div>
		</>
	)
}

export function TransferExport () {
	const toast = useToast()
	const accounts = useAccounts()
	const list = accounts.data ?? []
	const [busy, setBusy] = useState('')

	async function download (account: AccountRow) {
		setBusy(account.id)
		await toast.run(
			async () => {
				const exported = await api<unknown>(`/api/v1/accounts/${account.id}/export`)
				const blob = new Blob([JSON.stringify(exported, null, 2)], { type: 'application/json' })
				const url = URL.createObjectURL(blob)
				const link = document.createElement('a')
				link.href = url
				link.download = `${account.username}.hpm-account.json`
				link.click()
				URL.revokeObjectURL(url)
				return account
			},
			(a) => `Export downloaded for ${a.username}`,
		)
		setBusy('')
	}

	const columns: Column<AccountRow>[] = [
		{ key: 'username', header: 'User', sort: (a) => a.username, render: (a) => <Link to={`/accounts/${a.id}`}>{a.username}</Link> },
		{ key: 'domain', header: 'Primary domain', sort: (a) => a.primary_domain, className: 'mono' },
		{ key: 'status', header: 'Status', sort: (a) => a.status, render: (a) => <StatusPill status={a.status} /> },
		{
			key: 'export',
			header: '',
			sortable: false,
			align: 'right',
			render: (a) => (
				<button type="button" className="btn secondary small" disabled={busy === a.id} onClick={() => download(a)}>
					{busy === a.id ? 'Preparing…' : 'Download export'}
				</button>
			),
		},
	]

	return (
		<>
			<PageHeader
				title="Export Accounts"
				description="A portable JSON description of an account: domains, websites, DNS records, mail routing and databases. Import it on any Kelmor host."
				favoritePath="/transfers/export"
				actions={<Link className="btn secondary" to="/transfers/import">Import an account</Link>}
			/>
			<Notice tone="info">
				An export describes the account, it is not a data backup. For file, database and mailbox contents use an encrypted
				run from <Link to="/transfers/backups">Account Backups</Link>.
			</Notice>
			<DataTable
				rows={list}
				columns={columns}
				rowKey={(a) => a.id}
				loading={accounts.loading}
				error={accounts.error}
				searchPlaceholder="Search username or domain"
				noun="accounts"
				initialSort={{ key: 'username', dir: 'asc' }}
			/>
		</>
	)
}
