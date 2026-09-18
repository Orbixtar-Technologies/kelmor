import { useEffect, useRef, useState } from 'react'
import { Navigate, NavLink, Route, Routes } from 'react-router-dom'
import { api, APIClientError, asList, clearToken, consumeImpersonationSession, getToken, setToken } from './client'
import { ControlHubPage } from './control-hub-page'
import { canSeeControlHub, controlHubEntryPath, controlHubs } from './control-hubs'
import { BackupsPage } from './pages/backups-page'
import { DNSPage } from './pages/dns-page'
import { FilesPage } from './pages/files-page'
import { PasswordChangeForm } from './pages/password-change-form'
import { WebsitesPage } from './pages/websites-page'
import { createPendingGuard } from './pending-submit'
import { RequestSequence } from './request-sequence'
import { Can, CapProvider } from './rbac'
import { RequireCap } from './require-cap'

const websitesHub = controlHubs.find((hub) => hub.id === 'websites')
const domainsHub = controlHubs.find((hub) => hub.id === 'domains')
const backupsHub = controlHubs.find((hub) => hub.id === 'backups')

interface Me {
	user: { username: string; roles: string[] }
	actor: { account_ids?: string[]; capabilities?: Record<string, boolean> }
}

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [error, setError] = useState('')
	const [username, setUsername] = useState('livehost')
	const [password, setPassword] = useState('TenantPass!2026')
	const [passwordChangeRequired, setPasswordChangeRequired] = useState(false)
	const [checking, setChecking] = useState(() => {
		consumeImpersonationSession()
		return Boolean(getToken())
	})
	const session = useRef(new RequestSequence()).current
	const loginGuard = useRef(createPendingGuard()).current

	useEffect(() => {
		if (!getToken()) {
			setChecking(false)
			return
		}
		const request = session.begin('me')
		api<Me>('/api/v1/me').then((next) => {
			if (session.isCurrent(request)) setMe(next)
		}).catch(() => {
			if (session.isCurrent(request)) clearToken()
		}).finally(() => {
			if (session.isCurrent(request)) setChecking(false)
		})
	}, [session])

	if (checking) {
		return <main className="auth"><section><p className="eyebrow">Kelmor</p><h1>Kelmor Control</h1><p>Restoring your hosting session…</p></section></main>
	}

	if (!me) {
		return (
			<main className="auth">
				<section>
					<p className="eyebrow">Kelmor</p>
					<h1>Kelmor Control</h1>
					{passwordChangeRequired ? (
						<>
							<p>Your administrator requires you to choose a new password before continuing.</p>
							<PasswordChangeForm
								username={username}
								currentPassword={password}
								onSignedIn={(next) => {
									setPassword('')
									setPasswordChangeRequired(false)
									setMe(next)
								}}
							/>
							{error ? <p className="error" role="alert">{error}</p> : null}
						</>
					) : (
						<>
							<p>Tenant self-serve for the Kelmor product family. Host hardware controls live in Kelmor Director.</p>
							<form onSubmit={async (event) => {
								event.preventDefault()
								if (!loginGuard.tryStart()) return
								setError('')
								try {
									const login = await api<{ token: string }>('/api/v1/auth/login', {
										method: 'POST',
										body: JSON.stringify({ username, password }),
									})
									setToken(login.token)
									setMe(await api<Me>('/api/v1/me'))
									setPassword('')
								} catch (requestError) {
									if (requestError instanceof APIClientError && requestError.code === 'PASSWORD_CHANGE_REQUIRED') {
										setPasswordChangeRequired(true)
										return
									}
									setError(requestError instanceof Error ? requestError.message : 'Login failed')
								} finally {
									loginGuard.finish()
								}
							}}>
								<label>Username<input name="username" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" /></label>
								<label>Password<input name="password" value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" /></label>
								{error ? <p className="error" role="alert">{error}</p> : null}
								<button type="submit">Open my hosting</button>
							</form>
						</>
					)}
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
				<strong>Kelmor Control</strong>
				<span>{me.user.username}</span>
				<button type="button" onClick={() => { clearToken(); setMe(null) }}>Sign out</button>
			</header>
			<div className="body">
				<nav>
					{controlHubs.filter((hub) => canSeeControlHub(hub, caps)).map((hub) => (
						<NavLink key={hub.id} to={controlHubEntryPath(hub, caps)} end={hub.path === '/'}>
							{hub.label}
						</NavLink>
					))}
				</nav>
				<main>
					{!accountId ? <p>No hosting account is attached to this login yet. Ask the administrator to provision one, then sign in as that username.</p> : (
						<Routes>
							<Route path="/" element={<Dash accountId={accountId} />} />
							<Route path="/websites" element={<RequireCap cap="websites.read">{websitesHub ? <ControlHubPage hub={websitesHub} capabilities={caps} pages={{ sites: <WebsitesPage accountId={accountId} />, ssl: <Certificates accountId={accountId} /> }} /> : null}</RequireCap>} />
							<Route path="/domains" element={domainsHub && (caps['domains.read'] || caps['dns.read']) ? <ControlHubPage hub={domainsHub} capabilities={caps} pages={{ domains: <RequireCap cap="domains.read"><Domains accountId={accountId} /></RequireCap>, dns: <RequireCap cap="dns.read"><DNSPage accountId={accountId} /></RequireCap> }} /> : <RequireCap cap="domains.read"><Domains accountId={accountId} /></RequireCap>} />
							<Route path="/email" element={<RequireCap cap="mail.read"><Email accountId={accountId} /></RequireCap>} />
							<Route path="/databases" element={<RequireCap cap="databases.read"><Databases accountId={accountId} /></RequireCap>} />
							<Route path="/files" element={<RequireCap cap="files.read"><FilesPage accountId={accountId} /></RequireCap>} />
							<Route path="/backups" element={backupsHub && (caps['backups.read'] || caps['cron.read']) ? <ControlHubPage hub={backupsHub} capabilities={caps} pages={{ backups: <RequireCap cap="backups.read"><BackupsPage accountId={accountId} /></RequireCap>, cron: <RequireCap cap="cron.read"><Cron accountId={accountId} /></RequireCap> }} /> : <RequireCap cap="backups.read"><BackupsPage accountId={accountId} /></RequireCap>} />
							<Route path="/ssl" element={<Navigate to="/websites?tab=ssl" replace />} />
							<Route path="/dns" element={<Navigate to="/domains?tab=dns" replace />} />
							<Route path="/cron" element={<Navigate to="/backups?tab=cron" replace />} />
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
	const [usage, setUsage] = useState<any>(null)
	const [err, setErr] = useState('')
	useEffect(() => {
		Promise.all([
			api(`/api/v1/accounts/${accountId}`),
			api(`/api/v1/accounts/${accountId}/usage`).catch(() => null),
		]).then(([a, u]) => {
			setAcc(a)
			setUsage(u)
		}).catch((e) => setErr(e.message))
	}, [accountId])
	if (err) return <p className="error">{err}</p>
	if (!acc) return <p>Loading account…</p>
	return (
		<>
			<h1>{acc.primary_domain}</h1>
			<p>Status {acc.status}. Home {acc.home_path}. Linux UID {acc.linux_uid}.</p>
			{usage ? (
				<p>Disk {usage.disk_bytes || 0} bytes. Monthly transfer {usage.bandwidth_bytes || 0} bytes (nginx body bytes this calendar month). Sites return HTTP 509 when the package bandwidth cap is reached.</p>
			) : null}
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
					await api(`/api/v1/accounts/${accountId}/domains`, { method: 'POST', body: JSON.stringify({ fqdn: fd.get('fqdn'), type: fd.get('type') || 'addon' }) })
					setMsg('Provisioning queued')
					await load()
				} catch (err) { setMsg(err instanceof Error ? err.message : 'failed') }
			}}>
				<input name="fqdn" placeholder="addon.example.com" required />
				<select name="type" defaultValue="addon">
					<option value="addon">Addon (own site)</option>
					<option value="subdomain">Subdomain</option>
					<option value="alias">Alias (park on primary)</option>
				</select>
				<button type="submit">Attach domain</button>
			</form>
			</Can>
			{msg ? <p>{msg}</p> : null}
			<ul>{items.map((d) => (
				<li key={d.id}>
					{d.ascii_fqdn} ({d.type}) {d.status}
					<Can cap="domains.write">
						{d.type === 'primary' ? ' — primary' : (
							<button type="button" onClick={async () => {
								try {
									await api(`/api/v1/accounts/${accountId}/domains/${d.id}`, { method: 'DELETE' })
									setMsg('Domain retire queued')
									await load()
								} catch (err) { setMsg(err instanceof Error ? err.message : 'failed') }
							}}>Remove</button>
						)}
					</Can>
				</li>
			))}</ul>
		</>
	)
}


