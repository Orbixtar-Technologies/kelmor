import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountTabs, Dialog, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Account, Job, Package, Reseller, Usage } from '../types'

export function AccountSummaryPage () {
	const { id = '' } = useParams()
	const [account, setAccount] = useState<Account | null>(null)
	const [packages, setPackages] = useState<Package[]>([])
	const [resellers, setResellers] = useState<Reseller[]>([])
	const [usage, setUsage] = useState<Usage | null>(null)
	const [jobs, setJobs] = useState<Job[]>([])
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [passwordOpen, setPasswordOpen] = useState(false)
	const [terminateOpen, setTerminateOpen] = useState(false)
	const [confirmation, setConfirmation] = useState('')
	const navigate = useNavigate()
	const canModify = useCan('accounts.modify')
	const canSuspend = useCan('accounts.suspend')
	const canTerminate = useCan('accounts.terminate')

	const load = useCallback(() => {
		setError('')
		Promise.allSettled([
			api<Account>(`/api/v1/accounts/${id}`),
			api<{ items: Package[] }>('/api/v1/packages'),
			api<{ items: Reseller[] }>('/api/v1/resellers'),
			api<Usage>(`/api/v1/accounts/${id}/usage`),
			api<{ items: Job[] }>('/api/v1/jobs'),
		]).then(([accountResult, packageResult, resellerResult, usageResult, jobResult]) => {
			if (accountResult.status === 'fulfilled') setAccount(accountResult.value)
			else setError(messageFrom(accountResult.reason))
			if (packageResult.status === 'fulfilled') setPackages(asList(packageResult.value))
			if (resellerResult.status === 'fulfilled') setResellers(asList(resellerResult.value))
			if (usageResult.status === 'fulfilled') setUsage(usageResult.value)
			if (jobResult.status === 'fulfilled') setJobs(asList(jobResult.value).filter((job) => job.resource_id === id || job.payload.account_id === id))
		})
	}, [id])
	useEffect(load, [load])

	async function postAction (action: string) {
		try {
			const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${id}/${action}`, { method: 'POST', body: '{}' })
			setMessage(result.operation_id ? `Queued operation ${result.operation_id}.` : `${action} requested.`)
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	async function patchAccount (body: Record<string, unknown>) {
		try {
			await api(`/api/v1/accounts/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
			setMessage('Account settings updated.')
			load()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}

	if (!account && !error) return <><PageHeader title="Account Summary" description="Loading account state." /><LoadingState /></>
	if (!account) return <><PageHeader title="Account Summary" description="Account state and services." /><ErrorState error={error} onRetry={load} /></>
	const pkg = packages.find((entry) => entry.id === account.package_id)
	const reseller = resellers.find((entry) => entry.id === account.reseller_id)
	return (
		<>
			<PageHeader title={`Account Summary: ${account.username}`} description={`${account.primary_domain} · ${account.home_path}`} actions={<>{canSuspend ? <button type="button" className="secondary" onClick={() => postAction(account.status === 'suspended' ? 'unsuspend' : 'suspend')}>{account.status === 'suspended' ? 'Unsuspend' : 'Suspend'}</button> : null}<Link className="button-link" to={`/accounts/${id}/services`}>Manage services</Link></>} />
			<AccountTabs id={id} />
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<div className="summary-grid">
				<section className="panel"><h2>Overview</h2><dl className="detail-list">
					<div><dt>Status</dt><dd><StatusBadge value={account.status} /></dd></div><div><dt>Primary domain</dt><dd>{account.primary_domain}</dd></div>
					<div><dt>Linux identity</dt><dd>UID {account.linux_uid} / GID {account.linux_gid}</dd></div><div><dt>IP address</dt><dd>{account.ip_address || 'Shared address'}</dd></div>
					<div><dt>Shell</dt><dd>{account.shell_class}</dd></div><div><dt>Login</dt><dd>{account.login_disabled ? 'Disabled' : 'Enabled'}</dd></div>
					<div><dt>Reconciliation</dt><dd>{account.observed_revision} / {account.desired_revision}</dd></div>
				</dl></section>
				<section className="panel"><h2>Package & ownership</h2><form onSubmit={(event) => { event.preventDefault(); const data = new FormData(event.currentTarget); patchAccount({ package_id: data.get('package_id'), reseller_id: data.get('reseller_id'), primary_domain: data.get('primary_domain'), ip_address: data.get('ip_address'), login_disabled: data.get('login_disabled') === 'on' }) }}>
					<label>Package<select name="package_id" defaultValue={account.package_id}>{packages.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
					<label>Reseller<select name="reseller_id" defaultValue={account.reseller_id || ''}><option value="">Direct Kelmor account</option>{resellers.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
					<label>Primary domain<input name="primary_domain" defaultValue={account.primary_domain} /></label>
					<label>IP address<input name="ip_address" defaultValue={account.ip_address} placeholder="Shared address" /></label>
					<label className="checkbox-label"><input name="login_disabled" type="checkbox" defaultChecked={account.login_disabled} /> Disable owner login</label>
					{canModify ? <button type="submit">Save assignment</button> : <p className="subtle">Your role can view but not change this assignment.</p>}
				</form><p className="subtle">Current: {pkg?.name || account.package_id} · {reseller?.name || 'Direct'}</p></section>
				<section className="panel"><h2>Usage snapshot</h2>{usage ? <dl className="detail-list"><div><dt>Disk</dt><dd>{formatBytes(usage.disk_bytes)} / {formatBytes(pkg?.disk_bytes)}</dd></div><div><dt>Bandwidth</dt><dd>{formatBytes(usage.bandwidth_bytes)} / {formatBytes(pkg?.bandwidth_bytes_monthly)}</dd></div><div><dt>Memory</dt><dd>{formatBytes(usage.memory_bytes)}</dd></div><div><dt>Processes</dt><dd>{usage.process_count}</dd></div></dl> : <p>Usage is not yet available for this account.</p>}</section>
				<section className="panel"><h2>Security actions</h2><p>Rotate owner credentials or permanently terminate this account.</p><div className="button-row">{canModify ? <button type="button" onClick={() => setPasswordOpen(true)}>Rotate password</button> : null}{canTerminate ? <button type="button" className="danger" onClick={() => setTerminateOpen(true)}>Terminate account</button> : null}</div>{!canModify && !canTerminate ? <p className="subtle">No lifecycle actions are available to your role.</p> : null}</section>
			</div>
			<section className="panel"><h2>Recent related jobs</h2><div className="table-wrap"><table><thead><tr><th>Type</th><th>State</th><th>Progress</th><th>Error</th></tr></thead><tbody>{jobs.slice(0, 8).map((job) => <tr key={job.id}><td>{job.type}</td><td><StatusBadge value={job.state} /></td><td>{job.progress}%</td><td>{job.last_error || '—'}</td></tr>)}</tbody></table></div>{!jobs.length ? <p>No related jobs yet.</p> : null}</section>
			<Dialog open={passwordOpen} title="Rotate account password" onClose={() => setPasswordOpen(false)}>
				<form onSubmit={async (event) => {
					event.preventDefault()
					const data = new FormData(event.currentTarget)
					try {
						await api(`/api/v1/accounts/${id}/password`, { method: 'POST', body: JSON.stringify({ password: data.get('password'), force_change: data.get('force_change') === 'on' }) })
						setMessage('Password rotation queued.'); setPasswordOpen(false); load()
					} catch (requestError) { setMessage(messageFrom(requestError)) }
				}}>
					<label>New password<input name="password" type="password" minLength={12} required autoFocus /></label>
					<label className="checkbox-label"><input name="force_change" type="checkbox" /> Require the owner to change it at next login</label>
					<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setPasswordOpen(false)}>Cancel</button><button type="submit">Rotate password</button></footer>
				</form>
			</Dialog>
			<Dialog open={terminateOpen} title={`Terminate ${account.username}`} onClose={() => { setTerminateOpen(false); setConfirmation('') }}>
				<p>This permanently removes account services and the Linux identity. Enter <strong>{account.username}</strong> to continue.</p>
				<label>Username confirmation<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoFocus /></label>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setTerminateOpen(false)}>Cancel</button><button type="button" className="danger" disabled={confirmation !== account.username} onClick={async () => { await postAction('terminate'); setTerminateOpen(false); navigate('/accounts') }}>Terminate permanently</button></footer>
			</Dialog>
		</>
	)
}
