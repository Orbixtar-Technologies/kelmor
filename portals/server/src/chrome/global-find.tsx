import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Icon } from '../components/icons'
import { useDismiss } from '../lib/hooks'
import { scoreTool, visibleCatalog, type Tool } from '../nav/catalog'
import type { AccountRow } from '../components/account-picker'

export interface GlobalFindProps {
	caps: Record<string, boolean>
	accounts: AccountRow[]
}

interface Hit {
	key: string
	label: string
	detail: string
	where: string
	to: string
}

/**
 * Top-bar find. Matches both feature tools and hosting accounts by username or
 * domain, so "jump to a tool" and "jump to a customer" are one control.
 */
export function GlobalFind ({ caps, accounts }: GlobalFindProps) {
	const [query, setQuery] = useState('')
	const [open, setOpen] = useState(false)
	const [cursor, setCursor] = useState(0)
	const inputRef = useRef<HTMLInputElement>(null)
	const wrapRef = useDismiss<HTMLDivElement>(open, () => setOpen(false))
	const navigate = useNavigate()

	useEffect(() => {
		function onKey (event: KeyboardEvent) {
			const target = event.target as HTMLElement | null
			const typing = target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)
			if (event.key === '/' && !typing) {
				event.preventDefault()
				inputRef.current?.focus()
				setOpen(true)
			}
		}
		document.addEventListener('keydown', onKey)
		return () => document.removeEventListener('keydown', onKey)
	}, [])

	const tools = useMemo(() => visibleCatalog(caps).flatMap((category) => category.tools.map((tool) => ({ tool, category: category.name }))), [caps])

	const toolHits: Hit[] = useMemo(() => {
		if (!query.trim()) return []
		return tools
			.map(({ tool, category }) => ({ tool, category, score: scoreTool(tool, query) }))
			.filter((hit) => hit.score > 0)
			.sort((a, b) => b.score - a.score)
			.slice(0, 7)
			.map(({ tool, category }: { tool: Tool; category: string }) => ({
				key: `tool:${tool.path}`,
				label: tool.name,
				detail: tool.description,
				where: category,
				to: tool.path,
			}))
	}, [tools, query])

	const accountHits: Hit[] = useMemo(() => {
		const q = query.trim().toLowerCase()
		if (!q || !caps['accounts.read']) return []
		return accounts
			.filter((a) => a.username.toLowerCase().includes(q) || a.primary_domain.toLowerCase().includes(q))
			.slice(0, 6)
			.map((account) => ({
				key: `account:${account.id}`,
				label: account.username,
				detail: account.primary_domain,
				where: account.status,
				to: `/accounts/${account.id}`,
			}))
	}, [accounts, query, caps])

	const hits = [...toolHits, ...accountHits]

	function go (hit: Hit) {
		setOpen(false)
		setQuery('')
		inputRef.current?.blur()
		navigate(hit.to)
	}

	return (
		<div className="find" ref={wrapRef}>
			<span className="glass"><Icon name="search" size={14} /></span>
			<input
				ref={inputRef}
				type="text"
				role="combobox"
				aria-expanded={open && hits.length > 0}
				aria-controls="find-results"
				aria-label="Find features and accounts"
				placeholder="Find a feature or an account"
				value={query}
				onChange={(e) => {
					setQuery(e.target.value)
					setOpen(true)
					setCursor(0)
				}}
				onFocus={() => setOpen(true)}
				onKeyDown={(e) => {
					if (e.key === 'Escape') {
						setOpen(false)
						e.currentTarget.blur()
						return
					}
					if (e.key === 'ArrowDown') {
						e.preventDefault()
						setCursor((c) => Math.min(c + 1, hits.length - 1))
					}
					if (e.key === 'ArrowUp') {
						e.preventDefault()
						setCursor((c) => Math.max(c - 1, 0))
					}
					if (e.key === 'Enter' && hits[cursor]) {
						e.preventDefault()
						go(hits[cursor])
					}
				}}
			/>
			{!query ? <span className="kbd">/</span> : null}
			{open && query.trim() ? (
				<div className="find-results" id="find-results" role="listbox">
					{toolHits.length > 0 ? <p className="group">Features</p> : null}
					{toolHits.map((hit) => (
						<button
							key={hit.key}
							type="button"
							role="option"
							aria-selected={hits[cursor]?.key === hit.key}
							className={hits[cursor]?.key === hit.key ? 'active' : undefined}
							onMouseEnter={() => setCursor(hits.findIndex((h) => h.key === hit.key))}
							onClick={() => go(hit)}
						>
							<Icon name="compass" size={14} />
							<span>
								<strong>{hit.label}</strong>
								<br />
								<span className="muted small">{hit.detail}</span>
							</span>
							<span className="where">{hit.where}</span>
						</button>
					))}
					{accountHits.length > 0 ? <p className="group">Accounts</p> : null}
					{accountHits.map((hit) => (
						<button
							key={hit.key}
							type="button"
							role="option"
							aria-selected={hits[cursor]?.key === hit.key}
							className={hits[cursor]?.key === hit.key ? 'active' : undefined}
							onMouseEnter={() => setCursor(hits.findIndex((h) => h.key === hit.key))}
							onClick={() => go(hit)}
						>
							<Icon name="users" size={14} />
							<span>
								<strong>{hit.label}</strong>
								<br />
								<span className="muted small">{hit.detail}</span>
							</span>
							<span className="where">{hit.where}</span>
						</button>
					))}
					{hits.length === 0 ? <p className="none">No feature or account matches “{query}”.</p> : null}
				</div>
			) : null}
		</div>
	)
}
