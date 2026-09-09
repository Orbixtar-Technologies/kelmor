import { useMemo, useState, type ReactNode } from 'react'
import { Icon } from './icons'
import { EmptyState, Loading } from './ui'
import { useDismiss } from '../lib/hooks'

export interface Column<T> {
	key: string
	header: string
	/** Cell renderer. Falls back to the sort value when omitted. */
	render?: (row: T) => ReactNode
	/** Value used for sorting and for the client-side search index. */
	sort?: (row: T) => string | number
	align?: 'left' | 'right'
	className?: string
	sortable?: boolean
}

export interface RowAction<T> {
	label: string
	onSelect: (row: T) => void
	danger?: boolean
	hidden?: (row: T) => boolean
}

export interface DataTableProps<T> {
	rows: T[]
	columns: Column<T>[]
	rowKey: (row: T) => string
	loading?: boolean
	error?: string
	/** Placeholder for the built-in search box. Omit to hide the box. */
	searchPlaceholder?: string
	/** Extra controls rendered in the toolbar next to search. */
	filters?: ReactNode
	actions?: ReactNode
	rowActions?: RowAction<T>[]
	initialSort?: { key: string; dir: 'asc' | 'desc' }
	pageSize?: number
	selectable?: boolean
	selected?: string[]
	onSelectedChange?: (ids: string[]) => void
	empty?: ReactNode
	/** Label for the row count in the footer, e.g. "accounts". */
	noun?: string
}

