import { FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { ErrorState, PageHeader, SectionHeading } from '../components/ui'
import { messageFrom } from '../helpers'
import { hasCapabilities, useCapabilities } from '../rbac'
import type { Account, Package } from '../types'
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

function WhmToolBody ({ feature }: { feature: WhmFeature }) {
	const capabilities = useCapabilities()
	const [step, setStep] = useState(0)
	const [accountId, setAccountId] = useState('')
	const [accountFilter, setAccountFilter] = useState('')
	const [values, setValues] = useState<Record<string, string>>(() => defaultFieldValues(feature))
	const [accounts, setAccounts] = useState<Account[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [websites, setWebsites] = useState<Website[]>([])
	const [settings, setSettings] = useState<Record<string, string>>({})
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [busy, setBusy] = useState(false)
	const steps = feature.steps.length ? feature.steps : ['Configure', 'Review']
	const isLast = step >= steps.length - 1
	const selectedAccount = accounts.find((account) => account.id === accountId)
	const canSubmit = canSubmitFeature(feature, capabilities)

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

	function handleField (name: string, value: string) {
		setValues((current) => ({ ...current, [name]: value }))
		if (name === 'account_id') setAccountId(value)
	}

	async function handleSubmit (event: FormEvent) {
		event.preventDefault()
		if (!isLast) {
			setStep((current) => current + 1)
			return
		}
		setBusy(true)
		setError('')
		setMessage('')
		try {
			setMessage(await applyFeature({ feature, accountId, selectedAccount, values, accounts }))
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
			{error ? <ErrorState error={error} /> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}

			{feature.layout === 'status' ? <StatusPanel feature={feature} account={selectedAccount} /> : null}

			{feature.layout === 'restart' ? (
				<section className="form-panel">
					<SectionHeading title="Restart service" detail="Queues a typed restart. The agent never accepts an arbitrary shell command." />
					<p>Service: <code>{serviceName(feature)}</code></p>
					<button type="button" className="danger" disabled={!canSubmit || busy} onClick={() => {
						setBusy(true)
						setError('')
						setMessage('')
						void applyFeature({ feature, accountId, selectedAccount, values, accounts })
							.then(setMessage)
							.catch((reason) => setError(messageFrom(reason)))
							.finally(() => setBusy(false))
					}}>
						{busy ? 'Queueing…' : `Restart ${serviceName(feature)}`}
					</button>
				</section>
			) : null}

			{feature.layout !== 'status' && feature.layout !== 'restart' ? (
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
					{!isLast ? (
						<div className="form-grid">
							{(feature.fields ?? []).filter((field) => field.type !== 'account').map((field) => (
								<ToolField
									key={field.name}
									field={field}
									value={values[field.name] ?? settings[field.name] ?? field.defaultValue ?? ''}
									packages={packages}
									websites={websites}
									onChange={handleField}
								/>
							))}
						</div>
					) : (
						<dl className="detail-list">
							{needsAccountPicker(feature) ? <div><dt>Account</dt><dd>{selectedAccount ? `${selectedAccount.username} · ${selectedAccount.primary_domain}` : 'Not selected'}</dd></div> : null}
							{Object.entries(values).filter(([key, value]) => key !== 'account_id' && value).map(([key, value]) => (
								<div key={key}><dt>{key.replaceAll('_', ' ')}</dt><dd>{SECRET_FIELD.test(key) ? '••••••••' : value}</dd></div>
							))}
						</dl>
					)}
					<div className="page-actions">
						{step > 0 ? <button type="button" className="secondary" onClick={() => setStep((current) => current - 1)}>Back</button> : null}
						<button type="submit" disabled={busy || (isLast && !canSubmit)}>
							{busy ? 'Working…' : isLast ? confirmLabel(feature) : 'Continue'}
						</button>
					</div>
					{isLast && !canSubmit ? <p className="subtle">Your role can open this journey, but the API will refuse the write.</p> : null}
				</form>
			) : null}

			{feature.related?.length ? (
				<aside className="panel">
					<SectionHeading title="Related tools" />
					<ul className="list-plain">
						{feature.related.map((id) => {
							const related = featureById(id)
							if (!related) return null
							return <li key={id}><Link to={related.path}>{related.label}</Link></li>
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
	if (feature.id === 'terminal') {
		return (
			<section className="panel">
				<p>WHM exposes a root terminal. Kelmor does not. Privileged work goes through typed agent jobs: service restarts, account lifecycle, DNS, TLS, backups, and package changes.</p>
				<p>Use Restart Services, Service Status, and Process Manager for live host operations. There is no in-browser root shell.</p>
			</section>
		)
	}
	if (feature.id === 'phpmyadmin') {
		return (
			<section className="panel">
				<p>phpMyAdmin is not bundled. Open Database Manager and use the account&apos;s prefixed credentials.</p>
				<Link to="/sql">Open Database Manager</Link>
			</section>
		)
	}
	if (feature.id === 'change-root-password') {
		return (
			<section className="panel">
				<p>The Director login password is not the Ubuntu root password. Rotate the host root account from a signed-in SSH session on the box. Director never stores a root password.</p>
			</section>
		)
	}
	if (feature.id === 'skeleton-directory') {
		return (
			<section className="panel">
				<p>New account homes receive the host skeleton from <code>/etc/skel</code> during provision. Kelmor does not copy a custom WHM-style /root/cpanel3-skel tree.</p>
			</section>
		)
	}
	if (feature.id === 'initial-quota') {
		return (
			<section className="panel">
				<p>Disk caps are enforced from the account package even when the kernel has no usrquota mount. Process Manager and Account Usage show the live observations.</p>
				<Link to="/usage">Open Account Usage</Link>
			</section>
		)
	}
	if (feature.id === 'api-shell') {
		return (
			<section className="panel">
				<p>The control-plane contract is the OpenAPI document served by this API.</p>
				<p><a href="/openapi">Open /openapi</a> · <a href="/openapi.yaml">openapi.yaml</a></p>
			</section>
		)
	}
	if (feature.id === 'support-center' || feature.id === 'diagnostics-log') {
		return (
			<section className="panel">
				<p>For a support case collect: hostname from the top bar, recent Jobs, matching Audit events, and <code>PANEL_STATE_DIR/logs/api.jsonl</code> on the host.</p>
				<p><Link to="/jobs">Jobs</Link> · <Link to="/audit">Audit Trail</Link></p>
			</section>
		)
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

async function applyFeature ({
	feature, accountId, selectedAccount, values, accounts,
}: {
	feature: WhmFeature
	accountId: string
	selectedAccount?: Account
	values: Record<string, string>
	accounts: Account[]
}): Promise<string> {
	if (feature.layout === 'restart') {
		const name = serviceName(feature)
		await api(`/api/v1/server/services/${encodeURIComponent(name)}/restart`, { method: 'POST', body: '{}' })
		return `Restart queued for ${name}.`
	}

	if (feature.id === 'unsuspend-bandwidth') {
		const ids = accounts.filter((account) => account.status === 'suspended').map((account) => account.id)
		if (!ids.length) return 'No suspended accounts to unsuspend.'
		await api('/api/v1/accounts/bulk/unsuspend', { method: 'POST', body: JSON.stringify({ ids }) })
		return `Queued unsuspend for ${ids.length} account(s).`
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
		return 'WordPress install queued.'
	}

	if (feature.id === 'repair-mailbox-perms') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}`, { method: 'PATCH', body: JSON.stringify({}) })
		return 'Mail maps reconcile queued with account reconciliation.'
	}

	if (feature.accountAction === 'suspend') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}/suspend`, { method: 'POST', body: JSON.stringify({ reason: values.reason || '' }) })
		return 'Account suspend queued.'
	}
	if (feature.accountAction === 'unsuspend') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}/unsuspend`, { method: 'POST', body: '{}' })
		return 'Account unsuspend queued.'
	}
	if (feature.accountAction === 'terminate') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}/terminate`, { method: 'POST', body: '{}' })
		return 'Account terminate queued.'
	}
	if (feature.accountAction === 'password') {
		if (!accountId) throw new Error('Choose an account first.')
		await api(`/api/v1/accounts/${accountId}/password`, { method: 'POST', body: JSON.stringify({ password: values.password || values.new_password }) })
		return 'Password rotation queued.'
	}
	if (feature.accountAction === 'impersonate') {
		throw new Error('Open List Accounts or Account Summary to start a reasoned Control session.')
	}

	if (feature.accountAction === 'patch' || feature.id === 'quota-modification' || feature.id === 'limit-bandwidth' || feature.id === 'reset-bandwidth') {
		if (!accountId) throw new Error('Choose an account first.')
		const patch: Record<string, string> = {}
		if (values.ip_address !== undefined && feature.id !== 'quota-modification') patch.ip_address = values.ip_address
		if (values.package_id) patch.package_id = values.package_id
		if (values.shell_class) patch.shell_class = values.shell_class
		if (Object.keys(patch).length) {
			await api(`/api/v1/accounts/${accountId}`, { method: 'PATCH', body: JSON.stringify(patch) })
		}
		if (feature.settingKey) {
			await saveSetting(feature.settingKey, {
				...values,
				account_id: accountId,
				username: selectedAccount?.username || '',
			})
		}
		return 'Account change applied.'
	}

	if (feature.id === 'change-root-password' || feature.id === 'mysql-root-password') {
		throw new Error('Director does not store or rotate host root or MariaDB root passwords. Use a signed-in host session.')
	}

	if (feature.settingKey) {
		await saveSetting(feature.settingKey, values)
		return 'Settings saved.'
	}

	if (feature.layout === 'confirm' || feature.layout === 'wizard' || feature.layout === 'form') {
		await saveSetting(feature.id, values)
		return 'Request saved on this Director host.'
	}

	return 'Nothing to apply.'
}

async function saveSetting (key: string, values: Record<string, string>) {
	const safe: Record<string, string> = {}
	for (const [name, value] of Object.entries(values)) {
		if (SECRET_FIELD.test(name)) continue
		safe[name] = value
	}
	const current = await api<ServerSettings>('/api/v1/server/settings').catch(() => ({ values: {} }))
	await api('/api/v1/server/settings', {
		method: 'PATCH',
		body: JSON.stringify({ values: { ...(current.values || {}), [key]: safe } }),
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
	if (feature.accountAction === 'suspend') return 'Suspend account'
	if (feature.layout === 'settings' || feature.settingKey) return 'Save settings'
	return 'Apply'
}

function canSubmitFeature (feature: WhmFeature, capabilities: Record<string, boolean>) {
	const writeCaps = feature.capabilities.filter((capability) => /write|modify|create|restart|suspend|terminate|impersonate|restore/.test(capability))
	if (writeCaps.length) return writeCaps.some((capability) => capabilities[capability])
	if (feature.layout === 'settings' || feature.settingKey) return Boolean(capabilities['server.settings.write'])
	if (feature.matchAll) return hasCapabilities(capabilities, feature.capabilities)
	return feature.capabilities.some((capability) => capabilities[capability])
}
