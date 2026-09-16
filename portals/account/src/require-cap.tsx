import type { ReactNode } from 'react'
import { useCan } from './rbac'

export function RequireCap ({ cap, children }: { cap: string; children: ReactNode }) {
	if (!useCan(cap)) return <p role="alert">You do not have access to this tool.</p>
	return <>{children}</>
}
