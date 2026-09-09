import { useEffect, useState } from 'react'
import { api, asList } from './client'
import { Can } from './rbac'
import { Empty, fmtBytes, Notice, PageHeader } from './ui'

export function ImportAccount () {
	const [msg, setMsg] = useState('')
	return (
		<>
			<PageHeader
				title="Import"
				detail="Restore a native Kelmor export, or an extracted cpmove tree. Colliding usernames and domains are refused, then reconcile is queued."
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
	async function reload () {
		setItems(asList(await api<{ items: any[] }>('/api/v1/packages')))
	}
	useEffect(() => {
		reload().catch((e) => setMsg(e.message))
	}, [])
	return (
		<>
			<PageHeader
				title="Packages"
				detail="Reusable CPU, memory, I/O and feature limits enforced through slices and quotas."
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
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>CPU %</th>
						<th>Memory</th>
						<th>Disk</th>
						<th>Domains</th>
						<th>Mailboxes</th>
					</tr>
				</thead>
				<tbody>
					{items.map((p) => (
						<tr key={p.id}>
							<td>{p.name}</td>
							<td>{p.cpu_percent}</td>
							<td>{fmtBytes(p.memory_bytes)}</td>
							<td>{fmtBytes(p.disk_bytes)}</td>
							<td>{p.domains}</td>
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
			{items.length === 0 ? (
				<Empty
					title="No resellers yet"
					detail="Create one to delegate packages and customer accounts."
				/>
			) : (
				<table>
					<thead>
						<tr><th>Name</th><th>Status</th></tr>
					</thead>
					<tbody>
						{items.map((r) => (
							<tr key={r.id}>
								<td>{r.name}</td>
								<td>{r.status}</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	)
}

export function Monitor () {
	const [data, setData] = useState<any>(null)
	const [err, setErr] = useState('')
	useEffect(() => {
		api('/api/v1/server/monitor').then(setData).catch((e) => setErr(e.message))
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
	const items = data.accounts || []
	return (
		<>
			<PageHeader
				title="Account usage"
				detail={`Disk, monthly nginx bandwidth, inodes, and process totals from Kelmor Agent. Failed jobs: ${data.failed_jobs}. Certificates expiring within 14 days: ${data.certs_expiring}.`}
			/>
			{items.length === 0 ? (
				<Empty
					title="No account usage yet"
					detail="Provision an account, then reload this page."
				/>
			) : (
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
						{items.map((u: any) => (
							<tr key={u.account_id}>
								<td>{u.account_id}</td>
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
			)}
		</>
	)
}

export function Jobs () {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => {
		const load = () => api<{ items: any[] }>('/api/v1/jobs').then((r) => setItems(asList(r)))
		load()
		const id = setInterval(load, 1500)
		return () => clearInterval(id)
	}, [])
	return (
		<>
			<PageHeader
				title="Background jobs"
				detail="Durable PostgreSQL-style queue. Progress is observed, not guessed."
			/>
			{items.length === 0 ? (
				<Empty
					title="Queue is idle"
					detail="Provisioning, backups and certificate work will appear here."
				/>
			) : (
				<table>
					<thead>
						<tr><th>Type</th><th>State</th><th>%</th><th>Error</th></tr>
					</thead>
					<tbody>
						{items.map((j) => (
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
	)
}

export function Audit () {
	const [items, setItems] = useState<any[]>([])
	const [err, setErr] = useState('')
	useEffect(() => {
		api<{ items: any[] }>('/api/v1/audit-events')
			.then((r) => setItems(asList(r)))
			.catch((e) => setErr(e.message))
	}, [])
	if (err) return <Empty title="Audit unavailable" detail={err} />
	return (
		<>
			<PageHeader
				title="Privileged audit trail"
				detail="Secrets are redacted. Impersonation keeps the original actor."
			/>
			<table>
				<thead>
					<tr>
						<th>When</th>
						<th>Action</th>
						<th>Resource</th>
						<th>OK</th>
						<th>IP</th>
					</tr>
				</thead>
				<tbody>
					{items.map((e) => (
						<tr key={e.id}>
							<td>{e.occurred_at}</td>
							<td>{e.action}</td>
							<td>{e.resource_type}</td>
							<td>{e.success ? 'yes' : 'no'}</td>
							<td>{e.source_ip}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	)
}
