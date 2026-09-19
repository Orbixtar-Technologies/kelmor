import { FormEvent, useEffect, useState } from 'react'
import { api, asList } from '../client'
import { ErrorState, LoadingState } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface SupportGrant {
	id: string
	ticket: string
	expires_at: string
	username?: string
}

interface IssuedGrant {
	id: string
	token: string
	username: string
	password: string
	expires_at: string
	ticket: string
	role: string
}

export function SupportAccessPanel () {
	const canWrite = useCan('server.settings.write')
	const [ticket, setTicket] = useState('')
	const [hours, setHours] = useState('24')
	const [items, setItems] = useState<SupportGrant[]>([])
	const [issued, setIssued] = useState<IssuedGrant | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [busy, setBusy] = useState(false)

	function load () {
		if (!canWrite) {
			setLoading(false)
			return
		}
		setLoading(true)
		api<{ items?: SupportGrant[] }>('/api/v1/server/support-access')
			.then((result) => setItems(asList(result)))
			.catch((reason) => setError(messageFrom(reason)))
			.finally(() => setLoading(false))
	}

	useEffect(load, [canWrite])

	async function handleGrant (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setIssued(null)
		try {
			const result = await api<IssuedGrant>('/api/v1/server/support-access', {
				method: 'POST',
				body: JSON.stringify({ ticket: ticket.trim(), hours: Number(hours) || 24 }),
			})
			setIssued(result)
			setMessage('Support session issued. Copy the token and password now; they are not shown again.')
			setTicket('')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	async function handleRevoke (grantId: string) {
		setError('')
		try {
			await api(`/api/v1/server/support-access/${grantId}`, { method: 'DELETE' })
			setMessage('Support session revoked.')
			if (issued?.id === grantId) setIssued(null)
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	return (
		<section className="panel">
			<h2>Grant support access</h2>
			<p>Issues a real Director session for the reserved <code>kelmor-support</code> user with the <code>server_operator</code> privilege set. That role can read the host and restart services. It cannot change settings or grant further access.</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{canWrite ? (
				<form className="form-grid" onSubmit={handleGrant}>
					<label>Ticket id
						<input value={ticket} onChange={(event) => setTicket(event.target.value)} required maxLength={64} />
					</label>
					<label>Hours valid
						<input type="number" min={1} max={168} value={hours} onChange={(event) => setHours(event.target.value)} />
					</label>
					<button type="submit" disabled={busy || !ticket.trim()}>{busy ? 'Issuing…' : 'Issue support session'}</button>
				</form>
			) : <p>You need server.settings.write to issue support access.</p>}
			{issued ? (
				<dl className="detail-list">
					<div><dt>Username</dt><dd><code>{issued.username}</code></dd></div>
					<div><dt>Password</dt><dd><code>{issued.password}</code></dd></div>
					<div><dt>Session token</dt><dd><code>{issued.token}</code></dd></div>
					<div><dt>Role</dt><dd>{issued.role}</dd></div>
					<div><dt>Expires</dt><dd>{issued.expires_at}</dd></div>
				</dl>
			) : null}
			{loading ? <LoadingState label="Loading support sessions…" /> : null}
			{!loading && items.length ? (
				<div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Ticket</th><th>Expires</th><th>Actions</th></tr></thead>
					<tbody>
						{items.map((grant) => (
							<tr key={grant.id}>
								<td>{grant.ticket}</td>
								<td>{grant.expires_at}</td>
								<td>
									<button type="button" className="link-button" onClick={() => handleRevoke(grant.id)}>Revoke</button>
								</td>
							</tr>
						))}
					</tbody>
				</table></div>
			) : null}
		</section>
	)
}
