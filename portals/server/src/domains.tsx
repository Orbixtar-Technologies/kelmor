import { useEffect, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { api, asList } from './client'
import { Empty, PageHeader } from './ui'

interface AccountRow {
	id: string
	username: string
	primary_domain: string
	status: string
}

interface DomainRow {
	id: string
	account_id: string
	ascii_fqdn?: string
	fqdn?: string
	type: string
	status: string
}

export function Domains () {
	const [rows, setRows] = useState<Array<DomainRow & { username: string }>>([])
	const [err, setErr] = useState('')
	const [honest, setHonest] = useState('')
	const [q, setQ] = useState('')

	useEffect(() => {
		async function load () {
			const accs = asList(await api<{ items: AccountRow[] }>('/api/v1/accounts'))
			const out: Array<DomainRow & { username: string }> = []
			let walked = 0
			for (const acc of accs) {
				try {
					const ds = asList(await api<{ items: DomainRow[] }>(
						`/api/v1/accounts/${acc.id}/domains`,
					))
					walked++
					if (ds.length === 0) {
						out.push({
							id: acc.id + '-primary',
							account_id: acc.id,
							ascii_fqdn: acc.primary_domain,
							type: 'primary',
							status: acc.status,
							username: acc.username,
						})
						continue
					}
					for (const d of ds) {
						out.push({ ...d, username: acc.username })
					}
				} catch (e) {
					setHonest(e instanceof Error ? e.message : 'domain list failed')
				}
			}
			setRows(out)
			if (walked === accs.length) {
				setHonest('There is no host-wide DNS inventory endpoint. This table walks GET /accounts and GET /accounts/{id}/domains.')
			}
		}
		load().catch((e) => setErr(e.message))
	}, [])

	if (err) return <Empty title="DNS / Domains unavailable" detail={err} />
	const filtered = rows.filter((r) => {
		const hay = `${r.username} ${r.ascii_fqdn || r.fqdn || ''}`.toLowerCase()
		return hay.includes(q.trim().toLowerCase())
	})
	return (
		<>
			<PageHeader
				title="DNS / Domains"
				detail="Per-account domains from the control plane. Zone records stay on the account operations hub."
			/>
			<input
				className="search"
				placeholder="Filter username or domain"
				value={q}
				onChange={(e) => setQ(e.target.value)}
			/>
			{honest ? <p className="muted">{honest}</p> : null}
			{filtered.length === 0 ? (
				<Empty title="No domains yet" detail="Provision an account, then reload." />
			) : (
				<table>
					<thead>
						<tr>
							<th>Account</th>
							<th>Domain</th>
							<th>Type</th>
							<th>Status</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{filtered.map((r) => (
							<tr key={r.id}>
								<td>{r.username}</td>
								<td>{r.ascii_fqdn || r.fqdn}</td>
								<td>{r.type}</td>
								<td>{r.status}</td>
								<td>
									<NavLink to={`/accounts/${r.account_id}#dns`}>Open hub</NavLink>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	)
}
