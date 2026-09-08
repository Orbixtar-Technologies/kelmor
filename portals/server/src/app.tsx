import { useEffect, useState } from 'react'
import { NavLink, Route, Routes, useNavigate } from 'react-router-dom'
import { api, asList, clearToken, getToken, setToken } from './client'

interface User { username: string; roles: string[]; email: string }

export function App () {
	const [user, setUser] = useState<User | null>(null)
	const [error, setError] = useState('')

	useEffect(() => {
		if (!getToken()) return
		api<{ user: User }>('/api/v1/me').then((r) => setUser(r.user)).catch(() => clearToken())
	}, [])

	if (!user) {
		return (
			<main className="auth">
				<section className="card">
					<p className="eyebrow">Infrastructure console</p>
					<h1>Server Portal</h1>
					<p className="lede">Administer the host, resellers, packages and privileged jobs. Customer sites live in the Account Portal.</p>
					<form onSubmit={async (e) => {
						e.preventDefault()
						setError('')
						const fd = new FormData(e.currentTarget)
						try {
							const r = await api<{ token: string; user: User }>('/api/v1/auth/login', {
								method: 'POST',
								body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
							})
							setToken(r.token)
							setUser(r.user)
						} catch (err) {
							setError(err instanceof Error ? err.message : 'Login failed')
						}
					}}>
						<label>Username<input name="username" autoComplete="username" defaultValue="admin" /></label>
						<label>Password<input name="password" type="password" autoComplete="current-password" defaultValue="ChangeMeOnce!2026" /></label>
						{error ? <p className="error" role="alert">{error}</p> : null}
						<button type="submit">Sign in</button>
					</form>
				</section>
			</main>
		)
	}

	return (
		<div className="shell">
			<aside>
				<p className="brand">Server Portal</p>
				<nav>
					<NavLink to="/" end>Dashboard</NavLink>
					<NavLink to="/accounts">Accounts</NavLink>
					<NavLink to="/import">Import</NavLink>
					<NavLink to="/resellers">Resellers</NavLink>
					<NavLink to="/packages">Packages</NavLink>
					<NavLink to="/monitor">Usage</NavLink>
					<NavLink to="/jobs">Jobs</NavLink>
					<NavLink to="/audit">Audit</NavLink>
				</nav>
				<button className="ghost" onClick={() => { clearToken(); setUser(null) }}>Sign out {user.username}</button>
			</aside>
			<main className="content">
				<Routes>
					<Route path="/" element={<Dashboard />} />
					<Route path="/accounts" element={<Accounts />} />
					<Route path="/accounts/:id" element={<AccountDetail />} />
					<Route path="/import" element={<ImportAccount />} />
					<Route path="/resellers" element={<Resellers />} />
					<Route path="/packages" element={<Packages />} />
					<Route path="/monitor" element={<Monitor />} />
					<Route path="/jobs" element={<Jobs />} />
					<Route path="/audit" element={<Audit />} />
				</Routes>
			</main>
		</div>
	)
}

function Dashboard () {
	const [data, setData] = useState<any>(null)
	const [err, setErr] = useState('')
	useEffect(() => {
		api('/api/v1/server').then(setData).catch((e) => setErr(e.message))
	}, [])
	if (err) return <Empty title="Could not load host metrics" detail={err} />
	if (!data) return <Empty title="Reading host sensors" detail="Contacting the privileged agent for CPU, memory, disk and services." />
	const s = data.system
	return (
		<>
			<header><h1>Host operations</h1><p>Live observed state from the agent, not decorative charts.</p></header>
			<section className="metrics">
				<Metric label="Hostname" value={s.hostname} />
				<Metric label="Load 1" value={Number(s.load1).toFixed(2)} />
				<Metric label="Memory" value={fmtBytes(s.memory_used) + ' / ' + fmtBytes(s.memory_total)} />
				<Metric label="Disk" value={fmtBytes(s.disk_used) + ' / ' + fmtBytes(s.disk_total)} />
				<Metric label="Inodes" value={`${s.inodes_used} / ${s.inodes_total}`} />
				<Metric label="Uptime" value={`${Math.floor(s.uptime_seconds / 3600)}h`} />
				<Metric label="Accounts" value={String(data.stats.accounts)} />
				<Metric label="Failed jobs" value={String(data.stats.failedJobs)} />
			</section>
			<FirewallPanel />
			<h2>Services</h2>
			<table>
				<thead><tr><th>Service</th><th>Health</th><th>Running</th></tr></thead>
				<tbody>
					{data.services.map((svc: any) => (
						<tr key={svc.name}><td>{svc.name}</td><td>{svc.health}</td><td>{svc.observed_running ? 'yes' : 'no'}</td></tr>
					))}
				</tbody>
			</table>
		</>
	)
}

