import { api } from './client'

export function PageHeader ({
	title,
	detail,
}: {
	title: string
	detail: string
}) {
	return (
		<header className="page-head">
			<h1>{title}</h1>
			<p>{detail}</p>
		</header>
	)
}

export function Empty ({
	title,
	detail,
}: {
	title: string
	detail: string
}) {
	return (
		<section className="empty">
			<h2>{title}</h2>
			<p>{detail}</p>
		</section>
	)
}

export function Notice ({ children }: { children: string }) {
	if (!children) return null
	return <p className="notice">{children}</p>
}

export function Metric ({
	label,
	value,
}: {
	label: string
	value: string
}) {
	return (
		<article>
			<p>{label}</p>
			<strong>{value}</strong>
		</article>
	)
}

export function Pager ({
	page,
	pages,
	total,
	onPage,
	label,
}: {
	page: number
	pages: number
	total: number
	onPage: (n: number) => void
	label: string
}) {
	return (
		<div className="pager" role="navigation" aria-label={label}>
			<button
				type="button"
				className="ghost-inline"
				disabled={page <= 1}
				onClick={() => onPage(page - 1)}
			>
				Previous
			</button>
			<span className="muted">
				{label} {page}/{pages} ({total})
			</span>
			<button
				type="button"
				className="ghost-inline"
				disabled={page >= pages || total === 0}
				onClick={() => onPage(page + 1)}
			>
				Next
			</button>
		</div>
	)
}

export function EmptyRow ({ cols, text }: { cols: number; text: string }) {
	return (
		<tr>
			<td colSpan={cols} className="muted">{text}</td>
		</tr>
	)
}

export function fmtBytes (n: number) {
	if (!n) return '0 B'
	const u = ['B', 'KB', 'MB', 'GB', 'TB']
	let i = 0
	let v = n
	while (v >= 1024 && i < u.length - 1) {
		v /= 1024
		i++
	}
	return `${v.toFixed(1)} ${u[i]}`
}

export async function act (
	path: string,
	reload: () => Promise<void>,
	setMsg: (s: string) => void,
) {
	try {
		const r = await api<any>(path, { method: 'POST', body: '{}' })
		setMsg(`Queued ${r.operation_id || 'ok'}`)
		await reload()
	} catch (e) {
		setMsg(e instanceof Error ? e.message : 'failed')
	}
}
