import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountTabs, Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCapabilities } from '../rbac'
import type { Account, ResourceItem } from '../types'

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
	const [restoreBackup, setRestoreBackup] = useState<ResourceItem | null>(null)
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

		const accountLoad = api<Account>(`/api/v1/accounts/${requestedAccountId}`).then((result) => {
			if (!requests.isCurrent(accountRequest) || currentAccountId.current !== requestedAccountId) return
			setAccount(result)
			setLoadedAccountId(requestedAccountId)
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
			setMessage(result.token ? `Token created: ${result.token}. Copy it now.` : result.operation_id ? `Queued ${result.operation_id}.` : 'Resource created.')
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
	return (
		<>
			<PageHeader title={`${currentAccount.username}: Account Services`} description="Websites, domains, databases, mail, certificates, files, access, backups, and automation." actions={capabilities['dns.read'] ? <Link className="button-link secondary-link" to={`/dns?account=${id}`}>Manage DNS</Link> : undefined} />
			<AccountTabs id={id} />
			<div className="service-tabs" role="tablist" aria-label="Account service">
				{visibleServices.map((service) => <button key={service.id} type="button" role="tab" aria-selected={definition?.id === service.id} onClick={() => setSearchParams({ service: service.id }, { replace: true })}>{service.label}<span>{resources[service.id]?.length ?? '—'}</span></button>)}
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel service-panel">
				<div className="section-heading"><div><h2>{definition?.label}</h2><p>Changes are queued through Kelmor’s capability-enforced API.</p></div></div>
				{definition?.id === 'mail-domains' && capabilities['mail.write'] ? <MailDomainForm domains={items} onPatch={patchMailDomain} /> : definition?.writeCapability && capabilities[definition.writeCapability] ? <ServiceCreateForm service={definition.id} account={currentAccount} resources={resources} canListWebsites={Boolean(capabilities['websites.read'])} onCreate={create} /> : <p className="subtle">Available resources are read-only for your current role.</p>}
				{errors[active] ? <ErrorState title={`${definition?.label} unavailable`} error={errors[active]} onRetry={load} /> : null}
				{loading ? <LoadingState /> : null}
				{!loading && !errors[active] && definition ? <div className="table-wrap"><table className="dense-table"><thead><tr>{definition.columns.map((column) => <th key={column}>{column.replaceAll('_', ' ')}</th>)}<th>Actions</th></tr></thead>
					<tbody>{items.map((item) => <tr key={item.id}>{definition.columns.map((column) => <td key={column}>{column === 'status' || column === 'state' ? <StatusBadge value={valueOf(item, column)} /> : column.includes('bytes') || column === 'size' ? formatBytes(Number(item[column])) : valueOf(item, column)}</td>)}<td><div className="row-actions">
						{active === 'websites' && capabilities['applications.write'] && ['node', 'python'].includes(String(item.runtime)) ? <button type="button" className="link-button" onClick={() => create('applications', { website_id: item.id, runtime: item.runtime })}>Deploy application</button> : null}
						{active === 'backups' && item.state === 'succeeded' && capabilities['backups.restore'] ? <button type="button" className="link-button" onClick={() => setRestoreBackup(item)}>Review restore</button> : null}
						{definition.isDeletable && definition.writeCapability && capabilities[definition.writeCapability] ? <button type="button" className="link-button danger-text" onClick={() => remove(definition.endpoint, item.id)}>Delete</button> : null}
					</div></td></tr>)}</tbody>
				</table></div> : null}
				{!loading && !errors[active] && !items.length ? <EmptyState title={`No ${definition?.label.toLocaleLowerCase()} yet`} detail="Use the form above where available. Provisioning results will remain visible here." /> : null}
			</section>
			<Dialog open={Boolean(restoreBackup)} title="Review in-place restore" onClose={() => setRestoreBackup(null)}>
				<p>This queues an in-place restore from backup <strong>{restoreBackup?.id}</strong>. Current account files and service configuration may be replaced.</p>
				<dl className="detail-list"><div><dt>Destination</dt><dd>{restoreBackup ? valueOf(restoreBackup, 'destination') : '—'}</dd></div><div><dt>Checksum</dt><dd>{restoreBackup ? valueOf(restoreBackup, 'checksum') : '—'}</dd></div></dl>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setRestoreBackup(null)}>Cancel</button><button type="button" onClick={restore}>Queue restore</button></footer>
			</Dialog>
		</>
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
	if (service === 'domains') return <form className="inline-form" onSubmit={(event) => submit(event, 'domains', (data) => ({ fqdn: data.get('fqdn'), type: data.get('type'), dns_managed: true }))}><label>Domain<input name="fqdn" placeholder="shop.example.com" required /></label><label>Type<select name="type"><option value="addon">Addon</option><option value="subdomain">Subdomain</option><option value="alias">Alias</option></select></label><button type="submit">Add domain</button></form>
	if (service === 'websites') return <form className="inline-form" onSubmit={(event) => submit(event, 'websites', (data) => ({ domain_id: data.get('domain_id'), runtime: data.get('runtime'), document_root: `${account.home_path}/public_html` }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Runtime<select name="runtime"><option>php</option><option>static</option><option>node</option><option>python</option></select></label><button type="submit">Apply website</button></form>
	if (service === 'databases') return <form className="inline-form" onSubmit={(event) => submit(event, 'databases', (data) => ({ name: data.get('name'), engine: data.get('engine') }))}><label>Name<input name="name" required /></label><label>Engine<select name="engine"><option>mariadb</option><option>postgres</option><option>mysql</option></select></label><button type="submit">Create database</button></form>
	if (service === 'mailboxes') return <><form className="inline-form" onSubmit={(event) => submit(event, 'mail/mailboxes', (data) => ({ domain_id: data.get('domain_id'), local_part: data.get('local_part'), password: data.get('password') }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Local part<input name="local_part" required /></label><label>Password<input name="password" type="password" required /></label><button type="submit">Create mailbox</button></form><p className="subtle">Mailbox storage quota is enforced by the account package and cannot be overridden here.</p></>
	if (service === 'aliases') return <form className="inline-form" onSubmit={(event) => submit(event, 'mail/aliases', (data) => ({ domain_id: data.get('domain_id'), address: data.get('address'), destination: data.get('destination') }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Address<input name="address" required /></label><label>Destination<input name="destination" required /></label><button type="submit">Create alias</button></form>
	if (service === 'certificates') return <form className="inline-form" onSubmit={(event) => submit(event, 'certificates', (data) => ({ hostname: data.get('hostname') }))}><label>Hostname<input name="hostname" defaultValue={account.primary_domain} required /></label><button type="submit">Request certificate</button></form>
	if (service === 'files') return <form className="stack-form" onSubmit={(event) => submit(event, 'files', (data) => ({ path: data.get('path'), content: data.get('content') }))}><label>Path<input name="path" defaultValue="/public_html/index.html" required /></label><label>Contents<textarea name="content" rows={4} required /></label><button type="submit">Write file</button></form>
	if (service === 'backups') return <form className="inline-form" onSubmit={(event) => submit(event, 'backups', (data) => ({ kind: 'full', destination: data.get('destination') }))}><label>Destination<select name="destination"><option>local</option><option>sftp</option><option>s3</option></select></label><button type="submit">Queue encrypted backup</button></form>
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
