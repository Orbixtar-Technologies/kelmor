import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account, ResourceItem } from '../types'

interface FileEntry {
	name: string
	size?: number
	dir?: boolean
}

export function FileManagerPage () {
	const [params, setParams] = useSearchParams()
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [items, setItems] = useState<FileEntry[]>([])
	const [currentPath, setCurrentPath] = useState('/')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [editorOpen, setEditorOpen] = useState(false)
	const [editorPath, setEditorPath] = useState('')
	const [editorContent, setEditorContent] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('files.write')
	const accountId = params.get('account') || ''
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const account = accounts.find((entry) => entry.id === accountId)

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!currentAccountId.current && next[0]) setParams({ account: next[0].id }, { replace: true })
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [requests, setParams])

	const loadDirectory = useCallback((requestedAccountId: string, path: string) => {
		if (!requestedAccountId) return
		const request = requests.begin('files')
		setLoading(true)
		setError('')
		api<{ items: FileEntry[]; path: string }>(`/api/v1/accounts/${requestedAccountId}/files?path=${encodeURIComponent(path)}`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== requestedAccountId) return
			setItems(asList(result))
			setCurrentPath(result.path || path)
		}).catch((requestError) => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request) && currentAccountId.current === requestedAccountId) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		requests.invalidate('files')
		setItems([])
		setCurrentPath('/')
		setMessage('')
		if (accountId) loadDirectory(accountId, '/')
		else setLoading(false)
	}, [accountId, loadDirectory, requests])

	const breadcrumbs = useMemo(() => {
		const parts = currentPath.split('/').filter(Boolean)
		const crumbs = [{ label: 'Home', path: '/' }]
		let built = ''
		for (const part of parts) {
			built += `/${part}`
			crumbs.push({ label: part, path: built })
		}
		return crumbs
	}, [currentPath])

	function navigateTo (path: string) {
		if (!accountId) return
		loadDirectory(accountId, path)
	}

	function openEditor (path: string, content = '') {
		setEditorPath(path)
		setEditorContent(content)
		setEditorOpen(true)
	}

	async function saveFile () {
		if (!accountId || !editorPath) return
		try {
			await api(`/api/v1/accounts/${accountId}/files`, {
				method: 'POST',
				body: JSON.stringify({ path: editorPath, content: editorContent }),
			})
			setMessage(`Saved ${editorPath}`)
			setEditorOpen(false)
			loadDirectory(accountId, currentPath)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	function entryPath (entry: FileEntry) {
		return currentPath.endsWith('/') ? `${currentPath}${entry.name}` : `${currentPath}/${entry.name}`
	}

	async function readFile (entry: FileEntry) {
		if (!accountId) return
		const path = entryPath(entry)
		if (entry.dir) {
			navigateTo(path)
			return
		}
		try {
			const result = await api<{ content: string }>(`/api/v1/accounts/${accountId}/files/content?path=${encodeURIComponent(path)}`)
			openEditor(path.startsWith('/') ? path : `/${path}`, result.content)
		} catch {
			openEditor(path.startsWith('/') ? path : `/${path}`)
		}
	}

	async function deleteEntry (entry: FileEntry) {
		if (!accountId || !window.confirm(`Delete ${entry.name}?`)) return
		try {
			await api(`/api/v1/accounts/${accountId}/files`, {
				method: 'DELETE',
				body: JSON.stringify({ path: entryPath(entry) }),
			})
			setMessage(`Deleted ${entry.name}`)
			loadDirectory(accountId, currentPath)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function renameEntry (entry: FileEntry) {
		if (!accountId) return
		const nextName = window.prompt('Rename to', entry.name)
		if (!nextName || nextName === entry.name) return
		const parent = entryPath(entry).split('/').slice(0, -1).join('/') || '/'
		const newPath = `${parent}/${nextName}`.replace('//', '/')
		try {
			await api(`/api/v1/accounts/${accountId}/files`, {
				method: 'PATCH',
				body: JSON.stringify({ path: entryPath(entry), new_path: newPath }),
			})
			setMessage(`Renamed to ${nextName}`)
			loadDirectory(accountId, currentPath)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function createFolder () {
		if (!accountId) return
		const name = window.prompt('New folder name')
		if (!name) return
		const path = currentPath.endsWith('/') ? `${currentPath}${name}` : `${currentPath}/${name}`
		try {
			await api(`/api/v1/accounts/${accountId}/files/mkdir`, {
				method: 'POST',
				body: JSON.stringify({ path }),
			})
			setMessage(`Created folder ${name}`)
			loadDirectory(accountId, currentPath)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	async function uploadFile (file: File) {
		if (!accountId) return
		const buffer = await file.arrayBuffer()
		const bytes = new Uint8Array(buffer)
		let binary = ''
		for (const byte of bytes) binary += String.fromCharCode(byte)
		const path = currentPath.endsWith('/') ? `${currentPath}${file.name}` : `${currentPath}/${file.name}`
		try {
			await api(`/api/v1/accounts/${accountId}/files`, {
				method: 'POST',
				body: JSON.stringify({ path, content_b64: btoa(binary) }),
			})
			setMessage(`Uploaded ${file.name}`)
			loadDirectory(accountId, currentPath)
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	const sortedItems = [...items].sort((left, right) => {
		if (Boolean(left.dir) !== Boolean(right.dir)) return left.dir ? -1 : 1
		return left.name.localeCompare(right.name)
	})

	return (
		<>
			<PageHeader
				title="File Manager"
				description="Browse account home directories, edit text files, and manage web content paths."
				actions={account ? <Link className="button-link secondary-link" to={`/accounts/${accountId}/services?service=files`}>Account services</Link> : undefined}
			/>
			<div className="hub-toolbar panel">
				<AccountPicker
					accounts={accounts}
					value={accountId}
					filter={accountFilter}
					onFilterChange={setAccountFilter}
					onChange={(next) => setParams({ account: next }, { replace: true })}
				/>
				{account ? <dl className="hub-meta"><div><dt>Home</dt><dd><code>{account.home_path}</code></dd></div><div><dt>Domain</dt><dd>{account.primary_domain}</dd></div></dl> : null}
			</div>
			{message ? <p className="feedback" role="status">{message}</p> : null}
			{error ? <ErrorState error={error} onRetry={() => loadDirectory(accountId, currentPath)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to open its file manager." /> : null}
			{accountId ? <section className="panel file-manager">
				<nav className="file-breadcrumbs" aria-label="File path">
					{breadcrumbs.map((crumb, index) => (
						<span key={crumb.path}>
							{index > 0 ? <span className="crumb-sep">/</span> : null}
							<button type="button" className="link-button" onClick={() => navigateTo(crumb.path)}>{crumb.label}</button>
						</span>
					))}
				</nav>
				<div className="file-toolbar">
					<button type="button" className="secondary compact" disabled={currentPath === '/'} onClick={() => navigateTo(currentPath.split('/').slice(0, -1).join('/') || '/')}>Up</button>
					<button type="button" className="secondary compact" onClick={() => navigateTo('/public_html')}>public_html</button>
					{canWrite ? <>
						<button type="button" className="compact" onClick={() => openEditor(currentPath === '/' ? '/public_html/new-file.txt' : `${currentPath}/new-file.txt`)}>New file</button>
						<button type="button" className="secondary compact" onClick={createFolder}>New folder</button>
						<label className="upload-button secondary compact">
							Upload<input type="file" hidden onChange={(event) => { const file = event.target.files?.[0]; if (file) void uploadFile(file); event.target.value = '' }} />
						</label>
					</> : null}
				</div>
				{loading ? <LoadingState label="Reading directory…" /> : null}
				{!loading ? <div className="table-wrap"><table className="dense-table file-table">
					<thead><tr><th>Name</th><th>Size</th><th>Type</th><th>Actions</th></tr></thead>
					<tbody>
						{sortedItems.map((entry) => (
							<tr key={entry.name}>
								<td>
									<button type="button" className="file-name link-button" onClick={() => entry.dir ? navigateTo(currentPath.endsWith('/') ? `${currentPath}${entry.name}` : `${currentPath}/${entry.name}`) : readFile(entry)}>
										<span aria-hidden="true">{entry.dir ? '📁' : '📄'}</span> {entry.name}{entry.dir ? '/' : ''}
									</button>
								</td>
								<td>{entry.dir ? '—' : formatBytes(Number(entry.size || 0))}</td>
								<td>{entry.dir ? 'Directory' : 'File'}</td>
								<td className="row-actions">
									{!entry.dir && canWrite ? <button type="button" className="link-button" onClick={() => readFile(entry)}>Edit</button> : null}
									{canWrite ? <>
										<button type="button" className="link-button" onClick={() => renameEntry(entry)}>Rename</button>
										<button type="button" className="link-button danger-text" onClick={() => deleteEntry(entry)}>Delete</button>
									</> : null}
								</td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading && !sortedItems.length ? <EmptyState title="Empty directory" detail="This folder has no visible entries yet." action={canWrite ? <button type="button" onClick={() => openEditor('/public_html/index.html', '<!DOCTYPE html>\n<html>\n<head><title>Welcome</title></head>\n<body><h1>It works!</h1></body>\n</html>\n')}>Create index.html</button> : undefined} /> : null}
			</section> : null}
			<Dialog open={editorOpen} title={`Edit ${editorPath}`} onClose={() => setEditorOpen(false)}>
				<label>Path<input value={editorPath} onChange={(event) => setEditorPath(event.target.value)} /></label>
				<label>Contents<textarea rows={16} value={editorContent} onChange={(event) => setEditorContent(event.target.value)} /></label>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setEditorOpen(false)}>Cancel</button><button type="button" onClick={saveFile}>Save file</button></footer>
			</Dialog>
		</>
	)
}
