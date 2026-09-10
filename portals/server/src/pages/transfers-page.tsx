import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { PageHeader, SectionHeading } from '../components/ui'
import { downloadJSON, formatDate, messageFrom } from '../helpers'
import { useCan } from '../rbac'
import type { Account } from '../types'
import { summarizeExport, transferRollbackCopy } from './transfer-copy'

export function TransfersPage () {
	const [accounts, setAccounts] = useState<Account[]>([])
	const [message, setMessage] = useState('')
	const [nativeReview, setNativeReview] = useState<{ raw: string; username: string; domain: string; filename: string } | null>(null)
	const [archiveReview, setArchiveReview] = useState<{ root: string; username: string } | null>(null)
	const [copyReview, setCopyReview] = useState<{ sourceId: string; sourceName: string; username: string; domain: string } | null>(null)
	const [updatedAt, setUpdatedAt] = useState('')
	const canReadAccounts = useCan('accounts.read')
	const canCreateAccounts = useCan('accounts.create')
	const canReadBackups = useCan('backups.read')
	const canOperateHostImports = useCan('server.read')
	useEffect(() => {
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			setAccounts(asList(result))
			setUpdatedAt(new Date().toISOString())
		}).catch(() => setAccounts([]))
	}, [])

	async function importNative () {
		if (!nativeReview) return
		const query = new URLSearchParams()
		if (nativeReview.username) query.set('username', nativeReview.username)
		if (nativeReview.domain) query.set('domain', nativeReview.domain)
		try {
			const result = await api<{ resource_id: string }>('/api/v1/accounts/import' + (query.size ? `?${query}` : ''), { method: 'POST', body: nativeReview.raw })
			setMessage(`Native import queued for account ${result.resource_id}.`); setNativeReview(null)
		} catch (error) { setMessage(messageFrom(error)) }
	}
	async function importArchive () {
		if (!archiveReview) return
		try {
			const result = await api<{ resource_id: string }>('/api/v1/accounts/import/cpanel', { method: 'POST', body: JSON.stringify(archiveReview) })
			setMessage(`Extracted account archive queued for ${result.resource_id}.`); setArchiveReview(null)
		} catch (error) { setMessage(messageFrom(error)) }
	}
	async function copyAccount () {
		if (!copyReview) return
		try {
			const result = await api<{ resource_id: string }>(`/api/v1/accounts/${copyReview.sourceId}/migrate`, { method: 'POST', body: JSON.stringify({ username: copyReview.username, domain: copyReview.domain }) })
			setMessage(`Account copy queued for ${result.resource_id}.`)
			setCopyReview(null)
		} catch (error) { setMessage(messageFrom(error)) }
	}
	return <>
		<PageHeader title="Transfers & Backups" description="Move account configuration through reviewable native and extracted-archive journeys." />
		{updatedAt ? <p className="subtle">Last updated {formatDate(updatedAt)}.</p> : null}
		{message ? <p className="feedback" role="status">{message}</p> : null}
		<div className="transfer-grid">
			{canReadAccounts ? <section className="panel"><SectionHeading title="Native account export" detail="Download a portable Kelmor account definition." /><label>Account<select id="export-account">{accounts.map((account) => <option key={account.id} value={account.id}>{account.username} — {account.primary_domain}</option>)}</select></label><button type="button" onClick={async () => { const select = document.querySelector<HTMLSelectElement>('#export-account'); const account = accounts.find((entry) => entry.id === select?.value); if (!account) return; try { const payload = await api(`/api/v1/accounts/${account.id}/export`); const raw = JSON.stringify(payload); const summary = summarizeExport(raw); downloadJSON(`${account.username}.kelmor-account.json`, payload); setMessage(`Native export downloaded${summary ? ` (${summary.sizeLabel}, ${summary.counts.domains} domains, ${summary.counts.mailboxes} mailboxes)` : ''}.`) } catch (error) { setMessage(messageFrom(error)) } }}>Download export</button></section> : null}
			{canCreateAccounts && canOperateHostImports ? <section className="panel"><SectionHeading title="Native account import" detail="Review file identity, object counts, and collision-safe overrides before importing." /><form onSubmit={async (event) => { event.preventDefault(); const data = new FormData(event.currentTarget); const file = data.get('export') as File; setNativeReview({ raw: await file.text(), filename: file.name, username: String(data.get('username') || ''), domain: String(data.get('domain') || '') }) }}><label>Export file<input name="export" type="file" accept="application/json" required /></label><label>New username (optional)<input name="username" /></label><label>New primary domain (optional)<input name="domain" /></label><button type="submit">Review import</button></form>{nativeReview ? <NativeImportReview review={nativeReview} onCancel={() => setNativeReview(null)} onConfirm={importNative} /> : null}</section> : null}
			{canCreateAccounts && canOperateHostImports ? <section className="panel"><SectionHeading title="Extracted account archive" detail="Import an archive already extracted into a protected host directory." /><form onSubmit={(event) => { event.preventDefault(); const data = new FormData(event.currentTarget); setArchiveReview({ root: String(data.get('root')), username: String(data.get('username')) }) }}><label>Extracted directory<input name="root" placeholder="/var/tmp/account-archive-user" required /></label><label>Username<input name="username" required /></label><button type="submit">Review archive</button></form>{archiveReview ? <div className="review-callout"><strong>Archive review</strong><p>{archiveReview.root} will be inspected for {archiveReview.username}; matching usernames or domains stop the import with a conflict.</p><p>Only import archives from trusted operators. {transferRollbackCopy()}</p><div className="button-row"><button type="button" className="secondary" onClick={() => setArchiveReview(null)}>Cancel</button><button type="button" onClick={importArchive}>Confirm import</button></div></div> : null}</section> : null}
			{canCreateAccounts && canReadAccounts ? <section className="panel"><SectionHeading title="Copy existing account" detail="Create a new account identity from an existing native account definition and home directory." /><form onSubmit={(event) => { event.preventDefault(); const data = new FormData(event.currentTarget); const sourceId = String(data.get('source_id')); const source = accounts.find((account) => account.id === sourceId); if (!source) return; setCopyReview({ sourceId, sourceName: source.username, username: String(data.get('username')), domain: String(data.get('domain')) }) }}><label>Source account<select name="source_id">{accounts.map((account) => <option key={account.id} value={account.id}>{account.username}</option>)}</select></label><label>New username<input name="username" required /></label><label>New primary domain<input name="domain" required /></label><button type="submit">Review account copy</button></form>{copyReview ? <div className="review-callout"><strong>Ready to copy {copyReview.sourceName}</strong><p>New username: {copyReview.username} · New domain: {copyReview.domain}</p><p>This queues an account.provision-style copy job. {transferRollbackCopy()}</p><div className="button-row"><button type="button" className="secondary" onClick={() => setCopyReview(null)}>Cancel</button><button type="button" onClick={copyAccount}>Confirm account copy</button></div></div> : null}</section> : null}
			{canReadBackups ? <section className="panel"><SectionHeading title="Backup and restore journeys" detail="Encrypted archives include website files, databases, and mail. Retention follows each account package. Restore replaces current account data in place." /><p>Local destinations are ready on this host. SFTP and S3 need host destination credentials before queueing. Open an account to review readiness, history, checksums, and restore.</p><ul className="link-list">{accounts.slice(0, 8).map((account) => <li key={account.id}><Link to={`/accounts/${account.id}/services?service=backups`}>{account.username} backups →</Link></li>)}</ul></section> : null}
		</div>
	</>
}

function NativeImportReview ({ review, onCancel, onConfirm }: { review: { raw: string; username: string; domain: string; filename: string }; onCancel: () => void; onConfirm: () => void }) {
	const summary = summarizeExport(review.raw)
	return <div className="review-callout">
		<strong>Ready to import {review.filename}</strong>
		<p>Override username: {review.username || 'From export'} · Override domain: {review.domain || 'From export'}</p>
		{summary ? <p>Source {summary.username || 'unknown'} · {summary.domain || 'no domain'} · format {summary.formatVersion} · {summary.sizeLabel} · {summary.exportedAt || 'export time unknown'} · {summary.counts.domains} domains, {summary.counts.websites} websites, {summary.counts.databases} databases, {summary.counts.mailboxes} mailboxes, {summary.counts.dns_records} DNS records.</p> : <p>The file is not valid Kelmor export JSON. Confirming may fail.</p>}
		<p>Matching usernames or domains stop the import. {transferRollbackCopy()}</p>
		<div className="button-row"><button type="button" className="secondary" onClick={onCancel}>Cancel</button><button type="button" onClick={onConfirm}>Confirm import</button></div>
	</div>
}
