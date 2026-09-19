import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { PageHeader } from '../components/ui'
import { messageFrom } from '../helpers'
import { rebootImpactLines } from './firewall-copy'
import { useCan } from '../rbac'

interface SecurityToolPageProps {
	toolId: 'advisor' | 'reboot' | 'force-reboot'
}

const advisorChecks = [
	{ title: 'Host firewall', detail: 'Review and apply the Kelmor-managed inbound table.', href: '/security', label: 'Open Security' },
	{ title: 'Host access CIDRs', detail: 'Allow or deny Director by source address.', href: '/section/security?tool=host-access', label: 'Host Access Control' },
	{ title: 'Brute-force lockout', detail: 'Failed login thresholds for Director and Control.', href: '/section/security?tool=cphulk', label: 'Brute Force Protection' },
	{ title: 'Password policy', detail: 'Minimum complexity for account and mailbox passwords.', href: '/section/security?tool=password-strength', label: 'Password Strength' },
	{ title: 'Audit trail', detail: 'Search privileged actions after a change.', href: '/audit', label: 'Open Audit' },
]

export function SecurityToolPage ({ toolId }: SecurityToolPageProps) {
	if (toolId === 'advisor') {
		return (
			<>
				<PageHeader
					title="Security Advisor"
					description="Hardening checklist with links to the live tools. Kelmor does not score a WHM-style advisor report."
				/>
				<section className="panel">
					<h2>Checks</h2>
					<ul className="link-list">
						{advisorChecks.map((check) => (
							<li key={check.href}>
								<strong>{check.title}</strong>
								<p>{check.detail}</p>
								<Link to={check.href}>{check.label}</Link>
							</li>
						))}
					</ul>
				</section>
			</>
		)
	}
	return <RebootTool forceful={toolId === 'force-reboot'} />
}

function RebootTool ({ forceful }: { forceful: boolean }) {
	const canReboot = useCan('server.settings.write')
	const [confirmation, setConfirmation] = useState('')
	const [message, setMessage] = useState('')
	async function reboot () {
		if (confirmation !== 'REBOOT') return
		try {
			await api('/api/v1/server/reboot', { method: 'POST', body: JSON.stringify({ confirm: 'REBOOT' }) })
			setMessage('Host reboot was recorded and sent to the privileged agent.')
			setConfirmation('')
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}
	return (
		<>
			<PageHeader
				title={forceful ? 'Forceful Server Reboot' : 'Graceful Server Reboot'}
				description={forceful
					? 'Kelmor has one typed reboot path through the privileged agent. There is no harder reboot.'
					: 'Type REBOOT to drain services and restart the host through the privileged agent.'}
			/>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel danger-panel">
				<h2>Confirm reboot</h2>
				<ul>{rebootImpactLines().map((line) => <li key={line}>{line}</li>)}</ul>
				{forceful ? <p className="subtle">Forceful and graceful use the same agent reboot. Kelmor does not expose an immediate unclean reboot.</p> : null}
				{canReboot ? (
					<form onSubmit={(event) => { event.preventDefault(); void reboot() }}>
						<label>Type REBOOT to confirm<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" autoFocus /></label>
						<button type="submit" className="danger" disabled={confirmation !== 'REBOOT'}>Reboot host</button>
					</form>
				) : <p className="subtle">Your role cannot reboot this host.</p>}
				<p><Link to="/security">Open Security & Host Configuration</Link></p>
			</section>
		</>
	)
}