export function DataTable<T> (props: DataTableProps<T>) {
	const {
		rows, columns, rowKey, loading, error, searchPlaceholder, filters, actions, rowActions,
		initialSort, pageSize = 25, selectable, selected = [], onSelectedChange, empty, noun = 'rows',
	} = props

	const [search, setSearch] = useState('')
	const [sort, setSort] = useState(initialSort ?? { key: columns[0]?.key ?? '', dir: 'asc' as const })
	const [page, setPage] = useState(0)
	const [size, setSize] = useState(pageSize)

	const sortValue = useMemo(() => {
		const map = new Map(columns.map((c) => [c.key, c.sort]))
		return (row: T, key: string) => map.get(key)?.(row) ?? ''
	}, [columns])

	const searchable = useMemo(() => columns.filter((c) => c.sort), [columns])

	const filtered = useMemo(() => {
		const q = search.trim().toLowerCase()
		if (!q) return rows
		return rows.filter((row) => searchable.some((c) => String(c.sort!(row)).toLowerCase().includes(q)))
	}, [rows, search, searchable])

	const sorted = useMemo(() => {
		const out = [...filtered]
		out.sort((a, b) => {
			const av = sortValue(a, sort.key)
			const bv = sortValue(b, sort.key)
			let cmp = 0
			if (typeof av === 'number' && typeof bv === 'number') cmp = av - bv
			else cmp = String(av).localeCompare(String(bv), 'en', { numeric: true, sensitivity: 'base' })
			return sort.dir === 'asc' ? cmp : -cmp
		})
		return out
	}, [filtered, sort, sortValue])

	const pageCount = Math.max(1, Math.ceil(sorted.length / size))
	const current = Math.min(page, pageCount - 1)
	const visible = sorted.slice(current * size, current * size + size)
	const allOnPageSelected = visible.length > 0 && visible.every((row) => selected.includes(rowKey(row)))

	function toggleSort (key: string) {
		setSort((prev) => (prev.key === key ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }))
		setPage(0)
	}

	function toggleRow (id: string) {
		if (!onSelectedChange) return
		onSelectedChange(selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id])
	}

	function togglePageSelection () {
		if (!onSelectedChange) return
		const ids = visible.map(rowKey)
		onSelectedChange(allOnPageSelected ? selected.filter((s) => !ids.includes(s)) : [...new Set([...selected, ...ids])])
	}

	const showToolbar = !!(searchPlaceholder || filters || actions)

	return (
		<section className="panel">
			{showToolbar ? (
				<div className="table-toolbar">
					{searchPlaceholder ? (
						<input
							type="search"
							placeholder={searchPlaceholder}
							value={search}
							onChange={(e) => {
								setSearch(e.target.value)
								setPage(0)
							}}
							aria-label={searchPlaceholder}
						/>
					) : null}
					{filters}
					<span className="grow" />
					{actions}
				</div>
			) : null}

			{error ? (
				<EmptyState icon="alertCircle" title="Could not load this list">{error}</EmptyState>
			) : loading && rows.length === 0 ? (
				<Loading />
			) : sorted.length === 0 ? (
				empty ?? <EmptyState title={search ? 'Nothing matches that filter' : `No ${noun} yet`}>
					{search ? 'Clear the search box to see the full list.' : undefined}
				</EmptyState>
			) : (
				<div className="table-wrap">
					<table className="data">
						<thead>
							<tr>
								{selectable ? (
									<th style={{ width: 34 }}>
										<input
											type="checkbox"
											checked={allOnPageSelected}
											onChange={togglePageSelection}
											aria-label="Select all rows on this page"
										/>
									</th>
								) : null}
								{columns.map((column) => {
									const sortable = column.sortable !== false && !!column.sort
									return (
										<th
											key={column.key}
											className={[column.align === 'right' ? 'num' : '', sortable ? 'sortable' : ''].filter(Boolean).join(' ')}
											onClick={sortable ? () => toggleSort(column.key) : undefined}
											aria-sort={sort.key === column.key ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
										>
											{column.header}
											{sortable && sort.key === column.key ? (
												<span className="arrow">
													<Icon name={sort.dir === 'asc' ? 'chevronDown' : 'chevronRight'} size={11} />
												</span>
											) : null}
										</th>
									)
								})}
								{rowActions?.length ? <th style={{ width: 60 }} /> : null}
							</tr>
						</thead>
						<tbody>
							{visible.map((row) => {
								const id = rowKey(row)
								return (
									<tr key={id} className={selected.includes(id) ? 'selected' : undefined}>
										{selectable ? (
											<td>
												<input
													type="checkbox"
													checked={selected.includes(id)}
													onChange={() => toggleRow(id)}
													aria-label={`Select ${id}`}
												/>
											</td>
										) : null}
										{columns.map((column) => (
											<td
												key={column.key}
												className={[column.align === 'right' ? 'num' : '', column.className ?? ''].filter(Boolean).join(' ')}
											>
												{column.render ? column.render(row) : String(column.sort?.(row) ?? '')}
											</td>
										))}
										{rowActions?.length ? (
											<td>
												<RowMenu row={row} actions={rowActions} />
											</td>
										) : null}
									</tr>
								)
							})}
						</tbody>
					</table>
				</div>
			)}

			{sorted.length > 0 ? (
				<div className="table-footer">
					<span>
						Showing {current * size + 1}–{Math.min(sorted.length, current * size + size)} of {sorted.length} {noun}
						{filtered.length !== rows.length ? ` (filtered from ${rows.length})` : ''}
						{selected.length ? ` · ${selected.length} selected` : ''}
					</span>
					<div className="pager">
						<label className="small muted" htmlFor="page-size">Rows</label>
						<select
							id="page-size"
							value={size}
							onChange={(e) => {
								setSize(Number(e.target.value))
								setPage(0)
							}}
						>
							{[10, 25, 50, 100].map((n) => <option key={n} value={n}>{n}</option>)}
						</select>
						<button type="button" className="btn secondary small" disabled={current === 0} onClick={() => setPage(current - 1)}>
							<Icon name="chevronLeft" size={12} /> Previous
						</button>
						<span className="small">Page {current + 1} of {pageCount}</span>
						<button type="button" className="btn secondary small" disabled={current >= pageCount - 1} onClick={() => setPage(current + 1)}>
							Next <Icon name="chevronRight" size={12} />
						</button>
					</div>
				</div>
			) : null}
		</section>
	)
}

function RowMenu<T> ({ row, actions }: { row: T; actions: RowAction<T>[] }) {
	const [open, setOpen] = useState(false)
	const ref = useDismiss<HTMLDivElement>(open, () => setOpen(false))
	const usable = actions.filter((action) => !action.hidden?.(row))
	if (usable.length === 0) return null

	return (
		<div className="row-menu" ref={ref}>
			<button type="button" aria-haspopup="menu" aria-expanded={open} aria-label="Row actions" onClick={() => setOpen(!open)}>
				<Icon name="dots" size={15} />
			</button>
			{open ? (
				<div className="menu" role="menu">
					{usable.map((action) => (
						<button
							key={action.label}
							type="button"
							role="menuitem"
							className={action.danger ? 'menu-item danger' : 'menu-item'}
							onClick={() => {
								setOpen(false)
								action.onSelect(row)
							}}
						>
							{action.label}
						</button>
					))}
				</div>
			) : null}
		</div>
	)
}
