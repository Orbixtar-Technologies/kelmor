import { createContext, useContext } from 'react'

const Caps = createContext<Record<string, boolean>>({})

export function CapProvider ({ caps, children }: { caps: Record<string, boolean>; children: React.ReactNode }) {
	return <Caps.Provider value={caps}>{children}</Caps.Provider>
}

export function useCan (cap: string) {
	return !!useContext(Caps)[cap]
}

export function Can ({ cap, children }: { cap: string; children: React.ReactNode }) {
	if (!useCan(cap)) return null
	return <>{children}</>
}
