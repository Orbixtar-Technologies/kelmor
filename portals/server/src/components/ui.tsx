import { useEffect, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Icon, type IconName } from './icons'
import { useFavorites } from '../lib/hooks'
import { categoryOf, toolByPath } from '../nav/catalog'

export interface Crumb {
	label: string
	to?: string
}

export interface PageHeaderProps {
	title: string
	description?: ReactNode
	crumbs?: Crumb[]
	actions?: ReactNode
	/** Tool path used for the favourite star. Defaults to the current route. */
	favoritePath?: string
}

export function PageHeader ({ title, description, crumbs, actions, favoritePath }: PageHeaderProps) {
	const path = favoritePath ?? window.location.pathname
	const tool = toolByPath(path)
	const category = categoryOf(path)
	const { isFavorite, toggle } = useFavorites()
	const leaf = tool?.name ?? title
	const trail: Crumb[] = crumbs ?? [
		...(category && category.name !== leaf ? [{ label: category.name }] : []),
		{ label: leaf },
	]

	return (
		<>
			<Breadcrumbs crumbs={trail} />
			<div className="page-head">
				<div>
					<h1>{title}</h1>
					{description ? <p>{description}</p> : null}
				</div>
				<div className="head-actions">
					{actions}
					{tool ? (
						<button
							type="button"
							className={isFavorite(path) ? 'star on' : 'star'}
							aria-pressed={isFavorite(path)}
							title={isFavorite(path) ? 'Remove from favourites' : 'Add to favourites'}
							onClick={() => toggle(path)}
						>
							<Icon name="star" size={19} />
						</button>
					) : null}
				</div>
			</div>
		</>
	)
}

export function Breadcrumbs ({ crumbs }: { crumbs: Crumb[] }) {
	return (
		<nav className="crumbs" aria-label="Breadcrumb">
			<Link to="/">Home</Link>
			{crumbs.map((crumb) => (
				<span key={crumb.label} style={{ display: 'contents' }}>
					<span className="sep">
						<Icon name="chevronRight" size={11} />
					</span>
					{crumb.to ? <Link to={crumb.to}>{crumb.label}</Link> : <span>{crumb.label}</span>}
				</span>
			))}
		</nav>
	)
}

export interface PanelProps {
	title?: ReactNode
	subtitle?: ReactNode
	icon?: IconName
	actions?: ReactNode
	tight?: boolean
	children: ReactNode
}

export function Panel ({ title, subtitle, icon, actions, tight, children }: PanelProps) {
	return (
		<section className="panel">
			{title ? (
				<header>
					{icon ? <Icon name={icon} size={16} /> : null}
					<h2>{title}</h2>
					{subtitle ? <span className="sub">{subtitle}</span> : null}
					{actions ? <div className="panel-actions">{actions}</div> : null}
				</header>
			) : null}
			<div className={tight ? 'body tight' : 'body'}>{children}</div>
		</section>
	)
}

export type NoticeTone = 'info' | 'ok' | 'warn' | 'error'

const noticeIcon: Record<NoticeTone, IconName> = {
	info: 'info',
	ok: 'check',
	warn: 'alertTriangle',
	error: 'alertCircle',
}

export function Notice ({ tone = 'info', children }: { tone?: NoticeTone; children: ReactNode }) {
	return (
		<div className={`notice ${tone}`} role={tone === 'error' ? 'alert' : undefined}>
			<Icon name={noticeIcon[tone]} size={16} className="ico" />
			<div>{children}</div>
		</div>
	)
}

export function EmptyState ({ icon = 'inbox', title, children, action }: { icon?: IconName; title: string; children?: ReactNode; action?: ReactNode }) {
	return (
		<div className="empty">
			<Icon name={icon} size={30} className="ico" />
			<h3>{title}</h3>
			{children ? <p>{children}</p> : null}
			{action}
		</div>
	)
}

export function Loading ({ label = 'Loading' }: { label?: string }) {
	return (
		<div className="loading">
			<Icon name="refresh" size={16} className="spin" />
			<span>{label}…</span>
		</div>
	)
}

export type PillTone = 'ok' | 'warn' | 'bad' | 'busy' | 'idle'

export function Pill ({ tone, children }: { tone: PillTone; children: ReactNode }) {
	return <span className={`pill ${tone}`}>{children}</span>
}

