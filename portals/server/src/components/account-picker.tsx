import { useMemo } from 'react'
import type { Account } from '../types'

interface AccountPickerProps {
	accounts: Account[]
	value: string
	onChange: (accountId: string) => void
	label?: string
	filter?: string
	onFilterChange?: (query: string) => void
}

export function AccountPicker ({
	accounts,
	value,
	onChange,
	label = 'Account',
	filter = '',
	onFilterChange,
}: AccountPickerProps) {
	const filtered = useMemo(() => {
		const query = filter.trim().toLocaleLowerCase()
		if (!query) return accounts
		return accounts.filter((account) =>
			account.username.toLocaleLowerCase().includes(query)
			|| account.primary_domain.toLocaleLowerCase().includes(query),
		)
	}, [accounts, filter])

	return (
		<div className="account-picker">
			{onFilterChange ? (
				<label>
					Search accounts
					<input
						value={filter}
						placeholder="Username or domain"
						onChange={(event) => onFilterChange(event.target.value)}
					/>
				</label>
			) : null}
			<label>
				{label}
				<select value={value} onChange={(event) => onChange(event.target.value)}>
					<option value="">Select an account…</option>
					{filtered.map((account) => (
						<option key={account.id} value={account.id}>
							{account.username} · {account.primary_domain}
						</option>
					))}
				</select>
			</label>
		</div>
	)
}
