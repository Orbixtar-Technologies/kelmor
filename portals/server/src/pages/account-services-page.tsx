import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountTabs, Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, valueOf } from '../helpers'
import { groupAccountServices, resourceManagePath, resourcePrimaryLabel, serviceActionLabel } from './account-service-groups'
import { backupDestinationReadiness, backupScopeSummary } from './backup-copy'
import { DatabaseConnectionPanel } from './database-connection-panel'
import { RequestSequence } from '../request-sequence'
import { useCapabilities } from '../rbac'
import type { Account, Package, ResourceItem } from '../types'

interface ServiceDefinition {
	id: string
	label: string
	endpoint: string
	columns: string[]
	isDeletable?: boolean
	readCapability: string
	writeCapability?: string
}

const services: ServiceDefinition[] = [
	{ id: 'websites', label: 'Websites', endpoint: 'websites', columns: ['runtime', 'document_root', 'enabled'], isDeletable: true, readCapability: 'websites.read', writeCapability: 'websites.write' },
	{ id: 'domains', label: 'Domains', endpoint: 'domains', columns: ['ascii_fqdn', 'type', 'status'], isDeletable: true, readCapability: 'domains.read', writeCapability: 'domains.write' },
	{ id: 'databases', label: 'Databases', endpoint: 'databases', columns: ['name', 'engine', 'status'], isDeletable: true, readCapability: 'databases.read', writeCapability: 'databases.write' },
	{ id: 'mail-domains', label: 'Mail domains', endpoint: 'mail/domains', columns: ['domain_id', 'catchall_policy', 'status'], readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'mailboxes', label: 'Mailboxes', endpoint: 'mail/mailboxes', columns: ['local_part', 'quota_bytes', 'status'], isDeletable: true, readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'aliases', label: 'Aliases', endpoint: 'mail/aliases', columns: ['address', 'destination'], isDeletable: true, readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'certificates', label: 'Certificates', endpoint: 'certificates', columns: ['hostname', 'kind', 'status', 'not_after'], readCapability: 'websites.read', writeCapability: 'websites.write' },
	{ id: 'files', label: 'Files', endpoint: 'files?path=/public_html', columns: ['name', 'size', 'dir'], readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'backups', label: 'Backups & restore', endpoint: 'backups', columns: ['kind', 'state', 'destination', 'size_bytes'], readCapability: 'backups.read', writeCapability: 'backups.create' },
	{ id: 'cron', label: 'Cron', endpoint: 'cron', columns: ['schedule', 'command', 'enabled'], isDeletable: true, readCapability: 'cron.read', writeCapability: 'cron.write' },
	{ id: 'ssh', label: 'SSH / SFTP', endpoint: 'ssh-keys', columns: ['label', 'fingerprint', 'created_at'], isDeletable: true, readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'ftp', label: 'FTP', endpoint: 'ftp', columns: ['username', 'home_path', 'status'], isDeletable: true, readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'tokens', label: 'API tokens', endpoint: 'api-tokens', columns: ['name', 'prefix', 'scope', 'expires_at'], isDeletable: true, readCapability: 'api_tokens.read', writeCapability: 'api_tokens.write' },
	{ id: 'applications', label: 'Applications', endpoint: 'applications', columns: ['runtime', 'working_directory', 'status'], isDeletable: true, readCapability: 'applications.read', writeCapability: 'applications.write' },
]

const accountTokenCapabilities = [
	'domains.read', 'domains.write', 'websites.read', 'websites.write',
	'databases.read', 'databases.write', 'mail.read', 'mail.write',
	'files.read', 'files.write', 'backups.read', 'backups.create',
	'cron.read', 'cron.write',
]

