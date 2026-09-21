import { Link } from 'react-router-dom'
import { download } from '../client'
import {
	MAIL_NOTIFY_BANNER,
	NGINX_LOG_INSPECT_COPY,
} from '../catalog-honesty'
import { CopyableValue, EmptyState } from '../components/ui'
import { formatBytes, formatDate, messageFrom } from '../helpers'
import { accountHomePath } from './account-lifecycle-copy'
import type { Account, Package } from '../types'
import type { WhmFeature } from '../whm-catalog'

export interface NginxLogItem {
	kind: string
	path: string
	website_id?: string
	domain?: string
	present?: boolean
	size_bytes?: number
}

export interface MailRecipient {
	kind: string
	id: string
	username: string
	email: string
	primary_domain?: string
	label?: string
}

export interface BackupItem {
	id: string
	kind?: string
	state?: string
	destination?: string
	created_at?: string
	finished_at?: string
	restorable?: boolean
	account_id?: string
	size_bytes?: number
}

export interface ToolReviewModel {
	feature: WhmFeature
	account?: Account
	accounts: Account[]
	packages: Package[]
	values: Record<string, string>
	nginxLogs: NginxLogItem[]
	recipients: MailRecipient[]
	backups: BackupItem[]
	migrationAccounts: Account[]
	onDownloadError: (message: string) => void
}

export function isReviewStepLabel (label: string | undefined) {
	return Boolean(label && /^review\b/i.test(label))
}

export function fieldsForConfigureStep (feature: WhmFeature, step: number) {
	const fields = (feature.fields ?? []).filter((field) => field.type !== 'account')
	const reviewIndex = (feature.steps ?? []).findIndex((label) => isReviewStepLabel(label))
	const inputCount = reviewIndex >= 0 ? reviewIndex : Math.max((feature.steps?.length || 1) - 1, 1)
	if (feature.layout === 'wizard' && fields.length === inputCount && inputCount > 1) {
		const field = fields[step]
		return field ? [field] : []
	}
	return fields
}

export function restorableBackups (backups: BackupItem[]) {
	return backups.filter((backup) => backup.restorable !== false && !/failed|queued|running/i.test(backup.state || ''))
}

export function demoUsernames (raw: string) {
	return raw.split(/[\s,]+/).map((name) => name.trim()).filter(Boolean)
}

export function ToolReviewExtras ({
	feature, account, accounts, packages, values, nginxLogs, recipients, backups, migrationAccounts, onDownloadError,
}: ToolReviewModel) {
	if (feature.id === 'raw-nginx-log') {
		return <NginxLogReview account={account} items={nginxLogs} onDownloadError={onDownloadError} />
	}
	if (feature.id === 'rearrange-account') {
		return <RearrangeReview account={account} />
	}
	if (feature.id === 'ip-migration') {
		return <IPMigrationReview fromIP={values.from_ip} toIP={values.to_ip} accounts={migrationAccounts} />
	}
	if (feature.id === 'email-all-users' || feature.id === 'email-resellers') {
		return <MailNotifyReview values={values} recipients={recipients} />
	}
	if (feature.id === 'manage-demo-mode') {
		return <DemoModeReview values={values} accounts={accounts} />
	}
	if (feature.id === 'file-dir-restore') {
		return <FileRestoreReview account={account} values={values} backups={backups} />
	}
	if (feature.id === 'quota-modification' || feature.id === 'limit-bandwidth') {
		return <PackageLimitReview account={account} packages={packages} packageId={values.package_id} />
	}
	return null
}

