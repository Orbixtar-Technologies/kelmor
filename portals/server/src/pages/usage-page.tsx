import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from '../components/ui'
import { formatBytes, formatDate, messageFrom, percent } from '../helpers'
import type { Account, Package, Usage } from '../types'

interface MonitorResponse { accounts?: Usage[]; failed_jobs?: number; certs_expiring?: number }

export function UsagePage () {
	const [accounts, setAccounts] = useState<Account[]>([])
	const [packages, setPackages] = useState<Package[]>([])
	const [usage, setUsage] = useState<Usage[]>([])
	const [query, setQuery] = useState('')
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	async function load () {
		setLoading(true)
		setError('')
		try {
			const [accountResult, packageResult] = await Promise.all([api<{ items: Account[] }>('/api/v1/accounts'), api<{ items: Package[] }>('/api/v1/packages')])
			const nextAccounts = asList(accountResult)
			setAccounts(nextAccounts)
			setPackages(asList(packageResult))
			try {
				const usageResult = await api<MonitorResponse>('/api/v1/server/monitor')
				setUsage(usageResult.accounts || [])
			} catch {
				const usageResults = await Promise.allSettled(nextAccounts.map((account) => api<Usage>(`/api/v1/accounts/${account.id}/usage`)))
				setUsage(usageResults.flatMap((result) => result.status === 'fulfilled' ? [result.value] : []))
			}
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setLoading(false)
		}
	}
	useEffect(() => { void load() }, [])
	const rows = useMemo(() => accounts.map((account) => ({ account, pkg: packages.find((pkg) => pkg.id === account.package_id), usage: usage.find((entry) => entry.account_id === account.id) })).filter((row) => `${row.account.username} ${row.account.primary_domain}`.toLocaleLowerCase().includes(query.toLocaleLowerCase())), [accounts, packages, usage, query])
	return <>
		<PageHeader title="Account Usage" description="Compare measured account consumption with package capacity." actions={<button type="button" className="secondary" onClick={load}>Refresh</button>} />
		<div className="filter-bar"><label>Search accounts<input type="search" value={query} onChange={(event) => setQuery(event.target.value)} /></label></div>
		{error ? <ErrorState error={error} onRetry={load} /> : null}{loading ? <LoadingState label="Collecting account usage…" /> : null}
		{!loading && <div className="table-wrap"><table className="dense-table"><thead><tr><th>Account</th><th>Status</th><th>Package</th><th>Disk</th><th>Bandwidth</th><th>Memory</th><th>CPU</th><th>Inodes</th><th>Processes</th><th>Collected</th></tr></thead><tbody>{rows.map(({ account, pkg, usage: row }) => {
			const disk = percent(row?.disk_bytes, pkg?.disk_bytes); const bandwidth = percent(row?.bandwidth_bytes, pkg?.bandwidth_bytes_monthly)
			return <tr key={account.id}><td><Link to={`/accounts/${account.id}`}>{account.username}</Link><small>{account.primary_domain}</small></td><td><StatusBadge value={account.status} /></td><td>{pkg?.name || '—'}</td><td className={disk > 100 ? 'danger-text' : ''}>{row ? `${formatBytes(row.disk_bytes)} (${disk}%)` : '—'}</td><td className={bandwidth > 100 ? 'danger-text' : ''}>{row ? `${formatBytes(row.bandwidth_bytes)} (${bandwidth}%)` : '—'}</td><td>{row ? formatBytes(row.memory_bytes) : '—'}</td><td>{row ? `${row.cpu_percent}%` : '—'}</td><td>{row?.inode_count ?? '—'}</td><td>{row?.process_count ?? '—'}</td><td>{formatDate(row?.collected_at)}</td></tr>
		})}</tbody></table></div>}
		{!loading && !rows.length ? <EmptyState title="No account usage matches" detail="Filters remain available. Usage appears after host collection runs." /> : null}
	</>
}
