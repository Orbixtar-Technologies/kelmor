import { Link } from 'react-router-dom'

const widgetIcons: Record<string, string> = {
	home: '⌂', users: '👤', pause: 'Ⅱ', meter: '◔', plus: '+', box: '▣',
	briefcase: '▤', globe: '◎', pulse: '⌁', shield: '◇', transfer: '⇄',
	jobs: '≡', audit: '✓', chart: '▥', account: '◉', edit: '✎',
	trash: '×', key: '⌘', database: '▰', mail: '✉', lock: '▧',
	folder: '▤', webmail: '✉', files: '▤',
}

const widgetTones = [
	'widget-blue', 'widget-teal', 'widget-green', 'widget-purple',
	'widget-orange', 'widget-red', 'widget-slate',
]

interface WidgetCardProps {
	id: string
	label: string
	description: string
	path: string
	icon: string
	value?: string | number
	detail?: string
	toneIndex?: number
}

export function WidgetCard ({
	label,
	description,
	path,
	icon,
	value,
	detail,
	toneIndex = 0,
}: WidgetCardProps) {
	const tone = widgetTones[toneIndex % widgetTones.length]
	return (
		<Link className={`widget-card ${tone}`} to={path}>
			<span className="widget-icon" aria-hidden="true">{widgetIcons[icon] ?? '◆'}</span>
			<span className="widget-body">
				<strong>{label}</strong>
				<small>{description}</small>
				{value !== undefined ? <em>{value}</em> : null}
				{detail ? <span className="widget-detail">{detail}</span> : null}
			</span>
		</Link>
	)
}