export function GenericReviewRows ({
	feature, account, packages, values,
}: {
	feature: WhmFeature
	account?: Account
	packages: Package[]
	values: Record<string, string>
}) {
	const skip = new Set(['account_id', 'package_id', 'backup_id'])
	if (feature.id === 'raw-nginx-log' || feature.id === 'rearrange-account' || feature.id === 'manage-demo-mode') {
		skip.add('usernames')
	}
	if (feature.id === 'ip-migration') {
		skip.add('from_ip')
		skip.add('to_ip')
	}
	if (feature.id === 'email-all-users' || feature.id === 'email-resellers') {
		// Draft is repeated under the recipient list heading.
	}
	const rows = Object.entries(values).filter(([key, value]) => !skip.has(key) && value)
	const selectedPackage = packages.find((pkg) => pkg.id === values.package_id)
	return (
		<dl className="detail-list">
			{account ? <div><dt>Account</dt><dd>{account.username} · {account.primary_domain}</dd></div> : null}
			{values.package_id && feature.id !== 'quota-modification' && feature.id !== 'limit-bandwidth' ? (
				<div><dt>Package</dt><dd>{selectedPackage?.name || values.package_id}</dd></div>
			) : null}
			{rows.map(([key, value]) => (
				<div key={key}>
					<dt>{fieldLabel(feature, key)}</dt>
					<dd>{/password|secret|private[_-]?key|passwd/i.test(key) ? '••••••••' : value}</dd>
				</div>
			))}
		</dl>
	)
}

function fieldLabel (feature: WhmFeature, key: string) {
	return feature.fields?.find((field) => field.name === key)?.label || key.replaceAll('_', ' ')
}

function NginxLogReview ({
	account, items, onDownloadError,
}: {
	account?: Account
	items: NginxLogItem[]
	onDownloadError: (message: string) => void
}) {
	if (!account) {
		return <EmptyState title="Select an account" detail="Choose an account to list its nginx log paths." />
	}
	if (!items.length) {
		return (
			<EmptyState
				title="No nginx logs for this account"
				detail="Director has no home or vhost log paths yet. Provision the account and return here."
			/>
		)
	}
	const accountId = account.id
	async function handleDownload (item: NginxLogItem) {
		try {
			await download(
				`/api/v1/accounts/${accountId}/nginx-logs/content?path=${encodeURIComponent(item.path)}`,
				filenameForLog(item),
			)
		} catch (reason) {
			onDownloadError(messageFrom(reason))
		}
	}
	return (
		<>
			<p className="subtle">{NGINX_LOG_INSPECT_COPY}</p>
			<dl className="detail-list">
				{items.map((item) => (
					<div key={item.path}>
						<dt>{item.kind === 'error' ? 'Error' : 'Access'}{item.domain ? ` · ${item.domain}` : ''}</dt>
						<dd>
							<CopyableValue value={item.path} label={`${item.kind} log path`} />
							<small>{item.present ? `${formatBytes(item.size_bytes)} on host` : 'Not present on host yet'}</small>
							{item.present ? (
								<button type="button" className="link-button" onClick={() => { void handleDownload(item) }}>
									Download {filenameForLog(item)}
								</button>
							) : null}
						</dd>
					</div>
				))}
			</dl>
		</>
	)
}

function filenameForLog (item: NginxLogItem) {
	const parts = item.path.split('/')
	return parts[parts.length - 1] || `${item.kind}.log`
}

function RearrangeReview ({ account }: { account?: Account }) {
	if (!account) {
		return <EmptyState title="Select an account" detail="Choose an account to review its home directory." />
	}
	const home = account.home_path || accountHomePath(account.username)
	return (
		<>
			<dl className="detail-list">
				<div><dt>Home</dt><dd><CopyableValue value={home} label="home path" /></dd></div>
				<div><dt>UID/GID</dt><dd>{account.linux_uid}/{account.linux_gid}</dd></div>
			</dl>
		</>
	)
}

function IPMigrationReview ({
	fromIP, toIP, accounts,
}: {
	fromIP: string
	toIP: string
	accounts: Account[]
}) {
	return (
		<>
			<dl className="detail-list">
				<div><dt>Source IP</dt><dd>{fromIP || '—'}</dd></div>
				<div><dt>Destination IP</dt><dd>{toIP || '—'}</dd></div>
			</dl>
			{accounts.length ? (
				<ul className="list-plain">
					{accounts.map((account) => (
						<li key={account.id}>{account.username} · {account.primary_domain} · {account.ip_address || 'unset'}</li>
					))}
				</ul>
			) : (
				<EmptyState
					title="No accounts on this source IP"
					detail="Shared or unset addresses are not migrated. Enter a dedicated source IP that tenants currently use."
				/>
			)}
		</>
	)
}

