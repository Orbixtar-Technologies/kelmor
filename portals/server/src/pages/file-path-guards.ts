const PROTECTED_NAMES = ['.ssh', 'backups', 'logs', 'mail']

export function isProtectedName (name: string): boolean {
	const base = name.replace(/\/$/, '')
	if (PROTECTED_NAMES.includes(base)) return true
	if (base.startsWith('.panel')) return true
	if (base.startsWith('.') && base !== '.' && base !== '..') return true
	return false
}

export function isProtectedPath (currentPath: string, name = ''): boolean {
	const parts = `${currentPath}/${name}`.split('/').filter(Boolean)
	return parts.some((part) => isProtectedName(part))
}

export function fileKindLabel (name: string, isDir?: boolean): string {
	if (isProtectedName(name)) return isDir ? 'Protected directory' : 'Protected file'
	return isDir ? 'Account content' : 'Account file'
}

export function protectedPathWarning (currentPath: string): string {
	if (!isProtectedPath(currentPath)) return ''
	return 'This path is system-managed. Treat it as operational metadata, not website content. Rename and delete stay disabled unless you confirm the extra risk.'
}
