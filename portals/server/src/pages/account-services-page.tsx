import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountTabs, Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, valueOf } from '../helpers'
import { useCapabilities } from '../rbac'
import type { Account, ResourceItem } from '../types'

interface ServiceDefinition {
	id: string
	label: string
	endpoint: string
	columns: string[]
	deleteSegment?: string
	readCapability: string
	writeCapability?: string
}

const services: ServiceDefinition[] = [
	{ id: 'websites', label: 'Websites', endpoint: 'websites', columns: ['runtime', 'document_root', 'enabled'], deleteSegment: 'websites', readCapability: 'websites.read', writeCapability: 'websites.write' },
	{ id: 'domains', label: 'Domains', endpoint: 'domains', columns: ['ascii_fqdn', 'type', 'status'], deleteSegment: 'domains', readCapability: 'domains.read', writeCapability: 'domains.write' },
	{ id: 'databases', label: 'Databases', endpoint: 'databases', columns: ['name', 'engine', 'status'], deleteSegment: 'databases', readCapability: 'databases.read', writeCapability: 'databases.write' },
	{ id: 'mail-domains', label: 'Mail domains', endpoint: 'mail/domains', columns: ['domain_id', 'catchall_policy', 'status'], readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'mailboxes', label: 'Mailboxes', endpoint: 'mail/mailboxes', columns: ['local_part', 'quota_bytes', 'status'], deleteSegment: 'mail/mailboxes', readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'aliases', label: 'Aliases', endpoint: 'mail/aliases', columns: ['address', 'destination'], deleteSegment: 'mail/aliases', readCapability: 'mail.read', writeCapability: 'mail.write' },
	{ id: 'certificates', label: 'Certificates', endpoint: 'certificates', columns: ['hostname', 'kind', 'status', 'not_after'], readCapability: 'websites.read', writeCapability: 'websites.write' },
	{ id: 'files', label: 'Files', endpoint: 'files?path=/public_html', columns: ['name', 'size', 'dir'], readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'backups', label: 'Backups & restore', endpoint: 'backups', columns: ['kind', 'state', 'destination', 'size_bytes'], readCapability: 'backups.read', writeCapability: 'backups.create' },
	{ id: 'cron', label: 'Cron', endpoint: 'cron', columns: ['schedule', 'command', 'enabled'], deleteSegment: 'cron', readCapability: 'cron.read', writeCapability: 'cron.write' },
	{ id: 'ssh', label: 'SSH / SFTP', endpoint: 'ssh-keys', columns: ['label', 'fingerprint', 'created_at'], deleteSegment: 'ssh-keys', readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'ftp', label: 'FTP', endpoint: 'ftp', columns: ['username', 'home_path', 'status'], deleteSegment: 'ftp', readCapability: 'files.read', writeCapability: 'files.write' },
	{ id: 'tokens', label: 'API tokens', endpoint: 'api-tokens', columns: ['name', 'prefix', 'scope', 'expires_at'], deleteSegment: 'api-tokens', readCapability: 'api_tokens.read', writeCapability: 'api_tokens.write' },
	{ id: 'applications', label: 'Applications', endpoint: 'applications', columns: ['runtime', 'working_directory', 'status'], deleteSegment: 'applications', readCapability: 'applications.read', writeCapability: 'applications.write' },
]

