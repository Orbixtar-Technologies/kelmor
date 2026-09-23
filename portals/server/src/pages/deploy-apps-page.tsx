import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { messageFrom, valueOf } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

// Wizard step IDs
type Step = 'account' | 'domain' | 'source' | 'repo' | 'detect' | 'done'

interface WizardState {
	accountId: string
	domainId: string
	runtime: string
	source: 'repo' | 'empty'
	gitUrl: string
	gitBranch: string
	gitAuth: string
	autoDeploy: boolean
	startCommand: string
	workingDirectory: string
}

const defaultWizard: WizardState = {
	accountId: '',
	domainId: '',
	runtime: 'node',
	source: 'repo',
	gitUrl: '',
	gitBranch: 'main',
	gitAuth: '',
	autoDeploy: false,
	startCommand: '',
	workingDirectory: '',
}

export function DeployAppsPage () {
	const [params, setParams] = useSearchParams()
	const accountId = params.get('account') || ''
	const [step, setStep] = useState<Step>('account')
	const [wizard, setWizard] = useState<WizardState>({ ...defaultWizard, accountId })
	const [allAccounts, setAllAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [account, setAccount] = useState<Account | null>(null)
	const [domains, setDomains] = useState<ResourceItem[]>([])
	const [apps, setApps] = useState<ResourceItem[]>([])
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [lastJobId, setLastJobId] = useState('')
	const [webhookUrl, setWebhookUrl] = useState('')
	const [github, setGithub] = useState<{ app_configured?: boolean; connected?: boolean; login?: string; client_id?: string }>({})
	const [githubRepos, setGithubRepos] = useState<Array<{ full_name: string; clone_url: string; default_branch: string; private?: boolean }>>([])
	const [githubBusy, setGithubBusy] = useState(false)
	const [githubAppId, setGithubAppId] = useState('')
	const [githubAppSecret, setGithubAppSecret] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('applications.write')
	const canRead = useCan('applications.read')

	useEffect(() => {
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			setAllAccounts(asList(result))
		}).catch(() => {})
		refreshGithub()
	}, [])

	async function refreshGithub () {
		try {
			const status = await api<{ app_configured?: boolean; connected?: boolean; login?: string; client_id?: string }>('/api/v1/integrations/github/status')
			setGithub(status)
			if (status.connected) {
				const repos = await api<{ items: Array<{ full_name: string; clone_url: string; default_branch: string; private?: boolean }> }>('/api/v1/integrations/github/repos')
				setGithubRepos(repos.items || [])
			} else {
				setGithubRepos([])
			}
		} catch {
			setGithub({})
		}
	}

	async function connectGithub () {
		setGithubBusy(true)
		setError('')
		try {
			const result = await api<{ authorize_url: string }>('/api/v1/integrations/github/authorize')
			if (result.authorize_url) window.location.href = result.authorize_url
		} catch (err) {
			setError(messageFrom(err))
		} finally {
			setGithubBusy(false)
		}
	}

	async function disconnectGithub () {
		setGithubBusy(true)
		try {
			await api('/api/v1/integrations/github/connection', { method: 'DELETE' })
			await refreshGithub()
		} catch (err) {
			setError(messageFrom(err))
		} finally {
			setGithubBusy(false)
		}
	}

	async function saveGithubApp (e: React.FormEvent) {
		e.preventDefault()
		setGithubBusy(true)
		setError('')
		try {
			await api('/api/v1/integrations/github/app', { method: 'PUT', body: JSON.stringify({ client_id: githubAppId, client_secret: githubAppSecret }) })
			setGithubAppSecret('')
			setMessage('GitHub OAuth app saved. Users can now connect GitHub.')
			await refreshGithub()
		} catch (err) {
			setError(messageFrom(err))
		} finally {
			setGithubBusy(false)
		}
	}

	const loadAccount = useCallback((aid: string) => {
		if (!aid) { setAccount(null); setDomains([]); setApps([]); return }
		const token = requests.begin('load')
		setLoading(true)
		setError('')
		Promise.all([
			api<Account>(`/api/v1/accounts/${aid}`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${aid}/domains`),
			api<{ items: ResourceItem[] }>(`/api/v1/accounts/${aid}/applications`),
		]).then(([acc, domResult, appResult]) => {
			if (!requests.isCurrent(token)) return
			setAccount(acc)
			setDomains(asList(domResult))
			setApps(asList(appResult))
		}).catch((err) => {
			if (!requests.isCurrent(token)) return
			setError(messageFrom(err))
		}).finally(() => {
			if (requests.isCurrent(token)) setLoading(false)
		})
	}, [requests])

	useEffect(() => { if (accountId) loadAccount(accountId) }, [accountId, loadAccount])

	function selectAccount (aid: string) {
		setParams({ account: aid }, { replace: true })
		setWizard({ ...defaultWizard, accountId: aid })
		setStep('domain')
		setMessage('')
		setLastJobId('')
		setWebhookUrl('')
	}

	function set (patch: Partial<WizardState>) {
		setWizard((w) => ({ ...w, ...patch }))
	}

	async function deploy () {
		if (!wizard.accountId) return
		setError('')
		setMessage('Queuing deployment…')
		const body: Record<string, unknown> = {
			domain_id: wizard.domainId,
			runtime: wizard.runtime,
			auto_deploy: wizard.autoDeploy,
		}
		if (wizard.source === 'repo' && wizard.gitUrl) {
			body.git_url = wizard.gitUrl
			body.git_branch = wizard.gitBranch || 'main'
			if (wizard.gitAuth) body.git_auth = wizard.gitAuth
		}
		if (wizard.startCommand) body.start_command = wizard.startCommand
		if (wizard.workingDirectory) body.working_directory = wizard.workingDirectory
		try {
			const result = await api<{ operation_id?: string; application?: { webhook_url?: string } }>(
				`/api/v1/accounts/${wizard.accountId}/applications`,
				{ method: 'POST', body: JSON.stringify(body) },
			)
			setLastJobId(result.operation_id || '')
			setWebhookUrl(result.application?.webhook_url || '')
			setMessage('Deployment queued.')
			setStep('done')
			loadAccount(wizard.accountId)
		} catch (err) {
			setMessage('')
			setError(messageFrom(err))
		}
	}

	async function redeploy (appId: string) {
		if (!accountId) return
		setError('')
		try {
			const result = await api<{ operation_id?: string }>(
				`/api/v1/accounts/${accountId}/applications/${appId}/redeploy`,
				{ method: 'POST', body: '{}' },
			)
			setMessage(`Redeploy queued${result.operation_id ? ' · ' + result.operation_id : ''}.`)
			loadAccount(accountId)
		} catch (err) {
			setError(messageFrom(err))
		}
	}

	async function deleteApp (appId: string) {
		if (!accountId) return
		if (!window.confirm('Delete this application? The systemd service will be removed.')) return
		setError('')
		try {
			await api(`/api/v1/accounts/${accountId}/applications/${appId}`, { method: 'DELETE' })
			setMessage('Application deletion queued.')
			loadAccount(accountId)
		} catch (err) {
			setError(messageFrom(err))
		}
	}

	const selectedDomain = domains.find((d) => d.id === wizard.domainId)

	return (
		<>
			<PageHeader
				title="Deploy Apps"
				description="Connect a repository or upload code to deploy Node.js, Python, Go, and Ruby applications."
			/>

			{step !== 'done' && (
				<StepBar
					current={step}
					items={[
						{ id: 'account', label: 'Account' },
						{ id: 'domain', label: 'Domain' },
						{ id: 'source', label: 'Source' },
						{ id: 'repo', label: 'Repository' },
						{ id: 'detect', label: 'Detect' },
					]}
				/>
			)}

			{error ? <p className="feedback feedback--error" role="alert">{error}</p> : null}
			{message && step === 'done' ? <p className="feedback" role="status">{message}</p> : null}

			{/* ── Step: account ── */}
			{step === 'account' && (
				<section className="panel">
					<div className="section-heading"><div><h2>Select an account</h2><p>Choose the hosting account this application will run under.</p></div></div>
					<div className="inline-form">
						<label>Filter
							<input
								type="search"
								placeholder="username or domain"
								value={accountFilter}
								onChange={(e) => setAccountFilter(e.target.value)}
							/>
						</label>
					</div>
					<div className="table-wrap">
						<table className="dense-table">
							<thead><tr><th scope="col">Username</th><th scope="col">Domain</th><th scope="col"></th></tr></thead>
							<tbody>
								{allAccounts
									.filter((a) => {
										const q = accountFilter.toLowerCase()
										return !q || a.username.toLowerCase().includes(q) || a.primary_domain.toLowerCase().includes(q)
									})
									.slice(0, 50)
									.map((a) => (
										<tr key={a.id}>
											<td>{a.username}</td>
											<td>{a.primary_domain}</td>
											<td><button type="button" className="link-button" onClick={() => selectAccount(a.id)}>Select</button></td>
										</tr>
									))}
							</tbody>
						</table>
					</div>
				</section>
			)}

			{/* ── Step: domain ── */}
			{step === 'domain' && (
				<section className="panel">
					<div className="section-heading"><div><h2>Choose a domain</h2><p>A site will be created (or updated) for the chosen domain. The application will be reachable at that hostname.</p></div></div>
					{loading ? <LoadingState /> : (
						<form className="stack-form" onSubmit={(e) => { e.preventDefault(); setStep('source') }}>
							<label>Domain
								<select required value={wizard.domainId} onChange={(e) => set({ domainId: e.target.value })}>
									<option value="">— select —</option>
									{domains.map((d) => (
										<option key={d.id} value={d.id}>{String(d.ascii_fqdn || d.fqdn || d.id)}</option>
									))}
								</select>
							</label>
							<label>Runtime
								<select value={wizard.runtime} onChange={(e) => set({ runtime: e.target.value })}>
									<option value="node">Node.js</option>
									<option value="python">Python</option>
								</select>
							</label>
							<div className="row-actions">
								<button type="button" className="secondary" onClick={() => setStep('account')}>Back</button>
								<button type="submit" disabled={!wizard.domainId}>Next</button>
							</div>
						</form>
					)}
				</section>
			)}

			{/* ── Step: source ── */}
			{step === 'source' && (
				<section className="panel">
					<div className="section-heading"><div><h2>Choose a source</h2><p>Connect a Git repository or start with an empty working directory.</p></div></div>
					<div className="tool-launch-grid" style={{ marginBottom: '16px' }}>
						<button
							type="button"
							className={'tool-launch-card' + (wizard.source === 'repo' ? ' selected' : '')}
							onClick={() => { set({ source: 'repo' }); setStep('repo') }}
						>
							<strong>Connect repository</strong>
							<span>Clone from GitHub, GitLab, Gitea, or any HTTPS / SSH URL</span>
						</button>
						<button
							type="button"
							className={'tool-launch-card' + (wizard.source === 'empty' ? ' selected' : '')}
							onClick={() => { set({ source: 'empty' }); setStep('detect') }}
						>
							<strong>Upload / empty directory</strong>
							<span>Upload code via SFTP or File Manager, then deploy</span>
						</button>
					</div>
					<div className="row-actions">
						<button type="button" className="secondary" onClick={() => setStep('domain')}>Back</button>
					</div>
				</section>
			)}

			{/* ── Step: repo ── */}
			{step === 'repo' && (
				<section className="panel">
					<div className="section-heading"><div><h2>Repository</h2><p>The repository will be cloned into the application working directory at deploy time.</p></div></div>
					<form className="stack-form" onSubmit={(e) => { e.preventDefault(); setStep('detect') }}>
						<section className="panel" style={{ marginBottom: '16px' }}>
							<div className="section-heading"><div><h3>GitHub OAuth</h3><p>Connect GitHub and pick a repository instead of pasting a URL.</p></div></div>
							{github.app_configured ? (
								<div className="row-actions">
									{github.connected ? (
										<>
											<p className="subtle">Connected as <strong>{github.login || 'GitHub user'}</strong></p>
											<button type="button" className="secondary" disabled={githubBusy} onClick={disconnectGithub}>Disconnect</button>
										</>
									) : (
										<button type="button" disabled={githubBusy} onClick={connectGithub}>Connect GitHub</button>
									)}
								</div>
							) : (
								<div>
									<p className="subtle">Configure a GitHub OAuth App (callback <code>/api/v1/integrations/github/callback</code>) to enable one-click connect. URL paste still works.</p>
									<div className="stack-form" onSubmit={saveGithubApp as never}>
										<label>Client ID
											<input value={githubAppId} onChange={(e) => setGithubAppId(e.target.value)} placeholder="Iv1... or client id" />
										</label>
										<label>Client secret
											<input type="password" value={githubAppSecret} onChange={(e) => setGithubAppSecret(e.target.value)} autoComplete="off" />
										</label>
										<button type="button" disabled={githubBusy || !githubAppId || !githubAppSecret} onClick={(e) => saveGithubApp(e as unknown as React.FormEvent)}>Save OAuth app</button>
									</div>
								</div>
							)}
							{github.connected && githubRepos.length ? (
								<label>Connected repositories
									<select
										value={wizard.gitUrl}
										onChange={(e) => {
											const repo = githubRepos.find((item) => item.clone_url === e.target.value)
											set({ gitUrl: e.target.value, gitBranch: repo?.default_branch || wizard.gitBranch || 'main' })
										}}
									>
										<option value="">— pick a repo —</option>
										{githubRepos.map((repo) => (
											<option key={repo.full_name} value={repo.clone_url}>{repo.full_name}{repo.private ? ' (private)' : ''}</option>
										))}
									</select>
								</label>
							) : null}
						</section>
						<label>Git URL <small>(HTTPS or SSH)</small>
							<input
								type="url"
								placeholder="https://github.com/user/repo.git"
								value={wizard.gitUrl}
								onChange={(e) => set({ gitUrl: e.target.value })}
								required
							/>
						</label>
						<label>Branch
							<input
								type="text"
								placeholder="main"
								value={wizard.gitBranch}
								onChange={(e) => set({ gitBranch: e.target.value })}
							/>
						</label>
						<details>
							<summary style={{ cursor: 'pointer', color: 'var(--muted)', fontSize: '13px', marginBottom: '6px' }}>Authentication (optional)</summary>
							<label style={{ marginTop: '8px' }}>
								Deploy token or personal access token
								<input
									type="password"
									placeholder="ghp_xxxx or deploy token"
									value={wizard.gitAuth}
									onChange={(e) => set({ gitAuth: e.target.value })}
									autoComplete="off"
								/>
							</label>
							<p className="subtle" style={{ marginTop: '4px' }}>
								Token is sent to the host agent and never stored in logs.
								For SSH URLs leave this blank and add the deploy key to your repository.
							</p>
						</details>
						<label className="checkbox-label">
							<input
								type="checkbox"
								checked={wizard.autoDeploy}
								onChange={(e) => set({ autoDeploy: e.target.checked })}
							/>
							Enable auto-deploy (webhook)
						</label>
						{wizard.autoDeploy && (
							<p className="subtle">
								A signed webhook URL will be generated after deployment. Push events to that URL to re-clone and re-deploy automatically.
							</p>
						)}
						<div className="row-actions">
							<button type="button" className="secondary" onClick={() => setStep('source')}>Back</button>
							<button type="submit" disabled={!wizard.gitUrl}>Next</button>
						</div>
					</form>
				</section>
			)}

			{/* ── Step: detect ── */}
			{step === 'detect' && (
				<section className="panel">
					<div className="section-heading">
						<div>
							<h2>Detect &amp; deploy</h2>
							<p>
								The agent will auto-detect your runtime from{' '}
								<code>package.json</code>, <code>requirements.txt</code>, <code>go.mod</code>, or <code>Gemfile</code>.
								Override the start command if needed.
							</p>
						</div>
					</div>
					<div className="app-detect-summary">
						<dl className="detail-list">
							<div><dt>Account</dt><dd>{account?.username || wizard.accountId}</dd></div>
							<div><dt>Domain</dt><dd>{selectedDomain ? String(selectedDomain.ascii_fqdn || selectedDomain.id) : wizard.domainId}</dd></div>
							<div><dt>Runtime</dt><dd>{wizard.runtime === 'node' ? 'Node.js' : 'Python'}</dd></div>
							{wizard.source === 'repo' && wizard.gitUrl ? (
								<>
									<div><dt>Repository</dt><dd style={{ wordBreak: 'break-all' }}>{wizard.gitUrl}</dd></div>
									<div><dt>Branch</dt><dd>{wizard.gitBranch || 'main'}</dd></div>
									{wizard.autoDeploy && <div><dt>Auto-deploy</dt><dd>Webhook will be enabled</dd></div>}
								</>
							) : (
								<div><dt>Source</dt><dd>Working directory (upload code via SFTP)</dd></div>
							)}
						</dl>
					</div>
					<form className="stack-form" onSubmit={(e) => { e.preventDefault(); deploy() }}>
						<details>
							<summary style={{ cursor: 'pointer', color: 'var(--muted)', fontSize: '13px', marginBottom: '6px' }}>Override detection (advanced)</summary>
							<label style={{ marginTop: '8px' }}>
								Start command <small>(leave blank for auto-detect)</small>
								<input
									type="text"
									placeholder="e.g. /usr/bin/node dist/server.js"
									value={wizard.startCommand}
									onChange={(e) => set({ startCommand: e.target.value })}
								/>
							</label>
							<label>
								Working directory <small>(leave blank to use document root)</small>
								<input
									type="text"
									placeholder="e.g. /home/user/apps/myapp"
									value={wizard.workingDirectory}
									onChange={(e) => set({ workingDirectory: e.target.value })}
								/>
							</label>
						</details>
						<ul className="app-deploy-hints">
							<li>App must listen on the path in the <code>SOCKET_PATH</code> environment variable</li>
							<li>Detects Node servers, Vite/SPA build scripts, Python, Go, and Ruby</li>
							<li>First deploy installs dependencies, builds static apps, and starts the process on SOCKET_PATH</li>
							<li>Check Jobs for deployment status after queueing</li>
						</ul>
						{error ? <p className="feedback feedback--error" role="alert">{error}</p> : null}
						<div className="row-actions">
							<button type="button" className="secondary" onClick={() => setStep(wizard.source === 'repo' ? 'repo' : 'source')}>Back</button>
							{canWrite ? (
								<button type="submit">Deploy</button>
							) : (
								<p className="subtle">You need <code>applications.write</code> to deploy.</p>
							)}
						</div>
					</form>
				</section>
			)}

			{/* ── Step: done ── */}
			{step === 'done' && (
				<section className="panel">
					<div className="section-heading"><div><h2>Deployment queued</h2></div></div>
					<p>The deployment job was queued. Track progress in <Link to={`/jobs?account=${wizard.accountId}`}>Jobs</Link>.</p>
					{webhookUrl && (
						<div style={{ margin: '12px 0' }}>
							<p><strong>Webhook URL</strong></p>
							<code style={{ wordBreak: 'break-all', fontSize: '13px' }}>{window.location.origin}{webhookUrl}</code>
							<p className="subtle" style={{ marginTop: '4px' }}>
								Send a POST to this URL from your CI/CD pipeline to trigger automatic redeploys.
								Treat it as a secret.
							</p>
						</div>
					)}
					<div className="row-actions">
						<button type="button" onClick={() => { setStep('account'); setWizard({ ...defaultWizard }) }}>Deploy another app</button>
						{lastJobId && <Link className="button-link secondary-link" to={`/jobs?account=${wizard.accountId}`}>Open Jobs</Link>}
					</div>
				</section>
			)}

			{/* ── App list ── */}
			{accountId && canRead && step !== 'detect' && step !== 'repo' && step !== 'source' && step !== 'done' ? (
				<section className="panel" style={{ marginTop: '24px' }}>
					<div className="section-heading"><div><h2>Deployed apps · {apps.length}</h2></div></div>
					{loading ? <LoadingState /> : null}
					{!loading && !apps.length ? (
						<p className="subtle">No applications deployed on this account yet.</p>
					) : null}
					{!loading && apps.length ? (
						<div className="table-wrap">
							<table className="dense-table">
								<thead>
									<tr>
										<th scope="col">ID</th>
										<th scope="col">Runtime</th>
										<th scope="col">Status</th>
										<th scope="col">Repository</th>
										<th scope="col">Auto-deploy</th>
										<th scope="col">Actions</th>
									</tr>
								</thead>
								<tbody>
									{apps.map((app) => (
										<tr key={app.id}>
											<td><code style={{ fontSize: '11px' }}>{String(app.id).slice(0, 8)}</code></td>
											<td>{valueOf(app, 'runtime')}</td>
											<td><StatusBadge value={valueOf(app, 'status')} /></td>
											<td>
												{app.git_url ? (
													<span style={{ fontSize: '12px', wordBreak: 'break-all' }}>{String(app.git_url)} · {String(app.git_branch || 'main')}</span>
												) : (
													<span className="subtle">—</span>
												)}
											</td>
											<td>{app.auto_deploy ? 'On' : 'Off'}</td>
											<td>
												<div className="row-actions">
													{canWrite ? (
														<>
															<button type="button" className="link-button" onClick={() => redeploy(String(app.id))}>Redeploy</button>
															<button type="button" className="link-button danger-text" onClick={() => deleteApp(String(app.id))}>Delete</button>
														</>
													) : null}
													<Link className="link-button" to={`/jobs?account=${accountId}`}>Jobs</Link>
												</div>
											</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					) : null}
				</section>
			) : null}
		</>
	)
}

// Small step indicator at the top of the wizard.
function StepBar ({ current, items }: { current: Step; items: { id: Step; label: string }[] }) {
	const activeIndex = items.findIndex((i) => i.id === current)
	return (
		<ol className="steps" aria-label="Wizard progress">
			{items.map((item, index) => (
				<li
					key={item.id}
					aria-current={item.id === current ? 'step' : undefined}
					style={{ opacity: index > activeIndex ? 0.4 : 1 }}
				>
					<span>{index + 1}</span>{item.label}
				</li>
			))}
		</ol>
	)
}
