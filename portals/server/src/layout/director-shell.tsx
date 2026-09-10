import { useEffect, useRef, useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { api, asList } from '../client'
import { NotificationBell } from '../components/notification-bell'
import { ResourcesSidebar } from '../components/resources-sidebar'
import { useCapabilities } from '../rbac'
import { discoverTools, toolCatalog } from '../tool-catalog'
import { GlobalFind } from './global-find'
import { directorBreadcrumbs } from './breadcrumbs'
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
	files: 'File Manager', sql: 'Database Manager', email: 'Email Management',
	ssl: 'SSL / TLS', webmail: 'Webmail', updates: 'Software Updates',
}

export function DirectorShell ({ me, onSignOut }: DirectorShellProps) {
	const capabilities = useCapabilities()
	const [collapsed, setCollapsed] = useState(false)
	const [mobileOpen, setMobileOpen] = useState(false)
	const [accounts, setAccounts] = useState<Account[]>([])
	const [hostname, setHostname] = useState('host')
	const [server, setServer] = useState<ServerOverview | null>(null)
	const [serverFetchedAt, setServerFetchedAt] = useState<string>('')
	const [resourcesCollapsed, setResourcesCollapsed] = useState(false)
	const [notificationsOpen, setNotificationsOpen] = useState(false)
	const [adminOpen, setAdminOpen] = useState(false)
	const menuButtonRef = useRef<HTMLButtonElement>(null)
	const notificationsRef = useRef<HTMLDivElement>(null)
	const adminRef = useRef<HTMLDivElement>(null)
	const location = useLocation()
	const tools = discoverTools(toolCatalog, capabilities)
	const canViewResources = Boolean(capabilities['server.read'])

	useEffect(() => {
		if (capabilities['accounts.read']) api<{ items: Account[] }>('/api/v1/accounts').then((result) => setAccounts(asList(result))).catch(() => setAccounts([]))
		if (capabilities['server.read']) {
			api<ServerOverview>('/api/v1/server').then((result) => {
				setServer(result)
				setHostname(result.system.hostname)
				setServerFetchedAt(new Date().toISOString())
			}).catch(() => {
				setServer(null)
				setHostname('unavailable')
			})
		}
	}, [capabilities])

	useEffect(() => {
		function handlePointerDown (event: MouseEvent) {
			const target = event.target as Node
			if (notificationsOpen && notificationsRef.current && !notificationsRef.current.contains(target)) {
				setNotificationsOpen(false)
			}
			if (adminOpen && adminRef.current && !adminRef.current.contains(target)) {
				setAdminOpen(false)
			}
		}
		function handleKeyDown (event: KeyboardEvent) {
			if (event.key !== 'Escape') return
			if (notificationsOpen) {
				event.preventDefault()
				setNotificationsOpen(false)
				notificationsRef.current?.querySelector('button')?.focus()
			}
			if (adminOpen) {
				event.preventDefault()
				setAdminOpen(false)
			}
		}
		document.addEventListener('mousedown', handlePointerDown)
		document.addEventListener('keydown', handleKeyDown)
		return () => {
			document.removeEventListener('mousedown', handlePointerDown)
			document.removeEventListener('keydown', handleKeyDown)
		}
	}, [notificationsOpen, adminOpen])

	const accountId = location.pathname.match(/^\/accounts\/([^/]+)/)?.[1]
	const accountName = accountId && accountId !== 'create' ? accounts.find((account) => account.id === accountId)?.username : undefined
	const toolAccountId = new URLSearchParams(location.search).get('account') || ''
	const toolAccountName = toolAccountId ? accounts.find((account) => account.id === toolAccountId)?.username : undefined
	const crumbs = directorBreadcrumbs(location.pathname, crumbLabels, accountName, toolAccountName)
	function dismissMobileNavigation () {
		setMobileOpen(false)
		menuButtonRef.current?.focus()
	}
	function handleSidebarNavigation () {
		if (mobileOpen) {
			dismissMobileNavigation()
			return
		}
		setMobileOpen(false)
	}

	return (
		<div className={`director ${collapsed ? 'nav-collapsed' : ''} ${resourcesCollapsed ? 'resources-collapsed' : ''}`}>
			<Sidebar tools={tools} collapsed={collapsed} onCollapse={() => setCollapsed(!collapsed)} mobileOpen={mobileOpen} onNavigate={handleSidebarNavigation} onMobileDismiss={dismissMobileNavigation} />
			<div className="workspace">
				<header className="topbar">
					<button ref={menuButtonRef} type="button" className="mobile-menu icon-button" aria-label={mobileOpen ? 'Close navigation' : 'Open navigation'} aria-expanded={mobileOpen} aria-controls="director-sidebar" onClick={() => setMobileOpen((open) => !open)}>
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
					</button>
					<Link className="topbar-brand" to="/" aria-label="Kelmor Director home"><span>K</span><strong>Kelmor Director</strong></Link>
					<GlobalFind tools={tools} accounts={accounts} />
					<div className="top-actions">
						<div className="popover-wrap" ref={notificationsRef}>
							<button type="button" className="icon-button notifications-trigger" aria-label="Notifications" aria-expanded={notificationsOpen} onClick={() => { setNotificationsOpen(!notificationsOpen); setAdminOpen(false) }}>
								<NotificationBell />
							</button>
							{notificationsOpen ? (
								<div className="popover notifications-popover" role="dialog" aria-label="Notifications">
									<header className="popover-header"><strong>Notifications</strong></header>
									<p className="popover-empty">No unread alerts.{tools.some((tool) => tool.id === 'jobs') ? ' Failed jobs remain visible in Jobs.' : ''}</p>
									{tools.some((tool) => tool.id === 'jobs') ? <Link className="popover-action" to="/jobs" onClick={() => setNotificationsOpen(false)}>Open Jobs</Link> : null}
								</div>
							) : null}
						</div>
						<span className="hostname" title="Live hostname"><span className="hostname-dot" aria-hidden="true" />{hostname}</span>
						<div className="popover-wrap" ref={adminRef}>
							<button type="button" className="admin-button" aria-expanded={adminOpen} aria-haspopup="true" onClick={() => { setAdminOpen(!adminOpen); setNotificationsOpen(false) }}>
								<span>{me.user.username.slice(0, 1).toLocaleUpperCase()}</span>
								<span className="admin-button-label">{me.user.username}</span>
								<svg className="admin-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
							</button>
							{adminOpen ? (
								<div className="popover admin-menu" role="menu">
									<header className="popover-header"><strong>{me.user.display_name || me.user.username}</strong><small>{me.user.email}</small></header>
									<button type="button" role="menuitem" onClick={onSignOut}>Sign out</button>
								</div>
							) : null}
						</div>
					</div>
				</header>
				<nav className="breadcrumbs" aria-label="Breadcrumb">
					{crumbs.map((crumb, index) => (
						<span key={`${crumb.label}-${index}`}>
							{index > 0 ? ' / ' : null}
							{crumb.to && index < crumbs.length - 1 ? <Link to={crumb.to}>{crumb.label}</Link> : <span>{crumb.label}</span>}
						</span>
					))}
				</nav>
				<div className={`page-body ${canViewResources ? 'with-resources' : ''} ${resourcesCollapsed ? 'resources-collapsed' : ''}`}>
					<main className="page-content">
						<Outlet />
					</main>
					<ResourcesSidebar
						server={server}
						canViewStatus={canViewResources}
						updatedAt={serverFetchedAt}
						collapsed={resourcesCollapsed}
						onToggle={() => setResourcesCollapsed((current) => !current)}
					/>
				</div>
				<footer className="workspace-footer">Kelmor Director · Connected to {hostname} {location.state && typeof location.state === 'object' && 'message' in location.state ? `· ${String(location.state.message)}` : ''}</footer>
			</div>
		</div>
	)
}
