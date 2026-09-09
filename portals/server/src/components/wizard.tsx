import type { ReactNode } from 'react'
import { Icon } from './icons'

export interface WizardStep {
	id: string
	title: string
	/** Blocks Next until it returns an empty string. */
	validate?: () => string
	body: ReactNode
}

export interface WizardProps {
	steps: WizardStep[]
	index: number
	onIndexChange: (index: number) => void
	onSubmit: () => void
	submitLabel: string
	busy?: boolean
	error?: string
}

/**
 * Linear multi-step form with a step rail, per-step validation and a final
 * review step supplied by the caller as the last entry in `steps`.
 */
export function Wizard ({ steps, index, onIndexChange, onSubmit, submitLabel, busy, error }: WizardProps) {
	const step = steps[index]
	const isLast = index === steps.length - 1
	const blocked = step.validate?.() ?? ''

	return (
		<section className="panel">
			<ol className="steps">
				{steps.map((s, i) => (
					<li key={s.id} className={`step ${i === index ? 'current' : i < index ? 'done' : ''}`}>
						<span className="n">{i < index ? <Icon name="check" size={11} /> : i + 1}</span>
						<span>{s.title}</span>
					</li>
				))}
			</ol>
			<div className="body">
				{error ? (
					<div className="notice error" role="alert">
						<Icon name="alertCircle" size={16} className="ico" />
						<div>{error}</div>
					</div>
				) : null}
				{step.body}
				<div className="form-actions">
					<button type="button" className="btn secondary" disabled={index === 0 || busy} onClick={() => onIndexChange(index - 1)}>
						<Icon name="chevronLeft" size={13} /> Back
					</button>
					{isLast ? (
						<button type="button" className="btn" disabled={busy} onClick={onSubmit}>
							{busy ? 'Working…' : submitLabel}
						</button>
					) : (
						<button type="button" className="btn" disabled={!!blocked || busy} onClick={() => onIndexChange(index + 1)}>
							Next <Icon name="chevronRight" size={13} />
						</button>
					)}
					{blocked && !isLast ? <span className="small muted">{blocked}</span> : null}
					<span className="grow" style={{ flex: 1 }} />
					<span className="small muted">Step {index + 1} of {steps.length}</span>
				</div>
			</div>
		</section>
	)
}

export function ReviewList ({ rows }: { rows: Array<[string, ReactNode]> }) {
	return (
		<ul className="review">
			{rows.map(([key, value]) => (
				<li key={key}>
					<span className="k">{key}</span>
					<span className="v">{value ?? '—'}</span>
				</li>
			))}
		</ul>
	)
}