/** Maps the control-plane status vocabulary onto a consistent pill colour. */
export function statusTone (status: string): PillTone {
	switch (status) {
		case 'active':
		case 'succeeded':
		case 'healthy':
		case 'issued':
			return 'ok'
		case 'suspended':
		case 'degraded':
		case 'expiring':
			return 'warn'
		case 'failed':
		case 'dead':
		case 'terminated':
		case 'stopped':
			return 'bad'
		case 'provisioning':
		case 'running':
		case 'queued':
		case 'terminating':
		case 'pending':
			return 'busy'
		default:
			return 'idle'
	}
}

export function StatusPill ({ status }: { status: string }) {
	return <Pill tone={statusTone(status)}>{status || 'unknown'}</Pill>
}

export interface FieldProps {
	label: string
	hint?: ReactNode
	error?: string
	children: ReactNode
}

export function Field ({ label, hint, error, children }: FieldProps) {
	return (
		<div className="field">
			<label>{label}</label>
			{children}
			{error ? <span className="bad">{error}</span> : null}
			{hint && !error ? <span className="hint">{hint}</span> : null}
		</div>
	)
}

export function CheckField ({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (v: boolean) => void }) {
	return (
		<div>
			<div className="field inline">
				<input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
				<label>{label}</label>
			</div>
			{hint ? <span className="hint">{hint}</span> : null}
		</div>
	)
}

export interface ConfirmProps {
	title: string
	tone?: 'danger' | 'normal'
	confirmLabel: string
	/** When set, the operator must type this string before confirming. */
	typeToConfirm?: string
	busy?: boolean
	onConfirm: () => void
	onCancel: () => void
	children: ReactNode
}

/** Modal gate in front of destructive actions. */
export function ConfirmDialog ({ title, tone = 'danger', confirmLabel, typeToConfirm, busy, onConfirm, onCancel, children }: ConfirmProps) {
	const [typed, setTyped] = useState('')
	const blocked = !!typeToConfirm && typed.trim() !== typeToConfirm

	useEffect(() => {
		function onKey (event: KeyboardEvent) {
			if (event.key === 'Escape') onCancel()
		}
		document.addEventListener('keydown', onKey)
		return () => document.removeEventListener('keydown', onKey)
	}, [onCancel])

	return (
		<div className="scrim" role="dialog" aria-modal="true" aria-label={title}>
			<div className="modal">
				<header>
					<Icon name={tone === 'danger' ? 'alertTriangle' : 'info'} size={17} />
					<h2>{title}</h2>
				</header>
				<div className="body">
					{children}
					{typeToConfirm ? (
						<div style={{ marginTop: 14 }}>
							<Field label={`Type ${typeToConfirm} to confirm`}>
								<input value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus spellCheck={false} />
							</Field>
						</div>
					) : null}
				</div>
				<footer>
					<button type="button" className="btn secondary" onClick={onCancel}>Cancel</button>
					<button
						type="button"
						className={tone === 'danger' ? 'btn danger' : 'btn'}
						disabled={blocked || busy}
						onClick={onConfirm}
					>
						{busy ? 'Working…' : confirmLabel}
					</button>
				</footer>
			</div>
		</div>
	)
}

export function Drawer ({ title, onClose, children }: { title: ReactNode; onClose: () => void; children: ReactNode }) {
	useEffect(() => {
		function onKey (event: KeyboardEvent) {
			if (event.key === 'Escape') onClose()
		}
		document.addEventListener('keydown', onKey)
		return () => document.removeEventListener('keydown', onKey)
	}, [onClose])

	return (
		<aside className="drawer" role="dialog" aria-modal="false" aria-label={typeof title === 'string' ? title : 'Detail'}>
			<header>
				<h2>{title}</h2>
				<button type="button" className="btn secondary small close" onClick={onClose}>
					<Icon name="x" size={13} /> Close
				</button>
			</header>
			<div className="body">{children}</div>
		</aside>
	)
}

export function KeyValues ({ rows }: { rows: Array<[string, ReactNode]> }) {
	return (
		<ul className="kv">
			{rows.map(([key, value]) => (
				<li key={key}>
					<span className="k">{key}</span>
					<span className="v">{value ?? '—'}</span>
				</li>
			))}
		</ul>
	)
}

export function Meter ({ used, limit, className }: { used: number; limit: number; className: string }) {
	const percent = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0
	return (
		<span className={className} title={limit > 0 ? `${percent}% of limit` : 'Unmetered'}>
			<span style={{ width: `${percent}%` }} />
		</span>
	)
}
