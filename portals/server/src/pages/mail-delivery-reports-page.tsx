import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import {
	MONTH_OPTIONS,
	defaultReportDate,
	deliveryReportsApiPath,
	deliveryReportsHref,
	deliveryStatusBadge,
	formatDeliveryStatus,
	parseReportDate,
	trackDeliveryHref,
	type MailDeliveryAttempt,
	type MailDeliveryReport,
} from './mail-delivery-copy'

export function MailDeliveryReportsPage () {
	const [params, setParams] = useSearchParams()
	const initial = useMemo(() => {
		const parsed = parseReportDate({
			year: params.get('year') || '',
			month: params.get('month') || '',
			day: params.get('day') || '',
		})
		return 'date' in parsed ? parsed.date : defaultReportDate()
	}, [params])
	const [year, setYear] = useState(String(initial.year))
	const [month, setMonth] = useState(String(initial.month))
	const [day, setDay] = useState(String(initial.day))
	const [query, setQuery] = useState(params.get('q') || '')
	const [report, setReport] = useState<MailDeliveryReport | null>(null)
	const [selected, setSelected] = useState<MailDeliveryAttempt | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const ran = Boolean(params.get('year') && params.get('month') && params.get('day'))

	useEffect(() => {
		if (!ran) return
		const parsed = parseReportDate({
			year: params.get('year') || '',
			month: params.get('month') || '',
			day: params.get('day') || '',
		})
		if ('error' in parsed) {
			setError(parsed.error)
			setReport(null)
			return
		}
		setLoading(true)
		setError('')
		api<MailDeliveryReport>(deliveryReportsApiPath(parsed.date, params.get('q') || '')).then((next) => {
			setReport(next)
			setSelected(null)
		}).catch((requestError) => {
			setError(messageFrom(requestError))
			setReport(null)
		}).finally(() => setLoading(false))
	}, [params, ran])

	function handleSubmit (event: FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const parsed = parseReportDate({ year, month, day })
		if ('error' in parsed) {
			setError(parsed.error)
			return
		}
		const next = new URLSearchParams({
			year: String(parsed.date.year),
			month: String(parsed.date.month),
			day: String(parsed.date.day),
		})
		if (query.trim()) next.set('q', query.trim())
		setParams(next)
	}

	const items = report?.items || []

	return (
		<>
			<PageHeader
				title="Mail Delivery Reports"
				description="Find messages sent from and received by this host for one calendar day. Choose month, day, and year, then review whether each attempt delivered."
				actions={<Link className="button-link secondary-link" to={trackDeliveryHref(query)}>Track Delivery</Link>}
			/>
			<section className="panel">
				<h2>Run a delivery report</h2>
				<p>Classic WHM Mail Delivery Reports uses a calendar day. Kelmor reads Postfix syslog at <code>/var/log/mail.log</code>.</p>
				<form className="stack-form" onSubmit={handleSubmit}>
					<div className="form-grid">
						<label>Month
							<select name="month" value={month} onChange={(event) => setMonth(event.target.value)} required>
								{MONTH_OPTIONS.map((entry) => (
									<option key={entry.value} value={entry.value}>{entry.label}</option>
								))}
							</select>
						</label>
						<label>Day of month
							<select name="day" value={day} onChange={(event) => setDay(event.target.value)} required>
								{Array.from({ length: 31 }, (_, index) => index + 1).map((value) => (
									<option key={value} value={value}>{value}</option>
								))}
							</select>
						</label>
						<label>Year
							<input name="year" type="number" min={1970} max={2100} value={year} onChange={(event) => setYear(event.target.value)} required />
						</label>
					</div>
					<label>Recipient or sender (optional)
						<input name="q" type="search" value={query} placeholder="user@example.com" onChange={(event) => setQuery(event.target.value)} />
					</label>
					<button type="submit">Run report</button>
				</form>
			</section>
			{error ? <ErrorState error={error} /> : null}
			{loading ? <LoadingState label="Reading the host mail log…" /> : null}
			{!loading && ran && report ? (
				<section className="panel">
					<h2>Delivery attempts for {report.date}</h2>
					{report.message ? <p className="subtle" role="status">{report.message}</p> : null}
					{report.source ? <p className="subtle">Source: {report.source}{report.partial ? ' · partial' : ''}.</p> : null}
					{items.length ? (
						<div className="table-wrap"><table className="dense-table">
							<thead><tr><th>Time</th><th>Sender</th><th>Recipient</th><th>Status</th><th>Queue</th><th></th></tr></thead>
							<tbody>
								{items.map((item) => (
									<tr key={item.id}>
										<td>{formatDate(item.timestamp)}</td>
										<td>{item.sender || '—'}</td>
										<td>{item.recipient || '—'}</td>
										<td><StatusBadge value={deliveryStatusBadge(item.status)} /> {formatDeliveryStatus(item.status)}</td>
										<td><code>{item.queue_id}</code></td>
										<td><button type="button" className="link-button" onClick={() => setSelected(item)}>Details</button></td>
									</tr>
								))}
							</tbody>
						</table></div>
					) : <EmptyState title="No delivery attempts for that day" detail="The mail log had no matching Postfix lines. Try another date or clear the recipient filter." />}
					{selected ? (
						<article className="review-callout" aria-label="Delivery attempt details">
							<strong>{formatDeliveryStatus(selected.status)} · {selected.recipient || selected.sender || selected.queue_id}</strong>
							<p>Queue {selected.queue_id} · {selected.direction} · {formatDate(selected.timestamp)}</p>
							{selected.relay ? <p>Relay: {selected.relay}</p> : null}
							{selected.dsn ? <p>DSN: {selected.dsn}</p> : null}
							<p>{selected.message || 'No additional attempt text.'}</p>
							<p><Link to={trackDeliveryHref(selected.recipient || selected.sender || '')}>Track this address</Link></p>
						</article>
					) : null}
				</section>
			) : null}
			{!ran && !loading ? <EmptyState title="Choose a date to run the report" detail="Select month, day of month, and year, then run the delivery report. This is not the Jobs list." /> : null}
			<p className="subtle">Need live queue watching? Open <Link to={trackDeliveryHref()}>Track Delivery</Link>. Background mail jobs stay on <Link to="/jobs">Jobs</Link>.</p>
		</>
	)
}

export { deliveryReportsHref }
