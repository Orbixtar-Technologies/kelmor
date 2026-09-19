import { useState } from 'react'
import { Link } from 'react-router-dom'
import { download } from '../client'
import { ErrorState } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

export function DiagnosticsPanel () {
	const canRead = useCan('server.read')
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [busy, setBusy] = useState(false)

	async function handleDownload () {
		setBusy(true)
		setError('')
		setMessage('')
		try {
			await download('/api/v1/server/diagnostics', 'kelmor-diagnostics.tar.gz')
			setMessage('Diagnostics archive downloaded.')
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<h2>Diagnostics bundle</h2>
			<p>Download an authenticated archive of host config snapshots, recent jobs, audit events, and allowlisted logs. Secrets are redacted.</p>
			<p><Link to="/jobs">Jobs</Link> · <Link to="/audit">Audit Trail</Link></p>
			{error ? <ErrorState error={error} /> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<button type="button" disabled={!canRead || busy} onClick={() => { void handleDownload() }}>
				{busy ? 'Preparing…' : 'Download diagnostics archive'}
			</button>
		</section>
	)
}
