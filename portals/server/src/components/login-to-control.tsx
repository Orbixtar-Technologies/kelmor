import { useState } from 'react'
import { api } from '../client'
import { assignOpenedWindow, controlImpersonationUrl, controlPortalOrigin } from '../control-url'
import { messageFrom } from '../helpers'
import { useCan } from '../rbac'
import { Dialog } from './ui'

interface LoginToControlProps {
	accountId: string
	username: string
	variant?: 'button' | 'link'
	autoOpen?: boolean
}

interface ImpersonateResult {
	token: string
	control_url?: string
}

export function LoginToControl ({ accountId, username, variant = 'link', autoOpen = false }: LoginToControlProps) {
	const canImpersonate = useCan('accounts.impersonate')
	const [open, setOpen] = useState(autoOpen)
	const [reason, setReason] = useState('Operator requested Kelmor Control access')
	const [error, setError] = useState('')
	const [busy, setBusy] = useState(false)
	if (!canImpersonate) return null

	async function submit (event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault()
		setBusy(true)
		setError('')
		const popup = window.open('about:blank', '_blank')
		try {
			const result = await api<ImpersonateResult>(`/api/v1/accounts/${accountId}/impersonate`, {
				method: 'POST',
				body: JSON.stringify({ reason }),
			})
			const origin = controlPortalOrigin(window.location, { controlUrl: result.control_url })
			assignOpenedWindow(popup, controlImpersonationUrl(origin, result.token))
			setOpen(false)
		} catch (requestError) {
			popup?.close()
			setError(messageFrom(requestError))
		} finally {
			setBusy(false)
		}
	}

	return (
		<>
			{variant === 'button'
				? <button type="button" onClick={() => setOpen(true)}>Login to Control</button>
				: <button type="button" className="link-button" onClick={() => setOpen(true)}>Login to Control</button>}
			<Dialog open={open} title={`Login to Kelmor Control as ${username}`} onClose={() => setOpen(false)}>
				<form onSubmit={submit}>
					<p>Starts a 30-minute audited Control session for this account owner. The reason is stored on the session and in the audit trail. Control opens at the configured HTTPS tenant URL, not this Director host.</p>
					<label>Reason<input value={reason} onChange={(event) => setReason(event.target.value)} required minLength={4} autoFocus /></label>
					{error ? <p className="field-error" role="alert">{error}</p> : null}
					<footer className="dialog-form-actions">
						<button type="button" className="secondary" onClick={() => setOpen(false)}>Cancel</button>
						<button type="submit" disabled={busy}>{busy ? 'Opening…' : 'Open Kelmor Control'}</button>
					</footer>
				</form>
			</Dialog>
		</>
	)
}
