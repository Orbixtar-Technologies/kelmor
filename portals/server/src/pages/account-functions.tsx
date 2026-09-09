import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, listOf, patch, post, query } from '../client'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { AccountPicker, type AccountRow } from '../components/account-picker'
import { DataTable, type Column } from '../components/data-table'
import { CheckField, ConfirmDialog, EmptyState, Field, KeyValues, Meter, Notice, PageHeader, Panel, StatusPill } from '../components/ui'
import { ReviewList } from '../components/wizard'
import { formatBytes, meterClass, usedPercent } from '../lib/format'

interface PackageRow {
	id: string
	name: string
	disk_bytes: number
	bandwidth_bytes_monthly: number
	domains: number
	databases: number
	mailboxes: number
	cpu_percent: number
	memory_bytes: number
}

interface UsageRow {
	disk_bytes: number
	bandwidth_bytes: number
	inode_count: number
}

interface AccountsResponse {
	items: AccountRow[] | null
	usage: Record<string, UsageRow> | null
}

function useAccounts (params: Record<string, string> = {}) {
	const url = `/api/v1/accounts${query(params)}`
	return useLoad<AccountsResponse>(() => api<AccountsResponse>(url), [url])
}

function usePackages () {
	return useLoad<PackageRow[]>(() => listOf<PackageRow>('/api/v1/packages'), [])
}

/* ---------------- Modify an Account ---------------- */

