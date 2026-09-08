import { useEffect, useState } from 'react'
import { NavLink, Route, Routes } from 'react-router-dom'
import { api, asList, clearToken, getToken, setToken } from './client'

interface Me {
	user: { username: string; roles: string[] }
	actor: { account_ids?: string[]; accountIDs?: string[]; AccountIDs?: string[] }
}

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [error, setError] = useState('')

	useEffect(() => {
		if (!getToken()) return
		api<Me>('/api/v1/me').then(setMe).catch(() => clearToken())
	}, [])

	if (!me) {
		return (
			<main className="auth">
				<section>
					<h1>Account Portal</h1>
					<p>Manage websites, mail, DNS and files for your hosting account. Server hardware controls are not available here.</p>
					<form onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						try {
							const r = await api<{ token: string }>('/api/v1/auth/login', {
								method: 'POST',
								body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
							})
							setToken(r.token)
							setMe(await api<Me>('/api/v1/me'))
						} catch (err) {
							setError(err instanceof Error ? err.message : 'Login failed')
						}
					}}>
						<label>Username<input name="username" defaultValue="livehost" /></label>
						<label>Password<input name="password" type="password" defaultValue="TenantPass!2026" /></label>
						{error ? <p className="error">{error}</p> : null}
						<button type="submit">Open my hosting</button>
					</form>
				</section>
			</main>
		)
	}

	const ids = (me.actor as any).account_ids || []
	const accountId = ids[0] || ''
	return (
		<div className="shell">
			<header className="top">
				<strong>Account Portal</strong>
				<span>{me.user.username}</span>
				<button type="button" onClick={() => { clearToken(); setMe(null) }}>Sign out</button>
			</header>
			<div className="body">
				<nav>
					<NavLink to="/" end>Dashboard</NavLink>
					<NavLink to="/websites">Websites</NavLink>
					<NavLink to="/domains">Domains</NavLink>
					<NavLink to="/dns">DNS</NavLink>
					<NavLink to="/email">Email</NavLink>
					<NavLink to="/databases">Databases</NavLink>
					<NavLink to="/files">Files</NavLink>
					<NavLink to="/ssl">SSL/TLS</NavLink>
					<NavLink to="/backups">Backups</NavLink>
					<NavLink to="/cron">Cron</NavLink>
				</nav>
				<main>
					{!accountId ? <p>No hosting account is attached to this login yet. Ask the administrator to provision one, then sign in as that username.</p> : (
						<Routes>
							<Route path="/" element={<Dash accountId={accountId} />} />
							<Route path="/websites" element={<Websites accountId={accountId} />} />
							<Route path="/domains" element={<Domains accountId={accountId} />} />
							<Route path="/dns" element={<DNS accountId={accountId} />} />
							<Route path="/email" element={<Email accountId={accountId} />} />
							<Route path="/databases" element={<Databases accountId={accountId} />} />
							<Route path="/files" element={<Files accountId={accountId} />} />
							<Route path="/ssl" element={<List path={`/api/v1/accounts/${accountId}/certificates`} title="Certificates" />} />
							<Route path="/backups" element={<Backups accountId={accountId} />} />
							<Route path="/cron" element={<Cron accountId={accountId} />} />
						</Routes>
					)}
				</main>
			</div>
		</div>
	)
}

function Dash ({ accountId }: { accountId: string }) {
	const [acc, setAcc] = useState<any>(null)
	const [err, setErr] = useState('')
	useEffect(() => {
		api(`/api/v1/accounts/${accountId}`).then(setAcc).catch((e) => setErr(e.message))
	}, [accountId])
	if (err) return <p className="error">{err}</p>
	if (!acc) return <p>Loading account…</p>
	return (
		<>
			<h1>{acc.primary_domain}</h1>
			<p>Status {acc.status}. Home {acc.home_path}. Linux UID {acc.linux_uid}.</p>
		</>
	)
}

