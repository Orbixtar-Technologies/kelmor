import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState } from '../components/ui'
import { messageFrom } from '../helpers'
import { HostConsolePanel } from './host-console-panel'

interface MailQueueRow {
	queue_id?: string
	size?: number
	sender?: string
	recipient?: string
	arrival?: string
	state?: string
}

interface MailQueueResponse {
	items?: MailQueueRow[]
	source?: string
	partial?: boolean
	message?: string
}

export function MailQueuePanel () {
	const [items, setItems] = useState<MailQueueRow[]>([])
	const [note, setNote] = useState('')
	const [partial, setPartial] = useState(false)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	const showQueue = useCallback(() => {
		setLoading(true)
		setError('')
		api<MailQueueResponse>('/api/v1/mail/queue').then((result) => {
			setItems(asList(result))
			setNote(result.message || '')
			setPartial(Boolean(result.partial))
		}).catch((requestError) => {
			setError(messageFrom(requestError))
			setItems([])
		}).finally(() => setLoading(false))
	}, [])

	useEffect(() => {
		showQueue()
	}, [showQueue])

	return (
		<>
			<section className="panel">
				<h2>Postfix queue</h2>
				<p>Show mail queue reads the live Postfix listing. Delivery history is Mail Delivery Reports; live watching is Track Delivery.</p>
				<p>
					<Link to="/mail/delivery-reports">Mail Delivery Reports</Link>
					{' · '}
					<Link to="/mail/track-delivery">Track Delivery</Link>
					{' · '}
					<Link to="/deliverability">Deliverability</Link>
				</p>
				<button type="button" onClick={showQueue} disabled={loading}>Show mail queue</button>
				{error ? <ErrorState error={error} onRetry={showQueue} /> : null}
				{loading ? <LoadingState label="Reading the Postfix queue…" /> : null}
				{!loading && items.length ? (
					<div className="table-wrap">
						<table className="dense-table">
							<thead>
								<tr>
									<th>Queue ID</th>
									<th>State</th>
									<th>Sender</th>
									<th>Recipient</th>
									<th>Size</th>
									<th>Arrival</th>
								</tr>
							</thead>
							<tbody>
								{items.map((item, index) => (
									<tr key={item.queue_id || String(index)}>
										<td><code>{item.queue_id || '—'}</code></td>
										<td>{item.state || '—'}</td>
										<td>{item.sender || '—'}</td>
										<td>{item.recipient || '—'}</td>
										<td>{item.size || '—'}</td>
										<td>{item.arrival || '—'}</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				) : null}
				{!loading && !error && !items.length ? (
					<EmptyState
						title="Mail queue is empty"
						detail={note || 'No deferred or active Postfix messages. Live listings need a privileged Agent running postqueue.'}
					/>
				) : null}
				{partial && note ? <p className="subtle" role="status">{note}</p> : null}
			</section>
			<HostConsolePanel recipeIds={['postfix-flush', 'postfix-status']} />
		</>
	)
}