export function AccountModify () {
	const toast = useToast()
	const accounts = useAccounts()
	const resellers = useLoad<Array<{ id: string; name: string }>>(
		() => listOf<{ id: string; name: string }>('/api/v1/resellers').catch(() => []),
		[],
	)
	const [id, setId] = useState('')
	const [domain, setDomain] = useState('')
	const [ip, setIp] = useState('')
	const [loginDisabled, setLoginDisabled] = useState(false)
	const [resellerId, setResellerId] = useState('')
	const [busy, setBusy] = useState(false)

	const rows = accounts.data?.items ?? []
	const account = rows.find((a) => a.id === id)

	function choose (next: string) {
		setId(next)
		const picked = rows.find((a) => a.id === next)
		setDomain(picked?.primary_domain ?? '')
		setIp(picked?.ip_address ?? '')
		setLoginDisabled(!!picked?.login_disabled)
		setResellerId(picked?.reseller_id ?? '')
	}

	async function save () {
		if (!account) return
		setBusy(true)
		await toast.run(
			() => patch(`/api/v1/accounts/${account.id}`, {
				primary_domain: domain,
				ip_address: ip,
				login_disabled: loginDisabled,
				reseller_id: resellerId,
			}),
			() => `${account.username} updated. A reconcile job is applying the change.`,
		)
		await accounts.reload()
		setBusy(false)
	}

	return (
		<>
			<PageHeader
				title="Modify an Account"
				description="Change the primary domain, dedicated IP, reseller owner and shell login state. Every change queues a reconcile job."
				favoritePath="/accounts/modify"
			/>
			<Panel title="Choose an account" icon="users">
				<AccountPicker accounts={rows} value={id} onChange={choose} loading={accounts.loading} />
			</Panel>
			{account ? (
				<Panel
					title={`Settings for ${account.username}`}
					icon="edit"
					subtitle={<StatusPill status={account.status} />}
					actions={<Link className="btn secondary small" to={`/accounts/${account.id}`}>Open account summary</Link>}
				>
					<div className="form-grid">
						<Field label="Primary domain" hint="Renaming rewrites the vhost server name. The DNS zone is not renamed automatically.">
							<input value={domain} onChange={(e) => setDomain(e.target.value.toLowerCase().trim())} spellCheck={false} />
						</Field>
						<Field label="Dedicated IP address" hint="Blank serves this account from the shared host address.">
							<input value={ip} onChange={(e) => setIp(e.target.value.trim())} placeholder="shared" spellCheck={false} />
						</Field>
						<Field label="Owned by" hint="Moving an account changes which reseller can manage it.">
							<select value={resellerId} onChange={(e) => setResellerId(e.target.value)}>
								<option value="">Direct — no reseller</option>
								{(resellers.data ?? []).map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
							</select>
						</Field>
					</div>
					<div style={{ marginTop: 14 }}>
						<CheckField
							label="Disable interactive login"
							hint="The account keeps SFTP file access but cannot open a shell."
							checked={loginDisabled}
							onChange={setLoginDisabled}
						/>
					</div>
					<div className="form-actions">
						<button type="button" className="btn" disabled={busy} onClick={save}>{busy ? 'Saving…' : 'Save changes'}</button>
						<button type="button" className="btn secondary" onClick={() => choose(id)}>Reset</button>
					</div>
				</Panel>
			) : (
				<Panel title="Account settings" icon="edit">
					<EmptyState icon="compass" title="Pick an account to modify">
						Choose an account above. Its current domain, IP and login state load into the form.
					</EmptyState>
				</Panel>
			)}
		</>
	)
}

/* ---------------- Upgrade / Downgrade ---------------- */

export function AccountChangePackage () {
	const toast = useToast()
	const accounts = useAccounts()
	const packages = usePackages()
	const [id, setId] = useState('')
	const [packageId, setPackageId] = useState('')
	const [confirming, setConfirming] = useState(false)
	const [busy, setBusy] = useState(false)

	const rows = accounts.data?.items ?? []
	const usage = accounts.data?.usage ?? {}
	const account = rows.find((a) => a.id === id)
	const current = (packages.data ?? []).find((p) => p.id === account?.package_id)
	const next = (packages.data ?? []).find((p) => p.id === packageId)
	const shrinksBelowUsage = !!(next && account && usage[account.id] && next.disk_bytes > 0 && usage[account.id].disk_bytes > next.disk_bytes)

	async function apply () {
		if (!account || !next) return
		setBusy(true)
		setConfirming(false)
		await toast.run(
			() => patch(`/api/v1/accounts/${account.id}`, { package_id: next.id }),
			() => `${account.username} moved to ${next.name}. Limits reconcile on the next job run.`,
		)
		await accounts.reload()
		setBusy(false)
	}

	return (
		<>
			<PageHeader
				title="Upgrade / Downgrade an Account"
				description="Move an account onto a different package. Disk, transfer, mail and process limits are re-applied by the reconcile job."
				favoritePath="/accounts/change-package"
			/>
			<Panel title="Choose an account" icon="users">
				<AccountPicker
					accounts={rows}
					value={id}
					onChange={(next) => {
						setId(next)
						setPackageId('')
					}}
					loading={accounts.loading}
				/>
			</Panel>
			{account ? (
				<Panel title={`Package for ${account.username}`} icon="layers">
					<div className="form-grid">
						<Field label="Current package">
							<input value={current?.name ?? 'unassigned'} readOnly />
						</Field>
						<Field label="New package" hint="Only packages you can administer are listed.">
							<select value={packageId} onChange={(e) => setPackageId(e.target.value)}>
								<option value="">Select a package…</option>
								{(packages.data ?? []).filter((p) => p.id !== current?.id).map((p) => (
									<option key={p.id} value={p.id}>{p.name}</option>
								))}
							</select>
						</Field>
					</div>
					{next ? (
						<div style={{ marginTop: 16 }}>
							<ReviewList
								rows={[
									['Disk limit', <ChangeCell key="d" from={current?.disk_bytes} to={next.disk_bytes} format={formatBytes} />],
									['Monthly transfer', <ChangeCell key="b" from={current?.bandwidth_bytes_monthly} to={next.bandwidth_bytes_monthly} format={formatBytes} />],
									['Domains', <ChangeCell key="dom" from={current?.domains} to={next.domains} />],
									['Databases', <ChangeCell key="db" from={current?.databases} to={next.databases} />],
									['Mailboxes', <ChangeCell key="mb" from={current?.mailboxes} to={next.mailboxes} />],
									['CPU', <ChangeCell key="cpu" from={current?.cpu_percent} to={next.cpu_percent} format={(v) => `${v}%`} />],
									['Memory', <ChangeCell key="mem" from={current?.memory_bytes} to={next.memory_bytes} format={formatBytes} />],
								]}
							/>
						</div>
					) : null}
					{shrinksBelowUsage ? (
						<div style={{ marginTop: 14 }}>
							<Notice tone="warn">
								This account already stores more than the new disk limit. Kelmor keeps existing files but rejects new
								writes over the cap until the account is under quota.
							</Notice>
						</div>
					) : null}
					<div className="form-actions">
						<button type="button" className="btn" disabled={!next || busy} onClick={() => setConfirming(true)}>
							Apply package change
						</button>
					</div>
				</Panel>
			) : (
				<Panel title="Package change" icon="layers">
					<EmptyState icon="layers" title="Pick an account to move">Choose an account, then compare its current and new package side by side.</EmptyState>
				</Panel>
			)}

			{confirming && account && next ? (
				<ConfirmDialog
					title={`Move ${account.username} to ${next.name}?`}
					tone="normal"
					confirmLabel="Apply package change"
					busy={busy}
					onCancel={() => setConfirming(false)}
					onConfirm={apply}
				>
					<p>Kelmor rewrites the systemd slice, nginx limits and PHP-FPM worker count for this account on the next reconcile.</p>
				</ConfirmDialog>
			) : null}
		</>
	)
}

function ChangeCell ({ from, to, format }: { from?: number; to: number; format?: (v: number) => string }) {
	const render = format ?? ((v: number) => String(v))
	if (from === undefined) return <span>{render(to)}</span>
	const direction = to > from ? 'ok' : to < from ? 'warn' : 'idle'
	return (
		<span>
			<span className="muted">{render(from)}</span> → <strong className={direction === 'warn' ? 'bad' : undefined}>{render(to)}</strong>
		</span>
	)
}

/* ---------------- Manage Account Suspension ---------------- */

export function AccountSuspension () {
	const toast = useToast()
	const accounts = useAccounts()
	const [selected, setSelected] = useState<string[]>([])
	const [confirming, setConfirming] = useState<'suspend' | 'unsuspend' | null>(null)
	const [busy, setBusy] = useState(false)

	const rows = accounts.data?.items ?? []
	const targets = rows.filter((a) => selected.includes(a.id))

	async function run (action: 'suspend' | 'unsuspend') {
		setBusy(true)
		setConfirming(null)
		await toast.run(
			async () => {
				for (const account of targets) {
					await post(`/api/v1/accounts/${account.id}/${action}`)
				}
				return targets.length
			},
			(count) => `${action === 'suspend' ? 'Suspension' : 'Restoration'} queued for ${count} account${count === 1 ? '' : 's'}`,
		)
		setSelected([])
		await accounts.reload()
		setBusy(false)
	}

	const columns: Column<AccountRow>[] = [
		{ key: 'username', header: 'User', sort: (a) => a.username, render: (a) => <Link to={`/accounts/${a.id}`}>{a.username}</Link> },
		{ key: 'domain', header: 'Primary domain', sort: (a) => a.primary_domain, className: 'mono' },
		{ key: 'status', header: 'Status', sort: (a) => a.status, render: (a) => <StatusPill status={a.status} /> },
	]

	return (
		<>
			<PageHeader
				title="Manage Account Suspension"
				description="Suspending holds a site out of service and stops mail delivery without deleting anything. Restoring puts it straight back."
				favoritePath="/accounts/suspension"
			/>
			<Notice tone="info">
				Select one or more accounts, then suspend or restore them together. Files, databases and mailboxes are untouched
				either way — use <Link to="/accounts/terminate">Terminate Accounts</Link> to remove an account for good.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(a) => a.id}
				loading={accounts.loading}
				error={accounts.error}
				searchPlaceholder="Search username or domain"
				noun="accounts"
				selectable
				selected={selected}
				onSelectedChange={setSelected}
				initialSort={{ key: 'status', dir: 'asc' }}
				actions={
					<>
						<button type="button" className="btn secondary" disabled={selected.length === 0 || busy} onClick={() => setConfirming('unsuspend')}>
							Restore selected
						</button>
						<button type="button" className="btn" disabled={selected.length === 0 || busy} onClick={() => setConfirming('suspend')}>
							Suspend selected
						</button>
					</>
				}
			/>
			{confirming ? (
				<ConfirmDialog
					title={confirming === 'suspend' ? `Suspend ${targets.length} account(s)?` : `Restore ${targets.length} account(s)?`}
					tone={confirming === 'suspend' ? 'danger' : 'normal'}
					confirmLabel={confirming === 'suspend' ? 'Suspend accounts' : 'Restore accounts'}
					busy={busy}
					onCancel={() => setConfirming(null)}
					onConfirm={() => run(confirming)}
				>
					<p>{confirming === 'suspend'
						? 'Websites stop serving and mail is held until these accounts are restored.'
						: 'Websites start serving again and held mail resumes delivery.'}</p>
					<ul className="kv" style={{ marginTop: 10 }}>
						{targets.slice(0, 8).map((a) => (
							<li key={a.id}><span className="k">{a.username}</span><span className="v mono">{a.primary_domain}</span></li>
						))}
					</ul>
					{targets.length > 8 ? <p className="small muted">…and {targets.length - 8} more.</p> : null}
				</ConfirmDialog>
			) : null}
		</>
	)
}

