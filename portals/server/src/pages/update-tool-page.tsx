import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface UpdateStatus {
	state: string
	installed_release: string
	running_release?: string
	available_release?: string
	last_checked_at?: string
	error?: string
	automatic: boolean
	channel: string
}

interface UpdateToolPageProps {
	toolId: 'preferences' | 'changelog'
}

export function UpdateToolPage ({ toolId }: UpdateToolPageProps) {
	switch (toolId) {
		case 'preferences':
			return <UpdatePreferencesPage />
		case 'changelog':
			return <ChangeLogPage />
		default: {
			const unhandled: never = toolId
			return unhandled
		}
	}
}

function UpdatePreferencesPage () {
	const canManage = useCan('server.settings.write')
	const { status, loading, error, updatedAt, message, busy, load, setBusy, setMessage } = useUpdateStatus()

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

	return (
		<>
			<PageHeader
				title="Update Preferences"
				description="Release channel and automatic install policy for this host."
			/>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			{message ? <p className="feedback" role="status">{message}</p> : null}
			<section className="panel">
				<SectionHeading
					title="Install policy"
					detail="Director can enable or disable automatic install of the already verified release. The channel itself is set on the host update feed."
				/>
				{loading ? <LoadingState /> : error ? <ErrorState error={error} onRetry={() => { void load() }} /> : status ? (
					<dl className="detail-list">
						<div><dt>Channel</dt><dd>{status.channel}</dd></div>
						<div><dt>Automatic install</dt><dd>{status.automatic ? 'Enabled' : 'Disabled'}</dd></div>
						<div><dt>Installed release</dt><dd><code>{status.installed_release}</code></dd></div>
					</dl>
				) : null}
				<p className="subtle">Channel selection is not a Director API. Change the host feed configuration if this host should track a different signed channel. Check and install remain on Software Updates.</p>
				<div className="inline-form">
					{canManage ? (
						<button
							type="button"
							className="secondary"
							disabled={busy || loading || !status}
							onClick={() => toggleAutomatic(!status?.automatic)}
						>
							{status?.automatic ? 'Disable automatic updates' : 'Enable automatic updates'}
						</button>
					) : <p className="subtle">Your role can inspect the policy but cannot change it.</p>}
				</div>
				<p><Link to="/updates">Open Software Updates</Link></p>
			</section>
		</>
	)
}

function ChangeLogPage () {
	const { status, loading, error, updatedAt, load } = useUpdateStatus()
	return (
		<>
			<PageHeader
				title="Change Log"
				description="Operator-facing notes for this host’s Kelmor release channel."
			/>
			{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
			<section className="panel">
				<SectionHeading
					title="Channel releases"
					detail="Installed and available releases from the configured signed feed."
				/>
				{loading ? <LoadingState /> : error ? <ErrorState error={error} onRetry={() => { void load() }} /> : status ? (
					<dl className="detail-list">
						<div><dt>Installed</dt><dd>{status.installed_release}</dd></div>
						<div><dt>Available</dt><dd>{status.available_release || '—'}</dd></div>
						<div><dt>Channel</dt><dd>{status.channel}</dd></div>
						<div><dt>State</dt><dd><code>{status.state}</code></dd></div>
						<div><dt>Last checked</dt><dd>{status.last_checked_at || '—'}</dd></div>
					</dl>
				) : null}
				{status?.error ? <p className="field-error" role="alert">{status.error}</p> : null}
				<p>The update API does not publish release notes or a signed change log. This page shows the installed and available releases on the configured channel so operators can see what the host would install next.</p>
				<p><Link to="/updates">Open Software Updates</Link></p>
			</section>
		</>
	)
}

function useUpdateStatus () {
	const [status, setStatus] = useState<UpdateStatus | null>(null)
	const [loading, setLoading] = useState(true)
	const [busy, setBusy] = useState(false)
	const [message, setMessage] = useState('')
	const [error, setError] = useState('')
	const [updatedAt, setUpdatedAt] = useState('')
	const load = useCallback(async () => {
		setLoading(true)
		setError('')
		try {
			setStatus(await api<UpdateStatus>('/api/v1/server/updates'))
			setUpdatedAt(new Date().toISOString())
		} catch (requestError) {
			setError(messageFrom(requestError))
		} finally {
			setLoading(false)
		}
	}, [])
	useEffect(() => { void load() }, [load])
	return { status, loading, error, updatedAt, message, busy, load, setBusy, setMessage }
}
