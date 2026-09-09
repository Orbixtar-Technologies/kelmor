import { useEffect, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { api, asList } from './client'
import { Can } from './rbac'
import { act, Empty, Notice, PageHeader } from './ui'

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
	const [msg, setMsg] = useState('')

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
		reload().catch((e) => setMsg(e.message))
	}, [q, status])

	useEffect(() => {
		api<{ items: Named[] }>('/api/v1/packages')
			.then((r) => setPackages(asList(r)))
			.catch(() => setPackages([]))
		api<{ items: Named[] }>('/api/v1/resellers')
			.then((r) => setResellers(asList(r)))
			.catch(() => setResellers([]))
	}, [])

	function nameOf (list: Named[], id?: string) {
		if (!id) return '—'
		return list.find((x) => x.id === id)?.name || id
	}

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
			{items.length === 0 ? (
				<Empty
					title="No accounts match"
					detail="Create an account or clear the search filter."
				/>
			) : (
				<table>
					<thead>
						<tr>
							<th></th>
							<th>User</th>
							<th>Domain</th>
							<th>Status</th>
							<th>UID</th>
							<th>Package</th>
							<th>Reseller</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{items.map((a) => (
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
								<td>{a.status}</td>
								<td>{a.linux_uid}</td>
								<td>{nameOf(packages, a.package_id)}</td>
								<td>{nameOf(resellers, a.reseller_id)}</td>
								<td className="row-actions">
									<NavLink to={`/accounts/${a.id}`}>Open</NavLink>
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
								</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	)
}

export function CreateAccount () {
	const [packages, setPackages] = useState<Named[]>([])
	const [resellers, setResellers] = useState<Named[]>([])
	const [msg, setMsg] = useState('')
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
				detail="Provisions a Linux tenant, primary domain, and a reconcile job. This is not a decorative wizard."
			/>
			<Can cap="accounts.create">
				<form
					className="stack-form"
					onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						setMsg('')
						try {
							const r = await api<any>('/api/v1/accounts', {
								method: 'POST',
								headers: { 'Idempotency-Key': crypto.randomUUID() },
								body: JSON.stringify({
									username: fd.get('username'),
									primary_domain: fd.get('domain'),
									package_id: fd.get('package_id'),
									reseller_id: fd.get('reseller_id') || undefined,
									owner_email: fd.get('email'),
									owner_password: fd.get('password'),
								}),
							})
							setMsg(`Queued ${r.operation_id}`)
							nav(`/accounts/${r.resource_id || r.account?.id}`)
						} catch (err) {
							setMsg(err instanceof Error ? err.message : 'failed')
						}
					}}
				>
					<label>
						Username
						<input name="username" required autoComplete="off" />
					</label>
					<label>
						Primary domain
						<input name="domain" placeholder="example.test" required />
					</label>
					<label>
						Owner email
						<input name="email" type="email" />
					</label>
					<label>
						Owner password
						<input name="password" type="password" required />
					</label>
					<label>
						Package
						<select name="package_id" required>
							{packages.map((p) => (
								<option key={p.id} value={p.id}>{p.name}</option>
							))}
						</select>
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
