import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, asList } from '../client'
import { useCapabilities } from '../rbac'
import type { Account, FindResult, ResourceItem, ToolDefinition } from '../types'

export interface SearchableResource {
	id: string
	label: string
	description: string
	path: string
}

export interface AccountResourceCollections {
	databases?: ResourceItem[]
	domains?: ResourceItem[]
	websites?: ResourceItem[]
}

function matchScore (query: string, primary: string, secondary: string): number {
	const needle = query.toLocaleLowerCase()
	const label = primary.toLocaleLowerCase()
	const detail = secondary.toLocaleLowerCase()
	if (label === needle) return 100
	if (label.startsWith(needle)) return 80
	if (label.includes(needle)) return 60
	if (detail.startsWith(needle)) return 40
	if (detail.includes(needle)) return 20
	return 0
}

export function resourcesFromAccountCollections (account: Account, collections: AccountResourceCollections): SearchableResource[] {
	const resources: SearchableResource[] = []
	for (const database of collections.databases || []) {
		const name = typeof database.name === 'string' ? database.name : ''
		if (!name) continue
		resources.push({
			id: `${account.id}-database-${database.id}`,
			label: name,
			description: `Database · ${account.username}`,
			path: `/sql?account=${account.id}`,
		})
	}
	for (const domain of collections.domains || []) {
		const name = typeof domain.ascii_fqdn === 'string' ? domain.ascii_fqdn : typeof domain.fqdn === 'string' ? domain.fqdn : ''
		if (!name) continue
		resources.push({
			id: `${account.id}-domain-${domain.id}`,
			label: name,
			description: `Domain · ${account.username}`,
			path: `/dns?account=${account.id}`,
		})
	}
	for (const website of collections.websites || []) {
		const name = typeof website.hostname === 'string' ? website.hostname : typeof website.document_root === 'string' ? website.document_root : ''
		if (!name) continue
		resources.push({
			id: `${account.id}-website-${website.id}`,
			label: name,
			description: `Website · ${account.username}`,
			path: `/files?account=${account.id}`,
		})
	}
	return resources
}

export function rankFindResults (query: string, tools: ToolDefinition[], accounts: Account[], resources: SearchableResource[] = []): FindResult[] {
	const needle = query.trim()
	if (!needle) return []
	const toolResults = tools.map((tool) => ({
		id: tool.id,
		label: tool.label,
		description: `${tool.description} · ${tool.category}`,
		path: tool.path,
		kind: 'tool' as const,
		score: matchScore(needle, tool.label, `${tool.description} ${tool.category}`),
	}))
	const accountResults = accounts.map((account) => ({
		id: account.id,
		label: account.username,
		description: `${account.primary_domain} · ${account.status}`,
		path: `/accounts/${account.id}`,
		kind: 'account' as const,
		score: Math.max(matchScore(needle, account.username, account.primary_domain), matchScore(needle, account.primary_domain, account.username) - 1),
	}))
	const resourceResults = resources.map((resource) => ({
		...resource,
		kind: 'resource' as const,
		score: matchScore(needle, resource.label, resource.description),
	}))
	return [...toolResults, ...accountResults, ...resourceResults]
		.filter((result) => result.score > 0)
		.sort((left, right) => right.score - left.score || left.label.localeCompare(right.label))
		.slice(0, 12)
}

export function shouldHandleFindShortcut (key: string, modified: boolean, target: EventTarget | null): boolean {
	if (key !== '/' || modified) return false
	if (!(target instanceof HTMLElement)) return true
	return !target.matches('input, textarea, select, [contenteditable="true"]')
}

interface GlobalFindProps {
	tools: ToolDefinition[]
	accounts: Account[]
}

