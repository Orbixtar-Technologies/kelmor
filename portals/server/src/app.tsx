import { useCallback, useEffect, useMemo, useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { api, clearToken, getToken, listOf, post, setToken } from './client'
import { CapProvider, Forbidden } from './rbac'
import { ToastProvider } from './components/toast'
import { Icon } from './components/icons'
import { TopBar, type Alert } from './chrome/top-bar'
import { SideNav } from './chrome/side-nav'
import type { AccountRow } from './components/account-picker'
import { Home } from './pages/home'
import { AccountList } from './pages/account-list'
import { AccountCreate } from './pages/account-create'
import { AccountDetail } from './pages/account-detail'
import {
	AccountModify, AccountChangePackage, AccountSuspension, AccountTerminate,
	AccountPassword, AccountQuotas, AccountSummaryPicker,
} from './pages/account-functions'
import { PackageEditor, PackageList, FeatureManager } from './pages/packages'
import { ResellerCreate, ResellerDetail, ResellerList } from './pages/resellers'
import { DNSSECOverview, DNSZoneDetail, DNSZoneList, DNSAddZone } from './pages/dns'
import { HostFirewall, HostVitals, ProcessManager, ServerReboot, ServiceStatus } from './pages/server'
import { AccountBackups, BackupRestore, TransferExport, TransferImport, TransferMigrate } from './pages/transfers'
import { JobQueue } from './pages/jobs'
import { AuditLog } from './pages/audit'
import { Databases } from './pages/sql'
import { MailOverview } from './pages/email'
import { Certificates } from './pages/ssl'
import { UsageReport } from './pages/usage'

export interface User {
	username: string
	roles: string[]
	email: string
}

interface Me {
	user: User
	actor: { capabilities?: Record<string, boolean> }
}

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [booting, setBooting] = useState(!!getToken())

	useEffect(() => {
		if (!getToken()) return
		api<Me>('/api/v1/me')
			.then(setMe)
			.catch(() => clearToken())
			.finally(() => setBooting(false))
	}, [])

	if (booting) {
		return (
			<main className="auth">
				<div className="auth-card">
					<header>
						<span className="mark" aria-hidden="true">K</span>
						<span className="wordmark">Kelmor <span>Director</span></span>
					</header>
					<div style={{ padding: 26 }}>
						<Icon name="refresh" size={16} className="spin" /> Restoring your session…
					</div>
				</div>
			</main>
		)
	}

	if (!me) return <SignIn onSignedIn={setMe} />
	return <Shell me={me} onSignOut={() => { clearToken(); setMe(null) }} />
}

function SignIn ({ onSignedIn }: { onSignedIn: (me: Me) => void }) {
	const [error, setError] = useState('')
	const [busy, setBusy] = useState(false)

	return (
		<main className="auth">
			<section className="auth-card">
				<header>
					<span className="mark" aria-hidden="true">K</span>
					<div>
						<div className="wordmark">Kelmor <span>Director</span></div>
						<div className="small" style={{ opacity: 0.78 }}>Server administration</div>
					</div>
				</header>
				<form
					onSubmit={async (e) => {
						e.preventDefault()
						setError('')
						setBusy(true)
						const fd = new FormData(e.currentTarget)
						try {
							const r = await post<{ token: string }>('/api/v1/auth/login', {
								username: fd.get('username'),
								password: fd.get('password'),
							})
							setToken(r.token)
							onSignedIn(await api<Me>('/api/v1/me'))
						} catch (err) {
							setError(err instanceof Error ? err.message : 'Sign in failed')
						} finally {
							setBusy(false)
						}
					}}
				>
					<div className="field">
						<label htmlFor="username">Username</label>
						<input id="username" name="username" autoComplete="username" defaultValue="admin" required />
					</div>
					<div className="field">
						<label htmlFor="password">Password</label>
						<input id="password" name="password" type="password" autoComplete="current-password" defaultValue="ChangeMeOnce!2026" required />
					</div>
					{error ? (
						<div className="notice error" role="alert" style={{ marginBottom: 0 }}>
							<Icon name="alertCircle" size={16} className="ico" />
							<div>{error}</div>
						</div>
					) : null}
					<button type="submit" className="btn" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
				</form>
				<footer>
					Provider control plane for hosts, resellers, packages and privileged jobs. Hosting customers sign in to Kelmor Control.
				</footer>
			</section>
		</main>
	)
}

