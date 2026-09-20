import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader, SectionHeading } from '../components/ui'
import { formatDate, messageFrom } from '../helpers'
import { formatLastChecked } from './updates-copy'
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

interface ChangelogItem {
	version: string
	status: string
	source: string
	title?: string
	notes: string[]
}

interface ChangelogView {
	installed_release: string
	available_release?: string
	running_release?: string
	channel: string
	items: ChangelogItem[]
}

export const CHANGELOG_EMPTY_TITLE = 'No release notes yet'
export const CHANGELOG_EMPTY_DETAIL = 'The signed update feed and the installed changelog have not published notes for the installed or available release. Software Updates still shows what this host would install next.'
export const SOFTWARE_UPDATES_CTA = 'Open Software Updates'

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
	const [changelog, setChangelog] = useState<ChangelogView | null>(null)
	const [notesError, setNotesError] = useState('')
	const loadNotes = useCallback(async () => {
		setNotesError('')
		try {
			setChangelog(await api<ChangelogView>('/api/v1/server/updates/changelog'))
		} catch (requestError) {
			setNotesError(messageFrom(requestError))
			setChangelog(null)
		}
	}, [])
	useEffect(() => { void loadNotes() }, [loadNotes])
	const items = changelog?.items || []
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
						<div><dt>Last checked</dt><dd>{formatLastChecked(status.last_checked_at)}</dd></div>
					</dl>
				) : null}
				{status?.error ? <p className="field-error" role="alert">{status.error}</p> : null}
			</section>
			<section className="panel">
				<SectionHeading
					title="Release notes"
					detail="Notes come from the update feed changelog, a cached check, or the changelog compiled into this release. Director does not invent entries."
				/>
				{notesError ? <ErrorState error={notesError} onRetry={() => { void loadNotes() }} /> : null}
				{!notesError && items.length ? (
					<ol className="list-plain">
						{items.map((item) => (
							<li key={`${item.status}:${item.version}`}>
								<h3>{item.version} <small>{item.status} · {item.source}</small></h3>
								{item.title ? <p>{item.title}</p> : null}
								<ul>
									{item.notes.map((note) => <li key={note}>{note}</li>)}
								</ul>
							</li>
						))}
					</ol>
				) : null}
				{!notesError && changelog && !items.length ? (
					<EmptyState
						title={CHANGELOG_EMPTY_TITLE}
						detail={CHANGELOG_EMPTY_DETAIL}
						action={<Link className="button-link" to="/updates">{SOFTWARE_UPDATES_CTA}</Link>}
					/>
				) : null}
				{items.length ? <p><Link to="/updates">{SOFTWARE_UPDATES_CTA}</Link></p> : null}
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
