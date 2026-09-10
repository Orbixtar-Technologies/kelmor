import { useEffect, useId, useRef, useState } from 'react'
import { Link, NavLink } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useCapabilities } from '../rbac'

interface PageHeaderProps {
	title: string
	description: string
	actions?: ReactNode
}

export function PageHeader ({ title, description, actions }: PageHeaderProps) {
	return (
		<header className="page-header">
			<div><h1>{title}</h1><p>{description}</p></div>
			{actions ? <div className="page-actions">{actions}</div> : null}
		</header>
	)
}

interface EmptyStateProps {
	title: string
	detail: string
	action?: ReactNode
}

export function EmptyState ({ title, detail, action }: EmptyStateProps) {
	return <div className="empty-state"><strong>{title}</strong><p>{detail}</p>{action}</div>
}

export function LoadingState ({ label = 'Loading data…' }: { label?: string }) {
	return <div className="loading-state" role="status"><span aria-hidden="true" />{label}</div>
}

export function ErrorState ({ title = 'Could not load data', error, onRetry }: { title?: string; error: string; onRetry?: () => void }) {
	return <div className="error-state" role="alert"><strong>{title}</strong><p>{error}</p>{onRetry ? <button type="button" className="secondary" onClick={onRetry}>Try again</button> : null}</div>
}

export function StatusBadge ({ value }: { value: string | boolean | undefined }) {
	const text = typeof value === 'boolean' ? (value ? 'active' : 'inactive') : (value || 'unknown')
	const normalized = text.toLocaleLowerCase()
	const tone = ['active', 'healthy', 'succeeded', 'running', 'yes'].includes(normalized)
		? 'good'
		: ['failed', 'error', 'suspended', 'inactive', 'no'].includes(normalized) ? 'bad' : 'neutral'
	return <span className={`status-badge ${tone}`}>{text}</span>
}

export function Metric ({ label, value, detail }: { label: string; value: ReactNode; detail?: string }) {
	return <article className="metric"><span>{label}</span><strong>{value}</strong>{detail ? <small>{detail}</small> : null}</article>
}

interface DialogProps {
	open: boolean
	title: string
	children: ReactNode
	onClose: () => void
	actions?: ReactNode
}

export function Dialog ({ open, title, children, onClose, actions }: DialogProps) {
	const panelRef = useRef<HTMLDivElement>(null)
	const openerRef = useRef<HTMLElement | null>(null)
	const wasOpenRef = useRef(false)
	const onCloseRef = useRef(onClose)
	const titleId = useId()
	onCloseRef.current = onClose
	if (open && !wasOpenRef.current && typeof document !== 'undefined') {
		openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
	}
	wasOpenRef.current = open
	useEffect(() => {
		if (!open) return
		const panel = panelRef.current
		const focusables = panel?.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')
		if (!panel?.contains(document.activeElement)) (focusables?.[0] || panel)?.focus()
		function handleKeyDown (event: KeyboardEvent) {
			if (event.key === 'Escape') {
				onCloseRef.current()
				return
			}
			if (event.key !== 'Tab' || !panel) return
			const available = [...panel.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')]
			if (!available.length) {
				event.preventDefault()
				panel.focus()
				return
			}
			const first = available[0]
			const last = available[available.length - 1]
			if (!panel.contains(document.activeElement)) {
				event.preventDefault()
				;(event.shiftKey ? last : first).focus()
				return
			}
			if (event.shiftKey && document.activeElement === first) {
				event.preventDefault()
				last.focus()
			} else if (!event.shiftKey && document.activeElement === last) {
				event.preventDefault()
				first.focus()
			}
		}
		document.addEventListener('keydown', handleKeyDown)
		return () => {
			document.removeEventListener('keydown', handleKeyDown)
			openerRef.current?.focus()
			openerRef.current = null
		}
	}, [open])
	if (!open) return null
	return (
		<div className="dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
			<div ref={panelRef} className="dialog" role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}>
				<header><h2 id={titleId}>{title}</h2><button type="button" className="icon-button" aria-label="Close dialog" onClick={onClose}>×</button></header>
				<div className="dialog-body">{children}</div>
				{actions ? <footer>{actions}</footer> : null}
			</div>
		</div>
	)
}

export function Pagination ({ page, pageCount, total, onPage }: { page: number; pageCount: number; total: number; onPage: (page: number) => void }) {
	return (
		<nav className="pagination" aria-label="Pagination">
			<span>{total} results · Page {page} of {pageCount}</span>
			<div>
				<button type="button" className="secondary compact" disabled={page <= 1} onClick={() => onPage(page - 1)}>Previous</button>
				<button type="button" className="secondary compact" disabled={page >= pageCount} onClick={() => onPage(page + 1)}>Next</button>
			</div>
		</nav>
	)
}

export function SectionHeading ({ title, detail, action }: { title: string; detail?: string; action?: ReactNode }) {
	return <div className="section-heading"><div><h2>{title}</h2>{detail ? <p>{detail}</p> : null}</div>{action}</div>
}

export function AccountTabs ({ id }: { id: string }) {
	const capabilities = useCapabilities()
	return (
		<nav className="tabs" aria-label="Account sections">
			<NavLink to={`/accounts/${id}`} end>Overview</NavLink>
			<NavLink to={`/accounts/${id}/services`}>Services</NavLink>
			{capabilities['dns.read'] ? <NavLink to={`/dns?account=${id}`}>DNS</NavLink> : null}
			{capabilities['server.read'] || capabilities['accounts.read'] ? <NavLink to={`/jobs?account=${id}`}>Activity</NavLink> : null}
		</nav>
	)
}

interface CopyableValueProps {
	value: string
	label: string
}

export function CopyableValue ({ value, label }: CopyableValueProps) {
	const [copied, setCopied] = useState(false)
	if (!value) return <span>—</span>
	return (
		<span className="copyable-value">
			<span>{value}</span>
			<button type="button" className="link-button" onClick={async () => {
				try {
					await navigator.clipboard?.writeText(value)
				} catch {
					// Clipboard may be unavailable in older browsers or tests.
				}
				setCopied(true)
			}}>{copied ? 'Copied' : `Copy ${label}`}</button>
		</span>
	)
}

interface SecretValueProps {
	value: string
	label?: string
}

export function SecretValue ({ value, label = 'password' }: SecretValueProps) {
	const [revealed, setRevealed] = useState(false)
	const [copied, setCopied] = useState(false)
	if (!value) return <span>—</span>
	return (
		<span className="secret-value">
			<code>{revealed ? value : '••••••••'}</code>
			<button type="button" className="link-button" onClick={() => setRevealed((current) => !current)}>
				{revealed ? `Hide ${label}` : `Reveal ${label}`}
			</button>
			<button type="button" className="link-button" onClick={async () => {
				try {
					await navigator.clipboard?.writeText(value)
				} catch {
					// Clipboard may be unavailable in older browsers or tests.
				}
				setCopied(true)
			}}>{copied ? 'Copied' : `Copy ${label}`}</button>
		</span>
	)
}
