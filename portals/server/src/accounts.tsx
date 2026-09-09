import { useEffect, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { api, asList } from './client'
import { Can } from './rbac'
import { pageSlice } from './pager'
import { act, EmptyRow, fmtBytes, Notice, Pager, PageHeader } from './ui'
import { validateCreateAccount } from './validate'

interface AccountRow {
	id: string
	username: string
	primary_domain: string
	status: string
	linux_uid: number
	package_id?: string
	reseller_id?: string
}

interface Named {
	id: string
	name: string
}

export function AccountList () {
	const [items, setItems] = useState<AccountRow[]>([])
	const [packages, setPackages] = useState<Named[]>([])
	const [resellers, setResellers] = useState<Named[]>([])
	const [q, setQ] = useState('')
	const [status, setStatus] = useState('')
	const [picked, setPicked] = useState<Record<string, boolean>>({})
	const [disk, setDisk] = useState<Record<string, number>>({})
	const [diskNote, setDiskNote] = useState('')
	const [msg, setMsg] = useState('')
	const [page, setPage] = useState(1)

	async function reload () {
		const qs = new URLSearchParams()
		if (q) qs.set('q', q)
		if (status) qs.set('status', status)
		const r = await api<{ items: AccountRow[] }>(
			`/api/v1/accounts?${qs.toString()}`,
		)
		setItems(asList(r))
	}

	useEffect(() => {
		setPage(1)
		reload().catch((e) => setMsg(e.message))
	}, [q, status])

	useEffect(() => {
		api<{ items: Named[] }>('/api/v1/packages')
			.then((r) => setPackages(asList(r)))
			.catch(() => setPackages([]))
		api<{ items: Named[] }>('/api/v1/resellers')
			.then((r) => setResellers(asList(r)))
			.catch(() => setResellers([]))
		api<{ accounts?: Array<{ account_id: string; disk_bytes?: number }> }>('/api/v1/server/monitor')
			.then((r) => {
				const next: Record<string, number> = {}
				for (const u of r.accounts || [])
					next[u.account_id] = u.disk_bytes || 0
				setDisk(next)
				setDiskNote('')
			})
			.catch(() => {
				setDisk({})
				setDiskNote('Disk column needs GET /server/monitor (billing.usage.read). Shown as — when that capability is missing.')
			})
	}, [])

	function nameOf (list: Named[], id?: string) {
		if (!id) return '—'
		return list.find((x) => x.id === id)?.name || id
	}

	const paged = pageSlice(items, page)

	return (
		<>
			<PageHeader
				title="List Accounts"
				detail="Hosting accounts with Linux identities. Open a row for the operations hub."
			/>
			<div className="toolbar">
				<Can cap="accounts.create">
					<NavLink className="button-link" to="/accounts/create">
						Create Account
					</NavLink>
				</Can>
				<input
					className="search"
					placeholder="Search username or domain"
					value={q}
					onChange={(e) => setQ(e.target.value)}
				/>
				<select
					value={status}
					onChange={(e) => setStatus(e.target.value)}
					aria-label="Status filter"
				>
					<option value="">All statuses</option>
					<option value="active">active</option>
					<option value="provisioning">provisioning</option>
					<option value="suspended">suspended</option>
					<option value="terminating">terminating</option>
				</select>
				<button
					type="button"
					className="ghost-inline"
					onClick={async () => {
						try {
							const exp = await api<any>('/api/v1/accounts/export')
							const blob = new Blob(
								[JSON.stringify(exp, null, 2)],
								{ type: 'application/json' },
							)
							const url = URL.createObjectURL(blob)
							const a = document.createElement('a')
							a.href = url
							a.download = 'accounts-export.json'
							a.click()
							URL.revokeObjectURL(url)
							setMsg('Account list export downloaded')
						} catch (e) {
							setMsg(e instanceof Error ? e.message : 'export failed')
						}
					}}
				>
					Export list
				</button>
				<Can cap="accounts.suspend">
					<button
						type="button"
						onClick={async () => {
							const ids = Object.keys(picked).filter((id) => picked[id])
							if (ids.length === 0) {
								setMsg('Select accounts to suspend')
								return
							}
							try {
								const r = await api<any>('/api/v1/accounts/bulk/suspend', {
									method: 'POST',
									body: JSON.stringify({ ids }),
								})
								setMsg(`Queued ${ (r.operations || []).length } suspend jobs`)
								setPicked({})
								await reload()
							} catch (e) {
								setMsg(e instanceof Error ? e.message : 'bulk suspend failed')
							}
						}}
					>
						Suspend selected
					</button>
				</Can>
			</div>
			<Notice>{msg}</Notice>
			{diskNote ? <p className="muted">{diskNote}</p> : null}
			<Pager
				page={paged.page}
				pages={paged.pages}
				total={paged.total}
				onPage={setPage}
				label="Accounts"
			/>
			<table>
				<thead>
					<tr>
						<th></th>
						<th>User</th>
						<th>Domain</th>
						<th>Package</th>
						<th>Disk</th>
						<th>Status</th>
						<th>UID</th>
						<th>Reseller</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{paged.rows.length === 0 ? (
						<EmptyRow
							cols={9}
							text="No accounts match. Create an account or clear the search filter."
						/>
					) : paged.rows.map((a) => (
						<tr key={a.id}>
							<td>
								<input
									type="checkbox"
									checked={!!picked[a.id]}
									aria-label={`Select ${a.username}`}
									onChange={(e) => setPicked({
										...picked,
										[a.id]: e.target.checked,
									})}
								/>
							</td>
							<td>
								<NavLink to={`/accounts/${a.id}`}>{a.username}</NavLink>
							</td>
							<td>{a.primary_domain}</td>
							<td>{nameOf(packages, a.package_id)}</td>
							<td>{a.id in disk ? fmtBytes(disk[a.id]) : '—'}</td>
							<td>{a.status}</td>
							<td>{a.linux_uid}</td>
							<td>{nameOf(resellers, a.reseller_id)}</td>
							<td className="row-actions">
								<NavLink to={`/accounts/${a.id}`}>Open</NavLink>
								<Can cap="accounts.modify">
									<NavLink to="/accounts/package">Package</NavLink>
								</Can>
								<Can cap="accounts.suspend">
									<button
										type="button"
										onClick={() => act(`/api/v1/accounts/${a.id}/suspend`, reload, setMsg)}
									>
										Suspend
									</button>
									<button
										type="button"
										onClick={() => act(`/api/v1/accounts/${a.id}/unsuspend`, reload, setMsg)}
									>
										Unsuspend
									</button>
								</Can>
								<Can cap="accounts.terminate">
									<NavLink to="/accounts/terminate">Terminate</NavLink>
								</Can>
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

export function CreateAccount () {
	const [packages, setPackages] = useState<Named[]>([])
	const [resellers, setResellers] = useState<Named[]>([])
	const [msg, setMsg] = useState('')
	const [errors, setErrors] = useState<Record<string, string>>({})
	const nav = useNavigate()

	useEffect(() => {
		api<{ items: Named[] }>('/api/v1/packages')
			.then((r) => setPackages(asList(r)))
			.catch((e) => setMsg(e.message))
		api<{ items: Named[] }>('/api/v1/resellers')
			.then((r) => setResellers(asList(r)))
			.catch(() => setResellers([]))
	}, [])

	return (
		<>
			<PageHeader
				title="Create Account"
				detail="Guided provision: identity, package, then owner credentials. Fields match POST /accounts."
			/>
			<Can cap="accounts.create">
				<form
					className="guided-form"
					onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						const input = {
							username: String(fd.get('username') || ''),
							domain: String(fd.get('domain') || ''),
							email: String(fd.get('email') || ''),
							password: String(fd.get('password') || ''),
							package_id: String(fd.get('package_id') || ''),
						}
						const next = validateCreateAccount(input)
						setErrors(next)
						if (Object.keys(next).length) {
							setMsg('Fix the highlighted fields before provisioning.')
							return
						}
						setMsg('')
						try {
							const r = await api<any>('/api/v1/accounts', {
								method: 'POST',
								headers: { 'Idempotency-Key': crypto.randomUUID() },
								body: JSON.stringify({
									username: input.username,
									primary_domain: input.domain,
									package_id: input.package_id,
									reseller_id: fd.get('reseller_id') || undefined,
									owner_email: input.email,
									owner_password: input.password,
								}),
							})
							setMsg(`Queued ${r.operation_id}`)
							nav(`/accounts/${r.resource_id || r.account?.id}`)
						} catch (err) {
							setMsg(err instanceof Error ? err.message : 'failed')
						}
					}}
				>
					<fieldset>
						<legend>1. Identity</legend>
						<label>
							Username
							<input name="username" required autoComplete="off" aria-invalid={!!errors.username} />
							{errors.username ? <span className="field-error">{errors.username}</span> : null}
						</label>
						<label>
							Primary domain
							<input name="domain" placeholder="example.test" required aria-invalid={!!errors.domain} />
							{errors.domain ? <span className="field-error">{errors.domain}</span> : null}
						</label>
					</fieldset>
					<fieldset>
						<legend>2. Package</legend>
						<label>
							Package
							<select name="package_id" required aria-invalid={!!errors.package_id}>
								{packages.map((p) => (
									<option key={p.id} value={p.id}>{p.name}</option>
								))}
							</select>
							{errors.package_id ? <span className="field-error">{errors.package_id}</span> : null}
						</label>
						<label>
							Reseller
							<select name="reseller_id">
								<option value="">Direct (no reseller)</option>
								{resellers.map((r) => (
									<option key={r.id} value={r.id}>{r.name}</option>
								))}
							</select>
						</label>
					</fieldset>
					<fieldset>
						<legend>3. Owner</legend>
						<label>
							Owner email
							<input name="email" type="email" aria-invalid={!!errors.email} />
							{errors.email ? <span className="field-error">{errors.email}</span> : null}
						</label>
						<label>
							Owner password
							<input name="password" type="password" required aria-invalid={!!errors.password} />
							{errors.password ? <span className="field-error">{errors.password}</span> : null}
						</label>
					</fieldset>
					<div className="toolbar">
						<button type="submit">Provision account</button>
						<NavLink to="/accounts">Back to List Accounts</NavLink>
					</div>
				</form>
			</Can>
			<Notice>{msg}</Notice>
		</>
	)
}
