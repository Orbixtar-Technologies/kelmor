import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { hrefForTool, isDirectorHubActive, isSidebarToolActive, sidebarToolGroups } from '../nav-hubs'
import type { ToolDefinition } from '../types'

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
	const query = filter.toLocaleLowerCase()
	const homeTool = useMemo(() => tools.find((tool) => tool.id === 'home'), [tools])
	const groups = useMemo(() => {
		return sidebarToolGroups(tools)
			.map((group) => ({
				...group,
				tools: group.tools.filter((tool) => {
					if (!query) return true
					return `${tool.label} ${tool.category} ${group.hub.label}`.toLocaleLowerCase().includes(query)
				}),
			}))
			.filter((group) => group.tools.length)
	}, [tools, query])
	const closedHubs = closedOverride ?? new Set<string>()

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
		setClosedOverride(closed ? new Set(groups.map((group) => group.hub.id)) : new Set())
	}

	const showHome = Boolean(homeTool && (!query || `${homeTool.label} home`.toLocaleLowerCase().includes(query)))

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
				{showHome && homeTool ? (
					<ToolLink tool={homeTool} collapsed={collapsed} pathname={location.pathname} search={location.search} onNavigate={onNavigate} />
				) : null}
				{groups.map((group) => {
					const closed = query && !closedOverride ? false : closedHubs.has(group.hub.id)
					const containsCurrent = isDirectorHubActive(group.hub, location.pathname, location.search)
					return (
						<section key={group.hub.id} data-scope={group.hub.scope}>
							<button type="button" className={`category-heading ${containsCurrent ? 'category-current' : ''}`} aria-expanded={!closed} onClick={() => {
								const next = new Set(closedHubs)
								if (closed) next.delete(group.hub.id)
								else next.add(group.hub.id)
								setClosedOverride(next)
							}}><span>{group.hub.label}</span><span aria-hidden="true">{closed ? '›' : '⌄'}</span></button>
							{closed ? null : group.tools.map((tool) => (
								<ToolLink key={tool.id} tool={tool} collapsed={collapsed} pathname={location.pathname} search={location.search} onNavigate={onNavigate} />
							))}
						</section>
					)
				})}
			</nav>
			<button type="button" className="collapse-sidebar" onClick={onCollapse}>{collapsed ? '›' : '‹'}<span>{collapsed ? 'Expand navigation' : 'Collapse navigation'}</span></button>
		</aside>
	)
}

function ToolLink ({ tool, collapsed, pathname, search, onNavigate }: {
	tool: ToolDefinition
	collapsed: boolean
	pathname: string
	search: string
	onNavigate: () => void
}) {
	const isCurrent = isSidebarToolActive(tool, pathname, search)
	return (
		<Link to={hrefForTool(tool)} className={isCurrent ? 'active' : undefined} aria-current={isCurrent ? 'page' : undefined} title={collapsed ? tool.label : undefined} onClick={onNavigate}>
			<span className="nav-icon" aria-hidden="true">{icons[tool.icon] ?? '•'}</span><span>{tool.label}</span>
		</Link>
	)
}
