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
