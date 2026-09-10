import { useCallback, useEffect, useState } from 'react'
import { api } from '../client'
import { ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface UpdateStatus {
	state: string
	installed_release: string
	available_release?: string
	last_checked_at?: string
	error?: string
	automatic: boolean
	channel: string
}

export function UpdatesPage () {
	const canManage = useCan('server.settings.write')
	const [status, setStatus] = useState<UpdateStatus | null>(null)
	const [loading, setLoading] = useState(true)
	const [busy, setBusy] = useState(false)
	const [message, setMessage] = useState('')
	const [error, setError] = useState('')
	const load = useCallback(async () => {
		setLoading(true)
		setError('')
		try {
			setStatus(await api<UpdateStatus>('/api/v1/server/updates'))
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setLoading(false)
		}
	}, [])
	useEffect(() => { void load() }, [load])
	async function runAction (path: string, confirmText?: string) {
		if (confirmText && !window.confirm(confirmText)) return
		setBusy(true)
		setMessage('')
		try {
			await api(path, { method: 'POST', body: '{}' })
			setMessage('Update request accepted.')
			await load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}
	async function toggleAutomatic (automatic: boolean) {
		setBusy(true)
		setMessage('')
		try {
			await api('/api/v1/server/updates/settings', {
				method: 'PATCH',
				body: JSON.stringify({ automatic }),
			})
			setMessage(automatic ? 'Automatic updates enabled.' : 'Automatic updates disabled.')
			await load()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}
	const hasUpdate = Boolean(status?.available_release &&
		status.available_release !== status.installed_release)
	return <>
		<PageHeader
			title="Software Updates"
			description="Review the installed release, check the signed stable feed, and install verified updates."
		/>
		{message ? <p className="feedback" role="status">{message}</p> : null}
		<section className="panel">
			<SectionHeading title="Release status" detail="Updates are fetched from the configured HTTPS feed and verified with a pinned public key." />
			{loading ? <LoadingState /> : error ? <ErrorState error={error} /> : status ? <>
				<dl className="detail-list">
					<div><dt>Installed</dt><dd><code>{status.installed_release}</code></dd></div>
					<div><dt>Available</dt><dd><code>{status.available_release || '—'}</code></dd></div>
					<div><dt>Channel</dt><dd><code>{status.channel}</code></dd></div>
					<div><dt>State</dt><dd><code>{status.state}</code></dd></div>
					<div><dt>Automatic install</dt><dd>{status.automatic ? 'Enabled' : 'Disabled'}</dd></div>
					<div><dt>Last checked</dt><dd>{status.last_checked_at || '—'}</dd></div>
				</dl>
				{status.error ? <p className="field-error" role="alert">{status.error}</p> : null}
			</> : null}
			<div className="inline-form">
				<button type="button" disabled={busy || loading} onClick={() => runAction('/api/v1/server/updates/check')}>Check now</button>
				{canManage ? <>
					<button
						type="button"
						disabled={busy || loading || !hasUpdate}
						onClick={() => runAction(
							'/api/v1/server/updates/install',
							`Install release ${status?.available_release}? Services will restart after verification.`,
						)}
					>
						Install verified release
					</button>
					<button
						type="button"
						className="secondary"
						disabled={busy || loading || !status}
						onClick={() => toggleAutomatic(!status?.automatic)}
					>
						{status?.automatic ? 'Disable automatic updates' : 'Enable automatic updates'}
					</button>
				</> : <p className="subtle">Your role can inspect updates but cannot install them.</p>}
			</div>
		</section>
	</>
}
