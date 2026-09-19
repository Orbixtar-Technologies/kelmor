import { FormEvent, useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import {
	deliveryReportsHref,
	deliveryStatusBadge,
	formatDeliveryStatus,
	trackDeliveryApiPath,
	type MailDeliveryReport,
} from './mail-delivery-copy'

const REFRESH_MS = 10000

export function TrackDeliveryPage () {
	const [params, setParams] = useSearchParams()
	const query = params.get('q') || ''
	const [draft, setDraft] = useState(query)
	const [report, setReport] = useState<MailDeliveryReport | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	const load = useCallback(() => {
		setLoading(true)
		setError('')
		api<MailDeliveryReport>(trackDeliveryApiPath(query)).then((next) => {
			setReport(next)
		}).catch((requestError) => {
			setError(messageFrom(requestError))
			setReport(null)
		}).finally(() => setLoading(false))
	}, [query])

	useEffect(() => {
		setDraft(query)
		load()
	}, [load, query])

	useEffect(() => {
		const timer = window.setInterval(load, REFRESH_MS)
		return () => window.clearInterval(timer)
	}, [load])

	function handleSubmit (event: FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const next = new URLSearchParams()
		if (draft.trim()) next.set('q', draft.trim())
		setParams(next)
	}

	const items = report?.items || []
	const queue = report?.queue || []

	return (
		<>
			<PageHeader
				title="Track Delivery"
				description="Watch mail currently moving through this host: the live Postfix queue plus recent delivery attempts to or from an address."
				actions={<Link className="button-link secondary-link" to={deliveryReportsHref({
					year: new Date().getUTCFullYear(),
					month: new Date().getUTCMonth() + 1,
					day: new Date().getUTCDate(),
				}, draft)}>Mail Delivery Reports</Link>}
			/>
			<section className="panel">
				<h2>Track an address</h2>
				<p>Enter a recipient or sender. Kelmor refreshes the live queue and recent <code>/var/log/mail.log</code> lines every 10 seconds.</p>
				<form className="stack-form" onSubmit={handleSubmit}>
					<label>Recipient or sender
						<input name="q" type="search" value={draft} placeholder="user@example.com" onChange={(event) => setDraft(event.target.value)} />
					</label>
					<div className="button-row">
						<button type="submit">Track delivery</button>
						<button type="button" className="secondary" onClick={load}>Refresh now</button>
					</div>
				</form>
			</section>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading && !report ? <LoadingState label="Tracking host mail delivery…" /> : null}
			<section className="panel">
				<h2>Live queue</h2>
				{report?.message ? <p className="subtle" role="status">{report.message}</p> : null}
				{queue.length ? (
					<div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Queue</th><th>Sender</th><th>Recipient</th><th>Arrival</th><th>State</th></tr></thead>
						<tbody>
							{queue.map((item) => (
								<tr key={`${item.queue_id}:${item.recipient || item.sender}`}>
									<td><code>{item.queue_id}</code></td>
									<td>{item.sender || '—'}</td>
									<td>{item.recipient || '—'}</td>
									<td>{item.arrival || '—'}</td>
									<td><StatusBadge value="pending" /> {item.state}</td>
								</tr>
							))}
						</tbody>
					</table></div>
				) : <EmptyState title="No queued messages" detail={query ? 'Nothing in the live Postfix queue matches that address.' : 'The live Postfix queue is empty, or the Agent is not live on this host.'} />}
			</section>
			<section className="panel">
				<h2>Recent delivery attempts</h2>
				{loading ? <p className="subtle">Refreshing…</p> : null}
				{items.length ? (
					<div className="table-wrap"><table className="dense-table">
						<thead><tr><th>Time</th><th>Sender</th><th>Recipient</th><th>Status</th><th>Detail</th></tr></thead>
						<tbody>
							{items.map((item) => (
								<tr key={item.id}>
									<td>{formatDate(item.timestamp)}</td>
									<td>{item.sender || '—'}</td>
									<td>{item.recipient || '—'}</td>
									<td><StatusBadge value={deliveryStatusBadge(item.status)} /> {formatDeliveryStatus(item.status)}</td>
									<td>{item.message || item.relay || '—'}</td>
								</tr>
							))}
						</tbody>
					</table></div>
				) : <EmptyState title="No recent attempts" detail="Recent Postfix log lines will appear here after mail is accepted or delivered." />}
			</section>
			<p className="subtle">For a calendar-day history use <Link to={deliveryReportsHref({
				year: new Date().getUTCFullYear(),
				month: new Date().getUTCMonth() + 1,
				day: new Date().getUTCDate(),
			}, query)}>Mail Delivery Reports</Link>. This page is not the Jobs list.</p>
		</>
	)
}
