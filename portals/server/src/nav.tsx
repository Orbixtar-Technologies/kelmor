import { useEffect, useMemo, useRef, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { api, asList } from './client'
import {
	matchFind,
	NAV_GROUPS,
	visibleFunctions,
	type FindAccount,
	type FindHit,
} from './find'

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
	const grouped = NAV_GROUPS
		.map((group) => ({
			group,
			items: functions.filter((fn) => fn.group === group),
		}))
		.filter((g) => g.items.length > 0)

	return (
		<aside>
			<p className="brand">Kelmor Director</p>
			<FindBox caps={caps} />
			<nav>
				{grouped.map((g) => (
					<div className="nav-group" key={g.group}>
						<h2>{g.group}</h2>
						{g.items.map((fn) => (
							<NavLink
								key={fn.to}
								to={fn.to}
								end={fn.to === '/' || fn.to === '/accounts'}
							>
								{fn.label}
							</NavLink>
						))}
					</div>
				))}
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