function Email ({ accountId }: { accountId: string }) {
	const [boxes, setBoxes] = useState<any[]>([])
	const [domains, setDomains] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	async function load () {
		setBoxes((await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/mailboxes`)).items)
		setDomains((await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/domains`)).items)
	}
	useEffect(() => { load() }, [accountId])
	return (
		<>
			<h1>Email</h1>
			<p>Catch-all policy is reject (bounce), discard (store in a discard mailbox), or a local part that already exists.</p>
			<Can cap="mail.write">
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				await api(`/api/v1/accounts/${accountId}/mail/mailboxes`, { method: 'POST', body: JSON.stringify({
					domain_id: fd.get('domain_id'), local_part: fd.get('local_part'), password: fd.get('password'),
				}) })
				await load()
			}}>
				<select name="domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn || d.id}</option>)}</select>
				<input name="local_part" placeholder="mailbox" required />
				<input name="password" type="password" required />
				<button type="submit">Create mailbox</button>
			</form>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					await api(`/api/v1/accounts/${accountId}/mail/domains/${fd.get('mail_domain_id')}`, {
						method: 'PATCH',
						body: JSON.stringify({ catchall_policy: fd.get('catchall_policy') }),
					})
					setMsg('Catch-all queued')
					await load()
				} catch (err) {
					setMsg(err instanceof Error ? err.message : 'failed')
				}
			}}>
				<select name="mail_domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn || d.id}</option>)}</select>
				<input name="catchall_policy" placeholder="reject, discard, or info" defaultValue="reject" required />
				<button type="submit">Set catch-all</button>
			</form>
			<form onSubmit={async (e) => {
				e.preventDefault()
				const fd = new FormData(e.currentTarget)
				try {
					await api(`/api/v1/accounts/${accountId}/mail/aliases`, { method: 'POST', body: JSON.stringify({
						domain_id: fd.get('domain_id'), address: fd.get('address'), destination: fd.get('destination'),
					}) })
					setMsg('Alias queued')
					await load()
				} catch (err) { setMsg(err instanceof Error ? err.message : 'failed') }
			}}>
				<select name="domain_id">{domains.map((d) => <option key={d.id} value={d.id}>{d.ascii_fqdn || d.id}</option>)}</select>
				<input name="address" placeholder="sales" required />
				<input name="destination" placeholder="info or other@example.test" required />
				<button type="submit">Add alias</button>
			</form>
			</Can>
			{msg ? <p>{msg}</p> : null}
			<ul>{domains.map((d) => <li key={d.id}>{d.ascii_fqdn} catch-all {d.catchall_policy}</li>)}</ul>
			<ul>{boxes.map((b) => (
				<li key={b.id}>
					{b.local_part} — {b.status}
					<Can cap="mail.write">
						<button type="button" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/mail/mailboxes/${b.id}`, { method: 'DELETE' })
							await load()
						}}>Delete mailbox</button>
					</Can>
				</li>
			))}</ul>
			<MailAliases accountId={accountId} domains={domains} />
		</>
	)
}

function MailAliases ({ accountId, domains }: { accountId: string; domains: any[] }) {
	const [items, setItems] = useState<any[]>([])
	const load = () => api<{ items: any[] }>(`/api/v1/accounts/${accountId}/mail/aliases`).then((r) => setItems(asList(r)))
	useEffect(() => { load() }, [accountId])
	return (
		<>
			<h2>Aliases</h2>
			<ul>{items.map((a) => (
				<li key={a.id}>
					{a.address} → {a.destination}
					<Can cap="mail.write">
						<button type="button" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/mail/aliases/${a.id}`, { method: 'DELETE' })
							await load()
						}}>Delete</button>
					</Can>
				</li>
			))}
			</ul>
			{domains.length === 0 ? <p>No mail domains yet.</p> : null}
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
			<ul>{items.map((d) => (
				<li key={d.id}>
					{d.name} ({d.engine}) {d.status}
					<Can cap="databases.write">
						<button type="button" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/databases/${d.id}`, { method: 'DELETE' })
							setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/databases`)))
						}}>Delete</button>
					</Can>
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
			<ul>{items.map((c) => (
				<li key={c.id}>
					{c.schedule} {c.command}
					<Can cap="cron.write">
						<button type="button" className="link" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/cron/${c.id}`, { method: 'DELETE' })
							setItems(asList(await api<{ items: any[] }>(`/api/v1/accounts/${accountId}/cron`)))
						}}>Remove</button>
					</Can>
				</li>
			))}</ul>
		</>
	)
}
