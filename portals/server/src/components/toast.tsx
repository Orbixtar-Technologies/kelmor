import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { Icon } from './icons'

export type ToastTone = 'info' | 'ok' | 'error'

interface Toast {
	id: number
	tone: ToastTone
	message: ReactNode
}

interface ToastAPI {
	notify: (message: ReactNode, tone?: ToastTone) => void
	/** Runs `work`, reporting the resolved message or the thrown error. */
	run: <T>(work: () => Promise<T>, success: (result: T) => ReactNode) => Promise<T | undefined>
}

const ToastContext = createContext<ToastAPI>({ notify: () => {}, run: async () => undefined })

export function useToast () {
	return useContext(ToastContext)
}

export function ToastProvider ({ children }: { children: ReactNode }) {
	const [toasts, setToasts] = useState<Toast[]>([])

	const dismiss = useCallback((id: number) => {
		setToasts((prev) => prev.filter((t) => t.id !== id))
	}, [])

	const notify = useCallback((message: ReactNode, tone: ToastTone = 'info') => {
		setToasts((prev) => [...prev, { id: Date.now() + Math.random(), tone, message }])
	}, [])

	const run = useCallback<ToastAPI['run']>(async (work, success) => {
		try {
			const result = await work()
			notify(success(result), 'ok')
			return result
		} catch (err) {
			notify(err instanceof Error ? err.message : 'Request failed', 'error')
			return undefined
		}
	}, [notify])

	return (
		<ToastContext.Provider value={{ notify, run }}>
			{children}
			<div className="toasts" aria-live="polite">
				{toasts.map((toast) => (
					<ToastRow key={toast.id} toast={toast} onDismiss={dismiss} />
				))}
			</div>
		</ToastContext.Provider>
	)
}

function ToastRow ({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
	useEffect(() => {
		const timer = window.setTimeout(() => onDismiss(toast.id), toast.tone === 'error' ? 9000 : 5000)
		return () => window.clearTimeout(timer)
	}, [toast, onDismiss])

	return (
		<div className={`toast ${toast.tone}`}>
			<Icon name={toast.tone === 'error' ? 'alertCircle' : toast.tone === 'ok' ? 'check' : 'info'} size={16} />
			<div>{toast.message}</div>
			<button type="button" className="close" aria-label="Dismiss" onClick={() => onDismiss(toast.id)}>
				<Icon name="x" size={13} />
			</button>
		</div>
	)
}
