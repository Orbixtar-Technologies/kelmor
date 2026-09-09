import { useCallback, useEffect, useRef, useState } from 'react'

export interface LoadState<T> {
	data: T | null
	error: string
	loading: boolean
	reload: () => Promise<void>
	setData: (value: T | null) => void
}

/**
 * Runs an async loader on mount and whenever `deps` change, keeping the last
 * good value visible while a refresh is in flight. `intervalMs` polls.
 */
export function useLoad<T> (loader: () => Promise<T>, deps: unknown[] = [], intervalMs = 0): LoadState<T> {
	const [data, setData] = useState<T | null>(null)
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(true)
	const alive = useRef(true)
	const loaderRef = useRef(loader)
	loaderRef.current = loader

	const reload = useCallback(async () => {
		try {
			const next = await loaderRef.current()
			if (!alive.current) return
			setData(next)
			setError('')
		} catch (err) {
			if (!alive.current) return
			setError(err instanceof Error ? err.message : 'Request failed')
		} finally {
			if (alive.current) setLoading(false)
		}
	}, [])

	useEffect(() => {
		alive.current = true
		setLoading(true)
		reload()
		return () => {
			alive.current = false
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, deps)

	useEffect(() => {
		if (!intervalMs) return
		const timer = window.setInterval(() => {
			reload()
		}, intervalMs)
		return () => window.clearInterval(timer)
	}, [intervalMs, reload])

	return { data, error, loading, reload, setData }
}

/** Closes a popover when focus or a click lands outside of it. */
export function useDismiss<T extends HTMLElement> (open: boolean, onDismiss: () => void) {
	const ref = useRef<T>(null)
	useEffect(() => {
		if (!open) return
		function onPointerDown (event: MouseEvent) {
			if (ref.current && !ref.current.contains(event.target as Node)) onDismiss()
		}
		function onKey (event: KeyboardEvent) {
			if (event.key === 'Escape') onDismiss()
		}
		document.addEventListener('mousedown', onPointerDown)
		document.addEventListener('keydown', onKey)
		return () => {
			document.removeEventListener('mousedown', onPointerDown)
			document.removeEventListener('keydown', onKey)
		}
	}, [open, onDismiss])
	return ref
}

const favoritesKey = 'director_favorites'

function readFavorites (): string[] {
	try {
		const raw = JSON.parse(localStorage.getItem(favoritesKey) || '[]')
		return Array.isArray(raw) ? raw.filter((v) => typeof v === 'string') : []
	} catch {
		return []
	}
}

/**
 * Pinned tools shown on Home. Stored per browser and broadcast so the Home
 * favourites panel updates the moment a page header star is toggled.
 */
export function useFavorites () {
	const [favorites, setFavorites] = useState<string[]>(readFavorites)

	useEffect(() => {
		function sync () {
			setFavorites(readFavorites())
		}
		window.addEventListener('director:favorites', sync)
		window.addEventListener('storage', sync)
		return () => {
			window.removeEventListener('director:favorites', sync)
			window.removeEventListener('storage', sync)
		}
	}, [])

	const toggle = useCallback((path: string) => {
		const current = readFavorites()
		const next = current.includes(path) ? current.filter((p) => p !== path) : [...current, path]
		localStorage.setItem(favoritesKey, JSON.stringify(next))
		window.dispatchEvent(new Event('director:favorites'))
	}, [])

	return { favorites, toggle, isFavorite: (path: string) => favorites.includes(path) }
}
