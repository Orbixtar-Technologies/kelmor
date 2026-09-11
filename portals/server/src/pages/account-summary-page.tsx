import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountTabs, CopyableValue, Dialog, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom } from '../helpers'
import { bandwidthEnforcementCopy, isolationLines, lifecycleImpact, quotaEnforcementCopy } from './account-lifecycle-copy'
import { describeJobFailure, formatJobType } from './job-copy'
import { useCan } from '../rbac'
import type { Account, Job, Package, Reseller, Usage } from '../types'

type LifecycleAction = 'suspend' | 'unsuspend'

export function AccountSummaryPage () {
	const { id = '' } = useParams()
	const [searchParams] = useSearchParams()
	const task = searchParams.get('task') || ''
	const [account, setAccount] = useState<Account | null>(null)
	const [packages, setPackages] = useState<Package[]>([])
	const [resellers, setResellers] = useState<Reseller[]>([])
	const [usage, setUsage] = useState<Usage | null>(null)
	const [jobs, setJobs] = useState<Job[]>([])
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [terminateError, setTerminateError] = useState('')
	const [passwordOpen, setPasswordOpen] = useState(false)
	const [terminateOpen, setTerminateOpen] = useState(false)
	const [editingAssignment, setEditingAssignment] = useState(false)
	const [confirmation, setConfirmation] = useState('')
	const [pendingLifecycle, setPendingLifecycle] = useState<LifecycleAction | null>(null)
	const requestSequence = useRef(0)
	const assignmentRef = useRef<HTMLElement>(null)
	const navigate = useNavigate()
	const canModify = useCan('accounts.modify')
	const canSuspend = useCan('accounts.suspend')
	const canTerminate = useCan('accounts.terminate')
	function closeTerminate () {
		setTerminateOpen(false)
		setConfirmation('')
		setTerminateError('')
	}
	function openTerminate () {
		setConfirmation('')
		setTerminateError('')
		setTerminateOpen(true)
	}

	const load = useCallback(() => {
		const sequence = ++requestSequence.current
		setAccount(null)
		setUsage(null)
		setJobs([])
		setError('')
		Promise.allSettled([
			api<Account>(`/api/v1/accounts/${id}`),
			api<{ items: Package[] }>('/api/v1/packages'),
			api<{ items: Reseller[] }>('/api/v1/resellers'),
			api<Usage>(`/api/v1/accounts/${id}/usage`),
			api<{ items: Job[] }>('/api/v1/jobs'),
		]).then(([accountResult, packageResult, resellerResult, usageResult, jobResult]) => {
			if (sequence !== requestSequence.current) return
			if (accountResult.status === 'fulfilled') setAccount(accountResult.value)
			else setError(messageFrom(accountResult.reason))
			if (packageResult.status === 'fulfilled') setPackages(asList(packageResult.value))
			if (resellerResult.status === 'fulfilled') setResellers(asList(resellerResult.value))
			if (usageResult.status === 'fulfilled') setUsage(usageResult.value)
			if (jobResult.status === 'fulfilled') setJobs(asList(jobResult.value).filter((job) => job.resource_id === id || job.payload.account_id === id))
		})
	}, [id])
	useLayoutEffect(() => {
		load()
		return () => { requestSequence.current += 1 }
	}, [load])
	useEffect(() => {
		if (account?.id !== id) return
		if (task === 'password' && canModify) setPasswordOpen(true)
		if (task === 'terminate' && canTerminate) openTerminate()
		if (['package', 'modify'].includes(task)) {
			setEditingAssignment(true)
			assignmentRef.current?.scrollIntoView({ block: 'center' })
		}
	}, [account?.id, canModify, canTerminate, id, task])

	async function postAction (action: string): Promise<boolean> {
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${id}/${action}`, { method: 'POST', body: '{}' })
			setTerminateError('')
			setMessage(result.operation_id ? 'Queued. Track progress in Activity.' : `${action} requested.`)
			load()
			return true
		} catch (requestError) {
			const nextMessage = messageFrom(requestError)
			setMessage(nextMessage)
			if (action === 'terminate') setTerminateError(nextMessage)
			return false
		}
	}
	async function patchAccount (body: Record<string, unknown>) {
		try {
			await api(`/api/v1/accounts/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
			setMessage('Account settings updated.')
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}

	if ((!account || account.id !== id) && !error) return <><PageHeader title="Account Summary" description="Loading account state." /><LoadingState /></>
	if (!account || account.id !== id) return <><PageHeader title="Account Summary" description="Account state and services." /><ErrorState error={error} onRetry={load} /></>
	const pkg = packages.find((entry) => entry.id === account.package_id)
	const reseller = resellers.find((entry) => entry.id === account.reseller_id)
	const failedJobs = jobs.filter((job) => job.state === 'failed')
	const attention = [
		account.status === 'suspended' ? 'This account is suspended.' : '',
		account.observed_revision !== account.desired_revision ? 'Saved settings have not fully applied yet.' : '',
		failedJobs.length ? `${failedJobs.length} recent operation${failedJobs.length === 1 ? '' : 's'} failed.` : '',
	].filter(Boolean)
	return (
		<>
			<PageHeader title={account.username} description={`${account.primary_domain} · POSIX tenant ${account.status}`} actions={<>{canSuspend ? <button type="button" className={account.status === 'suspended' ? 'secondary' : 'danger'} onClick={() => setPendingLifecycle(account.status === 'suspended' ? 'unsuspend' : 'suspend')}>{account.status === 'suspended' ? 'Unsuspend' : 'Suspend'}</button> : null}<Link className="button-link" to={`/accounts/${id}/services`}>Manage services</Link></>} />
			<AccountTabs id={id} />
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<div className="summary-grid">
				<section className="panel"><h2>Account health</h2><dl className="detail-list">
					<div><dt>Status</dt><dd><StatusBadge value={account.status} /></dd></div>
					<div><dt>Primary domain</dt><dd><CopyableValue value={account.primary_domain} label="domain" /></dd></div>
					<div><dt>Plan</dt><dd>{pkg?.name || account.package_id}</dd></div>
					<div><dt>Home</dt><dd><CopyableValue value={account.home_path} label="home path" /></dd></div>
					<div><dt>Quota</dt><dd>{usage ? `${formatBytes(usage.disk_bytes)} / ${formatBytes(pkg?.disk_bytes)} disk` : 'Usage not yet collected'}</dd></div>
					<div><dt>Settings sync</dt><dd>{account.observed_revision === account.desired_revision ? 'Matches saved settings' : `Applying revision ${account.observed_revision} of ${account.desired_revision}`}</dd></div>
					<div><dt>Needs attention</dt><dd>{attention.length ? attention.join(' ') : 'No open issues'}</dd></div>
				</dl></section>
				<section className="panel"><h2>Usage</h2>{usage ? <dl className="detail-list"><div><dt>Disk</dt><dd>{formatBytes(usage.disk_bytes)} / {formatBytes(pkg?.disk_bytes)}</dd></div><div><dt>Bandwidth</dt><dd>{formatBytes(usage.bandwidth_bytes)} / {formatBytes(pkg?.bandwidth_bytes_monthly)}</dd></div><div><dt>Memory</dt><dd>{formatBytes(usage.memory_bytes)}</dd></div><div><dt>Processes</dt><dd>{usage.process_count}</dd></div></dl> : <p>Usage is not yet available for this account.</p>}<p className="subtle">{quotaEnforcementCopy()}</p><p className="subtle">{bandwidthEnforcementCopy()}</p></section>
				<section className="panel"><h2>POSIX isolation</h2><dl className="detail-list">
					<div><dt>Linux identity</dt><dd>UID {account.linux_uid} / GID {account.linux_gid}</dd></div>
					<div><dt>Home</dt><dd><CopyableValue value={account.home_path} label="home path" /></dd></div>
					<div><dt>IP address</dt><dd>{account.ip_address ? <CopyableValue value={account.ip_address} label="IP" /> : 'Shared address'}</dd></div>
					<div><dt>Shell</dt><dd>{account.shell_class}</dd></div>
					<div><dt>Owner login</dt><dd>{account.login_disabled ? 'Disabled' : 'Enabled'}</dd></div>
					<div><dt>Database prefix</dt><dd><code>{account.username}_</code></dd></div>
				</dl><ul className="isolation-list">{isolationLines(account).map((line) => <li key={line}>{line}</li>)}</ul></section>
				<section ref={assignmentRef} className={`panel ${['package', 'modify'].includes(task) || editingAssignment ? 'task-focus' : ''}`}><h2>Package & ownership</h2>
					<p className="subtle">Current: {pkg?.name || account.package_id} · {reseller?.name || 'Direct'}</p>
					{editingAssignment ? <form onSubmit={(event) => {
						event.preventDefault()
						const data = new FormData(event.currentTarget)
						const body = { package_id: data.get('package_id'), reseller_id: data.get('reseller_id'), primary_domain: data.get('primary_domain'), ip_address: data.get('ip_address'), login_disabled: data.get('login_disabled') === 'on' }
						if (!window.confirm(`Apply the reviewed package, ownership, domain, IP, and login settings to ${account.username}? ${lifecycleImpact('modify').join(' ')}`)) return
						void patchAccount(body)
						setEditingAssignment(false)
					}}>
						<label>Package<select name="package_id" defaultValue={account.package_id}>{packages.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
						<label>Reseller<select name="reseller_id" defaultValue={account.reseller_id || ''}><option value="">Direct Kelmor account</option>{resellers.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
						<label>Primary domain<input name="primary_domain" defaultValue={account.primary_domain} /></label>
						<label>IP address<input name="ip_address" defaultValue={account.ip_address} placeholder="Shared address" /></label>
						<label className="checkbox-label"><input name="login_disabled" type="checkbox" defaultChecked={account.login_disabled} /> Disable owner login</label>
						<div className="button-row">{canModify ? <button type="submit">Save assignment</button> : <p className="subtle">Your role can view but not change this assignment.</p>}<button type="button" className="secondary" onClick={() => setEditingAssignment(false)}>Cancel</button></div>
					</form> : <div className="button-row">{canModify ? <button type="button" className="secondary" onClick={() => setEditingAssignment(true)}>Edit assignment</button> : <p className="subtle">Your role can view but not change this assignment.</p>}</div>}
				</section>
				<section className="panel"><h2>Credentials</h2><p>Rotate the owner password used for Kelmor Control. This does not remove the account.</p><div className="button-row">{canModify ? <button type="button" onClick={() => setPasswordOpen(true)}>Rotate password</button> : <p className="subtle">Password rotation is unavailable to your role.</p>}</div></section>
				<section className="panel danger-panel"><h2>Terminate account</h2><p>Permanently removes hosted services, prefixed databases, DNS zones, and the Linux identity.</p><div className="button-row">{canTerminate ? <button type="button" className="danger" onClick={openTerminate}>Terminate account</button> : <p className="subtle">Termination is unavailable to your role.</p>}</div></section>
			</div>
			<section className="panel"><h2>Recent operations</h2><div className="table-wrap"><table><thead><tr><th scope="col">Operation</th><th scope="col">When</th><th scope="col">State</th><th scope="col">Next step</th></tr></thead><tbody>{jobs.slice(0, 8).map((job) => {
				const failure = describeJobFailure(job)
				return <tr key={job.id}><td>{formatJobType(job.type)}<small>{failure.resource}</small></td><td>{formatDate(job.created_at)}</td><td><StatusBadge value={job.state} /></td><td>{job.state === 'failed' ? <><span>{failure.reason}</span><Link className="link-button" to={`/jobs?account=${id}&selected=${job.id}`}>View details</Link></> : '—'}</td></tr>
			})}</tbody></table></div>{!jobs.length ? <p>No related jobs yet.</p> : <p className="subtle"><Link to={`/jobs?account=${id}`}>Open all account activity</Link></p>}</section>
			<Dialog open={passwordOpen} title="Rotate account password" onClose={() => setPasswordOpen(false)}>
				<form onSubmit={async (event) => {
					event.preventDefault()
					const data = new FormData(event.currentTarget)
					try {
						await api(`/api/v1/accounts/${id}/password`, { method: 'POST', body: JSON.stringify({ password: data.get('password'), must_change_password: data.get('must_change_password') === 'on' }) })
						setMessage('Password rotation queued.')
						setPasswordOpen(false)
						navigate(`/accounts/${id}`, { replace: true })
					} catch (requestError) { setMessage(messageFrom(requestError)) }
				}}>
					<label>New password<input name="password" type="password" minLength={12} required autoFocus /></label>
					<label className="checkbox-label"><input name="must_change_password" type="checkbox" /> Require the owner to change it at next login</label>
					<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setPasswordOpen(false)}>Cancel</button><button type="submit">Rotate password</button></footer>
				</form>
			</Dialog>
			<Dialog open={Boolean(pendingLifecycle)} title={pendingLifecycle === 'unsuspend' ? `Unsuspend ${account.username}` : `Suspend ${account.username}`} onClose={() => setPendingLifecycle(null)} actions={<>
				<button type="button" className="secondary" onClick={() => setPendingLifecycle(null)}>Cancel</button>
				<button type="button" className={pendingLifecycle === 'suspend' ? 'danger' : undefined} onClick={async () => {
					if (!pendingLifecycle) return
					if (await postAction(pendingLifecycle)) setPendingLifecycle(null)
				}}>{pendingLifecycle === 'unsuspend' ? 'Confirm unsuspend' : 'Confirm suspend'}</button>
			</>}>
				<ul>{lifecycleImpact(pendingLifecycle === 'unsuspend' ? 'unsuspend' : 'suspend').map((line) => <li key={line}>{line}</li>)}</ul>
				<p className="subtle">Queued work appears in <Link to={`/jobs?account=${id}`}>Jobs</Link>.</p>
			</Dialog>
			<Dialog open={terminateOpen} title={`Terminate ${account.username}`} onClose={closeTerminate} actions={<>
				<button type="button" className="secondary" onClick={closeTerminate}>Cancel</button>
				<button type="button" className="danger" disabled={confirmation !== account.username} onClick={async () => { if (await postAction('terminate')) { closeTerminate(); navigate('/accounts') } }}>Terminate permanently</button>
			</>}>
				<ul>{lifecycleImpact('terminate').map((line) => <li key={line}>{line}</li>)}</ul>
				<p>Enter <strong>{account.username}</strong> to continue.</p>
				{terminateError ? <p className="feedback" role="alert">{terminateError}</p> : null}
				<label>Username confirmation<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoFocus /></label>
			</Dialog>
		</>
	)
}
