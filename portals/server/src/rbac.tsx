import { createContext, useContext } from 'react'
import { Icon } from './components/icons'
import { PageHeader } from './components/ui'

const Caps = createContext<Record<string, boolean>>({})

export function CapProvider ({ caps, children }: { caps: Record<string, boolean>; children: React.ReactNode }) {
	return <Caps.Provider value={caps}>{children}</Caps.Provider>
}

export function useCaps () {
	return useContext(Caps)
}

export function useCan (cap: string) {
	return !!useContext(Caps)[cap]
}

export function Can ({ cap, children }: { cap: string; children: React.ReactNode }) {
	if (!useCan(cap)) return null
	return <>{children}</>
}

export function Forbidden ({ title, cap }: { title: string; cap?: string }) {
	return (
		<>
			<PageHeader title={title} description="This tool is not available to your role." />
			<section className="panel">
				<div className="empty">
					<Icon name="lock" size={30} className="ico" />
					<h3>Your role does not include this capability</h3>
					<p>
						Kelmor Director hides tools your account cannot use, and the control plane enforces the same check on
						every request.{cap ? ` This tool requires ${cap}.` : ''} Ask a server administrator to grant it.
					</p>
				</div>
			</section>
		</>
	)
}