export function AccountServicesPage () {
	const { id = '' } = useParams()
	const [searchParams, setSearchParams] = useSearchParams()
	const [account, setAccount] = useState<Account | null>(null)
	const [requestAccountId, setRequestAccountId] = useState(id)
	const [loadedAccountId, setLoadedAccountId] = useState('')
	const [resources, setResources] = useState<Record<string, ResourceItem[]>>({})
	const [errors, setErrors] = useState<Record<string, string>>({})
	const [loading, setLoading] = useState(true)
	const [message, setMessage] = useState('')
	const [pkg, setPkg] = useState<Package | null>(null)
	const [restoreBackup, setRestoreBackup] = useState<ResourceItem | null>(null)
	const [detail, setDetail] = useState<ResourceItem | null>(null)
	const [credentials, setCredentials] = useState<Record<string, string> | null>(null)
	const [toolUrls, setToolUrls] = useState<{ phpmyadmin_url?: string } | null>(null)
	const requests = useRef(new RequestSequence()).current
	const currentAccountId = useRef(id)
	currentAccountId.current = id
	const capabilities = useCapabilities()
	const visibleServices = useMemo(() => services.filter((service) => capabilities[service.readCapability]), [capabilities])
	const requestedService = searchParams.get('service') || 'websites'
	const definition = visibleServices.find((service) => service.id === requestedService) || visibleServices[0]
	const active = definition?.id || requestedService

	const load = useCallback(() => {
		const requestedAccountId = id
		const accountRequest = requests.begin('account')
		const serviceRequests = visibleServices.map((service) => ({
			service,
			token: requests.begin(`service:${service.id}`),
		}))
		setLoading(true)
		setRequestAccountId(requestedAccountId)
		setAccount(null)
		setLoadedAccountId('')
		setResources({})
		setErrors({})
		setMessage('')
		setRestoreBackup(null)
		setPkg(null)
		setCredentials(null)
		setToolUrls(null)

		const accountLoad = api<Account>(`/api/v1/accounts/${requestedAccountId}`).then((result) => {
			if (!requests.isCurrent(accountRequest) || currentAccountId.current !== requestedAccountId) return
			setAccount(result)
			setLoadedAccountId(requestedAccountId)
			api<{ items: Package[] }>('/api/v1/packages').then((packages) => {
				if (!requests.isCurrent(accountRequest) || currentAccountId.current !== requestedAccountId) return
				setPkg(asList(packages).find((entry) => entry.id === result.package_id) || null)
			}).catch(() => {
				if (requests.isCurrent(accountRequest) && currentAccountId.current === requestedAccountId) setPkg(null)
			})
		}).catch((error) => {
			if (!requests.isCurrent(accountRequest) || currentAccountId.current !== requestedAccountId) return
			setErrors((current) => ({ ...current, account: messageFrom(error) }))
		})
		const resourceLoads = serviceRequests.map(({ service, token }) => (
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/${service.endpoint}`).then((result) => {
				if (!requests.isCurrent(token) || currentAccountId.current !== requestedAccountId) return
				setResources((current) => ({ ...current, [service.id]: asList(result) }))
			}).catch((error) => {
				if (!requests.isCurrent(token) || currentAccountId.current !== requestedAccountId) return
				setErrors((current) => ({ ...current, [service.id]: messageFrom(error) }))
			})
		))
		Promise.allSettled([accountLoad, ...resourceLoads]).finally(() => {
			if (requests.isCurrent(accountRequest) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [id, requests, visibleServices])
	useEffect(load, [load])
	useEffect(() => {
		if (!id || requestedService !== 'databases') return
		const request = requests.begin('database-connection')
		api<{ credentials: Record<string, string> }>(`/api/v1/accounts/${id}/databases/credentials?engine=mariadb`).then((result) => {
			if (requests.isCurrent(request) && currentAccountId.current === id) setCredentials(result.credentials)
		}).catch(() => {
			if (requests.isCurrent(request) && currentAccountId.current === id) setCredentials(null)
		})
		api<{ phpmyadmin_url?: string }>(`/api/v1/accounts/${id}/admin-tools`).then((result) => {
			if (requests.isCurrent(request) && currentAccountId.current === id) setToolUrls(result)
		}).catch(() => {
			if (requests.isCurrent(request) && currentAccountId.current === id) setToolUrls(null)
		})
	}, [id, requestedService, requests])

	async function refreshService (requestedAccountId: string, serviceId: string) {
		const service = visibleServices.find((entry) => entry.id === serviceId)
		if (!service) return
		const request = requests.begin(`service:${service.id}`)
		try {
			const result = await api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/${service.endpoint}`)
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setResources((current) => ({ ...current, [service.id]: asList(result) }))
			setErrors((current) => ({ ...current, [service.id]: '' }))
		} catch (error) {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setErrors((current) => ({ ...current, [service.id]: messageFrom(error) }))
		}
	}
	async function create (endpoint: string, body: Record<string, unknown>) {
		const requestedAccountId = id
		const serviceId = active
		setMessage('')
		try {
			const result = await api<{ operation_id?: string; token?: string }>(`/api/v1/accounts/${requestedAccountId}/${endpoint}`, { method: 'POST', body: JSON.stringify(body) })
			if (currentAccountId.current !== requestedAccountId) return
			setMessage(result.token ? `Token created: ${result.token}. Copy it now.` : result.operation_id ? 'Queued. Track progress in Activity.' : 'Resource created.')
			await refreshService(requestedAccountId, serviceId)
			if (endpoint === 'applications' && serviceId !== 'applications') await refreshService(requestedAccountId, 'applications')
		} catch (error) {
			if (currentAccountId.current === requestedAccountId) setMessage(messageFrom(error))
		}
	}
	async function remove (segment: string, resourceId: string) {
		if (!window.confirm('Delete this resource? Dependent resources may block removal.')) return
		const requestedAccountId = id
		const serviceId = active
		try {
			await api(`/api/v1/accounts/${requestedAccountId}/${segment}/${resourceId}`, { method: 'DELETE' })
			if (currentAccountId.current !== requestedAccountId) return
			setMessage('Delete operation queued.')
			await refreshService(requestedAccountId, serviceId)
		} catch (error) {
			if (currentAccountId.current === requestedAccountId) setMessage(messageFrom(error))
		}
	}
	async function patchMailDomain (mailDomainId: string, catchallPolicy: string) {
		const requestedAccountId = id
		try {
			await api(`/api/v1/accounts/${requestedAccountId}/mail/domains/${mailDomainId}`, { method: 'PATCH', body: JSON.stringify({ catchall_policy: catchallPolicy }) })
			if (currentAccountId.current !== requestedAccountId) return
			setMessage('Mail routing update queued.')
			await refreshService(requestedAccountId, 'mail-domains')
		} catch (error) {
			if (currentAccountId.current === requestedAccountId) setMessage(messageFrom(error))
		}
	}
	async function restore () {
		if (!restoreBackup) return
		await create('restores', { backup_id: restoreBackup.id, mode: 'in_place' })
		setRestoreBackup(null)
	}

	const isCurrentRequest = requestAccountId === id
	const currentAccount = isCurrentRequest && loadedAccountId === id ? account : null
	if (!currentAccount && (loading || !isCurrentRequest)) return <><PageHeader title="Account Services" description="Loading account-linked services." /><LoadingState /></>
	if (!currentAccount) return <><PageHeader title="Account Services" description="Account-linked operations." /><ErrorState error={errors.account || 'Account unavailable.'} onRetry={load} /></>
	const items = definition ? resources[definition.id] || [] : []
	const groups = groupAccountServices(visibleServices)
	const managePath = definition ? resourceManagePath(definition.id, id) : undefined
	return (
		<>
			<PageHeader title={`${currentAccount.username}: Services`} description="Create a resource, then open an existing one for details and routine actions." />
			<AccountTabs id={id} />
			<div className="service-switcher">
				<label className="service-select-label" htmlFor="account-service">Service
					<select id="account-service" value={active} onChange={(event) => setSearchParams({ service: event.target.value }, { replace: true })}>
						{groups.map((group) => <optgroup key={group.id} label={group.label}>{group.items.map((service) => <option key={service.id} value={service.id}>{service.label} ({resources[service.id]?.length ?? 0})</option>)}</optgroup>)}
					</select>
				</label>
				<div className="service-groups" role="tablist" aria-label="Account service">
					{groups.map((group) => (
						<section key={group.id} className="service-group">
							<h3>{group.label}</h3>
							<div>
								{group.items.map((service) => (
									<button key={service.id} type="button" role="tab" aria-selected={definition?.id === service.id} onClick={() => setSearchParams({ service: service.id }, { replace: true })}>
										{service.label}<span className="service-count">{resources[service.id]?.length ?? 0}</span>
									</button>
								))}
							</div>
						</section>
					))}
				</div>
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{definition?.id === 'databases' ? <DatabaseConnectionPanel accountId={id} credentials={credentials} phpmyadminUrl={toolUrls?.phpmyadmin_url} /> : null}
			<section className="panel service-panel">
				<div className="section-heading"><div><h2>{definition?.label} · {items.length}</h2><p>Queued changes appear in Activity. Use Details to inspect an existing resource.</p></div></div>
				{definition?.id === 'mail-domains' && capabilities['mail.write'] ? <MailDomainForm domains={items} onPatch={patchMailDomain} /> : definition?.writeCapability && capabilities[definition.writeCapability] ? <ServiceCreateForm service={definition.id} account={currentAccount} resources={resources} canListWebsites={Boolean(capabilities['websites.read'])} onCreate={create} /> : <p className="subtle">Available resources are read-only for your current role.</p>}
				{definition?.id === 'backups' ? <p className="subtle">{backupScopeSummary(pkg?.backup_retention_days)}</p> : null}
				{errors[active] ? <ErrorState title={`${definition?.label} unavailable`} error={errors[active]} onRetry={load} /> : null}
				{loading ? <LoadingState /> : null}
				{!loading && !errors[active] && definition ? <div className="table-wrap"><table className="dense-table"><thead><tr><th>Resource</th>{definition.columns.map((column) => <th key={column} scope="col">{column.replaceAll('_', ' ')}</th>)}<th scope="col">Actions</th></tr></thead>
					<tbody>{items.map((item) => <tr key={item.id}><td><button type="button" className="link-button resource-name" onClick={() => setDetail(item)}>{resourcePrimaryLabel(definition.id, item)}</button></td>{definition.columns.map((column) => <td key={column}>{column === 'status' || column === 'state' ? <StatusBadge value={valueOf(item, column)} /> : column.includes('bytes') || column === 'size' ? formatBytes(Number(item[column])) : valueOf(item, column)}</td>)}<td><div className="row-actions">
						<button type="button" className="link-button" onClick={() => setDetail(item)}>Details</button>
						{managePath ? <Link to={managePath}>Manage</Link> : null}
						{active === 'websites' && capabilities['applications.write'] && ['node', 'python'].includes(String(item.runtime)) ? <button type="button" className="link-button" onClick={() => create('applications', { website_id: item.id, runtime: item.runtime })}>Deploy application</button> : null}
						{active === 'backups' && item.state === 'succeeded' && capabilities['backups.restore'] ? <button type="button" className="link-button" onClick={() => setRestoreBackup(item)}>Review restore</button> : null}
						{definition.isDeletable && definition.writeCapability && capabilities[definition.writeCapability] ? <button type="button" className="link-button danger-text" onClick={() => remove(definition.endpoint, item.id)}>Delete</button> : null}
					</div></td></tr>)}</tbody>
				</table></div> : null}
				{!loading && !errors[active] && !items.length ? <EmptyState
					title={definition?.id === 'backups' ? 'No backups yet' : `No ${definition?.label.toLocaleLowerCase()} yet`}
					detail={definition?.id === 'backups'
						? 'Queue a local backup to create the first encrypted archive. Restore and import stay available from Transfers after a backup succeeds.'
						: `Use ${serviceActionLabel(definition?.id || '')} above where available. New items stay listed here after they are queued.`}
					action={definition?.id === 'backups' ? <Link className="button-link secondary-link" to="/transfers">Open Transfers & restore</Link> : undefined}
				/> : null}
			</section>
			<Dialog open={Boolean(detail)} title={detail ? resourcePrimaryLabel(definition?.id || '', detail) : 'Resource details'} onClose={() => setDetail(null)}>
				{detail ? <dl className="detail-list">{Object.entries(detail).filter(([, value]) => value !== null && value !== undefined && value !== '').map(([key, value]) => <div key={key}><dt>{key.replaceAll('_', ' ')}</dt><dd>{typeof value === 'object' ? JSON.stringify(value) : String(value)}</dd></div>)}</dl> : null}
				<footer className="dialog-form-actions">
					<button type="button" className="secondary" onClick={() => setDetail(null)}>Close</button>
					{managePath ? <Link className="button-link" to={managePath}>Manage</Link> : null}
				</footer>
			</Dialog>
			<Dialog open={Boolean(restoreBackup)} title="Review in-place restore" onClose={() => setRestoreBackup(null)}>
				<p>This queues an in-place restore from backup <strong>{restoreBackup?.id}</strong>. Current account files and service configuration may be replaced.</p>
				<dl className="detail-list"><div><dt>Destination</dt><dd>{restoreBackup ? valueOf(restoreBackup, 'destination') : '—'}</dd></div><div><dt>Checksum</dt><dd>{restoreBackup ? valueOf(restoreBackup, 'checksum') : '—'}</dd></div></dl>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setRestoreBackup(null)}>Cancel</button><button type="button" onClick={restore}>Queue restore</button></footer>
			</Dialog>
		</>
	)
}

