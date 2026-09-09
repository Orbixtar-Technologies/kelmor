import { useEffect, useMemo, useState } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { Icon } from '../components/icons'
import { categoryOf, visibleCatalog } from '../nav/catalog'

const openKey = 'director_open_categories'

function readOpen (): string[] {
	try {
		const raw = JSON.parse(localStorage.getItem(openKey) || '[]')
		return Array.isArray(raw) ? raw.filter((v) => typeof v === 'string') : []
	} catch {
		return []
	}
}

/**
 * Category rail. Categories start minimized; the one holding the current route
 * opens itself, and the filter box temporarily expands whatever it matches.
 */
export function SideNav ({ caps, open }: { caps: Record<string, boolean>; open: boolean }) {
	const location = useLocation()
	const categories = useMemo(() => visibleCatalog(caps), [caps])
	const [expanded, setExpanded] = useState<string[]>(readOpen)
	const [filter, setFilter] = useState('')

	useEffect(() => {
		const active = categoryOf(location.pathname)
		if (!active) return
		setExpanded((prev) => (prev.includes(active.id) ? prev : [...prev, active.id]))
	}, [location.pathname])

	useEffect(() => {
		localStorage.setItem(openKey, JSON.stringify(expanded))
	}, [expanded])

	const q = filter.trim().toLowerCase()
	const shown = categories
		.map((category) => ({
			...category,
			tools: q
				? category.tools.filter(
					(tool) =>
						tool.name.toLowerCase().includes(q) ||
						(tool.keywords || '').includes(q) ||
						category.name.toLowerCase().includes(q),
				)
				: category.tools,
		}))
		.filter((category) => category.tools.length > 0)

	function toggle (id: string) {
		setExpanded((prev) => (prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]))
	}

	return (
		<nav className={open ? 'rail open' : 'rail'} aria-label="Feature categories">
			<div className="rail-search">
				<input
					type="search"
					placeholder="Filter categories and tools"
					value={filter}
					onChange={(e) => setFilter(e.target.value)}
					aria-label="Filter categories and tools"
				/>
			</div>
			<div className="rail-actions">
				<button type="button" onClick={() => setExpanded(categories.map((c) => c.id))}>Expand all</button>
				<button type="button" onClick={() => setExpanded([])}>Collapse all</button>
				<span className="muted" style={{ marginLeft: 'auto' }}>{shown.length} categories</span>
			</div>

			{shown.length === 0 ? (
				<p className="rail-empty">No tool matches “{filter}”.</p>
			) : (
				shown.map((category) => {
					const isOpen = q ? true : expanded.includes(category.id)
					return (
						<div className="rail-category" key={category.id}>
							<button type="button" aria-expanded={isOpen} onClick={() => toggle(category.id)}>
								<span className="chev"><Icon name="chevronRight" size={12} /></span>
								<Icon name={category.icon} size={15} />
								<span>{category.name}</span>
								<span className="count">{category.tools.length}</span>
							</button>
							{isOpen ? (
								<ul className="rail-tools">
									{category.tools.map((tool) => (
										<li key={tool.path}>
											<NavLink to={tool.path} end title={tool.description}>
												<Icon name={tool.icon} size={14} className="ico" />
												<span>{tool.name}</span>
											</NavLink>
										</li>
									))}
								</ul>
							) : null}
						</div>
					)
				})
			)}
		</nav>
	)
}
