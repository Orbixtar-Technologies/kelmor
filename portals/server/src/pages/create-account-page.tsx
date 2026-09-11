import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { buildAccountPayload, validateAccountStep } from '../account-wizard'
import { api, asList } from '../client'
import { ErrorState, PageHeader } from '../components/ui'
import { formatBytes, messageFrom } from '../helpers'
import { accountHomePath, provisionPipelineSteps } from './account-lifecycle-copy'
import { useCan } from '../rbac'
import type { AccountDraft, FieldErrors, Package, Reseller } from '../types'

const initialDraft: AccountDraft = { username: '', primaryDomain: '', ownerEmail: '', ownerPassword: '', packageId: '', resellerId: '' }

export function CreateAccountPage () {
	const [step, setStep] = useState(1)
	const [draft, setDraft] = useState(initialDraft)
	const [packages, setPackages] = useState<Package[]>([])
	const [resellers, setResellers] = useState<Reseller[]>([])
	const [errors, setErrors] = useState<FieldErrors>({})
	const [loadError, setLoadError] = useState('')
	const [submitting, setSubmitting] = useState(false)
	const navigate = useNavigate()
	const canChooseReseller = useCan('resellers.read')

	useEffect(() => {
		Promise.allSettled([api<{ items: Package[] }>('/api/v1/packages'), api<{ items: Reseller[] }>('/api/v1/resellers')])
			.then(([packageResult, resellerResult]) => {
				if (packageResult.status === 'rejected') { setLoadError(messageFrom(packageResult.reason)); return }
				const nextPackages = asList(packageResult.value)
				setPackages(nextPackages)
				if (resellerResult.status === 'fulfilled') setResellers(asList(resellerResult.value))
				setDraft((current) => ({ ...current, packageId: current.packageId || nextPackages[0]?.id || '' }))
			})
	}, [])

	function update (field: keyof AccountDraft, value: string) {
		setDraft((current) => ({ ...current, [field]: value }))
		setErrors((current) => ({ ...current, [field]: '' }))
	}
	function next () {
		const nextErrors = validateAccountStep(step, draft)
		setErrors(nextErrors)
		if (!Object.keys(nextErrors).length) {
			setStep(step + 1)
			return
		}
		const fieldNames: Record<string, string> = { username: 'username', primaryDomain: 'domain', ownerEmail: 'email', ownerPassword: 'password', packageId: 'package_id' }
		const first = fieldNames[Object.keys(nextErrors)[0]]
		document.querySelector<HTMLElement>(`[name="${first}"]`)?.focus()
	}
	async function submit () {
		setSubmitting(true)
		setLoadError('')
		try {
			const result = await api<{ operation_id: string }>('/api/v1/accounts', {
				method: 'POST',
				headers: { 'Idempotency-Key': crypto.randomUUID() },
				body: JSON.stringify(buildAccountPayload(draft)),
			})
			navigate(`/jobs?selected=${result.operation_id}`, { state: { message: `Account ${draft.username} queued.` } })
		} catch (error) {
			setLoadError(messageFrom(error))
			setSubmitting(false)
		}
	}
	const selectedPackage = packages.find((entry) => entry.id === draft.packageId)
	const reviewedPayload = buildAccountPayload(draft)

	return (
		<>
			<PageHeader title="Create Account" description="Review a POSIX hosting tenant before Kelmor allocates a Linux user, vhost, DNS zone, mail, and prefixed database." />
			<ol className="steps" aria-label="Account creation progress">
				{['Identity', 'Package & ownership', 'Review'].map((label, index) => <li key={label} className={step === index + 1 ? 'active' : step > index + 1 ? 'complete' : ''}><span>{index + 1}</span>{label}</li>)}
			</ol>
			{loadError ? <ErrorState title="Account could not be created" error={loadError} /> : null}
			<section className="form-panel">
				{step === 1 ? <>
					<div className="form-section-heading"><h2>Account identity</h2><p>The username becomes the Linux UID/GID and home {draft.username ? <code>{accountHomePath(draft.username)}</code> : <code>/home/&lt;username&gt;</code>}. Neighboring tenants cannot traverse that tree. Username, domain, and password are required. Owner email is optional and defaults to username@domain.</p></div>
					{Object.keys(errors).length ? <p className="error-state" role="alert">Fix the highlighted fields before continuing.</p> : null}
					<div className="form-grid">
						<label>Username <small>Required · 3–32 lowercase characters</small><input name="username" autoComplete="username" autoFocus value={draft.username} onChange={(event) => update('username', event.target.value)} aria-invalid={Boolean(errors.username)} />{errors.username ? <small className="field-error">{errors.username}</small> : null}</label>
						<label>Primary domain <small>Required</small><input name="domain" autoComplete="off" value={draft.primaryDomain} placeholder="example.com" onChange={(event) => update('primaryDomain', event.target.value)} aria-invalid={Boolean(errors.primaryDomain)} />{errors.primaryDomain ? <small className="field-error">{errors.primaryDomain}</small> : null}</label>
						<label>Owner email <small>Optional</small><input name="email" type="email" autoComplete="email" value={draft.ownerEmail} onChange={(event) => update('ownerEmail', event.target.value)} aria-invalid={Boolean(errors.ownerEmail)} />{errors.ownerEmail ? <small className="field-error">{errors.ownerEmail}</small> : null}</label>
						<label>Initial owner password <small>Required · at least 12 characters</small><input name="password" type="password" autoComplete="new-password" value={draft.ownerPassword} onChange={(event) => update('ownerPassword', event.target.value)} aria-invalid={Boolean(errors.ownerPassword)} />{errors.ownerPassword ? <small className="field-error">{errors.ownerPassword}</small> : null}</label>
					</div>
				</> : null}
				{step === 2 ? <>
					<div className="form-section-heading"><h2>Package and ownership</h2><p>Limits are enforced by the selected package.</p></div>
					<div className="form-grid">
						<label>Package<select name="package_id" value={draft.packageId} onChange={(event) => update('packageId', event.target.value)}><option value="">Select package</option>{packages.map((pkg) => <option key={pkg.id} value={pkg.id}>{pkg.name}</option>)}</select>{errors.packageId ? <small className="field-error">{errors.packageId}</small> : null}</label>
						{canChooseReseller ? <label>Reseller ownership<select name="reseller_id" value={draft.resellerId} onChange={(event) => update('resellerId', event.target.value)}><option value="">Direct Kelmor account</option>{resellers.map((reseller) => <option key={reseller.id} value={reseller.id}>{reseller.name}</option>)}</select></label> : <p className="subtle"><strong>Ownership:</strong> Your reseller account will be assigned automatically.</p>}
					</div>
					{selectedPackage ? <div className="limit-summary"><strong>{selectedPackage.name} limits</strong><span>Disk {formatBytes(selectedPackage.disk_bytes)}</span><span>Bandwidth {formatBytes(selectedPackage.bandwidth_bytes_monthly)}/month</span><span>{selectedPackage.domains} domains</span><span>{selectedPackage.mailboxes} mailboxes</span><span>CPU {selectedPackage.cpu_percent}%</span><span>Memory {formatBytes(selectedPackage.memory_bytes)}</span></div> : null}
				</> : null}
				{step === 3 ? <>
					<div className="form-section-heading"><h2>Review account</h2><p>These exact values will be submitted. Create queues a durable provision job — the account stays provisioning until Linux, DNS, mail, and database work finish.</p></div>
					<dl className="review-list">
						<div><dt>Username</dt><dd>{reviewedPayload.username}</dd></div>
						<div><dt>Linux home</dt><dd><code>{accountHomePath(reviewedPayload.username)}</code></dd></div>
						<div><dt>Submitted domain</dt><dd>{reviewedPayload.primary_domain}</dd></div>
						<div><dt>Submitted owner email</dt><dd>{reviewedPayload.owner_email}</dd></div>
						<div><dt>Package</dt><dd>{selectedPackage?.name}</dd></div><div><dt>Owner</dt><dd>{canChooseReseller ? (resellers.find((entry) => entry.id === draft.resellerId)?.name || 'Direct Kelmor account') : 'Your reseller account (assigned automatically)'}</dd></div>
						<div><dt>Initial password</dt><dd>••••••••••••</dd></div>
					</dl>
					<h3>Provisioning pipeline</h3>
					<ol className="pipeline-list">
						{provisionPipelineSteps(reviewedPayload.username, reviewedPayload.primary_domain).map((step) => <li key={step}>{step}</li>)}
					</ol>
				</> : null}
				<footer className="wizard-actions">
					<button type="button" className="secondary" disabled={step === 1 || submitting} onClick={() => setStep(step - 1)}>Back</button>
					{step < 3 ? <button type="button" onClick={next}>Continue</button> : <button type="button" disabled={submitting} onClick={submit}>{submitting ? 'Creating…' : 'Create account'}</button>}
				</footer>
			</section>
		</>
	)
}
