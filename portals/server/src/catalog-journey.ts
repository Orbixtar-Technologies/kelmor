import { isDirectorToolActive } from './layout/nav-active'
import { featureMatchScore } from './nav-hubs'
import { toToolDefinition, whmFeatures } from './whm-catalog'

const SELF_DESCRIBING = new Set([
	'/mail/delivery-reports',
	'/mail/track-delivery',
	'/ip-usage',
])

const DISTINGUISH_KEYS = ['view', 'task', 'tab', 'q', 'mode'] as const

export interface CatalogJourney {
	id: string
	title: string
	detail: string
}

export function catalogJourneyForLocation (pathname: string, search = ''): CatalogJourney | null {
	if (SELF_DESCRIBING.has(pathname)) return null
	const matches = whmFeatures
		.map((feature) => ({ feature, score: featureMatchScore(feature, pathname, search) }))
		.filter((entry) => entry.score >= 0)
		.sort((left, right) => right.score - left.score)
	const best = matches[0]?.feature
	if (!best) return null
	if (!isDirectorToolActive(toToolDefinition(best), pathname, search)) return null
	const target = new URL(best.path, 'https://director.local')
	const hasDistinguish = DISTINGUISH_KEYS.some((key) => target.searchParams.has(key))
	if (!hasDistinguish) return null
	return { id: best.id, title: best.label, detail: best.description }
}