/* ---------------- Terminate Accounts ---------------- */

export function AccountTerminate () {
	const toast = useToast()
	const accounts = useAccounts()
	const [selected, setSelected] = useState<string[]>([])
	const [confirming, setConfirming] = useState(false)
	const [busy, setBusy] = useState(false)

	const rows = (accounts.data?.items ?? []).filter((a) => a.status !== 'terminated')
	const targets = rows.filter((a) => selected.includes(a.id))

	async function run () {
		setBusy(true)
		setConfirming(false)
		await toast.run(
			async () => {
				for (const account of targets) {
					await post(`/api/v1/accounts/${account.id}/terminate`)
				}
				return targets.length
			},
			(count) => `Termination queued for ${count} account${count === 1 ? '' : 's'}`,
		)
		setSelected([])
		await accounts.reload()
		setBusy(false)
	}

	const columns: Column<AccountRow>[] = [
		{ key: 'username', header: 'User', sort: (a) => a.username, render: (a) => <Link to={`/accounts/${a.id}`}>{a.username}</Link> },
		{ key: 'domain', header: 'Primary domain', sort: (a) => a.primary_domain, className: 'mono' },
		{ key: 'status', header: 'Status', sort: (a) => a.status, render: (a) => <StatusPill status={a.status} /> },
	]

	return (
		<>
			<PageHeader
				title="Terminate Accounts"
				description="Permanent removal of the Linux identity, home directory, websites, databases, mailboxes and DNS zone."
				favoritePath="/accounts/terminate"
			/>
			<Notice tone="error">
				Termination cannot be undone from Director. Take a backup first from
				{' '}<Link to="/transfers/backups">Account Backups</Link> or an export from{' '}
				<Link to="/transfers/export">Export Accounts</Link> if the data may still be needed.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(a) => a.id}
				loading={accounts.loading}
				error={accounts.error}
				searchPlaceholder="Search username or domain"
				noun="accounts"
				selectable
				selected={selected}
				onSelectedChange={setSelected}
				actions={
					<button type="button" className="btn danger" disabled={selected.length === 0 || busy} onClick={() => setConfirming(true)}>
						Terminate {selected.length || ''} selected
					</button>
				}
			/>
			{confirming ? (
				<ConfirmDialog
					title={`Terminate ${targets.length} account(s)?`}
					confirmLabel="Terminate permanently"
					typeToConfirm="TERMINATE"
					busy={busy}
					onCancel={() => setConfirming(false)}
					onConfirm={run}
				>
					<p>This removes the Linux user, home directory, websites, databases, mailboxes and DNS zone for:</p>
					<ul className="kv" style={{ marginTop: 10 }}>
						{targets.slice(0, 8).map((a) => (
							<li key={a.id}><span className="k">{a.username}</span><span className="v mono">{a.primary_domain}</span></li>
						))}
					</ul>
					{targets.length > 8 ? <p className="small muted">…and {targets.length - 8} more.</p> : null}
				</ConfirmDialog>
			) : null}
		</>
	)
}