function BackupCreateForm ({ onCreate }: { onCreate: (endpoint: string, body: Record<string, unknown>) => Promise<void> }) {
	const [destination, setDestination] = useState('local')
	const readiness = backupDestinationReadiness(destination)
	return (
		<form className="inline-form" onSubmit={(event) => {
			event.preventDefault()
			onCreate('backups', { kind: 'full', destination })
		}}>
			<label>Destination
				<select name="destination" value={destination} onChange={(event) => setDestination(event.target.value)}>
					<option value="local">local</option>
					<option value="sftp">sftp</option>
					<option value="s3">s3</option>
				</select>
			</label>
			<button type="submit">Queue encrypted backup</button>
			<p className="subtle">{readiness.label}. {readiness.detail} Restore an existing archive from the list below or from Transfers.</p>
		</form>
	)
}

function MailDomainForm ({ domains, onPatch }: { domains: ResourceItem[]; onPatch: (id: string, policy: string) => Promise<void> }) {
	return <form className="inline-form" onSubmit={(event) => { event.preventDefault(); const data = new FormData(event.currentTarget); onPatch(String(data.get('mail_domain_id')), String(data.get('catchall_policy'))) }}><label>Mail domain<select name="mail_domain_id">{domains.map((domain) => <option key={domain.id} value={domain.id}>{valueOf(domain, 'domain_id')}</option>)}</select></label><label>Catch-all policy<input name="catchall_policy" placeholder="reject, discard, or local part" required /></label><button type="submit" disabled={!domains.length}>Update routing</button></form>
}

