import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import type { FeatureSet, Package } from '../types'

export function FeatureManagerPage () {
	const [featureSets, setFeatureSets] = useState<FeatureSet[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [selectedId, setSelectedId] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')

	function load () {
		setLoading(true)
		setError('')
		Promise.allSettled([
			api<{ items: FeatureSet[] }>('/api/v1/feature-sets'),
			api<{ items: Package[] }>('/api/v1/packages'),
		]).then(([featureResult, packageResult]) => {
			if (featureResult.status === 'fulfilled') {
				const next = asList(featureResult.value)
				setFeatureSets(next)
				setSelectedId((current) => current && next.some((entry) => entry.id === current) ? current : next[0]?.id || '')
			} else setError(messageFrom(featureResult.reason))
			if (packageResult.status === 'fulfilled') setPackages(asList(packageResult.value))
			setUpdatedAt(new Date().toISOString())
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])
	const selected = featureSets.find((entry) => entry.id === selectedId)
	const assigned = useMemo(() => packages.filter((pkg) => pkg.feature_set_id === selectedId), [packages, selectedId])
	const features = Object.entries(selected?.features || {}).sort(([left], [right]) => left.localeCompare(right))

	return (
		<>
			<PageHeader
				title="Feature Manager"
				description="Review package feature sets and which packages inherit them. Assign a set when you edit a package."
				actions={<Link className="button-link" to="/packages">Packages</Link>}
			/>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading feature sets…" /> : null}
			{!loading && selected ? <div className="state-columns">
				<section className="panel">
					<h2>Feature sets</h2>
					<div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Name</th><th>Features</th><th>Packages</th></tr></thead>
						<tbody>
							{featureSets.map((set) => {
								const count = packages.filter((pkg) => pkg.feature_set_id === set.id).length
								return (
									<tr key={set.id} className={set.id === selectedId ? 'task-focus' : undefined}>
										<td><button type="button" className="link-button" onClick={() => setSelectedId(set.id)}>{set.name}</button></td>
										<td>{Object.values(set.features || {}).filter(Boolean).length}</td>
										<td>{count}</td>
									</tr>
								)
							})}
						</tbody>
					</table></div>
				</section>
				<section className="panel">
					<h2>{selected.name}</h2>
					<p className="subtle">Enabled features are available to accounts whose package uses this set.</p>
					<ul className="feature-list">
						{features.map(([name, enabled]) => (
							<li key={name}><StatusBadge value={enabled} /> <span>{name}</span></li>
						))}
					</ul>
					<h3>Assigned packages</h3>
					{assigned.length ? <ul>{assigned.map((pkg) => <li key={pkg.id}><Link to="/packages">{pkg.name}</Link></li>)}</ul> : <p className="subtle">No package currently uses this feature set.</p>}
				</section>
			</div> : null}
			{!loading && !featureSets.length ? <EmptyState title="No feature sets" detail="The control plane has not seeded a feature catalog yet." /> : null}
		</>
	)
}
