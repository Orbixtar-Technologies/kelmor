import { useEffect, useState } from 'react'
import { NavLink, Route, Routes } from 'react-router-dom'
import { api, asList, clearToken, getToken, setToken } from './client'
import { Can, CapProvider } from './rbac'

interface Me {
	user: { username: string; roles: string[] }
	actor: { account_ids?: string[]; capabilities?: Record<string, boolean> }
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

	const ids = me.actor?.account_ids || []
	const accountId = ids[0] || ''
	const caps = me.actor?.capabilities || {}
	return (
		<CapProvider caps={caps}>
		<div className="shell">
			<header className="top">
				<strong>Account Portal</strong>
				<span>{me.user.username}</span>
				<button type="button" onClick={() => { clearToken(); setMe(null) }}>Sign out</button>
			</header>
			<div className="body">
				<nav>
					<NavLink to="/" end>Dashboard</NavLink>
					{caps['websites.read'] ? <NavLink to="/websites">Websites</NavLink> : null}
					{caps['domains.read'] ? <NavLink to="/domains">Domains</NavLink> : null}
					{caps['dns.read'] ? <NavLink to="/dns">DNS</NavLink> : null}
					{caps['mail.read'] ? <NavLink to="/email">Email</NavLink> : null}
					{caps['databases.read'] ? <NavLink to="/databases">Databases</NavLink> : null}
					{caps['files.read'] ? <NavLink to="/files">Files</NavLink> : null}
					{caps['websites.read'] ? <NavLink to="/ssl">SSL/TLS</NavLink> : null}
					{caps['backups.read'] ? <NavLink to="/backups">Backups</NavLink> : null}
					{caps['cron.read'] ? <NavLink to="/cron">Cron</NavLink> : null}
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
							<Route path="/ssl" element={<Certificates accountId={accountId} />} />
							<Route path="/backups" element={<Backups accountId={accountId} />} />
							<Route path="/cron" element={<Cron accountId={accountId} />} />
						</Routes>
					)}
				</main>
			</div>
		</div>
		</CapProvider>
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
			<Can cap="websites.write">
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
			</Can>
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

function Certificates ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const load = () => api<{ items: any[] }>(`/api/v1/accounts/${accountId}/certificates`).then((r) => setItems(asList(r)))
	useEffect(() => { load() }, [accountId])
	return (
		<>
			<h1>SSL/TLS</h1>
			<p>Issues a certificate through the control plane (Pebble in the lab, Let’s Encrypt on a public host).</p>
			<Can cap="websites.write">
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					await api(`/api/v1/accounts/${accountId}/certificates`, {
						method: 'POST',
						body: JSON.stringify({ hostname: fd.get('hostname') }),
					})
					setMsg('Certificate request queued')
					await load()
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'failed')
				}
			}}>
				<input name="hostname" placeholder="www.example.test" required />
				<button type="submit">Request certificate</button>
			</form>
			</Can>
			{msg ? <p>{msg}</p> : null}
			{items.length === 0 ? <p>No certificates yet.</p> : (
				<table>
					<thead><tr><th>Hostname</th><th>Kind</th><th>State</th></tr></thead>
					<tbody>{items.map((c) => (
						<tr key={c.id}><td>{c.hostname}</td><td>{c.kind}</td><td>{c.status}</td></tr>
					))}</tbody>
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
			<Can cap="domains.write">
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
			</Can>
			{msg ? <p>{msg}</p> : null}
			<ul>{items.map((d) => <li key={d.id}>{d.ascii_fqdn} ({d.type}) {d.status}</li>)}</ul>
		</>
	)
}

