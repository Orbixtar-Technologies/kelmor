import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Icon } from '../components/icons'
import { useDismiss } from '../lib/hooks'
import { GlobalFind } from './global-find'
import type { AccountRow } from '../components/account-picker'

export interface Alert {
	id: string
	tone: 'warn' | 'bad' | 'busy'
	title: string
	detail: string
	to: string
}

export interface TopBarProps {
	username: string
	roles: string[]
	hostname: string
	caps: Record<string, boolean>
	accounts: AccountRow[]
	alerts: Alert[]
	onSignOut: () => void
	onToggleRail: () => void
}

export function TopBar ({ username, roles, hostname, caps, accounts, alerts, onSignOut, onToggleRail }: TopBarProps) {
	return (
		<header className="top-bar">
			<button type="button" className="icon-button" aria-label="Toggle navigation" onClick={onToggleRail}>
				<Icon name="menu" size={18} />
			</button>
			<Link to="/" className="brand-link">
				<span className="mark" aria-hidden="true">K</span>
				<span className="wordmark">Kelmor <span>Director</span></span>
			</Link>
			<GlobalFind caps={caps} accounts={accounts} />
			<span className="spacer" />
			<Notifications alerts={alerts} />
			<span className="host-chip">
				<Icon name="server" size={14} />
				{hostname || 'this host'}
			</span>
			<UserMenu username={username} roles={roles} onSignOut={onSignOut} />
		</header>
	)
}

function Notifications ({ alerts }: { alerts: Alert[] }) {
	const [open, setOpen] = useState(false)
	const ref = useDismiss<HTMLDivElement>(open, () => setOpen(false))
	const navigate = useNavigate()

	return (
		<div className="menu-wrap" ref={ref}>
			<button
				type="button"
				className="icon-button"
				aria-label={`Notifications (${alerts.length})`}
				aria-expanded={open}
				onClick={() => setOpen(!open)}
			>
				<Icon name="bell" size={18} />
				{alerts.length > 0 ? <span className="badge-dot">{alerts.length > 9 ? '9+' : alerts.length}</span> : null}
			</button>
			{open ? (
				<div className="menu" style={{ minWidth: 340 }}>
					<header>Server notifications</header>
					{alerts.length === 0 ? (
						<p className="meta">Nothing needs attention. Failed jobs, suspended accounts and drifted state appear here.</p>
					) : (
						<ul>
							{alerts.map((alert) => (
								<li key={alert.id}>
									<button
										type="button"
										className="menu-item"
										onClick={() => {
											setOpen(false)
											navigate(alert.to)
										}}
									>
										<span className={`pill ${alert.tone}`} style={{ marginRight: 8 }}>{alert.title}</span>
										<span className="small muted">{alert.detail}</span>
									</button>
								</li>
							))}
						</ul>
					)}
				</div>
			) : null}
		</div>
	)
}

function UserMenu ({ username, roles, onSignOut }: { username: string; roles: string[]; onSignOut: () => void }) {
	const [open, setOpen] = useState(false)
	const ref = useDismiss<HTMLDivElement>(open, () => setOpen(false))

	return (
		<div className="menu-wrap" ref={ref}>
			<button type="button" className="user-button" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
				<span className="avatar">{username.slice(0, 1).toUpperCase()}</span>
				<span>{username}</span>
				<Icon name="chevronDown" size={13} />
			</button>
			{open ? (
				<div className="menu" role="menu">
					<header>Signed in as {username}</header>
					<p className="meta">Roles: {roles.length ? roles.join(', ') : 'none'}</p>
					<button type="button" role="menuitem" className="menu-item danger" onClick={onSignOut}>
						Sign out
					</button>
				</div>
			) : null}
		</div>
	)
}
