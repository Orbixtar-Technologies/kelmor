import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, idempotent, post } from '../client'
import { useLoad } from '../lib/hooks'
import { useToast } from '../components/toast'
import { CheckField, Field, Notice, PageHeader, Panel } from '../components/ui'
import { ReviewList, Wizard, type WizardStep } from '../components/wizard'
import { formatBytes, formatCount } from '../lib/format'

interface PackageRow {
	id: string
	name: string
	disk_bytes: number
	bandwidth_bytes_monthly: number
	domains: number
	databases: number
	mailboxes: number
	cpu_percent: number
	memory_bytes: number
	process_limit: number
	email_daily_limit: number
}

interface ResellerRow {
	id: string
	name: string
	status: string
}

interface Draft {
	username: string
	domain: string
	email: string
	password: string
	confirm: string
	packageId: string
	resellerId: string
	ipAddress: string
	mustChangePassword: boolean
	notes: string
}

const empty: Draft = {
	username: '',
	domain: '',
	email: '',
	password: '',
	confirm: '',
	packageId: '',
	resellerId: '',
	ipAddress: '',
	mustChangePassword: true,
	notes: '',
}

export function AccountCreate () {
	const navigate = useNavigate()
	const toast = useToast()
	const [draft, setDraft] = useState<Draft>(empty)
	const [index, setIndex] = useState(0)
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState('')

	const packages = useLoad<{ items: PackageRow[] | null }>(() => api('/api/v1/packages'), [])
	const resellers = useLoad<{ items: ResellerRow[] | null }>(
		() => api<{ items: ResellerRow[] | null }>('/api/v1/resellers').catch(() => ({ items: [] })),
		[],
	)

	const packageList = packages.data?.items ?? []
	const resellerList = resellers.data?.items ?? []
	const chosen = packageList.find((p) => p.id === draft.packageId)

	function set<K extends keyof Draft> (key: K, value: Draft[K]) {
		setDraft((prev) => ({ ...prev, [key]: value }))
	}

	const steps: WizardStep[] = [
		{
			id: 'identity',
			title: 'Account identity',
			validate: () => {
				if (!/^[a-z][a-z0-9-]{1,30}$/.test(draft.username)) return 'Username must start with a letter and use lowercase letters, digits or hyphens.'
				if (!/^[a-z0-9.-]+\.[a-z]{2,}$/i.test(draft.domain)) return 'Enter a valid primary domain.'
				if (draft.password.length < 12) return 'Use a password of at least 12 characters.'
				if (draft.password !== draft.confirm) return 'The two passwords do not match.'
				return ''
			},
			body: (
				<>
					<Notice tone="info">
						The username becomes the Linux identity, the home directory under <code>/home</code> and the owner login for
						Kelmor Control. It cannot be changed after provisioning.
					</Notice>
					<div className="form-grid">
						<Field label="Username" hint="Lowercase letters, digits and hyphens. 2–31 characters.">
							<input value={draft.username} onChange={(e) => set('username', e.target.value.toLowerCase().trim())} autoFocus spellCheck={false} />
						</Field>
						<Field label="Primary domain" hint="Kelmor creates the vhost, the DNS zone and mail routing for this domain.">
							<input value={draft.domain} onChange={(e) => set('domain', e.target.value.toLowerCase().trim())} spellCheck={false} placeholder="example.com" />
						</Field>
						<Field label="Owner email" hint="Used for account notices. Defaults to the owner at the primary domain.">
							<input type="email" value={draft.email} onChange={(e) => set('email', e.target.value.trim())} placeholder="owner@example.com" />
						</Field>
						<Field label="Owner password" hint="At least 12 characters. Also sets the SFTP password.">
							<input type="password" value={draft.password} onChange={(e) => set('password', e.target.value)} autoComplete="new-password" />
						</Field>
						<Field label="Confirm password" error={draft.confirm && draft.confirm !== draft.password ? 'Passwords do not match.' : undefined}>
							<input type="password" value={draft.confirm} onChange={(e) => set('confirm', e.target.value)} autoComplete="new-password" />
						</Field>
					</div>
					<div style={{ marginTop: 14 }}>
						<CheckField
							label="Require a password change at first sign-in"
							hint="Recorded on the owner user. Applied the next time the owner signs in to Kelmor Control."
							checked={draft.mustChangePassword}
							onChange={(v) => set('mustChangePassword', v)}
						/>
					</div>
				</>
			),
		},
		{
			id: 'package',
			title: 'Package and owner',
			validate: () => (draft.packageId ? '' : 'Choose the package that sets this account resource limits.'),
			body: (
				<>
					{packageList.length === 0 ? (
						<Notice tone="warn">
							No package exists yet. <Link to="/packages/new">Add a package</Link> before creating accounts — limits are
							enforced from the package, not from this form.
						</Notice>
					) : null}
					<div className="form-grid">
						<Field label="Package" hint="Disk, transfer, mail and process limits are enforced from the package.">
							<select value={draft.packageId} onChange={(e) => set('packageId', e.target.value)}>
								<option value="">Select a package…</option>
								{packageList.map((pkg) => (
									<option key={pkg.id} value={pkg.id}>{pkg.name}</option>
								))}
							</select>
						</Field>
						<Field label="Owned by" hint="Assign the account to a reseller, or keep it direct under this server.">
							<select value={draft.resellerId} onChange={(e) => set('resellerId', e.target.value)}>
								<option value="">Direct — no reseller</option>
								{resellerList.map((reseller) => (
									<option key={reseller.id} value={reseller.id}>{reseller.name}</option>
								))}
							</select>
						</Field>
					</div>
					{chosen ? (
						<fieldset style={{ marginTop: 16 }}>
							<legend>{chosen.name} limits</legend>
							<ReviewList
								rows={[
									['Disk', formatBytes(chosen.disk_bytes)],
									['Monthly transfer', formatBytes(chosen.bandwidth_bytes_monthly)],
									['Domains', formatCount(chosen.domains)],
									['Databases', formatCount(chosen.databases)],
									['Mailboxes', formatCount(chosen.mailboxes)],
									['CPU / memory', `${chosen.cpu_percent}% · ${formatBytes(chosen.memory_bytes)}`],
									['Processes', formatCount(chosen.process_limit)],
									['Outbound mail per day', formatCount(chosen.email_daily_limit)],
								]}
							/>
						</fieldset>
					) : null}
				</>
			),
		},
		{
			id: 'resources',
			title: 'Networking',
			body: (
				<>
					<Notice tone="info">
						Leave the IP address blank to serve this account from the shared host address. A dedicated address must
						already be bound on the host; Kelmor writes it into the vhost, it does not configure the interface.
					</Notice>
					<div className="form-grid">
						<Field label="Dedicated IP address" hint="Optional. Blank means the shared server address.">
							<input value={draft.ipAddress} onChange={(e) => set('ipAddress', e.target.value.trim())} placeholder="203.0.113.10" spellCheck={false} />
						</Field>
						<Field label="Operator note" hint="Stored with the provisioning job for the audit trail.">
							<input value={draft.notes} onChange={(e) => set('notes', e.target.value)} placeholder="Migrated from legacy host" />
						</Field>
					</div>
				</>
			),
		},
		{
			id: 'review',
			title: 'Review and provision',
			body: (
				<>
					<Notice tone="warn">
						Provisioning creates a Linux user, home directory, document root, DNS zone and mail domain through the
						privileged agent. Review the values below, then queue the job.
					</Notice>
					<ReviewList
						rows={[
							['Username', <strong key="u">{draft.username}</strong>],
							['Primary domain', <span key="d" className="mono">{draft.domain}</span>],
							['Owner email', draft.email || `owner@${draft.domain || 'example.com'}`],
							['Password', '•'.repeat(Math.min(draft.password.length, 16))],
							['Force password change', draft.mustChangePassword ? 'Yes' : 'No'],
							['Package', chosen?.name ?? '—'],
							['Disk limit', chosen ? formatBytes(chosen.disk_bytes) : '—'],
							['Monthly transfer limit', chosen ? formatBytes(chosen.bandwidth_bytes_monthly) : '—'],
							['Owned by', resellerList.find((r) => r.id === draft.resellerId)?.name ?? 'Direct — no reseller'],
							['IP address', draft.ipAddress || 'Shared server address'],
							['Operator note', draft.notes || '—'],
						]}
					/>
				</>
			),
		},
	]

	async function provision () {
		setBusy(true)
		setError('')
		try {
			const created = await post<{ operation_id: string; resource_id: string }>(
				'/api/v1/accounts',
				{
					username: draft.username,
					primary_domain: draft.domain,
					package_id: draft.packageId,
					reseller_id: draft.resellerId || undefined,
					owner_email: draft.email || undefined,
					owner_password: draft.password,
				},
				idempotent(),
			)
			if (draft.ipAddress) {
				await api(`/api/v1/accounts/${created.resource_id}`, {
					method: 'PATCH',
					body: JSON.stringify({ ip_address: draft.ipAddress }),
				}).catch(() => undefined)
			}
			toast.notify(`Provisioning queued for ${draft.username}. Job ${created.operation_id.slice(0, 8)} is running.`, 'ok')
			navigate(`/accounts/${created.resource_id}`)
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Provisioning could not be queued')
			setIndex(0)
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			<PageHeader
				title="Create a New Account"
				description="Four steps: account identity, package and owner, networking, then a review before the provisioning job is queued."
				favoritePath="/accounts/create"
				actions={<Link className="btn secondary" to="/accounts">Back to List Accounts</Link>}
			/>
			<Wizard
				steps={steps}
				index={index}
				onIndexChange={setIndex}
				onSubmit={provision}
				submitLabel="Provision account"
				busy={busy}
				error={error}
			/>
			<Panel title="What happens when you provision" icon="info">
				<ol className="small" style={{ margin: 0, paddingLeft: 20, color: 'var(--text-soft)', lineHeight: 1.9 }}>
					<li>Kelmor records the desired state and queues an <code>account.provision</code> job.</li>
					<li>A worker calls the privileged agent to create the Linux user, home directory and document root.</li>
					<li>The primary domain gets a vhost, an authoritative DNS zone and a mail domain with a DKIM key.</li>
					<li>Package limits are written to the account systemd slice, nginx and PHP-FPM.</li>
					<li>Progress is visible on the <Link to="/jobs">Job Queue</Link> and on the account summary.</li>
				</ol>
			</Panel>
		</>
	)
}
