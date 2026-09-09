import type { AccountDraft, AccountPayload, FieldErrors } from './types'

const usernamePattern = /^[a-z][a-z0-9_-]{2,31}$/
const domainPattern = /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/i
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function validateAccountStep (step: number, draft: AccountDraft): FieldErrors {
	const errors: FieldErrors = {}
	if (step === 1) {
		if (!usernamePattern.test(draft.username)) errors.username = 'Use 3–32 lowercase letters, numbers, underscores, or hyphens; start with a letter.'
		if (!domainPattern.test(draft.primaryDomain)) errors.primaryDomain = 'Enter a valid fully qualified domain.'
		if (draft.ownerEmail && !emailPattern.test(draft.ownerEmail)) errors.ownerEmail = 'Enter a valid email address.'
		if (draft.ownerPassword.length < 12) errors.ownerPassword = 'Use at least 12 characters.'
	}
	if (step === 2 && !draft.packageId) errors.packageId = 'Select a package.'
	return errors
}

export function buildAccountPayload (draft: AccountDraft): AccountPayload {
	return {
		username: draft.username.trim(),
		primary_domain: draft.primaryDomain.trim().toLocaleLowerCase(),
		owner_email: draft.ownerEmail.trim(),
		owner_password: draft.ownerPassword,
		package_id: draft.packageId,
		...(draft.resellerId ? { reseller_id: draft.resellerId } : {}),
	}
}
