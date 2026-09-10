import { useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { api, clearToken, getToken, setToken } from './client'
import { DirectorShell } from './layout/director-shell'
import { AccountServicesPage } from './pages/account-services-page'
import { AccountSummaryPage } from './pages/account-summary-page'
import { AccountsPage } from './pages/accounts-page'
import { AuditPage } from './pages/audit-page'
import { CreateAccountPage } from './pages/create-account-page'
import { DNSPage } from './pages/dns-page'
import { HomePage } from './pages/home-page'
import { JobsPage } from './pages/jobs-page'
import { PackagesPage } from './pages/packages-page'
import { ResellersPage } from './pages/resellers-page'
import { SecurityPage } from './pages/security-page'
import { ServiceStatusPage } from './pages/service-status-page'
import { TransfersPage } from './pages/transfers-page'
import { UpdatesPage } from './pages/updates-page'
import { UsagePage } from './pages/usage-page'
import { CapProvider, Forbidden, hasCapabilities } from './rbac'
import type { ReactNode } from 'react'
import type { Me, User } from './types'

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [checking, setChecking] = useState(Boolean(getToken()))
	useEffect(() => {
		if (!getToken()) return
		api<Me>('/api/v1/me').then(setMe).catch(() => clearToken()).finally(() => setChecking(false))
	}, [])
	if (checking) return <main className="auth"><div className="loading-state" role="status"><span />Restoring Kelmor Director session…</div></main>
	if (!me) return <LoginPage onLogin={setMe} />
	const capabilities = me.actor.capabilities || {}
	function allowed (requiredCapabilities: string | readonly string[], element: ReactNode) {
		const required = typeof requiredCapabilities === 'string' ? [requiredCapabilities] : requiredCapabilities
		return hasCapabilities(capabilities, required) ? element : <Forbidden title="Access restricted" />
	}
	return (
		<CapProvider caps={capabilities}>
			<Routes>
				<Route element={<DirectorShell me={me} onSignOut={() => { api('/api/v1/auth/logout', { method: 'POST', body: '{}' }).catch(() => undefined); clearToken(); setMe(null) }} />}>
					<Route index element={(capabilities['server.read'] || capabilities['accounts.read']) ? <HomePage /> : <Forbidden title="Home" />} />
					<Route path="accounts" element={allowed('accounts.read', <AccountsPage />)} />
					<Route path="accounts/create" element={allowed(['accounts.create', 'packages.read'], <CreateAccountPage />)} />
					<Route path="accounts/:id" element={allowed('accounts.read', <AccountSummaryPage />)} />
					<Route path="accounts/:id/services" element={allowed('accounts.read', <AccountServicesPage />)} />
					<Route path="packages" element={allowed('packages.read', <PackagesPage />)} />
					<Route path="resellers" element={allowed('resellers.read', <ResellersPage />)} />
					<Route path="dns" element={allowed('dns.read', <DNSPage />)} />
					<Route path="status" element={allowed('server.read', <ServiceStatusPage />)} />
					<Route path="security" element={allowed('server.read', <SecurityPage />)} />
					<Route path="transfers" element={allowed('accounts.read', <TransfersPage />)} />
					<Route path="import" element={allowed('accounts.read', <TransfersPage />)} />
					<Route path="jobs" element={(capabilities['server.read'] || capabilities['accounts.read']) ? <JobsPage /> : <Forbidden title="Jobs" />} />
					<Route path="updates" element={allowed('server.read', <UpdatesPage />)} />
					<Route path="audit" element={allowed('security.audit.read', <AuditPage />)} />
					<Route path="usage" element={allowed(['billing.usage.read', 'accounts.read', 'packages.read'], <UsagePage />)} />
					<Route path="monitor" element={allowed(['billing.usage.read', 'accounts.read', 'packages.read'], <UsagePage />)} />
					<Route path="*" element={<Navigate to="/" replace />} />
				</Route>
			</Routes>
		</CapProvider>
	)
}

interface LoginPageProps {
	onLogin: (me: Me) => void
}

function LoginPage ({ onLogin }: LoginPageProps) {
	const [error, setError] = useState('')
	return <main className="auth"><section className="login-card"><div className="login-brand"><span>K</span><div><strong>Kelmor Director</strong><small>Server administration</small></div></div><h1>Sign in</h1><p>Access your Kelmor host, accounts, services, and background operations.</p><form onSubmit={async (event) => {
		event.preventDefault(); setError('')
		const data = new FormData(event.currentTarget)
		try {
			const result = await api<{ token: string; user: User }>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username: data.get('username'), password: data.get('password') }) })
			setToken(result.token); onLogin(await api<Me>('/api/v1/me'))
		} catch (requestError) { setError(requestError instanceof Error ? requestError.message : 'Sign in failed.') }
	}}><label>Username<input name="username" autoComplete="username" required autoFocus /></label><label>Password<input name="password" type="password" autoComplete="current-password" required /></label>{error ? <p className="field-error" role="alert">{error}</p> : null}<button type="submit">Sign in to Kelmor Director</button></form></section></main>
}