/* ---------------- Force Password Change ---------------- */

export function AccountPassword () {
	const toast = useToast()
	const accounts = useAccounts()
	const [id, setId] = useState('')
	const [password, setPassword] = useState('')
	const [confirm, setConfirm] = useState('')
	const [mustChange, setMustChange] = useState(true)
	const [busy, setBusy] = useState(false)

	const rows = accounts.data?.items ?? []
	const account = rows.find((a) => a.id === id)
	const mismatch = !!confirm && confirm !== password
	const tooShort = !!password && password.length < 12

	async function submit () {
		if (!account) return
		setBusy(true)
		const done = await toast.run(
			() => post(`/api/v1/accounts/${account.id}/password`, { password, must_change_password: mustChange }),
			() => `Password rotated for ${account.username}. A reconcile job is applying the SFTP credential.`,
		)
		if (done) {
			setPassword('')
			setConfirm('')
		}
		setBusy(false)
	}

	return (
		<>
			<PageHeader
				title="Force Password Change"
				description="Rotate the account owner password and optionally require a new one at the next Kelmor Control sign-in."
				favoritePath="/accounts/password"
			/>
			<Panel title="Choose an account" icon="users">
				<AccountPicker accounts={rows} value={id} onChange={setId} loading={accounts.loading} />
			</Panel>
			<Panel title="New credential" icon="key">
				{!account ? (
					<EmptyState icon="key" title="Pick an account first">
						Password rotation applies to the account owner login and the SFTP credential for the same identity.
					</EmptyState>
				) : (
					<>
						<Notice tone="warn">
							The new password replaces the Kelmor Control sign-in and the SFTP credential for
							{' '}<strong>{account.username}</strong>. Deliver it over a channel the customer already trusts — Director
							never emails credentials.
						</Notice>
						<div className="form-grid">
							<Field label="New password" error={tooShort ? 'Use at least 12 characters.' : undefined} hint="At least 12 characters.">
								<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
							</Field>
							<Field label="Confirm password" error={mismatch ? 'The two passwords do not match.' : undefined}>
								<input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
							</Field>
						</div>
						<div style={{ marginTop: 14 }}>
							<CheckField
								label="Require a password change at next sign-in"
								hint="Recorded on the owner user so Kelmor Control prompts for a new password."
								checked={mustChange}
								onChange={setMustChange}
							/>
						</div>
						<div className="form-actions">
							<button
								type="button"
								className="btn"
								disabled={busy || tooShort || mismatch || password.length === 0}
								onClick={submit}
							>
								{busy ? 'Applying…' : 'Change password'}
							</button>
						</div>
					</>
				)}
			</Panel>
		</>
	)
}

