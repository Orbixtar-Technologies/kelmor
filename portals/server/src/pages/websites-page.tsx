import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom, valueOf } from '../helpers'
import {
	pendingPhpChanges,
	stageSelectedPhpVersions,
	websitePhpVersion,
	type PhpWebsiteRow,
} from '../multiphp-staging'
import {
	installedPHPVersions,
	isInstalledPHPVersion,
	isSupportedPHPVersion,
	missingHostPHPVersion,
	type PHPRuntime,
} from '../php-runtimes'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

interface WebsiteRow extends PhpWebsiteRow {
	account_username?: string
}

export function WebsitesPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [domains, setDomains] = useState<ResourceItem[]>([])
	const [rows, setRows] = useState<WebsiteRow[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [drafts, setDrafts] = useState<Record<string, string>>({})
	const [selected, setSelected] = useState<Record<string, boolean>>({})
	const [phpRuntimes, setPhpRuntimes] = useState<PHPRuntime[]>([])
	const [inventoryError, setInventoryError] = useState('')
	const [bulkVersion, setBulkVersion] = useState('')
	const [step, setStep] = useState(0)
	const [busy, setBusy] = useState(false)
	const [updatedAt, setUpdatedAt] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('websites.write')
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const account = accounts.find((entry) => entry.id === accountId)
	const pending = useMemo(() => pendingPhpChanges(rows, drafts), [drafts, rows])
	const installed = useMemo(() => installedPHPVersions(phpRuntimes), [phpRuntimes])
	const missingHostVersions = useMemo(() => {
		const found = new Set<string>()
		for (const website of rows) {
			if (String(website.runtime || 'php') !== 'php') continue
			const missing = missingHostPHPVersion(websitePhpVersion(website), installed)
			if (missing) found.add(missing)
		}
		return [...found]
	}, [installed, rows])
	const isReview = step === 1
	const selectedCount = rows.filter((website) => selected[website.id]).length
	const allSelected = rows.length > 0 && selectedCount === rows.length

	useEffect(() => {
		setDrafts({})
		setSelected({})
		setStep(0)
	}, [accountId])

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			setAccounts(asList(result))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests])

	useEffect(() => {
		const request = requests.begin('runtimes')
		setInventoryError('')
		api<{ items?: PHPRuntime[] }>('/api/v1/server/runtimes').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setPhpRuntimes(next)
			const versions = installedPHPVersions(next)
			setBulkVersion((current) => versions.includes(current) ? current : (versions[0] || ''))
		}).catch((requestError) => {
			if (!requests.isCurrent(request)) return
			setPhpRuntimes([])
			setBulkVersion('')
			setInventoryError(messageFrom(requestError) || 'Could not load installed PHP versions.')
		})
	}, [requests])

	const loadWebsites = useCallback((requestedAccountId: string, inventory: Account[]) => {
		const request = requests.begin('websites')
		setLoading(true)
		setError('')
		const targets = requestedAccountId
			? inventory.filter((entry) => entry.id === requestedAccountId)
			: inventory
		if (!targets.length) {
			setRows([])
			setDomains([])
			setLoading(false)
			return
		}
		const domainLoad = requestedAccountId
			? api<{ items: ResourceItem[] }>(`/api/v1/accounts/${requestedAccountId}/domains`).then((result) => {
				if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setDomains(asList(result))
			}).catch(() => {
				if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setDomains([])
			})
			: Promise.resolve(setDomains([]))
		const siteLoads = Promise.allSettled(targets.map((entry) => api<{ items: ResourceItem[] }>(`/api/v1/accounts/${entry.id}/websites`).then((result) => ({
			account: entry,
			items: asList(result),
		})))).then((results) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			const next: WebsiteRow[] = []
			let failures = 0
			results.forEach((result) => {
				if (result.status === 'rejected') {
					failures += 1
					return
				}
				result.value.items.forEach((item) => next.push({
					...item,
					account_id: result.value.account.id,
					account_username: result.value.account.username,
				}))
			})
			setRows(next)
			setUpdatedAt(new Date().toISOString())
			if (failures === results.length) setError('Could not load websites.')
		})
		Promise.allSettled([domainLoad, siteLoads]).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		if (!accounts.length) {
			setLoading(false)
			return
		}
		loadWebsites(accountId, accounts)
	}, [accountId, accounts, loadWebsites])

	async function createWebsite (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		if (!accountId) return
		const data = new FormData(event.currentTarget)
		const runtime = String(data.get('runtime') || 'php')
		const runtimeVersion = String(data.get('runtime_version') || '')
		if (runtime === 'php') {
			const problem = phpVersionMessage(runtimeVersion)
			if (problem) {
				setMessage(problem)
				return
			}
		}
		try {
			await api(`/api/v1/accounts/${accountId}/websites`, {
				method: 'POST',
				body: JSON.stringify({
					domain_id: data.get('domain_id'),
					runtime,
					runtime_version: runtimeVersion || undefined,
					document_root: `${account?.home_path || ''}/public_html`,
				}),
			})
			setMessage('Website provision queued. Changing PHP on an existing domain updates that site.')
			event.currentTarget.reset()
			loadWebsites(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	function phpVersionMessage (version: string) {
		if (!installed.length) return 'php-fpm is not installed'
		if (!isSupportedPHPVersion(version)) return `Unsupported PHP version ${version}.`
		if (!isInstalledPHPVersion(version, installed)) return `php-fpm ${version} is not installed`
		return ''
	}

	function stageVersion (website: WebsiteRow, version: string) {
		if (String(website.runtime || 'php') === 'php') {
			const problem = phpVersionMessage(version)
			if (problem) {
				setMessage(problem)
				return
			}
		}
		setMessage('')
		setDrafts((current) => ({ ...current, [website.id]: version }))
	}

	function handleStageSelected () {
		const problem = phpVersionMessage(bulkVersion)
		if (problem) {
			setMessage(problem)
			return
		}
		setMessage('')
		setDrafts((current) => stageSelectedPhpVersions(rows, selected, bulkVersion, current))
	}

	function handleToggle (websiteId: string, checked: boolean) {
		setSelected((current) => ({ ...current, [websiteId]: checked }))
	}

	function handleToggleAll (checked: boolean) {
		const next: Record<string, boolean> = {}
		if (checked) {
			for (const website of rows) next[website.id] = true
		}
		setSelected(next)
	}

	async function applyPending () {
		if (!pending.length) return
		for (const change of pending) {
			if (change.runtime === 'php') {
				const problem = phpVersionMessage(change.proposed)
				if (problem) {
					setMessage(problem)
					return
				}
			}
		}
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			let lastJob = ''
			for (const change of pending) {
				const result = await api<{ operation_id?: string }>(`/api/v1/accounts/${change.accountId}/websites`, {
					method: 'POST',
					body: JSON.stringify({
						domain_id: change.domainId,
						runtime: change.runtime,
						runtime_version: change.proposed,
						document_root: change.documentRoot,
					}),
				})
				if (result.operation_id) lastJob = result.operation_id
			}
			setMessage(queuedOpMessage({ operation_id: lastJob || undefined }, `Queued PHP version changes for ${pending.length} site(s).`))
			setJobId(lastJob)
			setDrafts({})
			setSelected({})
			setStep(0)
			loadWebsites(accountId, accounts)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}

	function handleStagingSubmit (event: FormEvent) {
		event.preventDefault()
		if (!isReview) {
			if (!pending.length) return
			setStep(1)
			return
		}
		void applyPending()
	}

	return (
		<>
			<PageHeader
				title={account ? `MultiPHP Manager · ${account.username}` : 'MultiPHP Manager'}
				description="Review website runtimes and queue PHP version changes for account vhosts, matching the WHM MultiPHP Manager journey."
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Websites" onChange={(next) => setParams(next ? { account: next } : {}, { replace: true })} />
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams(next ? { account: next } : {}, { replace: true })}
				/>
				<p className="subtle">Select an account to create a site or change its PHP version. The inventory can list every visible website. PHP selector changes stay local until you Review and Apply.</p>
			</div>
			<QueuedOpNotice message={message} accountId={accountId || undefined} jobId={jobId} />
			{inventoryError ? <ErrorState title="Could not load PHP inventory" error={inventoryError} /> : null}
			{missingHostVersions.length ? (
				<p className="feedback" role="alert">
					PHP {missingHostVersions.join(', ')} is not installed on this host. Choose an installed version.
				</p>
			) : null}
			{error ? <ErrorState error={error} onRetry={() => loadWebsites(accountId, accounts)} /> : null}
			{canWrite && accountId ? <section className="panel">
				<h2>Create or update a website</h2>
				<form className="inline-form" onSubmit={createWebsite}>
					<label>Domain<select name="domain_id" required>{domains.map((item) => <option key={item.id} value={item.id}>{valueOf(item, 'ascii_fqdn')}</option>)}</select></label>
					<label>Runtime<select name="runtime"><option value="php">PHP</option><option value="static">Static</option><option value="node">Node</option><option value="python">Python</option></select></label>
					<label>PHP version<select name="runtime_version">{installed.map((version) => <option key={version} value={version}>{version}</option>)}</select></label>
					<button type="submit" disabled={!domains.length}>Save website</button>
				</form>
			</section> : null}
			<section className="panel">
				<h2>Websites</h2>
				{loading ? <LoadingState label="Loading websites…" /> : null}
				{!loading && canWrite && rows.length ? (
					<form className="form-panel" onSubmit={handleStagingSubmit}>
						<ol className="steps" aria-label="Workflow">
							{['Select PHP versions', 'Review'].map((label, index) => (
								<li key={label} className={index === step ? 'active' : index < step ? 'complete' : ''}>
									<span>{index + 1}</span>{label}
								</li>
							))}
						</ol>
						<div className="form-section-heading">
							<h2>{isReview ? 'Review' : 'Select PHP versions'}</h2>
							<p>{isReview
								? `Director will queue a host php-fpm apply for ${pending.length} site(s).`
								: 'Changing a selector only stages a review. Apply queues website.provision through the Agent.'}</p>
						</div>
						{!isReview ? (
							<div className="inline-form">
								<label>
									PHP version for selected sites
									<select
										aria-label="PHP version for selected sites"
										value={bulkVersion}
										onChange={(event) => setBulkVersion(event.target.value)}
									>
										{installed.map((version) => <option key={version} value={version}>{version}</option>)}
									</select>
								</label>
								<button type="button" disabled={!selectedCount} onClick={handleStageSelected}>Stage selected</button>
							</div>
						) : null}
						<div className="table-wrap"><table className="dense-table">
							<thead>
								<tr>
									{!isReview ? (
										<th>
											<input
												type="checkbox"
												checked={allSelected}
												aria-label="Select all websites"
												onChange={(event) => handleToggleAll(event.target.checked)}
											/>
										</th>
									) : null}
									<th>Document root</th>
									{isReview ? <th>Change</th> : <><th>Runtime</th><th>Version</th></>}
									<th>Account</th>
									{isReview ? null : <><th>Enabled</th><th>Actions</th></>}
								</tr>
							</thead>
							<tbody>
								{(isReview ? pending.map((change) => rows.find((website) => website.id === change.websiteId)).filter(Boolean) as WebsiteRow[] : rows).map((website) => {
									const ownerId = String(website.account_id || accountId)
									const root = valueOf(website, 'document_root')
									const current = websitePhpVersion(website)
									const proposed = drafts[website.id] || current
									const selectedVersion = isInstalledPHPVersion(proposed, installed) ? proposed : ''
									const recordedMissing = Boolean(missingHostPHPVersion(current, installed))
									return (
										<tr key={`${ownerId}:${website.id}`}>
											{!isReview ? (
												<td>
													<input
														type="checkbox"
														checked={Boolean(selected[website.id])}
														aria-label={`Select ${root}`}
														onChange={(event) => handleToggle(website.id, event.target.checked)}
													/>
												</td>
											) : null}
											<td><strong>{root}</strong></td>
											{isReview ? (
												<td>{current} → {proposed}</td>
											) : (
												<>
													<td>{valueOf(website, 'runtime')}</td>
													<td>
														<label className="inline-select">
															<span className="sr-only">PHP version for {root}</span>
															<select
																value={selectedVersion}
																onChange={(event) => stageVersion(website, event.target.value)}
															>
																{recordedMissing ? <option value="">Choose installed PHP</option> : null}
																{installed.map((version) => <option key={version} value={version}>{version}</option>)}
															</select>
														</label>
													</td>
												</>
											)}
											<td><Link to={`/accounts/${ownerId}`}>{website.account_username || ownerId}</Link></td>
											{isReview ? null : (
												<>
													<td><StatusBadge value={Boolean(website.enabled)} /></td>
													<td>
														<div className="row-actions">
															<Link to={`/files?account=${ownerId}`}>Files</Link>
															<Link to={`/ssl?account=${ownerId}`}>SSL</Link>
														</div>
													</td>
												</>
											)}
										</tr>
									)
								})}
							</tbody>
						</table></div>
						<div className="page-actions">
							{isReview ? <button type="button" className="secondary" onClick={() => setStep(0)}>Back</button> : null}
							{pending.length ? (
								<button type="submit" disabled={busy || (isReview && !canWrite)}>
									{busy ? 'Working…' : isReview ? 'Apply' : 'Review'}
								</button>
							) : null}
						</div>
					</form>
				) : null}
				{!loading && !canWrite ? <div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Document root</th><th>Runtime</th><th>Version</th><th>Account</th><th>Enabled</th><th>Actions</th></tr></thead>
					<tbody>
						{rows.map((website) => {
							const ownerId = String(website.account_id || accountId)
							return (
								<tr key={`${ownerId}:${website.id}`}>
									<td><strong>{valueOf(website, 'document_root')}</strong></td>
									<td>{valueOf(website, 'runtime')}</td>
									<td>{valueOf(website, 'runtime_version')}</td>
									<td><Link to={`/accounts/${ownerId}`}>{website.account_username || ownerId}</Link></td>
									<td><StatusBadge value={Boolean(website.enabled)} /></td>
									<td>
										<div className="row-actions">
											<Link to={`/files?account=${ownerId}`}>Files</Link>
											<Link to={`/ssl?account=${ownerId}`}>SSL</Link>
										</div>
									</td>
								</tr>
							)
						})}
					</tbody>
				</table></div> : null}
				{!loading && !rows.length ? <EmptyState title="No websites yet" detail="Provision a domain, then create its website and PHP version here." /> : null}
			</section>
		</>
	)
}
