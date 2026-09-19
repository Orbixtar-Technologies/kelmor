import { FormEvent, useEffect, useState } from 'react'
import { api } from '../client'
import { QueuedOpNotice, queuedOpMessage } from '../components/queued-op-notice'
import { ErrorState, PageHeader } from '../components/ui'
import { messageFrom } from '../helpers'
import type { Me } from '../types'

interface ServerSettings {
	values?: Record<string, Record<string, string>>
}

export function TwoFactorPage () {
	const [me, setMe] = useState<Me | null>(null)
	const [required, setRequired] = useState(false)
	const [secret, setSecret] = useState('')
	const [otpauth, setOtpauth] = useState('')
	const [code, setCode] = useState('')
	const [error, setError] = useState('')
	const [message, setMessage] = useState('')
	const [jobId, setJobId] = useState('')

	function load () {
		api<Me>('/api/v1/me').then(setMe).catch((reason) => setError(messageFrom(reason)))
		api<ServerSettings>('/api/v1/server/settings').then((result) => {
			setRequired(result.values?.two_factor?.required === 'on' || result.values?.security_policies?.require_2fa === 'on')
		}).catch(() => undefined)
	}
	useEffect(load, [])

	async function handleEnroll () {
		setError('')
		try {
			const result = await api<{ secret: string; otpauth_url: string }>('/api/v1/auth/totp/enroll', { method: 'POST', body: '{}' })
			setSecret(result.secret)
			setOtpauth(result.otpauth_url)
			setMessage('Scan the otpauth URL, then confirm with a code.')
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	async function handleConfirm (event: FormEvent) {
		event.preventDefault()
		setError('')
		try {
			await api('/api/v1/auth/totp/confirm', { method: 'POST', body: JSON.stringify({ code }) })
			setSecret('')
			setOtpauth('')
			setCode('')
			setMessage('Authenticator enrolled. Sign-in now requires a TOTP code.')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	async function handleDisable (event: FormEvent) {
		event.preventDefault()
		setError('')
		try {
			await api('/api/v1/auth/totp/disable', { method: 'POST', body: JSON.stringify({ code }) })
			setCode('')
			setMessage('Authenticator removed for this operator.')
			load()
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	async function handlePolicy (event: FormEvent) {
		event.preventDefault()
		setError('')
		try {
			const result = await api<{ operation_id?: string }>('/api/v1/server/settings', {
				method: 'PATCH',
				body: JSON.stringify({ values: { two_factor: { required: required ? 'on' : 'off' } } }),
			})
			setJobId(result.operation_id || '')
			setMessage(queuedOpMessage(result, 'Host two-factor policy queued.'))
		} catch (reason) {
			setError(messageFrom(reason))
		}
	}

	const enabled = Boolean(me?.user.totp_enabled)

	return (
		<>
			<PageHeader
				title="Two-Factor Authentication"
				description="Enroll a TOTP authenticator for this Director operator. The host policy writes /etc/panel/two-factor.json."
			/>
			{error ? <ErrorState error={error} onRetry={load} /> : null}
			{message ? <QueuedOpNotice message={message} jobId={jobId} /> : null}
			<section className="panel">
				<p>This operator: <strong>{enabled ? 'TOTP enrolled' : 'TOTP not enrolled'}</strong></p>
				{!enabled ? <button type="button" onClick={handleEnroll}>Generate authenticator secret</button> : null}
				{otpauth ? <p>Add this URL to your authenticator: <code>{otpauth}</code></p> : null}
				{secret ? <p>Secret: <code>{secret}</code></p> : null}
				<form onSubmit={enabled ? handleDisable : handleConfirm}>
					<label>
						Authenticator code
						<input value={code} onChange={(event) => setCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" required />
					</label>
					<button type="submit">{enabled ? 'Disable TOTP' : 'Confirm enroll'}</button>
				</form>
			</section>
			<form className="panel" onSubmit={handlePolicy}>
				<label>
					<input type="checkbox" checked={required} onChange={(event) => setRequired(event.target.checked)} />
					Require TOTP for server administrators
				</label>
				<button type="submit">Apply on host</button>
			</form>
		</>
	)
}
