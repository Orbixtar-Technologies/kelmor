import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { hubEntryPath, isDirectorHubActive, visibleHubs, type NavHub } from '../nav-hubs'
import type { ToolDefinition } from '../types'

const icons: Record<string, string> = {
	home: '⌂', users: '👤', pause: 'Ⅱ', meter: '◔', plus: '+', box: '▣',
	briefcase: '▤', globe: '◎', pulse: '⌁', shield: '◇', transfer: '⇄',
	jobs: '≡', audit: '✓', chart: '▥', account: '◉', edit: '✎',
	trash: '×', key: '⌘', database: '▰', mail: '✉', lock: '▧',
	files: '▤', webmail: '✉', code: '{ }',
}

const HUB_GROUPS: Array<{ id: string; label: string; scope: NavHub['scope'] }> = [
	{ id: 'account', label: 'Account services', scope: 'account' },
	{ id: 'host', label: 'Host operations', scope: 'host' },
]

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
	const query = filter.toLocaleLowerCase()
	const hubs = useMemo(() => {
		return visibleHubs(tools).filter((hub) => {
			if (!query) return true
			if (`${hub.label} ${hub.description}`.toLocaleLowerCase().includes(query)) return true
			return tools.some((tool) => {
				if (hub.categories.includes(tool.category) && `${tool.label} ${tool.category}`.toLocaleLowerCase().includes(query)) return true
				return false
			})
		})
	}, [tools, query])
	const homeHub = hubs.find((hub) => hub.id === 'home')
	const groupedHubs = useMemo(() => {
		return HUB_GROUPS.map((group) => ({
			...group,
			hubs: hubs.filter((hub) => hub.id !== 'home' && hub.scope === group.scope),
		})).filter((group) => group.hubs.length)
	}, [hubs])
	const currentGroupId = useMemo(() => {
		for (const group of groupedHubs) {
			if (group.hubs.some((hub) => isDirectorHubActive(hub, location.pathname, location.search))) return group.id
		}
		return ''
	}, [groupedHubs, location.pathname, location.search])
	const closedGroups = useMemo(() => {
		if (closedOverride) return closedOverride
		return new Set(groupedHubs.map((group) => group.id).filter((id) => id !== currentGroupId))
	}, [closedOverride, groupedHubs, currentGroupId])

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
		setClosedOverride(closed ? new Set(groupedHubs.map((group) => group.id)) : new Set())
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
				{homeHub ? <HubLink hub={homeHub} collapsed={collapsed} pathname={location.pathname} search={location.search} onNavigate={onNavigate} /> : null}
				{groupedHubs.map((group) => {
					const closed = query && !closedOverride ? false : closedGroups.has(group.id)
					const containsCurrent = group.hubs.some((hub) => isDirectorHubActive(hub, location.pathname, location.search))
					return (
						<section key={group.id} data-scope={group.scope}>
							<button type="button" className={`category-heading ${containsCurrent ? 'category-current' : ''}`} aria-expanded={!closed} onClick={() => {
								const next = new Set(closedGroups)
								if (closed) next.delete(group.id)
								else next.add(group.id)
								setClosedOverride(next)
							}}><span>{group.label}</span><span aria-hidden="true">{closed ? '›' : '⌄'}</span></button>
							{closed ? null : group.hubs.map((hub) => (
								<HubLink key={hub.id} hub={hub} collapsed={collapsed} pathname={location.pathname} search={location.search} onNavigate={onNavigate} />
							))}
						</section>
					)
				})}
			</nav>
			<button type="button" className="collapse-sidebar" onClick={onCollapse}>{collapsed ? '›' : '‹'}<span>{collapsed ? 'Expand navigation' : 'Collapse navigation'}</span></button>
		</aside>
	)
}

function HubLink ({ hub, collapsed, pathname, search, onNavigate }: {
	hub: NavHub
	collapsed: boolean
	pathname: string
	search: string
	onNavigate: () => void
}) {
	const isCurrent = isDirectorHubActive(hub, pathname, search)
	return (
		<Link to={hubEntryPath(hub)} className={isCurrent ? 'active' : undefined} aria-current={isCurrent ? 'page' : undefined} title={collapsed ? hub.label : undefined} onClick={onNavigate}>
			<span className="nav-icon" aria-hidden="true">{icons[hub.icon] ?? '•'}</span><span>{hub.label}</span>
		</Link>
	)
}