function Shell ({ me, onSignOut }: { me: Me; onSignOut: () => void }) {
	const caps = useMemo(() => me.actor?.capabilities || {}, [me])
	const [railOpen, setRailOpen] = useState(false)
	const [accounts, setAccounts] = useState<AccountRow[]>([])
	const [hostname, setHostname] = useState('')
	const [alerts, setAlerts] = useState<Alert[]>([])

	const refreshContext = useCallback(async () => {
		if (caps['accounts.read']) {
			const rows = await listOf<AccountRow>('/api/v1/accounts').catch(() => [])
			setAccounts(rows)
		}
		if (caps['server.read']) {
			const overview = await api<{ system?: { hostname?: string } }>('/api/v1/server').catch(() => null)
			if (overview?.system?.hostname) setHostname(overview.system.hostname)
		}
	}, [caps])

	useEffect(() => {
		refreshContext()
	}, [refreshContext])

	useEffect(() => {
		let cancelled = false
		async function collect () {
			const next: Alert[] = []
			if (caps['server.read'] || caps['accounts.read']) {
				const jobs = await listOf<{ id: string; type: string; state: string; last_error?: string }>('/api/v1/jobs').catch(() => [])
				const failed = jobs.filter((j) => j.state === 'failed' || j.state === 'dead')
				for (const job of failed.slice(0, 5)) {
					next.push({
						id: `job-${job.id}`,
						tone: 'bad',
						title: 'Job failed',
						detail: `${job.type} — ${job.last_error || 'see job detail'}`,
						to: '/jobs',
					})
				}
			}
			if (caps['accounts.read']) {
				const suspended = accounts.filter((a) => a.status === 'suspended')
				if (suspended.length > 0) {
					next.push({
						id: 'suspended',
						tone: 'warn',
						title: `${suspended.length} suspended`,
						detail: 'Accounts are currently held out of service.',
						to: '/accounts/suspended',
					})
				}
			}
			if (!cancelled) setAlerts(next)
		}
		collect()
		const timer = window.setInterval(collect, 20000)
		return () => {
			cancelled = true
			window.clearInterval(timer)
		}
	}, [caps, accounts])

	function guard (cap: string, title: string, element: React.ReactNode) {
		return caps[cap] ? element : <Forbidden title={title} cap={cap} />
	}

	return (
		<CapProvider caps={caps}>
			<ToastProvider>
				<TopBar
					username={me.user.username}
					roles={me.user.roles || []}
					hostname={hostname}
					caps={caps}
					accounts={accounts}
					alerts={alerts}
					onSignOut={onSignOut}
					onToggleRail={() => setRailOpen((open) => !open)}
				/>
				<div className="shell">
					<SideNav caps={caps} open={railOpen} />
					<main className="content">
						<Routes>
							<Route path="/" element={<Home caps={caps} />} />

							<Route path="/accounts" element={guard('accounts.read', 'List Accounts', <AccountList variant="all" />)} />
							<Route path="/accounts/suspended" element={guard('accounts.read', 'List Suspended Accounts', <AccountList variant="suspended" />)} />
							<Route path="/accounts/over-quota" element={guard('accounts.read', 'Show Accounts Over Quota', <AccountList variant="over-quota" />)} />
							<Route path="/accounts/summary" element={guard('accounts.read', 'Account Summary', <AccountSummaryPicker />)} />
							<Route path="/accounts/create" element={guard('accounts.create', 'Create a New Account', <AccountCreate />)} />
							<Route path="/accounts/modify" element={guard('accounts.modify', 'Modify an Account', <AccountModify />)} />
							<Route path="/accounts/change-package" element={guard('accounts.modify', 'Upgrade / Downgrade an Account', <AccountChangePackage />)} />
							<Route path="/accounts/suspension" element={guard('accounts.suspend', 'Manage Account Suspension', <AccountSuspension />)} />
							<Route path="/accounts/terminate" element={guard('accounts.terminate', 'Terminate Accounts', <AccountTerminate />)} />
							<Route path="/accounts/password" element={guard('accounts.modify', 'Force Password Change', <AccountPassword />)} />
							<Route path="/accounts/quotas" element={guard('accounts.modify', 'Limit Bandwidth and Disk', <AccountQuotas />)} />
							<Route path="/accounts/:accountId" element={guard('accounts.read', 'Account Summary', <AccountDetail />)} />

							<Route path="/packages" element={guard('packages.read', 'Packages', <PackageList />)} />
							<Route path="/packages/new" element={guard('packages.write', 'Add a Package', <PackageEditor mode="create" />)} />
							<Route path="/packages/features" element={guard('packages.read', 'Feature Manager', <FeatureManager />)} />
							<Route path="/packages/:packageId" element={guard('packages.read', 'Edit Package', <PackageEditor mode="edit" />)} />

							<Route path="/resellers" element={guard('resellers.read', 'Resellers', <ResellerList />)} />
							<Route path="/resellers/new" element={guard('resellers.create', 'Add a Reseller', <ResellerCreate />)} />
							<Route path="/resellers/:resellerId" element={guard('resellers.read', 'Reseller', <ResellerDetail />)} />

							<Route path="/dns/zones" element={guard('dns.read', 'DNS Zone Manager', <DNSZoneList />)} />
							<Route path="/dns/add-zone" element={guard('dns.read', 'Add a DNS Zone', <DNSAddZone />)} />
							<Route path="/dns/dnssec" element={guard('dns.read', 'DNSSEC', <DNSSECOverview />)} />
							<Route path="/dns/zones/:accountId/:zoneId" element={guard('dns.read', 'DNS Zone', <DNSZoneDetail />)} />

							<Route path="/sql" element={guard('databases.read', 'Databases', <Databases />)} />
							<Route path="/email" element={guard('mail.read', 'Mail Domains and Mailboxes', <MailOverview />)} />
							<Route path="/ssl" element={guard('accounts.read', 'Certificates', <Certificates />)} />
							<Route path="/usage" element={guard('billing.usage.read', 'Bandwidth and Disk Usage', <UsageReport />)} />

							<Route path="/server/status" element={guard('server.read', 'Service Status', <ServiceStatus />)} />
							<Route path="/server/vitals" element={guard('server.read', 'Host Vitals', <HostVitals />)} />
							<Route path="/server/processes" element={guard('server.read', 'Process Manager', <ProcessManager />)} />
							<Route path="/server/reboot" element={guard('server.settings.write', 'Graceful Server Reboot', <ServerReboot />)} />

							<Route path="/security/firewall" element={guard('server.firewall.read', 'Host Firewall', <HostFirewall />)} />
							<Route path="/security/audit" element={guard('security.audit.read', 'Audit Log', <AuditLog />)} />

							<Route path="/transfers/backups" element={guard('backups.read', 'Account Backups', <AccountBackups />)} />
							<Route path="/transfers/restore" element={guard('backups.restore', 'Restore a Backup', <BackupRestore />)} />
							<Route path="/transfers/migrate" element={guard('accounts.create', 'Transfer or Migrate an Account', <TransferMigrate />)} />
							<Route path="/transfers/import" element={guard('accounts.create', 'Import an Account', <TransferImport />)} />
							<Route path="/transfers/export" element={guard('accounts.read', 'Export Accounts', <TransferExport />)} />

							<Route path="/jobs" element={guard('server.read', 'Job Queue', <JobQueue />)} />
							<Route path="/jobs/:jobId" element={guard('server.read', 'Job Queue', <JobQueue />)} />

							<Route path="*" element={<NotFound />} />
						</Routes>
					</main>
				</div>
			</ToastProvider>
		</CapProvider>
	)
}

function NotFound () {
	return (
		<>
			<div className="crumbs"><span>Not found</span></div>
			<div className="page-head"><div><h1>That tool does not exist</h1><p>Use the find box at the top or pick a category from the left.</p></div></div>
			<section className="panel">
				<div className="empty">
					<Icon name="compass" size={30} className="ico" />
					<h3>No Kelmor Director tool is registered at this address</h3>
					<p>The address may be from an older bookmark. Every tool is listed under a category in the left navigation.</p>
					<a className="btn" href="/">Back to Home</a>
				</div>
			</section>
		</>
	)
}
