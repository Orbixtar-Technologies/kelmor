import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { groupTools } from '../tool-catalog'
import type { ToolDefinition } from '../types'
import { isDirectorToolActive, navScopeForCategory } from './nav-active'

const icons: Record<string, string> = {
	home: '⌂', users: '👤', pause: 'Ⅱ', meter: '◔', plus: '+', box: '▣',
	briefcase: '▤', globe: '◎', pulse: '⌁', shield: '◇', transfer: '⇄',
	jobs: '≡', audit: '✓', chart: '▥', account: '◉', edit: '✎',
	trash: '×', key: '⌘', database: '▰', mail: '✉', lock: '▧',
	files: '▤', webmail: '✉', code: '{ }',
}

interface SidebarProps {
	tools: ToolDefinition[]
	collapsed: boolean
	onCollapse: () => void
	mobileOpen: boolean
	onNavigate: () => void
	onMobileDismiss: () => void
}

export function Sidebar ({ tools, collapsed, onCollapse, mobileOpen, onNavigate, onMobileDismiss }: SidebarProps) {
	const [filter, setFilter] = useState('')
	const [closedOverride, setClosedOverride] = useState<Set<string> | null>(null)
	const [isMobile, setIsMobile] = useState(() => window.matchMedia?.('(max-width: 780px)').matches ?? false)
	const sidebarRef = useRef<HTMLElement>(null)
	const location = useLocation()
	const groups = useMemo(() => groupTools(tools.filter((tool) => `${tool.label} ${tool.category}`.toLocaleLowerCase().includes(filter.toLocaleLowerCase()))), [tools, filter])
	const currentCategory = useMemo(() => {
		for (const [category, entries] of groups) {
			if (entries.some((tool) => isDirectorToolActive(tool, location.pathname, location.search))) return category
		}
		return ''
	}, [groups, location.pathname, location.search])
	const closedCategories = useMemo(() => {
		if (closedOverride) return closedOverride
		return new Set([...groups.keys()].filter((category) => category !== currentCategory))
	}, [closedOverride, groups, currentCategory])

	useEffect(() => {
		const media = window.matchMedia?.('(max-width: 780px)')
		if (!media) return
		const update = () => setIsMobile(media.matches)
		update()
		media.addEventListener('change', update)
		return () => media.removeEventListener('change', update)
	}, [])

	useEffect(() => {
		if (!isMobile || !mobileOpen) return
		sidebarRef.current?.querySelector<HTMLElement>('input, button, a[href]')?.focus()
		const current = sidebarRef.current?.querySelector<HTMLElement>('a.active')
		if (typeof current?.scrollIntoView === 'function') current.scrollIntoView({ block: 'nearest' })
	}, [isMobile, mobileOpen])

	function setAll (closed: boolean) {
		setClosedOverride(closed ? new Set(groups.keys()) : new Set())
	}

	return (
		<aside
			ref={sidebarRef}
			id="director-sidebar"
			className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileOpen ? 'mobile-open' : ''}`}
			aria-hidden={isMobile && !mobileOpen ? true : undefined}
			inert={isMobile && !mobileOpen ? true : undefined}
			onKeyDown={(event) => {
				if (isMobile && mobileOpen && event.key === 'Escape') {
					event.preventDefault()
					onMobileDismiss()
				}
			}}
		>
			<div className="sidebar-brand"><span className="brand-mark">K</span><strong>Kelmor Director</strong></div>
			<div className="sidebar-controls">
				<label className="sr-only" htmlFor="category-filter">Filter features</label>
				<input id="category-filter" value={filter} placeholder="Filter features" onChange={(event) => setFilter(event.target.value)} />
				<div><button type="button" onClick={() => setAll(false)}>Expand</button><button type="button" onClick={() => setAll(true)}>Collapse</button></div>
			</div>
			<nav className="feature-nav" aria-label="Director tools">
				{[...groups.entries()].map(([category, entries]) => {
					const closed = filter && !closedOverride ? false : closedCategories.has(category)
					const scope = navScopeForCategory(category)
					const containsCurrent = entries.some((tool) => isDirectorToolActive(tool, location.pathname, location.search))
					return (
						<section key={category} data-scope={scope}>
							<button type="button" className={`category-heading ${containsCurrent ? 'category-current' : ''}`} aria-expanded={!closed} onClick={() => {
								const next = new Set(closedCategories)
								if (closed) next.delete(category)
								else next.add(category)
								setClosedOverride(next)
							}}><span>{category}</span><span aria-hidden="true">{closed ? '›' : '⌄'}</span></button>
							{closed ? null : entries.map((tool) => {
								const isCurrent = isDirectorToolActive(tool, location.pathname, location.search)
								return (
									<Link key={tool.id} to={tool.path} className={isCurrent ? 'active' : undefined} aria-current={isCurrent ? 'page' : undefined} title={collapsed ? tool.label : undefined} onClick={onNavigate}>
										<span className="nav-icon" aria-hidden="true">{icons[tool.icon] ?? '•'}</span><span>{tool.label}</span>
									</Link>
								)
							})}
						</section>
					)
				})}
			</nav>
			<button type="button" className="collapse-sidebar" onClick={onCollapse}>{collapsed ? '›' : '‹'}<span>{collapsed ? 'Expand navigation' : 'Collapse navigation'}</span></button>
		</aside>
	)
}
