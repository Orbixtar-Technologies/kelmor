import { useEffect, useRef, useState } from 'react'
import { api } from '../client'
import { Can } from '../rbac'
import { RequestSequence } from '../request-sequence'

interface Zone {
	id: string
	name?: string
	dnssec_enabled?: boolean
}

interface RecordItem {
	id: string
	name?: string
	type?: string
	content?: string
}

export function DNSPage ({ accountId }: { accountId: string }) {
	const [zones, setZones] = useState<Zone[]>([])
	const [zoneId, setZoneId] = useState('')
	const [records, setRecords] = useState<RecordItem[]>([])
	const [msg, setMsg] = useState('')
	const requests = useRef(new RequestSequence()).current
	const currentAccountId = useRef(accountId)
	const currentZoneId = useRef(zoneId)
	currentAccountId.current = accountId
	currentZoneId.current = zoneId
	const zone = zones.find((entry) => entry.id === zoneId) || zones[0]

	async function load (requestedAccountId: string, requestedZoneId = '') {
		const request = requests.begin('dns')
		const zoneResult = await api<{ items: Zone[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones`)
		if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
		setZones(zoneResult.items || [])
		const nextZoneId = requestedZoneId || zoneId || zoneResult.items?.[0]?.id || ''
		if (nextZoneId && currentAccountId.current === requestedAccountId) setZoneId(nextZoneId)
		const selected = (zoneResult.items || []).find((entry) => entry.id === nextZoneId) || zoneResult.items?.[0]
		if (!selected) {
			setRecords([])
			return
		}
		const rec = await api<{ items: RecordItem[] }>(`/api/v1/accounts/${requestedAccountId}/dns/zones/${selected.id}/records`)
		if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
		if (requestedZoneId && currentZoneId.current && currentZoneId.current !== selected.id) return
		setRecords(rec.items || [])
	}

	useEffect(() => {
		load(accountId).catch((error) => setMsg(error instanceof Error ? error.message : 'failed'))
	}, [accountId])

	useEffect(() => {
		if (!zoneId) return
		load(accountId, zoneId).catch((error) => setMsg(error instanceof Error ? error.message : 'failed'))
	}, [zoneId])

	return (
		<>
			<h1>DNS</h1>
			{zones.length === 0 ? <p>No zones yet. They appear after account provisioning completes.</p> : (
				<>
					<label>Zone
						<select value={zone?.id || ''} onChange={(event) => setZoneId(event.target.value)}>
							{zones.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
						</select>
					</label>
					<p>{zone?.name} — DNSSEC {zone?.dnssec_enabled ? 'on' : 'off'}.</p>
					<Can cap="dns.write">
					<form onSubmit={async (event) => {
						event.preventDefault()
						if (!zone) return
						const requestedAccountId = accountId
						const requestedZoneId = zone.id
						const data = new FormData(event.currentTarget)
						await api(`/api/v1/accounts/${requestedAccountId}/dns/zones/${requestedZoneId}/records`, {
							method: 'POST',
							body: JSON.stringify({
								name: data.get('name'),
								type: data.get('type'),
								content: data.get('content'),
								ttl: Number(data.get('ttl') || 300),
							}),
						})
						if (currentAccountId.current !== requestedAccountId) return
						setMsg('Record queued for sync')
						await load(requestedAccountId, requestedZoneId)
					}}>
						<input name="name" placeholder="www" required />
						<select name="type">
							<option value="A">A</option>
							<option value="AAAA">AAAA</option>
							<option value="CNAME">CNAME</option>
							<option value="MX">MX</option>
							<option value="TXT">TXT</option>
						</select>
						<input name="content" placeholder="203.0.113.10" required />
						<input name="ttl" type="number" defaultValue={300} min={60} />
						<button type="submit">Add record</button>
					</form>
					</Can>
					{msg ? <p>{msg}</p> : null}
					<table>
						<thead><tr><th>Name</th><th>Type</th><th>Content</th></tr></thead>
						<tbody>{records.map((record) => (
							<tr key={record.id}><td>{record.name}</td><td>{record.type}</td><td>{record.content}</td></tr>
						))}</tbody>
					</table>
				</>
			)}
		</>
	)
}
