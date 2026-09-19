import { FormEvent, useEffect, useState } from 'react'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface SkeletonEntry {
	name: string
	dir?: boolean
	size?: number
}

export function SkeletonDirectoryPage () {
	const canWrite = useCan('server.settings.write')
	const [path, setPath] = useState('')
	const [items, setItems] = useState<SkeletonEntry[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [editorPath, setEditorPath] = useState('')
	const [editorContent, setEditorContent] = useState('')
	const [newName, setNewName] = useState('')

	function load (nextPath = path) {
		setLoading(true)
		setError('')
		api<{ items: SkeletonEntry[] }>(`/api/v1/server/skeleton?path=${encodeURIComponent(nextPath)}`)
			.then((result) => {
				setItems(asList(result))
				setPath(nextPath)
			})
			.catch((reason) => setError(messageFrom(reason)))
			.finally(() => setLoading(false))
	}
	useEffect(() => { load('') }, [])

	function openChild (entry: SkeletonEntry) {
		const next = [path, entry.name].filter(Boolean).join('/')
		if (entry.dir) {
			load(next)
			return
		}
		api<{ content: string }>(`/api/v1/server/skeleton/file?path=${encodeURIComponent(next)}`)
			.then((result) => {
				setEditorPath(next)
				setEditorContent(result.content || '')
			})
			.catch((reason) => setError(messageFrom(reason)))
	}

	async function handleSave (event: FormEvent) {
		event.preventDefault()
		setError('')
		try {
			await api('/api/v1/server/skeleton/file', {
				method: 'PUT',
				body: JSON.stringify({ path: editorPath, content: editorContent }),
			})
			setMessage(`Wrote ${editorPath} on the host.`)
			load(path)
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	async function handleDelete (entry: SkeletonEntry) {
		const target = [path, entry.name].filter(Boolean).join('/')
		setError('')
		try {
			await api(`/api/v1/server/skeleton/file?path=${encodeURIComponent(target)}`, { method: 'DELETE' })
			setMessage(`Removed ${target}.`)
			if (editorPath === target) {
				setEditorPath('')
				setEditorContent('')
			}
			load(path)
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	async function handleCreate () {
		const target = [path, newName].filter(Boolean).join('/')
		if (!newName.trim()) return
		setError('')
		try {
			if (newName.endsWith('/')) {
				await api('/api/v1/server/skeleton/mkdir', { method: 'POST', body: JSON.stringify({ path: target.replace(/\/$/, '') }) })
			} else {
				await api('/api/v1/server/skeleton/file', { method: 'PUT', body: JSON.stringify({ path: target, content: '' }) })
			}
			setNewName('')
			setMessage(`Created ${target} under /etc/skel.`)
			load(path)
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	const parent = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''

	return (
		<>
			<PageHeader
				title="Skeleton Directory"
				description="Files under /etc/skel are copied into new account homes. Edits apply on the host immediately."
			/>
			{error ? <ErrorState error={error} onRetry={() => load(path)} /> : null}
			{message ? <p role="status">{message}</p> : null}
			<section className="panel">
				<p>Current path: <code>/etc/skel{path ? `/${path}` : ''}</code></p>
				{path ? <button type="button" onClick={() => load(parent)}>Up one directory</button> : null}
				{loading ? <LoadingState label="Loading skeleton…" /> : (
					<div className="table-wrap">
						<table className="dense-table">
							<thead><tr><th>Name</th><th>Size</th>{canWrite ? <th>Actions</th> : null}</tr></thead>
							<tbody>
								{items.map((entry) => (
									<tr key={entry.name}>
										<td>
											<button type="button" className="linkish" onClick={() => openChild(entry)}>
												{entry.dir ? `${entry.name}/` : entry.name}
											</button>
										</td>
										<td>{entry.dir ? '—' : formatBytes(entry.size)}</td>
										{canWrite ? (
											<td>
												<button type="button" onClick={() => handleDelete(entry)}>Delete</button>
											</td>
										) : null}
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
				{!loading && !items.length ? <EmptyState title="Empty directory" detail="Create a file or folder to seed new homes." /> : null}
				{canWrite ? (
					<div className="row-actions">
						<label>
							New file or folder/
							<input value={newName} onChange={(event) => setNewName(event.target.value)} placeholder="public_html/ or .bashrc" />
						</label>
						<button type="button" onClick={handleCreate}>Create</button>
					</div>
				) : null}
			</section>
			{editorPath ? (
				<form className="panel" onSubmit={handleSave}>
					<h2>Edit {editorPath}</h2>
					<textarea value={editorContent} onChange={(event) => setEditorContent(event.target.value)} rows={16} />
					<button type="submit" disabled={!canWrite}>Save on host</button>
				</form>
			) : null}
		</>
	)
}