function Websites ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	const [domains, setDomains] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const load = () => Promise.all([
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/websites`).then((r) => setItems(asList(r))),
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/domains`).then((r) => setDomains(r.items || [])),
	])
	useEffect(() => { load() }, [accountId])
	return (
		<>
			<h1>Websites</h1>
			<p>PHP-FPM, static files, or a Node/Python unit applied through the privileged agent.</p>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					await api(`/api/v1/accounts/${accountId}/websites`, { method: 'POST', body: JSON.stringify({
						domain_id: fd.get('domain_id'), runtime: fd.get('runtime'),
					}) })
					setMsg('Website apply queued')
					await load()
				} catch (err) { setMsg(err instanceof Error ? err.message : 'failed') }
			}}>
				<select name="domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn}</option>)}</select>
				<select name="runtime">
					<option value="php">PHP 8.3</option>
					<option value="static">Static</option>
					<option value="node">Node</option>
					<option value="python">Python</option>
				</select>
				<button type="submit">Apply website</button>
			</form>
			{msg ? <p>{msg}</p> : null}
			{items.length === 0 ? <p>No websites yet. Provisioning creates one after the account job finishes.</p> : (
				<table>
					<thead><tr><th>Runtime</th><th>Document root</th><th>State</th></tr></thead>
					<tbody>{items.map((it) => (
						<tr key={it.id}><td>{it.runtime} {it.runtime_version}</td><td>{it.document_root}</td><td>{it.enabled === false ? 'disabled' : 'ready'}</td></tr>
					))}</tbody>
				</table>
			)}
		</>
	)
}

function List ({ path, title }: { path: string; title: string }) {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>(path).then((r) => setItems(asList(r))) }, [path])
	return (
		<>
			<h1>{title}</h1>
			{items.length === 0 ? <p>Nothing provisioned in this module yet. Create a resource or wait for the account job to finish.</p> : (
				<table>
					<thead><tr><th>Resource</th><th>Detail</th><th>State</th></tr></thead>
					<tbody>
						{items.map((it) => (
							<tr key={it.id}>
								<td>{it.fqdn || it.hostname || it.name || it.local_part || it.runtime || it.id}</td>
								<td>{it.document_root || it.engine || it.kind || it.runtime_version || ''}</td>
								<td>{it.status || (it.enabled === false ? 'disabled' : 'ready')}</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	)
}

function Domains ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const load = () => api<{ items: any[] }>(`/api/v1/accounts/${accountId}/domains`).then((r) => setItems(asList(r)))
	useEffect(() => { load() }, [accountId])
	return (
		<>
			<h1>Domains</h1>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					await api(`/api/v1/accounts/${accountId}/domains`, { method: 'POST', body: JSON.stringify({ fqdn: fd.get('fqdn'), type: 'addon' }) })
					setMsg('Provisioning queued')
					await load()
				} catch (err) { setMsg(err instanceof Error ? err.message : 'failed') }
			}}>
				<input name="fqdn" placeholder="addon.example.com" required />
				<button type="submit">Attach domain</button>
			</form>
			{msg ? <p>{msg}</p> : null}
			<ul>{items.map((d) => <li key={d.id}>{d.ascii_fqdn} ({d.type}) {d.status}</li>)}</ul>
		</>
	)
}

function DNS ({ accountId }: { accountId: string }) {
	const [zones, setZones] = useState<any[]>([])
	const [records, setRecords] = useState<any[]>([])
	useEffect(() => {
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/dns/zones`).then(async (r) => {
			setZones(r.items)
			if (r.items[0]) {
				const rec = await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/dns/zones/${r.items[0].id}/records`)
				setRecords(rec.items)
			}
		})
	}, [accountId])
	return (
		<>
			<h1>DNS</h1>
			{zones.length === 0 ? <p>No zones yet. They appear after account provisioning completes.</p> : (
				<table>
					<thead><tr><th>Name</th><th>Type</th><th>Content</th></tr></thead>
					<tbody>{records.map((r) => <tr key={r.id}><td>{r.name}</td><td>{r.type}</td><td>{r.content}</td></tr>)}</tbody>
				</table>
			)}
		</>
	)
}

