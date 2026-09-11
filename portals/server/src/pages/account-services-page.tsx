import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, Navigate, useParams, useSearchParams } from 'react-router-dom'
import { canonicalAccountToolPath, isHubAccountService } from '../account-tool-routes'
import { api, asList } from '../client'
import { AccountTabs, Dialog, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom, valueOf } from '../helpers'
import { groupAccountServices, resourcePrimaryLabel, serviceActionLabel } from './account-service-groups'
import { backupDestinationReadiness, backupScopeSummary } from './backup-copy'
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
	{ id: 'lists', label: 'Mailing lists', endpoint: 'mail/lists', columns: ['local_part', 'members', 'status'], isDeletable: true, readCapability: 'mail.read', writeCapability: 'mail.write' },
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
	const requests = useRef(new RequestSequence()).current
	const currentAccountId = useRef(id)
	currentAccountId.current = id
	const capabilities = useCapabilities()
	const visibleServices = useMemo(() => services.filter((service) => capabilities[service.readCapability]), [capabilities])
	const requestedService = searchParams.get('service') || ''
	const leftoverServices = useMemo(() => visibleServices.filter((service) => !isHubAccountService(service.id)), [visibleServices])
	const definition = leftoverServices.find((service) => service.id === requestedService) || leftoverServices[0]
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
	async function restore () {
		if (!restoreBackup) return
		await create('restores', { backup_id: restoreBackup.id, mode: 'in_place' })
		setRestoreBackup(null)
	}

	if (requestedService && isHubAccountService(requestedService)) {
		const dest = canonicalAccountToolPath(requestedService, id)
		if (dest) return <Navigate to={dest} replace />
	}

	const isCurrentRequest = requestAccountId === id
	const currentAccount = isCurrentRequest && loadedAccountId === id ? account : null
	if (!currentAccount && (loading || !isCurrentRequest)) return <><PageHeader title="Account Services" description="Loading account-linked services." /><LoadingState /></>
	if (!currentAccount) return <><PageHeader title="Account Services" description="Account-linked operations." /><ErrorState error={errors.account || 'Account unavailable.'} onRetry={load} /></>
	const items = definition ? resources[definition.id] || [] : []
	const groups = groupAccountServices(visibleServices)
	const leftoverGroups = groupAccountServices(leftoverServices)
	return (
		<>
			<PageHeader title={`${currentAccount.username}: Services`} description="Open the dedicated manager for each hosting resource. Access tools that have no separate hub stay on this page." />
			<AccountTabs id={id} />
			<section className="panel">
				<div className="section-heading"><div><h2>Hosting tools</h2><p>Each card opens the real manager for that resource. There is no second hop through this page.</p></div></div>
				<div className="tool-launch-grid">
					{groups.map((group) => group.items.filter((service) => isHubAccountService(service.id)).map((service) => {
						const path = canonicalAccountToolPath(service.id, id)
						if (!path) return null
						return (
							<Link key={service.id} className="tool-launch-card" to={path}>
								<strong>{service.label}</strong>
								<span>{resources[service.id]?.length ?? 0} on this account</span>
							</Link>
						)
					}))}
				</div>
			</section>
			{leftoverGroups.length ? <div className="service-switcher">
				<label className="service-select-label" htmlFor="account-service">Access and backups
					<select id="account-service" value={active} onChange={(event) => setSearchParams({ service: event.target.value }, { replace: true })}>
						{leftoverGroups.map((group) => <optgroup key={group.id} label={group.label}>{group.items.map((service) => <option key={service.id} value={service.id}>{service.label} ({resources[service.id]?.length ?? 0})</option>)}</optgroup>)}
					</select>
				</label>
				<div className="service-groups" role="tablist" aria-label="Access and backup tools">
					{leftoverGroups.map((group) => (
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
			</div> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel service-panel">
				<div className="section-heading"><div><h2>{definition?.label} · {items.length}</h2><p>These access tools are managed here. Certificates, mail, SQL, files, and websites open their own managers.</p></div></div>
				{definition?.writeCapability && capabilities[definition.writeCapability] ? <ServiceCreateForm service={definition.id} resources={resources} canListWebsites={Boolean(capabilities['websites.read'])} onCreate={create} /> : <p className="subtle">Available resources are read-only for your current role.</p>}
				{definition?.id === 'backups' ? <p className="subtle">{backupScopeSummary(pkg?.backup_retention_days)}</p> : null}
				{errors[active] ? <ErrorState title={`${definition?.label} unavailable`} error={errors[active]} onRetry={load} /> : null}
				{loading ? <LoadingState /> : null}
				{!loading && !errors[active] && definition ? <div className="table-wrap"><table className="dense-table"><thead><tr><th>Resource</th>{definition.columns.map((column) => <th key={column} scope="col">{column.replaceAll('_', ' ')}</th>)}<th scope="col">Actions</th></tr></thead>
					<tbody>{items.map((item) => <tr key={item.id}><td><button type="button" className="link-button resource-name" onClick={() => setDetail(item)}>{resourcePrimaryLabel(definition.id, item)}</button></td>{definition.columns.map((column) => <td key={column}>{column === 'status' || column === 'state' ? <StatusBadge value={valueOf(item, column)} /> : column.includes('bytes') || column === 'size' ? formatBytes(Number(item[column])) : valueOf(item, column)}</td>)}<td><div className="row-actions">
						<button type="button" className="link-button" onClick={() => setDetail(item)}>Details</button>
						{active === 'applications' && capabilities['applications.write'] && ['node', 'python'].includes(String(item.runtime)) ? <button type="button" className="link-button" onClick={() => create('applications', { website_id: item.id, runtime: item.runtime })}>Deploy application</button> : null}
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

interface ServiceCreateFormProps {
	service: string
	resources: Record<string, ResourceItem[]>
	canListWebsites: boolean
	onCreate: (endpoint: string, body: Record<string, unknown>) => Promise<void>
}

export function ServiceCreateForm ({ service, resources, canListWebsites, onCreate }: ServiceCreateFormProps) {
	const capabilities = useCapabilities()
	function submit (event: React.FormEvent<HTMLFormElement>, endpoint: string, payload: (data: FormData) => Record<string, unknown>) {
		event.preventDefault()
		onCreate(endpoint, payload(new FormData(event.currentTarget)))
	}
	if (service === 'backups') return <BackupCreateForm onCreate={onCreate} />
	if (service === 'ssh') return <><form className="stack-form" onSubmit={(event) => submit(event, 'ssh-keys', (data) => ({ label: data.get('label'), public_key: data.get('public_key') }))}><label>Key label<input name="label" required /></label><label>Public key<textarea name="public_key" rows={3} required /></label><button type="submit">Add SSH key</button></form><form className="inline-form" onSubmit={(event) => submit(event, 'sftp-password', (data) => ({ password: data.get('password') }))}><label>SFTP password<input name="password" type="password" required /></label><button type="submit">Set SFTP password</button></form></>
	if (service === 'tokens') return <form className="stack-form" onSubmit={(event) => submit(event, 'api-tokens', (data) => ({ name: data.get('name'), scope: 'account', capabilities: data.getAll('capability') }))}><label>Name<input name="name" required /></label><p className="subtle"><strong>Scope:</strong> Account. Tokens cannot be granted host-wide or arbitrary capabilities.</p><fieldset><legend>Account-safe capabilities</legend><div className="checkbox-grid">{accountTokenCapabilities.filter((capability) => capabilities[capability]).map((capability) => <label className="checkbox-label" key={capability}><input type="checkbox" name="capability" value={capability} />{capability}</label>)}</div></fieldset><button type="submit">Create token</button></form>
	if (service === 'applications') {
		if (!canListWebsites) return <p className="subtle">Application creation is disabled because your role cannot list account websites.</p>
		if (!(resources.websites || []).some((item) => ['node', 'python'].includes(String(item.runtime)))) return <p className="subtle">Create a Node or Python website in MultiPHP Manager before creating its application deployment.</p>
		return <p className="subtle">Choose <strong>Deploy application</strong> beside an account-owned Node or Python website. Runtime settings and working directory are derived from that website.</p>
	}
	return null
}
