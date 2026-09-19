import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { AccountToolWizard } from '../components/account-tool-wizard'
import { LoginToControl } from '../components/login-to-control'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { api, asList } from '../client'
import { dedicatedPath, type DedicatedToolId } from '../dedicated-tool-routes'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import { lifecycleImpact } from './account-lifecycle-copy'
import type { Account, Package, Reseller } from '../types'

interface AccountFunctionSpec {
	id: DedicatedToolId
	title: string
	description: string
	emptyDetail: string
	accountFilter: 'active' | 'terminated' | 'all'
}

const specs = {
	'change-ownership': {
		id: 'change-ownership',
		title: 'Change Ownership of an Account',
		description: 'Move one POSIX tenant to a reseller or back to direct (root) ownership, then queue reconcile.',
		emptyDetail: 'Search for the account whose reseller ownership you want to change.',
		accountFilter: 'active',
	},
	'modify-account': {
		id: 'modify-account',
		title: 'Modify an Account',
		description: 'Change domain, IP, package, reseller ownership, and login access for one tenant.',
		emptyDetail: 'Select the account to modify.',
		accountFilter: 'active',
	},
	'change-package': {
		id: 'change-package',
		title: 'Upgrade/Downgrade an Account',
		description: 'Move an account to a different resource package. Limits follow that package.',
		emptyDetail: 'Select the account whose package you want to change.',
		accountFilter: 'active',
	},
	'force-password': {
		id: 'force-password',
		title: 'Force Password Change',
		description: 'Rotate owner credentials and optionally require another change at next Control sign-in.',
		emptyDetail: 'Select the account whose owner password you want to rotate.',
		accountFilter: 'active',
	},
	'password-modification': {
		id: 'password-modification',
		title: 'Password Modification',
		description: 'Set a new owner password for an account without requiring a second change.',
		emptyDetail: 'Select the account whose owner password you want to set.',
		accountFilter: 'active',
	},
	'suspend-account': {
		id: 'suspend-account',
		title: 'Manage Account Suspension',
		description: 'Suspend or unsuspend an account and queue Linux, vhost, cron, and mail lockout.',
		emptyDetail: 'Select the account to suspend or unsuspend.',
		accountFilter: 'active',
	},
	'terminate-account': {
		id: 'terminate-account',
		title: 'Terminate Accounts',
		description: 'Permanently remove hosted services after typed confirmation.',
		emptyDetail: 'Select the account to terminate.',
		accountFilter: 'active',
	},
	'remove-terminated-account': {
		id: 'remove-terminated-account',
		title: 'Remove Terminated Accounts',
		description: 'Delete a terminated account from the panel so the username and domain can be reused.',
		emptyDetail: 'Select a terminated account to remove from the panel.',
		accountFilter: 'terminated',
	},
	'login-control': {
		id: 'login-control',
		title: 'Login to Kelmor Control',
		description: 'Open an audited HTTPS Control session as the account owner.',
		emptyDetail: 'Select the account to open in Kelmor Control.',
		accountFilter: 'active',
	},
	'account-summary': {
		id: 'account-summary',
		title: 'Account Summary',
		description: 'Open the hub for status, usage, isolation, and linked services for one tenant.',
		emptyDetail: 'Select the account whose summary you want to open.',
		accountFilter: 'all',
	},
	'api-tokens-whm': {
		id: 'api-tokens-whm',
		title: 'Manage API Tokens',
		description: 'Issue and revoke account-safe API tokens from Account Services.',
		emptyDetail: 'Select the account whose API tokens you want to manage.',
		accountFilter: 'active',
	},
} satisfies Record<string, AccountFunctionSpec>

type AccountFunctionKind = keyof typeof specs

interface AccountFunctionPageProps {
	toolId: AccountFunctionKind
}

