import { FormEvent, useEffect, useState } from 'react'
import { api, asList } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

export type ModuleKind = 'perl' | 'pear' | 'pecl' | 'ruby'

interface LanguageModule {
	kind: ModuleKind
	name: string
	status: string
	source?: string
}

const KIND_LABEL: Record<ModuleKind, string> = {
	perl: 'Perl module',
	pear: 'PEAR package',
	pecl: 'PECL extension',
	ruby: 'Ruby gem',
}

const KIND_FIELD: Record<ModuleKind, string> = {
	perl: 'Module',
	pear: 'Package',
	pecl: 'Extension',
	ruby: 'Gem',
}

interface HostModulesPanelProps {
	kind?: ModuleKind
}

export function HostModulesPanel ({ kind }: HostModulesPanelProps) {
	const canWrite = useCan('server.settings.write')
	const [items, setItems] = useState<LanguageModule[]>([])
	const [name, setName] = useState('')
	const [selectedKind, setSelectedKind] = useState<ModuleKind>(kind || 'pecl')
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')
	const [busy, setBusy] = useState(false)
	const kinds: ModuleKind[] = kind ? [kind] : ['pecl', 'pear', 'perl', 'ruby']

	function load () {
		setLoading(true)
		setError('')
		const query = kind ? `?kind=${encodeURIComponent(kind)}` : ''
		api<{ items?: LanguageModule[] }>(`/api/v1/server/modules${query}`)
			.then((result) => setItems(asList(result)))
			.catch((reason) => setError(messageFrom(reason)))
			.finally(() => setLoading(false))
	}

	useEffect(load, [kind])

	async function handleInstall (event: FormEvent) {
		event.preventDefault()
		setBusy(true)
		setError('')
		setMessage('')
		setJobId('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/modules', {
				method: 'POST',
				body: JSON.stringify({ kind: selectedKind, name: name.trim() }),
			})
			setMessage(queuedOpMessage(result, `${KIND_LABEL[selectedKind]} install queued.`))
			setJobId(result.operation_id || '')
			setName('')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		} finally {
			setBusy(false)
		}
	}

	return (
		<section className="panel">
			<h2>{kind ? KIND_LABEL[kind] : 'Language modules'}</h2>
			<p>Installs go through a typed Agent job. PHP extensions prefer the matching php-X.Y-* package, then pecl. Perl prefers lib*-perl, then cpan.</p>
			<QueuedOpNotice message={message} jobId={jobId} />
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{canWrite ? (
				<form className="form-grid" onSubmit={handleInstall}>
					{!kind ? (
						<label>Kind
							<select value={selectedKind} onChange={(event) => setSelectedKind(event.target.value as ModuleKind)}>
								{kinds.map((entry) => <option key={entry} value={entry}>{KIND_LABEL[entry]}</option>)}
							</select>
						</label>
					) : null}
					<label>{KIND_FIELD[selectedKind]}
						<input value={name} onChange={(event) => setName(event.target.value)} required maxLength={128} />
					</label>
					<button type="submit" disabled={busy || !name.trim()}>{busy ? 'Queueing…' : 'Install on host'}</button>
				</form>
			) : null}
			{loading ? <LoadingState label="Loading host modules…" /> : null}
			{!loading && items.length ? (
				<div className="table-wrap"><table className="dense-table">
					<thead><tr><th>Kind</th><th>Name</th><th>Status</th></tr></thead>
					<tbody>
						{items.map((item) => (
							<tr key={`${item.kind}:${item.name}`}>
								<td>{KIND_LABEL[item.kind] || item.kind}</td>
								<td><code>{item.name}</code></td>
								<td><StatusBadge value={item.status} /></td>
							</tr>
						))}
					</tbody>
				</table></div>
			) : null}
			{!loading && !items.length ? <EmptyState title="No host modules recorded" detail="Queue an install to apply a module on this Ubuntu host." /> : null}
		</section>
	)
}

export function moduleKindForFeature (id: string): ModuleKind | undefined {
	switch (id) {
		case 'perl-modules':
			return 'perl'
		case 'php-pear':
			return 'pear'
		case 'php-pecl':
			return 'pecl'
		case 'ruby-gems':
			return 'ruby'
		case 'module-installers':
			return undefined
		default:
			return undefined
	}
}
