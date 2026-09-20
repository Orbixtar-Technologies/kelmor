import { formatBytes, percent } from '../helpers'

interface UsageMeterProps {
	label: string
	used?: number
	limit?: number
}

export function UsageMeter ({ label, used = 0, limit = 0 }: UsageMeterProps) {
	const pct = percent(used, limit)
	const limitLabel = limit > 0 ? formatBytes(limit) : 'No package limits'
	return (
		<div className="usage-meter" aria-label={`${label} ${formatBytes(used)} of ${limitLabel}`}>
			<progress max={100} value={Math.min(100, pct)} />
			<small>{formatBytes(used)} / {limitLabel} ({pct}%)</small>
		</div>
	)
}