export function AccountFunctionPage ({ toolId }: AccountFunctionPageProps) {
	const spec = specs[toolId]
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [resellers, setResellers] = useState<Reseller[]>([])
	const [filter, setFilter] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const selectedId = params.get('account') || ''
	const canModify = useCan('accounts.modify')
	const canSuspend = useCan('accounts.suspend')
	const canTerminate = useCan('accounts.terminate')
	const canImpersonate = useCan('accounts.impersonate')
	const canReadTokens = useCan('api_tokens.read')
	const canReadResellers = useCan('resellers.read')

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: Account[] }>('/api/v1/accounts'),
			api<{ items: Package[] }>('/api/v1/packages'),
			canReadResellers ? api<{ items: Reseller[] }>('/api/v1/resellers') : Promise.resolve({ items: [] }),
		]).then(([accountResult, packageResult, resellerResult]) => {
			if (accountResult.status === 'fulfilled') setAccounts(asList(accountResult.value))
			else setError(messageFrom(accountResult.reason))
			if (packageResult.status === 'fulfilled') setPackages(asList(packageResult.value))
			if (resellerResult.status === 'fulfilled') setResellers(asList(resellerResult.value))
		}).finally(() => setLoading(false))
	}
	useEffect(load, [canReadResellers])

	const visible = useMemo(() => accounts.filter((account) => {
		if (spec.accountFilter === 'terminated') return account.status === 'terminated'
		if (spec.accountFilter === 'active') return account.status !== 'terminated'
		return true
	}), [accounts, spec.accountFilter])

	function selectAccount (accountId: string) {
		const next = new URLSearchParams(params)
		if (accountId) next.set('account', accountId)
		else next.delete('account')
		setParams(next, { replace: true })
		setMessage('')
		setJobId('')
	}

	async function patchAccount (accountId: string, body: Record<string, unknown>, fallback: string) {
		setMessage('')
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}`, {
				method: 'PATCH',
				body: JSON.stringify(body),
			})
			setJobId(result.operation_id || '')
			setMessage(queuedOpMessage(result, fallback))
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function postAccount (accountId: string, action: string, body: Record<string, unknown>, fallback: string) {
		setMessage('')
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${accountId}/${action}`, {
				method: 'POST',
				body: JSON.stringify(body),
			})
			setJobId(result.operation_id || '')
			setMessage(queuedOpMessage(result, fallback))
			load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<AccountToolWizard
			title={spec.title}
			description={spec.description}
			accounts={visible}
			selectedId={selectedId}
			onSelect={selectAccount}
			filter={filter}
			onFilterChange={setFilter}
			loading={loading}
			error={error}
			onRetry={load}
			emptyDetail={spec.emptyDetail}
		>
			{(account) => (
				<>
					<QueuedOpNotice message={message} accountId={account.id} jobId={jobId} />
					<AccountFunctionAction
						toolId={toolId}
						account={account}
						packages={packages}
						resellers={resellers}
						canModify={canModify}
						canSuspend={canSuspend}
						canTerminate={canTerminate}
						canImpersonate={canImpersonate}
						canReadTokens={canReadTokens}
						onPatch={patchAccount}
						onPost={postAccount}
					/>
				</>
			)}
		</AccountToolWizard>
	)
}

interface AccountFunctionActionProps {
	toolId: AccountFunctionKind
	account: Account
	packages: Package[]
	resellers: Reseller[]
	canModify: boolean
	canSuspend: boolean
	canTerminate: boolean
	canImpersonate: boolean
	canReadTokens: boolean
	onPatch: (accountId: string, body: Record<string, unknown>, fallback: string) => Promise<void>
	onPost: (accountId: string, action: string, body: Record<string, unknown>, fallback: string) => Promise<void>
}

