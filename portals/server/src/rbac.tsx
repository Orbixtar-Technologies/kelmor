import { createContext, useContext } from 'react'

const Caps = createContext<Record<string, boolean>>({})

export function CapProvider ({ caps, children }: { caps: Record<string, boolean>; children: React.ReactNode }) {
	return <Caps.Provider value={caps}>{children}</Caps.Provider>
}

export function useCan (cap: string) {
	return !!useContext(Caps)[cap]
}

export function useCapabilities () {
	return useContext(Caps)
}

export function hasCapabilities (capabilities: Record<string, boolean>, required: readonly string[]) {
	return required.every((capability) => capabilities[capability])
}

export function Forbidden ({ title }: { title: string }) {
	return (
		<section>
			<header><h1>{title}</h1><p>Your role does not include this capability.</p></header>
		</section>
	)
}
