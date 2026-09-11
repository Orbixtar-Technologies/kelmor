import { Link } from 'react-router-dom'
import type { Account } from '../types'

interface AccountScopeBarProps {
	accountId: string
	accounts: Account[]
	onChange: (accountId: string) => void
	toolLabel: string
}

export function AccountScopeBar ({ accountId, accounts, onChange, toolLabel }: AccountScopeBarProps) {
	const account = accounts.find((entry) => entry.id === accountId)
	if (!accountId) return null
	const scopeLabel = account ? `${account.username} · ${account.primary_domain}` : 'this account'
	return (
		<div className="context-chip" role="status">
			<p className="context-chip-copy">{toolLabel} for {scopeLabel}.</p>
			<div className="context-chip-actions">
				<label className="scope-change">
					<span>Change account</span>
					<select value={accountId} onChange={(event) => onChange(event.target.value)} aria-label="Change account">
						{accounts.map((entry) => <option key={entry.id} value={entry.id}>{entry.username}</option>)}
					</select>
				</label>
				<Link className="context-chip-link" to={`/accounts/${accountId}`}>Return to account</Link>
			</div>
		</div>
	)
}
