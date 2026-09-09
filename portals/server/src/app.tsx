import { useEffect, useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AccountDetail } from './account-detail'
import { AccountList, CreateAccount } from './accounts'
import { api, clearToken, getToken, setToken } from './client'
import { Dashboard } from './dashboard'
import { DirectorNav } from './nav'
import { Audit, ImportAccount, Jobs, Monitor, Packages, Resellers } from './pages'
import { CapProvider, Forbidden } from './rbac'

interface User { username: string; roles: string[]; email: string }
interface Me {
	user: User
	actor: { capabilities?: Record<string, boolean> }
}

export function App () {
	const [me, setMe] = useState<Me | null>(null)
	const [error, setError] = useState('')

	useEffect(() => {
		if (!getToken()) return
		api<Me>('/api/v1/me').then(setMe).catch(() => clearToken())
	}, [])

	if (!me) {
		return (
			<main className="auth">
				<section className="card">
					<p className="eyebrow">Kelmor</p>
					<h1>Kelmor Director</h1>
					<p className="lede">
						Provider control plane for the host, resellers, packages and
						privileged jobs. Tenants use Kelmor Control.
					</p>
					<form onSubmit={async (e) => {
						e.preventDefault()
						setError('')
						const fd = new FormData(e.currentTarget)
						try {
							const r = await api<{ token: string; user: User }>('/api/v1/auth/login', {
								method: 'POST',
								body: JSON.stringify({
									username: fd.get('username'),
									password: fd.get('password'),
								}),
							})
							setToken(r.token)
							setMe(await api<Me>('/api/v1/me'))
						} catch (err) {
							setError(err instanceof Error ? err.message : 'Login failed')
						}
					}}>
						<label>Username<input name="username" autoComplete="username" defaultValue="admin" /></label>
						<label>Password<input name="password" type="password" autoComplete="current-password" defaultValue="ChangeMeOnce!2026" /></label>
						{error ? <p className="error" role="alert">{error}</p> : null}
						<button type="submit">Sign in</button>
					</form>
				</section>
			</main>
		)
	}

	const caps = me.actor?.capabilities || {}
	return (
		<CapProvider caps={caps}>
			<div className="shell">
				<DirectorNav
					caps={caps}
					username={me.user.username}
					onSignOut={() => { clearToken(); setMe(null) }}
				/>
				<main className="content">
					<Routes>
						<Route path="/" element={caps['server.read'] ? <Dashboard /> : <Forbidden title="Dashboard" />} />
						<Route path="/accounts" element={caps['accounts.read'] ? <AccountList /> : <Forbidden title="List Accounts" />} />
						<Route path="/accounts/create" element={caps['accounts.create'] ? <CreateAccount /> : <Forbidden title="Create Account" />} />
						<Route path="/accounts/:id" element={caps['accounts.read'] ? <AccountDetail /> : <Forbidden title="Account" />} />
						<Route path="/import" element={caps['accounts.create'] ? <ImportAccount /> : <Forbidden title="Import" />} />
						<Route path="/resellers" element={caps['resellers.read'] ? <Resellers /> : <Forbidden title="Resellers" />} />
						<Route path="/packages" element={caps['packages.read'] ? <Packages /> : <Forbidden title="Packages" />} />
						<Route path="/monitor" element={caps['billing.usage.read'] ? <Monitor /> : <Forbidden title="Usage" />} />
						<Route path="/jobs" element={(caps['server.read'] || caps['accounts.read']) ? <Jobs /> : <Forbidden title="Jobs" />} />
						<Route path="/audit" element={caps['security.audit.read'] ? <Audit /> : <Forbidden title="Audit" />} />
					</Routes>
				</main>
			</div>
		</CapProvider>
	)
}
