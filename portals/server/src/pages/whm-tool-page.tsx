import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { EmptyState, ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { addonDomainsFrom, convertAddonDomainsHref, domainFqdn } from '../convert-addon'
import { messageFrom } from '../helpers'
import { hasCapabilities, useCapabilities } from '../rbac'
import type { Account, Package, ResourceItem } from '../types'
import { ClusterPanel } from './cluster-panel'
import { RemoteAccessPanel } from './remote-access-panel'
import { DiagnosticsPanel } from './diagnostics-panel'
import { HostAppsPanel, PHPRuntimePanel } from './host-apps-panel'
import { HostModulesPanel, moduleKindForFeature } from './host-modules-panel'
import { MySQLUpgradePanel } from './mysql-upgrade-panel'
import { PostgresConfigPanel } from './postgres-config-panel'
import { ThemeManagerPanel } from './theme-manager-panel'
import { SupportAccessPanel } from './support-access-panel'
import { HostConsolePanel, HostPasswordForm } from './host-console-panel'
import { FirewallPanel } from './firewall-panel'
import { MailQueuePanel } from './mail-queue-panel'
import { HotlinkPanel, ImageManagerPanel } from './site-policy-panels'
import { FeatureShowcasePage } from './feature-showcase-page'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import {
	CHROME_SETTINGS_SAVED,
	DEFERRED_SETTINGS_SAVED,
	HOST_SETTINGS_BANNER,
	LOCAL_SETTINGS_BANNER,
	QUOTA_PACKAGE_COPY,
	confirmLabelForFeature,
	isChromeSettingsKey,
	isDeferredSettingsKey,
	isHostSettingsFeature,
	isLocalSettingsFeature,
} from '../catalog-honesty'
import { hrefForFeature } from '../nav-hubs'
import { featureById, type WhmFeature, type WhmField } from '../whm-catalog'

interface ServerSettings {
	values?: Record<string, Record<string, string>>
}

interface Website {
	id: string
	document_root?: string
}

const SECRET_FIELD = /password|secret|private[_-]?key|passwd/i

export function WhmToolPage () {
	const { toolId = '' } = useParams()
	const feature = featureById(toolId)
	if (!feature) {
		return (
			<>
				<PageHeader title="Tool not found" description="This path is not in the Director catalog." />
				<p><Link to="/">Return home</Link></p>
			</>
		)
	}
	return <WhmToolBody feature={feature} />
}

export function WhmToolBody ({ feature }: { feature: WhmFeature }) {
	const capabilities = useCapabilities()
	const [step, setStep] = useState(0)
	const [accountId, setAccountId] = useState('')
	const [accountFilter, setAccountFilter] = useState('')
	const [values, setValues] = useState<Record<string, string>>(() => defaultFieldValues(feature))
	const [accounts, setAccounts] = useState<Account[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [websites, setWebsites] = useState<Website[]>([])
	const [addonDomains, setAddonDomains] = useState<ResourceItem[]>([])
	const [addonsLoading, setAddonsLoading] = useState(false)
	const [settings, setSettings] = useState<Record<string, string>>({})
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const steps = feature.steps.length ? feature.steps : ['Configure', 'Review']
	const isLast = step >= steps.length - 1
	const selectedAccount = accounts.find((account) => account.id === accountId)
	const canSubmit = canSubmitFeature(feature, capabilities)
	const isConvertAddon = feature.id === 'convert-addon'
	const convertEmpty = isConvertAddon && Boolean(accountId) && !addonsLoading && addonDomains.length === 0
	const convertReady = !isConvertAddon || (Boolean(accountId) && !addonsLoading && addonDomains.length > 0)

	useEffect(() => {
		const needsAccounts = feature.layout === 'account-action' || feature.fields?.some((field) => field.type === 'account')
		const needsPackages = feature.fields?.some((field) => field.type === 'package')
		const requests: Array<Promise<void>> = []
		if (needsAccounts) {
			requests.push(api<{ items: Account[] }>('/api/v1/accounts').then((result) => setAccounts(asList(result))).catch((reason) => setError(messageFrom(reason))))
		}
		if (needsPackages) {
			requests.push(api<{ items: Package[] }>('/api/v1/packages').then((result) => setPackages(asList(result))).catch((reason) => setError(messageFrom(reason))))
		}
		if (feature.layout === 'settings' || feature.settingKey || feature.layout === 'form' || feature.layout === 'wizard') {
			requests.push(api<ServerSettings>('/api/v1/server/settings').then((result) => {
				const stored = (feature.settingKey && result.values?.[feature.settingKey]) || {}
				setSettings(stored)
				setValues((current) => ({ ...defaultFieldValues(feature), ...stored, ...current }))
			}).catch(() => undefined))
		}
		if (!requests.length) return
		void Promise.allSettled(requests)
	}, [feature])

	useEffect(() => {
		if (!accountId || feature.id !== 'wp-toolkit') {
			setWebsites([])
			return
		}
		api<{ items: Website[] }>(`/api/v1/accounts/${accountId}/websites`)
			.then((result) => setWebsites(asList(result)))
			.catch(() => setWebsites([]))
	}, [accountId, feature.id])

	useEffect(() => {
		if (feature.id !== 'convert-addon') {
			setAddonDomains([])
			setAddonsLoading(false)
			return
		}
		setStep(0)
		setValues((current) => ({ ...current, addon_domain: '' }))
		if (!accountId) {
			setAddonDomains([])
			setAddonsLoading(false)
			return
		}
		setAddonsLoading(true)
		api<{ items: ResourceItem[] }>(`/api/v1/accounts/${accountId}/domains`)
			.then((result) => setAddonDomains(addonDomainsFrom(asList(result))))
			.catch(() => setAddonDomains([]))
			.finally(() => setAddonsLoading(false))
	}, [accountId, feature.id])

	function handleField (name: string, value: string) {
		setValues((current) => ({ ...current, [name]: value }))
		if (name === 'account_id') setAccountId(value)
	}

	async function handleSubmit (event: FormEvent) {
		event.preventDefault()
		if (!isLast) {
			if (isConvertAddon && !convertReady) return
			setStep((current) => current + 1)
			return
		}
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await applyFeature({ feature, accountId, values })
			setMessage(result.message)
			setJobId(result.jobId || '')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			<PageHeader title={feature.label} description={feature.description} />
			<p className="subtle">{feature.category} · {feature.layout} journey</p>
			{isLocalSettingsFeature(feature) ? <p className="settings-local-banner" role="status">{LOCAL_SETTINGS_BANNER}</p> : null}
			{isHostSettingsFeature(feature) ? <p className="settings-local-banner host-applied" role="status">{HOST_SETTINGS_BANNER}</p> : null}
			<QueuedOpNotice message={message} accountId={accountId || undefined} jobId={jobId} />
			{feature.id === 'quota-modification' || feature.id === 'limit-bandwidth' ? <p className="settings-local-banner host-applied" role="status">{QUOTA_PACKAGE_COPY}</p> : null}
			{error ? <ErrorState error={error} /> : null}

			{feature.layout === 'status' ? <StatusPanel feature={feature} account={selectedAccount} /> : null}

			{feature.layout === 'restart' ? (
				<section className="form-panel">
					<SectionHeading title="Restart service" detail="Queues a typed restart. The agent never accepts an arbitrary shell command." />
					<p>Service: <code>{serviceName(feature)}</code></p>
					<button type="button" className="danger" disabled={!canSubmit || busy} onClick={() => {
						setBusy(true)
						setError('')
						setMessage('')
						void applyFeature({ feature, accountId, values })
							.then((result) => { setMessage(result.message); setJobId(result.jobId || '') })
							.catch((reason) => setError(messageFrom(reason)))
							.finally(() => setBusy(false))
					}}>
						{busy ? 'Queueing…' : `Restart ${serviceName(feature)}`}
					</button>
				</section>
			) : null}

			{feature.id === 'theme-manager' ? <ThemeManagerPanel /> : null}

			{feature.id !== 'theme-manager' && feature.layout !== 'status' && feature.layout !== 'restart' ? (
				<form className="form-panel" onSubmit={handleSubmit}>
					<ol className="steps" aria-label="Workflow">
						{steps.map((label, index) => (
							<li key={label} className={index === step ? 'active' : index < step ? 'complete' : ''}>
								<span>{index + 1}</span>{label}
							</li>
						))}
					</ol>
					{!isLast ? (
						<div className="form-section-heading">
							<h2>{steps[step]}</h2>
							<p>{feature.description}</p>
						</div>
					) : (
						<div className="form-section-heading">
							<h2>Review</h2>
							<p>Confirm the change before Director writes it. Privileged work still goes through the API and typed agent jobs.</p>
						</div>
					)}
					{needsAccountPicker(feature) ? (
						<AccountPicker
							accounts={accounts}
							value={accountId}
							onChange={(id) => { setAccountId(id); handleField('account_id', id) }}
							filter={accountFilter}
							onFilterChange={setAccountFilter}
						/>
					) : null}
					{isConvertAddon && !accountId ? <p className="subtle">Select a source account to look for addon domains.</p> : null}
					{isConvertAddon && accountId && addonsLoading ? <LoadingState label="Loading addon domains…" /> : null}
					{convertEmpty ? (
						<EmptyState
							title="No addon domains to convert"
							detail="This account has no addon domains. Create an addon domain first, then return here to promote it to its own POSIX account."
							action={<Link to={convertAddonDomainsHref(accountId)}>Create or manage domains</Link>}
						/>
					) : null}
					{!isLast && convertReady ? (
						<div className="form-grid">
							{(feature.fields ?? []).filter((field) => field.type !== 'account').map((field) => {
								if (isConvertAddon && field.name === 'addon_domain') {
									return (
										<label key={field.name}>
											{field.label}
											<select
												value={values.addon_domain || ''}
												onChange={(event) => handleField('addon_domain', event.target.value)}
												required
											>
												<option value="">Select an addon domain…</option>
												{addonDomains.map((domain) => {
													const fqdn = domainFqdn(domain)
													return <option key={domain.id} value={fqdn}>{fqdn}</option>
												})}
											</select>
										</label>
									)
								}
								return (
									<ToolField
										key={field.name}
										field={field}
										value={values[field.name] ?? settings[field.name] ?? field.defaultValue ?? ''}
										packages={packages}
										websites={websites}
										onChange={handleField}
									/>
								)
							})}
						</div>
					) : null}
					{isLast && convertReady ? (
						<dl className="detail-list">
							{needsAccountPicker(feature) ? <div><dt>Account</dt><dd>{selectedAccount ? `${selectedAccount.username} · ${selectedAccount.primary_domain}` : 'Not selected'}</dd></div> : null}
							{Object.entries(values).filter(([key, value]) => key !== 'account_id' && value).map(([key, value]) => (
								<div key={key}><dt>{key.replaceAll('_', ' ')}</dt><dd>{SECRET_FIELD.test(key) ? '••••••••' : value}</dd></div>
							))}
						</dl>
					) : null}
					<div className="page-actions">
						{step > 0 && convertReady ? <button type="button" className="secondary" onClick={() => setStep((current) => current - 1)}>Back</button> : null}
						{convertReady ? (
							<button type="submit" disabled={busy || (isLast && !canSubmit)}>
								{busy ? 'Working…' : isLast ? confirmLabel(feature) : 'Continue'}
							</button>
						) : null}
					</div>
					{isLast && convertReady && !canSubmit ? <p className="subtle">Your role can open this journey, but the API will refuse the write.</p> : null}
				</form>
			) : null}

			{feature.related?.length ? (
				<aside className="panel">
					<SectionHeading title="Related tools" />
					<ul className="list-plain">
						{feature.related.map((id) => {
							const related = featureById(id)
							if (!related) return null
							return <li key={id}><Link to={hrefForFeature(related)}>{related.label}</Link></li>
						})}
					</ul>
				</aside>
			) : null}
		</>
	)
}

function ToolField ({ field, value, packages, websites, onChange }: {
	field: WhmField
	value: string
	packages: Package[]
	websites: Website[]
	onChange: (name: string, value: string) => void
}) {
	if (field.type === 'checkbox') {
		return (
			<label className="checkbox-label">
				<input type="checkbox" checked={value === 'on' || value === 'true'} onChange={(event) => onChange(field.name, event.target.checked ? 'on' : 'off')} />
				{field.label}
				{field.help ? <small>{field.help}</small> : null}
			</label>
		)
	}
	if (field.type === 'textarea') {
		return (
			<label>
				{field.label}
				{field.help ? <small>{field.help}</small> : null}
				<textarea value={value} onChange={(event) => onChange(field.name, event.target.value)} required={field.required} />
			</label>
		)
	}
	if (field.type === 'select') {
		return (
			<label>
				{field.label}
				<select value={value} onChange={(event) => onChange(field.name, event.target.value)}>
					{(field.options ?? []).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
				</select>
			</label>
		)
	}
	if (field.type === 'package') {
		return (
			<label>
				{field.label}
				<select value={value} onChange={(event) => onChange(field.name, event.target.value)} required={field.required}>
					<option value="">Select a package…</option>
					{packages.map((pkg) => <option key={pkg.id} value={pkg.id}>{pkg.name}</option>)}
				</select>
			</label>
		)
	}
	if (field.name === 'website_id' && websites.length) {
		return (
			<label>
				{field.label}
				<select value={value} onChange={(event) => onChange(field.name, event.target.value)} required={field.required}>
					<option value="">Select a website…</option>
					{websites.map((site) => <option key={site.id} value={site.id}>{site.document_root || site.id}</option>)}
				</select>
			</label>
		)
	}
	return (
		<label>
			{field.label}
			{field.help ? <small>{field.help}</small> : null}
			<input
				type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
				value={value}
				onChange={(event) => onChange(field.name, event.target.value)}
				required={field.required}
			/>
		</label>
	)
}

function StatusPanel ({ feature, account }: { feature: WhmFeature; account?: Account }) {
	if (feature.id === 'feature-showcase') return <FeatureShowcasePage />
	if (feature.id === 'terminal') return <HostConsolePanel />
	if (feature.id === 'phpmyadmin') return <HostAppsPanel kind="sql" />
	if (feature.id === 'market') return <HostAppsPanel kind="market" />
	if (feature.id === 'plugins') return <HostAppsPanel kind="plugin" />
	if (feature.id === 'mailman' || feature.id === 'reset-mailman') {
		return (
			<section className="panel">
				<p>Mailing lists are first-class Kelmor aliases with multiple members. Create and edit them in Email Management.</p>
				<Link to={feature.id === 'reset-mailman' ? '/email?tab=lists&task=reset' : '/email?tab=lists'}>Open mailing lists</Link>
			</section>
		)
	}
	if (feature.id === 'mail-queue') return <MailQueuePanel />
	if (feature.id === 'firewall' || feature.id === 'csf') return <FirewallPanel />
	if (feature.id === 'hotlink') return <HotlinkPanel />
	if (feature.id === 'image-manager') return <ImageManagerPanel />
	if (feature.id === 'mail-troubleshooter') {
		return (
			<>
				<section className="panel">
					<p>Use the audited Postfix recipes below to inspect and flush the live queue. Delivery history is Mail Delivery Reports; live watching is Track Delivery.</p>
					<p><Link to="/mail/delivery-reports">Mail Delivery Reports</Link> · <Link to="/mail/track-delivery">Track Delivery</Link> · <Link to="/deliverability">Deliverability</Link></p>
				</section>
				<HostConsolePanel recipeIds={['postfix-queue', 'postfix-flush', 'postfix-status']} />
			</>
		)
	}
	if (feature.id === 'change-root-password') {
		return <HostPasswordForm title="Change root password" endpoint="/api/v1/server/root-password" />
	}
	if (feature.id === 'mysql-root-password') {
		return <HostPasswordForm title="Change database root password" endpoint="/api/v1/server/database-root-password" includeCurrent />
	}
	if (feature.id === 'postgres-config') return <PostgresConfigPanel />
	if (feature.id === 'mysql-upgrade') return <MySQLUpgradePanel />
	if (feature.id === 'easyapache') return <PHPRuntimePanel />
	if (feature.id === 'api-shell') {
		return (
			<section className="panel">
				<p>The control-plane contract is the OpenAPI document served by this API.</p>
				<p><a href="/openapi">Open /openapi</a> · <a href="/openapi.yaml">openapi.yaml</a></p>
			</section>
		)
	}
	if (feature.id === 'support-center') {
		return (
			<section className="panel">
				<p>For a support case collect: hostname from the top bar, recent Jobs, matching Audit events, and a diagnostics archive.</p>
				<p><Link to="/jobs">Jobs</Link> · <Link to="/audit">Audit Trail</Link> · <Link to="/section/system?tool=diagnostics-log">Download diagnostics</Link></p>
			</section>
		)
	}
	if (feature.id === 'diagnostics-log') return <DiagnosticsPanel />
	if (feature.id === 'grant-support-access') return <SupportAccessPanel />
	if (feature.id === 'configuration-cluster') return <ClusterPanel />
	if (feature.id === 'remote-access-key') return <RemoteAccessPanel />
	if (feature.id === 'module-installers' || feature.id === 'perl-modules' || feature.id === 'php-pear' || feature.id === 'php-pecl' || feature.id === 'ruby-gems') {
		return <HostModulesPanel kind={moduleKindForFeature(feature.id)} />
	}
	if (feature.id === 'rearrange-account' && account) {
		return (
			<section className="panel">
				<dl className="detail-list">
					<div><dt>Home</dt><dd><code>{account.home_path}</code></dd></div>
					<div><dt>UID/GID</dt><dd>{account.linux_uid}/{account.linux_gid}</dd></div>
				</dl>
			</section>
		)
	}
	if (feature.id === 'raw-nginx-log' && account) {
		return (
			<section className="panel">
				<p>nginx writes account vhost logs next to the home tree. Copy these paths into File Manager or a signed-in host session.</p>
				<dl className="detail-list">
					<div><dt>Access</dt><dd><code>{account.home_path}/logs/access.log</code></dd></div>
					<div><dt>Error</dt><dd><code>{account.home_path}/logs/error.log</code></dd></div>
				</dl>
			</section>
		)
	}
	return (
		<section className="panel">
			<p>{feature.description}</p>
			<p className="subtle">This surface is part of the Director catalog. Destructive work still goes through the API and typed agent jobs.</p>
		</section>
	)
}

interface ApplyResult {
	message: string
	jobId?: string
}

function applied (result: { operation_id?: string } | undefined, fallback: string): ApplyResult {
	return { message: queuedOpMessage(result, fallback), jobId: result?.operation_id }
}

async function applyFeature ({
	feature, accountId, values,
}: {
	feature: WhmFeature
	accountId: string
	values: Record<string, string>
}): Promise<ApplyResult> {
	if (feature.layout === 'restart') {
		const name = serviceName(feature)
		const result = await api<{ operation_id?: string }>(`/api/v1/server/services/${encodeURIComponent(name)}/restart`, { method: 'POST', body: '{}' })
		return applied(result, `Restart queued for ${name}.`)
	}

	if (feature.id === 'unsuspend-bandwidth') {
		const result = await api<{ operations?: string[] }>('/api/v1/accounts/bulk/clear-bandwidth-hold', { method: 'POST', body: '{}' })
		const count = result.operations?.length || 0
		return { message: count ? `Queued bandwidth-hold clear for ${count} account(s).` : 'No bandwidth holds to clear.', jobId: result.operations?.[0] }
	}

	if (feature.id === 'file-dir-restore') {
		if (!accountId) throw new Error('Choose an account first.')
		if (!values.path) throw new Error('Enter a path under home.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/restores`, {
			method: 'POST',
			body: JSON.stringify({ path: values.path, backup_id: values.backup_id || undefined }),
		})
		return applied(result, 'Path restore queued.')
	}

	if (feature.id === 'email-all-users' || feature.id === 'email-resellers') {
		const result = await api<{ operation_id?: string }>('/api/v1/mail/notify', {
			method: 'POST',
			body: JSON.stringify({
				from: values.from,
				subject: values.subject,
				body: values.body,
				audience: feature.id === 'email-resellers' ? 'resellers' : 'owners',
			}),
		})
		return applied(result, 'Notification mail queued.')
	}

	if (feature.id === 'synchronize-dns') {
		const result = await api<{ operation_id?: string }>('/api/v1/dns/synchronize', { method: 'POST', body: '{}' })
		return applied(result, 'DNS synchronize queued.')
	}

	if (feature.id === 'dns-cleanup') {
		const result = await api<{ operation_id?: string }>('/api/v1/dns/cleanup', { method: 'POST', body: '{}' })
		return applied(result, 'DNS cleanup queued.')
	}

	if (feature.id === 'ip-migration') {
		const result = await api<{ operations?: string[] }>('/api/v1/accounts/ip-migration', {
			method: 'POST',
			body: JSON.stringify({ from_ip: values.from_ip, to_ip: values.to_ip }),
		})
		return { message: `Queued IP migration for ${result.operations?.length || 0} account(s).`, jobId: result.operations?.[0] }
	}

	if (feature.id === 'multi-modify' || feature.id === 'multi-ip' || feature.id === 'change-ownership-bulk') {
		const result = await api<{ operations?: string[] }>('/api/v1/accounts/bulk/modify', {
			method: 'POST',
			body: JSON.stringify({
				usernames: values.usernames,
				package_id: values.package_id,
				reseller_id: values.reseller_id,
				ip_address: values.ip_address,
			}),
		})
		return { message: `Queued account updates for ${result.operations?.length || 0} account(s).`, jobId: result.operations?.[0] }
	}

	if (feature.id === 'convert-addon') {
		if (!accountId) throw new Error('Choose a source account first.')
		const result = await api<{ resource_id?: string; operation_id?: string }>('/api/v1/accounts/convert-addon', {
			method: 'POST',
			body: JSON.stringify({
				account_id: accountId,
				addon_domain: values.addon_domain,
				username: values.username,
				package_id: values.package_id,
				owner_password: values.owner_password,
			}),
		})
		return applied(result, `Addon conversion queued as ${result.resource_id || 'a new account'}.`)
	}

	if (feature.id === 'domain-forwarding') {
		if (!accountId) throw new Error('Choose an account first.')
		const created = await api<{ operation_id?: string; resource_id?: string }>(`/api/v1/accounts/${accountId}/domains`, {
			method: 'POST',
			body: JSON.stringify({ fqdn: values.source, type: 'alias' }),
		})
		return applied(created, 'Alias domain queued. Use DNS Zone Manager if the target URL needs an extra record.')
	}

	if (feature.id === 'wp-toolkit') {
		if (!accountId) throw new Error('Choose an account first.')
		if (!values.website_id) throw new Error('Choose a website first.')
		await api(`/api/v1/accounts/${accountId}/wordpress`, {
			method: 'POST',
			body: JSON.stringify({
				website_id: values.website_id,
				title: values.title || 'WordPress',
				admin_user: values.admin_user,
				admin_password: values.admin_password,
				admin_email: values.admin_email,
			}),
		})
		return { message: 'WordPress install queued.' }
	}

	if (feature.id === 'repair-mailbox-perms') {
		if (!accountId) throw new Error('Choose an account first.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}`, { method: 'PATCH', body: JSON.stringify({}) })
		return applied(result, 'Mail maps reconcile queued with account reconciliation.')
	}

	if (feature.accountAction === 'suspend') {
		if (!accountId) throw new Error('Choose an account first.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/suspend`, { method: 'POST', body: JSON.stringify({ reason: values.reason || '' }) })
		return applied(result, 'Account suspend queued.')
	}
	if (feature.accountAction === 'unsuspend') {
		if (!accountId) throw new Error('Choose an account first.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/unsuspend`, { method: 'POST', body: '{}' })
		return applied(result, 'Account unsuspend queued.')
	}
	if (feature.accountAction === 'terminate') {
		if (!accountId) throw new Error('Choose an account first.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/terminate`, { method: 'POST', body: '{}' })
		return applied(result, 'Account terminate queued.')
	}
	if (feature.accountAction === 'remove') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}/remove`, { method: 'POST', body: '{}' })
		return { message: 'Terminated account removed. The username and domain can be reused.' }
	}
	if (feature.accountAction === 'password') {
		if (!accountId) throw new Error('Choose an account first.')
		const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/password`, { method: 'POST', body: JSON.stringify({ password: values.password || values.new_password }) })
		return applied(result, 'Password rotation queued.')
	}
	if (feature.accountAction === 'impersonate') {
		throw new Error('Open List Accounts or Account Summary to start a reasoned Control session.')
	}

	if (feature.accountAction === 'patch') {
		if (!accountId) throw new Error('Choose an account first.')
		const patch: Record<string, string> = {}
		if (values.ip_address !== undefined) patch.ip_address = values.ip_address
		if (values.package_id) patch.package_id = values.package_id
		if (values.shell_class) patch.shell_class = values.shell_class
		if (Object.keys(patch).length) {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}`, { method: 'PATCH', body: JSON.stringify(patch) })
			return applied(result, 'Host change queued.')
		}
		throw new Error('Choose a package or host field to apply.')
	}

	if (feature.id === 'change-root-password') {
		await api('/api/v1/server/root-password', { method: 'POST', body: JSON.stringify({ password: values.password || values.new_password }) })
		return { message: 'Root password applied on the host. It was not stored in Director.' }
	}
	if (feature.id === 'mysql-root-password') {
		await api('/api/v1/server/database-root-password', { method: 'POST', body: JSON.stringify({ current: values.current || values.current_password, password: values.password || values.new_password }) })
		return { message: 'Database root password applied on the host. It was not stored in Director.' }
	}
	if (feature.id === 'easyapache') {
		const versions = String(values.php_versions || '').split(/\s+/).filter(Boolean)
		for (const version of versions) {
			await api('/api/v1/server/runtimes', { method: 'POST', body: JSON.stringify({ version }) })
		}
		return { message: versions.length ? `PHP runtime install queued for ${versions.join(', ')}.` : 'No PHP versions selected.' }
	}

	if (feature.settingKey) {
		const result = await saveSetting(feature.settingKey, values)
		if (isDeferredSettingsKey(feature.settingKey)) return { message: DEFERRED_SETTINGS_SAVED, jobId: result.operation_id }
		if (isChromeSettingsKey(feature.settingKey)) return { message: CHROME_SETTINGS_SAVED, jobId: result.operation_id }
		return applied(result, 'Host apply queued.')
	}

	if (feature.layout === 'confirm' || feature.layout === 'wizard' || feature.layout === 'form') {
		const result = await saveSetting(feature.id.replaceAll('-', '_'), values)
		return applied(result, 'Host apply queued.')
	}

	return { message: 'Nothing to apply.' }
}

async function saveSetting (key: string, values: Record<string, string>) {
	const safe: Record<string, string> = {}
	for (const [name, value] of Object.entries(values)) {
		if (SECRET_FIELD.test(name)) continue
		safe[name] = value
	}
	return api<{ operation_id?: string }>('/api/v1/server/settings', {
		method: 'PATCH',
		body: JSON.stringify({ values: { [key]: safe } }),
	})
}

function defaultFieldValues (feature: WhmFeature) {
	const values: Record<string, string> = {}
	for (const field of feature.fields ?? []) {
		if (field.defaultValue) values[field.name] = field.defaultValue
	}
	return values
}

function needsAccountPicker (feature: WhmFeature) {
	return feature.layout === 'account-action' || Boolean(feature.fields?.some((field) => field.type === 'account'))
}

function serviceName (feature: WhmFeature) {
	return feature.fields?.find((field) => field.name === 'service')?.defaultValue || feature.id.replace('restart-', '')
}

function confirmLabel (feature: WhmFeature) {
	if (feature.layout === 'confirm' || feature.layout === 'restart') return 'Confirm'
	if (feature.accountAction === 'terminate') return 'Terminate account'
	if (feature.accountAction === 'remove') return 'Remove account'
	if (feature.accountAction === 'suspend') return 'Suspend account'
	return confirmLabelForFeature(feature) || 'Apply'
}

function canSubmitFeature (feature: WhmFeature, capabilities: Record<string, boolean>) {
	const writeCaps = feature.capabilities.filter((capability) => /write|modify|create|restart|suspend|terminate|impersonate|restore/.test(capability))
	if (writeCaps.length) return writeCaps.some((capability) => capabilities[capability])
	if (feature.layout === 'settings' || feature.settingKey) return Boolean(capabilities['server.settings.write'])
	if (feature.matchAll) return hasCapabilities(capabilities, feature.capabilities)
	return feature.capabilities.some((capability) => capabilities[capability])
}
