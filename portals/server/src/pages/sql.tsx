import { Link } from 'react-router-dom'
import { listOf } from '../client'
import { useLoad } from '../lib/hooks'
import { DataTable, type Column } from '../components/data-table'
import { EmptyState, Notice, PageHeader, Pill, StatusPill } from '../components/ui'
import type { AccountRow } from '../components/account-picker'

interface DatabaseRow {
	id: string
	account_id: string
	engine: string
	name: string
	status: string
}

interface Row extends DatabaseRow {
	username: string
}

export function Databases () {
	const accounts = useLoad<AccountRow[]>(() => listOf<AccountRow>('/api/v1/accounts'), [])
	const list = accounts.data ?? []
	const key = list.map((a) => a.id).join(',')

	const databases = useLoad<Row[]>(async () => {
		const perAccount = await Promise.all(
			list.map(async (account) => {
				const rows = await listOf<DatabaseRow>(`/api/v1/accounts/${account.id}/databases`).catch(() => [])
				return rows.map((row) => ({ ...row, username: account.username }))
			}),
		)
		return perAccount.flat()
	}, [key])

	const rows = databases.data ?? []
	const engines = [...new Set(rows.map((r) => r.engine))]

	const columns: Column<Row>[] = [
		{ key: 'name', header: 'Database', sort: (r) => r.name, render: (r) => <span className="mono"><strong>{r.name}</strong></span> },
		{ key: 'engine', header: 'Engine', sort: (r) => r.engine, render: (r) => <Pill tone="idle">{r.engine}</Pill> },
		{
			key: 'account',
			header: 'Account',
			sort: (r) => r.username,
			render: (r) => <Link to={`/accounts/${r.account_id}?tab=databases`}>{r.username}</Link>,
		},
		{ key: 'status', header: 'Status', sort: (r) => r.status, render: (r) => <StatusPill status={r.status} /> },
	]

	return (
		<>
			<PageHeader
				title="Databases"
				description="Every MariaDB and PostgreSQL database Kelmor manages on this host, grouped by the account that owns it."
				favoritePath="/sql"
			/>
			<Notice tone="info">
				Databases are created against an account so the package database limit is enforced and the credentials stay inside
				that account. Create one from the account summary, or let a WordPress install create it. Kelmor does not expose a
				free-floating database that belongs to no account.
			</Notice>
			<DataTable
				rows={rows}
				columns={columns}
				rowKey={(r) => r.id}
				loading={accounts.loading || databases.loading}
				error={accounts.error || databases.error}
				searchPlaceholder="Search database, engine or account"
				noun="databases"
				initialSort={{ key: 'account', dir: 'asc' }}
				filters={engines.length > 1 ? <span className="small muted">{engines.join(' · ')}</span> : null}
				empty={
					<EmptyState icon="database" title="No database exists yet" action={<Link className="btn secondary" to="/accounts">Open List Accounts</Link>}>
						Databases appear here as soon as an account creates one, including the database a WordPress install creates
						for itself.
					</EmptyState>
				}
			/>
		</>
	)
}
