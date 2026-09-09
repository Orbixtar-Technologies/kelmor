const RESERVED = [
	'root', 'panel', 'panel-agent', 'panel-backup', 'www-data',
	'nobody', 'postgres', 'mysql', 'kelmor', 'kelmor-agent',
	'kelmor-api', 'kelmor-worker',
]

export function validateUsername (raw: string) {
	const s = raw.trim()
	if (s.length < 2 || s.length > 32)
		return 'Username must be 2-32 characters'
	if (!/^[a-z][a-z0-9_-]*$/.test(s))
		return 'Username must be lowercase alphanumeric and start with a letter'
	if (RESERVED.includes(s))
		return 'Reserved username'
	return ''
}

export function validateDomain (raw: string) {
	const s = raw.trim().toLowerCase().replace(/\.$/, '')
	if (!s || s.length > 253)
		return 'Invalid domain length'
	if (/[ \t\n\r/]/.test(s))
		return 'Invalid domain characters'
	const labels = s.split('.')
	if (labels.length < 2)
		return 'Domain requires at least two labels'
	for (const label of labels) {
		if (!label || label.length > 63)
			return 'Invalid DNS label'
		if (label.startsWith('-') || label.endsWith('-'))
			return 'Invalid DNS label'
		if (!/^[a-z0-9-]+$/.test(label))
			return 'Invalid DNS label'
	}
	return ''
}

export function validateEmail (raw: string) {
	const s = raw.trim()
	if (!s) return ''
	if (!s.includes('@') || s.startsWith('@') || s.endsWith('@'))
		return 'Owner email must contain a local part and domain'
	return ''
}

export function validateOwnerPassword (raw: string) {
	if (!raw) return 'Owner password required'
	return ''
}

export interface CreateAccountInput {
	username: string
	domain: string
	email: string
	password: string
	package_id: string
}

export function validateCreateAccount (in_: CreateAccountInput) {
	const errors: Partial<Record<keyof CreateAccountInput, string>> = {}
	const username = validateUsername(in_.username)
	if (username) errors.username = username
	const domain = validateDomain(in_.domain)
	if (domain) errors.domain = domain
	const email = validateEmail(in_.email)
	if (email) errors.email = email
	const password = validateOwnerPassword(in_.password)
	if (password) errors.password = password
	if (!in_.package_id) errors.package_id = 'Package required'
	return errors
}
