import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { Dialog, ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import { firewallImpactLines, firewallPolicyPreview, firewallRollbackCopy, rebootImpactLines } from './firewall-copy'

interface Firewall {
	table: string
	file: string
}

export function SecurityPage () {
	const canInspectFirewall = useCan('server.firewall.read')
	const canFirewall = useCan('server.firewall.write')
	const canReboot = useCan('server.settings.write')
	const canReadAudit = useCan('security.audit.read')
	const [firewall, setFirewall] = useState<Firewall | null>(null)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const [firewallOpen, setFirewallOpen] = useState(false)
	const [firewallConfirmed, setFirewallConfirmed] = useState(false)
	const [firewallPhrase, setFirewallPhrase] = useState('')
	const [rebootOpen, setRebootOpen] = useState(false)
	const [confirmation, setConfirmation] = useState('')
	function closeFirewall () {
		setFirewallOpen(false)
		setFirewallConfirmed(false)
		setFirewallPhrase('')
	}
	function closeReboot () {
		setRebootOpen(false)
		setConfirmation('')
	}
	function loadFirewall () {
		if (!canInspectFirewall) return
		setError('')
		api<Firewall>('/api/v1/server/firewall').then((result) => {
			setFirewall(result)
			setUpdatedAt(new Date().toISOString())
		}).catch((requestError) => setError(messageFrom(requestError)))
	}
	useEffect(loadFirewall, [canInspectFirewall])
	async function applyFirewall () {
		if (firewallPhrase !== 'APPLY') return
		try {
			await api('/api/v1/server/firewall/apply', { method: 'POST', body: '{}' })
			setMessage('Firewall configuration applied.')
			closeFirewall()
		} catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	async function reboot () {
		if (confirmation !== 'REBOOT') return
		try { await api('/api/v1/server/reboot', { method: 'POST', body: JSON.stringify({ confirm: 'REBOOT' }) }); setMessage('Host reboot was recorded and sent to the privileged agent.'); closeReboot() } catch (requestError) { setMessage(messageFrom(requestError)) }
	}
	return <>
		<PageHeader title="Security & Host Configuration" description="Review privileged controls before applying changes to the host." actions={canReadAudit ? <Link className="button-link secondary-link" to="/audit">Open audit trail</Link> : undefined} />
		{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
		{message ? <p className="feedback" role="status">{message}</p> : null}
		<div className="summary-grid">
			<section className="panel"><SectionHeading title="Firewall" detail="Kelmor-managed inbound policy and hosting service allowances." />{!canInspectFirewall ? <p className="subtle">Your role cannot inspect host firewall configuration.</p> : error ? <ErrorState error={error} onRetry={loadFirewall} /> : firewall ? <dl className="detail-list"><div><dt>Table</dt><dd><code>{firewall.table}</code></dd></div><div><dt>Configuration</dt><dd><code>{firewall.file}</code></dd></div></dl> : <LoadingState />}{canFirewall ? <button type="button" onClick={() => setFirewallOpen(true)}>Review and apply</button> : canInspectFirewall ? <p className="subtle">Your role can inspect, but cannot apply firewall policy.</p> : null}</section>
			<section className="panel danger-panel"><SectionHeading title="Host reboot" detail="Active sites and sessions will disconnect until services return." />{canReboot ? <button type="button" className="danger" onClick={() => setRebootOpen(true)}>Request reboot</button> : <p className="subtle">Your role cannot reboot this host.</p>}</section>
			{canReadAudit ? <section className="panel"><SectionHeading title="Security audit" detail="Review successful and rejected privileged activity." /><p>Filter by action, outcome, actor, date, and resource.</p><Link to="/audit">Search audit events →</Link></section> : null}
		</div>
		<Dialog open={firewallOpen} title="Apply firewall configuration" onClose={closeFirewall} actions={<>
			<button type="button" className="secondary" onClick={closeFirewall}>Cancel</button>
			<button type="button" disabled={!firewallConfirmed || firewallPhrase !== 'APPLY'} onClick={applyFirewall}>Apply policy</button>
		</>}>
			<p>This replaces the live inbound policy with the Kelmor-managed <code>{firewall?.table}</code> table.</p>
			<ul>{firewallImpactLines().map((line) => <li key={line}>{line}</li>)}</ul>
			<pre>{firewallPolicyPreview()}</pre>
			<p className="subtle">{firewallRollbackCopy()}</p>
			<label className="checkbox-label"><input type="checkbox" checked={firewallConfirmed} onChange={(event) => setFirewallConfirmed(event.target.checked)} /> I reviewed the active management path</label>
			<label>Type APPLY to confirm<input value={firewallPhrase} onChange={(event) => setFirewallPhrase(event.target.value)} autoComplete="off" /></label>
		</Dialog>
		<Dialog open={rebootOpen} title="Reboot host" onClose={closeReboot} actions={<>
			<button type="button" className="secondary" onClick={closeReboot}>Cancel</button>
			<button type="button" className="danger" disabled={confirmation !== 'REBOOT'} onClick={reboot}>Reboot host</button>
		</>}>
			<ul>{rebootImpactLines().map((line) => <li key={line}>{line}</li>)}</ul>
			<label>Type REBOOT to confirm<input autoFocus value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" /></label>
		</Dialog>
	</>
}
