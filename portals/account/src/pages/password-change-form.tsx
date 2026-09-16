import { useRef, useState } from 'react'
import { api, APIClientError, clearToken, setToken } from '../client'
import { createPendingGuard } from '../pending-submit'

interface Me {
	user: { username: string; roles: string[] }
	actor: { account_ids?: string[]; capabilities?: Record<string, boolean> }
}

interface PasswordChangeFormProps {
	username: string
	currentPassword: string
	onSignedIn: (me: Me) => void
}

export function PasswordChangeForm ({ username, currentPassword, onSignedIn }: PasswordChangeFormProps) {
	const [newPassword, setNewPassword] = useState('')
	const [confirmPassword, setConfirmPassword] = useState('')
	const [error, setError] = useState('')
	const pending = useRef(createPendingGuard()).current

	return (
		<form onSubmit={async (event) => {
			event.preventDefault()
			if (!pending.tryStart()) return
			setError('')
			if (newPassword !== confirmPassword) {
				setError('New passwords do not match')
				pending.finish()
				return
			}
			try {
				await api('/api/v1/auth/complete-password-change', {
					method: 'POST',
					body: JSON.stringify({
						username,
						current_password: currentPassword,
						new_password: newPassword,
					}),
				})
				const login = await api<{ token: string }>('/api/v1/auth/login', {
					method: 'POST',
					body: JSON.stringify({ username, password: newPassword }),
				})
				setToken(login.token)
				onSignedIn(await api<Me>('/api/v1/me'))
			} catch (requestError) {
				clearToken()
				if (requestError instanceof APIClientError) setError(requestError.message)
				else setError(requestError instanceof Error ? requestError.message : 'Password change failed')
			} finally {
				pending.finish()
			}
		}}>
			<label>Username<input value={username} readOnly autoComplete="username" /></label>
			<label>Current password<input value={currentPassword} type="password" readOnly autoComplete="current-password" /></label>
			<label>New password<input value={newPassword} onChange={(event) => setNewPassword(event.target.value)} type="password" minLength={12} required autoComplete="new-password" autoFocus /></label>
			<label>Confirm new password<input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" minLength={12} required autoComplete="new-password" /></label>
			{error ? <p className="error" role="alert">{error}</p> : null}
			<button type="submit">Change password and sign in</button>
		</form>
	)
}
