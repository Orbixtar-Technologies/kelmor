import { Fragment, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, asList } from './client'
import { Can } from './rbac'
import { pageSlice } from './pager'
import { Empty, EmptyRow, fmtBytes, Notice, Pager, PageHeader } from './ui'

export function ImportAccount () {
	const [msg, setMsg] = useState('')
	return (
		<>
			<PageHeader
				title="Import / Migration"
				detail="Native Kelmor export, extracted cpmove, or migrate-from-account on the operations hub. Colliding usernames and domains are refused."
			/>
			<form
				className="stack-form"
				onSubmit={async (e) => {
					e.preventDefault()
					const fd = new FormData(e.currentTarget)
					const file = fd.get('export') as File
					setMsg('')
					try {
						const raw = await file.text()
						const qs = new URLSearchParams()
						if (fd.get('username')) qs.set('username', String(fd.get('username')))
						if (fd.get('domain')) qs.set('domain', String(fd.get('domain')))
						const suffix = qs.toString() ? '?' + qs.toString() : ''
						const r = await api<any>('/api/v1/accounts/import' + suffix, {
							method: 'POST',
							body: raw,
							headers: { 'Content-Type': 'application/json' },
						})
						setMsg(`Imported ${r.account?.username || r.resource_id}`)
					} catch (err) {
						setMsg(err instanceof Error ? err.message : 'import failed')
					}
				}}
			>
				<h2>Native export</h2>
				<input name="export" type="file" accept="application/json" required />
				<input name="username" placeholder="optional new username" />
				<input name="domain" placeholder="optional new primary domain" />
				<button type="submit">Import native export</button>
			</form>
			<form
				className="stack-form"
				onSubmit={async (e) => {
					e.preventDefault()
					const fd = new FormData(e.currentTarget)
					try {
						const r = await api<any>('/api/v1/accounts/import/cpanel', {
							method: 'POST',
							body: JSON.stringify({
								root: fd.get('root'),
								username: fd.get('username'),
							}),
						})
						setMsg(`cpmove import queued for ${r.account?.username || r.resource_id}`)
					} catch (err) {
						setMsg(err instanceof Error ? err.message : 'cpmove import failed')
					}
				}}
			>
				<h2>Extracted cpmove</h2>
				<input name="root" placeholder="/var/tmp/cpmove-user" required />
				<input name="username" placeholder="username" required />
				<button type="submit">Import extracted cpmove</button>
			</form>
			<Notice>{msg}</Notice>
		</>
	)
}