/* ---------------- Limit Bandwidth and Disk ---------------- */

export function AccountQuotas () {
	const accounts = useAccounts()
	const packages = usePackages()
	const rows = accounts.data?.items ?? []
	const usage = accounts.data?.usage ?? {}
	const packageById = useMemo(() => new Map((packages.data ?? []).map((p) => [p.id, p])), [packages.data])

	const columns: Column<AccountRow>[] = [
		{ key: 'username', header: 'User', sort: (a) => a.username, render: (a) => <Link to={`/accounts/${a.id}`}>{a.username}</Link> },
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
			header: 'Disk used',
			align: 'right',
			sort: (a) => usedPercent(usage[a.id]?.disk_bytes, packageById.get(a.package_id || '')?.disk_bytes),
			render: (a) => <QuotaBar used={usage[a.id]?.disk_bytes ?? 0} limit={packageById.get(a.package_id || '')?.disk_bytes ?? 0} />,
		},
		{
			key: 'bandwidth',
			header: 'Transfer used',
			align: 'right',
			sort: (a) => usedPercent(usage[a.id]?.bandwidth_bytes, packageById.get(a.package_id || '')?.bandwidth_bytes_monthly),
			render: (a) => <QuotaBar used={usage[a.id]?.bandwidth_bytes ?? 0} limit={packageById.get(a.package_id || '')?.bandwidth_bytes_monthly ?? 0} />,
		},
	]

	return (
		<>
			<PageHeader
				title="Limit Bandwidth and Disk"
				description="Kelmor enforces disk and monthly transfer from the package, not per account. Review consumption here, then adjust the package or move the account."
				favoritePath="/accounts/quotas"
			/>
			<Notice tone="info">
				Limits live on the package so they stay consistent across every account that shares it. To change one account
				only, move it to a different package with <Link to="/accounts/change-package">Upgrade / Downgrade an Account</Link>;
				to change every account on a plan, edit the plan in <Link to="/packages">Packages</Link>.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(a) => a.id}
				loading={accounts.loading || packages.loading}
				error={accounts.error}
				searchPlaceholder="Search username or package"
				noun="accounts"
				initialSort={{ key: 'disk', dir: 'desc' }}
			/>
		</>
	)
}

