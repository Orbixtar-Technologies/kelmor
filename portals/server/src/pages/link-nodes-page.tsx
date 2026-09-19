import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { ErrorState } from '../components/ui'
import { messageFrom } from '../helpers'
import { featureById } from '../whm-catalog'
import { WhmToolBody } from './whm-tool-page'

interface LinkedNode {
	url: string
	source: string
}

export function LinkNodesPage () {
	const [nodes, setNodes] = useState<LinkedNode[]>([])
	const [error, setError] = useState('')

	function load () {
		api<{ items: LinkedNode[] }>('/api/v1/server/nodes')
			.then((result) => setNodes(asList(result)))
			.catch((reason) => setError(messageFrom(reason)))
	}
	useEffect(load, [])

	return (
		<>
			<p className="subtle">
				Registered peer URLs are written to <code>/etc/panel/linked-nodes.json</code> and also update configuration-cluster membership.
				<Link to="/section/server?tool=configuration-cluster"> Open Configuration Cluster</Link>
			</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			<section className="panel">
				<h2>Registered nodes</h2>
				{nodes.length ? (
					<ul>
						{nodes.map((node) => (
							<li key={`${node.source}:${node.url}`}><code>{node.url}</code> · {node.source}</li>
						))}
					</ul>
				) : <p>No linked nodes or cluster peers yet.</p>}
			</section>
			<WhmToolBody feature={featureById('link-nodes')!} />
		</>
	)
}
