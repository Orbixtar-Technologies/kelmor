import { useMemo, useState } from 'react'
import { Icon } from './icons'
import { EmptyState, Loading, StatusPill } from './ui'

export interface AccountRow {
	id: string
	username: string
	primary_domain: string
	status: string
	package_id?: string
	reseller_id?: string
	linux_uid?: number
	ip_address?: string
	home_path?: string
	login_disabled?: boolean
}

export interface AccountPickerProps {
	accounts: AccountRow[]
	value: string
	onChange: (id: string) => void
	loading?: boolean
	label?: string
	/** Narrows the list, e.g. only active accounts for a suspend journey. */
	filter?: (account: AccountRow) => boolean
	emptyHint?: string
}

/**
 * Searchable account chooser used by every single-account operator journey, so
 * "pick the account, then act" looks the same everywhere in Director.
 */
export function AccountPicker ({ accounts, value, onChange, loading, label = 'Account', filter, emptyHint }: AccountPickerProps) {
	const [search, setSearch] = useState('')
	const pool = useMemo(() => (filter ? accounts.filter(filter) : accounts), [accounts, filter])
	const matches = useMemo(() => {
		const q = search.trim().toLowerCase()
		if (!q) return pool
		return pool.filter((a) => a.username.toLowerCase().includes(q) || a.primary_domain.toLowerCase().includes(q))
	}, [pool, search])

	if (loading) return <Loading label="Loading accounts" />
	if (pool.length === 0) {
		return (
			<EmptyState icon="users" title="No eligible accounts">
				{emptyHint ?? 'Create a hosting account first — this tool acts on an existing account.'}
			</EmptyState>
		)
	}

	return (
		<div className="field">
			<label htmlFor="account-picker">{label}</label>
			<div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
				<div style={{ position: 'relative', flex: '1 1 220px' }}>
					<input
						type="search"
						placeholder="Filter by username or domain"
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						aria-label="Filter accounts"
					/>
				</div>
				<select
					id="account-picker"
					value={value}
					onChange={(e) => onChange(e.target.value)}
					style={{ flex: '2 1 320px' }}
					size={matches.length > 6 ? 8 : undefined}
				>
					<option value="">Select an account…</option>
					{matches.map((account) => (
						<option key={account.id} value={account.id}>
							{account.username} — {account.primary_domain} ({account.status})
						</option>
					))}
				</select>
			</div>
			<span className="hint">
				<Icon name="users" size={12} /> {matches.length} of {pool.length} accounts shown
			</span>
		</div>
	)
}

export function AccountBadge ({ account }: { account: AccountRow }) {
	return (
		<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
			<strong>{account.username}</strong>
			<span className="muted small">{account.primary_domain}</span>
			<StatusPill status={account.status} />
		</span>
	)
}