function QuotaBar ({ used, limit }: { used: number; limit: number }) {
	const percent = usedPercent(used, limit)
	return (
		<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center', justifyContent: 'flex-end' }}>
			{limit > 0 ? <Meter used={used} limit={limit} className={meterClass(percent)} /> : null}
			<span className="nowrap">{formatBytes(used)}{limit > 0 ? <span className="muted"> / {formatBytes(limit)} ({percent}%)</span> : <span className="muted"> / unmetered</span>}</span>
		</span>
	)
}

/* ---------------- Account Summary picker ---------------- */

export function AccountSummaryPicker () {
	const navigate = useNavigate()
	const accounts = useAccounts()
	const [id, setId] = useState('')
	const rows = accounts.data?.items ?? []
	const account = rows.find((a) => a.id === id)

	return (
		<>
			<PageHeader
				title="Account Summary"
				description="Open the management hub for one account: domains, DNS, email, databases, SSL, files, backups and its job history."
				favoritePath="/accounts/summary"
			/>
			<Panel title="Choose an account" icon="compass">
				<AccountPicker accounts={rows} value={id} onChange={setId} loading={accounts.loading} />
				{account ? (
					<>
						<div style={{ marginTop: 16 }}>
							<KeyValues
								rows={[
									['Username', account.username],
									['Primary domain', <span key="d" className="mono">{account.primary_domain}</span>],
									['Status', <StatusPill key="s" status={account.status} />],
									['Linux UID', account.linux_uid ?? '—'],
									['Home directory', <span key="h" className="mono">{account.home_path ?? '—'}</span>],
								]}
							/>
						</div>
						<div className="form-actions">
							<button type="button" className="btn" onClick={() => navigate(`/accounts/${account.id}`)}>Open account summary</button>
						</div>
					</>
				) : null}
			</Panel>
		</>
	)
}
