import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, SectionHeading, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import type { FeatureSet, Package } from '../types'
import { assembleFeatureShowcase, type ShowcaseEntry, type ShowcaseHostApp } from './feature-showcase-catalog'

export const FEATURE_SHOWCASE_EMPTY_TITLE = 'No feature catalog'
export const FEATURE_SHOWCASE_EMPTY_DETAIL = 'This host has not published any package feature sets or host applications yet. Open Feature Manager to inspect and assign the seeded capability catalog.'
export const FEATURE_MANAGER_CTA = 'Open Feature Manager'

export function FeatureShowcasePage () {
	const [entries, setEntries] = useState<ShowcaseEntry[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: FeatureSet[] }>('/api/v1/feature-sets'),
			api<{ items: Package[] }>('/api/v1/packages'),
			api<{ items: ShowcaseHostApp[] }>('/api/v1/server/apps'),
		]).then(([featureResult, packageResult, appResult]) => {
			const featureSets = featureResult.status === 'fulfilled' ? asList(featureResult.value) : []
			const packages = packageResult.status === 'fulfilled' ? asList(packageResult.value) : []
			const hostApps = appResult.status === 'fulfilled' ? asList(appResult.value) : []
			const failures = [featureResult, packageResult, appResult].filter((result) => result.status === 'rejected')
			if (failures.length === 3) {
				setError(messageFrom(featureResult.status === 'rejected' ? featureResult.reason : failures[0].reason))
				setEntries([])
				return
			}
			setEntries(assembleFeatureShowcase({ featureSets, packages, hostApps }))
			if (!featureSets.length && featureResult.status === 'rejected' && !hostApps.length) {
				setError(messageFrom(featureResult.reason))
			}
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])
	const packageEntries = entries.filter((entry) => entry.group === 'package')
	const hostEntries = entries.filter((entry) => entry.group === 'host')

	return (
		<>
			<p className="subtle">
				<Link to="/features">{FEATURE_MANAGER_CTA}</Link>
				{' · '}
				<Link to="/packages">Packages</Link>
			</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading feature catalog…" /> : null}
			{!loading && entries.length ? (
				<>
					{packageEntries.length ? (
						<section className="panel">
							<SectionHeading
								title="Package capabilities"
								detail="Feature-set keys already assigned on packages. Status is taken from Feature Manager, not invented."
							/>
							<ShowcaseTable entries={packageEntries} />
						</section>
					) : null}
					{hostEntries.length ? (
						<section className="panel">
							<SectionHeading
								title="Host applications"
								detail="Apps the Kelmor Agent reports on this host."
							/>
							<ShowcaseTable entries={hostEntries} />
						</section>
					) : null}
				</>
			) : null}
			{!loading && !entries.length && !error ? (
				<EmptyState
					title={FEATURE_SHOWCASE_EMPTY_TITLE}
					detail={FEATURE_SHOWCASE_EMPTY_DETAIL}
					action={<Link className="button-link" to="/features">{FEATURE_MANAGER_CTA}</Link>}
				/>
			) : null}
		</>
	)
}

function ShowcaseTable ({ entries }: { entries: ShowcaseEntry[] }) {
	return (
		<div className="table-wrap"><table className="dense-table">
			<thead>
				<tr>
					<th>Capability</th>
					<th>Status</th>
					<th>Description</th>
					<th>Open</th>
				</tr>
			</thead>
			<tbody>
				{entries.map((entry) => (
					<tr key={entry.id}>
						<td>
							<strong>{entry.name}</strong>
							<small>{entry.detail}</small>
						</td>
						<td><StatusBadge value={entry.group === 'host' ? entry.status : entry.enabled} /></td>
						<td>{entry.description}</td>
						<td>
							{entry.href ? <Link to={entry.href}>{entry.toolLabel || 'Open tool'}</Link> : '—'}
						</td>
					</tr>
				))}
			</tbody>
		</table></div>
	)
}