interface ServiceCreateFormProps {
	service: string
	account: Account
	resources: Record<string, ResourceItem[]>
	canListWebsites: boolean
	onCreate: (endpoint: string, body: Record<string, unknown>) => Promise<void>
}

export function ServiceCreateForm ({ service, account, resources, canListWebsites, onCreate }: ServiceCreateFormProps) {
	const capabilities = useCapabilities()
	function submit (event: React.FormEvent<HTMLFormElement>, endpoint: string, payload: (data: FormData) => Record<string, unknown>) {
		event.preventDefault()
		onCreate(endpoint, payload(new FormData(event.currentTarget)))
	}
	const domains = resources.domains || []
	if (service === 'domains') return <form className="inline-form" onSubmit={(event) => submit(event, 'domains', (data) => ({ fqdn: data.get('fqdn'), type: data.get('type'), dns_managed: true }))}><label>Domain<input name="fqdn" placeholder="shop.example.com" required /></label><label>Type<select name="type"><option value="addon">Addon</option><option value="subdomain">Subdomain</option><option value="alias">Alias</option></select></label><button type="submit">{serviceActionLabel('domains')}</button></form>
	if (service === 'websites') return <form className="inline-form" onSubmit={(event) => submit(event, 'websites', (data) => ({ domain_id: data.get('domain_id'), runtime: data.get('runtime'), document_root: `${account.home_path}/public_html` }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Runtime<select name="runtime"><option>php</option><option>static</option><option>node</option><option>python</option></select></label><button type="submit">{serviceActionLabel('websites')}</button></form>
	if (service === 'databases') return <form className="inline-form" onSubmit={(event) => submit(event, 'databases', (data) => ({ name: data.get('name'), engine: data.get('engine') }))}><label>Name<input name="name" required /></label><label>Engine<select name="engine"><option>mariadb</option><option>postgres</option><option>mysql</option></select></label><button type="submit">{serviceActionLabel('databases')}</button></form>
	if (service === 'mailboxes') return <><form className="inline-form" onSubmit={(event) => submit(event, 'mail/mailboxes', (data) => ({ domain_id: data.get('domain_id'), local_part: data.get('local_part'), password: data.get('password') }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Local part<input name="local_part" required /></label><label>Password<input name="password" type="password" required /></label><button type="submit">Create mailbox</button></form><p className="subtle">Mailbox storage quota is enforced by the account package and cannot be overridden here.</p></>
	if (service === 'aliases') return <form className="inline-form" onSubmit={(event) => submit(event, 'mail/aliases', (data) => ({ domain_id: data.get('domain_id'), address: data.get('address'), destination: data.get('destination') }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Address<input name="address" required /></label><label>Destination<input name="destination" required /></label><button type="submit">Create alias</button></form>
	if (service === 'certificates') return <form className="inline-form" onSubmit={(event) => submit(event, 'certificates', (data) => ({ hostname: data.get('hostname') }))}><label>Hostname<input name="hostname" defaultValue={account.primary_domain} required /></label><button type="submit">Request certificate</button></form>
	if (service === 'files') return <form className="stack-form" onSubmit={(event) => submit(event, 'files', (data) => ({ path: data.get('path'), content: data.get('content') }))}><label>Path<input name="path" defaultValue="/public_html/index.html" required /></label><label>Contents<textarea name="content" rows={4} required /></label><button type="submit">Write file</button></form>
	if (service === 'backups') return <BackupCreateForm onCreate={onCreate} />
	if (service === 'cron') return <form className="inline-form" onSubmit={(event) => submit(event, 'cron', (data) => ({ schedule: data.get('schedule'), command: data.get('command'), working_directory: account.home_path, enabled: true }))}><label>Schedule<input name="schedule" defaultValue="0 * * * *" required /></label><label>Command<input name="command" required /></label><button type="submit">Add cron job</button></form>
	if (service === 'ssh') return <><form className="stack-form" onSubmit={(event) => submit(event, 'ssh-keys', (data) => ({ label: data.get('label'), public_key: data.get('public_key') }))}><label>Key label<input name="label" required /></label><label>Public key<textarea name="public_key" rows={3} required /></label><button type="submit">Add SSH key</button></form><form className="inline-form" onSubmit={(event) => submit(event, 'sftp-password', (data) => ({ password: data.get('password') }))}><label>SFTP password<input name="password" type="password" required /></label><button type="submit">Set SFTP password</button></form></>
	if (service === 'ftp') return <form className="inline-form" onSubmit={(event) => submit(event, 'ftp', (data) => ({ username: data.get('username'), password: data.get('password'), home_path: data.get('home_path') }))}><label>Username<input name="username" required /></label><label>Password<input name="password" type="password" required /></label><label>Home path<input name="home_path" defaultValue={`${account.home_path}/public_html`} required /></label><button type="submit">Create FTP user</button></form>
	if (service === 'tokens') return <form className="stack-form" onSubmit={(event) => submit(event, 'api-tokens', (data) => ({ name: data.get('name'), scope: 'account', capabilities: data.getAll('capability') }))}><label>Name<input name="name" required /></label><p className="subtle"><strong>Scope:</strong> Account. Tokens cannot be granted host-wide or arbitrary capabilities.</p><fieldset><legend>Account-safe capabilities</legend><div className="checkbox-grid">{accountTokenCapabilities.filter((capability) => capabilities[capability]).map((capability) => <label className="checkbox-label" key={capability}><input type="checkbox" name="capability" value={capability} />{capability}</label>)}</div></fieldset><button type="submit">Create token</button></form>
	if (service === 'applications') {
		if (!canListWebsites) return <p className="subtle">Application creation is disabled because your role cannot list account websites.</p>
		if (!(resources.websites || []).some((item) => ['node', 'python'].includes(String(item.runtime)))) return <p className="subtle">Create a Node or Python website before creating its application deployment.</p>
		return <p className="subtle">Choose <strong>Deploy application</strong> beside an account-owned Node or Python website. Runtime settings and working directory are derived from that website.</p>
	}
	return null
}
