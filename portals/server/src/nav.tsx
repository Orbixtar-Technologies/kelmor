import { useEffect, useMemo, useRef, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, asList } from './client'
import {
	matchFind,
	NAV_GROUPS,
	visibleFunctions,
	type FindAccount,
	type FindHit,
} from './find'

const COLLAPSE_KEY = 'director_nav_collapsed'

function loadCollapsed (): Record<string, boolean> {
	try {
		const raw = localStorage.getItem(COLLAPSE_KEY)
		if (!raw) return {}
		const parsed = JSON.parse(raw)
		if (parsed && typeof parsed === 'object') return parsed
	} catch {
		// keep defaults
	}
	return {}
}

export function DirectorNav ({
	caps,
	username,
	onSignOut,
}: {
	caps: Record<string, boolean>
	username: string
	onSignOut: () => void
}) {
	const functions = useMemo(() => visibleFunctions(caps), [caps])
	const loc = useLocation()
	const [collapsed, setCollapsed] = useState<Record<string, boolean>>(loadCollapsed)
	const grouped = NAV_GROUPS
		.map((group) => ({
			group,
			items: functions.filter((fn) => fn.group === group),
		}))
		.filter((g) => g.items.length > 0)

	function toggle (group: string) {
		setCollapsed((prev) => {
			const next = { ...prev, [group]: !prev[group] }
			localStorage.setItem(COLLAPSE_KEY, JSON.stringify(next))
			return next
		})
	}

	return (
		<aside>
			<p className="brand">Kelmor Director</p>
			<FindBox caps={caps} />
			<nav>
				{grouped.map((g) => {
					const isOpen = !collapsed[g.group]
					const hasActive = g.items.some((fn) =>
						fn.end
							? loc.pathname === fn.to
							: loc.pathname === fn.to || loc.pathname.startsWith(fn.to + '/'),
					)
					return (
						<div className="nav-group" key={g.group}>
							<button
								type="button"
								className="nav-group-toggle"
								aria-expanded={isOpen}
								onClick={() => toggle(g.group)}
							>
								{g.group}
								<span>{isOpen ? '−' : '+'}</span>
							</button>
							{isOpen || hasActive ? (
								<div className="nav-group-items" hidden={!isOpen && !hasActive}>
									{g.items.map((fn) => (
										<NavLink key={fn.to} to={fn.to} end={!!fn.end}>
											{fn.label}
										</NavLink>
									))}
								</div>
							) : null}
						</div>
					)
				})}
			</nav>
			<button className="ghost" type="button" onClick={onSignOut}>
				Sign out {username}
			</button>
		</aside>
	)
}

function FindBox ({ caps }: { caps: Record<string, boolean> }) {
	const [q, setQ] = useState('')
	const [accounts, setAccounts] = useState<FindAccount[]>([])
	const [open, setOpen] = useState(false)
	const nav = useNavigate()
	const inputRef = useRef<HTMLInputElement>(null)
	const functions = useMemo(() => visibleFunctions(caps), [caps])

	useEffect(() => {
		function onKey (e: KeyboardEvent) {
			if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return
			const t = e.target as HTMLElement | null
			if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable))
				return
			e.preventDefault()
			inputRef.current?.focus()
		}
		window.addEventListener('keydown', onKey)
		return () => window.removeEventListener('keydown', onKey)
	}, [])

	useEffect(() => {
		if (!q.trim() || !caps['accounts.read']) {
			setAccounts([])
			return
		}
		const handle = window.setTimeout(() => {
			api<{ items: FindAccount[] }>(
				`/api/v1/accounts?q=${encodeURIComponent(q.trim())}`,
			)
				.then((r) => setAccounts(asList(r)))
				.catch(() => setAccounts([]))
		}, 180)
		return () => window.clearTimeout(handle)
	}, [q, caps])

	const hits = matchFind(q, functions, accounts)

	function go (hit: FindHit) {
		setOpen(false)
		setQ('')
		nav(hit.to)
	}

	return (
		<div className="find-wrap">
			<label className="find-label" htmlFor="director-find">
				Find
			</label>
			<input
				id="director-find"
				ref={inputRef}
				className="find"
				placeholder="Functions or accounts"
				value={q}
				autoComplete="off"
				onChange={(e) => {
					setQ(e.target.value)
					setOpen(true)
				}}
				onFocus={() => setOpen(true)}
				onBlur={() => window.setTimeout(() => setOpen(false), 120)}
				onKeyDown={(e) => {
					if (e.key === 'Enter' && hits[0]) {
						e.preventDefault()
						go(hits[0])
					}
					if (e.key === 'Escape') {
						setOpen(false)
						setQ('')
					}
				}}
			/>
			{open && q.trim() ? (
				<ul className="find-results" role="listbox">
					{hits.length === 0 ? (
						<li className="muted">No matching function or account</li>
					) : hits.map((hit) => (
						<li key={hit.kind + hit.to}>
							<button type="button" onMouseDown={() => go(hit)}>
								<span>{hit.label}</span>
								<small>{hit.detail}</small>
							</button>
						</li>
					))}
				</ul>
			) : null}
		</div>
	)
}
