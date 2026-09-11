import { FormEvent, useEffect, useState } from 'react'
import { api, asList } from '../client'
import { EmptyState, ErrorState, LoadingState } from '../components/ui'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'

interface HostRecipe {
	id: string
	label: string
	description: string
}

export function HostConsolePanel () {
	const canWrite = useCan('server.settings.write')
	const [recipes, setRecipes] = useState<HostRecipe[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')
	const [output, setOutput] = useState('')
	const [busy, setBusy] = useState('')

	function load () {
		setLoading(true)
		setError('')
		api<{ items?: HostRecipe[] }>('/api/v1/server/console/recipes').then((result) => {
			setRecipes(asList(result))
		}).catch((requestError) => {
			setError(messageFrom(requestError))
		}).finally(() => setLoading(false))
	}

	useEffect(load, [])

	async function runRecipe (id: string) {
		setBusy(id)
		setOutput('')
		try {
			const result = await api<{ message?: string }>('/api/v1/server/console', {
				method: 'POST',
				body: JSON.stringify({ id }),
			})
			setOutput(result.message || 'Completed.')
		} catch (requestError) {
			setOutput(messageFrom(requestError))
		} finally {
			setBusy('')
		}
	}

	return (
		<section className="panel">
			<h2>Host console</h2>
			<p>Run typed host recipes through the privileged agent. Each run is audited. This is not a freeform root shell.</p>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{loading ? <LoadingState label="Loading recipes…" /> : null}
			{!loading && !recipes.length ? <EmptyState title="No recipes" detail="The API did not return host recipes." /> : null}
			<div className="tool-launch-grid">
				{recipes.map((recipe) => (
					<button
						key={recipe.id}
						type="button"
						className="tool-launch-card"
						disabled={!canWrite || Boolean(busy)}
						onClick={() => runRecipe(recipe.id)}
					>
						<strong>{recipe.label}</strong>
						<span>{busy === recipe.id ? 'Running…' : recipe.description}</span>
					</button>
				))}
			</div>
			{!canWrite ? <p className="subtle">Your role can review recipes. Running them requires server settings write.</p> : null}
			{output ? <pre className="console-output">{output}</pre> : null}
		</section>
	)
}

export function HostPasswordForm ({
	title,
	endpoint,
	includeCurrent,
}: {
	title: string
	endpoint: string
	includeCurrent?: boolean
}) {
	const canWrite = useCan(endpoint.includes('database') ? 'databases.write' : 'server.settings.write')
	const [message, setMessage] = useState('')

	async function submit (event: FormEvent<HTMLFormElement>) {
		event.preventDefault()
		const data = new FormData(event.currentTarget)
		setMessage('')
		try {
			await api(endpoint, {
				method: 'POST',
				body: JSON.stringify({
					current: data.get('current') || '',
					password: data.get('password'),
				}),
			})
			setMessage('Password applied on the host. Kelmor does not store it.')
			event.currentTarget.reset()
		} catch (requestError) {
			setMessage(messageFrom(requestError))
		}
	}

	return (
		<section className="panel">
			<h2>{title}</h2>
			<p>The new password is sent once to the typed agent and is never written to Director settings.</p>
			{canWrite ? <form className="stack-form" onSubmit={submit}>
				{includeCurrent ? <label>Current password<input name="current" type="password" autoComplete="current-password" /></label> : null}
				<label>New password<input name="password" type="password" minLength={8} required autoComplete="new-password" /></label>
				<button type="submit">Set password</button>
			</form> : <p className="subtle">Your role cannot rotate this password.</p>}
			{message ? <p className="feedback" role="status">{message}</p> : null}
		</section>
	)
}
