import { useEffect, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { api, asList } from './client'
import { act, Empty, fmtBytes, Notice, PageHeader } from './ui'

interface AccountRow {
	id: string
	username: string
	primary_domain: string
	status: string
	linux_uid: number
	package_id?: string
	reseller_id?: string
	home_path?: string
	ip_address?: string
	login_disabled?: boolean
}

interface Named { id: string; name: string }

function useAccounts () {
	const [accounts, setAccounts] = useState<AccountRow[]>([])
	const [err, setErr] = useState('')
	useEffect(() => {
		api<{ items: AccountRow[] }>('/api/v1/accounts')
			.then((r) => setAccounts(asList(r)))
			.catch((e) => setErr(e.message))
	}, [])
	return { accounts, err }
}

function AccountSelect ({
	accounts,
	value,
	onChange,
}: {
	accounts: AccountRow[]
	value: string
	onChange: (id: string) => void
}) {
	return (
		<label>
			Account
			<select value={value} onChange={(e) => onChange(e.target.value)} required>
				<option value="">Select an account</option>
				{accounts.map((a) => (
					<option key={a.id} value={a.id}>
						{a.username} · {a.primary_domain} · {a.status}
					</option>
				))}
			</select>
		</label>
	)
}

export function SuspendAccount () {
	const { accounts, err } = useAccounts()
	const [id, setId] = useState('')
	const [msg, setMsg] = useState('')
	async function reload () {
		return
	}
	if (err) return <Empty title="Could not list accounts" detail={err} />
	return (
		<>
			<PageHeader
				title="Suspend / Unsuspend"
				detail="Queues account.reconcile with the desired status. Sites return HTTP 503 while suspended."
			/>
			<div className="stack-form">
				<AccountSelect accounts={accounts} value={id} onChange={setId} />
				<div className="toolbar">
					<button
						type="button"
						disabled={!id}
						onClick={() => act(`/api/v1/accounts/${id}/suspend`, reload, setMsg)}
					>
						Suspend
					</button>
					<button
						type="button"
						disabled={!id}
						onClick={() => act(`/api/v1/accounts/${id}/unsuspend`, reload, setMsg)}
					>
						Unsuspend
					</button>
				</div>
			</div>
			<Notice>{msg}</Notice>
		</>
	)
}

export function TerminateAccount () {
	const { accounts, err } = useAccounts()
	const [id, setId] = useState('')
	const [msg, setMsg] = useState('')
	if (err) return <Empty title="Could not list accounts" detail={err} />
	const acc = accounts.find((a) => a.id === id)
	return (
		<>
			<PageHeader
				title="Terminate Account"
				detail="Removes the Linux user, websites, and mail after confirm. This is irreversible desired state."
			/>
			<div className="stack-form">
				<AccountSelect accounts={accounts} value={id} onChange={setId} />
				<button
					type="button"
					disabled={!id}
					onClick={() => {
						if (!acc) return
						if (!window.confirm(`Terminate ${acc.username}? This removes the Linux user, websites, and mail.`))
							return
						act(`/api/v1/accounts/${id}/terminate`, async () => {}, setMsg)
					}}
				>
					Terminate
				</button>
			</div>
			<Notice>{msg}</Notice>
		</>
	)
}

export function ChangePackage () {
	const { accounts, err } = useAccounts()
	const [packages, setPackages] = useState<Named[]>([])
	const [id, setId] = useState('')
	const [packageId, setPackageId] = useState('')
	const [msg, setMsg] = useState('')
	useEffect(() => {
		api<{ items: Named[] }>('/api/v1/packages')
			.then((r) => setPackages(asList(r)))
			.catch((e) => setMsg(e.message))
	}, [])
	useEffect(() => {
		const acc = accounts.find((a) => a.id === id)
		setPackageId(acc?.package_id || '')
	}, [id, accounts])
	if (err) return <Empty title="Could not list accounts" detail={err} />
	return (
		<>
			<PageHeader
				title="Change Package"
				detail="PATCH package_id and queue account.reconcile. Limits apply on the next observed apply."
			/>
			<form
				className="stack-form"
				onSubmit={async (e) => {
					e.preventDefault()
					if (!id || !packageId) {
						setMsg('Account and package required')
						return
					}
					try {
						const r = await api<any>(`/api/v1/accounts/${id}`, {
							method: 'PATCH',
							body: JSON.stringify({ package_id: packageId }),
						})
						setMsg(`Modify queued ${r.operation_id}`)
					} catch (e) {
						setMsg(e instanceof Error ? e.message : 'modify failed')
					}
				}}
			>
				<AccountSelect accounts={accounts} value={id} onChange={setId} />
				<label>
					Package
					<select value={packageId} onChange={(e) => setPackageId(e.target.value)} required>
						<option value="">Select a package</option>
						{packages.map((p) => (
							<option key={p.id} value={p.id}>{p.name}</option>
						))}
					</select>
				</label>
				<button type="submit">Save package</button>
			</form>
			<Notice>{msg}</Notice>
		</>
	)
}

export function ModifyAccount () {
	const { accounts, err } = useAccounts()
	const [packages, setPackages] = useState<Named[]>([])
	const [resellers, setResellers] = useState<Named[]>([])
	const [id, setId] = useState('')
	const [msg, setMsg] = useState('')
	const acc = accounts.find((a) => a.id === id)
	useEffect(() => {
		api<{ items: Named[] }>('/api/v1/packages')
			.then((r) => setPackages(asList(r)))
			.catch(() => setPackages([]))
		api<{ items: Named[] }>('/api/v1/resellers')
			.then((r) => setResellers(asList(r)))
			.catch(() => setResellers([]))
	}, [])
	if (err) return <Empty title="Could not list accounts" detail={err} />
	return (
		<>
			<PageHeader
				title="Modify Account"
				detail="Fields the control plane accepts on PATCH: package, primary domain, reseller, IP, login disabled."
			/>
			<form
				className="stack-form"
				onSubmit={async (e) => {
					e.preventDefault()
					if (!id) {
						setMsg('Select an account')
						return
					}
					const fd = new FormData(e.currentTarget)
					try {
						const r = await api<any>(`/api/v1/accounts/${id}`, {
							method: 'PATCH',
							body: JSON.stringify({
								package_id: fd.get('package_id'),
								primary_domain: fd.get('primary_domain'),
								reseller_id: fd.get('reseller_id') || '',
								ip_address: fd.get('ip_address') || '',
								login_disabled: fd.get('login_disabled') === 'on',
							}),
						})
						setMsg(`Modify queued ${r.operation_id}`)
					} catch (err) {
						setMsg(err instanceof Error ? err.message : 'modify failed')
					}
				}}
			>
				<AccountSelect accounts={accounts} value={id} onChange={setId} />
				{acc ? (
					<>
						<label>
							Package
							<select name="package_id" defaultValue={acc.package_id} key={acc.id + '-pkg'}>
								{packages.map((p) => (
									<option key={p.id} value={p.id}>{p.name}</option>
								))}
							</select>
						</label>
						<label>
							Primary domain
							<input name="primary_domain" defaultValue={acc.primary_domain} key={acc.id + '-dom'} />
						</label>
						<label>
							Reseller
							<select name="reseller_id" defaultValue={acc.reseller_id || ''} key={acc.id + '-res'}>
								<option value="">Direct (no reseller)</option>
								{resellers.map((r) => (
									<option key={r.id} value={r.id}>{r.name}</option>
								))}
							</select>
						</label>
						<label>
							IP address
							<input name="ip_address" defaultValue={acc.ip_address || ''} key={acc.id + '-ip'} />
						</label>
						<label className="check">
							<input
								name="login_disabled"
								type="checkbox"
								defaultChecked={!!acc.login_disabled}
								key={acc.id + '-login'}
							/>
							Login disabled
						</label>
						<button type="submit">Save account</button>
					</>
				) : <p className="muted">Select an account to edit.</p>}
			</form>
			<Notice>{msg}</Notice>
		</>
	)
}

export function AccountSummary () {
	const { accounts, err } = useAccounts()
	const [id, setId] = useState('')
	const [usage, setUsage] = useState<any>(null)
	const [jobs, setJobs] = useState<any[]>([])
	const [detail, setDetail] = useState<AccountRow | null>(null)
	const nav = useNavigate()
	useEffect(() => {
		if (!id) {
			setDetail(null)
			setUsage(null)
			setJobs([])
			return
		}
		api<AccountRow>(`/api/v1/accounts/${id}`).then(setDetail).catch(() => setDetail(null))
		api(`/api/v1/accounts/${id}/usage`).then(setUsage).catch(() => setUsage(null))
		api<{ items: any[] }>('/api/v1/jobs').then((r) => {
			setJobs(asList(r).filter((x) =>
				x.resource_id === id || (x.payload && x.payload.account_id === id),
			).slice(0, 8))
		}).catch(() => setJobs([]))
	}, [id])
	if (err) return <Empty title="Could not list accounts" detail={err} />
	return (
		<>
			<PageHeader
				title="Account Summary"
				detail="Observed identity and recent jobs. Open the hub for websites, DNS, mail, and backups."
			/>
			<div className="stack-form">
				<AccountSelect accounts={accounts} value={id} onChange={setId} />
			</div>
			{detail ? (
				<>
					<section className="summary-strip">
						<dl>
							<div><dt>User</dt><dd>{detail.username}</dd></div>
							<div><dt>Domain</dt><dd>{detail.primary_domain}</dd></div>
							<div><dt>Status</dt><dd>{detail.status}</dd></div>
							<div><dt>UID</dt><dd>{detail.linux_uid}</dd></div>
							<div><dt>Home</dt><dd>{detail.home_path || '—'}</dd></div>
							<div><dt>Disk</dt><dd>{usage ? fmtBytes(usage.disk_bytes || 0) : '—'}</dd></div>
						</dl>
					</section>
					<div className="toolbar">
						<button type="button" onClick={() => nav(`/accounts/${id}`)}>
							Open operations hub
						</button>
						<NavLink to="/accounts">List Accounts</NavLink>
					</div>
					<h2>Recent jobs</h2>
					{jobs.length === 0 ? <p className="muted">No jobs for this account.</p> : (
						<table>
							<thead>
								<tr><th>Type</th><th>State</th><th>%</th><th>Error</th></tr>
							</thead>
							<tbody>
								{jobs.map((j) => (
									<tr key={j.id}>
										<td>{j.type}</td>
										<td>{j.state}</td>
										<td>{j.progress}</td>
										<td>{j.last_error}</td>
									</tr>
								))}
							</tbody>
						</table>
					)}
				</>
			) : null}
		</>
	)
}
