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
	return (
		<p className="context-chip">
			<span>{toolLabel} for {account ? `${account.username} · ${account.primary_domain}` : 'this account'}.</span>
			<label className="scope-change">Change account
				<select value={accountId} onChange={(event) => onChange(event.target.value)}>
					{accounts.map((entry) => <option key={entry.id} value={entry.id}>{entry.username}</option>)}
				</select>
			</label>
			<Link to={`/accounts/${accountId}`}>Return to account</Link>
		</p>
	)
}
