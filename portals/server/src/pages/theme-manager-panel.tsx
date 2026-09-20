import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice } from '../components/queued-op-notice'
import { CHROME_SETTINGS_SAVED } from '../catalog-honesty'
import { ErrorState, LoadingState } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import { toolCatalog } from '../tool-catalog'
import {
	DEFAULT_FAVORITE_TOOL_IDS,
	moveFavorite,
	notifyThemeChanged,
	parseFavoriteIds,
	serializeFavoriteIds,
} from './theme-favorites'

interface ServerSettings {
	values?: Record<string, Record<string, string>>
}

export function ThemeManagerPanel () {
	const canWrite = useCan('server.settings.write')
	const [density, setDensity] = useState('comfortable')
	const [favoriteIds, setFavoriteIds] = useState<string[]>(DEFAULT_FAVORITE_TOOL_IDS)
	const [addId, setAddId] = useState('')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [busy, setBusy] = useState(false)
	const toolById = useMemo(() => new Map(toolCatalog.map((tool) => [tool.id, tool])), [])
	const pinned = favoriteIds.map((id) => toolById.get(id)).filter(Boolean)
	const available = toolCatalog.filter((tool) => tool.id !== 'home' && !favoriteIds.includes(tool.id))

	function load () {
		setLoading(true)
		setError('')
		api<ServerSettings>('/api/v1/server/settings').then((result) => {
			const stored = result.values?.theme || {}
			if (stored.density) setDensity(stored.density)
			const saved = parseFavoriteIds(stored.favorites)
			setFavoriteIds(saved.length ? saved : DEFAULT_FAVORITE_TOOL_IDS)
		}).catch((reason) => setError(messageFrom(reason))).finally(() => setLoading(false))
	}

	useEffect(load, [])

	async function handleSave (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		try {
			await api('/api/v1/server/settings', {
				method: 'PATCH',
				body: JSON.stringify({
					values: { theme: { density, favorites: serializeFavoriteIds(favoriteIds) } },
				}),
			})
			document.documentElement.dataset.density = density
			notifyThemeChanged()
			setMessage(CHROME_SETTINGS_SAVED)
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<h2>Choose density and favorites</h2>
			<p>Density updates Director chrome. Favorites are pinned on Home and at the top of the sidebar. This is an operator preference, not a host apply job.</p>
			<QueuedOpNotice message={message} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading theme preferences…" /> : null}
			{!loading && canWrite ? (
				<form className="form-grid theme-favorites-form" onSubmit={handleSave}>
					<label>Density
						<select value={density} onChange={(event) => setDensity(event.target.value)}>
							<option value="comfortable">Comfortable</option>
							<option value="compact">Compact</option>
						</select>
					</label>
					<ol className="theme-favorite-list" aria-label="Pinned favorite tools">
						{pinned.map((tool, index) => (
							<li key={tool!.id}>
								<strong>{tool!.label}</strong>
								<span>
									<button type="button" onClick={() => setFavoriteIds((current) => moveFavorite(current, index, -1))}>
										Move {tool!.label} up
									</button>
									<button type="button" onClick={() => setFavoriteIds((current) => moveFavorite(current, index, 1))}>
										Move {tool!.label} down
									</button>
									<button type="button" className="secondary" onClick={() => setFavoriteIds((current) => current.filter((id) => id !== tool!.id))}>
										Unpin {tool!.label}
									</button>
								</span>
							</li>
						))}
					</ol>
					<label>Add a favorite
						<select value={addId} onChange={(event) => setAddId(event.target.value)}>
							<option value="">Choose a tool</option>
							{available.map((tool) => (
								<option key={tool.id} value={tool.id}>{tool.label}</option>
							))}
						</select>
					</label>
					<button type="button" className="secondary" disabled={!addId} onClick={() => {
						if (!addId) return
						setFavoriteIds((current) => current.includes(addId) ? current : [...current, addId])
						setAddId('')
					}}>Pin tool</button>
					<button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Apply to chrome'}</button>
				</form>
			) : null}
		</section>
	)
}
