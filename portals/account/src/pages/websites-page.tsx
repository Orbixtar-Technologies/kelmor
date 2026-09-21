import { useEffect, useMemo, useRef, useState } from 'react'
import { api, asList } from '../client'
import {
	installedPHPVersions,
	isInstalledPHPVersion,
	isSupportedPHPVersion,
	missingHostPHPVersion,
	type PHPRuntime,
} from '../php-runtimes'
import { Can } from '../rbac'
import { RequestSequence } from '../request-sequence'

interface WebsiteItem {
	id: string
	domain_id?: string
	document_root?: string
	runtime?: string
	runtime_version?: string
	enabled?: boolean
}

interface DomainItem {
	id: string
	ascii_fqdn?: string
	type?: string
}

export function WebsitesPage ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<WebsiteItem[]>([])
	const [domains, setDomains] = useState<DomainItem[]>([])
	const [phpRuntimes, setPhpRuntimes] = useState<PHPRuntime[]>([])
	const [msg, setMsg] = useState('')
	const requests = useRef(new RequestSequence()).current
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const installed = useMemo(() => installedPHPVersions(phpRuntimes), [phpRuntimes])
	const missingHostVersions = useMemo(() => {
		const found = new Set<string>()
		for (const item of items) {
			if (String(item.runtime || 'php') !== 'php') continue
			const missing = missingHostPHPVersion(item.runtime_version || '', installed)
			if (missing) found.add(missing)
		}
		return [...found]
	}, [installed, items])

	const load = async (requestedAccountId: string) => {
		const request = requests.begin('websites')
		const [sites, nextDomains, runtimes] = await Promise.all([
			api<{ items: WebsiteItem[] }>(`/api/v1/accounts/${requestedAccountId}/websites`),
			api<{ items: DomainItem[] }>(`/api/v1/accounts/${requestedAccountId}/domains`),
			api<{ items?: PHPRuntime[] }>('/api/v1/server/runtimes').catch(() => ({ items: [] })),
		])
		if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
		setItems(asList(sites))
		setDomains(nextDomains.items || [])
		setPhpRuntimes(asList(runtimes))
	}

	useEffect(() => { void load(accountId) }, [accountId])

	function phpVersionMessage (version: string) {
		if (!installed.length) return 'php-fpm is not installed'
		if (!isSupportedPHPVersion(version)) return `Unsupported PHP version ${version}`
		if (!isInstalledPHPVersion(version, installed)) return `php-fpm ${version} is not installed`
		return ''
	}

	async function persistRuntime (website: WebsiteItem, runtimeVersion: string) {
		const problem = phpVersionMessage(runtimeVersion)
		if (problem) {
			setMsg(problem)
			return
		}
		await api(`/api/v1/accounts/${accountId}/websites`, {
			method: 'POST',
			body: JSON.stringify({
				domain_id: website.domain_id,
				runtime: website.runtime || 'php',
				runtime_version: runtimeVersion,
				document_root: website.document_root,
			}),
		})
		setMsg('PHP / runtime change queued.')
		await load(accountId)
	}

	return (
		<>
			<h1>Websites</h1>
			<p>PHP-FPM, static files, or a Node/Python unit applied through the privileged agent.</p>
			{missingHostVersions.length ? (
				<p role="alert">PHP {missingHostVersions.join(', ')} is not installed on this host. Choose an installed version.</p>
			) : null}
			<Can cap="websites.write">
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				const runtimeVersion = String(data.get('runtime_version') || '')
				if (String(data.get('runtime')) === 'php') {
					const problem = phpVersionMessage(runtimeVersion)
					if (problem) {
						setMsg(problem)
						return
					}
				}
				await api(`/api/v1/accounts/${accountId}/websites`, { method: 'POST', body: JSON.stringify({
					domain_id: data.get('domain_id'), runtime: data.get('runtime'), runtime_version: runtimeVersion || undefined,
				}) })
				setMsg('Website apply queued')
				await load(accountId)
			}}>
				<select name="domain_id">{domains.map((domain) => <option key={domain.id} value={domain.id}>{domain.ascii_fqdn}</option>)}</select>
				<select name="runtime">
					<option value="php">PHP</option>
					<option value="static">Static</option>
					<option value="node">Node</option>
					<option value="python">Python</option>
				</select>
				<select name="runtime_version" aria-label="PHP version">
					{installed.map((version) => <option key={version} value={version}>{version}</option>)}
				</select>
				<button type="submit">Apply website</button>
			</form>
			</Can>
			<Can cap="applications.write">
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				await api(`/api/v1/accounts/${accountId}/wordpress`, { method: 'POST', body: JSON.stringify({
					website_id: data.get('website_id'),
					title: data.get('title'),
					admin_user: data.get('admin_user'),
					admin_password: data.get('admin_password'),
					admin_email: data.get('admin_email'),
				}) })
				setMsg('WordPress install queued')
				await load(accountId)
			}}>
				<select name="website_id">{items.map((item) => <option key={item.id} value={item.id}>{item.document_root}</option>)}</select>
				<input name="title" placeholder="Site title" defaultValue="My site" required />
				<input name="admin_user" placeholder="wp admin" defaultValue="wpadmin" required />
				<input name="admin_password" type="password" minLength={8} required />
				<input name="admin_email" type="email" placeholder="owner@example.test" required />
				<button type="submit">Install WordPress</button>
			</form>
			</Can>
			{msg ? <p>{msg}</p> : null}
			{items.length === 0 ? <p>No websites yet. Provisioning creates one after the account job finishes.</p> : (
				<table>
					<thead><tr><th>Hostname</th><th>Runtime</th><th>Document root</th><th>State</th></tr></thead>
					<tbody>{items.map((item) => {
						const domain = domains.find((entry) => entry.id === item.domain_id)
						const recorded = item.runtime_version || '8.3'
						const selectedVersion = isInstalledPHPVersion(recorded, installed) ? recorded : ''
						return (
							<tr key={item.id}>
								<td>{domain?.ascii_fqdn || item.domain_id}</td>
								<td>
									{item.runtime} {item.runtime_version}
									<Can cap="websites.write">
										<label>
											PHP version for {item.document_root}
											<select
												value={selectedVersion}
												onChange={(event) => void persistRuntime(item, event.target.value)}
											>
												{missingHostPHPVersion(recorded, installed) ? <option value="">Choose installed PHP</option> : null}
												{installed.map((version) => <option key={version} value={version}>{version}</option>)}
											</select>
										</label>
									</Can>
								</td>
								<td>{item.document_root}</td>
								<td>{item.enabled === false ? 'disabled' : 'ready'}</td>
							</tr>
						)
					})}</tbody>
				</table>
			)}
		</>
	)
}
