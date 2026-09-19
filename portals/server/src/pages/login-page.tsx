import { useState } from 'react'
import { api, setToken } from '../client'
import type { Me, User } from '../types'

interface LoginPageProps {
	onLogin: (me: Me) => void
}

export function LoginPage ({ onLogin }: LoginPageProps) {
	const [error, setError] = useState('')
	return (
		<main className="auth">
			<section className="login-card">
				<div className="login-brand">
					<span>K</span>
					<div>
						<strong>Kelmor Director</strong>
						<small>Server administration</small>
					</div>
				</div>
				<h1>Sign in</h1>
				<p>Access your Kelmor host, accounts, services, and background operations.</p>
				<form onSubmit={async (event) => {
					event.preventDefault()
					setError('')
					const data = new FormData(event.currentTarget)
					try {
						const result = await api<{ token: string; user: User }>('/api/v1/auth/login', {
							method: 'POST',
							body: JSON.stringify({
								username: data.get('username'),
								password: data.get('password'),
								totp_code: data.get('totp_code'),
							}),
						})
						setToken(result.token)
						onLogin(await api<Me>('/api/v1/me'))
					} catch (requestError) {
						setError(requestError instanceof Error ? requestError.message : 'Sign in failed.')
					}
				}}
				>
					<label>Username<input name="username" autoComplete="username" required autoFocus /></label>
					<label>Password<input name="password" type="password" autoComplete="current-password" required /></label>
					<label>Authenticator code<input name="totp_code" inputMode="numeric" autoComplete="one-time-code" placeholder="Required after TOTP enroll" /></label>
					{error ? <p className="field-error" role="alert">{error}</p> : null}
					<button type="submit">Sign in to Kelmor Director</button>
				</form>
			</section>
		</main>
	)
}
