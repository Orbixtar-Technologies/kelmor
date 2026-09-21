import { FormEvent, useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, asList } from '../client'
import { AccountPicker } from '../components/account-picker'
import { EmptyState, ErrorState, LoadingState } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { RequestSequence } from '../request-sequence'
import { useCan } from '../rbac'
import type { Account } from '../types'

interface WebsiteRow {
	id: string
	domain_id?: string
	document_root?: string
}

interface DomainRow {
	id: string
	ascii_fqdn?: string
}

interface HotlinkPolicy {
	website_id?: string
	enabled?: boolean
	allow_direct?: boolean
	extensions?: string[]
	allowed_referers?: string[]
}

interface ImageRow {
	path?: string
	name?: string
	size?: number
}

function websiteLabel (website: WebsiteRow, domains: DomainRow[]) {
	const domain = domains.find((entry) => entry.id === website.domain_id)
	return domain?.ascii_fqdn || website.document_root || website.id
}

export function HotlinkPanel () {
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [accountId, setAccountId] = useState('')
	const [websites, setWebsites] = useState<WebsiteRow[]>([])
	const [domains, setDomains] = useState<DomainRow[]>([])
	const [websiteId, setWebsiteId] = useState('')
	const [enabled, setEnabled] = useState(false)
	const [allowDirect, setAllowDirect] = useState(true)
	const [extensions, setExtensions] = useState('jpg, jpeg, png, gif, webp, svg')
	const [referers, setReferers] = useState('')
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const requests = useRef(new RequestSequence()).current
	const canWrite = useCan('websites.write')
	const account = accounts.find((entry) => entry.id === accountId)

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!accountId && next[0]) setAccountId(next[0].id)
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [accountId, requests])

	const loadPolicy = useCallback((requestedAccount: string, requestedWebsite: string) => {
		if (!requestedAccount || !requestedWebsite) return
		const request = requests.begin('hotlink')
		setLoading(true)
		setError('')
		api<HotlinkPolicy>(`/api/v1/accounts/${requestedAccount}/websites/${requestedWebsite}/hotlink`).then((result) => {
			if (!requests.isCurrent(request)) return
			setEnabled(Boolean(result.enabled))
			setAllowDirect(result.allow_direct !== false)
			setExtensions((result.extensions || []).join(', '))
			setReferers((result.allowed_referers || []).join('\n'))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request)) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		if (!accountId) {
			setWebsites([])
			setDomains([])
			setWebsiteId('')
			return
		}
		const request = requests.begin('websites')
		Promise.all([
			api<{ items: WebsiteRow[] }>(`/api/v1/accounts/${accountId}/websites`),
			api<{ items: DomainRow[] }>(`/api/v1/accounts/${accountId}/domains`),
		]).then(([siteResult, domainResult]) => {
			if (!requests.isCurrent(request)) return
			const nextSites = asList(siteResult)
			setWebsites(nextSites)
			setDomains(asList(domainResult))
			setWebsiteId((current) => current && nextSites.some((site) => site.id === current) ? current : (nextSites[0]?.id || ''))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [accountId, requests])

	useEffect(() => {
		if (accountId && websiteId) loadPolicy(accountId, websiteId)
	}, [accountId, loadPolicy, websiteId])

	async function savePolicy (event: FormEvent) {
		event.preventDefault()
		if (!accountId || !websiteId) return
		setMessage('')
		try {
			await api(`/api/v1/accounts/${accountId}/websites/${websiteId}/hotlink`, {
				method: 'PUT',
				body: JSON.stringify({
					enabled,
					allow_direct: allowDirect,
					extensions: extensions.split(/[\s,]+/).filter(Boolean),
					allowed_referers: referers.split(/\n/).map((line) => line.trim()).filter(Boolean),
				}),
			})
			setMessage('Hotlink policy saved. nginx apply was queued on the host.')
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<section className="panel">
			<p>Block off-site embedding of images and other static files with nginx <code>valid_referers</code>. Empty referer lists still allow this vhost&apos;s server names.</p>
			<AccountPicker
				accounts={accounts}
				value={accountId}
				filter={accountFilter}
				onFilterChange={setAccountFilter}
				onChange={setAccountId}
			/>
			{error ? <ErrorState error={error} onRetry={() => loadPolicy(accountId, websiteId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to review its vhost hotlink policy." /> : null}
			{accountId && !websites.length && !loading ? (
				<EmptyState
					title="No websites"
					detail="This account has no nginx vhosts yet."
					action={<Link to={`/websites?account=${accountId}`}>Open MultiPHP Manager</Link>}
				/>
			) : null}
			{accountId && websites.length ? (
				<form className="stack-form" onSubmit={savePolicy}>
					<label>
						Website
						<select value={websiteId} onChange={(event) => setWebsiteId(event.target.value)}>
							{websites.map((website) => (
								<option key={website.id} value={website.id}>{websiteLabel(website, domains)}</option>
							))}
						</select>
					</label>
					{loading ? <LoadingState label="Loading hotlink policy…" /> : null}
					<label className="checkbox-label">
						<input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
						Enable hotlink protection
					</label>
					<label className="checkbox-label">
						<input type="checkbox" checked={allowDirect} onChange={(event) => setAllowDirect(event.target.checked)} />
						Allow direct / empty-referer requests
					</label>
					<label>
						Protected extensions
						<input value={extensions} onChange={(event) => setExtensions(event.target.value)} />
					</label>
					<label>
						Allowed referer hosts
						<textarea value={referers} onChange={(event) => setReferers(event.target.value)} rows={4} placeholder="cdn.example.com" />
					</label>
					{canWrite ? <button type="submit">Save hotlink policy</button> : <p className="subtle">Your role can inspect this policy.</p>}
					{message ? <p className="feedback" role="status">{message}</p> : null}
					{account ? <p className="subtle">Account home: <code>{account.home_path}</code></p> : null}
				</form>
			) : null}
		</section>
	)
}

export function ImageManagerPanel () {
	const [accounts, setAccounts] = useState<Account[]>([])
	const [accountFilter, setAccountFilter] = useState('')
	const [accountId, setAccountId] = useState('')
	const [websites, setWebsites] = useState<WebsiteRow[]>([])
	const [domains, setDomains] = useState<DomainRow[]>([])
	const [websiteId, setWebsiteId] = useState('')
	const [images, setImages] = useState<ImageRow[]>([])
	const [root, setRoot] = useState('')
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const requests = useRef(new RequestSequence()).current

	useEffect(() => {
		const request = requests.begin('accounts')
		api<{ items: Account[] }>('/api/v1/accounts').then((result) => {
			if (!requests.isCurrent(request)) return
			const next = asList(result)
			setAccounts(next)
			if (!accountId && next[0]) setAccountId(next[0].id)
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [accountId, requests])

	useEffect(() => {
		if (!accountId) {
			setWebsites([])
			setDomains([])
			setWebsiteId('')
			return
		}
		const request = requests.begin('websites')
		Promise.all([
			api<{ items: WebsiteRow[] }>(`/api/v1/accounts/${accountId}/websites`),
			api<{ items: DomainRow[] }>(`/api/v1/accounts/${accountId}/domains`),
		]).then(([siteResult, domainResult]) => {
			if (!requests.isCurrent(request)) return
			const nextSites = asList(siteResult)
			setWebsites(nextSites)
			setDomains(asList(domainResult))
			setWebsiteId((current) => current && nextSites.some((site) => site.id === current) ? current : (nextSites[0]?.id || ''))
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		})
	}, [accountId, requests])

	const loadImages = useCallback((requestedAccount: string, requestedWebsite: string) => {
		if (!requestedAccount) return
		const request = requests.begin('images')
		setLoading(true)
		setError('')
		const query = requestedWebsite ? `?website_id=${encodeURIComponent(requestedWebsite)}` : ''
		api<{ items?: ImageRow[]; root?: string }>(`/api/v1/accounts/${requestedAccount}/images${query}`).then((result) => {
			if (!requests.isCurrent(request)) return
			setImages(asList(result))
			setRoot(result.root || '')
		}).catch((requestError) => {
			if (requests.isCurrent(request)) setError(messageFrom(requestError))
		}).finally(() => {
			if (requests.isCurrent(request)) setLoading(false)
		})
	}, [requests])

	useEffect(() => {
		if (accountId) loadImages(accountId, websiteId)
		else setImages([])
	}, [accountId, loadImages, websiteId])

	return (
		<section className="panel">
			<p>Lists image files under the selected document root. Kelmor does not invent thumbnails or rewrite files here — edit them in File Manager.</p>
			<AccountPicker
				accounts={accounts}
				value={accountId}
				filter={accountFilter}
				onFilterChange={setAccountFilter}
				onChange={setAccountId}
			/>
			{accountId && websites.length ? (
				<label>
					Website
					<select value={websiteId} onChange={(event) => setWebsiteId(event.target.value)}>
						{websites.map((website) => (
							<option key={website.id} value={website.id}>{websiteLabel(website, domains)}</option>
						))}
					</select>
				</label>
			) : null}
			{error ? <ErrorState error={error} onRetry={() => loadImages(accountId, websiteId)} /> : null}
			{!accountId ? <EmptyState title="Select an account" detail="Choose a hosting account to list images in its document root." /> : null}
			{loading ? <LoadingState label="Scanning images…" /> : null}
			{root ? <p className="subtle">Document root: <code>{root}</code></p> : null}
			{!loading && accountId && !images.length ? (
				<EmptyState
					title="No images found"
					detail="No jpg, png, gif, webp, svg, or ico files under this document root."
					action={accountId ? <Link to={`/files?account=${accountId}`}>Open File Manager</Link> : undefined}
				/>
			) : null}
			{!loading && images.length ? (
				<div className="table-wrap">
					<table className="dense-table">
						<thead><tr><th>Name</th><th>Path</th><th>Size</th></tr></thead>
						<tbody>
							{images.map((image) => (
								<tr key={`${image.path}-${image.name}`}>
									<td><strong>{image.name || image.path}</strong></td>
									<td><code>{image.path || '—'}</code></td>
									<td>{formatBytes(image.size)}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			) : null}
		</section>
	)
}
