import { useEffect, useState } from 'react'
import { NavLink, useParams } from 'react-router-dom'
import { api, asList } from './client'
import { Can } from './rbac'
import { act, Empty, fmtBytes, Notice, PageHeader } from './ui'

interface Account {
	id: string
	username: string
	primary_domain: string
	linux_uid: number
	status: string
	home_path: string
	package_id?: string
	reseller_id?: string
	ip_address?: string
	login_disabled?: boolean
}

const HUB = [
	{ id: 'lifecycle', label: 'Lifecycle' },
	{ id: 'usage', label: 'Usage' },
	{ id: 'websites', label: 'Websites' },
	{ id: 'dns', label: 'DNS' },
	{ id: 'mail', label: 'Mail' },
	{ id: 'databases', label: 'Databases' },
	{ id: 'tls', label: 'TLS' },
	{ id: 'files', label: 'Files' },
	{ id: 'cron', label: 'Cron' },
	{ id: 'backups', label: 'Backups' },
	{ id: 'jobs', label: 'Jobs' },
]

export function AccountDetail () {
	const [acc, setAcc] = useState<Account | null>(null)
	const [jobs, setJobs] = useState<any[]>([])
	const [sites, setSites] = useState<any[]>([])
	const [domains, setDomains] = useState<any[]>([])
	const [zones, setZones] = useState<any[]>([])
	const [records, setRecords] = useState<any[]>([])
	const [mailboxes, setMailboxes] = useState<any[]>([])
	const [mailDomains, setMailDomains] = useState<any[]>([])
	const [files, setFiles] = useState<any[]>([])
	const [backups, setBackups] = useState<any[]>([])
	const [usage, setUsage] = useState<any>(null)
	const [dbs, setDbs] = useState<any[]>([])
	const [certs, setCerts] = useState<any[]>([])
	const [crons, setCrons] = useState<any[]>([])
	const [packages, setPackages] = useState<any[]>([])
	const [msg, setMsg] = useState('')
	const { id = '' } = useParams()

	async function reload () {
		setAcc(await api<Account>(`/api/v1/accounts/${id}`))
		const j = await api<{ items: any[] }>('/api/v1/jobs')
		setJobs(asList(j).filter((x) =>
			x.resource_id === id || (x.payload && x.payload.account_id === id),
		))
		const [ws, ds, zs, mds, mbs, fl, bks, us, db, cr, cert, pkgs] = await Promise.all([
			api<{ items: any[] }>(`/api/v1/accounts/${id}/websites`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/domains`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/dns/zones`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/mail/domains`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/mail/mailboxes`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/files?path=/public_html`),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/backups`),
			api(`/api/v1/accounts/${id}/usage`).catch(() => null),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/databases`).catch(() => ({ items: [] })),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/cron`).catch(() => ({ items: [] })),
			api<{ items: any[] }>(`/api/v1/accounts/${id}/certificates`).catch(() => ({ items: [] })),
			api<{ items: any[] }>('/api/v1/packages').catch(() => ({ items: [] })),
		])
		setSites(asList(ws))
		setDomains(asList(ds))
		setZones(asList(zs))
		setMailDomains(asList(mds))
		setMailboxes(asList(mbs))
		setFiles(asList(fl))
		setBackups(asList(bks))
		setUsage(us)
		setDbs(asList(db))
		setCrons(asList(cr))
		setCerts(asList(cert))
		setPackages(asList(pkgs))
		if (asList(zs)[0]) {
			const rec = await api<{ items: any[] }>(
				`/api/v1/accounts/${id}/dns/zones/${asList(zs)[0].id}/records`,
			)
			setRecords(asList(rec))
		}
	}

	useEffect(() => {
		reload().catch((e) => setMsg(e.message))
	}, [id])

	useEffect(() => {
		let stop = false
		async function tick () {
			try {
				const bks = await api<{ items: any[] }>(`/api/v1/accounts/${id}/backups`)
				if (!stop) setBackups(asList(bks))
			} catch {
				// keep last list until the next poll
			}
		}
		const timer = window.setInterval(tick, 1500)
		return () => {
			stop = true
			window.clearInterval(timer)
		}
	}, [id])

	if (!acc) {
		return (
			<Empty
				title="Loading account"
				detail={msg || 'Reading desired and observed state.'}
			/>
		)
	}

	return (
		<>
			<PageHeader
				title={acc.username}
				detail="Account operations hub — summary, actions, then resource forms."
			/>
			<p className="hub-crumb">
				<NavLink to="/accounts">List Accounts</NavLink>
				{' · '}
				<NavLink to="/accounts/summary">Account Summary</NavLink>
			</p>
			<section className="summary-strip" aria-label="Account summary">
				<dl>
					<div><dt>Domain</dt><dd>{acc.primary_domain}</dd></div>
					<div><dt>Status</dt><dd>{acc.status}</dd></div>
					<div><dt>UID</dt><dd>{acc.linux_uid}</dd></div>
					<div><dt>Home</dt><dd>{acc.home_path}</dd></div>
					<div><dt>Package</dt><dd>{packages.find((p) => p.id === acc.package_id)?.name || acc.package_id || '—'}</dd></div>
					<div><dt>Disk</dt><dd>{usage ? fmtBytes(usage.disk_bytes || 0) : '—'}</dd></div>
				</dl>
			</section>
			<Notice>{msg}</Notice>
			<nav className="hub-jump" aria-label="Account operations">
				{HUB.map((h) => (
					<a key={h.id} href={`#${h.id}`}>{h.label}</a>
				))}
			</nav>
			<section className="action-groups" aria-label="Action groups">
				<div>
					<h2>Lifecycle</h2>
					<div className="row">
						<Can cap="accounts.suspend">
							<button type="button" onClick={() => act(`/api/v1/accounts/${id}/suspend`, reload, setMsg)}>
								Suspend
							</button>
							<button type="button" onClick={() => act(`/api/v1/accounts/${id}/unsuspend`, reload, setMsg)}>
								Unsuspend
							</button>
						</Can>
						<Can cap="accounts.terminate">
							<button
								type="button"
								onClick={() => {
									if (!window.confirm(`Terminate ${acc.username}? This removes the Linux user, websites, and mail.`))
										return
									act(`/api/v1/accounts/${id}/terminate`, reload, setMsg)
								}}
							>
								Terminate
							</button>
						</Can>
					</div>
				</div>
				<div>
					<h2>Recent jobs</h2>
					{jobs.length === 0 ? <p className="muted">No jobs yet.</p> : (
						<table>
							<thead>
								<tr><th>Type</th><th>State</th><th>%</th></tr>
							</thead>
							<tbody>
								{jobs.slice(0, 5).map((j) => (
									<tr key={j.id}>
										<td>{j.type}</td>
										<td>{j.state}</td>
										<td>{j.progress}</td>
									</tr>
								))}
							</tbody>
						</table>
					)}
				</div>
			</section>

			<section id="lifecycle">
				<h2>Export and migrate</h2>
				<div className="row">
					<button
						type="button"
						onClick={async () => {
							const exp = await api<any>(`/api/v1/accounts/${id}/export`)
							const blob = new Blob(
								[JSON.stringify(exp, null, 2)],
								{ type: 'application/json' },
							)
							const url = URL.createObjectURL(blob)
							const a = document.createElement('a')
							a.href = url
							a.download = `${acc.username}.hpm-account.json`
							a.click()
							URL.revokeObjectURL(url)
							setMsg('Native export downloaded')
						}}
					>
						Download native export
					</button>
				</div>
				<Can cap="accounts.modify">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								const r = await api<any>(`/api/v1/accounts/${id}`, {
									method: 'PATCH',
									body: JSON.stringify({
										package_id: fd.get('package_id'),
										primary_domain: fd.get('primary_domain'),
									}),
								})
								setMsg(`Modify queued ${r.operation_id}`)
								await reload()
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'modify failed')
							}
						}}
					>
						<select name="package_id" defaultValue={acc.package_id}>
							{packages.map((p) => (
								<option key={p.id} value={p.id}>{p.name}</option>
							))}
						</select>
						<input
							name="primary_domain"
							defaultValue={acc.primary_domain}
							aria-label="Primary domain"
						/>
						<button type="submit">Save account</button>
					</form>
				</Can>
				<Can cap="accounts.impersonate">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								const r = await api<any>(`/api/v1/accounts/${id}/impersonate`, {
									method: 'POST',
									body: JSON.stringify({ reason: fd.get('reason') }),
								})
								setMsg(`Impersonation token issued for Kelmor Control Plane API (Bearer). Kelmor Control login does not consume this token yet. ${r.token}`)
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'impersonate failed')
							}
						}}
					>
						<input name="reason" placeholder="impersonation reason" required />
						<button type="submit">Issue impersonation token</button>
					</form>
				</Can>
				<Can cap="accounts.create">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								const r = await api<any>(`/api/v1/accounts/${id}/migrate`, {
									method: 'POST',
									body: JSON.stringify({
										username: fd.get('migrate_username'),
										domain: fd.get('migrate_domain'),
									}),
								})
								setMsg(`Migrated to ${r.account?.username || r.resource_id}`)
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'migrate failed')
							}
						}}
					>
						<input name="migrate_username" placeholder="new username" required />
						<input name="migrate_domain" placeholder="new primary domain" required />
						<button type="submit">Migrate to new account</button>
					</form>
				</Can>
			</section>

			<section id="usage">
				<h2>Usage</h2>
				{usage ? (
					<p>
						Disk {fmtBytes(usage.disk_bytes || 0)}. Monthly transfer{' '}
						{fmtBytes(usage.bandwidth_bytes || 0)}. Inodes {usage.inode_count || 0}.
						Collected {usage.collected_at || '—'}.
					</p>
				) : (
					<p className="muted">No usage sample yet. Collector writes this through the real usage API.</p>
				)}
			</section>

			<section id="websites">
				<h2>Websites</h2>
				<Can cap="websites.write">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							await api(`/api/v1/accounts/${id}/websites`, {
								method: 'POST',
								body: JSON.stringify({
									domain_id: fd.get('domain_id'),
									runtime: fd.get('runtime'),
									document_root: acc.home_path + '/public_html',
								}),
							})
							setMsg('Website apply queued')
							await reload()
						}}
					>
						<select name="domain_id">
							{domains.map((d) => (
								<option key={d.id} value={d.id}>{d.ascii_fqdn}</option>
							))}
						</select>
						<select name="runtime">
							<option value="php">PHP</option>
							<option value="static">Static</option>
							<option value="node">Node</option>
							<option value="python">Python</option>
						</select>
						<button type="submit">Apply website</button>
					</form>
				</Can>
				<table>
					<thead>
						<tr><th>Runtime</th><th>Root</th><th>Enabled</th></tr>
					</thead>
					<tbody>
						{sites.map((s) => (
							<tr key={s.id}>
								<td>{s.runtime} {s.runtime_version}</td>
								<td>{s.document_root}</td>
								<td>{s.enabled ? 'yes' : 'no'}</td>
							</tr>
						))}
					</tbody>
				</table>
			</section>

			<section id="dns">
				<h2>DNS {zones[0] ? zones[0].name : ''}</h2>
				{zones[0] ? (
					<Can cap="dns.write">
						<form
							className="row"
							onSubmit={async (e) => {
								e.preventDefault()
								const fd = new FormData(e.currentTarget)
								try {
									await api(`/api/v1/accounts/${id}/dns/zones/${zones[0].id}/records`, {
										method: 'POST',
										body: JSON.stringify({
											name: fd.get('name'),
											type: fd.get('type'),
											content: fd.get('content'),
											ttl: Number(fd.get('ttl') || 300),
										}),
									})
									setMsg('DNS record queued')
									await reload()
								} catch (err) {
									setMsg(err instanceof Error ? err.message : 'dns failed')
								}
							}}
						>
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
							<button type="submit">Add DNS record</button>
						</form>
					</Can>
				) : <p>Zone appears after provisioning.</p>}
				{records.length === 0 ? <p>No records yet.</p> : (
					<table>
						<thead>
							<tr><th>Name</th><th>Type</th><th>Content</th><th></th></tr>
						</thead>
						<tbody>
							{records.map((r) => (
								<tr key={r.id}>
									<td>{r.name}</td>
									<td>{r.type}</td>
									<td>{r.content}</td>
									<td>
										<Can cap="dns.write">
											<button
												type="button"
												onClick={async () => {
													await api(`/api/v1/accounts/${id}/dns/zones/${zones[0].id}/records/${r.id}`, { method: 'DELETE' })
													setMsg('DNS record deleted')
													await reload()
												}}
											>
												Delete
											</button>
										</Can>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</section>

			<section id="mail">
				<h2>Mail</h2>
				<Can cap="mail.write">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							await api(`/api/v1/accounts/${id}/mail/mailboxes`, {
								method: 'POST',
								body: JSON.stringify({
									domain_id: fd.get('domain_id'),
									local_part: fd.get('local_part'),
									password: fd.get('password'),
								}),
							})
							setMsg('Mailbox queued')
							await reload()
						}}
					>
						<select name="domain_id">
							{mailDomains.map((d) => (
								<option key={d.id} value={d.id}>{d.ascii_fqdn}</option>
							))}
						</select>
						<input name="local_part" placeholder="local part" required />
						<input name="password" type="password" placeholder="mailbox password" required />
						<button type="submit">Create mailbox</button>
					</form>
				</Can>
				<table>
					<thead>
						<tr><th>Mailbox</th><th>Status</th></tr>
					</thead>
					<tbody>
						{mailboxes.map((m) => (
							<tr key={m.id}>
								<td>{m.local_part}</td>
								<td>{m.status}</td>
							</tr>
						))}
					</tbody>
				</table>
			</section>

			<section id="databases">
				<h2>Databases</h2>
				<Can cap="databases.write">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								await api(`/api/v1/accounts/${id}/databases`, {
									method: 'POST',
									body: JSON.stringify({
										name: fd.get('name'),
										engine: fd.get('engine'),
									}),
								})
								setMsg('Database queued')
								await reload()
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'database failed')
							}
						}}
					>
						<input name="name" placeholder="suffix" required />
						<select name="engine">
							<option value="mariadb">MariaDB</option>
							<option value="postgres">PostgreSQL</option>
						</select>
						<button type="submit">Create database</button>
					</form>
				</Can>
				{dbs.length === 0 ? (
					<p className="muted">No databases yet. Provision creates a default MariaDB when the agent path succeeds.</p>
				) : (
					<table>
						<thead>
							<tr><th>Name</th><th>Engine</th><th>Status</th></tr>
						</thead>
						<tbody>
							{dbs.map((d) => (
								<tr key={d.id}>
									<td>{d.name}</td>
									<td>{d.engine}</td>
									<td>{d.status}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</section>

			<section id="tls">
				<h2>TLS certificates</h2>
				<Can cap="websites.write">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								await api(`/api/v1/accounts/${id}/certificates`, {
									method: 'POST',
									body: JSON.stringify({ hostname: fd.get('hostname') }),
								})
								setMsg('Certificate request queued')
								await reload()
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'certificate failed')
							}
						}}
					>
						<input
							name="hostname"
							defaultValue={acc.primary_domain}
							required
						/>
						<button type="submit">Request certificate</button>
					</form>
				</Can>
				{certs.length === 0 ? (
					<p className="muted">No certificates recorded yet.</p>
				) : (
					<table>
						<thead>
							<tr><th>Hostname</th><th>Status</th><th>Issuer</th></tr>
						</thead>
						<tbody>
							{certs.map((c) => (
								<tr key={c.id || c.hostname}>
									<td>{c.hostname}</td>
									<td>{c.status}</td>
									<td>{c.issuer}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</section>

			<section id="files">
				<h2>Files in public_html</h2>
				<Can cap="files.write">
					<form
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							await api(`/api/v1/accounts/${id}/files`, {
								method: 'POST',
								body: JSON.stringify({
									path: fd.get('path'),
									content: fd.get('content'),
								}),
							})
							setMsg('File written through the agent')
							await reload()
						}}
					>
						<input name="path" defaultValue="/public_html/index.html" />
						<textarea name="content" rows={4} placeholder="file contents" required />
						<button type="submit">Write file</button>
					</form>
				</Can>
				<ul>
					{files.map((f) => (
						<li key={f.name}>
							{f.dir ? f.name + '/' : `${f.name} (${f.size})`}
						</li>
					))}
				</ul>
			</section>

			<section id="cron">
				<h2>Cron</h2>
				<Can cap="cron.write">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							try {
								await api(`/api/v1/accounts/${id}/cron`, {
									method: 'POST',
									body: JSON.stringify({
										schedule: fd.get('schedule'),
										command: fd.get('command'),
									}),
								})
								setMsg('Cron queued')
								await reload()
							} catch (err) {
								setMsg(err instanceof Error ? err.message : 'cron failed')
							}
						}}
					>
						<input name="schedule" placeholder="0 * * * *" required />
						<input name="command" placeholder="command" required />
						<button type="submit">Add cron</button>
					</form>
				</Can>
				{crons.length === 0 ? (
					<p className="muted">No cron jobs.</p>
				) : (
					<table>
						<thead>
							<tr><th>Schedule</th><th>Command</th></tr>
						</thead>
						<tbody>
							{crons.map((c) => (
								<tr key={c.id}>
									<td>{c.schedule}</td>
									<td>{c.command}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</section>

			<section id="backups">
				<h2>Backups</h2>
				<Can cap="backups.create">
					<form
						className="row"
						onSubmit={async (e) => {
							e.preventDefault()
							const fd = new FormData(e.currentTarget)
							const destination = String(fd.get('destination') || 'local')
							await api(`/api/v1/accounts/${id}/backups`, {
								method: 'POST',
								body: JSON.stringify({ kind: 'full', destination }),
							})
							setMsg(`Backup queued to ${destination}`)
							await reload()
						}}
					>
						<select name="destination">
							<option value="local">Local disk</option>
							<option value="sftp">Offsite SFTP</option>
							<option value="s3">S3-compatible</option>
						</select>
						<button type="submit">Queue encrypted backup</button>
					</form>
				</Can>
				{backups.length === 0 ? <p>No backup runs yet.</p> : (
					<ul>
						{backups.map((b) => (
							<li
								key={b.id}
								data-backup-id={b.id}
								data-backup-state={b.state}
								data-backup-destination={b.destination}
							>
								{b.kind} {b.state} {b.destination}{' '}
								{b.checksum ? b.checksum.slice(0, 12) : ''}
								{b.state === 'succeeded' ? (
									<Can cap="backups.restore">
										<button
											type="button"
											data-restore={b.id}
											onClick={async () => {
												await api(`/api/v1/accounts/${id}/restores`, {
													method: 'POST',
													body: JSON.stringify({ backup_id: b.id, mode: 'in_place' }),
												})
												setMsg(`Restore queued for ${b.destination} backup`)
												await reload()
											}}
										>
											Restore
										</button>
									</Can>
								) : null}
							</li>
						))}
					</ul>
				)}
			</section>

			<section id="jobs">
				<h2>Related jobs</h2>
				{jobs.length === 0 ? <p>No jobs for this account.</p> : (
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
			</section>
		</>
	)
}
