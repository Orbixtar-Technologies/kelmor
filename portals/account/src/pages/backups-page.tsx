import { useEffect, useRef, useState } from 'react'
import { api, asList } from '../client'
import { createPendingGuard } from '../pending-submit'
import { Can } from '../rbac'

interface BackupItem {
	id: string
	kind?: string
	state?: string
	destination?: string
	checksum?: string
}

export function BackupsPage ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<BackupItem[]>([])
	const [notice, setNotice] = useState('')
	const [restorePending, setRestorePending] = useState(false)
	const restoreGuard = useRef(createPendingGuard()).current

	useEffect(() => {
		restoreGuard.finish()
		setRestorePending(false)
		let cancelled = false
		let timer = 0
		let inFlight = false
		async function poll () {
			if (cancelled || inFlight) return
			inFlight = true
			try {
				const result = await api<{ items: BackupItem[] }>(`/api/v1/accounts/${accountId}/backups`)
				if (!cancelled) setItems(asList(result))
			} catch {
				// keep last list until the next settled poll
			} finally {
				inFlight = false
				if (!cancelled) timer = window.setTimeout(poll, 1000)
			}
		}
		void poll()
		return () => {
			cancelled = true
			window.clearTimeout(timer)
		}
	}, [accountId])

	return (
		<>
			<h1>Backups</h1>
			<p>Encrypted HPM1 copies of home, databases, and mail. Local disk, offsite SFTP, and the loopback S3 store are live destinations on this host.</p>
			{notice ? <p className="notice">{notice}</p> : null}
			<Can cap="backups.create">
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				const destination = String(data.get('destination') || 'local')
				await api(`/api/v1/accounts/${accountId}/backups`, {
					method: 'POST',
					body: JSON.stringify({ kind: 'full', destination }),
				})
				setNotice(`Full backup queued to ${destination}`)
			}}>
				<select name="destination">
					<option value="local">Local disk</option>
					<option value="sftp">Offsite SFTP</option>
					<option value="s3">S3-compatible</option>
				</select>
				<button type="submit">Create full backup</button>
			</form>
			</Can>
			{items.length === 0 ? <p>No backup runs yet.</p> : (
			<ul>{items.map((backup) => (
				<li key={backup.id} data-backup-id={backup.id} data-backup-state={backup.state} data-backup-destination={backup.destination}>
					{backup.kind} {backup.state} {backup.destination} {backup.checksum ? `sha256:${backup.checksum.slice(0, 12)}` : ''}
					{backup.state === 'succeeded' ? (
						<Can cap="backups.restore">
						<button type="button" data-restore={backup.id} disabled={restorePending} onClick={async () => {
							if (!restoreGuard.tryStart()) return
							setRestorePending(true)
							try {
								await api(`/api/v1/accounts/${accountId}/restores`, { method: 'POST', body: JSON.stringify({ backup_id: backup.id, mode: 'in_place' }) })
								setNotice(`Restore queued for ${backup.destination} backup`)
							} catch {
								restoreGuard.finish()
								setRestorePending(false)
							}
						}}>Restore</button>
						</Can>
					) : null}
				</li>
			))}</ul>
			)}
		</>
	)
}
