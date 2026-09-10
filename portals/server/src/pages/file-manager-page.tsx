import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { AccountScopeBar } from '../components/account-scope-bar'
import { Dialog, EmptyState, ErrorState, LoadingState, PageHeader, Pagination } from '../components/ui'
import { formatBytes, formatDate, messageFrom } from '../helpers'
import { paginateRows } from '../table-helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account } from '../types'
import { fileKindLabel, isProtectedName, isProtectedPath, protectedPathWarning } from './file-path-guards'

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
	const [query, setQuery] = useState('')
	const [sortKey, setSortKey] = useState<'name' | 'size'>('name')
	const [page, setPage] = useState(1)
	const [updatedAt, setUpdatedAt] = useState('')
	const [pendingDelete, setPendingDelete] = useState<FileEntry | null>(null)
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
			setUpdatedAt(new Date().toISOString())
			setPage(1)
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
		if (!accountId) return
		if (isProtectedName(entry.name) || isProtectedPath(currentPath, entry.name)) {
			setPendingDelete(entry)
			return
		}
		if (!window.confirm(`Delete ${entry.name}? This cannot be undone from this page.`)) return
		await confirmDelete(entry)
	}

	async function confirmDelete (entry: FileEntry) {
		if (!accountId) return
		try {
			await api(`/api/v1/accounts/${accountId}/files`, {
				method: 'DELETE',
				body: JSON.stringify({ path: entryPath(entry) }),
			})
			setMessage(`Deleted ${entry.name}`)
			setPendingDelete(null)
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

	const filteredItems = items.filter((entry) => entry.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
	const sortedItems = [...filteredItems].sort((left, right) => {
		if (Boolean(left.dir) !== Boolean(right.dir)) return left.dir ? -1 : 1
		if (sortKey === 'size') return Number(left.size || 0) - Number(right.size || 0)
		return left.name.localeCompare(right.name, undefined, { numeric: true })
	})
	const paged = paginateRows(sortedItems, page, 40)
	const totalBytes = items.filter((entry) => !entry.dir).reduce((sum, entry) => sum + Number(entry.size || 0), 0)
	const pathWarning = protectedPathWarning(currentPath)

	return (
		<>
			<PageHeader
				title={account ? `File Manager · ${account.username}` : 'File Manager'}
				description="Browse account home directories, edit text files, and manage web content paths."
				actions={account ? <Link className="button-link secondary-link" to={`/accounts/${accountId}/services?service=files`}>Account services</Link> : undefined}
			/>
			<AccountScopeBar accountId={accountId} accounts={accounts} toolLabel="Files" onChange={(next) => setParams({ account: next }, { replace: true })} />
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
				{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)} · {items.length} entries · {formatBytes(totalBytes)}</p> : null}
				{pathWarning ? <p className="feedback" role="status">{pathWarning}</p> : null}
				<div className="file-toolbar">
					<label>Search files<input type="search" value={query} placeholder="Filter this folder" onChange={(event) => { setQuery(event.target.value); setPage(1) }} /></label>
					<label>Sort<select value={sortKey} onChange={(event) => setSortKey(event.target.value as 'name' | 'size')}><option value="name">Name</option><option value="size">Size</option></select></label>
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
						{paged.items.map((entry) => (
							<tr key={entry.name} className={isProtectedName(entry.name) ? 'file-row-protected' : undefined}>
								<td>
									<button type="button" className="file-name link-button" onClick={() => entry.dir ? navigateTo(currentPath.endsWith('/') ? `${currentPath}${entry.name}` : `${currentPath}/${entry.name}`) : readFile(entry)}>
										<span aria-hidden="true">{entry.dir ? '📁' : '📄'}</span> {entry.name}{entry.dir ? '/' : ''}
									</button>
								</td>
								<td>{entry.dir ? '—' : formatBytes(Number(entry.size || 0))}</td>
								<td>{fileKindLabel(entry.name, entry.dir)}</td>
								<td className="row-actions">
									{!entry.dir && canWrite ? <button type="button" className="link-button" onClick={() => readFile(entry)}>Edit</button> : null}
									{canWrite ? <button type="button" className="link-button" onClick={() => renameEntry(entry)}>Rename</button> : null}
									{canWrite ? <button type="button" className="link-button danger-text" onClick={() => deleteEntry(entry)}>Delete</button> : null}
								</td>
							</tr>
						))}
					</tbody>
				</table></div> : null}
				{!loading ? <Pagination page={paged.page} pageCount={paged.pageCount} total={paged.total} onPage={setPage} /> : null}
				{!loading && !sortedItems.length ? <EmptyState title="Empty directory" detail="This folder has no visible entries yet." action={canWrite ? <button type="button" onClick={() => openEditor('/public_html/index.html', '<!DOCTYPE html>\n<html>\n<head><title>Welcome</title></head>\n<body><h1>It works!</h1></body>\n</html>\n')}>Create index.html</button> : undefined} /> : null}
			</section> : null}
			<Dialog open={Boolean(pendingDelete)} title="Delete protected path" onClose={() => setPendingDelete(null)} actions={<>
				<button type="button" className="secondary" onClick={() => setPendingDelete(null)}>Cancel</button>
				<button type="button" className="danger" onClick={() => pendingDelete && confirmDelete(pendingDelete)}>Delete {pendingDelete?.name}</button>
			</>}>
				<p>This path is system-managed. Deleting it can break SSH, backups, mail, or panel metadata for the account.</p>
			</Dialog>
			<Dialog open={editorOpen} title={`Edit ${editorPath}`} onClose={() => setEditorOpen(false)}>
				<label>Path<input value={editorPath} onChange={(event) => setEditorPath(event.target.value)} /></label>
				<label>Contents<textarea rows={16} value={editorContent} onChange={(event) => setEditorContent(event.target.value)} /></label>
				<footer className="dialog-form-actions"><button type="button" className="secondary" onClick={() => setEditorOpen(false)}>Cancel</button><button type="button" onClick={saveFile}>Save file</button></footer>
			</Dialog>
		</>
	)
}