function MailNotifyReview ({
	values, recipients,
}: {
	values: Record<string, string>
	recipients: MailRecipient[]
}) {
	return (
		<>
			<p className="subtle">{MAIL_NOTIFY_BANNER}</p>
			<dl className="detail-list">
				<div><dt>From</dt><dd>{values.from || '—'}</dd></div>
				<div><dt>Subject</dt><dd>{values.subject || '—'}</dd></div>
				<div><dt>Message</dt><dd>{values.body || '—'}</dd></div>
			</dl>
			<h3>Recipients</h3>
			{recipients.length ? (
				<ul className="list-plain">
					{recipients.map((row) => (
						<li key={`${row.kind}-${row.id}`}>{row.username} · {row.email}{row.primary_domain ? ` · ${row.primary_domain}` : ''}</li>
					))}
				</ul>
			) : (
				<EmptyState
					title="No recipients"
					detail="No account owners (or reseller contacts) currently have an email address."
				/>
			)}
		</>
	)
}

function DemoModeReview ({
	values, accounts,
}: {
	values: Record<string, string>
	accounts: Account[]
}) {
	const names = demoUsernames(values.usernames || '')
	if (!names.length) {
		return (
			<EmptyState
				title="No demo accounts"
				detail="The Director demo set is empty. Enter usernames to mark demonstration tenants, or leave empty to clear the set."
			/>
		)
	}
	return (
		<ul className="list-plain">
			{names.map((name) => {
				const account = accounts.find((entry) => entry.username === name)
				return (
					<li key={name}>
						{name}
						{account ? ` · ${account.primary_domain} · ${account.home_path || accountHomePath(name)}` : ' · not an account on this host'}
					</li>
				)
			})}
		</ul>
	)
}

function FileRestoreReview ({
	account, values, backups,
}: {
	account?: Account
	values: Record<string, string>
	backups: BackupItem[]
}) {
	const ready = restorableBackups(backups)
	const selected = ready.find((backup) => backup.id === values.backup_id) || ready[0]
	return (
		<>
			<dl className="detail-list">
				<div><dt>Account</dt><dd>{account ? `${account.username} · ${account.primary_domain}` : 'Not selected'}</dd></div>
				<div><dt>Path</dt><dd><code>{values.path || 'public_html'}</code></dd></div>
				{selected ? (
					<>
						<div><dt>Backup</dt><dd>{selected.id}</dd></div>
						<div><dt>Taken</dt><dd>{formatDate(selected.finished_at || selected.created_at)}</dd></div>
						<div><dt>Kind</dt><dd>{selected.kind || 'full'} · {selected.destination || 'local'}</dd></div>
					</>
				) : null}
			</dl>
			<h3>Restorable backups</h3>
			{ready.length ? (
				<ul className="list-plain">
					{ready.map((backup) => (
						<li key={backup.id}>
							{backup.id}
							{backup.id === selected?.id ? ' · selected' : ''}
							{' · '}
							{formatDate(backup.finished_at || backup.created_at)}
							{backup.kind ? ` · ${backup.kind}` : ''}
						</li>
					))}
				</ul>
			) : (
				<EmptyState
					title="No restorable backups"
					detail="This account has no succeeded HPM1 archive in inventory. Configure a destination and queue a backup first."
					action={<Link to="/transfers">Configure or run backups</Link>}
				/>
			)}
		</>
	)
}

function PackageLimitReview ({
	account, packages, packageId,
}: {
	account?: Account
	packages: Package[]
	packageId: string
}) {
	const selected = packages.find((pkg) => pkg.id === packageId)
	return (
		<dl className="detail-list">
			<div><dt>Account</dt><dd>{account ? `${account.username} · ${account.primary_domain}` : 'Not selected'}</dd></div>
			<div><dt>Package</dt><dd>{selected?.name || (packageId ? 'Unknown package' : 'Not selected')}</dd></div>
			{selected ? (
				<>
					<div><dt>Disk quota</dt><dd>{formatBytes(selected.disk_bytes)}</dd></div>
					<div><dt>Monthly bandwidth</dt><dd>{formatBytes(selected.bandwidth_bytes_monthly)}</dd></div>
				</>
			) : null}
		</dl>
	)
}