export function GlobalFind ({ tools, accounts }: GlobalFindProps) {
	const capabilities = useCapabilities()
	const [query, setQuery] = useState('')
	const [open, setOpen] = useState(false)
	const [activeIndex, setActiveIndex] = useState(0)
	const [resources, setResources] = useState<SearchableResource[]>([])
	const loadedAccountsRef = useRef('')
	const inputRef = useRef<HTMLInputElement>(null)
	const navigate = useNavigate()
	const results = rankFindResults(query, tools, accounts, resources)

	async function loadResources () {
		const accountKey = accounts.map((account) => account.id).join(',')
		const canReadDatabases = Boolean(capabilities['databases.read'])
		const canReadDomains = Boolean(capabilities['domains.read'])
		if (!accountKey || loadedAccountsRef.current === accountKey || (!canReadDatabases && !canReadDomains)) return
		loadedAccountsRef.current = accountKey
		const collected = await Promise.all(accounts.slice(0, 20).map(async (account) => {
			const [databases, domains] = await Promise.all([
				canReadDatabases ? api<{ items: ResourceItem[] }>(`/api/v1/accounts/${account.id}/databases`).then(asList).catch(() => []) : Promise.resolve([]),
				canReadDomains ? api<{ items: ResourceItem[] }>(`/api/v1/accounts/${account.id}/domains`).then(asList).catch(() => []) : Promise.resolve([]),
			])
			return resourcesFromAccountCollections(account, { databases, domains })
		}))
		setResources(collected.flat())
	}
	const activeResultId = results[activeIndex] ? `global-find-option-${results[activeIndex].kind}-${results[activeIndex].id}` : undefined

	useEffect(() => {
		function onKeyDown (event: KeyboardEvent) {
			if (!shouldHandleFindShortcut(event.key, event.metaKey || event.ctrlKey || event.altKey, event.target)) return
			event.preventDefault()
			setOpen(true)
			inputRef.current?.focus()
		}
		window.addEventListener('keydown', onKeyDown)
		return () => window.removeEventListener('keydown', onKeyDown)
	}, [])
	useEffect(() => {
		setActiveIndex(0)
	}, [query])

	function choose (result: FindResult) {
		navigate(result.path)
		setOpen(false)
		setQuery('')
	}

	return (
		<div className="global-find">
			<label className="sr-only" htmlFor="global-find">Search tools and accounts</label>
			<span aria-hidden="true" className="find-icon">⌕</span>
			<input
				id="global-find"
				ref={inputRef}
				value={query}
				placeholder="Search tools and accounts"
				autoComplete="off"
				role="combobox"
				aria-autocomplete="list"
				aria-expanded={open && Boolean(query)}
				aria-controls="global-find-results"
				aria-activedescendant={open ? activeResultId : undefined}
				onFocus={() => { setOpen(true); void loadResources() }}
				onChange={(event) => { setQuery(event.target.value); setOpen(true) }}
				onKeyDown={(event) => {
					if (event.key === 'Escape') { setOpen(false); inputRef.current?.blur() }
					if (event.key === 'ArrowDown' && results.length) {
						event.preventDefault()
						setActiveIndex((current) => (current + 1) % results.length)
					}
					if (event.key === 'ArrowUp' && results.length) {
						event.preventDefault()
						setActiveIndex((current) => (current - 1 + results.length) % results.length)
					}
					if (event.key === 'Enter' && results[activeIndex]) choose(results[activeIndex])
				}}
			/>
			<kbd>/</kbd>
			{open && query ? (
				<div id="global-find-results" className="find-results" role="listbox" aria-label="Find results">
					{results.map((result, index) => (
						<button id={`global-find-option-${result.kind}-${result.id}`} key={`${result.kind}-${result.id}`} type="button" role="option" aria-selected={index === activeIndex} onMouseEnter={() => setActiveIndex(index)} onMouseDown={(event) => event.preventDefault()} onClick={() => choose(result)}>
							<span><strong>{result.label}</strong><small>{result.description}</small></span>
							<em>{result.kind}</em>
						</button>
					))}
					<p className="find-scope">Matches tools, account names and domains, plus loaded databases and domains.</p>
					{results.length === 0 ? <p>No matching tools, accounts, or resources.</p> : null}
				</div>
			) : null}
		</div>
	)
}