export function AccountServicesPage () {
	const { id = '' } = useParams()
	const [searchParams] = useSearchParams()
	const [account, setAccount] = useState<Account | null>(null)
	const [active, setActive] = useState(searchParams.get('service') || 'websites')
	const [resources, setResources] = useState<Record<string, ResourceItem[]>>({})
	const [errors, setErrors] = useState<Record<string, string>>({})
	const [loading, setLoading] = useState(true)
	const [message, setMessage] = useState('')
	const [restoreBackup, setRestoreBackup] = useState<ResourceItem | null>(null)
	const capabilities = useCapabilities()
	const visibleServices = useMemo(() => services.filter((service) => capabilities[service.readCapability]), [capabilities])

	const load = useCallback(() => {
		setLoading(true)
		Promise.allSettled([
			api<Account>(`/api/v1/accounts/${id}`),
			...visibleServices.map((service) => api<{ items: ResourceItem[] }>(`/api/v1/accounts/${id}/${service.endpoint}`)),
		]).then(([accountResult, ...results]) => {
			if (accountResult.status === 'fulfilled') setAccount(accountResult.value)
			else setErrors((current) => ({ ...current, account: messageFrom(accountResult.reason) }))
			const nextResources: Record<string, ResourceItem[]> = {}
			const nextErrors: Record<string, string> = {}
			results.forEach((result, index) => {
				const service = visibleServices[index]
				if (!service) return
				if (result.status === 'fulfilled') nextResources[service.id] = asList(result.value)
				else nextErrors[service.id] = messageFrom(result.reason)
			})
			setResources(nextResources)
			setErrors((current) => ({ account: current.account, ...nextErrors }))
		}).finally(() => setLoading(false))
	}, [id, visibleServices])
	useEffect(load, [load])

	async function create (endpoint: string, body: Record<string, unknown>) {
		setMessage('')
		try {
			const result = await api<{ operation_id?: string; token?: string }>(`/api/v1/accounts/${id}/${endpoint}`, { method: 'POST', body: JSON.stringify(body) })
			setMessage(result.token ? `Token created: ${result.token}. Copy it now.` : result.operation_id ? `Queued ${result.operation_id}.` : 'Resource created.')
			load()
		} catch (error) { setMessage(messageFrom(error)) }
	}
	async function remove (segment: string, resourceId: string) {
		if (!window.confirm('Delete this resource? Dependent resources may block removal.')) return
		try {
			await api(`/api/v1/accounts/${id}/${segment}/${resourceId}`, { method: 'DELETE' })
			setMessage('Delete operation queued.')
			load()
		} catch (error) { setMessage(messageFrom(error)) }
	}
	async function patchMailDomain (mailDomainId: string, catchallPolicy: string) {
		try {
			await api(`/api/v1/accounts/${id}/mail/domains/${mailDomainId}`, { method: 'PATCH', body: JSON.stringify({ catchall_policy: catchallPolicy }) })
			setMessage('Mail routing update queued.')
			load()
		} catch (error) { setMessage(messageFrom(error)) }
	}
	async function restore () {
		if (!restoreBackup) return
		await create('restores', { backup_id: restoreBackup.id, mode: 'in_place' })
		setRestoreBackup(null)
	}

	if (!account && loading) return <><PageHeader title="Account Services" description="Loading account-linked services." /><LoadingState /></>
	if (!account) return <><PageHeader title="Account Services" description="Account-linked operations." /><ErrorState error={errors.account || 'Account unavailable.'} onRetry={load} /></>
	const definition = visibleServices.find((service) => service.id === active) || visibleServices[0]
	const items = definition ? resources[definition.id] || [] : []
	return (
		<>
			<PageHeader title={`${account.username}: Account Services`} description="Websites, domains, databases, mail, certificates, files, access, backups, and automation." actions={<Link className="button-link secondary-link" to={`/dns?account=${id}`}>Manage DNS</Link>} />
			<AccountTabs id={id} />
			<div className="service-tabs" role="tablist" aria-label="Account service">
				{visibleServices.map((service) => <button key={service.id} type="button" role="tab" aria-selected={definition?.id === service.id} onClick={() => setActive(service.id)}>{service.label}<span>{resources[service.id]?.length ?? '—'}</span></button>)}
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel service-panel">
				<div className="section-heading"><div><h2>{definition?.label}</h2><p>Changes are queued through Kelmor’s capability-enforced API.</p></div></div>
				{definition?.id === 'mail-domains' && capabilities['mail.write'] ? <MailDomainForm domains={items} onPatch={patchMailDomain} /> : definition?.writeCapability && capabilities[definition.writeCapability] ? <ServiceCreateForm service={definition.id} account={account} resources={resources} onCreate={create} /> : <p className="subtle">Available resources are read-only for your current role.</p>}
				{errors[active] ? <ErrorState title={`${definition?.label} unavailable`} error={errors[active]} onRetry={load} /> : null}
				{loading ? <LoadingState /> : null}
				{!loading && !errors[active] && definition ? <div className="table-wrap"><table className="dense-table"><thead><tr>{definition.columns.map((column) => <th key={column}>{column.replaceAll('_', ' ')}</th>)}<th>Actions</th></tr></thead>
					<tbody>{items.map((item) => <tr key={item.id}>{definition.columns.map((column) => <td key={column}>{column === 'status' || column === 'state' ? <StatusBadge value={valueOf(item, column)} /> : column.includes('bytes') || column === 'size' ? formatBytes(Number(item[column])) : valueOf(item, column)}</td>)}<td><div className="row-actions">
						{active === 'backups' && item.state === 'succeeded' ? <button type="button" className="link-button" onClick={() => setRestoreBackup(item)}>Review restore</button> : null}
						{definition.deleteSegment && definition.writeCapability && capabilities[definition.writeCapability] ? <button type="button" className="link-button danger-text" onClick={() => remove(definition.deleteSegment || '', item.id)}>Delete</button> : null}
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
	onCreate: (endpoint: string, body: Record<string, unknown>) => Promise<void>
}

export function ServiceCreateForm ({ service, account, resources, onCreate }: ServiceCreateFormProps) {
	function submit (event: React.FormEvent<HTMLFormElement>, endpoint: string, payload: (data: FormData) => Record<string, unknown>) {
		event.preventDefault()
		onCreate(endpoint, payload(new FormData(event.currentTarget)))
	}
	const domains = resources.domains || []
	if (service === 'domains') return <form className="inline-form" onSubmit={(event) => submit(event, 'domains', (data) => ({ fqdn: data.get('fqdn'), type: data.get('type'), dns_managed: true }))}><label>Domain<input name="fqdn" placeholder="shop.example.com" required /></label><label>Type<select name="type"><option value="addon">Addon</option><option value="subdomain">Subdomain</option><option value="alias">Alias</option></select></label><button type="submit">Add domain</button></form>
	if (service === 'websites') return <form className="inline-form" onSubmit={(event) => submit(event, 'websites', (data) => ({ domain_id: data.get('domain_id'), runtime: data.get('runtime'), document_root: `${account.home_path}/public_html` }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Runtime<select name="runtime"><option>php</option><option>static</option><option>node</option><option>python</option></select></label><button type="submit">Apply website</button></form>
	if (service === 'databases') return <form className="inline-form" onSubmit={(event) => submit(event, 'databases', (data) => ({ name: data.get('name'), engine: data.get('engine') }))}><label>Name<input name="name" required /></label><label>Engine<select name="engine"><option>mariadb</option><option>postgres</option><option>mysql</option></select></label><button type="submit">Create database</button></form>
	if (service === 'mailboxes') return <form className="inline-form" onSubmit={(event) => submit(event, 'mail/mailboxes', (data) => ({ domain_id: data.get('domain_id'), local_part: data.get('local_part'), password: data.get('password'), quota_bytes: Number(data.get('quota_bytes')) }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Local part<input name="local_part" required /></label><label>Password<input name="password" type="password" required /></label><label>Quota bytes<input name="quota_bytes" type="number" defaultValue={1073741824} /></label><button type="submit">Create mailbox</button></form>
	if (service === 'aliases') return <form className="inline-form" onSubmit={(event) => submit(event, 'mail/aliases', (data) => ({ domain_id: data.get('domain_id'), address: data.get('address'), destination: data.get('destination') }))}><label>Domain<select name="domain_id">{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label><label>Address<input name="address" required /></label><label>Destination<input name="destination" required /></label><button type="submit">Create alias</button></form>
	if (service === 'certificates') return <form className="inline-form" onSubmit={(event) => submit(event, 'certificates', (data) => ({ hostname: data.get('hostname') }))}><label>Hostname<input name="hostname" defaultValue={account.primary_domain} required /></label><button type="submit">Request certificate</button></form>
	if (service === 'files') return <form className="stack-form" onSubmit={(event) => submit(event, 'files', (data) => ({ path: data.get('path'), content: data.get('content') }))}><label>Path<input name="path" defaultValue="/public_html/index.html" required /></label><label>Contents<textarea name="content" rows={4} required /></label><button type="submit">Write file</button></form>
	if (service === 'backups') return <form className="inline-form" onSubmit={(event) => submit(event, 'backups', (data) => ({ kind: 'full', destination: data.get('destination') }))}><label>Destination<select name="destination"><option>local</option><option>sftp</option><option>s3</option></select></label><button type="submit">Queue encrypted backup</button></form>
	if (service === 'cron') return <form className="inline-form" onSubmit={(event) => submit(event, 'cron', (data) => ({ schedule: data.get('schedule'), command: data.get('command'), working_directory: account.home_path, enabled: true }))}><label>Schedule<input name="schedule" defaultValue="0 * * * *" required /></label><label>Command<input name="command" required /></label><button type="submit">Add cron job</button></form>
	if (service === 'ssh') return <><form className="stack-form" onSubmit={(event) => submit(event, 'ssh-keys', (data) => ({ label: data.get('label'), public_key: data.get('public_key') }))}><label>Key label<input name="label" required /></label><label>Public key<textarea name="public_key" rows={3} required /></label><button type="submit">Add SSH key</button></form><form className="inline-form" onSubmit={(event) => submit(event, 'sftp-password', (data) => ({ password: data.get('password') }))}><label>SFTP password<input name="password" type="password" required /></label><button type="submit">Set SFTP password</button></form></>
	if (service === 'ftp') return <form className="inline-form" onSubmit={(event) => submit(event, 'ftp', (data) => ({ username: data.get('username'), password: data.get('password'), home_path: data.get('home_path') }))}><label>Username<input name="username" required /></label><label>Password<input name="password" type="password" required /></label><label>Home path<input name="home_path" defaultValue={`${account.home_path}/public_html`} required /></label><button type="submit">Create FTP user</button></form>
	if (service === 'tokens') return <form className="inline-form" onSubmit={(event) => submit(event, 'api-tokens', (data) => ({ name: data.get('name'), scope: data.get('scope'), capabilities: String(data.get('capabilities')).split(',').map((value) => value.trim()).filter(Boolean) }))}><label>Name<input name="name" required /></label><label>Scope<select name="scope"><option value="account">Account</option><option value="read">Read only</option></select></label><label>Capabilities<input name="capabilities" placeholder="domains.read, files.read" /></label><button type="submit">Create token</button></form>
	if (service === 'applications') return <form className="inline-form" onSubmit={(event) => submit(event, 'applications', (data) => ({ website_id: data.get('website_id'), runtime: data.get('runtime'), runtime_version: data.get('runtime_version'), working_directory: data.get('working_directory'), start_command: data.get('start_command') }))}><label>Website<select name="website_id">{(resources.websites || []).map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'document_root')}</option>)}</select></label><label>Runtime<select name="runtime"><option>node</option><option>python</option></select></label><label>Version<input name="runtime_version" placeholder="20" /></label><label>Working directory<input name="working_directory" defaultValue={`${account.home_path}/app`} /></label><label>Start command<input name="start_command" placeholder="npm start" required /></label><button type="submit">Deploy application</button></form>
	return null
}