function Email ({ accountId }: { accountId: string }) {
	const [boxes, setBoxes] = useState<any[]>([])
	const [domains, setDomains] = useState<any[]>([])
	useEffect(() => {
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/mailboxes`).then((r) => setBoxes(r.items))
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/domains`).then((r) => setDomains(r.items))
	}, [accountId])
	return (
		<>
			<h1>Email</h1>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/mail/mailboxes`, { method: 'POST', body: JSON.stringify({
					domain_id: fd.get('domain_id'), local_part: fd.get('local_part'), password: fd.get('password'),
				}) })
				setBoxes((await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/mailboxes`)).items)
			}}>
				<select name="domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn || d.id}</option>)}</select>
				<input name="local_part" placeholder="mailbox" required />
				<input name="password" type="password" required />
				<button type="submit">Create mailbox</button>
			</form>
			<ul>{boxes.map((b) => <li key={b.id}>{b.local_part} — {b.status}</li>)}</ul>
		</>
	)
}

function Databases ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>(`/api/v1/accounts/${accountId}/databases`).then((r) => setItems(asList(r))) }, [accountId])
	return (
		<>
			<h1>Databases</h1>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/databases`, { method: 'POST', body: JSON.stringify({ name: fd.get('name'), engine: fd.get('engine') }) })
				setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/databases`)))
			}}>
				<input name="name" placeholder="store" required />
				<select name="engine"><option value="mariadb">MariaDB</option><option value="postgres">PostgreSQL</option></select>
				<button type="submit">Create database</button>
			</form>
			<ul>{items.map((d) => <li key={d.id}>{d.name} ({d.engine}) {d.status}</li>)}</ul>
		</>
	)
}

function Files ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	const [path, setPath] = useState('/')
	useEffect(() => {
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/files?path=${encodeURIComponent(path)}`).then((r) => setItems(asList(r)))
	}, [accountId, path])
	return (
		<>
			<h1>Files</h1>
			<p>Path {path}</p>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/files`, {
					method: 'POST',
					body: JSON.stringify({ path: fd.get('path'), content: fd.get('content') }),
				})
				setPath(String(fd.get('path') || path))
			}}>
				<input name="path" defaultValue={path === '/' ? '/public_html/note.txt' : path} />
				<textarea name="content" rows={6} placeholder="file contents" required />
				<button type="submit">Write file</button>
			</form>
			<ul>
				{items.map((f) => (
					<li key={f.name}>
						{f.dir ? <button type="button" className="link" onClick={() => setPath((p) => (p.endsWith('/') ? p : p + '/') + f.name)}>{f.name}/</button> : <span>{f.name} ({f.size})</span>}
					</li>
				))}
			</ul>
		</>
	)
}

function Backups ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>(`/api/v1/accounts/${accountId}/backups`).then((r) => setItems(asList(r))) }, [accountId])
	return (
		<>
			<h1>Backups</h1>
			<button type="button" onClick={async () => {
				await api(`/api/v1/accounts/${accountId}/backups`, { method: 'POST', body: JSON.stringify({ kind: 'full', destination: 'local' }) })
				setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/backups`)))
			}}>Create full backup</button>
			<ul>{items.map((b) => (
				<li key={b.id}>
					{b.kind} {b.state} {b.destination} {b.checksum ? `sha256:${b.checksum.slice(0, 12)}` : ''}
					{b.state === 'succeeded' ? (
						<button type="button" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/restores`, { method: 'POST', body: JSON.stringify({ backup_id: b.id, mode: 'in_place' }) })
							setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/backups`)))
						}}>Restore</button>
					) : null}
				</li>
			))}</ul>
		</>
	)
}

function Cron ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	useEffect(() => { api<{ items: any[] }>(`/api/v1/accounts/${accountId}/cron`).then((r) => setItems(asList(r))) }, [accountId])
	return (
		<>
			<h1>Cron jobs</h1>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/cron`, { method: 'POST', body: JSON.stringify({
					schedule: fd.get('schedule'), command: fd.get('command'), enabled: true,
				}) })
				setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/cron`)))
			}}>
				<input name="schedule" defaultValue="0 * * * *" />
				<input name="command" placeholder="php cron.php" required />
				<button type="submit">Add job</button>
			</form>
			<ul>{items.map((c) => <li key={c.id}>{c.schedule} {c.command}</li>)}</ul>
		</>
	)
}
