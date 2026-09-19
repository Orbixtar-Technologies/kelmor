import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AccountToolWizard } from '../components/account-tool-wizard'
import { DATABASE_USER_MODEL } from '../catalog-honesty'
import { api, asList } from '../client'
import type { Account } from '../types'
import { messageFrom } from '../helpers'

interface SqlToolPageProps {
	toolId: 'password' | 'processes'
}

export function SqlToolPage ({ toolId }: SqlToolPageProps) {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [filter, setFilter] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const selectedId = params.get('account') || ''

	function load () {
		setLoading(true)
		setError('')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			setAccounts(asList(result).filter((account) => account.status !== 'terminated'))
		}).catch((requestError) => setError(messageFrom(requestError))).finally(() => setLoading(false))
	}
	useEffect(load, [])

	const isPassword = toolId === 'password'
	return (
		<AccountToolWizard
			title={isPassword ? 'Change Database User Password' : 'Show MySQL Processes'}
			description={isPassword
				? 'Kelmor auto-provisions one database user per engine. There is no rotate-user API; credentials stay in the account-owned file.'
				: 'Process lists stay on the SQL host. Director does not expose MariaDB THREADS or KILL.'}
			accounts={accounts}
			selectedId={selectedId}
			onSelect={(accountId) => {
				const next = new URLSearchParams(params)
				if (accountId) next.set('account', accountId)
				else next.delete('account')
				setParams(next, { replace: true })
			}}
			filter={filter}
			onFilterChange={setFilter}
			loading={loading}
			error={error}
			onRetry={load}
			emptyDetail={isPassword ? 'Select the account whose database credentials you want to review.' : 'Select the account whose SQL host you want to inspect.'}
		>
			{(account) => (
				<section className="panel">
					<h2>{isPassword ? `Database user for ${account.username}` : `SQL processes for ${account.username}`}</h2>
					<p className="subtle">{DATABASE_USER_MODEL}</p>
					{isPassword
						? <p>The auto-provisioned user password is stored on the host for this account. Director does not offer a rotate-user PATCH. Open Database Manager to review connection details.</p>
						: <p>Show/kill MariaDB threads is not a Kelmor API. Use the SQL host processlist, or open Database Manager for connection details.</p>}
					<Link className="button-link" to={`/sql?account=${account.id}`}>Open Database Manager</Link>
				</section>
			)}
		</AccountToolWizard>
	)
}