function AccountFunctionAction ({
	toolId,
	account,
	packages,
	resellers,
	canModify,
	canSuspend,
	canTerminate,
	canImpersonate,
	canReadTokens,
	onPatch,
	onPost,
}: AccountFunctionActionProps) {
	const navigate = useNavigate()
	const reseller = resellers.find((entry) => entry.id === account.reseller_id)
	const pkg = packages.find((entry) => entry.id === account.package_id)

	switch (toolId) {
		case 'change-ownership':
			return (
				<section className="panel">
					<h2>New ownership</h2>
					<p>Current owner: <strong>{reseller?.name || 'Direct Kelmor account'}</strong> · {account.username} · {account.primary_domain}</p>
					{canModify ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							void onPatch(account.id, { reseller_id: String(data.get('reseller_id') || '') }, `Ownership for ${account.username} queued.`)
						}}>
							<label>New owner
								<select name="reseller_id" defaultValue={account.reseller_id || ''} key={`${account.id}:${account.reseller_id || 'direct'}`}>
									<option value="">Direct Kelmor account</option>
									{resellers.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
								</select>
							</label>
							<ul>{lifecycleImpact('modify').map((line) => <li key={line}>{line}</li>)}</ul>
							<button type="submit">Change ownership</button>
						</form>
					) : <p className="subtle">Your role can view ownership but cannot change it.</p>}
				</section>
			)
		case 'modify-account':
			return (
				<section className="panel">
					<h2>Assignment</h2>
					{canModify ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							void onPatch(account.id, {
								package_id: data.get('package_id'),
								reseller_id: data.get('reseller_id'),
								primary_domain: data.get('primary_domain'),
								ip_address: data.get('ip_address'),
								login_disabled: data.get('login_disabled') === 'on',
							}, `Assignment for ${account.username} queued.`)
						}}>
							<label>Package<select name="package_id" defaultValue={account.package_id} key={`${account.id}-package`}>{packages.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
							<label>Reseller<select name="reseller_id" defaultValue={account.reseller_id || ''} key={`${account.id}-reseller`}><option value="">Direct Kelmor account</option>{resellers.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
							<label>Primary domain<input name="primary_domain" defaultValue={account.primary_domain} /></label>
							<label>IP address<input name="ip_address" defaultValue={account.ip_address} placeholder="Shared address" /></label>
							<label className="checkbox-label"><input name="login_disabled" type="checkbox" defaultChecked={account.login_disabled} /> Disable owner login</label>
							<ul>{lifecycleImpact('modify').map((line) => <li key={line}>{line}</li>)}</ul>
							<button type="submit">Save assignment</button>
						</form>
					) : <p className="subtle">Your role can view but not modify this account.</p>}
				</section>
			)
		case 'change-package':
			return (
				<section className="panel">
					<h2>Package</h2>
					<p>Current package: <strong>{pkg?.name || account.package_id}</strong></p>
					{canModify ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							void onPatch(account.id, { package_id: data.get('package_id') }, `Package change for ${account.username} queued.`)
						}}>
							<label>New package<select name="package_id" defaultValue={account.package_id} key={`${account.id}-pkg`}>{packages.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
							<button type="submit">Change package</button>
						</form>
					) : <p className="subtle">Your role cannot change packages.</p>}
				</section>
			)
		case 'force-password':
		case 'password-modification':
			return (
				<section className="panel">
					<h2>{toolId === 'force-password' ? 'Force a new password' : 'Set owner password'}</h2>
					{canModify ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							void onPost(account.id, 'password', {
								password: data.get('password'),
								must_change_password: data.get('must_change_password') === 'on',
							}, `Password rotation for ${account.username} queued.`)
						}}>
							<label>New password<input name="password" type="password" minLength={12} required autoFocus /></label>
							<label className="checkbox-label">
								<input name="must_change_password" type="checkbox" defaultChecked={toolId === 'force-password'} />
								Require the owner to change it at next login
							</label>
							<button type="submit">Rotate password</button>
						</form>
					) : <p className="subtle">Password rotation is unavailable to your role.</p>}
				</section>
			)
		case 'suspend-account':
			return (
				<section className="panel">
					<h2>{account.status === 'suspended' ? 'Unsuspend account' : 'Suspend account'}</h2>
					<ul>{lifecycleImpact(account.status === 'suspended' ? 'unsuspend' : 'suspend').map((line) => <li key={line}>{line}</li>)}</ul>
					{canSuspend ? (
						<button
							type="button"
							className={account.status === 'suspended' ? undefined : 'danger'}
							onClick={() => void onPost(account.id, account.status === 'suspended' ? 'unsuspend' : 'suspend', {}, `${account.username} queued for ${account.status === 'suspended' ? 'unsuspend' : 'suspend'}.`)}
						>
							{account.status === 'suspended' ? 'Confirm unsuspend' : 'Confirm suspend'}
						</button>
					) : <p className="subtle">Suspend is unavailable to your role.</p>}
				</section>
			)
		case 'terminate-account':
			return (
				<section className="panel danger-panel">
					<h2>Terminate {account.username}</h2>
					<ul>{lifecycleImpact('terminate').map((line) => <li key={line}>{line}</li>)}</ul>
					{canTerminate ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							if (String(data.get('username')) !== account.username) return
							void onPost(account.id, 'terminate', {}, `Termination of ${account.username} queued.`)
						}}>
							<label>Type the username to confirm<input name="username" autoComplete="off" required /></label>
							<button type="submit" className="danger">Terminate account</button>
						</form>
					) : <p className="subtle">Terminate is unavailable to your role.</p>}
				</section>
			)
		case 'remove-terminated-account':
			return (
				<section className="panel danger-panel">
					<h2>Remove {account.username}</h2>
					<ul>{lifecycleImpact('remove').map((line) => <li key={line}>{line}</li>)}</ul>
					{canTerminate ? (
						<form onSubmit={(event) => {
							event.preventDefault()
							const data = new FormData(event.currentTarget)
							if (String(data.get('username')) !== account.username) return
							void onPost(account.id, 'remove', {}, `${account.username} removed from the panel.`)
						}}>
							<label>Type the username to confirm<input name="username" autoComplete="off" required /></label>
							<button type="submit" className="danger">Remove account</button>
						</form>
					) : <p className="subtle">Removal is unavailable to your role.</p>}
				</section>
			)
		case 'login-control':
			return (
				<section className="panel">
					<h2>Open Kelmor Control</h2>
					<p>Starts a 30-minute audited Control session for {account.username}. Control opens at the public HTTPS tenant URL.</p>
					{canImpersonate
						? <LoginToControl accountId={account.id} username={account.username} variant="button" />
						: <p className="subtle">Impersonation is unavailable to your role.</p>}
				</section>
			)
		case 'account-summary':
			return (
				<section className="panel">
					<h2>Open {account.username}</h2>
					<p>{account.primary_domain} · {account.status}</p>
					<div className="button-row">
						<Link className="button-link" to={`/accounts/${account.id}`}>Open account summary</Link>
						<button type="button" className="secondary" onClick={() => navigate(`/accounts/${account.id}`)}>Continue to summary</button>
					</div>
				</section>
			)
		case 'api-tokens-whm':
			return (
				<section className="panel">
					<h2>API tokens for {account.username}</h2>
					<p>Tokens are issued from Account Services. This page is the token tool, not the account inventory.</p>
					{canReadTokens
						? <Link className="button-link" to={`/accounts/${account.id}/services?service=tokens`}>Open API tokens</Link>
						: <p className="subtle">API token management is unavailable to your role.</p>}
				</section>
			)
		default: {
			const exhaustive: never = toolId
			return exhaustive
		}
	}
}

export const accountFunctionRouteIds = Object.keys(specs) as AccountFunctionKind[]

export function accountFunctionPath (toolId: AccountFunctionKind): string {
	return dedicatedPath(toolId)
}
