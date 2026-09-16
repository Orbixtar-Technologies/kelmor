import { useEffect, useRef, useState } from 'react'
import { api, asList } from '../client'
import { Can } from '../rbac'
import { RequestSequence } from '../request-sequence'

interface FileEntry {
	name: string
	size?: number
	dir?: boolean
}

export function FilesPage ({ accountId }: { accountId: string }) {
	const [items, setItems] = useState<FileEntry[]>([])
	const [ftpUsers, setFtpUsers] = useState<Array<{ id: string; username?: string; home_path?: string }>>([])
	const [sshKeys, setSSHKeys] = useState<Array<{ id: string; label?: string; fingerprint?: string }>>([])
	const [path, setPath] = useState('/')
	const [editorAccountId, setEditorAccountId] = useState(accountId)
	const [editorPath, setEditorPath] = useState('')
	const [editorContent, setEditorContent] = useState('')
	const requests = useRef(new RequestSequence()).current
	const currentAccountId = useRef(accountId)
	currentAccountId.current = accountId
	const loadFTP = () => api<{ items: Array<{ id: string; username?: string; home_path?: string }> }>(`/api/v1/accounts/${accountId}/ftp`).then((result) => setFtpUsers(asList(result)))
	const loadSSH = () => api<{ items: Array<{ id: string; label?: string; fingerprint?: string }> }>(`/api/v1/accounts/${accountId}/ssh-keys`).then((result) => setSSHKeys(asList(result)))

	useEffect(() => {
		const request = requests.begin('files')
		api<{ items: FileEntry[] }>(`/api/v1/accounts/${accountId}/files?path=${encodeURIComponent(path)}`).then((result) => {
			if (!requests.isCurrent(request) || currentAccountId.current !== accountId) return
			setItems(asList(result))
		}).catch(() => {
			if (requests.isCurrent(request) && currentAccountId.current === accountId) setItems([])
		})
	}, [accountId, path, requests])
	useEffect(() => { void loadFTP(); void loadSSH() }, [accountId])

	async function readFile (entry: FileEntry) {
		const requestedAccountId = accountId
		const nextPath = path.endsWith('/') ? `${path}${entry.name}` : `${path}/${entry.name}`
		if (entry.dir) {
			setPath(nextPath)
			return
		}
		const result = await api<{ content: string }>(`/api/v1/accounts/${requestedAccountId}/files/content?path=${encodeURIComponent(nextPath)}`)
		setEditorAccountId(requestedAccountId)
		setEditorPath(nextPath)
		setEditorContent(result.content)
	}

	async function saveEditor () {
		if (!editorAccountId || !editorPath) return
		await api(`/api/v1/accounts/${editorAccountId}/files`, {
			method: 'POST',
			body: JSON.stringify({ path: editorPath, content: editorContent }),
		})
	}

	return (
		<>
			<h1>Files</h1>
			<p>Path {path}</p>
			<Can cap="files.write">
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				await api(`/api/v1/accounts/${accountId}/sftp-password`, {
					method: 'POST',
					body: JSON.stringify({ password: data.get('password') }),
				})
			}}>
				<label>SFTP password (chrooted to your home)
					<input name="password" type="password" minLength={8} required />
				</label>
				<button type="submit">Set SFTP password</button>
			</form>
			<h2>SSH public keys</h2>
			<p>Keys are written to <code>~/.ssh/authorized_keys</code> through the privileged agent. Password SSH is not used for the hosting account.</p>
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				await api(`/api/v1/accounts/${accountId}/ssh-keys`, {
					method: 'POST',
					body: JSON.stringify({ public_key: data.get('public_key'), label: data.get('label') }),
				})
				event.currentTarget.reset()
				await loadSSH()
			}}>
				<input name="public_key" placeholder="ssh-ed25519 AAAA…" required />
				<input name="label" placeholder="laptop" />
				<button type="submit">Add SSH key</button>
			</form>
			<ul>{sshKeys.map((key) => (
				<li key={key.id}>
					{key.label || key.fingerprint}
					<button type="button" onClick={async () => {
						await api(`/api/v1/accounts/${accountId}/ssh-keys/${key.id}`, { method: 'DELETE' })
						await loadSSH()
					}}>Remove</button>
				</li>
			))}</ul>
			<h2>FTP users</h2>
			<p>Virtual FTP logins map to this account and chroot to public_html (port 21).</p>
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				await api(`/api/v1/accounts/${accountId}/ftp`, {
					method: 'POST',
					body: JSON.stringify({ username: data.get('username'), password: data.get('password') }),
				})
				event.currentTarget.reset()
				await loadFTP()
			}}>
				<input name="username" placeholder="siteftp" required />
				<input name="password" type="password" minLength={8} required />
				<button type="submit">Create FTP user</button>
			</form>
			<ul>
				{ftpUsers.map((user) => (
					<li key={user.id}>
						{user.username} — {user.home_path}
						<button type="button" className="link" onClick={async () => {
							await api(`/api/v1/accounts/${accountId}/ftp/${user.id}`, { method: 'DELETE' })
							await loadFTP()
						}}>Remove</button>
					</li>
				))}
			</ul>
			<form onSubmit={async (event) => {
				event.preventDefault()
				const data = new FormData(event.currentTarget)
				const writePath = String(data.get('path') || path)
				await api(`/api/v1/accounts/${accountId}/files`, {
					method: 'POST',
					body: JSON.stringify({ path: writePath, content: data.get('content') }),
				})
				setPath(writePath)
			}}>
				<input name="path" defaultValue={path === '/' ? '/public_html/note.txt' : path} />
				<textarea name="content" rows={6} placeholder="file contents" required />
				<button type="submit">Write file</button>
			</form>
			</Can>
			{editorPath ? (
				<section>
					<h2>Edit {editorPath}</h2>
					<textarea aria-label="Editor contents" value={editorContent} onChange={(event) => setEditorContent(event.target.value)} rows={8} />
					<button type="button" onClick={() => void saveEditor()}>Save file</button>
				</section>
			) : null}
			<ul>
				{items.map((entry) => (
					<li key={entry.name}>
						{entry.dir
							? <button type="button" className="link" onClick={() => setPath((current) => (current.endsWith('/') ? current : current + '/') + entry.name)}>{entry.name}/</button>
							: <button type="button" className="link" onClick={() => void readFile(entry)}>{entry.name} ({entry.size})</button>}
					</li>
				))}
			</ul>
		</>
	)
}
