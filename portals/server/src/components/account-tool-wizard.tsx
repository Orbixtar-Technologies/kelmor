import type { ReactNode } from 'react'
import { AccountPicker } from './account-picker'
import { EmptyState, ErrorState, LoadingState, PageHeader } from './ui'
import type { Account } from '../types'

interface AccountToolWizardProps {
	title: string
	description: string
	accounts: Account[]
	selectedId: string
	onSelect: (accountId: string) => void
	filter: string
	onFilterChange: (query: string) => void
	loading: boolean
	error: string
	onRetry: () => void
	emptyDetail?: string
	children: (account: Account) => ReactNode
}

export function AccountToolWizard ({
	title,
	description,
	accounts,
	selectedId,
	onSelect,
	filter,
	onFilterChange,
	loading,
	error,
	onRetry,
	emptyDetail = 'Choose an account to continue this tool.',
	children,
}: AccountToolWizardProps) {
	const account = accounts.find((entry) => entry.id === selectedId)

	return (
		<>
			<PageHeader title={title} description={description} />
			{error ? <ErrorState error={error} onRetry={onRetry} /> : null}
			{loading ? <LoadingState label="Loading accounts…" /> : (
				<section className="panel" aria-label="Account picker">
					<h2>Select an account</h2>
					<p className="subtle">Search and choose the tenant for this tool. This is not the List Accounts inventory.</p>
					<AccountPicker
						accounts={accounts}
						value={selectedId}
						filter={filter}
						onFilterChange={onFilterChange}
						onChange={onSelect}
					/>
				</section>
			)}
			{!loading && !error && selectedId && !account ? (
				<EmptyState title="Account not in this list" detail="The selected account is missing or filtered out. Choose another tenant." />
			) : null}
			{!loading && !error && !selectedId ? <EmptyState title="No account selected" detail={emptyDetail} /> : null}
			{account ? children(account) : null}
		</>
	)
}
