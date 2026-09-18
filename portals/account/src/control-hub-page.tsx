import type { ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { activeControlTab, type ControlHub } from './control-hubs'

interface ControlHubPageProps {
	hub: ControlHub
	capabilities: Record<string, boolean>
	pages: Record<string, ReactNode>
}

export function ControlHubPage ({ hub, capabilities, pages }: ControlHubPageProps) {
	const [params] = useSearchParams()
	const active = activeControlTab(hub, params.get('tab'), capabilities)
	if (!active) return <p className="error">You do not have access to this tool.</p>
	const tabs = hub.tabs.filter((tab) => capabilities[tab.cap])

	return (
		<>
			{tabs.length > 1 ? (
				<nav className="tabs" aria-label={`${hub.label} sections`}>
					{tabs.map((tab) => {
						const href = tab.id === hub.tabs[0].id ? hub.path : `${hub.path}?tab=${encodeURIComponent(tab.id)}`
						const isCurrent = tab.id === active.id
						return (
							<Link key={tab.id} to={href} className={isCurrent ? 'active' : undefined} aria-current={isCurrent ? 'page' : undefined}>
								{tab.label}
							</Link>
						)
					})}
				</nav>
			) : null}
			{pages[active.id] ?? null}
		</>
	)
}
