import { useEffect, useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { api, asList } from '../client'
import { useCapabilities } from '../rbac'
import { discoverTools, toolCatalog } from '../tool-catalog'
import { GlobalFind } from './global-find'
import { Sidebar } from './sidebar'
import type { Account, Me, ServerOverview } from '../types'

interface DirectorShellProps {
	me: Me
	onSignOut: () => void
}

const crumbLabels: Record<string, string> = {
	accounts: 'Accounts', create: 'Create Account', services: 'Account Services',
	packages: 'Packages', resellers: 'Resellers', dns: 'DNS Management',
	status: 'Service Status', security: 'Security', transfers: 'Transfers & Backups',
	jobs: 'Jobs', audit: 'Audit Trail', usage: 'Account Usage',
}

export function DirectorShell ({ me, onSignOut }: DirectorShellProps) {
	const capabilities = useCapabilities()
	const [collapsed, setCollapsed] = useState(false)
	const [mobileOpen, setMobileOpen] = useState(false)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [hostname, setHostname] = useState('host')
	const [notificationsOpen, setNotificationsOpen] = useState(false)
	const [adminOpen, setAdminOpen] = useState(false)
	const location = useLocation()
	const tools = discoverTools(toolCatalog, capabilities)

	useEffect(() => {
		if (capabilities['accounts.read']) api<{ items: Account[] }>('/api/v1/accounts').then((result) => setAccounts(asList(result))).catch(() => setAccounts([]))
		if (capabilities['server.read']) api<ServerOverview>('/api/v1/server').then((result) => setHostname(result.system.hostname)).catch(() => setHostname('unavailable'))
	}, [capabilities])

	const parts = location.pathname.split('/').filter(Boolean)
	return (
		<div className={`director ${collapsed ? 'nav-collapsed' : ''}`}>
			<Sidebar tools={tools} collapsed={collapsed} onCollapse={() => setCollapsed(!collapsed)} mobileOpen={mobileOpen} onNavigate={() => setMobileOpen(false)} />
			<div className="workspace">
				<header className="topbar">
					<button type="button" className="mobile-menu icon-button" aria-label="Open navigation" onClick={() => setMobileOpen(!mobileOpen)}>☰</button>
					<Link className="topbar-brand" to="/" aria-label="Kelmor Director home"><span>K</span><strong>Kelmor Director</strong></Link>
					<GlobalFind tools={tools} accounts={accounts} />
					<div className="top-actions">
						<div className="popover-wrap">
							<button type="button" className="icon-button" aria-label="Notifications" aria-expanded={notificationsOpen} onClick={() => setNotificationsOpen(!notificationsOpen)}>♢<span className="notification-dot" /></button>
							{notificationsOpen ? <div className="popover notifications"><strong>Notifications</strong><p>No unread alerts. Failed jobs remain visible in Jobs.</p><Link to="/jobs" onClick={() => setNotificationsOpen(false)}>Open Jobs</Link></div> : null}
						</div>
						<span className="hostname" title="Live hostname">● {hostname}</span>
						<div className="popover-wrap">
							<button type="button" className="admin-button" aria-expanded={adminOpen} onClick={() => setAdminOpen(!adminOpen)}><span>{me.user.username.slice(0, 1).toLocaleUpperCase()}</span>{me.user.username}⌄</button>
							{adminOpen ? <div className="popover admin-menu"><strong>{me.user.display_name || me.user.username}</strong><small>{me.user.email}</small><button type="button" onClick={onSignOut}>Sign out</button></div> : null}
						</div>
					</div>
				</header>
				<div className="breadcrumbs" aria-label="Breadcrumb">
					<Link to="/">Home</Link>
					{parts.map((part, index) => <span key={`${part}-${index}`}>/ <span>{crumbLabels[part] || (index === 1 && parts[0] === 'accounts' ? accounts.find((account) => account.id === part)?.username : undefined) || part}</span></span>)}
				</div>
				<main className="page-content">
					<Outlet />
				</main>
				<footer className="workspace-footer">Kelmor Director · Connected to {hostname} {location.state && typeof location.state === 'object' && 'message' in location.state ? `· ${String(location.state.message)}` : ''}</footer>
			</div>
		</div>
	)
}