function DNS ({ accountId }: { accountId: string }) {
	const [zones, setZones] = useState<any[]>([])
	const [records, setRecords] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	async function load () {
		const r = await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/dns/zones`)
		setZones(r.items)
		if (r.items[0]) {
			const rec = await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/dns/zones/${r.items[0].id}/records`)
			setRecords(rec.items)
		} else {
			setRecords([])
		}
	}
	useEffect(() => { load().catch((e) => setMsg(e instanceof Error ? e.message : 'failed')) }, [accountId])
	return (
		<>
			<h1>DNS</h1>
			{zones.length === 0 ? <p>No zones yet. They appear after account provisioning completes.</p> : (
				<>
					<Can cap="dns.write">
					<form onSubmit={async (e) => {
						e.preventDefault()
						const fd = new FormData(e.currentTarget)
						try {
							await api(`/api/v1/accounts/${accountId}/dns/zones/${zones[0].id}/records`, {
								method: 'POST',
								body: JSON.stringify({
									name: fd.get('name'),
									type: fd.get('type'),
									content: fd.get('content'),
									ttl: Number(fd.get('ttl') || 300),
								}),
							})
							setMsg('Record queued for sync')
							await load()
						} catch (err) {
							setMsg(err instanceof Error ? err.message : 'failed')
						}
					}}>
						<input name="name" placeholder="www" required />
						<select name="type">
							<option value="A">A</option>
							<option value="AAAA">AAAA</option>
							<option value="CNAME">CNAME</option>
							<option value="MX">MX</option>
							<option value="TXT">TXT</option>
							<option value="NS">NS</option>
						</select>
						<input name="content" placeholder="203.0.113.10" required />
						<input name="ttl" type="number" defaultValue={300} min={60} />
						<button type="submit">Add record</button>
					</form>
					</Can>
					{msg ? <p>{msg}</p> : null}
					<table>
						<thead><tr><th>Name</th><th>Type</th><th>Content</th><th></th></tr></thead>
						<tbody>{records.map((r) => (
							<tr key={r.id}>
								<td>{r.name}</td><td>{r.type}</td><td>{r.content}</td>
								<td>
									<Can cap="dns.write">
										<button type="button" onClick={async () => {
											await api(`/api/v1/accounts/${accountId}/dns/zones/${zones[0].id}/records/${r.id}`, { method: 'DELETE' })
											setMsg('Record deleted')
											await load()
										}}>Delete</button>
									</Can>
								</td>
							</tr>
						))}</tbody>
					</table>
				</>
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
			<Can cap="mail.write">
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
			</Can>
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
			<Can cap="databases.write">
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
			</Can>
			<ul>{items.map((d) => <li key={d.id}>{d.name} ({d.engine}) {d.status}</li>)}</ul>
		</>
	)
}

function Files ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<any[]>([])
	const [ftpUsers, setFtpUsers] = useState<any[]>([])
	const [path, setPath] = useState('/')
	const loadFTP = () => api<{ items: any[] }>(`/api/v1/accounts/${accountId}/ftp`).then((r) => setFtpUsers(asList(r)))
	useEffect(() => {
		api<{ items: any[] }>(`/api/v1/accounts/${accountId}/files?path=${encodeURIComponent(path)}`).then((r) => setItems(asList(r)))
	}, [accountId, path])
	useEffect(() => { loadFTP() }, [accountId])
	return (
		<>
			<h1>Files</h1>
			<p>Path {path}</p>
			<Can cap="files.write">
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/sftp-password`, {
					method: 'POST',
					body: JSON.stringify({ password: fd.get('password') }),
				})
			}}>
				<label>SFTP password (chrooted to your home)
					<input name="password" type="password" minLength={8} required />
				</label>
				<button type="submit">Set SFTP password</button>
			</form>
			<h2>FTP users</h2>
			<p>Virtual FTP logins map to this account and chroot to public_html (port 21).</p>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/ftp`, {
					method: 'POST',
					body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
				})
				e.currentTarget.reset()
				await loadFTP()
			}}>
				<input name="username" placeholder="siteftp" required />
				<input name="password" type="password" minLength={8} required />
				<button type="submit">Create FTP user</button>
			</form>
			<ul>
				{ftpUsers.map((f) => (
					<li key={f.id}>
						{f.username} — {f.home_path}
						<button type="button" className="link" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/ftp/${f.id}`, { method: 'DELETE' })
							await loadFTP()
						}}>Remove</button>
					</li>
				))}
			</ul>
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
			</Can>
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
			<Can cap="backups.create">
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/backups`, {
					method: 'POST',
					body: JSON.stringify({ kind: 'full', destination: String(fd.get('destination') || 'local') }),
				})
				setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/backups`)))
			}}>
				<select name="destination">
					<option value="local">Local disk</option>
					<option value="sftp">Offsite SFTP</option>
					<option value="s3">S3-compatible</option>
				</select>
				<button type="submit">Create full backup</button>
			</form>
			</Can>
			<ul>{items.map((b) => (
				<li key={b.id}>
					{b.kind} {b.state} {b.destination} {b.checksum ? `sha256:${b.checksum.slice(0, 12)}` : ''}
					{b.state === 'succeeded' ? (
						<Can cap="backups.restore">
						<button type="button" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/restores`, { method: 'POST', body: JSON.stringify({ backup_id: b.id, mode: 'in_place' }) })
							setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/backups`)))
						}}>Restore</button>
						</Can>
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
			<Can cap="cron.write">
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
			</Can>
			<ul>{items.map((c) => <li key={c.id}>{c.schedule} {c.command}</li>)}</ul>
		</>
	)
}