function FirewallPanel () {
	const [msg, setMsg] = useState('')
	return (
		<section>
			<h2>Host firewall</h2>
			<p>Applies nftables <code>table inet panel</code> with a drop policy on inbound traffic, keeping loopback, established flows, and hosting plus already-bound management ports.</p>
			<button type="button" onClick={async () => {
				setMsg('')
				try {
					const r = await api<any>('/api/v1/server/firewall/apply', { method: 'POST', body: '{}' })
					setMsg(r.message || 'table inet panel applied')
				} catch (e) {
					setMsg(e instanceof Error ? e.message : 'apply failed')
				}
			}}>Apply table inet panel</button>
			{msg ? <p className="notice">{msg}</p> : null}
		</section>
	)
}

function Accounts () {
	const [items, setItems] = useState<any[]>([])
	const [packages, setPackages] = useState<any[]>([])
	const [resellers, setResellers] = useState<any[]>([])
	const [q, setQ] = useState('')
	const [msg, setMsg] = useState('')
	const nav = useNavigate()
	async function reload () {
		const r = await api<{ items: any[] }>(`/api/v1/accounts?q=${encodeURIComponent(q)}`)
		setItems(asList(r))
	}
	useEffect(() => { reload().catch((e) => setMsg(e.message)) }, [q])
	useEffect(() => {
		api<{ items: any[] }>('/api/v1/packages').then((r) => setPackages(asList(r)))
		api<{ items: any[] }>('/api/v1/resellers').then((r) => setResellers(asList(r)))
	}, [])
	return (
		<>
			<header><h1>Hosting accounts</h1><p>Each account gets a dedicated Linux identity and a reconciliation job.</p></header>
			<form className="row" onSubmit={async (e) => {
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
					await reload()
					nav('/jobs')
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'failed')
				}
			}}>
				<input name="username" placeholder="username" required />
				<input name="domain" placeholder="primary domain" required />
				<input name="email" placeholder="owner email" type="email" />
				<input name="password" placeholder="owner password" type="password" required />
				<select name="package_id">{packages.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select>
				<select name="reseller_id">
					<option value="">Direct (no reseller)</option>
					{resellers.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
				</select>
				<button type="submit">Provision account</button>
			</form>
			<input className="search" placeholder="Search username or domain" value={q} onChange={(e) => setQ(e.target.value)} />
			{msg ? <p className="notice">{msg}</p> : null}
			{items.length === 0 ? <Empty title="No accounts match" detail="Create a customer above or clear the search filter." /> : (
				<table>
					<thead><tr><th>User</th><th>Domain</th><th>Status</th><th>UID</th><th></th></tr></thead>
					<tbody>
						{items.map((a) => (
							<tr key={a.id}>
								<td><NavLink to={`/accounts/${a.id}`}>{a.username}</NavLink></td><td>{a.primary_domain}</td><td>{a.status}</td><td>{a.linux_uid}</td>
								<td>
									<button type="button" onClick={() => act(`/api/v1/accounts/${a.id}/suspend`, reload, setMsg)}>Suspend</button>
									<button type="button" onClick={() => act(`/api/v1/accounts/${a.id}/unsuspend`, reload, setMsg)}>Unsuspend</button>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	)
}

function AccountDetail () {
	const [acc, setAcc] = useState<any>(null)
	const [jobs, setJobs] = useState<any[]>([])
	const [sites, setSites] = useState<any[]>([])
	const [domains, setDomains] = useState<any[]>([])
	const [zones, setZones] = useState<any[]>([])
	const [records, setRecords] = useState<any[]>([])
	const [mailboxes, setMailboxes] = useState<any[]>([])
	const [mailDomains, setMailDomains] = useState<any[]>([])
	const [files, setFiles] = useState<any[]>([])
	const [backups, setBackups] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const id = window.location.pathname.split('/').pop() || ''
	async function reload () {
		setAcc(await api(`/api/v1/accounts/${id}`))
		const j = await api<{ items: any[] }>('/api/v1/jobs')
		setJobs(j.items.filter((x) => x.resource_id === id || (x.payload && x.payload.account_id === id)))
		const [ws, ds, zs, mds, mbs, fl, bks] = await Promise.all([
			api<{ items: any[] }>(`/api/v1/accounts/${id}/websites`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/domains`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/dns/zones`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/mail/domains`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/mail/mailboxes`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/files?path=/public_html`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/backups`),
		])
		setSites(ws.items || [])
		setDomains(ds.items || [])
		setZones(zs.items || [])
		setMailDomains(mds.items || [])
		setMailboxes(mbs.items || [])
		setFiles(fl.items || [])
		setBackups(bks.items || [])
		if (zs.items?.[0]) {
			const rec = await api<{ items: any[] }>(`/api/v1/accounts/${id}/dns/zones/${zs.items[0].id}/records`)
			setRecords(rec.items || [])
		}
	}
	useEffect(() => { reload().catch((e) => setMsg(e.message)) }, [id])
	if (!acc) return <Empty title="Loading account" detail={msg || 'Reading desired and observed state.'} />
	return (
		<>
			<header><h1>{acc.username}</h1><p>{acc.primary_domain} · UID {acc.linux_uid} · {acc.status} · {acc.home_path}</p></header>
			{msg ? <p className="notice">{msg}</p> : null}
			<div className="row">
				<button type="button" onClick={() => act(`/api/v1/accounts/${id}/suspend`, reload, setMsg)}>Suspend</button>
				<button type="button" onClick={() => act(`/api/v1/accounts/${id}/unsuspend`, reload, setMsg)}>Unsuspend</button>
				<button type="button" onClick={() => {
					if (!window.confirm(`Terminate ${acc.username}? This removes the Linux user, websites, and mail.`)) return
					act(`/api/v1/accounts/${id}/terminate`, reload, setMsg)
				}}>Terminate</button>
				<button type="button" onClick={async () => {
					const exp = await api<any>(`/api/v1/accounts/${id}/export`)
					const blob = new Blob([JSON.stringify(exp, null, 2)], { type: 'application/json' })
					const url = URL.createObjectURL(blob)
					const a = document.createElement('a')
					a.href = url
					a.download = `${acc.username}.hpm-account.json`
					a.click()
					URL.revokeObjectURL(url)
					setMsg('Native export downloaded')
				}}>Download native export</button>
				<button type="button" onClick={async () => {
					await api(`/api/v1/accounts/${id}/backups`, { method: 'POST', body: JSON.stringify({ kind: 'full', destination: 'local' }) })
					setMsg('Backup queued')
					await reload()
				}}>Queue encrypted backup</button>
			</div>
			<form className="row" onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					const r = await api<any>(`/api/v1/accounts/${id}/migrate`, {
						method: 'POST',
						body: JSON.stringify({ username: fd.get('migrate_username'), domain: fd.get('migrate_domain') }),
					})
					setMsg(`Migrated to ${r.account?.username || r.resource_id}`)
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'migrate failed')
				}
			}}>
				<input name="migrate_username" placeholder="new username" required />
				<input name="migrate_domain" placeholder="new primary domain" required />
				<button type="submit">Migrate to new account</button>
			</form>
			<h2>Websites</h2>
			<form className="row" onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${id}/websites`, { method: 'POST', body: JSON.stringify({
					domain_id: fd.get('domain_id'), runtime: fd.get('runtime'), document_root: acc.home_path + '/public_html',
				}) })
				setMsg('Website apply queued')
				await reload()
			}}>
				<select name="domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn}</option>)}</select>
				<select name="runtime">
					<option value="php">PHP</option>
					<option value="static">Static</option>
					<option value="node">Node</option>
					<option value="python">Python</option>
				</select>
				<button type="submit">Apply website</button>
			</form>
			<table>
				<thead><tr><th>Runtime</th><th>Root</th><th>Enabled</th></tr></thead>
				<tbody>{sites.map((s) => <tr key={s.id}><td>{s.runtime} {s.runtime_version}</td><td>{s.document_root}</td><td>{s.enabled ? 'yes' : 'no'}</td></tr>)}</tbody>
			</table>
			<h2>DNS {zones[0] ? zones[0].name : ''}</h2>
			{records.length === 0 ? <p>No records yet.</p> : (
				<table>
					<thead><tr><th>Name</th><th>Type</th><th>Content</th></tr></thead>
					<tbody>{records.map((r) => <tr key={r.id}><td>{r.name}</td><td>{r.type}</td><td>{r.content}</td></tr>)}</tbody>
				</table>
			)}
			<h2>Mail</h2>
			<form className="row" onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${id}/mail/mailboxes`, { method: 'POST', body: JSON.stringify({
					domain_id: fd.get('domain_id'), local_part: fd.get('local_part'), password: fd.get('password'),
				}) })
				setMsg('Mailbox queued')
				await reload()
			}}>
				<select name="domain_id">{mailDomains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn}</option>)}</select>
				<input name="local_part" placeholder="local part" required />
				<input name="password" type="password" placeholder="mailbox password" required />
				<button type="submit">Create mailbox</button>
			</form>
			<table>
				<thead><tr><th>Mailbox</th><th>Status</th></tr></thead>
				<tbody>{mailboxes.map((m) => <tr key={m.id}><td>{m.local_part}</td><td>{m.status}</td></tr>)}</tbody>
			</table>
			<h2>Files in public_html</h2>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${id}/files`, { method: 'POST', body: JSON.stringify({
					path: fd.get('path'), content: fd.get('content'),
				}) })
				setMsg('File written through the agent')
				await reload()
			}}>
				<input name="path" defaultValue="/public_html/index.html" />
				<textarea name="content" rows={4} placeholder="file contents" required />
				<button type="submit">Write file</button>
			</form>
			<ul>{files.map((f) => <li key={f.name}>{f.dir ? f.name + '/' : `${f.name} (${f.size})`}</li>)}</ul>
			<h2>Backups</h2>
			<ul>{backups.map((b) => <li key={b.id}>{b.kind} {b.state} {b.destination} {b.checksum ? b.checksum.slice(0, 12) : ''}</li>)}</ul>
			<h2>Related jobs</h2>
			{jobs.length === 0 ? <p>No jobs for this account.</p> : (
				<table>
					<thead><tr><th>Type</th><th>State</th><th>%</th><th>Error</th></tr></thead>
					<tbody>{jobs.map((j) => <tr key={j.id}><td>{j.type}</td><td>{j.state}</td><td>{j.progress}</td><td>{j.last_error}</td></tr>)}</tbody>
				</table>
			)}
		</>
	)
}

function ImportAccount () {
	const [msg, setMsg] = useState('')
	return (
		<>
			<header><h1>Native import</h1><p>Restore a hosting account export produced by this control plane. The importer refuses colliding usernames and domains, then queues reconcile.</p></header>
			<form onSubmit={async (e) => {
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
					const r = await api<any>('/api/v1/accounts/import' + suffix, { method: 'POST', body: raw, headers: { 'Content-Type': 'application/json' } })
					setMsg(`Imported ${r.account?.username || r.resource_id}`)
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'import failed')
				}
			}}>
				<input name="export" type="file" accept="application/json" required />
				<input name="username" placeholder="optional new username" />
				<input name="domain" placeholder="optional new primary domain" />
				<button type="submit">Import native export</button>
			</form>
			<form className="row" onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					const r = await api<any>('/api/v1/accounts/import/cpanel', { method: 'POST', body: JSON.stringify({
						root: fd.get('root'), username: fd.get('username'),
					}) })
					setMsg(`cPanel import queued for ${r.account?.username || r.resource_id}`)
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'cpanel import failed')
				}
			}}>
				<input name="root" placeholder="/var/tmp/cpmove-user" required />
				<input name="username" placeholder="username" required />
				<button type="submit">Import extracted cpmove</button>
			</form>
			{msg ? <p className="notice">{msg}</p> : null}
		</>
	)
}

function Packages () {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>('/api/v1/packages').then((r) => setItems(asList(r))) }, [])
	return (
		<>
			<header><h1>Packages</h1><p>Reusable CPU, memory, I/O and feature limits enforced through slices and quotas.</p></header>
			<form className="row" onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api('/api/v1/packages', { method: 'POST', body: JSON.stringify({
					name: fd.get('name'), disk_bytes: 10 << 30, bandwidth_bytes_monthly: 100 << 30,
					domains: 5, subdomains: 20, alias_domains: 10, databases: 5, database_users: 10,
					mailboxes: 20, mailbox_storage_bytes: 2 << 30, ftp_users: 5, cron_jobs: 10,
					application_instances: 3, backup_retention_days: 7, cpu_percent: 200,
					memory_bytes: 2 << 30, process_limit: 150, io_weight: 100, iops: 800,
					concurrent_web_requests: 100, email_daily_limit: 200,
				}) })
				const r = await api<{ items: any[] }>('/api/v1/packages')
				setItems(asList(r))
			}}>
				<input name="name" placeholder="package name" required />
				<button type="submit">Create package</button>
			</form>
			<table>
				<thead><tr><th>Name</th><th>CPU %</th><th>Memory</th><th>Disk</th></tr></thead>
				<tbody>{items.map((p) => <tr key={p.id}><td>{p.name}</td><td>{p.cpu_percent}</td><td>{fmtBytes(p.memory_bytes)}</td><td>{fmtBytes(p.disk_bytes)}</td></tr>)}</tbody>
			</table>
		</>
	)
}

function Resellers () {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>('/api/v1/resellers').then((r) => setItems(asList(r))) }, [])
	return (
		<>
			<header><h1>Resellers</h1><p>Delegated privileges. A reseller never sees root secrets or foreign customers.</p></header>
			<form className="row" onSubmit={async (e) => {
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
			}}>
				<input name="name" placeholder="reseller brand" required />
				<input name="username" placeholder="login username" required />
				<input name="email" type="email" placeholder="contact email" />
				<input name="password" type="password" placeholder="login password" required />
				<button type="submit">Create reseller</button>
			</form>
			{items.length === 0 ? <Empty title="No resellers yet" detail="Create one to delegate packages and customer accounts." /> : (
				<table><thead><tr><th>Name</th><th>Status</th></tr></thead>
					<tbody>{items.map((r) => <tr key={r.id}><td>{r.name}</td><td>{r.status}</td></tr>)}</tbody></table>
			)}
		</>
	)
}

function Monitor () {
	const [data, setData] = useState<any>(null)
	const [err, setErr] = useState('')
	useEffect(() => {
		api('/api/v1/server/monitor').then(setData).catch((e) => setErr(e.message))
	}, [])
	if (err) return <Empty title="Usage collector failed" detail={err} />
	if (!data) return <Empty title="Collecting account usage" detail="Walking each home directory through the control plane." />
	const items = data.accounts || []
	return (
		<>
			<header>
				<h1>Account usage</h1>
				<p>Disk, inodes, and process totals from the privileged agent walking each chrooted home. Failed jobs: {data.failed_jobs}. Certificates expiring within 14 days: {data.certs_expiring}.</p>
			</header>
			{items.length === 0 ? <Empty title="No account usage yet" detail="Provision an account, then reload this page." /> : (
				<table>
					<thead><tr><th>Account</th><th>Disk</th><th>Inodes</th><th>Processes</th><th>Memory</th><th>Collected</th></tr></thead>
					<tbody>
						{items.map((u: any) => (
							<tr key={u.account_id}>
								<td>{u.account_id}</td>
								<td>{fmtBytes(u.disk_bytes)}</td>
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

function Jobs () {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => {
		const load = () => api<{ items: any[] }>('/api/v1/jobs').then((r) => setItems(asList(r)))
		load()
		const id = setInterval(load, 1500)
		return () => clearInterval(id)
	}, [])
	return (
		<>
			<header><h1>Background jobs</h1><p>Durable PostgreSQL-style queue. Progress is observed, not guessed.</p></header>
			{items.length === 0 ? <Empty title="Queue is idle" detail="Provisioning, backups and certificate work will appear here." /> : (
				<table>
					<thead><tr><th>Type</th><th>State</th><th>%</th><th>Error</th></tr></thead>
					<tbody>{items.map((j) => <tr key={j.id}><td>{j.type}</td><td>{j.state}</td><td>{j.progress}</td><td>{j.last_error}</td></tr>)}</tbody>
				</table>
			)}
		</>
	)
}

function Audit () {
	const [items, setItems] = useState<any[]>([])
	const [err, setErr] = useState('')
	useEffect(() => { api<{ items: any[] }>('/api/v1/audit-events').then((r) => setItems(asList(r))).catch((e) => setErr(e.message)) }, [])
	if (err) return <Empty title="Audit unavailable" detail={err} />
	return (
		<>
			<header><h1>Privileged audit trail</h1><p>Secrets are redacted. Impersonation keeps the original actor.</p></header>
			<table>
				<thead><tr><th>When</th><th>Action</th><th>Resource</th><th>OK</th><th>IP</th></tr></thead>
				<tbody>{items.map((e) => <tr key={e.id}><td>{e.occurred_at}</td><td>{e.action}</td><td>{e.resource_type}</td><td>{e.success ? 'yes' : 'no'}</td><td>{e.source_ip}</td></tr>)}</tbody>
			</table>
		</>
	)
}

function Metric ({ label, value }: { label: string; value: string }) {
	return <article><p>{label}</p><strong>{value}</strong></article>
}

function Empty ({ title, detail }: { title: string; detail: string }) {
	return <section className="empty"><h2>{title}</h2><p>{detail}</p></section>
}

function fmtBytes (n: number) {
	if (!n) return '0 B'
	const u = ['B', 'KB', 'MB', 'GB', 'TB']
	let i = 0
	let v = n
	while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
	return `${v.toFixed(1)} ${u[i]}`
}

async function act (path: string, reload: () => Promise<void>, setMsg: (s: string) => void) {
	try {
		const r = await api<any>(path, { method: 'POST', body: '{}' })
		setMsg(`Queued ${r.operation_id}`)
		await reload()
	} catch (e) {
		setMsg(e instanceof Error ? e.message : 'failed')
	}
}
