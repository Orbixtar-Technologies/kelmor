import { Link, Navigate, useParams, useSearchParams } from 'react-router-dom'
import {
	hubById,
	hubForToolId,
	hubToolHref,
	toolsForHub,
	usesDedicatedManager,
} from '../nav-hubs'
import { PageHeader } from '../components/ui'
import { isDirectorToolActive } from '../layout/nav-active'
import { groupTools, toolCatalog } from '../tool-catalog'
import { featureById } from '../whm-catalog'
import { WhmToolBody } from './whm-tool-page'

interface HubTabsProps {
	hubId: string
	pathname: string
	search: string
}

export function HubTabs ({ hubId, pathname, search }: HubTabsProps) {
	const hub = hubById(hubId)
	if (!hub || hub.id === 'home') return null
	const tools = toolsForHub(hub, toolCatalog)
	const grouped = groupTools(tools)
	const currentTool = new URLSearchParams(search.startsWith('?') ? search : search ? `?${search}` : '').get('tool')

	return (
		<div className="hub-chrome">
			{[...grouped.entries()].map(([category, entries]) => (
				<section key={category} className="hub-tool-group">
					{grouped.size > 1 ? <h2 className="hub-group-label">{category}</h2> : null}
					<nav className="tabs hub-tabs" aria-label={`${category} tools`}>
						{entries.map((tool) => {
							const feature = featureById(tool.id)
							if (!feature) return null
							const href = hubToolHref(hub, feature)
							const isCurrent = pathname.startsWith('/section/')
								? currentTool === tool.id || (!currentTool && tool.id === hub.defaultToolId)
								: isDirectorToolActive(tool, pathname, search)
							return (
								<Link
									key={tool.id}
									to={href}
									className={isCurrent ? 'active' : undefined}
									aria-current={isCurrent ? 'page' : undefined}
								>
									{tool.label}
								</Link>
							)
						})}
					</nav>
				</section>
			))}
		</div>
	)
}

export function HubPage () {
	const { hubId = '' } = useParams()
	const [params] = useSearchParams()
	const hub = hubById(hubId)
	if (!hub || hub.id === 'home') return <Navigate to="/" replace />

	const requested = params.get('tool') || hub.defaultToolId
	const feature = featureById(requested)
	if (!feature || hubForToolId(feature.id)?.id !== hub.id) {
		return (
			<>
				<PageHeader title={hub.label} description={hub.description} />
				<p>This tool is not part of {hub.label}.</p>
			</>
		)
	}
	if (usesDedicatedManager(feature)) {
		return <Navigate to={hubToolHref(hub, feature)} replace />
	}
	return <WhmToolBody key={feature.id} feature={feature} />
}

export function ToolRedirect () {
	const { toolId = '' } = useParams()
	const hub = hubForToolId(toolId)
	const feature = featureById(toolId)
	if (!hub || !feature) {
		return (
			<>
				<PageHeader title="Tool not found" description="This path is not in the Director catalog." />
				<p><Link to="/">Return home</Link></p>
			</>
		)
	}
	return <Navigate to={hubToolHref(hub, feature)} replace />
}