export function Packages () {
	const [items, setItems] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const [q, setQ] = useState('')
	async function reload () {
		setItems(asList(await api<{ items: any[] }>('/api/v1/packages')))
	}
	useEffect(() => {
		reload().catch((e) => setMsg(e.message))
	}, [])
	const shown = items.filter((p) => {
		const hay = `${p.name}`.toLowerCase()
		return hay.includes(q.trim().toLowerCase())
	})
	return (
		<>
			<PageHeader
				title="Packages"
				detail="Reusable CPU, memory, I/O and feature limits enforced through slices and quotas. There is no package edit or delete API — create a new package and use Change Package on the account."
			/>
			<Can cap="packages.write">
				<form
					className="row"
					onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						const disk = Number(fd.get('disk_gb') || 10) * (1 << 30)
						const mem = Number(fd.get('memory_gb') || 2) * (1 << 30)
						await api('/api/v1/packages', {
							method: 'POST',
							body: JSON.stringify({
								name: fd.get('name'),
								disk_bytes: disk,
								bandwidth_bytes_monthly: Number(fd.get('bw_gb') || 100) * (1 << 30),
								domains: Number(fd.get('domains') || 5),
								subdomains: 20,
								alias_domains: 10,
								databases: Number(fd.get('databases') || 5),
								database_users: 10,
								mailboxes: Number(fd.get('mailboxes') || 20),
								mailbox_storage_bytes: 2 << 30,
								ftp_users: 5,
								cron_jobs: 10,
								application_instances: 3,
								backup_retention_days: 7,
								cpu_percent: Number(fd.get('cpu_percent') || 200),
								memory_bytes: mem,
								process_limit: 150,
								io_weight: 100,
								iops: 800,
								concurrent_web_requests: 100,
								email_daily_limit: 200,
							}),
						})
						await reload()
					}}
				>
					<input name="name" placeholder="package name" required />
					<input name="disk_gb" type="number" min={1} defaultValue={10} aria-label="Disk GB" />
					<input name="bw_gb" type="number" min={1} defaultValue={100} aria-label="Bandwidth GB" />
					<input name="memory_gb" type="number" min={1} defaultValue={2} aria-label="Memory GB" />
					<input name="cpu_percent" type="number" min={10} defaultValue={200} aria-label="CPU percent" />
					<input name="domains" type="number" min={1} defaultValue={5} aria-label="Domains" />
					<input name="databases" type="number" min={0} defaultValue={5} aria-label="Databases" />
					<input name="mailboxes" type="number" min={0} defaultValue={20} aria-label="Mailboxes" />
					<button type="submit">Create package</button>
				</form>
			</Can>
			<Notice>{msg}</Notice>
			<input
				className="search"
				placeholder="Search package name"
				value={q}
				onChange={(e) => setQ(e.target.value)}
			/>
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>CPU %</th>
						<th>Memory</th>
						<th>Disk</th>
						<th>Bandwidth</th>
						<th>Domains</th>
						<th>DBs</th>
						<th>Mailboxes</th>
					</tr>
				</thead>
				<tbody>
					{shown.length === 0 ? (
						<EmptyRow
							cols={8}
							text="No packages match. Create one above or clear the search."
						/>
					) : shown.map((p) => (
						<tr key={p.id}>
							<td>{p.name}</td>
							<td>{p.cpu_percent}</td>
							<td>{fmtBytes(p.memory_bytes)}</td>
							<td>{fmtBytes(p.disk_bytes)}</td>
							<td>{fmtBytes(p.bandwidth_bytes_monthly)}</td>
							<td>{p.domains}</td>
							<td>{p.databases}</td>
							<td>{p.mailboxes}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

export function Resellers () {
	const [items, setItems] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	useEffect(() => {
		api<{ items: any[] }>('/api/v1/resellers')
			.then((r) => setItems(asList(r)))
			.catch((e) => setMsg(e.message))
	}, [])
	return (
		<>
			<PageHeader
				title="Resellers"
				detail="Delegated privileges. A reseller never sees root secrets or foreign customers."
			/>
			<Can cap="resellers.create">
				<form
					className="row"
					onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						await api('/api/v1/resellers', {
							method: 'POST',
							body: JSON.stringify({
								name: fd.get('name'),
								username: fd.get('username'),
								password: fd.get('password'),
								email: fd.get('email'),
							}),
						})
						setItems(asList(await api<{ items: any[] }>('/api/v1/resellers')))
					}}
				>
					<input name="name" placeholder="reseller brand" required />
					<input name="username" placeholder="login username" required />
					<input name="email" type="email" placeholder="contact email" />
					<input name="password" type="password" placeholder="login password" required />
					<button type="submit">Create reseller</button>
				</form>
			</Can>
			<Notice>{msg}</Notice>
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>Brand</th>
						<th>Status</th>
						<th>Nameservers</th>
					</tr>
				</thead>
				<tbody>
					{items.length === 0 ? (
						<EmptyRow
							cols={4}
							text="No resellers yet. Create one to delegate packages and customer accounts."
						/>
					) : items.map((r) => (
						<tr key={r.id}>
							<td>{r.name}</td>
							<td>{r.brand_name || '—'}</td>
							<td>{r.status}</td>
							<td>{(r.nameservers || []).join(', ') || '—'}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

export function Monitor () {
	const [data, setData] = useState<any>(null)
	const [names, setNames] = useState<Record<string, string>>({})
	const [err, setErr] = useState('')
	const [q, setQ] = useState('')
	useEffect(() => {
		api('/api/v1/server/monitor').then(setData).catch((e) => setErr(e.message))
		api<{ items: { id: string; username: string }[] }>('/api/v1/accounts')
			.then((r) => {
				const next: Record<string, string> = {}
				for (const a of asList(r)) next[a.id] = a.username
				setNames(next)
			})
			.catch(() => setNames({}))
	}, [])
	if (err) return <Empty title="Usage collector failed" detail={err} />
	if (!data) {
		return (
			<Empty
				title="Collecting account usage"
				detail="Walking each home directory through the control plane."
			/>
		)
	}
	const items = (data.accounts || []).filter((u: any) => {
		const label = names[u.account_id] || u.account_id || ''
		return `${label} ${u.account_id}`.toLowerCase().includes(q.trim().toLowerCase())
	})
	return (
		<>
			<PageHeader
				title="Usage / Quotas"
				detail={`Disk, monthly nginx bandwidth, inodes, and process totals from Kelmor Agent. Failed jobs: ${data.failed_jobs}. Certificates expiring within 14 days: ${data.certs_expiring}. Package caps live on Packages; this page is observed usage. Account names come from GET /accounts when that capability is present.`}
			/>
			<input
				className="search"
				placeholder="Search username or account id"
				value={q}
				onChange={(e) => setQ(e.target.value)}
			/>
			<table>
				<thead>
					<tr>
						<th>Account</th>
						<th>Disk</th>
						<th>Bandwidth</th>
						<th>Inodes</th>
						<th>Processes</th>
						<th>Memory</th>
						<th>Collected</th>
					</tr>
				</thead>
				<tbody>
					{items.length === 0 ? (
						<EmptyRow
							cols={7}
							text="No account usage matches. Provision an account, then reload."
						/>
					) : items.map((u: any) => (
						<tr key={u.account_id}>
							<td>{names[u.account_id] || u.account_id}</td>
							<td>{fmtBytes(u.disk_bytes)}</td>
							<td>{fmtBytes(u.bandwidth_bytes)}</td>
							<td>{u.inode_count}</td>
							<td>{u.process_count}</td>
							<td>{fmtBytes(u.memory_bytes)}</td>
							<td>{u.collected_at}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

export function Jobs () {
	const [items, setItems] = useState<any[]>([])
	const [params, setParams] = useSearchParams()
	const state = params.get('state') || ''
	const [typeQ, setTypeQ] = useState('')
	useEffect(() => {
		const qs = state ? `?state=${encodeURIComponent(state)}` : ''
		const load = () => api<{ items: any[] }>(`/api/v1/jobs${qs}`).then((r) => setItems(asList(r)))
		load()
		const id = setInterval(load, 1500)
		return () => clearInterval(id)
	}, [state])
	const shown = items.filter((j) =>
		!typeQ.trim() || String(j.type || '').toLowerCase().includes(typeQ.trim().toLowerCase()),
	)
	return (
		<>
			<PageHeader
				title="Background jobs"
				detail="Durable queue table (shown even when idle). Filter by observed state (GET /jobs?state=) and type (this view). No retry or cancel API."
			/>
			<div className="toolbar">
				<select
					aria-label="Job state"
					value={state}
					onChange={(e) => {
						const next = new URLSearchParams(params)
						if (e.target.value) next.set('state', e.target.value)
						else next.delete('state')
						setParams(next)
					}}
				>
					<option value="">All states</option>
					<option value="queued">queued</option>
					<option value="running">running</option>
					<option value="succeeded">succeeded</option>
					<option value="failed">failed</option>
				</select>
				<input
					className="search"
					placeholder="Filter type"
					value={typeQ}
					onChange={(e) => setTypeQ(e.target.value)}
				/>
			</div>
			<table>
				<thead>
					<tr><th>Type</th><th>State</th><th>%</th><th>Error</th></tr>
				</thead>
				<tbody>
					{shown.length === 0 ? (
						<EmptyRow
							cols={4}
							text={items.length === 0
								? 'Queue is idle — no jobs in this view. Provisioning, backups and certificate work will appear as rows here. Retry and cancel are not exposed (no job mutation API).'
								: 'No jobs match this type filter.'}
						/>
					) : shown.map((j) => (
						<tr key={j.id} className={j.state === 'failed' ? 'row-failed' : undefined}>
							<td>{j.type}</td>
							<td>{j.state}</td>
							<td>{j.progress}</td>
							<td>{j.last_error}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}

function auditDetail (e: any) {
	const body: Record<string, unknown> = {}
	if (e.actor_type) body.actor_type = e.actor_type
	if (e.actor_id) body.actor_id = e.actor_id
	if (e.effective_actor_id) body.effective_actor_id = e.effective_actor_id
	if (e.account_id) body.account_id = e.account_id
	if (e.resource_id) body.resource_id = e.resource_id
	if (e.request_id) body.request_id = e.request_id
	if (e.user_agent) body.user_agent = e.user_agent
	if (e.before_state) body.before_state = e.before_state
	if (e.after_state) body.after_state = e.after_state
	if (e.metadata) body.metadata = e.metadata
	return JSON.stringify(body, null, 2)
}

export function Audit () {
	const [items, setItems] = useState<any[]>([])
	const [err, setErr] = useState('')
	const [q, setQ] = useState('')
	const [ok, setOk] = useState('')
	const [action, setAction] = useState('')
	const [page, setPage] = useState(1)
	const [open, setOpen] = useState('')
	useEffect(() => {
		api<{ items: any[] }>('/api/v1/audit-events')
			.then((r) => setItems(asList(r)))
			.catch((e) => setErr(e.message))
	}, [])
	useEffect(() => { setPage(1) }, [q, ok, action])
	if (err) return <Empty title="Audit unavailable" detail={err} />
	const actions = Array.from(new Set(items.map((e) => String(e.action || ''))))
		.filter(Boolean)
		.sort()
	const shown = items.filter((e) => {
		const hay = `${e.action} ${e.resource_type} ${e.resource_id} ${e.source_ip} ${e.actor_id}`.toLowerCase()
		if (!hay.includes(q.trim().toLowerCase())) return false
		if (ok === 'yes' && !e.success) return false
		if (ok === 'no' && e.success) return false
		if (action && e.action !== action) return false
		return true
	})
	const paged = pageSlice(shown, page)
	return (
		<>
			<PageHeader
				title="Privileged audit trail"
				detail="GET /audit-events (200 most recent). Secrets are redacted. There is no get-by-id API; expand a row for actor, resource, and state payloads."
			/>
			<div className="toolbar">
				<input
					className="search"
					placeholder="Search action, resource, IP, actor"
					value={q}
					onChange={(e) => setQ(e.target.value)}
				/>
				<select
					aria-label="Action filter"
					value={action}
					onChange={(e) => setAction(e.target.value)}
				>
					<option value="">All actions</option>
					{actions.map((a) => (
						<option key={a} value={a}>{a}</option>
					))}
				</select>
				<select
					aria-label="Result filter"
					value={ok}
					onChange={(e) => setOk(e.target.value)}
				>
					<option value="">All results</option>
					<option value="yes">OK</option>
					<option value="no">Failed</option>
				</select>
			</div>
			<Pager
				page={paged.page}
				pages={paged.pages}
				total={paged.total}
				onPage={setPage}
				label="Audit"
			/>
			<table>
				<thead>
					<tr>
						<th>When</th>
						<th>Action</th>
						<th>Resource</th>
						<th>OK</th>
						<th>IP</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{paged.rows.length === 0 ? (
						<EmptyRow
							cols={6}
							text="No audit events match this filter."
						/>
					) : paged.rows.map((e) => (
						<Fragment key={e.id}>
							<tr className={open === e.id ? 'expanded' : undefined}>
								<td>{e.occurred_at}</td>
								<td>{e.action}</td>
								<td>{e.resource_type}</td>
								<td>{e.success ? 'yes' : 'no'}</td>
								<td>{e.source_ip}</td>
								<td>
									<button
										type="button"
										className="ghost-inline"
										onClick={() => setOpen(open === e.id ? '' : e.id)}
									>
										{open === e.id ? 'Hide' : 'Details'}
									</button>
								</td>
							</tr>
							{open === e.id ? (
								<tr>
									<td colSpan={6}>
										<pre className="audit-detail">{auditDetail(e)}</pre>
									</td>
								</tr>
							) : null}
						</Fragment>
					))}
				</tbody>
			</table>
		</>
	)
}
