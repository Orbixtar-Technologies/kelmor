import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { Dialog, ErrorState, LoadingState } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import { firewallImpactLines, firewallPolicyPreview, firewallRollbackCopy } from './firewall-copy'

interface FirewallStatus {
	table?: string
	file?: string
	stack?: string
	csf_installed?: boolean
	label?: string
}

export function FirewallPanel () {
	const canInspect = useCan('server.firewall.read')
	const canApply = useCan('server.firewall.write')
	const [firewall, setFirewall] = useState<FirewallStatus | null>(null)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const [dialogOpen, setDialogOpen] = useState(false)
	const [confirmed, setConfirmed] = useState(false)
	const [phrase, setPhrase] = useState('')

	function closeDialog () {
		setDialogOpen(false)
		setConfirmed(false)
		setPhrase('')
	}

	function loadFirewall () {
		if (!canInspect) return
		setError('')
		api<FirewallStatus>('/api/v1/server/firewall').then((result) => {
			setFirewall(result)
			setUpdatedAt(new Date().toISOString())
		}).catch((requestError) => setError(messageFrom(requestError)))
	}

	useEffect(loadFirewall, [canInspect])

	async function applyFirewall () {
		if (phrase !== 'APPLY') return
		try {
			await api('/api/v1/server/firewall/apply', { method: 'POST', body: '{}' })
			setMessage('Firewall configuration applied.')
			closeDialog()
			loadFirewall()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<section className="panel">
			<h2>Host firewall</h2>
			<p>
				Kelmor manages inbound policy with nftables table <code>inet panel</code>.
				ConfigServer Firewall is not installed on this stack.
			</p>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{!canInspect ? <p className="subtle">Your role cannot inspect host firewall configuration.</p> : null}
			{error ? <ErrorState error={error} onRetry={loadFirewall} /> : null}
			{canInspect && !error && !firewall ? <LoadingState label="Loading firewall…" /> : null}
			{firewall ? (
				<dl className="detail-list">
					<div><dt>Stack</dt><dd>{firewall.stack || 'nftables'}</dd></div>
					<div><dt>Label</dt><dd>{firewall.label || 'Kelmor firewall'}</dd></div>
					<div><dt>Table</dt><dd><code>{firewall.table || 'inet panel'}</code></dd></div>
					<div><dt>Configuration</dt><dd><code>{firewall.file || '/etc/panel/nftables-panel.nft'}</code></dd></div>
					<div><dt>CSF installed</dt><dd>{firewall.csf_installed ? 'Yes' : 'No'}</dd></div>
				</dl>
			) : null}
			{canApply ? <button type="button" onClick={() => setDialogOpen(true)}>Review and apply</button> : canInspect ? <p className="subtle">Your role can inspect, but cannot apply firewall policy.</p> : null}
			<p><Link to="/security">Open Security &amp; Host Configuration</Link></p>
			<Dialog open={dialogOpen} title="Apply firewall configuration" onClose={closeDialog} actions={<>
				<button type="button" className="secondary" onClick={closeDialog}>Cancel</button>
				<button type="button" disabled={!confirmed || phrase !== 'APPLY'} onClick={applyFirewall}>Apply policy</button>
			</>}>
				<p>This replaces the live inbound policy with the Kelmor-managed <code>{firewall?.table || 'inet panel'}</code> table.</p>
				<ul>{firewallImpactLines().map((line) => <li key={line}>{line}</li>)}</ul>
				<pre>{firewallPolicyPreview()}</pre>
				<p className="subtle">{firewallRollbackCopy()}</p>
				<label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /> I reviewed the active management path</label>
				<label>Type APPLY to confirm<input value={phrase} onChange={(event) => setPhrase(event.target.value)} autoComplete="off" /></label>
			</Dialog>
		</section>
	)
}
